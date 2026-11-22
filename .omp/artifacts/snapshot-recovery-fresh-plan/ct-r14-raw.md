Spec ID: SNAPREC-SPEC-r14
Revision: r14
Candidate: snapshot-recovery-spec-r14@r14
Findings by ID: C5

**C5 — HIGH / statement-contradiction / confidence HIGH**  
Anchors: `.scratch/snapshot-recovery/spec.md:19,75,110` (R5, retention algorithm, AC1-6).

R5 promises at most five retained files and eviction “while more than four pre-existing remain.” AC1-6 seeds five pre-existing files per family, makes unlink fail, and requires exactly one failed eviction per family with successful return. All five pre-existing files plus the new quarantine necessarily remain: six, not five. The specified stopping condition also remains true after failure; retrying cannot meet AC1-6’s exact warning counts.

**Consequence:** T1 lacks a consistent, terminating failure-path contract; implementations can hang, over-delete alternative evidence, or produce incorrect retention-warning counts.

**Minimal amendment:** reconcile R5/§4 with AC1-5/AC1-6: select the oldest excess candidates once, attempt each at most once, preserve this run’s file, and explicitly permit exceeding the retention bound when listing/unlink fails. Pin attempt counts and surviving bytes in the existing ACs. Planner owns reconciliation; any substantive acceptance change requires affected-owner confirmation. Escalate the irreversible-deletion contract to configured non-OpenAI `spec_critic_escalated`.

This is a newly identified retention-failure invariant defect, not CR13-01’s cross-family-filter finding reopening. Reason for the earlier miss is unknown.

**Verified closures**
- CR13-01/02: family-scoped filtering and cross-family fixtures; collision advancement versus fatal reservation errors.
- CR13-03/04: disabled-mode Durable warning retained; fatal-first processing leaves prompts untouched.
- CR13-05/06/08: explicit retention classification/message literals, producer remediations, numeric no-block deadlines.
- CR13-09 and both open questions: five-reason admission table, prompts negative oracles, satisfiable successful-pruning wording, empty-string enablement.
- CR13-07 remains advisory: §9.7 still says the ticket mirrors r13; the supplied heading and repaired cases are r14. Update that account.

**Coverage and limits**
All nine requirements have AC references; task file unions match the 14-file envelope. Rename/reservation, cause preservation, warning ordering and startup/refresher paths were assessed against supplied source. Successful-path retention is specified; failure-path retention remains inconsistent. T1 and dependent T3 acceptance remain unresolved by C5.

Source checks confirmed `validErrorInfo` equality at `internal/observe/envelope.go:136-141`—the spec’s `:200-206` citation is stale—and producer remediation literals. §7 explicitly acknowledges unalerted events, consistent with `docs/logging-contract.md:607-614`.

Read-only specification review: no implementation, tests, hash recomputation, runtime proof or execution approval claimed. Registry identity relies on supplied bindings. Repairs require fresh candidate-bound review observations; historical observations remain intact.

### Verdict
REVISE### Verdict
REVISE