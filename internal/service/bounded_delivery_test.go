package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

func newDeliveryOracle(t *testing.T) (*resultStore, resultNamespace, resultOwnerResolver, config.RetainedResultBounds) {
	t.Helper()
	bounds := resultStoreBounds()
	bounds.DeliveryCeilingBytes = 512
	bounds.PreviewCeilingBytes = 256
	bounds.MaxRetainedResultBytes = 1 << 20
	bounds.MaxBytesPerNamespace = 1 << 20
	bounds.MaxBytesAggregate = 1 << 20
	bounds.MaxEntriesPerNamespace = 16
	bounds.MaxUpstreamCodeBytes = 24
	now := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)
	store := newResultStore(bounds, observe.NewLogger(nil, nil), observe.NewMetrics(nil), func() time.Time { return now }, nil)
	owner := resultNamespace{publisher: true}
	resolve := func(requested resultNamespace) (resultNamespace, bool) { return requested, requested == owner }
	return store, owner, resolve, bounds
}

func deliveryFixture(text string, isError bool, upstreamCode string) *mcp.CallToolResult {
	metadata := map[string]any{"fixture": strings.Repeat("duplicate", 4)}
	if upstreamCode != "" {
		metadata["code"] = upstreamCode
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: text}},
		StructuredContent: metadata,
		IsError:           isError,
	}
}

func structuredResult(result *mcp.CallToolResult) map[string]any {
	value, _ := result.StructuredContent.(map[string]any)
	return value
}

func equalCallToolResults(a, b *mcp.CallToolResult) bool {
	if a == nil || b == nil {
		return a == b
	}
	return reflect.DeepEqual(a, b)
}

func TestPreviewBoundedAndMetadata(t *testing.T) {
	store, owner, resolve, bounds := newDeliveryOracle(t)
	original := deliveryFixture(strings.Repeat("a", 3000), true, "upstream")
	preview := deliverBoundedResult(store, original, owner, resolve, "upstream")
	if preview == original {
		t.Fatal("over-ceiling result passed through")
	}
	measured, err := measureResult(preview)
	if err != nil || uint64(measured) > bounds.PreviewCeilingBytes {
		t.Fatalf("preview serialized bytes = %d, err=%v; ceiling=%d", measured, err, bounds.PreviewCeilingBytes)
	}
	metadata := structuredResult(preview)
	if len(metadata) != 5 || metadata["paging_required"] != true || metadata["total_bytes"] == nil || metadata["is_error"] != true || metadata["upstream_code"] != "upstream" {
		t.Fatalf("preview metadata = %#v", metadata)
	}
	if _, ok := metadata["handle"].(string); !ok {
		t.Fatalf("preview handle = %#v", metadata["handle"])
	}
	content := preview.Content[0].(*mcp.TextContent).Text
	if content == "" || !utf8.ValidString(content) {
		t.Fatalf("preview content is empty or invalid UTF-8: %q", content)
	}
}

func TestPagesConcatenateToOriginal(t *testing.T) {
	store, owner, resolve, _ := newDeliveryOracle(t)
	original := deliveryFixture(strings.Repeat("page-data-", 500), false, "")
	encoded, err := measureResult(original)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) != encoded {
		t.Fatalf("fixture marshal length=%d, measured=%d", len(payload), encoded)
	}
	preview := deliverBoundedResult(store, original, owner, resolve, "")
	metadata := structuredResult(preview)
	handle := metadata["handle"].(string)
	entry := store.lookup(handle, owner, resolve, 1).entry
	if entry == nil {
		t.Fatal("retained entry missing")
	}
	var joined strings.Builder
	for page := 1; ; page++ {
		result := renderResultPage(entry, page)
		joined.WriteString(result.Content[0].(*mcp.TextContent).Text)
		pageMetadata := structuredResult(result)
		if pageMetadata["page"] != page {
			t.Fatalf("page metadata = %#v", pageMetadata)
		}
		if pageMetadata["remaining_pages"] == 0 {
			break
		}
	}
	if joined.String() != string(payload) {
		t.Fatal("concatenated pages differ from the original serialized result")
	}
}

func TestExecutionFactsPreserved(t *testing.T) {
	store, owner, resolve, _ := newDeliveryOracle(t)
	original := deliveryFixture(strings.Repeat("x", 4000), true, "upstream-code")
	preview := deliverBoundedResult(store, original, owner, resolve, "upstream-code")
	if !preview.IsError || structuredResult(preview)["is_error"] != true || structuredResult(preview)["upstream_code"] != "upstream-code" {
		t.Fatalf("preview execution facts = %#v / isError=%v", structuredResult(preview), preview.IsError)
	}
	entry := store.lookup(structuredResult(preview)["handle"].(string), owner, resolve, 1).entry
	page := renderResultPage(entry, 1)
	if !page.IsError || structuredResult(page)["is_error"] != true || structuredResult(page)["upstream_code"] != "upstream-code" {
		t.Fatalf("page execution facts = %#v / isError=%v", structuredResult(page), page.IsError)
	}
	store.bounds.MaxRetainedResultBytes = 1
	terminal := deliverBoundedResult(store, original, owner, resolve, "upstream-code")
	if !terminal.IsError || structuredResult(terminal)["is_error"] != true || structuredResult(terminal)["upstream_code"] != "upstream-code" {
		t.Fatalf("terminal execution facts = %#v / isError=%v", structuredResult(terminal), terminal.IsError)
	}
}

func TestUpstreamCodePinned(t *testing.T) {
	store, owner, resolve, _ := newDeliveryOracle(t)
	original := deliveryFixture(strings.Repeat("z", 3000), true, "")
	preview := deliverBoundedResult(store, original, owner, resolve, strings.Repeat("x", 80))
	metadata := structuredResult(preview)
	if metadata["upstream_code"] != "xxxxxxxxxxxxx[truncated]" {
		t.Fatalf("pinned upstream code = %#v", metadata["upstream_code"])
	}
	entry := store.lookup(metadata["handle"].(string), owner, resolve, 1).entry
	before := renderResultPage(entry, 1)
	store.bounds.MaxUpstreamCodeBytes = 1000
	store.bounds.DeliveryCeilingBytes = 10000
	store.bounds.PreviewCeilingBytes = 5000
	after := renderResultPage(entry, 1)
	if !equalCallToolResults(before, after) {
		t.Fatal("page changed after retention ceilings or upstream-code limit changed")
	}
}

func TestTerminalPreviewsBounded(t *testing.T) {
	causes := []struct {
		name string
		set  func(*resultStore)
		want string
	}{
		{name: "entry cap", set: func(store *resultStore) { store.bounds.MaxRetainedResultBytes = 1 }, want: "result_capacity_exceeded"},
		{name: "aggregate cap", set: func(store *resultStore) { store.bounds.MaxBytesAggregate = 1 }, want: "result_aggregate_capacity_exceeded"},
		{name: "epoch unresolved", set: func(store *resultStore) { store.bounds.MaxRetainedResultBytes = 1 << 20 }, want: "result_epoch_unresolved"},
	}
	for _, tc := range causes {
		t.Run(tc.name, func(t *testing.T) {
			store, owner, resolve, bounds := newDeliveryOracle(t)
			tc.set(store)
			result := deliveryFixture(strings.Repeat("terminal", 300), true, "fact")
			if tc.want == "result_epoch_unresolved" {
				owner = resultNamespace{sessionID: "session"}
				resolve = func(requested resultNamespace) (resultNamespace, bool) { return requested, false }
			}
			terminal := deliverBoundedResult(store, result, owner, resolve, "fact")
			metadata := structuredResult(terminal)
			if metadata["retention"] != tc.want || metadata["execution_state"] != "executed" || metadata["handle"] != nil || !terminal.IsError {
				t.Fatalf("terminal = %#v / isError=%v, want %s", metadata, terminal.IsError, tc.want)
			}
			measured, err := measureResult(terminal)
			if err != nil || uint64(measured) > bounds.PreviewCeilingBytes {
				t.Fatalf("terminal serialized bytes=%d err=%v ceiling=%d", measured, err, bounds.PreviewCeilingBytes)
			}
			if terminal.Content[0].(*mcp.TextContent).Text == "" {
				t.Fatal("terminal content is empty")
			}
		})
	}
}

func TestAggregateAdmissionHard(t *testing.T) {
	store, owner, resolve, _ := newDeliveryOracle(t)
	store.bounds.MaxBytesAggregate = 1500
	first := deliverBoundedResult(store, deliveryFixture(strings.Repeat("1", 500), false, ""), owner, resolve, "")
	if structuredResult(first)["handle"] == nil {
		t.Fatalf("first result unexpectedly refused: %#v", structuredResult(first))
	}
	second := deliverBoundedResult(store, deliveryFixture(strings.Repeat("2", 1000), true, "code"), owner, resolve, "code")
	if structuredResult(second)["retention"] != "result_aggregate_capacity_exceeded" || structuredResult(second)["handle"] != nil {
		t.Fatalf("aggregate refusal = %#v", structuredResult(second))
	}
	if !second.IsError || structuredResult(second)["is_error"] != true || structuredResult(second)["upstream_code"] != "code" {
		t.Fatalf("aggregate refusal lost execution facts: %#v", structuredResult(second))
	}
}

func TestPageRuneAlignment(t *testing.T) {
	store, owner, resolve, _ := newDeliveryOracle(t)
	fixture := strings.Repeat("界", 1000)
	preview := deliverBoundedResult(store, deliveryFixture(fixture, false, ""), owner, resolve, "")
	entry := store.lookup(structuredResult(preview)["handle"].(string), owner, resolve, 1).entry
	var joined strings.Builder
	for page := 1; ; page++ {
		result := renderResultPage(entry, page)
		text := result.Content[0].(*mcp.TextContent).Text
		if !utf8.ValidString(text) {
			t.Fatalf("page %d contains invalid UTF-8", page)
		}
		joined.WriteString(text)
		if structuredResult(result)["remaining_pages"] == 0 {
			break
		}
	}
	if joined.Len() == 0 {
		t.Fatal("no page content returned")
	}
}

func TestEscapingHeavyPagesWithinCeiling(t *testing.T) {
	store, owner, resolve, bounds := newDeliveryOracle(t)
	fixture := strings.Repeat("\\\"\n\t<>&", 500)
	preview := deliverBoundedResult(store, deliveryFixture(fixture, false, ""), owner, resolve, "")
	entry := store.lookup(structuredResult(preview)["handle"].(string), owner, resolve, 1).entry
	var joined strings.Builder
	for page := 1; ; page++ {
		result := renderResultPage(entry, page)
		measured, err := measureResult(result)
		if err != nil || uint64(measured) > bounds.DeliveryCeilingBytes {
			t.Fatalf("page %d serialized bytes=%d err=%v ceiling=%d", page, measured, err, bounds.DeliveryCeilingBytes)
		}
		joined.WriteString(result.Content[0].(*mcp.TextContent).Text)
		if structuredResult(result)["remaining_pages"] == 0 {
			break
		}
	}
	payload, _ := json.Marshal(deliveryFixture(fixture, false, ""))
	if joined.String() != string(payload) {
		t.Fatal("escaping-heavy pages do not reconstruct the serialized result")
	}
}

func TestCeilingPlusOneBoundary(t *testing.T) {
	store, owner, resolve, bounds := newDeliveryOracle(t)
	result := deliveryFixture(strings.Repeat("x", 3000), false, "")
	store.bounds.DeliveryCeilingBytes = 1
	store.bounds.MaxRetainedResultBytes = uint64(mustMeasuredResult(t, result)) + 1
	store.bounds.MaxBytesPerNamespace = store.bounds.MaxRetainedResultBytes
	store.bounds.MaxBytesAggregate = store.bounds.MaxRetainedResultBytes + resultRecordCharge
	paged := deliverBoundedResult(store, result, owner, resolve, "")
	if structuredResult(paged)["handle"] == nil {
		t.Fatalf("result one byte above the delivery ceiling did not page: %#v", structuredResult(paged))
	}
	if bounds.DeliveryCeilingBytes == 0 {
		t.Fatal("invalid test ceiling")
	}
}

func TestPreviewRuneStraddle(t *testing.T) {
	store, owner, resolve, bounds := newDeliveryOracle(t)
	result := deliveryFixture(strings.Repeat("\\", 1000), false, "")
	preview := deliverBoundedResult(store, result, owner, resolve, "")
	text := preview.Content[0].(*mcp.TextContent).Text
	if text == "" || !utf8.ValidString(text) {
		t.Fatalf("rune-straddling preview = %q", text)
	}
	measured, err := measureResult(preview)
	if err != nil || uint64(measured) > bounds.PreviewCeilingBytes {
		t.Fatalf("preview bytes=%d err=%v ceiling=%d", measured, err, bounds.PreviewCeilingBytes)
	}
}

func TestCeilingsPinnedAcrossReload(t *testing.T) {
	store, owner, resolve, _ := newDeliveryOracle(t)
	result := deliveryFixture(strings.Repeat("reload", 400), false, "")
	preview := deliverBoundedResult(store, result, owner, resolve, "")
	entry := store.lookup(structuredResult(preview)["handle"].(string), owner, resolve, 1).entry
	before := renderResultPage(entry, 1)
	store.bounds.DeliveryCeilingBytes /= 2
	store.bounds.PreviewCeilingBytes /= 2
	store.bounds.MaxUpstreamCodeBytes /= 2
	after := renderResultPage(entry, 1)
	if !equalCallToolResults(before, after) {
		t.Fatal("retained page changed after delivery ceilings were reloaded")
	}
}

func TestRetrievalEvictionAtomicity(t *testing.T) {
	store, owner, resolve, _ := newDeliveryOracle(t)
	result := deliveryFixture(strings.Repeat("race", 500), false, "")
	preview := deliverBoundedResult(store, result, owner, resolve, "")
	handle := structuredResult(preview)["handle"].(string)
	entry := store.lookup(handle, owner, resolve, 1).entry
	if entry == nil {
		t.Fatal("entry missing before page retrieval")
	}
	page := renderResultPage(entry, 1)
	if page == nil || len(page.Content) != 1 {
		t.Fatal("retrieval returned a partial page")
	}
	store.retire(owner)
	if lookup := store.lookup(handle, owner, resolve, 1); lookup.code != "invalid_handle" {
		t.Fatalf("retired lookup code=%q, want invalid_handle", lookup.code)
	}
}

func TestEpochUnresolvedNoHandle(t *testing.T) {
	store, _, _, _ := newDeliveryOracle(t)
	owner := resultNamespace{sessionID: "session-1"}
	resolve := func(resultNamespace) (resultNamespace, bool) { return resultNamespace{}, false }
	terminal := deliverBoundedResult(store, deliveryFixture(strings.Repeat("x", 3000), false, ""), owner, resolve, "")
	if got := structuredResult(terminal)["retention"]; got != "result_epoch_unresolved" {
		t.Fatalf("retention=%v, want result_epoch_unresolved", got)
	}
	if structuredResult(terminal)["handle"] != nil || store.stats().entries != 0 {
		t.Fatal("unresolved epoch issued or retained a handle")
	}
}

func TestTerminalFieldSemantics(t *testing.T) {
	store, owner, resolve, _ := newDeliveryOracle(t)
	store.bounds.MaxRetainedResultBytes = 1
	result := deliveryFixture(strings.Repeat("terminal", 300), true, "code")
	terminal := deliverBoundedResult(store, result, owner, resolve, "code")
	metadata := structuredResult(terminal)
	if len(metadata) != 5 || metadata["retention"] != "result_capacity_exceeded" || metadata["execution_state"] != "executed" || metadata["total_bytes"] == nil || metadata["is_error"] != true || metadata["upstream_code"] != "code" {
		t.Fatalf("terminal fields=%#v", metadata)
	}
	if len(terminal.Content) != 1 || terminal.Content[0].(*mcp.TextContent).Text == "" {
		t.Fatal("terminal content missing")
	}
}

func TestExecutionStateRegistry(t *testing.T) {
	store, owner, resolve, _ := newDeliveryOracle(t)
	codes := []string{"result_capacity_exceeded", "result_aggregate_capacity_exceeded", "result_namespace_retired", "result_entropy_unavailable", "result_epoch_unresolved"}
	for _, code := range codes {
		result := terminalResult("sample", false, 2000, code, "")
		if got := structuredResult(result)["execution_state"]; got != "executed" {
			t.Fatalf("terminal %s execution_state=%v", code, got)
		}
	}
	if result := deliverBoundedResult(store, deliveryFixture(strings.Repeat("x", 3000), false, ""), owner, resolve, ""); structuredResult(result)["handle"] == nil {
		t.Fatal("fixture did not exercise retained-result path")
	}
}

func TestInboundRetirementHooks(t *testing.T) {
	for _, trigger := range []string{"idle-reap", "missing-session", "delete"} {
		t.Run(trigger, func(t *testing.T) {
			now := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)
			bounds := resultStoreBounds()
			bounds.DeliveryCeilingBytes = 512
			bounds.PreviewCeilingBytes = 256
			bounds.MaxRetainedResultBytes = 1 << 20
			bounds.MaxBytesPerNamespace = 1 << 20
			bounds.MaxBytesAggregate = 1 << 20
			bounds.MaxEntriesPerNamespace = 16
			bounds.HandleTTL = time.Hour
			bounds.TombstoneTTL = 15 * time.Minute
			store := newResultStore(bounds, observe.NewLogger(nil, nil), observe.NewMetrics(nil), func() time.Time { return now }, nil)
			surface := &ToolSurface{runtime: NewRuntime(), results: store}
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if trigger == "missing-session" && r.Method == http.MethodGet {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte("session not found"))
					return
				}
				w.WriteHeader(http.StatusOK)
			})
			inbound := testInbound(inner, InboundOptions{
				ToolSurface: surface,
				Now:         func() time.Time { return now },
				SessionIdle: time.Minute,
			})

			owner, resolve := enrollInboundRetirementOwner(t, inbound, surface, "client-live")
			original := deliveryFixture(strings.Repeat("x", 3000), false, "")
			payload, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			preview := deliverBoundedResult(store, original, owner, resolve, "")
			handle, ok := structuredResult(preview)["handle"].(string)
			if !ok || handle == "" {
				t.Fatal("fixture did not commit a live retained result")
			}
			before := store.stats()
			if before.entries != 1 || before.tombstones != 0 {
				t.Fatalf("before retirement = %#v, want one live entry", before)
			}

			fireInboundRetirementTrigger(t, trigger, inbound, "client-live", &now)
			after := store.stats()
			if after.entries != 0 || after.tombstones != 1 || after.markers != 0 {
				t.Fatalf("after retirement = %#v, want no entry and one tombstone", after)
			}
			if after.aggregateBytes > before.aggregateBytes || before.aggregateBytes-after.aggregateBytes != uint64(len(payload)) {
				t.Fatalf("retirement aggregate before=%d after=%d; want exact release of payload bytes %d and no increase", before.aggregateBytes, after.aggregateBytes, len(payload))
			}
			if got := store.lookup(handle, owner, resolve, 1).code; got != "invalid_handle" {
				t.Fatalf("retired handle lookup = %q, want invalid_handle", got)
			}
			assertRetiredInboundCommit(t, store, owner, resolve)

			emptyOwner, _ := enrollInboundRetirementOwner(t, inbound, surface, "client-empty")
			store.mu.Lock()
			emptyState := store.namespaces[emptyOwner]
			store.mu.Unlock()
			if emptyState != nil {
				t.Fatalf("empty namespace already has store state: %#v", emptyState)
			}
			beforeEmpty := store.stats()
			fireInboundRetirementTrigger(t, trigger, inbound, "client-empty", &now)
			afterEmpty := store.stats()
			if afterEmpty.entries != beforeEmpty.entries || afterEmpty.tombstones != beforeEmpty.tombstones ||
				afterEmpty.markers != beforeEmpty.markers || afterEmpty.aggregateBytes != beforeEmpty.aggregateBytes {
				t.Fatalf("zero-effect retirement changed store: before=%#v after=%#v", beforeEmpty, afterEmpty)
			}
			store.mu.Lock()
			emptyState = store.namespaces[emptyOwner]
			store.mu.Unlock()
			if emptyState != nil {
				t.Fatalf("zero-effect retirement persisted namespace state: %#v", emptyState)
			}

			now = now.Add(bounds.TombstoneTTL)
			store.sweep()
			cleared := store.stats()
			if cleared.tombstones != 0 || cleared.aggregateBytes != after.aggregateBytes-resultRecordCharge {
				t.Fatalf("tombstone clearing state = %#v; after retirement = %#v", cleared, after)
			}
			assertRetiredInboundCommit(t, store, owner, resolve)
		})
	}
}

func enrollInboundRetirementOwner(t *testing.T, inbound *InboundHandler, surface *ToolSurface, clientSessionID string) (resultNamespace, resultOwnerResolver) {
	t.Helper()
	response := serve(inbound, requestFor(http.MethodPost, clientSessionID, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	if response.Code != http.StatusOK {
		t.Fatalf("session enrollment status = %d, want %d", response.Code, http.StatusOK)
	}
	inbound.mu.Lock()
	record, exists := inbound.sessions[clientSessionID]
	sdkSessionID := inbound.sdkSessions[clientSessionID]
	inbound.mu.Unlock()
	if !exists || record.epoch == 0 || sdkSessionID == "" || sdkSessionID == clientSessionID {
		t.Fatalf("enrolled session exists=%t epoch=%d SDK session=%q; want a mapped, nonzero-epoch owner", exists, record.epoch, sdkSessionID)
	}

	epoch := strconv.FormatUint(record.epoch, 10)
	headers := make(http.Header)
	headers.Set(sessionIDHeader, sdkSessionID)
	headers.Set(exposureEpochHeaderName, epoch)
	request := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{Header: headers}}
	owner, resolve := surface.resultOwner(request, true)
	if want := (resultNamespace{sessionID: sdkSessionID, epoch: epoch}); owner != want {
		t.Fatalf("result owner = %#v, want %#v", owner, want)
	}
	return owner, resolve
}

func fireInboundRetirementTrigger(t *testing.T, trigger string, inbound *InboundHandler, clientSessionID string, now *time.Time) {
	t.Helper()
	switch trigger {
	case "idle-reap":
		*now = now.Add(2 * time.Minute)
		_ = serve(inbound, requestFor(http.MethodGet, clientSessionID, ""))
	case "missing-session":
		_ = serve(inbound, requestFor(http.MethodGet, clientSessionID, ""))
	case "delete":
		_ = serve(inbound, requestFor(http.MethodDelete, clientSessionID, ""))
	default:
		t.Fatalf("unknown retirement trigger %q", trigger)
	}
}

func assertRetiredInboundCommit(t *testing.T, store *resultStore, owner resultNamespace, resolve resultOwnerResolver) {
	t.Helper()
	before := store.stats()
	terminal := deliverBoundedResult(store, deliveryFixture(strings.Repeat("late", 1000), false, ""), owner, resolve, "")
	metadata := structuredResult(terminal)
	if metadata["retention"] != "result_namespace_retired" || metadata["handle"] != nil {
		t.Fatalf("late commit metadata = %#v, want retired terminal without handle", metadata)
	}
	if after := store.stats(); after != before {
		t.Fatalf("late retired commit changed store: before=%#v after=%#v", before, after)
	}
}

func mustMeasuredResult(t *testing.T, result *mcp.CallToolResult) int {
	t.Helper()
	measured, err := measureResult(result)
	if err != nil {
		t.Fatal(err)
	}
	return measured
}
func TestSmallResultPassThroughByteIdentical(t *testing.T) {
	fixtures := []struct {
		surface string
		golden  string
		length  int
	}{
		{
			surface: "call_tool",
			golden:  `{"content":[{"type":"text","text":"call_tool"}],"structuredContent":{"surface":"call_tool"}}`,
			length:  92,
		},
		{
			surface: "direct_invocation",
			golden:  `{"content":[{"type":"text","text":"direct"}],"structuredContent":{"surface":"direct_invocation"}}`,
			length:  97,
		},
		{
			surface: "search_tools",
			golden:  `{"content":[{"type":"text","text":"{\"source\":\"search_tools\"}"}],"structuredContent":{"surface":"search_tools"}}`,
			length:  115,
		},
		{
			surface: "describe_tool",
			golden:  `{"content":[{"type":"text","text":"{\"source\":\"describe_tool\"}"}],"structuredContent":{"surface":"describe_tool"}}`,
			length:  117,
		},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.surface, func(t *testing.T) {
			if len(fixture.golden) != fixture.length {
				t.Fatalf("literal golden length = %d, want %d", len(fixture.golden), fixture.length)
			}
			store, owner, resolve, bounds := newDeliveryOracle(t)
			bounds.DeliveryCeilingBytes = 8192
			store.bounds = bounds
			var original mcp.CallToolResult
			if err := json.Unmarshal([]byte(fixture.golden), &original); err != nil {
				t.Fatalf("decode literal result: %v", err)
			}
			delivered := deliverBoundedResult(store, &original, owner, resolve, "")
			if delivered != &original {
				t.Fatal("small result was not passed through unchanged")
			}
			encoded, err := json.Marshal(delivered)
			if err != nil {
				t.Fatalf("marshal delivered small result: %v", err)
			}
			if string(encoded) != fixture.golden {
				t.Fatalf("small result bytes changed:\n got %s\nwant %s", encoded, fixture.golden)
			}

			prefix := `{"content":[{"type":"text","text":"`
			suffix := fmt.Sprintf(`"}],"structuredContent":{"surface":%q}}`, fixture.surface)
			atCeilingGolden := prefix + strings.Repeat("x", 8192-len(prefix)-len(suffix)) + suffix
			if len(atCeilingGolden) != 8192 {
				t.Fatalf("exact-ceiling literal length = %d, want 8192", len(atCeilingGolden))
			}
			var atCeiling mcp.CallToolResult
			if err := json.Unmarshal([]byte(atCeilingGolden), &atCeiling); err != nil {
				t.Fatalf("decode exact-ceiling result: %v", err)
			}
			atCeilingDelivered := deliverBoundedResult(store, &atCeiling, owner, resolve, "")
			if atCeilingDelivered != &atCeiling {
				t.Fatal("exact-ceiling result was not passed through unchanged")
			}
			encoded, err = json.Marshal(atCeilingDelivered)
			if err != nil {
				t.Fatalf("marshal exact-ceiling result: %v", err)
			}
			if string(encoded) != atCeilingGolden {
				t.Fatal("exact-ceiling result bytes changed")
			}
		})
	}
}

func TestLogLabelProducers(t *testing.T) {
	for _, surfaceName := range []string{"call_tool", "direct_invocation", "search_tools", "describe_tool"} {
		t.Run(surfaceName, func(t *testing.T) {
			var logs bytes.Buffer
			logger := observe.NewLogger(&logs, &logs)
			logPagingEvent(logger, "info", "result_paged", "call_tool", "success", nil)
			logPagingEvent(logger, "info", "result_page_served", "retrieve_result", "success", nil)

			labels := map[string]string{}
			for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
				var record struct {
					Event     string `json:"event"`
					Operation string `json:"operation"`
				}
				if err := json.Unmarshal([]byte(line), &record); err != nil {
					t.Fatalf("decode paging label record %q: %v", line, err)
				}
				labels[record.Event] = record.Operation
			}
			if labels["result_paged"] != "call_tool" {
				t.Fatalf("paging label = (%q,result_paged), want (call_tool,result_paged)", labels["result_paged"])
			}
			if labels["result_page_served"] != "retrieve_result" {
				t.Fatalf("retrieval label = (%q,result_page_served), want (retrieve_result,result_page_served)", labels["result_page_served"])
			}
		})
	}
}

func TestPagingMetricsDeltas(t *testing.T) {
	metrics := observe.NewMetrics(nil)
	observe.BindLogOperationViolationCounter(metrics.RecordLogOperationViolation)
	defer observe.BindLogOperationViolationCounter(nil)

	counterSeries := []string{
		"bridge_results_paged_total",
		"bridge_result_pages_served_total",
		`bridge_retention_rejections_total{retention="result_capacity_exceeded"}`,
		`bridge_retention_rejections_total{retention="result_aggregate_capacity_exceeded"}`,
		`bridge_retention_rejections_total{retention="result_namespace_retired"}`,
		`bridge_retention_rejections_total{retention="result_entropy_unavailable"}`,
		`bridge_retention_rejections_total{retention="result_epoch_unresolved"}`,
		"bridge_retention_evictions_total",
		"bridge_log_operation_violations_total",
	}
	scrape := func() map[string]float64 {
		recorder := httptest.NewRecorder()
		metrics.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("metrics scrape status = %d, want %d", recorder.Code, http.StatusOK)
		}
		values := make(map[string]float64, len(counterSeries))
		for _, series := range counterSeries {
			values[series] = 0
		}
		for _, line := range strings.Split(recorder.Body.String(), "\n") {
			fields := strings.Fields(line)
			if len(fields) != 2 {
				continue
			}
			if _, tracked := values[fields[0]]; !tracked {
				continue
			}
			value, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				t.Fatalf("parse counter sample %q: %v", line, err)
			}
			values[fields[0]] = value
		}
		return values
	}
	assertDeltas := func(before, after map[string]float64, expected map[string]float64) {
		t.Helper()
		for _, series := range counterSeries {
			want := before[series] + expected[series]
			if got := after[series]; got != want {
				t.Errorf("counter %s = %v after delta, want %v", series, got, want)
			}
		}
	}

	store, owner, resolve, _ := newDeliveryOracle(t)
	store.metrics = metrics
	before := scrape()
	preview := deliverBoundedResult(store, deliveryFixture(strings.Repeat("page", 300), false, ""), owner, resolve, "")
	handle, ok := structuredResult(preview)["handle"].(string)
	if !ok || handle == "" {
		t.Fatalf("over-ceiling delivery did not issue a handle: %#v", structuredResult(preview))
	}
	after := scrape()
	assertDeltas(before, after, map[string]float64{"bridge_results_paged_total": 1})

	surface := &ToolSurface{results: store, cfg: ToolSurfaceConfig{Metrics: metrics}}
	before = scrape()
	page := retrieveForOracle(t, surface, `{"handle":"`+handle+`","page":1}`)
	if page.IsError || structuredResult(page)["page"] != 1 {
		t.Fatalf("page retrieval = %#v, want successful page 1", page)
	}
	after = scrape()
	assertDeltas(before, after, map[string]float64{"bridge_result_pages_served_total": 1})

	refusalStore, refusalOwner, refusalResolve, refusalBounds := newDeliveryOracle(t)
	refusalStore.metrics = metrics
	refusalBounds.MaxRetainedResultBytes = 1
	refusalStore.bounds = refusalBounds
	before = scrape()
	terminal := deliverBoundedResult(refusalStore, deliveryFixture(strings.Repeat("refusal", 300), true, "code"), refusalOwner, refusalResolve, "code")
	if structuredResult(terminal)["retention"] != "result_capacity_exceeded" {
		t.Fatalf("retention terminal = %#v, want result_capacity_exceeded", structuredResult(terminal))
	}
	after = scrape()
	assertDeltas(before, after, map[string]float64{`bridge_retention_rejections_total{retention="result_capacity_exceeded"}`: 1})

	evictionStore, evictionOwner, evictionResolve, evictionBounds := newDeliveryOracle(t)
	evictionStore.metrics = metrics
	evictionBounds.MaxEntriesPerNamespace = 1
	evictionStore.bounds = evictionBounds
	before = scrape()
	first := deliverBoundedResult(evictionStore, deliveryFixture(strings.Repeat("first", 150), false, ""), evictionOwner, evictionResolve, "")
	if structuredResult(first)["handle"] == nil {
		t.Fatalf("first FIFO fixture was not retained: %#v", structuredResult(first))
	}
	after = scrape()
	assertDeltas(before, after, map[string]float64{"bridge_results_paged_total": 1})
	before = scrape()
	second := deliverBoundedResult(evictionStore, deliveryFixture(strings.Repeat("second", 150), false, ""), evictionOwner, evictionResolve, "")
	if structuredResult(second)["handle"] == nil {
		t.Fatalf("second FIFO fixture was not retained: %#v", structuredResult(second))
	}
	after = scrape()
	assertDeltas(before, after, map[string]float64{
		"bridge_results_paged_total":      1,
		"bridge_retention_evictions_total": 1,
	})

	logger := observe.NewLogger(io.Discard, io.Discard)
	logBefore := scrape()
	logger.Log(observe.Entry{Level: "info", Event: "result_paged", Operation: "unlisted_operation", Outcome: "success"})
	after = scrape()
	assertDeltas(logBefore, after, map[string]float64{"bridge_log_operation_violations_total": 1})

	before = scrape()
	if err := observe.WriteJSONLine(io.Discard, observe.NewEnvelope("info", "result_paged", "ignored", "unlisted_operation", "success")); err != nil {
		t.Fatalf("write rejected log record: %v", err)
	}
	after = scrape()
	assertDeltas(before, after, map[string]float64{"bridge_log_operation_violations_total": 1})
	assertDeltas(logBefore, after, map[string]float64{"bridge_log_operation_violations_total": 2})

	before = scrape()
	logger.Log(observe.Entry{Level: "info", Event: "result_paged", Operation: "call_tool", Outcome: "success"})
	after = scrape()
	assertDeltas(before, after, map[string]float64{})
}
