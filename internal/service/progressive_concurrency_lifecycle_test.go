package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
)

func TestProgressiveMetadataConcurrencyAndLifecycle(t *testing.T) {
	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{
			name: "session enrollment, carrier preservation, DELETE, and lazy idle retirement",
			run:  TestProgressiveMetadataSessionEnrollment,
		},
		{
			name: "canonical exposure carrier header and request identity handling",
			run:  TestProgressiveMetadataSessionCarrier,
		},
		{
			name: "dynamic discovery survives concurrent commits and lifecycle errors",
			run:  TestProgressiveMetadataDiscoveryCommit,
		},
		{
			name: "discovery failures and cancellation preserve committed interests",
			run:  TestProgressiveMetadataDiscoveryFailuresDoNotCommit,
		},
		{
			name: "canceled discovery does not roll back a completed commit",
			run:  TestProgressiveMetadataDiscoveryCancellationKeepsCommit,
		},
		{
			name: "accepted generation publication is atomic and rebases newer state",
			run:  TestProgressiveMetadataProjectionPublication,
		},
		{
			name: "pin and dynamic transitions retain one complete shared union",
			run:  TestProgressiveMetadataUnionReferences,
		},
		{
			name: "departed targets lose interest and do not return on reappearance",
			run:  TestProgressiveMetadataDepartureInterestRemoval,
		},
		{
			name: "catalog departure unregisters dynamic interest without restoring it",
			run:  TestProgressiveMetadataCatalogDepartureAndReappearance,
		},
		{
			name: "large descriptor traversal covers every paginated tool",
			run:  TestProgressiveMetadataPaginationLargeDescriptorTraversal,
		},
		{
			name: "unpin transition preserves effective metadata and source descriptors",
			run:  TestProgressiveMetadataUnpinPreservesEffectiveMetadata,
		},
		{
			name: "captured direct execution survives projection and runtime changes",
			run:  TestProgressiveMetadataCapturedDirectExecution,
		},
		{
			name: "direct admission releases leases after cancellation, errors, and panic",
			run:  TestProgressiveMetadataDirectLeaseOwnership,
		},
		{
			name: "final dynamic target release removes union and owner references",
			run:  TestProgressiveMetadataFinalUnionRelease,
		},
		{
			name: "DELETE races reject unadmitted requests and preserve leased direct calls",
			run:  TestProgressiveMetadataRetirementRaces,
		},
		{
			name: "idle expiry prunes owner state and rejects delayed publication",
			run:  TestProgressiveMetadataIdleExpiryPruning,
		},
	}
	for _, test := range tests {
		t.Run(test.name, test.run)
	}
}

const progressiveLifecycleWaitTimeout = 5 * time.Second

func assertProgressiveOwnerPruned(t *testing.T, surface *ToolSurface, owner projectionOwner) {
	t.Helper()
	coordinator := surface.projectionCoordinator()
	coordinator.barrier.Lock()
	_, live := coordinator.liveOwners[owner]
	_, projection := coordinator.projections[owner]
	referenced := false
	for _, owners := range coordinator.interestReferences {
		if _, found := owners[owner]; found {
			referenced = true
			break
		}
	}
	for _, entry := range coordinator.union {
		if _, found := entry.references[owner]; found {
			referenced = true
			break
		}
	}
	coordinator.barrier.Unlock()
	if live || projection || referenced {
		t.Fatalf("retired owner remains live=%t projected=%t referenced=%t", live, projection, referenced)
	}
}

func TestProgressiveMetadataRetirementRaces(t *testing.T) {
	t.Run("post-stamp request barrier cannot resurrect a deleted session", func(t *testing.T) {
		harness := newProgressiveSessionHarness(t)
		sessionID := harness.initialize(t)
		initialList := harness.send(t, http.MethodPost, sessionID,
			`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`, nil)
		if status := initialList.Response.StatusCode; status < http.StatusOK || status >= http.StatusMultipleChoices {
			t.Fatalf("initial tools/list status = %d, want 2xx", status)
		}
		owner := harness.owner(t, sessionID)
		bindings := harness.assembly.Catalog().Bindings()
		if len(bindings) == 0 {
			t.Fatal("session assembly has no tool binding to seed owner interest")
		}
		target := logicalToolTarget{serverID: bindings[0].ServerID, rawName: bindings[0].RawName}
		harness.addInterest(owner, target)
		if err := harness.surface.publishOwnerProjection(owner, harness.assembly); err != nil {
			t.Fatalf("publish initial owner projection: %v", err)
		}
		assertProgressiveOwnerInterest(t, harness.surface, owner, target, true)

		stampedHeaders := make(chan [2]string, 1)
		next := harness.inbound.next
		harness.inbound.next = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				stampedHeaders <- [2]string{
					r.Header.Get("X-Mcp-Bridge-Exposure-Epoch"),
					r.Header.Get("Mcp-Session-Id"),
				}
			}
			next.ServeHTTP(w, r)
		})

		carrierResponse := harness.send(t, http.MethodPost, sessionID,
			`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`, nil)
		if status := carrierResponse.Response.StatusCode; status < http.StatusOK || status >= http.StatusMultipleChoices {
			t.Fatalf("carrier tools/list status = %d, want 2xx", status)
		}
		var carrier [2]string
		select {
		case carrier = <-stampedHeaders:
		case <-time.After(progressiveLifecycleWaitTimeout):
			t.Fatal("tools/list did not reach the post-stamp barrier")
		}
		if carrier[0] == "" {
			t.Error("X-Mcp-Bridge-Exposure-Epoch header is empty")
		}
		if carrier[1] != sessionID {
			t.Errorf("Mcp-Session-Id = %q, want %q", carrier[1], sessionID)
		}
		harness.inbound.next = next

		deleteResponse := harness.send(t, http.MethodDelete, sessionID, "", nil)
		if status := deleteResponse.Response.StatusCode; status < http.StatusOK || status >= http.StatusMultipleChoices {
			t.Fatalf("DELETE status = %d, want 2xx", status)
		}
		assertProgressiveOwnerPruned(t, harness.surface, owner)

		staleSessionID := harness.initialize(t)
		staleHeaders := make(chan [2]string, 1)
		next = harness.inbound.next
		harness.inbound.next = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.Header.Get("Mcp-Session-Id") == staleSessionID {
				r = r.Clone(r.Context())
				r.Header = r.Header.Clone()
				r.Header.Set(exposureEpochHeaderName, carrier[0])
				staleHeaders <- [2]string{
					r.Header.Get(exposureEpochHeaderName),
					r.Header.Get("Mcp-Session-Id"),
				}
			}
			next.ServeHTTP(w, r)
		})
		admissionReached := make(chan struct{}, 1)
		resume := make(chan struct{})
		harness.surface.testBeforeExposureAdmission = func() {
			admissionReached <- struct{}{}
			select {
			case <-resume:
			case <-time.After(progressiveLifecycleWaitTimeout):
				t.Error("timed out waiting to resume stale-epoch request")
			}
		}
		defer func() {
			harness.surface.testBeforeExposureAdmission = nil
			harness.inbound.next = next
			select {
			case <-resume:
			default:
				close(resume)
			}
		}()

		staleResponse := make(chan progressiveHTTPResponse, 1)
		go func() {
			staleResponse <- harness.send(t, http.MethodPost, staleSessionID,
				`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`, nil)
		}()
		select {
		case header := <-staleHeaders:
			if header[0] != carrier[0] {
				t.Errorf("stale request epoch = %q, want deleted session epoch %q", header[0], carrier[0])
			}
			if header[1] != staleSessionID {
				t.Errorf("stale request session ID = %q, want %q", header[1], staleSessionID)
			}
		case <-time.After(progressiveLifecycleWaitTimeout):
			t.Fatal("tools/list did not carry the deleted session epoch")
		}
		select {
		case <-admissionReached:
		case <-time.After(progressiveLifecycleWaitTimeout):
			t.Fatal("stale-epoch tools/list did not reach the pre-admission barrier")
		}
		assertProgressiveOwnerPruned(t, harness.surface, owner)
		close(resume)

		var response progressiveHTTPResponse
		select {
		case response = <-staleResponse:
		case <-time.After(progressiveLifecycleWaitTimeout):
			t.Fatal("stale-epoch tools/list did not finish after resuming")
		}
		if response.Response.StatusCode != http.StatusOK {
			t.Fatalf("stale-epoch tools/list status = %d, want 200", response.Response.StatusCode)
		}
		var rpcResponse struct {
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		payload := response.body
		if dataStart := bytes.Index(payload, []byte("data: ")); dataStart >= 0 {
			payload = payload[dataStart+len("data: "):]
			if end := bytes.IndexByte(payload, '\n'); end >= 0 {
				payload = payload[:end]
			}
		}
		if err := json.Unmarshal(payload, &rpcResponse); err != nil {
			t.Fatalf("decode stale-epoch request response: %v", err)
		}
		if rpcResponse.Error == nil || rpcResponse.Error.Code != -32600 ||
			rpcResponse.Error.Message != "invalid exposure request" {
			t.Fatalf("stale-epoch error = %+v; want invalid-request JSON-RPC error", rpcResponse.Error)
		}
		assertProgressiveOwnerPruned(t, harness.surface, owner)

		subsequentResponse := harness.send(t, http.MethodPost, sessionID,
			`{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`, nil)
		if status := subsequentResponse.Response.StatusCode; status != http.StatusOK {
			t.Fatalf("post-DELETE tools/list status = %d, want 200 after rehydration", status)
		}
		if len(subsequentResponse.body) == 0 || !bytes.Contains(subsequentResponse.body, []byte(`"result"`)) ||
			bytes.Contains(subsequentResponse.body, []byte(`"error"`)) {
			t.Fatalf("post-DELETE tools/list did not return a successful result: %s", subsequentResponse.body)
		}
	})

	t.Run("already-leased direct call completes and releases its lease", func(t *testing.T) {
		upstream := &directExecutionUpstream{
			calls:   make(chan directExecutionCall, 1),
			started: make(chan struct{}),
			release: make(chan struct{}),
			result:  json.RawMessage(`{"content":[{"type":"text","text":"lease-result"}]}`),
		}
		harness, sessionID, _, _, projected, assembly := newDirectExecutionFixture(t, upstream)
		releaseUpstream := func() {
			select {
			case <-upstream.release:
			default:
				close(upstream.release)
			}
		}
		defer releaseUpstream()

		server, found := assembly.Server(projected.target.serverID)
		if !found {
			t.Fatalf("assembly has no server %q", projected.target.serverID)
		}
		callResponse := make(chan progressiveHTTPResponse, 1)
		go func() {
			callResponse <- harness.send(t, http.MethodPost, sessionID,
				directCallBody(projected.descriptor.Name, `{}`), nil)
		}()
		select {
		case <-upstream.calls:
		case <-time.After(progressiveLifecycleWaitTimeout):
			t.Fatal("direct call did not reach the blocking upstream")
		}
		select {
		case <-upstream.started:
		case <-time.After(progressiveLifecycleWaitTimeout):
			t.Fatal("direct upstream call did not start")
		}
		if leases := trackerLeaseCount(server.tracker); leases != 1 {
			t.Fatalf("active direct call lease count = %d, want 1", leases)
		}

		deleteEntered := make(chan struct{}, 1)
		next := harness.inbound.next
		harness.inbound.next = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete {
				deleteEntered <- struct{}{}
			}
			next.ServeHTTP(w, r)
		})
		deleteResponseChan := make(chan progressiveHTTPResponse, 1)
		go func() {
			deleteResponseChan <- harness.send(t, http.MethodDelete, sessionID, "", nil)
		}()
		select {
		case <-deleteEntered:
		case <-time.After(progressiveLifecycleWaitTimeout):
			t.Fatal("DELETE did not enter the inbound handler")
		}
		if leases := trackerLeaseCount(server.tracker); leases != 1 {
			t.Fatalf("active direct call lease count during DELETE = %d, want 1", leases)
		}

		releaseUpstream()
		var response progressiveHTTPResponse
		select {
		case response = <-callResponse:
		case <-time.After(progressiveLifecycleWaitTimeout):
			t.Fatal("already-leased direct call did not finish after upstream release")
		}

		var deleteResponse progressiveHTTPResponse
		select {
		case deleteResponse = <-deleteResponseChan:
		case <-time.After(progressiveLifecycleWaitTimeout):
			t.Fatal("DELETE with active direct call did not finish")
		}
		if status := deleteResponse.Response.StatusCode; status < http.StatusOK || status >= http.StatusMultipleChoices {
			t.Fatalf("DELETE with active direct call status = %d, want 2xx", status)
		}
		if status := response.Response.StatusCode; status < http.StatusOK || status >= http.StatusMultipleChoices {
			t.Fatalf("leased direct call status = %d, want 2xx", status)
		}
		if len(response.body) != 0 {
			result := decodeDirectToolResponse(t, response.body)
			if result.Result.IsError {
				t.Fatal("already-leased direct call returned an error")
			}
			if len(result.Result.Content) == 0 || result.Result.Content[0].Text != "lease-result" {
				t.Fatalf("already-leased direct call content = %#v, want lease-result", result.Result.Content)
			}
		}
		if leases := trackerLeaseCount(server.tracker); leases != 0 {
			t.Fatalf("direct call lease count after completion = %d, want 0", leases)
		}

		subsequentResponse := harness.send(t, http.MethodPost, sessionID,
			directCallBody(projected.descriptor.Name, `{}`), nil)
		if status := subsequentResponse.Response.StatusCode; status < http.StatusOK || status >= http.StatusMultipleChoices {
			t.Fatalf("subsequent direct call status = %d, want 2xx after rehydration", status)
		}
		if len(subsequentResponse.body) == 0 {
			t.Fatal("rehydrated direct call returned no MCP response")
		}
		secondResult := decodeDirectToolResponse(t, subsequentResponse.body)
		if secondResult.Result.IsError || len(secondResult.Result.Content) == 0 || secondResult.Result.Content[0].Text != "lease-result" {
			t.Fatalf("rehydrated direct call content = %#v, want lease-result", secondResult.Result.Content)
		}
		select {
		case secondCall := <-upstream.calls:
			if secondCall.rawName != projected.binding.RawName {
				t.Fatalf("second upstream tool = %q, want %q", secondCall.rawName, projected.binding.RawName)
			}
		default:
			t.Fatal("rehydrated direct call did not produce the second upstream call")
		}
	})
}

func TestProgressiveMetadataIdleExpiryPruning(t *testing.T) {
	harness := newProgressiveSessionHarness(t)
	sessionID := harness.initialize(t)
	initialList := harness.send(t, http.MethodPost, sessionID,
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`, nil)
	if status := initialList.Response.StatusCode; status < http.StatusOK || status >= http.StatusMultipleChoices {
		t.Fatalf("initial tools/list status = %d, want 2xx", status)
	}
	owner := harness.owner(t, sessionID)
	bindings := harness.assembly.Catalog().Bindings()
	if len(bindings) == 0 {
		t.Fatal("session assembly has no tool binding to seed owner interest")
	}
	target := logicalToolTarget{serverID: bindings[0].ServerID, rawName: bindings[0].RawName}
	harness.addInterest(owner, target)
	if err := harness.surface.publishOwnerProjection(owner, harness.assembly); err != nil {
		t.Fatalf("publish initial owner projection: %v", err)
	}

	*harness.clock = harness.base.Add(2 * time.Second)
	response := harness.send(t, http.MethodPost, sessionID,
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`, nil)
	if status := response.Response.StatusCode; status != http.StatusOK {
		t.Fatalf("expired-session rehydration status = %d, want 200 (body=%s)", status, response.body)
	}
	assertProgressiveOwnerPruned(t, harness.surface, owner)
	if err := harness.surface.publishOwnerProjection(owner, harness.assembly); err == nil {
		t.Fatal("delayed publication for expired owner was accepted")
	}
	assertProgressiveOwnerPruned(t, harness.surface, owner)
}

func TestProgressiveMetadataFinalUnionRelease(t *testing.T) {
	upstream := &directExecutionUpstream{calls: make(chan directExecutionCall, 1)}
	harness, session, owner, _, _, assembly := newDirectExecutionFixture(t, upstream)
	dynamicTarget := logicalToolTarget{serverID: "alpha", rawName: "new"}
	harness.addInterest(owner, dynamicTarget)
	if err := harness.surface.publishOwnerProjection(owner, assembly); err != nil {
		t.Fatalf("publish dynamic owner interest: %v", err)
	}

	coordinator := harness.surface.projectionCoordinator()
	dynamicName := ""
	coordinator.barrier.Lock()
	for name, entry := range coordinator.union {
		if entry.target == dynamicTarget {
			dynamicName = name
			if _, referenced := entry.references[owner]; !referenced {
				coordinator.barrier.Unlock()
				t.Fatalf("dynamic union entry %q lacks its owner reference", name)
			}
		}
	}
	coordinator.barrier.Unlock()
	if dynamicName == "" {
		t.Fatal("published dynamic target did not enter the handler union")
	}

	if err := harness.surface.publishOwnerProjection(owner, assembly); err != nil {
		t.Fatalf("republish unchanged owner projection: %v", err)
	}
	repeatedNames := 0
	repeatedNameMatches := false
	repeatedOwnerReference := false
	coordinator.barrier.Lock()
	for name, entry := range coordinator.union {
		if entry.target != dynamicTarget {
			continue
		}
		repeatedNames++
		hasOwnerReference := false
		_, hasOwnerReference = entry.references[owner]
		repeatedNameMatches = repeatedNameMatches || name == dynamicName
		repeatedOwnerReference = repeatedOwnerReference || name == dynamicName && hasOwnerReference
	}
	coordinator.barrier.Unlock()
	if repeatedNames != 1 || !repeatedNameMatches || !repeatedOwnerReference {
		t.Fatalf("unchanged owner projection published %d dynamic entries with stable name=%t owner reference=%t; want one stable entry",
			repeatedNames, repeatedNameMatches, repeatedOwnerReference)
	}

	if response := harness.send(t, http.MethodDelete, session, "", nil); response.StatusCode >= http.StatusInternalServerError {
		t.Fatalf("DELETE status = %d, want successful owner retirement", response.StatusCode)
	}
	harness.assertRetired(t, owner)
	// A delayed publication from the retired incarnation must not restore its
	// dynamic target or owner references.
	harness.surface.publishOwnerProjection(owner, assembly)

	dynamicStillRegistered := false
	retiredOwnerRetained := false
	coordinator.barrier.Lock()
	for _, entry := range coordinator.union {
		dynamicStillRegistered = dynamicStillRegistered || entry.target == dynamicTarget
		_, referenced := entry.references[owner]
		retiredOwnerRetained = retiredOwnerRetained || referenced
	}
	for _, references := range coordinator.interestReferences {
		_, referenced := references[owner]
		retiredOwnerRetained = retiredOwnerRetained || referenced
	}
	coordinator.barrier.Unlock()
	if dynamicStillRegistered || retiredOwnerRetained {
		t.Fatalf("retired dynamic target %q remains registered=%t or referenced by its owner=%t",
			dynamicName, dynamicStillRegistered, retiredOwnerRetained)
	}
}
func assertProgressiveCatalogHasTarget(t *testing.T, assembly *Assembly, target logicalToolTarget, want bool) {
	t.Helper()
	found := false
	for _, binding := range assembly.Catalog().Bindings() {
		if binding.ServerID == target.serverID && binding.RawName == target.rawName {
			found = true
			break
		}
	}
	if found != want {
		t.Errorf("catalog target %s/%s present = %t; want %t", target.serverID, target.rawName, found, want)
	}
}

func assertProgressiveOwnerInterest(t *testing.T, surface *ToolSurface, owner projectionOwner, target logicalToolTarget, want bool) {
	t.Helper()
	coordinator := surface.projectionCoordinator()
	coordinator.barrier.Lock()
	projection := coordinator.projections[owner]
	projectionInterest := false
	if projection != nil {
		_, projectionInterest = projection.interests[target]
	}
	ownerReference := false
	if references := coordinator.interestReferences[target]; references != nil {
		_, ownerReference = references[owner]
	}
	unionOwnerReference := false
	for _, entry := range coordinator.union {
		if entry.target != target {
			continue
		}
		_, unionOwnerReference = entry.references[owner]
		if unionOwnerReference {
			break
		}
	}
	coordinator.barrier.Unlock()
	if projectionInterest != want || ownerReference != want || unionOwnerReference != want {
		t.Errorf("dynamic interest for %s/%s: projection=%t reference=%t union reference=%t; want %t",
			target.serverID, target.rawName, projectionInterest, ownerReference, unionOwnerReference, want)
	}
}

func TestProgressiveMetadataCatalogDepartureAndReappearance(t *testing.T) {
	harness, _, owner, _, _, _ := newDirectExecutionFixture(t, &directExecutionUpstream{calls: make(chan directExecutionCall, 1)})
	runtime := harness.surface.runtime
	surface := harness.surface
	toolDefinition := func(rawName string) core.ToolDefinition {
		return core.ToolDefinition{
			RawName:     rawName,
			Description: rawName,
			InputSchema: []byte(`{"type":"object"}`),
		}
	}
	availableTools := []core.ToolDefinition{toolDefinition("old"), toolDefinition("new")}
	deps := progressiveTestDeps(&progressiveTestUpstream{})
	deps.Discover = func(config.Server) ([]core.ToolDefinition, error) {
		return availableTools, nil
	}

	initial, err := runtime.Reload(progressiveTestConfig(t, "old"), deps)
	if err != nil {
		t.Fatalf("admit initial catalog: %v", err)
	}
	if err := surface.Published(initial); err != nil {
		t.Fatalf("publish initial catalog: %v", err)
	}
	target := logicalToolTarget{serverID: "alpha", rawName: "new"}
	assertProgressiveCatalogHasTarget(t, initial, target, true)
	if err := surface.commitDiscovery(owner, initial, map[logicalToolTarget]struct{}{target: {}}); err != nil {
		t.Fatalf("commit discovered dynamic target: %v", err)
	}
	assertProgressiveOwnerInterest(t, surface, owner, target, true)

	availableTools = []core.ToolDefinition{toolDefinition("old")}
	departed, err := runtime.Reload(progressiveTestConfig(t, "old"), deps)
	if err != nil {
		t.Fatalf("reload after dynamic target departure: %v", err)
	}
	if err := surface.Published(departed); err != nil {
		t.Fatalf("publish catalog departure: %v", err)
	}
	assertProgressiveCatalogHasTarget(t, departed, target, false)
	assertProgressiveOwnerInterest(t, surface, owner, target, false)

	availableTools = append(availableTools, toolDefinition("new"))
	restored, err := runtime.Reload(progressiveTestConfig(t, "old"), deps)
	if err != nil {
		t.Fatalf("reload after dynamic target reappearance: %v", err)
	}
	if err := surface.Published(restored); err != nil {
		t.Fatalf("publish catalog reappearance: %v", err)
	}
	assertProgressiveCatalogHasTarget(t, restored, target, true)
	assertProgressiveOwnerInterest(t, surface, owner, target, false)
}

func TestProgressiveMetadataPaginationLargeDescriptorTraversal(t *testing.T) {
	const descriptorCount = 1050
	definitions := make([]core.ToolDefinition, descriptorCount)
	interests := make(map[logicalToolTarget]struct{}, descriptorCount)
	for index := range definitions {
		rawName := fmt.Sprintf("tool_%04d", index)
		definitions[index] = core.ToolDefinition{
			RawName:     rawName,
			Description: rawName,
			InputSchema: []byte(`{"type":"object"}`),
		}
		interests[logicalToolTarget{serverID: "alpha", rawName: rawName}] = struct{}{}
	}

	harness, _, owner, _, _, _ := newDirectExecutionFixture(t, &directExecutionUpstream{calls: make(chan directExecutionCall, 1)})
	deps := progressiveTestDeps(&progressiveTestUpstream{})
	deps.Discover = func(config.Server) ([]core.ToolDefinition, error) {
		return definitions, nil
	}
	assembly, err := harness.surface.runtime.Reload(progressiveTestConfig(t, "tool_0000"), deps)
	if err != nil {
		t.Fatalf("assemble large test catalog: %v", err)
	}
	if got := len(assembly.Catalog().Bindings()); got != descriptorCount {
		t.Fatalf("catalog contains %d descriptors; want %d", got, descriptorCount)
	}

	surface := harness.surface
	if err := surface.Published(assembly); err != nil {
		t.Fatalf("publish large test catalog: %v", err)
	}
	if err := surface.commitDiscovery(owner, assembly, interests); err != nil {
		t.Fatalf("publish large descriptor projection: %v", err)
	}
	coordinator := surface.projectionCoordinator()
	coordinator.barrier.Lock()
	projection := coordinator.projections[owner]
	coordinator.barrier.Unlock()
	if projection == nil {
		t.Fatal("large descriptor projection was not published")
	}

	builtinTools := surface.builtinTools
	surface.builtinTools = nil
	surface.pageSize = 100
	cursor := ""
	finalCursor := "not-final"
	pageCount := 0
	names := make([]string, 0, descriptorCount)
	for {
		page, err := surface.ownerToolListPage(projection, cursor)
		if err != nil {
			t.Fatalf("list page %d: %v", pageCount+1, err)
		}
		pageCount++
		for _, tool := range page.Tools {
			names = append(names, tool.Name)
		}
		finalCursor = page.NextCursor
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if len(names) != descriptorCount {
		t.Fatalf("collected %d descriptors; want %d", len(names), descriptorCount)
	}
	if pageCount < 2 {
		t.Fatalf("traversed %d page(s); want multiple pages", pageCount)
	}
	if finalCursor != "" {
		t.Fatalf("final page next cursor = %q; want empty", finalCursor)
	}
	expectedNames := make(map[string]struct{}, descriptorCount)
	for index, definition := range definitions {
		name := core.PublicName("alpha", definition.RawName)
		if index == 0 {
			name = config.WireToolName("alpha", definition.RawName)
		}
		expectedNames[name] = struct{}{}
	}
	if len(expectedNames) != descriptorCount {
		t.Fatalf("expected-name set contains %d descriptors; want %d", len(expectedNames), descriptorCount)
	}
	seenNames := make(map[string]struct{}, descriptorCount)
	for index, name := range names {
		if index > 0 && name <= names[index-1] {
			t.Fatalf("descriptor names are not strictly ordered at index %d: %q after %q", index, name, names[index-1])
		}
		if _, found := expectedNames[name]; !found {
			t.Fatalf("unexpected descriptor name at index %d: %q", index, name)
		}
		if _, duplicate := seenNames[name]; duplicate {
			t.Fatalf("duplicate descriptor name at index %d: %q", index, name)
		}
		seenNames[name] = struct{}{}
	}
	if len(seenNames) != descriptorCount {
		t.Fatalf("collected %d unique descriptor names; want %d", len(seenNames), descriptorCount)
	}
	surface.builtinTools = builtinTools
	surface.pageSize = 0
	if len(builtinTools) != 3 {
		t.Fatalf("surface has %d built-in tools; want 3", len(builtinTools))
	}
	firstPage, err := surface.ownerToolListPage(projection, "")
	if err != nil {
		t.Fatalf("list default-size page 1: %v", err)
	}
	if len(firstPage.Tools) != 1000 {
		t.Fatalf("default-size page 1 contains %d tools; want 1000", len(firstPage.Tools))
	}
	if firstPage.NextCursor == "" {
		t.Fatal("default-size page 1 has no next cursor")
	}
	secondPage, err := surface.ownerToolListPage(projection, firstPage.NextCursor)
	if err != nil {
		t.Fatalf("list default-size page 2: %v", err)
	}
	if len(secondPage.Tools) != 53 {
		t.Fatalf("default-size page 2 contains %d tools; want 53", len(secondPage.Tools))
	}
	if secondPage.NextCursor != "" {
		t.Fatalf("default-size page 2 next cursor = %q; want empty", secondPage.NextCursor)
	}

	expectedDefaultNames := make(map[string]struct{}, descriptorCount+len(builtinTools))
	for name := range expectedNames {
		expectedDefaultNames[name] = struct{}{}
	}
	for _, tool := range builtinTools {
		if _, duplicate := expectedDefaultNames[tool.Name]; duplicate {
			t.Fatalf("built-in tool name %q duplicates an assembled descriptor", tool.Name)
		}
		expectedDefaultNames[tool.Name] = struct{}{}
	}
	if len(expectedDefaultNames) != 1053 {
		t.Fatalf("expected default-size traversal set contains %d tools; want 1053", len(expectedDefaultNames))
	}
	defaultNames := make([]string, 0, len(expectedDefaultNames))
	for _, tool := range firstPage.Tools {
		defaultNames = append(defaultNames, tool.Name)
	}
	for _, tool := range secondPage.Tools {
		defaultNames = append(defaultNames, tool.Name)
	}
	if len(defaultNames) != 1053 {
		t.Fatalf("default-size traversal collected %d tools; want 1053", len(defaultNames))
	}
	seenDefaultNames := make(map[string]struct{}, len(expectedDefaultNames))
	for index, name := range defaultNames {
		if index > 0 && name <= defaultNames[index-1] {
			t.Fatalf("default-size tool names are not strictly ordered at index %d: %q after %q", index, name, defaultNames[index-1])
		}
		if _, found := expectedDefaultNames[name]; !found {
			t.Fatalf("unexpected default-size tool name at index %d: %q", index, name)
		}
		if _, duplicate := seenDefaultNames[name]; duplicate {
			t.Fatalf("duplicate default-size tool name at index %d: %q", index, name)
		}
		seenDefaultNames[name] = struct{}{}
	}
	if len(seenDefaultNames) != len(expectedDefaultNames) {
		t.Fatalf("default-size traversal collected %d unique tools; want %d", len(seenDefaultNames), len(expectedDefaultNames))
	}
}
