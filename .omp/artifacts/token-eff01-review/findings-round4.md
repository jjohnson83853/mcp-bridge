# Ticket 01 spec — review round 4 findings ledger

Candidate revision r4 (`ticket01-bounded-results-v4`), verdict REQUEST CHANGES (bound).
Observation: `.omp/artifacts/token-eff01-review/observation-code-reviewer-r4-ad0c98a3dbde.json`.
Review budget (S15): 4 of 5 used. **One review unit remains** for this planning task.

## Verified closures (do not regress)

Round-3 H2, H3, H4, H5, H6, L1–L4; earlier F1, F2, F8, F12, F15. Byte accounting, handle namespace, storage bounds, error pass-through, compatibility and seam placement are all recorded adequate.

## Open findings

| ID | Class | Sev | Anchor | Defect |
| --- | --- | --- | --- | --- |
| R1 | ledger-state-drift | HIGH | §3 inventory, T3, §7 | The "complete" inventory still misses three exact-set/count assertions: `internal/service/tool_metadata_test.go:108-120` (`len(got)!=3`), `internal/service/tools_publish_atomic_test.go:50-57` (`len(got)!=4`), `cmd/mcp-bridge/serve_test.go:618` (`len(listed.Tools)!=3`). T3's mandated regression therefore fails in files it may not edit. |
| R2 | statement-contradiction | HIGH | §4.2, §4.3 partition, §4.4 | The upstream top-level `code` is copied verbatim into preview and page metadata while maximal-fill assumes metadata varying only by digits. A long code breaks the 1536 preview or the 8192 page, with no truncation or termination rule. |
| R3 | statement-contradiction | MEDIUM | §4.3, AC15 | AC15 demands rejecting an out-of-range page "before any store access", but `total_pages` needs the owner-namespaced lookup, and `page` is optional with default 1. Split pre-store shape checks from post-lookup range checks. |
| R4 | ledger-state-drift | MEDIUM | §8, T2/T4/T5 | The four metrics need `internal/observe/metrics.go` and the new log events need `docs/logging-contract.md`; neither is a write root or expected file, while §8 ties metric scraping to AC9/T2. |
| R5 | ac-not-falsifiable | MEDIUM | R6, AC6 | R6 requires preview/page shapes in the tool description, but AC6 checks only the stored input schema and README, and no task owns description text. |
| R6 | statement-contradiction | MEDIUM | §4.2, AC13/AC16 | The unretainable branch specifies no `Content` prefix and never states that MCP `isError` mirrors the original, so AC13 passes with a metadata-only preview and an unretainable error can ship with `isError` false. |
| R7 | statement-contradiction | LOW | §4.5, T2 | The invalidation list omits SDK-session-loss retirement (`internal/service/inbound.go:800-816` retires the owner; `:850`/`:921` mint a new epoch) while T2 names only reaper purge. |
| R8 | statement-contradiction | LOW | §6 T6 | T6 negative greps forbid phrases §6 requires documenting as still true at or below the ceiling, so T6 blocks truthful qualified wording. |

## Coordinator-verified built-in-surface inventory (authoritative for R1)

Count/set assertions that a fourth built-in breaks:

| File | Assertion |
| --- | --- |
| `internal/service/tools_test.go:512-548` | exact built-in descriptor set |
| `internal/service/progressive_session_test.go:180-183` | wanted tool names include the three built-ins |
| `internal/service/progressive_concurrency_lifecycle_test.go:697-731` | exact surface in lifecycle schedule |
| `internal/service/tool_metadata_test.go:108-120` | `want` map of three names, `len(got)!=3` |
| `internal/service/tools_publish_atomic_test.go:50-57` | `want` map of three names plus `alpha__good`, `len(got)!=4` |
| `cmd/mcp-bridge/serve_hybrid_exposure_test.go:201-204` | `tools/list length == 4`, three-name meta list |
| `cmd/mcp-bridge/serve_hybrid_reload_test.go:217,226,238,262` | 4/5/4 across reload phases |
| `cmd/mcp-bridge/serve_hybrid_progressive_reveal_test.go:352-390` | exact name sets |
| `cmd/mcp-bridge/serve_hybrid_progressive_reveal_lifecycle_test.go:278-286,492` | exact name sets |
| `cmd/mcp-bridge/serve_oauth_test.go:449,500` | `exposed_tools == 3` |
| `cmd/mcp-bridge/serve_test.go:618` | `len(listed.Tools) != 3` |

Presence-only assertions (`serve_hybrid_metadata_test.go:143`, `serve_oauth_test.go:1734`) are unaffected and need no change.

## Process constraint

This is the last authorized review unit. The r5 revision must close all eight findings; a further REVISE exhausts the S15 budget for this planning task and stops the branch for an owner decision.
