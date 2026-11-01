package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestServeHybridProgressiveRevealLifecycle(t *testing.T) {
	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{
			name: "real-wire discovery, isolation, metadata, and negative calls",
			run:  TestServeHybridProgressiveRevealDiscovery,
		},
		{
			name: "real-wire inventory pages and stale cursor rejection",
			run:  TestServeHybridProgressiveRevealPages,
		},
		{
			name: "real-wire session admission and forged carrier headers",
			run:  TestServeHybridProgressiveRevealSessionAdmission,
		},
		{
			name: "concurrent first requests and DELETE owner barrier",
			run:  testServeHybridProgressiveRevealConcurrentRetirement,
		},
		{
			name: "lazy idle retirement removes owner cursors and interests",
			run:  testServeHybridProgressiveRevealLazyRetirement,
		},
	}
	for _, test := range tests {
		t.Run(test.name, test.run)
	}
}

type progressiveRevealWireEnvelope struct {
	Result *struct {
		IsError bool `json:"isError"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
		NextCursor string `json:"nextCursor"`
	} `json:"result"`
	Error json.RawMessage `json:"error"`
}

type progressiveRevealWireResponse struct {
	status int
	header http.Header
	body   []byte
	err    error
}

type progressiveRevealWireHarness struct {
	t       *testing.T
	ctx     context.Context
	httpURL string
	client  *http.Client
}

func newProgressiveRevealWireHarness(t *testing.T, pageSize int, idle time.Duration) *progressiveRevealWireHarness {
	t.Helper()
	upstream := fakeUpstreamReadAndWriteTools(t)
	ctx, httpURL := startHybridBridgeWithProgressiveMetadataOverridesPageSizeAndIdle(t, upstream, pageSize, idle)
	return &progressiveRevealWireHarness{
		t:       t,
		ctx:     ctx,
		httpURL: httpURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (h *progressiveRevealWireHarness) request(method, session, body string) progressiveRevealWireResponse {
	return h.requestWithHeaders(method, session, body, func(header http.Header) {
		header["X-Mcp-Bridge-Exposure-Epoch"] = []string{"999", "1000"}
		header["x-mcp-bridge-exposure-epoch"] = []string{"0"}
	})
}

func (h *progressiveRevealWireHarness) requestWithHeaders(
	method, session, body string,
	setHeaders func(http.Header),
) progressiveRevealWireResponse {
	request, err := http.NewRequestWithContext(h.ctx, method, h.httpURL+"/mcp", strings.NewReader(body))
	if err != nil {
		return progressiveRevealWireResponse{err: err}
	}
	request.Header.Set("Authorization", "Bearer "+serveClientToken)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	if session != "" {
		request.Header.Set("Mcp-Session-Id", session)
		request.Header.Set("Mcp-Protocol-Version", "2025-11-25")
	}
	if setHeaders != nil {
		setHeaders(request.Header)
	}
	response, err := h.client.Do(request)
	if err != nil {
		return progressiveRevealWireResponse{err: err}
	}
	payload, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil {
		return progressiveRevealWireResponse{err: readErr}
	}
	if closeErr != nil {
		return progressiveRevealWireResponse{err: closeErr}
	}
	return progressiveRevealWireResponse{
		status: response.StatusCode,
		header: response.Header.Clone(),
		body:   payload,
	}
}

func (h *progressiveRevealWireHarness) send(method, session, body string) progressiveRevealWireResponse {
	h.t.Helper()
	response := h.request(method, session, body)
	if response.err != nil {
		h.t.Fatalf("%s /mcp: %v", method, response.err)
	}
	return response
}

func (h *progressiveRevealWireHarness) initialize(name string) string {
	h.t.Helper()
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"` + name + `","version":"1"}}}`
	response := h.send(http.MethodPost, "", body)
	if response.status != http.StatusOK {
		h.t.Fatalf("%s initialize status = %d (body=%s)", name, response.status, response.body)
	}
	session := response.header.Get("Mcp-Session-Id")
	if session == "" {
		h.t.Fatalf("%s initialize omitted Mcp-Session-Id", name)
	}
	initialized := h.send(http.MethodPost, session, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if initialized.status != http.StatusAccepted {
		h.t.Fatalf("%s initialized status = %d (body=%s)", name, initialized.status, initialized.body)
	}
	return session
}

func (h *progressiveRevealWireHarness) rpc(session, body string) (progressiveRevealWireResponse, progressiveRevealWireEnvelope) {
	h.t.Helper()
	response := h.send(http.MethodPost, session, body)
	if response.status != http.StatusOK {
		h.t.Fatalf("JSON-RPC request status = %d (body=%s)", response.status, response.body)
	}
	return response, decodeProgressiveRevealWireEnvelope(h.t, response.body)
}

func decodeProgressiveRevealWireEnvelope(t *testing.T, body []byte) progressiveRevealWireEnvelope {
	t.Helper()
	payload := bytes.TrimSpace(body)
	if index := bytes.Index(payload, []byte("data: ")); index >= 0 {
		payload = payload[index+len("data: "):]
		if end := bytes.IndexByte(payload, '\n'); end >= 0 {
			payload = payload[:end]
		}
	}
	var envelope progressiveRevealWireEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("decode JSON-RPC response: %v (body=%s)", err, body)
	}
	return envelope
}

func (h *progressiveRevealWireHarness) discoverWrite(session string) string {
	h.t.Helper()
	_, envelope := h.rpc(session, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search_tools","arguments":{"query":"file"}}}`)
	assertProgressiveRevealCallSuccess(h.t, "search_tools", envelope)
	return progressiveRevealWriteName(h.t, envelope)
}

func progressiveRevealWriteName(t *testing.T, envelope progressiveRevealWireEnvelope) string {
	t.Helper()
	if envelope.Result == nil || len(envelope.Result.Content) == 0 {
		t.Fatal("search_tools returned no result content")
	}
	var result struct {
		Results []struct {
			ToolName     string `json:"tool_name"`
			Server       string `json:"server"`
			UpstreamName string `json:"upstream_name"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(envelope.Result.Content[0].Text), &result); err != nil {
		t.Fatalf("decode search_tools results: %v", err)
	}
	for _, target := range result.Results {
		if target.Server == "alpha" && target.UpstreamName == "write" && target.ToolName != "" {
			return target.ToolName
		}
	}
	t.Fatalf("search_tools omitted alpha/write: %+v", result.Results)
	return ""
}

func progressiveRevealCallBody(t *testing.T, id int, name, arguments string) string {
	t.Helper()
	encodedName, err := json.Marshal(name)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid([]byte(arguments)) {
		t.Fatalf("invalid JSON tool arguments %q", arguments)
	}
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":%s,"arguments":%s}}`,
		id, encodedName, arguments)
}

func progressiveRevealListBody(id int, cursor string) string {
	if cursor == "" {
		return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/list","params":{}}`, id)
	}
	encodedCursor, _ := json.Marshal(cursor)
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/list","params":{"cursor":%s}}`,
		id, encodedCursor)
}

func assertProgressiveRevealCallSuccess(t *testing.T, label string, envelope progressiveRevealWireEnvelope) {
	t.Helper()
	if envelope.Result == nil || envelope.Result.IsError || progressiveRevealHasRPCError(envelope.Error) {
		t.Fatalf("%s failed: %+v", label, envelope)
	}
}

func assertProgressiveRevealCallFailure(t *testing.T, label string, response progressiveRevealWireResponse, envelope progressiveRevealWireEnvelope) {
	t.Helper()
	if response.status < 400 && !progressiveRevealHasRPCError(envelope.Error) &&
		(envelope.Result == nil || !envelope.Result.IsError) {
		t.Fatalf("%s unexpectedly succeeded: %+v", label, envelope)
	}
}

func progressiveRevealHasRPCError(raw json.RawMessage) bool {
	return len(raw) != 0 && string(raw) != "null"
}

func assertProgressiveRevealListNames(t *testing.T, envelope progressiveRevealWireEnvelope, allowedDynamic string) {
	t.Helper()
	if envelope.Result == nil {
		t.Fatalf("tools/list returned no result: %+v", envelope)
	}
	names := make([]string, 0, len(envelope.Result.Tools))
	for _, tool := range envelope.Result.Tools {
		names = append(names, tool.Name)
	}
	assertProgressiveRevealNameSet(t, names, allowedDynamic)
}
func assertProgressiveRevealListWithoutStaleInterest(t *testing.T, label string, envelope progressiveRevealWireEnvelope, dynamicName string) {
	t.Helper()
	if envelope.Result == nil || progressiveRevealHasRPCError(envelope.Error) {
		t.Fatalf("%s tools/list did not return a successful result: %+v", label, envelope)
	}
	for _, tool := range envelope.Result.Tools {
		if tool.Name == dynamicName {
			t.Fatalf("%s tools/list retained stale dynamic tool %q", label, dynamicName)
		}
	}
}

func assertProgressiveRevealNameSet(t *testing.T, names []string, allowedDynamic string) {
	t.Helper()
	expected := map[string]bool{
		"search_tools":  true,
		"describe_tool": true,
		"call_tool":     true,
		"alpha__read":   true,
	}
	if allowedDynamic != "" {
		expected[allowedDynamic] = true
	}
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if seen[name] {
			t.Errorf("tools/list duplicated %q", name)
		}
		seen[name] = true
		if !expected[name] {
			t.Errorf("tools/list included unexpected target %q", name)
		}
	}
	for name := range expected {
		if !seen[name] {
			t.Errorf("tools/list omitted %q; got %v", name, seen)
		}
	}
}
func assertProgressiveRevealListSnapshot(t *testing.T, envelope progressiveRevealWireEnvelope, dynamicName string) {
	t.Helper()
	if envelope.Result == nil {
		t.Fatalf("tools/list returned no result: %+v", envelope)
	}
	dynamicPresent := false
	for _, tool := range envelope.Result.Tools {
		dynamicPresent = dynamicPresent || tool.Name == dynamicName
	}
	if dynamicPresent {
		assertProgressiveRevealListNames(t, envelope, dynamicName)
		return
	}
	assertProgressiveRevealListNames(t, envelope, "")
}

func (h *progressiveRevealWireHarness) listAllNames(session string, requestID int) []string {
	h.t.Helper()
	var names []string
	cursor := ""
	for page := range 8 {
		_, envelope := h.rpc(session, progressiveRevealListBody(requestID+page, cursor))
		if envelope.Result == nil {
			h.t.Fatalf("tools/list page %d returned no result", page+1)
		}
		for _, tool := range envelope.Result.Tools {
			names = append(names, tool.Name)
		}
		cursor = envelope.Result.NextCursor
		if cursor == "" {
			return names
		}
	}
	h.t.Fatal("tools/list exceeded the lifecycle test page limit")
	return nil
}

func testServeHybridProgressiveRevealConcurrentRetirement(t *testing.T) {
	harness := newProgressiveRevealWireHarness(t, 0, 10*time.Minute)
	sessionA := harness.initialize("progressive-lifecycle-a")
	sessionB := harness.initialize("progressive-lifecycle-b")

	start := make(chan struct{})
	type concurrentResult struct {
		name     string
		response progressiveRevealWireResponse
	}
	results := make(chan concurrentResult, 2)
	go func() {
		<-start
		results <- concurrentResult{
			name:     "tools/list",
			response: harness.request(http.MethodPost, sessionA, progressiveRevealListBody(10, "")),
		}
	}()
	go func() {
		<-start
		results <- concurrentResult{
			name:     "search_tools",
			response: harness.request(http.MethodPost, sessionA, `{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"search_tools","arguments":{"query":"file"}}}`),
		}
	}()
	close(start)

	var listResponse, searchResponse progressiveRevealWireResponse
	for range 2 {
		result := <-results
		if result.response.err != nil {
			t.Fatalf("concurrent %s: %v", result.name, result.response.err)
		}
		if result.name == "tools/list" {
			listResponse = result.response
		} else {
			searchResponse = result.response
		}
	}
	if listResponse.status != http.StatusOK || searchResponse.status != http.StatusOK {
		t.Fatalf("concurrent first request statuses = list %d, search %d", listResponse.status, searchResponse.status)
	}
	listEnvelope := decodeProgressiveRevealWireEnvelope(t, listResponse.body)
	searchEnvelope := decodeProgressiveRevealWireEnvelope(t, searchResponse.body)
	if listEnvelope.Result == nil || progressiveRevealHasRPCError(listEnvelope.Error) {
		t.Fatalf("concurrent first tools/list failed: %+v", listEnvelope)
	}
	assertProgressiveRevealCallSuccess(t, "concurrent first search_tools", searchEnvelope)
	writeName := progressiveRevealWriteName(t, searchEnvelope)
	assertProgressiveRevealListSnapshot(t, listEnvelope, writeName)
	if listEnvelope.Result.NextCursor != "" {
		t.Fatalf("default-size tools/list unexpectedly paginated its small catalog: cursor=%q", listEnvelope.Result.NextCursor)
	}

	headerCases := []struct {
		name string
		set  func(http.Header)
	}{
		{name: "missing carrier", set: func(http.Header) {}},
		{name: "malformed carrier", set: func(header http.Header) {
			header.Set("X-Mcp-Bridge-Exposure-Epoch", "not-an-epoch")
		}},
		{name: "mismatched carrier", set: func(header http.Header) {
			header.Set("X-Mcp-Bridge-Exposure-Epoch", "999")
		}},
		{name: "mixed-case duplicate carriers", set: func(header http.Header) {
			header["X-Mcp-Bridge-Exposure-Epoch"] = []string{"999", "1000"}
			header["x-mcp-bridge-exposure-epoch"] = []string{"0"}
		}},
	}
	for index, test := range headerCases {
		response := harness.requestWithHeaders(
			http.MethodPost,
			sessionB,
			progressiveRevealListBody(12+index, ""),
			test.set,
		)
		if response.err != nil || response.status != http.StatusOK {
			t.Fatalf("%s request status = %d err %v", test.name, response.status, response.err)
		}
		assertProgressiveRevealCallSuccess(t, test.name, decodeProgressiveRevealWireEnvelope(t, response.body))
	}

	_, listA := harness.rpc(sessionA, progressiveRevealListBody(17, ""))
	assertProgressiveRevealListNames(t, listA, writeName)
	_, listB := harness.rpc(sessionB, progressiveRevealListBody(18, ""))
	assertProgressiveRevealListNames(t, listB, "")

	_, callA := harness.rpc(sessionA, progressiveRevealCallBody(t, 14, writeName, `{"path":"file.txt","data":"owner-a"}`))
	assertProgressiveRevealCallSuccess(t, "owner A discovered direct call", callA)
	responseB, callB := harness.rpc(sessionB, progressiveRevealCallBody(t, 15, writeName, `{"path":"file.txt","data":"owner-b"}`))
	assertProgressiveRevealCallFailure(t, "owner B guessed owner A's target", responseB, callB)
	_, pinnedB := harness.rpc(sessionB, progressiveRevealCallBody(t, 16, "alpha__read", `{"path":"file.txt"}`))
	assertProgressiveRevealCallSuccess(t, "owner B pinned call", pinnedB)
	deleted := harness.send(http.MethodDelete, sessionA, "")
	if deleted.status < 200 || deleted.status >= 300 {
		t.Fatalf("DELETE /mcp for owner A returned %d (body=%s)", deleted.status, deleted.body)
	}

	retiredA := harness.request(http.MethodPost, sessionA, progressiveRevealListBody(18, ""))
	if retiredA.err != nil || retiredA.status != http.StatusOK {
		t.Fatalf("deleted owner A request = status %d err %v; want 200 after session rehydration", retiredA.status, retiredA.err)
	}
	assertProgressiveRevealListWithoutStaleInterest(t, "deleted owner A", decodeProgressiveRevealWireEnvelope(t, retiredA.body), writeName)
	_, listBAfterDelete := harness.rpc(sessionB, progressiveRevealListBody(19, ""))
	assertProgressiveRevealListNames(t, listBAfterDelete, "")
	_, pinnedBAfterDelete := harness.rpc(sessionB, progressiveRevealCallBody(t, 20, "alpha__read", `{"path":"file.txt"}`))
	assertProgressiveRevealCallSuccess(t, "owner B pinned call after owner A DELETE", pinnedBAfterDelete)
	responseB, callBAfterDelete := harness.rpc(sessionB, progressiveRevealCallBody(t, 21, writeName, `{"path":"file.txt","data":"after-delete"}`))
	assertProgressiveRevealCallFailure(t, "owner B target after owner A DELETE", responseB, callBAfterDelete)
}

func testServeHybridProgressiveRevealLazyRetirement(t *testing.T) {
	idle := time.Second
	var clockMu sync.Mutex
	simulatedNow := time.Now()
	testServeNow = func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		return simulatedNow
	}
	t.Cleanup(func() { testServeNow = nil })
	advanceClock := func(duration time.Duration) {
		clockMu.Lock()
		simulatedNow = simulatedNow.Add(duration)
		clockMu.Unlock()
	}
	harness := newProgressiveRevealWireHarness(t, 2, idle)
	sessionB := harness.initialize("progressive-lifecycle-survivor")
	sessionC := harness.initialize("progressive-lifecycle-idle")
	_, baselineB := harness.rpc(sessionB, progressiveRevealListBody(21, ""))
	if baselineB.Result == nil || baselineB.Result.NextCursor == "" {
		t.Fatalf("owner B did not receive a paginated baseline: %+v", baselineB)
	}

	writeName := harness.discoverWrite(sessionC)
	_, listC := harness.rpc(sessionC, progressiveRevealListBody(22, ""))
	if listC.Result == nil || listC.Result.NextCursor == "" {
		t.Fatalf("owner C did not receive a paginated discovered list: %+v", listC)
	}
	cursorC := listC.Result.NextCursor
	_, callC := harness.rpc(sessionC, progressiveRevealCallBody(t, 23, writeName, `{"path":"file.txt","data":"owner-c"}`))
	assertProgressiveRevealCallSuccess(t, "owner C discovered direct call", callC)
	_, pinnedB := harness.rpc(sessionB, progressiveRevealCallBody(t, 24, "alpha__read", `{"path":"file.txt"}`))
	assertProgressiveRevealCallSuccess(t, "owner B pinned call before idle retirement", pinnedB)

	advanceClock(idle/2 + 50*time.Millisecond)
	_, _ = harness.rpc(sessionB, progressiveRevealListBody(25, ""))
	advanceClock(idle/2 + 50*time.Millisecond)
	survivorNames := harness.listAllNames(sessionB, 26)
	assertProgressiveRevealNameSet(t, survivorNames, "")

	retiredC := harness.request(http.MethodPost, sessionC, progressiveRevealListBody(27, ""))
	if retiredC.err != nil || retiredC.status != http.StatusOK {
		t.Fatalf("idle owner C request = status %d err %v; want 200 after admission rehydration", retiredC.status, retiredC.err)
	}
	assertProgressiveRevealListWithoutStaleInterest(t, "idle owner C", decodeProgressiveRevealWireEnvelope(t, retiredC.body), writeName)
	retiredCallC := harness.request(http.MethodPost, sessionC, progressiveRevealCallBody(t, 31, writeName, `{"path":"file.txt","data":"after-idle"}`))
	if retiredCallC.err != nil || retiredCallC.status != http.StatusOK {
		t.Fatalf("idle owner C direct call = status %d err %v; want 200 after admission rehydration", retiredCallC.status, retiredCallC.err)
	}
	assertProgressiveRevealCallFailure(t, "idle owner C direct call after expiry", retiredCallC,
		decodeProgressiveRevealWireEnvelope(t, retiredCallC.body))
	responseB, staleCursor := harness.rpc(sessionB, progressiveRevealListBody(28, cursorC))
	assertProgressiveRevealCallFailure(t, "owner B using owner C cursor", responseB, staleCursor)
	_, pinnedBAfterExpiry := harness.rpc(sessionB, progressiveRevealCallBody(t, 29, "alpha__read", `{"path":"file.txt"}`))
	assertProgressiveRevealCallSuccess(t, "owner B pinned call after owner C idle retirement", pinnedBAfterExpiry)
	responseB, dynamicAfterExpiry := harness.rpc(sessionB, progressiveRevealCallBody(t, 30, writeName, `{"path":"file.txt","data":"after-idle"}`))
	assertProgressiveRevealCallFailure(t, "owner B target after owner C idle retirement", responseB, dynamicAfterExpiry)
}
