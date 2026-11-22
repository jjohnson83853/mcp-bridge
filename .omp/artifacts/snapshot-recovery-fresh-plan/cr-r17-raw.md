## Code Review Summary

**Mode:** specification; candidate `snapshot-recovery-spec-r17@r17`; no implementation. Reviewed spec.md (requirements, design, ACs, §6.1), ticket mirror, `docs/logging-contract.md:607-614`, and code anchors. Registry SHA-256 comparison is not executable with the effective read-only toolset (no hash/shell) — review limit.

### Closure of r16 findings
- **F1 CLOSED.** R2:16, R5:19, D10:64, §4:78, §7:186, §8:197, AC1-3:110, AC1-10:117 agree: one run-level set, populated before pass 1 from `lstat`/`Readlink`/`Clean` at both configured paths, consulted by both passes. Reconstructed **A** (snapshot→oldest prompts seed): prompts pass skips it, unlinks `now-4s`, prompts total 5, quarantine read-through works. **B** (prompts→oldest snapshot seed, snapshot first): snapshot pass skips it, unlinks `now-4s`, then prompts link still resolves and moves; totals 5/1. No residual unlink/dangle on either layout.
- **F2 CLOSED.** §4:78 candidates = all unprotected pre-existing conforming files, oldest-first, attempted until exactly four pre-existing survive; R5:19, D10:64, §8:197, AC1-3:110 agree; protected file holds a surviving slot; AC1-3's five-seed layout evicts `now-4s`, total 5; AC1-9:116(a2) two-excess evicts two, total 5.
- **F3 CLOSED.** Every live candidate-spec citation is `wiring.go:486-493`; code confirms `case` :486, remediation :492, `}` :493, `:494` next case. Only the non-contractual r15 ticket mirror (:110) retains `486-491`.
- **F4 CLOSED.** §9.7:209 records heading r15 (`:91`), header r7 (`:7`), and line 11 `SNAPREC-SPEC-r3`; ticket lines 7/11 verified.
- **F5 CLOSED.** AC1-3:110 and AC1-10:117 (b-prot, b-xA/B) state the seeded target is store-rejected corrupt, so the link entry is what recovery moves.
- **Regression OK.** R/AC/T ids unchanged; §6.1:171 = 14 files = T1:134 (4) + T2 (6) + T3 (4); no new dependency (`go.mod` unchanged, std-lib); Load/Save/LoadPrompts/SavePrompts/ensureReplaceable behavior preserved (R1, D7, AC1-7; `store.go:85-110,149-198,295-306`); C6 same-family retained (AC1-3:110, AC1-10:117).

### Advisory LOWs
- **[LOW][coverage] One-hop protection.** `Readlink` protects the immediate target, not a chain. If a moved link's target is itself a pre-existing conforming quarantine symlink (the feature creates these at AC1-10(b)), retention can evict the ultimate conforming file: the moved link dangles and its backing bytes are deleted on the success path. R5:19, §4:78. Confidence: HIGH mechanism, MEDIUM reachability. Fix: resolve the chain (`EvalSymlinks`) or state the one-hop limit and narrow "resolved backing evidence".
- **[LOW][statement precision] Alias identity.** The set is lexical path strings; an alternate spelling of the same state directory (absolute alias or symlinked component) cannot match a listing candidate, so the same deletion/dangle consequence is reachable. R5:19, §4:78 ("path identities only"). Confidence: MEDIUM. Fix: compare file identity, or document the alias limit.
- **[LOW][statement precision] Other configured path as target.** A link target equal to the other family's configured path is moved by that family's recovery; the moved/configured link dangles (bytes survive at that quarantine). R2:16's "never touched" is broader than the retention-only rule; §4:78 addresses candidacy only. Confidence: HIGH. Fix: narrow R2's wording or name the interaction.

### Positive
The vacuous-edge table, A/B same-pass ordering assertions, explicit survivor counts, and F5 seeding are coherent, bounded, and testable.

### Limits
Specification mode; no tests exist or were run; no Git/LSP/hash capability; §9.12/§9.13 escalated critic and irreversible-deletion owner checkpoint remain open (admission is separate).

### Recommendation
APPROVE
