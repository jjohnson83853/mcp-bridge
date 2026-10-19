package main

import (
	"context"
	"encoding/json"
	"errors"
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

// concurrentPostResult carries one completed POST /mcp outcome. It is built in
// goroutines, so it never carries t.* state; the main goroutine asserts it.
type concurrentPostResult struct {
	status      int
	echoedID    string
	sessionID   string
	body        string
	err         error
}

// TestServeLoggingConcurrent keeps two concurrently executing request chains
// isolated under reverse completion (logging-observability ticket 06). Both
// requests present the SAME public X-Request-ID and the SAME JSON-RPC id, so
// every internal identity must be minted independently of client input. The
// two upstreams park each bridged call at a barrier: request 1 is admitted and
// held, request 2 is then admitted and released first, so request 2's complete
// terminal chain (http_complete, mcp_complete, backend_complete) is fully
// written before request 1 is allowed to finish.
func TestServeLoggingConcurrent(t *testing.T) {
	// barrierR1 closes when request 1's bridged call reaches its upstream
	// barrier; barrierR2 closes likewise for request 2. completeR1 and
	// completeR2 are the release gates the upstream handlers wait on.
	barrierR1 := make(chan struct{})
	barrierR2 := make(chan struct{})
	completeR1 := make(chan struct{})
	completeR2 := make(chan struct{})

	alpha := barrierUpstream(t, "read", concurrentReadSchema, barrierR1, completeR1)
	beta := barrierUpstream(t, "read", concurrentReadSchema, barrierR2, completeR2)
	ctx, httpURL, stdout, stop := startHTTPLoggingBridgeTwoUpstreams(t, alpha, beta)

	const publicID = "11111111111111111111111111111111"
	const clientName = "concurrent-client-secret"

	client := &http.Client{Timeout: 20 * time.Second}
	// post sends one POST /mcp without t.* calls, so the concurrent phase can
	// reuse it safely from goroutines (t.Fatalf is main-goroutine only).
	post := func(body, session string) concurrentPostResult {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, httpURL+"/mcp", strings.NewReader(body))
		if err != nil {
			return concurrentPostResult{err: err}
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
			return concurrentPostResult{err: err}
		}
		content, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil {
			return concurrentPostResult{err: readErr}
		}
		return concurrentPostResult{
			status:    response.StatusCode,
			echoedID:  response.Header.Get("X-Request-ID"),
			sessionID: response.Header.Get("Mcp-Session-Id"),
			body:      string(content),
		}
	}
	mustPost := func(body, session string) concurrentPostResult {
		t.Helper()
		result := post(body, session)
		if result.err != nil {
			t.Fatalf("POST /mcp: %v", result.err)
		}
		return result
	}

	// handshake runs one client's initialize/initialized exchange and returns
	// the issued session identity.
	handshake := func(rpcID string) string {
		t.Helper()
		initialize := `{"jsonrpc":"2.0","id":"` + rpcID + `","method":"initialize",` +
			`"params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"` + clientName + `","version":"1"}}}`
		result := mustPost(initialize, "")
		if result.status != http.StatusOK {
			t.Fatalf("initialize status = %d; want 200; body %s", result.status, result.body)
		}
		if extractJSONRPCPayload(t, result.body) == nil {
			t.Fatalf("initialize response missing JSON-RPC payload: %q", result.body)
		}
		session := result.sessionID
		if session == "" {
			t.Fatal("initialize omitted session identity")
		}
		notification := mustPost(`{"jsonrpc":"2.0","method":"notifications/initialized"}`, session)
		if notification.status != http.StatusAccepted {
			t.Fatalf("initialized notification status = %d; want 202", notification.status)
		}
		return session
	}

	search := func(session string) string {
		t.Helper()
		body := `{"jsonrpc":"2.0","id":"concurrent-search-id","method":"tools/call",` +
			`"params":{"name":"search_tools","arguments":{"query":"read"}}}`
		result := mustPost(body, session)
		if result.status != http.StatusOK {
			t.Fatalf("search_tools status = %d; body %s", result.status, result.body)
		}
		envelope := extractJSONRPCPayload(t, result.body)
		if envelope == nil {
			t.Fatalf("search_tools response missing JSON-RPC payload: %q", result.body)
		}
		resultValue, ok := envelope["result"].(map[string]any)
		if !ok {
			t.Fatalf("search_tools missing result object: %v", envelope)
		}
		structured, ok := resultValue["structuredContent"].(map[string]any)
		if !ok {
			t.Fatalf("search_tools missing structuredContent: %v", resultValue)
		}
		revision, _ := structured["catalog_revision"].(string)
		if revision == "" {
			t.Fatalf("search_tools missing catalog_revision in %v", structured)
		}
		return revision
	}

	// Both clients handshake independently before the concurrent phase, so the
	// barrier phase exercises exactly two in-flight tools/call operations.
	sessionA := handshake("concurrent-init-a")
	sessionB := handshake("concurrent-init-b")
	revision := search(sessionA)

	buildCall := func(tool string) string {
		args, err := json.Marshal(map[string]any{
			"tool_name":        tool,
			"catalog_revision": revision,
			"arguments":        map[string]any{},
		})
		if err != nil {
			t.Fatalf("marshal call arguments: %v", err)
		}
		return `{"jsonrpc":"2.0","id":1,"method":"tools/call",` +
			`"params":{"name":"call_tool","arguments":` + string(args) + `}}`
	}
	callBodyR1 := buildCall(core.PublicName("alpha", "read"))
	callBodyR2 := buildCall(core.PublicName("beta", "read"))

	doneR1 := make(chan concurrentPostResult, 1)
	doneR2 := make(chan concurrentPostResult, 1)

	// Request 1 is issued first and parks at the alpha barrier. Request 2's
	// goroutine waits for barrierR1 before sending, so R1's internal identity
	// is minted strictly before R2's and both chains overlap in flight.
	go func() {
		doneR1 <- post(callBodyR1, sessionA)
	}()
	go func() {
		select {
		case <-barrierR1:
		case <-time.After(30 * time.Second):
			doneR2 <- concurrentPostResult{err: errors.New("request 1 never reached its upstream barrier")}
			return
		}
		doneR2 <- post(callBodyR2, sessionB)
	}()

	select {
	case <-barrierR2:
	case <-time.After(30 * time.Second):
		t.Fatal("request 2 never reached its upstream barrier")
	}

	// Reverse completion: release request 2 first and wait until its full
	// terminal chain is captured before releasing request 1.
	close(completeR2)
	var r2 concurrentPostResult
	select {
	case r2 = <-doneR2:
	case <-time.After(30 * time.Second):
		t.Fatal("request 2 did not complete after release")
	}
	// The completion logger writes synchronously inside the inbound handler,
	// so request 2's response delivery implies its three terminal records are
	// already captured; the counts below prove it without racing request 1.
	waitFor(t, "request 2 terminal records before request 1 release", func() bool {
		captured := stdout.String()
		return strings.Count(captured, `"event":"http_complete"`) == 6 &&
			strings.Count(captured, `"event":"mcp_complete"`) == 1 &&
			strings.Count(captured, `"event":"backend_complete"`) == 1
	})

	close(completeR1)
	var r1 concurrentPostResult
	select {
	case r1 = <-doneR1:
	case <-time.After(30 * time.Second):
		t.Fatal("request 1 did not complete after release")
	}

	// Both bridged calls must behave like normal successful tool calls on the
	// wire: status 200, echoed public X-Request-ID, JSON-RPC success result
	// with the upstream echo content.
	assertConcurrentCallOutcome(t, r1, "request 1", publicID)
	assertConcurrentCallOutcome(t, r2, "request 2", publicID)

	// Drain the runtime before inspecting the captured terminal records.
	stop()
	captured := stdout.String()
	for _, forbidden := range []string{
		publicID, serveClientToken, clientName,
		"concurrent-init-a", "concurrent-init-b", "concurrent-search-id",
		sessionA, sessionB,
	} {
		if forbidden != "" && strings.Contains(captured, forbidden) {
			t.Errorf("captured stdout contains client/session-controlled value %q", forbidden)
		}
	}

	records := decodeCapturedRecords(t, captured)
	httpCompletions := filterRecords(records, "http_complete")
	mcpCompletions := filterRecords(records, "mcp_complete")
	backendCompletions := filterRecords(records, "backend_complete")

	// Exactly one http_complete per POST /mcp: 3 for client A (initialize,
	// initialized, search), 2 for client B, and one per concurrent tools/call.
	if len(httpCompletions) != 7 {
		t.Fatalf("http_complete count = %d; want exactly 7", len(httpCompletions))
	}
	// Only callTool emits mcp_complete/backend_complete; search_tools and the
	// handshakes log nothing at those boundaries.
	if len(mcpCompletions) != 2 {
		t.Fatalf("mcp_complete count = %d; want exactly 2", len(mcpCompletions))
	}
	if len(backendCompletions) != 2 {
		t.Fatalf("backend_complete count = %d; want exactly 2", len(backendCompletions))
	}

	// The last two http_completes are the concurrent tools/call POSTs. Under
	// reverse completion request 2's terminal chain precedes request 1's even
	// though request 1 was admitted first.
	httpR2 := httpCompletions[len(httpCompletions)-2]
	httpR1 := httpCompletions[len(httpCompletions)-1]
	for _, record := range []map[string]any{httpR1, httpR2} {
		for key, want := range map[string]any{
			"http_status": float64(200), "http_method": "POST", "route": "mcp",
			"stage": "none", "reason": "none", "http_outcome": "success",
		} {
			if got := record[key]; got != want {
				t.Errorf("concurrent tools/call http_complete %s = %v; want %v", key, got, want)
			}
		}
	}
	httpRequestID1 := wantInternalHexID(t, httpR1, "request_id")
	httpRequestID2 := wantInternalHexID(t, httpR2, "request_id")
	if httpRequestID1 == httpRequestID2 {
		t.Error("concurrent requests must receive distinct internal request_id values")
	}
	if httpRequestID1 == publicID || httpRequestID2 == publicID {
		t.Error("public X-Request-ID must never be used as the internal request_id")
	}

	// Each mcp_complete joins to exactly one internal request identity, and the
	// two operations carry distinct operation identities.
	mcpByRequest := map[string]map[string]any{}
	for _, record := range mcpCompletions {
		requestID := wantInternalHexID(t, record, "request_id")
		if _, duplicate := mcpByRequest[requestID]; duplicate {
			t.Errorf("two mcp_complete records share request_id %q", requestID)
		}
		mcpByRequest[requestID] = record
	}
	if mcpByRequest[httpRequestID1] == nil || mcpByRequest[httpRequestID2] == nil {
		t.Fatalf("mcp_complete request_ids = %v; want exactly {%q, %q}",
			concurrentIDKeys(mcpByRequest), httpRequestID1, httpRequestID2)
	}
	mcpR1, mcpR2 := mcpByRequest[httpRequestID1], mcpByRequest[httpRequestID2]
	for _, record := range mcpCompletions {
		if got := record["mcp_method"]; got != "tools_call" {
			t.Errorf("mcp_complete mcp_method = %v; want tools_call", got)
		}
		if got := record["mcp_message_kind"]; got != "request" {
			t.Errorf("mcp_complete mcp_message_kind = %v; want request", got)
		}
		if got := record["outcome"]; got != "success" {
			t.Errorf("mcp_complete outcome = %v; want success", got)
		}
		wantNonnegativeDuration(t, record, "mcp_complete")
	}
	operationID1 := wantInternalHexID(t, mcpR1, "operation_id")
	operationID2 := wantInternalHexID(t, mcpR2, "operation_id")
	if operationID1 == operationID2 {
		t.Error("concurrent requests must receive distinct operation_id values")
	}

	// Each backend_complete joins to the same request and operation as its
	// mcp_complete, with its own distinct backend_call identity.
	backendByRequest := map[string]map[string]any{}
	for _, record := range backendCompletions {
		requestID := wantInternalHexID(t, record, "request_id")
		if _, duplicate := backendByRequest[requestID]; duplicate {
			t.Errorf("two backend_complete records share request_id %q", requestID)
		}
		backendByRequest[requestID] = record
	}
	if backendByRequest[httpRequestID1] == nil || backendByRequest[httpRequestID2] == nil {
		t.Fatalf("backend_complete request_ids = %v; want exactly {%q, %q}",
			concurrentIDKeys(backendByRequest), httpRequestID1, httpRequestID2)
	}
	backendR1, backendR2 := backendByRequest[httpRequestID1], backendByRequest[httpRequestID2]
	backendCallID1 := wantInternalHexID(t, backendR1, "backend_call_id")
	backendCallID2 := wantInternalHexID(t, backendR2, "backend_call_id")
	if backendCallID1 == backendCallID2 {
		t.Error("concurrent requests must receive distinct backend_call_id values")
	}
	if got := backendR1["operation_id"]; got != operationID1 {
		t.Errorf("request 1 backend_complete operation_id = %v; want %v", got, operationID1)
	}
	if got := backendR2["operation_id"]; got != operationID2 {
		t.Errorf("request 2 backend_complete operation_id = %v; want %v", got, operationID2)
	}
	for _, record := range backendCompletions {
		if got := record["outcome"]; got != "success" {
			t.Errorf("backend_complete outcome = %v; want success", got)
		}
		wantNonnegativeDuration(t, record, "backend_complete")
	}
	if got := backendR1["server"]; got != "alpha" {
		t.Errorf("request 1 backend_complete server = %v; want alpha", got)
	}
	if got := backendR1["tool"]; got != core.PublicName("alpha", "read") {
		t.Errorf("request 1 backend_complete tool = %v; want %v", got, core.PublicName("alpha", "read"))
	}
	if got := backendR2["server"]; got != "beta" {
		t.Errorf("request 2 backend_complete server = %v; want beta", got)
	}
	if got := backendR2["tool"]; got != core.PublicName("beta", "read") {
		t.Errorf("request 2 backend_complete tool = %v; want %v", got, core.PublicName("beta", "read"))
	}

	// Reverse completion evidence: request 1 was admitted first (its barrier
	// opened request 2's send), yet request 2's terminal chain is fully
	// emitted before request 1 is released, so R2's records precede R1's in
	// the captured stream.
	if got := indexByRequestID(mcpCompletions, httpRequestID2); got > indexByRequestID(mcpCompletions, httpRequestID1) {
		t.Error("request 2's mcp_complete must precede request 1's under reverse completion")
	}
	if got := indexByRequestID(backendCompletions, httpRequestID2); got > indexByRequestID(backendCompletions, httpRequestID1) {
		t.Error("request 2's backend_complete must precede request 1's under reverse completion")
	}

	// No record mixes identities: any record carrying both request_id and
	// operation_id must pair them exactly as its own chain established them.
	pairing := map[string]string{
		httpRequestID1: operationID1,
		httpRequestID2: operationID2,
	}
	for _, record := range records {
		requestID, hasRequest := record["request_id"].(string)
		operationID, hasOperation := record["operation_id"].(string)
		if !hasRequest || !hasOperation {
			continue
		}
		if want, known := pairing[requestID]; !known || want != operationID {
			t.Errorf("record %v mixes request_id %q with operation_id %q", record["event"], requestID, operationID)
		}
	}
}

// assertConcurrentCallOutcome checks the public wire behavior of one completed
// concurrent tools/call: 200, echoed public identity, JSON-RPC success result
// carrying the upstream echo.
func assertConcurrentCallOutcome(t *testing.T, result concurrentPostResult, label, publicID string) {
	t.Helper()
	if result.err != nil {
		t.Fatalf("%s: %v", label, result.err)
	}
	if result.status != http.StatusOK {
		t.Fatalf("%s status = %d; want 200; body %s", label, result.status, result.body)
	}
	if got := result.echoedID; got != publicID {
		t.Fatalf("%s public X-Request-ID echo = %q; want %q", label, got, publicID)
	}
	envelope := extractJSONRPCPayload(t, result.body)
	if envelope == nil {
		t.Fatalf("%s response missing JSON-RPC payload: %q", label, result.body)
	}
	if envelope["jsonrpc"] != "2.0" {
		t.Errorf("%s response jsonrpc = %v; want 2.0", label, envelope["jsonrpc"])
	}
	resultValue, hasResult := envelope["result"]
	_, hasError := envelope["error"]
	if !hasResult || hasError {
		t.Fatalf("%s response must carry a JSON-RPC success result; got %v", label, envelope)
	}
	resultMap, ok := resultValue.(map[string]any)
	if !ok {
		t.Fatalf("%s tool result is not an object: %v", label, resultValue)
	}
	content, ok := resultMap["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("%s tool result missing content: %v", label, resultMap)
	}
	firstContent, ok := content[0].(map[string]any)
	if !ok || !strings.Contains(firstContent["text"].(string), "upstream") {
		t.Fatalf("%s tool result missing upstream echo: %v", label, resultMap)
	}
}

// barrierUpstream serves one MCP tool whose handler parks every bridged call
// until the test releases it, mirroring the shape of the shared fakeUpstream.
func barrierUpstream(t *testing.T, toolName, schema string, arrived, release chan struct{}) *httptest.Server {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "fake-upstream", Version: "1"}, nil)
	server.AddTool(&mcp.Tool{Name: toolName, Description: toolName + " tool", InputSchema: json.RawMessage(schema)},
		func(ctx context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			close(arrived)
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(45 * time.Second):
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{
				Text: "upstream " + toolName + " echoed",
			}}}, nil
		})
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true},
	))
	t.Cleanup(httpServer.Close)
	return httpServer
}

// startHTTPLoggingBridgeTwoUpstreams mirrors startHTTPLoggingBridgeWithUpstream
// but wires two independent upstream servers, because the shared helper fixes
// one always-successful upstream and the concurrent isolation scenario needs a
// distinct barrier per request chain.
func startHTTPLoggingBridgeTwoUpstreams(t *testing.T, alpha, beta *httptest.Server) (context.Context, string, *syncWriter, func()) {
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
		[]bridgeServerSpec{{id: "alpha", endpoint: alpha.URL}, {id: "beta", endpoint: beta.URL}})
	if _, err := os.ReadFile(configPath); err != nil {
		t.Fatalf("read fixture config: %v", err)
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
	waitFor(t, "readiness of the configured servers", func() bool {
		status, body, ok := probeBody(t, "http://"+operationsAddress+"/readyz")
		return ok && status == http.StatusOK &&
			strings.Contains(body, `"status":"ready"`) &&
			strings.Contains(body, "alpha") && strings.Contains(body, "beta")
	})
	return ctx, "http://" + httpAddress, stdout, stop
}

// concurrentIDKeys lists the request identities present in a completion map,
// used only for failure messages.
func concurrentIDKeys(mapping map[string]map[string]any) []string {
	keys := make([]string, 0, len(mapping))
	for key := range mapping {
		keys = append(keys, key)
	}
	return keys
}

func indexByRequestID(records []map[string]any, requestID string) int {
	for index, record := range records {
		if record["request_id"] == requestID {
			return index
		}
	}
	return -1
}

const concurrentReadSchema = `{"type":"object","properties":{},"additionalProperties":true}`
