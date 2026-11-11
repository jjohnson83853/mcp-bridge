package observe

// This file holds the closed diagnostic vocabulary and the concrete snapshot
// types of the oauth_diag_v1 HTTP completion schema. Every value that reaches
// the fixed-slot projection is validated against these finite sets; the
// snapshot itself carries no maps, slices, interfaces or custom marshaling.

// HTTPProcessContext is the independent per-process diagnostic incarnation.
// State is available/unavailable; ID is a 32-character lowercase hexadecimal
// identifier drawn once per process and stable across reloads.
type HTTPProcessContext struct {
	State string
	ID    string
}

// HTTPProvenance is the safe runtime provenance captured once at trusted
// startup wiring from authoritative existing interfaces. Every string field
// is a closed finite value or empty for its truthful unavailable state; the
// policy revision never hashes secret-bearing configuration and the source
// revision is a verified immutable VCS revision, separate from the process
// incarnation and from mutable deployment tags.
type HTTPProvenance struct {
	Environment                  string
	SourceRevision               string
	BuildState                   string
	PolicySource                 string
	OAuthEnabled                 *bool
	StaticConfigured             *bool
	ScopeRequired                *bool
	ClockSkewSeconds             *int
	EffectivePolicyRevisionState string
	EffectivePolicyRevision      string
	ResourcePathMode             string
}

// Read-only write-failure classes of the terminal observation. The raw error
// is retained internally and never serialized.
const (
	HTTPWriteNone         = "none"
	HTTPWriteDisconnected = "client_disconnected"
	HTTPWriteCancelled    = "request_cancelled"
	HTTPWriteDeadline     = "deadline_exceeded"
	HTTPWriteUnknown      = "unknown"
)

// Diagnostic error detail classes.
const (
	HTTPDetailValidation = "validation"
	HTTPDetailDependency = "dependency"
	HTTPDetailAdmission  = "admission"
	HTTPDetailProtocol   = "protocol"
	HTTPDetailTool       = "tool"
	HTTPDetailInternal   = "internal"
)

// Finite closed vocabularies of the typed HTTP schema. The encoder validates
// every string slot against the matching set before emitting it.
var (
	httpFailureReasonValues = []string{
		"none", "credentials_missing", "bearer_malformed", "jwt_malformed",
		"jwt_signature_invalid", "jwt_key_unavailable", "jwt_expired",
		"jwt_not_yet_valid", "jwt_issuer_invalid", "jwt_audience_invalid",
		"jwt_invalid_other", "scope_insufficient", "static_invalid",
		"auth_mode_unavailable", "session_header_missing", "session_absent",
		"session_fingerprint_mismatch", "host_denied", "origin_denied",
		"request_too_large", "session_capacity", "route_not_found",
		"protocol_rejected", "upstream_transport_failed", "upstream_auth_failed",
		"tool_error", "unavailable",
	}
	httpFailureStageValues = []string{
		"none", "route", "outer_oauth", "inner_credential", "host", "origin",
		"session", "body_limit", "capacity", "protocol", "upstream", "unavailable",
	}
	httpAuthModeValues           = []string{"oauth", "static", "none", "bypass", "unavailable"}
	httpCredentialValues         = []string{"not_evaluated", "missing", "accepted", "rejected", "unavailable"}
	httpScopeStateValues         = []string{"not_evaluated", "not_required", "accepted", "rejected", "unavailable"}
	httpSessionHeaderValues      = []string{"absent", "present"}
	httpSessionStateValues       = []string{"not_evaluated", "not_supplied", "absent", "rehydrated", "existing", "created", "unavailable"}
	httpBindingValues            = []string{"not_evaluated", "match", "mismatch", "unavailable"}
	httpSessionRefValues         = []string{"available", "unavailable", "not_applicable", "omitted_for_budget"}
	httpChallengeKindValues      = []string{"none", "resource_metadata", "insufficient_scope", "bearer_only", "other", "unavailable"}
	httpChallengeTargetValues    = []string{"none", "root_well_known", "path_appended_well_known", "other", "unavailable"}
	httpMetadataRegisteredValues = []string{"yes", "no", "not_applicable", "unavailable"}
	httpPolicySourceValues       = []string{"defaults", "environment", "file", "mixed", "unavailable"}
	httpPathModeValues           = []string{"origin_only", "path_bearing", "unavailable"}
	httpSessionBindingValues     = []string{"bearer_sha256", "unavailable"}
	httpUpstreamValues           = []string{"not_attempted", "succeeded", "transport_failed", "auth_failed", "tool_error", "multiple", "unavailable"}
	httpPathClassValues          = []string{"mcp", "metadata_root", "health", "other"}
	httpRouteOutcomeValues       = []string{"oauth_protected", "inbound_direct", "metadata", "health", "not_found", "other", "unavailable"}
	httpRelationValues           = []string{"none", "single", "multiple", "unavailable"}
	httpOperationIDStateValues   = []string{"available", "not_applicable", "unavailable", "omitted_for_budget"}
	httpBuildStateValues         = []string{"verified", "unavailable"}
	httpDetailClassValues        = []string{"validation", "dependency", "admission", "protocol", "tool", "internal", "unavailable"}
	httpDetailRetryValues        = []string{"not_attempted", "observed", "unavailable"}
	httpErrorClassValues         = []string{"correlation_unavailable", "client_disconnected", "unknown"}
	clientFamilyVocabulary       = []string{"openai", "claude", "cursor", "omp", "browser", "other", "absent"}
)


// HTTPDiagnosticErrorDetail is the optional closed error detail object.
type HTTPDiagnosticErrorDetail struct {
	Class string `json:"class"`
	Code  string `json:"code"`
	Retry string `json:"retry"`
}

// HTTPDiagnostic is the closed concrete terminal snapshot of one completed
// HTTP request under schema oauth_diag_v1. Fields bind one-to-one to the
// approved 60-key table; conditional values are empty/nil only for truthful
// unavailable or omitted states.
type HTTPDiagnostic struct {
	// Legacy stage field (existing physical key "stage", preserved semantics).
	LegacyStage string `json:"-"`

	// Boundary observation (existing physical keys).
	HTTPStatus      *int   `json:"http_status"`
	HTTPMethod      string `json:"http_method,omitempty"`
	Route           string `json:"route,omitempty"`
	HTTPOutcome     string `json:"http_outcome,omitempty"`
	HTTPBytesBucket string `json:"http_bytes_bucket,omitempty"`
	Operation       string `json:"operation,omitempty"`
	ClientFamily    string `json:"client_family,omitempty"`
	HTTPBytes       int64  `json:"http_bytes,omitempty"`
	ExposedTools    *int   `json:"exposed_tools,omitempty"`
	StatusState     string `json:"status_state,omitempty"`

	// Correlation.
	CorrelationState string `json:"correlation_state,omitempty"`
	RequestID        string `json:"request_id,omitempty"`

	// Semantic schema and provenance.
	DiagSchema      string `json:"diag_schema,omitempty"`
	BuildState      string `json:"build_state,omitempty"`
	ProcessState    string `json:"process_state,omitempty"`
	ProcessID       string `json:"process_id,omitempty"`

	// Operation linkage.
	OperationRelation string `json:"operation_relation,omitempty"`
	OperationIDState  string `json:"operation_id_state,omitempty"`
	OperationID       string `json:"operation_id,omitempty"`
	PathClass         string `json:"path_class,omitempty"`
	RouteOutcome      string `json:"route_outcome,omitempty"`

	// Bounded semantic duration, distinct from the preserved signed
	// legacy duration_ms measurement.
	DiagDurationMS int64 `json:"diag_duration_ms"`
	DurationCapped bool  `json:"duration_capped"`

	// First decisive rejection and evaluation states.
	FailureStage                 string                     `json:"failure_stage,omitempty"`
	FailureReason                string                     `json:"failure_reason,omitempty"`
	AuthMode                     string                     `json:"auth_mode,omitempty"`
	CredentialState              string                     `json:"credential_state,omitempty"`
	ScopeState                   string                     `json:"scope_state,omitempty"`
	SessionHeader                string                     `json:"session_header,omitempty"`
	SessionState                 string                     `json:"session_state,omitempty"`
	BindingRelation              string                     `json:"binding_relation,omitempty"`
	SessionRefState              string                     `json:"session_ref_state,omitempty"`
	SessionRef                   string                     `json:"session_ref,omitempty"`
	ChallengeKind                string                     `json:"challenge_kind,omitempty"`
	ChallengeTarget              string                     `json:"challenge_target,omitempty"`
	MetadataTargetRegistered     string                     `json:"metadata_target_registered,omitempty"`
	DiagnosticPolicyRevision     string                     `json:"diagnostic_policy_revision,omitempty"`
	EffectivePolicyRevisionState string                     `json:"effective_policy_revision_state,omitempty"`
	EffectivePolicyRevision      string                     `json:"effective_policy_revision,omitempty"`
	PolicySource                 string                     `json:"policy_source,omitempty"`
	OAuthEnabled                 *bool                      `json:"oauth_enabled"`
	StaticConfigured             *bool                      `json:"static_configured"`
	ScopeRequired                *bool                      `json:"scope_required"`
	ResourcePathMode             string                     `json:"resource_path_mode,omitempty"`
	SessionBinding               string                     `json:"session_binding,omitempty"`
	ClockSkewSeconds             *int                       `json:"clock_skew_seconds"`
	UpstreamState                string                     `json:"upstream_state,omitempty"`
	ErrorDetailState             string                     `json:"error_detail_state,omitempty"`
	ErrorDetail                  *HTTPDiagnosticErrorDetail `json:"error_detail,omitempty"`
}

// HTTPDecision is the closed typed evidence of one admission/auth decision.
// Zero-value fields are ignored; only authoritative branches contribute.
type HTTPDecision struct {
	FailureStage             string
	FailureReason            string
	AuthMode                 string
	CredentialState          string
	ScopeState               string
	SessionHeader            string
	SessionState             string
	BindingRelation          string
	ChallengeKind            string
	ChallengeTarget          string
	MetadataTargetRegistered string
	RouteOutcome             string
}

// HTTPSessionEvidence carries owned session evaluation evidence. Only closed
// states and independently generated owned references are accepted; session
// values and digests never enter this type.
type HTTPSessionEvidence struct {
	RefState string
	Ref      string
	State    string
	Binding  string
}

// HTTPOperationEvidence carries actual downstream operation linkage and
// outcome evidence.
type HTTPOperationEvidence struct {
	Relation string
	IDState  string
	ID       string
	Upstream string
}

// HTTPLegacyEvidence carries the summarised legacy classification the inner
// edge computed at its boundaries.
type HTTPLegacyEvidence struct {
	Stage     string
	Reason    string
	Operation string
}

// HTTPObservation is the read-only response observation returned by
// HTTPResponseObservation. Status 0 means the status was never observed
// through writer operations; WriteFailure is a closed class, never raw text.
type HTTPObservation struct {
	Status       int
	Bytes        int64
	WriteFailure string
}
