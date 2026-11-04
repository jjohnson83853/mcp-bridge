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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
)

type restartPeerState struct {
	initializes atomic.Int64
	executions  atomic.Int64
}

func newPeerHandler(generation string) (*restartPeerState, http.Handler) {
	state := &restartPeerState{}
	server := mcp.NewServer(&mcp.Implementation{Name: "restart-peer", Version: "1"}, nil)
	server.AddTool(&mcp.Tool{Name: "read", InputSchema: json.RawMessage(`{"type":"object"}`)},
		func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			state.executions.Add(1)
			return &mcp.CallToolResult{StructuredContent: map[string]any{"generation": generation}}, nil
		})
	inner := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	return state, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			http.Error(writer, "read request", http.StatusBadRequest)
			return
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
		if bytes.Contains(body, []byte(`"method":"initialize"`)) {
			state.initializes.Add(1)
		}
		inner.ServeHTTP(writer, request)
	})
}

func startUpstreamRestartBridge(t *testing.T, peerURL string, stateful bool) (context.Context, *mcp.ClientSession) {
	t.Helper()
	t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
	t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)

	httpAddress := freeAddress(t)
	operationsAddress := freeAddress(t)
	embeddings := fakeEmbeddings(t)
	directory := t.TempDir()
	configPath := filepath.Join(directory, "bridge.yaml")
	writeBridgeConfig(t, configPath, httpAddress, operationsAddress, filepath.Join(directory, "state"), embeddings.URL,
		[]bridgeServerSpec{{id: "alpha", endpoint: peerURL}})
	if stateful {
		config, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("read bridge config: %v", err)
		}
		const pins = "pins: []\n"
		if strings.Count(string(config), pins) != 1 {
			t.Fatalf("bridge config pins marker count = %d, want 1", strings.Count(string(config), pins))
		}
		config = []byte(strings.Replace(string(config), pins, "    stateless: false\n"+pins, 1))
		if err := os.WriteFile(configPath, config, 0o600); err != nil {
			t.Fatalf("write stateful bridge config: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	stdout, stderr := &syncWriter{}, &syncWriter{}
	done := make(chan int, 1)
	go func() { done <- runContext(ctx, []string{"--config", configPath}, stdout, stderr) }()
	var session *mcp.ClientSession
	t.Cleanup(func() {
		if session != nil {
			if err := session.Close(); err != nil {
				t.Errorf("close bridge client: %v", err)
			}
		}
		cancel()
		select {
		case code := <-done:
			if code != 0 {
				t.Errorf("bridge exited with status %d; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		case <-time.After(10 * time.Second):
			t.Errorf("bridge did not stop after cancellation")
		}
	})

	operationsURL := "http://" + operationsAddress
	waitFor(t, "ready bridge", func() bool {
		status, body, ok := probeBody(t, operationsURL+"/readyz")
		return ok && status == http.StatusOK && strings.Contains(body, `"status":"ready"`) && strings.Contains(body, "alpha")
	})
	session = connectBridge(t, ctx, "http://"+httpAddress+"/mcp", serveClientToken)
	return ctx, session
}

func findRestartTool(t *testing.T, ctx context.Context, session *mcp.ClientSession) (string, string) {
	t.Helper()
	var revision, toolName string
	waitFor(t, "published upstream read tool", func() bool {
		payload := searchPayload(t, ctx, session, "read")
		results, ok := payload["results"].([]any)
		if !ok || len(results) == 0 {
			return false
		}
		revision, toolName = catalogRevisionAndTool(t, payload, "alpha")
		return revision != "" && toolName == core.PublicName("alpha", "read")
	})
	return revision, toolName
}

func callRestartRead(t *testing.T, ctx context.Context, session *mcp.ClientSession, revision, toolName string) *mcp.CallToolResult {
	t.Helper()
	return callTool(t, ctx, session, catalog.ToolCallTool, map[string]any{
		"tool_name":        toolName,
		"catalog_revision": revision,
		"arguments":        map[string]any{},
	})
}

func hasGeneration(value any, want string) bool {
	switch value := value.(type) {
	case map[string]any:
		if generation, ok := value["generation"].(string); ok && generation == want {
			return true
		}
		for _, nested := range value {
			if hasGeneration(nested, want) {
				return true
			}
		}
	case []any:
		for _, nested := range value {
			if hasGeneration(nested, want) {
				return true
			}
		}
	}
	return false
}

func TestUpstreamRestartSurvivesInDefaultStatelessMode(t *testing.T) {
	beforeState, beforeHandler := newPeerHandler("before")
	activeHandler := beforeHandler
	var handlerMu sync.RWMutex
	peer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		handlerMu.RLock()
		handler := activeHandler
		handlerMu.RUnlock()
		handler.ServeHTTP(writer, request)
	}))
	t.Cleanup(peer.Close)

	ctx, session := startUpstreamRestartBridge(t, peer.URL, false)
	revision, toolName := findRestartTool(t, ctx, session)
	first := callRestartRead(t, ctx, session, revision, toolName)
	if first.IsError {
		t.Fatalf("initial tools/call returned isError=true: %v", structuredContent(t, first))
	}
	if !hasGeneration(structuredContent(t, first), "before") {
		t.Fatalf("initial structured result did not contain generation %q: %v", "before", structuredContent(t, first))
	}
	if got := beforeState.executions.Load(); got != 1 {
		t.Fatalf("pre-restart executions=%d, want 1", got)
	}

	afterState, afterHandler := newPeerHandler("after")
	handlerMu.Lock()
	activeHandler = afterHandler
	handlerMu.Unlock()
	second := callRestartRead(t, ctx, session, revision, toolName)
	if second.IsError {
		t.Fatalf("post-restart tools/call returned isError=true: %v", structuredContent(t, second))
	}
	if !hasGeneration(structuredContent(t, second), "after") {
		t.Fatalf("post-restart structured result did not contain generation %q: %v", "after", structuredContent(t, second))
	}
	if got := afterState.executions.Load(); got != 1 {
		t.Fatalf("post-restart executions=%d, want 1", got)
	}
}
func TestUpstreamPreservesSessionInExplicitStatefulMode(t *testing.T) {
	state, handler := newPeerHandler("stateful")
	peer := httptest.NewServer(handler)
	t.Cleanup(peer.Close)

	ctx, session := startUpstreamRestartBridge(t, peer.URL, true)
	revision, toolName := findRestartTool(t, ctx, session)
	for i := range 2 {
		result := callRestartRead(t, ctx, session, revision, toolName)
		if result.IsError {
			t.Fatalf("stateful tools/call %d returned isError=true: %v", i+1, structuredContent(t, result))
		}
		if !hasGeneration(structuredContent(t, result), "stateful") {
			t.Fatalf("stateful tools/call %d omitted the upstream generation: %v", i+1, structuredContent(t, result))
		}
	}
	if got := state.initializes.Load(); got != 1 {
		t.Fatalf("stateful initialize requests=%d, want 1", got)
	}
	if got := state.executions.Load(); got != 2 {
		t.Fatalf("stateful tool executions=%d, want 2", got)
	}
}

func captureProcessStdout(t *testing.T) func() []byte {
	t.Helper()
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writer
	restored := false
	t.Cleanup(func() {
		os.Stdout = original
		if !restored {
			_ = writer.Close()
		}
		_ = reader.Close()
	})
	return func() []byte {
		if !restored {
			os.Stdout = original
			if err := writer.Close(); err != nil {
				t.Errorf("close captured stdout writer: %v", err)
			}
			restored = true
		}
		output, err := io.ReadAll(reader)
		if err != nil {
			t.Errorf("read captured stdout: %v", err)
		}
		if err := reader.Close(); err != nil {
			t.Errorf("close captured stdout reader: %v", err)
		}
		return output
	}
}

func TestUpstreamEphemeralHandshakeLog(t *testing.T) {
	readStdout := captureProcessStdout(t)
	_, handler := newPeerHandler("legacy")
	peer := httptest.NewServer(handler)
	t.Cleanup(peer.Close)

	ctx, session := startUpstreamRestartBridge(t, peer.URL, false)
	revision, toolName := findRestartTool(t, ctx, session)
	result := callRestartRead(t, ctx, session, revision, toolName)
	if result.IsError {
		t.Fatalf("legacy upstream tools/call returned isError=true: %v", structuredContent(t, result))
	}
	output := readStdout()
	found := false
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		var event struct {
			Level           string `json:"level"`
			Event           string `json:"event"`
			BackendID       string `json:"backend_id"`
			ProtocolVersion string `json:"protocol_version"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode upstream stdout JSON %q: %v", line, err)
		}
		if event.Event != "upstream_ephemeral_handshake" {
			continue
		}
		if event.Level != "info" {
			t.Errorf("handshake event level=%q, want info", event.Level)
		}
		if event.BackendID != "alpha" {
			t.Errorf("handshake event backend_id=%q, want alpha", event.BackendID)
		}
		if event.ProtocolVersion == "" || event.ProtocolVersion >= "2026-07-28" {
			t.Errorf("handshake event protocol_version=%q, want a legacy version before 2026-07-28", event.ProtocolVersion)
		}
		found = true
	}
	if !found {
		t.Fatalf("stdout did not contain a valid upstream_ephemeral_handshake JSON event: %s", output)
	}
}
