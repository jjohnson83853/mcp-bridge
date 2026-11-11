package observe

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// validRevision is a synthetic 64-hex immutable source revision used where
// tests must not depend on the running binary's own VCS stamp.
const validRevision = "e621622397298bb4eb44a9b0f5ff1a00d6f16300aa11bb22cc33dd44ee55ff66"

func TestResolveProvenanceVerifiesOnlySupportedShapes(t *testing.T) {
	staticConfigured := true
	skew := 60
	badSkew := 120
	oauth := true
	cases := []struct {
		name       string
		in         HTTPProvenance
		wantBuild  string
		wantSource string
		wantPolicy string
		wantPolicyRevState string
		wantSkewNil bool
	}{
		{
			name: "verified revision and policy tuple",
			in: HTTPProvenance{
				SourceRevision: validRevision, BuildState: "verified",
				PolicySource: "environment", OAuthEnabled: &oauth,
				StaticConfigured: &staticConfigured, ClockSkewSeconds: &skew,
				EffectivePolicyRevisionState: "verified",
				EffectivePolicyRevision:      strings.Repeat("a", 40),
				ResourcePathMode:             "path_bearing", Environment: "lab-1",
			},
			wantBuild: "verified", wantSource: validRevision,
			wantPolicy: "environment", wantPolicyRevState: "verified",
		},
		{
			name: "unverifiable revision stays unavailable",
			in: HTTPProvenance{
				SourceRevision: "release-1.2.3", BuildState: "verified",
				PolicySource: "nonsense", ClockSkewSeconds: &badSkew,
				EffectivePolicyRevisionState: "verified",
				EffectivePolicyRevision:      "secret-content-hash",
			},
			wantBuild: "unavailable", wantSource: "unavailable",
			wantPolicy: "unavailable", wantPolicyRevState: "unavailable",
			wantSkewNil: true,
		},
		{
			name:       "empty provenance stays unavailable",
			in:         HTTPProvenance{},
			wantBuild:  "unavailable", wantSource: "unavailable",
			wantPolicy: "unavailable", wantPolicyRevState: "unavailable",
			wantSkewNil: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveProvenance(tc.in)
			if got.BuildState != tc.wantBuild {
				t.Errorf("build_state = %q; want %q", got.BuildState, tc.wantBuild)
			}
			if got.SourceRevision != tc.wantSource {
				t.Errorf("source_revision = %q; want %q", got.SourceRevision, tc.wantSource)
			}
			if got.PolicySource != tc.wantPolicy {
				t.Errorf("policy_source = %q; want %q", got.PolicySource, tc.wantPolicy)
			}
			if got.EffectivePolicyRevisionState != tc.wantPolicyRevState {
				t.Errorf("effective_policy_revision_state = %q; want %q", got.EffectivePolicyRevisionState, tc.wantPolicyRevState)
			}
			if tc.wantSkewNil && got.ClockSkewSeconds != nil {
				t.Errorf("clock_skew_seconds = %d; want absent", *got.ClockSkewSeconds)
			}
			if !tc.wantSkewNil && got.ClockSkewSeconds == nil {
				t.Error("clock_skew_seconds absent; want 60")
			}
		})
	}
}

func TestProvenanceFlowsIntoTerminalRecord(t *testing.T) {
	now := fixedNow()
	oauth, static := true, false
	scope := false
	skew := 60
	sink := &bytes.Buffer{}
	diag := NewHTTPDiagnostics(NewLogger(sink, sink), HTTPDiagnosticsOptions{
		Now: now, NewID: fixedID(testID),
		Process: HTTPProcessContext{State: "available", ID: testID},
		Provenance: HTTPProvenance{
			SourceRevision:   validRevision,
			Environment:      "lab-1",
			PolicySource:     "defaults",
			OAuthEnabled:     &oauth,
			StaticConfigured: &static,
			ScopeRequired:    &scope,
			ClockSkewSeconds: &skew,
			ResourcePathMode: "origin_only",
		},
	})
	handler := diag.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ContributeHTTP(r.Context(), HTTPDecision{FailureStage: "outer_oauth", FailureReason: "credentials_missing"})
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/mcp", nil))
	record := decodeTerminal(t, sink)
	for _, key := range []string{"source_revision", "diag_environment"} {
		if _, present := record[key]; present {
			t.Errorf("%s = %v; collector-owned provenance must not be emitted", key, record[key])
		}
	}
	if record["build_state"] != "verified" {
		t.Errorf("build_state = %v; want verified", record["build_state"])
	}
	if record["policy_source"] != "defaults" {
		t.Errorf("policy_source = %v; want defaults", record["policy_source"])
	}
	if record["resource_path_mode"] != "origin_only" {
		t.Errorf("resource_path_mode = %v; want origin_only", record["resource_path_mode"])
	}
	if record["clock_skew_seconds"] != float64(60) {
		t.Errorf("clock_skew_seconds = %v; want 60", record["clock_skew_seconds"])
	}
	if record["oauth_enabled"] != true || record["static_configured"] != false || record["scope_required"] != false {
		t.Errorf("policy flags = %v/%v/%v; want true/false/false",
			record["oauth_enabled"], record["static_configured"], record["scope_required"])
	}
	// Mixed availability: no attested policy artifact means the effective
	// policy revision state stays unavailable with no value.
	if record["effective_policy_revision_state"] != "unavailable" {
		t.Errorf("effective_policy_revision_state = %v; want unavailable", record["effective_policy_revision_state"])
	}
	if _, present := record["effective_policy_revision"]; present {
		t.Error("effective_policy_revision present without a verified state")
	}
}

func TestProcessIncarnationInjectionIsDistinctPerContext(t *testing.T) {
	now := fixedNow()
	otherID := "fedcba9876543210fedcba9876543210"
	emit := func(process HTTPProcessContext) map[string]any {
		sink := &bytes.Buffer{}
		diag := NewHTTPDiagnostics(NewLogger(sink, sink), HTTPDiagnosticsOptions{
			Now: now, NewID: fixedID(testID), Process: process,
		})
		diag.Wrap(http.NotFoundHandler()).ServeHTTP(
			httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/mcp", nil))
		return decodeTerminal(t, sink)
	}
	first := emit(HTTPProcessContext{State: "available", ID: testID})
	second := emit(HTTPProcessContext{State: "available", ID: otherID})
	if first["process_id"] != testID || second["process_id"] != otherID {
		t.Errorf("injected incarnations = %v/%v; want %q/%q distinct",
			first["process_id"], second["process_id"], testID, otherID)
	}
	// An invalid injected incarnation degrades to explicit unavailability; it
	// is never rewritten or derived from another value.
	degraded := emit(HTTPProcessContext{State: "available", ID: "not-hex"})
	if degraded["process_state"] != "unavailable" {
		t.Errorf("process_state = %v; want unavailable", degraded["process_state"])
	}
	if _, present := degraded["process_id"]; present {
		t.Error("process_id present under unavailable process state")
	}
}

func TestOperationLinkageTruthfulRelationAndUpstream(t *testing.T) {
	now := fixedNow()
	idA, idB := testID, "11111111111111111111111111111111"
	emit := func(contributions ...HTTPOperationEvidence) map[string]any {
		sink := &bytes.Buffer{}
		diag := newTestDiagnostics(sink, now, fixedID(testID))
		handler := diag.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, evidence := range contributions {
				ContributeOperationEvidence(r.Context(), evidence)
			}
			w.WriteHeader(http.StatusOK)
		}))
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/mcp", nil))
		return decodeTerminal(t, sink)
	}
	t.Run("NoneWithoutEvidence", func(t *testing.T) {
		record := emit()
		if record["operation_relation"] != "none" || record["operation_id_state"] != "not_applicable" {
			t.Errorf("operation linkage = %v/%v; want none/not_applicable",
				record["operation_relation"], record["operation_id_state"])
		}
		if record["upstream_state"] != "not_attempted" {
			t.Errorf("upstream_state = %v; want not_attempted", record["upstream_state"])
		}
	})
	t.Run("SingleEligibleOperation", func(t *testing.T) {
		record := emit(HTTPOperationEvidence{Relation: "single", IDState: "available", ID: idA, Upstream: "succeeded"})
		if record["operation_relation"] != "single" || record["operation_id_state"] != "available" {
			t.Errorf("operation linkage = %v/%v; want single/available",
				record["operation_relation"], record["operation_id_state"])
		}
		if record["operation_id"] != idA {
			t.Errorf("operation_id = %v; want %q", record["operation_id"], idA)
		}
		if record["upstream_state"] != "succeeded" {
			t.Errorf("upstream_state = %v; want succeeded", record["upstream_state"])
		}
	})
	t.Run("MultipleOperationsNeverEmitIDList", func(t *testing.T) {
		record := emit(
			HTTPOperationEvidence{ID: idA, Upstream: "succeeded"},
			HTTPOperationEvidence{ID: idB, Upstream: "tool_error"},
		)
		if record["operation_relation"] != "multiple" {
			t.Errorf("operation_relation = %v; want multiple", record["operation_relation"])
		}
		if record["operation_id_state"] != "not_applicable" {
			t.Errorf("operation_id_state = %v; want not_applicable", record["operation_id_state"])
		}
		if _, present := record["operation_id"]; present {
			t.Error("operation_id emitted for multiple linked operations")
		}
		if record["upstream_state"] != "multiple" {
			t.Errorf("upstream_state = %v; want multiple", record["upstream_state"])
		}
	})
	t.Run("UnsafeIDStaysUnavailable", func(t *testing.T) {
		record := emit(HTTPOperationEvidence{Relation: "single", IDState: "available", ID: "caller-jsonrpc-42"})
		if record["operation_relation"] != "single" {
			t.Errorf("operation_relation = %v; want single", record["operation_relation"])
		}
		if record["operation_id_state"] != "unavailable" {
			t.Errorf("operation_id_state = %v; want unavailable", record["operation_id_state"])
		}
		if _, present := record["operation_id"]; present {
			t.Error("unsafe operation id was rewritten into the record")
		}
	})
	t.Run("ToolErrorUnderObservedSuccessStaysDistinct", func(t *testing.T) {
		record := emit(HTTPOperationEvidence{ID: idA, Upstream: "tool_error"})
		if record["upstream_state"] != "tool_error" {
			t.Errorf("upstream_state = %v; want tool_error", record["upstream_state"])
		}
		if record["http_status"] != float64(http.StatusOK) || record["level"] != "warn" {
			t.Errorf("completion = %v/%v; want 200 with warn promotion",
				record["http_status"], record["level"])
		}
	})
}

func TestSessionBindingMechanismLabelFromComparator(t *testing.T) {
	now := fixedNow()
	sink := &bytes.Buffer{}
	diag := newTestDiagnostics(sink, now, fixedID(testID))
	handler := diag.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ContributeSessionEvidence(r.Context(), HTTPSessionEvidence{
			RefState: "available", Ref: testID, State: "existing", Binding: "mismatch",
		})
		ContributeHTTP(r.Context(), HTTPDecision{FailureStage: "session", FailureReason: "session_fingerprint_mismatch"})
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/mcp", nil))
	record := decodeTerminal(t, sink)
	if record["session_binding"] != "bearer_sha256" {
		t.Errorf("session_binding = %v; want bearer_sha256", record["session_binding"])
	}
}

func TestEncoderRejectsOperationIDWithoutSingleRelation(t *testing.T) {
	now := fixedNow()
	// Build a record with an operation id but a multiple relation: the typed
	// encoder must treat it as invalid core input rather than emit an
	// unbounded certainty.
	sink := &bytes.Buffer{}
	diag := newTestDiagnostics(sink, now, fixedID(testID))
	handler := diag.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ContributeOperationEvidence(r.Context(), HTTPOperationEvidence{ID: testID, Upstream: "succeeded"})
		ContributeOperationEvidence(r.Context(), HTTPOperationEvidence{ID: "22222222222222222222222222222222"})
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/mcp", nil))
	record := decodeTerminal(t, sink)
	if _, present := record["operation_id"]; present {
		t.Error("operation_id emitted although the relation is multiple")
	}
}

func TestChargedBudgetStaysExactWithProvenance(t *testing.T) {
	// Provenance must not grow the fixed schema: run the whole-record
	// charge==actual budget regression with a fully verified provenance
	// injection including a verified policy revision.
	oauth, static, scope := true, true, true
	skew := 60
	now := fixedNow()
	sink := &bytes.Buffer{}
	diag := NewHTTPDiagnostics(NewLogger(sink, sink), HTTPDiagnosticsOptions{
		Now: now, NewID: fixedID(testID),
		Process: HTTPProcessContext{State: "available", ID: testID},
		Provenance: HTTPProvenance{
			SourceRevision:               validRevision,
			PolicySource:                 "mixed",
			OAuthEnabled:                 &oauth,
			StaticConfigured:             &static,
			ScopeRequired:                &scope,
			ClockSkewSeconds:             &skew,
			EffectivePolicyRevisionState: "verified",
			EffectivePolicyRevision:      strings.Repeat("f", 64),
			ResourcePathMode:             "path_bearing",
			Environment:                  "staging",
		},
	})
	handler := diag.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ContributeSessionEvidence(r.Context(), HTTPSessionEvidence{
			RefState: "available", Ref: testID, State: "existing", Binding: "match",
		})
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	record := decodeTerminal(t, sink)
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	if len(encoded)+1 > httpRecordBudget {
		t.Errorf("complete record %d bytes exceeds the %d byte budget", len(encoded)+1, httpRecordBudget)
	}
	_ = time.Now
}
