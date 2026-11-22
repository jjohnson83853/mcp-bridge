## Code Review Summary

**Mode/pass:** specification only, initial (r14). **Inputs:** 13 (spec, ticket mirror, logging contract, go.mod, 10 code/test anchors). **Issues:** 6 (0 CRITICAL / 0 HIGH / 0 MEDIUM / 6 LOW).

**Closure evidence (checked against source, not the planner's claim):** CR13-01 → R5/§4(3)/AC1-3/AC1-8 both-family seeds + AC1-6 two-family result; CR13-02 → R3/D6/§4(2)/AC2-8(b-res)/AC1-9(b); CR13-03 → R7 carve-out + AC2-6 arm 1; CR13-04 → D2/§4/AC2-7 fatal-first (zero records, prompts untouched); CR13-05/08 → AC1-6 pins level/operation/outcome/code/kind/retryable/remediation/message, D9 literal; CR13-06 → T2 quotes `wiring.go:486-491` and `startup_failure.go:26-33` (both verified); CR13-09/OQ-1/OQ-2 → AC1-7, AC2-2 reason table, R5 "while more than four remain", AC2-6 arm 2 `""`. **Envelope/ids:** §6.1 write set (14) equals the §6 union exactly; R1–R9, AC1-1..10, AC2-1..9, T1–T3 unchanged vs the r13 artifact; `schema_version` 1. Verified no out-of-envelope test edit is needed: `logging_contract_test.go`'s "129-pin" is the pin-ceiling case and only `envelope-fields`/`config-codes` marked lists are code-compared (`:94-119`).

### Issues

**[LOW][verifiability][Conf: HIGH] Stale `validErrorInfo` anchor (6 occurrences)**
File: spec §3, §4, AC1-6, §7, §9.10, T2 inputs; `internal/observe/envelope.go`.
Issue: cited `envelope.go:200-206` is the tail of `validEnvelopeOutcome`; `validErrorInfo` is at `envelope.go:136-141`. The exact-equality claim is true there.
Fix: replace `:200-206` with `:136-141` everywhere.

**[LOW][coverage][Conf: HIGH] AC1-6 omits `resource.path`**
Anchor: AC1-6 vs §4 `emitRecoveryRecords` (`resource.path: stateDir`) and §7 "same path fields".
Issue: the "full shape" enumeration pins every field except `resource.path`; a wrong or absent directory value passes all named oracles.
Fix: add `resource.path`=configured state directory to AC1-6's assertion set.

**[LOW][provenance][Conf: HIGH] §9.7 misstates the ticket**
Anchor: spec §9.7 vs `.scratch/snapshot-recovery/issues/01-untrusted-snapshot-self-recovery.md`.
Issue: it claims the AC heading reads "mirror of spec r13" and orders a re-sync; the supplied ticket reads "mirror of spec r14" and its AC body already carries r14 text (cross-family seeds, `""` enable, fatal-first, b-res). The r7-prose/no-sha256 note is correct.
Fix: state the actual mirror revision; drop the re-sync instruction.

**[LOW][coverage][Conf: HIGH] R5 tie-break unoracled**
Anchor: R5/§4(3) "ties broken by full name ascending" vs AC1-3/AC1-9 (distinct timestamps only).
Issue: a wrong tie-break passes every listed oracle.
Fix: seed two conforming names sharing one parsed timestamp; assert the name-ascending survivor.

**[LOW][statement-contradiction][Conf: MEDIUM] R2 omits stat-error entries**
Anchor: R2 ("regular file or symlink resolving to one"; "non-regular entries … never renamed") vs §4(2)/R6/AC2-4 loop subcase.
Issue: an ELOOP entry is neither in R2's quarantine set nor covered by its never-rename sentence, yet AC2-4 requires quarantine of the link entry.
Fix: add "or an entry whose stat fails with a recoverable stat error (e.g. ELOOP)" to R2.

**[LOW][traceability][Conf: HIGH] AC1-6 emitter half is T3-owned but unlisted**
Anchor: T2 "Deliverable + ACs … AC1-6"; T3 Expected files `snapshot_recovery_test.go` (`TestEmitRecoveryRecords`) vs T3 deliverable "AC2-1..AC2-7, AC2-9"; §7 says T2/T3.
Fix: add AC1-6's cmd emitter half to T3's deliverable, or scope T2's claim to the envelope-level case.

### Positive Observations
- Family-scoped retention is stated once and mirrored in §4(3), AC1-3/1-6/1-8, §8(2), with both-family oracles that detect count leaks as well as cross-family unlinks.
- `fs.ErrExist`-vs-fatal is consistent across R3/D6/§4(2)/AC2-8(b-res)/AC1-9(b); the 4096-candidate exhaustion oracle is exact.
- R7 now preserves the Durable-loader record; AC2-6 arm 1 asserts the disabled arm emits neither enriched nor retention records.
- Stop-on-first-fatal has a real negative oracle (fatal-first: prompts untouched, zero warn records).
- Producer literals are quoted from real source; the off-arm and `state_directory` remediations match the code, and the fixed-message pin post-dates the generic default.

**Review limits:** no shell/hash or write tool in this profile — registry SHA-256/section-binding hashes were not recomputed (reviewed the supplied spec/ticket bytes); no `go test`/spec-lint run; no implementation exists, so all RED/GREEN clauses remain planned oracles.

### Recommendation
APPROVE