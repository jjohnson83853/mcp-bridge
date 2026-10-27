package observe

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

// HTTPDiagnostics owns terminal HTTP completion diagnostics: one private
// server-created owner per request holds the bounded state, the native
// writer and the sole response observation.
type HTTPDiagnostics struct {
	logger  *Logger
	now     func() time.Time
	newID   func() string
	process HTTPProcessContext
}

// HTTPDiagnosticsOptions configures the diagnostics owner. Production
// defaults are time.Now, cryptographic generation and a shared process
// context; the fields exist for trusted startup wiring and tests only.
type HTTPDiagnosticsOptions struct {
	Now     func() time.Time
	NewID   func() string
	Process HTTPProcessContext
}

// NewHTTPDiagnostics builds the completion owner. A nil Process opts into the
// one shared process-scoped incarnation, not an incarnation per constructor.
func NewHTTPDiagnostics(logger *Logger, opts HTTPDiagnosticsOptions) *HTTPDiagnostics {
	diagnostics := &HTTPDiagnostics{
		logger:  logger,
		now:     opts.Now,
		newID:   opts.NewID,
		process: opts.Process,
	}
	if diagnostics.now == nil {
		diagnostics.now = time.Now
	}
	if diagnostics.newID == nil {
		diagnostics.newID = func() string { return CorrelationID("") }
	}
	if diagnostics.process.State == "" {
		diagnostics.process = sharedProcessContext()
	}
	diagnostics.process = resolveProcessContext(diagnostics.process)
	return diagnostics
}

var (
	processOnce     sync.Once
	sharedProcess   HTTPProcessContext
)

// sharedProcessContext draws the independent 128-bit process incarnation once
// per process. One entropy failure leaves it explicitly unavailable with no
// retry or derived fallback.
func sharedProcessContext() HTTPProcessContext {
	processOnce.Do(func() {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			sharedProcess = HTTPProcessContext{State: "unavailable"}
			return
		}
		sharedProcess = HTTPProcessContext{State: "available", ID: hex.EncodeToString(random[:])}
	})
	return sharedProcess
}

// resolveProcessContext validates an explicit or shared process context.
func resolveProcessContext(process HTTPProcessContext) HTTPProcessContext {
	if process.State == "available" && validHexID(process.ID) {
		return process
	}
	return HTTPProcessContext{State: "unavailable"}
}

// Wrap places the single completion owner around the whole public listener
// composition, before routing and OAuth. Existing mux registrations are
// unchanged; the outer claim is the only finalizer for the request.
func (d *HTTPDiagnostics) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer, request, lease := d.Claim(w, r)
		if lease.OwnsCompletion() {
			defer lease.Finish()
		}
		next.ServeHTTP(writer, request)
	})
}

// HTTPOwnership is the small by-value claimant lease. Only a claimant lease
// finalizes; nonclaimant leases of nested or recovered owners cannot finish.
type HTTPOwnership struct {
	owner    *httpOwner
	claimant bool
}

// OwnsCompletion reports whether this lease owns the terminal finalization.
func (l HTTPOwnership) OwnsCompletion() bool {
	return l.claimant && l.owner != nil
}

// Finish snapshots the actual observed state and attempts the one terminal
// record. It runs identically on normal return and panic unwinding: no
// recover, no synthetic 500, no panic inspection.
func (l HTTPOwnership) Finish() {
	if l.claimant && l.owner != nil {
		l.owner.finish()
	}
}

// Claim detects an existing owner (typed context marker first, then private
// writer carrier) and otherwise allocates one facade with its embedded owner,
// generates the single owned identity, installs the diagnostic response
// header and returns the claimant lease. Caller-supplied diagnostic/internal
// headers are scrubbed; they can never supply identity.
func (d *HTTPDiagnostics) Claim(w http.ResponseWriter, r *http.Request) (http.ResponseWriter, *http.Request, HTTPOwnership) {
	if d == nil {
		return w, r, HTTPOwnership{}
	}
	if owner := ownerFromContext(r.Context()); owner != nil {
		// Existing owner: contribution only. The incoming writer chain is
		// preserved untouched — no new wrapper, identity, header or finalizer.
		return w, r, HTTPOwnership{owner: owner}
	}
	if owner := ownerFromCarrier(w); owner != nil {
		// Carrier recovery: reinstall the context marker when it was lost.
		if ownerFromContext(r.Context()) == nil {
			r = r.WithContext(withOwnerContext(r.Context(), owner))
		}
		return w, r, HTTPOwnership{owner: owner}
	}

	// The clock starts before owned generation so the measured duration
	// includes the generator's own work; the initial panic window before the
	// caller can install its defer remains unobserved, exactly as before.
	started := d.now()
	requestID := d.newID()

	mask, delegates := probeCapabilities(w)
	sessionHeader := httpSessionHeaderAbsent
	if r.Header.Get(sessionIDHeaderName) != "" {
		sessionHeader = httpSessionHeaderPresent
	}
	writer, owner := newHTTPFacadeWriter(httpOwnerInit{
		diagnostics:   d,
		writer:        w,
		delegates:     delegates,
		started:       started,
		requestID:     requestID,
		method:        r.Method,
		route:         legacyRoute(r.URL.Path),
		pathClass:     pathClass(r.URL.Path),
		sessionHeader: sessionHeader,
	}, mask)

	// Scrub caller diagnostic/internal values and install the owned response
	// value before routing, authentication or any write. The public
	// X-Request-ID policy is independent and untouched here.
	r.Header.Del("Mcp-Diagnostic-Id")
	r.Header.Del("X-Mcp-Bridge-Internal-Request-Id")
	r.Header.Del("X-Internal-Request-Id")
	if validHexID(requestID) {
		w.Header().Set("Mcp-Diagnostic-Id", requestID)
	}
	// An entropy or format failure records the unavailable state honestly:
	// no substitute identity, no retry, no caller/time/counter/hash fallback.

	r = r.WithContext(withOwnerContext(r.Context(), owner))
	return writer, r, HTTPOwnership{owner: owner, claimant: true}
}

// Header name constants shared with the inner edge and middleware.
const (
	sessionIDHeaderName        = "Mcp-Session-Id"
	httpSessionHeaderAbsent    = "absent"
	httpSessionHeaderPresent   = "present"
	internalRequestIDHeader    = "X-Mcp-Bridge-Internal-Request-Id"
)

// HTTPRequestID returns the owner's generated identity for trusted downstream
// reuse, or "" when no owner is present or correlation is unavailable.
func HTTPRequestID(ctx context.Context) string {
	owner := ownerFromContext(ctx)
	if owner == nil || owner.correlationState != CorrelationAvailable {
		return ""
	}
	return owner.requestID
}

// HTTPResponseObservation returns the read-only terminal observation: observed
// status (0 when unobserved), successful byte count and the closed write
// failure class. The retained error stays internal.
func HTTPResponseObservation(ctx context.Context) HTTPObservation {
	owner := ownerFromContext(ctx)
	if owner == nil {
		return HTTPObservation{WriteFailure: HTTPWriteNone}
	}
	return HTTPObservation{
		Status:       owner.status,
		Bytes:        owner.bytes,
		WriteFailure: owner.writeFailure,
	}
}

// NativeHTTPWriter returns the owner's original pre-observer writer so
// request-size limiting keeps the HTTP server's native close-on-limit
// signaling. Without an owner the supplied writer is returned unchanged.
func NativeHTTPWriter(ctx context.Context, w http.ResponseWriter) http.ResponseWriter {
	if owner := ownerFromContext(ctx); owner != nil {
		return owner.writer
	}
	return w
}

// ContributeHTTP applies closed typed decision evidence to the request's
// owner. Contributions without an owner marker are no-ops.
func ContributeHTTP(ctx context.Context, decision HTTPDecision) {
	if owner := ownerFromContext(ctx); owner != nil {
		owner.applyDecision(decision)
	}
}

// ContributeSessionEvidence applies owned session evaluation evidence.
func ContributeSessionEvidence(ctx context.Context, evidence HTTPSessionEvidence) {
	if owner := ownerFromContext(ctx); owner != nil {
		owner.applySessionEvidence(evidence)
	}
}

// ContributeOperationEvidence applies actual downstream operation linkage and
// outcome evidence.
func ContributeOperationEvidence(ctx context.Context, evidence HTTPOperationEvidence) {
	if owner := ownerFromContext(ctx); owner != nil {
		owner.applyOperationEvidence(evidence)
	}
}

// ContributeLegacyEvidence applies the summarised legacy classification the
// inner edge computed at its boundaries.
func ContributeLegacyEvidence(ctx context.Context, evidence HTTPLegacyEvidence) {
	if owner := ownerFromContext(ctx); owner != nil {
		owner.applyLegacyEvidence(evidence)
	}
}

// legacyEntry is the internal legacy core of a completion record before it
// joins the typed snapshot.
type legacyEntry struct {
	Time      time.Time
	Level     string
	RequestID string
	Operation string
	Duration  time.Duration
	Outcome   string
	Reason    string
}

// observeEntry joins the legacy core and the typed snapshot into one record.
func observeEntry(entry legacyEntry, diagnostic *HTTPDiagnostic) Entry {
	return Entry{
		Time:      entry.Time,
		Level:     entry.Level,
		Event:     "http_complete",
		RequestID: entry.RequestID,
		Operation: entry.Operation,
		Duration:  entry.Duration,
		Outcome:   entry.Outcome,
		Reason:    entry.Reason,
		HTTP:      diagnostic,
	}
}

// finiteEnum returns the value when it is a member of the closed set.
func finiteEnum(values []string, candidate string) (string, bool) {
	for _, value := range values {
		if candidate == value {
			return candidate, true
		}
	}
	return "", false
}

// applyDecision records closed decision evidence. The first decisive
// rejection is immutable: later evidence cannot overwrite it. Route outcome is
// first-wins so the outer OAuth evaluation cannot be relabeled downstream.
func (o *httpOwner) applyDecision(d HTTPDecision) {
	if !o.failureSet {
		if stage, ok := finiteEnum(httpFailureStageValues, d.FailureStage); ok && stage != "none" {
			if reason, ok := finiteEnum(httpFailureReasonValues, d.FailureReason); ok {
				o.failureStage = stage
				o.failureReason = reason
				o.failureSet = true
			}
		}
	}
	if value, ok := finiteEnum(httpAuthModeValues, d.AuthMode); ok {
		o.authMode = value
	}
	if value, ok := finiteEnum(httpCredentialValues, d.CredentialState); ok {
		o.credentialState = value
	}
	if value, ok := finiteEnum(httpScopeStateValues, d.ScopeState); ok {
		o.scopeState = value
	}
	if value, ok := finiteEnum(httpSessionHeaderValues, d.SessionHeader); ok {
		o.sessionHeader = value
	}
	if value, ok := finiteEnum(httpSessionStateValues, d.SessionState); ok {
		o.sessionState = value
	}
	if value, ok := finiteEnum(httpBindingValues, d.BindingRelation); ok {
		o.bindingRelation = value
	}
	if value, ok := finiteEnum(httpChallengeKindValues, d.ChallengeKind); ok {
		o.challengeKind = value
	}
	if value, ok := finiteEnum(httpChallengeTargetValues, d.ChallengeTarget); ok {
		o.challengeTarget = value
	}
	if value, ok := finiteEnum(httpMetadataRegisteredValues, d.MetadataTargetRegistered); ok {
		o.metadataRegistered = value
	}
	if value, ok := finiteEnum(httpRouteOutcomeValues, d.RouteOutcome); ok && !o.routeOutcomeSet {
		o.routeOutcome = value
		o.routeOutcomeSet = true
	}
}

func (o *httpOwner) applySessionEvidence(e HTTPSessionEvidence) {
	if value, ok := finiteEnum(httpSessionRefValues, e.RefState); ok {
		o.sessionRefState = value
	}
	if validHexID(e.Ref) {
		o.sessionRef = e.Ref
	}
	if value, ok := finiteEnum(httpSessionStateValues, e.State); ok {
		o.sessionState = value
	}
	if value, ok := finiteEnum(httpBindingValues, e.Binding); ok {
		o.bindingRelation = value
	}
}

func (o *httpOwner) applyOperationEvidence(e HTTPOperationEvidence) {
	if value, ok := finiteEnum(httpRelationValues, e.Relation); ok {
		o.operationRelation = value
	}
	if value, ok := finiteEnum(httpOperationIDStateValues, e.IDState); ok {
		o.operationIDState = value
	}
	if validHexID(e.ID) {
		o.operationID = e.ID
	}
	if value, ok := finiteEnum(httpUpstreamValues, e.Upstream); ok {
		o.upstreamState = value
	}
}

func (o *httpOwner) applyLegacyEvidence(e HTTPLegacyEvidence) {
	if value, ok := finiteEnum(legacyStageVocabulary, e.Stage); ok {
		o.legacyStage = value
	}
	if _, ok := finiteEnum(reasonVocabulary, e.Reason); ok {
		o.legacyReason = e.Reason
	}
	if _, ok := finiteEnum(operationVocabulary, e.Operation); ok {
		o.legacyOperation = e.Operation
	}
}

// legacyRoute mirrors the existing protected-route comparison for the legacy
// route field.
func legacyRoute(path string) string {
	if path == "/mcp" {
		return "mcp"
	}
	return "not_found"
}

// pathClass classifies the request path against the exact allowlist only;
// raw paths and queries are never emitted.
func pathClass(path string) string {
	switch path {
	case "/mcp":
		return "mcp"
	case "/.well-known/oauth-protected-resource":
		return "metadata_root"
	case "/livez", "/healthz", "/readyz":
		return "health"
	default:
		return "other"
	}
}

// legacyStageVocabulary mirrors the legacy stage values the existing inner
// edge already emits.
var legacyStageVocabulary = []string{
	"none", "http_route", "host_admission", "origin_admission", "client_auth",
	"session_admission", "request_limits", "mcp_decode", "mcp_dispatch",
	"tool_admission", "backend_dispatch", "backend_response", "response_delivery",
	"unknown",
}

var operationVocabulary = []string{
	"unknown", "request", "discover", "initialize", "tools_list", "tools_call",
	"call_tool", "search", "describe", "health", "describe_tool",
	"prompts_list", "prompts_get",
}

// maxDiagnosticDurationMS bounds the semantic duration.
const maxDiagnosticDurationMS = 86400000

// finish snapshots the actual observed state and attempts the one terminal
// record. The finished guard prevents any double sink attempt.
func (o *httpOwner) finish() {
	if o.finished || o.diagnostics == nil {
		return
	}
	o.finished = true
	entry, diagnostic := o.buildSnapshot()
	if o.diagnostics.logger == nil {
		return
	}
	o.diagnostics.logger.Log(observeEntry(entry, diagnostic))
}

// buildSnapshot computes the legacy entry values and the typed snapshot from
// the owner's actual state. It is read-only.
func (o *httpOwner) buildSnapshot() (legacyEntry, *HTTPDiagnostic) {
	now := o.diagnostics.now()
	elapsed := now.Sub(o.started)
	durationMS := elapsed.Milliseconds()

	// The bounded semantic duration clamps negative legacy deltas to zero and
	// caps beyond one day; the legacy field keeps its signed representation.
	diagMS := durationMS
	capped := false
	if diagMS < 0 {
		diagMS, capped = 0, true
	} else if diagMS > maxDiagnosticDurationMS {
		diagMS, capped = maxDiagnosticDurationMS, true
	}

	// Delivery classification, byte bucket and outcome: the same observation
	// the inner classifiers read, computed once from the owner state.
	httpOutcome := "success"
	if o.writeErr != nil {
		httpOutcome = "incomplete"
	} else if o.status >= http.StatusBadRequest {
		httpOutcome = "failure"
	}
	if o.status == 0 {
		httpOutcome = "unknown"
	}
	outcome := httpOutcome
	if outcome == "incomplete" {
		outcome = "failure"
	}

	bucket := "empty"
	hijacked := o.hijacked
	switch {
	case hijacked:
		bucket = "unavailable"
	case o.bytes == 0:
		bucket = "empty"
	case o.bytes <= 1024:
		bucket = "le_1k"
	case o.bytes <= 16384:
		bucket = "le_16k"
	case o.bytes <= 262144:
		bucket = "le_256k"
	case o.bytes <= 1048576:
		bucket = "le_1m"
	default:
		bucket = "gt_1m"
	}

	statusState := StatusUnobserved
	var status *int
	if o.statusObserved && o.status >= 100 && o.status <= 599 {
		observed := o.status
		statusState = StatusObserved
		status = &observed
	}

	legacyStage := orDefault(o.legacyStage, "unknown")
	legacyReason := orDefault(o.legacyReason, "unknown")
	operation := orDefault(o.legacyOperation, "unknown")
	level := severityLevel(legacyStage, legacyReason, o.status, o.upstreamState)

	diagnostic := &HTTPDiagnostic{
		LegacyStage:     legacyStage,
		HTTPStatus:      status,
		HTTPMethod:      httpMethodValue(o.method),
		Route:           o.route,
		HTTPOutcome:     httpOutcome,
		HTTPBytesBucket: bucket,
		StatusState:     statusState,
		CorrelationState: orDefault(o.correlationState, CorrelationUnavailable),
		DiagSchema:      "oauth_diag_v1",
		BuildState:      "unavailable",
		ProcessState:    o.diagnostics.process.State,
		ProcessID:       o.diagnostics.process.ID,
		OperationRelation: orDefault(o.operationRelation, "none"),
		OperationIDState:  orDefault(o.operationIDState, "not_applicable"),
		PathClass:       orDefault(o.pathClass, "other"),
		RouteOutcome:    orDefault(o.routeOutcome, "unavailable"),
		DiagDurationMS:  diagMS,
		DurationCapped:  capped,
		FailureStage:    orDefault(o.failureStage, "none"),
		FailureReason:   orDefault(o.failureReason, "none"),
		AuthMode:        orDefault(o.authMode, "unavailable"),
		CredentialState: orDefault(o.credentialState, "not_evaluated"),
		ScopeState:      orDefault(o.scopeState, "not_evaluated"),
		SessionHeader:   orDefault(o.sessionHeader, httpSessionHeaderAbsent),
		SessionState:    orDefault(o.sessionState, "not_evaluated"),
		BindingRelation: orDefault(o.bindingRelation, "not_evaluated"),
		SessionRefState: orDefault(o.sessionRefState, "not_applicable"),
		ChallengeKind:   orDefault(o.challengeKind, "none"),
		ChallengeTarget: orDefault(o.challengeTarget, "none"),
		MetadataTargetRegistered: orDefault(o.metadataRegistered, "not_applicable"),
		DiagnosticPolicyRevision: "oauth-diag-policy-v1",
		EffectivePolicyRevisionState: "unavailable",
		PolicySource:    "unavailable",
		ResourcePathMode: "unavailable",
		SessionBinding:  "unavailable",
		UpstreamState:   orDefault(o.upstreamState, "not_attempted"),
		ErrorDetailState: orDefault(o.errorDetailState, "not_applicable"),
		ErrorDetail:     o.errorDetail,
	}
	if o.correlationState == CorrelationAvailable {
		diagnostic.RequestID = o.requestID
	}
	diagnostic.OperationID = o.operationID
	diagnostic.SessionRef = o.sessionRef

	entry := legacyEntry{
		Time:      now,
		Level:     level,
		RequestID: o.requestID,
		Operation: operation,
		Duration:  elapsed,
		Outcome:   outcome,
		Reason:    legacyReason,
	}
	return entry, diagnostic
}

// orDefault resolves an unset contribution to its truthful default state.
func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// httpMethodValue collapses raw methods onto the finite semantic set.
func httpMethodValue(method string) string {
	switch method {
	case "GET", "POST", "DELETE", "HEAD", "OPTIONS", "PUT", "PATCH":
		return method
	default:
		return "OTHER"
	}
}

// severityLevel computes the existing owned-cause severity first, then only
// promotes info to warn for actual tool errors under an observed 200 and for
// unavailable responses. Existing warn/error is never downgraded.
func severityLevel(legacyStage, legacyReason string, status int, upstream string) string {
	level := "info"
	switch legacyStage {
	case "http_route", "host_admission", "origin_admission", "client_auth",
		"session_admission", "request_limits":
		level = "warn"
	case "response_delivery":
		if legacyReason == "client_disconnected" {
			level = "warn"
		} else {
			level = "error"
		}
	default:
		if status >= http.StatusBadRequest {
			if status >= 500 {
				level = "error"
			} else {
				level = "warn"
			}
		}
	}
	if level == "info" {
		if upstream == "tool_error" && status == http.StatusOK {
			level = "warn"
		}
		if status < 100 {
			// Unavailable response evidence promotes info to warn.
			level = "warn"
		}
	}
	return level
}
