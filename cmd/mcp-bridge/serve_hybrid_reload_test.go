package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
)

// fakeUpstreamReadWrite serves the alpha upstream exposing both the read and
// write tools so the reload subtests can pin, add, and remove real tools.
func fakeUpstreamReadWrite(t *testing.T) *httptest.Server {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "alpha", Version: "1"}, nil)
	for _, name := range []string{"read", "write"} {
		toolName := name
		server.AddTool(&mcp.Tool{
			Name:        toolName,
			Description: toolName + " tool",
			InputSchema: json.RawMessage(`{"type":"object","properties":{},"additionalProperties":true}`),
		}, func(_ context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: "upstream " + toolName + " saw " + string(request.Params.Arguments),
				}},
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

// rewriteHybridReloadPins rewrites the pins block of the existing bridge config
// in place. It resets the document with writeBridgeConfig first, so the
// replacement below always anchors on exactly one `pins: []` line.
func rewriteHybridReloadPins(t *testing.T, path, httpAddress, operationsAddress, storageDirectory, embeddingURL string, servers []bridgeServerSpec, pins []string) {
	t.Helper()
	writeBridgeConfig(t, path, httpAddress, operationsAddress, storageDirectory, embeddingURL, servers)
	if len(pins) == 0 {
		return
	}
	config, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture config for pins: %v", err)
	}
	var pinBlock strings.Builder
	pinBlock.WriteString("pins:\n")
	for _, pin := range pins {
		parts := strings.SplitN(pin, "/", 2)
		if len(parts) != 2 {
			t.Fatalf("invalid pin spec %q; want server/tool", pin)
		}
		pinBlock.WriteString(fmt.Sprintf("  - {server: %s, tool: %s}\n", parts[0], parts[1]))
	}
	config = []byte(strings.Replace(string(config), "pins: []", pinBlock.String(), 1))
	if err := os.WriteFile(path, config, 0o600); err != nil {
		t.Fatalf("write fixture config pins: %v", err)
	}
}

// toolChangeWatcher counts and signals tools/list_changed notifications the
// inbound client session receives from the bridge.
type toolChangeWatcher struct {
	mu       sync.Mutex
	received int
	notify   chan struct{}
}

func (w *toolChangeWatcher) record(context.Context, *mcp.ToolListChangedRequest) {
	w.mu.Lock()
	w.received++
	w.mu.Unlock()
	select {
	case w.notify <- struct{}{}:
	default:
	}
}

func (w *toolChangeWatcher) waitSignal(t *testing.T) {
	t.Helper()
	select {
	case <-w.notify:
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for notifications/tools/list_changed; received %d", w.count())
	}
}

func (w *toolChangeWatcher) count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.received
}

// connectBridgeToolChanged mirrors connectBridgePrompts but registers a
// ToolListChangedHandler. The protocol version is pinned to 2025-11-25 so the
// bridge serves the session as a legacy client and the tool surface is
// observable without SEP-2575 subscription routing.
func connectBridgeToolChanged(t *testing.T, ctx context.Context, endpoint string, watcher *toolChangeWatcher) *mcp.ClientSession {
	t.Helper()
	options := &mcp.ClientOptions{}
	if watcher != nil {
		options.ToolListChangedHandler = watcher.record
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "hybrid-reload-client", Version: "1"}, options)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   endpoint,
		HTTPClient: &http.Client{Transport: bearerTransport{token: serveClientToken, base: http.DefaultTransport}},
		MaxRetries: -1,
	}, &mcp.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
	if err != nil {
		t.Fatalf("connect bridge: %v", err)
	}
	return session
}

// listToolNames lists the tool names the session currently exposes.
func listToolNames(t *testing.T, ctx context.Context, session *mcp.ClientSession) []string {
	t.Helper()
	page, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	names := make([]string, 0, len(page.Tools))
	for _, tool := range page.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	return names
}

// listToolsContains reports whether name appears in the listed tool names.
func listToolsContains(names []string, name string) bool {
	return slices.Contains(names, name)
}

// directCallTool calls one bridge tool by its public name and returns the
// result with any transport error surfaced for the caller to assert.
func directCallTool(t *testing.T, ctx context.Context, session *mcp.ClientSession, name string) (*mcp.CallToolResult, error) {
	t.Helper()
	return session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: json.RawMessage(`{"path":"reload.txt"}`)})
}

// directCallToolText returns the concatenated text content of one tool result.
func directCallToolText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	var builder strings.Builder
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			builder.WriteString(text.Text)
		}
	}
	return builder.String()
}

// TestServeHybridSIGHUPReload is the process-level acceptance of Ticket 05:
// a SIGHUP-driven config reload updates the pinned tool surface, delivers
// notifications/tools/list_changed to a connected MCP client, and rejects an
// invalid candidate atomically while preserving the running generation.
func TestServeHybridSIGHUPReload(t *testing.T) {
	t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
	t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)

	// The test process must survive the SIGHUP it sends; the bridge registers
	// its own reload handler for the same signal.
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	defer signal.Stop(sighup)

	alpha := fakeUpstreamReadWrite(t)
	embeddings := fakeEmbeddings(t)
	httpAddress := freeAddress(t)
	operationsAddress := freeAddress(t)
	directory := t.TempDir()
	configPath := filepath.Join(directory, "bridge.yaml")
	storageDirectory := filepath.Join(directory, "state")
	servers := []bridgeServerSpec{{id: "alpha", endpoint: alpha.URL}}

	rewriteHybridReloadPins(t, configPath, httpAddress, operationsAddress, storageDirectory, embeddings.URL, servers,
		[]string{"alpha/read"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stdout, stderr := &syncWriter{}, &syncWriter{}
	done := make(chan int, 1)
	go func() { done <- runContext(ctx, []string{"--config", configPath}, stdout, stderr) }()
	operationsURL := "http://" + operationsAddress
	httpURL := "http://" + httpAddress
	waitFor(t, "readiness of the configured server", func() bool {
		status, body, ok := probeBody(t, operationsURL+"/readyz")
		return ok && status == http.StatusOK && strings.Contains(body, `"status":"ready"`) && strings.Contains(body, "alpha")
	})

	watcher := &toolChangeWatcher{notify: make(chan struct{}, 8)}
	session := connectBridgeToolChanged(t, ctx, httpURL+"/mcp", watcher)
	defer session.Close()

	// Initial check: the 3 meta-tools plus the preloaded alpha__read pin.
	initialNames := listToolNames(t, ctx, session)
	for _, meta := range []string{catalog.ToolSearchTools, catalog.ToolDescribeTool, catalog.ToolCallTool} {
		if !listToolsContains(initialNames, meta) {
			t.Errorf("initial tools/list missing meta-tool %q; tools = %v", meta, initialNames)
		}
	}
	if !listToolsContains(initialNames, "alpha__read") {
		t.Errorf("initial tools/list missing pinned tool alpha__read; tools = %v", initialNames)
	}
	if len(initialNames) != 4 {
		t.Errorf("initial tools/list = %d tools (%v); want 4 (3 meta-tools + alpha__read)", len(initialNames), initialNames)
	}

	t.Run("PinAdditionViaSIGHUP", func(t *testing.T) {
		rewriteHybridReloadPins(t, configPath, httpAddress, operationsAddress, storageDirectory, embeddings.URL, servers,
			[]string{"alpha/read", "alpha/write"})
		sendReload(t)
		watcher.waitSignal(t)

		var names []string
		waitFor(t, "pin addition reload", func() bool {
			names = listToolNames(t, ctx, session)
			return listToolsContains(names, "alpha__read") && listToolsContains(names, "alpha__write") && len(names) == 5
		})

		result, err := directCallTool(t, ctx, session, "alpha__write")
		if err != nil {
			t.Fatalf("direct tools/call alpha__write: %v", err)
		}
		if result.IsError {
			t.Errorf("direct tools/call alpha__write returned isError = true; result = %+v", result)
		}
		if text := directCallToolText(t, result); !strings.Contains(text, "upstream write") {
			t.Errorf("direct tools/call alpha__write content %q does not contain %q", text, "upstream write")
		}
	})

	t.Run("PinRemovalViaSIGHUP", func(t *testing.T) {
		rewriteHybridReloadPins(t, configPath, httpAddress, operationsAddress, storageDirectory, embeddings.URL, servers,
			[]string{"alpha/write"})
		sendReload(t)
		watcher.waitSignal(t)

		var names []string
		waitFor(t, "pin removal reload", func() bool {
			names = listToolNames(t, ctx, session)
			return listToolsContains(names, "alpha__write") && !listToolsContains(names, "alpha__read") && len(names) == 4
		})

		result, err := directCallTool(t, ctx, session, "alpha__read")
		if err != nil {
			// A JSON-RPC error from the transport also counts as the unknown
			// tool rejection; the SDK may surface it as an error.
			return
		}
		if !result.IsError {
			t.Errorf("direct tools/call alpha__read after pin removal returned success; result = %+v", result)
		}
	})

	t.Run("InvalidNewPinRejectedViaSIGHUP", func(t *testing.T) {
		beforeCount := watcher.count()
		rewriteHybridReloadPins(t, configPath, httpAddress, operationsAddress, storageDirectory, embeddings.URL, servers,
			[]string{"alpha/write", "alpha/nonexistent"})
		for drained := false; !drained; {
			select {
			case <-watcher.notify:
			default:
				drained = true
			}
		}
		sendReload(t)

		// Give the reload handler time to process and reject the candidate.
		// A valid reload would emit notifications/tools/list_changed within
		// this window; a rejected candidate must not.
		select {
		case <-watcher.notify:
			t.Errorf("invalid pin reload emitted notifications/tools/list_changed; want atomic rejection with no tool surface change")
		case <-time.After(2 * time.Second):
		}
		if got := watcher.count(); got != beforeCount {
			t.Errorf("notification count after invalid reload = %d; want unchanged %d", got, beforeCount)
		}

		names := listToolNames(t, ctx, session)
		if !listToolsContains(names, "alpha__write") {
			t.Errorf("tools/list after invalid reload missing preserved alpha__write; tools = %v", names)
		}
		if listToolsContains(names, "alpha__read") {
			t.Errorf("tools/list after invalid reload unexpectedly lists alpha__read; tools = %v", names)
		}

		result, err := directCallTool(t, ctx, session, "alpha__write")
		if err != nil {
			t.Fatalf("direct tools/call alpha__write after invalid reload: %v", err)
		}
		if result.IsError {
			t.Errorf("direct tools/call alpha__write after invalid reload returned isError = true; result = %+v", result)
		}
		if text := directCallToolText(t, result); !strings.Contains(text, "upstream write") {
			t.Errorf("direct tools/call alpha__write after invalid reload content %q does not contain %q", text, "upstream write")
		}
	})
}
