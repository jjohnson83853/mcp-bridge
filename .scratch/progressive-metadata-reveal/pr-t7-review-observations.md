# PR-T7 final review observations (ReviewPRT7Final)

Feature: `progressive-metadata-reveal`
Revision: `cursor-choice-reconciliation-7-session-admission`
Task scope: PR-T7 scoped code re-review following Option 1 architectural resolution.

## Review Result
- Reviewer: `code-reviewer` (`ReviewPRT7Final`)
- Verdict: `COMMENT` (0 CRITICAL, 0 HIGH, 2 MEDIUM, 0 LOW)
- Findings:
  1. `[MEDIUM] Lease-survival assertion runs before DELETE necessarily closes the session`: `deleteEntered` signaled before SDK delegation; test can pass without proving lease survives DELETE while in flight.
  2. `[MEDIUM] Common start gate does not deterministically exercise concurrent first enrollment`: Releasing goroutines with `close(start)` does not force overlap at enrollment.
- Positive observations:
  - Post-stamp case uses production HTTP DELETE, checks owner pruning, and asserts exact stale-carrier JSON-RPC error (`progressive_concurrency_lifecycle_test.go:169-263`).
  - DELETE response handling correctly accepts transport termination under supplied owner decision.
  - Lazy-retirement coverage uses controlled, mutex-protected clock rather than sleeps.
  - Inbound handling authenticates before stamping, strips exposure-header copies, no auth/policy bypass.
- Verification commands executed and passing:
  - `go test ./internal/service -run '^TestProgressiveMetadataRetirementRaces$' -count=1` -> PASS
  - `go test -race ./internal/service -run '^TestProgressiveMetadataConcurrencyAndLifecycle$' -count=1` -> PASS
  - `go test -race ./cmd/mcp-bridge -run '^TestServeHybridProgressiveRevealLifecycle$' -count=1` -> PASS
  - `go test -race ./internal/service ./cmd/mcp-bridge -run 'ProgressiveMetadata|ProgressiveReveal' -count=1` -> PASS
  - `gofmt -d internal/service/progressive_concurrency_lifecycle_test.go` -> clean
  - `go vet ./...` -> clean
  - `go run ./cmd/covcheck` -> source-line coverage 90.1% (5762/6395), gate 90%: true
