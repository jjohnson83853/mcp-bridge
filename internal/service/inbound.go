package service

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
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
	// The terminal timer starts at handler entry, before owned generation, so
	// the observed duration includes any work the generator itself performs.
	started := h.now()
	// internalID is generated independently of client input; the client value is
	// echoed on the wire but never used as a log identity.
	internalID := h.generator()

	var captured *bodyCapture
	var bounded *limitObserver
	status := &statusWriter{ResponseWriter: w}

	// Owned failure state, set exactly at the boundary of failure.
	finalReason := "none"
	finalStage := "none"
	counted := false // Track whether metrics should be recorded (not for early 404)

	defer func() {
		// Metrics: only for routed requests (not early 404).
		// "early404 previously returned before metrics defer; new logging must not silently start counting it."
		if h.metrics != nil && counted {
			outcome := "success"
			if status.status >= http.StatusBadRequest {
				outcome = "failure"
			}
			operation := inboundOperation(captured)
			h.metrics.RecordRequest(operation, outcome)
		}

		if h.logger != nil {
			elapsed := h.now().Sub(started)
			observedStatus := status.status
			// Severity follows the known owned cause, not the raw status
			// class: admission rejections are warn even at 5xx (session
			// capacity 503), and only unclassified internal failures are
			// error.
			level := "info"
			switch finalStage {
			case "http_route", "host_admission", "origin_admission", "client_auth",
				"session_admission", "request_limits":
				level = "warn"
			case "response_delivery":
				if finalReason == "client_disconnected" {
					level = "warn"
				} else {
					level = "error"
				}
			default:
				if observedStatus >= http.StatusBadRequest {
					if observedStatus >= 500 {
						level = "error"
					} else {
						level = "warn"
					}
				}
			}

			// http_outcome
			httpOutcome := "success"
			if status.writeErr != nil {
				// An observed delegated write failure means the response was
				// not fully delivered, regardless of the committed status.
				httpOutcome = "incomplete"
			} else if observedStatus >= http.StatusBadRequest {
				httpOutcome = "failure"
			}
			if observedStatus == 0 {
				httpOutcome = "unknown"
			}

			// Byte bucket
			bucket := "empty"
			switch {
			case status.bytes == 0:
				bucket = "empty"
			case status.bytes <= 1024:
				bucket = "le_1k"
			case status.bytes <= 16384:
				bucket = "le_16k"
			case status.bytes <= 262144:
				bucket = "le_256k"
			case status.bytes <= 1048576:
				bucket = "le_1m"
			default:
				bucket = "gt_1m"
			}

			var httpStatus any
			if observedStatus >= 100 && observedStatus <= 599 {
				httpStatus = observedStatus
			}

			route := "mcp"
			if r.URL.Path != "/mcp" {
				route = "not_found"
			}

			operation := inboundOperation(captured)

			// Upgraded HTTP terminals carry no tool/backend/server execution
			// identity: the record is boundary-scoped. Legacy diagnostics and
			// metrics keep their own server attribution elsewhere.
			// The terminal outcome vocabulary has no incomplete scalar: an
			// observed incomplete delivery is an overall failure, while the
			// HTTP-specific field keeps the finer incomplete value.
			outcome := httpOutcome
			if outcome == "incomplete" {
				outcome = "failure"
			}

			fields := map[string]any{
				"http_status":       httpStatus,
				"http_method":       r.Method,
				"route":             route,
				"http_outcome":      httpOutcome,
				"stage":             finalStage,
				"timing_scope":      "http",
				"http_bytes_bucket": bucket,
			}
			if internalID == "" {
				// The owned generator produced no identity: report the state
				// honestly instead of minting a replacement owned ID.
				fields["correlation_state"] = "unavailable"
			}
			h.logger.Log(observe.Entry{
				Level:     level,
				Event:     "http_complete",
				RequestID: internalID,
				Operation: operation,
				Duration:  elapsed,
				Outcome:   outcome,
				Reason:    finalReason,
				Fields:    fields,
			})
		}
	}()

	if r.URL.Path != "/mcp" {
		finalStage = "http_route"
		finalReason = "route_not_found"
		http.NotFound(status, r)
		return
	}

	// Now count metrics for the rest of the request.
	counted = true
	requestID := observe.CorrelationID(r.Header.Get("X-Request-ID"))
	r.Header.Set("X-Request-ID", requestID)
	status.Header().Set("X-Request-ID", requestID)
	captured = captureRequestMethod(r)

	bounds := h.bounds()
	if !hostAllowed(bounds.AllowedHosts, r.Host) {
		finalStage = "host_admission"
		finalReason = "host_rejected"
		http.Error(status, "forbidden", http.StatusForbidden)
		return
	}
	if !originAllowed(bounds.AllowedOrigins, r.Header.Get("Origin")) {
		finalStage = "origin_admission"
		finalReason = "origin_rejected"
		http.Error(status, "forbidden", http.StatusForbidden)
		return
	}
	if limits := bounds.MaxRequestBytes; limits > 0 {
		if declared := r.ContentLength; declared > limits {
			finalStage = "request_limits"
			finalReason = "request_too_large"
			http.Error(status, "request too large", http.StatusRequestEntityTooLarge)
			return
		}
		// The native writer (not the tracking wrapper) is passed so net/http's
		// close-on-limit signalling reaches the connection directly; the typed
		// limit error is observed as it passes through without extra reads.
		bounded = &limitObserver{ReadCloser: http.MaxBytesReader(w, r.Body, limits)}
		r.Body = bounded
	}

	tokenHash, ok := h.authenticate(r)
	if !ok {
		status.Header().Set("WWW-Authenticate", "Bearer")
		finalStage = "client_auth"
		finalReason = "client_auth_failed"
		http.Error(status, "unauthorized", http.StatusUnauthorized)
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
			status.Header().Set("WWW-Authenticate", "Bearer")
			finalStage = "session_admission"
			finalReason = "session_auth_failed"
			http.Error(status, "unauthorized", http.StatusUnauthorized)
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
				finalStage = "session_admission"
				finalReason = "session_capacity"
				http.Error(status, "session cap reached", http.StatusServiceUnavailable)
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
	if internalID != "" {
		rClone.Header.Set("X-Mcp-Bridge-Internal-Request-Id", internalID)
	}
	h.next.ServeHTTP(status, rClone)
	var maxBytes *http.MaxBytesError
	if bounded != nil && errors.As(bounded.observed, &maxBytes) {
		// The typed limit error was observed while the delegated handler read
		// the body: the first decisive cause is the admission bound, not the
		// rejected dispatch status it produced.
		finalStage = "request_limits"
		finalReason = "request_too_large"
	} else if status.writeErr != nil {
		// Only an actual delegated write failure establishes the delivery
		// stage; observing a normal Write or Flush is not a failure and a
		// successful delivery keeps stage "none".
		finalStage = "response_delivery"
		finalReason = writeFailureReason(status.writeErr)
	} else if status.status >= http.StatusBadRequest {
		// The delegated dispatch produced a rejected response; the fact was
		// established at the dispatch boundary.
		finalStage = "mcp_dispatch"
	} else {
		finalStage = "none"
	}

	if issuedID := status.Header().Get(sessionIDHeader); issuedID != "" {
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

// writeFailureReason classifies an observed delegated write failure into the
// finite terminal reason vocabulary. Classification is typed only: the raw
// error text is never parsed, retained, or emitted.
func writeFailureReason(err error) string {
	if errors.Is(err, net.ErrClosed) {
		return "client_disconnected"
	}
	if errors.Is(err, context.Canceled) {
		return "request_cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	return "unknown"
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

type statusWriter struct {
	http.ResponseWriter
	status   int
	bytes    int64
	writeErr error
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		// The first status is the response's status; forwarding a second one
		// would only log a superfluous call.
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

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

func (w *statusWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Write records the implicit 200 status of the first write, exactly as net/http
// does, so a handler that streams a body without naming a status never makes
// Flush emit a second, superfluous header.
func (w *statusWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += int64(n)
	if w.writeErr == nil {
		w.writeErr = err
	}
	return n, err
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
