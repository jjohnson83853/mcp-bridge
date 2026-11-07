# 02 - Configuration Reload Consistency, Regression, and Coverage Gate

Status: ready-for-agent
Blocked-by: 01

## What to build
Verify that dynamic configuration reloads update the advertised pinned inventory immediately, and that zero-pin baselines and full regression suites pass cleanly with coverage:
1. When configuration changes `pins: [...]` at runtime and reloads, `search_tools.Description` updates to match the newly published assembly without server restart or schema corruption.
2. Full race detector suite passes cleanly: `go test -race ./...`.
3. Instrumented statement coverage satisfies the project gate: `go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./... && go run ./cmd/covcheck` exits 0 with $\ge 90\%$ statement coverage.

## Acceptance Criteria
- [ ] AC-6: Configuration reload updating `pins:` immediately reflects the new inventory in `search_tools.Description` on the wire.
- [ ] AC-7: Full test suite passes under race detector (`go test -race ./...`).
- [ ] AC-8: Statement coverage meets or exceeds the 90% threshold (`go run ./cmd/covcheck`).
