package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddingEnvCLIProcess(t *testing.T) {
	if os.Getenv("EMBEDDING_ENV_TEST_CHILD") == "1" {
		for i, arg := range os.Args {
			if arg == "--" {
				os.Exit(run(os.Args[i+1:], os.Stdout, os.Stderr))
			}
		}
		os.Exit(2)
	}
	for _, mode := range []string{"valid", "missing", "empty"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
			t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)
			t.Setenv("EMBEDDING_PROCESS_MODEL", "PROCESS-MODEL-SENTINEL")
			t.Setenv("EMBEDDING_PROCESS_URL", "https://process-url-sentinel.invalid/v1")
			if mode == "missing" {
				if err := os.Unsetenv("EMBEDDING_PROCESS_MODEL"); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "empty" {
				t.Setenv("EMBEDDING_PROCESS_MODEL", "")
			}
			upstream := fakeUpstream(t, "read", `{"type":"object"}`)
			path := filepath.Join(t.TempDir(), "bridge.yaml")
			writeBridgeConfig(t, path, freeAddress(t), freeAddress(t), t.TempDir(), "https://literal.invalid", []bridgeServerSpec{{id: "alpha", endpoint: upstream.URL}})
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			doc := strings.Replace(string(raw), `model: "test-model"`, "model: {env: EMBEDDING_PROCESS_MODEL}", 1)
			doc = strings.Replace(doc, `base_url: "https://literal.invalid"`, "base_url: {env: EMBEDDING_PROCESS_URL}", 1)
			if err := os.WriteFile(path, []byte(doc), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestEmbeddingEnvCLIProcess$", "--", "--check-config", "--config", path)
			cmd.Env = append(os.Environ(), "EMBEDDING_ENV_TEST_CHILD=1")
			output, err := cmd.CombinedOutput()
			exit := 0
			if err != nil {
				failure, ok := err.(*exec.ExitError)
				if !ok {
					t.Fatalf("CLI subprocess setup: %v", err)
				}
				exit = failure.ExitCode()
			}
			want := 0
			if mode != "valid" {
				want = 1
			}
			if exit != want {
				t.Errorf("CLI subprocess exit=%d want=%d output=%s", exit, want, output)
			}
			if mode != "valid" {
				record := decodeEnvelopeLine(t, string(output))
				errorRecord, ok := record["error"].(map[string]any)
				if !ok || errorRecord["code"] != "reference_unresolved" {
					t.Errorf("subprocess error = %#v, want reference_unresolved", record["error"])
				}
				configRecord, ok := record["config"].(map[string]any)
				if !ok || configRecord["property_path"] != "embedding.model" || configRecord["shape"] != mode {
					t.Errorf("subprocess config = %#v, want embedding.model/%s", record["config"], mode)
				}
			} else {
				assertConfigValidationSuccess(t, string(output))
			}
			for _, sentinel := range []string{"PROCESS-MODEL-SENTINEL", "process-url-sentinel.invalid", "{env: EMBEDDING_PROCESS_MODEL}"} {
				if strings.Contains(string(output), sentinel) {
					t.Error("subprocess output leaked value/raw YAML")
				}
			}
		})
	}
}
