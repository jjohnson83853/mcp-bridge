# Ticket 01 spec — review round 10 findings ledger

Candidate revision r10 (`ticket01-bounded-results-v10`), verdict REQUEST CHANGES (bound).
Observation: `.omp/artifacts/token-eff01-review/observation-code-reviewer-r10-c662d64d51ba.json`.
Review budget (owner allocation 6): **5 of 6 used**, 1 remaining. A further REVISE stops the branch.

## Verified adequate (do not regress)

Whole-`CallToolResult` marshal accounting with per-occurrence duplicates and one metric for `total_bytes`, ceiling, preview and page fill; 8192/8193 and ≤1536 are the same form; termination with preview prefix, pinned page ceiling, no recursive paging; 32-byte crypto handles bound to `{kind,sessionID,epoch}` with one `invalid_handle` and two-phase validation; `nextEpoch` mutex-guarded so re-minted IDs cannot reach old epochs; per-namespace hard bounds, FIFO, own-only advisory eviction, atomic creation, immutable reads, tombstones; `isError`/`upstream_code` on preview and pages; small `errorResult`; `additionalProperties:false`; seam after `commitDiscovery`; inventory; T1–T7 ledger.

## Open findings

| ID | Class | Sev | Anchor | Defect |
| --- | --- | --- | --- | --- |
| F1 | statement-contradiction | HIGH | §4.5, §9, AC9/AC13 | The per-entry bound assumes one copy and no escaping, but `call_tool` emits the upstream content twice (`tools.go:869` and `:903-908`) and re-marshaling can escape up to 6×; and `max_response_bytes` is read (`command.go:109`) but enforced nowhere on upstream reads (`internal/upstream/client.go` installs no body limit). A maximal `call_tool` entry measures >16 MiB, so the hard 16 MiB per-session bound cannot admit it and R1 has no capacity terminal. |
| F2 | statement-contradiction | MEDIUM | R1, R3, R4, AC4 | R1's universal over-the-ceiling claim is unscoped while pages use the pinned ceiling, so a lowered ceiling makes a pinned page exceed the configured one and R1 would demand re-paging, contradicting R4. |
| F3 | ac-not-falsifiable | MEDIUM | AC13, T2/T4/T5 | Two of R1's three terminal branches (entropy failure, envelope infeasibility) have no inducible oracle and no owning seam. |
| F4 | ac-not-falsifiable | MEDIUM | R7, AC14, T2 | `crypto-random` is not falsified by pairwise distinctness, which a counter satisfies. |
| F5 | ledger-state-drift | LOW | §11 OD2-A | The spec adopts 16 MiB and drops `max_retained_result_bytes` as "OD2-A as amended" without a recorded amendment. The coordinator has now recorded the technical amendment in `decisions.md`; the spec must cite it. |

## Recording

`decisions.md` now holds the OD2 amendment (technical, S06): 16 MiB per session, `max_retained_result_bytes` removed, aggregate advisory. F5 is therefore a citation fix.

## Direction for the last repair

F1 requires one of two honest positions, and the spec must pick one with derivation:

- **Enforce the premise:** add the upstream response-body limit to the ticket's effects (`internal/upstream/client.go`), then derive the maximal entry from the enforced cap × duplicated forms × maximum escape expansion, and require the per-session bound to admit it.
- **Bound the guarantee:** state that the delivery ceiling is a *delivery* bound and that retention capacity is a typed terminal (`result_capacity_exceeded`) with the exact condition, keeping memory bounded by construction.

Either way, the entry metric must be named once and used by every bound, AC and config validation.
