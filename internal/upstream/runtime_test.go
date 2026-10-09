package upstream_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"errors"
	"github.com/jjohnson83853/mcp-bridge/internal/upstream"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestClientCallDeadlineDoesNotSetGlobalHTTPTimeout(t *testing.T) {
	client, err := upstream.NewClient(upstream.ClientOptions{CallTimeout: 150 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if client.Timeout != 0 {
		t.Fatalf("http.Client.Timeout=%s would truncate long-lived streams", client.Timeout)
	}
	ctx, cancel := upstream.CallContext(context.Background(), 150*time.Millisecond)
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > 150*time.Millisecond {
		t.Fatalf("call context deadline=%v, ok=%v", deadline, ok)
	}
}

func TestServerClientUsesPrivateCredentialsAndIndependentIdentity(t *testing.T) {
	var outbound *http.Request
	rt := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		outbound = request
		return nil, context.DeadlineExceeded
	})
	first, err := upstream.NewServerClient(upstream.ServerOptions{ID: "alpha", Endpoint: "https://alpha.invalid/mcp", BearerToken: "backend-one", CallTimeout: time.Second, RoundTripper: rt})
	if err != nil {
		t.Fatal(err)
	}
	second, err := upstream.NewServerClient(upstream.ServerOptions{ID: "beta", Endpoint: "https://beta.invalid/mcp", BearerToken: "backend-two", CallTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || first.HTTPClient == second.HTTPClient || first.MCPClient == second.MCPClient {
		t.Fatal("server clients share identity or transport state")
	}
	if first.NewTransport().MaxRetries != 5 || first.NewTransport().HTTPClient != first.HTTPClient {
		t.Fatal("transport retry budget or dedicated HTTP client was not set")
	}
	if first.HTTPClient.Timeout != 0 || second.HTTPClient.Timeout != 0 {
		t.Fatal("per-call deadlines must not truncate streams globally")
	}
	request, _ := http.NewRequest(http.MethodPost, first.Endpoint, nil)
	request.Header.Set("Authorization", "Bearer client-token")
	request.Header.Set("Cookie", "client-session=private")
	_, _ = first.HTTPClient.Transport.RoundTrip(request)
	if request.Header.Get("Authorization") != "Bearer client-token" {
		t.Fatal("credentials mutated inbound request")
	}
	if outbound == nil || outbound.Header.Get("Authorization") != "Bearer backend-one" {
		t.Fatal("backend credential not applied to cloned request")
	}
	if outbound.Header.Get("Cookie") != "" {
		t.Fatal("cookie crossed the server boundary")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestCallAdmissionIsBoundedAndBreakerProbeDoesNotAdmitTools(t *testing.T) {
	now := time.Unix(1_000, 0)
	client, err := upstream.NewServerClient(upstream.ServerOptions{ID: "alpha", Endpoint: "https://alpha.invalid/mcp", MaxConcurrentCalls: 1, BreakerThreshold: 1, BreakerOpenDuration: time.Second, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	release, err := client.AdmitCall(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.AdmitCall(context.Background()); !upstream.IsBusy(err) {
		t.Fatalf("second call error=%v, want immediate busy", err)
	}
	release()
	client.RecordFailure(upstream.FailureTransport)
	if _, err := client.AdmitCall(context.Background()); !upstream.IsUnavailable(err) {
		t.Fatalf("open breaker error=%v", err)
	}
	now = now.Add(2 * time.Second)
	probe, err := client.BeginProbe()
	if err != nil {
		t.Fatal(err)
	}
	if probe == nil {
		t.Fatal("expected probe completion callback")
	}
	if _, err := client.AdmitCall(context.Background()); !upstream.IsUnavailable(err) {
		t.Fatalf("half-open admitted tool call: %v", err)
	}
	probe(true)
	if _, err := client.AdmitCall(context.Background()); err != nil {
		t.Fatalf("healthy probe did not close breaker: %v", err)
	}
}

func TestBreakerCountsConsecutiveTransportFaultsOnly(t *testing.T) {
	client, err := upstream.NewServerClient(upstream.ServerOptions{ID: "alpha", Endpoint: "https://alpha.invalid/mcp", BreakerThreshold: 2})
	if err != nil {
		t.Fatal(err)
	}
	client.RecordFailure(upstream.FailureTransport)
	client.RecordSuccess()
	client.RecordFailure(upstream.FailureBusiness)
	client.RecordFailure(upstream.FailureTransport)
	if _, err := client.AdmitCall(context.Background()); err != nil {
		t.Fatalf("breaker opened before consecutive threshold: %v", err)
	}
	client.RecordFailure(upstream.FailureTransport)
	if _, err := client.AdmitCall(context.Background()); !upstream.IsUnavailable(err) {
		t.Fatalf("breaker did not open at threshold: %v", err)
	}
}

func TestServerCallSurfacesInputRequiredAfterOneDispatch(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "upstream", Version: "1"}, nil)
	var calls atomic.Int32
	server.AddTool(&mcp.Tool{Name: "needs-input", InputSchema: json.RawMessage(`{"type":"object"}`)}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		calls.Add(1)
		return &mcp.CallToolResult{
			InputRequests: mcp.InputRequestMap{"confirm": &mcp.ElicitParams{Mode: "form", Message: "confirm"}},
			RequestState:  "state",
		}, nil
	})
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true}))
	defer httpServer.Close()
	client, err := upstream.NewServerClient(upstream.ServerOptions{ID: "alpha", Endpoint: httpServer.URL})
	if err != nil {
		t.Fatal(err)
	}
	session, err := client.Connect(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if got := session.InitializeResult().ProtocolVersion; got != "2026-07-28" {
		t.Fatalf("negotiated protocol=%q, want latest 2026-07-28", got)
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "needs-input"})
	if err != nil {
		t.Fatalf("latest InputRequired result returned an error: %v", err)
	}
	if !result.NeedsInput() || len(result.InputRequests) != 1 || result.InputRequests["confirm"] == nil || result.RequestState != "state" {
		t.Fatalf("InputRequired result did not surface unchanged: %#v", result)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("tools/call dispatches=%d, want exactly one", got)
	}
}

func TestSDKClientWithElicitationRetriesInputRequiredCall(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "upstream", Version: "1"}, nil)
	var calls atomic.Int32
	server.AddTool(&mcp.Tool{Name: "needs-input", InputSchema: json.RawMessage(`{"type":"object"}`)}, func(_ context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		calls.Add(1)
		if len(request.Params.InputResponses) == 0 {
			return &mcp.CallToolResult{
				InputRequests: mcp.InputRequestMap{"confirm": &mcp.ElicitParams{Mode: "form", Message: "confirm"}},
				RequestState:  "state",
			}, nil
		}
		if _, ok := request.Params.InputResponses["confirm"].(*mcp.ElicitResult); !ok {
			return nil, errors.New("missing elicitation response")
		}
		return &mcp.CallToolResult{}, nil
	})
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true}))
	defer httpServer.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "characterization-client", Version: "1"}, &mcp.ClientOptions{
		ElicitationHandler: func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: "accept", Content: map[string]any{"confirmed": true}}, nil
		},
	})
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: httpServer.URL, DisableStandaloneSSE: true}, &mcp.ClientSessionOptions{ProtocolVersion: "2026-07-28"})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "needs-input"})
	if err != nil {
		t.Fatal(err)
	}
	if result.NeedsInput() || calls.Load() != 2 {
		t.Fatalf("result NeedsInput=%v, tool executions=%d; want completed result after exactly two executions", result.NeedsInput(), calls.Load())
	}
}

func TestLegacyServerInputRequiredMapsToUnsupportedCapability(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "upstream", Version: "1"}, &mcp.ServerOptions{SupportedProtocolVersions: []string{"2025-11-25"}})
	var calls atomic.Int32
	server.AddTool(&mcp.Tool{Name: "needs-input", InputSchema: json.RawMessage(`{"type":"object"}`)}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		calls.Add(1)
		return &mcp.CallToolResult{InputRequests: mcp.InputRequestMap{"confirm": &mcp.ElicitParams{Mode: "form", Message: "confirm"}}, RequestState: "state"}, nil
	})
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil))
	defer httpServer.Close()
	client, err := upstream.NewServerClient(upstream.ServerOptions{ID: "alpha", Endpoint: httpServer.URL})
	if err != nil {
		t.Fatal(err)
	}
	session, err := client.Connect(context.Background(), &mcp.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	_, err = client.CallTool(context.Background(), session, &mcp.CallToolParams{Name: "needs-input"})
	var callErr *upstream.CallError
	if !errors.As(err, &callErr) || callErr.Code != "unsupported_capability" || callErr.Retryable || callErr.ExecutionState != "unknown" {
		t.Fatalf("call error=%v; want unsupported_capability/unknown/nonretryable", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("tools/call dispatches=%d, want exactly one", got)
	}
}
