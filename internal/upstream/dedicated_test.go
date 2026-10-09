package upstream_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/upstream"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// T-04 dedicated per-server runtime RED gate (r8 plan §4.3, T-04 deliverable):
// one dedicated client/session owner per server that paginates tools/list,
// forwards exactly one tools/call with byte-exact args, reports input-required
// without re-dispatch, and releases its session on Close. The API under test
// does not exist yet, so this file fails to compile (intentional compile-RED).

type peerState struct {
	initializes atomic.Int32
	executions  atomic.Int32
	lastArgs    atomic.Value // string
}

// newPeer serves a real MCP peer over the streamable transport so discovery,
// pagination, cancellation and lifecycle are exercised against the SDK and
// wire, not a hand-rolled double.
func newPeer(t *testing.T, opts *mcp.ServerOptions, register func(*mcp.Server, *peerState)) (*httptest.Server, *peerState) {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "peer", Version: "1"}, opts)
	state := &peerState{}
	register(server, state)
	inner := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	counting := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read peer request: %v", err)
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
		if bytes.Contains(body, []byte(`"method":"initialize"`)) {
			state.initializes.Add(1)
		}
		inner.ServeHTTP(writer, request)
	})
	httpServer := httptest.NewServer(counting)
	t.Cleanup(httpServer.Close)
	return httpServer, state
}

func newDedicated(t *testing.T, endpoint string) *upstream.Dedicated {
	t.Helper()
	dedicated, err := upstream.NewDedicated(upstream.DedicatedOptions{Server: upstream.ServerOptions{
		ID:             "alpha",
		Endpoint:       endpoint,
		CallTimeout:    5 * time.Second,
		ConnectTimeout: 5 * time.Second,
	}})
	if err != nil {
		t.Fatalf("NewDedicated: %v", err)
	}
	t.Cleanup(func() { _ = dedicated.Close() })
	return dedicated
}

// One dedicated runtime owns exactly one session per server: repeated
// discovery must not re-initialize, and every tool of a paginated tools/list
// must be discovered with its schema preserved.
func TestDedicatedDiscoveryPaginatesWithOneSession(t *testing.T) {
	nested := json.RawMessage(`{"type":"object","properties":{"x":{"type":"integer"}},"required":["x"]}`)
	other := json.RawMessage(`{"type":"object","properties":{"limit":{"type":"integer"}}}`)
	httpServer, state := newPeer(t, &mcp.ServerOptions{PageSize: 2}, func(server *mcp.Server, _ *peerState) {
		for _, tool := range []*mcp.Tool{
			{Name: "alpha_one", Description: "first", InputSchema: other},
			{Name: "alpha_two", Description: "second", InputSchema: other},
			{Name: "alpha_three", Description: "third", InputSchema: nested},
			{Name: "alpha_four", Description: "fourth", InputSchema: other},
			{Name: "alpha_five", Description: "fifth", InputSchema: other},
		} {
			server.AddTool(tool, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
			})
		}
	})
	dedicated := newDedicated(t, httpServer.URL)

	first, err := dedicated.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	second, err := dedicated.Discover(context.Background())
	if err != nil {
		t.Fatalf("second Discover: %v", err)
	}
	if state.initializes.Load() != 1 {
		t.Fatalf("initialize requests=%d, want 1 (one session per server lifecycle)", state.initializes.Load())
	}
	if len(first) != 5 || len(second) != 5 {
		t.Fatalf("discovered tools=%d/%d, want 5/5 across pages", len(first), len(second))
	}
	names := make([]string, 0, len(first))
	byName := map[string]core.ToolDefinition{}
	for _, def := range first {
		names = append(names, def.RawName)
		byName[def.RawName] = def
	}
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	if !reflect.DeepEqual(names, sorted) {
		t.Fatalf("discovered names %v are not in deterministic sorted order", names)
	}
	exact, ok := byName["alpha_three"]
	if !ok {
		t.Fatalf("missing paginated tool alpha_three in %v", names)
	}
	if exact.Description != "third" {
		t.Fatalf("description=%q, want %q", exact.Description, "third")
	}
	var wantSchema, gotSchema any
	if err := json.Unmarshal(nested, &wantSchema); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(exact.InputSchema, &gotSchema); err != nil {
		t.Fatalf("input schema is not JSON: %v (%s)", err, exact.InputSchema)
	}
	if !reflect.DeepEqual(wantSchema, gotSchema) {
		t.Fatalf("input schema=%s, want semantic %s", exact.InputSchema, nested)
	}
	if bytes.ContainsAny(exact.InputSchema, " \n\t") {
		t.Fatalf("input schema %s is not stored compact", exact.InputSchema)
	}
}

// One forward = exactly one upstream execution with byte-exact args, and the
// exact upstream result bytes come back for the provenance envelope.
func TestDedicatedCallForwardsExactArgsOnce(t *testing.T) {
	httpServer, state := newPeer(t, nil, func(server *mcp.Server, state *peerState) {
		server.AddTool(&mcp.Tool{Name: "read", InputSchema: json.RawMessage(`{"type":"object"}`)},
			func(_ context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				state.executions.Add(1)
				state.lastArgs.Store(string(request.Params.Arguments))
				return &mcp.CallToolResult{StructuredContent: map[string]any{"echo": json.RawMessage(request.Params.Arguments)}}, nil
			})
	})
	dedicated := newDedicated(t, httpServer.URL)

	args := json.RawMessage(`{"limit":1,"ratio":1.10}`)
	result, outcome := dedicated.CallTool(context.Background(), "t_x", "alpha", "read", args)
	if outcome != nil {
		t.Fatalf("outcome=%+v, want nil for a complete call", outcome)
	}
	if state.executions.Load() != 1 {
		t.Fatalf("upstream executions=%d, want exactly 1", state.executions.Load())
	}
	if got := state.lastArgs.Load(); got != string(args) {
		t.Fatalf("upstream received args %q, want byte-exact %q", got, args)
	}
	if len(result) == 0 || !json.Valid(result) {
		t.Fatalf("result=%s, want the raw upstream CallToolResult JSON", result)
	}
	// The upstream Result is carried as the bridge received it. Note that the SDK
	// itself decodes CallToolResult.StructuredContent into `any`, so numeric
	// literals inside the upstream payload are already normalised by the SDK
	// before the bridge sees them; the echoed args only need the same numeric
	// value here. Client-supplied args bytes are asserted byte-exact above.
	var decoded struct {
		StructuredContent struct {
			Echo struct {
				Limit json.Number `json:"limit"`
				Ratio json.Number `json:"ratio"`
			} `json:"echo"`
		} `json:"structuredContent"`
	}
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if decoded.StructuredContent.Echo.Limit.String() != "1" || decoded.StructuredContent.Echo.Ratio.String() != "1.1" {
		t.Fatalf("echoed args=%+v, want limit 1 and ratio 1.1", decoded.StructuredContent.Echo)
	}
}

// An input-required upstream result is reported as such after exactly one
// execution: the bridge never fulfills or re-dispatches interactions.
func TestDedicatedInputRequiredNeverRedispatches(t *testing.T) {
	httpServer, state := newPeer(t, nil, func(server *mcp.Server, state *peerState) {
		server.AddTool(&mcp.Tool{Name: "confirm", InputSchema: json.RawMessage(`{"type":"object"}`)},
			func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				state.executions.Add(1)
				return &mcp.CallToolResult{
					InputRequests: mcp.InputRequestMap{"ask": &mcp.ElicitParams{Message: "confirm"}},
					RequestState:  "state-token",
				}, nil
			})
	})
	dedicated := newDedicated(t, httpServer.URL)

	result, outcome := dedicated.CallTool(context.Background(), "t_y", "alpha", "confirm", json.RawMessage(`{}`))
	if result != nil || outcome == nil || outcome.Failure == nil {
		t.Fatalf("result=%s outcome=%+v, want a structured failure", result, outcome)
	}
	if outcome.Failure.Code != "unsupported_capability" || outcome.Failure.ExecutionState != core.ExecutionUnknown {
		t.Fatalf("failure=%+v, want unsupported_capability/unknown", outcome.Failure)
	}
	if state.executions.Load() != 1 {
		t.Fatalf("upstream executions=%d, want exactly 1", state.executions.Load())
	}
}

// Close releases the session exactly once and later calls fail closed with an
// immediate, non-executing error.
func TestDedicatedCloseIsIdempotentAndFailsClosed(t *testing.T) {
	httpServer, _ := newPeer(t, nil, func(server *mcp.Server, _ *peerState) {
		server.AddTool(&mcp.Tool{Name: "read", InputSchema: json.RawMessage(`{"type":"object"}`)},
			func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
			})
	})
	dedicated := newDedicated(t, httpServer.URL)
	if _, err := dedicated.Discover(context.Background()); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if err := dedicated.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := dedicated.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := dedicated.Discover(context.Background()); err == nil {
		t.Fatal("Discover after Close succeeded; want closed error")
	} else if !upstream.IsUnavailable(err) {
		t.Fatalf("Discover after Close error=%v, want backend_unavailable", err)
	}
	_, outcome := dedicated.CallTool(context.Background(), "t_x", "alpha", "read", json.RawMessage(`{}`))
	if outcome == nil || outcome.Failure == nil || outcome.Failure.Code != "backend_unavailable" || outcome.Failure.ExecutionState != core.ExecutionNotStarted {
		t.Fatalf("outcome=%+v, want backend_unavailable/not_started closed outcome", outcome)
	}
}
