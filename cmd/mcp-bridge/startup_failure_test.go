package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
)

func TestStartupFailureRecords(t *testing.T) {
	tests := []struct {
		name        string
		failure     startupFailure
		event       string
		operation   string
		code        string
		kind        string
		retryable   bool
		remediation string
		resource    map[string]string
	}{
		{name: "state directory", failure: stateDirectoryFailure("/tmp/state"), event: "startup_failed", operation: "startup", code: "state_directory", kind: "resource", remediation: "create the configured state directory or fix its permissions", resource: map[string]string{"path": "/tmp/state"}},
		{name: "JWKS cache", failure: jwksCacheFailure(), event: "startup_failed", operation: "startup", code: "jwks_cache", kind: "dependency", retryable: true, remediation: "verify the identity provider URL and network reachability, then restart"},
		{name: "embedding provider", failure: embeddingProviderFailure(), event: "startup_failed", operation: "startup", code: "embedding_provider", kind: "dependency", remediation: "fix the embedding provider configuration or credentials"},
		{name: "tool surface", failure: toolSurfaceFailure(), event: "startup_failed", operation: "startup", code: "tool_surface", kind: "internal", remediation: "report this failure; the tool surface failed to initialize"},
		{name: "catalog loop", failure: catalogLoopFailure(), event: "startup_failed", operation: "startup", code: "catalog_loop", kind: "internal", remediation: "report this failure; the catalog loop failed to start"},
		{name: "inbound listener", failure: listenerBindFailure("inbound", "127.0.0.1:8000"), event: "startup_failed", operation: "startup", code: "listener_bind", kind: "resource", retryable: true, remediation: "free the configured address or change the listen setting", resource: map[string]string{"listener": "inbound", "address": "127.0.0.1:8000"}},
		{name: "operations listener", failure: listenerBindFailure("operations", "127.0.0.1:8001"), event: "startup_failed", operation: "startup", code: "listener_bind", kind: "resource", retryable: true, remediation: "free the configured address or change the listen setting", resource: map[string]string{"listener": "operations", "address": "127.0.0.1:8001"}},
		{name: "CLI usage", failure: cliUsageFailure(), event: "cli_usage_failed", operation: "cli", code: "cli_usage", kind: "usage", remediation: "use valid flags and arguments; run mcp-bridge --help for usage"},
		{name: "startup publication", failure: startupPublicationFailure(), event: "startup_publication_failed", operation: "startup", code: "publication_failed", kind: "internal", retryable: true, remediation: "verify the startup catalog and restart the service"},
		{name: "fallback", failure: fallbackStartupFailure(), event: "startup_failed", operation: "startup", code: "internal_error", kind: "internal", remediation: "report this failure; no specific class matched"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer
			writeStartupFailure(&stderr, test.failure)
			lines := strings.Split(strings.TrimSuffix(stderr.String(), "\n"), "\n")
			if len(lines) != 1 {
				t.Fatalf("records=%d, want exactly one: %q", len(lines), stderr.String())
			}
			record := decodeEnvelopeLine(t, lines[0]+"\n")
			if record["event"] != test.event || record["operation"] != test.operation ||
				record["level"] != "error" || record["outcome"] != "failure" || record["message"] == "" {
				t.Fatalf("envelope=%#v", record)
			}
			errorInfo, ok := record["error"].(map[string]any)
			if !ok || errorInfo["code"] != test.code || errorInfo["kind"] != test.kind ||
				errorInfo["retryable"] != test.retryable || errorInfo["remediation"] != test.remediation {
				t.Fatalf("error info=%#v", record["error"])
			}
			resource, hasResource := record["resource"].(map[string]any)
			if len(test.resource) == 0 {
				if hasResource {
					t.Fatalf("unexpected resource=%#v", resource)
				}
				return
			}
			if !hasResource || len(resource) != len(test.resource) {
				t.Fatalf("resource=%#v, want %#v", record["resource"], test.resource)
			}
			for key, want := range test.resource {
				if resource[key] != want {
					t.Errorf("resource[%q]=%v, want %q", key, resource[key], want)
				}
			}
		})
	}
}

func TestRunContextServeConfigValidationAttribution(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(bad, []byte("schema_version: 9"), 0o600); err != nil {
		t.Fatal(err)
	}
	serveExit, serveStdout, serveStderr := runConfigArgs(t, []string{"--config", bad})
	checkExit, checkStdout, checkStderr := runConfigArgs(t, []string{"--check-config", "--config", bad})
	if serveExit != 1 || checkExit != 1 || serveStdout != "" || checkStdout != "" {
		t.Fatalf("serve exit/output=%d/%q; check exit/output=%d/%q", serveExit, serveStdout, checkExit, checkStdout)
	}
	serveRecord := decodeEnvelopeLine(t, serveStderr)
	checkRecord := decodeEnvelopeLine(t, checkStderr)
	if serveRecord["event"] != "config_validation_failed" || serveRecord["operation"] != "load_config" {
		t.Fatalf("serve record event/operation=%v/%v", serveRecord["event"], serveRecord["operation"])
	}
	if !reflect.DeepEqual(serveRecord["config"], checkRecord["config"]) ||
		!reflect.DeepEqual(serveRecord["error"], checkRecord["error"]) ||
		serveRecord["message"] != checkRecord["message"] {
		t.Fatalf("serve attribution=%#v differs from --check-config=%#v", serveRecord, checkRecord)
	}
	if strings.Contains(serveStderr, "schema_version: 9") || strings.Contains(serveStderr, "mcp-bridge:") {
		t.Fatalf("serve output contains plaintext config: %q", serveStderr)
	}
}

func TestRunContextStartupFailureEnvelope(t *testing.T) {
	path := edgeConfig(t, "http://127.0.0.1:9/v1")
	cfg, err := config.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := cfg.Command()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings.StorageDirectory, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runContext(context.Background(), []string{"--config", path}, &stdout, &stderr); code != 1 || stdout.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	lines := strings.Split(strings.TrimSuffix(stderr.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("startup records=%d, want one: %q", len(lines), stderr.String())
	}
	record := decodeEnvelopeLine(t, lines[0]+"\n")
	errorInfo := record["error"].(map[string]any)
	resource := record["resource"].(map[string]any)
	if record["event"] != "startup_failed" || record["operation"] != "startup" ||
		record["level"] != "error" || record["outcome"] != "failure" ||
		errorInfo["code"] != "state_directory" || resource["path"] != settings.StorageDirectory {
		t.Fatalf("startup record=%#v", record)
	}
	if strings.Contains(stderr.String(), "not a directory") || strings.Contains(stderr.String(), "mcp-bridge:") {
		t.Fatalf("startup output leaked plaintext: %q", stderr.String())
	}
}

func runConfigArgs(t *testing.T, args []string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func assertStartupFailure(t *testing.T, err error, event, code, kind string, retryable bool, resource map[string]string) {
	t.Helper()
	var failure startupFailure
	if !errors.As(err, &failure) {
		t.Fatalf("error type=%T, want startupFailure: %v", err, err)
	}
	wantOperation := "startup"
	if event == "cli_usage_failed" {
		wantOperation = "cli"
	}
	if failure.event != event || failure.operation != wantOperation ||
		failure.level != "error" || failure.outcome != "failure" || failure.code != code ||
		failure.kind != kind || failure.retryable != retryable || failure.remediation == "" ||
		!reflect.DeepEqual(failure.resource, resource) {
		t.Fatalf("startup failure=%#v, want %s/%s/%s retryable=%t resource=%#v", failure, event, code, kind, retryable, resource)
	}
}
