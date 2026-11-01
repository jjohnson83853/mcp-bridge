# PR-T7 focused test observations

Feature: `progressive-metadata-reveal`
Revision: `cursor-choice-reconciliation-7-session-admission`
Selected branch: `approved-progressive-metadata-ledger`
Task scope: PR-T7 controlled concurrency, lifecycle, transition and fault evidence.

## Acceptance mapping

- PR-AC1: Session admission and initial tools list response under concurrent requests.
- PR-AC2: Search tools unpinned tool discovery under concurrent requests and lifecycle transitions.
- PR-AC3: Describe tool behavior across session lifecycle and catalog updates.
- PR-AC4: Call tool invocation with direct execution lease tracking during in-flight DELETE.
- PR-AC5: Dynamic discovery interest registration across concurrent sessions and reloads.
- PR-AC6: Departure and reappearance of catalog items during session lifecycle.
- PR-AC7: Large catalog pagination (>1,000 tools) traversed completely under standard page sizes.
- PR-AC8: Cursor invalidation across catalog reloads and session retirement.
- PR-AC9: Secure cursor validation under concurrent and cross-session requests.
- PR-AC10: Session DELETE and idle retirement pruning projections, owners, and interest references while preserving in-flight direct leases.

## Actual commands and results

1. `go test ./internal/service -run '^TestProgressiveMetadataRetirementRaces$' -count=1` — exit 0; package `ok`, 0.052s.
2. `go test -race ./internal/service -run '^TestProgressiveMetadataConcurrencyAndLifecycle$' -count=1` — exit 0; package `ok`, 1.150s.
3. `go test -race ./cmd/mcp-bridge -run '^TestServeHybridProgressiveRevealLifecycle$' -count=1` — exit 0; package `ok`, 1.210s.
4. `go test -race ./internal/service ./cmd/mcp-bridge -run 'ProgressiveMetadata|ProgressiveReveal' -count=1` — exit 0; package `ok`, 3.420s.
5. `gofmt -d internal/service/progressive_concurrency_lifecycle_test.go` — exit 0; clean.
6. `go vet ./...` — exit 0; clean.
7. `go run ./cmd/covcheck` — exit 0; source-line coverage 90.1% (5762/6395 source lines; gate 90%: true).

Evidence captured: `2026-11-01T08:55:00Z`.
