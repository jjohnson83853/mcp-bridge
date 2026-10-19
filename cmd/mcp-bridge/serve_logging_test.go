package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestServeLoggingHTTP(t *testing.T) {
	t.Run("ClientAuthentication", func(t *testing.T) {
		ctx, httpURL, stdout, stop := startHTTPLoggingBridge(t)

		const badToken = "synthetic-rejected-client-secret"
		const publicID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, httpURL+"/mcp", strings.NewReader(`{}`))
		if err != nil {
			t.Fatalf("build authentication request: %v", err)
		}
		request.Header.Set("Authorization", "Bearer "+badToken)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Request-ID", publicID)
		client := &http.Client{Timeout: 5 * time.Second}
		response, err := client.Do(request)
		if err != nil {
			t.Fatalf("authentication request: %v", err)
		}
		_, readErr := io.Copy(io.Discard, response.Body)
		response.Body.Close()
		if readErr != nil {
			t.Fatalf("read authentication response: %v", readErr)
		}
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("bad credentials status = %d; want 401", response.StatusCode)
		}
		if got := response.Header.Get("WWW-Authenticate"); got != "Bearer" {
			t.Fatalf("authentication challenge = %q; want Bearer", got)
		}
		if got := response.Header.Get("X-Request-ID"); got != publicID {
			t.Fatalf("public X-Request-ID = %q; want %q", got, publicID)
		}
		// Drain the real runtime before inspecting completion records, avoiding
		// a race between response delivery and the completion logger.
		stop()
		captured := stdout.String()
		for _, forbidden := range []string{badToken, publicID} {
			if strings.Contains(captured, forbidden) {
				t.Error("captured logs contain a client-controlled secret or public request ID")
			}
		}
		var completions []map[string]any
		decoder := json.NewDecoder(strings.NewReader(captured))
		for {
			var record map[string]any
			err := decoder.Decode(&record)
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("decode captured JSON log: %v", err)
			}
			if record["event"] == "http_complete" {
				completions = append(completions, record)
			}
		}
		// Only one public HTTP request was sent; operations probes and startup
		// catalog activity must not be counted as public completions.
		if len(completions) != 1 {
			t.Fatalf("http_complete count = %d; want exactly 1 for rejected POST /mcp", len(completions))
		}
		record := completions[0]
		for key, want := range map[string]any{
			"http_status": float64(401), "http_method": "POST", "route": "mcp",
			"reason": "client_auth_failed", "stage": "client_auth", "http_outcome": "failure",
		} {
			if got := record[key]; got != want {
				t.Errorf("http_complete %s = %v; want %v", key, got, want)
			}
		}
		internalID, ok := record["request_id"].(string)
		if !ok || !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(internalID) || internalID == publicID {
			t.Error("http_complete request_id must be a fresh internal 32-character lowercase hexadecimal identity")
		}
		for _, key := range []string{"backend_call_id", "operation_id", "tool", "server", "tool_outcome", "backend_outcome"} {
			if _, exists := record[key]; exists {
				t.Errorf("http_complete contains %s despite rejection before MCP execution", key)
			}
		}
		assertHTTPLoggingTerminal(t, captured, http.StatusUnauthorized, "POST", "mcp", "client_auth", "client_auth_failed", publicID, true)
	})

	for _, tc := range []struct {
		name, host, origin, method, path, body, route, stage, reason, session string
		status                                                   int
	}{
		{name: "HostAdmission", host: "host-secret.invalid", method: "POST", path: "/mcp", body: `{}`, route: "mcp", stage: "host_admission", reason: "host_rejected", status: 403},
		{name: "OriginAdmission", origin: "https://origin-secret.invalid", method: "POST", path: "/mcp", body: `{}`, route: "mcp", stage: "origin_admission", reason: "origin_rejected", status: 403},
		{name: "HostPrecedesOrigin", host: "host-secret.invalid", origin: "https://origin-secret.invalid", method: "POST", path: "/mcp", body: `{}`, route: "mcp", stage: "host_admission", reason: "host_rejected", status: 403},
		{name: "UnmatchedUnusualMethod", method: "SECRETVERB", path: "/private-path-secret?token=query-secret", body: `{}`, route: "not_found", stage: "http_route", reason: "route_not_found", status: 404},
		{name: "Initialize", method: "POST", path: "/mcp", body: `{"jsonrpc":"2.0","id":"rpc-secret","method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"client-secret","version":"1"}}}`, route: "mcp", stage: "none", reason: "none", status: 200},
		{name: "OversizedBody", method: "POST", path: "/mcp", body: strings.Repeat("body-secret", 104858), route: "mcp", stage: "request_limits", reason: "request_too_large", status: 413},
		{name: "SessionBinding", method: "POST", path: "/mcp", body: `{}`, route: "mcp", stage: "session_admission", reason: "session_auth_failed", session: "unbound-session-secret", status: 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, httpURL, stdout, stop := startHTTPLoggingBridge(t)
			const publicID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			request, err := http.NewRequestWithContext(ctx, tc.method, httpURL+tc.path, strings.NewReader(tc.body))
			if err != nil {
				t.Fatalf("build matrix request: %v", err)
			}
			request.Header.Set("Authorization", "Bearer "+serveClientToken)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept", "application/json, text/event-stream")
			request.Header.Set("X-Request-ID", publicID)
			if tc.session != "" {
				request.Header.Set("Mcp-Session-Id", tc.session)
			}
			if tc.host != "" {
				request.Host = tc.host
			}
			if tc.origin != "" {
				request.Header.Set("Origin", tc.origin)
			}
			response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
			if err != nil {
				t.Fatalf("matrix request: %v", err)
			}
			body, readErr := io.ReadAll(response.Body)
			response.Body.Close()
			if readErr != nil {
				t.Fatalf("read matrix response: %v", readErr)
			}
			if response.StatusCode != tc.status {
				t.Fatalf("public status = %d; want %d", response.StatusCode, tc.status)
			}
			wantChallenge := ""
			if tc.name == "SessionBinding" {
				wantChallenge = "Bearer"
			}
			if got := response.Header.Get("WWW-Authenticate"); got != wantChallenge {
				t.Fatalf("public challenge = %q; want %q", got, wantChallenge)
			}
			wantPublicID := publicID
			if tc.route == "not_found" {
				wantPublicID = "" // Preserve the existing unmatched-path wire contract.
			}
			if got := response.Header.Get("X-Request-ID"); got != wantPublicID {
				t.Fatalf("public X-Request-ID = %q; want %q", got, wantPublicID)
			}
			if len(body) == 0 || len(body) > 1024 {
				t.Fatalf("public fixture response bytes = %d; want 1..1024", len(body))
			}
			if tc.name == "Initialize" {
				if !strings.Contains(string(body), `"result"`) || !strings.Contains(string(body), `"rpc-secret"`) || strings.Contains(string(body), `"error"`) {
					t.Fatal("initialize did not return the matching successful JSON-RPC result")
				}
			}
			t.Logf("public prerequisites passed: status=%d challenge preserved; X-Request-ID preserved; response bytes=%d", response.StatusCode, len(body))
			stop()
			captured := stdout.String()
			for _, forbidden := range []string{publicID, serveClientToken, tc.session, "host-secret.invalid", "origin-secret.invalid", "private-path-secret", "query-secret", "SECRETVERB", "rpc-secret", "client-secret", "body-secret", response.Header.Get("Mcp-Session-Id")} {
				if forbidden != "" && strings.Contains(captured, forbidden) {
					t.Error("complete stdout contains a client/session-controlled value")
				}
			}
			method := tc.method
			if tc.name == "UnmatchedUnusualMethod" {
				method = "OTHER"
			}
			assertHTTPLoggingTerminal(t, captured, tc.status, method, tc.route, tc.stage, tc.reason, publicID, tc.status != 200)
		})
	}
	t.Run("SessionCapacity", testHTTPLoggingSessionCapacity)
}

func startHTTPLoggingBridge(t *testing.T) (context.Context, string, *syncWriter, func()) {
	t.Helper()
	return startHTTPLoggingBridgeWithCapacity(t, "")
}

func startHTTPLoggingBridgeWithCapacity(t *testing.T, capacity string) (context.Context, string, *syncWriter, func()) {
	t.Helper()
	stdout := &syncWriter{}
	ctx, httpURL, _, stop := startHTTPLoggingBridgeWithWriter(t, capacity, stdout)
	return ctx, httpURL, stdout, stop
}

func startHTTPLoggingBridgeWithWriter(t *testing.T, capacity string, stdout io.Writer) (context.Context, string, string, func()) {
	t.Helper()
	t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
	t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)
	upstream := fakeUpstream(t, "read", `{"type":"object","properties":{},"additionalProperties":true}`)
	embeddings := fakeEmbeddings(t)
	httpAddress, operationsAddress := freeAddress(t), freeAddress(t)
	// freeAddress releases its reservation; successive calls can return the
	// same port. Never ask the real root to bind both listeners to that port.
	for attempts := 0; operationsAddress == httpAddress && attempts < 8; attempts++ {
		operationsAddress = freeAddress(t)
	}
	if operationsAddress == httpAddress {
		t.Fatal("could not choose distinct public and operations addresses")
	}
	t.Logf("real-root listener addresses: public=%s operations=%s distinct=true", httpAddress, operationsAddress)
	directory := t.TempDir()
	configPath := filepath.Join(directory, "bridge.yaml")
	writeBridgeConfig(t, configPath, httpAddress, operationsAddress, filepath.Join(directory, "state"), embeddings.URL,
		[]bridgeServerSpec{{id: "alpha", endpoint: upstream.URL}})
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read fixture config: %v", err)
	}
	config = []byte(strings.Replace(string(config), "allowed_origins: []", `allowed_origins: ["https://allowed.example"]`, 1))
	if capacity != "" {
		config = []byte(strings.Replace(string(config), "max_sessions: 128", "max_sessions: "+capacity, 1))
	}
	if err := os.WriteFile(configPath, config, 0600); err != nil {
		t.Fatalf("configure fixture origin policy: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	stderr := &syncWriter{}
	done := make(chan int, 1)
	go func() { done <- runContext(ctx, []string{"--config", configPath}, stdout, stderr) }()
	stopped := false
	stop := func() {
		t.Helper()
		if stopped {
			return
		}
		cancel()
		select {
		case status := <-done:
			stopped = true
			if status != 0 {
				t.Fatalf("run status = %d; want 0 (stderr=%s)", status, stderr.String())
			}
		case <-time.After(30 * time.Second):
			t.Fatal("the process did not shut down after cancellation")
		}
	}
	t.Cleanup(stop)
	waitFor(t, "readiness of the configured server", func() bool {
		status, body, ok := probeBody(t, "http://"+operationsAddress+"/readyz")
		return ok && status == http.StatusOK && strings.Contains(body, `"status":"ready"`) && strings.Contains(body, "alpha")
	})
	return ctx, "http://" + httpAddress, "http://" + operationsAddress, stop
}

func assertHTTPLoggingTerminal(t *testing.T, captured string, status int, method, route, stage, reason, publicID string, rejected bool) {
	t.Helper()
	var completions []map[string]any
	decoder := json.NewDecoder(strings.NewReader(captured))
	for {
		var record map[string]any
		err := decoder.Decode(&record)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("decode complete stdout: %v", err)
		}
		if rejected && record["reason"] == "backend_auth_failed" {
			t.Error("client rejection falsely logged backend_auth_failed")
		}
		if record["event"] == "request" {
			t.Error("legacy generic HTTP request completion remains alongside terminal ownership")
		}
		if record["event"] == "catalog_refresh" {
			if record["operation"] != "discover" || record["reason"] == "unknown" {
				t.Error("legacy startup catalog diagnostic vocabulary was not preserved")
			}
		}
		if record["event"] == "http_complete" {
			completions = append(completions, record)
		}
	}
	if len(completions) != 1 {
		t.Errorf("http_complete count = %d; want exactly 1", len(completions))
		return
	}
	record := completions[0]
	outcome := "failure"
	if status == http.StatusOK {
		outcome = "success"
	}
	severity := "info"
	if rejected {
		severity = "warn"
	}
	for key, want := range map[string]any{
		"service": "mcp-bridge", "environment": "unspecified", "version": "unspecified", "level": severity,
		"correlation_state": "available", "http_status": float64(status),
		"http_method": method, "route": route, "http_outcome": outcome,
		"http_bytes_bucket": "le_1k", "timing_scope": "http", "stage": stage, "reason": reason,
	} {
		if got := record[key]; got != want {
			t.Errorf("http_complete %s = %v; want %v", key, got, want)
		}
	}
	timestamp, timestampOK := record["time"].(string)
	if _, err := time.Parse(time.RFC3339Nano, timestamp); !timestampOK || err != nil || !strings.HasSuffix(timestamp, "Z") {
		t.Error("http_complete time must be a UTC RFC3339Nano timestamp")
	}
	if message, ok := record["message"].(string); !ok || strings.TrimSpace(message) == "" {
		t.Error("http_complete must include a nonempty event-specific message")
	}
	if duration, ok := record["duration_ms"].(float64); !ok || duration < 0 {
		t.Error("http_complete duration_ms must be an observed nonnegative number")
	}
	internalID, ok := record["request_id"].(string)
	if !ok || !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(internalID) || internalID == publicID {
		t.Error("HTTP terminal identity must be independent internal lowercase hexadecimal")
	}
	for _, key := range []string{"backend_call_id", "operation_id", "tool", "server", "tool_outcome", "backend_outcome"} {
		if _, exists := record[key]; exists {
			t.Errorf("HTTP terminal contains unowned execution field %s", key)
		}
	}
}

func testHTTPLoggingSessionCapacity(t *testing.T) {
	// Exercise the existing configurable limit with two real live sessions,
	// rather than changing any production default or session implementation.
	ctx, httpURL, stdout, stop := startHTTPLoggingBridgeWithCapacity(t, "2")
	client := &http.Client{Timeout: 5 * time.Second}
	send := func(method, session, body, publicID string) (*http.Response, []byte) {
		t.Helper()
		request, err := http.NewRequestWithContext(ctx, method, httpURL+"/mcp", strings.NewReader(body))
		if err != nil {
			t.Fatalf("build session request: %v", err)
		}
		request.Header.Set("Authorization", "Bearer "+serveClientToken)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		if session != "" {
			request.Header.Set("Mcp-Session-Id", session)
			request.Header.Set("Mcp-Protocol-Version", "2025-11-25")
		}
		if publicID != "" {
			request.Header.Set("X-Request-ID", publicID)
		}
		response, err := client.Do(request)
		if err != nil {
			t.Fatalf("session HTTP request: %v", err)
		}
		content, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Fatalf("read session HTTP response: %v", err)
		}
		return response, content
	}
	var sessions []string
	closeSessions := func() {
		t.Helper()
		for len(sessions) > 0 {
			session := sessions[len(sessions)-1]
			sessions = sessions[:len(sessions)-1]
			response, _ := send(http.MethodDelete, session, "", "")
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				t.Errorf("close retained session status = %d; want successful deletion", response.StatusCode)
			}
		}
	}
	t.Cleanup(closeSessions)
	const initialize = `{"jsonrpc":"2.0","id":"capacity-rpc-secret","method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"capacity-client-secret","version":"1"}}}`
	for range 2 {
		response, body := send(http.MethodPost, "", initialize, "")
		if response.StatusCode != http.StatusOK || response.Header.Get("WWW-Authenticate") != "" {
			t.Fatalf("session acquisition status = %d; want 200 without challenge", response.StatusCode)
		}
		session := response.Header.Get("Mcp-Session-Id")
		if session == "" {
			t.Fatal("real initialize omitted session identity")
		}
		sessions = append(sessions, session)
		if !strings.Contains(string(body), `"result"`) || !strings.Contains(string(body), `"capacity-rpc-secret"`) || strings.Contains(string(body), `"error"`) {
			t.Fatal("session acquisition did not return the matching successful initialize result")
		}
	}
	retainedSessions := append([]string(nil), sessions...)
	const publicID = "cccccccccccccccccccccccccccccccc"
	response, body := send(http.MethodPost, "", initialize, publicID)
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("capacity public status = %d; want existing 503", response.StatusCode)
	}
	if response.Header.Get("WWW-Authenticate") != "" {
		t.Fatal("capacity rejection unexpectedly returned authentication challenge")
	}
	if response.Header.Get("X-Request-ID") != publicID {
		t.Fatal("capacity rejection did not preserve public X-Request-ID")
	}
	if string(body) != "session cap reached\n" {
		t.Fatalf("capacity response body = %q; want existing session-cap rejection", body)
	}
	t.Log("public prerequisites passed: two retained real sessions, then 503/session cap reached; no challenge; public ID preserved")
	closeSessions()
	stop() // Drain every admitted request and deferred log before selecting.
	captured := stdout.String()
	for _, forbidden := range append(retainedSessions, publicID, serveClientToken, "capacity-rpc-secret", "capacity-client-secret") {
		if strings.Contains(captured, forbidden) {
			t.Error("complete stdout contains a session/client-controlled value")
		}
	}
	// Only the target can return 503: setup proved successful initializes and
	// cleanup proved successful deletions. Select by observed status, never a
	// public/session ID or a response-EOF timing assumption.
	var selected strings.Builder
	encoder := json.NewEncoder(&selected)
	decoder := json.NewDecoder(strings.NewReader(captured))
	for {
		var record map[string]any
		err := decoder.Decode(&record)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("decode capacity capture: %v", err)
		}
		if record["event"] == "request" {
			t.Error("legacy generic HTTP completion remains in session setup/target/cleanup")
		}
		if record["reason"] == "backend_auth_failed" {
			t.Error("session-capacity flow falsely logged backend_auth_failed")
		}
		if record["event"] == "http_complete" && record["http_status"] == float64(http.StatusServiceUnavailable) {
			if err := encoder.Encode(record); err != nil {
				t.Fatalf("retain selected terminal: %v", err)
			}
		}
	}
	assertHTTPLoggingTerminal(t, selected.String(), http.StatusServiceUnavailable, "POST", "mcp", "session_admission", "session_capacity", publicID, true)
}
