## Code Review Summary

**Mode:** specification (repair pass), `snapshot-recovery-spec-r21@r21`; no implementation. **Limits:** no hash/shell tool, so the supplied SHA-256 digests were not recomputed; no tests or spec-lint run; the ticket mirror is excluded as contract (stale by instruction); prior-round raw reviews used as evidence only.

### Closure
- **M1 CLOSED.** `spec.md:16` (R2) drops "non-conforming" from the reconciliation exclusion, states both configured paths are eligible orphan targets by definition, and uses **NOT reconciled** while keeping only dangling untouched by the passes (loop/read-through moves and directory fatal restated). `spec.md:69` (D12): (ii) admits configured-path final targets, (iii) is the moved-element gate, (iv) requires the post-pass symlink entry plus `ErrNotExist`. `spec.md:83` (§4(3)): non-conforming is "a protection/retention class only — never a reconciliation-eligibility exclusion". Re-ran `(b-xB2)/(b-xB3)` (`:122`): both fire — (ii) passes (pre-pass chain ended at the existing regular `snapshot.json`/`prompts.json`), (iii) passes (moved by the other family's pass), (iv) passes (link still a symlink, `Stat` → `ENOENT`). **No condition vacuous:** (iii) still excludes never-moved targets; (ii) still excludes loop/escape/directory pre-pass classes (e.g. a cycle routed through the other configured path, where pass 2 moves it but the pre-pass class is cycle); (iv) is what excludes `(b-xA2)`.
- **L1 CLOSED.** `spec.md:122` (`b-xA2`): "(iii) PASSES … the failure is condition (iv)".
- **L2 REJECTION ACCEPTED.** Raw `internal/storage/store.go`: `:104` `ChecksumValue` call, `:105` guard, `:106` sentinel return, `:108` `promptTrusted`, `:185` catalog sentinel. §3/R6/AC1-2 anchors (`:104-106`, `:95-99`, `:101-102`) are correct; the r20 reviewer's `:103-105` was off by one.
- **A1–A4 CLOSED.** `(b-budget)`/`(b-mid)` seeded oracles at `:122`; `(b-xB-res)` post-reconciliation retention with 5 pre-existing seeds at `:122`; AC1-3 alias case (`:115`) now uses entry identity, no lexical-inequality wording; D12 (`:69`) names post-rename directory-sync failure and delegates to R3.
- **Regression OK.** R1–R9, AC1-1..10, AC2-1..9, T1–T3 unchanged; §6.1 is the same 14-file union (T1 4 + T2 6 + T3 4, `:141-146`); std-lib only, no `go.mod` change; Load/Save/LoadPrompts/SavePrompts/ensureReplaceable unchanged (R1/§4/T1 prohibitions); C6/CR15-02 retained; D11 component resolver retained; §9.11/§8(2) owner checkpoints still OPEN and unclaimed.

### Findings
**[LOW][statement-precision]** D12's rationale overgeneralizes (iii).
File: `.scratch/snapshot-recovery/spec.md:69`. Confidence: HIGH.
Text says a chain ending at "any other non-quarantine-named regular target still fails (iii)". Counterexample: `snapshot.json`(link) → `prompts.json`(link) → `keeper.json` (non-conforming regular); the prompts pass moves `prompts.json`, so (iii) passes and reconciliation is in fact correct (the chain was severed by this run). Operative (i)–(iv) are sound and no pinned oracle depends on this gloss, so consequence is limited to an implementer possibly adding a name-based extra gate.
Fix: state that such a chain fails (iii) only when no chain element other than the configured link moved — §9 item 17 (`:224`) already states it correctly.

### Advisory
**[LOW][editorial]** `(b-budget)` visited-set boundary.
File: `.scratch/snapshot-recovery/spec.md:122`. Confidence: LOW.
The fixture claims `k1..k4096` are recorded after a 4097-link chain; the exact boundary depends on whether the configured link's own splice counts against the 4096 budget. No FAILS clause asserts that set (the `k*` entries are vacuous protection), so this is non-blocking. Fix: state the budget counting rule or drop the boundary claim.

### Positive
Oracles are outcome-based (bytes, survivor/deletion counts, stream order) rather than mechanism-based; reconciliation result order is pinned; over-trigger is ruled out observably by `(b-xA2)`'s (iv) and by `(b-vac)`.

### Recommendation
APPROVE
