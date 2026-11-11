package observe_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

func TestLoggerEmitsSafeCorrelationAndServer(t *testing.T) {
	var output bytes.Buffer
	logger := observe.NewLogger(&output, &output)
	id := observe.CorrelationID("not-a-valid-id")
	if len(id) != 32 {
		t.Fatalf("correlation id length=%d", len(id))
	}
	for _, r := range id {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			t.Fatalf("correlation id=%q", id)
		}
	}
	logger.Log(observe.Entry{
		Level: "info", Event: "backend_complete", RequestID: id, Server: "alpha",
		Operation: "call_tool", Outcome: "success", Fields: map[string]any{"tool": "alpha__read"},
	})
	var record map[string]json.RawMessage
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode log record: %v: %q", err, output.String())
	}
	if string(record["request_id"]) != `"`+id+`"` || string(record["server"]) != `"alpha"` {
		t.Fatalf("missing safe correlation/server fields: %s", output.String())
	}
	if string(record["message"]) == `""` || string(record["schema_version"]) != "1" {
		t.Fatalf("incomplete envelope: %s", output.String())
	}
	if strings.Contains(output.String(), `"time":`) || strings.Contains(output.String(), `"reason":`) {
		t.Fatalf("legacy field appeared in envelope: %s", output.String())
	}
}
func TestEnvelopeFieldNamesMatchesIndependentR8Union(t *testing.T) {
	want := []string{
		"auth_mode", "backend_call_id", "backend_id", "binding_relation", "blocker_class", "build_state",
		"candidate_servers", "candidate_tools", "catalog_state", "catalog_tools",
		"challenge_kind", "challenge_target", "chunk_count", "chunk_index", "client_family", "clock_skew_seconds",
		"config", "config.actual_count", "config.column", "config.diagnostics_incomplete",
		"config.first_excess_index", "config.line", "config.max_count", "config.path",
		"config.pin_server", "config.pin_tool", "config.property_path", "config.ref_kind",
		"config.ref_name", "config.related.column", "config.related.line",
		"config.related.property_path", "config.related.ref_kind", "config.related.ref_name",
		"config.related.server", "config.remaining_stages", "config.server", "config.shape",
		"config_revision", "correlation_state", "count", "credential_state", "diag_duration_ms",
		"diag_schema", "diagnostic_policy_revision", "discovered_tools", "duration_capped", "duration_ms",
		"effective_policy_revision", "effective_policy_revision_state", "endpoint", "error", "error.code",
		"error.kind", "error.remediation", "error.retryable", "error_detail",
		"error_detail_state", "event", "exposed_tools", "failure_reason", "failure_stage",
		"final_content_count", "final_shape", "final_structured_presence", "final_text_presence",
		"generation", "http_bytes", "http_bytes_bucket", "http_method", "http_outcome",
		"http_status", "level", "mcp_message_kind", "mcp_method", "message",
		"metadata_target_registered", "mode", "next_attempt_ms", "oauth_enabled", "operation",
		"operation_id", "operation_id_state", "operation_relation", "outcome", "path_class",
		"pins_resolved", "pins_total", "policy_source", "process_id", "process_state",
		"protocol_version", "publication_impact", "refresh_id", "request_id", "resource",
		"resource.address", "resource.listener", "resource.path", "resource_path_mode",
		"retained_age_ms", "retained_generation", "route", "route_outcome", "schema_version",
		"scope_required", "scope_state", "server", "servers_discovered", "service",
		"session_binding", "session_header", "session_ref", "session_ref_state", "session_state",
		"slots", "stage", "static_configured", "status_state", "timing_scope",
		"tool", "tools_discovered", "tools_published", "trigger", "upstream_content_count",
		"upstream_shape", "upstream_state", "upstream_structured_presence",
		"upstream_text_presence",
	}
	if got := observe.EnvelopeFieldNames(); !reflect.DeepEqual(got, want) {
		t.Fatalf("EnvelopeFieldNames() = %v; want the independent r8 vocabulary %v", got, want)
	}
}

func TestDiscoveryFailureEnvelopeSanitizesEndpointAndRecoveryFields(t *testing.T) {
	tests := []struct {
		name        string
		stage       string
		reason      string
		errorCode   string
		retryable   bool
		status      int
		nextAttempt int64
	}{
		{
			name: "authentication during initialize", stage: "auth",
			reason: "authentication_rejected_during_initialize", errorCode: "client_missing",
			status: 401,
		},
		{
			name: "tools-list timeout", stage: "tools_list",
			reason: "timeout_context_deadline_exceeded_50ms", errorCode: "upstream_unavailable",
			retryable: true, nextAttempt: 5000,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			fields := map[string]any{
				"endpoint":       "https://user:credential-secret@upstream.example:8443/mcp?token=query-secret#body-secret",
				"failure_stage":  test.stage,
				"failure_reason": test.reason,
			}
			if test.status != 0 {
				fields["http_status"] = test.status
			}
			if test.nextAttempt != 0 {
				fields["next_attempt_ms"] = test.nextAttempt
			}
			observe.NewLogger(&stdout, &stderr).Log(observe.Entry{
				Level: "warn", Event: "upstream_discovery_failed", Server: "alpha",
				Operation: "refresh", Outcome: "failure",
				Error: observe.RuntimeErrorInfo(test.errorCode), Fields: fields,
			})

			var record map[string]any
			if err := json.Unmarshal(stderr.Bytes(), &record); err != nil {
				t.Fatalf("decode discovery record: %v: %q", err, stderr.String())
			}
			if record["level"] != "warn" || record["event"] != "upstream_discovery_failed" ||
				record["failure_stage"] != test.stage || record["failure_reason"] != test.reason {
				t.Fatalf("discovery record = %#v", record)
			}
			if record["endpoint"] != "https://upstream.example:8443/mcp" {
				t.Fatalf("endpoint = %v; want sanitized scheme/host/port/path", record["endpoint"])
			}
			if _, exists := record["http_status"]; exists != (test.status != 0) {
				t.Errorf("http_status presence = %t; want %t", exists, test.status != 0)
			}
			if _, exists := record["next_attempt_ms"]; exists != (test.nextAttempt != 0) {
				t.Errorf("next_attempt_ms presence = %t; want %t", exists, test.nextAttempt != 0)
			}
			errorInfo, ok := record["error"].(map[string]any)
			if !ok || errorInfo["code"] != test.errorCode || errorInfo["retryable"] != test.retryable {
				t.Errorf("error = %#v; want code=%s retryable=%t", record["error"], test.errorCode, test.retryable)
			}
			for _, secret := range []string{"credential-secret", "query-secret", "body-secret"} {
				if strings.Contains(stderr.String(), secret) {
					t.Errorf("discovery record leaked %s", secret)
				}
			}
			if stdout.Len() != 0 {
				t.Errorf("warn record went to stdout: %q", stdout.String())
			}
		})
	}
}

func TestLoggerEmitsTypedEnvelopeToSeverityStream(t *testing.T) {
	var stdout, stderr bytes.Buffer
	logger := observe.NewLogger(&stdout, &stderr)
	logger.Log(observe.Entry{
		Level: "error", Event: "backend_complete", Operation: "call_tool", Outcome: "failure",
		Error: observe.RuntimeErrorInfo("backend_unavailable"),
	})
	if stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("severity routing stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	var record map[string]json.RawMessage
	if err := json.Unmarshal(stderr.Bytes(), &record); err != nil {
		t.Fatalf("decode emitted envelope: %v: %q", err, stderr.String())
	}
	keys := make([]string, 0, len(record))
	for key := range record {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	wantKeys := []string{"error", "event", "level", "message", "operation", "outcome", "schema_version", "service"}
	if !reflect.DeepEqual(keys, wantKeys) {
		t.Fatalf("record keys = %v; want %v", keys, wantKeys)
	}
	for key, want := range map[string]string{
		"level": `"error"`, "service": `"mcp-bridge"`, "event": `"backend_complete"`,
		"operation": `"call_tool"`, "outcome": `"failure"`,
	} {
		if string(record[key]) != want {
			t.Errorf("%s = %s; want %s", key, record[key], want)
		}
	}
	if string(record["message"]) == `""` || string(record["schema_version"]) != "1" {
		t.Fatalf("incomplete required envelope fields: %s", stderr.String())
	}
	var nestedError map[string]json.RawMessage
	if err := json.Unmarshal(record["error"], &nestedError); err != nil {
		t.Fatalf("decode nested error: %v", err)
	}
	errorKeys := make([]string, 0, len(nestedError))
	for key := range nestedError {
		errorKeys = append(errorKeys, key)
	}
	sort.Strings(errorKeys)
	if want := []string{"code", "kind", "remediation", "retryable"}; !reflect.DeepEqual(errorKeys, want) {
		t.Fatalf("nested error keys = %v; want %v", errorKeys, want)
	}
	if strings.Contains(stderr.String(), `"time":`) || strings.Contains(stderr.String(), `"reason":`) {
		t.Fatalf("legacy fields present in envelope: %s", stderr.String())
	}
}

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) {
	return len(p) - 1, nil
}

func TestWriteJSONLineReturnsWriteFailures(t *testing.T) {
	writeErr := errors.New("write failed")
	if err := observe.WriteJSONLine(failingWriter{err: writeErr}, struct{}{}); !errors.Is(err, writeErr) {
		t.Fatalf("failing writer error = %v; want %v", err, writeErr)
	}
	if err := observe.WriteJSONLine(shortWriter{}, struct{}{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short writer error = %v; want %v", err, io.ErrShortWrite)
	}
	if err := observe.WriteJSONLine(nil, struct{}{}); err == nil {
		t.Fatal("nil output returned no error")
	}
	if err := observe.WriteJSONLine(io.Discard, make(chan int)); err == nil {
		t.Fatal("marshal failure returned no error")
	}
}
