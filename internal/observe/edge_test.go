package observe

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogRejectsInvalidEnvelopeFields(t *testing.T) {

	var stdout, stderr bytes.Buffer
	NewLogger(&stdout, &stderr).Log(Entry{
		Level: "trace-secret", Event: "attacker-event", Operation: "private-op", Outcome: "arbitrary",
		Server: "Bad_Server", Fields: map[string]any{"mode": "attacker-mode", "count": 3, "slots": "not-a-number"},
	})
	if stdout.Len() != 0 {
		t.Fatalf("invalid envelope fallback went to stdout: %s", stdout.String())
	}
	assertFallbackLine(t, stderr.String())
}
func TestMetricsHelpersBoundLabelsAndExposeAcceptedUpdates(t *testing.T) {
	m := NewMetrics([]string{"alpha"})
	if !m.RecordRequest("prompts_list", "success") || m.RecordRequest("", "success") {
		t.Fatal("request metric acceptance contract changed")
	}
	if !m.SetCatalogTools("alpha", 3) || !m.SetCatalogAge("alpha", 2.5) || !m.RecordRefresh("alpha", "success", "none") {
		t.Fatal("known server metric rejected")
	}
	if m.SetCatalogTools("bad_server", 3) || m.SetCatalogAge("bad_server", 2) || m.RecordRefresh("bad_server", "success", "none") {
		t.Fatal("unknown server metric accepted")
	}
	if !m.RecordReload("rejected", "invalid_configuration") || m.RecordReload("secret-outcome", "secret-reason") {
		t.Fatal("reload label vocabulary contract changed")
	}
	if !m.SetReadiness("alpha", Readiness(9)) || m.SetReadiness("bad_server", ReadinessReady) {
		t.Fatal("readiness server acceptance contract changed")
	}
	response := httptest.NewRecorder()
	m.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	text := response.Body.String()
	for _, want := range []string{`bridge_requests_total{operation="prompts_list",outcome="success",server="_bridge"} 1`, `bridge_catalog_tools{server="alpha"} 3`, `bridge_catalog_age_seconds{server="alpha"} 2.5`, `bridge_catalog_refresh_total{outcome="success",reason="none",server="alpha"} 1`, `bridge_config_reload_total{outcome="rejected",reason="invalid_configuration"} 1`, `bridge_readiness{server="alpha"} 9`} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in metrics: %s", want, text)
		}
	}
	for _, secret := range []string{"bad_server", "secret-outcome", "secret-reason"} {
		if strings.Contains(text, secret) {
			t.Errorf("unaccepted metric label leaked %q", secret)
		}
	}
}
func TestSnapshotRecoveryEnvelopeAdmission(t *testing.T) {
	snapshotError, ok := runtimeErrorInfo("snapshot_unavailable")
	wantSnapshotError := ErrorInfo{
		Code:        "snapshot_unavailable",
		Kind:        "resource",
		Retryable:   true,
		Remediation: "verify the durable snapshot and storage permissions, then retry",
	}
	if !ok || snapshotError != wantSnapshotError || !validErrorInfo(&snapshotError) {
		t.Fatalf("snapshot_unavailable info=%+v, ok=%t; want %+v", snapshotError, ok, wantSnapshotError)
	}
	mutatedSnapshotError := snapshotError
	mutatedSnapshotError.Retryable = false
	if validErrorInfo(&mutatedSnapshotError) {
		t.Fatal("snapshot_unavailable accepted a non-canonical error shape")
	}

	directoryError, ok := runtimeErrorInfo("state_directory")
	wantDirectoryError := ErrorInfo{
		Code:        "state_directory",
		Kind:        "resource",
		Retryable:   false,
		Remediation: "create the configured state directory or fix its permissions",
	}
	if !ok || directoryError != wantDirectoryError || !validErrorInfo(&directoryError) {
		t.Fatalf("state_directory info=%+v, ok=%t; want %+v", directoryError, ok, wantDirectoryError)
	}
	mutatedDirectoryError := directoryError
	mutatedDirectoryError.Remediation = "inspect the logs"
	if validErrorInfo(&mutatedDirectoryError) {
		t.Fatal("state_directory accepted a non-canonical error shape")
	}

	for _, reason := range []string{
		"corrupt_json", "checksum_mismatch", "unsupported_schema",
		"validation_failure", "read_error", "orphaned_link",
	} {
		t.Run(reason, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			NewLogger(&stdout, &stderr).Log(Entry{
				Level: "warn", Event: "startup_snapshot_rejected", Operation: "startup", Outcome: "degraded",
				Error: RuntimeErrorInfo("snapshot_unavailable"),
				Fields: map[string]any{
					"failure_reason": reason, "quarantine_path": "/state/snapshot.corrupt-2026-11-21T00:00:00Z.json",
					"resource.path": "/state",
				},
			})
			if stdout.Len() != 0 || stderr.Len() == 0 {
				t.Fatalf("record not admitted on stderr: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			record := decodeRecoveryEnvelope(t, stderr.Bytes())
			if record["schema_version"] != float64(1) || record["failure_reason"] != reason ||
				record["quarantine_path"] != "/state/snapshot.corrupt-2026-11-21T00:00:00Z.json" {
				t.Fatalf("record fields=%v", record)
			}
			if resource, ok := record["resource"].(map[string]any); !ok || resource["path"] != "/state" {
				t.Fatalf("resource=%v; want configured state path", record["resource"])
			}
		})
	}

	t.Run("retention failure message and state-directory class", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		NewLogger(&stdout, &stderr).Log(Entry{
			Level: "warn", Event: "snapshot_retention_failed", Operation: "startup", Outcome: "failure",
			Error: RuntimeErrorInfo("state_directory"),
			Fields: map[string]any{
				"quarantine_path": "/state/prompts.corrupt-2026-11-21T00:00:00Z.json",
				"resource.path":   "/state",
			},
		})
		if stdout.Len() != 0 || stderr.Len() == 0 {
			t.Fatalf("retention record not admitted on stderr: stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
		record := decodeRecoveryEnvelope(t, stderr.Bytes())
		if record["schema_version"] != float64(1) || record["message"] != "quarantine retention failed" ||
			record["quarantine_path"] != "/state/prompts.corrupt-2026-11-21T00:00:00Z.json" {
			t.Fatalf("retention record=%v", record)
		}
		if errorInfo, ok := record["error"].(map[string]any); !ok ||
			errorInfo["code"] != "state_directory" || errorInfo["kind"] != "resource" ||
			errorInfo["retryable"] != false ||
			errorInfo["remediation"] != "create the configured state directory or fix its permissions" {
			t.Fatalf("retention error=%v", record["error"])
		}
	})

	for name, fields := range map[string]map[string]any{
		"unknown failure reason": {"failure_reason": "private error", "quarantine_path": "/state/q.json", "resource.path": "/state"},
		"empty quarantine path":  {"failure_reason": "corrupt_json", "quarantine_path": "", "resource.path": "/state"},
		"non-string quarantine path": {"failure_reason": "corrupt_json", "quarantine_path": 17, "resource.path": "/state"},
	} {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			NewLogger(&stdout, &stderr).Log(Entry{
				Level: "warn", Event: "startup_snapshot_rejected", Operation: "startup", Outcome: "degraded",
				Error: RuntimeErrorInfo("snapshot_unavailable"), Fields: fields,
			})
			if stderr.Len() != 0 {
				t.Fatalf("invalid recovery fields admitted: %s", stderr.String())
			}
			assertFallbackLine(t, stdout.String())
		})
	}
}

func decodeRecoveryEnvelope(t *testing.T, encoded []byte) map[string]any {
	t.Helper()
	var record map[string]any
	if err := json.Unmarshal(encoded, &record); err != nil {
		t.Fatalf("decode recovery envelope: %v: %s", err, encoded)
	}
	return record
}
