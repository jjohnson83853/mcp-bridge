package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/core"
)

// TestServeLoggingFailures distinguishes the failure families of the
// logging-observability spec: tool-admission failures that never reach an
// upstream, and backend failures where the upstream itself reports an error
// inside mcp.CallToolResult. Each subtest drives the full session flow over
// the shared wire contract, then asserts the unchanged public JSON-RPC
// surface and the captured stdout terminal records.
func TestServeLoggingFailures(t *testing.T) {
	t.Run("ToolAdmissionFailure", func(t *testing.T) {
		ctx, httpURL, stdout, stop := startHTTPLoggingBridge(t)
		call := loggedFailureToolCall(t, ctx, httpURL, stdout, stop, func(rev string) map[string]any {
			return map[string]any{
				"tool_name":        core.PublicName("alpha", "nonexistent"),
				"catalog_revision": rev,
				"arguments":        map[string]any{},
			}
		})

		// Public assertions: valid JSON-RPC 2.0 envelope carrying the
		// tool-layer admission error.
		assertFailureEnvelope(t, call)
		result := failureResultObject(t, call.envelope)
		structured, ok := result["structuredContent"].(map[string]any)
		if !ok {
			t.Fatalf("public result missing structuredContent: %v", result)
		}
		if got := structured["code"]; got != "unknown_tool" {
			t.Errorf("structuredContent code = %v; want unknown_tool", got)
		}

		// Log assertions: the rejection terminates at admission and never
		// reaches an upstream.
		assertToolAdmissionLogs(t, call.captured)
	})

	t.Run("ToolReportedError", func(t *testing.T) {
		upstream := fakeUpstreamReportedError(t, "read",
			`{"type":"object","properties":{},"additionalProperties":true}`)
		ctx, httpURL, stdout, stop := startHTTPLoggingBridgeWithUpstream(t, upstream)
		call := loggedFailureToolCall(t, ctx, httpURL, stdout, stop, func(rev string) map[string]any {
			return map[string]any{
				"tool_name":        core.PublicName("alpha", "read"),
				"catalog_revision": rev,
				"arguments":        map[string]any{},
			}
		})

		// Public assertions: the upstream-reported tool failure stays inside
		// the JSON-RPC success envelope with isError: true.
		assertFailureEnvelope(t, call)
		failureResultObject(t, call.envelope)

		// Log assertions: both the backend boundary and the decoded
		// tools/call operation terminate with the tool_reported_error reason.
		assertToolReportedErrorLogs(t, call.captured)
	})
}

// assertFailureEnvelope validates the public JSON-RPC 2.0 transport surface
// shared by both failure families: HTTP 200, a 2.0 envelope echoing the
// request id, and a tool-layer error result instead of a JSON-RPC error
// object (bridge tool errors answer as IsError results).
func assertFailureEnvelope(t *testing.T, call loggedToolCall) {
	t.Helper()
	if call.status != http.StatusOK {
		t.Fatalf("public status = %d; want 200; body %s", call.status, call.envelope)
	}
	envelope := call.envelope
	if envelope == nil {
		t.Fatal("public response missing JSON-RPC payload")
	}
	if envelope["jsonrpc"] != "2.0" {
		t.Fatalf("public response jsonrpc = %v; want 2.0", envelope["jsonrpc"])
	}
	if got := envelope["id"]; got != "call-id" {
		t.Errorf("public response id = %v; want call-id", got)
	}
	if _, hasError := envelope["error"]; hasError {
		t.Fatalf("tool failure must surface as an IsError result, not a JSON-RPC error object; got %v", envelope)
	}
	result := failureResultObject(t, envelope)
	if got, ok := result["isError"].(bool); !ok || !got {
		t.Fatalf("public result isError = %v; want true", result["isError"])
	}
}

// failureResultObject extracts the tool-layer result object from the public
// response envelope.
func failureResultObject(t *testing.T, envelope map[string]any) map[string]any {
	t.Helper()
	result, ok := envelope["result"].(map[string]any)
	if !ok {
		t.Fatalf("public result is not an object: %v", envelope["result"])
	}
	return result
}

// assertToolAdmissionLogs asserts the captured stdout for a rejected
// tools/call: exactly one mcp_complete terminated at tool_admission with the
// unknown_tool reason, and zero backend_complete records because no upstream
// was attempted.
func assertToolAdmissionLogs(t *testing.T, captured string) {
	t.Helper()
	records := decodeCapturedRecords(t, captured)

	var admissions []map[string]any
	for _, rec := range filterRecords(records, "mcp_complete") {
		if rec["stage"] == "tool_admission" {
			admissions = append(admissions, rec)
		}
	}
	if len(admissions) != 1 {
		t.Fatalf("mcp_complete stage=tool_admission count = %d; want exactly 1", len(admissions))
	}
	mcp := admissions[0]
	if got := mcp["mcp_method"]; got != "tools_call" {
		t.Errorf("mcp_complete mcp_method = %v; want tools_call", got)
	}
	if got := mcp["outcome"]; got != "error" {
		t.Errorf("mcp_complete outcome = %v; want error", got)
	}
	if got := mcp["reason"]; got != "unknown_tool" {
		t.Errorf("mcp_complete reason = %v; want unknown_tool", got)
	}
	wantInternalHexID(t, mcp, "operation_id")
	wantNonnegativeDuration(t, mcp, "mcp_complete")

	if backendCount := len(filterRecords(records, "backend_complete")); backendCount != 0 {
		t.Fatalf("backend_complete count = %d; want 0: an unknown tool must never reach an upstream", backendCount)
	}
}

// assertToolReportedErrorLogs asserts the captured stdout for a tool-reported
// upstream failure: exactly one backend_complete and one mcp_complete, both
// terminated with the tool_reported_error reason and the backend_response
// stage on the mcp boundary.
func assertToolReportedErrorLogs(t *testing.T, captured string) {
	t.Helper()
	records := decodeCapturedRecords(t, captured)

	backends := filterRecords(records, "backend_complete")
	if len(backends) != 1 {
		t.Fatalf("backend_complete count = %d; want exactly 1 for the admitted dispatch", len(backends))
	}
	backend := backends[0]
	if got := backend["outcome"]; got != "error" {
		t.Errorf("backend_complete outcome = %v; want error", got)
	}
	if got := backend["reason"]; got != "tool_reported_error" {
		t.Errorf("backend_complete reason = %v; want tool_reported_error", got)
	}
	wantInternalHexID(t, backend, "backend_call_id")
	wantNonnegativeDuration(t, backend, "backend_complete")

	var decoded []map[string]any
	for _, rec := range filterRecords(records, "mcp_complete") {
		if rec["stage"] == "backend_response" {
			decoded = append(decoded, rec)
		}
	}
	if len(decoded) != 1 {
		t.Fatalf("mcp_complete stage=backend_response count = %d; want exactly 1", len(decoded))
	}
	mcp := decoded[0]
	if got := mcp["mcp_method"]; got != "tools_call" {
		t.Errorf("mcp_complete mcp_method = %v; want tools_call", got)
	}
	if got := mcp["outcome"]; got != "error" {
		t.Errorf("mcp_complete outcome = %v; want error", got)
	}
	if got := mcp["reason"]; got != "tool_reported_error" {
		t.Errorf("mcp_complete reason = %v; want tool_reported_error", got)
	}
	wantInternalHexID(t, mcp, "operation_id")
	wantNonnegativeDuration(t, mcp, "mcp_complete")
}

// loggedToolCall carries one completed failure-path tools/call: the public
// HTTP status, the decoded JSON-RPC envelope, and the drained captured stdout.
type loggedToolCall struct {
	status   int
	envelope map[string]any
	captured string
}

// loggedFailureToolCall drives the shared session flow over the exact wire
// contract: initialize to obtain the session identity,
// notifications/initialized, search_tools to discover the published catalog
// revision, then one call_tool whose arguments the caller derives from that
// revision. It drains the runtime before returning the public response and
// the captured stdout.
func loggedFailureToolCall(t *testing.T, ctx context.Context, httpURL string, stdout *syncWriter, stop func(), callArguments func(catalogRevision string) map[string]any) loggedToolCall {
	t.Helper()
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

	// Phase 1: initialize to obtain the public session identity.
	const initializeBody = `{"jsonrpc":"2.0","id":"init-failure-id","method":"initialize",` +
		`"params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"client-failures","version":"1"}}}`
	initResponse, initBody := send(initializeBody, "")
	if initResponse.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d; want 200; body %s", initResponse.StatusCode, string(initBody))
	}
	session := initResponse.Header.Get("Mcp-Session-Id")
	if session == "" {
		t.Fatal("initialize omitted session identity")
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

	// Phase 3: call_tool with the failure-oriented arguments.
	argsJSON, err := json.Marshal(callArguments(catalogRev))
	if err != nil {
		t.Fatalf("marshal call_tool arguments: %v", err)
	}
	callBody := `{"jsonrpc":"2.0","id":"call-id","method":"tools/call",` +
		`"params":{"name":"call_tool","arguments":` + string(argsJSON) + `}}`
	response, body := send(callBody, session)

	// Drain the runtime before inspecting captured terminal records.
	stop()
	return loggedToolCall{
		status:   response.StatusCode,
		envelope: extractJSONRPCPayload(t, string(body)),
		captured: stdout.String(),
	}
}

// fakeUpstreamReportedError serves one MCP tool whose handler answers with a
// well-formed CallToolResult carrying IsError: true, so the bridge decodes a
// tool-reported failure from a healthy upstream transport.
func fakeUpstreamReportedError(t *testing.T, toolName, schema string) *httptest.Server {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "fake-upstream", Version: "1"}, nil)
	server.AddTool(&mcp.Tool{Name: toolName, Description: toolName + " tool", InputSchema: json.RawMessage(schema)},
		func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: "upstream " + toolName + " reported a tool failure"}},
			}, nil
		})
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true},
	))
	t.Cleanup(httpServer.Close)
	return httpServer
}

// startHTTPLoggingBridgeWithUpstream mirrors startHTTPLoggingBridgeWithWriter
// but accepts the caller's upstream server, because the shared helper fixes
// the always-successful fakeUpstream and the failure paths need controlled
// upstream CallToolResults.
func startHTTPLoggingBridgeWithUpstream(t *testing.T, upstream *httptest.Server) (context.Context, string, *syncWriter, func()) {
	t.Helper()
	t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
	t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)
	stdout := &syncWriter{}
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
	directory := t.TempDir()
	configPath := filepath.Join(directory, "bridge.yaml")
	writeBridgeConfig(t, configPath, httpAddress, operationsAddress, filepath.Join(directory, "state"), embeddings.URL,
		[]bridgeServerSpec{{id: "alpha", endpoint: upstream.URL}})
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read fixture config: %v", err)
	}
	config = []byte(strings.Replace(string(config), "allowed_origins: []", `allowed_origins: ["https://allowed.example"]`, 1))
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
	return ctx, "http://" + httpAddress, stdout, stop
}
