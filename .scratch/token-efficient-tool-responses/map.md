# token-efficient-tool-responses — map

**Intent:** Bound and de-duplicate the bytes the bridge sends to LLM clients, without hiding
execution facts or weakening session isolation.

**Source:** owner review of a session where bridge discovery and generic dispatch consumed
~44% of billed prompt tokens; five concrete observations plus one amplification found in review
(search silently commits matches into the session projection, so compact responses still trigger
schema re-sends on later turns).

## Tickets

| # | Ticket | Priority | Blocked by |
| --- | --- | --- | --- |
| 01 | Bounded tool results with recoverable paging | P0 | — |
| 02 | Compact `search_tools` index | P0 | — |
| 03 | Search must not fan out matched schemas into later prompts | P0 | — |
| 04 | Eliminate bridge-within-bridge discovery recursion | P1 | — |
| 05 | Default field projections for known list operations | P1 | — |
| 06 | Duplicate result reuse without suppressing execution | P2 | 01 |

## Decisions so far

- 01–03 are the first frontier: without a ceiling and compact, non-fan-out discovery the other
  three cannot materially reduce billed context.
- Deduplication is delivery-only. Executions are never elided, so mutating tools keep side effects.
- The ceiling measures serialized bytes written to the client, including duplicated
  content-block/structured forms.
- Handles and references are bridge-issued, session-scoped, expiring; no client transcript ids.
- Projection defaults are an explicit allowlist justified by stored schemas, never name heuristics.

- Ticket 01 spec has completed **five** independent CodeReviewer rounds (S15 budget exhausted). Rounds 1–4
  returned 16, 9, 10 and 8 findings; round 5 returned 2 substantive plus 3 ledger findings and verified the
  round-4 closures. Ledgers: `.omp/artifacts/token-eff01-review/findings-round{1,2,3,4}.md` and observation
  artifacts `observation-code-reviewer-r{1..5}-*.json`. Spec at r5 is not approved; no engineer dispatch.
- Spec now closes: byte accounting, handle lifecycle and isolation, store bounds, error pass-through,
  schema compatibility, exposure seam, complete built-in-surface inventory, and every AC→task oracle except
  the round-5 findings.
- Remaining blockers are design-level, not wording: preview-feasibility validation (M1), the upstream-code
  promise across the unretainable branch (M2), description observability (L1), the purge-seam model (L2),
  and observability deliverables with no AC (L3).
- Review budget (S15): 5 of 5 completed reviews used. The planning branch stops here for an owner decision.

## Current execution record

`.omp/plans/execution-state.json` (feature-bound; tracker lifecycle grants nothing).
