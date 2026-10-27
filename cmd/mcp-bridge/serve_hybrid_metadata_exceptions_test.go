package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// metadataExceptionsUpstreamServer serves the alpha read and write tools with
// NO upstream metadata at all, so every corrected field a client observes
// comes from the configured defaults and the exact-tool exception, never from
// the backend.
func metadataExceptionsUpstreamServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "alpha", Version: "1"}, nil)
	for _, name := range []string{"read", "write"} {
		tool := &mcp.Tool{
			Name:        name,
			Description: "Metadata fixture " + name,
			InputSchema: json.RawMessage(readToolSchema),
		}
		server.AddTool(tool, func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
			}, nil
		})
	}
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true},
	))
	t.Cleanup(httpServer.Close)
	return httpServer
}

// metadataExceptionsDefaults is the broad alpha default: every pinned alpha
// tool claims read-only, non-destructive, idempotent.
const metadataExceptionsDefaults = `    tool_metadata:
      defaults:
        annotations:
          readOnlyHint: true
          destructiveHint: false
          idempotentHint: true
`

// metadataExceptionsTool is the exact raw-tool-name exception for write.
const metadataExceptionsTool = `      tools:
        write:
          annotations:
            readOnlyHint: false
            destructiveHint: true
            idempotentHint: false
`

// startMetadataExceptionsBridge starts the hybrid bridge with the alpha
// defaults plus the exact write exception; withTool toggles the tools mapping
// so the reload phase can remove it in place.
func startMetadataExceptionsBridge(t *testing.T, upstream *httptest.Server, configPath string, withTool bool) (context.Context, string) {
	t.Helper()
	t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
	t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)
	stdout := &syncWriter{}
	embeddings := fakeEmbeddings(t)
	httpAddress, operationsAddress := freeAddress(t), freeAddress(t)
	for attempts := 0; operationsAddress == httpAddress && attempts < 8; attempts++ {
		operationsAddress = freeAddress(t)
	}
	directory := filepath.Dir(configPath)
	writeBridgeConfig(t, configPath, httpAddress, operationsAddress, filepath.Join(directory, "state"), embeddings.URL,
		[]bridgeServerSpec{{id: "alpha", endpoint: upstream.URL}})
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read fixture config: %v", err)
	}
	config = []byte(strings.Replace(string(config), "allowed_origins: []", `allowed_origins: ["https://allowed.example"]`, 1))
	config = []byte(strings.Replace(string(config), "pins: []", "pins:\n  - {server: alpha, tool: read}\n  - {server: alpha, tool: write}\n", 1))
	metadata := metadataExceptionsDefaults
	if withTool {
		metadata += metadataExceptionsTool
	}
	config = []byte(strings.Replace(string(config), "    endpoint: \""+upstream.URL+"\"\n",
		"    endpoint: \""+upstream.URL+"\"\n"+metadata, 1))
	if err := os.WriteFile(configPath, config, 0600); err != nil {
		t.Fatalf("write fixture config: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	stderr := &syncWriter{}
	done := make(chan int, 1)
	go func() { done <- runContext(ctx, []string{"--config", configPath}, stdout, stderr) }()
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("bridge stdout: %s", stdout.String())
			t.Logf("bridge stderr: %s", stderr.String())
		}
	})
	stop := func() {
		cancel()
		select {
		case status := <-done:
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
	return ctx, "http://" + httpAddress
}

// initializeMetadataExceptionsClient performs the stateful SSE handshake and
// returns the live session id.
func initializeMetadataExceptionsClient(t *testing.T, ctx context.Context, client *http.Client, httpURL string) string {
	t.Helper()
	response, body := mcpSend(t, ctx, client, httpURL, "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"metadata-exceptions-client","version":"1"}}}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d; want 200 (body=%s)", response.StatusCode, body)
	}
	session := response.Header.Get("Mcp-Session-Id")
	response, body = mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("initialized notification status = %d; want 202 (body=%s)", response.StatusCode, body)
	}
	return session
}

// listMetadataExceptionsTools re-lists tools and returns the alpha__read and
// alpha__write entries plus the search_tools built-in.
func listMetadataExceptionsTools(t *testing.T, ctx context.Context, client *http.Client, httpURL, session string) (read, write, builtin *listedTool) {
	t.Helper()
	response, body := mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("tools/list status = %d; want 200 (body=%s)", response.StatusCode, body)
	}
	var envelope struct {
		Result *struct {
			Tools []listedTool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(ssePayload(body), &envelope); err != nil {
		t.Fatalf("decode tools/list response: %v (%s)", err, body)
	}
	for index := range envelope.Result.Tools {
		switch envelope.Result.Tools[index].Name {
		case "alpha__read":
			read = &envelope.Result.Tools[index]
		case "alpha__write":
			write = &envelope.Result.Tools[index]
		case "search_tools":
			builtin = &envelope.Result.Tools[index]
		}
	}
	if read == nil || write == nil || builtin == nil {
		t.Fatalf("tools/list omitted fixture tools: %s", body)
	}
	return read, write, builtin
}

// describeMetadataExceptionsTool describes one tool by name through the
// standard describe_tool seam and returns the decoded payload.
func describeMetadataExceptionsTool(t *testing.T, ctx context.Context, client *http.Client, httpURL, session, name string) map[string]json.RawMessage {
	t.Helper()
	response, body := mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search_tools","arguments":{"query":"`+name+`"}}}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("search_tools status = %d; want 200 (body=%s)", response.StatusCode, body)
	}
	var search struct {
		Result *struct {
			StructuredContent json.RawMessage `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(ssePayload(body), &search); err != nil {
		t.Fatalf("decode search_tools response: %v (%s)", err, body)
	}
	var searchPayload struct {
		Results []struct {
			ToolName        string `json:"tool_name"`
			CatalogRevision string `json:"catalog_revision"`
		} `json:"results"`
	}
	if err := json.Unmarshal(search.Result.StructuredContent, &searchPayload); err != nil {
		t.Fatalf("decode search payload: %v (%s)", err, search.Result.StructuredContent)
	}
	revision := ""
	for _, result := range searchPayload.Results {
		if result.ToolName == name {
			revision = result.CatalogRevision
			break
		}
	}
	if len(revision) != 64 {
		t.Fatalf("search_tools did not return %s with a revision: %s", name, search.Result.StructuredContent)
	}
	response, body = mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"describe_tool","arguments":{"tool_name":"`+name+`","catalog_revision":"`+revision+`"}}}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("describe_tool status = %d; want 200 (body=%s)", response.StatusCode, body)
	}
	var described struct {
		Result *struct {
			StructuredContent json.RawMessage `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(ssePayload(body), &described); err != nil {
		t.Fatalf("decode describe response: %v (%s)", err, body)
	}
	payload := map[string]json.RawMessage{}
	if err := json.Unmarshal(described.Result.StructuredContent, &payload); err != nil {
		t.Fatalf("decode describe payload: %v (%s)", err, described.Result.StructuredContent)
	}
	return payload
}

// TestServeHybridMetadataExactToolExceptions verifies on the real /mcp seam
// that the exact write exception wins over the broad alpha default while read
// keeps the default, the built-ins stay untouched, describe provenance names
// the winning tool layer, and removing only the tools mapping on reload
// reverts write to the current defaults in the same session.
func TestServeHybridMetadataExactToolExceptions(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "bridge.yaml")
	upstream := metadataExceptionsUpstreamServer(t)

	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)

	ctx, httpURL := startMetadataExceptionsBridge(t, upstream, configPath, true)
	client := &http.Client{Timeout: 5 * time.Second}
	session := initializeMetadataExceptionsClient(t, ctx, client, httpURL)

	read, write, builtin := listMetadataExceptionsTools(t, ctx, client, httpURL, session)
	var readHints, writeHints struct {
		ReadOnly    *bool `json:"readOnlyHint"`
		Destructive *bool `json:"destructiveHint"`
		Idempotent  *bool `json:"idempotentHint"`
	}
	if err := json.Unmarshal(read.Annotations, &readHints); err != nil {
		t.Fatalf("decode alpha__read annotations: %v (%s)", err, read.Annotations)
	}
	if readHints.ReadOnly == nil || !*readHints.ReadOnly {
		t.Fatalf("alpha__read lost the default readOnlyHint: %s (RED)", read.Annotations)
	}
	if err := json.Unmarshal(write.Annotations, &writeHints); err != nil {
		t.Fatalf("decode alpha__write annotations: %v (%s)", err, write.Annotations)
	}
	if writeHints.ReadOnly == nil || *writeHints.ReadOnly || writeHints.Destructive == nil || !*writeHints.Destructive || writeHints.Idempotent == nil || *writeHints.Idempotent {
		t.Fatalf("alpha__write did not carry the exact-tool exception: %s (RED)", write.Annotations)
	}
	if !strings.Contains(string(builtin.Annotations), `"readOnlyHint":true`) || strings.Contains(string(builtin.Annotations), "destructiveHint\":true") {
		t.Fatalf("built-in search_tools classification changed: %s (RED)", builtin.Annotations)
	}

	payload := describeMetadataExceptionsTool(t, ctx, client, httpURL, session, "alpha__write")
	var overrideFields []struct {
		Path  string `json:"path"`
		Layer string `json:"layer"`
	}
	if err := json.Unmarshal(payload["metadata_override_fields"], &overrideFields); err != nil {
		t.Fatalf("decode metadata_override_fields: %v (%s)", err, payload["metadata_override_fields"])
	}
	layers := map[string]string{}
	for _, field := range overrideFields {
		layers[field.Path] = field.Layer
	}
	for path, layer := range map[string]string{
		"annotations.readOnlyHint":    "tool",
		"annotations.destructiveHint": "tool",
		"annotations.idempotentHint":  "tool",
	} {
		if layers[path] != layer {
			t.Fatalf("describe provenance %q layer = %q; want %q (%s) (RED)", path, layers[path], layer, payload["metadata_override_fields"])
		}
	}

	// Remove only the tools mapping on reload: the same session reverts write
	// to the current defaults.
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read fixture config for reload: %v", err)
	}
	config = []byte(strings.Replace(string(config), metadataExceptionsTool, "", 1))
	if err := os.WriteFile(configPath, config, 0o600); err != nil {
		t.Fatalf("write fixture config for reload: %v", err)
	}
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGHUP); err != nil {
		t.Fatalf("raise SIGHUP: %v", err)
	}
	select {
	case <-hup:
	case <-time.After(10 * time.Second):
		t.Fatal("SIGHUP was not delivered to the test process")
	}
	waitFor(t, "write reverting to the defaults", func() bool {
		read, write, _ = listMetadataExceptionsTools(t, ctx, client, httpURL, session)
		if err := json.Unmarshal(write.Annotations, &writeHints); err != nil {
			t.Fatalf("decode alpha__write annotations after reload: %v (%s)", err, write.Annotations)
		}
		if writeHints.ReadOnly == nil || !*writeHints.ReadOnly || writeHints.Destructive == nil || *writeHints.Destructive {
			return false
		}
		if err := json.Unmarshal(read.Annotations, &readHints); err != nil {
			t.Fatalf("decode alpha__read annotations after reload: %v (%s)", err, read.Annotations)
		}
		return readHints.ReadOnly != nil && *readHints.ReadOnly
	})
}
