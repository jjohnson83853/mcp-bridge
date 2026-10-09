package service_test

import (
	"bytes"
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func tokenHash(values ...string) [][32]byte {
	hashes := make([][32]byte, 0, len(values))
	for _, value := range values {
		hashes = append(hashes, sha256.Sum256([]byte(value)))
	}
	return hashes
}
