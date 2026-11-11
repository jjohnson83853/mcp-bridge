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

	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestServeLoggingTimeouts covers the timeout/cancellation family of the
// logging-observability spec: a backend call whose configured call deadline
// expires must surface deadline_exceeded on both terminal records, with the
// mcp boundary terminating at the backend_dispatch stage.
func TestServeLoggingTimeouts(t *testing.T) {
	t.Run("BackendTimeoutEmitsDeadlineExceeded", func(t *testing.T) {
		// The upstream takes longer than the configured call deadline, so the
		// bridge's per-call context expires with DeadlineExceeded before any
		// upstream bytes arrive.
		upstream := fakeUpstreamHanging(t, "read", `{"type":"object","properties":{}}`)
		ctx, httpURL, stdout, stop := startHTTPLoggingBridgeWithUpstreamCallTimeout(t, upstream, "100ms")

		call := loggedFailureToolCall(t, ctx, httpURL, stdout, stop, func(catalogRevision string) map[string]any {
			return map[string]any{
				"tool_name":        core.PublicName("alpha", "read"),
				"catalog_revision": catalogRevision,
				"arguments":        map[string]any{},
			}
		})

		// Public surface: the deadline failure answers as a tool-layer error
		// inside the unchanged JSON-RPC envelope (or a gateway timeout), never
		// as a silent success.
		if call.status == http.StatusOK {
			assertFailureEnvelope(t, call)
			result := failureResultObject(t, call.envelope)
			if code, ok := result["code"].(string); ok && code == "" {
				t.Errorf("timeout result missing error code: %v", result)
			}
		} else if call.status != http.StatusGatewayTimeout {
			t.Fatalf("public timeout status = %d; want 200 with tool error or 504 gateway timeout; body %v",
				call.status, call.envelope)
		}

		// Captured terminal records: the deadline expiry must be classified as
		// deadline_exceeded on both boundaries.
		assertDeadlineExceededLogs(t, call.captured)
	})
}

// assertDeadlineExceededLogs asserts the captured stdout for a call whose
// backend dispatch expired: exactly one backend_complete terminated with
// outcome timeout (or error) and reason deadline_exceeded, plus exactly one
// mcp_complete terminated at the backend_dispatch stage with the same reason.
func assertDeadlineExceededLogs(t *testing.T, captured string) {
	t.Helper()
	records := decodeCapturedRecords(t, captured)

	backends := filterRecords(records, "backend_complete")
	if len(backends) != 1 {
		t.Fatalf("backend_complete count = %d; want exactly 1 for the admitted dispatch\nrecords: %s",
			len(backends), captured)
	}
	backend := backends[0]
	outcome, _ := backend["outcome"].(string)
	if outcome != "timeout" && outcome != "error" {
		t.Errorf("backend_complete outcome = %v; want timeout or error", backend["outcome"])
	}
	if errorInfo, ok := backend["error"].(map[string]any); !ok || errorInfo["code"] != "deadline_exceeded" || len(errorInfo) != 4 {
		t.Errorf("backend_complete error = %#v; want nested code deadline_exceeded", backend["error"])
	}
	if backend["operation"] != "call_tool" {
		t.Errorf("backend_complete operation = %v; want call_tool", backend["operation"])
	}
	wantInternalHexID(t, backend, "backend_call_id")
	wantNonnegativeDuration(t, backend, "backend_complete")

	var dispatches []map[string]any
	for _, rec := range filterRecords(records, "mcp_complete") {
		if rec["stage"] == "backend_dispatch" {
			dispatches = append(dispatches, rec)
		}
	}
	if len(dispatches) != 1 {
		t.Fatalf("mcp_complete stage=backend_dispatch count = %d; want exactly 1\nrecords: %s",
			len(dispatches), captured)
	}
	mcp := dispatches[0]
	if got := mcp["mcp_method"]; got != "tools_call" {
		t.Errorf("mcp_complete mcp_method = %v; want tools_call", got)
	}
	if got := mcp["outcome"]; got != "error" {
		t.Errorf("mcp_complete outcome = %v; want error", got)
	}
	if errorInfo, ok := mcp["error"].(map[string]any); !ok || errorInfo["code"] != "deadline_exceeded" || len(errorInfo) != 4 {
		t.Errorf("mcp_complete error = %#v; want nested code deadline_exceeded", mcp["error"])
	}
	if mcp["operation"] != "call_tool" {
		t.Errorf("mcp_complete operation = %v; want call_tool", mcp["operation"])
	}
	wantInternalHexID(t, mcp, "operation_id")
	wantNonnegativeDuration(t, mcp, "mcp_complete")
}

// fakeUpstreamHanging serves one MCP tool whose handler outlives the bridge's
// call deadline: it sleeps well past the shortest configured timeout, so the
// bridge observes its own DeadlineExceeded and never sees upstream bytes.
func fakeUpstreamHanging(t *testing.T, toolName, schema string) *httptest.Server {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "fake-upstream", Version: "1"}, nil)
	server.AddTool(&mcp.Tool{Name: toolName, Description: toolName + " tool", InputSchema: json.RawMessage(schema)},
		func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			time.Sleep(500 * time.Millisecond)
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{
				Text: "upstream " + toolName + " completed after the bridge deadline",
			}}}, nil
		})
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true},
	))
	t.Cleanup(httpServer.Close)
	return httpServer
}

// startHTTPLoggingBridgeWithUpstreamCallTimeout mirrors
// startHTTPLoggingBridgeWithUpstream but rewrites the shared fixture config's
// call_timeout, because the shared template fixes a 30s upstream call deadline
// and timeout paths need one shorter than the hanging upstream's response.
func startHTTPLoggingBridgeWithUpstreamCallTimeout(t *testing.T, upstream *httptest.Server, callTimeout string) (context.Context, string, *syncWriter, func()) {
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
	config = []byte(strings.Replace(string(config), `call_timeout: "30s"`, "call_timeout: \""+callTimeout+"\"", 1))
	if err := os.WriteFile(configPath, config, 0600); err != nil {
		t.Fatalf("configure fixture origin policy: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	stderr := &syncWriter{}
	done := make(chan int, 1)
	go func() { done <- runContext(ctx, []string{"--config", configPath}, stdout, io.MultiWriter(stderr, stdout)) }()
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
