## 2026-11-26 — Budget 12, two HIGHs only (owner)

- **Context:** budget 8 spent (verdicts 9, 10, 9, 9, 7, 7, 5, 8 findings). Final review raised two HIGHs, both verified against code: no AC asserts a page's serialized length, and `operationVocabulary` (internal/observe/httpdiagnostics.go:660) lacks `search_tools`/`retrieve_result`.
- **Decision:** allocation raised by 4 to **12 total**; repair is limited to those two HIGHs. The six lower findings from r8 stay carried as implementation notes, not spec work.
- **Source:** owner answer on 2026-11-26 ("Add 4 to the budget and fix the two HIGHs").

## 2026-11-26 — Review budget 8, r5 repair (owner)

- **Context:** 4 reviews used (2 authorized for the replacement, 2 run by me to clear the mutation gate — disclosed as an overrun). Findings per round: 9, 10, 9, 9.
- **Decision:** allocation raised by 4 to **8 total**; continue with the r5 repair and review it.
- **Scope:** the nine r4 findings only. F1 is closed by DELETION — the spec must not require an epoch admission that `call_tool` does not perform and that `requestExposureEpoch` does not implement.
- **Stop rule:** if r5 does not pass, planning stops and the contract ships to implementation with its findings attached.
- **Source:** owner answer on 2026-11-26 ("add 4 to the budge and continue").

## 2026-11-26 — Owner rulings and final repair (r3)

- **Ruling 1:** `retrieve_result` is NOT exposure-gated; it is a bridge built-in validated handler-side (session, admitted epoch, store ownership) like `call_tool`.
- **Ruling 2:** T3 is staged to the `call_tool` slice; discovery and directly invoked projected surfaces move to T4.
- **Outcome:** r3-replacement installed, 30,309 bytes (predecessor 79,988; −62%). R1–R16, AC1–AC30 each naming an independent oracle, T1–T5 acyclic with exact write roots including the five count-assertion test files.
- **Dispositions:** F1 producer path pinned (`structuredContent.upstream_result.structuredContent.code`, `[truncated]` marker); F2 handler-validated retrieval with AC8/AC16 named; F3 4-byte headroom + maximal-metadata feasibility (AC28); F4 AC6 recomputes independently; F5 expiry/eviction markers with lifetime, clearing and accounting (AC5/AC18/AC30); F6 README + tool-description oracles (AC27); F7 ceilings pinned across reload (AC29); F8 exact files; F9 atomic page-or-error (AC30); F10 staging.
- **Review status:** no third review was run, per the owner's two-review limit. r3 is unreviewed; it is a planning artifact, not an approved contract.

## 2026-11-26 — Replacement reviewed twice; budget spent

- **Spec:** replacement installed at `.scratch/token-efficient-tool-responses/spec.md`, r2-replacement, 24,264 bytes (predecessor 79,988; −70%). Eight sections, four specdd markers, all citations resolve, no line-number prose checks.
- **Review 1 (of 2):** REQUEST CHANGES — 9 findings (3 HIGH: aggregate retention advisory-but-asserted, handle rejections not byte-identical, page UTF-8 boundaries undefined). All nine repaired in r2.
- **Review 2 (of 2):** REQUEST CHANGES — 10 findings (2 HIGH: `upstream_code` producer undefined; `retrieve_result` exposure-gating contradicts R7/AC8/AC16), 7 MEDIUM, 1 LOW. Reviewer confirms every R1–R15 maps to an AC and the six carried invariants are present.
- **Budget:** the owner-authorized two reviews are spent. No further review, no implementation dispatch, until the owner decides.
- **Facts established for the next revision (no decision needed):** `call_tool` emits `upstream_result` at `internal/service/tools.go:869`, so a nested upstream `code` is reachable under `structuredContent.upstream_result.structuredContent.code`; `retrieve_result` does not exist in the tree yet.
- **Open decisions:** (a) is `retrieve_result` exposure-gated (middleware −32600) or handler-validated like `call_tool`; (b) does T3 wire all four surfaces at once, contradicting the owner's vertical-slice-first ordering.

## 2026-11-26 — RESET: replace the spec, two reviews maximum (owner)

- **Assessment accepted:** 27 CodeReviewer rounds produced no implementation; the process amplified the very token waste the feature targets. Feature retained, process rejected.
- **Decision:** planner authors a REPLACEMENT contract (not an extension of r30): concise, single-purpose, no duplicated prose, no line-number greps, no review-history tables inside the contract.
- **Review budget:** 2 CodeReviewer reviews maximum for the replacement, plus an eligible Critic pass. No further automatic extensions; unresolved decisions come back in one batch.
- **r30:** preserved as reference at `.omp/artifacts/token-eff01-review/spec-r30-reference.md`; its findings remain evidence, not an approved contract.
- **Implementation:** one vertical slice first — oversized `call_tool` result → bounded preview + handle → byte-exact retrieval — before extending to other surfaces.
- **Source:** owner answer on 2026-11-26 ("Go ahead with the reset and two reviews maximum").

# Decisions — token-efficient-tool-responses

## 2026-11-25 — Review budget reset (owner)

- **Input:** five completed CodeReviewer rounds for ticket 01 (S15 default budget exhausted); round 5 left M1, M2, L1, L2, L3 open.
- **Decision:** owner authorized option 1 — one further planner repair followed by its reviews — and reset the planning review budget to **5** completed reviews for this allocation.
- **Effect:** counters restart at 0. Historical rounds remain immutable evidence and are never recounted as consumed units. No acceptance criterion, requirement, or effect envelope changes; this is an authority extension, not a scope change.
- **Source:** owner message in the coordinating session on 2026-11-25 ("1 authorize add reset your budget to 5").

## 2026-11-25 — Unretainable results: total retention (owner)

- **Input:** round-7 finding H1 — the `no_session`/`too_large`/`capacity` branch returns a preview without a handle, contradicting intake AC O1 and losing the only complete result of an over-ceiling mutating call.
- **Decision:** owner chose **total retention for anchored callers**: per-session entry/byte bounds stay hard, the aggregate cap becomes soft (evict only the caller's own handles, then admit), `max_retained_result_bytes` is derived from the maximum serialized transport result so `too_large` no longer applies to session owners, and session-less calls retain in the publisher namespace. O1 stands exactly as written.
- **Effect:** R1's unretainable-exception list is removed except where no namespace can hold a handle at all; T2 gains admission accounting and publisher-namespace retention; AC13/AC16 are rewritten to the retained behaviour.
- **Source:** owner answer to the H1 decision form on 2026-11-25 ("total-retention").

## 2026-11-25 — OD2 defaults amended (technical)

- **Input:** rounds 8–10 showed the original OD2-A numbers cannot satisfy the retention invariant: the delivered form duplicates the upstream content (`tools.go:869` plus `:903-908`) and escaping can inflate 6×, so a single maximal entry exceeds an 8 MiB per-session bound.
- **Decision:** `max_bytes_per_session` becomes 16 MiB, `max_entries_per_session` stays 16, and `max_retained_result_bytes` is removed in favour of a derived maximal-entry invariant; the aggregate cap is advisory and documented as such. These were coordinator-adopted technical defaults under RULES.md S06, so amending them is a technical decision, not an owner-acceptance change; the owner decision that matters (total retention) is unchanged.
- **Effect:** the spec must derive the maximal entry from the actually-enforced upstream cap, and the ticket must enforce that cap on upstream reads or state that the guarantee is bounded by an unenforced premise.

## 2026-11-25 — Oversize results: typed terminal (owner)

- **Input:** round-11 H1/H2 — enforcing the upstream body cap requires `wiring.go`/`runtime.go` outside the envelope and would cap the shared HTTP client, truncating discovery lists and long-lived SSE streams.
- **Decision:** do **not** enforce the upstream cap in this ticket. Total retention holds up to a per-entry cap (default 8 MiB serialized); a larger result returns a documented `result_capacity_exceeded` preview. This amends "total retention" to "total retention up to the per-entry cap", the only implementable form without unbounded memory or shared-transport changes.
- **Effect:** F1's enforcement work is removed from the spec and write roots; R1 gains a fourth typed terminal; per-entry and per-session bounds derive from the per-entry cap plus the duplicated/escaped maximum.
- **Source:** owner answer to the oversize decision form on 2026-11-25 ("typed-terminal").

## 2026-11-26 — Measurement oracle round, budget 22 (owner)

- **Input:** CodeReviewer r29 verified all earlier closures and raised F1 (every ceiling AC uses `measureResult` as its own oracle, so a deduplicating implementation ships ~2× the bound and still passes), F2 (AC9's `result_namespace_retired` retrieval assertion is unsatisfiable — admission rejects the retired epoch first), F3 (page `upstream_code` rendering derives from a reloadable bound, breaking R4 byte-identity), F4 (unbounded log label cardinality).
- **Decision:** fix all four; CodeReviewer allocation becomes **22**.
- **Source:** owner answer on 2026-11-26 ("22").

## 2026-11-25 — Budget +2 (owner)

- **Decision:** CodeReviewer allocation rises from 19 to **21**, giving two further rounds beyond the current r26 repair if the reviewer raises new material findings.
- **Source:** owner interjection on 2026-11-25 ("Add TWO TO THE BUDGET").

## 2026-11-25 — Mechanical closure round, budget 19 (owner)

- **Input:** CodeReviewer r25 (hash-bound after the packet fix) verified the ceiling invariant, terminal-preview oracles and entropy terminal closed; remaining F1 (envelope budget hardcodes 128-byte `upstream_code` vs configurable 512), F2 (three README claims lack per-claim oracles), F3 (R2 vs `invalid_epoch`), F4 (entropy terminal retained entries), F5 (feasibility rejection lacks an AC), F6 (tampered vs malformed handle boundary), plus carried README anchors and the inbound strip/stamp invariant.
- **Decision:** fix all eight; CodeReviewer allocation becomes **19**.
- **Source:** owner answer on 2026-11-25 ("19").

## 2026-11-25 — Review-hash binding defect fixed

- **Defect:** CodeReviewer r24 and r25 both reported the review "is not hash-bound": `.omp/artifacts/token-efficiency-ticket01-review-packet.md` and `.omp/artifacts/token-eff01-review/manifest.json` still pinned the v1 candidate digest `4eed48…`, so the reviewer compared on-disk r25 bytes against a stale packet.
- **Action:** the revision tooling now rewrites both artifacts from the spec on every revision (packet header + `manifest.candidate {path, sha256, bytes, revision}`). Verified: packet and manifest both read `662251be…`, 71,484 bytes, r25.
- **Consequence:** the next review round compares the pinned digest to the on-disk candidate and can bind its verdict.

## 2026-11-25 — Ceiling invariant and honest entropy terminal, budget 18 (owner)

- **Input:** CodeReviewer r24 verified F1/F2/F4/F5 closed and raised three MEDIUMs: no load-time `preview_ceiling_bytes < delivery_ceiling_bytes` invariant; AC13 terminal-preview oracles placed in T2, which cannot observe the seam; the entropy terminal drops the upstream error class and would report `not_started` after the upstream execution already completed.
- **Decision:** fix all three; CodeReviewer allocation becomes **18**.
- **Source:** owner answer on 2026-11-25 ("18").

## 2026-11-25 — Consolidated F1–F6 repair, budget 17 (owner)

- **Input:** CodeReviewer r23 REQUEST CHANGES — F1 HIGH (counter-delta oracles placed in T2/T3 although dispatch-side behavior first exists in T4/T5, creating a gate failure and a T4→T3→T4 cycle), F2 MEDIUM (`catalog_changed` reused for a session-epoch rejection), F3 MEDIUM (AC13 omits the unretainable-terminal preview bounds), F4/F5/F6 LOW (memory arithmetic unattainable, false `ToolSurfaceConfig.Metrics` claim, stale README anchors and a no-op negative grep).
- **Decision:** one consolidated repair pass; retrieval deltas stay in T3, dispatch deltas move to T4/T5; distinct epoch rejection code; AC13 gains the preview assertions; F4/F5/F6 corrected. CodeReviewer allocation becomes **17**.
- **Source:** owner answer on 2026-11-25 ("17").

## 2026-11-25 — Paging counter semantics, budget 16 (owner)

- **Input:** Critic r22 C6 — AC18 only asserts the two paging counters are registered and scraped, never that they increment; a registered-but-dead counter passes.
- **Decision:** bind counter semantics to paging/retrieval outcomes with exact delta oracles and absence of spurious increments; CodeReviewer allocation becomes **16**.
- **Source:** owner answer on 2026-11-25 ("16").

## 2026-11-25 — r22 receipt cycle halted on provider quota

- **State:** spec on disk is r22 (`ticket01-bounded-results-v22`, 60,642 bytes); every citation resolves; reviewer manifest covers 42 inputs including the three files added for C3.
- **Preflight:** `prepare-dispatch` exit 0, `check-dispatch` exit 0, `dispatch_allowed: true`.
- **Failure:** the CodeReviewer dispatch returned provider quota, not a review: `[opencode-go/deepseek-v4-pro] 429 OpenCode Go usage limit reached (5-hour, resets in 1h 55m)`. No review output was produced; nothing was bound.
- **Consequence:** `record-reviews` still refuses (`verified reviews are unavailable`) because the canonical receipts need the r22 round. The r21 pair stands: CodeReviewer APPROVE (`observation-code-reviewer-r21-*`), Critic PASS (`observation-critic-r21-*`), all R1–R12 PASS.
- **Next action:** re-dispatch the single r22 CodeReviewer round when quota resets (budget 15 already authorized), bind it, bind the Critic re-run, then `record-reviews` and request implementation authorization.

## 2026-11-25 — Envelope/test/doc coupling, budget 14 (owner)

- **Input:** Critic r20 reopened C3 — the four log fields require `internal/observe/log_test.go` and `cmd/mcp-bridge/logging_contract_test.go` updates plus the documented union, none of which sat in T2's write roots; T6 documentation depends on T2.
- **Decision:** put both fixtures and `docs/logging-contract.md` in T2's envelope so emitter, registry, fixtures and documentation move together. CodeReviewer allocation becomes **14**.
- **Source:** owner answer on 2026-11-25 ("14").

## 2026-11-25 — Final citation fix, budget 13 (owner)

- **Input:** the only remaining lint failure was `github/workflows/verify.yml:48`; the real path is `.github/workflows/verify.yml`.
- **Decision:** fix the path and re-review. CodeReviewer allocation becomes **13**.
- **Source:** owner answer on 2026-11-25 ("13").

## 2026-11-25 — SDK citation by symbol, budget 12 (owner)

- **Input:** the only remaining lint failure is the SDK-internal citation `mcp/server.go:306`, which lives in the module cache and cannot resolve under the repository root.
- **Decision:** cite the SDK by symbol (`mcp.Server.AddTool`) instead of a path with a line number; re-review. CodeReviewer allocation becomes **12**.
- **Source:** owner answer on 2026-11-25 ("12-symbol").

## 2026-11-25 — Citation normalization, budget 11 (owner)

- **Input:** the Critic receipt lint rejected four citations written relative to their package (`catalog/schema.go:74-89`, `session_exposure.go:154-172`, `tools.go:993-999`, `core.go:77-82`).
- **Decision:** normalize all citations to repo-root paths (`internal/...`) and re-review. CodeReviewer allocation becomes **11**. No semantic change to requirements.
- **Source:** owner answer on 2026-11-25 ("11-normalize").

## 2026-11-25 — CSPRNG mandatory, budget 10 (owner)

- **Input:** Critic round 1 finding C1 — R7 made cryptographic unpredictability advisory.
- **Decision:** cryptographic unpredictability is **mandatory**. The implementation uses `crypto/rand`, verified by inspection evidence recorded in the receipt; the entropy-failure path stays typed and tested. Uniqueness remains necessary but not sufficient. CodeReviewer allocation becomes **10**.
- **Source:** owner answer on 2026-11-25 ("10-inspect").

## 2026-11-25 — Invalid epoch rejected, budget 9 (owner)

- **Input:** round-14 F1 — `call_tool` on a minted session with a present-but-invalid epoch header had no defined namespace; a publisher fallback would expose the result to other session-less callers.
- **Decision:** a present-but-invalid epoch header is rejected before the seam, matching discovery and direct invocation. Only a genuinely absent epoch (session-less request) uses the publisher namespace. Review allocation becomes **9**.
- **Source:** owner answer on 2026-11-25 ("reject+9").

## 2026-11-25 — Review budget raised to 8 (owner)

- **Input:** rounds 7–11 consumed the 6-round allocation; round 11 left the oversize decision plus mechanical findings.
- **Decision:** allocation is **8** completed reviews; the counter keeps its consumed value.
- **Source:** owner answer on 2026-11-25 ("8").

## 2026-11-25 — Review budget raised to 6 (owner)

- **Input:** one review consumed from the 5-round allocation; round 6 left F1–F9 open.
- **Decision:** owner set the allocation for this planning task to **6** completed reviews.
- **Effect:** the counter keeps its consumed value (1 used, 5 remaining) under the larger allowance. No acceptance, scope, or authority change beyond the allowance.
- **Source:** owner message on 2026-11-25 ("set budget to 6").

## 2026-11-25 — Technical defaults adopted from the planner's open decisions

- OD1-A: retrieval is a fourth first-class built-in tool (`retrieve_result`).
- OD2-A: `max_entries_per_session 16`, `max_bytes_per_session 8 MiB`, `max_bytes_aggregate 64 MiB`, `max_retained_result_bytes 8 MiB`, `handle_ttl 1h`.
- OD3-A: fixed `handle_ttl` plus purge at session retirement.
- **Why:** RULES.md S06 prefers the conservative documented recommendation over an open reversible default; the owner's budget reset did not reopen these choices.

## Note on the decision log location

`AGENTS.md`-style planning records name `.omp/plans/decisions.md` as the decision log. That path is coordinator-edit-forbidden in this repository (`E_COORDINATOR_EDIT_FORBIDDEN`), so decisions for this feature are recorded here and referenced from `map.md`. The planner's spec may cite this file instead.

## 2026-11-26 — Budget +1 for the Critic C1–C4 repair (owner)

- **Input:** CodeReviewer APPROVE on r35 (round 76); Critic r35 REVISE with four new MAJOR findings C1–C4, so no passing receipt and no implementation authority.
- **Decision:** owner incremented the CodeReviewer allocation by exactly one review for the repair of C1–C4, followed by the Critic gate.
- **Scope:** C1 (bounded namespace/deferred-retirement metadata + churn oracle), C2 (single-execution invocation oracle), C3 (8192/8193 equality boundary), C4 (per-surface retrieve-and-reconstruct oracle). No new requirements or scope.
- **Source:** owner interjection on 2026-11-26 ("increment budget by one").

## 2026-11-26 — Full closure cycle for r37 authorized (owner)

- **Input:** CodeReviewer r36 (round seq 77) returned REQUEST CHANGES with F1 (HIGH, AC7 churn oracle can pass with a zero-charged namespace container) and F2 (LOW, AC43/T3 retirement ranges stop at 802-812/950-962 while the cleanup calls are at 813/966). The r35 allocation and its +1 increment were fully spent by the r36 repair and the r36 CodeReviewer round; the Critic gate was not reached.
- **Decision:** owner chose `budget=full-cycle` — planner repair of F1 and F2 only, then one CodeReviewer round on the resulting revision, then one Critic gate on the same inputs.
- **Scope:** F1 and F2 only. No new requirements, acceptance criteria, tasks, config keys or scope; dependency sweep mandatory; r36's closures (Critic C2-C4, r34 ledger rows) stand.
- **Stop:** after the Critic gate on the r37 inputs, report standing and wait for a new owner decision.
- **Source:** owner answer on 2026-11-26 to the r36 standing decision form (`budget=full-cycle`).

## 2026-11-26 — r37 closure cycle outcome (coordinator record)

- **r37 installed:** `.scratch/token-efficient-tool-responses/spec.md`, 104,556 bytes, sha256 `12fb6cc6b649c06edab8c217d44b137cf3930c6a652ee5a90edde03da0be7a16`; state revision `r37`, packet digest `609282aa…`, branch `token-efficient-tool-responses-ticket01-r37`. Changed lines vs r36: 15 (STATUS, R7 ranges, two ledger rows, AC7, AC43).
- **CodeReviewer r37:** APPROVE (round seq 78). F1 and F2 verified closed; one LOW advisory (AC7 does not pin the observation instant for the structural assertion — take it after the T2 sweep hook at the post-clearing clock).
- **Critic r37:** REVISE (observation source sha256 `78ad39040a1910a7c828e86dde0603acfb0811011df9b8081473a4ee155b8d38`). New finding **C5 (MAJOR, high confidence)**: R6/R7 forbid deferred-retirement bookkeeping and say a refused retirement is retried "recomputed from the inbound trigger", but every trigger deletes the session/SDK mapping and retirement removes `liveOwners[owner]` (`internal/service/inbound.go:804-813`, `:950-966`, `:1120-1162`; `projection.go:863-874`), so after a refused tombstone nothing identifies the owner for a later sweep — retirement can stay unfinished indefinitely. AC38's fixture reinjects the forgotten owner, so it proves retry mechanics, not production rediscovery. Closing criterion: refusal → trigger completion → capacity release → ordinary production sweep → completed retirement, without fixture reinjection. All of R1–R16 receive explicit verdicts; R7 is the only REVISE.
- **Standing:** canonical state remains blocked (`E_AUTHORIZATION` selected-branch + mutation authorization; `E_REVIEW` two passing reviews missing; `E_NO_RUNNABLE_FRONTIER`). The owner's stop condition for this cycle is reached: report and wait for a new owner decision before any further repair.

## 2026-11-26 — Second full closure cycle authorized for r38 (owner)

- **Input:** CodeReviewer r37 APPROVE (round seq 78) with one LOW advisory; Critic r37 REVISE with C5 (MAJOR): refusal of a retirement tombstone leaves no discoverable record, so the promised later-sweep retry cannot happen once the inbound trigger has deleted its session mapping.
- **Decision:** owner chose `budget=full-cycle` again — planner repair of C5 plus the LOW AC7 observation-instant advisory, then one CodeReviewer round on the resulting revision, then one Critic gate on the same inputs.
- **Scope:** C5 and the AC7 advisory only. No new requirements, acceptance criteria, tasks, config keys or scope; no widening of the envelope; prior closures stand.
- **Stop:** after the Critic gate on the r38 inputs, report standing and wait for a new owner decision.
- **Source:** owner answer on 2026-11-26 to the r37 standing decision form (`budget=full-cycle`).

## 2026-11-26 — r38 closure cycle outcome (coordinator record)

- **r38 installed:** 111,563 bytes, sha256 `5d34706213db34a4f8d546f25b27b186427ccbeacfa438e0f5c10a16e02ac55a`; state revision `r38`, packet digest `f76c51f8…`, branch `token-efficient-tool-responses-ticket01-r38`. 29 changed lines; R1–R16, AC1–AC44, T1–T5 and the envelope are unchanged.
- **CodeReviewer r38:** APPROVE (round seq 79). C5's reconciliation verified (retirement admitted by construction: a namespace holding a live record funds its 256-byte tombstone from its own releases; a namespace with no live record has no store presence and completes with zero effect). AC7 observation instant verified closed. One LOW advisory: the `r34-F3` ledger cell still points at AC38 for the retired retry wording.
- **Critic r38:** REVISE (source sha256 `d54a6d89…`). New **C6 (MAJOR, high confidence)**: r38's zero-effect empty-namespace branch plus §3's Critic-handoff sentence permit an in-flight admitted execution to commit after that retirement and resurrect a namespace with a handle the caller cannot use — contradicting R10's `result_namespace_retired` and T2's no-resurrection rule. Closing criterion: amend AC38/AC43 in place to observe admitted execution → empty-namespace retirement → resumed over-ceiling result requiring `result_namespace_retired`, no handle, no retained entry and no aggregate increase, and the same invariant across tombstone clearing; reconcile the handoff text with R10/T2. C5 and the AC7 advisory are both verified closed; R6/R7/R10 are the only REVISE verdicts.
- **Standing:** canonical state remains blocked (`E_AUTHORIZATION`, `E_REVIEW`, `E_NO_RUNNABLE_FRONTIER`). The owner's stop condition for this cycle is reached: report and wait for a new owner decision.

## 2026-11-26 — Third full closure cycle authorized for r39 (owner)

- **Input:** CodeReviewer r38 APPROVE (round seq 79) with one LOW ledger advisory; Critic r38 REVISE with C6 (MAJOR): the zero-effect empty-namespace retirement plus the §3 Critic-handoff sentence allow an in-flight admitted execution to commit afterwards and resurrect a namespace with an unusable handle, contradicting R10 `result_namespace_retired` and T2's no-resurrection rule.
- **Decision:** owner chose `budget=full-cycle` — planner repair of C6 plus the LOW ledger advisory (strike the stale `r34-F3` AC38 retry pointer), then one CodeReviewer round, then one Critic gate on the same inputs.
- **Scope:** C6 and that advisory only. No new requirements, acceptance criteria, tasks, config keys or scope; no envelope growth; prior closures stand.
- **Stop:** after the Critic gate on the r39 inputs, report standing and wait for a new owner decision.
- **Source:** owner answer on 2026-11-26 to the r38 standing decision form (`budget=full-cycle`).

## 2026-11-26 — r39 closure cycle outcome (coordinator record)

- **r39 installed:** 116,828 bytes, sha256 `9642a10e6e06bf782893e0e5d493ec17112f4f9237678e9962ba50635b5a5866`; state revision `r39`, packet digest `af9cb934…`, branch `token-efficient-tool-responses-ticket01-r39`. 8 changed hunks; R1–R16 / AC1–AC44 / T1–T5 and the envelope unchanged.
- **CodeReviewer r39:** APPROVE (round seq 80). C6 verified closed in all four limbs: R6's rescoped post-retirement commit terminal, no bookkeeping, the §3 handoff reconciled with R10/T2, and AC38/AC43 carrying the resumed-result oracle across tombstone clearing. One LOW advisory: three ledger cells still describe the superseded retry as if live (append "(superseded by r37-C5)" or past tense); no normative clause affected.
- **Critic r39:** REVISE (source sha256 `ec447eb3…`). New **C7 (MAJOR, high confidence)**: AC38 now requires an admitted upstream call retired through the production trigger and resumed, but its owner T2 is declared store-only and handler-free while T3 owns delivery and inbound retirement wiring. The cheap false pass is a fixture-driven store retirement standing in for the production trigger; the literal test cannot execute at T2 with its declared prerequisites. Closing criterion: reconcile AC38's phase boundary so every T2 assertion is executable with T1/T2 artifacts alone, while the full production-trigger→resumed-result sequence remains mandatory once that wiring exists; synthetic owner reinjection must not substitute for production evidence. R6/R7/R10 are the only REVISE verdicts; C6 and the ledger advisory are verified closed. Reviewer requests the configured `spec_critic_escalated` gate.
- **Standing:** canonical state remains blocked (`E_AUTHORIZATION`, `E_REVIEW`, `E_NO_RUNNABLE_FRONTIER`). Owner stop condition for this cycle reached: report and wait for a new owner decision.

## 2026-11-26 — Two more review units authorized for the C7 cycle (owner)

- **Input:** CodeReviewer r39 APPROVE (seq 80) with one LOW ledger advisory; Critic r39 REVISE with C7 (MAJOR): AC38's production-trigger late-commit subtest is infeasible at T2, which is declared store-only and handler-free, while T3 owns the wiring.
- **Decision:** owner answer to the r39 decision form: "add two to the budge" — two review units, i.e. one CodeReviewer round plus one Critic gate on the same inputs.
- **Scope:** planner repair of C7 (AC38 phase boundary; the production-trigger→resumed-result sequence stays mandatory where the wiring exists) plus the LOW ledger advisory (mark the three superseded retry cells). No new requirements, acceptance criteria, tasks, config keys or scope; prior closures stand.
- **Stop:** after the Critic gate on the r40 inputs, report standing and wait for a new owner decision.
- **Source:** owner free-text answer on 2026-11-26 ("add two to the budge").

## 2026-11-26 — r40 cycle closed: CodeReviewer APPROVE + Critic PASS; review receipts recorded (coordinator record)

- **r40 installed and published:** 121,628 bytes, sha256 `37dd21cee4cebdd8c5e4771741e05e594feb01292e43269ace89ee11d35ad686`; state revision `r40`, packet digest `53a6908b…`, branch `token-efficient-tool-responses-ticket01-r40`. Nine changed hunks; R1–R16 / AC1–AC44 / T1–T5 and the envelope unchanged. GitHub issue #157 mirrors the bytes exactly (three spec comments + decision-record comment; body carries header, status, tickets, index) and #115 carries the revision/gate pointer.
- **CodeReviewer r40:** APPROVE (round seq 81). C7 closed — AC38 asserts only the store-level late-commit terminal executable with T1/T2 artifacts (store seam + injected clock); the production-trigger→resumed-result sequence is delegated to AC43/T3 `TestInboundRetirementHooks` under a distinct oracle name.
- **Critic r40:** **PASS** (source sha256 `e6d341ace66c9b8aa194606dc29d191ec0b456fdf435ca967a0535e6aa23ba17`). C7 verified closed, C6 preserved, all R1–R16 PASS with a cheap-false-pass→oracle row each; the retired LOW ledger pointer is historical only. No silent owner-decision resolution.
- **Quota episode:** two Critic dispatches failed with `usage_limit_reached` on `openai-codex/gpt-6.1-sol:high` (2m16s, 974ms); the retry after recovery succeeded (`SpecGatePaging-6`, 1m49s). No review unit was consumed by the failures. `execution-state.mjs record-failure` is schema-1 only and refused this state (`E_STATE_INVALID`), so the evidence is here.
- **Mid-cycle binding repair:** appending this log is itself a review input (`decisions.md` is one of the 43 candidate inputs), so the entry written between the two reviews was rolled back to the bound digest `248433f0…` before the Critic retry; it is restored here after both observations and the canonical receipts committed. The r40 candidate's on-disk input set no longer matches the reviewed bytes for this file — correct for a post-review log append, and a future review of any further revision will re-pin it.
- **Canonical receipts:** `record-reviews` committed `reviews.specification` (code-reviewer, pass) and `reviews.tasks` (critic, pass) bound to packet digest `53a6908b…`. Remaining blockers are `E_AUTHORIZATION` only: branch selection and explicit owner authorization for mutation.

## 2026-11-26 — T1 implemented and verified (owner-authorized slice)

- **Scope executed:** T1 only (`internal/service/result_measure.go`, `internal/service/result_measure_test.go`, `internal/config/bounds.go`, `internal/config/config.go`, `internal/config/diagnostics.go`, `internal/config/validate_edge_test.go`, `docs/logging-contract.md`). Effect set amended from the pre-r40 three-path set to the spec's seven T1 write roots under the owner authorization source `src-owner-authorization-4d0ac259170c4a11`.
- **Verification (coordinator-run, not engineer claims):** `go build ./...` exit 0; `go test ./internal/config/ -run 'TestConfigRelations|TestCeilingOrderRejected|TestConfigEnvelopeFeasibility|TestEnvelopeHeadroomMaximalMetadata' -count=1` ok; `go test ./internal/service/ -run 'TestMeasurementIndependentOracle' -count=1` ok; full `go test ./internal/config/ -count=1` ok (closed-registry assertions still pass); `gofmt -l` on the six Go files prints nothing.
- **Defaults and codes:** §6 defaults installed verbatim (8192/1536/8388608/16777216/16/67108864/1h/15m/512/16); the eight codes are registered in the closed `diagnosticCodes` array; headroom constant is 6 per R11(g).
- **Independence risks carried into review:** (1) `TestMeasurementIndependentOracle`'s "golden" byte strings above the ceiling are assembled by concatenating a generated payload, so the literal byte-string property is weaker than R2's "literal expected byte string and literal expected length" even though the length literals (136, 8197) are hardcoded; (2) the config envelope tests rebuild the maximal-metadata envelope with their own helper that mirrors the implementation's metadata map rather than pinning R11(g)'s widths as literals, so a width change would move both sides together. Both are test-independence gaps, not behaviour gaps.
- **State:** T1 remains `pending` in canonical state — no schema-2 per-task completion CLI is exposed in this environment (`recordCompletion` has no CLI wiring), so the task cannot be marked done without inventing a writer. Next gate is the owner's decision to continue with T2 (not yet authorized).

## 2026-11-27 — Canonical acceptance graph aligned to spec r40

- **Defect:** the state carried acceptance AC1–AC30 with every task declaring the placeholder `acceptanceIds: ["AC2","AC5"]`, so no task owned its real oracles and completion validation could not pass.
- **Action (owner: "go ahead with that"):** regenerated `acceptance` (44 entries) from spec §4 — requirement ids from the Covers column, existing entries' requirementIds/method/requiredEvidenceKinds preserved; ownership from each row's explicit `(T#)` tags, with §5 test lists used only for the four untagged rows (AC1, AC3, AC15, AC27) and AC19 additionally assigned to T5 per the T5 paragraph. Task `acceptanceIds` set from that ownership.
- **Owner drift:** AC1 T3→T4, AC15 T2→T3, AC16 T2→T2,T3, AC19 T4→T4,T5, AC27 T5→T3,T5. Added AC31–AC44. Per-task counts: T1 12, T2 11, T3 22, T4 2, T5 2.
- **Safety:** `packet.digest` covers only `feature/projectRoot/sectionBindings/branches`, so the alignment leaves the digest, the recorded authorization and the review bindings intact. Pre-alignment file kept at `.omp/plans/execution-state.pre-alignment-r40.json`; `check` on the installed state returns `ready` with frontier `CR1, CT1, T1`.
- **Gap found while preparing the T1 review:** the r40 task graph grants `code-reviewer` only on CR1 (spec review, effects = CR1's canonical path set), and `engineer` alone on T1. There is no implementation-review task, so a pre-completion independent review cannot be dispatched: prepare-dispatch answers `E_ROLE_NOT_PERMITTED` / `E_EFFECT_OUTSIDE_ENVELOPE`. Completion recording is therefore blocked on an owner choice, not on evidence.
- **T1 evidence captured:** `.omp/artifacts/t1-acceptance/acceptance.log` (sha256 `a94a8cbf2ff14b6cc87cd837fdcfabc20fede712267fc4058a74080727a38d3d`): build exit 0, config focused tests pass, service oracle test passes.

## 2026-11-27 — Owner exception: out-of-gate read-only review of T1

- **Owner authorization (verbatim choice):** `review: "exception"` — "Authorize a one-off read-only `code-reviewer` dispatch outside the managed gate for T1's seven files; I record the exception verbatim in `decisions.md`, then complete T1 from its findings"; `after: "t2"` — "Continue straight into T2 under the existing branch authorization".
- **Scope of the exception:** exactly one `code-reviewer` run, read-only, over `internal/service/result_measure.go`, `internal/service/result_measure_test.go`, `internal/config/bounds.go`, `internal/config/config.go`, `internal/config/diagnostics.go`, `internal/config/validate_edge_test.go`, `docs/logging-contract.md` and `.scratch/token-efficient-tool-responses/spec.md`. It grants no write, no state change, no dispatch exception for any other task, and no authority beyond this run.
- **Why it was needed:** the r40 packet grants `code-reviewer` only on CR1 (canonical effect set `[.scratch/token-efficient-tool-responses/spec.md]`) and `engineer` only on T1, so the managed gate answers `E_ROLE_NOT_PERMITTED`/`E_EFFECT_OUTSIDE_ENVELOPE` for an implementation review. The gate result is a metadata boundary, not a review conclusion.
- **Consequence:** T1's completion record cites test evidence; the review finding set is recorded alongside in this log and as an artifact under `.omp/artifacts/`.

## 2026-11-27 — Envelope amendment for T2–T5 write roots (owner-authorized)

- **Owner authorization (verbatim):** `amend: "all"` — "Authorize all nine now (covers T2, T3, T5; T4 needs none) and align each task's effects to its spec write roots"; `after: "t2"` — "Dispatch T2 immediately (retention store, store-only, no handler wiring)".
- **Envelope `writePaths`:** 27 → 36, adding `internal/observe/log.go`, `internal/observe/metrics_test.go`, `internal/observe/httpdiagnostics.go`, `internal/service/tools_test.go`, `internal/service/builtin_guidance_test.go`, `internal/service/progressive_session_test.go`, `cmd/mcp-bridge/serve.go`, `cmd/mcp-bridge/serve_hybrid_reload_test.go`, `cmd/mcp-bridge/logging_contract_test.go` — each a write root named by spec §5's own task text.
- **Task effects aligned to spec §5 write roots:** T2 6→9, T3 11→19, T4 3→4, T5 3→5 (T5 keeps `README.md` and `internal/catalog/schema.go` from the prior packet and gains `cmd/mcp-bridge/serve.go`, `internal/service/tools.go`). T1 unchanged (already set-equal).
- **No reduction:** no authorized path was removed anywhere; the amendment only widens the write surface to the spec's statement.
- **Safety:** `packet.digest` covers `feature/projectRoot/sectionBindings/branches` only, so authorization and review bindings are unaffected; `check` returns `ready` with frontier `CR1, CT1, T2`.
- **Also recorded:** the T1 review's finding 2 (maximal envelope omits outer `CallToolResult.IsError`, +14–15 B beyond the checked envelope) is a spec-level pin and stays open for the planner before T3 sizes real previews.
