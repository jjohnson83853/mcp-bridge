# r8 review deferrals — backlog

Owner decision (2026-11-11): r8 narrows to what the reviewed revision can actually assert;
everything that cannot be done without further refinement moves here.

Source of every item: `.scratch/structured-logging/review-code-reviewer-r8-revise.md`
(independent code-reviewer review of `r8-pin-diagnostics`; run
`structured-logging-r8-pin-diagnostics` = `succeeded` in `.omp/plans/review-runs.json`).
The verdict was `revise`, so no review was recorded and nothing was waived.

These are **pre-spec** tickets: they carry evidence and the blocking question, not acceptance
criteria. ACs and design text are authored by the planner once the open question is ruled on,
because writing them is exactly what the r8 authorization withheld.

| Ticket | Kind | Blocking question |
|---|---|---|
| 13 startup pin-evaluation leg | grilling | Is the "startup" leg the first post-startup pass, or an explicit startup pin check? |
| 14 refresh counts and age assertions | task | Do AC08-3/AC08-2 get extended to AD16's full field set? |
| 15 reload acceptance record | task | Which AC asserts `config_reload_applied` and the additive rejected-half fields? |
| 16 pin position retention + origin accessor | task | Widen the envelope into `internal/config`, or re-scope AC09-2? |
| 17 r8 artifact consistency drift | task | Mechanical; held so spec/state hashes do not churn twice |
| 18 owner ruling: misleading-counter gap | grilling | Accept the counter gap (on what terms) or keep it pending? |
| 19 reload-leg runtime parity | task | Reload-leg parity beyond pin identity + `config.property_path` — discovery evidence, retry guidance and publication impact on the reload-side record — needs a new AD plus envelope widening; which AD licenses it and what reload record carries it? |

Pick order follows the tracker convention: first open, unblocked, unclaimed by number.
None of these block r8; r8 does not block any of them.

## Harness changes (owner request 2026-11-11)

Evidence: 11 code-reviewer reports sit in this directory (`r4`, `r5`, `r6`, `r7`,
`r7-closure1`, `r8`, `narrow1`–`narrow4b`). Every report that carries a verdict says
`REVISE`/`REQUEST CHANGES`; there are zero passes and zero critic runs. The findings recur in
six classes — AC-not-falsifiable, R→AC coverage gap, ledger/state drift, ticket-mirror drift,
undefined term, shared oracle — plus severity inflation (LOW findings blocking a pass).
The per-round owner asks were **not** gate demands: `execution-review-sequence.mjs` states that
observations "bind bytes and actor claims, not consent or identity", and its `--gate-check` only
requires a current independent positive code-reviewer observation for the exact candidate and
inputs. The asks followed from a self-written clause in the r8 authorization ("no change to
acceptance criteria or requirement text beyond the reviewed r8 revision", quoted back at
`review-code-reviewer-r8-revise.md:39`) plus review-seed clause 2b, which obliged the reviewer to
enumerate every prior finding as closed/deferred/unaddressed and so manufactured a residual list
every round. Neither clause is in the live state's `authorization` (mode/id/digest/branch only).

These are **harness** tickets, not spec tickets: they change `/home/opencode/.omp/agent/workflow-policy/`
and the planner/reviewer seeds, so they need owner routing and are not part of the r8 envelope.

| Ticket | Kind | Change — and the evidence it would have caught |
|---|---|---|
| 20 pre-dispatch spec linter | task | `spec-lint.mjs` run by the planner before it returns: AC↔producer bijection, R→AC coverage, every AC states an observable plus expected value (or a test that can fail), every `file:line` resolves, glossary-only terms, no two ACs sharing an oracle, event-name/count reconciliation, ledger↔state edge equality, specdd marker integrity. Would have caught the AC-not-falsifiable class (r8 `AC10-1`/`AC11-3`, narrow4b `AC09/AC10` level gap), `:199` "17→24", `:203` "fallback stream", `AC12-3` oracle reuse, and the T10 `Blocked-by` drift. |
| 21 severity-gated verdict | task | `pass` = zero open HIGH/MEDIUM; LOW recorded as advisory and never grounds for `REQUEST CHANGES`. Reviewer keeps full judgement of HIGH/MEDIUM. Would have passed rounds whose only open items were LOW (narrow4b §6 carries four such). |
| 22 drop closure enumeration | task | Replace review-seed clause 2b with "verify the delta's claimed closures; report regressions and new HIGH/MEDIUM only". 2b is what made every round produce a fresh residual list and therefore a fresh "amendment needing authorization". |
| 23 pre-authorized closure in the authorization record | task | At authorization time, record: findings that add no R/AC/T id and do not widen the envelope are closed by verbatim transcription (planner text or the reviewer's own fix sentence) for up to N rounds, without a new owner ask. Removes the per-round approval loop without transferring authority. |
| 24 mechanical round cap | task | `maxRounds` on the candidate record; `execution-review-sequence.mjs` refuses round N+1 without an explicit owner flag. The cap belongs in code, not in prose the coordinator can forget. |
| 25 findings taxonomy feedback | task | Maintain `review-findings-taxonomy.md` (class → evidence → check) and pass it to the planner every round; compile each class into ticket 20's rules. Ten rounds rediscovering six classes is the proof the loop is missing. |
| 26 pipeline order | task | linter (seconds, free) → code-reviewer (semantic) → critic. Stops burning 7-minute reviewer runs on mechanical drift. |
| 27 plan-arbitrator subagent | agent | A read-only `plan-arbitrator` subagent invoked when the S15 stop condition fires (four consecutive failed plan-change rounds, or no meaningfully different request left). Inputs: the round history (each round's request, verdict, findings), the current spec/tickets/state, and the reviewer's reports. Output: a bounded decision — (a) the minimal closure delta that ends the loop, (b) a severity triage separating blocking defects from advisory ones, or (c) a stop-and-escalate recommendation with the residual classes. Purpose: the coordinator authors each round from inside the loop and keeps re-opening the same defect classes (ticket-mirror drift, unasserted fields, stale citations); an independent arbiter with the full round history can see the pattern the coordinator cannot. Read-only, no edits, no dispatch authority. |

**Open, not ticketed:** the narrow4b review residuals (round 6, packet `8a3bba8d…`) are still an
owner decision — authorize `narrow5` for the R25 `level` clause plus the induced-failure wording,
record them as deferred gaps and proceed to the critic, or stop r8. They are deliberately not
ticket rows so the decision is not pre-judged.

