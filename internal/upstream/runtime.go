package upstream

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ServerOptions struct {
	ID                  string
	Endpoint            string
	BearerToken         string
	Headers             http.Header
	ConnectTimeout      time.Duration
	CallTimeout         time.Duration
	MaxConcurrentCalls  int
	BreakerThreshold    int
	BreakerOpenDuration time.Duration
	Now                 func() time.Time
	TransportMaxRetries int
	Transport           *http.Transport
	RoundTripper        http.RoundTripper
}

type ServerClient struct {
	ID                  string
	Endpoint            string
	HTTPClient          *http.Client
	MCPClient           *mcp.Client
	TransportMaxRetries int
	callTimeout         time.Duration
	slots               chan struct{}
	mu                  sync.Mutex
	failures            int
	threshold           int
	openDuration        time.Duration
	openedAt            time.Time
	halfOpen            bool
	probeActive         bool
	now                 func() time.Time
}

type discoveryHTTPStatusKey struct{}

type discoveryHTTPStatus struct {
	mu     sync.Mutex
	status int
}

func (s *discoveryHTTPStatus) reset() {
	s.mu.Lock()
	s.status = 0
	s.mu.Unlock()
}

func (s *discoveryHTTPStatus) set(status int) {
	s.mu.Lock()
	s.status = status
	s.mu.Unlock()
}

func (s *discoveryHTTPStatus) get() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

type discoveryStatusTransport struct {
	next http.RoundTripper
}

func (t discoveryStatusTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.next.RoundTrip(request)
	if response != nil {
		if status, ok := request.Context().Value(discoveryHTTPStatusKey{}).(*discoveryHTTPStatus); ok {
			status.set(response.StatusCode)
		}
	}
	return response, err
}

func (c *ServerClient) trackDiscoveryStatus() {
	next := c.HTTPClient.Transport
	if next == nil {
		next = http.DefaultTransport
	}
	c.HTTPClient.Transport = discoveryStatusTransport{next: next}
}

type runtimeError string

func (e runtimeError) Error() string { return string(e) }

type CallError struct {
	Code           string
	Retryable      bool
	ExecutionState string
}

func (e *CallError) Error() string { return e.Code }

const (
	errBusy        runtimeError = "backend_busy"
	errUnavailable runtimeError = "backend_unavailable"
)

func IsBusy(err error) bool        { return errors.Is(err, errBusy) }
func IsUnavailable(err error) bool { return errors.Is(err, errUnavailable) }

// transportRejectedCode mirrors jsonrpc2.ErrRejected (-32005), the wire code the
// SDK transport uses when it could not deliver a request at all. The SDK
// aliases jsonrpc.Error to jsonrpc2.WireError, so every SDK error matches
// errors.As(*jsonrpc.Error); this code is what separates a transport fault from
// a remote protocol answer.
const transportRejectedCode = -32005

// isTransportRejection reports whether one SDK wire error is a transport-level
// rejection rather than an answer from the peer.
func isTransportRejection(rpcError *jsonrpc.Error) bool {
	return rpcError != nil && rpcError.Code == transportRejectedCode
}

func NewServerClient(opts ServerOptions) (*ServerClient, error) {
	parsed, err := url.ParseRequestURI(opts.Endpoint)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("invalid upstream endpoint")
	}
	if opts.ID == "" {
		return nil, fmt.Errorf("server id is required")
	}
	if opts.MaxConcurrentCalls <= 0 {
		opts.MaxConcurrentCalls = 8
	}
	if opts.BreakerThreshold <= 0 {
		opts.BreakerThreshold = 3
	}
	if opts.BreakerOpenDuration <= 0 {
		opts.BreakerOpenDuration = 30 * time.Second
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.TransportMaxRetries == 0 {
		opts.TransportMaxRetries = 5
	}
	client, err := NewClient(ClientOptions{ConnectTimeout: opts.ConnectTimeout, Transport: opts.Transport})
	if err != nil {
		return nil, err
	}
	base := opts.RoundTripper
	if base == nil {
		base = client.Transport
	}
	client.Transport = credentialsTransport{base: base, bearer: opts.BearerToken, headers: opts.Headers.Clone()}
	return &ServerClient{ID: opts.ID, Endpoint: parsed.String(), HTTPClient: client,
		MCPClient:   NewMCPClient(&mcp.Implementation{Name: "mcp-bridge-" + opts.ID, Version: "1"}),
		callTimeout: opts.CallTimeout, slots: make(chan struct{}, opts.MaxConcurrentCalls),
		TransportMaxRetries: opts.TransportMaxRetries,
		threshold:           opts.BreakerThreshold, openDuration: opts.BreakerOpenDuration, now: opts.Now}, nil
}

func (c *ServerClient) NewTransport() *mcp.StreamableClientTransport {
	return &mcp.StreamableClientTransport{Endpoint: c.Endpoint, HTTPClient: c.HTTPClient, MaxRetries: c.TransportMaxRetries}
}

func (c *ServerClient) Connect(ctx context.Context, options *mcp.ClientSessionOptions) (*mcp.ClientSession, error) {
	return c.MCPClient.Connect(ctx, c.NewTransport(), options)
}

func (c *ServerClient) CallContext(parent context.Context) (context.Context, context.CancelFunc) {
	return CallContext(parent, c.callTimeout)
}

// CallTool applies the per-server timeout and immediate admission rules before
// making one SDK call. It never retries or fulfills interaction requests.
func (c *ServerClient) CallTool(parent context.Context, session *mcp.ClientSession, params *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	if session == nil {
		return nil, &CallError{Code: "backend_unavailable", ExecutionState: "not_started"}
	}
	ctx, cancel := c.CallContext(parent)
	defer cancel()
	release, err := c.AdmitCall(ctx)
	if err != nil {
		if IsBusy(err) {
			return nil, &CallError{Code: "backend_busy", ExecutionState: "not_started"}
		}
		if IsUnavailable(err) {
			return nil, &CallError{Code: "backend_unavailable", ExecutionState: "not_started"}
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, &CallError{Code: "deadline_exceeded", ExecutionState: "not_started"}
		}
		return nil, err
	}
	defer release()
	result, err := session.CallTool(ctx, params)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			c.RecordFailure(FailureDeadline)
			return nil, &CallError{Code: "deadline_exceeded", ExecutionState: "unknown"}
		}
		if errors.Is(err, context.Canceled) {
			c.RecordFailure(FailureCancelled)
			return nil, &CallError{Code: "internal_error", ExecutionState: "unknown"}
		}
		var rpcError *jsonrpc.Error
		if errors.As(err, &rpcError) && !isTransportRejection(rpcError) {
			if strings.HasSuffix(rpcError.Message, "client does not support elicitation") {
				return nil, &CallError{Code: "unsupported_capability", ExecutionState: "unknown"}
			}
			return nil, &CallError{Code: "upstream_protocol_error", ExecutionState: "unknown"}
		}
		c.RecordFailure(FailureTransport)
		return nil, &CallError{Code: "backend_unavailable", ExecutionState: "unknown"}
	}
	if result == nil {
		c.RecordFailure(FailureMalformed)
		return nil, &CallError{Code: "upstream_protocol_error", ExecutionState: "unknown"}
	}
	if result.NeedsInput() {
		return nil, &CallError{Code: "unsupported_capability", ExecutionState: "unknown"}
	}
	c.RecordSuccess()
	return result, nil
}

// AdmitCall never queues behind a saturated server and never admits execution
// during an open or half-open breaker. The returned callback releases a slot.
func (c *ServerClient) AdmitCall(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.halfOpen || !c.openedAt.IsZero() {
		return nil, errUnavailable
	}
	select {
	case c.slots <- struct{}{}:
		return func() { <-c.slots }, nil
	default:
		return nil, errBusy
	}
}

type FailureKind uint8

const (
	FailureTransport FailureKind = iota + 1
	FailureMalformed
	FailureDeadline
	FailureAuthentication
	FailureBusiness
	FailureCancelled
	FailureOverload
)

func (c *ServerClient) RecordFailure(kind FailureKind) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if kind == FailureAuthentication {
		c.openedAt = c.now()
		c.halfOpen = false
		c.probeActive = false
		return
	}
	if kind != FailureTransport && kind != FailureMalformed && kind != FailureDeadline {
		return
	}
	c.failures++
	if c.failures >= c.threshold {
		c.openedAt = c.now()
		c.halfOpen = false
		c.probeActive = false
	}
}

func (c *ServerClient) RecordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.openedAt.IsZero() && !c.halfOpen {
		c.failures = 0
	}
}

// BeginProbe reserves the only half-open probe. The callback completes a
// non-executing initialize/discover/catalog probe; it cannot dispatch tools.
func (c *ServerClient) BeginProbe() (func(bool), error) {
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.openedAt.IsZero() || now.Before(c.openedAt.Add(c.openDuration)) || c.probeActive {
		return nil, errUnavailable
	}
	c.halfOpen, c.probeActive = true, true
	return func(success bool) {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.probeActive = false
		if success {
			c.failures, c.openedAt, c.halfOpen = 0, time.Time{}, false
		} else {
			c.openedAt, c.halfOpen = c.now(), false
		}
	}, nil
}

type credentialsTransport struct {
	base    http.RoundTripper
	bearer  string
	headers http.Header
}

func (t credentialsTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	outbound := request.Clone(request.Context())
	outbound.Header = request.Header.Clone()
	outbound.Header.Del("Authorization")
	outbound.Header.Del("Cookie")
	if t.bearer != "" {
		outbound.Header.Set("Authorization", "Bearer "+t.bearer)
	}
	for key, values := range t.headers {
		outbound.Header.Del(key)
		for _, value := range values {
			outbound.Header.Add(key, value)
		}
	}
	return t.base.RoundTrip(outbound)
}
