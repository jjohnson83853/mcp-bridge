package observe

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newTestDiagnostics builds a diagnostics owner with fixed clock and entropy.
func newTestDiagnostics(sink *bytes.Buffer, now func() time.Time, newID func() string) *HTTPDiagnostics {
	logger := NewLogger(sink)
	return NewHTTPDiagnostics(logger, HTTPDiagnosticsOptions{Now: now, NewID: newID})
}

func fixedNow() func() time.Time {
	t := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return func() time.Time { return t }
}

func fixedID(id string) func() string {
	return func() string { return id }
}

const testID = "0123456789abcdef0123456789abcdef"

func decodeTerminal(t *testing.T, sink *bytes.Buffer) map[string]any {
	t.Helper()
	var terminal map[string]any
	for _, line := range strings.Split(strings.TrimSpace(sink.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("decode record: %v", err)
		}
		if record["event"] == "http_complete" {
			if terminal != nil {
				t.Fatal("more than one http_complete record")
			}
			terminal = record
		}
	}
	if terminal == nil {
		t.Fatal("no http_complete record emitted")
	}
	return terminal
}

func TestOwnershipMatrix(t *testing.T) {
	now := fixedNow()
	t.Run("OuterOwnerSingleTerminal", func(t *testing.T) {
		sink := &bytes.Buffer{}
		diag := newTestDiagnostics(sink, now, fixedID(testID))
		innerCalls := 0
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			innerCalls++
			// A nested claim inside the outer owner is nonclaimant: the same
			// writer chain is preserved and no second header is installed.
			w2, r2, lease := diag.Claim(w, r)
			if lease.OwnsCompletion() {
				t.Error("nested claim must not own completion")
			}
			if w2 != w {
				t.Error("existing owner must preserve the incoming writer chain")
			}
			_ = r2
		})
		handler := diag.Wrap(inner)
		r := httptest.NewRequest(http.MethodGet, "/mcp", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		if innerCalls != 1 {
			t.Fatalf("inner calls = %d; want 1", innerCalls)
		}
		decodeTerminal(t, sink)
	})
	t.Run("DirectInnerClaims", func(t *testing.T) {
		sink := &bytes.Buffer{}
		diag := newTestDiagnostics(sink, now, fixedID(testID))
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _, lease := diag.Claim(w, r)
			if !lease.OwnsCompletion() {
				t.Error("standalone inner call must claim completion")
			}
			// A claimant lease owns the terminal record; without Finish the
			// claim would never emit http_complete.
			lease.Finish()
		})
		r := httptest.NewRequest(http.MethodGet, "/mcp", nil)
		rec := httptest.NewRecorder()
		inner.ServeHTTP(rec, r)
		decodeTerminal(t, sink)
	})
	t.Run("CarrierRecovery", func(t *testing.T) {
		sink := &bytes.Buffer{}
		diag := newTestDiagnostics(sink, now, fixedID(testID))
		var carrierWriter http.ResponseWriter
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate a lost context marker: a fresh request context without
			// the owner, while the writer still carries the facade.
			carrierWriter = w
			r = r.WithContext(httptest.NewRequest(http.MethodGet, "/mcp", nil).Context())
			_, _, lease := diag.Claim(w, r)
			if lease.OwnsCompletion() {
				t.Error("recovered owner is contribution-only: carrier recovery must not claim completion")
			}
		})
		handler := diag.Wrap(inner)
		r := httptest.NewRequest(http.MethodGet, "/mcp", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		_ = carrierWriter
		decodeTerminal(t, sink)
	})
}

func TestEntropyFailureLeavesNoSubstitute(t *testing.T) {
	now := fixedNow()
	calls := 0
	sink := &bytes.Buffer{}
	diag := newTestDiagnostics(sink, now, func() string { calls++; return "" })
	handler := diag.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := HTTPRequestID(r.Context()); id != "" {
			t.Errorf("HTTPRequestID = %q; want empty on entropy failure", id)
		}
	}))
	r := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	r.Header.Set("Mcp-Diagnostic-Id", "ffffffffffffffffffffffffffffffff")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)
	if calls != 1 {
		t.Fatalf("NewID calls = %d; want exactly 1 (no retry)", calls)
	}
	if rec.Header().Get("Mcp-Diagnostic-Id") != "" {
		t.Error("entropy failure must not install a diagnostic header")
	}
	record := decodeTerminal(t, sink)
	if got := record["correlation_state"]; got != "unavailable" {
		t.Fatalf("correlation_state = %v; want unavailable", got)
	}
	if _, present := record["request_id"]; present {
		t.Error("request_id must be absent on entropy failure")
	}
	if _, present := record["error"]; present {
		t.Errorf("canonical correlation error = %v; want no raw error object in typed bytes", record["error"])
	}
}

func TestPanicUnwindStillFinalizes(t *testing.T) {
	sink := &bytes.Buffer{}
	diag := newTestDiagnostics(sink, fixedNow(), fixedID(testID))
	handler := diag.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	r := httptest.NewRequest(http.MethodGet, "/livez", nil)
	rec := httptest.NewRecorder()
	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic must propagate, not be absorbed")
			}
		}()
		handler.ServeHTTP(rec, r)
	}()
	record := decodeTerminal(t, sink)
	if got := record["http_method"]; got != "GET" {
		t.Fatalf("http_method = %v; want GET", got)
	}
}

func TestZeroWriteStatusUnavailablePromotesWarn(t *testing.T) {
	sink := &bytes.Buffer{}
	diag := newTestDiagnostics(sink, fixedNow(), fixedID(testID))
	handler := diag.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	r := httptest.NewRequest(http.MethodGet, "/livez", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)
	record := decodeTerminal(t, sink)
	if got, ok := record["http_status"]; ok && got != nil {
		t.Fatalf("http_status = %v; want absent/null", got)
	}
	if got := record["level"]; got != "warn" {
		t.Fatalf("level = %v; want warn for unavailable response", got)
	}
}

func TestToolErrorUnderTwoHundredWarns(t *testing.T) {
	sink := &bytes.Buffer{}
	diag := newTestDiagnostics(sink, fixedNow(), fixedID(testID))
	handler := diag.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ContributeOperationEvidence(r.Context(), HTTPOperationEvidence{
			Relation: "single", IDState: "not_applicable", Upstream: "tool_error",
		})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)
	record := decodeTerminal(t, sink)
	if got := record["level"]; got != "warn" {
		t.Fatalf("level = %v; want warn for tool_error under 200", got)
	}
	if got := record["upstream_state"]; got != "tool_error" {
		t.Fatalf("upstream_state = %v; want tool_error", got)
	}
	if got := record["failure_reason"]; got != "none" {
		t.Fatalf("failure_reason = %v; want none (no false rejection)", got)
	}
}

func TestFirstDecisiveCauseImmutable(t *testing.T) {
	sink := &bytes.Buffer{}
	diag := newTestDiagnostics(sink, fixedNow(), fixedID(testID))
	handler := diag.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ContributeHTTP(r.Context(), HTTPDecision{
			FailureStage: "outer_oauth", FailureReason: "credentials_missing",
		})
		ContributeHTTP(r.Context(), HTTPDecision{
			FailureStage: "session", FailureReason: "session_absent",
		})
		w.WriteHeader(http.StatusUnauthorized)
	}))
	r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)
	record := decodeTerminal(t, sink)
	if got := record["failure_stage"]; got != "outer_oauth" {
		t.Fatalf("failure_stage = %v; want outer_oauth", got)
	}
	if got := record["failure_reason"]; got != "credentials_missing" {
		t.Fatalf("failure_reason = %v; want credentials_missing", got)
	}
}

func TestControlledClockDurationExpansionAndClamp(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// The composed clock REGRESSES a fixed step per call: the finish tick is
	// earlier than the entry tick, so the signed duration delta is negative
	// while the bounded semantic duration clamps to zero.
	calls := 0
	clock := func() time.Time {
		calls++
		return base.Add(-time.Duration(calls) * 500 * time.Millisecond)
	}
	sink := &bytes.Buffer{}
	diag := newTestDiagnostics(sink, clock, fixedID(testID))
	handler := diag.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	r := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)
	record := decodeTerminal(t, sink)
	// Signed legacy duration: negative deltas stay negative (expanded window).
	duration, ok := record["duration_ms"].(float64)
	if !ok {
		t.Fatal("duration_ms missing")
	}
	if duration >= 0 {
		t.Fatalf("duration_ms = %v; want a signed (negative) expanded delta preserved", duration)
	}
	// Bounded semantic duration clamps negatives to zero.
	if got := record["diag_duration_ms"]; got != float64(0) {
		t.Fatalf("diag_duration_ms = %v; want 0", got)
	}
	if got := record["duration_capped"]; got != true {
		t.Fatalf("duration_capped = %v; want true for negative legacy delta", got)
	}
}

func TestPrivacyCollisionAndBudgets(t *testing.T) {
	// Hostile legacy Fields carrying every owned key plus sentinels must never
	// reach the typed bytes: the typed lane ignores Fields entirely.
	hostileKeys := slotKeys
	sentinel := "LEAKED-SECRET-SENTINEL"
	fields := map[string]any{}
	for _, key := range hostileKeys {
		fields[key] = sentinel
	}
	fields["nested"] = map[string]any{"token": sentinel, "authorization": sentinel}

	status := 401
	entry := Entry{
		Time:      fixedNow()(),
		Level:     "warn",
		Event:     "http_complete",
		RequestID: testID,
		Operation: "request",
		Duration:  5 * time.Millisecond,
		Outcome:   "failure",
		Reason:    "none",
		Fields:    fields,
		HTTP: &HTTPDiagnostic{
			LegacyStage:                  "client_auth",
			HTTPStatus:                   &status,
			HTTPMethod:                   "POST",
			Route:                        "mcp",
			HTTPOutcome:                  "failure",
			HTTPBytesBucket:              "le_1k",
			StatusState:                  "observed",
			CorrelationState:             CorrelationAvailable,
			RequestID:                    testID,
			DiagSchema:                   "oauth_diag_v1",
			BuildState:                   "unavailable",
			ProcessState:                 "available",
			ProcessID:                    testID,
			FailureStage:                 "outer_oauth",
			FailureReason:                "credentials_missing",
			AuthMode:                     "oauth",
			CredentialState:              "missing",
			DiagDurationMS:               5,
			OperationID:                  testID,
			OperationRelation:            "single",
			OperationIDState:             "available",
			SessionRef:                   testID,
			BindingRelation:              "not_evaluated",
			SessionRefState:              "available",
			ErrorDetailState:             "included",
			PathClass:                    "mcp",
			RouteOutcome:                 "oauth_protected",
			ScopeState:                   "not_evaluated",
			SessionHeader:                "present",
			SessionState:                 "not_evaluated",
			ChallengeKind:                "none",
			ChallengeTarget:              "none",
			MetadataTargetRegistered:     "not_applicable",
			DiagnosticPolicyRevision:     "oauth-diag-policy-v1",
			EffectivePolicyRevisionState: "unavailable",
			PolicySource:                 "unavailable",
			ResourcePathMode:             "unavailable",
			SessionBinding:               "unavailable",
			UpstreamState:                "tool_error",
			ErrorDetail: &HTTPDiagnosticErrorDetail{
				Class: "validation", Code: "credentials_missing", Retry: "not_attempted",
			},
		},
	}

	sink := &bytes.Buffer{}
	logger := NewLogger(sink)
	logger.Log(entry)
	output := sink.String()
	if !strings.Contains(output, `"diag_schema":"oauth_diag_v1"`) {
		t.Fatal("typed record missing diag_schema")
	}
	if strings.Contains(output, sentinel) {
		t.Fatal("hostile Fields content leaked into typed record bytes")
	}
	if strings.Contains(output, `"fields"`) {
		t.Fatal("typed record must not carry legacy fields")
	}
	// Byte budgets on a maximal valid fixture.
	var record map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &record); err != nil {
		t.Fatalf("decode typed record: %v", err)
	}
	if len(output) > httpRecordBudget {
		t.Fatalf("complete record = %d bytes; want <= %d", len(output), httpRecordBudget)
	}
	encoder := &httpEncoder{}
	if !encoder.build(entry) {
		t.Fatal("maximal valid fixture failed validation")
	}
	core, optional := encoder.charges()
	if core > httpCoreBudget {
		t.Fatalf("core = %d; want <= %d", core, httpCoreBudget)
	}
	if optional > httpOptionalBudget {
		t.Fatalf("optional = %d; want <= %d", optional, httpOptionalBudget)
	}
}

func TestChargedBytesEqualEncodedRecord(t *testing.T) {
	// The budget arithmetic must charge exactly what encode emits, for every
	// slot kind. A maximal fixture mixing string, int and error-detail slots
	// with optional slots present is compared against the real encoded bytes.
	status := 401
	entry := Entry{
		Time:      fixedNow()(),
		Level:     "warn",
		Event:     "http_complete",
		RequestID: testID,
		Operation: "request",
		Duration:  5 * time.Millisecond,
		Outcome:   "failure",
		Reason:    "none",
		HTTP: &HTTPDiagnostic{
			LegacyStage:                  "client_auth",
			HTTPStatus:                   &status,
			HTTPMethod:                   "POST",
			Route:                        "mcp",
			HTTPOutcome:                  "failure",
			HTTPBytesBucket:              "le_1k",
			StatusState:                  "observed",
			CorrelationState:             CorrelationAvailable,
			RequestID:                    testID,
			DiagSchema:                   "oauth_diag_v1",
			BuildState:                   "unavailable",
			ProcessState:                 "available",
			ProcessID:                    testID,
			FailureStage:                 "outer_oauth",
			FailureReason:                "credentials_missing",
			AuthMode:                     "oauth",
			CredentialState:              "missing",
			DiagDurationMS:               5,
			OperationID:                  testID,
			OperationRelation:            "single",
			OperationIDState:             "available",
			SessionRef:                   testID,
			BindingRelation:              "not_evaluated",
			SessionRefState:              "available",
			ErrorDetailState:             "included",
			PathClass:                    "mcp",
			RouteOutcome:                 "oauth_protected",
			ScopeState:                   "not_evaluated",
			SessionHeader:                "present",
			SessionState:                 "not_evaluated",
			ChallengeKind:                "none",
			ChallengeTarget:              "none",
			MetadataTargetRegistered:     "not_applicable",
			DiagnosticPolicyRevision:     "oauth-diag-policy-v1",
			EffectivePolicyRevisionState: "unavailable",
			PolicySource:                 "unavailable",
			ResourcePathMode:             "unavailable",
			SessionBinding:               "unavailable",
			UpstreamState:                "tool_error",
			ErrorDetail: &HTTPDiagnosticErrorDetail{
				Class: "validation", Code: "credentials_missing", Retry: "not_attempted",
			},
		},
	}
	encoder := &httpEncoder{}
	if !encoder.build(entry) {
		t.Fatal("maximal mixed-slot fixture failed validation")
	}
	core, optional := encoder.charges()
	// The three framing bytes ('{', '}' and the terminal newline) sit outside
	// the slot charges, so the charged total plus framing must equal the
	// actual serialized record length.
	if got, want := core+optional+3, encoder.buf.Len(); got != want {
		t.Fatalf("charged %d bytes; encoded record is %d bytes", got, want)
	}
}

func TestShortSinkNonfatal(t *testing.T) {
	sink := &shortWriter{}
	logger := NewLogger(sink)
	diag := NewHTTPDiagnostics(logger, HTTPDiagnosticsOptions{Now: fixedNow(), NewID: fixedID(testID)})
	handler := diag.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	r := httptest.NewRequest(http.MethodGet, "/livez", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)
	// One attempt, no panic, no retry.
	if sink.writes != 1 {
		t.Fatalf("sink write attempts = %d; want exactly 1", sink.writes)
	}
}

type shortWriter struct {
	writes int
}

func (w *shortWriter) Write(p []byte) (int, error) {
	w.writes++
	if len(p) > 8 {
		return 8, io.ErrShortWrite
	}
	return len(p), nil
}

func TestGeneratorDeterminism(t *testing.T) {
	if testing.Short() {
		t.Skip("generator determinism requires the go toolchain")
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go toolchain not found: %v", err)
	}
	// Locate the module root from the package directory.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(filepath.Dir(cwd)) // observe -> internal -> root

	generate := func(dir string) []byte {
		out := filepath.Join(dir, "httpwriter_facades_generated.go")
		cmd := exec.Command(goTool, "run", "./internal/observe/internal/genhttpwriter", "-out", out)
		cmd.Dir = root
		if combined, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("generator run: %v\n%s", err, combined)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}

	dir1 := t.TempDir()
	dir2 := t.TempDir()
	first := generate(dir1)
	second := generate(dir2)
	if !bytes.Equal(first, second) {
		t.Fatal("generator output is not deterministic across runs")
	}
	checkedIn, err := os.ReadFile(filepath.Join(cwd, "httpwriter_facades_generated.go"))
	if err != nil {
		t.Fatalf("read checked-in generated file: %v", err)
	}
	if !bytes.Equal(first, checkedIn) {
		t.Fatal("generated output does not match the checked-in file")
	}
}
