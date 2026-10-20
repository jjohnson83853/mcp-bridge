package main

import (
	"bytes"
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

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// readToolSchema is the exact upstream schema the preloaded read tool must
// expose verbatim on the public wire.
const readToolSchema = `{"properties":{"path":{"type":"string"}},"required":["path"],"type":"object"}`

// fakeUpstreamReadTool serves the alpha upstream providing tool read.
func fakeUpstreamReadTool(t *testing.T) *httptest.Server {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "alpha", Version: "1"}, nil)
	server.AddTool(&mcp.Tool{
		Name:        "read",
		Description: "Read a file from disk",
		InputSchema: json.RawMessage(readToolSchema),
	}, func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
		}, nil
	})
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true},
	))
	t.Cleanup(httpServer.Close)
	return httpServer
}

// startHybridBridgeWithUpstream mirrors startHTTPLoggingBridgeWithUpstream but
// rewrites the generated config's pins: [] line to pin alpha's read tool, so
// the hybrid catalog is preloaded with alpha__read.
func startHybridBridgeWithUpstream(t *testing.T, upstream *httptest.Server) (context.Context, string, *syncWriter, func()) {
	t.Helper()
	t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
	t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)
	stdout := &syncWriter{}
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
	writeBridgeConfig(t, configPath, httpAddress, operationsAddress, filepath.Join(directory, "state"), embeddings.URL,
		[]bridgeServerSpec{{id: "alpha", endpoint: upstream.URL}})
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read fixture config: %v", err)
	}
	config = []byte(strings.Replace(string(config), "allowed_origins: []", `allowed_origins: ["https://allowed.example"]`, 1))
	config = []byte(strings.Replace(string(config), "pins: []", "pins:\n  - {server: alpha, tool: read}\n", 1))
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

// mcpSend performs one raw POST /mcp JSON-RPC exchange against the bridge.
func mcpSend(t *testing.T, ctx context.Context, client *http.Client, httpURL, session, body string) (*http.Response, []byte) {
	t.Helper()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, httpURL+"/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build %s request: %v", body, err)
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
		t.Fatalf("POST /mcp: %v", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read %s response: %v", body, err)
	}
	return response, payload
}

func TestServeHybridPreloadedToolsListExposure(t *testing.T) {
	upstream := fakeUpstreamReadTool(t)
	ctx, httpURL, _, _ := startHybridBridgeWithUpstream(t, upstream)
	client := &http.Client{Timeout: 5 * time.Second}

	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"hybrid-exposure-client","version":"1"}}}`
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

	var envelope struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  *struct {
			Tools []struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				InputSchema json.RawMessage `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	// With the SDK default transport (no JSONResponse override), request
	// responses arrive as SSE frames; extract the data line before decoding.
	payload := body
	if index := bytes.Index(payload, []byte("data: ")); index >= 0 {
		payload = payload[index+len("data: "):]
		if end := bytes.IndexByte(payload, '\n'); end >= 0 {
			payload = payload[:end]
		}
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

	names := make(map[string]struct {
		Description string
		Schema      json.RawMessage
	}, len(envelope.Result.Tools))
	for _, tool := range envelope.Result.Tools {
		if _, duplicate := names[tool.Name]; duplicate {
			t.Errorf("tools/list duplicates tool %q", tool.Name)
		}
		names[tool.Name] = struct {
			Description string
			Schema      json.RawMessage
		}{tool.Description, tool.InputSchema}
	}
	if got := len(envelope.Result.Tools); got != 4 {
		t.Fatalf("tools/list length = %d (%v); want 4: search_tools, describe_tool, call_tool, alpha__read (body=%s)", got, names, body)
	}
	for _, meta := range []string{catalog.ToolSearchTools, catalog.ToolDescribeTool, catalog.ToolCallTool} {
		if _, present := names[meta]; !present {
			t.Errorf("tools/list missing meta-tool %q", meta)
		}
	}
	preloaded, present := names["alpha__read"]
	if !present {
		t.Fatalf("tools/list missing preloaded pinned tool alpha__read; tools = %v", names)
	}
	if preloaded.Description != "Read a file from disk" {
		t.Errorf("alpha__read description = %q; want %q", preloaded.Description, "Read a file from disk")
	}
	if string(preloaded.Schema) != readToolSchema {
		t.Errorf("alpha__read inputSchema = %s; want %s", preloaded.Schema, readToolSchema)
	}
}
