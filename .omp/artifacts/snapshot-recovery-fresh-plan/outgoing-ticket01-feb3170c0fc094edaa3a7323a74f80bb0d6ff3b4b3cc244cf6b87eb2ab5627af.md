# 01 — Unreadable snapshot must not wedge startup: quarantine and continue

**Type:** task

**Status:** ready-for-agent (execution remains gated by independent reviews and owner authorization)

**Blocked by:** None — spec authored as `.scratch/snapshot-recovery/spec.md` r7; owner confirmed the default-on quarantine policy 2026-11-21.

Pre-spec backlog item: it records the incident and the required behavior. Acceptance criteria and
design text are planner-owned; the accepted criteria are mirrored below from
`.scratch/snapshot-recovery/spec.md` (SNAPREC-SPEC-r3) and are not authored here.

**Owner decision (2026-11-21):** `MCP_SNAPSHOT_QUARANTINE` defaults to ON (recovery enabled);
`off` remains a fail-closed opt-out for manual-inspection deployments.

## Observed failure (deployed, 2026-11-16)

`mcp-bridge-financial` exited code 1 on every restart with one generic record:

```json
{"event":"startup_failed","operation":"startup","outcome":"failure",
 "error":{"code":"internal_error","kind":"internal","retryable":false,
          "remediation":"report this failure; no specific class matched"}}
```

Deleting `snapshot.json` from the state directory was the controlled experiment: the container
started immediately and served normally. The snapshot was unreadable by `Store.Load` — the exact
cause (corrupt JSON, checksum mismatch, unsupported schema, failed snapshot validation, or an
unreadable path) was not recoverable from the logs. The deleted bytes are gone; the operator had
no way to inspect them.

## Code path (verified)

- `internal/storage/store.go:180-230` — `Load` reads and validates the envelope, checksum, and
  `Snapshot.Validate`; `trusted` is set only on success.
- `internal/storage/store.go:295-305` — `ensureReplaceable` refuses to overwrite any snapshot it
  cannot independently load; this is intentional (`plan §4.9`, bytes survive for inspection).
- `internal/service/loop.go:144-151` — startup already tolerates a `Durable` failure: it logs
  `startup_snapshot_rejected` (code `snapshot_unavailable`, level `warn`) and proceeds with no
  cached state.
- `internal/service/reload.go:723-725` — the later persist during admission then hits
  `ensureReplaceable`, returns `storage: refusing to overwrite untrusted snapshot: …` wrapped
  without `service.ErrPublicationFailed`.
- `cmd/mcp-bridge/serve.go:340` and `cmd/mcp-bridge/startup_failure.go:108-112` — `startupFailureForError`
  maps only `ErrPublicationFailed`; everything else becomes the generic fallback. The fatal record
  discards both the underlying storage error and its typed state.

So the process accepts the degraded state, then dies trying to publish the new generation, and
reports a class that gives the operator nothing to act on.

## Required behavior

A snapshot the store refuses is **recoverable startup state**, not a fatal one:

- Detect `ErrCorrupt`/read failure during startup state open or admission.
- Preserve the bytes: atomically rename the rejected `snapshot.json` to a bounded, timestamped
  quarantine name in the same directory (for example `snapshot.corrupt-2026-11-16T23:04:48.123456789Z.json`)
  before publishing a fresh snapshot. Never silently delete, truncate, or overwrite it first.
- Continue startup with no restored catalog, then publish and persist the new generation normally.
- Emit one typed warning identifying the quarantined path and a finite reason (corrupt JSON,
  checksum mismatch, unsupported schema, validation failure, read error). Do not emit
  `internal_error` for this condition.
- Bound retention of quarantine files so a repeating crash cannot fill the state directory.
- If the directory is not writable, or the rename fails, keep startup fatal and classify it as a
  resource/state-directory failure with the real error preserved in logs.
- A store that successfully loads must keep the current no-overwrite protection.

## Blocking questions for the planner

1. Where is the single recovery locus: `storage.Open`, the `Durable` loader in `Loop.Start`, or a
   new `Store` recovery method called once before admission? Recovery in `ensureReplaceable` would
   also affect mid-run saves and must not silently mutate state during normal operation.
2. Which load failures are quarantinable (`ErrCorrupt` classes + unreadable file) versus fatal
   (permissions, read-only filesystem, full disk, `snapshot.json` being a non-file)? A directory
   at the snapshot path cannot be renamed by the same rule.
3. Quarantine naming/retention: keep N newest, or one per distinct content hash? Does the path go
   in the warning's `resource.path` (the schema already has a resource map)?
4. Should an explicit operator opt-out exist (fail-closed) for deployments that require manual
   inspection, and what is its default?
5. Does recovery also cover the two other persisted files (`prompt snapshot` and any temp files)
   or only the catalog snapshot?

## Acceptance direction (for the planner, not AC text)

- Container with a corrupt/unsupported `snapshot.json` starts successfully, serves upstreams, and
  writes a fresh valid snapshot on the next admission.
- `snapshot.corrupt-*` exists with the original bytes; a second restart does not quarantine again.
- Warning record names the reason and path; no `startup_failed` record is emitted for this case.
- Read-only or unwritable state directory still fails startup, with a state/resource classification.
- A valid snapshot is never renamed or overwritten by the recovery path.
## Accepted acceptance criteria (mirror of spec r11)

Mirror ids `AC-1`..`AC-10` are ticket-local labels where present; spec ids are authoritative. New r6 additions are marked; the r7–r11 in-place amendments cover AC1-1, AC1-2, AC1-3, AC1-5, AC1-8, AC1-9, AC1-10, AC2-2, AC2-4, AC2-5, AC2-6, AC2-7, AC2-8 and AC2-9 (spec r7 amended AC1-1, AC1-3, AC1-8, AC1-9, AC2-4, AC2-6, AC2-7, AC2-9; spec r8 amended AC1-5, AC1-9, AC2-2, AC2-5; spec r9 amended AC1-2 and AC1-9; spec r10 amended AC1-1, AC2-4 and AC2-8; spec r11 amended AC1-1, AC1-5, AC1-10, AC2-4, AC2-6 and AC2-8; ids are authoritative in the spec).

- **AC1-1** (mirror `AC-1`, R6): valid `snapshot.json` and `prompts.json` are left byte-identical, no quarantine file is created, and a zero recovery result is returned; with both paths absent, zero recoveries are returned, no file is created, and — with QuarantineKeep+1 (6) pre-seeded conforming `snapshot.corrupt-<RFC3339Nano-Z>.json` files — the directory listing is unchanged afterwards (no pruning, no additions), making the no-retention-pass clause observable. Symlink subcase (r10): `snapshot.json` is a symlink to a valid regular snapshot file — `Load` succeeds through the link, the recovery call returns zero recoveries, mutates nothing, and the symlink entry is left untouched (regularity is resolved through the link; today's admission behavior preserved). Broken-symlink subcase (r11, no new id): `snapshot.json` is a symlink to a NONEXISTENT target (`os.Stat` reports `fs.ErrNotExist`) — treated exactly as the absent case: zero recoveries, no quarantine file, the link entry left byte-untouched (never unlinked, renamed, replaced, or followed into a file creation), and with QuarantineKeep+1 (6) pre-seeded conforming quarantine files the directory listing is unchanged afterwards. FAILS if any file changes, a quarantine file appears, a nonzero result is returned, a seeded conforming file is pruned, in the valid-symlink subcase the symlink is renamed, replaced, or its target modified, or in the broken-symlink subcase the broken link is unlinked, renamed, replaced, or a file appears at the configured path.
- **AC1-2** (mirror `AC-2`, R9): each seeded rejection class yields its `failure_reason` — invalid JSON `corrupt_json`; envelope with trailing data `corrupt_json`; checksum mismatch `checksum_mismatch`; unknown schema `unsupported_schema`; failed `Snapshot.Validate` `validation_failure`; regular-file read failure via the injected read seam `read_error` (no chmod/root dependency). Schema-1 declaration (r9): every failure of the LEGACY schema-1 branch (seed a `snapshot.json` with `schema_version` 1 that fails `decodeSnapshotV1` — decode, trailing data, or frozen-checksum mismatch) yields `corrupt_json`; `checksum_mismatch` is produced ONLY by the schema-2 checksum sentinel, never by a schema-1 seed. FAILS if any seeded class yields a different reason, or a schema-1 checksum-mismatch seed yields `checksum_mismatch`.
- **AC1-3** (mirror `AC-3`, R5): with 5 pre-existing conforming quarantine files plus one non-conforming entry (`snapshot.corrupt-not-a-timestamp.json`), one more quarantine leaves exactly the 5 newest conforming files — i.e. the just-created file plus the 4 newest pre-existing, per the QuarantineKeep-1 pre-existing rule — removes the oldest conforming file, and leaves the non-conforming entry untouched. Verify: storage test asserting the surviving set and the untouched foreign file. FAILS if 6 files remain (off-by-one) or the foreign file is touched.
- **AC1-4** (mirror `AC-4`, R7/R2): with only `prompts.json` corrupt, a `prompts.corrupt-*.json` file carries the original bytes and `snapshot.json` is untouched.
- **AC1-5** (mirror `AC-5`, R5; r6; amended r11): With the unlink seam injecting a failure, quarantine still succeeds, `RecoverRejected` returns nil error and `RetentionFailures` naming the unremoved file of the affected family, and the quarantined file exists at its quarantine path. Verify: (a) storage test with the injected seam; (a-list, r11, no new id) storage test with the injected directory-LISTING seam failing for the affected family (default `os.ReadDir` behind an unexported seam): quarantine still succeeds, the error is nil, and `RetentionFailures` contains EXACTLY one entry for that family whose `quarantine_path` equals THIS RUN's just-created quarantine file (deterministic, validator-representable absolute `.json` path), and that family's prune is skipped; (b) cmd unit test of `emitRecoveryRecords` with a fixed `RetentionFailure` asserting exactly one `snapshot_retention_failed` record with level `warn`, operation `startup`, outcome `failure`, `error.code=snapshot_unavailable`, fixed message `quarantine retention failed`, `resource.path` = state directory, `quarantine_path` set, and `error.remediation` exactly `verify the durable snapshot and storage permissions, then retry` (the shared emitted text — a field-different ErrorInfo collapses to `logging_write_failed` and fails this assertion). FAILS if the error return is non-nil, no/extra records appear, the remediation/message/field values differ, or the listing-failure scenario returns a non-nil error, reports a wrong family count or path, or unlinks any pre-existing file.
- **AC1-6** (R5, NEW r6): With conforming files seeded in BOTH families and the unlink seam failing for both, `RecoverRejected` returns nil error and `RetentionFailures` contains exactly two entries, one per family. Verify: (a) storage test asserting the two-entry result with correct `Family` values; (b) `emitRecoveryRecords` unit test asserting exactly two `snapshot_retention_failed` records whose `quarantine_path` values discriminate the families. FAILS if either family's record is missing or a third record appears.
- **AC1-7** (R1, NEW r6): Negative reload/save oracle: with corrupt `snapshot.json` seeded, direct `Store.Load` still returns an error satisfying `errors.Is(err, ErrCorrupt)`, the admission persist path (`internal/service/reload.go:721-725` through `ensureReplaceable`) still fails exactly as today, and NO `snapshot.corrupt-*.json` file is created by any of these calls — only `RecoverRejected` mutates. Verify: storage/service-level test asserting the error identities and family count 0 after the negative calls. FAILS if a quarantine file appears outside `RecoverRejected` or `ensureReplaceable`'s refusal disappears.
- **AC1-8** (R7, NEW r6): Prompts retention bound with foreign and offset forms: seed 5 conforming `prompts.corrupt-<RFC3339Nano-Z>.json` files, one `prompts.corrupt-not-a-timestamp.json`, and one `prompts.corrupt-2026-11-21T12:00:00+02:00.json` (offset form); one more prompts quarantine leaves exactly the 5 newest conforming-Z files, removes the oldest conforming file, and leaves the foreign and offset entries untouched. Verify: storage test asserting the surviving set. FAILS if the offset or foreign name is pruned or counted.
- **AC1-9** (mirror `AC-9`, R2,R5, NEW r6; amended r7/r8/r9): (a) Prune never deletes this run's file: inject a fixed now; seed conforming `snapshot.corrupt` files whose parsed timestamps equal and exceed now; quarantine a corrupt snapshot under that clock. Verify: storage test asserting the just-created quarantine file exists with its seeded bytes, the survivor count equals `QuarantineKeep` INCLUDING this run's file (`QuarantineKeep-1` pre-existing survivors), and the 4 newest pre-existing conforming names survive while the oldest pre-existing name is removed. FAILS if this run's file is missing, its bytes differ, `QuarantineKeep` pre-existing files survive (off-by-one), or the oldest pre-existing conforming name remains. (b) Byte-identical-name collision (r9): seed EXACTLY ONE pre-existing quarantine file whose name byte-equals the candidate the fixed clock would first choose (`snapshot.corrupt-<RFC3339Nano(now)>.json`) holding known seed bytes, plus the corrupt `snapshot.json` with distinct known bytes; run recovery under the fixed clock. Verify: (i) the pre-existing file still exists at its ORIGINAL name with bytes EXACTLY unchanged (never overwritten, never renamed, never pruned in this scenario), (ii) the quarantined bytes exist at the DISTINCT advanced name `snapshot.corrupt-<RFC3339Nano(now+1ns)>.json` with bytes EXACTLY the corrupt bytes, (iii) the two quarantine paths are distinct, and (iv) `snapshot.json` is absent (renamed). FAILS if the pre-existing file's bytes differ or it is gone (overwrite), if only one quarantine file exists (the seed absorbed the rename), if the two paths are equal, or if the corrupt bytes are not preserved.
- **AC1-10** (mirror `AC-10`, R2, NEW r6; amended r11): `read_error` preserves bytes: seed a regular `snapshot.json` with known bytes, inject a read-seam failure; `RecoverRejected` yields reason `read_error` AND the quarantine file exists with bytes equal to the seeded bytes, original absent. Symlink subcase (r11, no new id): `snapshot.json` is a SYMLINK to a corrupt/read-failing REGULAR target — exactly ONE `Recovery` is returned, the LINK ENTRY is what moves into the quarantine name (no canonicalization; the target file is never touched), byte equality is read THROUGH the quarantine path (which follows the moved link), the target file with its bytes remains exactly preserved at the target path, and the configured path is ABSENT. Prompts subcase (r11, no new id): the same `read_error` oracle executes for the PROMPTS family (prompts `read_error`, `prompts.corrupt-*.json` bytes preserved, snapshot family count 0). Verify: storage test asserting reason and byte equality for all three scenarios. FAILS if the reason differs, the quarantined bytes differ (read through the quarantine path), the target file is modified or absent, the link is canonicalized or rewritten, a second `Recovery` appears, or the prompts subcase touches the snapshot family.
- **AC2-1** (mirror `AC-1`, R1/R2): with corrupt `snapshot.json` seeded, startup admission completes, a fresh valid snapshot is persisted, and the quarantine file's bytes equal the seeded bytes.
- **AC2-2** (mirror `AC-2`, R4): exactly one stderr `startup_snapshot_rejected`/`warn`/`degraded` record with `error.code=snapshot_unavailable`, `failure_reason=corrupt_json`, `resource.path` equal to the configured state directory, `quarantine_path` equal to the quarantine path, and zero `startup_failed` records. Both-corrupt subcase (r8): seed corrupt `snapshot.json` AND corrupt `prompts.json` in ONE pass — startup still proceeds, and the stderr stream contains EXACTLY TWO `startup_snapshot_rejected` records, both warn/degraded/`snapshot_unavailable`/`failure_reason=corrupt_json`, whose `quarantine_path` values are distinct, absolute, and discriminate the families (`snapshot.corrupt-` vs `prompts.corrupt-` base names), each family's quarantined bytes equal to its seeded bytes. FAILS if a second-family record is missing, a third record appears, or any family's quarantined bytes differ.
- **AC2-3** (mirror `AC-3`, R5): a second run against the same state dir creates no new quarantine file and emits no new warning record.
- **AC2-4** (mirror `AC-4`, R3; amended r10/r11): a directory at the `snapshot.json` path makes startup fail with one `startup_failed` record, `error.code=state_directory`, `error.kind=resource`, `resource.path` set; the directory remains untouched (exactly one `startup_failed` record, produced by runContext from the returned startupFailure). Symlink subcase (r10): a symlink at the `snapshot.json` path that RESOLVES TO A DIRECTORY produces the same single `startup_failed`/`state_directory` record (`Stat` resolves the link before the regular-file check); the symlink entry and its target directory remain untouched. Symlink-loop subcase (r11, no new id): `snapshot.json` is a SYMLINK LOOP (resolution fails with ELOOP — an `os.Stat` error other than `fs.ErrNotExist`): the same single `startup_failed`/`state_directory` record is emitted (no FileInfo exists, so no regularity check or read is possible and the file is not quarantined); the loop link remains untouched. A symlink to a REGULAR file never routes here — it is admitted (AC1-1 symlink subcase). FAILS if a symlink is renamed or unlinked, the target directory is touched, or a second `startup_failed` record appears — in EITHER subcase.
- **AC2-5** (mirror `AC-5`, R3): corrupt snapshot in a read-only state directory makes startup fail with `code=state_directory` and leaves the corrupt bytes at the original path; the chmod lane may skip under root. Route named (r8): with the state directory at 0o555, `storage.Open` fails FIRST (the `.writer.lock` create/open inside Open, serve.go:96-98 returning `stateDirectoryFailure` before any recovery call), so the recovery rename is never attempted — AC2-5 asserts the storage.Open/state-directory route, NOT the rename class; the R3 rename-failure class is deterministically verified root-independently by AC2-8(b)'s injected rename-failure seam.
- **AC2-6** (mirror `AC-6`, R8; r7; amended r11): arm 1 — with `MCP_SNAPSHOT_QUARANTINE=off` (including case variants) and a corrupt snapshot, no file is renamed, no quarantine family exists, and startup fails with today's wedge (at least one `startup_failed` record with `error.code=internal_error`); exactly one `startup_snapshot_rejected` record appears — the retained Durable-loader record — carrying neither `failure_reason` nor `quarantine_path`, and ZERO records with event `snapshot_retention_failed` (r11: R8 forbids every recovery record — recovery AND retention — when disabled); original bytes are unchanged and the quarantine family count is zero. Arm 2 (r7) — with `MCP_SNAPSHOT_QUARANTINE=1` and a corrupt snapshot, recovery RUNS: a quarantine file exists with the original bytes, exactly one recovery record carries `failure_reason` and `quarantine_path`, and startup completes. FAILS if arm 2 quarantines nothing or emits the retention/field-less record shape, or if arm 1 emits any `snapshot_retention_failed` record.
- **AC2-7** (mirror `AC-7`, R3/R4; new r5): with corrupt `snapshot.json` and a directory at `prompts.json`, recovery quarantines the catalog and then hits the prompts fatal; startup fails with exactly one `startup_failed`/`state_directory` record and exactly one `startup_snapshot_rejected` record carrying `failure_reason=corrupt_json` and `quarantine_path` equal to the catalog quarantine path; the quarantine file's bytes equal the seeded catalog bytes and the prompts directory remains untouched.
- **AC2-8** (mirror `AC-8`, R3; new r5; amended r8/r10/r11): deterministic storage-level seams, executed for BOTH families (r11 — prompts runs assert `prompts.corrupt-` names and an untouched snapshot family): (a) injected post-rename sync failure → fatal error wrapping `ErrSnapshotUnrecoverable`, result still carries the `Recovery`, bytes at the quarantine path, original absent; (b) injected rename failure → fatal, original bytes remain at the original path, no quarantine file, empty result — with the reserve-then-rename design (b) additionally asserts the reservation is cleaned up (no empty conforming-named file remains); AC1-9(b) is the collision-class oracle for the never-overwrite invariant. Reservation error (b-res, r11, no new id): the reservation seam (default `os.OpenFile`) injected with an error that is NOT an already-exists error (e.g. `EACCES`) → fatal wrapping `ErrSnapshotUnrecoverable`; original bytes EXACTLY intact at the original path; NO rename attempted (no quarantine file, family count unchanged); empty result; no reservation residue. Reservation exhaustion (c, r10): with a FIXED injected clock, pre-seed EXACTLY 4096 conforming names `snapshot.corrupt-<RFC3339Nano(now+k·1ns)>.json` for k=0..4095 plus a corrupt `snapshot.json` with known bytes; run recovery under the fixed clock and assert `errors.Is(err, ErrSnapshotUnrecoverable)`, an EMPTY `Recoveries`, the original bytes intact at the original path, and a snapshot-family count of EXACTLY 4096 (no 4097th conforming name created). FAILS if the error is nil or not `ErrSnapshotUnrecoverable`, any `Recovery` is returned, the original bytes differ or the file is gone (any rename occurred), or the family count is not 4096; in (b-res), FAILS if the original is modified, any quarantine or reservation file appears, or the error is not `ErrSnapshotUnrecoverable`.
- **AC2-9** (mirror `AC-9`, R7,R4, NEW r6; amended r7): With corrupt `prompts.json` ONLY (`snapshot.json` valid), startup proceeds and serves; exactly one `startup_snapshot_rejected` record carrying `failure_reason=corrupt_json` and `quarantine_path` equal to the prompts quarantine path; `prompts.corrupt-*.json` bytes equal the seeded bytes; snapshot family count 0. The fresh `prompts.json` is persisted by the loop's async initial prompt refresh (`refresher.Trigger()`/`refresher.Run(ctx)`, internal/service/loop.go:172 vicinity — admission itself skips PersistPrompts, reload.go:728); the harness polls with a bounded deadline until a fresh `prompts.json` exists and LoadPrompts succeeds; deadline expiry is a FAIL.
