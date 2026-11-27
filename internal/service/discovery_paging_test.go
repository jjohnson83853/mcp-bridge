package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

const pagingTestTextSize = 12000

func newPagingTestObservation(t *testing.T, surfaceName string, logger *observe.Logger) (*progressiveSessionHarness, string, *mcp.CallToolResult, string) {
	t.Helper()
	text := strings.Repeat("x", pagingTestTextSize)
	switch surfaceName {
	case "call_tool", "direct_invocation":
		upstreamResult := fmt.Sprintf(`{"content":[{"type":"text","text":%q}],"structuredContent":{"code":"upstream-code"},"isError":true}`, text)
		upstream := &directExecutionUpstream{
			calls:  make(chan directExecutionCall, 2),
			result: json.RawMessage(upstreamResult),
		}
		harness, sessionID, owner, projection, _, assembly := newDirectExecutionFixture(t, upstream)
		configurePagingTestSurface(harness, logger)
		if surfaceName == "direct_invocation" {
			if owner.kind != projectionSession || projection == nil || harness.projection(owner) == nil || len(projection.tools) == 0 {
				t.Fatal("direct paging fixture has no live owner projection")
			}
			response := harness.send(t, http.MethodPost, sessionID, directCallBody(projection.tools[0].Name, `{}`), nil)
			result := decodePagingToolResult(t, response)
			return harness, sessionID, result, upstreamResult
		}

		arguments := fmt.Sprintf(`{"tool_name":"alpha__old","catalog_revision":%q,"arguments":{}}`, assembly.CatalogRevision())
		response := harness.send(t, http.MethodPost, sessionID, pagingToolCallBody(catalog.ToolCallTool, arguments), nil)
		result := decodePagingToolResult(t, response)
		metadata := structuredResult(result)
		handle, _ := metadata["handle"].(string)
		store := harness.surface.results
		store.mu.Lock()
		entry := store.entries[handle]
		var originalPayload []byte
		if entry != nil {
			originalPayload = append([]byte(nil), entry.payload...)
		}
		store.mu.Unlock()
		if entry == nil {
			t.Fatal("call_tool paging entry missing from result store")
		}
		var original struct {
			StructuredContent map[string]any `json:"structuredContent"`
		}
		if err := json.Unmarshal(originalPayload, &original); err != nil {
			t.Fatalf("decode retained call_tool result: %v", err)
		}
		requestID, _ := original.StructuredContent["request_id"].(string)
		// request_id is generated per invocation; all other retained bytes are golden literals.
		upstreamGolden := fmt.Sprintf(`{"content":[{"text":%q,"type":"text"}],"isError":true,"structuredContent":{"code":"upstream-code"}}`, text)
		payloadGolden := fmt.Sprintf(`{"request_id":%q,"server":"alpha","tool_name":"alpha__old","upstream_result":%s}`, requestID, upstreamGolden)
		golden := fmt.Sprintf(`{"content":[{"type":"text","text":%q}],"structuredContent":%s,"isError":true}`, text, payloadGolden)
		return harness, sessionID, result, golden

	case "search_tools", "describe_tool":
		harness := newProgressiveSessionHarness(t)
		description := "needle " + text
		searchDescription := "needle " + strings.Repeat("d", 29)
		definitions := []core.ToolDefinition{{
			RawName:     "old",
			Description: description,
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}}
		if surfaceName == "search_tools" {
			definitions[0].Description = "old tool"
			for _, suffix := range []string{"a", "b", "c", "d"} {
				rawName := strings.Repeat("x", 56) + suffix
				definitions = append(definitions, core.ToolDefinition{
					RawName:     rawName,
					Description: searchDescription,
					InputSchema: json.RawMessage(`{"type":"object"}`),
				})
			}
		}
		deps := progressiveTestDeps(&progressiveTestUpstream{})
		deps.Discover = func(config.Server) ([]core.ToolDefinition, error) {
			return definitions, nil
		}
		assembly, err := harness.surface.runtime.Reload(progressiveTestConfig(t, "old"), deps)
		if err != nil {
			t.Fatalf("reload paging fixture catalog: %v", err)
		}
		if err := harness.surface.Published(assembly); err != nil {
			t.Fatalf("publish paging fixture catalog: %v", err)
		}
		harness.assembly = assembly
		configurePagingTestSurface(harness, logger)
		sessionID := harness.initialize(t)
		if surfaceName == "search_tools" {
			response := harness.send(t, http.MethodPost, sessionID, pagingToolCallBody(catalog.ToolSearchTools, `{"query":"needle","top_k":4}`), nil)
			hits := make([]string, 0, 4)
			// Equal BM25 scores tie-break on stable core.PublicName hashes, not wire-name order.
			suffixes := []string{"c", "a", "d", "b"}
			scores := []string{"0.00819672131147541", "0.008064516129032258", "0.007936507936507936", "0.0078125"}
			for index, suffix := range suffixes {
				rawName := strings.Repeat("x", 56) + suffix
				hits = append(hits, fmt.Sprintf(`{"description":%q,"score":%s,"tool_name":%q}`, searchDescription,
					scores[index], "alpha__"+rawName))
			}
			payload := fmt.Sprintf(`{"catalog_revision":%q,"degraded_reasons":["embed_provider_unavailable"],"mode":"keyword","results":[%s]}`,
				assembly.CatalogRevision(), strings.Join(hits, ","))
			golden := pagingTextResultGolden(payload)
			if len(golden) <= 1024 {
				t.Fatalf("search paging fixture bytes = %d; want more than the 1024-byte test ceiling", len(golden))
			}
			return harness, sessionID, decodePagingToolResult(t, response), golden
		}

		arguments := `{"tool_name":"alpha__old"}`
		response := harness.send(t, http.MethodPost, sessionID, pagingToolCallBody(catalog.ToolDescribeTool, arguments), nil)
		payload := fmt.Sprintf(`{"availability":"available","catalog_revision":%q,"description":%q,"input_schema":{"type":"object"},"pinned":true,"server":"alpha","tool_name":"alpha__old","upstream_name":"old","validation_status":"supported"}`, assembly.CatalogRevision(), description)
		return harness, sessionID, decodePagingToolResult(t, response), pagingTextResultGolden(payload)
	default:
		t.Fatalf("unknown paging surface %q", surfaceName)
		return nil, "", nil, ""
	}
}

func configurePagingTestSurface(harness *progressiveSessionHarness, logger *observe.Logger) {
	bounds := resultStoreBounds()
	bounds.DeliveryCeilingBytes = 1024
	bounds.PreviewCeilingBytes = 512
	bounds.MaxRetainedResultBytes = 1 << 20
	bounds.MaxBytesPerNamespace = 1 << 20
	bounds.MaxBytesAggregate = 1 << 20
	bounds.MaxEntriesPerNamespace = 16
	bounds.MaxUpstreamCodeBytes = 24
	harness.surface.cfg.RetainedResults = func() config.RetainedResultBounds { return bounds }
	harness.surface.logger = logger
}

func pagingToolCallBody(name, arguments string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, name, arguments)
}

func decodePagingToolResult(t *testing.T, response progressiveHTTPResponse) *mcp.CallToolResult {
	t.Helper()
	if response.Response == nil || response.StatusCode != http.StatusOK {
		t.Fatalf("tool call status = %v, want 200 (body=%s)", response.Response, response.body)
	}
	wire := decodeDirectToolResponse(t, response.body).Result
	content := make([]mcp.Content, len(wire.Content))
	for index, block := range wire.Content {
		if block.Type != "text" {
			t.Fatalf("tool result content[%d] type = %q, want text", index, block.Type)
		}
		content[index] = &mcp.TextContent{Text: block.Text}
	}
	return &mcp.CallToolResult{Content: content, StructuredContent: wire.StructuredContent, IsError: wire.IsError}
}

func pagingTextResultGolden(payload string) string {
	return fmt.Sprintf(`{"content":[{"type":"text","text":%s}],"structuredContent":%s}`, strconv.Quote(payload), payload)
}

func retrievePagingTestPage(t *testing.T, harness *progressiveSessionHarness, sessionID, handle string, page int) *mcp.CallToolResult {
	t.Helper()
	arguments := fmt.Sprintf(`{"handle":%q,"page":%d}`, handle, page)
	response := harness.send(t, http.MethodPost, sessionID, pagingToolCallBody(catalog.ToolRetrieveResult, arguments), nil)
	return decodePagingToolResult(t, response)
}

func TestPagingAllSurfaces(t *testing.T) {
	for _, surfaceName := range []string{"call_tool", "direct_invocation", "search_tools", "describe_tool"} {
		t.Run(surfaceName, func(t *testing.T) {
			harness, sessionID, preview, golden := newPagingTestObservation(t, surfaceName, nil)
			if preview == nil || golden == "" {
				t.Fatalf("paging fixture returned result=%#v and golden length=%d", preview, len(golden))
			}
			metadata := structuredResult(preview)
			wantUpstreamCode := surfaceName == "call_tool" || surfaceName == "direct_invocation"
			if preview.IsError != wantUpstreamCode || metadata["is_error"] != wantUpstreamCode || metadata["paging_required"] != true || metadata["total_bytes"] != float64(len(golden)) {
				t.Fatalf("preview = isError:%v metadata:%#v; golden bytes=%d", preview.IsError, metadata, len(golden))
			}
			handle, ok := metadata["handle"].(string)
			if !ok || len(handle) != 43 {
				t.Fatalf("preview handle = %#v, want 43 characters", metadata["handle"])
			}
			if wantUpstreamCode {
				if metadata["upstream_code"] != "upstream-code" {
					t.Fatalf("preview upstream_code = %#v", metadata["upstream_code"])
				}
			} else if _, exists := metadata["upstream_code"]; exists {
				t.Fatalf("bridge-generated preview unexpectedly has upstream_code: %#v", metadata)
			}
			if len(metadata) != 4+boolCount(wantUpstreamCode) {
				t.Fatalf("preview metadata fields = %#v", metadata)
			}
			measured, err := measureResult(preview)
			if err != nil || measured > 512 {
				t.Fatalf("preview serialized bytes = %d, err=%v; want <= 512", measured, err)
			}
			previewText := preview.Content[0].(*mcp.TextContent).Text
			if previewText == "" || !utf8.ValidString(previewText) {
				t.Fatalf("preview content is empty or invalid UTF-8: %q", previewText)
			}

			var joined strings.Builder
			var firstPage *mcp.CallToolResult
			totalPages := 0
			for page := 1; ; page++ {
				result := retrievePagingTestPage(t, harness, sessionID, handle, page)
				pageMetadata := structuredResult(result)
				if pageMetadata["page"] != float64(page) {
					t.Fatalf("page %d metadata = %#v", page, pageMetadata)
				}
				if page == 1 {
					firstPage = result
					totalPageCount, _ := pageMetadata["total_pages"].(float64)
					totalPages = int(totalPageCount)
				}
				if result.IsError != wantUpstreamCode || pageMetadata["is_error"] != wantUpstreamCode {
					t.Fatalf("page %d execution error facts = %#v / %v", page, pageMetadata, result.IsError)
				}
				if wantUpstreamCode && pageMetadata["upstream_code"] != "upstream-code" {
					t.Fatalf("page %d upstream_code = %#v", page, pageMetadata["upstream_code"])
				}
				joined.WriteString(result.Content[0].(*mcp.TextContent).Text)
				if pageMetadata["remaining_pages"] == float64(0) {
					break
				}
			}
			if totalPages < 2 {
				t.Fatalf("fixture produced %d pages, want multiple", totalPages)
			}
			actual := joined.String()
			if actual != golden {
				firstDifferent := 0
				for firstDifferent < len(actual) && firstDifferent < len(golden) && actual[firstDifferent] == golden[firstDifferent] {
					firstDifferent++
				}
				start := firstDifferent - 32
				if start < 0 {
					start = 0
				}
				end := firstDifferent + 64
				if end > len(actual) {
					end = len(actual)
				}
				t.Fatalf("concatenated page bytes differ from independent golden at %d: got=%q want=%q", firstDifferent, actual[start:end], golden[start:end])
			}
			repeated := retrievePagingTestPage(t, harness, sessionID, handle, 1)
			if !equalCallToolResults(firstPage, repeated) {
				t.Fatal("re-retrieving page 1 changed its bytes")
			}
		})
	}
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}
