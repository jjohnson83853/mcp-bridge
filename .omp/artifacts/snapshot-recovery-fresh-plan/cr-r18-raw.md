## Code Review Summary

**Mode:** specification; candidate `snapshot-recovery-spec-r18@r18`; no implementation.
**Files Reviewed:** 6 source anchors + spec/ticket/docs. **Issues:** 6 (2 MEDIUM, 4 LOW).
**Limits:** registry SHA-256 comparison not executable (no hash/shell); no tests exist or were run; collector delivery out of scope.

### Closure of r17 (independently verified)
- **C1 CLOSED** for the recorded counterexample. R2:16, R5:19, D10:64, §4:78, §8:197 populate one run-level set from both configured paths before pass 1. AC1-10(b-chain):117 realizes q1..q5 with q1→q5, `snapshot.json`→q1: q1(lstat)+q5(Stat) protected, q4 unlinked, survivors = this run's moved link + q1,q2,q3,q5 = 5, q5 bytes intact, read-through moved→q1→q5; the moved entry is the link itself (no canonicalization). (b-chain2) pins s1,s2,s5 and evicts s4.
- **C2 CLOSED.** No cap in R5/D10/§4(3)/§8(2); nine-file backlog = exactly five deletions, four pre-existing survivors + this run's file; bound on survivors only. All five agree.
- **Alias identity CLOSED:** dev+inode on both sides with lexical fallback (R2/R5/D10/§4/§8); AC1-3 alias subcase. **Move-through** named in R2/R5/§4/§8, owning family reports only, R2 narrowed to "never unlinked … by retention".
- **Regression OK:** R1–R9/AC1-1..10/AC2-1..9/T1–T3 unchanged; §6.1 = 14 files = T1(4)+T2(6)+T3(4); `go.mod` unchanged (std-lib `os.SameFile`/`Mkfifo`); Load/Save/LoadPrompts/SavePrompts/ensureReplaceable preserved (R1/D7/AC1-7); C6 same-family retained (AC1-3, AC1-10 b-prot).
- Dangling and alias modes terminate harmlessly; ELOOP/cycle does not (F1); non-conforming/directory targets are vacuous but unoracled (L4).

### Findings
**F1 [MEDIUM][statement-contradiction]** Stat-failing protected links cannot match. R5:19, §4:78 store intermediate links by `lstat` identity, but candidacy compares the candidate's `Stat` identity, falling back lexically only "when its `Stat` fails"; a cycle entry's `Stat` fails `ELOOP` and no fallback path was recorded for it, so c1/c2 are unprotected — contradicting R5's "visited entries still protected" and AC1-10(b-eloop):117 ("skips BOTH cycle entries as protected"). Confidence HIGH (text); consequence is dangling/evidence loss, not rejected bytes. The oracle is insensitive (one eviction always lands on c5). Fix: record every visited link entry's cleaned path as a fallback (or match by entry identity) and re-seed the cycle as the oldest entries.

**F2 [MEDIUM][correctness, residual C1-class deletion]** Relative targets are joined "onto the state directory (the entry's containing directory)" (R5:19, §4:78). That held for r17's single hop but not for intermediate links. `/s/snapshot.json`→`sub/l1`; `/s/sub/l1`→`../snapshot.corrupt-q5.json`: correct target is `/s/snapshot.corrupt-q5.json`, but the stated join yields an out-of-directory path → vacuous protection → q5 evicted, moved link dangles, only rejected bytes lost. Confidence HIGH (text); reachability needs a subdirectory link, which §4(3) explicitly admits into the walk. Fix: join on `filepath.Dir` of the link just read. Repair-induced.

**L1 [LOW][coverage]** Candidate `Stat`-identity matching over-protects any conforming symlink resolving into a protected target, so the success path can exceed `QuarantineKeep`, contradicting R5:19's ordinary-case bound. Fix: match by entry identity.

**L2 [LOW][coverage]** The configured-path move-through (R2:16, §4:78, §8:197) has no AC oracle; b-xA/b-xB use conforming targets.

**L3 [LOW][completeness]** The >4-protected carve-out is stated, bounded and consistent across R5/D10/§4/§8, but no AC exercises it; in the mixed case exhaustion deletes every unprotected pre-existing file though the bound cannot be restored — §8(2) and the §9.11:213 owner flag omit that breadth.

**L4 [LOW][coverage]** "Non-conforming final target" and "directory final target" modes have no oracle (mechanically vacuous).

### Positive
The vacuous-edge table, explicit survivor/deletion arithmetic, F5 seeding, and same-pass ordering assertions are coherent and testable.

### Recommendation
REQUEST CHANGES
