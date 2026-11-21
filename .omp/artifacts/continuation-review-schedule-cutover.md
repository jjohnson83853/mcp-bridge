**Implement: dispatch-gate review-schedule change (mcp-bridge workflow-policy)**

Scope: this prompt covers only the gate implementation. No breaker work, no planner, no spec revision.

## Current behavior (verified)
- `execution-review-sequence.mjs` -> `roundCapIssues(candidate, registry, ownerOverride)`: once `history.rounds.length >= candidate.maxRounds` it returns `E_ROUND_CAP` unless an explicit owner override is passed. Exit requires the override.
- `execution-gate.ts`: `E_REVIEW` = "A required passing independent review is missing; obtain it before retrying dispatch." Every dispatch is denied when any blocking finding remains.
- `review-findings-taxonomy.md` already says LOW is advisory and lists `severity-inflation` as a defect, but blocking is decided by the reviewer's verdict, not by class.

## Target schedule
- Rounds 1-2: block on substantive classes only:
  (a) `ac-not-falsifiable` / `r-ac-coverage-gap` (requirement without a falsifiable oracle),
  (b) contradiction between statements (new class id `statement-contradiction`),
  (c) unresolved concurrency/lifecycle hazard (new class id `lifecycle-hazard`).
- Rounds 3+: never block; pass with a residual register attached to the delivery record.
- Severity orders the report only. LOW/SMEDIUM/HIGH never decide blocking.
- Round 2 is delta review (changed hunks + cross-references).
- Mechanical checks (`stale-citation`, anchor resolution, oracle presence) are pre-submission; they consume no round and never produce `E_ROUND_CAP`.

## Change list
1. `review-findings-taxonomy.md`
   - Add `statement-contradiction` and `lifecycle-hazard` to Classes with the same shape as existing entries (evidence, check).
   - Replace "the severity is what makes the finding blocking" with: class decides blocking, severity orders the report.
   - Document the taper and the residual register.
2. `execution-review-sequence.mjs`
   - Add `blockingFindings(round)`: returns only `(a)/(b)/(c)` class entries.
   - `roundCapIssues`: at `rounds >= 2` (third round), return `[]` when `blockingFindings` is empty; drop the `ownerOverride` requirement for that case. Keep `E_ROUND_CAP` only for cap-continuity mismatches, not for exhaustion.
   - Add `residualRegister(registry, lineageKey)`: collect non-blocking findings from rounds >= 3 with `class`, `severity`, `anchor`, `fix`.
   - Keep `lineageReviewHistory` and `reviewRoundCount` signatures unchanged.
3. `execution-gate.ts`
   - Where `E_REVIEW` is raised for a missing passing review, consult the taper: at `rounds >= 2` with no `blockingFindings`, treat the review as satisfied and attach `residualRegister` to the dispatch payload instead of denying.
   - Do not weaken `E_AUTHORIZATION`, `E_VIEW_BINDING`, `E_PATH_ESCAPE`, `E_EFFECT_OUTSIDE_ENVELOPE`, `E_LOCK_ORDER`, or the tg hook.
4. Tests
   - `execution-review-sequence.test.mjs`: LOW-only round 1 -> not blocking; contradiction round 1 -> blocking; contradiction round 3 -> not blocking + residual entry; two cap-continuity mismatch cases unchanged.
   - `execution-gate*.test.ts`: same three cases through the dispatch path.

## Constraints
- Snapshot each edited file (bytes + sha256) to `.omp/artifacts/review-schedule-cutover/` before editing.
- Do not edit `.omp/plans/execution-state.json`, `.omp/plans/ticket-28-*`, review registries, denials, or counters.
- Do not dispatch planner/reviewer/engineer to build this change. Main is the implementer.
- Use the runner the tree already uses for `.ts` suites; add no new runner.
- Behavior changes only in a fresh session: the extension modules are loaded at session start.

## Acceptance
1. A round-1 `stale-citation`/LOW finding does not deny dispatch.
2. A round-1 contradiction denies with `E_REVIEW` and names the class and anchor.
3. A round-3 contradiction allows dispatch and records a residual entry.
4. No owner override is needed to exit at round 3+.
Existing suites pass. Changes are limited to the files listed
   in Change list and required artifacts. Preserve unrelated
   pre-existing changes.
