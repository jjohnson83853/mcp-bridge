package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/core"
)

type directExecutionCall struct {
	serverID string
	rawName  string
	args     json.RawMessage
}

type directExecutionUpstream struct {
	calls         chan directExecutionCall
	started       chan struct{}
	startedOnce   sync.Once
	release       chan struct{}
	waitForCancel bool
	panicOnCall   bool
	result        json.RawMessage
	outcome       *core.UpstreamOutcome
}

func (u *directExecutionUpstream) CallTool(ctx context.Context, _, serverID, rawName string, args json.RawMessage) (json.RawMessage, *core.UpstreamOutcome) {
	u.calls <- directExecutionCall{serverID: serverID, rawName: rawName, args: append(json.RawMessage(nil), args...)}
	if u.started != nil {
		u.startedOnce.Do(func() { close(u.started) })
	}
	if u.panicOnCall {
		panic("direct upstream panic")
	}
	if u.waitForCancel {
		<-ctx.Done()
		return nil, &core.UpstreamOutcome{Failure: &core.UpstreamFailure{Code: "cancelled", ExecutionState: core.ExecutionUnknown}}
	}
	if u.release != nil {
		select {
		case <-u.release:
		case <-ctx.Done():
			return nil, &core.UpstreamOutcome{Failure: &core.UpstreamFailure{Code: "cancelled", ExecutionState: core.ExecutionUnknown}}
		}
	}
	return append(json.RawMessage(nil), u.result...), u.outcome
}

func newDirectExecutionFixture(t *testing.T, upstream *directExecutionUpstream) (*progressiveSessionHarness, string, projectionOwner, *toolProjection, projectedTool, *Assembly) {
	t.Helper()
	harness := newProgressiveSessionHarness(t)
	sessionID := harness.initialize(t)
	listed := harness.send(t, http.MethodPost, sessionID, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`, nil)
	if listed.Response == nil || listed.Response.StatusCode != http.StatusOK {
		t.Fatalf("tools/list failed: %#v %s", listed.Response, listed.body)
	}
	owner := harness.owner(t, sessionID)
	projection := harness.projection(owner)
	if projection == nil || len(projection.tools) == 0 {
		t.Fatal("session projection has no direct tools")
	}
	name := projection.tools[0].Name
	projected, ok := projection.byName[name]
	if !ok || projected.binding == nil {
		t.Fatalf("projected tool %q has no binding", name)
	}
	original := harness.surface.runtime.Current()
	assembly := *original
	assembly.servers = make(map[string]ServerRuntime, len(original.servers))
	for id, server := range original.servers {
		assembly.servers[id] = server
	}
	server, ok := assembly.servers[projected.binding.ServerID]
	if !ok {
		t.Fatalf("assembly has no server %q", projected.binding.ServerID)
	}
	server.Client = upstream
	assembly.servers[projected.binding.ServerID] = server
	coordinator := harness.surface.projectionCoordinator()
	coordinator.barrier.Lock()
	harness.surface.runtime.current.Store(&assembly)
	coordinator.barrier.Unlock()
	harness.assembly = &assembly
	return harness, sessionID, owner, projection, projected, &assembly
}

func directCallBody(name string, args string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, name, args)
}

func trackerLeaseCount(tracker *leaseTracker) int {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	return tracker.leases
}
func cloneDirectTestProjection(coordinator *projectionCoordinator, owner projectionOwner) *toolProjection {
	coordinator.barrier.Lock()
	current := coordinator.projections[owner]
	next := *current
	next.byName = make(map[string]projectedTool, len(current.byName))
	for name, tool := range current.byName {
		next.byName[name] = tool
	}
	next.tools = append([]*mcp.Tool(nil), current.tools...)
	coordinator.barrier.Unlock()
	return &next
}

func replaceDirectTestProjection(coordinator *projectionCoordinator, owner projectionOwner, next *toolProjection) {
	coordinator.barrier.Lock()
	coordinator.projections[owner] = next
	coordinator.barrier.Unlock()
}

func removeDirectTestProjectionTool(coordinator *projectionCoordinator, owner projectionOwner, name string) {
	next := cloneDirectTestProjection(coordinator, owner)
	delete(next.byName, name)
	tools := make([]*mcp.Tool, 0, len(next.tools))
	for _, tool := range next.tools {
		if tool.Name != name {
			tools = append(tools, tool)
		}
	}
	next.tools = tools
	replaceDirectTestProjection(coordinator, owner, next)
}

func directAdapterForTest(t *testing.T, surface *ToolSurface, assembly *Assembly, projected projectedTool, schema json.RawMessage) boundDirectExecution {
	t.Helper()
	lease, ok := assembly.Acquire(projected.binding.ServerID)
	if !ok {
		t.Fatal("could not acquire direct adapter test lease")
	}
	binding := projected.binding
	if schema != nil {
		binding = cloneProjectionBinding(binding)
		binding.Def.InputSchema = append(json.RawMessage(nil), schema...)
	}
	return boundDirectExecution{
		surface:     surface,
		assembly:    assembly,
		target:      projected.target,
		binding:     binding,
		inputSchema: append(json.RawMessage(nil), schemaOrProjected(schema, projected.inputSchema)...),
		lease:       lease,
	}
}

func schemaOrProjected(schema, projected json.RawMessage) json.RawMessage {
	if schema != nil {
		return schema
	}
	return projected
}

type directToolResultWire struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StructuredContent map[string]any `json:"structuredContent"`
	IsError           bool           `json:"isError"`
}

type directToolResponseWire struct {
	Result directToolResultWire `json:"result"`
}

func decodeDirectToolResponse(t *testing.T, body []byte) directToolResponseWire {
	t.Helper()
	var response directToolResponseWire
	payload := body
	if dataStart := strings.Index(string(body), "data: "); dataStart >= 0 {
		payload = body[dataStart+len("data: "):]
		if end := strings.IndexByte(string(payload), '\n'); end >= 0 {
			payload = payload[:end]
		}
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("decode direct tool response: %v; body=%s", err, body)
	}
	return response
}

func assertDirectErrorFields(t *testing.T, isError bool, structured map[string]any, code, message, executionState string) {
	t.Helper()
	if !isError {
		t.Fatal("direct error result IsError = false, want true")
	}
	if len(structured) != 4 {
		t.Fatalf("direct error structured content = %#v, want exactly four bridge error fields", structured)
	}
	if got := structured["code"]; got != code {
		t.Errorf("direct error code = %v, want %q", got, code)
	}
	if got := structured["message"]; got != message {
		t.Errorf("direct error message = %v, want %q", got, message)
	}
	if got, ok := structured["retryable"].(bool); !ok || got {
		t.Errorf("direct error retryable = %#v, want false", structured["retryable"])
	}
	if got := structured["execution_state"]; got != executionState {
		t.Errorf("direct error execution_state = %v, want %q", got, executionState)
	}
}

func assertDirectErrorResult(t *testing.T, result *mcp.CallToolResult, code, message, executionState string) {
	t.Helper()
	if result == nil {
		t.Fatal("direct error result is nil")
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("direct error structured content has type %T, want an object", result.StructuredContent)
	}
	assertDirectErrorFields(t, result.IsError, structured, code, message, executionState)
	if len(result.Content) != 1 {
		t.Fatalf("direct error content has %d blocks, want 1", len(result.Content))
	}
	content, ok := result.Content[0].(*mcp.TextContent)
	if !ok || content.Text != message {
		t.Fatalf("direct error content = %#v, want text %q", result.Content[0], message)
	}
}

func TestProgressiveMetadataCapturedDirectExecution(t *testing.T) {
	upstream := &directExecutionUpstream{
		calls:   make(chan directExecutionCall, 1),
		started: make(chan struct{}),
		release: make(chan struct{}),
		result:  json.RawMessage(`{"content":[{"type":"text","text":"captured-result"}]}`),
	}
	harness, sessionID, owner, projection, projected, assembly := newDirectExecutionFixture(t, upstream)
	name := projection.tools[0].Name
	coordinator := harness.surface.projectionCoordinator()
	captured := make(chan struct{})
	resume := make(chan struct{})
	var resumeOnce sync.Once
	resumeCall := func() { resumeOnce.Do(func() { close(resume) }) }
	releaseUpstream := func() {
		select {
		case <-upstream.release:
		default:
			close(upstream.release)
		}
	}
	coordinator.testAfterDirectCapture = func() {
		close(captured)
		<-resume
	}
	defer func() {
		resumeCall()
		coordinator.testAfterDirectCapture = nil
		releaseUpstream()
	}()

	response := make(chan progressiveHTTPResponse, 1)
	go func() {
		response <- harness.send(t, http.MethodPost, sessionID, directCallBody(name, `{"captured":"value"}`), nil)
	}()
	select {
	case <-captured:
	case <-time.After(5 * time.Second):
		t.Fatal("direct call did not reach its captured-admission barrier")
	}
	// Make the live schema stricter after capture; execution must keep validating
	// against the schema paired with the admitted binding.
	currentProjection := cloneDirectTestProjection(coordinator, owner)
	currentTool := currentProjection.byName[name]
	currentTool.binding = cloneProjectionBinding(currentTool.binding)
	currentSchema := json.RawMessage(`{"type":"object","required":["live"]}`)
	currentTool.binding.Def.InputSchema = append(json.RawMessage(nil), currentSchema...)
	currentTool.inputSchema = append(json.RawMessage(nil), currentSchema...)
	currentProjection.byName[name] = currentTool
	replaceDirectTestProjection(coordinator, owner, currentProjection)

	server := assembly.servers[projected.binding.ServerID]
	if leases := trackerLeaseCount(server.tracker); leases != 1 {
		t.Errorf("captured direct admission owns %d leases, want 1", leases)
	}
	coordinator.server.RemoveTools(name)
	next := &Assembly{servers: make(map[string]ServerRuntime)}
	if err := harness.surface.runtime.accept(next, assembly); err != nil {
		t.Fatalf("advance accepted runtime: %v", err)
	}
	retirePrevious(assembly, next, ReloadDeps{Close: func(core.Upstream) error { return nil }})
	resumeCall()

	select {
	case <-upstream.started:
	case <-time.After(5 * time.Second):
		t.Fatal("admitted direct call did not dispatch after registry removal and runtime advancement")
	}
	barrierAvailable := make(chan struct{})
	go func() {
		coordinator.barrier.Lock()
		coordinator.barrier.Unlock()
		close(barrierAvailable)
	}()
	select {
	case <-barrierAvailable:
	case <-time.After(5 * time.Second):
		t.Fatal("coordination barrier remained held during upstream execution")
	}
	releaseUpstream()

	var got progressiveHTTPResponse
	select {
	case got = <-response:
	case <-time.After(5 * time.Second):
		t.Fatal("direct call did not return after upstream release")
	}
	if got.Response == nil || got.Response.StatusCode != http.StatusOK || !strings.Contains(string(got.body), "captured-result") {
		t.Fatalf("captured direct result was not preserved: %#v %s", got.Response, got.body)
	}
	call := <-upstream.calls
	if call.serverID != projected.binding.ServerID || call.rawName != projected.binding.RawName || string(call.args) != `{"captured":"value"}` {
		t.Fatalf("dispatch did not use captured binding and raw arguments: %#v", call)
	}
	if leases := trackerLeaseCount(server.tracker); leases != 0 {
		t.Fatalf("captured lease remains after completion: %d", leases)
	}
}

func TestProgressiveMetadataDirectLeaseOwnership(t *testing.T) {
	upstream := &directExecutionUpstream{
		calls:   make(chan directExecutionCall, 1),
		started: make(chan struct{}),
		release: make(chan struct{}),
		result:  json.RawMessage(`{"content":[{"type":"text","text":"lease-result"}]}`),
	}
	harness, sessionID, _, projection, projected, assembly := newDirectExecutionFixture(t, upstream)
	name := projection.tools[0].Name
	server := assembly.servers[projected.binding.ServerID]
	coordinator := harness.surface.projectionCoordinator()
	captured := make(chan struct{})
	resume := make(chan struct{})
	var resumeOnce sync.Once
	resumeCall := func() { resumeOnce.Do(func() { close(resume) }) }
	releaseUpstream := func() {
		select {
		case <-upstream.release:
		default:
			close(upstream.release)
		}
	}
	coordinator.testAfterDirectCapture = func() {
		close(captured)
		<-resume
	}
	defer func() {
		resumeCall()
		coordinator.testAfterDirectCapture = nil
		releaseUpstream()
	}()

	drained := make(chan struct{}, 1)
	response := make(chan progressiveHTTPResponse, 1)
	go func() {
		response <- harness.send(t, http.MethodPost, sessionID, directCallBody(name, `{}`), nil)
	}()
	select {
	case <-captured:
	case <-time.After(5 * time.Second):
		t.Fatal("direct call did not reach lease transfer")
	}
	if leases := trackerLeaseCount(server.tracker); leases != 1 {
		t.Errorf("lease count before adapter transfer is %d, want 1", leases)
	}
	server.tracker.retire(func() { drained <- struct{}{} })
	select {
	case <-drained:
		t.Fatal("assembly drained while the direct adapter still owned its lease")
	default:
	}
	resumeCall()
	select {
	case <-upstream.started:
	case <-time.After(5 * time.Second):
		t.Fatal("accepted direct call did not reach upstream")
	}
	select {
	case <-drained:
		t.Fatal("assembly drained before execution released its lease")
	default:
	}
	releaseUpstream()
	select {
	case got := <-response:
		if got.Response == nil || got.Response.StatusCode != http.StatusOK || !strings.Contains(string(got.body), "lease-result") {
			t.Fatalf("direct result changed while owning lease: %#v %s", got.Response, got.body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("direct call did not return")
	}
	select {
	case <-drained:
	case <-time.After(5 * time.Second):
		t.Fatal("transferred lease did not drain after execution")
	}
	if leases := trackerLeaseCount(server.tracker); leases != 0 {
		t.Fatalf("lease count after execution is %d, want 0", leases)
	}
	if got := len(upstream.calls); got != 1 {
		t.Fatalf("upstream dispatch count is %d, want 1", got)
	}
	t.Run("validation error releases transferred lease", func(t *testing.T) {
		upstream := &directExecutionUpstream{calls: make(chan directExecutionCall, 1)}
		harness, _, _, _, projected, assembly := newDirectExecutionFixture(t, upstream)
		schema := json.RawMessage(`{"type":"object","required":["value"],"properties":{"value":{"type":"string"}}}`)
		execution := directAdapterForTest(t, harness.surface, assembly, projected, schema)
		server := assembly.servers[projected.binding.ServerID]
		drained := make(chan struct{}, 1)
		server.tracker.retire(func() { drained <- struct{}{} })
		result, err := execution.call(context.Background(), json.RawMessage(`[]`))
		if err != nil || result == nil {
			t.Fatalf("validation result: result=%#v err=%v", result, err)
		}
		assertDirectErrorResult(t, result, "invalid_arguments", "arguments do not satisfy the tool's stored input schema", core.ExecutionNotStarted)
		select {
		case <-drained:
		case <-time.After(5 * time.Second):
			t.Fatal("validation rejection did not release the transferred lease")
		}
		if leases := trackerLeaseCount(server.tracker); leases != 0 {
			t.Errorf("validation rejection retained %d leases, want 0", leases)
		}
		if got := len(upstream.calls); got != 0 {
			t.Fatalf("invalid arguments dispatched %d upstream calls", got)
		}
	})
	t.Run("upstream error releases lease", func(t *testing.T) {
		upstream := &directExecutionUpstream{
			calls:  make(chan directExecutionCall, 1),
			result: json.RawMessage(`{}`),
			outcome: &core.UpstreamOutcome{Failure: &core.UpstreamFailure{
				Code: "upstream_failed", ExecutionState: core.ExecutionUnknown,
			}},
		}
		harness, _, _, _, projected, assembly := newDirectExecutionFixture(t, upstream)
		execution := directAdapterForTest(t, harness.surface, assembly, projected, nil)
		server := assembly.servers[projected.binding.ServerID]
		drained := make(chan struct{}, 1)
		server.tracker.retire(func() { drained <- struct{}{} })
		result, err := execution.call(context.Background(), json.RawMessage(`{}`))
		if err != nil || result == nil {
			t.Fatalf("upstream error mapping: result=%#v err=%v", result, err)
		}
		assertDirectErrorResult(t, result, "upstream_failed", "the upstream call did not complete", core.ExecutionUnknown)
		select {
		case <-drained:
		case <-time.After(5 * time.Second):
			t.Fatal("upstream error did not release the transferred lease")
		}
		if leases := trackerLeaseCount(server.tracker); leases != 0 {
			t.Errorf("upstream error retained %d leases, want 0", leases)
		}
		if got := len(upstream.calls); got != 1 {
			t.Fatalf("upstream error dispatch count is %d, want 1", got)
		}
	})
	t.Run("tool-reported error preserves upstream result", func(t *testing.T) {
		upstream := &directExecutionUpstream{
			calls:  make(chan directExecutionCall, 1),
			result: json.RawMessage(`{"content":[{"type":"text","text":"upstream-reported failure"}],"structuredContent":{"origin":"upstream","details":{"count":2}},"isError":true}`),
		}
		harness, sessionID, _, projection, projected, _ := newDirectExecutionFixture(t, upstream)
		got := harness.send(t, http.MethodPost, sessionID, directCallBody(projection.tools[0].Name, `{}`), nil)
		if got.Response == nil || got.Response.StatusCode != http.StatusOK {
			t.Fatalf("tool-reported error response: %#v %s", got.Response, got.body)
		}
		result := decodeDirectToolResponse(t, got.body).Result
		if !result.IsError {
			t.Fatal("tool-reported error lost IsError=true")
		}
		if len(result.Content) != 1 || result.Content[0].Type != "text" || result.Content[0].Text != "upstream-reported failure" {
			t.Fatalf("tool-reported error content = %#v, want the original upstream text block", result.Content)
		}
		details, ok := result.StructuredContent["details"].(map[string]any)
		if len(result.StructuredContent) != 2 || !ok || result.StructuredContent["origin"] != "upstream" || len(details) != 1 || details["count"] != float64(2) {
			t.Fatalf("tool-reported error structured content = %#v, want the original upstream object", result.StructuredContent)
		}
		if _, substituted := result.StructuredContent["code"]; substituted {
			t.Fatalf("tool-reported error was replaced with a bridge error envelope: %#v", result.StructuredContent)
		}
		if got := len(upstream.calls); got != 1 {
			t.Fatalf("tool-reported error dispatch count is %d, want 1", got)
		}
		if leases := trackerLeaseCount(harness.assembly.servers[projected.binding.ServerID].tracker); leases != 0 {
			t.Fatalf("tool-reported error retained %d leases", leases)
		}
	})
	t.Run("cancellation releases lease", func(t *testing.T) {
		upstream := &directExecutionUpstream{
			calls:         make(chan directExecutionCall, 1),
			started:       make(chan struct{}),
			waitForCancel: true,
		}
		harness, _, _, _, projected, assembly := newDirectExecutionFixture(t, upstream)
		execution := directAdapterForTest(t, harness.surface, assembly, projected, nil)
		server := assembly.servers[projected.binding.ServerID]
		drained := make(chan struct{}, 1)
		server.tracker.retire(func() { drained <- struct{}{} })
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan struct{})
		var result *mcp.CallToolResult
		var callErr error
		go func() {
			result, callErr = execution.call(ctx, json.RawMessage(`{}`))
			close(done)
		}()
		select {
		case <-upstream.started:
		case <-time.After(5 * time.Second):
			t.Fatal("cancel test did not reach upstream")
		}
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("cancelled execution did not return")
		}
		if callErr != nil {
			t.Fatalf("cancelled execution returned error: %v", callErr)
		}
		assertDirectErrorResult(t, result, "cancelled", "the upstream call did not complete", core.ExecutionUnknown)
		select {
		case <-drained:
		case <-time.After(5 * time.Second):
			t.Fatal("cancellation did not release the transferred lease")
		}
		if leases := trackerLeaseCount(server.tracker); leases != 0 {
			t.Errorf("cancelled execution retained %d leases, want 0", leases)
		}
		if got := len(upstream.calls); got != 1 {
			t.Errorf("cancelled execution dispatch count is %d, want 1", got)
		}
	})
	t.Run("panic unwinding releases lease", func(t *testing.T) {
		upstream := &directExecutionUpstream{
			calls:       make(chan directExecutionCall, 1),
			panicOnCall: true,
		}
		harness, _, _, _, projected, assembly := newDirectExecutionFixture(t, upstream)
		execution := directAdapterForTest(t, harness.surface, assembly, projected, nil)
		server := assembly.servers[projected.binding.ServerID]
		drained := make(chan struct{}, 1)
		server.tracker.retire(func() { drained <- struct{}{} })
		var panicValue any
		func() {
			defer func() { panicValue = recover() }()
			_, _ = execution.call(context.Background(), json.RawMessage(`{}`))
		}()
		if panicValue == nil {
			t.Fatal("upstream panic did not unwind through the bound adapter")
		}
		select {
		case <-drained:
		case <-time.After(5 * time.Second):
			t.Fatal("panic unwinding did not release the transferred lease")
		}
	})
	t.Run("pre-transfer panic releases admission lease", func(t *testing.T) {
		upstream := &directExecutionUpstream{calls: make(chan directExecutionCall, 1)}
		harness, _, _, _, projected, assembly := newDirectExecutionFixture(t, upstream)
		coordinator := harness.surface.projectionCoordinator()
		execution := directAdapterForTest(t, harness.surface, assembly, projected, nil)
		server := assembly.servers[projected.binding.ServerID]
		drained := make(chan struct{}, 2)
		var leasesAtCapture int
		sentinel := &struct{ message string }{message: "before transfer"}
		coordinator.testAfterDirectCapture = func() {
			leasesAtCapture = trackerLeaseCount(server.tracker)
			server.tracker.retire(func() { drained <- struct{}{} })
			panic(sentinel)
		}
		defer func() { coordinator.testAfterDirectCapture = nil }()

		var panicValue any
		func() {
			defer func() { panicValue = recover() }()
			_, _ = harness.surface.callDirectExecution(context.Background(), execution, json.RawMessage(`{}`))
		}()
		if panicValue != sentinel {
			t.Fatalf("pre-transfer panic = %#v, want sentinel %#v", panicValue, sentinel)
		}
		if leasesAtCapture != 1 {
			t.Fatalf("lease count at capture was %d, want 1", leasesAtCapture)
		}
		select {
		case <-drained:
		case <-time.After(5 * time.Second):
			t.Fatal("pre-transfer panic did not release the admission lease")
		}
		select {
		case <-drained:
			t.Fatal("pre-transfer panic drained the server more than once")
		default:
		}
		if leases := trackerLeaseCount(server.tracker); leases != 0 {
			t.Fatalf("pre-transfer panic retained %d leases", leases)
		}
		if got := len(upstream.calls); got != 0 {
			t.Fatalf("pre-transfer panic dispatched %d upstream calls", got)
		}
	})
	t.Run("invisible names and stale projections do not dispatch", func(t *testing.T) {
		upstream := &directExecutionUpstream{calls: make(chan directExecutionCall, 1)}
		harness, sessionA, ownerA, projectionA, projected, assembly := newDirectExecutionFixture(t, upstream)
		name := projectionA.tools[0].Name
		for _, hidden := range []string{"unknown-name", "undiscovered-name"} {
			harness.send(t, http.MethodPost, sessionA, directCallBody(hidden, `{}`), nil)
		}
		sessionB := harness.initialize(t)
		harness.send(t, http.MethodPost, sessionB, `{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`, nil)
		ownerB := harness.owner(t, sessionB)
		originalB := harness.projection(ownerB)

		inconsistent := cloneDirectTestProjection(harness.surface.projectionCoordinator(), ownerB)
		inconsistent.owner = ownerA
		replaceDirectTestProjection(harness.surface.projectionCoordinator(), ownerB, inconsistent)
		harness.send(t, http.MethodPost, sessionB, directCallBody(name, `{}`), nil)
		replaceDirectTestProjection(harness.surface.projectionCoordinator(), ownerB, originalB)

		stale := cloneDirectTestProjection(harness.surface.projectionCoordinator(), ownerB)
		stale.runtimeGeneration = assembly.Generation() - 1
		replaceDirectTestProjection(harness.surface.projectionCoordinator(), ownerB, stale)
		harness.send(t, http.MethodPost, sessionB, directCallBody(name, `{}`), nil)
		replaceDirectTestProjection(harness.surface.projectionCoordinator(), ownerB, originalB)

		coordinator := harness.surface.projectionCoordinator()
		coordinator.barrier.Lock()
		coordinator.acceptedGeneration = assembly.Generation() + 1
		coordinator.barrier.Unlock()
		harness.send(t, http.MethodPost, sessionA, directCallBody(name, `{}`), nil)
		coordinator.barrier.Lock()
		coordinator.acceptedGeneration = assembly.Generation()
		coordinator.barrier.Unlock()

		removeDirectTestProjectionTool(coordinator, ownerA, name)
		harness.send(t, http.MethodPost, sessionA, directCallBody(name, `{}`), nil)
		if got := len(upstream.calls); got != 0 {
			t.Fatalf("invisible direct names dispatched %d upstream calls", got)
		}
		if leases := trackerLeaseCount(assembly.servers[projected.binding.ServerID].tracker); leases != 0 {
			t.Fatalf("rejected direct calls retained %d leases", leases)
		}
	})
	t.Run("inconsistent captured schema rejects admission", func(t *testing.T) {
		upstream := &directExecutionUpstream{calls: make(chan directExecutionCall, 1)}
		harness, sessionID, owner, projection, projected, assembly := newDirectExecutionFixture(t, upstream)
		name := projection.tools[0].Name
		inconsistent := cloneDirectTestProjection(harness.surface.projectionCoordinator(), owner)
		current := inconsistent.byName[name]
		current.inputSchema = append(append(json.RawMessage(nil), current.inputSchema...), ' ')
		inconsistent.byName[name] = current
		replaceDirectTestProjection(harness.surface.projectionCoordinator(), owner, inconsistent)

		got := harness.send(t, http.MethodPost, sessionID, directCallBody(name, `{}`), nil)
		if got.Response == nil || got.Response.StatusCode != http.StatusOK {
			t.Fatalf("inconsistent-schema rejection response: %#v %s", got.Response, got.body)
		}
		result := decodeDirectToolResponse(t, got.body).Result
		assertDirectErrorFields(t, result.IsError, result.StructuredContent, "catalog_changed", "the catalog revision changed", core.ExecutionNotStarted)
		if got := len(upstream.calls); got != 0 {
			t.Fatalf("inconsistent-schema call dispatched %d upstream calls", got)
		}
		if leases := trackerLeaseCount(assembly.servers[projected.binding.ServerID].tracker); leases != 0 {
			t.Fatalf("inconsistent-schema rejection retained %d leases", leases)
		}
	})
	t.Run("draining server rejects without dispatch", func(t *testing.T) {
		upstream := &directExecutionUpstream{calls: make(chan directExecutionCall, 1)}
		harness, sessionID, _, projection, projected, assembly := newDirectExecutionFixture(t, upstream)
		server := assembly.servers[projected.binding.ServerID]
		server.tracker.retire(func() {})
		harness.send(t, http.MethodPost, sessionID, directCallBody(projection.tools[0].Name, `{}`), nil)
		if got := len(upstream.calls); got != 0 {
			t.Fatalf("draining direct call dispatched %d upstream calls", got)
		}
		if leases := trackerLeaseCount(server.tracker); leases != 0 {
			t.Fatalf("draining rejection retained %d leases", leases)
		}
	})
}
