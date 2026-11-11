package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
)

func edgeConfig(t *testing.T, embeddingURL string) string {
	t.Helper()
	t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
	t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeBridgeConfig(t, path, "127.0.0.1:0", "127.0.0.1:0", filepath.Join(t.TempDir(), "state"), embeddingURL, []bridgeServerSpec{{id: "edge", endpoint: "http://127.0.0.1:9"}})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte("servers:\npins:"), []byte("servers: []\npins:"), 1)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestRunCheckConfigAndMissingFile(t *testing.T) {
	path := edgeConfig(t, "http://127.0.0.1:9/v1")
	var out, errOut bytes.Buffer
	if code := run([]string{"--check-config", "--config", path}, &out, &errOut); code != 0 {
		t.Fatalf("valid check exit = %d, stderr=%q", code, errOut.String())
	}
	assertConfigValidationSuccess(t, out.String())
	if errOut.Len() != 0 {
		t.Fatalf("valid check stderr = %q, want empty", errOut.String())
	}
	bad := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(bad, []byte("schema_version: [bad"), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errOut.Reset()
	code := run([]string{"--check-config", "--config", bad}, &out, &errOut)
	if code != 1 {
		t.Fatalf("invalid check exit = %d, want 1", code)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
	lines := strings.Split(strings.TrimSuffix(errOut.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("stderr lines = %d (%q), want 1", len(lines), errOut.String())
	}
	record := decodeEnvelopeLine(t, errOut.String())
	if record["event"] != "config_validation_failed" || record["operation"] != "load_config" {
		t.Fatalf("event/operation = %v/%v, want config_validation_failed/load_config", record["event"], record["operation"])
	}
	errorRecord, ok := record["error"].(map[string]any)
	if !ok || errorRecord["code"] != "yaml_syntax" {
		t.Fatalf("error = %#v, want yaml_syntax", record["error"])
	}
	configRecord, ok := record["config"].(map[string]any)
	if !ok || configRecord["path"] != bad || configRecord["property_path"] != "<document>" {
		t.Fatalf("config = %#v, want the supplied path and document property", record["config"])
	}
	line, lineOK := configRecord["line"].(float64)
	if !lineOK || line < 1 {
		t.Fatalf("parser line = %v, want one-based integer", configRecord["line"])
	}
	if _, exists := configRecord["column"]; exists {
		t.Errorf("column = %v, want omitted when parser provides no column", configRecord["column"])
	}
	if configRecord["remaining_stages"] != "not_checked" {
		t.Fatalf("remaining_stages = %v, want not_checked", configRecord["remaining_stages"])
	}
	if code := runContext(context.Background(), []string{"--config", filepath.Join(t.TempDir(), "absent")}, &out, &errOut); code == 0 {
		t.Fatal("missing config succeeded")
	}
}
func TestServeStartupFailures(t *testing.T) {
	t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
	t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)
	var out, errOut bytes.Buffer
	invalid := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(invalid, []byte("schema_version: 9"), 0600); err != nil {
		t.Fatal(err)
	}
	err := serve(context.Background(), invalid, &out, &errOut)
	if err == nil {
		t.Fatal("invalid config succeeded")
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
	var diagnostics *config.Diagnostics
	if !errors.As(err, &diagnostics) || diagnostics == nil {
		t.Fatalf("invalid config error type=%T, want *config.Diagnostics", err)
	}
	path := edgeConfig(t, "http://127.0.0.1:9/v1")
	cfg, err := config.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := cfg.Command()
	if err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err = os.WriteFile(blocked, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte(settings.StorageDirectory), []byte(blocked), 1)
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err = serve(context.Background(), path, &out, &errOut); err == nil {
		t.Fatal("state directory failure was not returned")
	}
	assertStartupFailure(t, err, "startup_failed", "state_directory", "resource", false, map[string]string{"path": blocked})
	provider := edgeConfig(t, "http://127.0.0.1:9/v1")
	data, err = os.ReadFile(provider)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte("openai-compatible"), []byte("unsupported"), 1)
	if err = os.WriteFile(provider, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err = serve(context.Background(), provider, &out, &errOut); err == nil {
		t.Fatal("embedding provider failure was not returned")
	}
	assertStartupFailure(t, err, "startup_failed", "embedding_provider", "dependency", false, nil)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	_, port, _ := net.SplitHostPort(l.Addr().String())
	occupied := edgeConfig(t, "http://127.0.0.1:9/v1")
	data, err = os.ReadFile(occupied)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte("127.0.0.1:0"), []byte("127.0.0.1:"+port), 1)
	if err = os.WriteFile(occupied, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err = serve(context.Background(), occupied, &out, &errOut); err == nil {
		t.Fatal("inbound listener failure was not returned")
	}
	assertStartupFailure(t, err, "startup_failed", "listener_bind", "resource", true, map[string]string{"listener": "inbound", "address": "127.0.0.1:" + port})
	startupLogs := &bytes.Buffer{}
	loop, err := service.NewLoop(service.NewRuntime(), service.LoopOptions{
		Load: func() (*config.Config, error) { return &config.Config{}, nil },
		Deps: service.RefreshDeps{ReloadDeps: service.ReloadDeps{
			Discover:            func(config.Server) ([]core.ToolDefinition, error) { return nil, nil },
			NewClient:           func(config.Server) (core.Upstream, error) { return nil, nil },
			ModelIdentityDigest: strings.Repeat("a", 64),
			Now:                 func() time.Time { return time.Unix(1, 0) },
		}},
		Published: func(*service.Assembly) error { return errors.New("startup publication secret") },
		Logf:      operatorLog(startupLogs),
	})
	if err != nil {
		t.Fatalf("new startup publication loop: %v", err)
	}
	startErr := loop.Start(context.Background())
	if !errors.Is(startErr, service.ErrPublicationFailed) {
		t.Fatalf("loop startup error=%v, want publication failure", startErr)
	}
	if startupLogs.Len() != 0 {
		t.Fatalf("loop.Start emitted a second publication record: %q", startupLogs.String())
	}
	failure := startupFailureForError(startErr)
	assertStartupFailure(t, failure, "startup_publication_failed", "publication_failed", "internal", true, nil)
	writeStartupFailure(startupLogs, failure)
	lines := strings.Split(strings.TrimSuffix(startupLogs.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("startup publication records=%d, want exactly one: %q", len(lines), startupLogs.String())
	}
	record := decodeEnvelopeLine(t, lines[0]+"\n")
	if record["event"] != "startup_publication_failed" {
		t.Fatalf("startup publication event=%v, want startup_publication_failed", record["event"])
	}
	if strings.Contains(startupLogs.String(), "startup publication secret") ||
		strings.Contains(startupLogs.String(), "reload_publication_failed") {
		t.Fatalf("startup publication output leaked or used another event: %q", startupLogs.String())
	}
}
func TestRestartOnlyFieldsAndSmallWiringHelpers(t *testing.T) {
	base := config.Command{}
	cases := []struct {
		name   string
		mutate func(*config.Command)
	}{
		{"http.listen", func(c *config.Command) { c.HTTPListen = "x" }}, {"operations.listen", func(c *config.Command) { c.OperationsListen = "x" }}, {"storage.directory", func(c *config.Command) { c.StorageDirectory = "x" }}, {"storage.max_snapshot_bytes", func(c *config.Command) { c.MaxSnapshotBytes++ }}, {"catalog.refresh_interval", func(c *config.Command) { c.RefreshInterval++ }}, {"operations.requests_per_second", func(c *config.Command) { c.OpsRequestsPerSecond++ }}, {"operations.burst", func(c *config.Command) { c.OpsBurst++ }}, {"search", func(c *config.Command) { c.Search.DefaultTopK++ }}, {"embedding", func(c *config.Command) { c.Embedding.Dimensions++ }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			tc.mutate(&c)
			if got := restartOnlyField(base, c); got != tc.name {
				t.Fatalf("got %q", got)
			}
		})
	}
	h := newConfigHolder()
	candidate := &config.Config{}
	h.stage(candidate, base)
	if h.publishedSettings() != nil || h.settings().HTTPListen != base.HTTPListen {
		t.Fatal("stage published prematurely")
	}
	h.commit()
	if h.publishedSettings() == nil {
		t.Fatal("commit failed")
	}
	if ids := configuredServerIDs(&config.Config{}); len(ids) != 0 {
		t.Fatalf("ids=%v", ids)
	}
	if orNone("") != "none" || orNone("bad") != "bad" {
		t.Fatal("orNone")
	}
	var logs bytes.Buffer
	operatorLog(&logs)("event")
	record := decodeEnvelopeLine(t, logs.String())
	assertOperatorEnvelope(t, record, "warn", "internal_warning", "reload", "failure", "bridge encountered an internal warning", "internal_error", "internal", false, "inspect the bridge logs and configuration")
}

func TestOperatorLogPublicationFailureClassification(t *testing.T) {
	const secret = "publication-error-secret"
	var stdout, logs bytes.Buffer
	logf := operatorLog(&logs)
	logf("configuration reload publication failed after runtime acceptance: %v", fmt.Errorf("%w: %s", service.ErrPublicationFailed, secret))
	publicationLog := logs.String()
	if stdout.Len() != 0 {
		t.Fatalf("warn publication record went to stdout: %q", stdout.String())
	}
	publication := decodeEnvelopeLine(t, publicationLog)
	assertOperatorEnvelope(t, publication, "warn", "reload_publication_failed", "publish", "failure", "configuration reload publication failed", "internal_error", "internal", false, "inspect the bridge logs and configuration")
	if strings.Contains(publicationLog, secret) {
		t.Fatal("publication log leaked the underlying error")
	}

	logs.Reset()
	logf("configuration reload rejected: %v", errors.New(secret))
	rejectionLog := logs.String()
	rejection := decodeEnvelopeLine(t, rejectionLog)
	assertOperatorEnvelope(t, rejection, "warn", "reload_rejected", "reload", "failure", "configuration reload rejected", "internal_error", "internal", false, "inspect the bridge logs and configuration")
	if strings.Contains(rejectionLog, secret) {
		t.Fatal("candidate rejection log leaked its message")
	}
}
func TestRefreshPublicationFailureClassificationThroughLoop(t *testing.T) {
	const secret = "refresh-publication-error-secret"
	var logs bytes.Buffer
	operator := operatorLog(&logs)
	logged := make(chan struct{}, 1)
	logf := func(format string, args ...any) {
		operator(format, args...)
		select {
		case logged <- struct{}{}:
		default:
		}
	}

	publicationCalls := 0
	loop, err := service.NewLoop(service.NewRuntime(), service.LoopOptions{
		Load: func() (*config.Config, error) { return &config.Config{}, nil },
		Deps: service.RefreshDeps{ReloadDeps: service.ReloadDeps{
			Discover:            func(config.Server) ([]core.ToolDefinition, error) { return nil, nil },
			NewClient:           func(config.Server) (core.Upstream, error) { return nil, nil },
			ModelIdentityDigest: strings.Repeat("a", 64),
			Now:                 func() time.Time { return time.Unix(1, 0) },
		}},
		Published: func(*service.Assembly) error {
			publicationCalls++
			if publicationCalls == 2 {
				return errors.New(secret)
			}
			return nil
		},
		Logf: logf,
	})
	if err != nil {
		t.Fatalf("new loop: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- loop.Start(ctx) }()
	select {
	case <-logged:
	case err := <-done:
		t.Fatalf("loop exited before the refresh publication log: %v", err)
	case <-time.After(5 * time.Second):
		cancel()
		<-done
		t.Fatal("timed out waiting for the refresh publication log")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("loop shutdown: %v", err)
	}

	publicationLog := logs.String()
	if strings.Count(publicationLog, "\n") != 1 {
		t.Fatalf("refresh publication records=%d, want exactly one: %q", strings.Count(publicationLog, "\n"), publicationLog)
	}
	publication := decodeEnvelopeLine(t, publicationLog)
	assertOperatorEnvelope(t, publication, "warn", "reload_publication_failed", "publish", "failure", "configuration reload publication failed", "internal_error", "internal", false, "inspect the bridge logs and configuration")
	if strings.Contains(publicationLog, secret) {
		t.Fatalf("refresh publication log leaked its underlying callback error: %q", publicationLog)
	}
	if publicationCalls != 2 {
		t.Fatalf("publication calls = %d; want startup and refresh callbacks", publicationCalls)
	}
}

func TestPublishedFailureKeepsAcceptedAdmissionBounds(t *testing.T) {
	runtime := service.NewRuntime()
	accepted, err := runtime.Reload(&config.Config{}, service.ReloadDeps{
		Discover: func(config.Server) ([]core.ToolDefinition, error) {
			return nil, nil
		},
		NewClient: func(config.Server) (core.Upstream, error) {
			return nil, nil
		},
		ModelIdentityDigest: strings.Repeat("a", 64),
		Now:                 func() time.Time { return time.Unix(1, 0) },
	})
	if err != nil {
		t.Fatalf("accept runtime assembly: %v", err)
	}
	if runtime.Current() != accepted {
		t.Fatal("runtime did not retain the accepted assembly")
	}

	holder := newConfigHolder()
	previousSettings := config.Command{
		MaxRequestBytes: 1024,
		MaxSessions:     1,
		SessionIdle:     time.Second,
		AllowedHosts:    []string{"previous.example"},
		AllowedOrigins:  []string{"https://previous.example"},
	}
	acceptedSettings := config.Command{
		MaxRequestBytes: 4096,
		MaxSessions:     3,
		SessionIdle:     2 * time.Second,
		AllowedHosts:    []string{"accepted.example"},
		AllowedOrigins:  []string{"https://accepted.example"},
	}
	holder.stage(&config.Config{}, previousSettings)
	holder.commit()
	holder.stage(&config.Config{}, acceptedSettings)

	projectionErr := errors.New("projection failed")
	err = publishAcceptedAssembly(
		accepted,
		func(*service.Assembly) {},
		func(*service.Assembly) error { return projectionErr },
		func(*service.Assembly) { holder.commit() },
	)
	if !errors.Is(err, projectionErr) {
		t.Fatalf("publication error = %v, want %v", err, projectionErr)
	}
	if runtime.Current() != accepted {
		t.Fatal("runtime no longer matches the accepted assembly")
	}

	wantBounds := service.InboundBounds{
		MaxRequestBytes: acceptedSettings.MaxRequestBytes,
		MaxSessions:     acceptedSettings.MaxSessions,
		SessionIdle:     acceptedSettings.SessionIdle,
		AllowedHosts:    acceptedSettings.AllowedHosts,
		AllowedOrigins:  acceptedSettings.AllowedOrigins,
	}
	if got := holder.bounds(); !reflect.DeepEqual(got, wantBounds) {
		t.Fatalf("admission bounds = %#v, want accepted assembly bounds %#v", got, wantBounds)
	}
}
