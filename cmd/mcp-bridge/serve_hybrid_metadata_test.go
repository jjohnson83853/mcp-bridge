package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// countingUpstreamServer builds the alpha upstream MCP handler whose tool
// invocations increment counter, so metadata reads can prove zero upstream
// operations.
func countingUpstreamServer(t *testing.T, counter *atomic.Int64) http.Handler {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "alpha", Version: "1"}, nil)
	server.AddTool(&mcp.Tool{
		Name:        "read",
		Description: "Read a file from disk",
		InputSchema: json.RawMessage(readToolSchema),
	}, func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		counter.Add(1)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
		}, nil
	})
	server.AddTool(&mcp.Tool{
		Name:        "write",
		Description: "Write data to a file on disk",
		InputSchema: json.RawMessage(writeToolSchema),
	}, func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		counter.Add(1)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "written"}},
		}, nil
	})
	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true},
	)
}

// countingUpstreamTools serves the alpha upstream read and write tools behind
// an operation counter so metadata reads can prove zero upstream operations.
func countingUpstreamTools(t *testing.T, counter *atomic.Int64) *httptest.Server {
	t.Helper()
	httpServer := httptest.NewServer(countingUpstreamServer(t, counter))
	t.Cleanup(httpServer.Close)
	return httpServer
}

// bridgeAnnotations decodes the standard tool-annotation hints from one raw
// tools/list annotation object.
type bridgeAnnotations struct {
	ReadOnly    *bool `json:"readOnlyHint"`
	Destructive *bool `json:"destructiveHint"`
	Idempotent  *bool `json:"idempotentHint"`
	OpenWorld   *bool `json:"openWorldHint"`
}

// metadataListedTools captures the wire shape of one tools/list entry that the
// metadata classification test cares about.
type metadataListedTools struct {
	Annotations json.RawMessage `json:"annotations"`
	Description string          `json:"description"`
	Name        string          `json:"name"`
}

// TestServeHybridMetadataBuiltinAnnotations verifies built-in tool classifications
// on the real /mcp seam: search_tools and describe_tool are read-only,
// idempotent, closed-world discovery with explicit destructiveHint false;
// call_tool stays potentially mutating, potentially destructive, non-idempotent
// and open-world; discovery calls perform zero upstream operations.
func TestServeHybridMetadataBuiltinAnnotations(t *testing.T) {
	counter := &atomic.Int64{}
	upstream := countingUpstreamTools(t, counter)
	ctx, httpURL, _, _ := startHybridBridgeWithUpstream(t, upstream)
	client := &http.Client{Timeout: 5 * time.Second}

	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"hybrid-metadata-client","version":"1"}}}`
	response, body := mcpSend(t, ctx, client, httpURL, "", initialize)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d; want 200 (body=%s)", response.StatusCode, body)
	}
	session := response.Header.Get("Mcp-Session-Id")
	if session == "" {
		t.Fatal("initialize omitted Mcp-Session-Id")
	}

	response, body = mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("initialized notification status = %d; want 202 (body=%s)", response.StatusCode, body)
	}

	response, body = mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("tools/list status = %d; want 200 (body=%s)", response.StatusCode, body)
	}
	payload := body
	if index := bytes.Index(payload, []byte("data: ")); index >= 0 {
		payload = payload[index+len("data: "):]
		if end := bytes.IndexByte(payload, '\n'); end >= 0 {
			payload = payload[:end]
		}
	}
	var envelope struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  *struct {
			Tools []metadataListedTools `json:"tools"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("tools/list response not JSON: %v (body=%s)", err, body)
	}
	if envelope.Error != nil {
		t.Fatalf("tools/list returned JSON-RPC error %+v (body=%s)", envelope.Error, body)
	}
	if envelope.JSONRPC != "2.0" || envelope.ID != 2 || envelope.Result == nil {
		t.Fatalf("tools/list envelope = %+v (body=%s); want JSON-RPC 2.0 success for id 2", envelope, body)
	}

	annotations := make(map[string]bridgeAnnotations, len(envelope.Result.Tools))
	descriptions := make(map[string]string, len(envelope.Result.Tools))
	for _, tool := range envelope.Result.Tools {
		annotations[tool.Name] = decodeBridgeAnnotations(t, tool.Name, tool.Annotations)
		descriptions[tool.Name] = tool.Description
	}
	for _, name := range []string{"search_tools", "describe_tool", "call_tool"} {
		if _, present := annotations[name]; !present {
			t.Fatalf("tools/list missing bridge tool %q (tools=%v; body=%s)", name, toolNames(annotations, envelope.Result.Tools), body)
		}
	}
	wantDescriptions := map[string]string{
		"search_tools":  "Search the bridge's indexed tools. The following tools are already directly declared in your tools list and MUST be invoked directly instead of searching: alpha__read. Unpinned tools remain discoverable through this tool.",
		"describe_tool": "Describe one indexed tool with its exact stored schema. DO NOT use this tool to discover input schemas for tools that are already declared in your tools list; invoke those tools directly. Use this tool only to inspect tool metadata, provenance, or unpinned tools.",
		"call_tool":     "Call one indexed upstream tool through the bridge. Directly declared tools in your active tools list MUST be invoked directly instead of calling this multiplexer. Use this tool only to invoke unpinned upstream tools discovered via search_tools.",
	}
	for name, want := range wantDescriptions {
		if got := descriptions[name]; got != want {
			t.Errorf("%s description = %q; want %q", name, got, want)
		}
	}

	discovery := map[string]bridgeAnnotations{
		"search_tools":  annotations["search_tools"],
		"describe_tool": annotations["describe_tool"],
	}
	for name, got := range discovery {
		if got.ReadOnly == nil || !*got.ReadOnly {
			t.Errorf("%s readOnlyHint = %v; want true", name, derefBool(got.ReadOnly))
		}
		if got.Idempotent == nil || !*got.Idempotent {
			t.Errorf("%s idempotentHint = %v; want true", name, derefBool(got.Idempotent))
		}
		if got.OpenWorld == nil || *got.OpenWorld {
			t.Errorf("%s openWorldHint = %v; want false", name, derefBool(got.OpenWorld))
		}
		if got.Destructive == nil || *got.Destructive {
			t.Errorf("%s destructiveHint = %v; want false", name, derefBool(got.Destructive))
		}
	}

	generic := annotations["call_tool"]
	wantFalse := false
	wantTrue := true
	if got := derefBool(generic.ReadOnly); got != nil && *got != wantFalse {
		t.Errorf("call_tool readOnlyHint = %v; want false", *generic.ReadOnly)
	}
	if got := derefBool(generic.Destructive); got != nil && *got != wantTrue {
		t.Errorf("call_tool destructiveHint = %v; want true", *generic.Destructive)
	}
	if got := derefBool(generic.Idempotent); got != nil && *got != wantFalse {
		t.Errorf("call_tool idempotentHint = %v; want false", *generic.Idempotent)
	}
	if got := derefBool(generic.OpenWorld); got != nil && *got != wantTrue {
		t.Errorf("call_tool openWorldHint = %v; want true", *generic.OpenWorld)
	}

	// Discovery must cause zero upstream operations; generic dispatch reaches
	// the fake upstream exactly once.
	extractSSE := func(t *testing.T, raw []byte) []byte {
		t.Helper()
		payload := raw
		if index := bytes.Index(payload, []byte("data: ")); index >= 0 {
			payload = payload[index+len("data: "):]
			if end := bytes.IndexByte(payload, '\n'); end >= 0 {
				payload = payload[:end]
			}
		}
		return payload
	}
	decodeText := func(t *testing.T, raw []byte, step string) (bool, string) {
		t.Helper()
		var inner struct {
			Result *struct {
				IsError bool `json:"isError"`
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(extractSSE(t, raw), &inner); err != nil {
			t.Fatalf("%s response not JSON: %v (body=%s)", step, err, raw)
		}
		if inner.Error != nil {
			t.Fatalf("%s returned JSON-RPC error %+v (body=%s)", step, inner.Error, raw)
		}
		if inner.Result == nil {
			t.Fatalf("%s envelope missing result (body=%s)", step, raw)
		}
		text := ""
		for _, item := range inner.Result.Content {
			text += item.Text
		}
		return inner.Result.IsError, text
	}

	var revision string
	response, body = mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search_tools","arguments":{"query":"metadata"}}}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("search_tools status = %d; want 200 (body=%s)", response.StatusCode, body)
	}
	if isError, text := decodeText(t, body, "search_tools"); isError {
		t.Fatalf("search_tools result isError = true (text=%q, body=%s)", text, body)
	} else {
		var searchResponse struct {
			CatalogRevision string `json:"catalog_revision"`
		}
		if err := json.Unmarshal([]byte(text), &searchResponse); err != nil {
			t.Fatalf("search_tools content not JSON: %v (text=%q)", err, text)
		}
		revision = searchResponse.CatalogRevision
	}
	if counter.Load() != 0 {
		t.Fatalf("search_tools performed %d upstream operations; want 0", counter.Load())
	}

	if revision == "" {
		t.Fatal("search_tools omitted catalog_revision; describe_tool cannot be exercised")
	}
	describeArguments, err := json.Marshal(map[string]any{
		"tool_name":        "alpha__read",
		"catalog_revision": revision,
	})
	if err != nil {
		t.Fatalf("marshal describe_tool arguments: %v", err)
	}
	response, body = mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"describe_tool","arguments":`+string(describeArguments)+`}}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("describe_tool status = %d; want 200 (body=%s)", response.StatusCode, body)
	}
	if isError, text := decodeText(t, body, "describe_tool"); isError {
		t.Fatalf("describe_tool result isError = true (text=%q, body=%s)", text, body)
	}
	if counter.Load() != 0 {
		t.Fatalf("describe_tool performed %d upstream operations; want 0", counter.Load())
	}

	callArguments, err := json.Marshal(map[string]any{
		"tool_name":        "alpha__read",
		"catalog_revision": revision,
		"arguments":        map[string]any{"path": "metadata.txt"},
	})
	if err != nil {
		t.Fatalf("marshal call_tool arguments: %v", err)
	}
	response, body = mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"call_tool","arguments":`+string(callArguments)+`}}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("call_tool status = %d; want 200 (body=%s)", response.StatusCode, body)
	}
	if isError, text := decodeText(t, body, "call_tool"); isError {
		t.Fatalf("call_tool result isError = true (text=%q, body=%s)", text, body)
	} else if !bytes.Contains([]byte(text), []byte("ok")) {
		t.Errorf("call_tool content %q does not contain %q", text, "ok")
	}
	if counter.Load() != 1 {
		t.Fatalf("call_tool left the upstream operation counter at %d; want exactly 1", counter.Load())
	}
}

// decodeBridgeAnnotations parses one raw annotations object; a missing
// annotations object surfaces as all-nil hints so the classification
// assertions fail with evidence instead of a decode error.
func decodeBridgeAnnotations(t *testing.T, name string, raw json.RawMessage) bridgeAnnotations {
	t.Helper()
	var decoded bridgeAnnotations
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("%s annotations not JSON: %v (raw=%s)", name, err, raw)
		}
	}
	return decoded
}

func derefBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func toolNames(annotations map[string]bridgeAnnotations, tools []metadataListedTools) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		_ = annotations
		names = append(names, tool.Name)
	}
	return names
}

func searchToolsDescriptionOnWire(t *testing.T, ctx context.Context, client *http.Client, httpURL, session string) string {
	t.Helper()
	response, body := mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("tools/list status = %d; want 200 (body=%s)", response.StatusCode, body)
	}
	var envelope struct {
		Result *struct {
			Tools []metadataListedTools `json:"tools"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(ssePayload(body), &envelope); err != nil {
		t.Fatalf("decode tools/list response: %v (%s)", err, body)
	}
	if envelope.Error != nil {
		t.Fatalf("tools/list returned JSON-RPC error %+v (body=%s)", envelope.Error, body)
	}
	if envelope.Result == nil {
		t.Fatalf("tools/list response omitted result (body=%s)", body)
	}
	for _, tool := range envelope.Result.Tools {
		if tool.Name == "search_tools" {
			return tool.Description
		}
	}
	t.Fatalf("tools/list omitted search_tools (body=%s)", body)
	return ""
}

// TestServeHybridMetadataReloadUpdatesGuidance checks the same live HTTP
// session before and after SIGHUP reloads, including a multi-pin replacement
// and a transition to the zero-pin baseline.
func TestServeHybridMetadataReloadUpdatesGuidance(t *testing.T) {
	t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
	t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)

	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)

	alpha := fakeUpstreamReadWrite(t)
	beta := fakeUpstreamReadWrite(t)
	embeddings := fakeEmbeddings(t)
	httpAddress, operationsAddress := freeAddress(t), freeAddress(t)
	for attempts := 0; operationsAddress == httpAddress && attempts < 8; attempts++ {
		operationsAddress = freeAddress(t)
	}
	if operationsAddress == httpAddress {
		t.Fatal("could not choose distinct public and operations addresses")
	}
	directory := t.TempDir()
	configPath := filepath.Join(directory, "bridge.yaml")
	servers := []bridgeServerSpec{
		{id: "alpha", endpoint: alpha.URL},
		{id: "beta", endpoint: beta.URL},
	}
	rewriteHybridReloadPins(t, configPath, httpAddress, operationsAddress, filepath.Join(directory, "state"), embeddings.URL, servers,
		[]string{"alpha/read", "alpha/write"})

	ctx, cancel := context.WithCancel(context.Background())
	stdout, stderr := &syncWriter{}, &syncWriter{}
	done := make(chan int, 1)
	go func() { done <- runContext(ctx, []string{"--config", configPath}, stdout, stderr) }()
	t.Cleanup(func() {
		cancel()
		select {
		case status := <-done:
			if status != 0 {
				t.Errorf("run status = %d; want 0 (stderr=%s)", status, stderr.String())
			}
		case <-time.After(30 * time.Second):
			t.Error("the process did not shut down after cancellation")
		}
	})
	waitFor(t, "readiness of both configured servers", func() bool {
		status, body, ok := probeBody(t, "http://"+operationsAddress+"/readyz")
		return ok && status == http.StatusOK &&
			strings.Contains(body, `"status":"ready"`) &&
			strings.Contains(body, "alpha") &&
			strings.Contains(body, "beta")
	})

	client := &http.Client{Timeout: 5 * time.Second}
	httpURL := "http://" + httpAddress
	response, body := mcpSend(t, ctx, client, httpURL, "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"metadata-reload-guidance-client","version":"1"}}}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d; want 200 (body=%s)", response.StatusCode, body)
	}
	session := response.Header.Get("Mcp-Session-Id")
	if session == "" {
		t.Fatal("initialize omitted Mcp-Session-Id")
	}
	response, body = mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("initialized notification status = %d; want 202 (body=%s)", response.StatusCode, body)
	}

	assertDescription := func(phase, want string) {
		t.Helper()
		waitFor(t, phase, func() bool {
			return searchToolsDescriptionOnWire(t, ctx, client, httpURL, session) == want
		})
	}
	reloadAndAssert := func(phase, want string) {
		t.Helper()
		sendReload(t)
		select {
		case <-hup:
		case <-time.After(10 * time.Second):
			t.Fatal("SIGHUP was not delivered to the test process")
		}
		assertDescription(phase, want)
	}

	assertDescription("initial two-pin guidance", "Search the bridge's indexed tools. The following tools are already directly declared in your tools list and MUST be invoked directly instead of searching: alpha__read, alpha__write. Unpinned tools remain discoverable through this tool.")

	rewriteHybridReloadPins(t, configPath, httpAddress, operationsAddress, filepath.Join(directory, "state"), embeddings.URL, servers,
		[]string{"beta/read", "beta/write"})
	reloadAndAssert("guidance for the reloaded beta pins", "Search the bridge's indexed tools. The following tools are already directly declared in your tools list and MUST be invoked directly instead of searching: beta__read, beta__write. Unpinned tools remain discoverable through this tool.")

	rewriteHybridReloadPins(t, configPath, httpAddress, operationsAddress, filepath.Join(directory, "state"), embeddings.URL, servers, nil)
	reloadAndAssert("zero-pin guidance after reload", "Search the bridge's indexed tools. Tools declared in your tools list MUST be invoked directly instead of searching. Unpinned tools remain discoverable through this tool.")
}
