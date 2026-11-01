# PR-T4 focused test observations

Feature: `progressive-metadata-reveal`  
Revision: `cursor-choice-reconciliation-7-session-admission`  
Selected branch: `approved-progressive-metadata-ledger`  
Task scope: PR-T4 direct-execution slice only. Baseline anchor is HEAD `c44f56dc101de5a6cc4095ebd2a509dc6ae36451`; the scoped review bundle explicitly notes that this is not a clean pre-T4 snapshot. Review-scope artifact SHA-256: `703c2415f6a6e552abafee9fc8a4b52e205ddc8684e88af4882bdf4482e93a8f`.

## Acceptance mapping

- PR-AC6: `TestProgressiveMetadataCapturedDirectExecution` verifies the captured route, schema, arguments, result and lease behavior across SDK registration removal/runtime advancement. `TestProgressiveMetadataDirectLeaseOwnership` exercises lease cleanup and direct-execution error/cancellation paths.
- PR-AC7: the same focused tests cover captured direct binding behavior, mapped errors, validation and upstream outcomes, cancellation, and zero-dispatch rejection cases within PR-T4's scope.
- PR-AC9: the same focused tests cover stale/inconsistent/schema/draining rejection and captured execution/lease invariants within PR-T4's scope.

These results do not claim the full feature-wide PR-AC6/7/9 criteria; dynamic reveal, pagination, controlled HTTP/MCP integration and other task-owned cases remain for later tasks.

## Actual commands and results

1. `go test ./internal/service -run '^TestProgressiveMetadata(CapturedDirectExecution|DirectLeaseOwnership)$' -count=1` — exit 0; package `ok`, 0.044s.
2. `go test -race ./internal/service -run '^TestProgressiveMetadata(CapturedDirectExecution|DirectLeaseOwnership)$' -count=1` — exit 0; package `ok`, 1.129s.

Both results were recorded in the existing PR-T4 decisions log after the final test-helper correction. The original execution wall-clock timestamps were not retained; this artifact records the evidence-capture time `2026-10-31T18:22:15Z`.
