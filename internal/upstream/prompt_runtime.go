package upstream

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GetPrompt performs one guarded prompt request against an initialized session.
func (c *ServerClient) GetPrompt(parent context.Context, session *mcp.ClientSession, params *mcp.GetPromptParams) (*mcp.GetPromptResult, error) {
	if session == nil || params == nil {
		return nil, &CallError{Code: "backend_unavailable", ExecutionState: "not_started"}
	}

	ctx, cancel := c.CallContext(parent)
	defer cancel()

	release, err := c.AdmitCall(ctx)
	if err != nil {
		if IsBusy(err) {
			return nil, &CallError{Code: "backend_busy", ExecutionState: "not_started"}
		}
		return nil, &CallError{Code: "backend_unavailable", ExecutionState: "not_started"}
	}
	defer release()

	result, err := session.GetPrompt(ctx, params)
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
			// An upstream protocol-level rejection is a remote answer, not a
			// transport fault, so it must not trip the per-server breaker.
			return nil, &CallError{Code: "upstream_protocol_error", ExecutionState: "unknown"}
		}
		c.RecordFailure(FailureTransport)
		return nil, &CallError{Code: "backend_unavailable", ExecutionState: "unknown"}
	}
	if result == nil {
		c.RecordFailure(FailureMalformed)
		return nil, &CallError{Code: "upstream_protocol_error", ExecutionState: "unknown"}
	}

	c.RecordSuccess()
	return result, nil
}
