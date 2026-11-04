# Stateless Tool Call Execution Bugfix Specification (r2)

Status: ready-for-agent

<!-- specdd:requirements:start -->
### 1. Requirements
- R1: In stateless transport mode (`Stateless: true`, `MCP_STATELESS=true`), pinned preloaded tool calls arriving from a stateless client (`req.GetSession() == nil || req.GetSession().ID() == ""`) must resolve `t.publisherOwner` as the request owner instead of failing with `codeUnknownTool` / `"unknown preloaded tool"`.
- R2: In stateless transport mode, built-in tool calls (`search_tools`, `describe_tool`) arriving from a stateless client must resolve `t.publisherOwner` as the request owner, pass `discoveryOwnerValid`, and execute without failing with `codeCatalogChanged` / `"the catalog revision changed"`.
- R3: In stateless transport mode, `commitDiscovery` must acquire `publicationMu`, recheck `discoveryOwnerValid(owner, assembly)` for completion-time generation validity, and for `owner.kind == projectionPublisher`, return `nil` without mutating the shared publisher projection.
- R4: Stateful session requests (`req.GetSession() != nil && req.GetSession().ID() != ""`) must preserve existing behavior verbatim: require valid exposure epoch header, lookup live session owner, and enforce progressive discovery mutation and validation.
- R5: `TestChatGPTDeveloperModeStatelessDiscoveryWire` in `cmd/mcp-bridge/serve_oauth_test.go` must configure a pinned tool fixture (`alpha__read`), assert that `alpha__read` is present in `tools/list`, and assert that direct `tools/call` for `alpha__read` and `search_tools` succeed without `isError` or `unknown preloaded tool` or `the catalog revision changed`.
- R6: Full repository test suite passes clean under `go test -race -p 1 -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...` and meets the aggregate coverage gate (`go run ./cmd/covcheck` >= 90.0%).
<!-- specdd:requirements:end -->

<!-- specdd:design:start -->
### 2. Design
In `internal/service/tools.go`:
Change `directRequestOwner(req *mcp.CallToolRequest)` to a method `(t *ToolSurface) directRequestOwner(req *mcp.CallToolRequest) (projectionOwner, bool)`:
```go
func (t *ToolSurface) directRequestOwner(req *mcp.CallToolRequest) (projectionOwner, bool) {
	if req == nil {
		return projectionOwner{}, false
	}
	session := req.GetSession()
	if session == nil || session.ID() == "" {
		t.projectionCoordinator()
		return t.publisherOwner, true
	}
	epoch, valid := requestExposureEpoch(req)
	if !valid {
		return projectionOwner{}, false
	}
	return projectionOwner{kind: projectionSession, sessionID: session.ID(), epoch: epoch}, true
}
```
Update callsites in `callPinnedTool`, `searchTools`, and `describeTool` from `directRequestOwner(req)` to `t.directRequestOwner(req)`.

In `internal/service/projection.go`:
In `discoveryOwnerValid`:
```go
func (t *ToolSurface) discoveryOwnerValid(owner projectionOwner, assembly *Assembly) bool {
	if (owner.kind != projectionSession && owner.kind != projectionPublisher) || assembly == nil {
		return false
	}
	coordinator := t.projectionCoordinator()
	coordinator.barrier.Lock()
	defer coordinator.barrier.Unlock()
	projection := coordinator.projections[owner]
	_, live := coordinator.liveOwners[owner]
	return t.runtime.current.Load() == assembly &&
		coordinator.acceptedKnown &&
		coordinator.acceptedGeneration == assembly.Generation() &&
		live &&
		projection != nil &&
		projection.owner == owner &&
		projection.runtimeGeneration == assembly.Generation()
}
```
In `commitDiscovery`:
```go
func (t *ToolSurface) commitDiscovery(owner projectionOwner, assembly *Assembly, targets map[logicalToolTarget]struct{}) error {
	if assembly == nil {
		return errAcceptedGenerationMismatch
	}
	coordinator := t.projectionCoordinator()
	coordinator.publicationMu.Lock()
	defer coordinator.publicationMu.Unlock()
	if !t.discoveryOwnerValid(owner, assembly) {
		return errAcceptedGenerationMismatch
	}
	if owner.kind == projectionPublisher {
		// Stateless clients validate completion-time generation but do not mutate the shared publisher projection.
		return nil
	}
	if owner.kind != projectionSession {
		return errAcceptedGenerationMismatch
	}
	// ... remainder of commitDiscovery for projectionSession unchanged
```

In `cmd/mcp-bridge/serve_oauth_test.go`:
Add `writeBridgeConfigWithPins` helper or update harness config writer so `TestChatGPTDeveloperModeStatelessDiscoveryWire` configures `pins: [{server: "alpha", tool: "read"}]`.
In `TestChatGPTDeveloperModeStatelessDiscoveryWire`:
1. Verify `tools/list` returns `alpha__read`, `search_tools`, `describe_tool`, and `call_tool`.
2. Call `tools/call` with `name: "alpha__read"`, `arguments: {"path": "test"}`:
   assert `result["isError"] != true`, verifying that preloaded tool calls succeed in stateless mode (R1).
3. Call `tools/call` with `name: "search_tools"`, `arguments: {"query": "test"}`:
   assert `result["isError"] != true`, verifying that search tools succeed in stateless mode (R2, R3).
<!-- specdd:design:end -->

<!-- specdd:tasks:start -->
### 3. Tasks & Acceptance Criteria
- Tasks:
  - T1: Stateless Tool Call and Preloaded Tool Execution Admission
    - Files: `internal/service/tools.go`, `internal/service/projection.go`, `cmd/mcp-bridge/serve_oauth_test.go`, `coverage.out`
    - Acceptance: AC1, AC2, AC3, AC4, AC5, AC6
- Acceptance Criteria:
  - AC1: `directRequestOwner` returns `t.publisherOwner, true` when `req.GetSession() == nil || req.GetSession().ID() == ""`.
  - AC2: `discoveryOwnerValid` accepts `projectionPublisher` alongside `projectionSession`.
  - AC3: `commitDiscovery` acquires `publicationMu`, revalidates `discoveryOwnerValid`, and returns `nil` for `projectionPublisher`.
  - AC4: Stateful requests preserve epoch validation and session owner checks (verified by `TestDirectExecutionRejectsUnauthenticatedCaller`).
  - AC5: `TestChatGPTDeveloperModeStatelessDiscoveryWire` asserts `search_tools` and pinned `alpha__read` calls succeed without `isError` or `unknown preloaded tool` or `the catalog revision changed`.
  - AC6: Full test suite passes under race detector with fresh aggregate coverage meeting >= 90.0%.
<!-- specdd:tasks:end -->

<!-- specdd:envelope:start -->
### 4. Envelope & Verification
- Allowed files:
  - `internal/service/tools.go`
  - `internal/service/projection.go`
  - `cmd/mcp-bridge/serve_oauth_test.go`
  - `coverage.out`
- Prohibited scope: auth middleware, router configuration, config schemas, storage schemas.
- RED/GREEN sequence:
  - RED: Update `TestChatGPTDeveloperModeStatelessDiscoveryWire` with pinned tool fixture and assertions; observe failure (`unknown preloaded tool` on `alpha__read` and `the catalog revision changed` on `search_tools`).
  - GREEN: Update `internal/service/tools.go` and `internal/service/projection.go`; observe test passing.
- Full verification command:
  `go test -race -p 1 -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./... && go run ./cmd/covcheck`
<!-- specdd:envelope:end -->

<!-- specdd:observability:start -->
### 5. Observability Assessment
Existing structured logging and metrics are strictly preserved:
- `t.logToolAdmissionFailure` emits structured `tool_admission_failure` events with finite, typed reasons (`unknown_tool`, `catalog_changed`, `invalid_arguments`).
- `backend_complete` log event records upstream dispatch outcomes and latencies.
- Admission outcomes for `projectionPublisher` reuse existing request and correlation logging without introducing new log events or high-cardinality label values.
- Justified exception: No new metrics or Prometheus labels are needed as stateless requests increment existing `http_requests_total` and operational counters.
<!-- specdd:observability:end -->

<!-- specdd:rollback:start -->
### 6. Rollback Procedure
If regressions occur:
1. Revert changes to `internal/service/tools.go`, `internal/service/projection.go`, and `cmd/mcp-bridge/serve_oauth_test.go` (`git checkout HEAD -- internal/service/ cmd/mcp-bridge/`).
2. Run `go test -race ./...` to verify clean restoration of prior state.
<!-- specdd:rollback:end -->
