package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
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
}

// ToolSurface registers and serves the bridge's own three tools. Every handler
// reads the published generation at call time, so a reload is observed without
// re-registering the tool list.
type ToolSurface struct {
	runtime *Runtime
	cfg     ToolSurfaceConfig

	cached   *search.Searcher
	cacheKey string
}

// NewToolSurface validates the tool surface's own configuration.
func NewToolSurface(runtime *Runtime, cfg ToolSurfaceConfig) (*ToolSurface, error) {
	if runtime == nil {
		return nil, errors.New("service: tool surface needs a runtime")
	}
	if cfg.Model.Dimensions <= 0 {
		return nil, errors.New("service: tool surface needs the embedding model dimensions")
	}
	return &ToolSurface{runtime: runtime, cfg: cfg}, nil
}

// Register installs exactly the three bridge tools on server with the exact
// §4.4 input schemas. It is not safe for concurrent use with tool calls.
func (t *ToolSurface) Register(server *mcp.Server) error {
	if server == nil {
		return errors.New("service: tool surface needs a server")
	}
	server.AddTool(&mcp.Tool{
		Name:        catalog.ToolSearchTools,
		Description: "Search the bridge's indexed tools.",
		InputSchema: catalog.SearchToolsInputSchema(),
	}, t.searchTools)
	server.AddTool(&mcp.Tool{
		Name:        catalog.ToolDescribeTool,
		Description: "Describe one indexed tool with its exact stored schema.",
		InputSchema: catalog.DescribeToolInputSchema(),
	}, t.describeTool)
	server.AddTool(&mcp.Tool{
		Name:        catalog.ToolCallTool,
		Description: "Call one indexed upstream tool through the bridge.",
		InputSchema: catalog.CallToolInputSchema(),
	}, t.callTool)
	return nil
}

// searcher returns the searcher of the published generation, rebuilding it
// only when the generation's catalog, model identity or pin set changed so the
// search layer's own caches keep their value.
func (t *ToolSurface) searcher(assembly *Assembly) (*search.Searcher, error) {
	key := fmt.Sprintf("%d\x00%s\x00%s\x00%d", assembly.Generation(), assembly.CatalogRevision(), t.cfg.Model.Digest(), len(assembly.Pins()))
	if t.cached != nil && t.cacheKey == key {
		return t.cached, nil
	}
	searcher, err := search.NewSearcher(t.snapshot(assembly), t.cfg.Provider, t.cfg.Search)
	if err != nil {
		return nil, err
	}
	t.cached = searcher
	t.cacheKey = key
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
	return &mcp.CallToolResult{StructuredContent: searchPayload(response)}, nil
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
	binding, ok := assembly.catalog.Binding(wire.ToolName)
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
		"pinned":            assembly.PinnedNames()[wire.ToolName],
		"last_success_at":   timestamp(health.LastSuccessAt),
		"validation_status": "supported",
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
	return &mcp.CallToolResult{StructuredContent: payload}, nil
}

// callTool admits and forwards exactly one execution (plan §4.4/§4.5).
func (t *ToolSurface) callTool(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var wire struct {
		ToolName        string          `json:"tool_name"`
		CatalogRevision string          `json:"catalog_revision"`
		Arguments       json.RawMessage `json:"arguments"`
	}
	if err := decodeStrict(rawArguments(req), &wire); err != nil {
		return errorResult(search.CodeInvalidArguments, "call arguments are not a valid call request", core.ExecutionNotStarted), nil
	}
	assembly := t.runtime.Current()
	if assembly == nil {
		return errorResult(codeBackendUnavailable, "no catalog generation is published", core.ExecutionNotStarted), nil
	}
	// Revision admission precedes every other check, so a stale expected
	// revision is never answered with a schema verdict about the new catalog.
	if wire.CatalogRevision != assembly.CatalogRevision() {
		return errorResult(codeCatalogChanged, "the catalog revision changed", core.ExecutionNotStarted), nil
	}
	binding, ok := assembly.catalog.Binding(wire.ToolName)
	if !ok {
		return errorResult(codeUnknownTool, "no such tool in the published catalog", core.ExecutionNotStarted), nil
	}
	health, _ := assembly.Health(binding.ServerID)
	if health.Availability != AvailabilityAvailable {
		return errorResult(codeBackendUnavailable, "the server of this tool is not available", core.ExecutionNotStarted), nil
	}
	args := wire.Arguments
	if len(bytes.TrimSpace(args)) == 0 {
		args = json.RawMessage(`{}`)
	}
	if len(binding.Def.InputSchema) > 0 {
		if err := catalog.ValidateSchemaDocument(string(binding.Def.InputSchema)); err != nil {
			return errorResult(codeUnsupportedSchema, "the stored input schema cannot be applied", core.ExecutionNotStarted), nil
		}
		if err := catalog.ValidateSchema(string(binding.Def.InputSchema), string(args)); err != nil {
			// The validator's own text may quote submitted values, so the
			// rejection is answered with a fixed, payload-free message.
			return errorResult(search.CodeInvalidArguments, "arguments do not satisfy the tool's stored input schema", core.ExecutionNotStarted), nil
		}
	}

	lease, ok := assembly.Acquire(binding.ServerID)
	if !ok {
		return errorResult(codeBackendUnavailable, "the server of this tool is draining", core.ExecutionNotStarted), nil
	}
	defer lease.Release()
	envelope, dispatchErr := assembly.catalog.Dispatch(ctx, wire.ToolName, wire.CatalogRevision, args, lease.Server().Client)
	if dispatchErr != nil {
		return errorResult(dispatchErr.Code, dispatchMessage(dispatchErr.Code), dispatchErr.ExecutionState), nil
	}

	upstream := decodeUpstreamResult(envelope.UpstreamResult)
	payload := map[string]any{
		"server":           envelope.Server,
		"catalog_revision": envelope.CatalogRevision,
		"catalog_status":   health.Status,
		"availability":     health.Availability,
		"tool_name":        wire.ToolName,
		"request_id":       newRequestID(),
		"upstream_result":  rawValue(envelope.UpstreamResult),
	}
	return &mcp.CallToolResult{
		Content:           upstream.Content,
		StructuredContent: payload,
		IsError:           upstream.IsError,
	}, nil
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
		results = append(results, map[string]any{
			"tool_name":        result.ToolName,
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
