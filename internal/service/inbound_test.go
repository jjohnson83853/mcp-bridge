package service_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/auth"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

func TestInboundHandlerMatchesOnlyExactMCPRoute(t *testing.T) {
	reached := false
	handler := service.NewInboundHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { reached = true; w.WriteHeader(http.StatusNoContent) }), tokenHash("client-token"))
	for _, path := range []string{"/mcp/alpha", "/mcp/", "/other"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set("Authorization", "Bearer client-token")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != http.StatusNotFound {
			t.Fatalf("path=%q status=%d, want 404", path, response.Code)
		}
	}
	if reached {
		t.Fatal("non-exact route reached MCP handler")
	}
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer client-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusNoContent || !reached {
		t.Fatalf("/mcp status=%d reached=%v", response.Code, reached)
	}
}

func TestInboundAuthenticationProtectsEveryRequest(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values []string
	}{{"missing", nil}, {"non bearer", []string{"Basic token"}}, {"duplicate", []string{"Bearer client-token", "Bearer client-token"}}, {"comma list", []string{"Bearer client-token, Bearer client-token"}}} {
		t.Run(tc.name, func(t *testing.T) {
			handler := service.NewInboundHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("unauthenticated request reached MCP handler") }), tokenHash("client-token"))
			for _, method := range []string{http.MethodPost, http.MethodGet, http.MethodDelete} {
				req := httptest.NewRequest(method, "/mcp", nil)
				for _, value := range tc.values {
					req.Header.Add("Authorization", value)
				}
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, req)
				if response.Code != http.StatusUnauthorized {
					t.Fatalf("method=%s status=%d, want 401", method, response.Code)
				}
				if response.Header().Get("WWW-Authenticate") != "Bearer" {
					t.Fatalf("method=%s challenge=%q", method, response.Header().Get("WWW-Authenticate"))
				}
			}
		})
	}
}

func TestAuthenticationCoversDiscoverAndInitialize(t *testing.T) {
	server := httptest.NewServer(service.NewInboundHandler(service.NewStreamableHandler(service.NewMCPServer()), tokenHash("token")))
	defer server.Close()
	probes := []struct {
		name, body, methodHeader string
	}{
		{"discover", `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"wire-client","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}}}`, "server/discover"},
		{"initialize", `{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"2026-07-28","capabilities":{},"clientInfo":{"name":"wire-client","version":"1"}}}`, ""},
	}
	for _, probe := range probes {
		t.Run(probe.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, server.URL+"/mcp", bytes.NewBufferString(probe.body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			if probe.methodHeader != "" {
				req.Header.Set("Mcp-Method", probe.methodHeader)
			}
			req.Header.Set("Mcp-Protocol-Version", "2026-07-28")
			response, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("unauthenticated %s status=%d", probe.name, response.StatusCode)
			}
			if response.Header.Get("WWW-Authenticate") != "Bearer" {
				t.Fatalf("challenge=%q", response.Header.Get("WWW-Authenticate"))
			}
		})
	}
}

func TestSessionBoundToTokenFingerprint(t *testing.T) {
	server := httptest.NewServer(service.NewInboundHandler(service.NewStreamableHandler(service.NewMCPServer()), tokenHash("token-one", "token-two")))
	defer server.Close()
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"wire-client","version":"1"}}}`
	req, err := http.NewRequest(http.MethodPost, server.URL+"/mcp", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer token-one")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("initialize status=%d", response.StatusCode)
	}
	sessionID := response.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("initialize omitted session ID")
	}
	notify := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	for _, tc := range []struct {
		token string
		want  int
	}{{"token-two", http.StatusNotFound}, {"token-one", http.StatusAccepted}} {
		req, err := http.NewRequest(http.MethodPost, server.URL+"/mcp", bytes.NewBufferString(notify))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+tc.token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Mcp-Protocol-Version", "2025-11-25")
		req.Header.Set("Mcp-Session-Id", sessionID)
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != tc.want {
			t.Fatalf("token=%q status=%d, want %d", tc.token, response.StatusCode, tc.want)
		}
	}
}

func TestInboundHandlerLogsAndCountsWithoutLoggingCredentials(t *testing.T) {
	var logs bytes.Buffer
	metrics := observe.NewMetrics([]string{"alpha"})
	handler := service.NewInboundHandler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Request-ID") != "0123456789abcdef0123456789abcdef" {
				t.Errorf("request id was not validated and forwarded")
			}
			w.WriteHeader(http.StatusNoContent)
		}),
		tokenHash("sensitive-token"),
		service.InboundOptions{Logger: observe.NewLogger(&logs), Metrics: metrics},
	)
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer sensitive-token")
	req.Header.Set("X-Request-ID", "0123456789abcdef0123456789abcdef")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Header().Get("X-Request-ID") != req.Header.Get("X-Request-ID") {
		t.Fatal("validated request ID was not returned")
	}
	if strings.Contains(logs.String(), "sensitive-token") {
		t.Fatal("request log leaked bearer credential")
	}
	metricsResponse := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(metricsResponse, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(metricsResponse.Body.String(), `bridge_requests_total{operation="request",outcome="success",server="_bridge"} 1`) {
		t.Fatalf("inbound request metric missing: %s", metricsResponse.Body.String())
	}
}

func TestInboundResponseWriterPreservesStreamingFlush(t *testing.T) {
	handler := service.NewInboundHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("inbound wrapper hid streaming Flush support")
			return
		}
		flusher.Flush()
	}), tokenHash("token"))
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer token")
	handler.ServeHTTP(httptest.NewRecorder(), req)
}

func TestInboundHandlerContextClaimsBypass(t *testing.T) {
	// claimsMiddleware stands in for AuthMiddleware: it injects validated
	// claims into the request context before the inbound edge sees the request.
	claimsMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(auth.ContextWithClaims(r.Context(), &auth.Claims{Subject: "oidc-user", Issuer: "https://issuer.example"}))
			next.ServeHTTP(w, r)
		})
	}

	t.Run("admits when static tokens are empty", func(t *testing.T) {
		reached := false
		handler := claimsMiddleware(service.NewInboundHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			reached = true
			w.WriteHeader(http.StatusNoContent)
		}), nil))
		req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
		req.Header.Set("Authorization", "Bearer token-not-in-allowlist")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != http.StatusNoContent || !reached {
			t.Fatalf("context-authenticated status=%d reached=%v", response.Code, reached)
		}
	})

	t.Run("admits when bearer token does not match static tokens", func(t *testing.T) {
		reached := false
		handler := claimsMiddleware(service.NewInboundHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			reached = true
			w.WriteHeader(http.StatusNoContent)
		}), tokenHash("published-token")))
		req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
		req.Header.Set("Authorization", "Bearer token-not-in-allowlist")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != http.StatusNoContent || !reached {
			t.Fatalf("context-authenticated status=%d reached=%v", response.Code, reached)
		}
	})

	t.Run("still requires a well-formed bearer header", func(t *testing.T) {
		reached := false
		handler := claimsMiddleware(service.NewInboundHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			reached = true
		}), nil))
		req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("missing bearer header status=%d, want 401", response.Code)
		}
		if response.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Fatalf("challenge=%q, want Bearer", response.Header().Get("WWW-Authenticate"))
		}
		if reached {
			t.Fatal("context claims bypassed bearer header requirement")
		}
	})

	t.Run("creates and validates sessions from the context-authenticated token", func(t *testing.T) {
		inner := service.NewInboundHandler(service.NewStreamableHandler(service.NewMCPServer()), nil)
		server := httptest.NewServer(claimsMiddleware(inner))
		defer server.Close()
		post := func(client *http.Client, url, token, sessionID string) (*http.Response, error) {
			req, err := http.NewRequest(http.MethodPost, url+"/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			req.Header.Set("Mcp-Protocol-Version", "2025-11-25")
			if sessionID != "" {
				req.Header.Set("Mcp-Session-Id", sessionID)
			}
			return client.Do(req)
		}
		client := &http.Client{}

		// initialize creates the session bound to the bearer token fingerprint
		// that accompanied the context-authenticated request.
		initReq, err := http.NewRequest(http.MethodPost, server.URL+"/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"wire-client","version":"1"}}}`))
		if err != nil {
			t.Fatal(err)
		}
		initReq.Header.Set("Authorization", "Bearer context-token")
		initReq.Header.Set("Content-Type", "application/json")
		initReq.Header.Set("Accept", "application/json, text/event-stream")
		response, err := client.Do(initReq)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("initialize status=%d", response.StatusCode)
		}
		sessionID := response.Header.Get("Mcp-Session-Id")
		if sessionID == "" {
			t.Fatal("initialize omitted session ID")
		}

		// A follow-up request carrying the same context claims and the same
		// bearer token fingerprint is admitted through the session.
		response, err = post(client, server.URL, "context-token", sessionID)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		if response.StatusCode != http.StatusAccepted {
			t.Fatalf("session follow-up status=%d, want %d", response.StatusCode, http.StatusAccepted)
		}

		// A request without context claims is still subject to the static
		// token check, and no static token is published here, so even the
		// session-owning bearer token is rejected on that path.
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
		req.Header.Set("Authorization", "Bearer context-token")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Mcp-Protocol-Version", "2025-11-25")
		req.Header.Set("Mcp-Session-Id", sessionID)
		recorder := httptest.NewRecorder()
		inner.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("non-claims request status=%d, want 401", recorder.Code)
		}

		// With context claims restored, a different bearer token does not
		// reopen the session: the session stays bound to its fingerprint.
		response, err = post(client, server.URL, "other-token", sessionID)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("token mismatch status=%d, want 404", response.StatusCode)
		}
	})
}

func tokenHash(values ...string) [][32]byte {
	hashes := make([][32]byte, 0, len(values))
	for _, value := range values {
		hashes = append(hashes, sha256.Sum256([]byte(value)))
	}
	return hashes
}
func TestInboundRehydratesAuthenticatedUnknownSession(t *testing.T) {
	t.Run("valid OAuth token", func(t *testing.T) {
		authMiddleware, token := inboundOAuthTestMiddleware(t)
		first := newInboundHTTPServer(t, authMiddleware(service.NewInboundHandler(
			service.NewStreamableHandler(service.NewMCPServer()), nil,
		)))
		sessionID := initializeInboundTestSession(t, first, token)

		sink := &bytes.Buffer{}
		diag := observe.NewHTTPDiagnostics(observe.NewLogger(sink), observe.HTTPDiagnosticsOptions{
			NewID: func() string { return "0123456789abcdef0123456789abcdef" },
		})
		sdkSessions := &inboundSDKSessionRecorder{next: service.NewStreamableHandler(service.NewMCPServer()), initialized: make(chan string, 2)}
		restarted := service.NewInboundHandler(sdkSessions, nil, service.InboundOptions{Diagnostics: diag})
		second := newInboundHTTPServer(t, diag.Wrap(authMiddleware(restarted)))
		response := requestInboundTest(t, second, http.MethodPost, token, sessionID, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
		requireMCPResult(t, response, "1")
		sdkID := <-sdkSessions.initialized
		if sdkID == "" || sdkID == sessionID || response.Header.Get("Mcp-Session-Id") != "" {
			t.Fatalf("rehydrated client ID %q, SDK ID %q, response ID header %q", sessionID, sdkID, response.Header.Get("Mcp-Session-Id"))
		}
		secondResponse := requestInboundTest(t, second, http.MethodPost, token, sessionID, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
		requireMCPResult(t, secondResponse, "2")
		if secondResponse.Header.Get("Mcp-Session-Id") != "" {
			t.Fatalf("second request rewrote the client-visible session ID: %q", secondResponse.Header.Get("Mcp-Session-Id"))
		}
		select {
		case repeatedSDKID := <-sdkSessions.initialized:
			t.Fatalf("second request bootstrapped another SDK session %q", repeatedSDKID)
		default:
		}
		requireRehydratedDiagnostic(t, sink, "oauth_protected")
	})

	t.Run("inbound_direct route", func(t *testing.T) {
		first := newInboundHTTPServer(t, service.NewInboundHandler(
			service.NewStreamableHandler(service.NewMCPServer()), tokenHash("client-token"),
		))
		sessionID := initializeInboundTestSession(t, first, "client-token")

		sink := &bytes.Buffer{}
		diag := observe.NewHTTPDiagnostics(observe.NewLogger(sink), observe.HTTPDiagnosticsOptions{
			NewID: func() string { return "0123456789abcdef0123456789abcdef" },
		})
		restarted := service.NewInboundHandler(
			service.NewStreamableHandler(service.NewMCPServer()), tokenHash("client-token"),
			service.InboundOptions{Diagnostics: diag},
		)
		second := newInboundHTTPServer(t, diag.Wrap(restarted))
		response := requestInboundTest(t, second, http.MethodPost, "client-token", sessionID, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
		requireMCPResult(t, response, "1")
		requireRehydratedDiagnostic(t, sink, "inbound_direct")
	})
}

func TestInboundRehydratesInitializeHeaderPreservation(t *testing.T) {
	first := newInboundHTTPServer(t, service.NewInboundHandler(
		service.NewStreamableHandler(service.NewMCPServer()), tokenHash("client-token"),
	))
	clientID := initializeInboundTestSession(t, first, "client-token")

	sdkSessions := &inboundSDKSessionRecorder{
		next:        service.NewStreamableHandler(service.NewMCPServer()),
		initialized: make(chan string, 2),
	}
	restarted := newInboundHTTPServer(t, service.NewInboundHandler(sdkSessions, tokenHash("client-token")))
	initialize := requestInboundTest(t, restarted, http.MethodPost, "client-token", clientID,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"inbound-test","version":"1"}}}`)
	requireMCPResult(t, initialize, "1")
	if got := initialize.Header.Get("Mcp-Session-Id"); got != clientID {
		t.Fatalf("initialize response session ID = %q, want client ID %q", got, clientID)
	}

	sdkID := <-sdkSessions.initialized
	if sdkID == "" {
		t.Fatalf("rehydrated client ID %q, SDK-issued ID %q", clientID, sdkID)
	}
	list := requestInboundTest(t, restarted, http.MethodPost, "client-token", clientID,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	requireMCPResult(t, list, "2")
	if list.Header.Get("Mcp-Session-Id") != "" {
		t.Fatalf("tools/list rewrote the client-visible session ID: %q", list.Header.Get("Mcp-Session-Id"))
	}
	select {
	case extraID := <-sdkSessions.initialized:
		t.Fatalf("initialize created an unexpected extra SDK session %q", extraID)
	default:
	}
}

func TestInboundRehydratesLargeInitialize(t *testing.T) {
	first := newInboundHTTPServer(t, service.NewInboundHandler(
		service.NewStreamableHandler(service.NewMCPServer()), tokenHash("client-token"),
	))
	clientID := initializeInboundTestSession(t, first, "client-token")

	restarted := newInboundHTTPServer(t, service.NewInboundHandler(
		service.NewStreamableHandler(service.NewMCPServer()), tokenHash("client-token"),
	))
	body := `{"jsonrpc":"2.0","id":99,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"` +
		strings.Repeat("x", 8192) + `","version":"1"}}}`
	initialize := requestInboundTest(t, restarted, http.MethodPost, "client-token", clientID, body)
	requireMCPResult(t, initialize, "99")
	list := requestInboundTest(t, restarted, http.MethodPost, "client-token", clientID,
		`{"jsonrpc":"2.0","id":100,"method":"tools/list","params":{}}`)
	requireMCPResult(t, list, "100")
}

func TestInboundSyntheticBootstrapFailureRollsBack(t *testing.T) {
	var initializeCalls, deleteCalls, forwardedCalls int
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalls++
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == http.MethodPost && r.Header.Get("Mcp-Session-Id") == "" {
			initializeCalls++
			w.Header().Set("Mcp-Session-Id", "sdk-failed")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"rehydrate","error":{"code":-32603,"message":"failed"}}`))
			return
		}
		forwardedCalls++
		w.WriteHeader(http.StatusNotFound)
	})
	clientID := "client-session-after-restart"
	server := newInboundHTTPServer(t, service.NewInboundHandler(next, tokenHash("client-token")))
	for _, requestID := range []string{"1", "2"} {
		body := `{"jsonrpc":"2.0","id":` + requestID + `,"method":"tools/list","params":{}}`
		response := requestInboundTest(t, server, http.MethodPost, "client-token", clientID, body)
		if response.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("bootstrap failure status=%d, want 503", response.StatusCode)
		}
	}
	if initializeCalls != 2 || deleteCalls != 2 || forwardedCalls != 0 {
		t.Fatalf("bootstrap calls=%d SDK deletes=%d forwarded requests=%d; want 2, 2, 0",
			initializeCalls, deleteCalls, forwardedCalls)
	}
}

func TestInboundChunkedOversizedUnknownSessionRejectedBeforeDispatch(t *testing.T) {
	dispatches := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dispatches++
		w.WriteHeader(http.StatusOK)
	})
	handler := service.NewInboundHandler(next, tokenHash("client-token"), service.InboundOptions{
		Bound: func() service.InboundBounds {
			return service.InboundBounds{MaxRequestBytes: 8}
		},
	})
	server := newInboundHTTPServer(t, handler)
	request, err := http.NewRequest(http.MethodPost, server.URL+"/mcp", strings.NewReader(strings.Repeat("x", 32)))
	if err != nil {
		t.Fatalf("build chunked request: %v", err)
	}
	request.ContentLength = -1
	request.Header.Set("Authorization", "Bearer client-token")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Mcp-Session-Id", "unknown-after-restart")
	request.Header.Set("Mcp-Protocol-Version", "2025-06-18")
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatalf("send chunked request: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("chunked oversized request status=%d, want 413", response.StatusCode)
	}
	if dispatches != 0 {
		t.Fatalf("oversized request reached SDK %d times, want zero", dispatches)
	}
}

func TestInboundRehydratesAfterMappedSDKSessionDisappears(t *testing.T) {
	first := newInboundHTTPServer(t, service.NewInboundHandler(
		service.NewStreamableHandler(service.NewMCPServer()), tokenHash("client-token"),
	))
	clientID := initializeInboundTestSession(t, first, "client-token")

	sink := &bytes.Buffer{}
	diag := observe.NewHTTPDiagnostics(observe.NewLogger(sink), observe.HTTPDiagnosticsOptions{
		NewID: func() string { return "0123456789abcdef0123456789abcdef" },
	})
	sdkSessions := &inboundSDKSessionRecorder{
		next:        service.NewStreamableHandler(service.NewMCPServer()),
		initialized: make(chan string, 3),
	}
	restarted := service.NewInboundHandler(sdkSessions, tokenHash("client-token"), service.InboundOptions{Diagnostics: diag})
	server := newInboundHTTPServer(t, diag.Wrap(restarted))

	firstList := requestInboundTest(t, server, http.MethodPost, "client-token", clientID,
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	requireMCPResult(t, firstList, "1")
	firstSDKID := <-sdkSessions.initialized

	remove := httptest.NewRequest(http.MethodDelete, "/mcp", nil)
	remove.Header.Set("Mcp-Session-Id", firstSDKID)
	remove.Header.Set("Mcp-Protocol-Version", "2025-06-18")
	removed := httptest.NewRecorder()
	sdkSessions.next.ServeHTTP(removed, remove)
	if removed.Code >= http.StatusBadRequest {
		t.Fatalf("SDK session DELETE status=%d", removed.Code)
	}

	missing := requestInboundTest(t, server, http.MethodPost, "client-token", clientID,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("missing SDK session status=%d, want 404", missing.StatusCode)
	}
	recovered := requestInboundTest(t, server, http.MethodPost, "client-token", clientID,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`)
	requireMCPResult(t, recovered, "3")
	secondSDKID := <-sdkSessions.initialized
	if secondSDKID == "" || secondSDKID == firstSDKID {
		t.Fatalf("SDK session was not replaced after disappearance: first=%q second=%q", firstSDKID, secondSDKID)
	}

	records := strings.Split(strings.TrimSpace(sink.String()), "\n")
	if len(records) == 0 || !strings.Contains(records[len(records)-1], `"session_state":"rehydrated"`) ||
		!strings.Contains(records[len(records)-1], `"failure_stage":"none"`) {
		t.Fatalf("recovered request diagnostics did not report successful rehydration: %s", sink.String())
	}
}

func TestInboundDoesNotRehydrateBeforeOAuthAuthentication(t *testing.T) {
	authMiddleware, _ := inboundOAuthTestMiddleware(t)
	for _, authorization := range []string{"", "Bearer invalid"} {
		t.Run(map[bool]string{true: "missing", false: "invalid"}[authorization == ""], func(t *testing.T) {
			forwarded := false
			inbound := service.NewInboundHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				forwarded = true
			}), nil)
			handler := authMiddleware(inbound)
			request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
			request.Header.Set("Mcp-Session-Id", "pre-restart-session")
			if authorization != "" {
				request.Header.Set("Authorization", authorization)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized || forwarded {
				t.Fatalf("status=%d forwarded=%v, want 401 without forwarding", response.Code, forwarded)
			}
		})
	}
}

func TestInboundUnknownSessionDeleteIsIdempotent(t *testing.T) {
	forwarded := false
	next := service.NewStreamableHandler(service.NewMCPServer())
	inbound := service.NewInboundHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwarded = true
		next.ServeHTTP(w, r)
	}), tokenHash("client-token"))
	server := newInboundHTTPServer(t, inbound)

	response := requestInboundTest(t, server, http.MethodDelete, "client-token", "pre-restart-session", "")
	if response.StatusCode != http.StatusNoContent || forwarded {
		t.Fatalf("status=%d forwarded=%v, want 204 without forwarding", response.StatusCode, forwarded)
	}
}

func TestInboundRehydratedSessionRemainsBearerBound(t *testing.T) {
	first := newInboundHTTPServer(t, service.NewInboundHandler(
		service.NewStreamableHandler(service.NewMCPServer()), tokenHash("token-one", "token-two"),
	))
	sessionID := initializeInboundTestSession(t, first, "token-one")

	sink := &bytes.Buffer{}
	diag := observe.NewHTTPDiagnostics(observe.NewLogger(sink), observe.HTTPDiagnosticsOptions{
		NewID: func() string { return "0123456789abcdef0123456789abcdef" },
	})
	forwarded := 0
	next := service.NewStreamableHandler(service.NewMCPServer())
	inbound := service.NewInboundHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwarded++
		next.ServeHTTP(w, r)
	}), tokenHash("token-one", "token-two"), service.InboundOptions{Diagnostics: diag})
	restarted := newInboundHTTPServer(t, diag.Wrap(inbound))

	firstBearer := requestInboundTest(t, restarted, http.MethodPost, "token-one", sessionID, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	requireMCPResult(t, firstBearer, "1")
	requireRehydratedDiagnostic(t, sink, "inbound_direct")
	forwardedBeforeMismatch := forwarded
	secondBearer := requestInboundTest(t, restarted, http.MethodPost, "token-two", sessionID, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	if secondBearer.StatusCode != http.StatusNotFound || forwarded != forwardedBeforeMismatch {
		t.Fatalf("second bearer status=%d forwarded=%d, want 404 without forwarding", secondBearer.StatusCode, forwarded)
	}
	if !strings.Contains(sink.String(), `"failure_reason":"session_fingerprint_mismatch"`) {
		t.Fatal("second bearer was not diagnosed as a session fingerprint mismatch")
	}
}

func TestInboundRehydrationHonorsSessionCapacity(t *testing.T) {
	handler := service.NewInboundHandler(
		service.NewStreamableHandler(service.NewMCPServer()), tokenHash("client-token"),
		service.InboundOptions{MaxSessions: 1},
	)
	server := newInboundHTTPServer(t, handler)
	sessionID := initializeInboundTestSession(t, server, "client-token")
	atCapacity := requestInboundTest(t, server, http.MethodPost, "client-token", "another-session", `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	if atCapacity.StatusCode != http.StatusServiceUnavailable || sessionID == "" {
		t.Fatalf("at-capacity status=%d initialized session=%q, want 503 with one live session", atCapacity.StatusCode, sessionID)
	}
}

type inboundTestResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

func newInboundHTTPServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

type inboundSDKSessionRecorder struct {
	next        http.Handler
	initialized chan string
}

func (h *inboundSDKSessionRecorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bootstrap := r.Method == http.MethodPost && r.Header.Get("Mcp-Session-Id") == ""
	h.next.ServeHTTP(w, r)
	if bootstrap {
		h.initialized <- w.Header().Get("Mcp-Session-Id")
	}
}

func initializeInboundTestSession(t *testing.T, server *httptest.Server, token string) string {
	t.Helper()
	response := requestInboundTest(t, server, http.MethodPost, token, "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"inbound-test","version":"1"}}}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("initialize status=%d body=%s", response.StatusCode, response.Body)
	}
	sessionID := response.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("initialize response did not include Mcp-Session-Id")
	}
	initialized := requestInboundTest(t, server, http.MethodPost, token, sessionID, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if initialized.StatusCode != http.StatusAccepted {
		t.Fatalf("initialized notification status=%d body=%s", initialized.StatusCode, initialized.Body)
	}
	return sessionID
}

func requestInboundTest(t *testing.T, server *httptest.Server, method, token, sessionID, body string) inboundTestResponse {
	t.Helper()
	request, err := http.NewRequest(method, server.URL+"/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build MCP request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if sessionID != "" {
		request.Header.Set("Mcp-Session-Id", sessionID)
		request.Header.Set("Mcp-Protocol-Version", "2025-06-18")
	}
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatalf("send MCP request: %v", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read MCP response: %v", err)
	}
	return inboundTestResponse{StatusCode: response.StatusCode, Header: response.Header.Clone(), Body: responseBody}
}

func requireMCPResult(t *testing.T, response inboundTestResponse, requestID string) {
	t.Helper()
	body := string(response.Body)
	if response.StatusCode != http.StatusOK || !strings.Contains(body, `"id":`+requestID) ||
		!strings.Contains(body, `"result"`) || strings.Contains(body, `"error"`) {
		t.Fatalf("MCP response status=%d body=%s, want successful result for request ID %s", response.StatusCode, response.Body, requestID)
	}
}

func requireRehydratedDiagnostic(t *testing.T, sink *bytes.Buffer, route string) {
	t.Helper()
	record := sink.String()
	for _, field := range []string{
		`"session_state":"rehydrated"`,
		`"binding_relation":"not_evaluated"`,
		`"failure_stage":"none"`,
		`"failure_reason":"none"`,
		`"route_outcome":"` + route + `"`,
	} {
		if !strings.Contains(record, field) {
			t.Errorf("diagnostic record does not contain %s: %s", field, record)
		}
	}
	if !strings.Contains(record, `"session_ref_state":"available"`) || !strings.Contains(record, `"session_ref":"`) {
		t.Errorf("rehydrated diagnostic omitted session-reference evidence: %s", record)
	}
}

func inboundOAuthTestMiddleware(t *testing.T) (func(http.Handler) http.Handler, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate OAuth signing key: %v", err)
	}
	public, err := jwk.FromRaw(&key.PublicKey)
	if err != nil {
		t.Fatalf("create OAuth public JWK: %v", err)
	}
	if err := public.Set(jwk.KeyIDKey, "inbound-test-key"); err != nil {
		t.Fatalf("set OAuth key ID: %v", err)
	}
	if err := public.Set(jwk.AlgorithmKey, jwa.RS256); err != nil {
		t.Fatalf("set OAuth key algorithm: %v", err)
	}
	keys := jwk.NewSet()
	if err := keys.AddKey(public); err != nil {
		t.Fatalf("add OAuth public key: %v", err)
	}
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	token := jwt.New()
	for name, value := range map[string]any{
		jwt.SubjectKey:    "inbound-test-user",
		jwt.IssuerKey:     "https://issuer.test",
		jwt.AudienceKey:   []string{"https://resource.test"},
		jwt.IssuedAtKey:   now,
		jwt.ExpirationKey: now.Add(time.Hour),
		"scope":           "mcp:connect",
	} {
		if err := token.Set(name, value); err != nil {
			t.Fatalf("set OAuth claim %q: %v", name, err)
		}
	}
	signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256, key))
	if err != nil {
		t.Fatalf("sign OAuth token: %v", err)
	}
	config := auth.Config{
		PocketIDIssuer:   "https://issuer.test",
		MCPResourceURL:   "https://resource.test",
		MCPRequiredScope: "mcp:connect",
	}
	return auth.NewAuthMiddleware(config, auth.StaticKeySet{Set: keys}, auth.WithNow(func() time.Time { return now })), string(signed)
}
