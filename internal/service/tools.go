package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/search"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

// ToolSurfaceConfig carries the non-secret search settings of the tool
// surface. A nil Provider is a supported configuration: search degrades to
// keyword mode with a finite reason (plan §4.4/§4.8).
type ToolSurfaceConfig struct {
	Search   search.Config
	Provider search.Provider
	Model    search.ModelIdentity

	// Logger receives the tool surface's own terminal observability records.
	// A nil Logger is a supported configuration: the surface serves with
	// request observability disabled rather than failing validation.
	Logger *observe.Logger
}

// ToolSurface registers and serves the bridge's own three tools. Every handler
// reads the published generation at call time, so a reload is observed without
// re-registering the tool list.
type ToolSurface struct {
	runtime *Runtime
	cfg     ToolSurfaceConfig
	logger  *observe.Logger

	cached   *search.Searcher
	cacheKey string

	mu          sync.Mutex
	server      *mcp.Server
	pinnedTools map[string]core.ResolvedPin
}

// NewToolSurface validates the tool surface's own configuration.
func NewToolSurface(runtime *Runtime, cfg ToolSurfaceConfig) (*ToolSurface, error) {
	if runtime == nil {
		return nil, errors.New("service: tool surface needs a runtime")
	}
	if cfg.Model.Dimensions <= 0 {
		return nil, errors.New("service: tool surface needs the embedding model dimensions")
	}
	return &ToolSurface{runtime: runtime, cfg: cfg, logger: cfg.Logger, pinnedTools: make(map[string]core.ResolvedPin)}, nil
}

// boolPtr returns a fresh pointer for an advisory hint literal, so callers
// never share a mutable hint value.
func boolPtr(value bool) *bool { return &value }

// Register installs exactly the three bridge tools on server with the exact
// §4.4 input schemas. It is not safe for concurrent use with tool calls.
func (t *ToolSurface) Register(server *mcp.Server) error {
	if server == nil {
		return errors.New("service: tool surface needs a server")
	}
	t.mu.Lock()
	t.server = server
	t.mu.Unlock()
	server.AddTool(&mcp.Tool{
		Name:        catalog.ToolSearchTools,
		Description: "Search the bridge's indexed tools.",
		InputSchema: catalog.SearchToolsInputSchema(),
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(true),
		},
	}, t.searchTools)
	server.AddTool(&mcp.Tool{
		Name:        catalog.ToolDescribeTool,
		Description: "Describe one indexed tool with its exact stored schema.",
		InputSchema: catalog.DescribeToolInputSchema(),
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(true),
		},
	}, t.describeTool)
	server.AddTool(&mcp.Tool{
		Name:        catalog.ToolCallTool,
		Description: "Call one indexed upstream tool through the bridge.",
		InputSchema: catalog.CallToolInputSchema(),
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: boolPtr(true),
			IdempotentHint:  false,
			OpenWorldHint:   boolPtr(true),
		},
	}, t.callTool)
	return nil
}

// Published reconciles the inbound SDK tool registry with one immutable
// publication: pinned prerequisite tools are exposed exactly as configured.
// It is safe for concurrent use.
func (t *ToolSurface) Published(assembly *Assembly) {
	if assembly == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.server == nil {
		return
	}
	pins := assembly.Pins()
	next := make(map[string]core.ResolvedPin, len(pins))
	for _, pin := range pins {
		if pin.Binding == nil {
			continue
		}
		name := config.WireToolName(pin.PinnedTool.ServerID, pin.PinnedTool.RawName)
		next[name] = pin
	}
	for name, previous := range t.pinnedTools {
		if _, exists := next[name]; !exists {
			t.server.RemoveTools(name)
			delete(t.pinnedTools, name)
		} else {
			// Idempotency: skip registration when the binding is unchanged.
			if previous.Binding != nil && reflect.DeepEqual(previous.Binding, next[name].Binding) {
				delete(next, name)
			}
		}
	}
	descriptors := make(map[string]*mcp.Tool, len(next))
	for name, pin := range next {
		descriptor, err := sdkToolDescriptor(name, pin.Binding.Def)
		if err != nil {
			// One invalid descriptor rejects the whole candidate generation:
			// the published surface must never be a partial tool set with a
			// pinned tool silently missing, so nothing from this assembly is
			// registered and the previous generation stays published.
			if t.logger != nil {
				t.logger.Log(observe.Entry{
					Level:     "warn",
					Event:     "reload",
					RequestID: "",
					Operation: "publish",
					Duration:  0,
					Outcome:   "rejected",
					Reason:    "invalid_metadata",
					Fields:    map[string]any{"reason": "invalid_metadata"},
				})
			}
			return
		}
		descriptors[name] = descriptor
	}
	for name, descriptor := range descriptors {
		t.server.AddTool(descriptor, t.callPinnedTool)
		t.pinnedTools[name] = next[name]
	}
}

// callPinnedTool dispatches one direct invocation of a preloaded pinned tool
// (plan §4.4). The wire tool name selects the pin's binding, the stored input
// schema validates the arguments, and the dispatch reuses the same upstream
// execution path as the bridge's own call tool.
func (t *ToolSurface) callPinnedTool(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Observability scope mirrors callTool: the internal request identity
	// rides the private carrier the inbound edge attached to this POST and is
	// never caller controlled; the operation identity scopes this decoded
	// tools/call execution independently.
	var requestID string
	corrState := "unavailable"
	if req != nil && req.Extra != nil && req.Extra.Header != nil {
		carrier := req.Extra.Header.Get("X-Mcp-Bridge-Internal-Request-Id")
		if len(carrier) == 32 && validHex(carrier) {
			requestID = carrier
			corrState = "valid"
		}
	}
	operationID := observe.CorrelationID("")
	start := time.Now()
	toolName := ""
	if req != nil && req.Params != nil {
		toolName = req.Params.Name
	}

	// The wire name selects the pin. The registered map is authoritative;
	// the published assembly's own pins cover a call that races a publication.
	t.mu.Lock()
	pin, found := t.pinnedTools[toolName]
	t.mu.Unlock()
	assembly := t.runtime.Current()
	if !found && assembly != nil {
		for _, candidate := range assembly.Pins() {
			if candidate.Binding == nil {
				continue
			}
			if config.WireToolName(candidate.PinnedTool.ServerID, candidate.PinnedTool.RawName) == toolName {
				pin, found = candidate, true
				break
			}
		}
	}
	if !found {
		t.logToolAdmissionFailure(ctx, requestID, operationID, corrState, start, "unknown_tool")
		return errorResult(codeUnknownTool, "unknown preloaded tool", core.ExecutionNotStarted), nil
	}
	if assembly == nil {
		t.logToolAdmissionFailure(ctx, requestID, operationID, corrState, start, "backend_unavailable")
		return errorResult(codeBackendUnavailable, "no catalog generation is published", core.ExecutionNotStarted), nil
	}
	binding := pin.Binding
	args := rawArguments(req)
	if len(bytes.TrimSpace(args)) == 0 {
		args = json.RawMessage(`{}`)
	}
	if len(binding.Def.InputSchema) > 0 {
		if err := catalog.ValidateSchemaDocument(string(binding.Def.InputSchema)); err != nil {
			t.logToolAdmissionFailure(ctx, requestID, operationID, corrState, start, "invalid_arguments")
			return errorResult(search.CodeInvalidArguments, "arguments do not satisfy the tool's stored input schema", core.ExecutionNotStarted), nil
		}
		if err := catalog.ValidateSchema(string(binding.Def.InputSchema), string(args)); err != nil {
			// The validator's own text may quote submitted values, so the
			// rejection is answered with a fixed, payload-free message.
			t.logToolAdmissionFailure(ctx, requestID, operationID, corrState, start, "invalid_arguments")
			return errorResult(search.CodeInvalidArguments, "arguments do not satisfy the tool's stored input schema", core.ExecutionNotStarted), nil
		}
	}
	lease, ok := assembly.Acquire(binding.ServerID)
	if !ok {
		t.logToolAdmissionFailure(ctx, requestID, operationID, corrState, start, "backend_busy")
		return errorResult(codeBackendUnavailable, "the server of this tool is draining", core.ExecutionNotStarted), nil
	}
	defer lease.Release()
	backendCallID := observe.CorrelationID("")
	backendStart := time.Now()
	envelope, dispatchErr := assembly.catalog.Dispatch(ctx, core.PublicName(pin.PinnedTool.ServerID, pin.PinnedTool.RawName), assembly.CatalogRevision(), args, lease.Server().Client)
	if dispatchErr != nil {
		if t.logger != nil {
			outcome := "error"
			if dispatchErr.Code == "deadline_exceeded" {
				outcome = "timeout"
			}
			t.logger.Log(observe.Entry{
				Level:     "warn",
				Event:     "backend_complete",
				RequestID: requestID,
				Server:    binding.ServerID,
				Operation: "call_tool",
				Duration:  time.Since(backendStart),
				Outcome:   outcome,
				Reason:    dispatchErr.Code,
				Fields: map[string]any{
					"request_id":      requestID,
					"operation_id":    operationID,
					"backend_call_id": backendCallID,
					"tool":            core.PublicName(binding.ServerID, binding.RawName),
					"outcome":         outcome,
					"reason":          dispatchErr.Code,
					"duration_ms":     time.Since(backendStart).Milliseconds(),
				},
			})
			t.logger.Log(observe.Entry{
				Level:     "warn",
				Event:     "mcp_complete",
				RequestID: requestID,
				Operation: "call_tool",
				Duration:  time.Since(start),
				Outcome:   "error",
				Reason:    dispatchErr.Code,
				Fields: map[string]any{
					"request_id":       requestID,
					"operation_id":     operationID,
					"mcp_method":       "tools_call",
					"mcp_message_kind": "request",
					"stage":            "backend_dispatch",
					"outcome":          "error",
					"reason":           dispatchErr.Code,
					"duration_ms":      time.Since(start).Milliseconds(),
				},
			})
		}
		t.contributeOperation(requestID, operationID, upstreamStateFromDispatchCode(dispatchErr.Code))
		return errorResult(dispatchErr.Code, dispatchMessage(dispatchErr.Code), dispatchErr.ExecutionState), nil
	}
	upstream := decodeUpstreamResult(envelope.UpstreamResult)
	// A healthy transport delivering a tool-reported failure keeps the
	// classes distinct: upstream succeeded transporting, the tool errored.
	if upstream.IsError {
		t.contributeOperation(requestID, operationID, "tool_error")
	} else {
		t.contributeOperation(requestID, operationID, "succeeded")
	}
	uCount, uText, uStructured := summarizeShape(upstream.Content, upstream.StructuredContent)
	if t.logger != nil {
		// The admitted dispatch is terminal evidence of its own boundary:
		// one record per actual upstream call, joined to the operation and
		// request scope by the fresh identities minted above. A healthy
		// transport delivering a tool-reported failure terminates as error.
		level, outcome := "info", "success"
		if upstream.IsError {
			level, outcome = "warn", "error"
		}
		reason := "none"
		if upstream.IsError {
			reason = "tool_reported_error"
		}
		t.logger.Log(observe.Entry{
			Level:     level,
			Event:     "backend_complete",
			RequestID: requestID,
			Server:    binding.ServerID,
			Operation: "call_tool",
			Duration:  time.Since(backendStart),
			Outcome:   outcome,
			Reason:    reason,
			Fields: map[string]any{
				"request_id":                   requestID,
				"operation_id":                 operationID,
				"correlation_state":            corrState,
				"backend_call_id":              backendCallID,
				"tool":                         core.PublicName(binding.ServerID, binding.RawName),
				"outcome":                      outcome,
				"reason":                       reason,
				"upstream_content_count":       uCount,
				"upstream_text_presence":       uText,
				"upstream_structured_presence": uStructured,
				"duration_ms":                  time.Since(backendStart).Milliseconds(),
			},
		})
		t.logger.Log(observe.Entry{
			Level:     level,
			Event:     "mcp_complete",
			RequestID: requestID,
			Operation: "call_tool",
			Duration:  time.Since(start),
			Outcome:   outcome,
			Reason:    reason,
			Fields: map[string]any{
				"request_id":        requestID,
				"operation_id":      operationID,
				"correlation_state": corrState,
				"mcp_method":        "tools_call",
				"mcp_message_kind":  "request",
				"outcome":           outcome,
				"reason":            reason,
				"duration_ms":       time.Since(start).Milliseconds(),
			},
		})
	}
	return &mcp.CallToolResult{
		Content:           upstream.Content,
		StructuredContent: upstream.StructuredContent,
		IsError:           upstream.IsError,
	}, nil
}

// searcher returns the searcher of the published generation, rebuilding it
// only when the generation's catalog, model identity or pin set changed so the
// search layer's own caches keep their value.
func (t *ToolSurface) searcher(assembly *Assembly) (*search.Searcher, error) {
	key := fmt.Sprintf("%d\x00%s\x00%s\x00%d", assembly.Generation(), assembly.CatalogRevision(), t.cfg.Model.Digest(), len(assembly.Pins()))
	t.mu.Lock()
	if t.cached != nil && t.cacheKey == key {
		cached := t.cached
		t.mu.Unlock()
		return cached, nil
	}
	t.mu.Unlock()
	searcher, err := search.NewSearcher(t.snapshot(assembly), t.cfg.Provider, t.cfg.Search)
	if err != nil {
		return nil, err
	}
	t.mu.Lock()
	t.cached = searcher
	t.cacheKey = key
	t.mu.Unlock()
	return searcher, nil
}

// snapshot is the immutable search aggregate of one generation: the indexed
// tools are exactly the generation's published catalog bindings, and the
// vectors are the durable embeddings whose content and model identity still
// match them (plan §4.8/§4.9).
func (t *ToolSurface) snapshot(assembly *Assembly) search.Snapshot {
	result := search.Snapshot{
		Revision: assembly.CatalogRevision(),
		Model:    t.cfg.Model,
	}
	embeddings := map[string]storageEmbedding{}
	if durable := assembly.Snapshot(); durable != nil {
		for _, row := range durable.Embeddings {
			if row.ModelIdentityDigest != t.cfg.Model.Digest() {
				continue
			}
			embeddings[row.ToolIdentity] = storageEmbedding{contentHash: row.ContentHash, vector: row.Vector}
		}
	}
	pinned := assembly.PinnedNames()
	for _, binding := range assembly.catalog.Bindings() {
		contentHash := search.ContentHash(binding.RawName, binding.Def.Description)
		document := search.Document{
			ServerID:    binding.ServerID,
			RawName:     binding.RawName,
			PublicName:  core.PublicName(binding.ServerID, binding.RawName),
			Description: binding.Def.Description,
			Pinned:      pinned[core.PublicName(binding.ServerID, binding.RawName)],
		}
		identity := storage.ToolIdentity(binding.ServerID, binding.RawName)
		if embedding, ok := embeddings[identity]; ok &&
			embedding.contentHash == contentHash &&
			len(embedding.vector) == t.cfg.Model.Dimensions {
			document.Vector = embedding.vector
		}
		result.Documents = append(result.Documents, document)
	}
	for _, serverID := range assembly.ServerIDs() {
		health, ok := assembly.Health(serverID)
		if !ok {
			continue
		}
		result.Backends = append(result.Backends, search.Backend{
			Server:        serverID,
			Status:        health.Status,
			Availability:  health.Availability,
			Reason:        health.Reason,
			Revision:      assembly.CatalogRevision(),
			LastSuccessAt: health.LastSuccessAt,
		})
	}
	return result
}

// storageEmbedding is one indexed embedding of the published snapshot.
type storageEmbedding struct {
	contentHash string
	vector      []float64
}

// searchTools answers one global search (plan §4.4).
func (t *ToolSurface) searchTools(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	request, err := search.DecodeRequest(rawArguments(req), t.cfg.Search)
	if err != nil {
		return failureResult(err), nil
	}
	assembly := t.runtime.Current()
	if assembly == nil {
		return errorResult(codeBackendUnavailable, "no catalog generation is published", core.ExecutionNotStarted), nil
	}
	searcher, err := t.searcher(assembly)
	if err != nil {
		return failureResult(err), nil
	}
	response, err := searcher.Search(ctx, request)
	if err != nil {
		return failureResult(err), nil
	}
	payload := searchPayload(response)
	raw, err := json.Marshal(payload)
	if err != nil {
		return errorResult(codeInternalError, "failed to marshal search response", core.ExecutionNotStarted), nil
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(raw)}},
		StructuredContent: payload,
	}, nil
}

// resolveBinding locates a published binding by either the wire tool name or
// its derived public name (server/hash), so hybrid wire-name callers resolve
// to the same binding as public-hash callers. The second return value is the
// public tool name the binding was found under.
func resolveBinding(catalog *core.Bridge, toolName string) (*core.Binding, string, bool) {
	if catalog == nil {
		return nil, "", false
	}
	if binding, ok := catalog.Binding(toolName); ok {
		return &binding, toolName, true
	}
	if server, tool, ok := config.ParseWireToolName(toolName); ok {
		pub := core.PublicName(server, tool)
		if binding, ok := catalog.Binding(pub); ok {
			return &binding, pub, true
		}
	}
	return nil, "", false
}

// describeTool answers one exact stored definition (plan §4.4).
func (t *ToolSurface) describeTool(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var wire struct {
		ToolName        string `json:"tool_name"`
		CatalogRevision string `json:"catalog_revision"`
	}
	if err := decodeStrict(rawArguments(req), &wire); err != nil {
		return errorResult(search.CodeInvalidArguments, "describe arguments are not a valid describe request", core.ExecutionNotStarted), nil
	}
	assembly := t.runtime.Current()
	if assembly == nil {
		return errorResult(codeBackendUnavailable, "no catalog generation is published", core.ExecutionNotStarted), nil
	}
	if wire.CatalogRevision != assembly.CatalogRevision() {
		return errorResult(codeCatalogChanged, "the catalog revision changed", core.ExecutionNotStarted), nil
	}
	binding, pub, ok := resolveBinding(assembly.catalog, wire.ToolName)
	if !ok {
		return errorResult(codeUnknownTool, "no such tool in the published catalog", core.ExecutionNotStarted), nil
	}
	health, _ := assembly.Health(binding.ServerID)
	payload := map[string]any{
		"server":            binding.ServerID,
		"catalog_revision":  assembly.CatalogRevision(),
		"catalog_status":    health.Status,
		"availability":      health.Availability,
		"tool_name":         wire.ToolName,
		"upstream_name":     binding.RawName,
		"description":       binding.Def.Description,
		"pinned":            assembly.PinnedNames()[pub],
		"last_success_at":   timestamp(health.LastSuccessAt),
		"validation_status": "supported",
	}
	if metadata := upstreamMetadataValue(binding.Def.Metadata); metadata != nil {
		payload["upstream_metadata"] = metadata
	}
	if assembly.PinnedNames()[pub] {
		// Pinned tools answer with the generation's effective view: the
		// server defaults applied over the source, plus the provenance of
		// every winning field path. An empty list means no correction.
		effective, overrides, ok := assembly.PinMetadataView(binding.ServerID, binding.RawName)
		if ok {
			payload["metadata_override_fields"] = overrides
			if effectiveView := upstreamMetadataValue(effective); effectiveView != nil {
				payload["metadata"] = effectiveView
			}
		}
	}
	if schema := binding.Def.InputSchema; len(schema) > 0 {
		if err := catalog.ValidateSchemaDocument(string(schema)); err != nil {
			payload["validation_status"] = "unsupported"
			payload["validation_reason"] = "the stored input schema is not resolvable"
		}
		payload["input_schema"] = rawValue(schema)
	}
	if schema := binding.Def.OutputSchema; len(schema) > 0 {
		payload["output_schema"] = rawValue(schema)
	}
	if t.logger != nil {
		t.logger.Log(observe.Entry{
			Level:     "info",
			Event:     "mcp_complete",
			RequestID: "",
			Operation: "describe_tool",
			Duration:  0,
			Outcome:   "success",
			Reason:    "none",
			Fields: map[string]any{
				"reason": "none",
			},
		})
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return errorResult(codeInternalError, "failed to marshal describe response", core.ExecutionNotStarted), nil
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(raw)}},
		StructuredContent: payload,
	}, nil
}

// logToolAdmissionFailure terminates the tools/call operation at the
// tool_admission stage: exactly one mcp_complete record per rejected call,
// emitted before any upstream dispatch is attempted.
func (t *ToolSurface) logToolAdmissionFailure(ctx context.Context, requestID, operationID, corrState string, start time.Time, reason string) {
	// The operation ran its admission boundary without an upstream attempt;
	// the terminal record links this decoded operation with the truthful
	// no-upstream state regardless of logging availability.
	t.contributeOperation(requestID, operationID, "not_attempted")
	if t.logger == nil {
		return
	}
	duration := time.Since(start)
	t.logger.Log(observe.Entry{
		Level:     "warn",
		Event:     "mcp_complete",
		RequestID: requestID,
		Operation: "call_tool",
		Duration:  duration,
		Outcome:   "error",
		Reason:    reason,
		Fields: map[string]any{
			"request_id":        requestID,
			"operation_id":      operationID,
			"mcp_method":        "tools_call",
			"stage":             "tool_admission",
			"outcome":           "error",
			"reason":            reason,
			"correlation_state": corrState,
			"duration_ms":       duration.Milliseconds(),
		},
	})
}

// validHex reports whether the provided string is a valid lowercase hexadecimal identifier.
func validHex(s string) bool {
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

// callTool admits and forwards exactly one execution (plan §4.4/§4.5).
func (t *ToolSurface) callTool(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Observability scope: the internal request identity rides the private
	// carrier the inbound edge attached to this POST and is never caller
	// controlled; the operation identity scopes this decoded tools/call
	// execution independently. A missing or invalid carrier leaves the
	// request identity empty and reports correlation as unavailable.
	var requestID string
	corrState := "unavailable"
	if req != nil && req.Extra != nil && req.Extra.Header != nil {
		carrier := req.Extra.Header.Get("X-Mcp-Bridge-Internal-Request-Id")
		if len(carrier) == 32 && validHex(carrier) {
			requestID = carrier
			corrState = "valid"
		}
	}
	operationID := observe.CorrelationID("")
	start := time.Now()
	var wire struct {
		ToolName        string          `json:"tool_name"`
		CatalogRevision string          `json:"catalog_revision"`
		Arguments       json.RawMessage `json:"arguments"`
	}
	if err := decodeStrict(rawArguments(req), &wire); err != nil {
		t.logToolAdmissionFailure(ctx, requestID, operationID, corrState, start, "invalid_arguments")
		return errorResult(search.CodeInvalidArguments, "call arguments are not a valid call request", core.ExecutionNotStarted), nil
	}
	assembly := t.runtime.Current()
	if assembly == nil {
		t.logToolAdmissionFailure(ctx, requestID, operationID, corrState, start, "backend_unavailable")
		return errorResult(codeBackendUnavailable, "no catalog generation is published", core.ExecutionNotStarted), nil
	}
	// Revision admission precedes every other check, so a stale expected
	// revision is never answered with a schema verdict about the new catalog.
	if wire.CatalogRevision != assembly.CatalogRevision() {
		t.logToolAdmissionFailure(ctx, requestID, operationID, corrState, start, "catalog_changed")
		return errorResult(codeCatalogChanged, "the catalog revision changed", core.ExecutionNotStarted), nil
	}
	binding, pub, ok := resolveBinding(assembly.catalog, wire.ToolName)
	if !ok {
		t.logToolAdmissionFailure(ctx, requestID, operationID, corrState, start, "unknown_tool")
		return errorResult(codeUnknownTool, "no such tool in the published catalog", core.ExecutionNotStarted), nil
	}
	health, _ := assembly.Health(binding.ServerID)
	if health.Availability != AvailabilityAvailable {
		t.logToolAdmissionFailure(ctx, requestID, operationID, corrState, start, "backend_unavailable")
		return errorResult(codeBackendUnavailable, "the server of this tool is not available", core.ExecutionNotStarted), nil
	}
	args := wire.Arguments
	if len(bytes.TrimSpace(args)) == 0 {
		args = json.RawMessage(`{}`)
	}
	if len(binding.Def.InputSchema) > 0 {
		if err := catalog.ValidateSchemaDocument(string(binding.Def.InputSchema)); err != nil {
			t.logToolAdmissionFailure(ctx, requestID, operationID, corrState, start, "unsupported_schema")
			return errorResult(codeUnsupportedSchema, "the stored input schema cannot be applied", core.ExecutionNotStarted), nil
		}
		if err := catalog.ValidateSchema(string(binding.Def.InputSchema), string(args)); err != nil {
			// The validator's own text may quote submitted values, so the
			// rejection is answered with a fixed, payload-free message.
			t.logToolAdmissionFailure(ctx, requestID, operationID, corrState, start, "invalid_arguments")
			return errorResult(search.CodeInvalidArguments, "arguments do not satisfy the tool's stored input schema", core.ExecutionNotStarted), nil
		}
	}

	lease, ok := assembly.Acquire(binding.ServerID)
	if !ok {
		t.logToolAdmissionFailure(ctx, requestID, operationID, corrState, start, "backend_busy")
		return errorResult(codeBackendUnavailable, "the server of this tool is draining", core.ExecutionNotStarted), nil
	}
	defer lease.Release()
	backendCallID := observe.CorrelationID("")
	backendStart := time.Now()
	envelope, dispatchErr := assembly.catalog.Dispatch(ctx, pub, wire.CatalogRevision, args, lease.Server().Client)
	if dispatchErr != nil {
		if t.logger != nil {
			outcome := "error"
			if dispatchErr.Code == "deadline_exceeded" {
				outcome = "timeout"
			}
			t.logger.Log(observe.Entry{
				Level:     "warn",
				Event:     "backend_complete",
				RequestID: requestID,
				Server:    binding.ServerID,
				Operation: "call_tool",
				Duration:  time.Since(backendStart),
				Outcome:   outcome,
				Reason:    dispatchErr.Code,
				Fields: map[string]any{
					"request_id":      requestID,
					"operation_id":    operationID,
					"backend_call_id": backendCallID,
					"tool":            core.PublicName(binding.ServerID, binding.RawName),
					"outcome":         outcome,
					"reason":          dispatchErr.Code,
					"duration_ms":     time.Since(backendStart).Milliseconds(),
				},
			})
			t.logger.Log(observe.Entry{
				Level:     "warn",
				Event:     "mcp_complete",
				RequestID: requestID,
				Operation: "call_tool",
				Duration:  time.Since(start),
				Outcome:   "error",
				Reason:    dispatchErr.Code,
				Fields: map[string]any{
					"request_id":       requestID,
					"operation_id":     operationID,
					"mcp_method":       "tools_call",
					"mcp_message_kind": "request",
					"stage":            "backend_dispatch",
					"outcome":          "error",
					"reason":           dispatchErr.Code,
					"duration_ms":      time.Since(start).Milliseconds(),
				},
			})
		}
		t.contributeOperation(requestID, operationID, upstreamStateFromDispatchCode(dispatchErr.Code))
		return errorResult(dispatchErr.Code, dispatchMessage(dispatchErr.Code), dispatchErr.ExecutionState), nil
	}
	upstream := decodeUpstreamResult(envelope.UpstreamResult)
	// A healthy transport delivering a tool-reported failure keeps the
	// classes distinct: upstream succeeded transporting, the tool errored.
	if upstream.IsError {
		t.contributeOperation(requestID, operationID, "tool_error")
	} else {
		t.contributeOperation(requestID, operationID, "succeeded")
	}
	uCount, uText, uStructured := summarizeShape(upstream.Content, upstream.StructuredContent)
	if t.logger != nil {
		// The admitted dispatch is terminal evidence of its own boundary:
		// one record per actual upstream call, joined to the operation and
		// request scope by the fresh identities minted above. A healthy
		// transport delivering a tool-reported failure terminates as error.
		level, outcome := "info", "success"
		if upstream.IsError {
			level, outcome = "warn", "error"
		}
		fields := map[string]any{
			"request_id":                   requestID,
			"operation_id":                 operationID,
			"correlation_state":            corrState,
			"backend_call_id":              backendCallID,
			"tool":                         core.PublicName(binding.ServerID, binding.RawName),
			"outcome":                      outcome,
			"upstream_content_count":       uCount,
			"upstream_text_presence":       uText,
			"upstream_structured_presence": uStructured,
			"duration_ms":                  time.Since(backendStart).Milliseconds(),
		}
		reason := "none"
		if upstream.IsError {
			reason = "tool_reported_error"
		}
		fields["reason"] = reason
		t.logger.Log(observe.Entry{
			Level:     level,
			Event:     "backend_complete",
			RequestID: requestID,
			Server:    binding.ServerID,
			Operation: "call_tool",
			Duration:  time.Since(backendStart),
			Outcome:   outcome,
			Reason:    reason,
			Fields:    fields,
		})
	}

	payload := map[string]any{
		"server":           envelope.Server,
		"catalog_revision": envelope.CatalogRevision,
		"catalog_status":   health.Status,
		"availability":     health.Availability,
		"tool_name":        wire.ToolName,
		"request_id":       newRequestID(),
		"upstream_result":  rawValue(envelope.UpstreamResult),
	}
	fCount, fText, fStructured := summarizeShape(upstream.Content, payload)
	if t.logger != nil {
		// The decoded MCP operation is terminal evidence of its own boundary:
		// one record per tools/call the surface actually executed. A
		// tool-reported failure terminates at the backend_response stage.
		level, outcome := "info", "success"
		if upstream.IsError {
			level, outcome = "warn", "error"
		}
		fields := map[string]any{
			"request_id":                requestID,
			"operation_id":              operationID,
			"correlation_state":         corrState,
			"mcp_method":                "tools_call",
			"mcp_message_kind":          "request",
			"outcome":                   outcome,
			"final_content_count":       fCount,
			"final_text_presence":       fText,
			"final_structured_presence": fStructured,
			"duration_ms":               time.Since(start).Milliseconds(),
		}
		reason := "none"
		if upstream.IsError {
			reason = "tool_reported_error"
			fields["stage"] = "backend_response"
		}
		fields["reason"] = reason
		t.logger.Log(observe.Entry{
			Level:     level,
			Event:     "mcp_complete",
			RequestID: requestID,
			Operation: "call_tool",
			Duration:  time.Since(start),
			Outcome:   outcome,
			Reason:    reason,
			Fields:    fields,
		})
	}
	return &mcp.CallToolResult{
		Content:           upstream.Content,
		StructuredContent: payload,
		IsError:           upstream.IsError,
	}, nil
}

// summarizeShape inspects a tool result's content blocks and structured
// payload, reporting the block count and whether text or structured data
// is present. It feeds terminal logging without leaking payload contents.
func summarizeShape(content []mcp.Content, structured any) (int, string, string) {
	contentCount := len(content)
	textPresence := "absent"
	for _, c := range content {
		if _, ok := c.(*mcp.TextContent); ok {
			textPresence = "present"
			break
		}
	}
	structuredPresence := "absent"
	if structured != nil {
		structuredPresence = "present"
	}
	return contentCount, textPresence, structuredPresence
}

// decodeUpstreamResult reads the upstream's complete CallToolResult so the
// outer result preserves its content blocks and error flag unchanged.
func decodeUpstreamResult(raw json.RawMessage) mcp.CallToolResult {
	if len(raw) == 0 {
		return mcp.CallToolResult{}
	}
	var result mcp.CallToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return mcp.CallToolResult{}
	}
	return result
}

// searchPayload renders one search response with the §4.4 field names.
func searchPayload(response search.Response) map[string]any {
	results := make([]map[string]any, 0, len(response.Results))
	for _, result := range response.Results {
		toolName := result.ToolName
		if result.Pinned {
			toolName = config.WireToolName(result.Server, result.UpstreamName)
		}
		results = append(results, map[string]any{
			"tool_name":        toolName,
			"server":           result.Server,
			"upstream_name":    result.UpstreamName,
			"description":      result.Description,
			"score":            result.Score,
			"catalog_revision": result.CatalogRevision,
			"catalog_status":   result.CatalogStatus,
			"availability":     result.Availability,
			"last_success_at":  timestamp(result.LastSuccessAt),
			"pinned":           result.Pinned,
		})
	}
	backends := make([]map[string]any, 0, len(response.Backends))
	for _, backend := range response.Backends {
		backends = append(backends, map[string]any{
			"server":          backend.Server,
			"status":          backend.Status,
			"availability":    backend.Availability,
			"reason":          backend.Reason,
			"revision":        backend.Revision,
			"last_success_at": timestamp(backend.LastSuccessAt),
		})
	}
	reasons := make([]string, 0, len(response.DegradedReasons))
	reasons = append(reasons, response.DegradedReasons...)
	return map[string]any{
		"catalog_revision": response.CatalogRevision,
		"mode":             response.Mode,
		"degraded_reasons": reasons,
		"results":          results,
		"backends":         backends,
	}
}

// Bridge tool-layer error codes beyond the core dispatch set.
const (
	codeCatalogChanged     = "catalog_changed"
	codeUnknownTool        = "unknown_tool"
	codeBackendUnavailable = "backend_unavailable"
	codeUnsupportedSchema  = "unsupported_schema"
	codeInternalError      = "internal_error"
)

// contributeOperation links the terminal HTTP completion record to this
// decoded operation through the existing server-owned operation identity and
// the actual upstream outcome class. The contribution is a no-op without an
// in-flight owner and never trusts caller-supplied identities.
func (t *ToolSurface) contributeOperation(requestID, operationID, upstream string) {
	observe.ContributeOperationEvidenceByID(requestID, observe.HTTPOperationEvidence{
		Relation: "single", IDState: "available", ID: operationID, Upstream: upstream,
	})
}

// upstreamStateFromDispatchCode maps an actual existing typed dispatch
// failure code onto the finite upstream evidence classes. Unmapped codes stay
// explicitly unavailable rather than being rewritten into a supported
// certainty.
func upstreamStateFromDispatchCode(code string) string {
	switch code {
	case "backend_auth_failed":
		return "auth_failed"
	case "backend_unavailable", "backend_busy", "deadline_exceeded",
		"upstream_protocol_error":
		return "transport_failed"
	default:
		return "unavailable"
	}
}

// failureResult maps a search-layer failure onto the tool-layer error shape.
func failureResult(err error) *mcp.CallToolResult {
	var failure *search.Failure
	if errors.As(err, &failure) {
		return errorResult(failure.Code, failure.Message, core.ExecutionNotStarted)
	}
	return errorResult(codeInternalError, "the bridge could not answer the request", core.ExecutionNotStarted)
}

// errorResult is the structured failure every bridge tool answers with.
func errorResult(code, message, executionState string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: message}},
		StructuredContent: map[string]any{
			"code":            code,
			"message":         message,
			"retryable":       false,
			"execution_state": executionState,
		},
	}
}

// dispatchMessage is the fixed, payload-free text for one dispatch code.
func dispatchMessage(code string) string {
	switch code {
	case codeCatalogChanged:
		return "the catalog revision changed"
	case codeUnknownTool:
		return "no such tool in the published catalog"
	case "unsupported_capability":
		return "the upstream requested an unsupported interaction"
	default:
		return "the upstream call did not complete"
	}
}

// rawArguments is the client's exact argument bytes, never re-encoded.
func rawArguments(req *mcp.CallToolRequest) json.RawMessage {
	if req == nil || req.Params == nil {
		return nil
	}
	return req.Params.Arguments
}

// decodeStrict rejects unknown fields and non-object arguments.
func decodeStrict(raw json.RawMessage, target any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return errors.New("service: empty arguments")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

// rawValue decodes stored JSON for embedding in a structured result. Invalid
// input decodes to nil rather than being rewritten.
func rawValue(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil
	}
	return value
}

// timestamp renders one success time; a never-succeeded server reports null.
func timestamp(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC().Format(time.RFC3339Nano)
}

// newRequestID is the opaque per-execution correlation id of the envelope.
func newRequestID() string {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(id[:])
}
