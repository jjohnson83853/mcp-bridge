package observe

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// This file extends the ticket-02 budget/omission/collision regressions to the
// session outcome fields: the session reference is an optional slot dropped in
// the approved order, and hostile session values are suppressed, never
// inherited, mutated or emitted.

// sessionMaximalFixture builds a maximal valid fixture carrying every session
// slot, mirroring the ticket-02 maximal valid record.
func sessionMaximalFixture() (Entry, string) {
	status := 401
	testID := "0123456789abcdef0123456789abcdef"
	entry := Entry{
		Time:      time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		Level:     "warn",
		Event:     "http_complete",
		RequestID: testID,
		Operation: "request",
		Duration:  5 * time.Millisecond,
		Outcome:   "failure",
		Reason:    "session_auth_failed",
		HTTP: &HTTPDiagnostic{
			LegacyStage:                  "session_admission",
			Operation:                    "request",
			ClientFamily:                 "openai",
			HTTPBytes:                    0,
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
			FailureStage:                 "session",
			FailureReason:                "session_fingerprint_mismatch",
			AuthMode:                     "oauth",
			CredentialState:              "accepted",
			ScopeState:                   "accepted",
			SessionHeader:                "present",
			SessionState:                 "existing",
			BindingRelation:              "mismatch",
			SessionRefState:              "available",
			SessionRef:                   testID,
			OperationID:                  testID,
			OperationRelation:            "single",
			OperationIDState:             "available",
			PathClass:                    "mcp",
			RouteOutcome:                 "oauth_protected",
			ChallengeKind:                "none",
			ChallengeTarget:              "none",
			MetadataTargetRegistered:     "not_applicable",
			DiagnosticPolicyRevision:     "oauth-diag-policy-v1",
			EffectivePolicyRevisionState: "unavailable",
			PolicySource:                 "unavailable",
			ResourcePathMode:             "unavailable",
			SessionBinding:               "unavailable",
			UpstreamState:                "not_attempted",
			ErrorDetailState:             "included",
			ErrorDetail: &HTTPDiagnosticErrorDetail{
				Class: "admission", Code: "session_fingerprint_mismatch", Retry: "not_attempted",
			},
		},
	}
	return entry, testID
}

func TestSessionSlotBudgetArithmeticMatchesEncodedBytes(t *testing.T) {
	entry, _ := sessionMaximalFixture()
	encoder := &httpEncoder{}
	if !encoder.build(entry) {
		t.Fatal("session maximal fixture failed validation")
	}
	core, optional := encoder.charges()
	if core > httpCoreBudget || optional > httpOptionalBudget || core+optional+3 != encoder.buf.Len() {
		t.Fatalf("charges core=%d optional=%d encoded=%d", core, optional, encoder.buf.Len())
	}
	// The session reference charge sits in the optional budget, not core.
	if !strings.Contains(encoder.buf.String(), `"session_ref":"0123456789abcdef0123456789abcdef"`) {
		t.Fatal("session reference slot missing from maximal record")
	}
}

func TestSessionReferenceOmissionOrderAndStateFlip(t *testing.T) {
	// White-box: the approved omission order drops the error detail, then the
	// operation ID, then the session reference, each flipping exactly its own
	// state slot; core is never dropped.
	encoder := &httpEncoder{}
	entry, testID := sessionMaximalFixture()
	if !encoder.build(entry) {
		t.Fatal("fixture failed validation")
	}
	encoder.dropForBudget()
	if encoder.slots[56].present || encoder.slots[57].present || encoder.slots[58].present {
		t.Fatal("optional slots survived the budget drop")
	}
	if got := encoder.slots[55].str; got != "omitted_for_budget" {
		t.Fatalf("error_detail_state=%q", got)
	}
	if got := encoder.slots[26].str; got != "omitted_for_budget" {
		t.Fatalf("operation_id_state=%q", got)
	}
	if got := encoder.slots[40].str; got != "omitted_for_budget" {
		t.Fatalf("session_ref_state=%q, want omitted_for_budget with relation preserved", got)
	}
	// Core session truth survives the optional omission.
	if got := encoder.slots[38].str; got != "existing" {
		t.Fatalf("session_state=%q lost for budget", got)
	}
	if got := encoder.slots[39].str; got != "mismatch" {
		t.Fatalf("binding_relation=%q lost for budget", got)
	}
	// The dropped record still encodes within budget.
	core, optional := encoder.charges()
	if core > httpCoreBudget || optional > httpOptionalBudget {
		t.Fatalf("post-drop charges core=%d optional=%d", core, optional)
	}
	_ = testID
}

func TestHostileSessionValuesSuppressRatherThanEmit(t *testing.T) {
	// Invalid session enumeration values are core-invalid: the whole typed
	// record is suppressed, nothing is emitted, and no stale value can leak.
	status := 401
	corruptions := map[string]func(*HTTPDiagnostic){
		"session_state":     func(d *HTTPDiagnostic) { d.SessionState = "LEAKED-SECRET-SENTINEL" },
		"binding_relation":  func(d *HTTPDiagnostic) { d.BindingRelation = "LEAKED-SECRET-SENTINEL" },
		"session_header":    func(d *HTTPDiagnostic) { d.SessionHeader = "LEAKED-SECRET-SENTINEL" },
		"session_ref_state": func(d *HTTPDiagnostic) { d.SessionRefState = "LEAKED-SECRET-SENTINEL" },
	}
	for name, corrupt := range corruptions {
		t.Run(name, func(t *testing.T) {
			entry, testID := sessionMaximalFixture()
			corrupt(entry.HTTP)
			entry.HTTP.HTTPStatus = &status
			encoder := &httpEncoder{}
			if encoder.build(entry) {
				t.Fatalf("hostile %s value produced a record", name)
			}
			if strings.Contains(encoder.buf.String(), "LEAKED-SECRET-SENTINEL") {
				t.Fatalf("hostile %s value leaked into emitted bytes", name)
			}
			// A malformed session reference (right length, hostile bytes) must
			// not be emitted either.
			entry, _ = sessionMaximalFixture()
			entry.HTTP.SessionRef = "0123456789abcdef0123456789abcdeX"
			entry.HTTP.HTTPStatus = &status
			encoder = &httpEncoder{}
			if encoder.build(entry) {
				t.Fatal("malformed session reference produced a record")
			}
			if strings.Contains(encoder.buf.String(), testID[:31]+"X") {
				t.Fatal("malformed session reference leaked into emitted bytes")
			}
		})
	}
}

func TestSessionReferenceNotInheritedAcrossRecords(t *testing.T) {
	// Delete-first: an encoder that emitted a session reference must leave no
	// residue for the next record; each record builds fresh state.
	first, _ := sessionMaximalFixture()
	sink := &bytes.Buffer{}
	logger := NewLogger(sink)
	logger.Log(first)
	logger.Log(Entry{
		Time:      time.Date(2026, 1, 1, 12, 0, 1, 0, time.UTC),
		Level:     "warn",
		Event:     "http_complete",
		RequestID: "fedcba9876543210fedcba9876543210",
		Operation: "request",
		Duration:  5 * time.Millisecond,
		Outcome:   "failure",
		Reason:    "session_auth_failed",
		HTTP: func() *HTTPDiagnostic {
			entry, _ := sessionMaximalFixture()
			entry.Time = time.Date(2026, 1, 1, 12, 0, 1, 0, time.UTC)
			entry.RequestID = "fedcba9876543210fedcba9876543210"
			// Absent entry: no reference, truthful not_applicable state.
			entry.HTTP.SessionRef = ""
			entry.HTTP.SessionRefState = "not_applicable"
			entry.HTTP.FailureReason = "session_absent"
			entry.HTTP.SessionState = "absent"
			entry.HTTP.BindingRelation = "not_evaluated"
			return entry.HTTP
		}(),
	})
	lines := strings.Split(strings.TrimSpace(sink.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("records=%d, want 2", len(lines))
	}
	var second map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("decode second record: %v", err)
	}
	if _, ok := second["session_ref"]; ok {
		t.Fatalf("second record inherited the first record's session reference: %v", second["session_ref"])
	}
	if second["session_ref_state"] != "not_applicable" || second["session_state"] != "absent" {
		t.Fatalf("second record session_ref_state=%v session_state=%v", second["session_ref_state"], second["session_state"])
	}
}
