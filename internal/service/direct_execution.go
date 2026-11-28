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

const directProjectionMember = "mcp_bridge_projection"

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
	request      mcp.Request
}

func (e boundDirectExecution) call(ctx context.Context, args json.RawMessage) (*mcp.CallToolResult, error) {
	defer e.lease.Release()

	if len(e.inputSchema) > 0 {
		if err := catalog.ValidateSchemaDocument(string(e.inputSchema)); err != nil {
			e.surface.logAdmissionFailure(ctx, e.requestID, e.operationID, e.correlation, e.requestStart, "call_tool", "invalid_arguments")
			return errorResult(search.CodeInvalidArguments, "arguments do not satisfy the tool's stored input schema", core.ExecutionNotStarted), nil
		}
		if err := catalog.ValidateSchema(string(e.inputSchema), string(args)); err != nil {
			// The validator's own text may quote submitted values, so use a fixed message.
			e.surface.logAdmissionFailure(ctx, e.requestID, e.operationID, e.correlation, e.requestStart, "call_tool", "invalid_arguments")
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
	var projectionDecisionValue projectionDecision
	if !upstream.IsError {
		fieldsBudget := maxProjectionFieldsBudget(16*67 + 1)
		if e.surface.cfg.RetainedResults != nil {
			fieldsBudget = projectionFieldsBudget(e.surface.cfg.RetainedResults())
		}
		projectionDecisionValue = applyDirectProjection(&upstream, e.binding.ServerID, e.binding.RawName, args, fieldsBudget)
		if projectionDecisionDisclosed(projectionDecisionValue) {
			outcome, logEvent := "skipped", true
			if projectionDecisionValue.Applied {
				outcome, logEvent = "applied", false
			}
			e.surface.recordProjectionDecision(e.binding.ServerID, projectionDecisionValue, outcome, logEvent)
		}
	}
	result := &mcp.CallToolResult{
		Content:           upstream.Content,
		StructuredContent: upstream.StructuredContent,
		IsError:           upstream.IsError,
	}
	if projectionDecisionDisclosed(projectionDecisionValue) {
		return e.surface.boundedCallToolResult(e.request, result, directUpstreamCode(upstream.StructuredContent), projectionDecisionValue), nil
	}
	return e.surface.boundedCallToolResult(e.request, result, directUpstreamCode(upstream.StructuredContent)), nil
}
func directUpstreamCode(structured any) string {
	content, ok := structured.(map[string]any)
	if !ok {
		return ""
	}
	code, _ := content["code"].(string)
	return code
}

func applyDirectProjection(result *mcp.CallToolResult, server, tool string, rawArguments json.RawMessage, fieldsBudget int) projectionDecision {
	if result == nil || result.IsError {
		return projectionDecision{}
	}
	_, args := projectionRequestArgs(nil, rawArguments)
	path := projectionPath(args)
	admission, err := decideProjection(server, tool, path, args, 0, fieldsBudget)
	if err != nil || admission.Entry == "" {
		return projectionDecision{}
	}
	structured, _ := result.StructuredContent.(map[string]any)
	_, collision := structured[directProjectionMember]
	candidate := *result
	decision, err := transformProjectionResult(&candidate, admission, server, tool, path, args, fieldsBudget)
	if err != nil {
		return projectionFailure(admission, projectionReasonUnknownFieldSelection)
	}
	if collision && decision.Applied {
		decision = projectionFailure(decision, projectionReasonDisclosureKeyCollision)
	} else if decision.Applied {
		*result = candidate
	}
	if projectionDecisionDisclosed(decision) && !collision {
		switch structured := result.StructuredContent.(type) {
		case nil:
			result.StructuredContent = projectionDisclosure(decision)
		case map[string]any:
			if structured != nil {
				updated := make(map[string]any, len(structured)+1)
				for key, value := range structured {
					updated[key] = value
				}
				updated[directProjectionMember] = projectionDisclosure(decision)
				result.StructuredContent = updated
			}
		}
	}
	return decision
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
			Error:     observe.RuntimeErrorInfo(dispatchErr.Code),
			Fields: map[string]any{
				"operation_id":    e.operationID,
				"backend_call_id": backendCallID,
				"tool":            core.PublicName(e.target.serverID, e.target.rawName),
			},
		})
		e.surface.logger.Log(observe.Entry{
			Level:     "warn",
			Event:     "mcp_complete",
			RequestID: e.requestID,
			Operation: "call_tool",
			Duration:  time.Since(e.requestStart),
			Outcome:   "error",
			Error:     observe.RuntimeErrorInfo(dispatchErr.Code),
			Fields: map[string]any{
				"operation_id":     e.operationID,
				"mcp_method":       "tools_call",
				"mcp_message_kind": "request",
				"stage":            "backend_dispatch",
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
		Error:     observe.RuntimeErrorInfo(reason),
		Fields: map[string]any{
			"operation_id":                 e.operationID,
			"correlation_state":            e.correlation,
			"backend_call_id":              backendCallID,
			"tool":                         core.PublicName(e.target.serverID, e.target.rawName),
			"upstream_content_count":       contentCount,
			"upstream_text_presence":       textPresence,
			"upstream_structured_presence": structuredPresence,
		},
	})
	e.surface.logger.Log(observe.Entry{
		Level:     level,
		Event:     "mcp_complete",
		RequestID: e.requestID,
		Operation: "call_tool",
		Duration:  time.Since(e.requestStart),
		Outcome:   outcome,
		Error:     observe.RuntimeErrorInfo(reason),
		Fields: map[string]any{
			"operation_id":      e.operationID,
			"correlation_state": e.correlation,
			"mcp_method":        "tools_call",
			"mcp_message_kind":  "request",
		},
	})
}
