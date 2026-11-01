package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

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
		if tool.Name == core.PublicName("alpha", "new") || tool.Name == core.PublicName("alpha", "shared") {
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
	releaseRegistry := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseRegistry) }) }
	var blockFirstRegistryAdd sync.Once
	coordinator.testBeforeRegistryAdd = func() {
		registryEntered <- struct{}{}
		blockFirstRegistryAdd.Do(func() { <-releaseRegistry })
	}
	defer func() {
		release()
		coordinator.testBeforeRegistryAdd = nil
	}()

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
	registryCount := 0
	deadline := time.After(5 * time.Second)
	for registryCount == 0 && len(finished) < 2 {
		select {
		case <-registryEntered:
			registryCount++
		case completedCall := <-completed:
			finished = append(finished, completedCall)
		case <-deadline:
			t.Fatal("timed out waiting for concurrent discovery preparation and completion")
		}
	}
	if registryCount > 0 {
		coordinator.barrier.Lock()
		committedCount := 0
		for _, rawName := range []string{"new", "shared"} {
			name := core.PublicName("alpha", rawName)
			if coordinator.union[name] == nil {
				continue
			}
			for owner, projection := range coordinator.projections {
				if owner.kind == projectionSession && projection != nil {
					if _, found := projection.byName[name]; found {
						committedCount++
						break
					}
				}
			}
		}
		coordinator.barrier.Unlock()
		if committedCount == 0 {
			t.Error("registry reconciliation paused before any projection and union commit")
		}
		select {
		case returned := <-completed:
			finished = append(finished, returned)
			t.Error("discovery returned success before SDK registry reconciliation completed")
		default:
		}
	}
	release()
	for len(finished) < 2 {
		select {
		case <-registryEntered:
			registryCount++
		case call := <-completed:
			finished = append(finished, call)
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for concurrent discovery calls after registry release")
		}
	}
	for len(registryEntered) > 0 {
		<-registryEntered
		registryCount++
	}
	if registryCount != 2 {
		t.Fatalf("concurrent discovery registry additions observed = %d; want 2", registryCount)
	}
	for _, call := range finished {
		if call.err != nil || call.call == nil || call.call.IsError {
			t.Errorf("%s discovery failed: result=%#v err=%v", call.name, call.call, call.err)
			continue
		}
		response := decodeSearch(call.call)
		if response.CatalogRevision != assembly.CatalogRevision() {
			t.Errorf("%s search revision = %q; want accepted %q", call.name, response.CatalogRevision, assembly.CatalogRevision())
		}
		if len(response.Results) == 0 {
			t.Errorf("%s discovery returned no target", call.name)
		}
	}

	coordinator.barrier.Lock()
	sameSessionProjectionHasBothTargets := false
	for owner, projection := range coordinator.projections {
		if owner.kind != projectionSession || projection == nil {
			continue
		}
		hasBothTargets := true
		for _, target := range []logicalToolTarget{
			{serverID: "alpha", rawName: "new"},
			{serverID: "alpha", rawName: "shared"},
		} {
			name := core.PublicName(target.serverID, target.rawName)
			if _, found := projection.byName[name]; !found {
				hasBothTargets = false
				break
			}
			if _, found := projection.interests[target]; !found {
				hasBothTargets = false
				break
			}
		}
		if hasBothTargets {
			sameSessionProjectionHasBothTargets = true
			break
		}
	}
	coordinator.barrier.Unlock()
	if !sameSessionProjectionHasBothTargets {
		t.Fatal("concurrent searches from one session did not retain both tools and interests in its projection")
	}

	rejectedName := core.PublicName("alpha", "new")
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
	select {
	case <-registryEntered:
	default:
		t.Error("revealing an existing union tool for session B did not signal a registry list change")
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
		page, err := check.client.ListTools(context.Background(), nil)
		if err != nil {
			t.Fatalf("tools/list after %s discovery: %v", check.rawName, err)
		}
		name := core.PublicName("alpha", check.rawName)
		found := false
		for _, tool := range page.Tools {
			if tool.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("successful %s discovery was not listed for its owner", check.rawName)
		}
		call, err := progressiveCallTool(t, check.client, assembly.CatalogRevision(), name, json.RawMessage(`{}`))
		if err != nil || call == nil || call.IsError {
			t.Errorf("generic call to discovered %s failed: result=%#v err=%v", check.rawName, call, err)
		}
	}
	if calls := upstream.callsSnapshot(); len(calls) != 2 {
		t.Errorf("discovery plus two direct calls made %d upstream operations; want only 2 calls", len(calls))
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
		ToolName:        core.PublicName("alpha", "new"),
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

func TestProgressiveMetadataDiscoveryCancellationKeepsCommit(t *testing.T) {
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
	coordinator.barrier.Unlock()
	if projectionSnapshotErr != nil {
		t.Fatalf("snapshot session projection: %v", projectionSnapshotErr)
	}
	if !projectionBefore.present {
		t.Fatal("session projection was not admitted before cancelled discovery")
	}
	targetName := core.PublicName("alpha", "new")
	if _, alreadyExposed := projectionBefore.byName[targetName]; alreadyExposed {
		t.Fatalf("cancellation target %q was already exposed", targetName)
	}

	registryAttempted := make(chan struct{}, 1)
	callContext, cancelCall := context.WithCancel(context.Background())
	defer cancelCall()
	coordinator.testBeforeRegistryAdd = func() {
		registryAttempted <- struct{}{}
		cancelCall()
	}
	t.Cleanup(func() { coordinator.testBeforeRegistryAdd = nil })
	call, err := client.CallTool(callContext, &mcp.CallToolParams{
		Name:      "search_tools",
		Arguments: json.RawMessage(`{"query":"new"}`),
	})
	if err == nil && (call == nil || !call.IsError) {
		t.Fatalf("postcommit cancellation was not reported as failure: %#v", call)
	}
	select {
	case <-registryAttempted:
	default:
		t.Fatal("cancelled discovery did not reach the postcommit registry-add hook")
	}

	coordinator.barrier.Lock()
	projectionAfter, projectionSnapshotErr := snapshotProgressiveProjection(coordinator.projections[owner])
	projectionContainsTarget := false
	if projectionAfter.present {
		_, projectionContainsTarget = projectionAfter.byName[targetName]
	}
	unionEntry := coordinator.union[targetName]
	unionContainsOwner := unionEntry != nil && hasProjectionReference(unionEntry, owner)
	_, interestContainsOwner := coordinator.interestReferences[logicalToolTarget{serverID: "alpha", rawName: "new"}][owner]
	coordinator.barrier.Unlock()
	if projectionSnapshotErr != nil {
		t.Fatalf("snapshot postcommit projection: %v", projectionSnapshotErr)
	}
	if !projectionAfter.present || reflect.DeepEqual(projectionAfter, projectionBefore) || !projectionContainsTarget {
		t.Errorf("postcommit cancellation removed the discovered tool from the owner projection")
	}
	if !unionContainsOwner || !interestContainsOwner {
		t.Errorf("postcommit cancellation removed the owner's union or discovery interest")
	}
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
