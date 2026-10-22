package service_test

import (
	"bytes"
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/auth"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
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
	}{{"token-two", http.StatusUnauthorized}, {"token-one", http.StatusAccepted}} {
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
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("token mismatch status=%d, want 401", response.StatusCode)
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
