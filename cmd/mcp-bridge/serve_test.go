package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
)

const (
	serveClientToken  = "serve-test-client-token"
	serveEmbeddingKey = "serve-test-embedding-key"
	serveDimensions   = 8
)

// syncWriter guards one buffer the process logger writes from request
// goroutines, so the assertions never race the process itself.
type syncWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *syncWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func freeAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve address: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release address: %v", err)
	}
	return address
}

// fakeUpstream serves one MCP tool that echoes the arguments it received, so a
// bridged call proves the exact bytes reached this upstream.
func fakeUpstream(t *testing.T, toolName, schema string) *httptest.Server {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "fake-upstream", Version: "1"}, nil)
	server.AddTool(&mcp.Tool{Name: toolName, Description: toolName + " tool", InputSchema: json.RawMessage(schema)},
		func(_ context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{
				Text: "upstream " + toolName + " saw " + string(request.Params.Arguments),
			}}}, nil
		})
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true},
	))
	t.Cleanup(httpServer.Close)
	return httpServer
}

// fakeEmbeddings answers OpenAI-compatible embedding requests with finite,
// non-zero vectors of the configured width.
func fakeEmbeddings(t *testing.T) *httptest.Server {
	t.Helper()
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		type item struct {
			Index     int       `json:"index"`
			Embedding []float64 `json:"embedding"`
		}
		items := make([]item, len(body.Input))
		for i := range body.Input {
			vector := make([]float64, serveDimensions)
			for j := range vector {
				vector[j] = 0.25 + float64(j)/100
			}
			items[i] = item{Index: i, Embedding: vector}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": items})
	}))
	t.Cleanup(httpServer.Close)
	return httpServer
}

type bridgeServerSpec struct {
	id              string
	endpoint        string
	promptsDisabled bool
}

// writeBridgeConfig writes one complete operator document with the default
// catalog refresh interval. Reload phases rewrite the same path with different
// membership.
func writeBridgeConfig(t *testing.T, path, httpAddress, operationsAddress, storageDirectory, embeddingURL string, servers []bridgeServerSpec) {
	t.Helper()
	writeBridgeConfigInterval(t, path, httpAddress, operationsAddress, storageDirectory, embeddingURL, "5m", servers)
}

// writeBridgeConfigInterval is writeBridgeConfig with an explicit catalog
// refresh interval, so a test can observe periodic re-discovery without
// signalling the process.
func writeBridgeConfigInterval(t *testing.T, path, httpAddress, operationsAddress, storageDirectory, embeddingURL, refreshInterval string, servers []bridgeServerSpec) {
	t.Helper()
	document := fmt.Sprintf(`schema_version: 1
http:
  listen: %q
  allowed_hosts: [%q]
  allowed_origins: []
  max_request_bytes: 1048576
  requests_per_second: 50
  burst: 100
  max_sessions: 128
  session_idle_timeout: "10m"
operations:
  listen: %q
  requests_per_second: 10
  burst: 20
storage:
  directory: %q
  max_snapshot_bytes: 268435456
catalog:
  refresh_interval: %q
  stale_after: "15m"
  max_tools_per_server: 10000
  max_tools_total: 20000
  max_pages_per_server: 1000
  max_schema_bytes: 262144
  max_description_bytes: 32768
search:
  default_top_k: 10
  max_top_k: 50
  timeout: "5s"
  query_cache_entries: 256
  query_cache_ttl: "5m"
  keyword_weight: 0.5
  embedding_weight: 0.5
  rrf_constant: 60
embedding:
  provider: "openai-compatible"
  model: "test-model"
  model_revision: "test-revision"
  base_url: %q
  dimensions: %d
  api_key: {env: SERVE_EMBEDDING_KEY}
  timeout: "3s"
  max_concurrency: 2
  batch_size: 32
  max_response_bytes: 4194304
client_auth:
  bearer_tokens:
    - {env: SERVE_CLIENT_TOKEN}
upstream_defaults:
  connect_timeout: "5s"
  call_timeout: "30s"
  refresh_timeout: "15s"
  max_concurrent_calls: 8
  max_event_bytes: 4194304
  max_response_bytes: 8388608
  breaker:
    consecutive_failures: 3
    open_duration: "30s"
    half_open_probes: 1
`, httpAddress, httpAddress, operationsAddress, storageDirectory, refreshInterval, embeddingURL, serveDimensions)
	var builder strings.Builder
	builder.WriteString(document)
	builder.WriteString("servers:\n")
	for _, server := range servers {
		builder.WriteString(fmt.Sprintf("  - id: %s\n    endpoint: %q\n", server.id, server.endpoint))
		if server.promptsDisabled {
			builder.WriteString("    prompts_disabled: true\n")
		}
	}
	builder.WriteString("pins: []\n")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

// bearerTransport injects the operator token the inbound edge authenticates.
type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t bearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(clone)
}

func connectBridge(t *testing.T, ctx context.Context, endpoint, token string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "serve-test-client", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   endpoint,
		HTTPClient: &http.Client{Transport: bearerTransport{token: token, base: http.DefaultTransport}},
		MaxRetries: -1,
	}, nil)
	if err != nil {
		t.Fatalf("connect bridge: %v", err)
	}
	return session
}

func callTool(t *testing.T, ctx context.Context, session *mcp.ClientSession, name string, arguments map[string]any) *mcp.CallToolResult {
	t.Helper()
	raw, err := json.Marshal(arguments)
	if err != nil {
		t.Fatalf("marshal %s arguments: %v", name, err)
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: json.RawMessage(raw)})
	if err != nil {
		t.Fatalf("tools/call %s: %v", name, err)
	}
	return result
}

func structuredContent(t *testing.T, result *mcp.CallToolResult) map[string]any {
	t.Helper()
	payload, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structuredContent = %T (%v); want an object", result.StructuredContent, result.StructuredContent)
	}
	return payload
}

func searchPayload(t *testing.T, ctx context.Context, session *mcp.ClientSession, query string) map[string]any {
	t.Helper()
	return structuredContent(t, callTool(t, ctx, session, catalog.ToolSearchTools, map[string]any{"query": query}))
}

// catalogRevisionAndTool finds one server's advertised tool and the revision
// that must be echoed back to admit a call.
func catalogRevisionAndTool(t *testing.T, payload map[string]any, serverID string) (string, string) {
	t.Helper()
	revision, ok := payload["catalog_revision"].(string)
	if !ok || revision == "" {
		t.Fatalf("search payload revision = %v", payload["catalog_revision"])
	}
	results, ok := payload["results"].([]any)
	if !ok {
		t.Fatalf("search payload results = %T", payload["results"])
	}
	for _, entry := range results {
		row, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if row["server"] == serverID {
			name, ok := row["tool_name"].(string)
			if !ok || name == "" {
				t.Fatalf("search result tool_name = %v", row["tool_name"])
			}
			return revision, name
		}
	}
	t.Fatalf("catalog revision %s has no tool of server %q: %v", revision, serverID, results)
	return "", ""
}

type serveWireResponse struct {
	status  int
	headers http.Header
	body    map[string]any
}

func postServeWire(t *testing.T, endpoint, sessionID string, id any, method string, params map[string]any) serveWireResponse {
	t.Helper()
	requestBody := map[string]any{"jsonrpc": "2.0", "method": method}
	if id != nil {
		requestBody["id"] = id
	}
	if params != nil {
		requestBody["params"] = params
	}
	raw, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal %s request: %v", method, err)
	}
	request, err := http.NewRequest(http.MethodPost, endpoint+"/mcp", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("build %s request: %v", method, err)
	}
	request.Header.Set("Authorization", "Bearer "+serveClientToken)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		request.Header.Set("Mcp-Session-Id", sessionID)
		request.Header.Set("Mcp-Protocol-Version", "2025-03-26")
	}
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatalf("POST %s: %v", method, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read %s response: %v", method, err)
	}
	body := strings.TrimSpace(string(responseBody))
	for _, line := range strings.Split(body, "\n") {
		if data, ok := strings.CutPrefix(strings.TrimSpace(line), "data: "); ok {
			body = data
			break
		}
	}
	var decoded map[string]any
	if body != "" {
		if err := json.Unmarshal([]byte(body), &decoded); err != nil {
			t.Fatalf("decode %s response %q: %v", method, responseBody, err)
		}
	}
	return serveWireResponse{status: response.StatusCode, headers: response.Header.Clone(), body: decoded}
}

func requireServeWireToolResult(t *testing.T, response serveWireResponse) map[string]any {
	t.Helper()
	if response.status != http.StatusOK {
		t.Fatalf("tools/call status = %d; want 200 (body=%v)", response.status, response.body)
	}
	result, ok := response.body["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/call response = %v; want JSON-RPC result", response.body)
	}
	return result
}

func requireServeWireStructuredContent(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	payload, ok := result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("structuredContent = %T (%v); want an object", result["structuredContent"], result["structuredContent"])
	}
	return payload
}

func assertServeWireCompletion(t *testing.T, stdout *syncWriter, response serveWireResponse, operation, reason, upstreamState string, wantBackend bool) {
	t.Helper()
	requestID := response.headers.Get("Mcp-Diagnostic-Id")
	if len(requestID) != 32 {
		t.Fatalf("Mcp-Diagnostic-Id = %q; want 32-character request ID", requestID)
	}
	var httpRecords, mcpRecords, backendRecords []map[string]any
	collect := func() {
		httpRecords, mcpRecords, backendRecords = nil, nil, nil
		for _, record := range decodeCapturedRecords(t, stdout.String()) {
			if record["request_id"] != requestID {
				continue
			}
			switch record["event"] {
			case "http_complete":
				httpRecords = append(httpRecords, record)
			case "mcp_complete":
				mcpRecords = append(mcpRecords, record)
			case "backend_complete":
				backendRecords = append(backendRecords, record)
			}
		}
	}
	waitFor(t, operation+" wire completion records", func() bool {
		collect()
		return len(httpRecords) == 1 && len(mcpRecords) == 1 && (!wantBackend || len(backendRecords) == 1)
	})
	collect()
	if len(httpRecords) != 1 || len(mcpRecords) != 1 {
		t.Fatalf("%s completion records: http=%d mcp=%d", operation, len(httpRecords), len(mcpRecords))
	}
	if wantBackend && len(backendRecords) != 1 || !wantBackend && len(backendRecords) != 0 {
		t.Fatalf("%s backend completion records = %d; want backend=%t", operation, len(backendRecords), wantBackend)
	}
	httpRecord, mcpRecord := httpRecords[0], mcpRecords[0]
	if httpRecord["request_id"] != mcpRecord["request_id"] {
		t.Fatalf("completion request IDs differ: http=%v mcp=%v", httpRecord["request_id"], mcpRecord["request_id"])
	}
	for _, record := range append(append([]map[string]any(nil), httpRecords...), mcpRecords...) {
		for _, key := range []string{"schema_version", "level", "service", "event", "message", "operation", "outcome"} {
			if value, ok := record[key]; !ok || value == "" {
				t.Errorf("%s record lacks %s: %#v", record["event"], key, record)
			}
		}
		if record["schema_version"] != float64(1) || record["service"] != "mcp-bridge" {
			t.Errorf("invalid envelope identity: %#v", record)
		}
	}
	if mcpRecord["operation"] != operation {
		t.Errorf("mcp_complete operation = %v; want %q", mcpRecord["operation"], operation)
	}
	wantOutcome := "success"
	if reason != "none" {
		wantOutcome = "error"
		if mcpRecord["stage"] != "tool_admission" {
			t.Errorf("mcp_complete stage = %v; want tool_admission", mcpRecord["stage"])
		}
		errorInfo, ok := mcpRecord["error"].(map[string]any)
		if !ok || errorInfo["code"] != reason || len(errorInfo) != 4 {
			t.Errorf("mcp_complete error = %#v; want nested code %q", mcpRecord["error"], reason)
		}
	} else if _, present := mcpRecord["error"]; present {
		t.Errorf("successful mcp_complete carries an error: %#v", mcpRecord["error"])
	}
	if mcpRecord["outcome"] != wantOutcome {
		t.Errorf("mcp_complete outcome = %v; want %q", mcpRecord["outcome"], wantOutcome)
	}
	if _, present := mcpRecord["reason"]; present {
		t.Error("mcp_complete emitted the removed flat reason field")
	}
	if upstreamState != "" && httpRecord["upstream_state"] != upstreamState {
		t.Errorf("http_complete upstream_state = %v; want %q", httpRecord["upstream_state"], upstreamState)
	}
	if wantBackend {
		record := backendRecords[0]
		if record["operation"] != "call_tool" || record["event"] != "backend_complete" || record["service"] != "mcp-bridge" {
			t.Errorf("invalid backend completion identity: %#v", record)
		}
		if message, ok := record["message"].(string); !ok || message == "" {
			t.Errorf("backend_complete message = %v; want non-empty", record["message"])
		}
	}
}

func getBody(t *testing.T, url string) (int, string) {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer response.Body.Close()
	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(response.Body); err != nil {
		t.Fatalf("read %s: %v", url, err)
	}
	return response.StatusCode, buffer.String()
}

// probeBody polls one URL: a listener that has not bound yet (or is already
// gone) reports not-ok instead of failing the test, because the polling
// conditions below must be able to observe "not serving yet".
func probeBody(t *testing.T, url string) (int, string, bool) {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		return 0, "", false
	}
	defer response.Body.Close()
	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(response.Body); err != nil {
		return 0, "", false
	}
	return response.StatusCode, buffer.String(), true
}

// waitFor polls one condition until it holds or the deadline passes.
func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func sendReload(t *testing.T) {
	t.Helper()
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("find process: %v", err)
	}
	if err := process.Signal(syscall.SIGHUP); err != nil {
		t.Fatalf("send SIGHUP: %v", err)
	}
}

func TestSessionAdmissionEnabledParsing(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{value: "true", want: true},
		{value: "TRUE", want: true},
		{value: "True", want: true},
		{value: "1", want: true},
		{value: "false"},
		{value: "0"},
		{value: ""},
	} {
		t.Run(fmt.Sprintf("%q", tc.value), func(t *testing.T) {
			if got := sessionAdmissionEnabled(tc.value); got != tc.want {
				t.Errorf("sessionAdmissionEnabled(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

// TestServeCompositionRootServesCatalogAndSurvivesReload is the process-level
// acceptance of plan §4.2/§4.5/§4.6/§4.7/§4.12: the operator document becomes a
// serving bridge (inbound auth + bounds, local catalog, ops routes), a SIGHUP
// reload changes membership without dropping the working generation, and a
// rejected candidate never disturbs it.
func TestServeCompositionRootServesCatalogAndSurvivesReload(t *testing.T) {
	t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
	t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)

	// The test process must survive the SIGHUP it sends; the process under test
	// registers its own handler for the same signal.
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	defer signal.Stop(sighup)

	readSchema := `{"type":"object","properties":{"limit":{"type":"integer"}},"additionalProperties":true}`
	listSchema := `{"type":"object","properties":{},"additionalProperties":true}`
	alpha := fakeUpstream(t, "read", readSchema)
	beta := fakeUpstream(t, "list", listSchema)
	embeddings := fakeEmbeddings(t)

	httpAddress := freeAddress(t)
	operationsAddress := freeAddress(t)
	directory := t.TempDir()
	configPath := filepath.Join(directory, "bridge.yaml")
	storageDirectory := filepath.Join(directory, "state")
	writeBridgeConfig(t, configPath, httpAddress, operationsAddress, storageDirectory, embeddings.URL, []bridgeServerSpec{{id: "alpha", endpoint: alpha.URL}})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stdout, stderr := &syncWriter{}, &syncWriter{}
	done := make(chan int, 1)
	go func() { done <- runContext(ctx, []string{"--config", configPath}, stdout, stderr) }()

	operationsURL := "http://" + operationsAddress
	httpURL := "http://" + httpAddress

	// Liveness is the event loop only; readiness needs one usable server.
	waitFor(t, "live operations listener", func() bool {
		status, body, ok := probeBody(t, operationsURL+"/livez")
		return ok && status == http.StatusOK && strings.Contains(body, "live")
	})
	waitFor(t, "readiness of the configured server", func() bool {
		status, body, ok := probeBody(t, operationsURL+"/readyz")
		return ok && status == http.StatusOK && strings.Contains(body, `"status":"ready"`) && strings.Contains(body, "alpha")
	})

	// The local startup and refresh path is observable on the operations route.
	waitFor(t, "catalog refresh metric", func() bool {
		status, body, ok := probeBody(t, operationsURL+"/metrics")
		return ok && status == http.StatusOK &&
			strings.Contains(body, "bridge_catalog_refresh_total") &&
			strings.Contains(body, `bridge_readiness{server="alpha"}`)
	})

	// The inbound edge enforces the configured host: even a request carrying a
	// valid client token is refused on an untrusted Host, while an anonymous
	// request is refused by authentication before the host check can run.
	request, err := http.NewRequest(http.MethodPost, httpURL+"/mcp", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("build host-check request: %v", err)
	}
	request.Host = "untrusted.invalid"
	request.Header.Set("Authorization", "Bearer "+serveClientToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("host-check request: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("untrusted Host status = %d; want 403", response.StatusCode)
	}

	// An unauthenticated request is refused, so the token wiring is real.
	response, err = http.Post(httpURL+"/mcp", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("anonymous request: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous /mcp status = %d; want 401", response.StatusCode)
	}

	session := connectBridge(t, ctx, httpURL+"/mcp", serveClientToken)
	defer session.Close()

	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	if len(listed.Tools) != 3 {
		t.Fatalf("advertised %d tools; want exactly 3", len(listed.Tools))
	}

	// The cold store has no cached catalog: the first pass discovers, embeds and
	// publishes it, so a search returns the configured server's tool.
	var revision, readName string
	waitFor(t, "first published catalog", func() bool {
		if session == nil {
			return false
		}
		payload := searchPayload(t, ctx, session, "read")
		results, ok := payload["results"].([]any)
		if !ok || len(results) == 0 {
			return false
		}
		revision, readName = catalogRevisionAndTool(t, payload, "alpha")
		return revision != "" && readName == "alpha__read"
	})

	// A bridged call reaches the real upstream through the published generation.
	call := callTool(t, ctx, session, catalog.ToolCallTool, map[string]any{
		"tool_name":        readName,
		"catalog_revision": revision,
		"arguments":        map[string]any{"limit": 3},
	})
	envelope := structuredContent(t, call)
	if envelope["server"] != "alpha" || envelope["tool_name"] != readName {
		t.Fatalf("call envelope = %v; want alpha/%s", envelope, readName)
	}
	for _, key := range []string{"request_id", "upstream_result"} {
		if _, ok := envelope[key]; !ok {
			t.Errorf("call envelope lacks %q: %v", key, envelope)
		}
	}
	for _, key := range []string{"catalog_revision", "catalog_status", "availability"} {
		if _, exists := envelope[key]; exists {
			t.Errorf("call envelope unexpectedly contains %q: %v", key, envelope)
		}
	}
	if len(call.Content) == 0 {
		t.Fatalf("call returned no content: %#v", call)
	}
	text, ok := call.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("call content = %T", call.Content[0])
	}
	if !strings.Contains(text.Text, `{"limit":3}`) {
		t.Fatalf("upstream saw %q; want the forwarded arguments", text.Text)
	}

	// Adding a server is the reload path: SIGHUP admits the new membership while
	// the already working generation keeps serving.
	writeBridgeConfig(t, configPath, httpAddress, operationsAddress, storageDirectory, embeddings.URL, []bridgeServerSpec{
		{id: "alpha", endpoint: alpha.URL},
		{id: "beta", endpoint: beta.URL},
	})
	sendReload(t)
	var betaRevision, betaName string
	waitFor(t, "beta to join the published catalog", func() bool {
		payload := searchPayload(t, ctx, session, "list")
		results, ok := payload["results"].([]any)
		if !ok || len(results) == 0 {
			return false
		}
		for _, entry := range results {
			row, ok := entry.(map[string]any)
			if !ok || row["server"] != "beta" {
				continue
			}
			betaRevision, betaName = catalogRevisionAndTool(t, payload, "beta")
			return true
		}
		return false
	})
	if betaName != "beta__list" {
		t.Fatalf("beta tool name = %q; want beta__list", betaName)
	}
	betaCall := callTool(t, ctx, session, catalog.ToolCallTool, map[string]any{
		"tool_name":        betaName,
		"catalog_revision": betaRevision,
		"arguments":        map[string]any{"verbose": true},
	})
	betaText, ok := betaCall.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(betaText.Text, `{"verbose":true}`) {
		t.Fatalf("beta call content = %#v", betaCall.Content)
	}

	// The global catalog revision moved with the membership change, so a call
	// still carrying the pre-reload revision is refused with zero dispatch and
	// is never silently rerouted to a different server (plan §4.3: mismatch is
	// catalog_changed; unrelated server changes also invalidate the revision).
	staleCall := callTool(t, ctx, session, catalog.ToolCallTool, map[string]any{
		"tool_name":        readName,
		"catalog_revision": revision,
		"arguments":        map[string]any{"limit": 5},
	})
	if !staleCall.IsError {
		t.Fatalf("stale revision was admitted: %#v", staleCall)
	}
	staleEnvelope := structuredContent(t, staleCall)
	if staleEnvelope["code"] != "catalog_changed" || staleEnvelope["execution_state"] != core.ExecutionNotStarted {
		t.Fatalf("stale revision refusal = %#v; want catalog_changed/not_started", staleEnvelope)
	}

	// The server published before the reload keeps serving: membership was added,
	// never dropped, so alpha answers at the new revision.
	alphaStillWorks := callTool(t, ctx, session, catalog.ToolCallTool, map[string]any{
		"tool_name":        readName,
		"catalog_revision": betaRevision,
		"arguments":        map[string]any{"limit": 5},
	})
	if alphaStillWorks.IsError {
		t.Fatalf("alpha no longer served after the reload: %#v", alphaStillWorks)
	}
	alphaText, ok := alphaStillWorks.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(alphaText.Text, `{"limit":5}`) {
		t.Fatalf("alpha call content = %#v", alphaStillWorks.Content)
	}

	// A rejected candidate leaves the published generation untouched.
	if err := os.WriteFile(configPath, []byte("schema_version: 1\nservers:\n  - id: broken\n    endpoint: \"://bad\"\n"), 0o600); err != nil {
		t.Fatalf("write rejected candidate: %v", err)
	}
	offset := len(stderr.String())
	sendReload(t)
	waitFor(t, "a rejected reload to be logged", func() bool {
		return strings.Contains(stderr.String()[offset:], `"event":"config_reload_rejected"`)
	})
	rejectionCount := 0
	rejectionFound := false
	for _, line := range strings.Split(strings.TrimSpace(stderr.String()[offset:]), "\n") {
		if line == "" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("invalid structured reload record: %v", err)
		}
		if event["event"] != "config_reload_rejected" {
			continue
		}
		rejectionCount++
		if event["level"] != "warn" || event["operation"] != "reload" || event["outcome"] != "failure" {
			t.Errorf("reload envelope changed: %#v", event)
		}
		errorInfo, errorOK := event["error"].(map[string]any)
		configInfo, configOK := event["config"].(map[string]any)
		if !errorOK || len(errorInfo) != 4 {
			t.Errorf("reload error shape = %#v; want AD8 error object", event["error"])
		}
		for _, field := range []string{"code", "kind", "retryable", "remediation"} {
			if _, ok := errorInfo[field]; !ok {
				t.Errorf("reload error missing %q: %#v", field, errorInfo)
			}
		}
		if errorOK && configOK && errorInfo["code"] == "url_invalid" && configInfo["property_path"] == "servers[0].endpoint" {
			rejectionFound = true
		}
	}
	if rejectionCount != 1 {
		t.Errorf("got %d config_reload_rejected records; want exactly one", rejectionCount)
	}
	if !rejectionFound {
		t.Fatalf("rejected reload lacked nested url_invalid diagnostic context: %s", stderr.String()[offset:])
	}
	afterRejection := callTool(t, ctx, session, catalog.ToolCallTool, map[string]any{
		"tool_name":        betaName,
		"catalog_revision": betaRevision,
		"arguments":        map[string]any{"verbose": false},
	})
	if afterRejection.IsError {
		t.Fatalf("rejected candidate disturbed the published generation: %#v", afterRejection)
	}
	status, metricsBody := getBody(t, operationsURL+"/metrics")
	if status != http.StatusOK || !strings.Contains(metricsBody, "bridge_config_reload_total") {
		t.Fatalf("reload metric missing from %d body: %s", status, metricsBody)
	}

	cancel()
	select {
	case status := <-done:
		if status != 0 {
			t.Fatalf("run status = %d; want 0 (stderr=%s)", status, stderr.String())
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the process did not shut down after cancellation")
	}
	if response, err := http.Get(operationsURL + "/livez"); err == nil {
		response.Body.Close()
		t.Fatalf("operations listener still serving after shutdown: %d", response.StatusCode)
	}
}
func TestServeCatalogErgonomicsLiveWire(t *testing.T) {
	_, httpURL, stdout, stop := startHTTPLoggingBridge(t)
	defer stop()

	initialize := postServeWire(t, httpURL, "", 1, "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":   map[string]any{},
		"clientInfo":     map[string]any{"name": "wire-test", "version": "1"},
	})
	if initialize.status != http.StatusOK {
		t.Fatalf("initialize status = %d; want 200", initialize.status)
	}
	sessionID := initialize.headers.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("initialize response omitted Mcp-Session-Id")
	}
	initialized := postServeWire(t, httpURL, sessionID, nil, "notifications/initialized", map[string]any{})
	if initialized.status != http.StatusOK && initialized.status != http.StatusAccepted && initialized.status != http.StatusNoContent {
		t.Fatalf("initialized notification status = %d; want 200, 202, or 204", initialized.status)
	}

	initialList := postServeWire(t, httpURL, sessionID, 2, "tools/list", map[string]any{})
	if initialList.status != http.StatusOK {
		t.Fatalf("initial tools/list status = %d; want 200", initialList.status)
	}
	initialResult, ok := initialList.body["result"].(map[string]any)
	if !ok {
		t.Fatalf("initial tools/list response = %v; want result", initialList.body)
	}
	initialTools, ok := initialResult["tools"].([]any)
	if !ok {
		t.Fatalf("initial tools/list tools = %T; want array", initialResult["tools"])
	}
	for _, item := range initialTools {
		tool, ok := item.(map[string]any)
		if ok && tool["name"] == "alpha__read" {
			t.Fatal("alpha__read was visible before search discovery")
		}
	}

	nextID := 3
	callWireTool := func(name string, arguments map[string]any) (serveWireResponse, map[string]any, map[string]any) {
		t.Helper()
		response := postServeWire(t, httpURL, sessionID, nextID, "tools/call", map[string]any{
			"name": name, "arguments": arguments,
		})
		nextID++
		result := requireServeWireToolResult(t, response)
		return response, result, requireServeWireStructuredContent(t, result)
	}

	searchResponse, searchResult, search := callWireTool("search_tools", map[string]any{"query": "read"})
	isSearchError, _ := searchResult["isError"].(bool)
	if isSearchError {
		t.Fatalf("search_tools returned an error: %v", search)
	}
	if _, exists := search["backends"]; exists {
		t.Fatalf("search payload includes pruned backends: %v", search["backends"])
	}
	revision, ok := search["catalog_revision"].(string)
	if !ok || len(revision) != 64 {
		t.Fatalf("search catalog_revision = %v; want a 64-character digest", search["catalog_revision"])
	}
	results, ok := search["results"].([]any)
	if !ok {
		t.Fatalf("search results = %T; want array", search["results"])
	}
	const wireToolName = "alpha__read"
	foundSearchTool := false
	for _, item := range results {
		row, ok := item.(map[string]any)
		if !ok || row["server"] != "alpha" || row["upstream_name"] != "read" {
			continue
		}
		foundSearchTool = true
		if row["tool_name"] != wireToolName || row["availability"] != "available" {
			t.Errorf("search row = %v; want tool_name=%q and availability=available", row, wireToolName)
		}
	}
	if !foundSearchTool {
		t.Fatalf("search did not return alpha/read: %v", results)
	}
	if searchResponse.status != http.StatusOK {
		t.Fatalf("search_tools status = %d; want 200", searchResponse.status)
	}

	listed := postServeWire(t, httpURL, sessionID, nextID, "tools/list", map[string]any{})
	nextID++
	if listed.status != http.StatusOK {
		t.Fatalf("tools/list after discovery status = %d; want 200", listed.status)
	}
	listResult, ok := listed.body["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/list response = %v; want result", listed.body)
	}
	listedTools, ok := listResult["tools"].([]any)
	if !ok {
		t.Fatalf("tools/list tools = %T; want array", listResult["tools"])
	}
	foundListedTool := false
	for _, item := range listedTools {
		tool, ok := item.(map[string]any)
		if ok && tool["name"] == wireToolName {
			foundListedTool = true
			break
		}
	}
	if !foundListedTool {
		t.Fatalf("tools/list omitted newly discovered %q: %v", wireToolName, listedTools)
	}

	staleRevision := revision
	if staleRevision[0] == '0' {
		staleRevision = "1" + staleRevision[1:]
	} else {
		staleRevision = "0" + staleRevision[1:]
	}
	cases := []struct {
		name            string
		tool            string
		revision        string
		includeRevision bool
		wantError       bool
		reason          string
	}{
		{name: "describe omitted revision", tool: "describe_tool"},
		{name: "describe empty revision", tool: "describe_tool", includeRevision: true},
		{name: "call omitted revision", tool: "call_tool"},
		{name: "call empty revision", tool: "call_tool", includeRevision: true},
		{name: "describe stale revision", tool: "describe_tool", revision: staleRevision, includeRevision: true, wantError: true, reason: "catalog_changed"},
		{name: "call stale revision", tool: "call_tool", revision: staleRevision, includeRevision: true, wantError: true, reason: "catalog_changed"},
		{name: "describe malformed revision", tool: "describe_tool", revision: "not-hex", includeRevision: true, wantError: true, reason: "invalid_arguments"},
		{name: "call malformed revision", tool: "call_tool", revision: "not-hex", includeRevision: true, wantError: true, reason: "invalid_arguments"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			arguments := map[string]any{"tool_name": wireToolName}
			if tc.includeRevision {
				arguments["catalog_revision"] = tc.revision
			}
			if tc.tool == "call_tool" {
				arguments["arguments"] = map[string]any{}
			}
			response, result, payload := callWireTool(tc.tool, arguments)
			isError, _ := result["isError"].(bool)
			if isError != tc.wantError {
				t.Fatalf("%s isError = %v; want %t (result=%v)", tc.tool, result["isError"], tc.wantError, result)
			}
			reason := tc.reason
			if !tc.wantError {
				reason = "none"
				if tc.tool == "describe_tool" && payload["catalog_revision"] != revision {
					t.Errorf("describe catalog_revision = %v; want %q", payload["catalog_revision"], revision)
				}
				if tc.tool == "call_tool" {
					for _, key := range []string{"server", "tool_name", "request_id", "upstream_result"} {
						if _, exists := payload[key]; !exists {
							t.Errorf("call_tool envelope lacks %q: %v", key, payload)
						}
					}
					for _, key := range []string{"catalog_revision", "catalog_status", "availability"} {
						if _, exists := payload[key]; exists {
							t.Errorf("call_tool envelope contains pruned %q: %v", key, payload)
						}
					}
				}
			} else if payload["code"] != tc.reason {
				t.Errorf("%s refusal code = %v; want %q", tc.tool, payload["code"], tc.reason)
			}
			upstreamState := ""
			if tc.tool == "describe_tool" || tc.wantError {
				upstreamState = "not_attempted"
			}
			assertServeWireCompletion(t, stdout, response, tc.tool, reason, upstreamState, tc.tool == "call_tool" && !tc.wantError)
		})
	}
}
