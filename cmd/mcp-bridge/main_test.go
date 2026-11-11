package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckConfigSuccessDoesNotPrintConfigurationValues(t *testing.T) {
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
	record := assertConfigValidationSuccess(t, stdout.String())
	configRecord := record["config"].(map[string]any)
	if configRecord["path"] != configPath {
		t.Fatalf("config.path = %#v, want %q", configRecord["path"], configPath)
	}
	for _, value := range []string{"TEST_UPSTREAM_SECRET", "TEST_CLIENT_SECRET", "https://alpha.invalid/mcp"} {
		if strings.Contains(stdout.String()+stderr.String(), value) {
			t.Fatalf("success record serialized configuration value %q", value)
		}
	}
	for _, secret := range []string{"upstream-private-value", "client-private-value"} {
		if strings.Contains(stdout.String()+stderr.String(), secret) {
			t.Fatalf("output leaked secret %q", secret)
		}
	}
}

func TestCheckConfigReturnsNonzeroOnInvalidInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")
	var stdout, stderr bytes.Buffer
	if got := run([]string{"--check-config", "--config", path}, &stdout, &stderr); got != 1 {
		t.Fatalf("invalid config exit=%d, want 1", got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	record := decodeEnvelopeLine(t, stderr.String())
	if record["event"] != "config_validation_failed" || record["operation"] != "load_config" {
		t.Fatalf("event/operation = %v/%v, want config_validation_failed/load_config", record["event"], record["operation"])
	}
}
