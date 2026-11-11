package upstream

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type DedicatedOptions struct {
	Server        ServerOptions
	Stateless     bool
	Logger        *observe.Logger
	RetryInterval time.Duration
}

// Dedicated owns one upstream client and session for one configured server.
type Dedicated struct {
	client        *ServerClient
	stateless     bool
	logger        *observe.Logger
	retryInterval time.Duration

	requiresHandshake bool

	mu      sync.Mutex
	session *mcp.ClientSession
	closed  bool
}

type discoveryFailure struct {
	cause       error
	reasonClass string
}

func (e *discoveryFailure) Error() string         { return "upstream discovery failed" }
func (e *discoveryFailure) Unwrap() error         { return e.cause }
func (e *discoveryFailure) RefreshReason() string { return e.reasonClass }

// NewDedicated validates the server identity and builds the dedicated runtime.
func NewDedicated(opts DedicatedOptions) (*Dedicated, error) {
	if opts.Logger == nil {
		return nil, errors.New("upstream: logger is required")
	}
	client, err := NewServerClient(opts.Server)
	if err != nil {
		return nil, err
	}
	client.trackDiscoveryStatus()
	return &Dedicated{
		client: client, stateless: opts.Stateless, logger: opts.Logger, retryInterval: opts.RetryInterval,
	}, nil
}

// Discover returns the complete, deterministically ordered tool catalog of the
// server. Every page must succeed: a partial or malformed discovery is an error
// so the caller keeps its last-known-good catalog instead of a truncated one.
func (d *Dedicated) Discover(ctx context.Context) ([]core.ToolDefinition, error) {
	status := &discoveryHTTPStatus{}
	ctx = context.WithValue(ctx, discoveryHTTPStatusKey{}, status)
	var session *mcp.ClientSession
	var err error
	if d.stateless {
		d.mu.Lock()
		if d.closed {
			d.mu.Unlock()
			return nil, d.reportDiscoveryFailure(ctx, status, "initialize", time.Now(), errUnavailable)
		}
		d.mu.Unlock()
		status.reset()
		started := time.Now()
		session, err = d.client.Connect(ctx, nil)
		if err != nil {
			return nil, d.reportDiscoveryFailure(ctx, status, "initialize", started, err)
		}
		defer session.Close()
	} else {
		status.reset()
		started := time.Now()
		session, err = d.sessionFor(ctx)
		if err != nil {
			return nil, d.reportDiscoveryFailure(ctx, status, "initialize", started, err)
		}
	}
	if res := session.InitializeResult(); res != nil && res.ProtocolVersion < "2026-07-28" {
		d.mu.Lock()
		d.requiresHandshake = true
		d.mu.Unlock()
	}
	var defs []core.ToolDefinition
	cursor := ""
	for {
		status.reset()
		started := time.Now()
		page, err := session.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			return nil, d.reportDiscoveryFailure(ctx, status, "tools_list", started, err)
		}
		if page == nil {
			return nil, d.reportDiscoveryFailure(ctx, status, "tools_list", started, errors.New("empty tools page"))
		}
		for _, tool := range page.Tools {
			if tool == nil || tool.Name == "" {
				return nil, d.reportDiscoveryFailure(ctx, status, "tools_list", started, errors.New("invalid tool definition"))
			}
			inputSchema, err := canonicalSchema(tool.InputSchema)
			if err != nil {
				return nil, d.reportDiscoveryFailure(ctx, status, "tools_list", started, err)
			}
			outputSchema, err := canonicalSchema(tool.OutputSchema)
			if err != nil {
				return nil, d.reportDiscoveryFailure(ctx, status, "tools_list", started, err)
			}
			metadata, err := metadataFromSDKTool(tool)
			if err != nil {
				return nil, d.reportDiscoveryFailure(ctx, status, "tools_list", started, err)
			}
			defs = append(defs, core.ToolDefinition{
				RawName:      tool.Name,
				Description:  tool.Description,
				InputSchema:  inputSchema,
				OutputSchema: outputSchema,
				Metadata:     metadata,
			})
		}
		if page.NextCursor == "" {
			break
		}
		if page.NextCursor == cursor {
			return nil, d.reportDiscoveryFailure(ctx, status, "tools_list", started, errors.New("tools cursor did not advance"))
		}
		cursor = page.NextCursor
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].RawName < defs[j].RawName })
	return defs, nil
}

func (d *Dedicated) reportDiscoveryFailure(ctx context.Context, status *discoveryHTTPStatus, phase string, started time.Time, cause error) error {
	httpStatus := status.get()
	stage, reason, reasonClass := classifyDiscoveryFailure(ctx, phase, started, cause, httpStatus)
	fields := map[string]any{
		"endpoint":       observe.SanitizeEndpoint(d.client.Endpoint),
		"failure_stage":  stage,
		"failure_reason": reason,
	}
	if httpStatus > 0 {
		fields["http_status"] = httpStatus
	}
	errorInfo := observe.RuntimeErrorInfo(reasonClass)
	if errorInfo.Retryable && d.retryInterval > 0 {
		fields["next_attempt_ms"] = d.retryInterval.Milliseconds()
	}
	d.logger.Log(observe.Entry{
		Level: "warn", Event: "upstream_discovery_failed", Server: d.client.ID,
		Operation: "refresh", Outcome: "failure", Error: errorInfo, Fields: fields,
	})
	return &discoveryFailure{cause: cause, reasonClass: reasonClass}
}

func classifyDiscoveryFailure(ctx context.Context, phase string, started time.Time, err error, status int) (string, string, string) {
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return "auth", "authentication_rejected_during_" + phase, service.RefreshReasonClientMissing
	}
	if errors.Is(err, errUnavailable) {
		return "initialize", "initialize_failed", service.RefreshReasonClientMissing
	}
	stage, reason := discoveryNetworkFailure(err)
	if phase != "initialize" || stage == "" {
		stage = phase
	}
	if timeoutClass, ok := discoveryTimeoutClass(err); ok {
		timeoutMillis := time.Since(started).Milliseconds()
		if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
			if budget := deadline.Sub(started).Milliseconds(); budget > 0 {
				timeoutMillis = budget
			}
		}
		if timeoutMillis < 1 {
			timeoutMillis = 1
		}
		if timeoutMillis > 86_400_000 {
			timeoutMillis = 86_400_000
		}
		return stage, fmt.Sprintf("timeout_%s_%dms", timeoutClass, timeoutMillis), service.RefreshReasonUnavailable
	}
	if status >= 500 {
		return stage, "upstream_http_error", service.RefreshReasonUnavailable
	}
	if reason != "" {
		return stage, reason, service.RefreshReasonUnavailable
	}
	if phase == "initialize" {
		return "initialize", "initialize_failed", service.RefreshReasonDiscoveryFailed
	}
	return "tools_list", "tools_list_failed", service.RefreshReasonDiscoveryFailed
}

func discoveryNetworkFailure(err error) (string, string) {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "dns", "dns_lookup_failed"
	}
	var certificateErr *tls.CertificateVerificationError
	if errors.As(err, &certificateErr) {
		return "tls", "tls_handshake_failed"
	}
	var recordErr tls.RecordHeaderError
	if errors.As(err, &recordErr) {
		return "tls", "tls_handshake_failed"
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		switch strings.ToLower(opErr.Op) {
		case "dial", "connect":
			return "connect", "connect_failed"
		case "tls":
			return "tls", "tls_handshake_failed"
		}
	}
	return "", ""
}

func discoveryTimeoutClass(err error) (string, bool) {
	if errors.Is(err, context.DeadlineExceeded) {
		return "context_deadline_exceeded", true
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) && networkErr.Timeout() {
		return "network_timeout", true
	}
	return "", false
}

func validProtocolVersion(version string) bool {
	if version == "2026-07-28" {
		return true
	}
	for _, advertised := range service.AdvertisedProtocolVersions() {
		if version == advertised {
			return true
		}
	}
	return false
}

func (d *Dedicated) logEphemeralHandshake(start time.Time, protocolVersion string) {
	fields := map[string]any{"backend_id": d.client.ID}
	if validProtocolVersion(protocolVersion) {
		fields["protocol_version"] = protocolVersion
	}
	d.logger.Log(observe.Entry{
		Level:     "info",
		Event:     "upstream_ephemeral_handshake",
		Operation: "handshake",
		Duration:  time.Since(start),
		Outcome:   "success",
		Fields:    fields,
	})
}

// RequiresHandshake reports whether discovery identified an upstream that
// requires an initialization handshake before tool calls.
func (d *Dedicated) RequiresHandshake() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.requiresHandshake
}

// ID is the configured server identity this runtime is dedicated to.
func (d *Dedicated) ID() string { return d.client.ID }

// Endpoint is the normalized endpoint this runtime is bound to.
func (d *Dedicated) Endpoint() string { return d.client.Endpoint }

// Client exposes the underlying per-server client so probes and breaker state
// stay owned by this runtime alone.
func (d *Dedicated) Client() *ServerClient { return d.client }

// session opens the session on first use and reuses it for the runtime's whole
// lifecycle. A closed runtime never reconnects.
func (d *Dedicated) sessionFor(ctx context.Context) (*mcp.ClientSession, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil, errUnavailable
	}
	if d.session != nil {
		return d.session, nil
	}
	session, err := d.client.Connect(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("upstream: connect %s: %w", d.client.ID, err)
	}
	d.session = session
	return session, nil
}

// DiscoverPrompts returns the complete prompt list only when the initialized
// upstream advertises prompt support.
func (d *Dedicated) DiscoverPrompts(ctx context.Context) ([]*mcp.Prompt, error) {
	session, err := d.sessionFor(ctx)
	if err != nil {
		return nil, err
	}
	initialized := session.InitializeResult()
	if initialized == nil || initialized.Capabilities == nil || initialized.Capabilities.Prompts == nil {
		return nil, nil
	}
	return discoverPromptPages(ctx, d.client.ID, func(ctx context.Context, cursor string) (*mcp.ListPromptsResult, error) {
		return session.ListPrompts(ctx, &mcp.ListPromptsParams{Cursor: cursor})
	})
}

func discoverPromptPages(ctx context.Context, serverID string, list func(context.Context, string) (*mcp.ListPromptsResult, error)) ([]*mcp.Prompt, error) {
	var prompts []*mcp.Prompt
	cursor := ""
	for {
		page, err := list(ctx, cursor)
		if err != nil {
			return nil, fmt.Errorf("upstream: discover prompts %s: %w", serverID, err)
		}
		if page == nil {
			return nil, fmt.Errorf("upstream: discover prompts %s: empty page", serverID)
		}
		for _, prompt := range page.Prompts {
			if prompt == nil || prompt.Name == "" {
				return nil, fmt.Errorf("upstream: discover prompts %s: prompt without a name", serverID)
			}
			prompts = append(prompts, prompt)
		}
		if page.NextCursor == "" {
			break
		}
		if page.NextCursor == cursor {
			return nil, fmt.Errorf("upstream: discover prompts %s: pagination cursor did not advance", serverID)
		}
		cursor = page.NextCursor
	}
	sort.Slice(prompts, func(i, j int) bool { return prompts[i].Name < prompts[j].Name })
	return prompts, nil
}

// CallTool implements core.Upstream. pub is carried for provenance only; the
// exact raw args bytes reach upstream unchanged. It performs at most one
// upstream execution and reports failures as safe, payload-free outcomes.
func (d *Dedicated) CallTool(ctx context.Context, pub, serverID, rawName string, args json.RawMessage) (json.RawMessage, *core.UpstreamOutcome) {
	_ = pub
	if serverID != "" && serverID != d.client.ID {
		return nil, &core.UpstreamOutcome{Failure: &core.UpstreamFailure{Code: "unknown_tool", ExecutionState: core.ExecutionNotStarted}}
	}
	var session *mcp.ClientSession
	var err error
	if d.stateless {
		d.mu.Lock()
		if d.closed {
			d.mu.Unlock()
			return nil, &core.UpstreamOutcome{Failure: &core.UpstreamFailure{Code: "backend_unavailable", ExecutionState: core.ExecutionNotStarted}}
		}
		d.mu.Unlock()
		start := time.Now()
		session, err = d.client.Connect(ctx, nil)
		if err != nil {
			return nil, &core.UpstreamOutcome{Failure: &core.UpstreamFailure{Code: "backend_unavailable", ExecutionState: core.ExecutionNotStarted}}
		}
		defer session.Close()

		d.mu.Lock()
		reqHandshake := d.requiresHandshake
		d.mu.Unlock()

		if reqHandshake {
			protocolVersion := ""
			if result := session.InitializeResult(); result != nil {
				protocolVersion = result.ProtocolVersion
			}
			d.logEphemeralHandshake(start, protocolVersion)
		}
	} else {
		session, err = d.sessionFor(ctx)
		if err != nil {
			return nil, &core.UpstreamOutcome{Failure: &core.UpstreamFailure{Code: "backend_unavailable", ExecutionState: core.ExecutionNotStarted}}
		}
	}
	arguments, err := decodeArguments(args)
	if err != nil {
		return nil, &core.UpstreamOutcome{Failure: &core.UpstreamFailure{Code: "invalid_arguments", ExecutionState: core.ExecutionNotStarted}}
	}
	result, err := d.client.CallTool(ctx, session, &mcp.CallToolParams{Name: rawName, Arguments: arguments})
	if err != nil {
		return nil, &core.UpstreamOutcome{Failure: failureOutcome(err)}
	}
	if result == nil {
		return nil, &core.UpstreamOutcome{Failure: &core.UpstreamFailure{Code: "upstream_protocol_error", ExecutionState: core.ExecutionUnknown}}
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, &core.UpstreamOutcome{Failure: &core.UpstreamFailure{Code: "upstream_protocol_error", ExecutionState: core.ExecutionUnknown}}
	}
	return encoded, nil
}

// Close releases the session and permanently closes the runtime. It is safe to
// call repeatedly.
func (d *Dedicated) Close() error {
	d.mu.Lock()
	session := d.session
	d.session = nil
	d.closed = true
	d.mu.Unlock()
	if session == nil {
		return nil
	}
	return session.Close()
}

// failureOutcome maps an execution error to the §4.4 safe code set. Codes are
// never retryable; the execution state is not_started only before admission.
func failureOutcome(err error) *core.UpstreamFailure {
	var callError *CallError
	if errors.As(err, &callError) {
		state := callError.ExecutionState
		if state != core.ExecutionNotStarted {
			state = core.ExecutionUnknown
		}
		return &core.UpstreamFailure{Code: callError.Code, ExecutionState: state}
	}
	if IsBusy(err) {
		return &core.UpstreamFailure{Code: "backend_busy", ExecutionState: core.ExecutionNotStarted}
	}
	if IsUnavailable(err) {
		return &core.UpstreamFailure{Code: "backend_unavailable", ExecutionState: core.ExecutionNotStarted}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &core.UpstreamFailure{Code: "deadline_exceeded", ExecutionState: core.ExecutionUnknown}
	}
	if errors.Is(err, context.Canceled) {
		return &core.UpstreamFailure{Code: "internal_error", ExecutionState: core.ExecutionUnknown}
	}
	return &core.UpstreamFailure{Code: "backend_unavailable", ExecutionState: core.ExecutionNotStarted}
}

// decodeArguments re-materialises the exact client args for the SDK call while
// preserving number literals (json.Number) and object key order stability.
func decodeArguments(args json.RawMessage) (any, error) {
	trimmed := bytes.TrimSpace(args)
	if len(trimmed) == 0 {
		return nil, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

// canonicalSchema renders a wire schema as compact JSON so identical schemas
// always produce identical catalog bytes.
func canonicalSchema(schema any) (json.RawMessage, error) {
	switch typed := schema.(type) {
	case nil:
		return nil, nil
	case json.RawMessage:
		return compactJSON(typed)
	case []byte:
		return compactJSON(typed)
	default:
		encoded, err := json.Marshal(schema)
		if err != nil {
			return nil, fmt.Errorf("schema is not JSON: %w", err)
		}
		return compactJSON(encoded)
	}
}

func compactJSON(encoded []byte) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(encoded)
	if len(trimmed) == 0 {
		return nil, nil
	}
	var buffer bytes.Buffer
	if err := json.Compact(&buffer, trimmed); err != nil {
		return nil, fmt.Errorf("schema is not valid JSON: %w", err)
	}
	return json.RawMessage(buffer.Bytes()), nil
}
