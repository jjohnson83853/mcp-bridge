package main

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/core"
)

// TestServeLoggingExecution exercises the real composition root through a
// complete successful tools/call flow and asserts the three-boundary
// correlation chain required by the logging-observability spec (AC2, AC4,
// AC5, AC8): an internal http_complete, a decoded mcp_complete and the actual
// admitted backend_complete join on internally generated parents.
func TestServeLoggingExecution(t *testing.T) {
	t.Run("SuccessfulToolCall", func(t *testing.T) {
		ctx, httpURL, stdout, stop := startHTTPLoggingBridge(t)

		const publicID = "cccccccccccccccccccccccccccccccc"
		const syntheticClientSecret = "synthetic-client-execution-secret"
		const syntheticArgumentSecret = "synthetic-argument-execution-secret"

		client := &http.Client{Timeout: 5 * time.Second}
		send := func(body string, session string) (*http.Response, []byte) {
			t.Helper()
			request, err := http.NewRequestWithContext(ctx, http.MethodPost, httpURL+"/mcp", strings.NewReader(body))
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			request.Header.Set("Authorization", "Bearer "+serveClientToken)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept", "application/json, text/event-stream")
			request.Header.Set("X-Request-ID", publicID)
			if session != "" {
				request.Header.Set("Mcp-Session-Id", session)
				request.Header.Set("Mcp-Protocol-Version", "2025-11-25")
			}
			response, err := client.Do(request)
			if err != nil {
				t.Fatalf("send %s: %v", body, err)
			}
			content, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil {
				t.Fatalf("read response body: %v", err)
			}
			return response, content
		}

		// Phase 1: real initialize on the reused public session.
		const initializeBody = `{"jsonrpc":"2.0","id":"init-secret-id","method":"initialize",` +
			`"params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"client-secret","version":"1"}}}`
		initResponse, initBody := send(initializeBody, "")
		if initResponse.StatusCode != http.StatusOK {
			t.Fatalf("initialize status = %d; want 200; body %s", initResponse.StatusCode, string(initBody))
		}
		session := initResponse.Header.Get("Mcp-Session-Id")
		if session == "" {
			t.Fatal("initialize omitted session identity")
		}
		initResult := extractJSONRPCPayload(t, string(initBody))
		if initResult == nil {
			t.Fatalf("initialize response missing JSON-RPC result: %q", string(initBody))
		}

		// Phase 2: notifications/initialized handshake required before tools/call.
		send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`, session)

		// Phase 2b: search_tools to discover the published catalog revision.
		searchBody := `{"jsonrpc":"2.0","id":"search-id","method":"tools/call",` +
			`"params":{"name":"search_tools","arguments":{"query":"read"}}}`
		searchResp, searchRaw := send(searchBody, session)
		if searchResp.StatusCode != http.StatusOK {
			t.Fatalf("search_tools status = %d; body %s", searchResp.StatusCode, string(searchRaw))
		}
		searchEnvelope := extractJSONRPCPayload(t, string(searchRaw))
		searchResult, ok := searchEnvelope["result"].(map[string]any)
		if !ok {
			t.Fatalf("search_tools missing result object: %v", searchEnvelope)
		}
		searchStructured, ok := searchResult["structuredContent"].(map[string]any)
		if !ok {
			t.Fatalf("search_tools missing structuredContent: %v", searchResult)
		}
		catalogRev, _ := searchStructured["catalog_revision"].(string)
		if catalogRev == "" {
			t.Fatalf("search_tools missing catalog_revision in %v", searchStructured)
		}

		// Phase 3: real tools/call through bridge's call_tool with admitted public tool name
		// on the same session, with synthetic secrets that must never reach the log stream.
		argsJSON, _ := json.Marshal(map[string]any{
			"tool_name":        core.PublicName("alpha", "read"),
			"catalog_revision": catalogRev,
			"arguments": map[string]any{
				"client_id": syntheticClientSecret,
				"query":     syntheticArgumentSecret,
			},
		})
		callBody := `{"jsonrpc":"2.0","id":"call-secret-id","method":"tools/call",` +
			`"params":{"name":"call_tool","arguments":` + string(argsJSON) + `}}`
		response, body := send(callBody, session)

		// Public assertions: unchanged transport and JSON-RPC behavior.
		if response.StatusCode != http.StatusOK {
			t.Fatalf("public status = %d; want 200; body %s", response.StatusCode, string(body))
		}
		if got := response.Header.Get("X-Request-ID"); got != publicID {
			t.Fatalf("public X-Request-ID = %q; want %q", got, publicID)
		}
		envelope := extractJSONRPCPayload(t, string(body))
		if envelope == nil {
			t.Fatalf("public response missing JSON-RPC payload: %q", string(body))
		}
		if envelope["jsonrpc"] != "2.0" {
			t.Fatalf("public response jsonrpc = %v; want 2.0", envelope["jsonrpc"])
		}
		result, hasResult := envelope["result"]
		_, hasError := envelope["error"]
		if !hasResult || hasError {
			t.Fatalf("public response must carry a JSON-RPC success result; got %v", envelope)
		}
		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("tool result is not an object: %v", result)
		}
		content, ok := resultMap["content"].([]any)
		if !ok || len(content) == 0 {
			t.Fatalf("tool result missing content: %v", resultMap)
		}
		firstContent, ok := content[0].(map[string]any)
		if !ok || !strings.Contains(firstContent["text"].(string), "upstream") {
			t.Fatalf("tool result missing upstream echo: %v", resultMap)
		}

		// Drain the runtime before inspecting captured terminal records.
		stop()
		captured := stdout.String()

		// Synthetic secrets in submitted identities/arguments never appear.
		for _, forbidden := range []string{
			publicID, serveClientToken, syntheticClientSecret, syntheticArgumentSecret,
			"call-secret-id", session,
		} {
			if forbidden != "" && strings.Contains(captured, forbidden) {
				t.Errorf("captured stdout contains forbidden secret %q", forbidden)
			}
		}

		records := decodeCapturedRecords(t, captured)
		httpCompletions := filterRecords(records, "http_complete")
		mcpCompletions := filterRecords(records, "mcp_complete")
		backendCompletions := filterRecords(records, "backend_complete")

		// Boundary 1: the tools/call POST emitted an http_complete with an internal request id.
		if len(httpCompletions) == 0 {
			t.Fatal("no http_complete records emitted")
		}
		callHTTP := httpCompletions[len(httpCompletions)-1]
		httpRequestID := wantInternalHexID(t, callHTTP, "request_id")
		// Boundary 2: exactly one mcp_complete for the decoded tools/call operation,
		// joined to the HTTP parent.
		var callOps []map[string]any
		for _, rec := range mcpCompletions {
			if rec["mcp_method"] == "tools_call" {
				callOps = append(callOps, rec)
			}
		}
		if len(callOps) != 1 {
			t.Fatalf("mcp_complete count = %d; want exactly 1 for the decoded tools/call operation", len(callOps))
		}
		mcp := callOps[0]
		operationID := wantInternalHexID(t, mcp, "operation_id")
		if got := mcp["request_id"]; got != httpRequestID {
			t.Errorf("mcp_complete parent request_id = %v; want %v", got, httpRequestID)
		}
		if got := mcp["mcp_method"]; got != "tools_call" {
			t.Errorf("mcp_complete mcp_method = %v; want tools_call", got)
		}
		if got := mcp["mcp_message_kind"]; got != "request" {
			t.Errorf("mcp_complete mcp_message_kind = %v; want request", got)
		}
		if got := mcp["outcome"]; got != "success" {
			t.Errorf("mcp_complete outcome = %v; want success", got)
		}
		wantNonnegativeDuration(t, mcp, "mcp_complete")

		// Boundary 3: exactly one backend_complete for the admitted dispatch.
		if len(backendCompletions) != 1 {
			t.Fatalf("backend_complete count = %d; want exactly 1 for the admitted backend dispatch", len(backendCompletions))
		}
		backend := backendCompletions[0]
		wantInternalHexID(t, backend, "backend_call_id")
		if got := backend["operation_id"]; got != operationID {
			t.Errorf("backend_complete parent operation_id = %v; want %v", got, operationID)
		}
		if got := backend["request_id"]; got != httpRequestID {
			t.Errorf("backend_complete parent request_id = %v; want %v", got, httpRequestID)
		}
		if got := backend["server"]; got != "alpha" {
			t.Errorf("backend_complete server = %v; want alpha", got)
		}
		if got := backend["tool"]; got != core.PublicName("alpha", "read") {
			t.Errorf("backend_complete tool = %v; want %v", got, core.PublicName("alpha", "read"))
		}
		if got := backend["outcome"]; got != "success" {
			t.Errorf("backend_complete outcome = %v; want success", got)
		}
		wantNonnegativeDuration(t, backend, "backend_complete")
	})
}

// extractJSONRPCPayload finds the JSON-RPC object inside a response body that
// may be plain JSON or a Server-Sent Events stream.
func extractJSONRPCPayload(t *testing.T, body string) map[string]any {
	t.Helper()
	if strings.HasPrefix(strings.TrimSpace(body), "{") {
		var envelope map[string]any
		if err := json.Unmarshal([]byte(body), &envelope); err == nil {
			return envelope
		}
	}
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if data, ok := strings.CutPrefix(trimmed, "data:"); ok {
			var envelope map[string]any
			if err := json.Unmarshal([]byte(strings.TrimSpace(data)), &envelope); err == nil {
				return envelope
			}
		}
	}
	return nil
}

// decodeCapturedRecords parses the complete captured stdout into individual
// JSON log records, failing on any undecodable byte range.
func decodeCapturedRecords(t *testing.T, captured string) []map[string]any {
	t.Helper()
	var records []map[string]any
	decoder := json.NewDecoder(strings.NewReader(captured))
	for {
		var record map[string]any
		err := decoder.Decode(&record)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("decode captured JSON log record: %v", err)
		}
		records = append(records, record)
	}
	return records
}

func filterRecords(records []map[string]any, event string) []map[string]any {
	var filtered []map[string]any
	for _, record := range records {
		if record["event"] == event {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

var internalIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// wantInternalHexID asserts the record carries a fresh internal 32-character
// lowercase hexadecimal identity in the given field.
func wantInternalHexID(t *testing.T, record map[string]any, field string) string {
	t.Helper()
	value, ok := record[field].(string)
	if !ok || !internalIDPattern.MatchString(value) {
		t.Errorf("%s %s = %v; want a 32-character lowercase hexadecimal internal id", record["event"], field, record[field])
	}
	return value
}

func wantNonnegativeDuration(t *testing.T, record map[string]any, label string) {
	t.Helper()
	duration, ok := record["duration_ms"].(float64)
	if !ok || duration < 0 {
		t.Errorf("%s duration_ms = %v; want a nonnegative number", label, record["duration_ms"])
	}
}

// quotedName renders an admitted public tool name as a JSON string literal.
func quotedName(name string) string {
	encoded, err := json.Marshal(name)
	if err != nil {
		panic("marshal admitted tool name: " + err.Error())
	}
	return string(encoded)
}
