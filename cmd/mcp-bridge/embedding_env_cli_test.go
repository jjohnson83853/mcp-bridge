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
				assertConfigValidationSuccess(t, out.String())
				if stderr.Len() != 0 {
					t.Errorf("valid CLI stderr = %q, want empty", stderr.String())
				}
			} else {
				if exit != 1 {
					t.Errorf("invalid CLI exit=%d want=1", exit)
				}
				record := decodeEnvelopeLine(t, stderr.String())
				errorRecord, ok := record["error"].(map[string]any)
				if !ok || errorRecord["code"] != "reference_unresolved" {
					t.Errorf("CLI error = %#v, want reference_unresolved", record["error"])
				}
				configRecord, ok := record["config"].(map[string]any)
				if !ok || configRecord["property_path"] != "embedding.model" || configRecord["shape"] != mode {
					t.Errorf("CLI config attribution = %#v, want embedding.model/%s", record["config"], mode)
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
