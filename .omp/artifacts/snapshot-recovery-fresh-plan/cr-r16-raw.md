## Code Review Summary
Mode: specification; candidate `snapshot-recovery-spec-r16@r16`. Reviewed bytes: `.scratch/snapshot-recovery/spec.md` as read this session. No registry digest was supplied → hash identity unverified (limit). Issues: 1 HIGH, 1 MEDIUM, 3 LOW.

### Closure
- **C6 same-family: CLOSED** — R2:16 / R5:19 / D10:64 / §4:78 / §8:198 / AC1-3:110 / AC1-10:117 agree: resolved target excluded, oldest-first skips to the next-oldest unprotected, bound stays 5 (protected file holds a slot), target bytes + read-through + survivor counts pinned (`now-4s` unlinked). Cross-family scope gap → F1.
- **CR15-01 CLOSED**: ticket:91 heading/body r15 (tie-break, C5/D10 pins, `resource.path`), ticket:7 r7 header, no sha256; §9.7:211 lag consistent.
- **CR15-02 CLOSED**: T1:139 (AC1-1..5,7..10; AC1-6 storage half; AC2-8), T3:159 (AC1-6 cmd half; AC2-1..7,9), T2:151 (AC1-6 envelope half); disjoint files; §6.1:171 14 = 4+6+4.
- **CR15-03 CLOSED**: wiring.go:486 case / :492 remediation, log.go:228 func / :275 default; consistent at every citation; r15 stale values appear only as history.
- **CR15-04 CLOSED**: AC2-2:119 four-field shape; T2:148 RED pins both `ErrorInfo` shapes; envelope.go:136-141 exact equality.
- **CR15-05 CLOSED**: AC1-3:110, AC1-9:116(a2): 6 seeds → two evictions, total 5, third unlink = FAIL.
- **Regression OK**: all R/AC/T ids intact; no new dependency; Load/Save/LoadPrompts/SavePrompts/ensureReplaceable unchanged.

### Issues
**[HIGH][irreversible-deletion] Cross-family symlink target is unprotected**
spec.md:16,19,78,198 | Confidence HIGH
R2/§8 promise any symlinked target is never touched; R5/§4 protect only inside the moved family's pass and assume a foreign target "was never a candidate". (A) `snapshot.json`→pre-existing `prompts.corrupt-X`, both families corrupt: the snapshot pass protects nothing (foreign); the prompts pass evicts X oldest-first → `snapshot.corrupt-<now>.json` dangles and the rejected bytes are gone on the success path. (B) `prompts.json`→`snapshot.corrupt-X` beyond the newest four: the snapshot pass (runs first) unlinks X before the prompts family is examined → link dangles, `Stat` ⇒ `ErrNotExist` ⇒ silent no-recovery. AC1-3/AC1-8/AC1-10 pin same-family targets only.
Fix: run-level protected-target set consulted by both passes (including the unmoved link when the other family runs first); add a cross-family AC subcase.

**[MEDIUM][statement-contradiction] §4(3) candidate set contradicts R5/AC1-3**
spec.md:78 vs 19,110 | Confidence HIGH
Candidates are "pre-existing files beyond the 4 newest slots", then minus the protected file: with 5 seeds and the protected oldest (AC1-3's own layout) the set is empty → no eviction → 6 retained, contradicting R5 ("protected file occupies one of the four slots") and AC1-3 (`now-4s` unlinked, total 5). The same sentence's "until exactly four pre-existing remain" and §8:198 state the correct rule.
Fix: define candidates as all unprotected pre-existing files, oldest-first, attempted until four pre-existing survive.

### Low advisories
- [LOW] `wiring.go:486-494`: `:494` is the next `case`; the block is 486-493 (`:486`/`:492` exact).
- [LOW] §9.7 omits ticket:11's second stale reference `SNAPREC-SPEC-r3` (heading r15, header r7); mirror is non-contractual.
- [LOW] AC1-3 symlink subcase and AC1-10(b-prot) never state the seeded target must be rejected; "exactly one Recovery" only implies it.

### Limits
No digest supplied; specification mode, no implementation/tests exist and none were run; §9.12's escalated critic and irreversible-deletion owner checkpoint remain open (admission is separate).

### Recommendation
REQUEST CHANGES
