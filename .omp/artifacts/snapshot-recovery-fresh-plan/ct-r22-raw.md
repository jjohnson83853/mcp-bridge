Spec ID: snapshot-recovery-spec-r22
Revision: r22
Findings by ID: C4, C5

**C4 — MAJOR; class=lifecycle-hazard; confidence=HIGH.**
Anchor: `.scratch/snapshot-recovery/spec.md:68,83,122` (D11, protection, AC1-10).
On-stack **inode identity alone** still mistakes finite resolution for a cycle when hard-linked symlinks have different containing directories.

Static counterexample: create directories `d/d`; `snapshot.json→a`; `a→d/l`; make `d/l` a hard link to symlink `a` (same inode and target text); set `d/d/l→../../q5`. With five conforming seeds and corrupt `q5` oldest, filesystem resolution reaches `q5`. D11 instead stops at `d/l`, whose inode matches active `a`, leaving `q5` unprotected. Retention deletes `q5`, destroying backing bytes and dangling the quarantine link.

Linux permits these hard links: [link(2), VERSIONS](https://man7.org/linux/man-pages/man2/link.2.html).
Minimal fix: distinguish active expansions by resolved containing-directory context as well as identity; keep protection membership separate. Add this layout under AC1-10, asserting `q5` survives, `q4` is evicted, and read-through preserves bytes. This is a **new C1-class instance**, not reopening the repaired repeated-alias fixture; the earlier miss’s reason is unknown.

**C5 — MAJOR; class=statement-contradiction; confidence=HIGH.**
Anchor: `spec.md:23,69,83` (R9 versus D12).
For `snapshot.json↔prompts.json`, pass 1 quarantines snapshot as `read_error`; pass 2 sees dangling prompts and no-ops; reconciliation rejects its pre-pass loop class. Admission republishes snapshot, making prompts resolve to catalog bytes. The unchanged `SavePrompts` guard rejects them, preventing prompt healing; restart quarantines prompts again—contradicting R9’s persistence/no-new-quarantine promise.

Source support: `store.go:84-126`, `reload.go:723-740`. Independently confirmed by static tracing; also identified in `cr-r22-raw.md`. Existing defect; earlier miss’s reason unknown.
Minimal fix: reconcile R9/D12 explicitly and pin the mutual-link startup/restart behavior under AC2-9. Narrowing the promised recovery behavior requires affected-owner confirmation, not merely weaker assertions.

**Verified textually**
- Original C1 fixture repaired: alias expansions pop before subsequent parent hops; b-realias/b-revisit require `q5` preservation, `q4` eviction and byte read-through; b-cycle retains true ancestor-cycle termination.
- C3’s moved-chain-element rationale is corrected. D11/AC1-10 agree on 4096 splices, excluding the configured input hop and ordinary components; termination/protection clauses remain specified.
- C7/D12 ordinary reconciliation, C6/CR15 protections, R→AC→T coverage, fixed IDs, 14-file envelope and no-new-dependency constraint remain.
- §7 documents collection/alert exceptions and consequences.

**Limits:** §9.19 consistently records the historical r21 manifest; eleven non-spec digests match r22. Exact-byte recomputation is unavailable, not a spec defect. No runtime/tests executed. Both owner checkpoints remain OPEN and unclaimed; non-OpenAI escalation remains unavailable. T1/T3 correctness closure is incomplete; repairs require newly bound reviews.

### Verdict
REVISE
