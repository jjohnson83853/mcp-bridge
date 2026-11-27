package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
)

func TestProgressiveMetadataDiscoveryCommit(t *testing.T) {
	upstream := &progressiveTestUpstream{}
	runtime := NewRuntime()
	deps := progressiveTestDeps(upstream)
	assembly, err := runtime.Reload(progressiveTestConfig(t, "old"), deps)
	if err != nil {
		t.Fatalf("admit generation: %v", err)
	}
	server := NewMCPServer()
	surface := progressiveToolSurface(t, runtime, server)
	if err := surface.Published(assembly); err != nil {
		t.Fatalf("publish initial pinned projection: %v", err)
	}
	const tokenA = "progressive-discovery-token-a"
	const tokenB = "progressive-discovery-token-b"
	inbound := NewInboundHandler(
		NewStreamableHandler(server),
		[][32]byte{
			sha256.Sum256([]byte(tokenA)),
			sha256.Sum256([]byte(tokenB)),
		},
		InboundOptions{ToolSurface: surface},
	)
	httpServer := httptest.NewServer(inbound)
	t.Cleanup(httpServer.Close)
	clientA := connectProgressiveDiscoveryClient(t, httpServer, tokenA)
	clientB := connectProgressiveDiscoveryClient(t, httpServer, tokenB)
	for _, client := range []*mcp.ClientSession{clientA, clientB} {
		if _, err := client.ListTools(context.Background(), nil); err != nil {
			t.Fatalf("initialize progressive session tool list: %v", err)
		}
	}

	search := func(client *mcp.ClientSession, query string) (*mcp.CallToolResult, error) {
		return client.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "search_tools",
			Arguments: json.RawMessage(`{"query":"` + query + `"}`),
		})
	}
	decodeSearch := func(result *mcp.CallToolResult) struct {
		CatalogRevision string `json:"catalog_revision"`
		Results         []struct {
			ToolName     string `json:"tool_name"`
			Server       string `json:"server"`
			UpstreamName string `json:"upstream_name"`
		} `json:"results"`
	} {
		t.Helper()
		var response struct {
			CatalogRevision string `json:"catalog_revision"`
			Results         []struct {
				ToolName     string `json:"tool_name"`
				Server       string `json:"server"`
				UpstreamName string `json:"upstream_name"`
			} `json:"results"`
		}
		if result == nil || len(result.Content) == 0 {
			t.Fatalf("search response has no text content: %#v", result)
		}
		text, ok := result.Content[0].(*mcp.TextContent)
		if !ok {
			t.Fatalf("search response content type = %T; want text", result.Content[0])
		}
		if err := json.Unmarshal([]byte(text.Text), &response); err != nil {
			t.Fatalf("decode search response: %v (text=%q)", err, text.Text)
		}
		return response
	}

	empty, err := search(clientA, "no-matching-progressive-tool")
	if err != nil {
		t.Fatalf("empty discovery search: %v", err)
	}
	if empty.IsError {
		t.Fatalf("empty discovery search returned isError: %#v", empty)
	}
	if len(decodeSearch(empty).Results) != 0 {
		t.Fatalf("empty discovery search returned results: %#v", decodeSearch(empty).Results)
	}
	beforeEmpty, err := clientA.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list after empty discovery: %v", err)
	}
	for _, tool := range beforeEmpty.Tools {
		if tool.Name == "alpha__new" || tool.Name == "alpha__shared" {
			t.Fatalf("empty discovery exposed %q", tool.Name)
		}
	}

	coordinator := surface.projectionCoordinator()
	coordinator.barrier.Lock()
	var discoveryOwner projectionOwner
	var originalProjection *toolProjection
	for owner, projection := range coordinator.projections {
		if owner.kind == projectionSession {
			discoveryOwner = owner
			originalProjection = projection
			break
		}
	}
	coordinator.barrier.Unlock()
	if originalProjection == nil {
		t.Fatal("session projections were not admitted before discovery")
	}
	invalidTarget := logicalToolTarget{serverID: "alpha", rawName: "not-in-catalog"}
	if err := surface.commitDiscovery(discoveryOwner, assembly, map[logicalToolTarget]struct{}{invalidTarget: {}}); err == nil {
		t.Fatal("discovery preparation accepted a target absent from the catalog")
	}
	coordinator.barrier.Lock()
	projectionUnchanged := coordinator.projections[discoveryOwner] == originalProjection
	_, invalidTargetExposed := coordinator.union[core.PublicName(invalidTarget.serverID, invalidTarget.rawName)]
	coordinator.barrier.Unlock()
	if !projectionUnchanged || invalidTargetExposed {
		t.Fatal("failed discovery preparation changed projection or union state")
	}
	registryEntered := make(chan struct{}, 2)
	coordinator.testBeforeRegistryAdd = func() {
		select {
		case registryEntered <- struct{}{}:
		default:
		}
	}
	defer func() { coordinator.testBeforeRegistryAdd = nil }()

	projectionsBefore, unionBefore := snapshotProgressiveCoordinatorState(t, coordinator)
	type result struct {
		name string
		call *mcp.CallToolResult
		err  error
	}
	completed := make(chan result, 2)
	start := make(chan struct{})
	go func() {
		<-start
		call, err := search(clientA, "new")
		completed <- result{name: "new", call: call, err: err}
	}()
	go func() {
		<-start
		call, err := search(clientA, "shared")
		completed <- result{name: "shared", call: call, err: err}
	}()
	close(start)

	finished := make([]result, 0, 2)
	deadline := time.After(5 * time.Second)
	for len(finished) < 2 {
		select {
		case call := <-completed:
			finished = append(finished, call)
		case <-deadline:
			t.Fatal("timed out waiting for concurrent search calls")
		}
	}
	if len(registryEntered) != 0 {
		t.Errorf("concurrent searches triggered %d SDK registry additions; want 0", len(registryEntered))
	}
	projectionsAfter, unionAfter := snapshotProgressiveCoordinatorState(t, coordinator)
	if !reflect.DeepEqual(projectionsAfter, projectionsBefore) || !reflect.DeepEqual(unionAfter, unionBefore) {
		t.Error("concurrent searches changed projections, interests, or registry union")
	}
	for _, call := range finished {
		if call.err != nil || call.call == nil || call.call.IsError {
			t.Errorf("%s search failed: result=%#v err=%v", call.name, call.call, call.err)
			continue
		}
		response := decodeSearch(call.call)
		if response.CatalogRevision != assembly.CatalogRevision() {
			t.Errorf("%s search revision = %q; want accepted %q", call.name, response.CatalogRevision, assembly.CatalogRevision())
		}
		if len(response.Results) == 0 {
			t.Errorf("%s search returned no target", call.name)
		}
	}

	coordinator.barrier.Lock()
	sameSessionProjectionHasSearchedTargets := false
	for owner, projection := range coordinator.projections {
		if owner.kind != projectionSession || projection == nil {
			continue
		}
		if _, hasNew := projection.byName["alpha__new"]; hasNew {
			sameSessionProjectionHasSearchedTargets = true
		}
		if _, hasShared := projection.byName["alpha__shared"]; hasShared {
			sameSessionProjectionHasSearchedTargets = true
		}
	}
	coordinator.barrier.Unlock()
	if sameSessionProjectionHasSearchedTargets {
		t.Fatal("concurrent searches added searched tools to a session projection")
	}

	projectionsBeforeBSearch, unionBeforeBSearch := snapshotProgressiveCoordinatorState(t, coordinator)
	rejectedName := "alpha__new"
	clientBTools, err := clientB.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list for distinct bearer after discovery: %v", err)
	}
	for _, tool := range clientBTools.Tools {
		if tool.Name == rejectedName {
			t.Errorf("client B tools/list exposed client A's discovered tool %q", rejectedName)
		}
	}
	callsBeforeRejected := len(upstream.callsSnapshot())
	rejected, err := clientB.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      rejectedName,
		Arguments: json.RawMessage(`{}`),
	})
	if err == nil && (rejected == nil || !rejected.IsError) {
		t.Errorf("client B call to client A's discovered tool was not rejected: result=%#v err=%v", rejected, err)
	}
	if calls := len(upstream.callsSnapshot()); calls != callsBeforeRejected {
		t.Errorf("rejected client B call added an upstream operation: before=%d after=%d", callsBeforeRejected, calls)
	}

	revealedShared, err := search(clientB, "shared")
	if err != nil || revealedShared == nil || revealedShared.IsError {
		t.Fatalf("session B discovery of session A's existing union tool failed: result=%#v err=%v", revealedShared, err)
	}
	if len(registryEntered) != 0 {
		t.Error("search for an existing union tool signaled an SDK registry change")
	}
	projectionsAfterBSearch, unionAfterBSearch := snapshotProgressiveCoordinatorState(t, coordinator)
	if !reflect.DeepEqual(projectionsAfterBSearch, projectionsBeforeBSearch) || !reflect.DeepEqual(unionAfterBSearch, unionBeforeBSearch) {
		t.Error("session B search changed projections, interests, or registry union")
	}

	beforeRevision := runtime.Current().CatalogRevision()
	coordinator.barrier.Lock()
	beforeProjections := make(map[projectionOwner]*toolProjection, len(coordinator.projections))
	for owner, projection := range coordinator.projections {
		beforeProjections[owner] = projection
	}
	coordinator.barrier.Unlock()
	repeated, err := search(clientA, "new")
	if err != nil || repeated == nil || repeated.IsError {
		t.Fatalf("repeated unchanged discovery failed: result=%#v err=%v", repeated, err)
	}
	repeatedResponse := decodeSearch(repeated)
	if repeatedResponse.CatalogRevision != beforeRevision || runtime.Current() != assembly {
		t.Errorf("repeated discovery changed the accepted catalog revision")
	}
	coordinator.barrier.Lock()
	projectionsChanged := len(coordinator.projections) != len(beforeProjections)
	for owner, projection := range beforeProjections {
		if coordinator.projections[owner] != projection {
			projectionsChanged = true
			break
		}
	}
	coordinator.barrier.Unlock()
	if projectionsChanged {
		t.Error("repeated unchanged discovery changed projection state")
	}
	select {
	case <-registryEntered:
		t.Error("repeated unchanged discovery signaled an SDK registry change")
	default:
	}

	for _, check := range []struct {
		client  *mcp.ClientSession
		rawName string
	}{{clientA, "new"}, {clientB, "shared"}} {
		name := "alpha__" + check.rawName
		tools := snapshotProgressiveClientTools(t, check.client)
		if _, found := tools[name]; found {
			t.Errorf("search for %s exposed unpinned tool %q in tools/list", check.rawName, name)
		}
		rejected, err := check.client.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      name,
			Arguments: json.RawMessage(`{}`),
		})
		if err == nil && (rejected == nil || !rejected.IsError) {
			t.Errorf("direct call to searched but unexposed %s was not rejected: result=%#v err=%v", check.rawName, rejected, err)
		}
		if calls := len(upstream.callsSnapshot()); calls != 0 {
			t.Errorf("search and rejected direct calls made %d upstream operations; want 0", calls)
		}
	}
	exposed, err := clientA.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      catalog.ToolExposeTools,
		Arguments: json.RawMessage(`{"tool_names":["alpha__new","alpha__shared"]}`),
	})
	if err != nil || exposed == nil || exposed.IsError {
		t.Fatalf("explicit expose_tools failed: result=%#v err=%v", exposed, err)
	}
	toolsAfterExpose := snapshotProgressiveClientTools(t, clientA)
	for _, name := range []string{"alpha__new", "alpha__shared"} {
		if _, found := toolsAfterExpose[name]; !found {
			t.Errorf("tools/list omitted %q after explicit expose_tools", name)
		}
		result, err := clientA.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      name,
			Arguments: json.RawMessage(`{}`),
		})
		if err != nil || result == nil || result.IsError {
			t.Errorf("direct call to explicitly exposed %q failed: result=%#v err=%v", name, result, err)
		}
	}
	if calls := len(upstream.callsSnapshot()); calls != 2 {
		t.Errorf("explicit exposure and direct calls made %d upstream operations; want 2", calls)
	}
}
func TestSearchNoImplicitExposure(t *testing.T) {
	upstream := &progressiveTestUpstream{}
	runtime := NewRuntime()
	deps := progressiveTestDeps(upstream)
	assembly, err := runtime.Reload(progressiveTestConfig(t, "old"), deps)
	if err != nil {
		t.Fatalf("admit generation: %v", err)
	}
	server := NewMCPServer()
	surface := progressiveToolSurface(t, runtime, server)
	if err := surface.Published(assembly); err != nil {
		t.Fatalf("publish initial pinned projection: %v", err)
	}
	const token = "progressive-no-implicit-exposure-token"
	inbound := NewInboundHandler(
		NewStreamableHandler(server),
		[][32]byte{sha256.Sum256([]byte(token))},
		InboundOptions{ToolSurface: surface},
	)
	httpServer := httptest.NewServer(inbound)
	t.Cleanup(httpServer.Close)
	client := connectProgressiveDiscoveryClient(t, httpServer, token)
	if _, err := client.ListTools(context.Background(), nil); err != nil {
		t.Fatalf("initialize progressive session tool list: %v", err)
	}

	toolsBefore := snapshotProgressiveClientTools(t, client)
	pinnedName := "alpha__old"
	if _, found := toolsBefore[pinnedName]; !found {
		t.Fatalf("configured pin %q missing before search", pinnedName)
	}
	for _, name := range []string{"alpha__new", "alpha__shared"} {
		if _, found := toolsBefore[name]; found {
			t.Fatalf("unexposed search target %q listed before search", name)
		}
	}
	coordinator := surface.projectionCoordinator()
	projectionsBefore, unionBefore := snapshotProgressiveCoordinatorState(t, coordinator)
	registrySignals := make(chan struct{}, 1)
	coordinator.testBeforeRegistryAdd = func() {
		select {
		case registrySignals <- struct{}{}:
		default:
		}
	}
	t.Cleanup(func() { coordinator.testBeforeRegistryAdd = nil })

	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "search_tools",
		Arguments: json.RawMessage(`{"query":"new"}`),
	})
	if err != nil || result == nil || result.IsError {
		t.Fatalf("admitted search failed: result=%#v err=%v", result, err)
	}
	if len(result.Content) == 0 {
		t.Fatal("search response has no content")
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("search response content type = %T; want text", result.Content[0])
	}
	var response struct {
		Results []struct {
			ToolName string `json:"tool_name"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(text.Text), &response); err != nil {
		t.Fatalf("decode search response: %v", err)
	}
	foundMatch := false
	for _, match := range response.Results {
		if match.ToolName == "alpha__new" {
			foundMatch = true
			break
		}
	}
	if !foundMatch {
		t.Fatalf("search response did not include expected match alpha__new: %#v", response.Results)
	}

	toolsAfter := snapshotProgressiveClientTools(t, client)
	if !reflect.DeepEqual(toolsAfter, toolsBefore) {
		t.Errorf("tools/list changed after search: before=%v after=%v", toolsBefore, toolsAfter)
	}
	for _, name := range []string{"alpha__new", "alpha__shared"} {
		if _, found := toolsAfter[name]; found {
			t.Errorf("search exposed unpinned tool %q", name)
		}
	}
	projectionsAfter, unionAfter := snapshotProgressiveCoordinatorState(t, coordinator)
	if !reflect.DeepEqual(projectionsAfter, projectionsBefore) || !reflect.DeepEqual(unionAfter, unionBefore) {
		t.Error("search changed projections, interests, or registry union")
	}
	if len(registrySignals) != 0 {
		t.Errorf("search triggered %d SDK registry additions; want 0", len(registrySignals))
	}
	rejected, err := client.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "alpha__new",
		Arguments: json.RawMessage(`{}`),
	})
	if err == nil && (rejected == nil || !rejected.IsError) {
		t.Errorf("direct call to searched but unexposed alpha__new was not rejected: result=%#v err=%v", rejected, err)
	}
	if calls := upstream.callsSnapshot(); len(calls) != 0 {
		t.Errorf("search plus rejected direct call made %d upstream operations; want 0", len(calls))
	}
}

func TestProgressiveMetadataDiscoveryFailuresDoNotCommit(t *testing.T) {
	upstream := &progressiveTestUpstream{}
	runtime := NewRuntime()
	assembly, err := runtime.Reload(progressiveTestConfig(t, "old"), progressiveTestDeps(upstream))
	if err != nil {
		t.Fatalf("admit generation: %v", err)
	}
	server := NewMCPServer()
	surface := progressiveToolSurface(t, runtime, server)
	if err := surface.Published(assembly); err != nil {
		t.Fatalf("publish initial pinned projection: %v", err)
	}
	const token = "progressive-discovery-failure-token"
	inbound := NewInboundHandler(
		NewStreamableHandler(server),
		[][32]byte{sha256.Sum256([]byte(token))},
		InboundOptions{ToolSurface: surface},
	)
	httpServer := httptest.NewServer(inbound)
	t.Cleanup(httpServer.Close)
	client := connectProgressiveDiscoveryClient(t, httpServer, token)
	if _, err := client.ListTools(context.Background(), nil); err != nil {
		t.Fatalf("initialize progressive session tool list: %v", err)
	}

	coordinator := surface.projectionCoordinator()
	coordinator.barrier.Lock()
	var owner projectionOwner
	var projectionBefore progressiveProjectionSnapshot
	var projectionSnapshotErr error
	for candidate, projection := range coordinator.projections {
		if candidate.kind == projectionSession {
			owner = candidate
			projectionBefore, projectionSnapshotErr = snapshotProgressiveProjection(projection)
			break
		}
	}
	unionBefore, unionSnapshotErr := snapshotProgressiveUnion(coordinator.union)
	coordinator.barrier.Unlock()
	if projectionSnapshotErr != nil {
		t.Fatalf("snapshot session projection: %v", projectionSnapshotErr)
	}
	if unionSnapshotErr != nil {
		t.Fatalf("snapshot projection union: %v", unionSnapshotErr)
	}
	if !projectionBefore.present {
		t.Fatal("session projection was not admitted before negative discovery cases")
	}
	registrySignals := make(chan struct{}, 1)
	coordinator.testBeforeRegistryAdd = func() {
		select {
		case registrySignals <- struct{}{}:
		default:
		}
	}
	t.Cleanup(func() { coordinator.testBeforeRegistryAdd = nil })

	originalModel := surface.cfg.Model
	surface.cfg.Model.Dimensions = 0
	failedSearch, err := client.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "search_tools",
		Arguments: json.RawMessage(`{"query":"new"}`),
	})
	surface.cfg.Model = originalModel
	if err != nil {
		t.Fatalf("call search_tools with forced searcher construction failure: %v", err)
	}
	if failedSearch == nil || !failedSearch.IsError {
		t.Fatalf("forced searcher failure was not reported: %#v", failedSearch)
	}
	assertProgressiveDiscoveryUnchanged(t, coordinator, owner, projectionBefore, unionBefore, registrySignals)

	describeArguments, err := json.Marshal(struct {
		ToolName        string `json:"tool_name"`
		CatalogRevision string `json:"catalog_revision"`
	}{
		ToolName:        "alpha__new",
		CatalogRevision: strings.Repeat("b", 64),
	})
	if err != nil {
		t.Fatalf("encode stale describe request: %v", err)
	}
	staleDescribe, err := client.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "describe_tool",
		Arguments: json.RawMessage(describeArguments),
	})
	if err != nil {
		t.Fatalf("call describe_tool with stale revision: %v", err)
	}
	if staleDescribe == nil || !staleDescribe.IsError {
		t.Fatalf("stale describe revision was not rejected: %#v", staleDescribe)
	}
	if response, ok := staleDescribe.StructuredContent.(map[string]any); !ok || response["code"] != codeCatalogChanged {
		t.Fatalf("stale describe response = %#v; want catalog_changed", staleDescribe.StructuredContent)
	}
	assertProgressiveDiscoveryUnchanged(t, coordinator, owner, projectionBefore, unionBefore, registrySignals)
}

func TestProgressiveMetadataSearchCancellationMakesNoCommit(t *testing.T) {
	upstream := &progressiveTestUpstream{}
	runtime := NewRuntime()
	assembly, err := runtime.Reload(progressiveTestConfig(t, "old"), progressiveTestDeps(upstream))
	if err != nil {
		t.Fatalf("admit generation: %v", err)
	}
	server := NewMCPServer()
	surface := progressiveToolSurface(t, runtime, server)
	if err := surface.Published(assembly); err != nil {
		t.Fatalf("publish initial pinned projection: %v", err)
	}
	const token = "progressive-discovery-cancellation-token"
	inbound := NewInboundHandler(
		NewStreamableHandler(server),
		[][32]byte{sha256.Sum256([]byte(token))},
		InboundOptions{ToolSurface: surface},
	)
	httpServer := httptest.NewServer(inbound)
	t.Cleanup(httpServer.Close)
	client := connectProgressiveDiscoveryClient(t, httpServer, token)
	if _, err := client.ListTools(context.Background(), nil); err != nil {
		t.Fatalf("initialize progressive session tool list: %v", err)
	}

	coordinator := surface.projectionCoordinator()
	coordinator.barrier.Lock()
	var owner projectionOwner
	var projectionBefore progressiveProjectionSnapshot
	var projectionSnapshotErr error
	for candidate, projection := range coordinator.projections {
		if candidate.kind == projectionSession {
			owner = candidate
			projectionBefore, projectionSnapshotErr = snapshotProgressiveProjection(projection)
			break
		}
	}
	unionBefore, unionSnapshotErr := snapshotProgressiveUnion(coordinator.union)
	coordinator.barrier.Unlock()
	if projectionSnapshotErr != nil {
		t.Fatalf("snapshot session projection: %v", projectionSnapshotErr)
	}
	if unionSnapshotErr != nil {
		t.Fatalf("snapshot projection union: %v", unionSnapshotErr)
	}
	if !projectionBefore.present {
		t.Fatal("session projection was not admitted before cancelled discovery")
	}
	if _, alreadyExposed := projectionBefore.byName["alpha__new"]; alreadyExposed {
		t.Fatal("cancellation target alpha__new was already exposed")
	}

	registrySignals := make(chan struct{}, 1)
	admissionSignals := make(chan struct{}, 1)
	coordinator.testBeforeRegistryAdd = func() {
		select {
		case registrySignals <- struct{}{}:
		default:
		}
	}
	surface.testAfterDiscoveryAdmission = func() {
		select {
		case admissionSignals <- struct{}{}:
		default:
		}
	}
	t.Cleanup(func() {
		coordinator.testBeforeRegistryAdd = nil
		surface.testAfterDiscoveryAdmission = nil
	})

	callContext, cancelCall := context.WithCancel(context.Background())
	cancelCall()
	call, err := client.CallTool(callContext, &mcp.CallToolParams{
		Name:      "search_tools",
		Arguments: json.RawMessage(`{"query":"new"}`),
	})
	if err == nil && (call == nil || !call.IsError) {
		t.Fatalf("pre-dispatch cancellation unexpectedly returned success: %#v", call)
	}
	select {
	case <-admissionSignals:
		t.Fatal("pre-dispatch cancellation reached the discovery handler")
	default:
	}
	assertProgressiveDiscoveryUnchanged(t, coordinator, owner, projectionBefore, unionBefore, registrySignals)
}
func TestSearchLateOwnerRevalidation(t *testing.T) {
	testProgressiveLateOwnerRevalidation(t, "search_tools")
}

func TestDescribeLateOwnerRevalidation(t *testing.T) {
	testProgressiveLateOwnerRevalidation(t, "describe_tool")
}

func testProgressiveLateOwnerRevalidation(t *testing.T, operation string) {
	t.Helper()
	for _, scenario := range []struct {
		name     string
		mutation string
	}{
		{name: "publication between admission and completion", mutation: "publication"},
		{name: "owner retirement between admission and completion", mutation: "retirement"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			upstream := &progressiveTestUpstream{}
			runtime := NewRuntime()
			deps := progressiveTestDeps(upstream)
			assembly, err := runtime.Reload(progressiveTestConfig(t, "old"), deps)
			if err != nil {
				t.Fatalf("admit initial generation: %v", err)
			}
			candidate := progressiveTestConfig(t, "old")
			server := NewMCPServer()
			surface := progressiveToolSurface(t, runtime, server)
			if err := surface.Published(assembly); err != nil {
				t.Fatalf("publish initial pinned projection: %v", err)
			}
			const token = "progressive-late-revalidation-token"
			inbound := NewInboundHandler(
				NewStreamableHandler(server),
				[][32]byte{sha256.Sum256([]byte(token))},
				InboundOptions{ToolSurface: surface},
			)
			httpServer := httptest.NewServer(inbound)
			t.Cleanup(httpServer.Close)
			client := connectProgressiveDiscoveryClient(t, httpServer, token)
			if _, err := client.ListTools(context.Background(), nil); err != nil {
				t.Fatalf("initialize progressive session tool list: %v", err)
			}

			coordinator := surface.projectionCoordinator()
			coordinator.barrier.Lock()
			var owner projectionOwner
			for candidate := range coordinator.projections {
				if candidate.kind == projectionSession {
					owner = candidate
					break
				}
			}
			coordinator.barrier.Unlock()
			if owner.kind != projectionSession {
				t.Fatal("session owner was not admitted before discovery")
			}
			projectionsBefore, unionBefore := snapshotProgressiveCoordinatorState(t, coordinator)
			registrySignals := make(chan struct{}, 1)
			coordinator.testBeforeRegistryAdd = func() {
				select {
				case registrySignals <- struct{}{}:
				default:
				}
			}
			t.Cleanup(func() { coordinator.testBeforeRegistryAdd = nil })

			var hookCalls int
			var interleaveError string
			var baselineProjections map[projectionOwner]progressiveProjectionSnapshot
			var baselineUnion map[string]progressiveUnionEntrySnapshot
			surface.testAfterDiscoveryAdmission = func() {
				hookCalls++
				switch scenario.mutation {
				case "publication":
					if _, err := runtime.Reload(candidate, deps); err != nil {
						interleaveError = "publish replacement generation: " + err.Error()
						return
					}
				case "retirement":
					if !surface.retireLiveOwner(owner) {
						interleaveError = "retire session owner: owner was not live"
						return
					}
				}
				baselineProjections, baselineUnion = snapshotProgressiveCoordinatorState(t, coordinator)
			}
			t.Cleanup(func() { surface.testAfterDiscoveryAdmission = nil })

			toolName := operation
			arguments := json.RawMessage(`{"query":"new"}`)
			if operation == "describe_tool" {
				arguments = json.RawMessage(`{"tool_name":"alpha__new","catalog_revision":"` + assembly.CatalogRevision() + `"}`)
			}
			result, err := client.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      toolName,
				Arguments: arguments,
			})
			if err != nil {
				t.Fatalf("%s transport failed: %v", operation, err)
			}
			if result == nil || !result.IsError {
				t.Fatalf("%s returned success after %s: %#v", operation, scenario.mutation, result)
			}
			rejection, ok := result.StructuredContent.(map[string]any)
			if !ok || rejection["code"] != codeCatalogChanged ||
				rejection["message"] != "the catalog revision changed" ||
				rejection["execution_state"] != core.ExecutionNotStarted {
				t.Fatalf("%s rejection = %#v; want catalog_changed, exact message, and not-started execution", operation, result.StructuredContent)
			}
			if hookCalls != 1 {
				t.Errorf("%s post-work admission hook calls = %d; want 1", operation, hookCalls)
			}
			if interleaveError != "" {
				t.Fatal(interleaveError)
			}
			if baselineProjections == nil || baselineUnion == nil {
				t.Fatal("interleaving baseline was not captured after the mutation")
			}
			if scenario.mutation == "publication" &&
				(!reflect.DeepEqual(projectionsBefore, baselineProjections) || !reflect.DeepEqual(unionBefore, baselineUnion)) {
				t.Error("publication changed projections, interests, or registry union")
			}
			projectionsAfter, unionAfter := snapshotProgressiveCoordinatorState(t, coordinator)
			if !reflect.DeepEqual(projectionsAfter, baselineProjections) || !reflect.DeepEqual(unionAfter, baselineUnion) {
				t.Error("handler changed projections, interests, or registry union after the interleaved mutation")
			}
			if len(registrySignals) != 0 {
				t.Errorf("%s rejection triggered %d SDK registry additions", operation, len(registrySignals))
			}
		})
	}
}

func snapshotProgressiveClientTools(t *testing.T, client *mcp.ClientSession) map[string][]byte {
	t.Helper()
	tools := make(map[string][]byte)
	var cursor string
	for {
		var params *mcp.ListToolsParams
		if cursor != "" {
			params = &mcp.ListToolsParams{Cursor: cursor}
		}
		page, err := client.ListTools(context.Background(), params)
		if err != nil {
			t.Fatalf("tools/list: %v", err)
		}
		if page == nil {
			t.Fatal("tools/list returned a nil page")
		}
		for _, tool := range page.Tools {
			descriptor, err := json.Marshal(tool)
			if err != nil {
				t.Fatalf("marshal tool %q: %v", tool.Name, err)
			}
			if _, exists := tools[tool.Name]; exists {
				t.Fatalf("tools/list returned duplicate tool %q", tool.Name)
			}
			tools[tool.Name] = descriptor
		}
		if page.NextCursor == "" {
			return tools
		}
		cursor = page.NextCursor
	}
}

func snapshotProgressiveCoordinatorState(
	t *testing.T,
	coordinator *projectionCoordinator,
) (map[projectionOwner]progressiveProjectionSnapshot, map[string]progressiveUnionEntrySnapshot) {
	t.Helper()
	coordinator.barrier.Lock()
	defer coordinator.barrier.Unlock()

	projections := make(map[projectionOwner]progressiveProjectionSnapshot, len(coordinator.projections))
	for owner, projection := range coordinator.projections {
		snapshot, err := snapshotProgressiveProjection(projection)
		if err != nil {
			t.Fatalf("snapshot projection for owner %#v: %v", owner, err)
		}
		projections[owner] = snapshot
	}
	union, err := snapshotProgressiveUnion(coordinator.union)
	if err != nil {
		t.Fatalf("snapshot projection union: %v", err)
	}
	return projections, union
}

type progressiveProjectedToolSnapshot struct {
	target        logicalToolTarget
	binding       *core.Binding
	inputSchema   json.RawMessage
	descriptor    []byte
	descriptorKey []byte
}

type progressiveProjectionSnapshot struct {
	present           bool
	owner             projectionOwner
	generation        uint64
	runtimeGeneration uint64
	tools             [][]byte
	byName            map[string]progressiveProjectedToolSnapshot
	interests         map[logicalToolTarget]struct{}
}

type progressiveUnionEntrySnapshot struct {
	present       bool
	descriptor    []byte
	descriptorKey []byte
	target        logicalToolTarget
	references    map[projectionOwner]struct{}
}

func snapshotProgressiveDescriptor(descriptor *mcp.Tool) ([]byte, error) {
	if descriptor == nil {
		return nil, nil
	}
	return json.Marshal(descriptor)
}

func snapshotProgressiveProjection(projection *toolProjection) (progressiveProjectionSnapshot, error) {
	if projection == nil {
		return progressiveProjectionSnapshot{}, nil
	}
	snapshot := progressiveProjectionSnapshot{
		present:           true,
		owner:             projection.owner,
		generation:        projection.generation,
		runtimeGeneration: projection.runtimeGeneration,
	}
	if projection.tools != nil {
		snapshot.tools = make([][]byte, len(projection.tools))
		for index, descriptor := range projection.tools {
			copied, err := snapshotProgressiveDescriptor(descriptor)
			if err != nil {
				return progressiveProjectionSnapshot{}, err
			}
			snapshot.tools[index] = copied
		}
	}
	if projection.byName != nil {
		snapshot.byName = make(map[string]progressiveProjectedToolSnapshot, len(projection.byName))
		for name, tool := range projection.byName {
			descriptor, err := snapshotProgressiveDescriptor(tool.descriptor)
			if err != nil {
				return progressiveProjectionSnapshot{}, err
			}
			snapshot.byName[name] = progressiveProjectedToolSnapshot{
				target:        tool.target,
				binding:       snapshotProgressiveBinding(tool.binding),
				inputSchema:   json.RawMessage(cloneProgressiveBytes(tool.inputSchema)),
				descriptor:    descriptor,
				descriptorKey: cloneProgressiveBytes(tool.descriptorKey),
			}
		}
	}
	if projection.interests != nil {
		snapshot.interests = make(map[logicalToolTarget]struct{}, len(projection.interests))
		for target := range projection.interests {
			snapshot.interests[target] = struct{}{}
		}
	}
	return snapshot, nil
}

func snapshotProgressiveBinding(binding *core.Binding) *core.Binding {
	if binding == nil {
		return nil
	}
	snapshot := *binding
	snapshot.Def.InputSchema = json.RawMessage(cloneProgressiveBytes(binding.Def.InputSchema))
	snapshot.Def.OutputSchema = json.RawMessage(cloneProgressiveBytes(binding.Def.OutputSchema))
	snapshot.Def.Metadata.Icons = json.RawMessage(cloneProgressiveBytes(binding.Def.Metadata.Icons))
	snapshot.Def.Metadata.Annotations = json.RawMessage(cloneProgressiveBytes(binding.Def.Metadata.Annotations))
	snapshot.Def.Metadata.Meta = json.RawMessage(cloneProgressiveBytes(binding.Def.Metadata.Meta))
	return &snapshot
}

func cloneProgressiveBytes(source []byte) []byte {
	if source == nil {
		return nil
	}
	snapshot := make([]byte, len(source))
	copy(snapshot, source)
	return snapshot
}

func snapshotProgressiveUnion(union map[string]*unionTool) (map[string]progressiveUnionEntrySnapshot, error) {
	if union == nil {
		return nil, nil
	}
	snapshot := make(map[string]progressiveUnionEntrySnapshot, len(union))
	for name, entry := range union {
		if entry == nil {
			snapshot[name] = progressiveUnionEntrySnapshot{}
			continue
		}
		descriptor, err := snapshotProgressiveDescriptor(entry.descriptor)
		if err != nil {
			return nil, err
		}
		references := make(map[projectionOwner]struct{}, len(entry.references))
		for owner := range entry.references {
			references[owner] = struct{}{}
		}
		if entry.references == nil {
			references = nil
		}
		snapshot[name] = progressiveUnionEntrySnapshot{
			present:       true,
			descriptor:    descriptor,
			descriptorKey: cloneProgressiveBytes(entry.descriptorKey),
			target:        entry.target,
			references:    references,
		}
	}
	return snapshot, nil
}

func assertProgressiveDiscoveryUnchanged(
	t *testing.T,
	coordinator *projectionCoordinator,
	owner projectionOwner,
	projectionBefore progressiveProjectionSnapshot,
	unionBefore map[string]progressiveUnionEntrySnapshot,
	registrySignals <-chan struct{},
) {
	t.Helper()
	coordinator.barrier.Lock()
	projectionAfter, projectionSnapshotErr := snapshotProgressiveProjection(coordinator.projections[owner])
	unionAfter, unionSnapshotErr := snapshotProgressiveUnion(coordinator.union)
	coordinator.barrier.Unlock()
	if projectionSnapshotErr != nil {
		t.Fatalf("snapshot owner projection after failed discovery: %v", projectionSnapshotErr)
	}
	if unionSnapshotErr != nil {
		t.Fatalf("snapshot projection union after failed discovery: %v", unionSnapshotErr)
	}
	if !reflect.DeepEqual(projectionAfter, projectionBefore) {
		t.Error("failed progressive discovery changed the affected owner projection")
	}
	if !reflect.DeepEqual(unionAfter, unionBefore) {
		t.Error("failed progressive discovery changed the projection union")
	}
	select {
	case <-registrySignals:
		t.Error("failed progressive discovery signaled an SDK registry change")
	default:
	}
}

func connectProgressiveDiscoveryClient(t *testing.T, server *httptest.Server, token string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "progressive-discovery-client", Version: "1"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint: server.URL + "/mcp",
		HTTPClient: &http.Client{Transport: progressiveDiscoveryAuthTransport{
			base:  http.DefaultTransport,
			token: token,
		}},
		MaxRetries: -1,
	}
	session, err := client.Connect(context.Background(), transport, &mcp.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
	if err != nil {
		t.Fatalf("connect progressive discovery client: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

type progressiveDiscoveryAuthTransport struct {
	base  http.RoundTripper
	token string
}

func (transport progressiveDiscoveryAuthTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	authorized := request.Clone(request.Context())
	authorized.Header = request.Header.Clone()
	authorized.Header.Set("Authorization", "Bearer "+transport.token)
	return transport.base.RoundTrip(authorized)
}
