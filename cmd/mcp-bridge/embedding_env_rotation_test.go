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

func TestEmbeddingEnvReloadSameReferenceRotation(t *testing.T) {
	for _, field := range []string{"model", "base_url"} {
		t.Run(field, func(t *testing.T) {
			t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
			t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)
			hup := make(chan os.Signal, 1)
			signal.Notify(hup, syscall.SIGHUP)
			defer signal.Stop(hup)
			upstream := fakeUpstream(t, "read", `{"type":"object"}`)
			provider := fakeEmbeddings(t)
			state := t.TempDir()
			path := filepath.Join(t.TempDir(), "bridge.yaml")
			ops := freeAddress(t)
			writeBridgeConfig(t, path, freeAddress(t), ops, state, provider.URL, []bridgeServerSpec{{id: "alpha", endpoint: upstream.URL}})
			literal, err := os.ReadFile(path)
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
						t.Errorf("serve exit=%d: %s", code, stderr.String())
					}
				case <-time.After(15 * time.Second):
					t.Error("shutdown timeout")
				}
			}()
			waitFor(t, "literal ready", func() bool { status, _, ok := probeBody(t, "http://"+ops+"/readyz"); return ok && status == 200 })
			_, initialMetrics := getBody(t, "http://"+ops+"/metrics")
			if !strings.Contains(initialMetrics, `bridge_config_reload_total{outcome="success",reason="none"} 1`) {
				t.Fatalf("startup success metric missing: %s", initialMetrics)
			}
			value, old := "test-model", `model: "test-model"`
			if field == "base_url" {
				value = provider.URL
				old = fmt.Sprintf("base_url: %q", provider.URL)
			}
			t.Setenv("EMBEDDING_ROTATION_VALUE", value)
			doc := strings.Replace(string(literal), old, field+": {env: EMBEDDING_ROTATION_VALUE}", 1)
			if err := os.WriteFile(path, []byte(doc), 0600); err != nil {
				t.Fatal(err)
			}
			offset := len(out.String())
			sendReload(t)
			waitFor(t, "native same-value candidate outcome", func() bool {
				_, metrics := getBody(t, "http://"+ops+"/metrics")
				return strings.Contains(metrics, `bridge_config_reload_total{outcome="success",reason="none"} 2`) || strings.Contains(out.String()[offset:], `"outcome":"failure"`)
			})
			// This is a feature assertion; baseline must not count later checks if it fails.
			if strings.Contains(out.String()[offset:], `"outcome":"failure"`) {
				t.Fatalf("same-effective native candidate admission: %s", out.String()[offset:])
			}
			before, err := os.ReadFile(filepath.Join(state, "snapshot.json"))
			if err != nil {
				t.Fatal(err)
			}
			rotated := "ROTATED-SAME-REF-SENTINEL"
			if field == "base_url" {
				rotated = "https://rotated-same-ref.invalid/v1"
			}
			t.Setenv("EMBEDDING_ROTATION_VALUE", rotated)
			offset = len(out.String())
			sendReload(t)
			waitFor(t, "rotation rejection", func() bool { return strings.Contains(out.String()[offset:], `"event":"reload"`) })
			found := false
			for _, line := range strings.Split(strings.TrimSpace(out.String()[offset:]), "\n") {
				var event map[string]any
				if err := json.Unmarshal([]byte(line), &event); err != nil {
					t.Fatal(err)
				}
				if event["code"] == "restart_required" && event["location"] == "embedding" {
					found = true
				}
				if strings.Contains(line, rotated) {
					t.Error("rotated value leaked")
				}
			}
			if !found {
				t.Errorf("same-reference effective change not restart_required: %s", out.String()[offset:])
			}
			after, err := os.ReadFile(filepath.Join(state, "snapshot.json"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Error("rejected rotation rewrote durable snapshot")
			}
			status, _, ok := probeBody(t, "http://"+ops+"/readyz")
			if !ok || status != 200 {
				t.Error("live runtime lost after rotation")
			}
		})
	}
}
