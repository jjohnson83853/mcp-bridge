Spec ID: SNAPREC-SPEC-r15
Revision: r15
Candidate: snapshot-recovery-spec-r15@r15
Findings by ID: C6

**C6 — HIGH / statement-contradiction / confidence HIGH**

Anchors: `.scratch/snapshot-recovery/spec.md:16,19,77,116` — R2, R5, quarantine/retention algorithm, AC1-10.

R2 promises preservation of a moved symlink’s target; R5 protects only this run’s quarantine **name**, not its backing bytes.

Concrete counterexample:
- Seed five conforming snapshot quarantines, with corrupt regular file `snapshot.corrupt-<oldest>.json` oldest.
- Make `snapshot.json` a relative symlink to that file.
- Recovery moves the link to this run’s quarantine name.
- Retention then unlinks the oldest pre-existing file—the link’s target.

The specified successful pass returns with a dangling quarantine link and deletes the rejected bytes. No concurrent writer or injected failure is needed. This contradicts R2 and AC1-10’s target-preservation/read-through oracle.

**Minimal amendment:** reconcile retention with symlink-backed evidence preservation: protect the backing evidence, explicitly resolve the resulting bound/order semantics, and add this fixture under existing AC1-3/AC1-10 with target-byte and quarantine-readability assertions. Do not weaken preservation merely to satisfy retention tests. Request coordinator escalation to configured non-OpenAI `spec_critic_escalated` for this irreversible-deletion interaction.

This is a new symlink/retention interaction, not C5 reopening. Reason for the earlier miss is unknown.

**Verified r14 closures**
- C5’s original failure-path contradiction is closed: R5/D10/§4/AC1-5/AC1-6 specify finite attempts, first-failure termination, zero-attempt listing failure, permitted excess retention, exact failure counts and surviving bytes.
- Corrected `validErrorInfo` anchor matches `internal/observe/envelope.go:136-141`.
- AC1-6 now pins `resource.path`; AC1-3 specifies the timestamp tie-break; R2 explicitly includes stat-error entries.
- Not all six advisory closures are complete: §9.7 still describes an r14 ticket, but the supplied mirror is r15; T1 still claims AC1-6 wholesale despite T2/T3’s split. These match CR15-01/02 and remain advisory.

**Coverage and limits**
R1–R9 have AC coverage; task write sets match the 14-file envelope. Reservation, fatal ordering, warning counts, classifier compatibility and startup/refresher publication were assessed against referenced source. §7 documents the collector boundary and unalerted-event consequence, consistent with `docs/logging-contract.md:580-614`.

C6 leaves T1 and dependent T3 technically unresolved. §9’s owner checkpoint remains required. Repairs require fresh candidate-bound reviews; historical observations remain unchanged.

Read-only specification assessment: no implementation, commands, runtime results, hash recomputation or execution approval claimed. Registry identity relies on supplied bindings.

### Verdict
REVISE