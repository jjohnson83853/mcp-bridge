package observe

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
)

// HTTPDiagnostics owns terminal HTTP completion diagnostics: one private
// server-created owner per request holds the bounded state, the native
// writer and the sole response observation.
type HTTPDiagnostics struct {
	logger     *Logger
	now        func() time.Time
	newID      func() string
	process    HTTPProcessContext
	provenance HTTPProvenance
}

// HTTPDiagnosticsOptions configures the diagnostics owner. Production
// defaults are time.Now, cryptographic generation and a shared process
// context; the fields exist for trusted startup wiring and tests only.
type HTTPDiagnosticsOptions struct {
	Now        func() time.Time
	NewID      func() string
	Process    HTTPProcessContext
	Provenance HTTPProvenance
}

// NewHTTPDiagnostics builds the completion owner. A nil Process opts into the
// one shared process-scoped incarnation, not an incarnation per constructor.
func NewHTTPDiagnostics(logger *Logger, opts HTTPDiagnosticsOptions) *HTTPDiagnostics {
	diagnostics := &HTTPDiagnostics{
		logger:     logger,
		now:        opts.Now,
		newID:      opts.NewID,
		process:    opts.Process,
		provenance: resolveProvenance(opts.Provenance),
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
	processOnce   sync.Once
	sharedProcess HTTPProcessContext
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

// resolveProvenance validates the trusted startup provenance. Unverifiable
// values fall back to their explicit unavailable/absent states; nothing is
// derived, rewritten or substituted. A source revision counts as verified
// only when it is 40–64 lowercase hexadecimal characters; mutable tags,
// arbitrary hash-shaped values and dirty (unpushed) builds stay unavailable.
func resolveProvenance(p HTTPProvenance) HTTPProvenance {
	resolved := HTTPProvenance{
		SourceRevision:               "unavailable",
		BuildState:                   "unavailable",
		PolicySource:                 "unavailable",
		EffectivePolicyRevisionState: "unavailable",
		ResourcePathMode:             "unavailable",
	}
	if value, ok := safeEnvironmentValue(p.Environment); ok {
		resolved.Environment = value
	}
	if validRevisionShape(p.SourceRevision) {
		resolved.SourceRevision = p.SourceRevision
		resolved.BuildState = "verified"
	}
	if value, ok := finiteEnum(httpPolicySourceValues, p.PolicySource); ok {
		resolved.PolicySource = value
	}
	resolved.OAuthEnabled = p.OAuthEnabled
	resolved.StaticConfigured = p.StaticConfigured
	resolved.ScopeRequired = p.ScopeRequired
	if p.ClockSkewSeconds != nil && *p.ClockSkewSeconds == verifiedClockSkewSeconds {
		skew := *p.ClockSkewSeconds
		resolved.ClockSkewSeconds = &skew
	}
	if p.EffectivePolicyRevisionState == "verified" && validRevisionShape(p.EffectivePolicyRevision) {
		resolved.EffectivePolicyRevisionState = "verified"
		resolved.EffectivePolicyRevision = p.EffectivePolicyRevision
	}
	if value, ok := finiteEnum(httpPathModeValues, p.ResourcePathMode); ok {
		resolved.ResourcePathMode = value
	}
	return resolved
}

// verifiedClockSkewSeconds is the middleware's fixed validation tolerance,
// recorded only when the trusted wiring attests exactly this value.
const verifiedClockSkewSeconds = 60

// validRevisionShape reports whether the value is 40–64 lowercase hexadecimal
// characters: the accepted shape of a verified source revision and of an
// attested nonsecret policy-content artifact identifier.
func validRevisionShape(value string) bool {
	if len(value) < 40 || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

// ValidSourceRevision reports whether the value is the only accepted verified
// immutable source revision shape: 40–64 lowercase hexadecimal characters.
func ValidSourceRevision(value string) bool {
	return validRevisionShape(value)
}

// safeEnvironmentValue bounds the environment alias alphabet: 1–32 lowercase
// letters, digits, underscore and hyphen, or the explicit unavailable marker.
func safeEnvironmentValue(value string) (string, bool) {
	if value == "unavailable" {
		return value, true
	}
	if len(value) == 0 || len(value) > 32 {
		return "", false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return "", false
		}
	}
	return value, true
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
		clientFamily:  classifyClientFamily(r.UserAgent()),
	}, mask)

	// Scrub caller diagnostic/internal values and install the owned response
	// value before routing, authentication or any write. The public
	// X-Request-ID policy is independent and untouched here.
	r.Header.Del("Mcp-Diagnostic-Id")
	r.Header.Del("X-Mcp-Bridge-Internal-Request-Id")
	r.Header.Del("X-Internal-Request-Id")
	if validHexID(requestID) {
		w.Header().Set("Mcp-Diagnostic-Id", requestID)
		registerOwner(requestID, owner)
	}
	// An entropy or format failure records the unavailable state honestly:
	// no substitute identity, no retry, no caller/time/counter/hash fallback.

	r = r.WithContext(withOwnerContext(r.Context(), owner))
	return writer, r, HTTPOwnership{owner: owner, claimant: true}
}

// Header name constants shared with the inner edge and middleware.
const (
	sessionIDHeaderName      = "Mcp-Session-Id"
	httpSessionHeaderAbsent  = "absent"
	httpSessionHeaderPresent = "present"
	internalRequestIDHeader  = "X-Mcp-Bridge-Internal-Request-Id"
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

// registerOwner records one in-flight owner under its owned request identity.
func registerOwner(requestID string, owner *httpOwner) {
	ownerRegistryMu.Lock()
	if ownerRegistry == nil {
		ownerRegistry = make(map[string]*httpOwner)
	}
	ownerRegistry[requestID] = owner
	ownerRegistryMu.Unlock()
}

// removeOwner drops the in-flight entry exactly at finalization; the registry
// never outlives the request.
func removeOwner(requestID string) {
	ownerRegistryMu.Lock()
	delete(ownerRegistry, requestID)
	ownerRegistryMu.Unlock()
}

// ownerForRequestID returns the in-flight owner that generated this exact
// server-owned request identity, or nil. A malformed value can never match:
// caller-supplied or JSON-RPC identities are not eligible linkage keys.
func ownerForRequestID(requestID string) *httpOwner {
	if !validHexID(requestID) {
		return nil
	}
	ownerRegistryMu.Lock()
	defer ownerRegistryMu.Unlock()
	return ownerRegistry[requestID]
}

// ContributeOperationEvidenceByID links actual downstream operation evidence
// to the terminal record of the request that minted this owned request
// identity. The layer that decodes MCP operations runs inside the pinned SDK
// session whose context is detached from the per-request context, so the
// in-flight owner lookup is the only truthful join. It is a no-op without an
// in-flight owner, after finalization, or for any non-owned identity.
func ContributeOperationEvidenceByID(requestID string, evidence HTTPOperationEvidence) {
	owner := ownerForRequestID(requestID)
	if owner == nil {
		return
	}
	// Concurrent JSON-RPC batch operations arrive on separate goroutines; the
	// finished check and the owner-state mutation are one atomic step under
	// the same lock finish() takes, so a late contribution can never race a
	// finalizing snapshot and no update is ever lost.
	ownerStateMu.Lock()
	defer ownerStateMu.Unlock()
	if !owner.finished {
		owner.applyOperationEvidence(evidence)
	}
}

// ContributeExposedTools records the number of exposed tools for a request.
// The request context is preferred; requestID is the detached-owner fallback.
func ContributeExposedTools(ctx context.Context, requestID string, count int) {
	owner := ownerFromContext(ctx)
	if owner == nil {
		owner = ownerForRequestID(requestID)
	}
	if owner == nil {
		return
	}
	ownerStateMu.Lock()
	defer ownerStateMu.Unlock()
	if !owner.finished {
		owner.exposedTools = &count
	}
}

// ownerStateMu serializes owner evidence mutation against finalization for
// the detached request-identity path. The registry mutex covers only the
// in-flight map; owner field state needs its own ordering point.
var ownerStateMu sync.Mutex

var (
	ownerRegistryMu sync.Mutex
	ownerRegistry   map[string]*httpOwner
)

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
	// The established binding comparator's mechanism label: once an actual
	// match/mismatch comparison ran, the fixed mechanism name is established.
	// It is a label, never a token digest.
	if e.Binding == "match" || e.Binding == "mismatch" {
		o.sessionBinding = "bearer_sha256"
	}
}

// applyOperationEvidence applies actual downstream operation linkage and
// outcome evidence. The relation is derived from distinct server-owned
// operation identities the owner actually observed, so it stays truthful:
// none until evidence arrives, single on one eligible ID, multiple on
// several. Unsafe ID values count as observed operations but are never
// emitted or rewritten into supported certainty.
func (o *httpOwner) applyOperationEvidence(e HTTPOperationEvidence) {
	eligible := validHexID(e.ID)
	switch {
	case eligible:
		if o.operationCount == 0 {
			o.operationFirstID = e.ID
			o.operationCount = 1
		} else if e.ID != o.operationFirstID {
			o.operationCount++
		}
	case e.ID != "":
		o.unsafeOperationID = true
	}
	if value, ok := finiteEnum(httpUpstreamValues, e.Upstream); ok && value != "not_attempted" {
		if o.upstreamFirst == "" {
			o.upstreamFirst = value
		} else if value != o.upstreamFirst {
			o.upstreamDiverged = true
		}
	}
	o.reconcileOperationState(e)
}

// reconcileOperationState derives the emitted relation/ID state from the
// observed operation identities, keeping an explicitly contributed finite
// relation as evidence when no ID observation contradicts it.
func (o *httpOwner) reconcileOperationState(e HTTPOperationEvidence) {
	switch {
	case o.operationCount > 1 || (o.unsafeOperationID && o.operationCount > 0):
		o.operationRelation = "multiple"
		o.operationIDState = "not_applicable"
		o.operationID = ""
	case o.operationCount == 1:
		o.operationRelation = "single"
		o.operationIDState = "available"
		o.operationID = o.operationFirstID
	case o.unsafeOperationID:
		// Only unsafe identities were observed: the operation linkage stays
		// single if the contribution says so, but the ID is unavailable.
		if value, ok := finiteEnum(httpRelationValues, e.Relation); ok {
			o.operationRelation = value
		}
		o.operationIDState = "unavailable"
		o.operationID = ""
	default:
		if value, ok := finiteEnum(httpRelationValues, e.Relation); ok {
			o.operationRelation = value
		}
		if value, ok := finiteEnum(httpOperationIDStateValues, e.IDState); ok {
			o.operationIDState = value
		}
	}
	if o.upstreamDiverged {
		o.upstreamState = "multiple"
	} else if o.upstreamFirst != "" {
		o.upstreamState = o.upstreamFirst
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

func classifyClientFamily(ua string) string {
	lower := strings.ToLower(ua)
	switch {
	case strings.Contains(lower, "openai"):
		return "openai"
	case strings.Contains(lower, "claude"), strings.Contains(lower, "anthropic"):
		return "claude"
	case strings.Contains(lower, "cursor"):
		return "cursor"
	case strings.Contains(ua, "omp/"):
		return "omp"
	case strings.Contains(lower, "mozilla"), strings.Contains(lower, "chrome"), strings.Contains(lower, "safari"):
		return "browser"
	case ua != "":
		return "other"
	default:
		return "absent"
	}
}

var operationVocabulary = []string{
	"unknown", "request", "discover", "initialize", "initialized", "tools_list",
	"tools_call", "call_tool", "resources_list", "resources_read", "ping",
	"sse_stream", "search", "describe", "health", "describe_tool",
	"prompts_list", "prompts_get",
}

// maxDiagnosticDurationMS bounds the semantic duration.
const maxDiagnosticDurationMS = 86400000

// finish snapshots the actual observed state and attempts the one terminal
// record. The finished guard prevents any double sink attempt.
func (o *httpOwner) finish() {
	// The finished flag and the final mutation window share ownerStateMu with
	// the detached evidence path: once the flag is set, no late contribution
	// can interleave between it and the snapshot read.
	ownerStateMu.Lock()
	if o.finished || o.diagnostics == nil {
		ownerStateMu.Unlock()
		return
	}
	o.finished = true
	ownerStateMu.Unlock()
	// The registry entry dies with the completed request: later contributions
	// can never mutate a terminal record.
	removeOwner(o.requestID)
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
		LegacyStage:                  legacyStage,
		HTTPStatus:                   status,
		HTTPMethod:                   httpMethodValue(o.method),
		Route:                        o.route,
		HTTPOutcome:                  httpOutcome,
		HTTPBytesBucket:              bucket,
		Operation:                    operation,
		ClientFamily:                 o.clientFamily,
		HTTPBytes:                    o.bytes,
		StatusState:                  statusState,
		CorrelationState:             orDefault(o.correlationState, CorrelationUnavailable),
		DiagSchema:                   "oauth_diag_v1",
		DiagEnvironment:              o.diagnostics.provenance.Environment,
		SourceRevision:               o.diagnostics.provenance.SourceRevision,
		BuildState:                   o.diagnostics.provenance.BuildState,
		ProcessState:                 o.diagnostics.process.State,
		ProcessID:                    o.diagnostics.process.ID,
		OperationRelation:            orDefault(o.operationRelation, "none"),
		OperationIDState:             orDefault(o.operationIDState, "not_applicable"),
		PathClass:                    orDefault(o.pathClass, "other"),
		RouteOutcome:                 orDefault(o.routeOutcome, "unavailable"),
		DiagDurationMS:               diagMS,
		DurationCapped:               capped,
		FailureStage:                 orDefault(o.failureStage, "none"),
		FailureReason:                orDefault(o.failureReason, "none"),
		AuthMode:                     orDefault(o.authMode, "unavailable"),
		CredentialState:              orDefault(o.credentialState, "not_evaluated"),
		ScopeState:                   orDefault(o.scopeState, "not_evaluated"),
		SessionHeader:                orDefault(o.sessionHeader, httpSessionHeaderAbsent),
		SessionState:                 orDefault(o.sessionState, "not_evaluated"),
		BindingRelation:              orDefault(o.bindingRelation, "not_evaluated"),
		SessionRefState:              orDefault(o.sessionRefState, "not_applicable"),
		ChallengeKind:                orDefault(o.challengeKind, "none"),
		ChallengeTarget:              orDefault(o.challengeTarget, "none"),
		MetadataTargetRegistered:     orDefault(o.metadataRegistered, "not_applicable"),
		DiagnosticPolicyRevision:     "oauth-diag-policy-v1",
		EffectivePolicyRevisionState: o.diagnostics.provenance.EffectivePolicyRevisionState,
		EffectivePolicyRevision:      o.diagnostics.provenance.EffectivePolicyRevision,
		PolicySource:                 o.diagnostics.provenance.PolicySource,
		OAuthEnabled:                 o.diagnostics.provenance.OAuthEnabled,
		StaticConfigured:             o.diagnostics.provenance.StaticConfigured,
		ScopeRequired:                o.diagnostics.provenance.ScopeRequired,
		ResourcePathMode:             o.diagnostics.provenance.ResourcePathMode,
		SessionBinding:               orDefault(o.sessionBinding, "unavailable"),
		ClockSkewSeconds:             o.diagnostics.provenance.ClockSkewSeconds,
		UpstreamState:                orDefault(o.upstreamState, "not_attempted"),
		ErrorDetailState:             orDefault(o.errorDetailState, "not_applicable"),
		ErrorDetail:                  o.errorDetail,
	}
	if o.correlationState == CorrelationAvailable {
		diagnostic.RequestID = o.requestID
	}
	diagnostic.OperationID = o.operationID
	diagnostic.SessionRef = o.sessionRef
	if o.legacyOperation == "tools_list" && httpOutcome == "success" && o.exposedTools != nil {
		diagnostic.ExposedTools = o.exposedTools
	}

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
