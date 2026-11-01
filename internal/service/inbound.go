package service

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/auth"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

const sessionIDHeader = "Mcp-Session-Id"

// InboundHandler is the inbound edge: it exposes exactly /mcp, authenticates
// every method, applies the admission bounds of the current generation and
// tracks live sessions.
type InboundHandler struct {
	next        http.Handler
	tokenHashes [][32]byte
	options     InboundOptions

	mu        sync.Mutex
	sessions  map[string]sessionRecord
	lastSweep time.Time
	logger    *observe.Logger
	metrics   *observe.Metrics

	// generator produces the server-owned correlation ID. Default is
	// observe.CorrelationID(""), but tests may replace it per-instance
	// before traffic; production never mutates it after construction.
	generator func() string

	// diagOnce guards the lazy standalone fallback owner construction.
	diagOnce            sync.Once
	fallbackDiagnostics *observe.HTTPDiagnostics
}

type sessionRecord struct {
	token    [32]byte
	lastSeen time.Time
	epoch    uint64
	// ref is the 32 lowercase hexadecimal diagnostic reference drawn once
	// when the live session was admitted; empty means the reference is
	// explicitly unavailable. It is retained only with the session.
	ref string
}

// InboundBounds are the admission bounds of the currently published generation.
// Every zero field means "unbounded", i.e. today's behaviour; an empty
// AllowedHosts or AllowedOrigins list disables that check.
type InboundBounds struct {
	MaxRequestBytes int64
	MaxSessions     int
	SessionIdle     time.Duration
	AllowedHosts    []string
	AllowedOrigins  []string
}

// InboundOptions are the generation-aware admission options of the inbound
// edge. The accessors are consulted per request so a configuration reload that
// changes tokens or bounds takes effect without restarting the process.
type InboundOptions struct {
	Logger  *observe.Logger
	Metrics *observe.Metrics

	// Diagnostics supplies the shared outer completion owner. nil means the
	// handler claims through its own lazily built fallback owner using the
	// same logger, Now and private generator conventions.
	Diagnostics *observe.HTTPDiagnostics

	// Static bounds, used when Bound is nil.
	MaxRequestBytes int64
	MaxSessions     int
	SessionIdle     time.Duration
	AllowedHosts    []string
	AllowedOrigins  []string

	// Bound returns the bounds of the current generation. nil means the static
	// bounds above.
	Bound func() InboundBounds
	// Tokens returns the accepted token fingerprints of the current
	// generation. nil means the static slice passed to NewInboundHandler.
	Tokens func() [][32]byte
	// Now is the reaping/expiry clock; nil means time.Now.
	Now func() time.Time

	// NewSessionRef draws one independent diagnostic reference for a newly
	// admitted live session. nil uses cryptographic generation. An error or
	// malformed value leaves the reference explicitly unavailable: no retry,
	// no derived fallback, no substitute identity.
	NewSessionRef func() (string, error)

	// ToolSurface owns the process-local projection incarnation associated
	// with each ordinary response-issued session.
	ToolSurface *ToolSurface
}

// NewInboundHandler exposes exactly /mcp and authenticates every method. Tokens
// are supplied as SHA-256 fingerprints so the handler does not retain secrets.
func NewInboundHandler(next http.Handler, tokenHashes [][32]byte, options ...InboundOptions) *InboundHandler {
	var opts InboundOptions
	if len(options) > 0 {
		opts = options[0]
	}
	handler := &InboundHandler{
		next:        next,
		tokenHashes: append([][32]byte(nil), tokenHashes...),
		options:     opts,
		sessions:    make(map[string]sessionRecord),
		logger:      opts.Logger,
		metrics:     opts.Metrics,
		generator: func() string {
			return observe.CorrelationID("")
		},
	}
	if opts.ToolSurface != nil {
		opts.ToolSurface.registerReceivingMiddleware()
	}
	return handler
}

func (h *InboundHandler) bounds() InboundBounds {
	if h.options.Bound != nil {
		return h.options.Bound()
	}
	return InboundBounds{
		MaxRequestBytes: h.options.MaxRequestBytes,
		MaxSessions:     h.options.MaxSessions,
		SessionIdle:     h.options.SessionIdle,
		AllowedHosts:    h.options.AllowedHosts,
		AllowedOrigins:  h.options.AllowedOrigins,
	}
}

// liveSessions reports the number of tracked sessions; tests use it to pin the
// session bookkeeping.
func (h *InboundHandler) liveSessions() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.sessions)
}

func (h *InboundHandler) acceptedTokens() [][32]byte {
	if h.options.Tokens != nil {
		if tokens := h.options.Tokens(); tokens != nil {
			return tokens
		}
	}
	return h.tokenHashes
}

func (h *InboundHandler) now() time.Time {
	if h.options.Now != nil {
		return h.options.Now()
	}
	return time.Now()
}

// defaultSessionRef draws one independent 128-bit reference cryptographically.
func defaultSessionRef() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(random[:]), nil
}

// newSessionRef draws the diagnostic reference for a newly admitted live
// session. One entropy failure or malformed draw leaves it explicitly
// unavailable: no retry, no recursive logging, no derived fallback.
func (h *InboundHandler) newSessionRef() (string, bool) {
	generate := h.options.NewSessionRef
	if generate == nil {
		generate = defaultSessionRef
	}
	ref, err := generate()
	if err != nil || !validDiagnosticRefShape(ref) {
		return "", false
	}
	return ref, true
}

// validDiagnosticRefShape reports whether the value is the only reference
// shape the typed schema allows: 32 lowercase hexadecimal characters.
func validDiagnosticRefShape(value string) bool {
	if len(value) != 32 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func (h *InboundHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Work on a request-local clone so diagnostic and admission header
	// normalization never mutates the caller's request.
	r = r.Clone(r.Context())
	// The existing outer owner is claimed first; when none is present (a
	// standalone inner deployment) the handler claims through its own lazily
	// built fallback owner using the same logger, clock and generator.
	diagnostics := h.diagnostics()
	writer, r, lease := diagnostics.Claim(w, r)
	if lease.OwnsCompletion() {
		// The claimant lease finalizes the single terminal record on normal
		// return and panic unwinding alike: no recover, no synthetic 500.
		defer lease.Finish()
	}
	// Actual dispatch classification: requests reaching this handler that were
	// not already classified as OAuth-protected were dispatched directly to
	// the inbound edge. First-wins keeps the earlier oauth_protected label.
	observe.ContributeHTTP(r.Context(), observe.HTTPDecision{RouteOutcome: "inbound_direct"})

	var captured *bodyCapture
	var bounded *limitObserver

	// The first decisive admission rejection, recorded exactly at the boundary
	// that rejected the request, in the legacy stage/reason vocabulary.
	var rejection legacyRejection

	// Owned failure state, set exactly at the boundary of failure.
	counted := false // Track whether metrics should be recorded (not for early 404)

	// The legacy summarisation defer is registered after claimant Finish so
	// LIFO unwinding runs it first: it reads the SAME owner observation the
	// body/delivery classifiers use, then contributes legacy evidence.
	defer func() {
		// Metrics: only for routed requests (not early 404).
		// "early404 previously returned before metrics defer; new logging must not silently start counting it."
		if h.metrics != nil && counted {
			obs := observe.HTTPResponseObservation(r.Context())
			outcome := "success"
			if obs.Status >= http.StatusBadRequest || obs.WriteFailure != observe.HTTPWriteNone {
				outcome = "failure"
			}
			operation := inboundOperation(captured)
			h.metrics.RecordRequest(operation, outcome)
		}

		if h.logger != nil {
			obs := observe.HTTPResponseObservation(r.Context())
			// Legacy classification from the owner observation.
			legacyStage, legacyReason, operation := h.legacyClassification(r, obs, captured, rejection)

			observe.ContributeLegacyEvidence(r.Context(), observe.HTTPLegacyEvidence{
				Stage: legacyStage, Reason: legacyReason, Operation: operation,
			})
		}
	}()

	if r.URL.Path != "/mcp" {
		observe.ContributeHTTP(r.Context(), observe.HTTPDecision{
			FailureStage: "route", FailureReason: "route_not_found",
			AuthMode: "none",
		})
		http.NotFound(writer, r)
		return
	}

	// Now count metrics for the rest of the request.
	counted = true
	requestID := observe.CorrelationID(r.Header.Get("X-Request-ID"))
	r.Header.Set("X-Request-ID", requestID)
	writer.Header().Set("X-Request-ID", requestID)
	captured = captureRequestMethod(r)

	bounds := h.bounds()
	if !hostAllowed(bounds.AllowedHosts, r.Host) {
		rejection = legacyRejection{"host_admission", "host_rejected"}
		observe.ContributeHTTP(r.Context(), observe.HTTPDecision{
			FailureStage: "host", FailureReason: "host_rejected",
		})
		http.Error(writer, "forbidden", http.StatusForbidden)
		return
	}
	if !originAllowed(bounds.AllowedOrigins, r.Header.Get("Origin")) {
		rejection = legacyRejection{"origin_admission", "origin_rejected"}
		observe.ContributeHTTP(r.Context(), observe.HTTPDecision{
			FailureStage: "origin", FailureReason: "origin_rejected",
		})
		http.Error(writer, "forbidden", http.StatusForbidden)
		return
	}
	if limits := bounds.MaxRequestBytes; limits > 0 {
		if declared := r.ContentLength; declared > limits {
			rejection = legacyRejection{"request_limits", "request_too_large"}
			observe.ContributeHTTP(r.Context(), observe.HTTPDecision{
				FailureStage: "body_limit", FailureReason: "request_too_large",
			})
			http.Error(writer, "request too large", http.StatusRequestEntityTooLarge)
			return
		}
		// The native writer (not the facade) is passed so net/http's
		// close-on-limit signalling reaches the connection directly; the typed
		// limit error is observed as it passes through without extra reads.
		bounded = &limitObserver{ReadCloser: http.MaxBytesReader(observe.NativeHTTPWriter(r.Context(), writer), r.Body, limits)}
		r.Body = bounded
	}

	tokenHash, ok := h.authenticate(r)
	if !ok {
		writer.Header().Set("WWW-Authenticate", "Bearer")
		rejection = legacyRejection{"client_auth", "client_auth_failed"}
		observe.ContributeHTTP(r.Context(), observe.HTTPDecision{
			FailureStage: "inner_credential", FailureReason: "static_invalid",
			AuthMode: "static", CredentialState: "rejected",
			// The session entry and comparator were not evaluated at this
			// boundary; only header presence (its own slot) is observed.
			SessionState: "not_evaluated",
		})
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}

	sessionID := r.Header.Get(sessionIDHeader)
	rClone := r.Clone(r.Context())
	removeHeaderCaseInsensitive(rClone.Header, exposureEpochHeaderName)
	if sessionID != "" {
		h.mu.Lock()
		now := h.now()
		retiredOwners := h.reapExpiredLocked(now)
		bound, exists := h.sessions[sessionID]
		bindingMatch := exists && subtle.ConstantTimeCompare(bound.token[:], tokenHash[:]) == 1
		if bindingMatch {
			// Idle is inactivity, so an accepted request refreshes the record
			// without replacing its diagnostic reference or incarnation.
			sessionEpoch := bound.epoch
			h.sessions[sessionID] = sessionRecord{token: bound.token, lastSeen: now, ref: bound.ref, epoch: sessionEpoch}
			if sessionEpoch != 0 {
				rClone.Header.Set(exposureEpochHeaderName, strconv.FormatUint(sessionEpoch, 10))
			}
		}
		h.mu.Unlock()
		h.cleanupRetiredOwners(retiredOwners)
		if !bindingMatch {
			if !exists {
				// A supplied session header with no stored entry: the entry is
				// absent and the comparator never ran. The reference was
				// deleted with the session, so nothing is retained here.
				observe.ContributeHTTP(r.Context(), observe.HTTPDecision{
					FailureStage: "session", FailureReason: "session_absent",
					SessionState: "absent", BindingRelation: "not_evaluated",
				})
				observe.ContributeSessionEvidence(r.Context(), observe.HTTPSessionEvidence{
					RefState: "not_applicable", State: "absent", Binding: "not_evaluated",
				})
			} else {
				// An existing entry bound to a different bearer: the comparator
				// mismatched. The live reference appears in the server record
				// only; it is never written to the response.
				observe.ContributeHTTP(r.Context(), observe.HTTPDecision{
					FailureStage: "session", FailureReason: "session_fingerprint_mismatch",
					SessionState: "existing", BindingRelation: "mismatch",
				})
				observe.ContributeSessionEvidence(r.Context(), observe.HTTPSessionEvidence{
					RefState: diagnosticRefState(bound.ref), Ref: bound.ref,
					State: "existing", Binding: "mismatch",
				})
			}
			// Session admission failures (absent entry or binding mismatch)
			// return 404 without a Bearer challenge so MCP clients treat the
			// session as lost or unusable, reinitialize, and retry.
			rejection = legacyRejection{"session_admission", "session_auth_failed"}
			http.Error(writer, "session not found", http.StatusNotFound)
			return
		}
		observe.ContributeHTTP(r.Context(), observe.HTTPDecision{
			SessionState: "existing", BindingRelation: "match",
		})
		observe.ContributeSessionEvidence(r.Context(), observe.HTTPSessionEvidence{
			RefState: diagnosticRefState(bound.ref), Ref: bound.ref,
			State: "existing", Binding: "match",
		})
	} else {
		// Creating a new session: enforce the session cap after reaping.
		if maximum := bounds.MaxSessions; maximum > 0 {
			h.mu.Lock()
			now := h.now()
			retiredOwners := h.reapExpiredLocked(now)
			atCapacity := len(h.sessions) >= maximum
			h.mu.Unlock()
			h.cleanupRetiredOwners(retiredOwners)
			if atCapacity {
				rejection = legacyRejection{"session_admission", "session_capacity"}
				observe.ContributeHTTP(r.Context(), observe.HTTPDecision{
					FailureStage: "capacity", FailureReason: "session_capacity",
				})
				http.Error(writer, "session cap reached", http.StatusServiceUnavailable)
				return
			}
		}
	}

	// Preserve the existing server-owned correlation ID hygiene on the clone.
	rClone.Header.Del("X-Mcp-Bridge-Internal-Request-Id")
	if internalID := observe.HTTPRequestID(r.Context()); internalID != "" {
		rClone.Header.Set("X-Mcp-Bridge-Internal-Request-Id", internalID)
	}
	h.next.ServeHTTP(writer, rClone)
	var maxBytes *http.MaxBytesError
	if bounded != nil && errors.As(bounded.observed, &maxBytes) {
		// The typed limit error was observed while the delegated handler read
		// the body: the first decisive cause is the admission bound, not the
		// rejected dispatch status it produced.
		rejection = legacyRejection{"request_limits", "request_too_large"}
		observe.ContributeHTTP(r.Context(), observe.HTTPDecision{
			FailureStage: "body_limit", FailureReason: "request_too_large",
		})
	} else {
		obs := observe.HTTPResponseObservation(r.Context())
		if obs.WriteFailure != observe.HTTPWriteNone {
			// Only an actual delegated write failure establishes the delivery
			// stage; observing a normal Write or Flush is not a failure and a
			// successful delivery keeps stage "none".
			observe.ContributeHTTP(r.Context(), observe.HTTPDecision{
				FailureStage: "unavailable", FailureReason: writeFailureReasonFrom(obs.WriteFailure),
			})
		}
	}

	if issuedID := writer.Header().Get(sessionIDHeader); issuedID != "" && r.Method != http.MethodDelete {
		h.mu.Lock()
		record, exists := h.sessions[issuedID]
		created := !exists
		var ref string
		refAvailable := true
		if exists {
			// A response-issued identifier for an already tracked session
			// replaces the record without redrawing its reference or epoch.
			record.token = tokenHash
			record.lastSeen = h.now()
			ref = record.ref
		} else {
			// An admitted live session was created: draw its one independent
			// diagnostic reference and retain it only with this session.
			ref, refAvailable = h.newSessionRef()
			record = sessionRecord{token: tokenHash, lastSeen: h.now(), ref: ref}
		}
		if record.epoch == 0 && h.options.ToolSurface != nil {
			owner, err := h.options.ToolSurface.newLiveOwner(issuedID)
			if err != nil {
				panic(err)
			}
			record.epoch = owner.epoch
		}
		h.sessions[issuedID] = record
		h.mu.Unlock()
		if created && sessionID == "" {
			// Terminal created evidence; no comparator ran, and the reference
			// is the pseudonym recorded with the new session only.
			observe.ContributeHTTP(r.Context(), observe.HTTPDecision{
				SessionState: "created", BindingRelation: "not_evaluated",
			})
			observe.ContributeSessionEvidence(r.Context(), observe.HTTPSessionEvidence{
				RefState: refEvidenceState(ref, refAvailable), Ref: ref,
				State: "created", Binding: "not_evaluated",
			})
		}
	} else if sessionID == "" {
		// The request proceeded without a supplied or issued session.
		observe.ContributeHTTP(r.Context(), observe.HTTPDecision{
			SessionState: "not_supplied", BindingRelation: "not_evaluated",
		})
		observe.ContributeSessionEvidence(r.Context(), observe.HTTPSessionEvidence{
			RefState: "not_applicable", State: "not_supplied", Binding: "not_evaluated",
		})
	}

	if r.Method == http.MethodDelete && sessionID != "" {
		h.mu.Lock()
		record, exists := h.sessions[sessionID]
		delete(h.sessions, sessionID)
		var retiredOwner projectionOwner
		if exists && record.epoch != 0 && h.options.ToolSurface != nil {
			retiredOwner = projectionOwner{kind: projectionSession, sessionID: sessionID, epoch: record.epoch}
			h.options.ToolSurface.retireLiveOwnerAtInboundLock(retiredOwner)
		}
		h.mu.Unlock()
		if retiredOwner.epoch != 0 {
			h.options.ToolSurface.cleanupRetiredOwner(retiredOwner)
		}
	}
}

// diagnostics returns the shared outer owner when composed, or lazily builds
// the standalone fallback owner once. The fallback reuses the handler's
// logger, clock and generator conventions, never per-request state.
func (h *InboundHandler) diagnostics() *observe.HTTPDiagnostics {
	if h.options.Diagnostics != nil {
		return h.options.Diagnostics
	}
	h.diagOnce.Do(func() {
		h.fallbackDiagnostics = observe.NewHTTPDiagnostics(h.logger, observe.HTTPDiagnosticsOptions{
			Now:   h.options.Now,
			NewID: h.generator,
		})
	})
	return h.fallbackDiagnostics
}

// legacyRejection records the legacy stage/reason of the first decisive
// admission rejection, in the summarised legacy vocabulary.
type legacyRejection struct {
	stage, reason string
}

// diagnosticRefState reports the truthful reference state of a retained
// session record.
func diagnosticRefState(ref string) string {
	if validDiagnosticRefShape(ref) {
		return "available"
	}
	return "unavailable"
}

// refEvidenceState reports the reference state of a newly drawn reference.
func refEvidenceState(ref string, drawn bool) string {
	if drawn && validDiagnosticRefShape(ref) {
		return "available"
	}
	return "unavailable"
}

// legacyClassification computes the summarised legacy stage/reason/operation
// from the owner observation and the recorded decisive rejection, mirroring
// the branch order of the previous inner-only terminal defer.
func (h *InboundHandler) legacyClassification(r *http.Request, obs observe.HTTPObservation, captured *bodyCapture, rejection legacyRejection) (stage, reason, operation string) {
	operation = inboundOperation(captured)
	if r.URL.Path != "/mcp" {
		return "http_route", "route_not_found", operation
	}
	if rejection.stage != "" {
		// Legacy precedence: a decisive admission rejection (declared-limit
		// rejection or an observed MaxBytesError) classifies as
		// request_limits/request_too_large even when the write then failed.
		return rejection.stage, rejection.reason, operation
	}
	if obs.WriteFailure != observe.HTTPWriteNone {
		return "response_delivery", writeFailureReasonFrom(obs.WriteFailure), operation
	}
	if obs.Status >= http.StatusBadRequest {
		// The delegated dispatch produced a rejected response; the fact was
		// established at the dispatch boundary.
		return "mcp_dispatch", "none", operation
	}
	return "none", "none", operation
}

// writeFailureReasonFrom maps an owner write-failure class into the finite
// legacy terminal reason vocabulary. Classification is typed only: the raw
// error text is never parsed, retained, or emitted.
func writeFailureReasonFrom(class string) string {
	switch class {
	case observe.HTTPWriteDisconnected:
		return "client_disconnected"
	case observe.HTTPWriteCancelled:
		return "request_cancelled"
	case observe.HTTPWriteDeadline:
		return "deadline_exceeded"
	default:
		return "unknown"
	}
}

// reapExpiredLocked removes sessions whose idle window has elapsed. Caller
// holds h.mu. Sweeps at most once per second to keep the per-request cost
// bounded; the sweep clock is the injected reaping clock.
func (h *InboundHandler) reapExpiredLocked(now time.Time) []projectionOwner {
	idle := h.bounds().SessionIdle
	if idle <= 0 {
		return nil
	}
	if !h.lastSweep.IsZero() && now.Sub(h.lastSweep) < time.Second && len(h.sessions) < 64 {
		return nil
	}
	h.lastSweep = now
	var retired []projectionOwner
	for id, record := range h.sessions {
		if now.Sub(record.lastSeen) > idle {
			delete(h.sessions, id)
			if record.epoch != 0 && h.options.ToolSurface != nil {
				owner := projectionOwner{kind: projectionSession, sessionID: id, epoch: record.epoch}
				h.options.ToolSurface.retireLiveOwnerAtInboundLock(owner)
				retired = append(retired, owner)
			}
		}
	}
	return retired
}

func (h *InboundHandler) cleanupRetiredOwners(owners []projectionOwner) {
	if h.options.ToolSurface == nil {
		return
	}
	for _, owner := range owners {
		h.options.ToolSurface.cleanupRetiredOwner(owner)
	}
}

func removeHeaderCaseInsensitive(header http.Header, name string) {
	for key := range header {
		if strings.EqualFold(key, name) {
			delete(header, key)
		}
	}
}

// limitObserver records the error returned by the bounded request reader so
// the terminal record can classify an over-limit body from its typed error
// without re-reading the stream, copying the body, or parsing error prose.
type limitObserver struct {
	io.ReadCloser
	observed error
}

func (o *limitObserver) Read(p []byte) (int, error) {
	n, err := o.ReadCloser.Read(p)
	if err != nil {
		o.observed = err
	}
	return n, err
}

// methodCaptureLimit bounds how much request body the edge buffers to classify
// the JSON-RPC method for the request metric. A message is far smaller than
// this in practice, and a longer body simply classifies as the aggregate
// "request" operation instead of growing the capture.
const methodCaptureLimit = 4096

// bodyCapture records a bounded prefix of a request body. The streamable
// handler reads the body while the request is being served, and the deferred
// classification reads the capture afterwards, so access is guarded.
type bodyCapture struct {
	mu    sync.Mutex
	buf   []byte
	limit int
}

func (c *bodyCapture) Write(p []byte) (int, error) {
	c.mu.Lock()
	if room := c.limit - len(c.buf); room > 0 {
		if len(p) < room {
			room = len(p)
		}
		c.buf = append(c.buf, p[:room]...)
	}
	c.mu.Unlock()
	return len(p), nil
}

func (c *bodyCapture) captured() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte(nil), c.buf...)
}

// requestBody keeps the original body closable while the capture observes what
// the handler reads.
type requestBody struct {
	io.Reader
	io.Closer
}

// captureRequestMethod tees a bounded prefix of a POST body into a capture and
// returns it, or nil when the request carries no message to classify.
func captureRequestMethod(r *http.Request) *bodyCapture {
	if r.Method != http.MethodPost || r.Body == nil || r.Body == http.NoBody {
		return nil
	}
	capture := &bodyCapture{limit: methodCaptureLimit}
	r.Body = requestBody{Reader: io.TeeReader(r.Body, capture), Closer: r.Body}
	return capture
}

// inboundOperation classifies one inbound message into the finite metric
// vocabulary. Only the prompt surface is named individually; every other
// method stays the single aggregate "request" operation, so no client-chosen
// text can reach a label.
func inboundOperation(captured *bodyCapture) string {
	if captured == nil {
		return "request"
	}
	var header struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal(captured.captured(), &header); err != nil {
		return "request"
	}
	switch header.Method {
	case "prompts/list":
		return "prompts_list"
	case "prompts/get":
		return "prompts_get"
	}
	return "request"
}

// hostAllowed reports whether the request Host is admitted. An empty allowlist
// disables the check; otherwise the observed Host must match an entry exactly.
func hostAllowed(allowed []string, host string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, entry := range allowed {
		if strings.EqualFold(entry, host) {
			return true
		}
	}
	return false
}

// originAllowed reports whether the request Origin is admitted. An empty
// allowlist disables the check, and a request without an Origin header (a
// non-browser client) is always admitted.
func originAllowed(allowed []string, origin string) bool {
	if len(allowed) == 0 || origin == "" {
		return true
	}
	candidate := strings.TrimSuffix(origin, "/")
	for _, entry := range allowed {
		if strings.EqualFold(strings.TrimSuffix(entry, "/"), candidate) {
			return true
		}
	}
	return false
}

func (h *InboundHandler) authenticate(r *http.Request) ([32]byte, bool) {
	values := r.Header.Values("Authorization")
	if len(values) != 1 {
		return [32]byte{}, false
	}
	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return [32]byte{}, false
	}
	candidate := sha256.Sum256([]byte(parts[1]))
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		return candidate, true
	}
	match := 0
	for _, allowed := range h.acceptedTokens() {
		match |= subtle.ConstantTimeCompare(candidate[:], allowed[:])
	}
	return candidate, match == 1
}
