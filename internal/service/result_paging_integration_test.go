package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
)

func TestReadmeInventoryAndPromise(t *testing.T) {
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatalf("read README.md from repository root: %v", err)
	}

	for _, heading := range []string{
		"## Tool surface",
		"### Progressive metadata reveal and session projections",
	} {
		section := readmeAnchorSection(t, string(readme), heading)
		assertReadmeInventoryAndPromise(t, heading, section)
	}
}

func readmeAnchorSection(t *testing.T, readme, heading string) string {
	t.Helper()
	if strings.Count(readme, heading) != 1 {
		t.Fatalf("README anchor %q occurs %d times, want exactly once", heading, strings.Count(readme, heading))
	}
	start := strings.Index(readme, heading) + len(heading)
	remainder := readme[start:]
	end := strings.Index(remainder, "\n## ")
	if end < 0 {
		t.Fatalf("README anchor %q has no following section boundary", heading)
	}
	return remainder[:end]
}

func assertReadmeInventoryAndPromise(t *testing.T, heading, section string) {
	t.Helper()
	for _, tool := range []string{"search_tools", "describe_tool", "call_tool", "retrieve_result"} {
		if !strings.Contains(section, "`"+tool+"`") {
			t.Errorf("README anchor %q does not positively name %q", heading, tool)
		}
	}

	lower := strings.ToLower(section)
	for _, term := range []string{"handle", "page", "total_pages", "bounded preview"} {
		if !strings.Contains(lower, term) {
			t.Errorf("README anchor %q does not assert %q", heading, term)
		}
	}
	if !strings.Contains(lower, "byte-exact") && !strings.Contains(lower, "byte-for-byte") {
		t.Errorf("README anchor %q does not assert byte-exact reconstruction", heading)
	}
	if !strings.Contains(lower, "reconstruct") && !strings.Contains(lower, "reconstruction") {
		t.Errorf("README anchor %q does not describe page reconstruction", heading)
	}

	plain := strings.ToLower(strings.ReplaceAll(section, "`", ""))
	for _, forbidden := range []string{"complete upstream result", "complete original calltoolresult"} {
		if strings.Contains(plain, forbidden) {
			t.Errorf("README anchor %q contains unqualified promise %q", heading, forbidden)
		}
	}
}

func TestRetainedResultPagingExpiryMidSequence(t *testing.T) {
	harness, sessionID, upstream, now, handleTTL, assembly := newCountedPagingHarness(t)
	preview := invokeCountedCallTool(t, harness, sessionID, assembly)
	metadata := structuredResult(preview)
	handle, ok := metadata["handle"].(string)
	if !ok || handle == "" {
		t.Fatalf("over-ceiling result handle = %#v", metadata["handle"])
	}

	firstPage := retrievePagingTestPage(t, harness, sessionID, handle, 1)
	assertPagingExecutionFacts(t, "call_tool", firstPage)
	pageMetadata := structuredResult(firstPage)
	totalPages, ok := pageMetadata["total_pages"].(float64)
	if !ok || totalPages < 3 {
		t.Fatalf("first page total_pages = %#v, want at least three pages", pageMetadata["total_pages"])
	}
	firstBytes := marshalPagingResult(t, firstPage)
	repeatedPage := retrievePagingTestPage(t, harness, sessionID, handle, 1)
	assertPagingExecutionFacts(t, "call_tool", repeatedPage)
	if repeatedBytes := marshalPagingResult(t, repeatedPage); !bytes.Equal(firstBytes, repeatedBytes) {
		t.Fatalf("re-retrieved page 1 changed bytes:\n got %s\nwant %s", repeatedBytes, firstBytes)
	}
	assertUpstreamInvocationCount(t, upstream, 1)

	*now = now.Add(handleTTL + time.Nanosecond)
	expired := retrievePagingTestPage(t, harness, sessionID, handle, 2)
	assertExpiredRetrievalShape(t, expired)
	assertUpstreamInvocationCount(t, upstream, 1)
}

func TestPerSurfaceHandleRetrievalRoundTrip(t *testing.T) {
	for _, surfaceName := range []string{"call_tool", "direct_invocation", "search_tools", "describe_tool"} {
		t.Run(surfaceName, func(t *testing.T) {
			harness, sessionID, preview, golden := newPagingTestObservation(t, surfaceName, nil)
			if preview == nil || golden == "" {
				t.Fatalf("paging fixture returned preview=%#v and golden length=%d", preview, len(golden))
			}
			handle, ok := structuredResult(preview)["handle"].(string)
			if !ok || handle == "" {
				t.Fatalf("%s preview handle = %#v", surfaceName, structuredResult(preview)["handle"])
			}

			var joined strings.Builder
			var firstPageBytes []byte
			totalPages := 0
			for page := 1; ; page++ {
				result := retrievePagingTestPage(t, harness, sessionID, handle, page)
				assertPagingExecutionFacts(t, surfaceName, result)
				pageMetadata := structuredResult(result)
				if got := pageMetadata["page"]; got != float64(page) {
					t.Fatalf("%s page metadata page=%#v, want %d", surfaceName, got, page)
				}
				if page == 1 {
					totalPageValue, ok := pageMetadata["total_pages"].(float64)
					if !ok {
						t.Fatalf("%s page 1 total_pages = %#v", surfaceName, pageMetadata["total_pages"])
					}
					totalPages = int(totalPageValue)
					firstPageBytes = marshalPagingResult(t, result)
				}
				joined.WriteString(pagingTextContent(t, result))
				if pageMetadata["remaining_pages"] == float64(0) {
					if page != totalPages {
						t.Fatalf("%s completed on page %d, total_pages=%d", surfaceName, page, totalPages)
					}
					break
				}
			}
			if totalPages < 2 {
				t.Fatalf("%s total_pages=%d, want multiple pages", surfaceName, totalPages)
			}
			if got := joined.String(); got != golden {
				t.Fatalf("%s page bytes do not reconstruct its independent golden result", surfaceName)
			}

			repeatedPage := retrievePagingTestPage(t, harness, sessionID, handle, 1)
			assertPagingExecutionFacts(t, surfaceName, repeatedPage)
			if repeatedBytes := marshalPagingResult(t, repeatedPage); !bytes.Equal(firstPageBytes, repeatedBytes) {
				t.Fatalf("%s re-retrieved page 1 changed bytes:\n got %s\nwant %s", surfaceName, repeatedBytes, firstPageBytes)
			}
		})
	}
}

func TestSingleExecutionAcrossFullAndExpiredRetrieval(t *testing.T) {
	t.Run("full multi-page retrieval", func(t *testing.T) {
		harness, sessionID, upstream, _, _, assembly := newCountedPagingHarness(t)
		preview := invokeCountedCallTool(t, harness, sessionID, assembly)
		handle, ok := structuredResult(preview)["handle"].(string)
		if !ok || handle == "" {
			t.Fatalf("over-ceiling result handle = %#v", structuredResult(preview)["handle"])
		}

		var firstPageBytes []byte
		for page := 1; ; page++ {
			result := retrievePagingTestPage(t, harness, sessionID, handle, page)
			assertPagingExecutionFacts(t, "call_tool", result)
			pageMetadata := structuredResult(result)
			if page == 1 {
				firstPageBytes = marshalPagingResult(t, result)
				if totalPages, ok := pageMetadata["total_pages"].(float64); !ok || totalPages < 3 {
					t.Fatalf("page 1 total_pages = %#v, want at least three pages", pageMetadata["total_pages"])
				}
			}
			assertUpstreamInvocationCount(t, upstream, 1)
			if pageMetadata["remaining_pages"] == float64(0) {
				break
			}
		}

		repeated := retrievePagingTestPage(t, harness, sessionID, handle, 1)
		assertPagingExecutionFacts(t, "call_tool", repeated)
		if repeatedBytes := marshalPagingResult(t, repeated); !bytes.Equal(firstPageBytes, repeatedBytes) {
			t.Fatalf("re-retrieved page 1 changed bytes:\n got %s\nwant %s", repeatedBytes, firstPageBytes)
		}
		assertUpstreamInvocationCount(t, upstream, 1)
	})

	t.Run("expiry mid-sequence", func(t *testing.T) {
		harness, sessionID, upstream, now, handleTTL, assembly := newCountedPagingHarness(t)
		preview := invokeCountedCallTool(t, harness, sessionID, assembly)
		handle, ok := structuredResult(preview)["handle"].(string)
		if !ok || handle == "" {
			t.Fatalf("over-ceiling result handle = %#v", structuredResult(preview)["handle"])
		}
		first := retrievePagingTestPage(t, harness, sessionID, handle, 1)
		assertPagingExecutionFacts(t, "call_tool", first)
		if totalPages, ok := structuredResult(first)["total_pages"].(float64); !ok || totalPages < 3 {
			t.Fatalf("page 1 total_pages = %#v, want at least three pages", structuredResult(first)["total_pages"])
		}
		assertUpstreamInvocationCount(t, upstream, 1)

		*now = now.Add(handleTTL + time.Nanosecond)
		assertExpiredRetrievalShape(t, retrievePagingTestPage(t, harness, sessionID, handle, 2))
		assertUpstreamInvocationCount(t, upstream, 1)
	})
}

func assertPagingExecutionFacts(t *testing.T, surfaceName string, result *mcp.CallToolResult) {
	t.Helper()
	wantError := surfaceName == "call_tool" || surfaceName == "direct_invocation"
	metadata := structuredResult(result)
	if result.IsError != wantError || metadata["is_error"] != wantError {
		t.Fatalf("%s page execution facts = isError:%v metadata:%#v, want is_error=%v", surfaceName, result.IsError, metadata, wantError)
	}
	if wantError {
		if metadata["upstream_code"] != "upstream-code" {
			t.Fatalf("%s page upstream_code = %#v, want upstream-code", surfaceName, metadata["upstream_code"])
		}
	} else if _, exists := metadata["upstream_code"]; exists {
		t.Fatalf("%s bridge-generated page unexpectedly includes upstream_code: %#v", surfaceName, metadata)
	}
}

func newCountedPagingHarness(t *testing.T) (*progressiveSessionHarness, string, *directExecutionUpstream, *time.Time, time.Duration, *Assembly) {
	t.Helper()
	text := strings.Repeat("x", 30000)
	upstream := &directExecutionUpstream{
		calls:  make(chan directExecutionCall, 4),
		result: json.RawMessage(fmt.Sprintf(`{"content":[{"type":"text","text":%q}],"structuredContent":{"code":"upstream-code"},"isError":true}`, text)),
	}
	harness, sessionID, _, _, _, assembly := newDirectExecutionFixture(t, upstream)
	configurePagingTestSurface(harness, nil)
	bounds := harness.surface.cfg.RetainedResults()
	now := harness.base
	harness.surface.results = newResultStore(bounds, nil, nil, func() time.Time { return now }, nil)
	return harness, sessionID, upstream, &now, bounds.HandleTTL, assembly
}

func invokeCountedCallTool(t *testing.T, harness *progressiveSessionHarness, sessionID string, assembly *Assembly) *mcp.CallToolResult {
	t.Helper()
	arguments := fmt.Sprintf(`{"tool_name":"alpha__old","catalog_revision":%q,"arguments":{}}`, assembly.CatalogRevision())
	response := harness.send(t, http.MethodPost, sessionID, pagingToolCallBody(catalog.ToolCallTool, arguments), nil)
	return decodePagingToolResult(t, response)
}

func marshalPagingResult(t *testing.T, result *mcp.CallToolResult) []byte {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal tool result: %v", err)
	}
	return encoded
}

func pagingTextContent(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) != 1 {
		t.Fatalf("page content blocks = %d, want one", len(result.Content))
	}
	block, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("page content type = %T, want TextContent", result.Content[0])
	}
	return block.Text
}

func assertExpiredRetrievalShape(t *testing.T, result *mcp.CallToolResult) {
	t.Helper()
	if result == nil || !result.IsError {
		t.Fatalf("expired retrieval = %#v, want an error tool result", result)
	}
	if got := pagingTextContent(t, result); got != "result handle expired" {
		t.Fatalf("expired retrieval text = %q, want fixed expiry message", got)
	}
	metadata := structuredResult(result)
	if len(metadata) != 4 || metadata["code"] != "result_expired" {
		t.Fatalf("expired retrieval structured content = %#v, want only the typed bridge error fields", metadata)
	}
	for _, field := range []string{"handle", "page", "total_pages", "remaining_pages", "remaining_bytes", "is_error", "upstream_code", "paging_required", "total_bytes"} {
		if _, exists := metadata[field]; exists {
			t.Fatalf("expired retrieval unexpectedly includes paging metadata %q: %#v", field, metadata)
		}
	}
}

func assertUpstreamInvocationCount(t *testing.T, upstream *directExecutionUpstream, want int) {
	t.Helper()
	if got := len(upstream.calls); got != want {
		t.Fatalf("upstream invocation count = %d, want %d", got, want)
	}
}
