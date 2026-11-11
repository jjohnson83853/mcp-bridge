# Engineer dispatch — review-pause v1

Run from a main session rooted at `/home/opencode/.omp/agent` (no managed state there, so the coordinator-edit gate does not apply). Owner approved the design at `workflow-policy/review-pause-design.md`; implement T1–T3 only.

## Target files (exact)
- `/home/opencode/.omp/agent/workflow-policy/execution-review-sequence.mjs`
- `/home/opencode/.omp/agent/workflow-policy/execution-review-sequence.test.mjs`
- `/home/opencode/.omp/agent/workflow-policy/execution-gate.ts`
- `/home/opencode/.omp/agent/workflow-policy/execution-gate.test.ts`

Non-goals: no global policy files, no receipt schema changes, no new roles, no verdict synthesis, no edits to `apply-reviews.mjs` (deleted), no changes to other CODES semantics.

## T1 — detector, materiality, classifier (execution-review-sequence.mjs)
Add pure exports; no writes, no I/O:
```js
export function reviewStallSignal(candidate, registry, reviewsHistory) // {lineageKey, nonPassStreak, noProgressCount, carriedOpenIds, newBlockingIds, stalled}
export function reviewMateriality(priorReview, nextReview) // {material, resolvedIds, newEvidenceClasses}
export function classifyReviewFindings(findings, ownerDecisionIds) // {ownerDecision, technicalRepair, demoted}
```
Rules:
- `lineageKey = candidateId.replace(/-r\d+$/, '')`; if the id has no `-rN` suffix, use the full id (no mis-split).
- `nonPassStreak`: consecutive non-PASS verdicts for the lineage, newest first.
- `noProgressCount`: increments when `reviewMateriality` is false (hash-only delta).
- `stalled = (nonPassStreak >= 2 && noProgressCount >= 1) || nonPassStreak >= 3`.
- Classifier order: (1) every finding has exact path/locator + implementable change inside the approved envelope → technicalRepair; (2) contradiction of owner-settled D-ids, acceptance-contract change, gate-integrity facts, or iterate-vs-accept-advisory-only → ownerDecision; (3) repo-answerable → demoted to technicalRepair; (4) mixed → split.
- Counters are computed in exactly one place (this module); the gate must not recompute.

RED first: add fixtures for AC1–AC4, AC6, AC7 to `execution-review-sequence.test.mjs`; they must fail before the exports exist.

## T2 — pause gate (execution-gate.ts, `createToolCallGate`)
- Confirm the `reviewSequenceIssues` call site with one bounded grep; record the line.
- Add `E_REVIEW_PAUSE` to the local `CODES` map only.
- On `stalled` + `ownerDecision` + no open pause for `lineageKey`: return a structured pause instead of the generic `E_REVIEW` denial, and persist `reviewPause {pauseId, lineageKey, revision, questionIds, askedAt, packetDigest}` in canonical state.
- While a pause is open, deny candidate dispatches with `E_REVIEW_PAUSE`.
- Resume: accept a receipt bound to `pauseId` + `questionId` + answer digest; clear the pause; admit exactly one review round. Never reset attempt counters (S08).
- Pause cap: 2 per lineage; beyond that return a machine-readable blocked result, not another question.
- Fail-open: detector absent/unparsable → today's behavior (normal denial), never a hard failure.

RED first: AC1/AC5 fixtures in `execution-gate.test.ts` must fail on current code.

## T3 — verification
- `node --test --test-reporter=dot execution-review-sequence.test.mjs`
- `node --test --test-reporter=dot execution-gate.test.ts`
- Full policy suite with coverage ≥90% lines, same command convention as existing policy tests.
- Report exact commands, exit codes, and any blocked item with the call-site evidence.

## Acceptance
AC1 stall signal + §4F payload; AC2 single REVISE → no pause; AC3 all-locator → zero questions; AC4 hash-only delta → nonMaterial, `-r8` rename + restart → same lineageKey and counters; AC5 valid resume admits one dispatch, mismatched pauseId keeps `E_REVIEW_PAUSE`; AC6 different lineage admitted during a pause; AC7 state/registry JSON valid and restart re-derives identical signal.

## Constraints
- ≤42 read/search operations before the first edit; if exceeded, stop and report stalled.
- No external SDK/vendor research.
- Do not run the full repo suite mid-flight; run the three checks once at the end.
