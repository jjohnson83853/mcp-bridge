package service

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/search"
)

const progressiveConfig = `schema_version: 1
servers:
  - id: alpha
    endpoint: https://alpha.invalid/mcp
    credentials:
      bearer: {env: PROGRESSIVE_ALPHA_SECRET}
client_auth:
  bearer_tokens:
    - {env: PROGRESSIVE_CLIENT_TOKEN}
pins:
  - {server: alpha, tool: %s}
`

type progressiveTestUpstream struct {
	mu    sync.Mutex
	calls []recordedProgressiveCall
}

type recordedProgressiveCall struct {
	serverID string
	rawName  string
}

func (u *progressiveTestUpstream) CallTool(_ context.Context, _, serverID, rawName string, _ json.RawMessage) (json.RawMessage, *core.UpstreamOutcome) {
	u.mu.Lock()
	u.calls = append(u.calls, recordedProgressiveCall{serverID: serverID, rawName: rawName})
	u.mu.Unlock()
	return json.RawMessage(`{"content":[{"type":"text","text":"ok"}],"isError":false}`), nil
}

func (u *progressiveTestUpstream) callsSnapshot() []recordedProgressiveCall {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]recordedProgressiveCall(nil), u.calls...)
}

func progressiveTestConfig(t *testing.T, pin string) *config.Config {
	t.Helper()
	t.Setenv("PROGRESSIVE_ALPHA_SECRET", "alpha-secret")
	t.Setenv("PROGRESSIVE_CLIENT_TOKEN", "client-token")
	cfg, err := config.Parse([]byte(fmt.Sprintf(progressiveConfig, pin)), "progressive-test.yaml")
	if err != nil {
		t.Fatalf("parse progressive test config: %v", err)
	}
	return cfg
}

func progressiveTestDeps(upstream core.Upstream) ReloadDeps {
	definitions := []core.ToolDefinition{
		{RawName: "old", Description: "old route", InputSchema: json.RawMessage(`{"type":"object"}`), Metadata: core.ToolMetadata{Title: "old descriptor"}},
		{RawName: "new", Description: "new route", InputSchema: json.RawMessage(`{"type":"object"}`), Metadata: core.ToolMetadata{Title: "new descriptor"}},
		{RawName: "shared", Description: "shared route", InputSchema: json.RawMessage(`{"type":"object"}`)},
	}
	return ReloadDeps{
		Discover: func(config.Server) ([]core.ToolDefinition, error) {
			return definitions, nil
		},
		NewClient: func(config.Server) (core.Upstream, error) {
			return upstream, nil
		},
		ModelIdentityDigest: strings.Repeat("a", 64),
		Now: func() time.Time { return time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC) },
	}
}

func progressiveToolSurface(t *testing.T, runtime *Runtime, server *mcp.Server) *ToolSurface {
	t.Helper()
	surface, err := NewToolSurface(runtime, ToolSurfaceConfig{
		Model: search.ModelIdentity{Provider: "test", Model: "projection", Dimensions: 1, FormatVersion: "v1"},
	})
	if err != nil {
		t.Fatalf("NewToolSurface: %v", err)
	}
	if err := surface.Register(server); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return surface
}

func progressiveCallTool(t *testing.T, client *mcp.ClientSession, revision, name string, arguments json.RawMessage) (*mcp.CallToolResult, error) {
	t.Helper()
	input, err := json.Marshal(struct {
		ToolName       string          `json:"tool_name"`
		CatalogRevision string         `json:"catalog_revision"`
		Arguments      json.RawMessage `json:"arguments"`
	}{
		ToolName:       name,
		CatalogRevision: revision,
		Arguments:      arguments,
	})
	if err != nil {
		t.Fatalf("encode generic call_tool arguments: %v", err)
	}
	return client.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "call_tool",
		Arguments: json.RawMessage(input),
	})
}

func progressiveJSONValuesEqual(actual any, expected json.RawMessage) (bool, error) {
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		return false, fmt.Errorf("marshal actual JSON value: %w", err)
	}

	var actualValue, expectedValue any
	if err := json.Unmarshal(actualJSON, &actualValue); err != nil {
		return false, fmt.Errorf("decode actual JSON value: %w", err)
	}
	if err := json.Unmarshal(expected, &expectedValue); err != nil {
		return false, fmt.Errorf("decode expected JSON value: %w", err)
	}
	return reflect.DeepEqual(actualValue, expectedValue), nil
}

func TestProgressiveMetadataProjectionPublication(t *testing.T) {
	olderUpstream := &progressiveTestUpstream{}
	newerUpstream := &progressiveTestUpstream{}
	runtime := NewRuntime()
	older, err := runtime.Reload(progressiveTestConfig(t, "old"), progressiveTestDeps(olderUpstream))
	if err != nil {
		t.Fatalf("admit older generation: %v", err)
	}
	server := NewMCPServer()
	surface := progressiveToolSurface(t, runtime, server)

	newSchema := json.RawMessage(`{"type":"object","properties":{"generation":{"const":"new"}},"required":["generation"]}`)
	newOutputSchema := json.RawMessage(`{"type":"object","properties":{"accepted":{"const":"new"}},"required":["accepted"]}`)
	newerDefinitions := []core.ToolDefinition{{
		RawName:      "new",
		Description:  "new generation route",
		InputSchema:  newSchema,
		OutputSchema: newOutputSchema,
		Metadata:     core.ToolMetadata{Title: "new generation descriptor"},
	}}
	newerDeps := progressiveTestDeps(newerUpstream)
	newerDeps.Discover = func(config.Server) ([]core.ToolDefinition, error) {
		return newerDefinitions, nil
	}
	newerCandidate := progressiveTestConfig(t, "new")
	newerCandidate.Servers[0].Endpoint = "https://alpha-new.invalid/mcp"

	coordinator := surface.projectionCoordinator()
	entered := make(chan struct{})
	barrierAvailable := make(chan bool, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseOlder := func() { releaseOnce.Do(func() { close(release) }) }
	olderDone := make(chan error, 1)
	joinedOlder := false
	coordinator.testDuringProjectionPreparation = func() {
		available := coordinator.barrier.TryLock()
		if available {
			coordinator.barrier.Unlock()
		}
		barrierAvailable <- available
		close(entered)
		<-release
	}
	defer func() {
		releaseOlder()
		if !joinedOlder {
			select {
			case <-olderDone:
			case <-time.After(5 * time.Second):
				t.Error("timed out joining older publication goroutine after release")
			}
		}
		coordinator.testDuringProjectionPreparation = nil
	}()

	go func() {
		olderDone <- surface.Published(older)
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("older publication did not enter projection preparation")
	}
	select {
	case available := <-barrierAvailable:
		if !available {
			t.Fatal("projection preparation held the admission barrier")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out checking the admission barrier during preparation")
	}

	newer, err := runtime.Reload(newerCandidate, newerDeps)
	if err != nil {
		t.Fatalf("accept newer generation while older publication is prepared: %v", err)
	}
	releaseOlder()
	select {
	case err := <-olderDone:
		joinedOlder = true
		if err != nil {
			t.Fatalf("stale older publication error = %v; want a no-op", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for older publication after release")
	}
	coordinator.testDuringProjectionPreparation = nil
	if current := runtime.Current(); current != newer {
		t.Fatalf("current generation = %p; want newer generation %p", current, newer)
	}
	if err := surface.Published(newer); err != nil {
		t.Fatalf("publish newer generation: %v", err)
	}

	client := connectPromptClient(t, server, nil)
	page, err := client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	listed := make(map[string]*mcp.Tool, len(page.Tools))
	for _, tool := range page.Tools {
		listed[tool.Name] = tool
	}
	newTool := listed["alpha__new"]
	if newTool == nil || newTool.Title != "new generation descriptor" || newTool.Description != "new generation route" {
		t.Fatalf("accepted projection after in-flight older publication = %#v; want the complete newer descriptor", newTool)
	}
	if oldTool := listed["alpha__old"]; oldTool != nil {
		t.Fatalf("in-flight older publication restored descriptor %q", oldTool.Name)
	}
	if newTool != nil {
		if equal, err := progressiveJSONValuesEqual(newTool.InputSchema, newSchema); err != nil {
			t.Errorf("compare listed input schema: %v", err)
		} else if !equal {
			t.Errorf("listed input schema = %#v; want newer schema %s", newTool.InputSchema, canonicalJSON(newSchema))
		}
		if equal, err := progressiveJSONValuesEqual(newTool.OutputSchema, newOutputSchema); err != nil {
			t.Errorf("compare listed output schema: %v", err)
		} else if !equal {
			t.Errorf("listed output schema = %#v; want newer schema %s", newTool.OutputSchema, canonicalJSON(newOutputSchema))
		}
	}

	result, err := progressiveCallTool(t, client, runtime.Current().CatalogRevision(), "alpha__new", json.RawMessage(`{"generation":"new"}`))
	if err != nil {
		t.Fatalf("call accepted route: %v", err)
	}
	if result.IsError {
		t.Fatalf("accepted route returned an MCP tool error: %#v", result)
	}
	if calls := olderUpstream.callsSnapshot(); len(calls) != 0 {
		t.Errorf("older upstream calls after accepted publication = %#v; want none", calls)
	}
	calls := newerUpstream.callsSnapshot()
	if len(calls) != 1 || calls[0] != (recordedProgressiveCall{serverID: "alpha", rawName: "new"}) {
		t.Errorf("upstream route after in-flight older publication = %#v; want newer alpha/new", calls)
	}
	deadOwner, err := surface.newLiveOwner("session-commit-race")
	if err != nil {
		t.Fatalf("create owner for liveness revalidation: %v", err)
	}
	deadEntered := make(chan struct{})
	deadRelease := make(chan struct{})
	deadDone := make(chan error, 1)
	var deadReleaseOnce sync.Once
	releaseDead := func() { deadReleaseOnce.Do(func() { close(deadRelease) }) }
	joinedDead := false
	coordinator.testDuringProjectionPreparation = func() {
		close(deadEntered)
		<-deadRelease
	}
	defer func() {
		releaseDead()
		if !joinedDead {
			select {
			case <-deadDone:
			case <-time.After(5 * time.Second):
				t.Error("timed out joining dead-owner publication after release")
			}
		}
		coordinator.testDuringProjectionPreparation = nil
	}()
	go func() { deadDone <- surface.publishOwnerProjection(deadOwner, newer) }()
	select {
	case <-deadEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("owner publication did not enter staged projection preparation")
	}
	coordinator.barrier.Lock()
	delete(coordinator.liveOwners, deadOwner)
	coordinator.barrier.Unlock()
	releaseDead()
	select {
	case err := <-deadDone:
		joinedDead = true
		if err == nil {
			t.Fatal("publication for a dead owner committed successfully")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for dead-owner publication after release")
	}
	coordinator.testDuringProjectionPreparation = nil
	coordinator.barrier.Lock()
	_, deadOwnerLive := coordinator.liveOwners[deadOwner]
	_, deadOwnerProjection := coordinator.projections[deadOwner]
	_, deadOwnerReference := coordinator.union["alpha__new"].references[deadOwner]
	coordinator.barrier.Unlock()
	if deadOwnerLive || deadOwnerProjection || deadOwnerReference {
		t.Errorf("dead owner was committed: live=%v projection=%v reference=%v", deadOwnerLive, deadOwnerProjection, deadOwnerReference)
	}
	deadPage, err := client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list after dead-owner rejection: %v", err)
	}
	if len(deadPage.Tools) == 0 {
		t.Error("dead-owner rejection removed the publisher's visible tools")
	}
	deadCall, err := progressiveCallTool(t, client, runtime.Current().CatalogRevision(), "alpha__new", json.RawMessage(`{"generation":"new"}`))
	if err != nil || deadCall == nil || deadCall.IsError {
		t.Errorf("publisher route after dead-owner rejection = (%#v, %v); want callable publisher registration", deadCall, err)
	}
	revisedUpstream := &progressiveTestUpstream{}
	revisedDefinitions := []core.ToolDefinition{{
		RawName:      "new",
		Description:  "replacement route",
		InputSchema:  newSchema,
		OutputSchema: newOutputSchema,
		Metadata:     core.ToolMetadata{Title: "replacement descriptor"},
	}}
	revisedDeps := progressiveTestDeps(revisedUpstream)
	revisedDeps.Discover = func(config.Server) ([]core.ToolDefinition, error) {
		return revisedDefinitions, nil
	}
	revisedCandidate := progressiveTestConfig(t, "new")
	revisedCandidate.Servers[0].Endpoint = "https://alpha-revised.invalid/mcp"
	revised, err := runtime.Reload(revisedCandidate, revisedDeps)
	if err != nil {
		t.Fatalf("accept replacement generation: %v", err)
	}

	registryEntered := make(chan struct{})
	registryRelease := make(chan struct{})
	registryDone := make(chan error, 1)
	var releaseRegistryOnce sync.Once
	releaseRegistry := func() { releaseRegistryOnce.Do(func() { close(registryRelease) }) }
	joinedRegistry := false
	coordinator.testBeforeRegistryAdd = func() {
		close(registryEntered)
		<-registryRelease
	}
	defer func() {
		releaseRegistry()
		if !joinedRegistry {
			select {
			case <-registryDone:
			case <-time.After(5 * time.Second):
				t.Error("timed out joining replacement publication after release")
			}
		}
		coordinator.testBeforeRegistryAdd = nil
	}()
	go func() { registryDone <- surface.Published(revised) }()
	select {
	case <-registryEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("replacement publication did not reach the registry handoff")
	}

	during, err := client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list during same-name replacement: %v", err)
	}
	var duringTool *mcp.Tool
	for _, tool := range during.Tools {
		if tool.Name == "alpha__new" {
			duringTool = tool
		}
	}
	if duringTool == nil || duringTool.Title != "new generation descriptor" {
		t.Errorf("tools/list during replacement = %#v; want complete old registration", duringTool)
	}
	duringCall, err := client.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "alpha__new",
		Arguments: json.RawMessage(`{"generation":"new"}`),
	})
	if err != nil || duringCall == nil || !duringCall.IsError {
		t.Errorf("tools/call during same-name replacement = (%#v, %v); want stale rejection before new descriptor publication", duringCall, err)
	}
	if calls := revisedUpstream.callsSnapshot(); len(calls) != 0 {
		t.Errorf("new route was called while tools/list still exposed the old descriptor: %#v", calls)
	}

	releaseRegistry()
	select {
	case err := <-registryDone:
		joinedRegistry = true
		if err != nil {
			t.Fatalf("publish replacement generation: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for replacement publication after release")
	}
	after, err := client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list after same-name replacement: %v", err)
	}
	var newToolListed bool
	for _, tool := range after.Tools {
		if tool.Name == "alpha__new" && tool.Title == "replacement descriptor" {
			newToolListed = true
		}
	}
	if !newToolListed {
		t.Error("tools/list after replacement lacks the complete new registration")
	}
	afterCall, err := progressiveCallTool(t, client, runtime.Current().CatalogRevision(), "alpha__new", json.RawMessage(`{"generation":"new"}`))
	if err != nil || afterCall.IsError {
		t.Errorf("tools/call after same-name replacement = (%#v, %v); want new registration callable", afterCall, err)
	}
	if calls := revisedUpstream.callsSnapshot(); len(calls) != 1 ||
		calls[0] != (recordedProgressiveCall{serverID: "alpha", rawName: "new"}) {
		t.Errorf("replacement upstream calls = %#v; want only the call after new registration publication", calls)
	}
}

func TestProgressiveMetadataUnionReferences(t *testing.T) {
	olderUpstream := &progressiveTestUpstream{}
	newerUpstream := &progressiveTestUpstream{}
	runtime := NewRuntime()

	olderSharedSchema := json.RawMessage(`{"type":"object","properties":{"generation":{"const":"old"}},"required":["generation"]}`)
	olderDefinitions := []core.ToolDefinition{
		{RawName: "old", Description: "old pin route", InputSchema: json.RawMessage(`{"type":"object"}`), Metadata: core.ToolMetadata{Title: "old pin descriptor"}},
		{RawName: "new", Description: "new pin route", InputSchema: json.RawMessage(`{"type":"object"}`), Metadata: core.ToolMetadata{Title: "new pin descriptor"}},
		{RawName: "shared", Description: "shared old route", InputSchema: olderSharedSchema, Metadata: core.ToolMetadata{Title: "shared old descriptor"}},
	}
	olderDeps := progressiveTestDeps(olderUpstream)
	olderDeps.Discover = func(config.Server) ([]core.ToolDefinition, error) {
		return olderDefinitions, nil
	}
	olderCandidate := progressiveTestConfig(t, "shared")
	olderCandidate.Pins = []map[string]string{
		{"server": "alpha", "tool": "old"},
		{"server": "alpha", "tool": "shared"},
	}
	older, err := runtime.Reload(olderCandidate, olderDeps)
	if err != nil {
		t.Fatalf("admit initial pins: %v", err)
	}

	server := NewMCPServer()
	surface := progressiveToolSurface(t, runtime, server)
	if err := surface.Published(older); err != nil {
		t.Fatalf("publish initial generation: %v", err)
	}
	firstOwner, err := surface.newLiveOwner("session-a")
	if err != nil {
		t.Fatalf("create first live owner: %v", err)
	}
	remainingOwner, err := surface.newLiveOwner("session-b")
	if err != nil {
		t.Fatalf("create remaining live owner: %v", err)
	}
	if err := surface.publishOwnerProjection(firstOwner, older); err != nil {
		t.Fatalf("publish first owner projection: %v", err)
	}
	if err := surface.publishOwnerProjection(remainingOwner, older); err != nil {
		t.Fatalf("publish remaining owner projection: %v", err)
	}

	initialTargets := make(map[logicalToolTarget]struct{})
	for _, binding := range older.catalog.Bindings() {
		initialTargets[logicalToolTarget{serverID: binding.ServerID, rawName: binding.RawName}] = struct{}{}
	}
	retainedCandidate := logicalToolTarget{serverID: "alpha", rawName: "new"}
	departedCandidate := logicalToolTarget{serverID: "alpha", rawName: "old"}
	for _, target := range []logicalToolTarget{retainedCandidate, departedCandidate} {
		if _, ok := initialTargets[target]; !ok {
			t.Fatalf("initial catalog does not contain discovery-interest fixture %v", target)
		}
	}
	ownerInterests := map[logicalToolTarget]struct{}{
		retainedCandidate: {},
		departedCandidate: {},
	}
	coordinator := surface.projectionCoordinator()
	for _, owner := range []projectionOwner{firstOwner, remainingOwner} {
		previous := coordinator.ownerProjection(owner)
		specs := make([]projectionToolSpec, 0, len(previous.tools))
		for _, descriptor := range previous.tools {
			projected := previous.byName[descriptor.Name]
			specs = append(specs, projectionToolSpec{name: descriptor.Name, binding: projected.binding})
		}
		projection, err := prepareProjection(owner, previous, older.Generation(), specs, ownerInterests)
		if err != nil {
			t.Fatalf("prepare owner discovery interests: %v", err)
		}
		coordinator.barrier.Lock()
		_, err = coordinator.replaceProjectionLocked(owner, projection)
		coordinator.barrier.Unlock()
		if err != nil {
			t.Fatalf("seed owner discovery interests: %v", err)
		}
	}

	newSharedSchema := json.RawMessage(`{"type":"object","properties":{"generation":{"const":"new"}},"required":["generation"]}`)
	newSharedOutputSchema := json.RawMessage(`{"type":"object","properties":{"accepted":{"const":"new"}},"required":["accepted"]}`)
	newerDefinitions := []core.ToolDefinition{
		{RawName: "new", Description: "new pin route", InputSchema: json.RawMessage(`{"type":"object"}`), Metadata: core.ToolMetadata{Title: "new pin descriptor"}},
		{
			RawName:      "shared",
			Description:  "shared new route",
			InputSchema:  newSharedSchema,
			OutputSchema: newSharedOutputSchema,
			Metadata:     core.ToolMetadata{Title: "shared new descriptor"},
		},
	}
	newerDeps := progressiveTestDeps(newerUpstream)
	newerDeps.Discover = func(config.Server) ([]core.ToolDefinition, error) {
		return newerDefinitions, nil
	}
	newerCandidate := progressiveTestConfig(t, "shared")
	newerCandidate.Servers[0].Endpoint = "https://alpha-new.invalid/mcp"
	newerCandidate.Pins = []map[string]string{
		{"server": "alpha", "tool": "shared"},
		{"server": "alpha", "tool": "new"},
	}
	newer, err := runtime.Reload(newerCandidate, newerDeps)
	if err != nil {
		t.Fatalf("accept newer pin generation: %v", err)
	}
	newerTargets := make(map[logicalToolTarget]struct{})
	for _, binding := range newer.catalog.Bindings() {
		newerTargets[logicalToolTarget{serverID: binding.ServerID, rawName: binding.RawName}] = struct{}{}
	}
	var retainedInterest, departedInterest logicalToolTarget
	for target := range ownerInterests {
		if _, remains := newerTargets[target]; remains {
			retainedInterest = target
		} else {
			departedInterest = target
		}
	}
	if retainedInterest == (logicalToolTarget{}) || departedInterest == (logicalToolTarget{}) {
		t.Fatalf("newer catalog did not distinguish retained and departed discovery interests: %#v", newerTargets)
	}

	if err := surface.Published(newer); err != nil {
		t.Errorf("reconcile accepted newer generation: %v", err)
	}

	checkSchema := func(label string, actual any, expected json.RawMessage) {
		t.Helper()
		equal, err := progressiveJSONValuesEqual(actual, expected)
		if err != nil {
			t.Errorf("compare %s: %v", label, err)
		} else if !equal {
			t.Errorf("%s = %#v; want JSON value %s", label, actual, canonicalJSON(expected))
		}
	}
	assertReconciled := func(stage string) {
		t.Helper()
		coordinator.barrier.Lock()
		defer coordinator.barrier.Unlock()

		shared, ok := coordinator.union["alpha__shared"]
		if !ok {
			t.Errorf("%s: shared union entry is absent", stage)
		} else {
			if shared.descriptor.Title != "shared new descriptor" || shared.descriptor.Description != "shared new route" {
				t.Errorf("%s: shared union descriptor = %#v; want complete newer descriptor", stage, shared.descriptor)
			}
			checkSchema(stage+" shared union input schema", shared.descriptor.InputSchema, newSharedSchema)
			checkSchema(stage+" shared union output schema", shared.descriptor.OutputSchema, newSharedOutputSchema)
			if shared.target != (logicalToolTarget{serverID: "alpha", rawName: "shared"}) {
				t.Errorf("%s: shared union target = %#v; want alpha/shared", stage, shared.target)
			}
			for _, owner := range []projectionOwner{firstOwner, remainingOwner} {
				if _, ok := shared.references[owner]; !ok {
					t.Errorf("%s: shared union entry lost live owner %#v", stage, owner)
				}
			}
		}
		if _, ok := coordinator.union["alpha__old"]; ok {
			t.Errorf("%s: departed pin alpha__old remains in the union", stage)
		}
		if _, ok := coordinator.union["alpha__new"]; !ok {
			t.Errorf("%s: newly added pin alpha__new is absent from the union", stage)
		}
		retainedReferences := coordinator.interestReferences[retainedInterest]
		for _, owner := range []projectionOwner{firstOwner, remainingOwner} {
			projection := coordinator.projections[owner]
			if projection == nil {
				t.Errorf("%s: owner %#v has no projection", stage, owner)
				continue
			}
			if projection.runtimeGeneration != newer.Generation() {
				t.Errorf("%s: owner %#v projection generation = %d; want %d", stage, owner, projection.runtimeGeneration, newer.Generation())
			}
			projected, ok := projection.byName["alpha__shared"]
			if !ok {
				t.Errorf("%s: owner %#v lacks shared pin projection", stage, owner)
			} else {
				if projected.descriptor.Title != "shared new descriptor" || projected.descriptor.Description != "shared new route" {
					t.Errorf("%s: owner %#v shared descriptor = %#v; want complete newer descriptor", stage, owner, projected.descriptor)
				}
				checkSchema(stage+" owner shared input schema", projected.inputSchema, newSharedSchema)
				checkSchema(stage+" owner shared output schema", projected.descriptor.OutputSchema, newSharedOutputSchema)
			}
			if _, ok := projection.interests[retainedInterest]; !ok {
				t.Errorf("%s: owner %#v lost retained interest %#v", stage, owner, retainedInterest)
			}
			if _, ok := projection.interests[departedInterest]; ok {
				t.Errorf("%s: owner %#v retained departed interest %#v", stage, owner, departedInterest)
			}
			if _, ok := retainedReferences[owner]; !ok {
				t.Errorf("%s: retained interest %#v lost owner reference %#v", stage, retainedInterest, owner)
			}
		}
		if _, ok := coordinator.interestReferences[departedInterest]; ok {
			t.Errorf("%s: departed interest %#v still has coordinator references", stage, departedInterest)
		}
	}
	assertReconciled("first reconciliation")
	if err := surface.Published(newer); err != nil {
		t.Errorf("repeat accepted-generation reconciliation: %v", err)
	}
	assertReconciled("repeated reconciliation")

	client := connectPromptClient(t, server, nil)
	page, err := client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list after newer generation: %v", err)
	}
	listed := make(map[string]*mcp.Tool, len(page.Tools))
	sharedCount := 0
	for _, tool := range page.Tools {
		listed[tool.Name] = tool
		if tool.Name == "alpha__shared" {
			sharedCount++
		}
	}
	if sharedCount != 1 {
		t.Errorf("tools/list has %d current alpha__shared descriptors; want one", sharedCount)
	}
	sharedTool := listed["alpha__shared"]
	if sharedTool == nil {
		t.Error("tools/list lacks shared pinned name")
	} else {
		if sharedTool.Title != "shared new descriptor" || sharedTool.Description != "shared new route" {
			t.Errorf("tools/list shared descriptor = %#v; want complete newer descriptor", sharedTool)
		}
		checkSchema("tools/list shared input schema", sharedTool.InputSchema, newSharedSchema)
		checkSchema("tools/list shared output schema", sharedTool.OutputSchema, newSharedOutputSchema)
	}
	if listed["alpha__old"] != nil {
		t.Errorf("tools/list retained departed pin alpha__old: %#v", listed["alpha__old"])
	}
	if listed["alpha__new"] == nil {
		t.Error("tools/list lacks newly pinned alpha__new")
	}
	if departed, err := progressiveCallTool(t, client, runtime.Current().CatalogRevision(), "alpha__old", json.RawMessage(`{}`)); err != nil || departed == nil || !departed.IsError {
		t.Errorf("generic call to departed catalog target alpha__old = (%#v, %v); want an MCP tool error", departed, err)
	}

	dynamicName := core.PublicName("alpha", "new")
	callNamed := func(stage, name string) {
		t.Helper()
		result, err := progressiveCallTool(t, client, runtime.Current().CatalogRevision(), name, json.RawMessage(`{}`))
		if err != nil {
			t.Errorf("%s call retained %s registration: %v", stage, name, err)
			return
		}
		if result.IsError {
			t.Errorf("%s retained %s registration returned an MCP tool error: %#v", stage, name, result)
		}
	}



	callShared := func(stage string) {
		t.Helper()
		result, err := progressiveCallTool(t, client, runtime.Current().CatalogRevision(), "alpha__shared", json.RawMessage(`{"generation":"new"}`))
		if err != nil {
			t.Errorf("%s call shared registration: %v", stage, err)
			return
		}
		if result.IsError {
			t.Errorf("%s shared registration returned an MCP tool error: %#v", stage, result)
		}
	}
	callShared("before owner retirement")
	if calls := newerUpstream.callsSnapshot(); len(calls) != 1 || calls[0] != (recordedProgressiveCall{serverID: "alpha", rawName: "shared"}) {
		t.Errorf("new upstream route calls before owner retirement = %#v; want alpha/shared", calls)
	}
	if calls := olderUpstream.callsSnapshot(); len(calls) != 0 {
		t.Errorf("older upstream route calls after newer publication = %#v; want none", calls)
	}
	// The accepted generation pins alpha__new, but that target remains an
	// owner interest. A later rebase unpins it without losing discoverability
	// or its callable route.
	dynamicCandidate := progressiveTestConfig(t, "shared")
	dynamicCandidate.Servers[0].Endpoint = "https://alpha-new.invalid/mcp"
	dynamicCandidate.Pins = []map[string]string{{"server": "alpha", "tool": "shared"}}
	dynamic, err := runtime.Reload(dynamicCandidate, newerDeps)
	if err != nil {
		t.Fatalf("accept generation that makes alpha__new dynamic: %v", err)
	}
	if err := surface.Published(dynamic); err != nil {
		t.Fatalf("publish generation that makes alpha__new dynamic: %v", err)
	}
	dynamicPage, err := client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list after dynamic rebase: %v", err)
	}
	dynamicListed := make(map[string]*mcp.Tool, len(dynamicPage.Tools))
	for _, tool := range dynamicPage.Tools {
		dynamicListed[tool.Name] = tool
	}
	if tool := dynamicListed[dynamicName]; tool == nil || tool.Title != "new pin descriptor" {
		t.Errorf("tools/list after pin-to-dynamic rebase %s = %#v; want retained descriptor", dynamicName, tool)
	}
	if dynamicListed["alpha__new"] != nil {
		t.Errorf("tools/list after pin-to-dynamic rebase retained canonical alias alpha__new: %#v", dynamicListed["alpha__new"])
	}
	if dynamicListed["alpha__old"] != nil {
		t.Errorf("tools/list after dynamic rebase retained departed alpha__old: %#v", dynamicListed["alpha__old"])
	}
	callNamed("after pin-to-dynamic rebase", dynamicName)
	if calls := newerUpstream.callsSnapshot(); len(calls) != 2 ||
		calls[0] != (recordedProgressiveCall{serverID: "alpha", rawName: "shared"}) ||
		calls[1] != (recordedProgressiveCall{serverID: "alpha", rawName: "new"}) {
		t.Errorf("new upstream calls after dynamic rebase = %#v; want shared then new", calls)
	}
	repinnedCandidate := progressiveTestConfig(t, "shared")
	repinnedCandidate.Servers[0].Endpoint = "https://alpha-new.invalid/mcp"
	repinnedCandidate.Pins = []map[string]string{
		{"server": "alpha", "tool": "shared"},
		{"server": "alpha", "tool": "new"},
	}
	repinned, err := runtime.Reload(repinnedCandidate, newerDeps)
	if err != nil {
		t.Fatalf("accept generation that makes alpha__new pinned again: %v", err)
	}
	if err := surface.Published(repinned); err != nil {
		t.Fatalf("publish generation that makes alpha__new pinned again: %v", err)
	}
	repinnedPage, err := client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list after dynamic-to-pin rebase: %v", err)
	}
	repinnedListed := make(map[string]*mcp.Tool, len(repinnedPage.Tools))
	for _, tool := range repinnedPage.Tools {
		repinnedListed[tool.Name] = tool
	}
	if tool := repinnedListed["alpha__new"]; tool == nil || tool.Title != "new pin descriptor" {
		t.Errorf("tools/list after dynamic-to-pin rebase alpha__new = %#v; want the pinned descriptor", tool)
	}
	if repinnedListed[dynamicName] != nil {
		t.Errorf("tools/list after dynamic-to-pin rebase retained opaque alias %s: %#v", dynamicName, repinnedListed[dynamicName])
	}
	if repinnedListed["alpha__old"] != nil {
		t.Errorf("tools/list after dynamic-to-pin rebase retained departed alpha__old: %#v", repinnedListed["alpha__old"])
	}
	callNamed("after dynamic-to-pin rebase", "alpha__new")
	if calls := newerUpstream.callsSnapshot(); len(calls) != 3 ||
		calls[0] != (recordedProgressiveCall{serverID: "alpha", rawName: "shared"}) ||
		calls[1] != (recordedProgressiveCall{serverID: "alpha", rawName: "new"}) ||
		calls[2] != (recordedProgressiveCall{serverID: "alpha", rawName: "new"}) {
		t.Errorf("new upstream calls after dynamic-to-pin rebase = %#v; want shared, new, new", calls)
	}

	if !surface.retireLiveOwner(firstOwner) {
		t.Error("first live owner was not retired")
	}
	coordinator.barrier.Lock()
	sharedAfterRetirement := coordinator.union["alpha__shared"]
	if sharedAfterRetirement == nil {
		t.Error("shared union entry disappeared after one owner retired")
	} else {
		if _, ok := sharedAfterRetirement.references[firstOwner]; ok {
			t.Error("retired owner still references shared union entry")
		}
		if _, ok := sharedAfterRetirement.references[remainingOwner]; !ok {
			t.Error("remaining owner lost shared union reference")
		}
	}
	coordinator.barrier.Unlock()
	callShared("after owner retirement")
	calls := newerUpstream.callsSnapshot()
	if len(calls) != 4 ||
		calls[0] != (recordedProgressiveCall{serverID: "alpha", rawName: "shared"}) ||
		calls[1] != (recordedProgressiveCall{serverID: "alpha", rawName: "new"}) ||
		calls[2] != (recordedProgressiveCall{serverID: "alpha", rawName: "new"}) ||
		calls[3] != (recordedProgressiveCall{serverID: "alpha", rawName: "shared"}) {
		t.Errorf("new upstream route calls after owner retirement = %#v; want shared, new, new, shared", calls)
	}
}

func TestProgressiveMetadataSchemaFailureKeepsProjection(t *testing.T) {
	server := NewMCPServer()
	surface := progressiveToolSurface(t, NewRuntime(), server)
	good := core.ToolDefinition{
		RawName:     "good",
		Description: "good route",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}
	surface.Published(&Assembly{pins: []core.ResolvedPin{{
		PinnedTool: core.PinnedTool{ServerID: "alpha", RawName: "good"},
		Binding:    &core.Binding{ServerID: "alpha", RawName: "good", Def: good},
	}}})

	broken := core.ToolDefinition{
		RawName:     "broken",
		Description: "invalid schema",
		InputSchema: json.RawMessage(`{"properties":{}}`),
	}
	surface.Published(&Assembly{pins: []core.ResolvedPin{{
		PinnedTool: core.PinnedTool{ServerID: "alpha", RawName: "broken"},
		Binding:    &core.Binding{ServerID: "alpha", RawName: "broken", Def: broken},
	}}})

	client := connectPromptClient(t, server, nil)
	page, err := client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list after invalid schema publication: %v", err)
	}
	listed := make(map[string]bool, len(page.Tools))
	for _, tool := range page.Tools {
		listed[tool.Name] = true
	}
	if !listed["alpha__good"] || listed["alpha__broken"] {
		t.Fatalf("tools/list after invalid schema publication = %v; want only the previous valid pin", listed)
	}
}

func TestProgressiveMetadataDepartureInterestRemoval(t *testing.T) {
	coordinator := newProjectionCoordinator()
	first, err := coordinator.newLiveOwner("session-a")
	if err != nil {
		t.Fatalf("create first owner: %v", err)
	}
	remaining, err := coordinator.newLiveOwner("session-b")
	if err != nil {
		t.Fatalf("create remaining owner: %v", err)
	}
	target := logicalToolTarget{serverID: "alpha", rawName: "shared"}
	interests := map[logicalToolTarget]struct{}{target: {}}

	for _, owner := range []projectionOwner{first, remaining} {
		projection, err := prepareProjection(owner, nil, 1, nil, interests)
		if err != nil {
			t.Fatalf("prepare owner interest: %v", err)
		}
		coordinator.barrier.Lock()
		_, err = coordinator.replaceProjectionLocked(owner, projection)
		coordinator.barrier.Unlock()
		if err != nil {
			t.Fatalf("publish owner interest: %v", err)
		}
	}

	departed, err := prepareProjection(first, coordinator.ownerProjection(first), 1, nil, nil)
	if err != nil {
		t.Fatalf("prepare departure projection: %v", err)
	}
	coordinator.barrier.Lock()
	_, err = coordinator.replaceProjectionLocked(first, departed)
	coordinator.barrier.Unlock()
	if err != nil {
		t.Fatalf("publish departure projection: %v", err)
	}

	references := coordinator.interestReferences[target]
	if len(references) != 1 {
		t.Fatalf("interest references after one departure = %d; want one live owner", len(references))
	}
	if _, ok := references[remaining]; !ok {
		t.Fatal("departure removed the remaining owner's interest")
	}
}

func TestProgressiveMetadataUnpinPreservesEffectiveMetadata(t *testing.T) {
	source := core.ToolDefinition{
		RawName:     "read",
		Description: "Source description",
		InputSchema: json.RawMessage(`{"type":"object"}`),
		Metadata: core.ToolMetadata{
			Title:       "Source title",
			Annotations: json.RawMessage(`{"readOnlyHint":true,"destructiveHint":false,"idempotentHint":true}`),
			Meta:        json.RawMessage(`{"owner":"source"}`),
		},
	}
	metadata := &config.ToolMetadataConfig{
		Defaults: config.MetadataPatch{
			"title":       json.RawMessage(`"Default title"`),
			"description": json.RawMessage(`"Default description"`),
			"annotations": json.RawMessage(`{"readOnlyHint":false,"destructiveHint":false,"idempotentHint":true}`),
			"_meta":        json.RawMessage(`{"owner":"default"}`),
		},
		Tools: map[string]config.MetadataPatch{
			"read": {
				"title":       json.RawMessage(`"Exact title"`),
				"annotations": json.RawMessage(`{"destructiveHint":true,"idempotentHint":false}`),
				"_meta":        json.RawMessage(`{"owner":"exact"}`),
			},
		},
	}
	candidate := func(pinned bool) *config.Config {
		result := progressiveTestConfig(t, "read")
		result.Servers[0].ToolMetadata = metadata
		if !pinned {
			result.Pins = nil
		}
		return result
	}

	upstream := &progressiveTestUpstream{}
	deps := progressiveTestDeps(upstream)
	deps.Discover = func(config.Server) ([]core.ToolDefinition, error) {
		return []core.ToolDefinition{source}, nil
	}
	runtime := NewRuntime()
	initial, err := runtime.Reload(candidate(true), deps)
	if err != nil {
		t.Fatalf("accept initially pinned generation: %v", err)
	}
	server := NewMCPServer()
	surface := progressiveToolSurface(t, runtime, server)
	if err := surface.Published(initial); err != nil {
		t.Fatalf("publish initially pinned generation: %v", err)
	}
	client := connectPromptClient(t, server, nil)

	assertEffectiveMetadata := func(stage, exposedName string) {
		t.Helper()
		page, err := client.ListTools(context.Background(), nil)
		if err != nil {
			t.Fatalf("tools/list %s: %v", stage, err)
		}
		names := make([]string, len(page.Tools))
		for index, tool := range page.Tools {
			names[index] = tool.Name
		}
		var listed *mcp.Tool
		for _, tool := range page.Tools {
			if tool.Name == exposedName {
				listed = tool
				break
			}
		}
		if listed == nil || listed.Title != "Exact title" || listed.Description != "Default description" {
			t.Fatalf("tools/list %s descriptor = %#v among %v; want exact title and default description", stage, listed, names)
		}
		annotationJSON, err := json.Marshal(listed.Annotations)
		if err != nil {
			t.Fatalf("marshal annotations %s: %v", stage, err)
		}
		hints := annotationsFor(t, core.ToolMetadata{Annotations: annotationJSON})
		for key, want := range map[string]bool{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
		} {
			if got := hintValue(t, hints, key); got != want {
				t.Errorf("tools/list %s %s = %v, want %v", stage, key, got, want)
			}
		}
		if equal, err := progressiveJSONValuesEqual(listed.Meta, json.RawMessage(`{"owner":"exact"}`)); err != nil {
			t.Errorf("compare _meta %s: %v", stage, err)
		} else if !equal {
			t.Errorf("tools/list %s _meta = %#v; want exact override", stage, listed.Meta)
		}
	}
	assertEffectiveMetadata("initial pin", config.WireToolName("alpha", "read"))

	owner, err := surface.newLiveOwner("metadata-interest")
	if err != nil {
		t.Fatalf("create interested owner: %v", err)
	}
	target := logicalToolTarget{serverID: "alpha", rawName: "read"}
	interest, err := prepareProjection(owner, nil, initial.Generation(), nil, map[logicalToolTarget]struct{}{target: {}})
	if err != nil {
		t.Fatalf("prepare interested owner projection: %v", err)
	}
	coordinator := surface.projectionCoordinator()
	coordinator.barrier.Lock()
	_, err = coordinator.replaceProjectionLocked(owner, interest)
	coordinator.barrier.Unlock()
	if err != nil {
		t.Fatalf("publish interested owner projection: %v", err)
	}

	unpinned, err := runtime.Reload(candidate(false), deps)
	if err != nil {
		t.Fatalf("accept unpinned generation: %v", err)
	}
	if err := surface.Published(unpinned); err != nil {
		t.Fatalf("publish unpinned generation: %v", err)
	}
	assertEffectiveMetadata("accepted unpin", core.PublicName("alpha", "read"))
	if bindings := unpinned.Catalog().Bindings(); len(bindings) != 1 ||
		bindings[0].Def.Metadata.Title != "Source title" {
		t.Fatalf("unpin mutated source catalog metadata: %#v", bindings)
	}

	repinned, err := runtime.Reload(candidate(true), deps)
	if err != nil {
		t.Fatalf("accept repinned generation: %v", err)
	}
	if err := surface.Published(repinned); err != nil {
		t.Fatalf("publish repinned generation: %v", err)
	}
	assertEffectiveMetadata("accepted repin", config.WireToolName("alpha", "read"))

	unpinnedAgain, err := runtime.Reload(candidate(false), deps)
	if err != nil {
		t.Fatalf("accept second unpin: %v", err)
	}
	if err := surface.Published(unpinnedAgain); err != nil {
		t.Fatalf("publish second unpin: %v", err)
	}
	assertEffectiveMetadata("second accepted unpin", core.PublicName("alpha", "read"))
}

func TestProgressiveMetadataRegisteredCallRejectsUnownedSession(t *testing.T) {
	makeAssembly := func(generation uint64, serverID, rawName string, upstream core.Upstream) (*Assembly, *core.Binding) {
		definition := core.ToolDefinition{
			RawName:     rawName,
			Description: "same registered descriptor",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}
		catalog := core.NewBridge()
		endpoint := "https://" + serverID + ".invalid/mcp"
		if err := catalog.Register(serverID, endpoint, []core.ToolDefinition{definition}); err != nil {
			t.Fatalf("register %s/%s catalog binding: %v", serverID, rawName, err)
		}
		assembly := &Assembly{
			generation: generation,
			revision:   catalog.Revision(),
			catalog:    catalog,
			servers: map[string]ServerRuntime{
				serverID: {
					ID:        serverID,
					Endpoint:  endpoint,
					Client:    upstream,
					available: true,
					tracker:   &leaseTracker{},
				},
			},
		}
		return assembly, &core.Binding{ServerID: serverID, RawName: rawName, Def: definition}
	}

	oldUpstream := &progressiveTestUpstream{}
	newUpstream := &progressiveTestUpstream{}
	oldAssembly, oldBinding := makeAssembly(1, "alpha", "old", oldUpstream)
	newAssembly, newBinding := makeAssembly(2, "beta", "new", newUpstream)
	runtime := NewRuntime()
	runtime.current.Store(oldAssembly)
	server := NewMCPServer()
	surface := progressiveToolSurface(t, runtime, server)
	coordinator := surface.projectionCoordinator()
	owner := coordinator.newPublisher()
	registeredName := core.PublicName("alpha", "old")
	oldProjection, err := prepareProjection(owner, nil, oldAssembly.Generation(), []projectionToolSpec{{
		name:    registeredName,
		binding: oldBinding,
	}}, nil)
	if err != nil {
		t.Fatalf("prepare initial projection: %v", err)
	}
	coordinator.barrier.Lock()
	coordinator.acceptedKnown = true
	coordinator.acceptedGeneration = oldAssembly.Generation()
	initialDelta, err := coordinator.replaceProjectionLocked(owner, oldProjection)
	coordinator.barrier.Unlock()
	if err != nil {
		t.Fatalf("publish initial projection: %v", err)
	}
	surface.applyRegistryDelta(initialDelta)
	client := connectPromptClient(t, server, nil)

	newProjection, err := prepareProjection(owner, oldProjection, newAssembly.Generation(), []projectionToolSpec{{
		name:    registeredName,
		binding: newBinding,
	}}, nil)
	if err != nil {
		t.Fatalf("prepare newer projection: %v", err)
	}
	coordinator.barrier.Lock()
	runtime.current.Store(newAssembly)
	coordinator.acceptedGeneration = newAssembly.Generation()
	newDelta, err := coordinator.replaceProjectionLocked(owner, newProjection)
	coordinator.barrier.Unlock()
	if err != nil {
		t.Fatalf("publish newer projection: %v", err)
	}
	if len(newDelta.add) != 1 {
		t.Fatalf("newer same-name projection registry additions = %d, want one pending re-registration", len(newDelta.add))
	}

	registryReady := make(chan struct{})
	registryRelease := make(chan struct{})
	registryDone := make(chan struct{})
	coordinator.testBeforeRegistryAdd = func() {
		close(registryReady)
		<-registryRelease
	}
	go func() {
		surface.applyRegistryDelta(newDelta)
		close(registryDone)
	}()
	select {
	case <-registryReady:
	case <-time.After(5 * time.Second):
		close(registryRelease)
		t.Fatal("same-name handler did not remain registered during reconciliation")
	}

	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      registeredName,
		Arguments: json.RawMessage(`{}`),
	})
	close(registryRelease)
	<-registryDone
	coordinator.testBeforeRegistryAdd = nil
	if err != nil {
		t.Fatalf("call unowned registered public tool name: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("unowned call through registered public tool = %#v; want a rejected MCP tool result", result)
	}
	if calls := oldUpstream.callsSnapshot(); len(calls) != 0 {
		t.Fatalf("historical route calls = %#v; want none", calls)
	}
	if calls := newUpstream.callsSnapshot(); len(calls) != 0 {
		t.Fatalf("unowned current route calls = %#v; want none", calls)
	}
}
