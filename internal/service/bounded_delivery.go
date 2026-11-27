package service

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"unicode/utf8"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

const resultPageMaxWidth = uint64(math.MaxUint32)

func deliverBoundedResult(store *resultStore, result *mcp.CallToolResult, owner resultNamespace, resolve resultOwnerResolver, upstreamCode string) *mcp.CallToolResult {
	payload, err := json.Marshal(result)
	if err != nil {
		return errorResult(codeInternalError, "the bridge could not encode the tool result", core.ExecutionUnknown)
	}
	bounds := store.bounds
	if uint64(len(payload)) <= bounds.DeliveryCeilingBytes {
		return result
	}

	if owner.sessionID != "" && resolve != nil {
		if _, ok := resolve(owner); !ok {
			retention := "result_namespace_retired"
			if owner.epoch == "" {
				retention = "result_epoch_unresolved"
			}
			return renderResultTerminal(payload, result.IsError, upstreamCode, retention, bounds)
		}
	}

	commit := store.commit(owner, resolve, payload, upstreamCode, result.IsError)
	if commit.handle != "" {
		lookup := store.lookup(commit.handle, owner, resolve, 1)
		if lookup.entry == nil {
			return errorResult(codeInternalError, "the retained tool result is unavailable", core.ExecutionUnknown)
		}
		if store.logger != nil {
			logPagingEvent(store.logger, "info", "result_paged", "call_tool", "success", map[string]any{"total_bytes": len(payload)})
		}
		return renderResultPreview(lookup.entry)
	}
	return renderResultTerminal(payload, result.IsError, upstreamCode, commit.retention, bounds)
}

func renderResultPreview(entry *resultEntry) *mcp.CallToolResult {
	metadata := map[string]any{
		"paging_required": true,
		"handle":          entry.handle,
		"total_bytes":     uint64(len(entry.payload)),
		"is_error":        entry.isError,
	}
	if entry.upstreamCode != "" {
		metadata["upstream_code"] = entry.upstreamCode
	}
	content := boundedResultPrefix(entry.payload, entry.previewCeilingBytes, previewSizingMetadata(entry), entry.upstreamCode, entry.maxUpstreamCodeBytes)
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: content}},
		StructuredContent: metadata,
		IsError:           entry.isError,
	}
}

func renderResultPage(entry *resultEntry, page int) *mcp.CallToolResult {
	if entry == nil || page < 1 {
		return invalidArgumentsLookup().rejection
	}
	totalPages, start, end := resultPageLayout(entry, page)
	if totalPages == 0 || page > totalPages {
		return invalidArgumentsLookup().rejection
	}
	metadata := map[string]any{
		"handle":           entry.handle,
		"page":             page,
		"total_pages":      totalPages,
		"remaining_pages":  totalPages - page,
		"total_bytes":      uint64(len(entry.payload)),
		"remaining_bytes":  uint64(len(entry.payload) - end),
		"is_error":         entry.isError,
	}
	if entry.upstreamCode != "" {
		metadata["upstream_code"] = entry.upstreamCode
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(entry.payload[start:end])}},
		StructuredContent: metadata,
		IsError:           entry.isError,
	}
}

func resultPageCount(entry *resultEntry) int {
	total, _, _ := resultPageLayout(entry, 0)
	return total
}

func resultPageLayout(entry *resultEntry, requested int) (total, start, end int) {
	if entry == nil || len(entry.payload) == 0 {
		return 0, 0, 0
	}
	budget, ok := resultEnvelopeContentBudget(pageSizingMetadata(entry), entry.upstreamCode, entry.maxUpstreamCodeBytes, entry.deliveryCeilingBytes)
	if !ok {
		return 0, 0, 0
	}
	for offset := 0; offset < len(entry.payload); {
		total++
		pageStart := offset
		var escapedBytes uint64
		for offset < len(entry.payload) {
			r, width := utf8.DecodeRune(entry.payload[offset:])
			runeBytes := uint64(escapedJSONRuneLength(r))
			if escapedBytes+runeBytes > budget {
				break
			}
			escapedBytes += runeBytes
			offset += width
		}
		if offset == pageStart {
			return 0, 0, 0
		}
		if total == requested {
			start, end = pageStart, offset
		}
	}
	return total, start, end
}

func renderResultTerminal(payload []byte, isError bool, upstreamCode, retention string, bounds config.RetainedResultBounds) *mcp.CallToolResult {
	metadata := map[string]any{
		"retention":       retention,
		"execution_state": "executed",
		"total_bytes":     uint64(len(payload)),
		"is_error":        isError,
	}
	if upstreamCode != "" {
		metadata["upstream_code"] = upstreamCode
	}
	content := boundedResultPrefix(payload, bounds.PreviewCeilingBytes, terminalSizingMetadata(retention, upstreamCode), upstreamCode, bounds.MaxUpstreamCodeBytes)
	return terminalResult(content, isError, len(payload), retention, upstreamCode)
}

func terminalResult(content string, isError bool, totalBytes int, retention, upstreamCode string) *mcp.CallToolResult {
	metadata := map[string]any{
		"retention":       retention,
		"execution_state": "executed",
		"total_bytes":     totalBytes,
		"is_error":        isError,
	}
	if upstreamCode != "" {
		metadata["upstream_code"] = upstreamCode
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: content}},
		StructuredContent: metadata,
		IsError:           isError,
	}
}

func previewSizingMetadata(entry *resultEntry) map[string]any {
	return map[string]any{
		"paging_required": true,
		"handle":          entry.handle,
		"total_bytes":     uint64(math.MaxUint64),
		"is_error":        false,
		"upstream_code":   entry.upstreamCode,
	}
}

func pageSizingMetadata(entry *resultEntry) map[string]any {
	return map[string]any{
		"handle":           entry.handle,
		"page":             resultPageMaxWidth,
		"total_pages":      resultPageMaxWidth,
		"remaining_pages":  resultPageMaxWidth,
		"total_bytes":      uint64(math.MaxUint64),
		"remaining_bytes":  uint64(math.MaxUint64),
		"is_error":         false,
		"upstream_code":    entry.upstreamCode,
	}
}

func terminalSizingMetadata(retention, upstreamCode string) map[string]any {
	return map[string]any{
		"retention":       "result_aggregate_capacity_exceeded",
		"execution_state": "executed",
		"total_bytes":     uint64(math.MaxUint64),
		"is_error":        false,
		"upstream_code":   upstreamCode,
	}
}

func boundedResultPrefix(payload []byte, ceiling uint64, metadata map[string]any, upstreamCode string, maxUpstreamCodeBytes uint64) string {
	base := &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{}},
		StructuredContent: metadata,
	}
	measured, err := measureResult(base)
	if err != nil {
		return ""
	}
	codeWidth := escapedJSONLength(upstreamCode)
	if codeWidth > maxUpstreamCodeBytes {
		return ""
	}
	baseBytes := uint64(measured) + maxUpstreamCodeBytes - codeWidth
	if baseBytes >= ceiling {
		return ""
	}
	budget := ceiling - baseBytes
	end := 0
	var used uint64
	for offset := 0; offset < len(payload); {
		r, width := utf8.DecodeRune(payload[offset:])
		cost := uint64(escapedJSONRuneLength(r))
		if used+cost > budget {
			break
		}
		used += cost
		offset += width
		end = offset
	}
	return string(payload[:end])
}

func resultEnvelopeContentBudget(metadata map[string]any, upstreamCode string, maxUpstreamCodeBytes, ceiling uint64) (uint64, bool) {
	base := &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{}},
		StructuredContent: metadata,
	}
	measured, err := measureResult(base)
	if err != nil {
		return 0, false
	}
	codeWidth := escapedJSONLength(upstreamCode)
	if codeWidth > maxUpstreamCodeBytes {
		return 0, false
	}
	baseBytes := uint64(measured) + maxUpstreamCodeBytes - codeWidth
	if baseBytes >= ceiling {
		return 0, false
	}
	return ceiling - baseBytes, true
}

func (t *ToolSurface) boundedCallToolResult(request mcp.Request, result *mcp.CallToolResult, upstreamCode string) *mcp.CallToolResult {
	if t.cfg.RetainedResults == nil {
		return result
	}
	t.resultMu.Lock()
	defer t.resultMu.Unlock()
	bounds := t.cfg.RetainedResults()
	upstreamCode = boundedUpstreamCode(upstreamCode, bounds.MaxUpstreamCodeBytes)
	if t.results == nil {
		t.results = newResultStore(bounds, t.logger, t.cfg.Metrics, nil, nil)
	} else {
		t.results.mu.Lock()
		t.results.bounds = bounds
		t.results.mu.Unlock()
	}
	owner, resolve := t.resultOwner(request, true)
	return deliverBoundedResult(t.results, result, owner, resolve, upstreamCode)
}

func (t *ToolSurface) sweepRetainedResults() {
	t.resultMu.Lock()
	defer t.resultMu.Unlock()
	if t.results != nil {
		t.results.sweep()
	}
}

// retireRetainedResultNamespace is R7's release-funded retirement trigger.
// A namespace holding no live record completes with zero store effect inside retire.
func (t *ToolSurface) retireRetainedResultNamespace(owner projectionOwner) {
	if owner.kind != projectionSession || owner.epoch == 0 {
		return
	}
	t.resultMu.Lock()
	defer t.resultMu.Unlock()
	if t.results == nil {
		return
	}
	t.results.retire(resultNamespace{
		sessionID: owner.sessionID,
		epoch:     strconv.FormatUint(owner.epoch, 10),
	})
}


func (t *ToolSurface) resultOwner(request mcp.Request, allowUnresolvedEpoch bool) (resultNamespace, resultOwnerResolver) {
	if request == nil || request.GetExtra() == nil || request.GetExtra().Header == nil {
		return publisherResultOwner(t), publisherResultResolver
	}
	sessionID := request.GetExtra().Header.Get(sessionIDHeader)
	if sessionID == "" {
		return publisherResultOwner(t), publisherResultResolver
	}
	owner := resultNamespace{sessionID: sessionID}
	if epoch, ok := requestExposureEpoch(request); ok {
		owner.epoch = strconv.FormatUint(epoch, 10)
	}
	resolve := func(requested resultNamespace) (resultNamespace, bool) {
		if requested != owner || requested.publisher {
			return resultNamespace{}, false
		}
		if requested.epoch == "" && allowUnresolvedEpoch {
			return requested, true
		}
		if requested.epoch == "" {
			return resultNamespace{}, false
		}
		epoch, err := strconv.ParseUint(requested.epoch, 10, 64)
		if err != nil || epoch == 0 {
			return resultNamespace{}, false
		}
		liveOwner := projectionOwner{kind: projectionSession, sessionID: requested.sessionID, epoch: epoch}
		coordinator := t.projectionCoordinator()
		coordinator.barrier.Lock()
		_, live := coordinator.liveOwners[liveOwner]
		coordinator.barrier.Unlock()
		return requested, live
	}
	return owner, resolve
}

func publisherResultOwner(t *ToolSurface) resultNamespace {
	_ = t
	return resultNamespace{publisher: true}
}

func publisherResultResolver(owner resultNamespace) (resultNamespace, bool) {
	return owner, owner.publisher && owner.sessionID == "" && owner.epoch == ""
}

func (t *ToolSurface) retrieveResult(_ context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input struct {
		Handle json.RawMessage `json:"handle"`
		Page   json.RawMessage `json:"page"`
	}
	if err := decodeStrict(rawArguments(request), &input); err != nil {
		return t.rejectResultRetrieval("invalid_arguments"), nil
	}
	var handle string
	if len(input.Handle) == 0 || json.Unmarshal(input.Handle, &handle) != nil || !validResultHandle(handle) {
		return t.rejectResultRetrieval("invalid_arguments"), nil
	}
	t.resultMu.Lock()
	defer t.resultMu.Unlock()
	if t.results == nil {
		return t.rejectResultRetrieval("invalid_handle"), nil
	}
	owner, resolve := t.resultOwner(request, false)
	lookup := t.results.lookup(handle, owner, resolve, 1)
	if lookup.rejection != nil {
		t.logRetrievalFailure(lookup.code)
		return lookup.rejection, nil
	}
	if lookup.code != "" {
		t.logRetrievalFailure(lookup.code)
		return retrievalCodeResult(lookup.code), nil
	}
	page := 1
	if len(input.Page) > 0 {
		if err := json.Unmarshal(input.Page, &page); err != nil || page < 1 {
			return t.rejectResultRetrieval("invalid_arguments"), nil
		}
	}
	totalPages := resultPageCount(lookup.entry)
	if page > totalPages {
		return t.rejectResultRetrieval("invalid_arguments"), nil
	}
	result := renderResultPage(lookup.entry, page)
	if t.cfg.Metrics != nil {
		t.cfg.Metrics.RecordResultPageServed()
	}
	if t.logger != nil {
		logPagingEvent(t.logger, "info", "result_page_served", "retrieve_result", "success", map[string]any{"page": page, "total_pages": totalPages})
	}
	return result, nil
}

func (t *ToolSurface) rejectResultRetrieval(code string) *mcp.CallToolResult {
	lookup := invalidArgumentsLookup()
	if code == "invalid_handle" {
		lookup = invalidHandleLookup()
	}
	t.logRetrievalFailure(code)
	return lookup.rejection
}

func (t *ToolSurface) logRetrievalFailure(code string) {
	if t.logger == nil {
		return
	}
	message := code
	logPagingEvent(t.logger, "warn", code, "retrieve_result", "error", nil)
	_ = message
}

func retrievalCodeResult(code string) *mcp.CallToolResult {
	message := "result retrieval failed"
	switch code {
	case "result_expired":
		message = "result handle expired"
	case "result_evicted":
		message = "result handle was evicted"
	}
	return errorResult(code, message, core.ExecutionUnknown)
}

func logPagingEvent(logger *observe.Logger, level, event, operation, outcome string, fields map[string]any) {
	logger.Log(observe.Entry{Level: level, Event: event, Operation: operation, Outcome: outcome, Fields: fields})
}
