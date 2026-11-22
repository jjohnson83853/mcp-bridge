Spec ID: snapshot-recovery-spec-r17
Revision: r17
Findings by ID: C1, C2

References below are to `.scratch/snapshot-recovery/spec.md` unless otherwise named.

**Verified closures**
- F1’s original A/B layouts are repaired: R2/R5, D10 and §4 populate protection before either family pass. AC1-3/AC1-10 require the protected `now-5s` target to survive, `now-4s` to be evicted, two recoveries and successful quarantine read-through. C6’s direct same-family case remains covered. The broader invariant still fails under C1.
- F2’s §4 candidate-set defect is repaired: all unprotected pre-existing files are eligible, oldest-first, stopping at four survivors. A repair-introduced contradiction remains under C2.
- F3–F5 are closed: `wiring.go:486-493` matches the source; §9.7 records ticket references r15/r7/r3; AC1-3/AC1-10 explicitly seed rejected target bytes.
- CR15-01–05 remain textually closed: mirror lag, split AC1-6 ownership, corrected anchors, complete ErrorInfo shapes and two-excess eviction oracles. R/AC/T identifiers, the 14-file §6.1 envelope and task decomposition are unchanged.

**C1 — HIGH / irreversible-deletion / confidence HIGH**
Anchor: R2/R5 (`:16,19`), §4 (`:78`), AC1-10 (`:117`).
One `Readlink` plus lexical cleaning protects only the immediate target. Seed five conforming snapshot quarantine entries: newest q1 is a relative symlink to oldest q5, which contains corrupt bytes; `snapshot.json` points to q1. Recovery protects q1, then retention deletes unprotected q5. Both quarantine links dangle and the rejected backing bytes are irreversibly lost despite successful retention.

This is a new F1/C6-class instance, not recurrence of A/B. Quarantined symlinks are explicitly supported, so reachability needs no concurrent attacker. The r17 CodeReviewer mentions this but underrates its consequence; the reason for the earlier miss is unknown.

Minimal amendment: specify protection of the complete backing chain, including resolution failures and compatibility with the retention bound; extend AC1-10 with this deterministic counterexample. Do not weaken R2 to pass it.

**C2 — MEDIUM / statement-contradiction / confidence HIGH**
Anchor: §8(2) (`:197`) versus R5/D10/§4.
The newly added assertion “at most the `QuarantineKeep−1` oldest … files are ever deleted” caps deletion at four. Nine pre-existing conforming regular files require five successful deletions to leave four plus this run’s file. Both promises cannot hold.

Minimal amendment: remove the four-deletion cap and state that successful deletion count is the excess above four pre-existing survivors; cover this backlog within AC1-3. This regression was introduced by the F2 repair.

**Coverage and limits**
Reviewed traceability, planned RED/GREEN, warning counts, reservation/rename ordering, ownership and §9 checkpoints. §7 documents scoped observability exceptions; collector delivery remains unverified. No implementation or tests were run, and SHA-256 comparison was unavailable through the effective tools. These are text-level findings, not runtime claims.

T1’s technical clearance remains unresolved. No approvals are granted or gate records changed. Coordinator escalation to the configured non-OpenAI `spec_critic_escalated` remains requested but unavailable—a limit, not a verdict substitute. Owner confirmation remains required before irreversible retention implementation.

### Verdict
REVISE
