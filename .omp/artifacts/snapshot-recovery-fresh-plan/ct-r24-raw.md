Spec ID: snapshot-recovery-spec-r24
Revision: r24
Findings by ID: C5, C8

**C5 — HIGH; lifecycle-hazard; confidence HIGH.**
Anchors: `.scratch/snapshot-recovery/spec.md:69,83` (D12 moved-entry identity and eligibility), `:122` (hard-linked symlink fixtures).

The moved-entry guard conflates distinct hard-linked directory entries. Static counterexample:
- `snapshot.json` is a symlink targeting `snapshot.json`.
- `prompts.json` is a hard link to that symlink: same inode and target text, different directory entry.
- Both pre-pass chains loop. Pass 1 quarantines snapshot as `read_error`; prompts then dangles and its pass no-ops.
- D12 records moves by pre-move **entry identity**. Prompts shares the moved snapshot inode, so the guard incorrectly treats prompts as already moved and excludes reconciliation.

Admission recreates snapshot; prompts now resolves to catalog bytes. The unchanged `SavePrompts` guard rejects them (`store.go:84-126`), and restart quarantines prompts—another C5-class self-healing failure. The ordinary mutual-pair fixture is repaired; this is an adjacent alias instance, not reopening that fixture. The earlier miss’s reason is unknown.

**Minimal amendment:** distinguish moved directory entries by original configured path plus identity, and distinguish partner from self using the recorded chain-entry path—not inode alone. Keep retention protection separate. Add this layout under AC2-9, requiring `[snapshot read_error, prompts orphaned_link]`, preserved link text, fresh regular prompts, and second-run no-op. No new IDs/files are needed.

**C8 — LOW; evidence precision; confidence HIGH.**
Anchor: `spec.md:245` says fixtures are “still passing under the stricter key,” although no implementation exists and §9.6 declares tests planned. Replace with “remain required planned oracles”; do not imply executed GREEN.

**Verified closures**
- CR23 F1/F2: D12/§4 require a live post-pass symlink and exclude self-moved entries in both classes. AC1-10(b-xA2) and AC2-9’s both-moved layout correctly require two recoveries, zero reconciliations, and no missing-source fatal. Universal closure remains incomplete because of C5 above.
- Base C5 mutual pair: one reconciliation, deterministic two-warning order, R3 failures preserved, retention after reconciliation, fresh snapshot/prompts and restart no-op are coherently specified.
- F3–F6 and C4: Readlink/seeded-retention oracles, directory-identity expansion keys, deciding-condition citation and spelling corrections are present; hard-alias finite traversal remains distinguishable from true cycles.
- R→AC→T coverage, T1/T2→T3 dependencies and the 14-file envelope remain. §7 documents collection/metric/alert exceptions; the unalerted consequence matches `logging-contract.md:607-614`.

**Recommendation:** revise T1/T3’s identity contract and oracle, then obtain newly bound reviews. Keep owner checkpoints and requested non-OpenAI escalation open.

**Limits:** static review only; no edits/tests. Registry digests match the supplied manifest, but byte hashes could not be recomputed with available tools. No execution permission is granted.

### Verdict
REVISE