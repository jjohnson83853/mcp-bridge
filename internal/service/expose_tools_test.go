package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/search"
)

type exposeToolsHarness struct {
	runtime   *Runtime
	surface   *ToolSurface
	http      *httptest.Server
	client    *mcp.ClientSession
	upstream  *progressiveTestUpstream
}

type exposeToolsPayload struct {
	Exposed        []string `json:"exposed"`
	Declared       []string `json:"declared"`
	CatalogRevision string  `json:"catalog_revision"`
}

type exposeToolsState struct {
	projections map[projectionOwner]progressiveProjectionSnapshot
	union       map[string]progressiveUnionEntrySnapshot
	tools       map[string][]byte
}

func newExposeToolsHarness(t *testing.T, logger *observe.Logger) *exposeToolsHarness {
	t.Helper()
	upstream := &progressiveTestUpstream{}
	runtime := NewRuntime()
	assembly, err := runtime.Reload(progressiveTestConfig(t, "old"), progressiveTestDeps(upstream))
	if err != nil {
		t.Fatalf("admit expose_tools test generation: %v", err)
	}
	server := NewMCPServer()
	surface, err := NewToolSurface(runtime, ToolSurfaceConfig{
		Model:  search.ModelIdentity{Provider: "test", Model: "expose-tools", Dimensions: 1, FormatVersion: "v1"},
		Logger: logger,
	})
	if err != nil {
		t.Fatalf("NewToolSurface: %v", err)
	}
	if err := surface.Register(server); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := surface.Published(assembly); err != nil {
		t.Fatalf("publish expose_tools test generation: %v", err)
	}
	token := "client-token"
	inbound := NewInboundHandler(
		NewStreamableHandler(server),
		[][32]byte{sha256.Sum256([]byte(token))},
		InboundOptions{ToolSurface: surface},
	)
	httpServer := httptest.NewServer(inbound)
	t.Cleanup(httpServer.Close)
	client := connectProgressiveDiscoveryClient(t, httpServer, token)
	snapshotProgressiveClientTools(t, client)
	return &exposeToolsHarness{
		runtime:  runtime,
		surface:  surface,
		http:     httpServer,
		client:   client,
		upstream: upstream,
	}
}

func encodeExposeToolsArguments(t *testing.T, names []string, revision *string) json.RawMessage {
	t.Helper()
	arguments := map[string]any{"tool_names": names}
	if revision != nil {
		arguments["catalog_revision"] = *revision
	}
	raw, err := json.Marshal(arguments)
	if err != nil {
		t.Fatalf("marshal expose_tools arguments: %v", err)
	}
	return raw
}

func callExposeTools(t *testing.T, client *mcp.ClientSession, names []string, revision *string) (*mcp.CallToolResult, error) {
	t.Helper()
	return client.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      catalog.ToolExposeTools,
		Arguments: encodeExposeToolsArguments(t, names, revision),
	})

}
func mustCallExposeTools(t *testing.T, client *mcp.ClientSession, names []string, revision *string) *mcp.CallToolResult {
	t.Helper()
	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      catalog.ToolExposeTools,
		Arguments: encodeExposeToolsArguments(t, names, revision),
	})
	if err != nil {
		t.Fatalf("expose_tools: %v", err)
	}
	if result == nil {
		t.Fatal("expose_tools returned a nil result")
	}
	return result
}

func exposeToolsRequest(arguments json.RawMessage) *mcp.CallToolRequest {
	return &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: arguments}}
}

func decodeExposeToolsPayload(t *testing.T, result *mcp.CallToolResult) exposeToolsPayload {
	t.Helper()
	if result == nil || len(result.Content) == 0 {
		t.Fatalf("expose_tools result has no content: %#v", result)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expose_tools content type = %T; want text", result.Content[0])
	}
	var payload exposeToolsPayload
	if err := json.Unmarshal([]byte(text.Text), &payload); err != nil {
		t.Fatalf("decode expose_tools payload %q: %v", text.Text, err)
	}
	return payload
}

func assertExposeToolsError(t *testing.T, result *mcp.CallToolResult, code string) {
	t.Helper()
	if result == nil || !result.IsError {
		t.Fatalf("expose_tools result = %#v; want tool error %q", result, code)
	}
	if got := structuredResult(result)["code"]; got != code {
		t.Fatalf("expose_tools error code = %v; want %q", got, code)
	}
}

func snapshotExposeToolsState(t *testing.T, harness *exposeToolsHarness, client *mcp.ClientSession) exposeToolsState {
	t.Helper()
	projections, union := snapshotProgressiveCoordinatorState(t, harness.surface.projectionCoordinator())
	return exposeToolsState{
		projections: projections,
		union:       union,
		tools:       snapshotProgressiveClientTools(t, client),
	}
}

func assertExposeToolsStateEqual(t *testing.T, got, want exposeToolsState) {
	t.Helper()
	if !reflect.DeepEqual(got.projections, want.projections) {
		t.Error("expose_tools changed a projection or interest set")
	}
	if !reflect.DeepEqual(got.union, want.union) {
		t.Error("expose_tools changed the projection union")
	}
	if !reflect.DeepEqual(got.tools, want.tools) {
		t.Error("expose_tools changed tools/list")
	}
}

func snapshotExposePublisherTools(t *testing.T, surface *ToolSurface) map[string][]byte {
	t.Helper()
	coordinator := surface.projectionCoordinator()
	coordinator.barrier.Lock()
	projection := coordinator.projections[surface.publisherOwner]
	coordinator.barrier.Unlock()
	if projection == nil {
		t.Fatal("publisher projection is missing")
	}
	page, err := surface.ownerToolListPage(projection, "")
	if err != nil {
		t.Fatalf("publisher tools/list: %v", err)
	}
	tools := make(map[string][]byte, len(page.Tools))
	for _, tool := range page.Tools {
		encoded, err := json.Marshal(tool)
		if err != nil {
			t.Fatalf("marshal publisher tool %q: %v", tool.Name, err)
		}
		tools[tool.Name] = encoded
	}
	return tools
}

func captureExposeRegistrySignals(t *testing.T, surface *ToolSurface) <-chan struct{} {
	t.Helper()
	coordinator := surface.projectionCoordinator()
	signals := make(chan struct{}, 8)
	coordinator.testBeforeRegistryAdd = func() {
		select {
		case signals <- struct{}{}:
		default:
		}
	}
	t.Cleanup(func() { coordinator.testBeforeRegistryAdd = nil })
	return signals
}

func assertNoExposeRegistrySignal(t *testing.T, signals <-chan struct{}) {
	t.Helper()
	select {
	case <-signals:
		t.Error("no-op or rejected expose_tools request signaled an SDK registry addition")
	default:
	}
}

func TestExposeToolsSuccess(t *testing.T) {
	harness := newExposeToolsHarness(t, nil)
	pinned := config.WireToolName("alpha", "old")
	unpinned := effectiveWireName("alpha", "new")
	alias := core.PublicName("alpha", "new")
	emptyRevision := ""

	result := mustCallExposeTools(t, harness.client, []string{pinned, unpinned, alias}, &emptyRevision)
	if result.IsError {
		t.Fatalf("expose_tools failed: %v", structuredResult(result))
	}
	payload := decodeExposeToolsPayload(t, result)
	if !reflect.DeepEqual(payload.Declared, []string{unpinned, pinned}) {
		t.Errorf("declared = %q; want sorted distinct projected names [%q %q]", payload.Declared, unpinned, pinned)
	}
	if !reflect.DeepEqual(payload.Exposed, []string{unpinned}) {
		t.Errorf("exposed = %q; want only newly declared unpinned target %q", payload.Exposed, unpinned)
	}
	if payload.CatalogRevision != harness.runtime.Current().CatalogRevision() {
		t.Errorf("catalog_revision = %q; want %q", payload.CatalogRevision, harness.runtime.Current().CatalogRevision())
	}

	listed := snapshotProgressiveClientTools(t, harness.client)
	if _, found := listed[pinned]; !found {
		t.Fatalf("configured pin %q disappeared from tools/list", pinned)
	}
	if _, found := listed[unpinned]; !found {
		t.Fatalf("explicitly declared tool %q is missing from tools/list", unpinned)
	}
	direct, err := harness.client.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      unpinned,
		Arguments: json.RawMessage(`{}`),
	})
	if err != nil || direct == nil || direct.IsError {
		t.Fatalf("direct call after expose_tools failed: result=%#v err=%v", direct, err)
	}
}

func TestExposeToolsBoundRejected(t *testing.T) {
	harness := newExposeToolsHarness(t, nil)
	before := snapshotExposeToolsState(t, harness, harness.client)
	signals := captureExposeRegistrySignals(t, harness.surface)
	nineNames := make([]string, 9)
	for i := range nineNames {
		nineNames[i] = fmt.Sprintf("alpha__tool%d", i)
	}
	cases := []struct {
		name  string
		names []string
	}{
		{name: "empty", names: []string{}},
		{name: "over bound", names: nineNames},
		{name: "identical duplicate", names: []string{"alpha__new", "alpha__new"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := mustCallExposeTools(t, harness.client, tc.names, nil)
			assertExposeToolsError(t, result, "invalid_arguments")
			assertExposeToolsStateEqual(t, snapshotExposeToolsState(t, harness, harness.client), before)
			assertNoExposeRegistrySignal(t, signals)
		})
	}
}

func TestExposeToolsDeterministicErrors(t *testing.T) {
	harness := newExposeToolsHarness(t, nil)
	signals := captureExposeRegistrySignals(t, harness.surface)
	beforeUnknown := snapshotExposeToolsState(t, harness, harness.client)
	unknown := mustCallExposeTools(t, harness.client, []string{"alpha__new", "alpha__missing"}, nil)
	assertExposeToolsError(t, unknown, "unknown_tool")
	assertExposeToolsStateEqual(t, snapshotExposeToolsState(t, harness, harness.client), beforeUnknown)
	assertNoExposeRegistrySignal(t, signals)

	first := mustCallExposeTools(t, harness.client, []string{"alpha__new"}, nil)
	if first.IsError {
		t.Fatalf("first expose_tools call failed: %v", structuredResult(first))
	}
	select {
	case <-signals:
	default:
		t.Fatal("first exposure did not signal an SDK registry addition")
	}
	afterFirst := snapshotExposeToolsState(t, harness, harness.client)

	repeated := mustCallExposeTools(t, harness.client, []string{"alpha__new"}, nil)
	if repeated.IsError {
		t.Fatalf("repeated expose_tools call failed: %v", structuredResult(repeated))
	}
	repeatedPayload := decodeExposeToolsPayload(t, repeated)
	if len(repeatedPayload.Exposed) != 0 {
		t.Errorf("repeat exposed = %q; want empty because target was already declared", repeatedPayload.Exposed)
	}
	assertExposeToolsStateEqual(t, snapshotExposeToolsState(t, harness, harness.client), afterFirst)
	assertNoExposeRegistrySignal(t, signals)

	pinned := config.WireToolName("alpha", "old")
	allPinned := mustCallExposeTools(t, harness.client, []string{pinned}, nil)
	if allPinned.IsError {
		t.Fatalf("all-pinned expose_tools call failed: %v", structuredResult(allPinned))
	}
	pinnedPayload := decodeExposeToolsPayload(t, allPinned)
	if !reflect.DeepEqual(pinnedPayload.Declared, []string{pinned}) || len(pinnedPayload.Exposed) != 0 {
		t.Errorf("all-pinned payload = %+v; want declared [%q] and no exposed names", pinnedPayload, pinned)
	}
	assertExposeToolsStateEqual(t, snapshotExposeToolsState(t, harness, harness.client), afterFirst)
	assertNoExposeRegistrySignal(t, signals)
}

func TestExposeToolsMalformedInputs(t *testing.T) {
	harness := newExposeToolsHarness(t, nil)
	revision := harness.runtime.Current().CatalogRevision()
	staleRevision := strings.Repeat("0", 64)
	if staleRevision == revision {
		staleRevision = strings.Repeat("1", 64)
	}
	invalidRevision := "not-a-catalog-digest"
	before := snapshotExposeToolsState(t, harness, harness.client)
	signals := captureExposeRegistrySignals(t, harness.surface)
	cases := []struct {
		name     string
		names    []string
		revision *string
		want     string
	}{
		{name: "malformed name with accepted revision", names: []string{"invalid name"}, revision: &revision, want: "invalid_arguments"},
		{name: "malformed revision precedes malformed name", names: []string{"invalid name"}, revision: &invalidRevision, want: "invalid_arguments"},
		{name: "stale revision precedes malformed name", names: []string{"invalid name"}, revision: &staleRevision, want: "catalog_changed"},
		{name: "stale revision precedes unknown name", names: []string{"alpha__missing"}, revision: &staleRevision, want: "catalog_changed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := mustCallExposeTools(t, harness.client, tc.names, tc.revision)
			assertExposeToolsError(t, result, tc.want)
			assertExposeToolsStateEqual(t, snapshotExposeToolsState(t, harness, harness.client), before)
			assertNoExposeRegistrySignal(t, signals)
		})
	}
}

func TestExposeToolsPublisherAndRetiredEpoch(t *testing.T) {
	harness := newExposeToolsHarness(t, nil)
	arguments := encodeExposeToolsArguments(t, []string{"alpha__new"}, nil)
	publisherResult, err := harness.surface.exposeTools(context.Background(), exposeToolsRequest(arguments))
	if err != nil {
		t.Fatalf("publisher expose_tools returned protocol error: %v", err)
	}
	assertExposeToolsError(t, publisherResult, "catalog_changed")

	coordinator := harness.surface.projectionCoordinator()
	coordinator.barrier.Lock()
	var owner projectionOwner
	foundOwner := false
	for candidate := range coordinator.liveOwners {
		if candidate.kind == projectionSession {
			if foundOwner {
				coordinator.barrier.Unlock()
				t.Fatal("expected one live session owner")
			}
			owner, foundOwner = candidate, true
		}
	}
	coordinator.barrier.Unlock()
	if !foundOwner {
		t.Fatal("session owner was not admitted")
	}
	if !harness.surface.retireLiveOwner(owner) {
		t.Fatal("failed to retire the session owner")
	}
	_, err = callExposeTools(t, harness.client, []string{"alpha__new"}, nil)
	var rpcErr *jsonrpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != -32600 || rpcErr.Message != "invalid exposure request" {
		t.Fatalf("retired-epoch expose_tools error = %T %v; want JSON-RPC -32600 invalid exposure request", err, err)
	}
}

func TestExposeSessionIsolation(t *testing.T) {
	harness := newExposeToolsHarness(t, nil)
	clientB := connectProgressiveDiscoveryClient(t, harness.http, "client-token")
	beforeA := snapshotProgressiveClientTools(t, harness.client)
	beforeB := snapshotProgressiveClientTools(t, clientB)
	result := mustCallExposeTools(t, harness.client, []string{"alpha__new"}, nil)
	if result.IsError {
		t.Fatalf("session A expose_tools failed: %v", structuredResult(result))
	}
	afterA := snapshotProgressiveClientTools(t, harness.client)
	if _, found := afterA["alpha__new"]; !found {
		t.Fatal("session A tools/list omitted explicitly exposed alpha__new")
	}
	if _, found := beforeA["alpha__new"]; found {
		t.Fatal("session A unexpectedly listed alpha__new before exposure")
	}
	afterB := snapshotProgressiveClientTools(t, clientB)
	if !reflect.DeepEqual(afterB, beforeB) {
		t.Error("session A exposure changed session B tools/list")
	}
	_, err := clientB.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "alpha__new",
		Arguments: json.RawMessage(`{}`),
	})
	var rpcErr *jsonrpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != -32600 || rpcErr.Message != "invalid exposure request" {
	}
	if calls := len(harness.upstream.callsSnapshot()); calls != 0 {
		t.Errorf("isolated exposure dispatched %d upstream calls; want 0", calls)
	}
}

func TestDescribeDoesNotExpose(t *testing.T) {
	harness := newExposeToolsHarness(t, nil)
	before := snapshotProgressiveClientTools(t, harness.client)
	revision := harness.runtime.Current().CatalogRevision()
	arguments, err := json.Marshal(map[string]any{
		"tool_name":       "alpha__new",
		"catalog_revision": revision,
	})
	if err != nil {
		t.Fatal(err)
	}
	described, err := harness.client.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      catalog.ToolDescribeTool,
		Arguments: json.RawMessage(arguments),
	})
	if err != nil || described == nil || described.IsError {
		t.Fatalf("describe_tool failed: result=%#v err=%v", described, err)
	}
	after := snapshotProgressiveClientTools(t, harness.client)
	if !reflect.DeepEqual(after, before) {
		t.Error("describe_tool changed the session tools/list projection")
	}
	direct, err := harness.client.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "alpha__new",
		Arguments: json.RawMessage(`{}`),
	})
	if err == nil && (direct == nil || !direct.IsError) {
		t.Fatal("direct call to a described but unexposed tool unexpectedly succeeded")
	}
	if calls := len(harness.upstream.callsSnapshot()); calls != 0 {
		t.Errorf("describe-only flow dispatched %d upstream calls; want 0", calls)
	}
}

func TestPinsPreservedWithoutSearchFanout(t *testing.T) {
	harness := newExposeToolsHarness(t, nil)
	pinned := config.WireToolName("alpha", "old")
	sessionBefore := snapshotProgressiveClientTools(t, harness.client)
	publisherBefore := snapshotExposePublisherTools(t, harness.surface)
	pinnedSessionDescriptor, found := sessionBefore[pinned]
	if !found {
		t.Fatalf("session tools/list omitted pin %q before operations", pinned)
	}
	pinnedPublisherDescriptor, found := publisherBefore[pinned]
	if !found {
		t.Fatalf("publisher tools/list omitted pin %q before operations", pinned)
	}

	searchResult, err := harness.client.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      catalog.ToolSearchTools,
		Arguments: json.RawMessage(`{"query":"new"}`),
	})
	if err != nil || searchResult == nil || searchResult.IsError {
		t.Fatalf("search_tools failed: result=%#v err=%v", searchResult, err)
	}
	sessionAfterSearch := snapshotProgressiveClientTools(t, harness.client)
	publisherAfterSearch := snapshotExposePublisherTools(t, harness.surface)
	if !reflect.DeepEqual(sessionAfterSearch, sessionBefore) {
		t.Error("search_tools changed session tools/list")
	}
	if !reflect.DeepEqual(publisherAfterSearch, publisherBefore) {
		t.Error("search_tools changed publisher tools/list")
	}

	exposed := mustCallExposeTools(t, harness.client, []string{pinned, "alpha__new"}, nil)
	if exposed.IsError {
		t.Fatalf("expose_tools failed: %v", structuredResult(exposed))
	}
	sessionAfterExpose := snapshotProgressiveClientTools(t, harness.client)
	publisherAfterExpose := snapshotExposePublisherTools(t, harness.surface)
	if !reflect.DeepEqual(publisherAfterExpose, publisherBefore) {
		t.Error("session expose_tools changed publisher tools/list")
	}
	if !bytes.Equal(sessionAfterExpose[pinned], pinnedSessionDescriptor) {
		t.Error("session pin descriptor changed after search/expose")
	}
	if !bytes.Equal(publisherAfterExpose[pinned], pinnedPublisherDescriptor) {
		t.Error("publisher pin descriptor changed after search/expose")
	}
	if _, found := sessionAfterExpose["alpha__new"]; !found {
		t.Error("explicitly exposed tool alpha__new is missing from session tools/list")
	}
	if _, found := publisherAfterExpose["alpha__new"]; found {
		t.Error("session exposure added alpha__new to publisher tools/list")
	}
}

func TestExposeToolsLogs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	harness := newExposeToolsHarness(t, observe.NewLogger(&stdout, &stderr))
	success := mustCallExposeTools(t, harness.client, []string{"alpha__new"}, nil)
	if success.IsError {
		t.Fatalf("expose_tools success call failed: %v", structuredResult(success))
	}
	unknown := mustCallExposeTools(t, harness.client, []string{"alpha__missing"}, nil)
	assertExposeToolsError(t, unknown, "unknown_tool")
	invalidRevision := "not-a-digest"
	invalid := mustCallExposeTools(t, harness.client, []string{"alpha__new"}, &invalidRevision)
	assertExposeToolsError(t, invalid, "invalid_arguments")
	staleRevision := strings.Repeat("0", 64)
	if staleRevision == harness.runtime.Current().CatalogRevision() {
		staleRevision = strings.Repeat("1", 64)
	}
	stale := mustCallExposeTools(t, harness.client, []string{"alpha__new"}, &staleRevision)
	assertExposeToolsError(t, stale, "catalog_changed")
	malformedName := mustCallExposeTools(t, harness.client, []string{"invalid name"}, nil)
	assertExposeToolsError(t, malformedName, "invalid_arguments")

	successEntries := decodeExposeLogLines(t, stdout.String())
	if len(successEntries) != 1 {
		t.Fatalf("stdout has %d expose_tools log entries; want one: %s", len(successEntries), stdout.String())
	}
	entry := successEntries[0]
	if entry["event"] != "mcp_complete" || entry["operation"] != catalog.ToolExposeTools || entry["outcome"] != "success" || entry["level"] != "info" {
		t.Errorf("success log entry = %v; want info mcp_complete expose_tools success", entry)
	}
	assertExposeToolsLogEnvelope(t, entry)
	if entry["mcp_method"] != "tools_call" {
		t.Errorf("success mcp_method = %v; want tools_call", entry["mcp_method"])
	}

	failureEntries := decodeExposeLogLines(t, stderr.String())
	wantCodes := []string{"unknown_tool", "invalid_arguments", "catalog_changed"}
	if len(failureEntries) != len(wantCodes) {
		t.Fatalf("stderr has %d expose_tools log entries; want %d: %s", len(failureEntries), len(wantCodes), stderr.String())
	}
	for i, failure := range failureEntries {
		if failure["event"] != "mcp_complete" || failure["operation"] != catalog.ToolExposeTools || failure["outcome"] != "error" || failure["level"] != "warn" {
			t.Errorf("failure log entry = %v; want warn mcp_complete expose_tools error", failure)
		}
		assertExposeToolsLogEnvelope(t, failure)
		if failure["stage"] != "tool_admission" {
			t.Errorf("failure stage = %v; want tool_admission", failure["stage"])
		}
		errorInfo, ok := failure["error"].(map[string]any)
		if !ok || errorInfo["code"] != wantCodes[i] {
			t.Errorf("failure error = %v; want code %q", failure["error"], wantCodes[i])
		}
	}
}

func assertExposeToolsLogEnvelope(t *testing.T, entry map[string]any) {
	t.Helper()
	for _, field := range []string{"service", "request_id", "operation_id", "correlation_state", "duration_ms"} {
		if _, found := entry[field]; !found {
			t.Errorf("log entry omitted %q: %v", field, entry)
		}
	}
	for _, field := range []string{"service", "request_id"} {
		value, ok := entry[field].(string)
		if !ok || value == "" {
			t.Errorf("log entry %q = %v; want non-empty string", field, entry[field])
		}
	}
}

func decodeExposeLogLines(t *testing.T, output string) []map[string]any {
	t.Helper()
	if strings.TrimSpace(output) == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	entries := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("decode emitted log line %q: %v", line, err)
		}
		entries = append(entries, entry)
	}
	return entries
}

func TestExposeToolsBackendUnavailable(t *testing.T) {
	surface := &ToolSurface{runtime: NewRuntime()}
	result, err := surface.exposeTools(context.Background(), exposeToolsRequest(json.RawMessage(`{"tool_names":["alpha__new"]}`)))
	if err != nil {
		t.Fatalf("expose_tools without a published catalog returned protocol error: %v", err)
	}
	assertExposeToolsError(t, result, "backend_unavailable")
}
