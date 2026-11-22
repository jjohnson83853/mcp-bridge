## Code Review Summary

**Mode:** Specification (no implementation exists; absent impl tests are not defects here).
**Candidate:** `snapshot-recovery-spec-r4@r4`; reviewed bytes as supplied.
**Files Reviewed:** 19 listed inputs plus `internal/observe/envelope.go`, `internal/observe/log.go`, `cmd/mcp-bridge/wiring.go`, `cmd/mcp-bridge/serve_edge_test.go`, `cmd/mcp-bridge/startup_failure_test.go`.

### By Severity
- CRITICAL: 0
- HIGH: 5
- MEDIUM: 7
- LOW: 2

### Issues

[HIGH][specification-contradiction] R4's `failure_reason` is not emittable.
Anchor: spec §2 R4, §4 design ("observe.NewEnvelope + observe.WriteJSONLine"), §6.1. `observe.Envelope` has no `FailureReason` field (`internal/observe/envelope.go:12-31`), and the `Entry.Fields` path rejects the five values via `validFailureReason` (`envelope.go:220-244,362-369`), causing the whole record to fall back to `logging_write_failed`. `internal/observe/envelope.go` is not a T2 write path. Consequence: AC2-2 is unsatisfiable as specified. Fix: add `internal/observe/envelope.go` to the envelope/T2 scope and extend the whitelist (or drop the record-level `failure_reason` oracle).

[HIGH][specification-contradiction] R8 "no warning" contradicts retained code.
Anchor: spec §2 R8. Existing `Loop.Start` already emits a structured `startup_snapshot_rejected` warn/degraded record through `operatorLog` (`internal/service/loop.go:148-151`; `serve.go:129`; `wiring.go:485-492`). Thus opt-out cannot be both "no warning" and "today's wedge semantics verbatim"; §9 mislabels this record as unstructured free text. AC2-6 does not check warnings. Fix: scope R8 to "no quarantine rename and no new recovery record" and assert the retained record.

[HIGH][specification-contradiction] Prune-failure reporting has no channel and wrong vocabulary.
Anchor: spec §4 step 4 vs signature `RecoverRejected(...) ([]Recovery, error)`, and R5. The API cannot return a non-fatal per-family prune warning; using `internal_warning` contradicts AD13's pinned `reload` operation and prose that it is "a defect ... not a normal operator action" (`docs/logging-contract.md:481-488`). No AC covers prune failure. Fix: return a typed prune-warning result and define a correctly pinned event; add an AC.

[HIGH][lifecycle-hazard] Mixed fatal drops an already-quarantined record.
Anchor: §4 order ("catalog first ... fatal stops") and R4 exactly-one-record. If catalog quarantine succeeds and prompts is non-regular, the fatal `error` return discards the catalog `Recovery`; T2 emits no warning for the renamed catalog. No AC covers this transition. Fix: return accumulated recoveries with the fatal error and require emission; add an AC.

[HIGH][r-ac-coverage-gap] R3 post-rename `syncDirectory` failure is untested.
Anchor: §2 R3, §4 step 3, §5/§6. No AC or planned test asserts the sync-failure branch or the resulting state (bytes at quarantine path, original absent). Fix: inject sync failure through a seam and assert classification plus file state.

[MEDIUM][r-ac-coverage-gap] Trailing-data rejection lacks a reason mapping/oracle.
Anchor: R2 lists it; §4 step 2 maps only decode/checksum/schema/validate; AC1-2 has five rows. Fix: map it to `corrupt_json` explicitly and add the AC1-2 row.

[MEDIUM][ac-not-falsifiable] `read_error` and rename-failure oracles depend on permissions.
Anchor: AC1-2 (unreadable file) and AC2-5 (chmod 0o555). Under root the `read_error` row cannot be exercised and AC2-5 skips, leaving R3's rename-failure class unverified. Fix: deterministic fault injection, or require an explicit non-root verification lane.

[MEDIUM][r-ac-coverage-gap] R6 `ErrNoSnapshot` no-op is uncovered.
Anchor: R6 vs AC1-1 (valid-only). No oracle asserts absent-file → zero recoveries, no rename, no retention pass. Fix: add an absent-snapshot subcase.

[MEDIUM][lifecycle-hazard] Irreversible prune assumes parseable family names.
Anchor: R5, §4 step 4, §8 irreversible step. Handling/order of unparsable or foreign `snapshot.corrupt-*` names is unspecified; a prune could delete operator files. Fix: strict timestamp parse, ignore/refuse non-conforming names, document.

[MEDIUM][specification-contradiction] Raw cause logging conflicts with the logging contract.
Anchor: R3/§4 "log the underlying error text on stderr" vs "Every application log record ... is one newline-delimited JSON object" and "Arbitrary Go error text is never a log field"; no permitted event/field exists. Fix: define a typed record or drop the raw-text requirement.

[MEDIUM][specification-contradiction] `resource.path` semantics.
Anchor: R4 uses the quarantine path, but the contract says `resource` carries "only the configured path" (`docs/logging-contract.md:259`); a semantic change would bump `schema_version` under the evolution rule, not just add a note. Confidence MEDIUM. Fix: state the semantic change and version consequence, or use a validated event-specific path field.

[MEDIUM][lifecycle-hazard] Opt-out parsing is case-undefined.
Anchor: R8. Existing env conventions use `EqualFold`; `OFF` would enable auto-rename. Fix: define case-insensitive `off`.

[LOW][editorial] Stale anchors: no `runServe`; `storage.Open` check is `serve.go:96-98`, not `:100-102`; `runtime_edge_test.go:70` is a `durableLoader` unit test, not a serve-admission harness.

[LOW][editorial] §6.1 read paths omit `serve_edge_test.go`/`startup_failure_test.go` needed for AC2 work; T1's decision-log write to `.omp/artifacts` is outside the envelope; the "verbatim" and "1:1 R1-R8" claims are inaccurate.

### Open Questions
- Retention sort key is unspecified; RFC3339Nano variable-length fractions make lexicographic name ordering unreliable (Confidence: LOW).
- Whether test execution occurs as root (would silently void two R3/R9 oracles) is unknown.

### Positive Observations
- Sentinels chained under `ErrCorrupt` preserve `errors.Is` at `store_test.go:232-269` and `snapshot_edge_test.go:29`.
- R1 locus, `ensureReplaceable` behavior, `trusted` stay-false flow, and AD13 `snapshot_unavailable` object verified against source.
- Specdd markers are unique, ordered, and non-nested; ticket AC mirror matches r4 ids.

### Recommendation
REQUEST CHANGES