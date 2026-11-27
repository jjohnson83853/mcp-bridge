# Ticket 01 spec — review round 7 findings ledger

Candidate revision r7 (`ticket01-bounded-results-v7`), verdict REQUEST CHANGES (bound).
Observation: `.omp/artifacts/token-eff01-review/observation-code-reviewer-r7-*.json`.
Review budget (owner allocation 6): **2 of 6 used**, 4 remaining.

## Owner resolution

H1 is resolved by owner decision: **total retention** (decision log `.scratch/token-efficient-tool-responses/decisions.md`). Per-session bounds stay hard; the aggregate cap becomes soft; `max_retained_result_bytes` is derived from the maximum serialized transport result; session-less calls retain in the publisher namespace.

## Verified adequate (do not regress)

K1 byte accounting with the complete serialized envelope plus escape reserve; K2 termination and pinned-ceiling pages; K3 crypto handles with epoch binding and one rejection shape; K4 per-session bounds, FIFO, creating-session-only eviction, atomic reserve, serialized reads; K5 error class across preview and pages; K6 `additionalProperties:false` with shape-before-store and range-after-lookup; K7 seam after `commitDiscovery` and the built-in inventory; ledger dependencies and oracles; docs claim set. Round-6 closures F1, F2, F3, F4, F6, F7, F8, F9 verified.

## Open findings

| ID | Class | Sev | Anchor | Defect |
| --- | --- | --- | --- | --- |
| H1 | ac-weakened | HIGH | R1, §4.0, §4.2, AC13 | The unretainable branch returns no handle and `paging_required:false`, so an over-ceiling result becomes unrecoverable; intake O1 requires preview+handle for every over-ceiling result and no owner decision ratified the exception. An over-ceiling mutating call loses its only complete result. |
| H2 | lifecycle-hazard | HIGH | §4.5, AC8, AC9 | `result_evicted`/`result_expired` must be returned after removal, but eviction deletes the entry and the sweep purges it; no bounded tombstone is defined, so the code returned depends on sweep timing and any unbounded tombstone would breach R9. |
| H3 | ledger-state-drift | MEDIUM | §4.5, §7, T2 | The sweep stops through a `Close()`/shutdown hook that does not exist: `ToolSurface` has no `Close`, and `serve.go` only closes listeners and drains the runtime; `cleanupRetiredOwner` lives in `projection.go` while the only lifecycle write root is `inbound.go`. |
| H4 | ac-not-falsifiable | MEDIUM | AC9, §8, AC18 | §8 assigns `result_retention_rejected` and `result_evicted_due_to_bound` to AC9, but AC9 asserts no log event and AC18 only checks that one event marshals. |
| H5 | statement-contradiction | LOW | R10 | R10 calls the source "the upstream's own top-level structured `upstream_code`", while §4.2/AC10 define it as upstream `structuredContent.code` copied into `upstream_code`. |

## Low-confidence questions raised (answer in the repair)

- Retired-epoch callers: exposure-admission rejection (which JSON-RPC error) versus `invalid_handle`; R7/AC7 must agree.
- Metrics ownership: per-surface `*observe.Metrics` injected through `ToolSurfaceConfig` versus package-level collectors, with the multi-instance/test consequence stated.
- Crypto/rand failure during handle generation: map to a typed failure mode.
