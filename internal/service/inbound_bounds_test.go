package service

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// sessionTestToken is the plaintext bearer token the inbound tests present; the
// helper hashes it into the fingerprints the handler keeps.
const sessionTestToken = "test-session-token"

// stubInnerHandler records whether the wrapped inbound handler reached it and
// captures the request body so tests can observe the enforced byte limit.
type stubInnerHandler struct {
	reached  bool
	bodyText string
	bodyErr  error
}

func (s *stubInnerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.reached = true
	body, err := io.ReadAll(r.Body)
	s.bodyErr = err
	s.bodyText = string(body)
}

// sessionIssuingHandler stands in for the streamable MCP handler, which issues
// a session identifier on a session-initialising POST. Every other request
// passes through untouched.
type sessionIssuingHandler struct {
	next http.Handler
	seq  int
}

func (h *sessionIssuingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.Header.Get(sessionIDHeader) == "" {
		h.seq++
		w.Header().Set(sessionIDHeader, fmt.Sprintf("session-%d", h.seq))
	}
	h.next.ServeHTTP(w, r)
}

// testInbound builds the inbound edge in front of a session-issuing inner
// handler, so the tests exercise admission without a real MCP server.
func testInbound(inner http.Handler, options ...InboundOptions) *InboundHandler {
	return NewInboundHandler(&sessionIssuingHandler{next: inner}, tokenHashFor(sessionTestToken), options...)
}

// tokenHashFor is the test-side fingerprint of a plaintext token.
func tokenHashFor(token string) [][32]byte {
	return [][32]byte{sha256.Sum256([]byte(token))}
}

// assertBearerAuth asserts a bare 401 carrying exactly one challenge, so these
// tests fail on any unrelated regression.
func assertBearerAuth(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("response: %v", rec)
	}
	if len(rec.Header().Values("WWW-Authenticate")) != 1 {
		t.Fatalf("response: %v", rec)
	}
}

// requestFor builds an authenticated POST/GET with an optional session id.
func requestFor(method, sessionID, body string) *http.Request {
	req := httptest.NewRequest(method, "/mcp", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+sessionTestToken)
	if sessionID != "" {
		req.Header.Set(sessionIDHeader, sessionID)
	}
	return req
}

// serve runs one request through the handler and returns the recorder.
func serve(inbound *InboundHandler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	inbound.ServeHTTP(rec, req)
	return rec
}

func TestMaxRequestBytesOversizedDeclaredBody(t *testing.T) {
	inner := &stubInnerHandler{}
	inbound := testInbound(inner, InboundOptions{MaxRequestBytes: 16})
	req := requestFor(http.MethodPost, "", strings.Repeat("x", 64))
	req.Header.Set("Content-Length", "64")
	req.ContentLength = 64

	rec := serve(inbound, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("response: %v", rec)
	}
	if inner.reached {
		t.Fatalf("inner reached: %v", rec)
	}
}

func TestMaxRequestBytesUndeclaredBodyLimited(t *testing.T) {
	inner := &stubInnerHandler{}
	inbound := testInbound(inner, InboundOptions{MaxRequestBytes: 16})
	req := requestFor(http.MethodPost, "", strings.Repeat("x", 64))
	req.Header.Del("Content-Length")
	req.ContentLength = -1

	rec := serve(inbound, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("response: %v", rec)
	}
	if !inner.reached {
		t.Fatalf("inner reached: %v", rec)
	}
	if inner.bodyText != strings.Repeat("x", 16) {
		t.Fatalf("body: %q", inner.bodyText)
	}
	var limited *http.MaxBytesError
	if !errors.As(inner.bodyErr, &limited) {
		t.Fatalf("bodyErr: %v", inner.bodyErr)
	}
	if limited.Limit != 16 {
		t.Fatalf("bodyErr: %v", inner.bodyErr)
	}
}

func TestMaxRequestBytesOversizedBodyOnGetNoPanic(t *testing.T) {
	inner := &stubInnerHandler{}
	inbound := testInbound(inner, InboundOptions{MaxRequestBytes: 16})
	req := requestFor(http.MethodGet, "", "")

	rec := serve(inbound, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("response: %v", rec)
	}
	if !inner.reached {
		t.Fatalf("inner reached: %v", rec)
	}
}

func TestMaxSessionsCap(t *testing.T) {
	inner := &stubInnerHandler{}
	inbound := testInbound(inner, InboundOptions{MaxSessions: 1})

	rec := serve(inbound, requestFor(http.MethodPost, "", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("response: %v", rec)
	}
	first := rec.Header().Get(sessionIDHeader)
	if first == "" {
		t.Fatalf("response: %v", rec)
	}

	rec = serve(inbound, requestFor(http.MethodPost, "", ""))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("response: %v", rec)
	}
	if rec.Header().Get(sessionIDHeader) != "" {
		t.Fatalf("response: %v", rec)
	}
	if inbound.liveSessions() != 1 {
		t.Fatalf("live sessions: %d", inbound.liveSessions())
	}

	rec = serve(inbound, requestFor(http.MethodPost, first, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("response: %v", rec)
	}
	if inbound.liveSessions() != 1 {
		t.Fatalf("live sessions: %d", inbound.liveSessions())
	}
}

func TestSessionIdleExpiry(t *testing.T) {
	clock := time.Unix(1_000, 0)
	inner := &stubInnerHandler{}
	inbound := testInbound(inner, InboundOptions{SessionIdle: time.Minute, Now: func() time.Time { return clock }})

	rec := serve(inbound, requestFor(http.MethodPost, "", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("response: %v", rec)
	}
	id := rec.Header().Get(sessionIDHeader)

	// The session expires after the idle window and is refused, not re-created.
	clock = clock.Add(time.Minute + time.Second)
	rec = serve(inbound, requestFor(http.MethodPost, id, ""))
	assertBearerAuth(t, rec)
	if rec.Header().Get(sessionIDHeader) != "" {
		t.Fatalf("response: %v", rec)
	}
	if inbound.liveSessions() != 0 {
		t.Fatalf("live sessions: %d", inbound.liveSessions())
	}
}

func TestSessionIdleWithinWindow(t *testing.T) {
	clock := time.Unix(1_000, 0)
	inner := &stubInnerHandler{}
	inbound := testInbound(inner, InboundOptions{SessionIdle: time.Minute, Now: func() time.Time { return clock }})

	rec := serve(inbound, requestFor(http.MethodPost, "", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("response: %v", rec)
	}
	id := rec.Header().Get(sessionIDHeader)

	clock = clock.Add(time.Minute - time.Second)
	rec = serve(inbound, requestFor(http.MethodPost, id, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("response: %v", rec)
	}

	// Activity refreshed the record, so the session survives another window
	// measured from the last request rather than from creation.
	clock = clock.Add(time.Minute - time.Second)
	rec = serve(inbound, requestFor(http.MethodPost, id, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("response: %v", rec)
	}
}

func TestInboundBoundsZeroOptionsUnchanged(t *testing.T) {
	inner := &stubInnerHandler{}
	inbound := testInbound(inner)

	rec := serve(inbound, requestFor(http.MethodPost, "", "hello"))
	if rec.Code != http.StatusOK {
		t.Fatalf("response: %v", rec)
	}
	if !inner.reached {
		t.Fatalf("inner reached: %v", rec)
	}
	id := rec.Header().Get(sessionIDHeader)
	if id == "" {
		t.Fatalf("response: %v", rec)
	}

	inner.reached = false
	rec = serve(inbound, requestFor(http.MethodGet, id, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("response: %v", rec)
	}
	if !inner.reached {
		t.Fatalf("inner reached: %v", rec)
	}
}

func TestInboundBoundsCapReapsExpired(t *testing.T) {
	clock := time.Unix(1_000, 0)
	inner := &stubInnerHandler{}
	inbound := testInbound(inner, InboundOptions{MaxSessions: 1, SessionIdle: time.Minute, Now: func() time.Time { return clock }})

	rec := serve(inbound, requestFor(http.MethodPost, "", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("response: %v", rec)
	}

	// The cap is admitted because the expired session is reaped first.
	clock = clock.Add(time.Minute + time.Second)
	rec = serve(inbound, requestFor(http.MethodPost, "", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("response: %v", rec)
	}
	if inbound.liveSessions() != 1 {
		t.Fatalf("live sessions: %d", inbound.liveSessions())
	}
}

func TestInboundBoundsRefuseForeignHostAndOrigin(t *testing.T) {
	allowHost := InboundOptions{AllowedHosts: []string{"bridge.example"}}
	allowOrigin := InboundOptions{AllowedOrigins: []string{"https://app.example"}}

	tests := []struct {
		name   string
		opts   InboundOptions
		host   string
		origin string
		want   int
		reach  bool
	}{
		{"allowed host", allowHost, "bridge.example", "", http.StatusOK, true},
		{"allowed host is case-insensitive", allowHost, "BRIDGE.example", "", http.StatusOK, true},
		{"foreign host", allowHost, "evil.example", "", http.StatusForbidden, false},
		{"allowed origin", allowOrigin, "", "https://app.example", http.StatusOK, true},
		{"allowed origin ignores trailing slash", allowOrigin, "", "https://app.example/", http.StatusOK, true},
		{"foreign origin", allowOrigin, "", "https://evil.example", http.StatusForbidden, false},
		{"absent origin is admitted", allowOrigin, "", "", http.StatusOK, true},
		{"no allowlists admits anything", InboundOptions{}, "anything.example", "https://anything.example", http.StatusOK, true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			inner := &stubInnerHandler{}
			inbound := testInbound(inner, testCase.opts)
			req := requestFor(http.MethodPost, "", "")
			if testCase.host != "" {
				req.Host = testCase.host
			}
			if testCase.origin != "" {
				req.Header.Set("Origin", testCase.origin)
			}

			rec := serve(inbound, req)

			if rec.Code != testCase.want {
				t.Fatalf("response: %v", rec)
			}
			if inner.reached != testCase.reach {
				t.Fatalf("inner reached: %v", rec)
			}
		})
	}
}
