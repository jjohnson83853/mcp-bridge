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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
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
	return newDedicatedWithStateless(t, endpoint, false)
}

func newDedicatedWithStateless(t *testing.T, endpoint string, stateless bool) *upstream.Dedicated {
	t.Helper()
	dedicated, err := upstream.NewDedicated(upstream.DedicatedOptions{Stateless: stateless, Logger: observe.NewLogger(io.Discard, io.Discard), Server: upstream.ServerOptions{
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

// Stateless calls use a fresh MCP session after the peer handler is replaced.
func TestDedicatedStatelessToolCallsSurvivesServerRestart(t *testing.T) {
	newPeerHandler := func(generation string) (*peerState, http.Handler) {
		state := &peerState{}
		server := mcp.NewServer(&mcp.Implementation{Name: "peer", Version: "1"}, nil)
		server.AddTool(&mcp.Tool{Name: "read", InputSchema: json.RawMessage(`{"type":"object"}`)},
			func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				state.executions.Add(1)
				return &mcp.CallToolResult{StructuredContent: map[string]any{"generation": generation}}, nil
			})
		inner := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
		return state, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
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
	}

	state, activeHandler := newPeerHandler("before")
	var handlerMu sync.RWMutex
	httpServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		handlerMu.RLock()
		handler := activeHandler
		handlerMu.RUnlock()
		handler.ServeHTTP(writer, request)
	}))
	t.Cleanup(httpServer.Close)
	dedicated := newDedicatedWithStateless(t, httpServer.URL, true)

	callAndCheckGeneration := func(want string) {
		t.Helper()
		result, outcome := dedicated.CallTool(context.Background(), "t_read", "alpha", "read", json.RawMessage(`{}`))
		if outcome != nil {
			t.Fatalf("CallTool outcome=%+v, want success", outcome)
		}
		var decoded struct {
			StructuredContent struct {
				Generation string `json:"generation"`
			} `json:"structuredContent"`
		}
		if err := json.Unmarshal(result, &decoded); err != nil {
			t.Fatalf("decode CallTool result %s: %v", result, err)
		}
		if decoded.StructuredContent.Generation != want {
			t.Fatalf("generation=%q, want %q", decoded.StructuredContent.Generation, want)
		}
	}

	callAndCheckGeneration("before")
	if state.initializes.Load() != 1 {
		t.Fatalf("initializations before restart=%d, want 1", state.initializes.Load())
	}

	handlerMu.Lock()
	state, activeHandler = newPeerHandler("after")
	handlerMu.Unlock()
	callAndCheckGeneration("after")
	if state.initializes.Load() != 1 {
		t.Fatalf("initializations after restart=%d, want fresh session initialization", state.initializes.Load())
	}
	if state.executions.Load() != 1 {
		t.Fatalf("executions after restart=%d, want 1", state.executions.Load())
	}
}

// Stateful calls retain one initialized MCP session across tool executions.
func TestDedicatedStatefulPreservesSession(t *testing.T) {
	httpServer, state := newPeer(t, nil, func(server *mcp.Server, state *peerState) {
		server.AddTool(&mcp.Tool{Name: "read", InputSchema: json.RawMessage(`{"type":"object"}`)},
			func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				state.executions.Add(1)
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
			})
	})
	dedicated := newDedicatedWithStateless(t, httpServer.URL, false)

	for i := 0; i < 2; i++ {
		if _, outcome := dedicated.CallTool(context.Background(), "t_read", "alpha", "read", json.RawMessage(`{}`)); outcome != nil {
			t.Fatalf("CallTool outcome=%+v, want success", outcome)
		}
	}
	if state.initializes.Load() != 1 {
		t.Fatalf("initializations=%d, want 1 persistent session", state.initializes.Load())
	}
	if state.executions.Load() != 2 {
		t.Fatalf("executions=%d, want 2", state.executions.Load())
	}
}

func TestDedicatedAdaptiveEphemeralHandshake(t *testing.T) {
	httpServer, _ := newPeer(t, nil, func(server *mcp.Server, _ *peerState) {
		server.AddTool(&mcp.Tool{Name: "read", InputSchema: json.RawMessage(`{"type":"object"}`)},
			func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
			})
	})
	buf := &bytes.Buffer{}
	dedicated, err := upstream.NewDedicated(upstream.DedicatedOptions{
		Stateless: true,
		Logger:    observe.NewLogger(buf, io.Discard),
		Server: upstream.ServerOptions{
			ID:       "alpha",
			Endpoint: httpServer.URL,
		},
	})
	if err != nil {
		t.Fatalf("NewDedicated: %v", err)
	}
	t.Cleanup(func() { _ = dedicated.Close() })

	if _, err := dedicated.Discover(context.Background()); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if !dedicated.RequiresHandshake() {
		t.Fatal("RequiresHandshake() = false after discovering a legacy upstream")
	}
	if _, outcome := dedicated.CallTool(context.Background(), "t_read", "alpha", "read", json.RawMessage(`{}`)); outcome != nil {
		t.Fatalf("CallTool outcome=%+v, want success", outcome)
	}
	for _, want := range []string{
		`"schema_version":1`, `"level":"info"`, `"service":"mcp-bridge"`,
		`"event":"upstream_ephemeral_handshake"`, `"operation":"handshake"`, `"outcome":"success"`,
		`"message":"upstream protocol handshake observed"`, `"backend_id":"alpha"`, `"duration_ms":`,
	} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Errorf("handshake log %q does not contain %q", buf.String(), want)
		}
	}
}

func TestNewDedicatedRequiresLogger(t *testing.T) {
	dedicated, err := upstream.NewDedicated(upstream.DedicatedOptions{
		Server: upstream.ServerOptions{ID: "alpha", Endpoint: "http://127.0.0.1:1"},
	})
	if err == nil || dedicated != nil {
		t.Fatalf("NewDedicated without logger = (%v, %v); want nil and an error", dedicated, err)
	}
}

func TestDedicatedDiscoveryFailureReportsStageStatusAndSanitizedEndpoint(t *testing.T) {
	peer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("X-Private", "header-secret")
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte("body-secret"))
	}))
	t.Cleanup(peer.Close)

	var logs bytes.Buffer
	dedicated, err := upstream.NewDedicated(upstream.DedicatedOptions{
		Logger:        observe.NewLogger(&logs, &logs),
		RetryInterval: time.Minute,
		Server: upstream.ServerOptions{
			ID:                  "alpha",
			Endpoint:            peer.URL + "/mcp?token=query-secret",
			BearerToken:         "credential-secret",
			Headers:             http.Header{"X-Private": {"header-secret"}},
			ConnectTimeout:      time.Second,
			TransportMaxRetries: 1,
		},
	})
	if err != nil {
		t.Fatalf("NewDedicated: %v", err)
	}
	t.Cleanup(func() { _ = dedicated.Close() })

	if _, err := dedicated.Discover(context.Background()); err == nil {
		t.Fatal("Discover succeeded against an unauthorized peer")
	}
	if logs.Len() == 0 {
		t.Fatal("unauthorized discovery emitted no attribution record")
	}

	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &record); err != nil {
		t.Fatalf("decode discovery attribution record: %v", err)
	}
	if record["event"] != "upstream_discovery_failed" || record["level"] != "warn" || record["server"] != "alpha" {
		t.Fatalf("discovery record = %#v; want warn upstream_discovery_failed for alpha", record)
	}
	if record["failure_stage"] != "auth" || record["failure_reason"] != "authentication_rejected_during_initialize" {
		t.Errorf("stage/reason = %v/%v; want auth/authentication_rejected_during_initialize", record["failure_stage"], record["failure_reason"])
	}
	if status, ok := record["http_status"].(float64); !ok || status != http.StatusUnauthorized {
		t.Errorf("http_status = %v; want %d", record["http_status"], http.StatusUnauthorized)
	}
	if record["endpoint"] != peer.URL+"/mcp" {
		t.Errorf("endpoint = %v; want sanitized scheme/host/port/path", record["endpoint"])
	}
	if _, exists := record["next_attempt_ms"]; exists {
		t.Errorf("next_attempt_ms present for correction-required authentication failure")
	}
	errorInfo, ok := record["error"].(map[string]any)
	if !ok || errorInfo["code"] != "client_missing" || errorInfo["kind"] != "configuration" || errorInfo["retryable"] != false {
		t.Errorf("error = %#v; want client_missing configuration retryable=false", record["error"])
	}
	for _, secret := range []string{"credential-secret", "header-secret", "query-secret", "body-secret"} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("discovery record leaked %s", secret)
		}
	}
}

func TestDedicatedToolsListTimeoutReportsClassValueAndScheduledRetry(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "peer", Version: "1"}, nil)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	listStarted := make(chan struct{}, 1)
	peer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read peer request: %v", err)
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
		if bytes.Contains(body, []byte(`"method":"tools/list"`)) {
			select {
			case listStarted <- struct{}{}:
			default:
			}
			<-request.Context().Done()
			return
		}
		handler.ServeHTTP(writer, request)
	}))
	t.Cleanup(peer.Close)

	var logs bytes.Buffer
	dedicated, err := upstream.NewDedicated(upstream.DedicatedOptions{
		Logger:        observe.NewLogger(&logs, &logs),
		RetryInterval: time.Minute,
		Server: upstream.ServerOptions{
			ID:                  "alpha",
			Endpoint:            peer.URL + "/mcp",
			ConnectTimeout:      time.Second,
			TransportMaxRetries: 1,
		},
	})
	if err != nil {
		t.Fatalf("NewDedicated: %v", err)
	}
	t.Cleanup(func() { _ = dedicated.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	discoveryDone := make(chan error, 1)
	go func() {
		_, err := dedicated.Discover(ctx)
		discoveryDone <- err
	}()
	select {
	case <-listStarted:
	case err := <-discoveryDone:
		t.Fatalf("discovery failed before tools/list: %v", err)
	case <-time.After(time.Second):
		t.Fatal("tools/list request did not start")
	}
	if err := <-discoveryDone; err == nil {
		t.Fatal("Discover succeeded after tools/list timeout")
	}

	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &record); err != nil {
		t.Fatalf("decode discovery timeout record: %v", err)
	}
	if record["level"] != "warn" || record["event"] != "upstream_discovery_failed" ||
		record["failure_stage"] != "tools_list" {
		t.Fatalf("tools-list timeout attribution = %#v", record)
	}
	reason, ok := record["failure_reason"].(string)
	const prefix = "timeout_context_deadline_exceeded_"
	if !ok || !strings.HasPrefix(reason, prefix) || !strings.HasSuffix(reason, "ms") {
		t.Fatalf("failure_reason = %v; want bounded timeout class and value", record["failure_reason"])
	}
	milliseconds, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(reason, prefix), "ms"))
	if err != nil || milliseconds <= 0 || milliseconds > 500 {
		t.Fatalf("timeout value in failure_reason = %q; want 1..500ms", reason)
	}
	if _, exists := record["http_status"]; exists {
		t.Errorf("http_status present although tools/list timed out before a response")
	}
	errorInfo, ok := record["error"].(map[string]any)
	if !ok || errorInfo["code"] != "upstream_unavailable" || errorInfo["retryable"] != true {
		t.Errorf("error = %#v; want retryable upstream_unavailable", record["error"])
	}
	if retry, ok := record["next_attempt_ms"].(float64); !ok || retry != float64(time.Minute.Milliseconds()) {
		t.Errorf("next_attempt_ms = %v; want scheduled one-minute retry", record["next_attempt_ms"])
	}
}
