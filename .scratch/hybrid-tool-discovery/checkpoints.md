# Hybrid Tool Discovery — Checkpoint Evidence

Canonical specification: [HYBRID-DISCOVERY-r3](spec.md)
Ticket graph & dependency map: [map.md](map.md)

## Specification & Review History

### 1. Initial Specification (r1) Adversarial Review
- Agent: `HybridDiscoveryAnalyst` (analyst)
- Verdict: **BLOCK** (4 Critical, 0 High, 5 Medium, 2 Low)
- Key findings:
  1. Wire name pattern contradiction: existing schemas restrict tool names to `^t_[a-z2-7]{52}$`, while r1 proposed names like `rag__search_documents`.
  2. Nonexistent component cited: r1 referenced an unbuilt "bridge tool name sanitizer".
  3. LKG semantic reversal: r1 proposed fail-closed reload on unreachable pins, reversing established LKG behavior.
  4. Missing normative bounds: unbounded preloaded tools, unconstrained wire name lengths, unbounded schema sizes.

### 2. Revision r2 Amendments
- Regex contract amendment: added single regex alternation `^(t_[a-z2-7]{52}|[a-z][a-z0-9-]{0,31}__[a-zA-Z0-9_-]{1,94})$`.
- Removed phantom sanitizer reference; established normative validation against `^[a-zA-Z0-9_-]+$`.
- Reconciled reload semantics: startup fails closed; runtime reload preserves LKG for unreachable upstreams.
- Added normative limits: max 64 preloaded tools, max 128-char wire names, max 64 KiB schemas.

### 3. Revision r2 Independent Review
- Agent: `HybridDiscoveryReviewer` (analyst)
- Verdict: **APPROVE-WITH-AMENDMENTS** (0 Critical, 0 High, 5 Medium, 8 Low)
- Verified all 4 prior critical findings resolved cleanly.
- Recommended 5 medium clarifications folded into r3.

### 4. Revision r3 Consolidation
- Specification updated to `HYBRID-DISCOVERY-r3`:
  1. *Reload deletion:* If an upstream server is reachable but an existing pinned tool was deleted, reload fails closed atomically with a typed diagnostic.
  2. *Oracle definition:* `active_pins` in `3 + len(active_pins)` defined as the successfully resolved preloaded tools in the active generation.
  3. *Degraded LKG surface:* Degraded LKG tools remain advertised in `tools/list` with cached schemas; direct execution returns `isError: true` with reason `backend_unavailable`.
  4. *Validation channel:* Argument validation failures standardized to MCP tool error results with `isError: true`.
  5. *Wire name scope:* Clarified `describe_tool` accepts canonical wire names and hash names (no un-namespaced names).
  6. *Tickets re-baselined:* Tickets 01–05 updated to cite `HYBRID-DISCOVERY-r3`.

---

## SpecDD Checkpoints Ledger

- [x] **Checkpoint 1 (Requirements)**: User Stories 1–30, problem statement, and scope boundaries in `HYBRID-DISCOVERY-r3`. Confirmed and approved by user (2026-10-19).
- [x] **Checkpoint 2 (Design)**: Contract Amendments 1–2, Implementation Decisions 1–7, and Testing Decisions in `HYBRID-DISCOVERY-r3`. Confirmed and approved by user (2026-10-19).
- [x] **Checkpoint 3 (Task Ledger & Implementation Gate)**: Five-ticket graph 01 → 02 → {03, 04} → 05 in `.scratch/hybrid-tool-discovery/issues/`. Confirmed and approved by user (2026-10-19). Implementation gate open.

## Ticket 01 Execution Evidence

- TDD test guide established deterministic frozen RED baseline in `internal/config/pins_bounds_test.go` (`TestPinCountBounds`, `TestPinToolNameValidation`, `TestWireToolNameHelpers`).
- `Ticket01ExecutionEngineer` implemented:
  - Pin ceiling check in `internal/config/config.go`: $\le 64$ configured pins allowed; $>64$ rejected with `pin_ceiling` diagnostic.
  - Tool name validation: standard identifier characters `^[a-zA-Z0-9_-]+$` enforced (`pin_tool_name` / `invalid_chars`); total wire name length $\le 128$ characters enforced (`pin_tool_name` / `too_long`).
  - Canonical helpers `WireToolName(server, tool)` and `ParseWireToolName(wire)`.
  - Contract Amendment 1 applied in `internal/catalog/schema.go`: `PublicToolNamePattern = ^(t_[a-z2-7]{52}|[a-z][a-z0-9-]{0,31}__[a-zA-Z0-9_-]{1,94})$`.
  - Schema regex tests added in `internal/catalog/catalog_test.go`.
- Verification: `go test ./internal/config` (exit 0) and `go test ./internal/catalog` (exit 0). Full-suite source-line coverage at 91.9%.
- Ticket 01 is complete and verified.

## Ticket 02 Execution Evidence

- TDD test guide `Ticket02TddGuide-2` established deterministic frozen RED baseline in `cmd/mcp-bridge/serve_hybrid_exposure_test.go` (`TestServeHybridPreloadedToolsListExposure`).
- `Ticket02ExecutionEngineer-2` and `Ticket02AlignEngineer` implemented:
  - Deferred startup pin resolution in `internal/service/reload.go`: in `buildStartup` mode (contacts no upstream, catalog holds only durable snapshot), pins are deferred (`next.pins = nil`). Live discovery pass resolves pins in `buildReload` mode via `core.AddPins`.
  - Publication hook in `cmd/mcp-bridge/serve.go`: wired `surface.Published(assembly)` inside `Loop.Start`.
  - Dynamic tool surface reconciliation in `internal/service/tools.go`: `Published(assembly)` synchronizes pinned tools on `mcp.Server` with canonical wire names (`<server_id>__<upstream_tool_name>`), upstream descriptions, and full `InputSchema` (`json.RawMessage`). Idempotent updates skip re-registration, and unpinned tools are removed via `RemoveTools`.
  - Transport & test contract alignment: restored standard HTTP 202 Accepted for MCP notifications in `internal/service/inbound.go` and `internal/service/server.go`; aligned `readToolSchema` in `serve_hybrid_exposure_test.go` with catalog-canonical compact sorted-key JSON representation (`{"properties":...,"required":...,"type":"object"}`).
- Verification:
  - `go test ./cmd/mcp-bridge -run '^TestServeHybridPreloadedToolsListExposure$' -count=1` -> ok (0.035s).
  - Full test suite: `go test ./cmd/mcp-bridge/... ./internal/... -count=1` -> 11 packages ok (2.6s).
  - Coverage gate: `go run ./cmd/covcheck -profile=/tmp/coverage.out` -> 91.8% (3728/4062), gate 90%: true.
- Ticket 02 is complete and verified.

## Ticket 03 Execution Evidence

- TDD test guide `Ticket03TddGuide-2` established deterministic frozen RED baseline in `cmd/mcp-bridge/serve_hybrid_dispatch_test.go` (`TestServeHybridDirectPreloadedToolExecution`), confirming deterministic failure against the `pendingDispatch` stub.
- `Ticket03ExecutionEngineer-2` implemented:
  - Replaced `pendingDispatch` placeholder with `callPinnedTool` in `internal/service/tools.go`.
  - Direct execution dispatch flow: resolves pin from `t.pinnedTools` (under `t.mu`) with `assembly.Pins()` fallback for publication races; validates arguments against `binding.Def.InputSchema` with fail-closed rejection (`search.CodeInvalidArguments`, `IsError: true`, upstream not hit); acquires per-server execution lease via `assembly.Acquire`; dispatches through `assembly.catalog.Dispatch` using canonical public hash name; unpacks and returns upstream content/structured payload/IsError verbatim as MCP `CallToolResult`.
  - Observability: emits structured `backend_complete` and `mcp_complete` entries recording duration, operation, request ID, backend call ID, and outcome/reason tokens.
- Verification:
  - Focused selector `TestServeHybridDirectPreloadedToolExecution` (Success, InvalidArgument, UnknownTool): ok (0.039s).
  - Both hybrid tests: `go test ./cmd/mcp-bridge -run '^TestServeHybrid' -count=1` -> ok.
  - Full test suite: `go test ./cmd/mcp-bridge/... ./internal/... -count=1` -> 11 packages ok.
  - Coverage gate: `go run ./cmd/covcheck -profile=/tmp/coverage.out` -> 91.4% (3772/4129), gate 90%: true.
- Ticket 03 is complete and verified.

## Ticket 04 Execution Evidence

- TDD test guide `Ticket04TddGuide-2` established deterministic frozen RED baseline in `cmd/mcp-bridge/serve_hybrid_coexistence_test.go` (`TestServeHybridCoexistence`), confirming deterministic failures on wire-name lookup gaps in `search_tools`, `describe_tool`, and `call_tool`.
- `Ticket04ExecutionEngineer-2` implemented:
  - Wire name resolution helper `resolveBinding` in `internal/service/tools.go`: transparently bridges wire-name format (`<server>__<raw>`) and opaque hash names (`t_...`) to their canonical catalog binding and resolved public key.
  - `describeTool`: resolves both wire names and hash names; sets `"pinned"` based on the resolved tool's public key in `assembly.PinnedNames()`.
  - `callTool`: resolves both wire names and hash names; dispatches upstream through `assembly.catalog.Dispatch` using the resolved canonical public name.
  - `searchPayload`: preloaded tools report their advertised wire name (`config.WireToolName`) alongside `"pinned": true`; long-tail tools retain the opaque hash name (`t_...`) alongside `"pinned": false`.
- Verification:
  - Focused selector `TestServeHybridCoexistence` (SearchToolsPinnedAndUnpinned, DescribeToolByWireName, CallToolEnvelopeByWireName): ok (0.037s).
  - All hybrid suites: `go test ./cmd/mcp-bridge -run '^TestServeHybrid' -count=1` -> ok (0.094s).
  - Full test suite: `go test ./cmd/mcp-bridge/... ./internal/... -count=1` -> 11 packages ok.
  - Coverage gate: `go run ./cmd/covcheck -profile=/tmp/coverage.out` -> 91.4% (3784/4141), gate 90%: true.
- Ticket 04 is complete and verified.

## Ticket 05 Execution Evidence

- TDD test guide `Ticket05TddGuide-2` authored integration tests in `cmd/mcp-bridge/serve_hybrid_reload_test.go` (`TestServeHybridSIGHUPReload`), testing:
  - `PinAdditionViaSIGHUP`: config rewrite + SIGHUP adds `alpha__write`, delivers `notifications/tools/list_changed` over SSE via `ToolListChangedHandler`, updates `tools/list` immediately to include both pins, and enables direct execution of `alpha__write`.
  - `PinRemovalViaSIGHUP`: SIGHUP removes `alpha__read`, delivers `notifications/tools/list_changed`, removes `alpha__read` from `tools/list`, and direct execution of `alpha__read` fails closed with unknown-tool rejection.
  - `InvalidNewPinRejectedViaSIGHUP`: candidate introducing invalid pin `alpha/nonexistent` is rejected atomically with zero client notification, preserving running generation and keeping existing pins callable.
- `Ticket05TddGuide-2-2` authored unit test in `internal/service/refresh_test.go` (`TestRefreshPreservesLKGPinsForFailedServer`):
  - Verifies that when an upstream fails discovery during refresh, LKG catalog definitions and pins are preserved with active bindings (`len(next.Pins()) == 1`, `Binding != nil`).
  - Verifies that `Assembly.Acquire` fails closed (`ok == false`, `lease == nil`) for unavailable/degraded servers, guaranteeing that execution during outages fails safely.
- Verification:
  - Focused selector `TestServeHybridSIGHUPReload`: ok (2.06s).
  - Focused selector `TestRefreshPreservesLKGPinsForFailedServer`: ok (0.00s).
  - All hybrid suites: `go test ./cmd/mcp-bridge -run '^TestServeHybrid' -count=1` -> ok (2.15s).
  - Full test suite: `go test ./cmd/mcp-bridge/... ./internal/... -count=1` -> 11 packages ok.
  - Coverage gate: `go run ./cmd/covcheck -profile=/tmp/coverage.out` -> 91.6% (3793/4143), gate 90%: true.
- Ticket 05 is complete and verified.

## Independent Review & Final Quality Verification

- Independent security review (`HybridSecurityReviewer`): **PASS-WITH-WARNINGS** (no criticals or highs; validated pin ceiling $\le 64$, wire name bounds, schema validation, fail-closed error formatting, zero secret leakage in logs).
- Independent code review (`HybridCodeReviewer`): identified 1 Critical (C1: data race on searcher cache under concurrent requests) and 1 High (H1: 94-char tool name bound mismatch with `PublicToolNamePattern`).
- `Ticket05ReviewFixEngineer` resolved both findings:
  - C1: `internal/service/tools.go`: protected `t.cached` and `t.cacheKey` reads and writes under `t.mu.Lock()` with early unlock on hit and lock acquisition on write. Added `TestToolSurfaceConcurrentSearcher` in `internal/service/tools_test.go` (16 workers $\times$ 4 concurrent queries). Verified race-free with `go test -race`.
  - H1: `internal/config/config.go`: enforced `len(tool) > 94 || len(pin["server"])+2+len(tool) > 128`, rejecting overlong tool names with `Code: "pin_tool_name"`, `Shape: "too_long"` aligned to `PublicToolNamePattern`.
- Final Verification:
  - `go test -race ./internal/service -run 'TestToolSurfaceConcurrentSearcher' -count=1` -> ok (1.087s)
  - `go test -race ./cmd/mcp-bridge -run '^TestServeHybrid' -count=1` -> ok (3.260s)
  - Full suite: `go test -coverpkg=./... -coverprofile=/tmp/coverage.out ./...` -> 11 packages ok.
  - Coverage gate: `go run ./cmd/covcheck -profile=/tmp/coverage.out` -> 91.6% (3799/4149), gate 90%: true.
- All 5 tickets in `HYBRID-DISCOVERY-r3` are complete, reviewed, and verified.
