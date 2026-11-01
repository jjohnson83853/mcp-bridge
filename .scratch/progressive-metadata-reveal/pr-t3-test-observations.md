# PR-T3 focused test observations

Feature: `progressive-metadata-reveal`  
Task: `PR-T3`  
Baseline: `c44f56dc101de5a6cc4095ebd2a509dc6ae36451`  
Scope packet: `/tmp/mcp-bridge-PR-T3-review.md` (SHA-256 `b07a5be62856540403e159c7f69f999390b5efa5ce4c148af82ae5cd075334af`)

## Results

All commands exited 0.

| UTC | Command | Result |
|---|---|---|
| 2026-10-31T02:17:11Z | `go test ./internal/service -run '^TestProgressiveMetadataSession(Carrier|Enrollment)$' -count=1` | PASS; package `ok`, 0.016s |
| 2026-10-31T02:17:11Z | `go test ./cmd/mcp-bridge -run '^TestServeHybridProgressiveRevealSessionAdmission$' -count=1` | PASS; package `ok`, 0.036s |
| 2026-10-31T02:17:11Z | `go test -race ./internal/service -run '^TestProgressiveMetadataSession(Carrier|Enrollment)$' -count=1` | PASS; package `ok`, 1.060s |
| 2026-10-31T02:17:11Z | `go test -race ./cmd/mcp-bridge -run '^TestServeHybridProgressiveRevealSessionAdmission$' -count=1` | PASS; package `ok`, 1.090s |
| 2026-10-31T02:24:05Z | `go test ./cmd/mcp-bridge -run '^TestServeHybridDirectPreloadedToolExecution$' -count=1` | PASS; package `ok`, 0.041s |
| 2026-10-31T02:24:05Z | `go test -race ./cmd/mcp-bridge -run '^TestServeHybridDirectPreloadedToolExecution$' -count=1` | PASS; package `ok`, 1.096s |

## Bounded acceptance coverage

- PR-AC1: carrier/enrollment tests verify stable per-session epoch use and lazy baseline projection with current pins.
- PR-AC5: `TestServeHybridDirectPreloadedToolExecution` invokes an ordinary `tools/call` on an initialized session before metadata discovery.
- PR-AC7: carrier/admission tests cover forged carrier inputs, unauthorized and session-mismatch rejection, and unregistered-session rejection.
- PR-AC8: enrollment tests cover DELETE/expiry retirement and rejected delayed publication.
- PR-AC9: enrollment tests exercise 24 concurrent first relevant requests and are race-enabled; this does not replace the later full fault/barrier lifecycle matrix.
- PR-AC10: hybrid progressive-session integration test uses independent SDK sessions and validates their own current carriers.

This is PR-T3 contribution evidence only. It is not a full-suite run, project-wide coverage measurement, or proof that all aggregate acceptance criteria are complete.
