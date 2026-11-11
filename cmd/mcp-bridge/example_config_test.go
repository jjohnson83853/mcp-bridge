package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// samplePath is the shipped configuration sample, relative to this package.
var samplePath = filepath.Join("..", "..", "configs", "config.example.yaml")

// sampleEnv holds the environment references the shipped sample names, each with
// a canary value that must never reach the console.
var sampleEnv = map[string]string{
	"BRIDGE_EMBEDDING_KEY":     "embedding-canary-4f1a",
	"BRIDGE_CLIENT_TOKEN":      "client-canary-4f1b",
	"DEVELOPER_UPSTREAM_TOKEN": "developer-canary-4f1c",
	"PORTAINER_UPSTREAM_TOKEN": "portainer-canary-4f1d",
}

const sampleFileSecret = "file-secret-canary-4f1e"

// stageSample copies the shipped sample into a private directory and materialises
// every reference it names, so the document can be validated end to end without
// touching the repository or any real credential.
func stageSample(t *testing.T) string {
	t.Helper()
	sample, err := os.ReadFile(samplePath)
	if err != nil {
		t.Fatalf("read shipped sample: %v", err)
	}
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, sample, 0o600); err != nil {
		t.Fatal(err)
	}
	// The sample's {file:} reference is relative to the document directory.
	secretDir := filepath.Join(dir, "secrets")
	if err := os.Mkdir(secretDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secretDir, "developer-api-key"), []byte(sampleFileSecret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, value := range sampleEnv {
		t.Setenv(name, value)
	}
	return configPath
}

// TestExampleConfigPassesCheck pins that the documented sample is a complete
// document: once its {env:}/{file:} references resolve, the documented
// --check-config command emits a success envelope without configuration values.
func TestExampleConfigPassesCheck(t *testing.T) {
	configPath := stageSample(t)
	var stdout, stderr bytes.Buffer

	if code := runContext(context.Background(), []string{"--check-config", "--config", configPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("check-config exit = %d, stderr = %q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("check-config stderr = %q, want empty", stderr.String())
	}
	record := assertConfigValidationSuccess(t, stdout.String())
	configRecord := record["config"].(map[string]any)
	if configRecord["path"] != configPath {
		t.Fatalf("config.path = %#v, want %q", configRecord["path"], configPath)
	}
	for _, value := range []string{"BRIDGE_CLIENT_TOKEN", "BRIDGE_EMBEDDING_KEY", "DEVELOPER_UPSTREAM_TOKEN", "PORTAINER_UPSTREAM_TOKEN", "developer-api-key"} {
		if strings.Contains(stdout.String(), value) {
			t.Fatalf("check-config serialized configuration value %q", value)
		}
	}
	for _, secret := range append([]string{sampleFileSecret}, mapValues(sampleEnv)...) {
		if strings.Contains(stdout.String()+stderr.String(), secret) {
			t.Fatalf("check-config printed the secret value %q", secret)
		}
	}
}

// TestShippedSampleFailsWithoutOneReference pins that resolution failures are
// reported as configuration errors, so a missing credential cannot pass the
// documented preflight. An unset environment variable must NOT be relabelled
// empty, and a t.Setenv-empty variable must NOT be relabelled unset.
func TestShippedSampleFailsWithoutOneReference(t *testing.T) {
	configPath := stageSample(t)
	t.Setenv("BRIDGE_CLIENT_TOKEN", "")
	var stdout, stderr bytes.Buffer

	code := runContext(context.Background(), []string{"--check-config", "--config", configPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("check-config exit = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	lines := strings.Split(strings.TrimSuffix(stderr.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("stderr lines = %d (%q), want 1", len(lines), stderr.String())
	}
	record := decodeEnvelopeLine(t, stderr.String())
	errorRecord, ok := record["error"].(map[string]any)
	if !ok || errorRecord["code"] != "secret_empty" {
		t.Fatalf("error = %#v, want secret_empty", record["error"])
	}
	configRecord, ok := record["config"].(map[string]any)
	if !ok {
		t.Fatalf("config = %#v, want object", record["config"])
	}
	if configRecord["path"] != configPath || configRecord["property_path"] != "client_auth.bearer_tokens[0]" ||
		configRecord["ref_kind"] != "env" || configRecord["ref_name"] != "BRIDGE_CLIENT_TOKEN" ||
		configRecord["shape"] != "empty" {
		t.Fatalf("config attribution = %#v", configRecord)
	}
}

func mapValues(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}
