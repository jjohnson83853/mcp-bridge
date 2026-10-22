package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Begin from a legal literal runtime: native candidate diagnostics are assertions,
// not startup setup failures. Each case has its own process-env lifetime.
func TestEmbeddingEnvReloadBaseline(t *testing.T) {
	for _, field := range []string{"model", "base_url"} {
		for _, mode := range []string{"changed", "missing", "empty"} {
			t.Run(field+"/"+mode, func(t *testing.T) {
				t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
				t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)
				t.Setenv("EMBEDDING_RELOAD_VALUE", "")
				hup := make(chan os.Signal, 1)
				signal.Notify(hup, syscall.SIGHUP)
				defer signal.Stop(hup)
				upstream := fakeUpstream(t, "read", `{"type":"object"}`)
				provider := fakeEmbeddings(t)
				dir := t.TempDir()
				path := filepath.Join(dir, "bridge.yaml")
				ops := freeAddress(t)
				writeBridgeConfig(t, path, freeAddress(t), ops, filepath.Join(dir, "state"), provider.URL, []bridgeServerSpec{{id: "alpha", endpoint: upstream.URL}})
				original, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				out, stderr := &syncWriter{}, &syncWriter{}
				done := make(chan int, 1)
				go func() { done <- runContext(ctx, []string{"--config", path}, out, stderr) }()
				defer func() {
					cancel()
					select {
					case code := <-done:
						if code != 0 {
							t.Errorf("serve exit=%d stderr=%s", code, stderr.String())
						}
					case <-time.After(15 * time.Second):
						t.Error("serve failed to stop")
					}
				}()
				waitFor(t, "literal runtime ready", func() bool { status, _, ok := probeBody(t, "http://"+ops+"/readyz"); return ok && status == 200 })
				snapshotBefore, err := os.ReadFile(filepath.Join(dir, "state", "snapshot.json"))
				if err != nil {
					t.Fatal(err)
				}
				beforeStatus, before := getBody(t, "http://"+ops+"/metrics")
				if beforeStatus != 200 {
					t.Fatal("metrics unavailable")
				}
				if mode == "missing" {
					if err := os.Unsetenv("EMBEDDING_RELOAD_VALUE"); err != nil {
						t.Fatal(err)
					}
				} else if mode == "changed" {
					value := "ROTATED-MODEL-SENTINEL"
					if field == "base_url" {
						value = "https://rotated-sentinel.invalid/v1"
					}
					t.Setenv("EMBEDDING_RELOAD_VALUE", value)
				}
				old := `model: "test-model"`
				if field == "base_url" {
					old = fmt.Sprintf("base_url: %q", provider.URL)
				}
				document := strings.Replace(string(original), old, field+": {env: EMBEDDING_RELOAD_VALUE}", 1)
				if document == string(original) {
					t.Fatal("fixture replacement failed")
				}
				if err := os.WriteFile(path, []byte(document), 0600); err != nil {
					t.Fatal(err)
				}
				offset := len(out.String())
				sendReload(t)
				waitFor(t, "rejected reload emission", func() bool { return strings.Contains(out.String()[offset:], "reload") })
				code, shape, location := "reference_unresolved", mode, "embedding."+field
				if mode == "changed" {
					code = "restart_required"
					shape = "immutable"
					location = "embedding"
				}
				found := false
				for _, line := range strings.Split(strings.TrimSpace(out.String()[offset:]), "\n") {
					if line == "" {
						continue
					}
					var event map[string]any
					if err := json.Unmarshal([]byte(line), &event); err != nil {
						t.Fatalf("invalid structured sink record: %v", err)
					}
					if event["event"] == "reload" && (event["level"] != "warn" || event["outcome"] != "failure") {
						t.Error("reload failure envelope changed")
					}
					if event["code"] == code && event["shape"] == shape && event["location"] == location && event["config_path"] == path {
						found = true
					}
					if stamp, ok := event["time"].(string); ok {
						if _, err := time.Parse(time.RFC3339Nano, stamp); err != nil {
							t.Error("invalid timestamp")
						}
					}
					for _, sentinel := range []string{"ROTATED-MODEL-SENTINEL", "rotated-sentinel.invalid", "{env: EMBEDDING_RELOAD_VALUE}"} {
						if strings.Contains(line, sentinel) {
							t.Error("reload sink leaked value/raw YAML")
						}
					}
				}
				if !found {
					t.Errorf("want attributed code=%s shape=%s; sink=%s", code, shape, out.String()[offset:])
				}
				status, _, ok := probeBody(t, "http://"+ops+"/readyz")
				if !ok || status != 200 {
					t.Error("rejected candidate disturbed live readiness")
				}
				snapshotAfter, err := os.ReadFile(filepath.Join(dir, "state", "snapshot.json"))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(snapshotBefore, snapshotAfter) {
					t.Error("invalid candidate changed durable live state")
				}
				_, after := getBody(t, "http://"+ops+"/metrics")
				series := `bridge_config_reload_total{outcome="rejected",reason="invalid_configuration"} 1`
				if strings.Contains(before, series) || !strings.Contains(after, series) {
					t.Errorf("one rejected reload delta missing: %s", after)
				}
				if strings.Contains(after, "EMBEDDING_RELOAD_VALUE") || strings.Contains(after, "SENTINEL") {
					t.Error("env-derived metric label/value")
				}
			})
		}
	}
}
