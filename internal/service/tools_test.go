package service_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/search"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

// toolSurfaceConfig pins the tool surface's own advertised tools and their
// pins (plan §4.4, T-05b RED).
const toolSurfaceConfig = `schema_version: 1
servers:
  - id: alpha
    endpoint: https://alpha.invalid/mcp
    credentials:
      bearer: {env: TOOLS_ALPHA_SECRET}
client_auth:
  bearer_tokens:
    - {env: TOOLS_CLIENT_TOKEN}
pins:
  - {server: alpha, tool: read}
`

const (
	toolReadSchema = `{"type":"object","properties":{"limit":{"type":"integer","minimum":1}},"required":["limit"],"additionalProperties":false}`
	// A reference the standalone schema document cannot resolve: the bridge
	// must report it as unsupported rather than rewrite or guess it.
	toolUnsupportedSchema = `{"type":"object","properties":{"x":{"$ref":"#/$defs/missing"}}}`
)

// recordingClient is one upstream that records the exact forwarded bytes and
// answers with a complete upstream CallToolResult.
type recordingClient struct {
	mu    sync.Mutex
	calls []recordedCall
}

type recordedCall struct {
	serverID string
	rawName  string
	args     string
}

func (c *recordingClient) CallTool(_ context.Context, _, serverID, rawName string, args json.RawMessage) (json.RawMessage, *core.UpstreamOutcome) {
	c.mu.Lock()
	c.calls = append(c.calls, recordedCall{serverID: serverID, rawName: rawName, args: string(args)})
	c.mu.Unlock()
	return json.RawMessage(`{"content":[{"type":"text","text":"upstream body"}],"structuredContent":{"rows":1},"isError":false}`), nil
}

func (c *recordingClient) recorded() []recordedCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]recordedCall(nil), c.calls...)
}

// toolHarness is the live runtime plus the recorded upstream for a two-tool
// catalog: alpha/read (supported schema) and alpha/write (unsupported schema).
type toolHarness struct {
	runtime    *service.Runtime
	assembly   *service.Assembly
	upstream   *recordingClient
	readName   string
	writeName  string
	surface    *service.ToolSurface
	server     *mcp.Server
	session    *mcp.ClientSession
	serverInit *mcp.Implementation
}

func newToolHarnessWithDefinitions(t *testing.T, cfg *config.Config, surfaceCfg service.ToolSurfaceConfig, defs []core.ToolDefinition) *toolHarness {
	t.Helper()
	upstream := &recordingClient{}
	runtime := service.NewRuntime()
	assembly, err := runtime.Reload(cfg, service.ReloadDeps{
		Discover: func(config.Server) ([]core.ToolDefinition, error) { return defs, nil },
		NewClient: func(config.Server) (core.Upstream, error) {
			return upstream, nil
		},
		Now:                 func() time.Time { return time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC) },
		ModelIdentityDigest: strings.Repeat("a", 64),
		Persist:             func(*storage.Snapshot) error { return nil },
	})
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}

	return startToolHarness(t, runtime, assembly, upstream, surfaceCfg)
}

func newToolHarness(t *testing.T, cfg *config.Config, surfaceCfg service.ToolSurfaceConfig) *toolHarness {
	t.Helper()
	defs := []core.ToolDefinition{
		{RawName: "read", Description: "read one row", InputSchema: json.RawMessage(toolReadSchema)},
		{RawName: "write", Description: "write one row", InputSchema: json.RawMessage(toolUnsupportedSchema)},
	}
	return newToolHarnessWithDefinitions(t, cfg, surfaceCfg, defs)
}

// newAdmittedToolHarness is the cold-start sibling of newToolHarness: the
// runtime admits the given durable snapshot instead of discovering upstreams,
// so the surface's own tools answer from the restored catalog and vectors.
func newAdmittedToolHarness(t *testing.T, cfg *config.Config, surfaceCfg service.ToolSurfaceConfig, durable *storage.Snapshot, modelDigest string) *toolHarness {
	t.Helper()
	upstream := &recordingClient{}
	runtime := service.NewRuntime()
	assembly, err := runtime.AdmitStartup(cfg, service.ReloadDeps{
		Discover: func(config.Server) ([]core.ToolDefinition, error) {
			return nil, errors.New("a startup admission must not discover")
		},
		NewClient: func(config.Server) (core.Upstream, error) {
			return upstream, nil
		},
		Now:                 func() time.Time { return time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC) },
		ModelIdentityDigest: modelDigest,
		Persist:             func(*storage.Snapshot) error { return nil },
	}, durable)
	if err != nil {
		t.Fatalf("AdmitStartup: %v", err)
	}
	return startToolHarness(t, runtime, assembly, upstream, surfaceCfg)
}

// startToolHarness registers the surface on an authenticated inbound server and
// connects a client session to it.
func startToolHarness(t *testing.T, runtime *service.Runtime, assembly *service.Assembly, upstream *recordingClient, surfaceCfg service.ToolSurfaceConfig) *toolHarness {
	t.Helper()
	harness := &toolHarness{
		runtime:    runtime,
		assembly:   assembly,
		upstream:   upstream,
		readName:   core.PublicName("alpha", "read"),
		writeName:  core.PublicName("alpha", "write"),
		serverInit: &mcp.Implementation{Name: "mcp-bridge", Version: "test"},
	}
	surface, err := service.NewToolSurface(runtime, surfaceCfg)
	if err != nil {
		t.Fatalf("NewToolSurface: %v", err)
	}
	harness.surface = surface

	server := mcp.NewServer(harness.serverInit, nil)
	if err := surface.Register(server); err != nil {
		t.Fatalf("Register: %v", err)
	}
	harness.server = server
	if err := surface.Published(assembly); err != nil {
		t.Fatalf("Published: %v", err)
	}

	const token = "tool-harness-test-token"
	inbound := service.NewInboundHandler(
		service.NewStreamableHandler(server),
		[][32]byte{sha256.Sum256([]byte(token))},
		service.InboundOptions{Logger: surfaceCfg.Logger, ToolSurface: surface},
	)
	httpServer := httptest.NewServer(inbound)
	t.Cleanup(httpServer.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint:   httpServer.URL + "/mcp",
		HTTPClient: &http.Client{Transport: toolHarnessAuthTransport{base: http.DefaultTransport, token: token}},
		MaxRetries: -1,
	}
	session, err := client.Connect(context.Background(), transport, &mcp.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	harness.session = session
	t.Cleanup(func() { _ = session.Close() })
	return harness
}

type toolHarnessAuthTransport struct {
	base  http.RoundTripper
	token string
}

func (transport toolHarnessAuthTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	authorized := request.Clone(request.Context())
	authorized.Header = request.Header.Clone()
	authorized.Header.Set("Authorization", "Bearer "+transport.token)
	return transport.base.RoundTrip(authorized)

}

func (h *toolHarness) call(t *testing.T, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := h.callErr(name, args)
	if err != nil {
		t.Fatalf("tools/call %s: %v", name, err)
	}
	return result
}

// callErr is the concurrency-safe variant of call: it reports failures as a
// returned error so worker goroutines never invoke t.Fatal (which must only
// run on the test goroutine).
func (h *toolHarness) callErr(name string, args map[string]any) (*mcp.CallToolResult, error) {
	raw, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("marshal args: %w", err)
	}
	result, err := h.session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: json.RawMessage(raw)})
	if err != nil {
		return nil, fmt.Errorf("tools/call %s: %w", name, err)
	}
	return result, nil
}

func structured(t *testing.T, result *mcp.CallToolResult) map[string]any {
	t.Helper()
	payload, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structuredContent = %T (%v); want an object", result.StructuredContent, result.StructuredContent)
	}
	return payload
}

func canonical(t *testing.T, value any) string {
	t.Helper()
	if raw, ok := value.(json.RawMessage); ok {
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("decode raw JSON: %v", err)
		}
		value = decoded
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(raw)
}

type syncBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func (b *syncBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buffer.Reset()
}

func decodeToolLogRecords(t *testing.T, output string) []map[string]any {
	t.Helper()
	if strings.TrimSpace(output) == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	records := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("decode tool log record: %v", err)
		}
		records = append(records, record)
	}
	return records
}

func assertToolOperationLogs(t *testing.T, logs *syncBuffer, operation, outcome, reason, upstreamState string, admission bool) {
	t.Helper()
	var rawLogs string
	waitFor(t, "request-correlated http_complete", func() bool {
		rawLogs = logs.String()
		requestID := ""
		for _, record := range decodeToolLogRecords(t, rawLogs) {
			if record["event"] == "mcp_complete" {
				requestID, _ = record["request_id"].(string)
			}
			if record["event"] == "http_complete" && requestID != "" && record["request_id"] == requestID {
				return true
			}
		}
		return false
	})
	var completions, httpCompletions, backendCompletions []map[string]any
	for _, record := range decodeToolLogRecords(t, rawLogs) {
		switch record["event"] {
		case "mcp_complete":
			completions = append(completions, record)
		case "http_complete":
			httpCompletions = append(httpCompletions, record)
		case "backend_complete":
			backendCompletions = append(backendCompletions, record)
		}
	}
	if len(completions) != 1 {
		t.Fatalf("mcp_complete records = %d; want exactly one: %s", len(completions), rawLogs)
	}
	completion := completions[0]
	for key, want := range map[string]any{
		"schema_version": float64(1), "service": "mcp-bridge",
		"event": "mcp_complete", "operation": operation, "outcome": outcome,
	} {
		if completion[key] != want {
			t.Errorf("mcp_complete %s = %v; want %v", key, completion[key], want)
		}
	}
	if level, ok := completion["level"].(string); !ok || (level != "info" && level != "warn") {
		t.Errorf("mcp_complete level = %v; want info or warn", completion["level"])
	}
	if message, ok := completion["message"].(string); !ok || message == "" {
		t.Errorf("mcp_complete message = %v; want non-empty", completion["message"])
	}
	if reason == "none" {
		if _, exists := completion["error"]; exists {
			t.Errorf("successful completion carries an error: %v", completion["error"])
		}
	} else if err, ok := completion["error"].(map[string]any); !ok || err["code"] != reason || len(err) != 4 {
		t.Errorf("mcp_complete error = %#v; want nested code %q and four-field shape", completion["error"], reason)
	}
	if _, exists := completion["reason"]; exists {
		t.Error("flat reason must not be emitted")
	}
	if admission && completion["stage"] != "tool_admission" {
		t.Errorf("mcp_complete stage = %v; want tool_admission", completion["stage"])
	}
	requestID, ok := completion["request_id"].(string)
	if !ok || len(requestID) != 32 {
		t.Fatalf("mcp_complete request_id = %v; want the 32-character server-owned request ID", completion["request_id"])
	}
	matchingHTTP := 0
	for _, record := range httpCompletions {
		if record["request_id"] != requestID {
			continue
		}
		matchingHTTP++
		if record["upstream_state"] != upstreamState {
			t.Errorf("http_complete upstream_state = %v; want %q", record["upstream_state"], upstreamState)
		}
	}
	if matchingHTTP != 1 {
		t.Errorf("http_complete records joined to request %q = %d; want exactly one: %s", requestID, matchingHTTP, rawLogs)
	}
	wantBackend := operation == "call_tool" && upstreamState != "not_attempted"
	matchingBackend := 0
	for _, record := range backendCompletions {
		if record["request_id"] != requestID {
			continue
		}
		matchingBackend++
		if record["operation"] != "call_tool" || record["schema_version"] != float64(1) ||
			record["service"] != "mcp-bridge" || record["event"] != "backend_complete" {
			t.Errorf("invalid backend completion envelope: %#v", record)
		}
		if message, ok := record["message"].(string); !ok || message == "" {
			t.Errorf("backend_complete message = %v; want non-empty", record["message"])
		}
	}
	wantBackendCount := 0
	if wantBackend {
		wantBackendCount = 1
	}
	if matchingBackend != wantBackendCount {
		t.Errorf("backend_complete records joined to request %q = %d; want %d: %s", requestID, matchingBackend, wantBackendCount, rawLogs)
	}
}

func TestToolSurfaceDescribeCatalogRevisionAdmission(t *testing.T) {
	t.Setenv("TOOLS_ALPHA_SECRET", "alpha-secret")
	t.Setenv("TOOLS_CLIENT_TOKEN", "client-token")
	var logs syncBuffer
	h := newToolHarness(t, parseReloadConfig(t, toolSurfaceConfig), service.ToolSurfaceConfig{
		Model:  search.ModelIdentity{Provider: "openai", Model: "embed-1", Dimensions: 4, FormatVersion: "v1"},
		Logger: observe.NewLogger(&logs, &logs),
	})
	currentRevision := h.assembly.CatalogRevision()
	cases := []struct {
		name            string
		revision        string
		includeRevision bool
		wantCode        string
		wantReason      string
	}{
		{name: "omitted", wantReason: "none"},
		{name: "empty", includeRevision: true, revision: "", wantReason: "none"},
		{name: "matching", includeRevision: true, revision: currentRevision, wantReason: "none"},
		{name: "stale", includeRevision: true, revision: strings.Repeat("b", 64), wantCode: "catalog_changed", wantReason: "catalog_changed"},
		{name: "malformed", includeRevision: true, revision: "not-hex", wantCode: "invalid_arguments", wantReason: "invalid_arguments"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := map[string]any{"tool_name": h.readName}
			if tc.includeRevision {
				args["catalog_revision"] = tc.revision
			}
			logs.Reset()
			result := h.call(t, catalog.ToolDescribeTool, args)
			if tc.wantCode == "" {
				if result.IsError {
					t.Fatalf("describe_tool rejected %s revision: %s", tc.name, canonical(t, structured(t, result)))
				}
				if got := structured(t, result)["catalog_revision"]; got != currentRevision {
					t.Fatalf("describe revision = %v; want current revision %s", got, currentRevision)
				}
				assertToolOperationLogs(t, &logs, "describe_tool", "success", "none", "not_attempted", false)
				return
			}
			if !result.IsError {
				t.Fatalf("describe_tool accepted %s revision", tc.name)
			}
			if got := structured(t, result)["code"]; got != tc.wantCode {
				t.Fatalf("describe code = %v; want %q", got, tc.wantCode)
			}
			assertToolOperationLogs(t, &logs, "describe_tool", "error", tc.wantReason, "not_attempted", true)
		})
	}
	if calls := h.upstream.recorded(); len(calls) != 0 {
		t.Fatalf("describe_tool dispatched upstream calls: %+v", calls)
	}
}

func TestToolSurfaceCallCatalogRevisionAdmission(t *testing.T) {
	t.Setenv("TOOLS_ALPHA_SECRET", "alpha-secret")
	t.Setenv("TOOLS_CLIENT_TOKEN", "client-token")
	var logs syncBuffer
	h := newToolHarness(t, parseReloadConfig(t, toolSurfaceConfig), service.ToolSurfaceConfig{
		Model:  search.ModelIdentity{Provider: "openai", Model: "embed-1", Dimensions: 4, FormatVersion: "v1"},
		Logger: observe.NewLogger(&logs, &logs),
	})
	currentRevision := h.assembly.CatalogRevision()
	cases := []struct {
		name            string
		revision        string
		includeRevision bool
		wantCode        string
		wantReason      string
	}{
		{name: "omitted", wantReason: "none"},
		{name: "empty", includeRevision: true, revision: "", wantReason: "none"},
		{name: "matching", includeRevision: true, revision: currentRevision, wantReason: "none"},
		{name: "stale", includeRevision: true, revision: strings.Repeat("b", 64), wantCode: "catalog_changed", wantReason: "catalog_changed"},
		{name: "malformed", includeRevision: true, revision: "not-hex", wantCode: "invalid_arguments", wantReason: "invalid_arguments"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := map[string]any{
				"tool_name": h.readName,
				"arguments": map[string]any{"limit": 1},
			}
			if tc.includeRevision {
				args["catalog_revision"] = tc.revision
			}
			before := len(h.upstream.recorded())
			logs.Reset()
			result := h.call(t, catalog.ToolCallTool, args)
			if tc.wantCode == "" {
				if result.IsError {
					t.Fatalf("call_tool rejected %s revision: %s", tc.name, canonical(t, structured(t, result)))
				}
				for _, key := range []string{"catalog_revision", "catalog_status", "availability"} {
					if _, ok := structured(t, result)[key]; ok {
						t.Fatalf("call_tool payload unexpectedly contains pruned field %q: %v", key, structured(t, result))
					}
				}
				if calls := len(h.upstream.recorded()); calls != before+1 {
					t.Fatalf("upstream calls = %d; want %d", calls, before+1)
				}
				assertToolOperationLogs(t, &logs, "call_tool", "success", "none", "succeeded", false)
				return
			}
			if !result.IsError {
				t.Fatalf("call_tool accepted %s revision", tc.name)
			}
			if got := structured(t, result)["code"]; got != tc.wantCode {
				t.Fatalf("call code = %v; want %q", got, tc.wantCode)
			}
			if calls := len(h.upstream.recorded()); calls != before {
				t.Fatalf("rejected call dispatched upstream: got %d calls, want %d", calls, before)
			}
			assertToolOperationLogs(t, &logs, "call_tool", "error", tc.wantReason, "not_attempted", true)
		})
	}
}

// The bridge advertises its five built-ins with the exact schemas, plus its configured
// pin (PR-R7/PR-AC1).
func assertAdvertisedBuiltinsAndPin(t *testing.T, tools []*mcp.Tool, pinName string, topKDefault, topKMax int) {
	t.Helper()

	wantBuiltins := map[string]map[string]any{
		catalog.ToolSearchTools:  catalog.SearchToolsInputSchema(topKDefault, topKMax),
		catalog.ToolDescribeTool: catalog.DescribeToolInputSchema(),
		catalog.ToolCallTool:     catalog.CallToolInputSchema(),
		catalog.ToolRetrieveResult: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"handle": map[string]any{"type": "string", "pattern": "^[A-Za-z0-9_-]{43}$"},
				"page":   map[string]any{"type": "integer", "minimum": 1, "default": 1},
			},
			"required":             []string{"handle"},
			"additionalProperties": false,
		},
		catalog.ToolExposeTools: catalog.ExposeToolsInputSchema(),
	}
	seen := make(map[string]bool, len(tools))
	for _, tool := range tools {
		if seen[tool.Name] {
			t.Fatalf("advertised tool %q more than once", tool.Name)
		}
		seen[tool.Name] = true

		schema, builtin := wantBuiltins[tool.Name]
		if !builtin && tool.Name != pinName {
			t.Fatalf("unexpected advertised tool %q", tool.Name)
		}
		if builtin {
			if got := canonical(t, tool.InputSchema); got != canonical(t, schema) {
				t.Fatalf("%s inputSchema = %s; want %s", tool.Name, got, canonical(t, schema))
			}
		}
	}
	for name := range wantBuiltins {
		if !seen[name] {
			t.Errorf("missing advertised built-in %q", name)
		}
	}
	if !seen[pinName] {
		t.Errorf("missing advertised pinned tool %q", pinName)
	}
}

func TestToolSurfaceAdvertisesExactlyItsFiveTools(t *testing.T) {
	t.Setenv("TOOLS_ALPHA_SECRET", "alpha-secret")
	t.Setenv("TOOLS_CLIENT_TOKEN", "client-token")
	h := newToolHarness(t, parseReloadConfig(t, toolSurfaceConfig), service.ToolSurfaceConfig{
		Model: search.ModelIdentity{Provider: "openai", Model: "embed-1", Dimensions: 4, FormatVersion: "v1"},
	})

	listed, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	assertAdvertisedBuiltinsAndPin(t, listed.Tools, config.WireToolName("alpha", "read"), 3, 4)
}

// call_tool wraps exactly one upstream execution in the provenance envelope and
// forwards the client's exact argument bytes (T-05b, plan §4.4/§4.5).
func TestToolSurfaceCallToolReturnsProvenanceEnvelope(t *testing.T) {
	t.Setenv("TOOLS_ALPHA_SECRET", "alpha-secret")
	t.Setenv("TOOLS_CLIENT_TOKEN", "client-token")
	h := newToolHarness(t, parseReloadConfig(t, toolSurfaceConfig), service.ToolSurfaceConfig{
		Model: search.ModelIdentity{Provider: "openai", Model: "embed-1", Dimensions: 4, FormatVersion: "v1"},
	})

	result := h.call(t, catalog.ToolCallTool, map[string]any{
		"tool_name":        h.readName,
		"catalog_revision": h.assembly.CatalogRevision(),
		"arguments":        map[string]any{"limit": 1},
	})
	if result.IsError {
		t.Fatalf("call_tool reported an error: %s", canonical(t, structured(t, result)))
	}
	payload := structured(t, result)

	for _, key := range []string{"server", "tool_name", "request_id", "upstream_result"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("envelope is missing required field %q: %v", key, payload)
		}
	}
	for _, key := range []string{"catalog_revision", "catalog_status", "availability"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("envelope unexpectedly contains pruned field %q: %v", key, payload)
		}
	}
	if payload["server"] != "alpha" {
		t.Fatalf("envelope server = %v; want alpha", payload["server"])
	}
	if payload["tool_name"] != h.readName {
		t.Fatalf("envelope tool_name = %v; want %q", payload["tool_name"], h.readName)
	}
	requestID, _ := payload["request_id"].(string)
	if requestID == "" {
		t.Fatalf("envelope request_id = %v; want a non-empty id", payload["request_id"])
	}

	upstream, ok := payload["upstream_result"].(map[string]any)
	if !ok {
		t.Fatalf("envelope upstream_result = %T; want the original CallToolResult", payload["upstream_result"])
	}
	if upstream["isError"] != false {
		t.Fatalf("upstream_result.isError = %v; want false", upstream["isError"])
	}
	if rows, ok := upstream["structuredContent"].(map[string]any); !ok || rows["rows"] != float64(1) {
		t.Fatalf("upstream_result.structuredContent = %v; want {rows:1}", upstream["structuredContent"])
	}
	if len(result.Content) == 0 {
		t.Fatal("the outer result dropped the upstream content blocks")
	}

	calls := h.upstream.recorded()
	if len(calls) != 1 {
		t.Fatalf("upstream executions = %d; want exactly 1", len(calls))
	}
	if calls[0].serverID != "alpha" || calls[0].rawName != "read" || calls[0].args != `{"limit":1}` {
		t.Fatalf("forwarded call = %+v; want alpha/read with the exact client bytes", calls[0])
	}

	// The same request re-issued is a new request id, never a cached result.
	second := structured(t, h.call(t, catalog.ToolCallTool, map[string]any{
		"tool_name":        h.readName,
		"catalog_revision": h.assembly.CatalogRevision(),
		"arguments":        map[string]any{"limit": 1},
	}))
	if second["request_id"] == payload["request_id"] {
		t.Fatal("two executions reused one request id")
	}
	if len(h.upstream.recorded()) != 2 {
		t.Fatalf("upstream executions = %d after two calls; want 2", len(h.upstream.recorded()))
	}
}

// Admission failures never reach upstream: a stale revision, an unknown tool,
// and schema-invalid arguments each answer with the structured code.
func TestToolSurfaceCallToolRejectsBeforeAnyExecution(t *testing.T) {
	t.Setenv("TOOLS_ALPHA_SECRET", "alpha-secret")
	t.Setenv("TOOLS_CLIENT_TOKEN", "client-token")
	h := newToolHarness(t, parseReloadConfig(t, toolSurfaceConfig), service.ToolSurfaceConfig{
		Model: search.ModelIdentity{Provider: "openai", Model: "embed-1", Dimensions: 4, FormatVersion: "v1"},
	})

	cases := []struct {
		name string
		args map[string]any
		code string
	}{
		{
			name: "stale revision",
			args: map[string]any{"tool_name": h.readName, "catalog_revision": strings.Repeat("b", 64), "arguments": map[string]any{"limit": 1}},
			code: "catalog_changed",
		},
		{
			// Revision admission precedes validation: a stale expected
			// revision is never answered with a schema verdict.
			name: "stale revision with invalid arguments",
			args: map[string]any{"tool_name": h.readName, "catalog_revision": strings.Repeat("b", 64), "arguments": map[string]any{"extra": true}},
			code: "catalog_changed",
		},
		{
			name: "unknown tool",
			args: map[string]any{"tool_name": "t_" + strings.Repeat("a", 52), "catalog_revision": h.assembly.CatalogRevision(), "arguments": map[string]any{}},
			code: "unknown_tool",
		},
		{
			name: "missing required argument",
			args: map[string]any{"tool_name": h.readName, "catalog_revision": h.assembly.CatalogRevision(), "arguments": map[string]any{}},
			code: "invalid_arguments",
		},
		{
			name: "unknown argument field",
			args: map[string]any{"tool_name": h.readName, "catalog_revision": h.assembly.CatalogRevision(), "arguments": map[string]any{"limit": 1, "extra": true}},
			code: "invalid_arguments",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := h.call(t, catalog.ToolCallTool, tc.args)
			if !result.IsError {
				t.Fatalf("admission failure %q answered successfully: %s", tc.name, canonical(t, structured(t, result)))
			}
			payload := structured(t, result)
			if payload["code"] != tc.code {
				t.Fatalf("code = %v; want %q (%s)", payload["code"], tc.code, canonical(t, payload))
			}
			if payload["execution_state"] != core.ExecutionNotStarted {
				t.Fatalf("execution_state = %v; want %q", payload["execution_state"], core.ExecutionNotStarted)
			}
			if calls := h.upstream.recorded(); len(calls) != 0 {
				t.Fatalf("rejected %q still executed upstream: %+v", tc.name, calls)
			}
		})
	}
}

// describe_tool serves the stored exact schemas with validation_status, and an
// unresolvable stored schema is reported unsupported without rewriting it.
func TestToolSurfaceDescribeToolServesExactStoredSchema(t *testing.T) {
	t.Setenv("TOOLS_ALPHA_SECRET", "alpha-secret")
	t.Setenv("TOOLS_CLIENT_TOKEN", "client-token")
	const outputSchema = `{"type":"object","properties":{"rows":{"type":"integer"}}}`
	definitions := []core.ToolDefinition{
		{
			RawName:      "read",
			Description:  "read one row",
			InputSchema:  json.RawMessage(toolReadSchema),
			OutputSchema: json.RawMessage(outputSchema),
			Metadata: core.ToolMetadata{
				Title:       "read one row",
				Annotations: json.RawMessage(`{"readOnlyHint":true}`),
				Meta:        json.RawMessage(`{"source":"test"}`),
			},
		},
		{RawName: "write", Description: "write one row", InputSchema: json.RawMessage(toolUnsupportedSchema)},
	}
	metadataConfig := strings.Replace(toolSurfaceConfig, "    endpoint: https://alpha.invalid/mcp\n",
		"    endpoint: https://alpha.invalid/mcp\n    tool_metadata:\n      defaults:\n        title: effective read\n", 1)
	h := newToolHarnessWithDefinitions(t, parseReloadConfig(t, metadataConfig), service.ToolSurfaceConfig{
		Model: search.ModelIdentity{Provider: "openai", Model: "embed-1", Dimensions: 4, FormatVersion: "v1"},
	}, definitions)

	result := h.call(t, catalog.ToolDescribeTool, map[string]any{
		"tool_name":        h.readName,
		"catalog_revision": h.assembly.CatalogRevision(),
	})
	if result.IsError {
		t.Fatalf("describe_tool reported an error: %s", canonical(t, structured(t, result)))
	}
	payload := structured(t, result)
	for _, key := range []string{"server", "catalog_revision", "tool_name", "upstream_name", "description", "pinned", "availability", "validation_status", "input_schema"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("describe payload is missing required field %q: %v", key, payload)
		}
	}
	for _, key := range []string{"last_success_at", "catalog_status"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("describe payload unexpectedly contains pruned field %q: %v", key, payload)
		}
	}
	if payload["server"] != "alpha" || payload["upstream_name"] != "read" {
		t.Fatalf("describe provenance = %v; want alpha/read", payload)
	}
	if payload["description"] != "read one row" {
		t.Fatalf("description = %v; want the stored description", payload["description"])
	}
	if payload["validation_status"] != "supported" {
		t.Fatalf("validation_status = %v; want supported", payload["validation_status"])
	}
	if got := canonical(t, payload["input_schema"]); got != canonical(t, json.RawMessage(toolReadSchema)) {
		t.Fatalf("input_schema = %s; want the stored exact schema", got)
	}
	if payload["pinned"] != true {
		t.Fatalf("pinned = %v; want true for the configured pin", payload["pinned"])
	}
	if got := canonical(t, payload["output_schema"]); got != canonical(t, json.RawMessage(outputSchema)) {
		t.Fatalf("output_schema = %s; want the stored exact schema", got)
	}
	upstreamMetadata, ok := payload["upstream_metadata"].(map[string]any)
	if !ok || upstreamMetadata["title"] != "read one row" {
		t.Fatalf("upstream_metadata = %#v; want the source metadata", payload["upstream_metadata"])
	}
	effectiveMetadata, ok := payload["metadata"].(map[string]any)
	if !ok || effectiveMetadata["title"] != "effective read" {
		t.Fatalf("metadata = %#v; want the effective pin metadata", payload["metadata"])
	}
	if _, ok := payload["metadata_override_fields"]; !ok {
		t.Fatalf("describe payload omitted metadata_override_fields: %v", payload)
	}

	unsupported := structured(t, h.call(t, catalog.ToolDescribeTool, map[string]any{
		"tool_name":        h.writeName,
		"catalog_revision": h.assembly.CatalogRevision(),
	}))
	if unsupported["validation_status"] != "unsupported" {
		t.Fatalf("validation_status = %v; want unsupported for an unresolvable schema", unsupported["validation_status"])
	}
	if reason, _ := unsupported["validation_reason"].(string); reason == "" {
		t.Fatalf("unsupported describe carries no reason: %v", unsupported)
	}
	if got := canonical(t, unsupported["input_schema"]); got != canonical(t, json.RawMessage(toolUnsupportedSchema)) {
		t.Fatalf("unsupported input_schema = %s; want the stored schema unchanged", got)
	}

	mismatch := h.call(t, catalog.ToolDescribeTool, map[string]any{
		"tool_name":        h.readName,
		"catalog_revision": strings.Repeat("b", 64),
	})
	if !mismatch.IsError {
		t.Fatal("describe_tool accepted a stale revision")
	}
	if code := structured(t, mismatch)["code"]; code != "catalog_changed" {
		t.Fatalf("stale describe code = %v; want catalog_changed", code)
	}
}

// search_tools answers from the durable snapshot: keyword ranking and an
// explicit degradation without a provider.
func TestToolSurfaceSearchToolsReturnsRankedProvenance(t *testing.T) {
	t.Setenv("TOOLS_ALPHA_SECRET", "alpha-secret")
	t.Setenv("TOOLS_CLIENT_TOKEN", "client-token")
	h := newToolHarness(t, parseReloadConfig(t, toolSurfaceConfig), service.ToolSurfaceConfig{
		Model: search.ModelIdentity{Provider: "openai", Model: "embed-1", Dimensions: 4, FormatVersion: "v1"},
	})

	result := h.call(t, catalog.ToolSearchTools, map[string]any{"query": "read row"})
	if result.IsError {
		t.Fatalf("search_tools reported an error: %s", canonical(t, structured(t, result)))
	}
	if len(result.Content) != 1 {
		t.Fatalf("search_tools content length = %d; want 1", len(result.Content))
	}
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok || textContent.Text == "" {
		t.Fatalf("search_tools content[0] = %T (%v); want non-empty *mcp.TextContent", result.Content[0], result.Content[0])
	}
	var decodedSearch map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &decodedSearch); err != nil {
		t.Fatalf("unmarshal search_tools text content: %v", err)
	}
	if decodedSearch["catalog_revision"] != h.assembly.CatalogRevision() {
		t.Fatalf("decoded search revision = %v; want %v", decodedSearch["catalog_revision"], h.assembly.CatalogRevision())
	}
	payload := structured(t, result)
	if payload["catalog_revision"] != h.assembly.CatalogRevision() {
		t.Fatalf("search revision = %v; want the published revision", payload["catalog_revision"])
	}
	if payload["mode"] != search.ModeKeyword {
		t.Fatalf("mode = %v; want %q without an embed provider", payload["mode"], search.ModeKeyword)
	}
	reasons, ok := payload["degraded_reasons"].([]any)
	if !ok || len(reasons) == 0 {
		t.Fatalf("degraded_reasons = %v; want a finite reason without a provider", payload["degraded_reasons"])
	}
	results, ok := payload["results"].([]any)
	if !ok || len(results) != 2 {
		t.Fatalf("results = %v; want both matching tools", payload["results"])
	}
	match, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("first search result = %T; want an object", results[0])
	}
	if match["tool_name"] != "alpha__read" {
		t.Fatalf("first result tool_name = %v; want alpha__read first under its wire name", match["tool_name"])
	}
	for _, key := range []string{"catalog_revision", "mode", "degraded_reasons", "results"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("search payload is missing required field %q: %v", key, payload)
		}
	}
	if _, ok := payload["backends"]; ok {
		t.Fatalf("search payload unexpectedly contains backends: %v", payload["backends"])
	}
	for _, result := range results {
		row, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("search result = %T; want an object", result)
		}
		for _, key := range []string{"tool_name", "description", "score"} {
			if _, ok := row[key]; !ok {
				t.Fatalf("search result is missing required field %q: %v", key, row)
			}
		}
		score, ok := row["score"].(float64)
		if !ok || score <= 0 {
			t.Fatalf("search result score = %v; want a positive score", row["score"])
		}
		for _, key := range []string{"last_success_at", "catalog_status", "catalog_revision"} {
			if _, ok := row[key]; ok {
				t.Fatalf("search result unexpectedly contains pruned field %q: %v", key, row)
			}
		}
	}

	filtered := h.call(t, catalog.ToolSearchTools, map[string]any{"query": "read row", "servers": []string{"gamma"}})
	if !filtered.IsError {
		t.Fatal("search_tools accepted an unconfigured server filter")
	}
	if code := structured(t, filtered)["code"]; code != string(search.CodeInvalidFilter) {
		t.Fatalf("filter rejection code = %v; want %q", code, search.CodeInvalidFilter)
	}
}

func TestToolSurfaceSearchToolsUsesWireBoundaryAndFallbackNames(t *testing.T) {
	t.Setenv("TOOLS_ALPHA_SECRET", "alpha-secret")
	t.Setenv("TOOLS_CLIENT_TOKEN", "client-token")
	longRawName := strings.Repeat("x", 64)
	boundaryRawName := strings.Repeat("x", 57)
	definitions := []core.ToolDefinition{
		{RawName: "read", Description: "read one row", InputSchema: json.RawMessage(toolReadSchema)},
		{RawName: "read.v2", Description: "candidate invalid", InputSchema: json.RawMessage(toolReadSchema)},
		{RawName: longRawName, Description: "candidate long", InputSchema: json.RawMessage(toolReadSchema)},
		{RawName: boundaryRawName, Description: "candidate boundary", InputSchema: json.RawMessage(toolReadSchema)},
	}
	h := newToolHarnessWithDefinitions(t, parseReloadConfig(t, toolSurfaceConfig), service.ToolSurfaceConfig{
		Model: search.ModelIdentity{Provider: "openai", Model: "embed-1", Dimensions: 4, FormatVersion: "v1"},
	}, definitions)

	result := h.call(t, catalog.ToolSearchTools, map[string]any{"query": "candidate"})
	if result.IsError {
		t.Fatalf("search_tools reported an error: %s", canonical(t, structured(t, result)))
	}
	results, ok := structured(t, result)["results"].([]any)
	if !ok || len(results) != 3 {
		t.Fatalf("search results = %v; want the invalid, long, and boundary tools", structured(t, result)["results"])
	}
	expected := map[string]struct{}{
		core.PublicName("alpha", "read.v2"): {},
		core.PublicName("alpha", longRawName): {},
		"alpha__" + boundaryRawName: {},
	}
	for _, item := range results {
		entry, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("search result = %T; want an object", item)
		}
		toolName, ok := entry["tool_name"].(string)
		if !ok {
			t.Errorf("search result tool_name = %v; want a string", entry["tool_name"])
			continue
		}
		if _, found := expected[toolName]; !found {
			t.Errorf("search returned unexpected tool_name %q", toolName)
			continue
		}
		delete(expected, toolName)
	}
	if len(expected) != 0 {
		t.Errorf("search omitted fallback tool names: %v", expected)
	}
}
func matchingSearchDefinitions(term string, count int) []core.ToolDefinition {
	definitions := []core.ToolDefinition{
		{RawName: "read", Description: "pinned read", InputSchema: json.RawMessage(toolReadSchema)},
	}
	for index := range count {
		definitions = append(definitions, core.ToolDefinition{
			RawName:     fmt.Sprintf("%s_%d", term, index),
			Description: term,
			InputSchema: json.RawMessage(toolReadSchema),
		})
	}
	return definitions
}

func newCompactSearchHarness(t *testing.T, definitions []core.ToolDefinition, cfg search.Config) *toolHarness {
	t.Helper()
	t.Setenv("TOOLS_ALPHA_SECRET", "alpha-secret")
	t.Setenv("TOOLS_CLIENT_TOKEN", "client-token")
	return newToolHarnessWithDefinitions(t, parseReloadConfig(t, toolSurfaceConfig), service.ToolSurfaceConfig{
		Search: cfg,
		Model:  search.ModelIdentity{Provider: "openai", Model: "embed-1", Dimensions: 4, FormatVersion: "v1"},
	}, definitions)
}

func TestToolSurfaceSearchToolsCompactHitFields(t *testing.T) {
	h := newCompactSearchHarness(t, []core.ToolDefinition{
		{RawName: "read", Description: "pinned read", InputSchema: json.RawMessage(toolReadSchema)},
		{RawName: "compact", Description: "first line\nsecond\tline", InputSchema: json.RawMessage(toolReadSchema)},
	}, search.Config{})

	payload := structured(t, h.call(t, catalog.ToolSearchTools, map[string]any{"query": "compact"}))
	results, ok := payload["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("search results = %v; want one compact hit", payload["results"])
	}
	row, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("search hit = %T; want an object", results[0])
	}
	if len(row) != 3 {
		t.Fatalf("search hit fields = %v; want exactly description, score, tool_name", row)
	}
	for _, key := range []string{"description", "score", "tool_name"} {
		if _, ok := row[key]; !ok {
			t.Fatalf("search hit is missing %q: %v", key, row)
		}
	}
	if row["description"] != "first line second line" {
		t.Fatalf("search hit description = %q; want normalized one-line text", row["description"])
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal search payload: %v", err)
	}
	for _, key := range []string{"input_schema", "output_schema"} {
		if strings.Contains(string(encoded), key) {
			t.Fatalf("search payload contains schema field %q: %s", key, encoded)
		}
	}
}

func TestToolSurfaceSearchToolsDefaultCount(t *testing.T) {
	h := newCompactSearchHarness(t, matchingSearchDefinitions("needle", 5), search.Config{
		TopKDefault: 5,
		TopKMax:     20,
	})

	result := h.call(t, catalog.ToolSearchTools, map[string]any{"query": "needle"})
	if result.IsError {
		t.Fatalf("search_tools reported an error: %s", canonical(t, structured(t, result)))
	}
	payload := structured(t, result)
	results, ok := payload["results"].([]any)
	if !ok || len(results) != 3 {
		t.Fatalf("default search result count = %v; want 3", payload["results"])
	}
}

func TestToolSurfaceSearchToolsRejectsTopKAboveCap(t *testing.T) {
	h := newCompactSearchHarness(t, matchingSearchDefinitions("needle", 5), search.Config{
		TopKDefault: 5,
		TopKMax:     20,
	})

	result := h.call(t, catalog.ToolSearchTools, map[string]any{"query": "needle", "top_k": 5})
	if !result.IsError {
		t.Fatal("search_tools accepted top_k=5 above the bridge cap")
	}
	payload := structured(t, result)
	if payload["code"] != "invalid_arguments" {
		t.Fatalf("top_k=5 error code = %v; want invalid_arguments", payload["code"])
	}
	if _, ok := payload["results"]; ok {
		t.Fatalf("rejected search unexpectedly returned results: %v", payload)
	}
}

func TestToolSurfaceSearchToolsClipMetadata(t *testing.T) {
	definitions := matchingSearchDefinitions("needle", 6)
	definitions = append(definitions,
		core.ToolDefinition{RawName: "few_a", Description: "few", InputSchema: json.RawMessage(toolReadSchema)},
		core.ToolDefinition{RawName: "few_b", Description: "few", InputSchema: json.RawMessage(toolReadSchema)},
	)
	h := newCompactSearchHarness(t, definitions, search.Config{TopKDefault: 3, TopKMax: 4})

	clipped := structured(t, h.call(t, catalog.ToolSearchTools, map[string]any{"query": "needle", "top_k": 4}))
	results, ok := clipped["results"].([]any)
	if !ok || len(results) != 4 {
		t.Fatalf("clipped result count = %v; want 4", clipped["results"])
	}
	if clipped["match_count"] != float64(6) || clipped["truncated"] != true {
		t.Fatalf("clipped metadata = match_count:%v truncated:%v; want 6/true", clipped["match_count"], clipped["truncated"])
	}

	unclipped := structured(t, h.call(t, catalog.ToolSearchTools, map[string]any{"query": "few", "top_k": 4}))
	unclippedResults, ok := unclipped["results"].([]any)
	if !ok || len(unclippedResults) != 2 {
		t.Fatalf("unclipped result count = %v; want 2", unclipped["results"])
	}
	for _, key := range []string{"match_count", "truncated"} {
		if _, ok := unclipped[key]; ok {
			t.Fatalf("unclipped payload unexpectedly contains %q: %v", key, unclipped)
		}
	}
}

func TestToolSurfaceSearchToolsDescriptionBounded(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		want      string
		truncated bool
	}{
		{
			name:      "controls",
			input:     strings.Repeat("verbose ", 8) + "\t\n\x00\x1f\x7f\u0085\u2028\u2029" + strings.Repeat(" suffix", 8),
			want:      strings.Repeat("verbose ", 4) + "v…",
			truncated: true,
		},
		{name: "quotes", input: strings.Repeat("\"", 24), want: strings.Repeat("\"", 16) + "…", truncated: true},
		{name: "exact", input: strings.Repeat("\"", 18), want: strings.Repeat("\"", 18)},
		{name: "over", input: strings.Repeat("a", 37), want: strings.Repeat("a", 33) + "…", truncated: true},
		{name: "unicode", input: "漢", want: "漢"},
		{name: "empty", input: "", want: ""},
		{name: "fitting", input: "alpha\nbeta", want: "alpha beta"},
	}
	definitions := []core.ToolDefinition{
		{RawName: "read", Description: "pinned read", InputSchema: json.RawMessage(toolReadSchema)},
	}
	for _, test := range cases {
		definitions = append(definitions, core.ToolDefinition{
			RawName:     test.name,
			Description: test.input,
			InputSchema: json.RawMessage(toolReadSchema),
		})
	}
	h := newCompactSearchHarness(t, definitions, search.Config{})

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			query := test.name
			result := h.call(t, catalog.ToolSearchTools, map[string]any{"query": query})
			if result.IsError {
				t.Fatalf("search_tools reported an error: %s", canonical(t, structured(t, result)))
			}
			payload := structured(t, result)
			results, ok := payload["results"].([]any)
			if !ok || len(results) != 1 {
				t.Fatalf("search results = %v; want one hit for %q", payload["results"], query)
			}
			row, ok := results[0].(map[string]any)
			if !ok {
				t.Fatalf("search hit = %T; want an object", results[0])
			}
			description, ok := row["description"].(string)
			if !ok {
				t.Fatalf("search description = %T; want a string", row["description"])
			}
			if description != test.want {
				t.Fatalf("search description = %q; want %q", description, test.want)
			}
			encoded, err := json.Marshal(description)
			if err != nil {
				t.Fatalf("marshal search description: %v", err)
			}
			encodedBytes := len(encoded) - 2
			if encodedBytes > 36 {
				t.Fatalf("encoded description bytes = %d; want <=36 (%q)", encodedBytes, description)
			}
			if got := strings.HasSuffix(description, "…"); got != test.truncated {
				t.Fatalf("description truncated = %v; want %v (%q)", got, test.truncated, description)
			}
			if test.name == "controls" {
				for _, r := range description {
					if r <= 0x1f || (r >= 0x7f && r <= 0x9f) || r == '\u2028' || r == '\u2029' {
						t.Fatalf("normalized description contains control/line-separator U+%04X: %q", r, description)
					}
				}
				repeated, ok := structured(t, h.call(t, catalog.ToolSearchTools, map[string]any{"query": query}))["results"].([]any)
				if !ok || len(repeated) != 1 {
					t.Fatalf("repeated search results = %v; want one hit", repeated)
				}
				repeatedRow, _ := repeated[0].(map[string]any)
				if repeatedRow["description"] != description {
					t.Fatalf("normalized description changed across calls: %q != %q", repeatedRow["description"], description)
				}
			}
		})
	}
}

func TestToolSurfaceSearchToolsByteBudget(t *testing.T) {
	tests := []struct {
		name                 string
		matchCount           int
		inputDescription     string
		wantDescription      string
		encodedDescriptionBy int
		clipped              bool
	}{
		{
			name:                 "exact max unmarked escape",
			matchCount:           4,
			inputDescription:     strings.Repeat("\"", 18),
			wantDescription:      strings.Repeat("\"", 18),
			encodedDescriptionBy: 36,
		},
		{
			name:                 "clipped degraded unmarked escape",
			matchCount:           6,
			inputDescription:     strings.Repeat("\"", 18),
			wantDescription:      strings.Repeat("\"", 18),
			encodedDescriptionBy: 36,
			clipped:              true,
		},
		{
			name:                 "clipped degraded truncated escape",
			matchCount:           6,
			inputDescription:     strings.Repeat("\"", 24),
			wantDescription:      strings.Repeat("\"", 16) + "…",
			encodedDescriptionBy: 35,
			clipped:              true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definitions := []core.ToolDefinition{
				{RawName: "read", Description: "pinned read", InputSchema: json.RawMessage(toolReadSchema)},
			}
			for index := range test.matchCount {
				definitions = append(definitions, core.ToolDefinition{
					RawName:     fmt.Sprintf("needle_%d_%s", index, strings.Repeat("x", 48)),
					Description: test.inputDescription,
					InputSchema: json.RawMessage(toolReadSchema),
				})
			}
			h := newCompactSearchHarness(t, definitions, search.Config{TopKDefault: 4, TopKMax: 4})

			result := h.call(t, catalog.ToolSearchTools, map[string]any{"query": "needle", "top_k": 4})
			if result.IsError {
				t.Fatalf("search_tools reported an error: %s", canonical(t, structured(t, result)))
			}
			payload := structured(t, result)
			results, ok := payload["results"].([]any)
			if !ok || len(results) != 4 {
				t.Fatalf("search result count = %v; want 4", payload["results"])
			}
			if test.clipped {
				if payload["match_count"] != float64(test.matchCount) || payload["truncated"] != true {
					t.Fatalf("clip metadata = match_count:%v truncated:%v; want %d/true", payload["match_count"], payload["truncated"], test.matchCount)
				}
				if payload["mode"] != search.ModeKeyword {
					t.Fatalf("search mode = %v; want degraded keyword mode", payload["mode"])
				}
				reasons, ok := payload["degraded_reasons"].([]any)
				if !ok || len(reasons) == 0 {
					t.Fatalf("degraded_reasons = %v; want a degradation reason", payload["degraded_reasons"])
				}
			} else {
				for _, key := range []string{"match_count", "truncated"} {
					if _, ok := payload[key]; ok {
						t.Fatalf("unclipped payload unexpectedly contains %q: %v", key, payload)
					}
				}
			}

			for _, rawResult := range results {
				row, ok := rawResult.(map[string]any)
				if !ok {
					t.Fatalf("search hit = %T; want an object", rawResult)
				}
				toolName, ok := row["tool_name"].(string)
				if !ok || len(toolName) != 64 {
					t.Fatalf("search hit tool_name = %v; want a 64-character wire name", row["tool_name"])
				}
				description, ok := row["description"].(string)
				if !ok || description != test.wantDescription {
					t.Fatalf("search hit description = %q; want %q", row["description"], test.wantDescription)
				}
				encodedDescription, err := json.Marshal(description)
				if err != nil {
					t.Fatalf("marshal search description: %v", err)
				}
				if got := len(encodedDescription) - 2; got != test.encodedDescriptionBy {
					t.Fatalf("encoded description bytes = %d; want %d", got, test.encodedDescriptionBy)
				}
			}

			measured, err := measureResult(result)
			if err != nil {
				t.Fatalf("measure full CallToolResult: %v", err)
			}
			if measured > 2048 {
				t.Fatalf("serialized CallToolResult = %d bytes; want <=2048", measured)
			}
		})
	}
}

// measureResult mirrors the service package's package-private helper for this
// external-package end-to-end test.
func measureResult(result *mcp.CallToolResult) (int, error) {
	encoded, err := json.Marshal(result)
	if err != nil {
		return 0, err
	}
	return len(encoded), nil
}

// The advertised tool list remains stable across reload, with one of each built-in
// plus its configured pin.
func TestToolSurfaceSurvivesReloadWithoutDuplicatingTools(t *testing.T) {
	t.Setenv("TOOLS_ALPHA_SECRET", "alpha-secret")
	t.Setenv("TOOLS_CLIENT_TOKEN", "client-token")
	h := newToolHarness(t, parseReloadConfig(t, toolSurfaceConfig), service.ToolSurfaceConfig{
		Model: search.ModelIdentity{Provider: "openai", Model: "embed-1", Dimensions: 4, FormatVersion: "v1"},
	})
	before, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list before reload: %v", err)
	}
	after, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list after reload: %v", err)
	}
	if !reflect.DeepEqual(before.Tools, after.Tools) {
		t.Fatal("the advertised tool list changed without a reload")
	}
	assertAdvertisedBuiltinsAndPin(t, after.Tools, config.WireToolName("alpha", "read"), 3, 4)
}
func TestToolSurfaceDescribeToolReturnsTextContent(t *testing.T) {
	t.Setenv("TOOLS_ALPHA_SECRET", "alpha-secret")
	t.Setenv("TOOLS_CLIENT_TOKEN", "client-token")
	h := newToolHarness(t, parseReloadConfig(t, toolSurfaceConfig), service.ToolSurfaceConfig{
		Model: search.ModelIdentity{Provider: "openai", Model: "embed-1", Dimensions: 4, FormatVersion: "v1"},
	})

	result := h.call(t, catalog.ToolDescribeTool, map[string]any{
		"tool_name":        h.readName,
		"catalog_revision": h.assembly.CatalogRevision(),
	})
	if result.IsError {
		t.Fatalf("describe_tool reported an error: %s", canonical(t, structured(t, result)))
	}
	if len(result.Content) != 1 {
		t.Fatalf("describe_tool content length = %d; want 1", len(result.Content))
	}
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok || textContent.Text == "" {
		t.Fatalf("describe_tool content[0] = %T (%v); want non-empty *mcp.TextContent", result.Content[0], result.Content[0])
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &decoded); err != nil {
		t.Fatalf("unmarshal describe_tool text content: %v", err)
	}
	if decoded["tool_name"] != h.readName || decoded["server"] != "alpha" || decoded["upstream_name"] != "read" {
		t.Fatalf("decoded describe fields = %v; want tool_name, server, and upstream_name matching payload", decoded)
	}
}

// TestToolSurfaceConcurrentSearcher races many concurrent search_tools calls
// against the searcher cache: every goroutine starts before the cache is
// populated, so they contend on the cache-miss rebuild and the cached store,
// and later rounds contend on the cache-hit read. A lost lock around
// t.cached/t.cacheKey surfaces as a data race under the race detector.
// Failures are collected per goroutine and asserted on the test goroutine,
// never with t.Fatal inside a worker.
func TestToolSurfaceConcurrentSearcher(t *testing.T) {
	t.Setenv("TOOLS_ALPHA_SECRET", "alpha-secret")
	t.Setenv("TOOLS_CLIENT_TOKEN", "client-token")
	h := newToolHarness(t, parseReloadConfig(t, toolSurfaceConfig), service.ToolSurfaceConfig{
		Model: search.ModelIdentity{Provider: "openai", Model: "embed-1", Dimensions: 4, FormatVersion: "v1"},
	})

	const workers = 16
	const callsPerWorker = 4
	results := make([]*mcp.CallToolResult, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			for range callsPerWorker {
				result, err := h.callErr(catalog.ToolSearchTools, map[string]any{"query": "read"})
				if err != nil {
					errs[slot] = fmt.Errorf("worker %d call: %w", slot, err)
					return
				}
				if result.IsError {
					errs[slot] = fmt.Errorf("worker %d search_tools answered with an error: %v", slot, result.StructuredContent)
					return
				}
				results[slot] = result
			}
		}(i)
	}
	wg.Wait()
	for slot, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
		if results[slot] == nil {
			t.Fatalf("worker %d produced no result", slot)
		}
	}
}
