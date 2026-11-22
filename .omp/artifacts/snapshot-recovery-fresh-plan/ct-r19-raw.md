Spec ID: snapshot-recovery-spec-r19 (document: SNAPREC-SPEC-r19)
Revision: r19
Findings by ID: C1, C7

References to `spec.md` mean `.scratch/snapshot-recovery/spec.md`.

**C1 — MAJOR/HIGH; statement-contradiction; confidence HIGH**
Anchors: `spec.md:19,65,79,118` — R5, D11, chain walk, AC1-10.

The original F2 fixture is repaired, but lexical cleaning still permits backing-byte deletion. Seed five conforming quarantines q1–q5, q5 oldest and corrupt. Create real directories `/s/sub` and `/s/deep`, `/s/sub/alias → /s/deep`, and:
`snapshot.json → sub/alias/../q5`
Here q5 denotes its full conforming filename.

Filesystem resolution reaches `/s/q5`. The specified `Clean(target)` instead produces `sub/q5`, absent. Protection misses the real q5; recovery moves the link, then retention deletes q5, leaving the quarantine dangling and destroying rejected bytes.

This is a new C1/F2-class instance, not reopening the original fixture. Earlier miss: unknown. Go’s `filepath` source explicitly distinguishes lexical cleaning from component-wise symlink resolution.

**Minimal amendment:** resolve directory symlinks before interpreting subsequent `..` components; retain complete-chain protection. Add this composed alias/parent-hop fixture under AC1-10, asserting q5 preserved, q4 evicted and quarantine read-through successful.

**C7 — MAJOR/HIGH; lifecycle-hazard; confidence HIGH**
Anchors: `spec.md:118` (b-xB2), R9; `internal/service/reload.go:537-550,723-740`; `internal/storage/store.go:112-126`.

The new move-through oracle stops before an incompatible startup consequence. With corrupt regular `snapshot.json` and `prompts.json → snapshot.json`, snapshot recovery moves the target; prompts recovery intentionally leaves its newly dangling link untouched. Admission then publishes a fresh **catalog** at `snapshot.json`. The prompts link now resolves to that catalog. Initial refresh calls unchanged `SavePrompts`, whose no-overwrite guard rejects `LoadPrompts` of catalog bytes. Fresh prompt persistence remains blocked, contradicting R9’s self-healing promise and potentially producing repeated persistence warnings.

**Minimal amendment:** reconcile newly orphaned configured links with self-healing inside startup recovery, distinguishing originally dangling links. Revise b-xB2 with affected-owner confirmation and add startup/refresh/second-run assertions; preserve no-overwrite protection. This consequence is exposed by r19’s explicit L2 fixture; earlier miss unknown.

**Verified coverage**
- F1: c4↔c5 are oldest, protected by entry identity/path; c3 alone is evicted.
- Original F2 subdirectory/alternating layouts preserve q5 and evict q4.
- L1 entry matching, L2 recovery counts, L3 six-survivor mixed carve-out and L4 vacuous modes are specified.
- C6 direct/cross-family cases, CR15 pins, CR17/18 recorded fixtures, fixed IDs, 14-file envelope, task ownership and no-new-dependency constraint remain covered.
- Reservation/fatal ordering, warning shapes and §7 observability exceptions are explicit.

Limits: read-only specification assessment; no implementation/tests executed or SHA-256 recomputation available. Non-OpenAI escalation remains unavailable, not a verdict substitute. Owner deletion checkpoint remains open. T1/T3 technical clearance is unresolved; repairs require fresh candidate-bound reviews. No approvals or records changed.

### Verdict
REVISE
