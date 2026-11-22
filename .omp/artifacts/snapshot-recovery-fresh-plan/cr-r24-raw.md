## Code Review — SNAPREC-SPEC-r24 (specification mode; repair pass closing CR23 F1–F6)

**Findings**

- **[LOW][editorial]** `spec.md:228` (confidence HIGH) — §9.21's review-target text says "zero `orphaned_label` records"; that reason value does not exist (R6's closed set has `orphaned_link`), so the reviewer checklist names a non-existent token. Fix: replace `orphaned_label` with `orphaned_link`.
- **[LOW][statement-precision]** `spec.md:122` AC1-10(b-dangle) (confidence MEDIUM) — the mutual no-fire on a pre-pass dangling chain cites only (m-iii); the deciding condition is (m-ii) (chain must be loop/stat-error class), the same omission CR23-F5 corrected in AC1-1(d). Fix: cite "(m-ii)+(m-iii)".

**Closure evidence**

- **F1 CLOSED.** §4(3), R2, R6, D12(m-iv), §8(2) now require the configured entry to still exist post-passes as a live `lstat` symlink and `Stat` `ErrNotExist`; every residual "or is absent" hit is historical narration or an explicitly rejected alternative (grep-verified). Absence is stated as a no-op (no fatal, no record, no mutation), and AC1-10(b-xA2) now pins exactly two `Recovery` entries with (iii) passing and (iv) failing — no third reconciliation, no `ENOENT` fatal.
- **F2 CLOSED.** D12 applies the same moved-entry guard to the mutual class and states the counting rule (≤1 reconciliation/run; partner alone; ZERO when both configured entries moved). AC2-9's both-moved variant asserts two `read_error` records, zero `orphaned_link`, zero `startup_failed`, fresh publishes, and second-run no-op; R3 fatal discipline for reconciliation failures is unchanged and unreachable there.
- **F3 CLOSED.** AC2-9's orphaned-link and mutual subcases add `Readlink(<quarantine>) == "snapshot.json"` (no target read), and the seeded mutual-class retention variant asserts exactly one eviction (`now-5s`) and 5 surviving prompts-family files — both are executable fixtures with explicit FAILS clauses.
- **F4 CLOSED.** D11 pins the key context to the containing directory's own `lstat` identity (device+inode), path spelling as tie-break/fallback, and names the path-string alias misrouting as harmless/unoracled; C4's (b-hardalias)/(b-hardalias2)/(b-hardcycle) reasoning remains coherent under the identity key.
- **F5 CLOSED** (AC1-1(d) cites (m-ii)+(m-iii)). **F6 CLOSED** — no `non-mutal` remains.

**Regression checks:** ids R1–R9, D1–D12, AC1-1..AC1-10, AC2-1..AC2-9, T1–T3 unchanged; §6.1 union is the same 14 files (4+6+4, no new write path); std-lib only (`os.Lstat`/`Readlink`/`Link`, `syscall.Mkfifo`; `go.mod` go 1.25.0, no new require); spot-verified anchors `store.go:104/112/124/149/295`, `wiring.go:486`, `startup_failure.go:26-33`, `envelope.go:103/136/244/575`, `log.go:228`, `snapshot.go:111`, `logging-contract.md:607/609/614`; mutual-link direction confirmed in `round-12-seed.json` and no confirmation claimed beyond it (§8(2), §9.11 keep retention-unlink and the non-mutual carve-out OPEN); no review, approval, or test-execution claim.

**Review limits:** no hashing tool — supplied SHA-256 identity unverified (same capability gap as C2). No tests run (no implementation). Supplied `spec-lint-r24.out` shows the same 20 pre-existing mirror-drift/shared-oracle findings as r23 — no new lint finding.

## Verdict: APPROVE