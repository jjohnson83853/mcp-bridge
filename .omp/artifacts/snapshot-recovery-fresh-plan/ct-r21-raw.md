Spec ID: snapshot-recovery-spec-r21
Revision: r21
Findings by ID: C1, C2, C3

**C1 — MAJOR; resolver/protection correctness; HIGH confidence.**
Anchor: `.scratch/snapshot-recovery/spec.md:68,83,122` (D11, §4(3), AC1-10). The unconditional “identity revisit ⇒ cycle” rule rejects finite paths that traverse the same directory symlink twice.

Counterexample: seed five conforming snapshot quarantines, oldest `q5` containing rejected bytes; create directory `deep`, link `alias → deep`, and `snapshot.json → alias/../alias/../q5`. Filesystem resolution reaches `<state>/q5`. The specified pre-pass instead stops at the second `alias` identity, never protecting `q5`. Recovery moves the configured link, then retention deletes `q5`, destroying its backing evidence.

This is a **new C1-class instance**, not failure of the repaired parent-hop fixture; the earlier miss’s reason is unknown. Minimal amendment: distinguish cyclic resolution from finite repeated traversal using resolution state or correctly scoped expansion tracking, retaining the budget. Add an AC1-10 fixture asserting `q5` survives, `q4` is evicted, and quarantine read-through preserves bytes. T1’s complete-chain protection closure is not established.

**C2 — BLOCKING evidence gap; candidate binding; HIGH confidence.**
The assignment requires comparing all twelve inputs against supplied SHA-256 digests. Read tools expose content but no computed digest; available-tool searches found no authorized local hashing/execution capability. Consequently, exact-byte identity remains unverified. Provide independently computed, candidate-bound digest evidence or a read-only hashing capability. This review cannot establish exact-input approval.

**C3 — MINOR; statement precision; HIGH confidence.**
Anchor: `spec.md:69`, D12’s assertion that every other non-quarantine-named final target fails condition (iii). A chain through the other configured symlink can lose that intermediate entry to recovery while ending at `keeper.json`; (iii) then passes. Amend to “fails (iii) when no other chain element moved.” Operative conditions need not change.

**Verified coverage**
- M1’s operative exclusion corrections work: configured targets qualify; b-xB2/b-xB3 satisfy detection; non-conforming retention classification remains separate.
- L1 correctly cites (iv). L2 rejection is supported by `store.go:104–106`, independently confirmed with numbered grep.
- A1/A2 fixtures are present; A3 wording and A4 fatal-list amendment are present. Budget boundary counting remains imprecise.
- R→AC→T mapping, fixed IDs, 14-file union, no-new-dependency constraint, C6/CR15 protections, warning counts, ownership split and §9 mirror warning remain specified. §7 documents collection/alert exceptions and their consequences.

Both owner checkpoints remain **OPEN and unclaimed**. The unavailable non-OpenAI `spec_critic_escalated` reviewer remains an escalation limit, not a substitute verdict. No implementation, tests, mutations or execution permission claimed. Textual correctness requires C1 repair; exact-byte certification additionally lacks C2 evidence.

### Verdict
BLOCKED
