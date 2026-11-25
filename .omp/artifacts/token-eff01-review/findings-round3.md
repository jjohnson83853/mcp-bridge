# Ticket 01 spec — review round 3 findings ledger

Candidate revision r3 (`ticket01-bounded-results-v3`), verdict REQUEST CHANGES (bound).
Observation: `.omp/artifacts/token-eff01-review/observation-code-reviewer-r3-bdd6943aef9d.json`.
Review budget (S15): 3 of 5 used; 2 remaining.

## Verified closures (do not regress)

Round-2 findings R-01, R-02, R-03, R-04 (ownership), R-06, R-07, R-08, R-09; round-1 F1, F2, F4, F6, F8, F9, F10, F12, F14.

## Open findings

| ID | Class | Sev | Anchor | Defect |
| --- | --- | --- | --- | --- |
| H1 | ledger-state-drift | HIGH | §6 write roots; `cmd/mcp-bridge` | Adding `retrieve_result` breaks exact built-in-surface assertions that are neither write roots nor owned by a task: `serve_hybrid_exposure_test.go:201` (want 4); `serve_hybrid_reload_test.go:225,238,262` (4/5/4); `serve_hybrid_progressive_reveal_lifecycle_test.go:278-286,492`; `serve_hybrid_progressive_reveal_test.go:352-390` (exact name sets); `serve_oauth_test.go:449,500` (`exposed_tools==3`). §3 lists only three internal tests. Every mandatory `go test ./cmd/mcp-bridge/` regression then fails. |
| H2 | statement-contradiction | MEDIUM | §4.3 step 1; AC17 | `admitExposureRequest` (`session_exposure.go:105-120`) admits session-less requests to the publisher projection, so the observable for session-less retrieval is `invalid_handle` in that namespace, not rejection. AC17/`TestRetrieveResultRequiresExposure` cannot pass as written. |
| H3 | ac-not-falsifiable | MEDIUM | §6 T6 | The changed-claim list omits `README.md:288` ("All three declare…"), `:338` ("search_tools includes the revision…"), and `:443` ("exactly the three bridge built-ins"). T6's greps pass while those documented claims remain false. |
| H4 | statement-contradiction | MEDIUM | §2 R1/R3 vs §4.5 | R1/R3 assert absolute 1536/8192 while §4.5 makes both ceilings configurable; state that the preview and page bounds *are* the configured ceilings (defaults 1536/8192). |
| H5 | r-ac-coverage-gap | MEDIUM | §5 oracles, T1–T7 | Oracles for AC1, AC2, AC3, AC4, AC5, AC7, AC8, AC9, AC10, AC11, AC14 appear in no task `-run`. AC2 is owned by T1, which may not wire handlers yet asserts delivery behavior. |
| H6 | statement-contradiction | MEDIUM | §4.3 AC7 | "Zero content" is false: `errorResult` (`tools.go:1037-1048`) always emits one TextContent and is the reused shape. Require zero retained-result and zero page bytes. |
| L1 | ac-not-falsifiable | LOW | §5 AC3 | `remaining_pages`/`remaining_bytes` arithmetic is undefined; "internally consistent" admits any convention. |
| L2 | ac-not-falsifiable | LOW | §5 AC6 | Preview/page shapes cannot appear in a stored input schema; AC6 does not test that clause. |
| L3 | statement-contradiction | LOW | §4.5 config validation | `handle_ttl` is omitted from config validation; zero or negative is accepted and instantly expires every handle. |
| L4 | ledger-state-drift | LOW | §6 T7 | T7's validation writes `coverage.out` while T7 prohibits any write, and no write root covers it. |

## Adequacy recorded (keep)

K1 SDK `CallToolResult.MarshalJSON` form with per-occurrence duplicate counting; K2 no recursive paging; K3 CSPRNG handles with epoch namespace and one rejection shape; K4 FIFO with creating-session-only eviction and atomic admission; K5 dispatch failures stay small; K6 fourth tool with `additionalProperties:false` and no `full`; K7 seam after `commitDiscovery` apart from H1/H4; K8 UTF-8-safe slices with byte-exact concatenation.

## Review process note

The reviewer reported that the batch packet named superseded v1 bytes/digest. The coordinator has since made the reviewer context revision-agnostic (review the on-disk bytes; the file declares its own revision). Task text for round 4 is regenerated from that context.
