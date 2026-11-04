# Upstream Stateless Wiring Bugfix Specification (r6)

Status: ready-for-agent

<!-- specdd:requirements:start -->
### 1. Requirements
- R1: In `cmd/mcp-bridge/wiring.go`, `newClient` must pass `Stateless: server.ServerStateless()` to `upstream.DedicatedOptions` when instantiating `upstream.Dedicated`, ensuring each backend runtime respects its configured or default stateless mode.
- R2: In default stateless mode (`stateless: true`), subsequent tool calls following an upstream backend server restart must succeed without failing with `backend_unavailable`, connection drops, or stale session errors, executing against the restarted generation.
- R3: In explicit stateful mode (`stateless: false`), the upstream client must preserve its persistent session across multiple tool calls without reconnecting on every call.
- R4: Full repository test suite passes clean under `go test -race -p 1 -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...` and meets the aggregate coverage gate (`go run ./cmd/covcheck` >= 90.0%).
<!-- specdd:requirements:end -->

<!-- specdd:design:start -->
### 2. Design
In `cmd/mcp-bridge/wiring.go`:
In `func (r *upstreamRegistry) newClient(server config.Server) (core.Upstream, error)`:
Line 175 currently reads:
```go
client, err := upstream.NewDedicated(upstream.DedicatedOptions{Server: options})
```
Update to:
```go
client, err := upstream.NewDedicated(upstream.DedicatedOptions{Server: options, Stateless: server.ServerStateless()})
```

In `cmd/mcp-bridge/serve_upstream_restart_test.go`:
Add integration tests:
1. `TestUpstreamRestartSurvivesInDefaultStatelessMode`:
   - Construct a dynamic peer handler modeled on `internal/upstream/dedicated_test.go:250-318`:
     A factory `newPeerHandler(generation string)` creates a fresh `mcp.NewServer` exposing a `read` tool returning `{"generation": generation}`, wrapped in a fresh `mcp.NewStreamableHTTPHandler`.
   - The test HTTP server wraps an `activeHandler` pointer protected by an `RWMutex`, acquiring `RLock()`, copying the handler reference, releasing `RUnlock()` before calling `handler.ServeHTTP(writer, request)` so in-flight requests never block swaps.
   - Configure a bridge instance targeting this server with default settings (`stateless` omitted from YAML, defaulting to `true`).
   - Execute an initial `tools/call` for the `read` tool through the bridge; assert `isError == false`, decoded structured generation is `"before"`, and pre-restart execution count is 1.
   - Swap `activeHandler` to `newPeerHandler("after")` to simulate an upstream server restart where the old session storage is discarded.
   - Execute a second `tools/call` for the `read` tool through the bridge; assert `isError == false`, decoded structured generation is `"after"`, and post-restart execution count is 1.
   - On RED (before the wiring fix), this second call fails with `backend_unavailable` because the bridge's Dedicated runtime retained the dead session from the first generation. On GREEN, the call succeeds statelessly against the fresh generation.
2. `TestUpstreamPreservesSessionInExplicitStatefulMode`:
   - Configure a bridge instance with an upstream server explicitly set to `stateless: false` in YAML.
   - Execute two consecutive `tools/call` requests through the bridge; assert both succeed and the upstream server observes exactly 1 initialize request and 2 tool executions across both calls, proving session reuse.
3. Observability assertion within `serve_upstream_restart_test.go`:
   - For an upstream peer advertising a legacy protocol version (< 2026-07-28), capture `os.Stdout` (which `internal/upstream/dedicated.go:52` defaults to when `opts.LogWriter == nil`) via pipe redirection (`os.Pipe()`) with strict `t.Cleanup` restoration.
   - Parse and assert that `upstream_ephemeral_handshake` is emitted as valid JSON with `level == "info"`, `event == "upstream_ephemeral_handshake"`, and matching `backend_id`.
<!-- specdd:design:end -->

<!-- specdd:tasks:start -->
### 3. Tasks & Acceptance Criteria
- Tasks:
  - T1: Upstream Server Stateless Wiring and Restart Resilience
    - Files: `cmd/mcp-bridge/wiring.go`, `cmd/mcp-bridge/serve_upstream_restart_test.go`, `coverage.out`
    - Acceptance: AC1, AC2, AC3, AC4
- Acceptance Criteria:
  - AC1: `newClient` in `cmd/mcp-bridge/wiring.go` passes `Stateless: server.ServerStateless()` to `upstream.DedicatedOptions`.
  - AC2: In default stateless mode, `tools/call` succeeds after an upstream server restart with fresh handler session state, returning generation `"after"` and `isError == false`.
  - AC3: In explicit stateful mode (`stateless: false`), persistent session reuse across multiple calls is preserved (1 initialization, 2 executions).
  - AC4: Full test suite passes under race detector with fresh aggregate coverage meeting >= 90.0%, and logging/metrics regressions in `serve_logging_compatibility_test.go` and `serve_upstream_restart_test.go` pass clean.
<!-- specdd:tasks:end -->

<!-- specdd:envelope:start -->
### 4. Envelope & Verification
- Allowed files:
  - `cmd/mcp-bridge/wiring.go`
  - `cmd/mcp-bridge/serve_upstream_restart_test.go`
  - `coverage.out`
- Prohibited scope: auth middleware, config schema, storage schema, internal/service, internal/upstream.
- RED/GREEN sequence:
  - RED: Add integration test `TestUpstreamRestartSurvivesInDefaultStatelessMode` in `cmd/mcp-bridge/serve_upstream_restart_test.go`; observe failure (`backend_unavailable` on the second call after restart because `Dedicated` retained the dead session from generation `"before"`).
  - GREEN: Update `cmd/mcp-bridge/wiring.go` to pass `Stateless: server.ServerStateless()`; observe test passing with generation `"after"`.
- Full verification command:
  `go test -race -p 1 -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./... && go run ./cmd/covcheck`
<!-- specdd:envelope:end -->

<!-- specdd:observability:start -->
### 5. Observability Assessment
Existing logging and metrics are strictly preserved and bound to T1/AC4 verification:
- Observability verification binding:
  - Bound to T1/AC4: Retained metric emissions are verified by existing regression tests (`cmd/mcp-bridge/serve_logging_compatibility_test.go:211-330` asserting Prometheus exposition of `bridge_requests_total` on `http://<operations.listen>/metrics`).
  - Handshake event schema & delivery: In `cmd/mcp-bridge/serve_upstream_restart_test.go`, capture process `os.Stdout` (using `os.Pipe()` with `t.Cleanup` restoration) during legacy peer connection, assert that `upstream_ephemeral_handshake` is emitted as valid JSON, and decode it into a struct verifying `level == "info"`, `event == "upstream_ephemeral_handshake"`, and matching `backend_id`.
  - Inbound operations metrics are exposed on the HTTP operations listener (`http://<operations.listen>/metrics`) serving Prometheus format.
- Handshake log schema & delivery details:
  - Emitted schema: `internal/upstream/dedicated.go:247-254` emits `{"level": "info", "event": "upstream_ephemeral_handshake", "backend_id": string, "duration_ms": int64, "protocol_version": string}` directly to the configured writer, which defaults to `os.Stdout` (`internal/upstream/dedicated.go:52`).
  - Missing fields justification: The event intentionally omits `timestamp`, `service`, `environment`, and caller `request_id` correlation fields because it is emitted deep within the transport connection initialization of `upstream.Dedicated`. Reason: The emitter remains transport-local and does not receive caller correlation metadata. Consequence: Handshake logs are correlated by `backend_id` and sequence in the process output stream rather than caller distributed trace IDs or wall-clock timestamps.
- Justified exceptions:
  - Metric scrapers, dashboards, and alerting rules: Excluded from this bugfix scope because no new metric series or label sets are created (`bridge_requests_total` is existing). Reason: Scope is strictly limited to wiring the existing stateless mode. Consequence: Operators continue to monitor bridge throughput and error rates using existing alerts on `bridge_requests_total`; no new dashboards or alert tuning are needed.
  - Remote log shipping and retention: Unchanged and unverified in local tests. Reason: Verification of external log shipping daemons (Vector/Loki) requires live cluster infrastructure not present in the local test environment. Consequence: Tests verify that valid JSON is emitted to `os.Stdout`; downstream log aggregation and retention are covered by deployment-level infrastructure monitoring.
<!-- specdd:observability:end -->

<!-- specdd:rollback:start -->
### 6. Rollback Procedure
If regressions occur:
1. In-flight / uncommitted rollback:
   - Reverse only the single line in `cmd/mcp-bridge/wiring.go` (`Stateless: server.ServerStateless()`), preserving any unrelated edits:
     `git diff cmd/mcp-bridge/wiring.go` to verify the single hunk, then discard it.
   - Delete `cmd/mcp-bridge/serve_upstream_restart_test.go` if created by this task (`rm -f cmd/mcp-bridge/serve_upstream_restart_test.go`).
2. Committed rollback:
   - Revert the isolated fix commit via `git revert --no-edit <commit-sha>`.
3. Verification:
   - Run `go test -race -p 1 -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./... && go run ./cmd/covcheck` to verify clean restoration.
4. Consequence:
   - Rolling back restores the restart vulnerability where restarted upstream backend servers cause subsequent calls to fail with `backend_unavailable`.
<!-- specdd:rollback:end -->
