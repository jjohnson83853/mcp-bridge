package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddingEnvCLI(t *testing.T) {
	for _, mode := range []string{"valid", "missing", "empty"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
			t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)
			t.Setenv("EMBEDDING_CLI_MODEL", "MODEL-SENTINEL-CLI")
			t.Setenv("EMBEDDING_CLI_URL", "https://url-sentinel-cli.invalid/v1")
			if mode == "missing" {
				if err := os.Unsetenv("EMBEDDING_CLI_MODEL"); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "empty" {
				t.Setenv("EMBEDDING_CLI_MODEL", "")
			}
			upstream := fakeUpstream(t, "read", `{"type":"object"}`)
			path := filepath.Join(t.TempDir(), "bridge.yaml")
			writeBridgeConfig(t, path, freeAddress(t), freeAddress(t), t.TempDir(), "https://literal.invalid", []bridgeServerSpec{{id: "alpha", endpoint: upstream.URL}})
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			text := strings.Replace(string(data), `model: "test-model"`, "model: {env: EMBEDDING_CLI_MODEL}", 1)
			text = strings.Replace(text, `base_url: "https://literal.invalid"`, "base_url: {env: EMBEDDING_CLI_URL}", 1)
			if err := os.WriteFile(path, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			var out, stderr bytes.Buffer
			exit := runContext(context.Background(), []string{"--check-config", "--config", path}, &out, &stderr)
			if mode == "valid" {
				if exit != 0 {
					t.Errorf("native CLI admission exit=%d stderr=%s", exit, stderr.String())
				}
			} else {
				if exit != 1 {
					t.Errorf("invalid CLI exit=%d want=1", exit)
				}
				for _, fragment := range []string{`key="embedding.model"`, "code=reference_unresolved", "shape=" + mode} {
					if !strings.Contains(stderr.String(), fragment) {
						t.Errorf("CLI metadata missing %s: %s", fragment, stderr.String())
					}
				}
			}
			for _, sentinel := range []string{"MODEL-SENTINEL-CLI", "url-sentinel-cli.invalid", "{env: EMBEDDING_CLI_MODEL}"} {
				if strings.Contains(out.String()+stderr.String(), sentinel) {
					t.Error("CLI leaked value/raw YAML")
				}
			}
		})
	}
}
