# 01 — Unreadable snapshot must not wedge startup: quarantine and continue

**Type:** task

**Status:** ready-for-agent (execution remains gated by independent reviews and owner authorization)

**Blocked by:** None — spec authored as `.scratch/snapshot-recovery/spec.md` r3; owner confirmed the default-on quarantine policy 2026-11-21.

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
  quarantine name in the same directory (for example `snapshot.corrupt-20261116T230448Z.json`)
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
## Accepted acceptance criteria (mirror of spec r3)

Mirror ids `AC-1`..`AC-6` are the ticket-local labels; the spec ids are authoritative.

- **AC1-1** (mirror `AC-1`, R6): with a valid `snapshot.json` and `prompts.json`, the recovery call leaves both files byte-identical, creates no quarantine file, and emits no warning.
- **AC1-2** (mirror `AC-2`, R9): each seeded rejection class yields its `failure_reason` — invalid JSON `corrupt_json`; checksum mismatch `checksum_mismatch`; unknown schema `unsupported_schema`; failed `Snapshot.Validate` `validation_failure`; unreadable regular file `read_error`.
- **AC1-3** (mirror `AC-3`, R5): with 5 pre-existing quarantine files, one more quarantine leaves exactly 5 and removes the oldest.
- **AC1-4** (mirror `AC-4`, R7/R2): with only `prompts.json` corrupt, a `prompts.corrupt-*.json` file carries the original bytes and `snapshot.json` is untouched.
- **AC2-1** (mirror `AC-1`, R1/R2): with corrupt `snapshot.json` seeded, startup admission completes, a fresh valid snapshot is persisted, and the quarantine file's bytes equal the seeded bytes.
- **AC2-2** (mirror `AC-2`, R4): exactly one stderr record `event=startup_snapshot_rejected`, level `warn`, outcome `degraded`, `error.code=snapshot_unavailable`, `failure_reason=corrupt_json`, `resource.path` equal to the quarantine path, and zero `startup_failed` records.
- **AC2-3** (mirror `AC-3`, R5): a second run against the same state dir creates no new quarantine file and emits no new warning record.
- **AC2-4** (mirror `AC-4`, R3): a directory at the `snapshot.json` path makes startup fail with one `startup_failed` record, `error.code=state_directory`, `error.kind=resource`, `resource.path` set; the directory remains untouched.
- **AC2-5** (mirror `AC-5`, R3): corrupt snapshot in a read-only state directory makes startup fail with `code=state_directory` and leaves the corrupt bytes at the original path.
- **AC2-6** (mirror `AC-6`, R8): with `MCP_SNAPSHOT_QUARANTINE=off` and a corrupt snapshot, no file is renamed, no quarantine family exists, and startup fails with today's semantics.
