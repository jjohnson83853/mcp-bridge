package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

	// The inbound edge enforces the configured host before authentication.
	request, err := http.NewRequest(http.MethodPost, httpURL+"/mcp", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("build host-check request: %v", err)
	}
	request.Host = "untrusted.invalid"
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
		return revision != "" && readName == core.PublicName("alpha", "read")
	})

	// A bridged call reaches the real upstream through the published generation.
	call := callTool(t, ctx, session, catalog.ToolCallTool, map[string]any{
		"tool_name":        readName,
		"catalog_revision": revision,
		"arguments":        map[string]any{"limit": 3},
	})
	envelope := structuredContent(t, call)
	if envelope["server"] != "alpha" || envelope["catalog_revision"] != revision {
		t.Fatalf("call envelope = %v", envelope)
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
	if betaName != core.PublicName("beta", "list") {
		t.Fatalf("beta tool name = %q; want %q", betaName, core.PublicName("beta", "list"))
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
	sendReload(t)
	waitFor(t, "a rejected reload to be logged", func() bool {
		return strings.Contains(stdout.String(), "reload")
	})
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
