package service

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
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
}

// NewInboundHandler exposes exactly /mcp and authenticates every method. Tokens
// are supplied as SHA-256 fingerprints so the handler does not retain secrets.
func NewInboundHandler(next http.Handler, tokenHashes [][32]byte, options ...InboundOptions) *InboundHandler {
	var opts InboundOptions
	if len(options) > 0 {
		opts = options[0]
	}
	return &InboundHandler{
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

func (h *InboundHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
			SessionState: sessionStateForAuth(r),
		})
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}

	sessionID := r.Header.Get(sessionIDHeader)
	if sessionID != "" {
		h.mu.Lock()
		now := h.now()
		h.reapExpiredLocked(now)
		bound, exists := h.sessions[sessionID]
		valid := exists && subtle.ConstantTimeCompare(bound.token[:], tokenHash[:]) == 1
		if valid {
			// Idle is inactivity, so an accepted request refreshes the record.
			h.sessions[sessionID] = sessionRecord{token: bound.token, lastSeen: now}
		}
		h.mu.Unlock()
		if !valid {
			writer.Header().Set("WWW-Authenticate", "Bearer")
			rejection = legacyRejection{"session_admission", "session_auth_failed"}
			observe.ContributeHTTP(r.Context(), observe.HTTPDecision{
				FailureStage: "session", FailureReason: "session_fingerprint_mismatch",
				SessionState: "existing",
			})
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
	} else {
		// Creating a new session: enforce the session cap after reaping.
		if maximum := bounds.MaxSessions; maximum > 0 {
			h.mu.Lock()
			now := h.now()
			h.reapExpiredLocked(now)
			if len(h.sessions) >= maximum {
				h.mu.Unlock()
				rejection = legacyRejection{"session_admission", "session_capacity"}
				observe.ContributeHTTP(r.Context(), observe.HTTPDecision{
					FailureStage: "capacity", FailureReason: "session_capacity",
				})
				http.Error(writer, "session cap reached", http.StatusServiceUnavailable)
				return
			}
			h.mu.Unlock()
		}
	}

	// The next stage receives a shallow clone with a copied header map, so
	// mutations here never leak into the caller's request while identity
	// hygiene stays intact: a client-supplied internal header is never trusted
	// as an identity and is replaced only by the owned generated one.
	rClone := r.Clone(r.Context())
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

	if issuedID := writer.Header().Get(sessionIDHeader); issuedID != "" {
		h.mu.Lock()
		h.sessions[issuedID] = sessionRecord{token: tokenHash, lastSeen: h.now()}
		h.mu.Unlock()
	}
	if r.Method == http.MethodDelete && sessionID != "" {
		h.mu.Lock()
		delete(h.sessions, sessionID)
		h.mu.Unlock()
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

// sessionStateForAuth reports the truthful session evaluation state at the
// inner credential rejection point: a supplied session header was evaluated
// before credential acceptance, an absent one was not.
func sessionStateForAuth(r *http.Request) string {
	if r.Header.Get(sessionIDHeader) != "" {
		return "existing"
	}
	return "not_evaluated"
}

// legacyRejection records the legacy stage/reason of the first decisive
// admission rejection, in the summarised legacy vocabulary.
type legacyRejection struct {
	stage, reason string
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
func (h *InboundHandler) reapExpiredLocked(now time.Time) {
	idle := h.bounds().SessionIdle
	if idle <= 0 {
		return
	}
	if !h.lastSweep.IsZero() && now.Sub(h.lastSweep) < time.Second && len(h.sessions) < 64 {
		return
	}
	h.lastSweep = now
	for id, record := range h.sessions {
		if now.Sub(record.lastSeen) > idle {
			delete(h.sessions, id)
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
