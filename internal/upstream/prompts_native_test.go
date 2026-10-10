package upstream

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPromptGetRoutingAndSafeErrors(t *testing.T) {
	client := &ServerClient{callTimeout: time.Millisecond}
	_, err := client.GetPrompt(context.Background(), nil, &mcp.GetPromptParams{Name: "secret"})
	var callErr *CallError
	if !errors.As(err, &callErr) || callErr.Code != "backend_unavailable" || callErr.ExecutionState != "not_started" {
		t.Fatalf("nil-session error = %#v, want unavailable/not_started", err)
	}
	dedicated := &Dedicated{client: &ServerClient{ID: "alpha"}}
	_, err = dedicated.GetPrompt(context.Background(), "beta", "raw-secret", nil)
	if err == nil || err.Error() != "upstream: server identity mismatch" {
		t.Fatalf("identity guard error = %v", err)
	}
}

// promptPeer exposes one upstream prompt and counts handler invocations so a
// test can prove exactly-once execution without inspecting internal state.
func promptPeer(t *testing.T, options ServerOptions, handler mcp.PromptHandler) (*httptest.Server, *ServerClient, *mcp.ClientSession, *atomic.Int32) {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "prompt-peer", Version: "1"}, nil)
	calls := &atomic.Int32{}
	server.AddPrompt(&mcp.Prompt{Name: "peer-prompt"}, func(ctx context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		calls.Add(1)
		return handler(ctx, request)
	})
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil))
	t.Cleanup(httpServer.Close)
	options.ID, options.Endpoint = "peer", httpServer.URL
	options.ConnectTimeout = time.Second
	if options.CallTimeout == 0 {
		options.CallTimeout = 2 * time.Second
	}
	if options.BreakerThreshold == 0 {
		options.BreakerThreshold = 5
	}
	client, err := NewServerClient(options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.HTTPClient.CloseIdleConnections)
	session, err := client.Connect(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return httpServer, client, session, calls
}

func failureCount(client *ServerClient) int {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.failures
}

func requireCallError(t *testing.T, err error, code, state string) *CallError {
	t.Helper()
	var callErr *CallError
	if !errors.As(err, &callErr) {
		t.Fatalf("error = %#v, want *CallError %s/%s", err, code, state)
	}
	if callErr.Code != code || callErr.ExecutionState != state {
		t.Fatalf("error = %+v, want code=%s state=%s", callErr, code, state)
	}
	return callErr
}

func TestPromptGetGuardedWirePaths(t *testing.T) {
	t.Run("deadline records one failure and never retries", func(t *testing.T) {
		// The handler parks until the deadline fires or the test releases it, so
		// the peer is genuinely in flight and the test can never hang.
		release := make(chan struct{})
		_, client, session, calls := promptPeer(t, ServerOptions{CallTimeout: 30 * time.Millisecond}, func(ctx context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-release:
				return nil, errors.New("released after deadline")
			}
		})
		t.Cleanup(func() { close(release) })
		_, err := client.GetPrompt(context.Background(), session, &mcp.GetPromptParams{Name: "peer-prompt"})
		requireCallError(t, err, "deadline_exceeded", "unknown")
		if got := calls.Load(); got != 1 {
			t.Fatalf("prompt invocations=%d, want exactly 1", got)
		}
		if got := failureCount(client); got != 1 {
			t.Fatalf("recorded failures=%d, want 1", got)
		}
	})

	t.Run("upstream protocol error is safe and not a transport fault", func(t *testing.T) {
		const secret = "raw-secret-name /srv/private/socket transport failure"
		_, client, session, calls := promptPeer(t, ServerOptions{}, func(context.Context, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return nil, errors.New(secret)
		})
		_, err := client.GetPrompt(context.Background(), session, &mcp.GetPromptParams{Name: "peer-prompt"})
		callErr := requireCallError(t, err, "upstream_protocol_error", "unknown")
		if got := calls.Load(); got != 1 {
			t.Fatalf("prompt invocations=%d, want exactly 1", got)
		}
		if got := failureCount(client); got != 0 {
			t.Fatalf("recorded failures=%d, want 0 for a protocol error", got)
		}
		if callErr.Error() != "upstream_protocol_error" {
			t.Fatalf("error text = %q, must be the safe code only", callErr.Error())
		}
	})

	t.Run("transport failure after the peer disappears", func(t *testing.T) {
		httpServer, client, session, _ := promptPeer(t, ServerOptions{}, func(context.Context, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{}, nil
		})
		// CloseClientConnections drops the live connection without waiting for it
		// (Close alone would block on the keep-alive request); Close then takes
		// the listener down so the SDK cannot silently reconnect and succeed.
		httpServer.CloseClientConnections()
		httpServer.Close()
		_, err := client.GetPrompt(context.Background(), session, &mcp.GetPromptParams{Name: "peer-prompt"})
		requireCallError(t, err, "backend_unavailable", "unknown")
		if got := failureCount(client); got != 1 {
			t.Fatalf("recorded failures=%d, want 1 transport fault", got)
		}
	})

	t.Run("saturated admission is refused before execution", func(t *testing.T) {
		_, client, session, calls := promptPeer(t, ServerOptions{MaxConcurrentCalls: 1}, func(context.Context, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{}, nil
		})
		client.slots <- struct{}{}
		_, err := client.GetPrompt(context.Background(), session, &mcp.GetPromptParams{Name: "peer-prompt"})
		<-client.slots
		requireCallError(t, err, "backend_busy", "not_started")
		if got := calls.Load(); got != 0 {
			t.Fatalf("prompt invocations=%d, want 0", got)
		}
	})

	t.Run("cancellation before admission never executes", func(t *testing.T) {
		_, client, session, calls := promptPeer(t, ServerOptions{}, func(context.Context, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{}, nil
		})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := client.GetPrompt(ctx, session, &mcp.GetPromptParams{Name: "peer-prompt"})
		requireCallError(t, err, "backend_unavailable", "not_started")
		if got := calls.Load(); got != 0 {
			t.Fatalf("prompt invocations=%d, want 0", got)
		}
	})

	t.Run("cancellation mid-call is internal_error without a breaker fault", func(t *testing.T) {
		entered := make(chan struct{})
		release := make(chan struct{})
		_, client, session, calls := promptPeer(t, ServerOptions{CallTimeout: 5 * time.Second}, func(ctx context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			close(entered)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-release:
				return nil, errors.New("released after cancellation")
			}
		})
		t.Cleanup(func() { close(release) })
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			_, err := client.GetPrompt(ctx, session, &mcp.GetPromptParams{Name: "peer-prompt"})
			done <- err
		}()
		<-entered
		cancel()
		err := <-done
		requireCallError(t, err, "internal_error", "unknown")
		if got := calls.Load(); got != 1 {
			t.Fatalf("prompt invocations=%d, want exactly 1", got)
		}
		// The breaker counts connect/transport/malformed/deadline only, so a
		// caller-side cancellation must not advance the failure count.
		if got := failureCount(client); got != 0 {
			t.Fatalf("recorded failures=%d, want 0 for a cancellation", got)
		}
	})
}
