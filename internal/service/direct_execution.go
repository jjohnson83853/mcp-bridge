package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/search"
)

type boundDirectExecution struct {
	surface      *ToolSurface
	assembly     *Assembly
	target       logicalToolTarget
	binding      *core.Binding
	inputSchema  json.RawMessage
	lease        *Lease
	requestID    string
	operationID  string
	correlation  string
	requestStart time.Time
}

func (e boundDirectExecution) call(ctx context.Context, args json.RawMessage) (*mcp.CallToolResult, error) {
	defer e.lease.Release()

	if len(e.inputSchema) > 0 {
		if err := catalog.ValidateSchemaDocument(string(e.inputSchema)); err != nil {
			e.surface.logToolAdmissionFailure(ctx, e.requestID, e.operationID, e.correlation, e.requestStart, "invalid_arguments")
			return errorResult(search.CodeInvalidArguments, "arguments do not satisfy the tool's stored input schema", core.ExecutionNotStarted), nil
		}
		if err := catalog.ValidateSchema(string(e.inputSchema), string(args)); err != nil {
			// The validator's own text may quote submitted values, so use a fixed message.
			e.surface.logToolAdmissionFailure(ctx, e.requestID, e.operationID, e.correlation, e.requestStart, "invalid_arguments")
			return errorResult(search.CodeInvalidArguments, "arguments do not satisfy the tool's stored input schema", core.ExecutionNotStarted), nil
		}
	}

	backendCallID := observe.CorrelationID("")
	backendStart := time.Now()
	envelope, dispatchErr := e.assembly.catalog.Dispatch(ctx, core.PublicName(e.binding.ServerID, e.binding.RawName), e.assembly.CatalogRevision(), args, e.lease.Server().Client)
	if dispatchErr != nil {
		e.logDispatchFailure(backendCallID, backendStart, dispatchErr)
		return errorResult(dispatchErr.Code, dispatchMessage(dispatchErr.Code), dispatchErr.ExecutionState), nil
	}

	upstream := decodeUpstreamResult(envelope.UpstreamResult)
	if upstream.IsError {
		e.surface.contributeOperation(e.requestID, e.operationID, "tool_error")
	} else {
		e.surface.contributeOperation(e.requestID, e.operationID, "succeeded")
	}
	e.logDispatchSuccess(backendCallID, backendStart, upstream)
	return &mcp.CallToolResult{
		Content:           upstream.Content,
		StructuredContent: upstream.StructuredContent,
		IsError:           upstream.IsError,
	}, nil
}

func (e boundDirectExecution) logDispatchFailure(backendCallID string, backendStart time.Time, dispatchErr *core.DispatchError) {
	if e.surface.logger != nil {
		outcome := "error"
		if dispatchErr.Code == "deadline_exceeded" {
			outcome = "timeout"
		}
		e.surface.logger.Log(observe.Entry{
			Level:     "warn",
			Event:     "backend_complete",
			RequestID: e.requestID,
			Server:    e.target.serverID,
			Operation: "call_tool",
			Duration:  time.Since(backendStart),
			Outcome:   outcome,
			Reason:    dispatchErr.Code,
			Fields: map[string]any{
				"request_id":      e.requestID,
				"operation_id":    e.operationID,
				"backend_call_id": backendCallID,
				"tool":            core.PublicName(e.target.serverID, e.target.rawName),
				"outcome":         outcome,
				"reason":          dispatchErr.Code,
				"duration_ms":     time.Since(backendStart).Milliseconds(),
			},
		})
		e.surface.logger.Log(observe.Entry{
			Level:     "warn",
			Event:     "mcp_complete",
			RequestID: e.requestID,
			Operation: "call_tool",
			Duration:  time.Since(e.requestStart),
			Outcome:   "error",
			Reason:    dispatchErr.Code,
			Fields: map[string]any{
				"request_id":       e.requestID,
				"operation_id":     e.operationID,
				"mcp_method":       "tools_call",
				"mcp_message_kind": "request",
				"stage":            "backend_dispatch",
				"outcome":          "error",
				"reason":           dispatchErr.Code,
				"duration_ms":      time.Since(e.requestStart).Milliseconds(),
			},
		})
	}
	e.surface.contributeOperation(e.requestID, e.operationID, upstreamStateFromDispatchCode(dispatchErr.Code))
}

func (e boundDirectExecution) logDispatchSuccess(backendCallID string, backendStart time.Time, upstream mcp.CallToolResult) {
	contentCount, textPresence, structuredPresence := summarizeShape(upstream.Content, upstream.StructuredContent)
	if e.surface.logger == nil {
		return
	}
	level, outcome := "info", "success"
	if upstream.IsError {
		level, outcome = "warn", "error"
	}
	reason := "none"
	if upstream.IsError {
		reason = "tool_reported_error"
	}
	e.surface.logger.Log(observe.Entry{
		Level:     level,
		Event:     "backend_complete",
		RequestID: e.requestID,
		Server:    e.target.serverID,
		Operation: "call_tool",
		Duration:  time.Since(backendStart),
		Outcome:   outcome,
		Reason:    reason,
		Fields: map[string]any{
			"request_id":                   e.requestID,
			"operation_id":                 e.operationID,
			"correlation_state":            e.correlation,
			"backend_call_id":              backendCallID,
			"tool":                         core.PublicName(e.target.serverID, e.target.rawName),
			"outcome":                      outcome,
			"reason":                       reason,
			"upstream_content_count":       contentCount,
			"upstream_text_presence":       textPresence,
			"upstream_structured_presence": structuredPresence,
			"duration_ms":                  time.Since(backendStart).Milliseconds(),
		},
	})
	e.surface.logger.Log(observe.Entry{
		Level:     level,
		Event:     "mcp_complete",
		RequestID: e.requestID,
		Operation: "call_tool",
		Duration:  time.Since(e.requestStart),
		Outcome:   outcome,
		Reason:    reason,
		Fields: map[string]any{
			"request_id":        e.requestID,
			"operation_id":      e.operationID,
			"correlation_state": e.correlation,
			"mcp_method":        "tools_call",
			"mcp_message_kind":  "request",
			"outcome":           outcome,
			"reason":            reason,
			"duration_ms":       time.Since(e.requestStart).Milliseconds(),
		},
	})
}
