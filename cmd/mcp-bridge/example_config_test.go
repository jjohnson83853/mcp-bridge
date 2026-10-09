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

// TestShippedSampleValidatesWithItsReferencesResolved pins that the documented
// sample is a complete document: once its {env:}/{file:} references resolve, the
// documented --check-config command succeeds and reports only reference names,
// never a secret value.
func TestShippedSampleValidatesWithItsReferencesResolved(t *testing.T) {
	configPath := stageSample(t)
	var stdout, stderr bytes.Buffer

	if code := runContext(context.Background(), []string{"--check-config", "--config", configPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("check-config exit = %d, stderr = %q", code, stderr.String())
	}
	out := stdout.String()
	report := "configuration valid: Config{schema_version:1 servers:2 secret_refs:[env:BRIDGE_CLIENT_TOKEN env:BRIDGE_EMBEDDING_KEY env:DEVELOPER_UPSTREAM_TOKEN env:PORTAINER_UPSTREAM_TOKEN file:secrets/developer-api-key]}"
	if got := strings.TrimSpace(out); got != report {
		t.Fatalf("check-config report = %q, want %q", got, report)
	}
	for _, secret := range append([]string{sampleFileSecret}, mapValues(sampleEnv)...) {
		if strings.Contains(stdout.String()+stderr.String(), secret) {
			t.Fatalf("check-config printed the secret value %q", secret)
		}
	}
}

// TestShippedSampleFailsWithoutOneReference pins that resolution failures are
// reported as configuration errors, so a missing credential cannot pass the
// documented preflight.
func TestShippedSampleFailsWithoutOneReference(t *testing.T) {
	configPath := stageSample(t)
	t.Setenv("BRIDGE_CLIENT_TOKEN", "")
	var stdout, stderr bytes.Buffer

	if code := runContext(context.Background(), []string{"--check-config", "--config", configPath}, &stdout, &stderr); code == 0 {
		t.Fatal("check-config succeeded with an empty client token")
	}
	if !strings.Contains(stderr.String(), "invalid configuration:") {
		t.Fatalf("stderr = %q, want an invalid-configuration report", stderr.String())
	}
}

func mapValues(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}
