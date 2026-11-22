## Code Review Summary

**Mode:** Specification, `snapshot-recovery-spec-r6@r6` (no implementation exists). **Files Reviewed:** 30 (26 supplied inputs plus `cmd/mcp-bridge/main.go`, `cmd/mcp-bridge/config_envelope_test.go`, `internal/observe/log_manifest_test.go`, frozen r5 review). **Review limits:** read-only; no git diff/LSP/test/state execution; no unrun claims. **Total Issues:** 10 (CRITICAL 0, HIGH 3, MEDIUM 2, LOW 5).

### Issues

**[HIGH][statement-contradiction]** T2 omits `cmd/mcp-bridge/config_envelope_test.go`, which pins `len(EnvelopeFieldNames())==129` and sortedness (`:168-172`). `quarantine_path` makes 130, and insertion "after `protocol_version`" (`envelope.go:667`; next entry is `publication_impact`) is unsorted, so T3's full-suite gate cannot pass inside the §6.1 envelope. Fix: add that file to T2/§6.1, set 130, and insert after `publication_impact` in `envelope.go`, the docs union, and the `log_test.go` literal.

**[HIGH][statement-contradiction]** Double `startup_failed`. §4/T2 call `writeStartupFailure(stderr, stateDirectoryFailure(store.Dir()))` in serve and return the error; `runContext` also writes every returned error (`main.go:59-68`). AC2-4/AC2-7 "one record" therefore fails: duplicate `state_directory`, or `state_directory` plus fallback `internal_error` when the raw fatal is returned. Fix: serve emits accumulated warnings only and returns `stateDirectoryFailure(store.Dir())`; main remains the sole writer.

**[HIGH][statement-contradiction]** Prune off-by-one. §4 step 4 excludes this run's file, then unlinks beyond `QuarantineKeep` among the remainder, leaving 6 when 5 pre-exist. R5 (≤5 per family) and AC1-3/AC1-9 require 5 survivors with the oldest removed. Fix: unlink beyond `QuarantineKeep-1` after exclusion.

**[MEDIUM][statement-contradiction]** §4 step 3 records the `Recovery` only after `syncDirectory` succeeds, but AC2-8(a) requires the result to carry it on a post-rename sync fatal, and R4 requires one warning per quarantined file. Fix: append the Recovery immediately after rename success, before sync.

**[MEDIUM][ac-not-falsifiable]** AC1-8's "offset form" `prompts.corrupt-20261121T120000+02:00.json` is not RFC3339 either (missing separators), so plain `time.Parse` also treats it as foreign; the literal-`Z` rule is unpinned. Fix: `prompts.corrupt-2026-11-21T12:00:00+02:00.json`.

**[LOW][ac-not-falsifiable]** AC2-9's "next prompt save" has no named trigger; startup admission skips `PersistPrompts` (`reload.go:728`), leaving only the async initial refresh (`loop.go:172`). Name/poll it or assert persistence at service level.

**[LOW][editorial]** §9 risk(4) says the retained AC2-6 record "collapses to `logging_write_failed`"; §3 correctly shows wiring.go:485-492 writes via `observe.WriteJSONLine`, not `buildEnvelope`. Correct the note.

**[LOW][editorial]** Ticket example `snapshot.corrupt-20261116T230448Z.json` is not RFC3339Nano-`Z` under R5 and would never be pruned; stale and misleading.

**[LOW][r-ac-coverage-gap]** R8's "any other value enables recovery" is untested; AC2-6 covers only `off`/`OFF`. Add one `enabled`/`1` case.

**[LOW][ac-not-falsifiable]** AC1-1's absent subcase "performs no retention pass" is unobservable unless >`QuarantineKeep` conforming files are seeded; seed and assert unchanged, or drop the clause.

### Open Questions
- Does "after `protocol_version`" mean literal position or alphabetical placement (tests require after `publication_impact`)?
- Should R3's "only output for the condition" be narrowed so R4 warnings for an already-quarantined file before a fatal remain permitted?

### Positive Observations
- §9 ledger matches the frozen r5 review exactly (H1-H3, M4-M9, L(a)-(e) = 3/6/5). H1 is now emittable: `emitRecoveryRecords` uses `Entry.Fields`/`Logger.Log`, `NewEnvelope` is unused, and the `snapshot_unavailable` case is field-equal to wiring.go:485-492 and docs:477.
- R9 is safe: tested callers assert only `errors.Is(err, ErrCorrupt)`.
- R8 default ON is owner-confirmed, `EqualFold` specified, and AC2-6 discrimination holds; §6.1 otherwise matches §6 with no unauthorized widening.

### Recommendation
REQUEST CHANGES