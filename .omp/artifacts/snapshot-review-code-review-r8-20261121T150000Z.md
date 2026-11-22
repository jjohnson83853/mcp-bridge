## Code Review Summary

**Mode:** Specification only — `snapshot-recovery-spec-r8@r8`; no implementation exists. **Files Reviewed:** 26 (spec, ticket, backlog, domain/issue-tracker docs, logging contract, go.mod, 17 live code anchors, r7 review artifact, findings taxonomy). **Review limits:** read-only; no `go test`, `git diff`, LSP, or state-tool/candidate-hash verification; conclusions rest on the on-disk candidate bytes and the cited anchors. **Total Issues:** 4 (HIGH 1, MEDIUM 1, LOW 2).

**r7 closure verified:** all six are closed by the reviewer's exact stated fixes — M1 shared `snapshot_retention_failed` remediation (AD13 row, §7, R5, AC1-5b `error.remediation`, T2 edge_test) with no new code/field/id; L(a) AC2-2 both-corrupt subcase with distinct absolute `quarantine_path`; L(b) §7 absolute-path wording; L(c) ticket header/list/AC1-3/AC1-9 now match r8; L(d) AC1-9 names the 4 survivors and oldest removal; L(e) AC2-5 names the `storage.Open` route and AC2-8(b) the rename oracle. Envelope unchanged: R1–R9, AC1-1..AC1-10, AC2-1..AC2-9, T1–T3; §6.1 write paths equal the T1/T2 sets; all four specdd markers present; R→AC trace complete.

### By Severity
- CRITICAL: 0
- HIGH: 1
- MEDIUM: 1
- LOW: 2

### Issues

**[HIGH][lifecycle-hazard] Quarantine rename can overwrite a preserved file**
Anchor: spec §4 step 3 (`os.Rename(path, quarantine)`) vs R2, AC1-9, §8. Confidence: HIGH.
Issue: The name is `snapshot.corrupt-<RFC3339Nano(now)>.json`; `os.Rename` replaces an existing destination. A byte-identical pre-existing name — reachable with the backward clock the r7 prune note already treats as a threat, and with AC1-9's fixed clock — is destroyed, violating R2 "bytes are preserved exactly" and §8 "additive". AC1-9 cannot distinguish overwrite from prune: when the equal-timestamp seed is re-occupied, the amended "4 newest pre-existing survive" set passes vacuously for one seed arrangement and fails for another; the seed count is unstated.
Fix: specify target-exists handling (Lstat; advance the timestamp deterministically 1ns until free, or use an O_EXCL link/rename), add a byte-identical-name subcase asserting both files' original bytes survive at distinct paths, and state the seed count.

**[MEDIUM][statement-contradiction] §8's unlink bound is false after bound exceedance**
Anchor: §8 vs R5/§4 step 4. Confidence: HIGH.
Issue: "At most QuarantineKeep-1 pre-existing files are unlinked per pass" is false: the design unlinks N−(QuarantineKeep−1), and §8 itself names the post-sync-fatal case where the bound is exceeded (N=10 ⇒ 6 unlinks). It understates an irreversible deletion.
Fix: "unlinks every pre-existing conforming file beyond the newest QuarantineKeep-1 (at most QuarantineKeep-1 survive per pass)".

**[LOW][ticket-mirror-drift] Ticket mirror omits three r8 amendments**
Anchor: ticket:91-103 vs AC1-5/AC2-2/AC2-5 and §10. Confidence: HIGH.
Issue: Mirrored AC1-5 lacks the `error.remediation` assertion, AC2-2 lacks the both-corrupt subcase, and AC2-5 lacks the Open-route/AC2-8(b) clause, while line 93 claims coverage; §10's four REPLACE blocks are already applied on disk. Spec ids are authoritative, so advisory.
Fix: add the three mirror lines (or declare them summary-level) and mark §10 applied/none.

**[LOW][stale-citation] §9 risk(3) cites the wrong seam oracles**
Anchor: §9 "Highest-risk calls to review" (3). Confidence: HIGH.
Issue: It requires AC1-3/AC1-9 survivor assertions "with and without seam failures"; those ACs use no seam (AC1-5/AC1-6 do).
Fix: name AC1-5/AC1-6 for the seam variant.

### Open Questions
**[LOW][r-ac-coverage-gap] Schema-1 rejection reasons are undecidable inside T1's write set.** Confidence: MEDIUM. §4 maps reasons "from sentinel" and AC1-2 asserts checksum→checksum_mismatch, but `Load`'s legacy branch returns one error for all `decodeSnapshotV1` failures (`snapshot_v1.go:135-157`) and T1's exact files exclude `snapshot_v1.go`; whichever sentinel wraps that branch mislabels the other class. Fix: declare schema-1 failures `corrupt_json` (checksum_mismatch schema-2 only) or widen T1 with typed legacy sentinels plus a schema-1 AC case.

### Positive Observations
- The M1 fix is genuinely emittable: AD13 row, §7, R5, T2's edge_test case, and AC1-5b all carry the identical text; `validErrorInfo` field-equality (`envelope.go:136-142`) plus `ErrorInfo` JSON tags yield `error.remediation`.
- Anchors hold: `serve.go:96-98`, `main.go:59-68`, `startup_failure.go:26-33/121-133`, `wiring.go:485-492`, `loop.go:150/172`, `reload.go:728`, `envelope.go:668-669`, pin 129 (`config_envelope_test.go:166-172`), docs AD13 columns.
- `config.go:380-382` makes `storage.directory` absolute (AC2-2 satisfiable); `durableLoader`/`promptLoader` map ErrNoSnapshot→nil; existing tests assert only `errors.Is(ErrCorrupt)` (`store_test.go:232-269`, `snapshot_edge_test.go:29`).
- Warning counts are pinned across scenarios (AC2-2 one/two, AC2-3 zero-new, AC2-6 one retained, AC2-7 one, AC2-9 one).

### Recommendation
REQUEST CHANGES