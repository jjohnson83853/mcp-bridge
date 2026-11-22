package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
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
		stage       string
		cause       string
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
		{name: "snapshot recovery", failure: snapshotRecoveryFailure("/tmp/state"), event: "startup_failed", operation: "startup", code: "snapshot_recovery", kind: "resource", remediation: "report this failure; startup snapshot recovery failed", stage: "snapshot_recovery", cause: "snapshot_unrecoverable", resource: map[string]string{"path": "/tmp/state"}},
		{name: "catalog loop start", failure: catalogLoopStartFailure(storage.ErrCorrupt), event: "startup_failed", operation: "startup", code: "catalog_loop_start", kind: "internal", remediation: "report this failure; the catalog loop failed to start or persist its startup snapshot", stage: "catalog_loop_start", cause: "admission_persist_failed"},
		{name: "catalog loop drain", failure: catalogLoopDrainFailure(), event: "startup_failed", operation: "startup", code: "catalog_loop_drain", kind: "internal", remediation: "report this failure; the catalog loop did not drain within the shutdown budget", stage: "catalog_loop_drain", cause: "drain_budget_exceeded"},
		{name: "listener serve", failure: listenerServeFailure("inbound", "127.0.0.1:8000"), event: "startup_failed", operation: "startup", code: "listener_serve", kind: "internal", remediation: "report this failure; the listener stopped serving", stage: "listener_serve", cause: "listener_serve_failed", resource: map[string]string{"listener": "inbound", "address": "127.0.0.1:8000"}},
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
			if record["schema_version"] != float64(1) {
				t.Fatalf("schema_version=%v, want 1", record["schema_version"])
			}
			if test.stage != "" && (record["startup_stage"] != test.stage || record["startup_cause"] != test.cause) {
				t.Fatalf("stage/cause=%v/%v, want %q/%q", record["startup_stage"], record["startup_cause"], test.stage, test.cause)
			}
			if test.stage == "" {
				if _, ok := record["startup_stage"]; ok {
					t.Fatalf("unexpected startup_stage key: %#v", record)
				}
				if _, ok := record["startup_cause"]; ok {
					t.Fatalf("unexpected startup_cause key: %#v", record)
				}
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

func TestStartupFailureForErrorClassifiesStages(t *testing.T) {
	document := readLoggingContract(t)
	tests := []struct {
		name        string
		err         error
		stage       string
		code        string
		cause       string
		remediation string
		resource    map[string]string
	}{
		{name: "catalog start generic", err: carryStartupStage(errors.New("catalog start detail"), startupStageCatalogLoopStart, "", ""), stage: "catalog_loop_start", code: "catalog_loop_start", cause: "generic_loop_start", remediation: "report this failure; the catalog loop failed to start or persist its startup snapshot"},
		{name: "catalog start corrupt snapshot", err: carryStartupStage(fmt.Errorf("wrapped: %w", storage.ErrCorrupt), startupStageCatalogLoopStart, "", ""), stage: "catalog_loop_start", code: "catalog_loop_start", cause: "admission_persist_failed", remediation: "report this failure; the catalog loop failed to start or persist its startup snapshot"},
		{name: "catalog drain", err: carryStartupStage(errors.New("drain detail"), startupStageCatalogLoopDrain, "", ""), stage: "catalog_loop_drain", code: "catalog_loop_drain", cause: "drain_budget_exceeded", remediation: "report this failure; the catalog loop did not drain within the shutdown budget"},
		{name: "listener serve", err: carryStartupStage(errors.New("listener detail"), startupStageListenerServe, "inbound", "127.0.0.1:8000"), stage: "listener_serve", code: "listener_serve", cause: "listener_serve_failed", remediation: "report this failure; the listener stopped serving", resource: map[string]string{"listener": "inbound", "address": "127.0.0.1:8000"}},
		{name: "unknown carrier stage", err: carryStartupStage(errors.New("unknown detail"), "not_a_stage", "", ""), code: "internal_error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			failure := startupFailureForError(test.err)
			if failure.code != test.code || failure.stage != test.stage || failure.cause != test.cause {
				t.Fatalf("failure code/stage/cause=%q/%q/%q, want %q/%q/%q", failure.code, failure.stage, failure.cause, test.code, test.stage, test.cause)
			}
			var output bytes.Buffer
			writeStartupFailure(&output, failure)
			lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
			if len(lines) != 1 {
				t.Fatalf("records=%d, want 1: %q", len(lines), output.String())
			}
			record := decodeEnvelopeLine(t, lines[0]+"\n")
			assertStartupFailureContractRow(t, document, record, test.code)
			if failure.code == "internal_error" {
				if _, ok := record["startup_stage"]; ok {
					t.Fatalf("unknown stage emitted startup_stage: %s", output.String())
				}
				if _, ok := record["startup_cause"]; ok {
					t.Fatalf("unknown stage emitted startup_cause: %s", output.String())
				}
			}
			if len(test.resource) != 0 {
				resource, ok := record["resource"].(map[string]any)
				if !ok || resource["listener"] != test.resource["listener"] || resource["address"] != test.resource["address"] {
					t.Fatalf("resource=%#v, want %#v", record["resource"], test.resource)
				}
			}
		})
	}

	publication := startupFailureForError(carryStartupStage(service.ErrPublicationFailed, startupStageCatalogLoopStart, "", ""))
	if publication.code != "publication_failed" || publication.stage != "" || publication.cause != "" {
		t.Fatalf("publication classification=%#v", publication)
	}
	corruptPublication := startupFailureForError(carryStartupStage(errors.Join(service.ErrPublicationFailed, storage.ErrCorrupt), startupStageCatalogLoopStart, "", ""))
	if corruptPublication.code != "publication_failed" || corruptPublication.stage != "" || corruptPublication.cause != "" {
		t.Fatalf("corrupt publication classification=%#v", corruptPublication)
	}
	diagnostics := &config.Diagnostics{Records: []config.Diagnostic{{Code: "invalid_config"}}}
	diagnosticCarrier := carryStartupStage(diagnostics, startupStageCatalogLoopStart, "", "")
	var gotDiagnostics *config.Diagnostics
	if !errors.As(diagnosticCarrier, &gotDiagnostics) || gotDiagnostics != diagnostics {
		t.Fatalf("diagnostics carrier lost its typed cause: %#v", gotDiagnostics)
	}
}

func assertStartupFailureContractRow(t *testing.T, document string, record map[string]any, code string) {
	t.Helper()
	errorInfo, ok := record["error"].(map[string]any)
	if !ok || errorInfo["code"] != code {
		t.Fatalf("startup error info=%#v, want code %q", record["error"], code)
	}
	want := []string{
		"`startup_failed`", "`error`", "`startup`", "`failure`", "`" + code + "`",
		"`" + fmt.Sprint(errorInfo["kind"]) + "`", "`" + fmt.Sprint(errorInfo["retryable"]) + "`",
		"`" + fmt.Sprint(errorInfo["remediation"]) + "`", "`application startup failed`",
	}
	if got := loggingContractClassRow(document, "startup_failed", code); !reflect.DeepEqual(got, want) {
		t.Fatalf("contract row for %s = %v, want %v", code, got, want)
	}
}
func TestStartupStageFailureOmitsRawCauseAndConfiguredSecrets(t *testing.T) {
	const secret = "startup-stage-configured-secret-sentinel"
	t.Setenv("SERVE_CLIENT_TOKEN", secret)
	t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)
	configPath := filepath.Join(t.TempDir(), "bridge.yaml")
	upstream := fakeUpstream(t, "read", `{"type":"object","properties":{},"additionalProperties":true}`)
	writeBridgeConfig(t, configPath, "127.0.0.1:0", "127.0.0.1:0", filepath.Join(t.TempDir(), "state"), "http://127.0.0.1:9/v1", []bridgeServerSpec{{id: "alpha", endpoint: upstream.URL}})
	if _, err := config.ParseFile(configPath); err != nil {
		t.Fatalf("parse secret-reference test config: %v", err)
	}
	cause := errors.New("raw cause contains " + secret)
	failure := startupFailureForError(carryStartupStage(cause, startupStageCatalogLoopStart, "", ""))
	var output bytes.Buffer
	writeStartupFailure(&output, failure)
	line := strings.TrimSpace(output.String())
	if strings.Contains(line, secret) || strings.Contains(line, cause.Error()) {
		t.Fatalf("startup line leaked secret or cause: %q", line)
	}
	record := decodeEnvelopeLine(t, line+"\n")
	if record["schema_version"] != float64(1) || record["startup_stage"] != "catalog_loop_start" || record["startup_cause"] != "generic_loop_start" {
		t.Fatalf("startup fields=%#v", record)
	}
}

func TestStartupStageFailureOmitsConfiguredSecretOnRealPath(t *testing.T) {
	const secret = "startup-real-path-secret-sentinel"
	t.Setenv("MCP_SNAPSHOT_QUARANTINE", "off")
	harness := newSnapshotRecoveryHarness(t)
	t.Setenv("SERVE_CLIENT_TOKEN", secret)
	path := filepath.Join(harness.stateDir, storage.SnapshotFile)
	seedRecoveryPath(t, path, []byte("{"))
	code, stderr := runSnapshotRecoveryToExit(t, harness)
	if code != 1 {
		t.Fatalf("exit=%d, want 1; stderr=%q", code, stderr)
	}
	if strings.Contains(stderr, secret) {
		t.Fatalf("stderr leaked configured secret: %q", stderr)
	}
	records := recoveryLogRecords(t, stderr)
	failures := recoveryEvents(records, "startup_failed")
	if len(failures) != 1 {
		t.Fatalf("startup failure records=%#v", failures)
	}
	failure := failures[0]
	info := failure["error"].(map[string]any)
	if info["code"] != "catalog_loop_start" || failure["startup_stage"] != "catalog_loop_start" || failure["startup_cause"] != "admission_persist_failed" {
		t.Fatalf("startup failure classification=%#v", failure)
	}
	if strings.Contains(stderr, "refusing to overwrite untrusted snapshot") || strings.Contains(stderr, "snapshot is corrupt") {
		t.Fatalf("startup failure leaked raw cause: %q", stderr)
	}
	document, err := os.ReadFile(harness.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(document, []byte("{env: SERVE_CLIENT_TOKEN}")) {
		t.Fatal("startup config does not reference the client-token environment secret")
	}
	assertStartupFailureContractRow(t, readLoggingContract(t), failure, "catalog_loop_start")
}

func TestStartupListenerServeFailureClassified(t *testing.T) {
	t.Setenv("MCP_SNAPSHOT_QUARANTINE", "off")
	t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
	t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)
	httpAddress, operationsAddress := freeAddress(t), freeAddress(t)
	configPath := filepath.Join(t.TempDir(), "bridge.yaml")
	embeddings := fakeEmbeddings(t)
	upstream := fakeUpstream(t, "read", `{"type":"object","properties":{},"additionalProperties":true}`)
	writeBridgeConfig(t, configPath, httpAddress, operationsAddress, filepath.Join(t.TempDir(), "state"), embeddings.URL, []bridgeServerSpec{{id: "alpha", endpoint: upstream.URL}})
	original := serveListenerFn
	serveListenerFn = func(server *http.Server, listener net.Listener) error {
		if listener.Addr().String() == httpAddress {
			_ = listener.Close()
			return errors.New("synthetic listener serve failure")
		}
		return serveListener(server, listener)
	}
	defer func() { serveListenerFn = original }()
	ctx := context.Background()
	stdout, stderr := &syncWriter{}, &syncWriter{}
	done := make(chan int, 1)
	go func() { done <- runContext(ctx, []string{"--config", configPath}, stdout, stderr) }()
	var code int
	select {
	case code = <-done:
	case <-time.After(20 * time.Second):
		t.Fatalf("listener failure did not finish within 20s; stderr=%q", stderr.String())
	}
	if code != 1 {
		t.Fatalf("exit=%d, want 1; stderr=%q", code, stderr.String())
	}
	records := recoveryLogRecords(t, stderr.String())
	failures := recoveryEvents(records, "startup_failed")
	if len(failures) != 1 {
		t.Fatalf("startup failure records=%#v", records)
	}
	failure := failures[0]
	info := failure["error"].(map[string]any)
	resource := failure["resource"].(map[string]any)
	if info["code"] != "listener_serve" || failure["startup_stage"] != "listener_serve" || failure["startup_cause"] != "listener_serve_failed" || resource["listener"] != "inbound" || resource["address"] != httpAddress {
		t.Fatalf("listener failure classification=%#v", failure)
	}
	assertStartupFailureContractRow(t, readLoggingContract(t), failure, "listener_serve")
}

func TestStartupGracefulShutdownEmitsNoStartupFailure(t *testing.T) {
	t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
	t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)
	httpAddress, operationsAddress := freeAddress(t), freeAddress(t)
	configPath := filepath.Join(t.TempDir(), "bridge.yaml")
	embeddings := fakeEmbeddings(t)
	upstream := fakeUpstream(t, "read", `{"type":"object","properties":{},"additionalProperties":true}`)
	writeBridgeConfig(t, configPath, httpAddress, operationsAddress, filepath.Join(t.TempDir(), "state"), embeddings.URL, []bridgeServerSpec{{id: "alpha", endpoint: upstream.URL}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stdout, stderr := &syncWriter{}, &syncWriter{}
	done := make(chan int, 1)
	go func() { done <- runContext(ctx, []string{"--config", configPath}, stdout, stderr) }()
	operationsURL := "http://" + operationsAddress
	waitFor(t, "operations listener readiness", func() bool {
		status, _, ok := probeBody(t, operationsURL+"/livez")
		return ok && status == http.StatusOK
	})
	cancel()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("graceful exit=%d; stderr=%q", code, stderr.String())
		}
	case <-time.After(20 * time.Second):
		t.Fatalf("graceful shutdown did not finish; stderr=%q", stderr.String())
	}
	if got := recoveryEvents(recoveryLogRecords(t, stderr.String()), "startup_failed"); len(got) != 0 {
		t.Fatalf("graceful shutdown emitted startup failures: %#v", got)
	}
}

func TestUnknownStartupErrorKeepsDistinctUnknownCode(t *testing.T) {
	const sentinel = "sentinel unknown"
	failure := startupFailureForError(errors.New(sentinel))
	if failure.code != "internal_error" || failure.remediation != "report this failure; no specific class matched" || failure.stage != "" || failure.cause != "" {
		t.Fatalf("unknown failure=%#v", failure)
	}
	var output bytes.Buffer
	writeStartupFailure(&output, failure)
	if strings.Contains(output.String(), sentinel) {
		t.Fatalf("unknown error leaked raw text: %q", output.String())
	}
	record := decodeEnvelopeLine(t, output.String())
	if record["schema_version"] != float64(1) {
		t.Fatalf("schema_version=%v, want 1", record["schema_version"])
	}
	if _, ok := record["startup_stage"]; ok {
		t.Fatalf("unknown error emitted startup_stage: %#v", record)
	}
	if _, ok := record["startup_cause"]; ok {
		t.Fatalf("unknown error emitted startup_cause: %#v", record)
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

func TestSnapshotRecoveryFailureRecord(t *testing.T) {
	const directory = "/safe/state"
	cause := errors.New("private filesystem detail")
	failure := snapshotRecoveryFailure(directory)
	if failure.code != "snapshot_recovery" || failure.stage != "snapshot_recovery" || failure.cause != "snapshot_unrecoverable" || failure.resource["path"] != directory {
		t.Fatalf("snapshot failure=%#v", failure)
	}
	var output bytes.Buffer
	writeStartupFailure(&output, failure)
	if strings.Contains(output.String(), cause.Error()) {
		t.Fatalf("startup recovery record leaked cause: %q", output.String())
	}
	record := decodeEnvelopeLine(t, output.String())
	if record["startup_stage"] != "snapshot_recovery" || record["startup_cause"] != "snapshot_unrecoverable" ||
		record["resource"].(map[string]any)["path"] != directory ||
		record["error"].(map[string]any)["code"] != "snapshot_recovery" {
		t.Fatalf("snapshot recovery record=%#v", record)
	}
}
func TestStartupFailureErrorIsStableAndSanitized(t *testing.T) {
	const secret = "private-cause-secret-sentinel"
	tests := []struct {
		name    string
		failure startupFailure
		message string
	}{
		{name: "state directory", failure: stateDirectoryFailure("/secret/path"), message: "application startup failed"},
		{name: "JWKS cache", failure: jwksCacheFailure(), message: "application startup failed"},
		{name: "embedding provider", failure: embeddingProviderFailure(), message: "application startup failed"},
		{name: "tool surface", failure: toolSurfaceFailure(), message: "application startup failed"},
		{name: "catalog loop", failure: catalogLoopFailure(), message: "application startup failed"},
		{name: "snapshot recovery", failure: snapshotRecoveryFailure("/secret/path"), message: "application startup failed"},
		{name: "catalog loop start", failure: catalogLoopStartFailure(errors.New(secret)), message: "application startup failed"},
		{name: "catalog loop start corrupt", failure: catalogLoopStartFailure(fmt.Errorf("%s: %w", secret, storage.ErrCorrupt)), message: "application startup failed"},
		{name: "catalog loop drain", failure: catalogLoopDrainFailure(), message: "application startup failed"},
		{name: "listener serve", failure: listenerServeFailure("inbound", secret), message: "application startup failed"},
		{name: "listener bind", failure: listenerBindFailure("inbound", secret), message: "application startup failed"},
		{name: "CLI usage", failure: cliUsageFailure(), message: "command-line arguments are invalid"},
		{name: "publication", failure: startupPublicationFailure(), message: "startup publication failed"},
		{name: "fallback", failure: fallbackStartupFailure(), message: "application startup failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.failure.Error(); got != test.message {
				t.Fatalf("Error()=%q, want %q", got, test.message)
			}
			if strings.Contains(test.failure.Error(), secret) || strings.Contains(test.failure.Error(), "secret/path") {
				t.Fatalf("Error() exposed cause or secret text: %q", test.failure.Error())
			}
		})
	}

	wrapped := carryStartupStage(errors.New(secret), startupStageCatalogLoopStart, "", "")
	if got := wrapped.Error(); got != secret {
		t.Fatalf("startup stage wrapper Error()=%q, want original cause %q", got, secret)
	}
}

func TestWriteStartupFailureLoggingFallback(t *testing.T) {
	fallback, err := os.CreateTemp(t.TempDir(), "logging-fallback-*")
	if err != nil {
		t.Fatal(err)
	}
	originalStderr := os.Stderr
	os.Stderr = fallback
	defer func() { os.Stderr = originalStderr }()

	writeStartupFailure(loggingContractFailWriter{}, fallbackStartupFailure())
	if err := fallback.Sync(); err != nil {
		t.Fatal(err)
	}
	if _, err := fallback.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	contents, err := io.ReadAll(fallback)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != `{"schema_version":1,"level":"warn","service":"mcp-bridge","event":"logging_write_failed","message":"logging write failed","operation":"logging","outcome":"failure"}`+"\n" {
		t.Fatalf("logging fallback=%q", contents)
	}
}
