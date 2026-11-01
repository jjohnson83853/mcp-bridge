package service

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const exposureEpochHeaderName = "X-Mcp-Bridge-Exposure-Epoch"

func (t *ToolSurface) registerReceivingMiddleware() {
	t.mu.Lock()
	server := t.server
	if server == nil || t.receivingMiddlewareRegistered {
		t.mu.Unlock()
		return
	}
	t.receivingMiddlewareRegistered = true
	t.mu.Unlock()
	server.AddReceivingMiddleware(t.receivingMiddleware)
}

func (t *ToolSurface) receivingMiddleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
		relevant, isList, directTool := t.requiresExposure(method, request)
		if !relevant {
			return next(ctx, method, request)
		}

		if t.testBeforeExposureAdmission != nil {
			t.testBeforeExposureAdmission()
		}
		projection, err := t.admitExposureRequest(request)
		if err != nil {
			return nil, err
		}
		if directTool != "" {
			if _, visible := projection.byName[directTool]; !visible {
				return nil, invalidExposureRequest()
			}
		}
		if isList {
			listReq, _ := request.(*mcp.ListToolsRequest)
			cursor := ""
			if listReq != nil && listReq.Params != nil {
				cursor = listReq.Params.Cursor
			}
			return t.ownerToolListPage(projection, cursor)
		}
		return next(ctx, method, request)
	}
}

func (t *ToolSurface) requiresExposure(method string, request mcp.Request) (relevant, list bool, directTool string) {
	if method == "tools/list" {
		return true, true, ""
	}
	if method != "tools/call" {
		return false, false, ""
	}
	call, ok := request.(*mcp.CallToolRequest)
	if !ok || call == nil || call.Params == nil {
		return false, false, ""
	}
	switch call.Params.Name {
	case catalog.ToolCallTool:
		return false, false, ""
	case catalog.ToolSearchTools, catalog.ToolDescribeTool:
		return true, false, ""
	default:
		if t.isProjectedToolName(call.Params.Name) {
			return true, false, call.Params.Name
		}
		return false, false, ""
	}
}

func (t *ToolSurface) isProjectedToolName(name string) bool {
	coordinator := t.projectionCoordinator()
	coordinator.barrier.Lock()
	defer coordinator.barrier.Unlock()
	_, exists := coordinator.union[name]
	return exists
}

func (t *ToolSurface) admitExposureRequest(request mcp.Request) (*toolProjection, error) {
	epoch, valid := requestExposureEpoch(request)
	if !valid || request.GetSession() == nil {
		return nil, invalidExposureRequest()
	}
	sessionID := request.GetSession().ID()
	if sessionID == "" {
		return nil, invalidExposureRequest()
	}
	owner := projectionOwner{kind: projectionSession, sessionID: sessionID, epoch: epoch}
	coordinator := t.projectionCoordinator()
	coordinator.barrier.Lock()
	_, live := coordinator.liveOwners[owner]
	coordinator.barrier.Unlock()
	if !live {
		return nil, invalidExposureRequest()
	}
	projection, err := t.ensureOwnerProjection(owner)
	if err != nil {
		if rpcErr, ok := err.(*jsonrpc.Error); ok {
			return nil, rpcErr
		}
		return nil, &jsonrpc.Error{
			Code:    jsonrpc.CodeInternalError,
			Message: "exposure unavailable",
		}
	}
	return projection, nil
}

func requestExposureEpoch(request mcp.Request) (uint64, bool) {
	if request == nil {
		return 0, false
	}
	extra := request.GetExtra()
	if extra == nil || extra.Header == nil {
		return 0, false
	}
	count := 0
	value := ""
	for name, values := range extra.Header {
		if !strings.EqualFold(name, exposureEpochHeaderName) {
			continue
		}
		for _, candidate := range values {
			count++
			value = candidate
		}
	}
	if count != 1 {
		return 0, false
	}
	epoch, err := strconv.ParseUint(value, 10, 64)
	if err != nil || epoch == 0 || strconv.FormatUint(epoch, 10) != value {
		return 0, false
	}
	return epoch, true
}

func invalidExposureRequest() error {
	return &jsonrpc.Error{Code: jsonrpc.CodeInvalidRequest, Message: "invalid exposure request"}
}

func (t *ToolSurface) ensureOwnerProjection(owner projectionOwner) (*toolProjection, error) {
	coordinator := t.projectionCoordinator()
	for {
		coordinator.barrier.Lock()
		if _, live := coordinator.liveOwners[owner]; !live {
			coordinator.barrier.Unlock()
			return nil, invalidExposureRequest()
		}
		if projection := coordinator.projections[owner]; projection != nil {
			coordinator.barrier.Unlock()
			return projection, nil
		}
		coordinator.barrier.Unlock()

		if assembly := t.runtime.Current(); assembly != nil {
			if err := t.publishOwnerProjection(owner, assembly); err != nil {
				return nil, err
			}
			continue
		}

		coordinator.publicationMu.Lock()
		coordinator.barrier.Lock()
		if _, live := coordinator.liveOwners[owner]; !live {
			coordinator.barrier.Unlock()
			coordinator.publicationMu.Unlock()
			return nil, invalidExposureRequest()
		}
		if projection := coordinator.projections[owner]; projection != nil {
			coordinator.barrier.Unlock()
			coordinator.publicationMu.Unlock()
			return projection, nil
		}
		if t.runtime.Current() != nil {
			coordinator.barrier.Unlock()
			coordinator.publicationMu.Unlock()
			continue
		}
		projection, err := prepareProjection(owner, nil, 0, nil, nil)
		var delta registryDelta
		if err == nil {
			delta, err = coordinator.replaceProjectionLocked(owner, projection)
		}
		coordinator.barrier.Unlock()
		if err == nil {
			t.applyRegistryDelta(delta)
		}
		coordinator.publicationMu.Unlock()
		if err != nil {
			return nil, err
		}
	}
}

func (t *ToolSurface) ownerToolListPage(projection *toolProjection, cursor string) (*mcp.ListToolsResult, error) {
	tools := make([]*mcp.Tool, 0, len(t.builtinTools)+len(projection.tools))
	tools = append(tools, t.builtinTools...)
	tools = append(tools, projection.tools...)
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})

	pageSize := t.pageSize
	if pageSize <= 0 {
		pageSize = mcp.DefaultPageSize
	}
	total := len(tools)
	offset := 0
	if cursor != "" {
		var err error
		offset, err = decodeAndValidateCursor(t.processSecret, cursor, projection.owner.epoch, projection.generation, pageSize, total)
		if err != nil {
			return nil, err
		}
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	pageItems := tools[offset:end]
	nextCursor := ""
	if end < total {
		nextCursor = encodeCursor(t.processSecret, projection.owner.epoch, projection.generation, uint64(end))
	}
	return &mcp.ListToolsResult{Tools: pageItems, NextCursor: nextCursor}, nil
}
