## Code Review Summary

**Mode:** specification only — `snapshot-recovery-spec-r10@r10`; no implementation exists.
**Files Reviewed:** 28 bound packet inputs (opened or anchor-verified; `go.mod`, `config.go`, `metrics.go`, `edge_test.go`, `runtime_edge_test.go`, `serve_edge_test.go`, `startup_failure_test.go`, `store_edge_test.go` were not decisive), plus the r9 review artifact, `review-findings-taxonomy.md`, W09E, the r10 registration script and review registry.

**r9 closure (verified against the candidate bytes):**
- CR9-1 MEDIUM `r-ac-coverage-gap` — **CLOSED**. AC2-8(c) exists in place, no new id; fixed clock, exactly 4096 seeds at now+0..now+4095ns, corrupt snapshot with known bytes; asserts `errors.Is(err, ErrSnapshotUnrecoverable)`, empty `Recoveries`, original bytes intact, family count 4096; T3 carries the schedule; §3 arithmetic is now ≈4.1 µs and the reachability is accepted/oracled. The 4096-attempt bound in §4 step 3 matches the seed set exactly, so an off-by-one escape is detectable.
- CR9-2 MEDIUM `lifecycle-hazard` — **CLOSED as stated**: R3/R6/§4 step 1 are `os.Stat` on the resolved target; a symlinked regular file is admitted; a symlink-to-directory is fatal; AC1-1(c) and AC2-4's subcase are in place without new ids; the Lstat-first fatal is gone (only historical references remain). Residual branches of the same decision tree are Issues 1–2.
- CR9-3 LOW `statement-contradiction` — **CLOSED** (correct arithmetic; reachability admitted).
- CR9-4 LOW `ticket-mirror-drift` — **CLOSED in substance**: ticket lines 91/93/95/96/103/108/112 are r10-complete, including AC1-2's schema-1 declaration (line 96) and AC1-9(b)(iv) (line 103). Only the spec's status text is stale (Issue 8).
- CR9-5 LOW `stale-citation` — **CLOSED**: every re-derived `store.go`/`snapshot_v1.go` anchor resolves (Load 149; wraps 161/168/175/178/182/185/188/191; `ensureReplaceable` 295/wrap 305; `syncDirectory` 309; `decodeSnapshotV1` 135/`schema-1 checksum mismatch` 150).
- CR9-6 LOW `binding-gap` — **CLOSED**: `snapshot_v1.go` is a §6.1 read path; the write set is byte-identical to T1/T2; the packet includes it.
- OQ1/OQ2 accepted residuals; OQ3 §8 narrowed to the success condition with per-family failure reporting. No R/AC/T id, no write path, and no §6.1 scope was added or removed; the four specdd markers are present.

**Limits:** read-only; no tests, `git diff`, LSP, `spec-lint`, or SHA-256 computation was available, so registry-hash equality of the reviewed bytes is unverified and the ticket was read at review time. The prompt's “owner-authorized r10/r11 planning budget” is not evidenced by any bound input or the registry entry (`maxRounds` 4, zero r10 rounds used); I cannot verify it, and the spec does not assert it.

### By Severity
- CRITICAL: 0
- HIGH: 0
- MEDIUM: 3
- LOW: 5

### Issues

**[MEDIUM][r-ac-coverage-gap][HIGH] The r10 symlink decision tree is admitted but only its two convenient branches are oracled.**
Anchor: R2, R3, R6, §4 step 1 Note vs AC1-1(c), AC2-4 subcase and AC1-10. Confidence: HIGH.
Issue: R3/§4 step 1 state that a symlink to a regular file is treated as that file and that the rename moves the **link entry** (no canonicalization, target untouched). AC1-1(c) covers only a *valid* link (no-op); AC2-4 covers link→directory (fatal); every byte/rename oracle (AC1-9, AC1-10) seeds direct files. Two stated behaviors are unoracled: (i) a link to a **corrupt** regular target — no AC asserts one `Recovery`, that the quarantine path is the moved link, that the target survives at its original path with its exact bytes, and that the configured path is absent; (ii) a **broken** link, which R6 declares the absent case (today starts fine) — no AC asserts zero recoveries and an untouched link entry. A canonicalizing implementation that renames the resolved target (relocating the operator's file) or a fatal-on-dangling-link implementation passes every r10 AC, contradicting the §7 non-goal and the r9 concern the Stat remedy was meant to close.
Fix: add in-place subcases (no new id): AC1-10 gains “corrupt symlinked regular snapshot” (one Recovery, link entry moved, target bytes exactly preserved at the target path, configured path absent); AC1-1 gains “broken symlink” (zero recoveries, link entry untouched, no quarantine/prune).

**[MEDIUM][statement-contradiction][HIGH] §4 step 1 has no branch for an `os.Stat` error other than `ErrNotExist`, while §4 step 2 assumes regularity was proven.**
Anchor: §4 step 1 vs §4 step 2 “regular file already proven by step 1”; R3 fatal enumeration. Confidence: HIGH.
Issue: step 1 defines only `ErrNotExist → skip` and `resolved mode non-regular → fatal`. A concrete input — `snapshot.json` is a symlink loop (`ELOOP`) or any EACCES/EIO/EIO-on-Lookup — returns an error with no `FileInfo`; the mode check cannot run and step 2's precondition is false. The spec is silent on whether this is fatal, `read_error`+quarantine, or a nil-`FileInfo` panic; no AC and no seam pins it, and R3's fatal class (non-regular, rename failure, sync failure) does not list it.
Fix: amend R3/§4 step 1 in place: any `os.Stat` error other than `fs.ErrNotExist` is fatal wrapping `ErrSnapshotUnrecoverable` with the path (same `state_directory` route); extend AC2-4's harness with a symlink-loop subcase (no new id) asserting one `startup_failed/state_directory` record and an untouched link.

**[MEDIUM][r-ac-coverage-gap][HIGH] The reservation's non-`EEXIST` failure is unclassified and unoracled on the never-overwrite path.**
Anchor: §4 step 3 vs R2/R3; AC2-8(b)/(c). Confidence: HIGH.
Issue: step 3 defines `EEXIST → +1ns` and exhaustion → fatal, but not `os.OpenFile` failures such as `ENOSPC`, `EACCES`, `EMFILE`, or `EROFS`. No rule says the original must remain untouched and no rename may occur; AC2-8(b) injects a *rename* failure and AC2-8(c) injects `EEXIST` exhaustion, so neither covers it. An implementation could retry the whole walk, return a bare OS error, or (if it ignores the reservation result) attempt the rename without a successful reservation — the last directly threatens R2's invariant.
Fix: state in §4 step 3 and R3 in place that any reservation error other than an already-exists error is fatal wrapping `ErrSnapshotUnrecoverable`, with the original untouched and no rename; extend AC2-8(b) (no new id) with a reservation-error seam asserting fatal + original bytes + no quarantine file + empty result.

**[LOW][r-ac-coverage-gap][HIGH] R7's “same read_error/rename/sync handling” for prompts has no prompts oracle.**
Anchor: R7 vs AC1-4, AC1-8, AC2-7, AC2-9; AC1-10 and AC2-8(a)/(b) are snapshot-only. Confidence: HIGH.
Issue: R7 explicitly requires identical prompts handling for read failure, rename failure, and post-rename sync failure. The prompts ACs pin corruption, retention, the fatal non-regular case, and startup success, but not a prompts read-seam failure or a prompts rename/sync fatal. OQ1's “one family-parameterized code path” mitigates but was ruled only for collision naming.
Fix: extend AC1-10 with a prompts read_error subcase and state that AC2-8(a)/(b) execute for both families (or add a prompts seam case), no new id.

**[LOW][r-ac-coverage-gap][HIGH] AC2-6 arm 1 does not assert R8's “no `snapshot_retention_failed` record”.**
Anchor: R8 vs AC2-6. Confidence: HIGH.
Issue: R8 forbids a retention record when recovery is disabled; AC2-6 arm 1 asserts no rename, family count 0, the wedge, and exactly one retained `startup_snapshot_rejected` record, but counts no `snapshot_retention_failed` records. A stray emitter record would pass.
Fix: add “zero `snapshot_retention_failed` records” to AC2-6 arm 1's verify list.

**[LOW][r-ac-coverage-gap][MEDIUM] A retention directory-listing failure is unclassified and has no representable retention record.**
Anchor: §4 step 4 “list entries”, R5, §8 “never silent” vs AC1-5/AC1-6. Confidence: MEDIUM.
Issue: only unlink failures are defined; an `os.ReadDir` failure has no rule or seam, and R5's `quarantine_path` (“first unremoved file of that family”) has no valid value when nothing was listed. The family could be silently unreported, contradicting §8.
Fix: define listing failure as non-fatal for that family with a deterministic `quarantine_path` (e.g., this run's just-created file) or as fatal; add it to AC1-5's injected-seam matrix.

**[LOW][stale-citation][HIGH] `addEnvelopeFields` is cited as `envelope.go:244-423`; the function actually spans 244-475.**
Anchor: §3 VERIFIED (r6 carry) vs `internal/observe/envelope.go`. Confidence: HIGH.
Issue: the cited range ends 52 lines before the function's `default: return false` (471-472) and the closing `return true` (475). The very evidence that unknown keys are rejected — which forces the new `quarantine_path` case — sits outside the cited range.
Fix: cite `envelope.go:244-475`, with the default at `:471-472`.

**[LOW][ticket-mirror-drift][HIGH] §10's “pending” status and §3's ticket:91 description are stale; the mirror itself is complete.**
Anchor: §3 VERIFIED (r10) (ii) “lines 91 (`mirror of spec r9`)”, §10 PRIOR DELTA STATUS / “r10 delta pending application below” vs ticket:91/93/95/96/103/108/112. Confidence: HIGH.
Issue: the ticket already carries all seven r10 WITH blocks (byte-checked), so the delta is applied, not pending, and §10's REPLACE sources no longer exist on disk (the same class as CR9-4, recurring in accounting only). The 1:1 mirror content is correct.
Fix: record the r10 delta APPLIED with byte-equality verification of the seven lines and mark “none pending”, or restate the REPLACE blocks as already-applied targets; correct §3's line-91 description.

### Open Questions
- **[LOW]** §6.1's read paths omit `cmd/mcp-bridge/startup_failure_test.go`, although T2's or-set permits writing it and the packet hashes it, while `internal/observe/log_manifest_test.go` is a read path but not a bound input. Is the read list an exhaustive allowlist? No write-set widening is implied.
- **[LOW]** The AUTO-DECISION rationale “eventMessage must return it non-empty for buildEnvelope” is not the binding constraint: the `default` arm returns `mcp-bridge event`, so non-emptiness is already satisfied; the fixed-message requirement is the real driver. No behavioral consequence; the required case is specified.

### Positive Observations
- AC2-8(c) is genuinely falsifiable and pins the exact candidate space; T3 carries the per-oracle schedule; the 4.096 µs arithmetic is correct and the reachability is explicitly accepted rather than hand-waved.
- The Stat remedy is applied consistently across R3, R6, §4 step 1, AC1-1(c) and AC2-4, and the Lstat-first fatal is fully withdrawn — the r9 contradiction is closed, not merely reworded.
- The ticket mirror content is byte-complete for all seven r10 lines, including the two omissions the r9 review named (schema-1 declaration; AC1-9(b)(iv)).
- All re-derived storage anchors resolve; the envelope insertion point is correctly alphabetical (`protocol_version` 667 → `publication_impact` 668 → `refresh_id` 669), the config pin is 129 at `config_envelope_test.go:166-172`, and both the docs union and the independent `log_test.go` literal lack `quarantine_path`.
- The envelope and file-set envelopes are unchanged; §6.1's write set equals T1/T2 exactly and the `snapshot_v1.go` addition is read-only.
- T2's docs edits do not force edits outside the write set: the closed 23-event map in `logging_contract_test.go:700` iterates its own literal and does not assert equality with the documented event set.
- AC2-7's warning-before-fatal accumulation and AC2-2's both-corrupt two-record oracle correctly make R4's per-file quantifier falsifiable.

### Recommendation
REQUEST CHANGES