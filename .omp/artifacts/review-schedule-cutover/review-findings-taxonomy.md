# Review findings taxonomy

Reference for the planner and the code-reviewer. Every class below was observed in the
structured-logging r4–narrow9 loop (11 code-reviewer reports, 13 REVISE verdicts). The
planner reads this before returning a revision; the reviewer labels findings by class.

A finding carries `class` (one of the ids below), `severity` (`HIGH`/`MEDIUM`/`LOW`),
`anchor` (`file:line`), and `fix` (one sentence). Class decides blocking;
severity orders the report only.

## Classes

### `ac-not-falsifiable`
An AC cannot fail: it asserts presence, a positive-only tail, or a condition the code
cannot produce. Evidence: narrow3 AC08-7 "debug-only" against `log.go:33-35` (no level
threshold); narrow8 AC12-1 did not assert `schema_version: 1`, so an emitted `2` passed.
Check: every AC names an observable and an expected value, or a test that can fail. A
"must not" clause needs a counterexample the test can construct.

### `r-ac-coverage-gap`
A requirement has no AC, or an AC covers a requirement the spec no longer states.
Evidence: r8 R24 demanded parity AC11-1 declared refresh-only; narrow2 same root.
Check: every R id appears in at least one AC; every AC cites an R id; the R→AC map in
the spec matches the AC tags.

### `statement-contradiction`
Two statements impose incompatible behavior under the same conditions.
Evidence: cite both statements and a concrete input or state where they disagree.
Check: reconcile scope and precedence so one observable outcome satisfies both.

### `lifecycle-hazard`
An unresolved concurrency or lifecycle schedule violates a stated invariant.
Evidence: identify the competing actors, transition order and invalid state.
Check: walk creation, use, invalidation, failure and restart through that schedule.

### `ledger-state-drift`
The task ledger, the state projection, and the ticket files disagree on dependencies,
effects, or status. Evidence: narrow1 T10 effects vs ledger; narrow4 T10 `blockedBy`
vs "Deps: T8"; narrow8 `tasks[T8].blockedBy` missing its T7 edge.
Check: for every task, `blockedBy` in state equals the ledger's dependency list; the
effects set equals the ledger's write paths; the ticket's `Blocked-by` equals both.

### `ticket-mirror-drift`
The spec is corrected but the ticket file keeps the pre-fix text, breaking the declared
1:1 spec↔ticket AC mapping. Evidence: narrow5 `issues/09:14`, `10:14`, `12:14` stale;
narrow7 three closures never reached the tickets; narrow8 `issues/12:12` stale.
Check: for every AC, the ticket's AC text is byte-equal to the spec's AC text (or the
ticket cites the spec line and the spec line is the single source).

### `undefined-term`
A term in an AC, ruling, or count is not defined in the glossary or the spec.
Evidence: r8 `:199` "17→24", `:203` "fallback stream".
Check: every noun phrase that carries a count or a state name resolves to a glossary
entry or a spec definition; every count matches the enumerated set.

### `shared-oracle`
Two ACs are verified by the same test or the same assertion, so one can pass while the
other is false. Evidence: narrow4b AC12-3 reused AC12-1's oracle; narrow8 AC12-1 vs
AC04-1 both asserted `schema_version` presence.
Check: each AC names a distinct oracle (test name + assertion); no two ACs share a test
function unless the spec says they are one criterion.

### `severity-inflation`
A finding blocks solely because of its severity rather than its class and round.
Evidence: a HIGH mechanical finding blocks, or a LOW substantive finding is ignored.
Check: class decides blocking; severity orders the report only.

### `stale-citation`
A `file:line` or a count in the spec no longer matches the repository. Evidence: r8
`:199` count, `:203` term; narrow8 "one line off" citation.
Check: every `file:line` resolves and the cited line contains the claimed text; every
count matches the enumerated set at the cited anchor.

## Linter mapping

`spec-lint.mjs` implements the mechanical checks for `ac-not-falsifiable` (heuristic),
`r-ac-coverage-gap`, `ledger-state-drift`, `ticket-mirror-drift`, `undefined-term`
(glossary), `shared-oracle` (test-name reuse), and `stale-citation`. The semantic
classes (`ac-not-falsifiable` deep cases, `r-ac-coverage-gap` intent) remain the
reviewer's judgement; the linter only flags the mechanical signature.

## Round discipline

- Before submission, run mechanical checks for stale citations, anchor resolution
  and oracle presence. These consume no review round and never produce `E_ROUND_CAP`.
- Rounds 1–2 block only `ac-not-falsifiable`, `r-ac-coverage-gap`,
  `statement-contradiction` and `lifecycle-hazard`. Severity never decides blocking.
- Round 2 is delta review: changed hunks plus their cross-references.
- Rounds 3+ never block on findings and require no exhaustion override.
  `E_ROUND_CAP` is reserved for cap-continuity mismatches.
- Attach the residual register to the dispatch/delivery record: findings from
  rounds 3+ retain `class`, `severity`, `anchor` and `fix`. Residuals are not approvals
  and do not relax source binding, identity, authorization or access checks.

