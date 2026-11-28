package observe

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestLoggerEmitsSafeCorrelationAndServer(t *testing.T) {
	var output bytes.Buffer
	logger := NewLogger(&output, &output)
	id := CorrelationID("not-a-valid-id")
	if len(id) != 32 {
		t.Fatalf("correlation id length=%d", len(id))
	}
	for _, r := range id {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			t.Fatalf("correlation id=%q", id)
		}
	}
	logger.Log(Entry{
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
func TestBindingReinstatedEntryRecordShape(t *testing.T) {
	var output bytes.Buffer
	NewLogger(&output, &output).Log(Entry{
		Level: "warn", Event: "binding_reinstated", Server: "alpha",
		Operation: "startup", Outcome: "degraded",
	})
	var record map[string]json.RawMessage
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode binding reinstated record: %v: %q", err, output.String())
	}
	for field, want := range map[string]string{
		"level": `"warn"`, "event": `"binding_reinstated"`,
		"message":   `"configured server binding reinstated"`,
		"operation": `"startup"`, "outcome": `"degraded"`, "server": `"alpha"`,
	} {
		if got := string(record[field]); got != want {
			t.Errorf("%s = %s; want %s", field, got, want)
		}
	}
	if _, exists := record["error"]; exists {
		t.Fatalf("unexpected error object in binding reinstated record: %s", output.String())
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
		"projection_reason", "protocol_version", "publication_impact", "quarantine_path", "refresh_id", "request_id", "resource",
		"resource.address", "resource.listener", "resource.path", "resource_path_mode",
		"retained_age_ms", "retained_generation", "route", "route_outcome", "schema_version",
		"scope_required", "scope_state", "server", "servers_discovered", "service",
		"session_binding", "session_header", "session_ref", "session_ref_state", "session_state",
		"slots", "stage", "startup_cause", "startup_stage", "static_configured", "status_state", "timing_scope",
		"tool", "tools_discovered", "tools_published", "trigger", "upstream_content_count",
		"upstream_shape", "upstream_state", "upstream_structured_presence",
		"upstream_text_presence",
	}
	if got := EnvelopeFieldNames(); !reflect.DeepEqual(got, want) {
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
			NewLogger(&stdout, &stderr).Log(Entry{
				Level: "warn", Event: "upstream_discovery_failed", Server: "alpha",
				Operation: "refresh", Outcome: "failure",
				Error: RuntimeErrorInfo(test.errorCode), Fields: fields,
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
	logger := NewLogger(&stdout, &stderr)
	logger.Log(Entry{
		Level: "error", Event: "backend_complete", Operation: "call_tool", Outcome: "failure",
		Error: RuntimeErrorInfo("backend_unavailable"),
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

type shortLogWriter struct{}

func (shortLogWriter) Write(p []byte) (int, error) {
	return len(p) - 1, nil
}

func TestWriteJSONLineReturnsWriteFailures(t *testing.T) {
	writeErr := errors.New("write failed")
	if err := WriteJSONLine(failingWriter{err: writeErr}, struct{}{}); !errors.Is(err, writeErr) {
		t.Fatalf("failing writer error = %v; want %v", err, writeErr)
	}
	if err := WriteJSONLine(shortLogWriter{}, struct{}{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short writer error = %v; want %v", err, io.ErrShortWrite)
	}
	if err := WriteJSONLine(nil, struct{}{}); err == nil {
		t.Fatal("nil output returned no error")
	}
	if err := WriteJSONLine(io.Discard, make(chan int)); err == nil {
		t.Fatal("marshal failure returned no error")
	}
}

func TestLoggerHandlesNilAndOppositeStreamFailures(t *testing.T) {
	const fallbackLine = `{"schema_version":1,"level":"warn","service":"mcp-bridge","event":"logging_write_failed","message":"logging write failed","operation":"logging","outcome":"failure"}` + "\n"
	var fallback bytes.Buffer
	logger := NewLogger(nil, &fallback)
	logger.Log(Entry{Level: "info", Event: "backend_complete", Operation: "call_tool", Outcome: "success"})
	if fallback.String() != fallbackLine {
		t.Fatalf("nil stdout fallback=%q", fallback.String())
	}

	var stdout bytes.Buffer
	logger = NewLogger(&stdout, failingWriter{err: errors.New("closed stderr")})
	logger.Log(Entry{Level: "error", Event: "backend_complete", Operation: "call_tool", Outcome: "failure", Error: RuntimeErrorInfo("backend_unavailable")})
	if stdout.String() != fallbackLine {
		t.Fatalf("failed stderr did not fall back to stdout: %q", stdout.String())
	}

	logger = NewLogger(failingWriter{err: errors.New("closed stdout")}, &fallback)
	logger.Log(Entry{Level: "info", Event: "backend_complete", Operation: "call_tool", Outcome: "success"})
	if strings.Count(fallback.String(), `"event":"logging_write_failed"`) != 2 {
		t.Fatalf("failed stdout did not fall back to stderr: %q", fallback.String())
	}

	var discarded bytes.Buffer
	NewLogger(&discarded, &discarded).Log(Entry{})
	if strings.Count(discarded.String(), `"event":"logging_write_failed"`) != 1 {
		t.Fatalf("invalid envelope fallback=%q", discarded.String())
	}
	var nilLogger *Logger
	nilLogger.Log(Entry{})
}

func TestCorrelationIDAcceptsOnlyCanonicalHeaders(t *testing.T) {
	for _, header := range []string{"0123456789abcdef0123456789abcdef", "ffffffffffffffffffffffffffffffff"} {
		if got := CorrelationID(header); got != header {
			t.Errorf("CorrelationID(%q) = %q; want unchanged header", header, got)
		}
	}
	for _, header := range []string{"", "0123456789ABCDEF0123456789ABCDEF", " 0123456789abcdef0123456789abcdef"} {
		if got := CorrelationID(header); got == header || !validHexID(got) {
			t.Errorf("CorrelationID(%q) = %q; want a fresh canonical identifier", header, got)
		}
	}
}

func TestFiniteLogValuesUseClosedFallbacks(t *testing.T) {
	for _, test := range []struct {
		name string
		got  string
		want string
	}{
		{name: "outcome known", got: finiteOutcome("success"), want: "success"},
		{name: "outcome unknown", got: finiteOutcome("NaN"), want: "unknown"},
		{name: "reason known", got: finiteReason("backend_unavailable"), want: "backend_unavailable"},
		{name: "reason unknown", got: finiteReason("-1"), want: "unknown"},
		{name: "finite known", got: finite("allowed", "fallback", "allowed", "other"), want: "allowed"},
		{name: "finite invalid", got: finite("+Inf", "fallback", "allowed", "other"), want: "fallback"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("got %q; want %q", test.got, test.want)
			}
		})
	}
}

func TestSafeServerAcceptsAndRejectsBoundedNames(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
		want  bool
	}{
		{name: "bridge", value: "_bridge", want: true},
		{name: "lowercase", value: "alpha", want: true},
		{name: "digits and dash", value: "alpha-2", want: true},
		{name: "empty", value: ""},
		{name: "too long", value: strings.Repeat("a", 33)},
		{name: "uppercase", value: "Alpha"},
		{name: "leading digit", value: "2alpha"},
		{name: "invalid character", value: "alpha_name"},
	} {
		if got := safeServer(test.value); got != test.want {
			t.Errorf("safeServer(%q) = %t; want %t", test.value, got, test.want)
		}
	}
}

func TestSensitiveKeyMatchingNormalizesSeparatorsAndCase(t *testing.T) {
	for key, want := range map[string]bool{
		"Authorization": true,
		"api-key":       true,
		"session_token": true,
		"tool":          false,
		"duration_ms":   false,
	} {
		if got := sensitive(key); got != want {
			t.Errorf("sensitive(%q) = %t; want %t", key, got, want)
		}
	}
}

func TestLogHTTPUsesTypedDiagnosticsDecisionAndFailurePaths(t *testing.T) {
	status := 401
	valid := Entry{
		Level: "info", Event: "http_complete", RequestID: testID, Operation: "request",
		Duration: 5 * time.Millisecond, Outcome: "failure",
		HTTP: &HTTPDiagnostic{
			LegacyStage: "client_auth", Operation: "request", ClientFamily: "openai",
			HTTPBytes: 0, HTTPStatus: &status, HTTPMethod: "POST", Route: "mcp",
			HTTPOutcome: "failure", HTTPBytesBucket: "le_1k", StatusState: "observed",
			CorrelationState: CorrelationAvailable, RequestID: testID, DiagSchema: "oauth_diag_v1",
			BuildState: "unavailable", ProcessState: "available", ProcessID: testID, OperationRelation: "single",
			OperationIDState: "available", OperationID: testID, PathClass: "mcp", RouteOutcome: "oauth_protected",
			DiagDurationMS: 5, FailureStage: "outer_oauth", FailureReason: "credentials_missing",
			AuthMode: "oauth", CredentialState: "missing", ScopeState: "not_evaluated",
			SessionHeader: "absent", SessionState: "not_evaluated", BindingRelation: "not_evaluated",
			SessionRefState: "available", SessionRef: testID, ChallengeKind: "none", ChallengeTarget: "none",
			MetadataTargetRegistered:     "not_applicable",
			DiagnosticPolicyRevision:     "oauth-diag-policy-v1",
			EffectivePolicyRevisionState: "unavailable", PolicySource: "unavailable",
			ResourcePathMode: "unavailable", SessionBinding: "unavailable",
			UpstreamState: "not_attempted", ErrorDetailState: "included",
			ErrorDetail: &HTTPDiagnosticErrorDetail{Class: "validation", Code: "credentials_missing", Retry: "not_attempted"},
		},
	}
	t.Run("valid typed record reaches stdout", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		NewLogger(&stdout, &stderr).logHTTP(valid)
		if stdout.Len() == 0 || stderr.Len() != 0 {
			t.Fatalf("typed diagnostic streams stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
	})
	t.Run("wrong event is omitted", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		entry := valid
		entry.Event = "backend_complete"
		NewLogger(&stdout, &stderr).logHTTP(entry)
		if stdout.Len() != 0 || stderr.Len() != 0 {
			t.Fatalf("non-terminal HTTP event emitted: stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
	})
	t.Run("invalid core records fallback", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		entry := valid
		entry.HTTP = &HTTPDiagnostic{}
		NewLogger(&stdout, &stderr).logHTTP(entry)
		if stdout.Len() != 0 || !strings.Contains(stderr.String(), `"event":"logging_write_failed"`) {
			t.Fatalf("invalid typed record fallback stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
	})
}

func TestLoggerRoutesWarningAndShortWritesToFallback(t *testing.T) {
	var stdout, stderr bytes.Buffer
	NewLogger(&stdout, &stderr).Log(Entry{Level: "warn", Event: "backend_complete", Operation: "call_tool", Outcome: "failure"})
	if stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("warn routing stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	var fallback bytes.Buffer
	NewLogger(shortLogWriter{}, &fallback).Log(Entry{Level: "info", Event: "backend_complete", Operation: "call_tool", Outcome: "success"})
	if !strings.Contains(fallback.String(), `"event":"logging_write_failed"`) {
		t.Fatalf("short stdout write did not use fallback: %q", fallback.String())
	}
}

func TestWriteLoggingFailureFallbackUsesOppositeStandardStream(t *testing.T) {
	capture, err := os.CreateTemp(t.TempDir(), "stdout-*.log")
	if err != nil {
		t.Fatal(err)
	}
	originalStdout := os.Stdout
	os.Stdout = capture
	WriteLoggingFailureFallback(os.Stderr)
	os.Stdout = originalStdout
	if err := capture.Close(); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(capture.Name())
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != string(loggingWriteFailedLine) {
		t.Fatalf("opposite-stream fallback = %q; want %q", contents, loggingWriteFailedLine)
	}
}
func TestLogLabelEnumeration(t *testing.T) {
	wantOperations := []string{
		"unknown", "request", "discover", "initialize", "initialized", "tools_list",
		"tools_call", "call_tool", "resources_list", "resources_read", "ping",
		"sse_stream", "search", "describe", "health", "describe_tool",
		"prompts_list", "prompts_get", "refresh", "prompt_load", "prompt_persist",
		"startup", "handshake", "reload", "cli", "search_tools", "retrieve_result",
		"load_config", "check_config", "publish", "logging", "expose_tools",
	}
	if !reflect.DeepEqual(operationVocabulary, wantOperations) {
		t.Fatalf("operation vocabulary = %#v; want %#v", operationVocabulary, wantOperations)
	}

	newEvents := []string{
		"result_paged", "result_page_served", "result_expired", "result_evicted",
		"invalid_handle", "invalid_arguments", "result_capacity_exceeded",
		"result_aggregate_capacity_exceeded", "result_namespace_retired",
		"result_entropy_unavailable", "result_epoch_unresolved", "log_vocabulary_violation",
		"projection_skipped", "projection_rejected",
	}
	for _, event := range newEvents {
		if eventMessage(event) == "" {
			t.Errorf("new event %q has no registered message", event)
		}
	}
	if eventMessage("unlisted_event") != "" {
		t.Fatal("unregistered event has a fallback message")
	}

	assertViolation := func(name string, line []byte) {
		t.Helper()
		var record map[string]any
		if err := json.Unmarshal(bytes.TrimSpace(line), &record); err != nil {
			t.Fatalf("%s emitted invalid JSON: %v: %q", name, err, line)
		}
		if record["level"] != "warn" || record["event"] != "log_vocabulary_violation" || record["operation"] != "logging" {
			t.Fatalf("%s record = %#v; want warning vocabulary-violation record", name, record)
		}
		if strings.Contains(string(line), "unlisted_operation") || strings.Contains(string(line), "unlisted_event") {
			t.Fatalf("%s leaked the rejected value: %s", name, line)
		}
	}

	var stdout, stderr bytes.Buffer
	NewLogger(&stdout, &stderr).Log(Entry{
		Level: "info", Event: "result_paged", Operation: "unlisted_operation", Outcome: "success",
	})
	assertViolation("Logger.Log operation", stderr.Bytes())
	var direct bytes.Buffer
	if err := WriteJSONLine(&direct, NewEnvelope("info", "result_paged", "ignored", "unlisted_operation", "success")); err != nil {
		t.Fatalf("WriteJSONLine operation injection: %v", err)
	}
	assertViolation("NewEnvelope/WriteJSONLine operation", direct.Bytes())

	stdout.Reset()
	stderr.Reset()
	NewLogger(&stdout, &stderr).Log(Entry{
		Level: "info", Event: "unlisted_event", Operation: "logging", Outcome: "success",
	})
	assertViolation("Logger.Log event", stderr.Bytes())
	direct.Reset()
	if err := WriteJSONLine(&direct, NewEnvelope("info", "unlisted_event", "ignored", "logging", "success")); err != nil {
		t.Fatalf("WriteJSONLine event injection: %v", err)
	}
	assertViolation("NewEnvelope/WriteJSONLine event", direct.Bytes())
}
func TestProjectionEventsRenderEnumeratedReasons(t *testing.T) {
	tests := []struct {
		reason  string
		event   string
		outcome string
	}{
		{"full_requested", "projection_skipped", "degraded"},
		{"upstream_select_present", "projection_skipped", "degraded"},
		{"upstream_error", "projection_skipped", "degraded"},
		{"payload_unparseable", "projection_skipped", "degraded"},
		{"payload_shape_unexpected", "projection_skipped", "degraded"},
		{"item_shape_unexpected", "projection_skipped", "degraded"},
		{"structured_content_not_mirrorable", "projection_skipped", "degraded"},
		{"disclosure_key_collision", "projection_skipped", "degraded"},
		{"malformed_request", "projection_rejected", "denied"},
		{"not_supported", "projection_rejected", "denied"},
		{"select_conflict", "projection_rejected", "denied"},
		{"unknown_field_selection", "projection_rejected", "denied"},
	}
	for _, test := range tests {
		t.Run(test.reason, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			NewLogger(&stdout, &stderr).Log(Entry{
				Level: "warn", Event: test.event, Operation: "call_tool", Outcome: test.outcome,
				Fields: map[string]any{"projection_reason": test.reason},
			})
			if stdout.Len() != 0 || strings.Count(stderr.String(), "\n") != 1 {
				t.Fatalf("rendered streams stdout=%q stderr=%q; want one warning record", stdout.String(), stderr.String())
			}
			var record map[string]any
			if err := json.Unmarshal(bytes.TrimSpace(stderr.Bytes()), &record); err != nil {
				t.Fatalf("rendered invalid JSON: %v: %q", err, stderr.String())
			}
			if record["event"] != test.event || record["projection_reason"] != test.reason {
				t.Fatalf("record = %#v; want event %q and the single reason %q", record, test.event, test.reason)
			}
			if record["event"] == "log_vocabulary_violation" {
				t.Fatalf("registered projection event became a vocabulary violation: %s", stderr.String())
			}
		})
	}
	if _, ok := buildEnvelope(Entry{
		Level: "warn", Event: "projection_skipped", Operation: "call_tool", Outcome: "degraded",
		Fields: map[string]any{"projection_reason": "unbounded-payload"},
	}); ok {
		t.Fatal("unlisted projection reason was accepted")
	}
}
