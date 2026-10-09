package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckConfigPrintsReferenceNamesWithoutValues(t *testing.T) {
	t.Setenv("TEST_UPSTREAM_SECRET", "upstream-private-value")
	t.Setenv("TEST_CLIENT_SECRET", "client-private-value")
	configPath := filepath.Join(t.TempDir(), "bridge.yaml")
	content := `schema_version: 1
servers:
  - id: alpha
    endpoint: https://alpha.invalid/mcp
    credentials:
      bearer: {env: TEST_UPSTREAM_SECRET}
client_auth:
  bearer_tokens:
    - {env: TEST_CLIENT_SECRET}
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if got := run([]string{"--check-config", "--config", configPath}, &stdout, &stderr); got != 0 {
		t.Fatalf("run status=%d stderr=%s", got, stderr.String())
	}
	for _, text := range []string{"TEST_UPSTREAM_SECRET", "TEST_CLIENT_SECRET"} {
		if !strings.Contains(stdout.String(), text) {
			t.Fatalf("output omitted reference name %q: %s", text, stdout.String())
		}
	}
	for _, secret := range []string{"upstream-private-value", "client-private-value"} {
		if strings.Contains(stdout.String()+stderr.String(), secret) {
			t.Fatalf("output leaked secret %q", secret)
		}
	}
}

func TestCheckConfigReturnsNonzeroOnInvalidInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := run([]string{"--check-config", "--config", filepath.Join(t.TempDir(), "missing.yaml")}, &stdout, &stderr); got == 0 {
		t.Fatal("invalid config returned success")
	}
}
