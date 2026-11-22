## Code Review Summary

**Mode:** specification only — `snapshot-recovery-spec-r9@r9`; no implementation exists. **Files Reviewed:** 13 (spec incl. four specdd regions + §6.1, ticket, r8 review as evidence, envelope.go, store.go, snapshot_v1.go, config_envelope_test.go, serve.go, main.go, startup_failure.go, wiring.go, reload.go, loop.go). **Limits:** read-only; no tests, `git diff`, LSP, or registry-hash verification; ticket read at review time may post-date binding.

**r8 closure:** all five items amended as prescribed; §9 ledger maps each with its class; envelope ids unchanged (R1–R9, AC1-1..10, AC2-1..9, T1–T3); §6.1 write set equals T1/T2 and excludes snapshot_v1.go; all four markers present.

### By Severity
- CRITICAL: 0
- HIGH: 0
- MEDIUM: 2
- LOW: 4

### Issues

**[MEDIUM][r-ac-coverage-gap][HIGH] Reservation-exhaustion fatal has no oracle.**
Anchor: R2 "bounded, exhaustion fatal", R3 r9 amendment, §4 step 3, §7 vs AC2-8 (only a/b), AC1-9(b) (one collision), T3 schedule. Confidence: HIGH.
Issue: an unbounded retry loop, a post-budget overwrite, a leaked reservation, or a non-fatal return after 4096 collisions passes every AC; §4's "covers every R with distinct oracles" is false.
Fix: add AC2-8(c), no new id — fixed clock, pre-seed 4096 conforming names at now+0..now+4095ns plus corrupt snapshot with known bytes; assert `errors.Is(err, ErrSnapshotUnrecoverable)`, empty `Recoveries`, original bytes intact, family count 4096; add the T3 schedule.

**[MEDIUM][lifecycle-hazard][HIGH] Lstat-first fatal contradicts R6 for a symlinked-but-valid snapshot.**
Anchor: R6 vs R3/§4 step 1. Confidence: HIGH.
Issue: `os.ReadFile` follows symlinks (today starts fine), while Lstat reports `ModeSymlink` → fatal before Load, so such a deployment fatals every restart with remediation "fix its permissions"; no AC pins Lstat vs Stat (a directory passes either). Carried from earlier revisions.
Fix: state the "regular file" precondition in R6/R3/step 1 and add a symlink subcase to AC2-4 (no new id), or use Stat and admit symlinked regular targets.

**[LOW][statement-contradiction][HIGH] §3 r9 AUTO-DECISION arithmetic is wrong.** "4096-advance loop (≈41 days of 1ns collisions)" — 4096 attempts × 1ns ≈ 4.1µs, so "practically unexhaustable" is false and 4096 empty pre-created names force the fatal. Fix: correct the estimate; either raise the bound or record reachability as accepted and oracled (Issue 1).

**[LOW][ticket-mirror-drift][HIGH] §10 says the r9 delta is "pending application", but ticket:91/93/99/103/106/109 already carry r9 text, so its REPLACE sources (e.g. "mirror of spec r8") no longer exist; ticket:96 omits the r9 schema-1 declaration though ticket:93 claims r9 amended AC1-2; ticket:103 omits AC1-9(b)(iv).** Fix: record the r9 delta APPLIED with byte-equality verification and complete those two mirror lines.

**[LOW][stale-citation][HIGH] store.go anchors are stale.** Load is 149-195 (cited 124-167); schema-2 checksum producer is 184-186 (cited "152-155", which lands in probe/switch); legacy wrap 164-169; default 187-189; Validate 190-192; ensureReplaceable 295-306. §4's normative "sole producer at 152-155" mispoints. Fix: re-derive the anchors.

**[LOW][binding-gap][HIGH] `snapshot_v1.go:135/:150` is decisive evidence for the schema-1 declaration but is absent from §6.1 read paths and the packet's reviewed inputs.** Fix: add it as a read path or mark the citation planner-only (no write-set change).

### Open Questions
- **[LOW]** prompts-family collision has no oracle (AC1-9(b) is snapshot-only) though R7 claims identical naming; the shared T1 helper mitigates. Confidence: LOW.
- **[LOW]** "never overwrite absolutely" assumes no same-user unlink of the reservation between O_EXCL and rename; the writer flock plus 0o700 state dir mitigate. Confidence: LOW.
- **[LOW]** §8's "at most QuarantineKeep-1 survive per pass" holds only when that pass's unlinks succeed; AC1-5 keeps reported beyond-bound files. Confidence: MEDIUM.

### Positive Observations
- Reserve-then-rename is the stronger offered fix and is correctly sequenced: O_EXCL before rename, deterministic 1ns advance, original untouched on exhaustion, reservation unlinked on rename failure; AC1-9(b) states the seed count and asserts distinct paths with both byte sets — genuinely falsifiable.
- §8 uses the reviewer's verbatim wording, consistent with R5/step 4; §9 risk(3) names AC1-5/AC1-6.
- Schema-1 closure is sound: `decodeSnapshotV1` (135, checksum 150) returns plain errors; the store.go:164-169 call-site wrap preserves `errors.Is(err, ErrCorrupt)` without envelope widening.
- T2 anchors verified: `addEnvelopeFields` unknown-key default `false` (so the `quarantine_path` case is required), `failure_reason` case 361 / `validFailureReason` 220, `resource.path` case, pin 129 at config_envelope_test.go:166-172, `publication_impact`→`refresh_id` insertion point, `stateDirectoryFailure` value type, `runContext` sole writer.

### Recommendation
REQUEST CHANGES