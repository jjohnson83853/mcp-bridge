package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func runtimePeer(t *testing.T, options ServerOptions, register func(*mcp.Server, *atomic.Int32)) (*httptest.Server, *ServerClient) {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "edge-peer", Version: "1"}, nil)
	calls := &atomic.Int32{}
	register(server, calls)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)
	options.ID, options.Endpoint = "edge", httpServer.URL
	options.ConnectTimeout = time.Second
	client, err := NewServerClient(options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.HTTPClient.CloseIdleConnections)
	return httpServer, client
}

func TestServerCallAssemblesToolResultContent(t *testing.T) {
	_, client := runtimePeer(t, ServerOptions{}, func(server *mcp.Server, calls *atomic.Int32) {
		server.AddTool(&mcp.Tool{Name: "multi", InputSchema: json.RawMessage(`{"type":"object"}`)}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			calls.Add(1)
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "first"}, &mcp.TextContent{Text: "second"}}, StructuredContent: map[string]any{"ok": false}}, nil
		})
	})
	session, err := client.Connect(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := client.CallTool(context.Background(), session, &mcp.CallToolParams{Name: "multi"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		IsError bool `json:"isError"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Structured map[string]bool `json:"structuredContent"`
	}
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if !got.IsError || len(got.Content) != 2 || got.Content[0].Text != "first" || got.Content[1].Text != "second" || got.Structured["ok"] {
		t.Fatalf("assembled result %s", encoded)
	}
}

func TestServerCallReportsBusyWhenConcurrencyLimitIsOccupied(t *testing.T) {
	entered, unblock := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	_, client := runtimePeer(t, ServerOptions{MaxConcurrentCalls: 1}, func(server *mcp.Server, _ *atomic.Int32) {
		server.AddTool(&mcp.Tool{Name: "hold", InputSchema: json.RawMessage(`{"type":"object"}`)}, func(ctx context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			calls.Add(1)
			close(entered)
			select {
			case <-unblock:
				return &mcp.CallToolResult{}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		})
	})
	session, err := client.Connect(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	finished := make(chan error, 1)
	go func() {
		_, err := client.CallTool(context.Background(), session, &mcp.CallToolParams{Name: "hold"})
		finished <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first call did not enter")
	}
	_, err = client.CallTool(context.Background(), session, &mcp.CallToolParams{Name: "hold"})
	var callErr *CallError
	if !errors.As(err, &callErr) || callErr.Code != "backend_busy" || callErr.ExecutionState != "not_started" {
		t.Fatalf("second call error=%v, want backend_busy/not_started", err)
	}
	close(unblock)
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("first call did not finish")
	}
	if calls.Load() != 1 {
		t.Fatalf("executed calls=%d, want 1", calls.Load())
	}
}

func TestServerCallBreakerAndHalfOpenProbeTransitions(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var executions atomic.Int32
	_, client := runtimePeer(t, ServerOptions{BreakerThreshold: 1, BreakerOpenDuration: time.Minute, Now: func() time.Time { return now }}, func(server *mcp.Server, _ *atomic.Int32) {
		server.AddTool(&mcp.Tool{Name: "read", InputSchema: json.RawMessage(`{"type":"object"}`)}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			executions.Add(1)
			return &mcp.CallToolResult{}, nil
		})
	})
	session, err := client.Connect(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	client.RecordFailure(FailureTransport)
	assertUnavailable := func() {
		t.Helper()
		_, err := client.CallTool(context.Background(), session, &mcp.CallToolParams{Name: "read"})
		var callErr *CallError
		if !errors.As(err, &callErr) || callErr.Code != "backend_unavailable" || callErr.ExecutionState != "not_started" {
			t.Fatalf("call error=%v, want backend_unavailable/not_started", err)
		}
	}
	assertUnavailable()
	if executions.Load() != 0 {
		t.Fatalf("executions while open=%d", executions.Load())
	}
	if _, err := client.BeginProbe(); !IsUnavailable(err) {
		t.Fatalf("early probe error=%v, want unavailable", err)
	}
	now = now.Add(time.Minute)
	complete, err := client.BeginProbe()
	if err != nil {
		t.Fatal(err)
	}
	assertUnavailable()
	complete(true)
	if _, err := client.CallTool(context.Background(), session, &mcp.CallToolParams{Name: "read"}); err != nil {
		t.Fatalf("call after successful probe: %v", err)
	}
	if executions.Load() != 1 {
		t.Fatalf("executions=%d, want one after breaker recovery", executions.Load())
	}
}

func TestServerCallWithoutSessionIsNotStarted(t *testing.T) {
	client, err := NewServerClient(ServerOptions{ID: "empty", Endpoint: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.CallTool(context.Background(), nil, &mcp.CallToolParams{Name: "read"})
	var callErr *CallError
	if !errors.As(err, &callErr) || callErr.Code != "backend_unavailable" || callErr.ExecutionState != "not_started" {
		t.Fatalf("nil-session error=%v, want backend_unavailable/not_started", err)
	}
}
