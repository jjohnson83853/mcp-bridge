# Ticket 02 — planning and review ledger (#116)

Feature: `token-efficient-tool-responses` ticket 02, compact `search_tools` index.
Canonical execution state for this ticket: not yet installed (planning candidate phase).

## Authority and inputs

- Owner selection: current session, "lets focus on ticket 116" (2026-11-27). This selects the ticket for planning; implementation still requires milestone authorization.
- Owner review budget: **10 completed CodeReviewer reviews for this ticket**, from the recorded owner ruling in `.scratch/token-efficient-tool-responses/decisions.md` (2026-11-27): "each ticket should start with a budget of 10" (S15 owner-set positive integer).
- Authoritative intake: GitHub `jjohnson83853/mcp-bridge#116`, body sha256 `ffdf8986f121d53af965d963b9510e7170521cf21f8814c9bfda33bdee27ade7`, updated 2026-11-26T14:32:22Z; AC1–AC8 are the eight issue checkboxes.
- Refinement: `.omp/artifacts/token-eff02/refinement.md` (READY, second pass).
- Ticket 01 status observed: delivered commit `522a1e5d68b279a7b185a8cbb9bb13a45662596e` (`verify-delivery` exit 0, clean); canonical completion reports `E_MARKER_INVALID` from the stale `.superpowers/sdd/GATE_OPEN` snapshot-recovery marker. Pre-existing bookkeeping failure, out of ticket-02 scope; not repaired here.

## Review budget accounting

| Unit | Stage | Invocation | Candidate/revision | Outcome |
| --- | --- | --- | --- | --- |
| — | initial refinement | `RefineTicket116` | issue body digest above | BLOCKED (five gaps) |
| — | refinement re-assessment | `RefineTicket116b` | same digest + supplied evidence | READY |
| — | planner | `PlanTicket116` | r1 pending | pending |

Consumed CodeReviewer reviews: **0 / 10**. Critic consumes no budget unit.
Prior lineage: none (new ticket). No budget reset; count is cumulative across sessions.

## Findings / decisions

- 2026-11-27: ticket-03 session-exposure changes excluded; search discovery identity validation stays internal.
- 2026-11-27: operator config compatibility preserved unless AC3 requires otherwise; planner to state exact mechanism.

## Review round 1 (2026-11-27)

- CodeReviewer SpecReviewCompactIndex (invocation ff14a51a-c826-4c6f-a00d-2ea143d939d1) on candidate token-efficient-tool-responses-ticket02-spec@r1: REQUEST CHANGES, recorded as review round seq 82.
- Findings: F1 HIGH (ac-not-falsifiable: multiline description not normalized, false ingestion-validation claim); F2 MEDIUM (statement-contradiction: byte arithmetic omits second-order text escaping); F3 MEDIUM (r-ac-coverage-gap: T3 fixture inventory incomplete); F4-F6 LOW (advertised default>maximum; AC2 edge cases; rollback doc scope).
- Owner directive "add 10 to your budge": ticket-02 budget increased from 10 to 20; no other scope change.
- Consumed CodeReviewer reviews: 1 / 20. Next: planner repair r2 addressing F1-F6 at the root invariant, then fresh candidate + CodeReviewer round 2.

## Owner narrowing directive (2026-11-27)

Owner: "widening means add unnecessary work to the plan ... I would rather have the agent file tickets either with the harness or the repo for things that aren't really necessary to deploy the ticket."

- Rule for ticket 02: resolve review findings by narrowing claims or deferring out-of-scope hardening; do not add new product behavior, acceptance criteria or documentation beyond what the ticket requires to deploy.
- Deferred finding F4 (degenerate config default_top_k > max_top_k) to repo issue #165. r3 removes the F4 behavior clamp and scopes R2 to valid operator configs.
- F6 rollback-tolerance documentation dropped from r3 (not required by the ticket).
- r2 spec was authored and its state installed but no review was launched; r2 is superseded by r3 before any review budget is consumed. Budget remains 1/20.

## Review round 2 (2026-11-27)

- CodeReviewer SpecReviewCompactIndexR3 (invocation 4a4e3336-5c0e-4d68-9108-df20a9ed03f3) on candidate token-efficient-tool-responses-ticket02-spec-r3: REQUEST CHANGES.
- Findings: N1 HIGH (fast path bypasses encoded-length invariant); N2/N4/N5 MEDIUM (degenerate-config claim false; advertised-vs-enforced divergence for zero-bounds surfaces; arithmetic undercount); N3 MEDIUM (F3 inventory not exact); N6/N7 LOW (U+2028/29; citations).
- r2 was authored/installed but never reviewed; r3 superseded it. Review budget consumed: 2 / 20.
- Next: planner r4 minimal corrections; no new scope per owner narrowing directive.

## Review round 3 + Critic (2026-11-27)

- CodeReviewer SpecReviewCompactIndexR4 (invocation df2aa5fc-d316-4c75-a0b5-ca62db52d807): APPROVE; 1 MEDIUM (CR4-1 missing serve_endpoint_rebind_test.go rewrite) + 2 LOW citation findings. Bound as review round 3.
- Critic CriticCompactIndexR4: REVISE — C1 MAJOR (same fixture gap), C2 MAJOR (§7 search logging/metrics claims unsupported), C3 MAJOR (§9.2 reload oracle impossible; search settings restart-only), C4 MINOR (nil cfg impossible; use zero-value cfg).
- Critic verdict kept as evidence; no critic observation bound because the run store has no critic invocation record for r4 and r4 is superseded. Does not consume CodeReviewer budget.
- Consumed CodeReviewer reviews: 3 / 20. Next: planner r5 minimal corrections C1–C4, then fresh CR + Critic on r5.

## Review round 4 + Critic second pass (2026-11-27)

- CodeReviewer SpecReviewCompactIndexR5 (invocation 3acaf4bc-fab9-4220-bb6d-ec2c4b57b72c): APPROVE; only CR5-1 LOW citation drift. Bound as review round 4.
- Critic first pass: BLOCKED solely on inability to recompute SHA-256 (no hashing tool). Re-dispatched with corrected evidence boundary (harness admission validates digests; judge content).
- Critic second pass: REVISE — C2 MAJOR (§7 still claims log tests verify delivery path), C5 MAJOR (AC5/T4 36-byte truncated all-quote fixture unreachable: 18 quotes = 36 encoded unmarked; longer truncates to 16 quotes + ellipsis = 35).
- Budget: 4 / 20 CodeReviewer reviews. Next: planner r6 minimal fixes, fresh CR + Critic.
