package main

import (
	"bytes"
	"context"
	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if code := run([]string{"--check-config", "--config", bad}, &out, &errOut); code == 0 || !strings.Contains(errOut.String(), "invalid configuration:") {
		t.Fatalf("invalid check: %d %q", code, errOut.String())
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
	if err := serve(context.Background(), invalid, &out, &errOut); err == nil || !strings.Contains(err.Error(), "configuration:") {
		t.Fatalf("invalid config: %v", err)
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
