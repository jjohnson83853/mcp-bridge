# PR-T2 test observations

Feature: `progressive-metadata-reveal`  
Revision: `cursor-choice-reconciliation-7-session-admission`  
Task: `PR-T2`  
Branch: `approved-progressive-metadata-ledger`  
Packet digest: `aa2052a78b1bb904a4d9df500a86c48efa309dfc64e37d05a4b5fc2d5c9af96c`

This records PR-T2's contribution to PR-AC6/8/9 only. It does not claim those broad acceptance criteria are complete; integrated lifecycle, direct-call, cursor, and HTTP/MCP evidence remains with later tasks.

## Coordinator-run focused verification

All commands exited 0:

- `go test ./internal/service -run '^TestProgressiveMetadata(ProjectionPublication|UnionReferences)$' -count=1` — PASS.
- `go test ./cmd/mcp-bridge -run '^TestRefreshPublicationFailureClassificationThroughLoop$' -count=1` — PASS.
- `go test -race ./internal/service -run '^TestProgressiveMetadata(ProjectionPublication|UnionReferences)$' -count=1` — PASS.
- `go test -race ./cmd/mcp-bridge -run '^TestRefreshPublicationFailureClassificationThroughLoop$' -count=1` — PASS.

## Descriptor-comparison benchmark

Command: `go test ./internal/service -run '^$' -bench '^BenchmarkPinnedToolCallDescriptorComparison$' -benchmem -count=5` — PASS. No timing threshold was applied.

Five observed equal-descriptor runs: 224,348–232,828 ns/op; 88,904–88,912 B/op; 1,306 allocs/op. Five changed-descriptor runs: 1,371–1,398 ns/op; 528 B/op; 6 allocs/op.

The implementer also reported deterministic RED for `go test ./cmd/mcp-bridge -run '^TestRefreshPublicationFailureClassificationThroughLoop$' -count=1` (observed `invalid_config`, expected `internal_error`) followed by GREEN on the unchanged test. The coordinator independently observed the post-fix pass; the implementer's RED output was reported, not independently reproduced.

Observed by coordinator at `2026-10-30T20:44:19Z`.