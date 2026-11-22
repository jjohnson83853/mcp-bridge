## Code Review Summary

**Mode:** Specification only — candidate `snapshot-recovery-spec-r7@r7`; no implementation exists. **Files Reviewed:** 27 hashed inputs plus ~16 live anchors (`envelope.go`, `log.go`, `log_test.go`, `store.go`, `serve.go`, `main.go`, `wiring.go`, `startup_failure.go`, `loop.go`, `reload.go`, `refresh.go`, `config/command.go`, `config/config.go`, `logging_contract_test.go`, `log_manifest_test.go`, `docs/logging-contract.md`, r6 review artifact, r7 projection). **Review limits:** read-only; no `go test`, `git diff`, LSP, or state-tool execution; no unrun results claimed. **Total Issues:** 6 (CRITICAL 0, HIGH 0, MEDIUM 1, LOW 5).

**r6 closure verified: all 10 findings closed as stated.** H1: `config_envelope_test.go` now in T2 (§6 line 73) and §6.1 read/write sets; pin 129→130 matches the live pin at `config_envelope_test.go:166-172`; `quarantine_path` after `publication_impact` is alphabetically correct in the live list (`envelope.go:668-670`) and in the docs union and `log_test.go` literal. H2: serve returns `stateDirectoryFailure(store.Dir())`; `startup_failure.go:26-33` returns a `startupFailure` value, `main.go:59-68` is the sole writer via `errors.As`. H3: `QuarantineKeep-1` arithmetic is consistent across R5, §4, §8, AC1-3, AC1-9. M1: Recovery appended at rename time. M2: AC1-8's offset literal is parseable RFC3339. All five LOWs closed (trigger named and `loop.go:170-180`/`reload.go:728` confirm the path; risk(4) corrected; ticket example conforming; AC2-6 arm 2; AC1-1 seeded no-prune oracle). Envelope unchanged: 9 R, 19 AC, 3 T, no new/renamed id; §6.1 matches §6 with only the approved `config_envelope_test.go` coverage edit; four specdd markers and r7 section bindings present.

### By Severity
- CRITICAL: 0
- HIGH: 0
- MEDIUM: 1 (blocking class)
- LOW: 5

### Issues

**[MEDIUM][statement-contradiction] The documented `snapshot_retention_failed` remediation cannot be emitted by the specified code path**
File: `.scratch/snapshot-recovery/spec.md` §7 (line 89) and T2 docs edit (1) (§4, line 48); vs §4 envelope edit (a) and cmd wiring (line 47); `internal/observe/envelope.go:92-142`
Confidence: HIGH
Issue: The retention record is emitted with `Error = observe.RuntimeErrorInfo("snapshot_unavailable")`, and `validErrorInfo` requires the entry's `ErrorInfo` to equal the canonical value for that code exactly (`envelope.go:136-142`). T2 fixes `snapshot_unavailable` to the R4 remediation `verify the durable snapshot and storage permissions, then retry`. §7 and the new AD13 row instead document `inspect the state directory; an old quarantine file could not be removed`. Only one of the two can be true: the emitted record carries the R4 text, so the AD13 table and §7 are wrong; or an implementer hand-builds the documented `ErrorInfo`, which `validErrorInfo` rejects, collapsing the record to `logging_write_failed` and failing AC1-5b. No test compares the AD13 remediation column to `RuntimeErrorInfo` (`logging_contract_test.go` checks only messages/operations/fields), so the gate would not catch it. This predates r7 (carried since r5) but remains unresolved in the reviewed candidate.
Fix: Set the new AD13 row and §7 remediation to the shared text `verify the durable snapshot and storage permissions, then retry` (no code, field, id or schema change), and add an `error.remediation` assertion to AC1-5b so the emitted value is pinned.

**[LOW][r-ac-coverage-gap] R4's per-file quantifier is only exercised with one quarantined file**
File: `.scratch/snapshot-recovery/spec.md` R4 / AC2-2 / AC2-7 / AC2-9
Confidence: MEDIUM
Issue: No oracle seeds both `snapshot.json` and `prompts.json` corrupt in one pass and asserts exactly two `startup_snapshot_rejected` records with distinct `quarantine_path` values. AC2-7 exercises the accumulation loop with one `Recovery`; AC1-6b exercises the plural loop only for retention records. A dedupe/keying regression in `emitRecoveryRecords` could drop the second file's warning while every AC passes.
Fix: Add a both-corrupt subcase (no new id) asserting two R4 records discriminating the families by `quarantine_path`.

**[LOW][editorial] §7 calls `quarantine_path` a "state-directory-relative filename"**
File: `.scratch/snapshot-recovery/spec.md` §7 (line 89) vs §4(c) (line 48) and AC2-2
Confidence: HIGH
Issue: §4(c) requires a non-empty **absolute** path with a `.json` suffix; AC2-2 asserts equality with the full quarantine path. §7's "relative filename" wording contradicts both. `config.Parse` resolves a relative `storage.directory` against the config file (`internal/config/config.go:380-381`), so the absolute requirement is satisfiable; only the prose is wrong.
Fix: Reword §7 to "absolute quarantine file path inside the state directory".

**[LOW][editorial] Ticket mirror is stale for the r7 in-place amendments**
File: `.scratch/snapshot-recovery/issues/01-untrusted-snapshot-self-recovery.md:88-113`
Confidence: HIGH
Issue: The header says r7 corrects AC1-1, AC1-8, AC2-6 and AC2-9 only, while spec r7 also amended AC1-3, AC1-9, AC2-4 and AC2-7 in place. The mirrored AC1-9 text retains the pre-r7 ambiguous "survivor count honors `QuarantineKeep` excluding this run's file" phrasing and drops the off-by-one FAILS clause. Spec ids are declared authoritative, so this is advisory, but a ticket-only reader can infer the H3 off-by-one reading.
Fix: Re-mirror AC1-3/AC1-9 (and the amended-id list) from spec r7.

**[LOW][ac-not-falsifiable] AC1-9's "only older pre-existing conforming files were pruned" is ambiguous**
File: `.scratch/snapshot-recovery/spec.md` AC1-9 (line 61)
Confidence: MEDIUM
Issue: The scenario seeds timestamps that "equal and exceed `now`", so a pruned pre-existing file need not be older than the new file; "older" can only mean ordered-oldest among pre-existing. The determinate assertion (`QuarantineKeep-1` pre-existing survivors) still makes the oracle decidable, but the FAILS text does not name the pruning set.
Fix: Replace with "the 4 newest pre-existing conforming names survive; the oldest pre-existing name is removed".

**[LOW][editorial] AC2-5 does not exercise the rename-failure route it names**
File: `.scratch/snapshot-recovery/spec.md` AC2-5 (line 66) vs `cmd/mcp-bridge/serve.go:96-98`
Confidence: MEDIUM
Issue: With the state directory at `0o555`, `storage.Open` usually fails first at the `.writer.lock` create/open (already returning `stateDirectoryFailure` at `:96-98`), so the recovery rename is never attempted. The AC still passes (code `state_directory`, original bytes intact) but for the Open route, not the claimed rename class. AC2-8(b) does cover rename failure deterministically, so verification is not lost.
Fix: State that AC2-5 asserts the Open/state-directory route and that AC2-8(b) is the rename-class oracle.

### Open Questions
- If the distinct retention remediation is genuinely required, the only coherent route is a separate error code with its own `runtimeErrorInfo` entry — a closed-vocabulary widening requiring owner authorization. Confirm that the simpler shared-remediation fix is intended.

### Positive Observations
- Every r6 finding is closed by the reviewer's exact stated fix, with anchors verified against the live tree rather than restated.
- The single-writer fatal route is type-coherent and closes the double-`startup_failed` hazard without touching `main.go`/`startup_failure.go`; AC2-4/AC2-7 pin exactly-one records.
- `durableLoader`/`promptLoader` map `ErrNoSnapshot` to nil (`wiring.go:565-590`), so AC2-2's one-record oracle and AC2-6's field-less retained record are both achievable as written.
- R9 preservation is sound: only `Load`/`LoadPrompts` wrapping changes, and existing callers assert only `errors.Is(err, ErrCorrupt)`.
- AC1-1, AC1-3, AC1-8, AC1-9, AC2-6 arm 2 and AC2-9 are now falsifiable with determinate FAILS clauses; envelope and task boundaries are unchanged.

### Recommendation
Acknowledge the r6 closure and the clean envelope, but the AD13/§7 remediation contradiction is a blocking-class statement contradiction at high confidence and must be reconciled before implementation.

### Recommendation
REQUEST CHANGES