package observe

import (
	"bytes"
	"strconv"
	"time"
)

// This file owns the typed HTTP lane of the existing logger sink. The legacy
// path is byte-for-byte unchanged; a non-http_complete Entry with HTTP set is
// an invalid record and suppresses the typed lane (nonfatal, no partial
// write). The typed path builds fixed slots, validates them, measures the
// exact encoded length once and attempts one bounded sink write.

// Byte budgets of the complete record.
const (
	httpCoreBudget     = 3584
	httpOptionalBudget = 512
	httpRecordBudget   = 4096
)

// httpMessage is the fixed message of the terminal record.
const httpMessage = "http request completed"

// httpEncoder builds and writes one typed record. A fresh encoder and buffer
// are allocated per record by logHTTP; growth is capped at the fixed record
// budget.
type httpEncoder struct {
	buf        bytes.Buffer
	slots      [numHTTPSlots]httpSlotValue
	optional   [3]int
	optionalIx int
}

type httpSlotValue struct {
	present bool
	kind    httpSlotKind
	str     string
	i       int64
	b       bool
	obj     *HTTPDiagnosticErrorDetail
}

type httpSlotKind uint8

const (
	slotString httpSlotKind = iota
	slotInt
	slotBool
	slotNull
	slotErrorDetail
	slotErrorClass
)

// numHTTPSlots is the total slot count: 59 core plus 3 optional, plus the
// legacy outcome and canonical error fields.
const numHTTPSlots = 64

// Optional slot indexes, in omission order.
const (
	optErrorDetail = iota
	optOperationID
	optSessionRef
)

// slotKeys are the physical keys in fixed order.
var slotKeys = [numHTTPSlots]string{
	"time", "level", "event", "message", "service", "environment", "version",
	"generation", "duration_ms", "http_outcome", "reason", "stage",
	"http_status", "http_method", "route", "http_bytes_bucket", "timing_scope",
	"correlation_state", "request_id", "diag_schema", "diag_environment", "source_revision",
	"build_state", "process_state", "process_id", "operation_relation",
	"operation_id_state", "path_class", "route_outcome", "status_state",
	"diag_duration_ms", "duration_capped", "failure_stage", "failure_reason",
	"auth_mode", "credential_state", "scope_state", "session_header",
	"session_state", "binding_relation", "session_ref_state", "challenge_kind",
	"challenge_target", "metadata_target_registered", "diagnostic_policy_revision",
	"effective_policy_revision_state", "effective_policy_revision",
	"policy_source", "oauth_enabled", "static_configured", "scope_required",
	"resource_path_mode", "session_binding", "clock_skew_seconds",
	"upstream_state", "error_detail_state", "error_detail", "operation_id",
	"session_ref",
	"outcome", "error",
}

// httpErrorClassValues re-declared from the validation table for the
// slot-level check of the conditional core error object.
func validHTTPErrorClass(class string) bool {
	return class == "correlation_unavailable" || class == "client_disconnected" ||
		class == "unknown"
}

func validDetailClass(class string) bool {
	switch class {
	case "validation", "dependency", "admission", "protocol", "tool", "internal":
		return true
	default:
		return false
	}
}

func validDetailRetry(retry string) bool {
	return retry == "not_attempted" || retry == "observed" || retry == "unavailable"
}

func validDetailState(state string) bool {
	return state == "included" || state == "not_applicable" || state == "unavailable" ||
		state == "omitted_for_budget"
}

// stringEnum validates membership; empty values are only valid where the
// default absence semantics require them.
func stringEnum(value string, allowed []string) (string, bool) {
	return finiteEnum(allowed, value)
}

func validTimestamp(value string) bool {
	if len(value) == 0 || len(value) > 30 || value[len(value)-1] != 'Z' {
		return false
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return false
	}
	return parsed.Format(time.RFC3339Nano) == value
}

func validLevel(value string) bool {
	return value == "info" || value == "warn" || value == "error"
}

func validOutcome(value string) bool {
	return value == "success" || value == "failure" || value == "incomplete" || value == "unknown"
}

func validLegacyReason(value string) bool {
	_, ok := finiteEnum(reasonVocabulary, value)
	return ok
}

func validLegacyStage(value string) bool {
	_, ok := finiteEnum(legacyStageVocabulary, value)
	return ok
}

func validOperation(value string) bool {
	_, ok := finiteEnum(operationVocabulary, value)
	return ok
}

func validMethod(value string) bool {
	switch value {
	case "GET", "POST", "DELETE", "HEAD", "OPTIONS", "PUT", "PATCH", "OTHER":
		return true
	default:
		return false
	}
}

func validRoute(value string) bool {
	return value == "mcp" || value == "not_found" || value == "other"
}

func validCorrelationState(value string) bool {
	return value == "available" || value == "unavailable"
}

func validStatusState(value string) bool {
	return value == "observed" || value == "unavailable"
}

func validHTTPBytesBucket(value string) bool {
	switch value {
	case "empty", "le_1k", "le_16k", "le_256k", "le_1m", "gt_1m", "unavailable":
		return true
	default:
		return false
	}
}

func validHTTPOutcome(value string) bool {
	return validOutcome(value)
}

// build validates every field of the record and fills the fixed slots.
// Invalid core is release-blocking: ok=false suppresses the whole record.
func (e *httpEncoder) build(entry Entry) bool {
	diag := entry.HTTP
	if diag == nil {
		return false
	}
	if entry.Event != "http_complete" {
		return false
	}

	// 0 time
	if !validTimestamp(entry.Time.UTC().Format(time.RFC3339Nano)) {
		return false
	}
	e.slots[0] = httpSlotValue{present: true, str: entry.Time.UTC().Format(time.RFC3339Nano)}
	// 1 level
	if !validLevel(entry.Level) {
		return false
	}
	e.slots[1] = httpSlotValue{present: true, str: entry.Level}
	// 2 event
	e.slots[2] = httpSlotValue{present: true, str: "http_complete"}
	// 3 message
	e.slots[3] = httpSlotValue{present: true, str: httpMessage}
	// 4 service
	e.slots[4] = httpSlotValue{present: true, str: "mcp-bridge"}
	// 5 environment
	e.slots[5] = httpSlotValue{present: true, str: "unspecified"}
	// 6 version
	e.slots[6] = httpSlotValue{present: true, str: "unspecified"}
	// 7 generation
	e.slots[7] = httpSlotValue{present: true, kind: slotInt, i: 0}
	// 8 duration_ms (preserved signed integer representation)
	e.slots[8] = httpSlotValue{present: true, kind: slotInt, i: entry.Duration.Milliseconds()}
	// 9 http_outcome
	if !validHTTPOutcome(diag.HTTPOutcome) {
		return false
	}
	e.slots[9] = httpSlotValue{present: true, str: diag.HTTPOutcome}
	// 10 reason
	if !validLegacyReason(entry.Reason) {
		return false
	}
	e.slots[10] = httpSlotValue{present: true, str: entry.Reason}
	// 11 stage
	if !validLegacyStage(diag.LegacyStage) {
		return false
	}
	e.slots[11] = httpSlotValue{present: true, str: diag.LegacyStage}
	// 12 http_status: observed integer or explicit null when unobserved.
	if diag.HTTPStatus != nil {
		status := *diag.HTTPStatus
		if status < 100 || status > 599 {
			return false
		}
		e.slots[12] = httpSlotValue{present: true, kind: slotInt, i: int64(status)}
	} else {
		e.slots[12] = httpSlotValue{present: true, kind: slotNull}
	}
	// 13 http_method
	if !validMethod(diag.HTTPMethod) {
		return false
	}
	e.slots[13] = httpSlotValue{present: true, str: diag.HTTPMethod}
	// 14 route
	if !validRoute(diag.Route) {
		return false
	}
	e.slots[14] = httpSlotValue{present: true, str: diag.Route}
	// 15 http_bytes_bucket
	if !validHTTPBytesBucket(diag.HTTPBytesBucket) {
		return false
	}
	e.slots[15] = httpSlotValue{present: true, str: diag.HTTPBytesBucket}
	// 16 timing_scope
	e.slots[16] = httpSlotValue{present: true, str: "http"}
	// 17 correlation_state
	if !validCorrelationState(diag.CorrelationState) {
		return false
	}
	e.slots[17] = httpSlotValue{present: true, str: diag.CorrelationState}
	// 18 request_id (conditional on correlation availability)
	if diag.CorrelationState == CorrelationAvailable {
		if !validHexID(diag.RequestID) {
			return false
		}
		e.slots[18] = httpSlotValue{present: true, str: diag.RequestID}
	}
	// 19 diag_schema
	if diag.DiagSchema != "oauth_diag_v1" {
		return false
	}
	e.slots[19] = httpSlotValue{present: true, str: diag.DiagSchema}
	// 20 diag_environment: absent when unavailable.
	if diag.DiagEnvironment != "" && diag.DiagEnvironment != "unavailable" {
		e.slots[20] = httpSlotValue{present: true, str: diag.DiagEnvironment}
	}
	// 21 source_revision: absent when unavailable.
	if diag.SourceRevision != "" && diag.SourceRevision != "unavailable" {
		e.slots[21] = httpSlotValue{present: true, str: diag.SourceRevision}
	}
	// 22 build_state
	if diag.BuildState != "verified" && diag.BuildState != "unavailable" {
		return false
	}
	e.slots[22] = httpSlotValue{present: true, str: diag.BuildState}
	// 23 process_state
	if diag.ProcessState != "available" && diag.ProcessState != "unavailable" {
		return false
	}
	e.slots[23] = httpSlotValue{present: true, str: diag.ProcessState}
	// 24 process_id (conditional on available state)
	if diag.ProcessState == "available" {
		if !validHexID(diag.ProcessID) {
			return false
		}
		e.slots[24] = httpSlotValue{present: true, str: diag.ProcessID}
	}
	// 25 operation_relation
	relation, ok := stringEnum(diag.OperationRelation, httpRelationValues)
	if !ok {
		return false
	}
	e.slots[25] = httpSlotValue{present: true, str: relation}
	// 26 operation_id_state
	idState, ok := stringEnum(diag.OperationIDState, httpOperationIDStateValues)
	if !ok {
		return false
	}
	e.slots[26] = httpSlotValue{present: true, str: idState}
	// 27 path_class
	pathClass, ok := stringEnum(diag.PathClass, httpPathClassValues)
	if !ok {
		return false
	}
	e.slots[27] = httpSlotValue{present: true, str: pathClass}
	// 28 route_outcome
	routeOutcome, ok := stringEnum(diag.RouteOutcome, httpRouteOutcomeValues)
	if !ok {
		return false
	}
	e.slots[28] = httpSlotValue{present: true, str: routeOutcome}
	// 29 status_state
	if !validStatusState(diag.StatusState) {
		return false
	}
	e.slots[29] = httpSlotValue{present: true, str: diag.StatusState}
	// 30 diag_duration_ms: always present, clamped 0..86400000.
	if diag.DiagDurationMS < 0 || diag.DiagDurationMS > maxDiagnosticDurationMS {
		return false
	}
	e.slots[30] = httpSlotValue{present: true, kind: slotInt, i: diag.DiagDurationMS}
	// 31 duration_capped: always present.
	e.slots[31] = httpSlotValue{present: true, kind: slotBool, b: diag.DurationCapped}
	// 32 failure_stage
	stage, ok := stringEnum(diag.FailureStage, httpFailureStageValues)
	if !ok {
		return false
	}
	e.slots[32] = httpSlotValue{present: true, str: stage}
	// 33 failure_reason
	reason, ok := stringEnum(diag.FailureReason, httpFailureReasonValues)
	if !ok {
		return false
	}
	e.slots[33] = httpSlotValue{present: true, str: reason}
	// 34 auth_mode
	authMode, ok := stringEnum(diag.AuthMode, httpAuthModeValues)
	if !ok {
		return false
	}
	e.slots[34] = httpSlotValue{present: true, str: authMode}
	// 35 credential_state
	credential, ok := stringEnum(diag.CredentialState, httpCredentialValues)
	if !ok {
		return false
	}
	e.slots[35] = httpSlotValue{present: true, str: credential}
	// 36 scope_state
	scope, ok := stringEnum(diag.ScopeState, httpScopeStateValues)
	if !ok {
		return false
	}
	e.slots[36] = httpSlotValue{present: true, str: scope}
	// 37 session_header
	header, ok := stringEnum(diag.SessionHeader, httpSessionHeaderValues)
	if !ok {
		return false
	}
	e.slots[37] = httpSlotValue{present: true, str: header}
	// 38 session_state
	sessionState, ok := stringEnum(diag.SessionState, httpSessionStateValues)
	if !ok {
		return false
	}
	e.slots[38] = httpSlotValue{present: true, str: sessionState}
	// 39 binding_relation
	binding, ok := stringEnum(diag.BindingRelation, httpBindingValues)
	if !ok {
		return false
	}
	e.slots[39] = httpSlotValue{present: true, str: binding}
	// 40 session_ref_state
	refState, ok := stringEnum(diag.SessionRefState, httpSessionRefValues)
	if !ok {
		return false
	}
	e.slots[40] = httpSlotValue{present: true, str: refState}
	// 41 challenge_kind
	kind, ok := stringEnum(diag.ChallengeKind, httpChallengeKindValues)
	if !ok {
		return false
	}
	e.slots[41] = httpSlotValue{present: true, str: kind}
	// 42 challenge_target
	target, ok := stringEnum(diag.ChallengeTarget, httpChallengeTargetValues)
	if !ok {
		return false
	}
	e.slots[42] = httpSlotValue{present: true, str: target}
	// 43 metadata_target_registered
	registered, ok := stringEnum(diag.MetadataTargetRegistered, httpMetadataRegisteredValues)
	if !ok {
		return false
	}
	e.slots[43] = httpSlotValue{present: true, str: registered}
	// 44 diagnostic_policy_revision
	if len(diag.DiagnosticPolicyRevision) == 0 || len(diag.DiagnosticPolicyRevision) > 64 {
		return false
	}
	e.slots[44] = httpSlotValue{present: true, str: diag.DiagnosticPolicyRevision}
	// 45 effective_policy_revision_state
	if diag.EffectivePolicyRevisionState != "verified" && diag.EffectivePolicyRevisionState != "unavailable" {
		return false
	}
	e.slots[45] = httpSlotValue{present: true, str: diag.EffectivePolicyRevisionState}
	// 46 effective_policy_revision (conditional)
	if diag.EffectivePolicyRevisionState == "verified" {
		if len(diag.EffectivePolicyRevision) == 0 || len(diag.EffectivePolicyRevision) > 64 {
			return false
		}
		e.slots[46] = httpSlotValue{present: true, str: diag.EffectivePolicyRevision}
	}
	// 47 policy_source
	source, ok := stringEnum(diag.PolicySource, httpPolicySourceValues)
	if !ok {
		return false
	}
	e.slots[47] = httpSlotValue{present: true, str: source}
	// 48 oauth_enabled: three-state bool (nil = unavailable → null)
	e.slots[48] = httpSlotValue{present: true, kind: slotNull}
	if diag.OAuthEnabled != nil {
		e.slots[48] = httpSlotValue{present: true, kind: slotBool, b: *diag.OAuthEnabled}
	}
	// 49 static_configured
	e.slots[49] = httpSlotValue{present: true, kind: slotNull}
	if diag.StaticConfigured != nil {
		e.slots[49] = httpSlotValue{present: true, kind: slotBool, b: *diag.StaticConfigured}
	}
	// 50 scope_required
	e.slots[50] = httpSlotValue{present: true, kind: slotNull}
	if diag.ScopeRequired != nil {
		e.slots[50] = httpSlotValue{present: true, kind: slotBool, b: *diag.ScopeRequired}
	}
	// 51 resource_path_mode
	pathMode, ok := stringEnum(diag.ResourcePathMode, httpPathModeValues)
	if !ok {
		return false
	}
	e.slots[51] = httpSlotValue{present: true, str: pathMode}
	// 52 session_binding
	bindingMode, ok := stringEnum(diag.SessionBinding, httpSessionBindingValues)
	if !ok {
		return false
	}
	e.slots[52] = httpSlotValue{present: true, str: bindingMode}
	// 53 clock_skew_seconds: present only when verified.
	if diag.ClockSkewSeconds != nil {
		e.slots[53] = httpSlotValue{present: true, kind: slotInt, i: int64(*diag.ClockSkewSeconds)}
	}
	// 54 upstream_state
	upstream, ok := stringEnum(diag.UpstreamState, httpUpstreamValues)
	if !ok {
		return false
	}
	e.slots[54] = httpSlotValue{present: true, str: upstream}
	// 55 error_detail_state
	if !validDetailState(diag.ErrorDetailState) {
		return false
	}
	e.slots[55] = httpSlotValue{present: true, str: diag.ErrorDetailState}
	// 56 error_detail (optional conditional core error object)
	if diag.ErrorDetail != nil {
		detail := diag.ErrorDetail
		if !validDetailClass(detail.Class) || len(detail.Code) == 0 || len(detail.Code) > 70 || !validDetailRetry(detail.Retry) {
			return false
		}
		e.slots[56] = httpSlotValue{present: true, kind: slotErrorDetail, obj: detail}
	}
	// 57 operation_id (optional)
	if diag.OperationID != "" {
		if !validHexID(diag.OperationID) {
			return false
		}
		e.slots[57] = httpSlotValue{present: true, str: diag.OperationID}
	}
	// 58 session_ref (optional)
	if diag.SessionRef != "" {
		if !validHexID(diag.SessionRef) {
			return false
		}
		e.slots[58] = httpSlotValue{present: true, str: diag.SessionRef}
	}
	// 59 outcome: the legacy terminal outcome, identical validation to
	// http_outcome.
	if !validOutcome(entry.Outcome) {
		return false
	}
	e.slots[59] = httpSlotValue{present: true, str: entry.Outcome}
	// 60 error: the canonical nested error object. A failed request with
	// unavailable correlation projects correlation_unavailable; otherwise
	// only the closed delivery failure classes carry an error object,
	// mirroring the legacy lane. Success or unavailable-evidence records
	// carry no error object.
	if diag.CorrelationState == CorrelationUnavailable {
		if entry.Outcome == "failure" {
			e.slots[60] = httpSlotValue{present: true, kind: slotErrorClass, str: "correlation_unavailable"}
		}
	} else if class := deliveryErrorClass(entry.Reason); class != "" {
		e.slots[60] = httpSlotValue{present: true, kind: slotErrorClass, str: class}
	}

	core, optional := e.charges()
	if core > httpCoreBudget || optional > httpOptionalBudget || core+optional+2 > httpRecordBudget {
		e.dropForBudget()
		core, optional = e.charges()
		if core > httpCoreBudget || optional > httpOptionalBudget || core+optional+2 > httpRecordBudget {
			return false
		}
	}
	e.encode(core + optional + 2)
	return true
}

// charges measures the encoded record cost: core plus optional bytes and the
// enclosing braces plus newline. It is pure arithmetic over the validated
// slots, shared by the budget decision and the verification tests.
func (e *httpEncoder) charges() (core, optional int) {
	first := true
	for index, slot := range e.slots {
		if !slot.present {
			continue
		}
		// Exactly the bytes encode emits for this slot: opening key quote,
		// key, closing key quote and colon; string values additionally
		// carry their opening and closing value quotes.
		charge := 1 + len(slotKeys[index]) + len(`":`)
		if slot.kind == slotString {
			charge += 2
		}
		charge += slot.encodedLength()
		if !first {
			charge++ // comma before this slot, exactly as emitted
		}
		first = false
		if index >= 56 && index <= 58 {
			optional += charge
		} else {
			core += charge
		}
	}
	return core, optional
}

// encodedLength reports the exact encoded byte cost of one slot value.
func (s *httpSlotValue) encodedLength() int {
	switch s.kind {
	case slotInt:
		return len(strconv.FormatInt(s.i, 10))
	case slotBool:
		return len(strconv.FormatBool(s.b))
	case slotNull:
		return len("null")
	case slotErrorDetail:
		return len(`{"class":"`) + len(s.obj.Class) + len(`","code":"`) + len(s.obj.Code) +
			len(`","retry":"`) + len(s.obj.Retry) + len(`"}`)
	case slotErrorClass:
		return len(`{"class":"`) + len(s.str) + len(`"}`)
	default:
		return len(s.str)
	}
}

// dropForBudget omits optionals in the approved order and flips their state
// slots to omitted_for_budget. Core is never dropped.
func (e *httpEncoder) dropForBudget() {
	drop := func(slotIndex int, stateIndex int, stateValue string) {
		if e.slots[slotIndex].present {
			e.slots[slotIndex].present = false
			e.slots[stateIndex] = httpSlotValue{present: true, str: stateValue}
		}
	}
	drop(56, 55, "omitted_for_budget")
	drop(57, 26, "omitted_for_budget")
	drop(58, 40, "omitted_for_budget")
}

// encode writes the validated record once into the bounded buffer. The exact
// measured length is known upfront; the buffer is preallocated and never
// regrown.
func (e *httpEncoder) encode(length int) {
	e.buf.Grow(length)
	e.buf.WriteByte('{')
	first := true
	for index, slot := range e.slots {
		if !slot.present {
			continue
		}
		if !first {
			e.buf.WriteByte(',')
		}
		first = false
		e.buf.WriteByte('"')
		e.buf.WriteString(slotKeys[index])
		e.buf.WriteString(`":`)
		switch slot.kind {
		case slotInt:
			e.buf.WriteString(strconv.FormatInt(slot.i, 10))
		case slotBool:
			e.buf.WriteString(strconv.FormatBool(slot.b))
		case slotNull:
			e.buf.WriteString("null")
		case slotErrorDetail:
			e.buf.WriteString(`{"class":"`)
			e.buf.WriteString(slot.obj.Class)
			e.buf.WriteString(`","code":"`)
			e.buf.WriteString(slot.obj.Code)
			e.buf.WriteString(`","retry":"`)
			e.buf.WriteString(slot.obj.Retry)
			e.buf.WriteString(`"}`)
		case slotErrorClass:
			e.buf.WriteString(`{"class":"`)
			e.buf.WriteString(slot.str)
			e.buf.WriteString(`"}`)
		default:
			e.buf.WriteByte('"')
			e.buf.WriteString(slot.str)
			e.buf.WriteByte('"')
		}
	}
	e.buf.WriteByte('}')
	e.buf.WriteByte('\n')
}
