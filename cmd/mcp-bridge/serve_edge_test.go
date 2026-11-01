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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
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
	if code := run([]string{"--check-config", "--config", path}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "configuration valid:") {
		t.Fatalf("valid check: %d %q", code, errOut.String())
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
	// Exact attribution for the syntax failure: document key, yaml_syntax code,
	// safe available parser position, and the not_checked footer, without
	// freezing yaml.v3 prose.
	prefix := fmt.Sprintf("invalid configuration: config=%q key=%q line=", bad, "<document>")
	suffix := " code=yaml_syntax property=document shape=invalid_yaml: cannot decode YAML configuration; remaining_stages=not_checked"
	if !strings.HasPrefix(lines[0], prefix) {
		t.Fatalf("stderr = %q, want prefix %q", lines[0], prefix)
	}
	if !strings.HasSuffix(lines[0], suffix) {
		t.Fatalf("stderr = %q, want suffix %q", lines[0], suffix)
	}
	// The remaining middle text is a safe parser position (two integers).
	middle := strings.TrimSuffix(strings.TrimPrefix(lines[0], prefix), suffix)
	parts := strings.Split(middle, " column=")
	if len(parts) != 2 {
		t.Fatalf("stderr = %q, want a single parser position", lines[0])
	}
	if _, err := strconv.Atoi(parts[0]); err != nil {
		t.Fatalf("parser line %q is not an integer: %v", parts[0], err)
	}
	if _, err := strconv.Atoi(parts[1]); err != nil {
		t.Fatalf("parser column %q is not an integer: %v", parts[1], err)
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
	// Exact schema-9 attribution: single record, schema_version key, expected1
	// code/property/shape, static reason, later diagnostics deliberately
	// suppressed via the remaining-stages footer.
	want := fmt.Sprintf("configuration: config=%q key=%q code=schema_version property=schema_version shape=unsupported_integer: schema_version must equal 1; remaining_stages=not_checked", invalid, "schema_version")
	if err.Error() != want {
		t.Fatalf("invalid config error = %q, want %q", err.Error(), want)
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
	if err = serve(context.Background(), path, &out, &errOut); err == nil || !strings.Contains(err.Error(), "state directory") {
		t.Fatalf("state: %v", err)
	}
	provider := edgeConfig(t, "http://127.0.0.1:9/v1")
	data, err = os.ReadFile(provider)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte("openai-compatible"), []byte("unsupported"), 1)
	if err = os.WriteFile(provider, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err = serve(context.Background(), provider, &out, &errOut); err == nil || !strings.Contains(err.Error(), "embedding provider") {
		t.Fatalf("provider: %v", err)
	}
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
	if err = serve(context.Background(), occupied, &out, &errOut); err == nil || !strings.Contains(err.Error(), "inbound listener") {
		t.Fatalf("listener: %v", err)
	}
}
func TestRestartOnlyFieldsAndSmallWiringHelpers(t *testing.T) {
	base := config.Command{}
	cases := []struct {
		name   string
		mutate func(*config.Command)
	}{
		{"http.listen", func(c *config.Command) { c.HTTPListen = "x" }}, {"operations.listen", func(c *config.Command) { c.OperationsListen = "x" }}, {"storage.directory", func(c *config.Command) { c.StorageDirectory = "x" }}, {"storage.max_snapshot_bytes", func(c *config.Command) { c.MaxSnapshotBytes++ }}, {"catalog.refresh_interval", func(c *config.Command) { c.RefreshInterval++ }}, {"operations.requests_per_second", func(c *config.Command) { c.OpsRequestsPerSecond++ }}, {"operations.burst", func(c *config.Command) { c.OpsBurst++ }}, {"search", func(c *config.Command) { c.Search.DefaultTopK++ }}, {"embedding", func(c *config.Command) { c.Embedding.Dimensions++ }}}
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
	operatorLog(observe.NewLogger(&logs))("event")
	if !strings.Contains(logs.String(), "invalid_config") {
		t.Fatal("operator log classification")
	}
}

func TestOperatorLogPublicationFailureClassification(t *testing.T) {
	const secret = "publication-error-secret"
	var logs bytes.Buffer
	logf := operatorLog(observe.NewLogger(&logs))
	logf("configuration reload publication failed after runtime acceptance: %v", fmt.Errorf("%w: %s", service.ErrPublicationFailed, secret))
	publicationLog := logs.String()
	if !strings.Contains(publicationLog, `"reason":"internal_error"`) {
		t.Fatalf("publication log = %q; want the redacted post-accept failure class", publicationLog)
	}
	if strings.Contains(publicationLog, secret) {
		t.Fatal("publication log leaked the underlying error")
	}

	logs.Reset()
	logf("configuration reload rejected: %s", secret)
	rejectionLog := logs.String()
	if !strings.Contains(rejectionLog, `"reason":"invalid_config"`) ||
		strings.Contains(rejectionLog, `"reason":"internal_error"`) {
		t.Fatalf("candidate rejection log = %q; want invalid_config only", rejectionLog)
	}
	if strings.Contains(rejectionLog, secret) {
		t.Fatal("candidate rejection log leaked its message")
	}
}
func TestRefreshPublicationFailureClassificationThroughLoop(t *testing.T) {
	const secret = "refresh-publication-error-secret"
	var logs bytes.Buffer
	operator := operatorLog(observe.NewLogger(&logs))
	logged := make(chan struct{}, 1)
	logf := func(format string, args ...any) {
		operator(format, args...)
		logged <- struct{}{}
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
	if !strings.Contains(publicationLog, `"reason":"internal_error"`) {
		t.Fatalf("refresh publication log = %q; want internal_error", publicationLog)
	}
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
