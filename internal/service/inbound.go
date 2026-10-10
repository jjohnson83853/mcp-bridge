package service

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

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
	if r.URL.Path != "/mcp" {
		http.NotFound(w, r)
		return
	}
	requestID := observe.CorrelationID(r.Header.Get("X-Request-ID"))
	r.Header.Set("X-Request-ID", requestID)
	started := h.now()
	status := &statusWriter{ResponseWriter: w}
	status.Header().Set("X-Request-ID", requestID)
	captured := captureRequestMethod(r)
	defer func() {
		outcome := "success"
		if status.status >= http.StatusBadRequest {
			outcome = "failure"
		}
		operation := inboundOperation(captured)
		if h.metrics != nil {
			h.metrics.RecordRequest(operation, outcome)
		}
		if h.logger != nil {
			reason := "none"
			if status.status == http.StatusUnauthorized {
				reason = "backend_auth_failed"
			}
			h.logger.Log(observe.Entry{Level: "info", Event: "request", RequestID: requestID, Server: "_bridge", Operation: operation, Duration: h.now().Sub(started), Outcome: outcome, Reason: reason})
		}
	}()

	bounds := h.bounds()
	if !hostAllowed(bounds.AllowedHosts, r.Host) || !originAllowed(bounds.AllowedOrigins, r.Header.Get("Origin")) {
		http.Error(status, "forbidden", http.StatusForbidden)
		return
	}
	if limits := bounds.MaxRequestBytes; limits > 0 {
		if declared := r.ContentLength; declared > limits {
			http.Error(status, "request too large", http.StatusRequestEntityTooLarge)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, limits)
	}

	tokenHash, ok := h.authenticate(r)
	if !ok {
		status.Header().Set("WWW-Authenticate", "Bearer")
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
				http.Error(status, "session cap reached", http.StatusServiceUnavailable)
				return
			}
			h.mu.Unlock()
		}
	}

	h.next.ServeHTTP(status, r)
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
	status int
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
	return w.ResponseWriter.Write(p)
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
	match := 0
	for _, allowed := range h.acceptedTokens() {
		match |= subtle.ConstantTimeCompare(candidate[:], allowed[:])
	}
	return candidate, match == 1
}
