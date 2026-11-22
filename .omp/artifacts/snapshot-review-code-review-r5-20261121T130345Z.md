## Code Review Summary

**Mode:** Specification (no implementation exists; absent impl tests are not defects). **Candidate:** snapshot-recovery-spec-r5@r5. **Files Reviewed:** 19 supplied inputs plus `internal/observe/{envelope.go,log.go}`, `cmd/mcp-bridge/{serve.go,main.go,startup_failure.go,wiring.go,logging_contract_test.go}`, `internal/storage/store.go`, `internal/service/{loop.go,reload.go}`, `docs/logging-contract.md`. **Review limits:** read-only; no git diff/LSP/test execution; lint not run; candidate bytes taken as supplied.
**Total Issues:** 14 — CRITICAL 0, HIGH 3, MEDIUM 6, LOW 5.

### Issues

**[HIGH][statement-contradiction]** §4's Entry/Fields emitter cannot emit R4's record. `buildEnvelope` rejects the AD13 `error` object because `snapshot_unavailable` is absent from `runtimeErrorInfo` (`internal/observe/envelope.go:103-130,144-165`), so `Logger.Log` writes `logging_write_failed` (`internal/observe/log.go:85-95`). `NewEnvelope` (`envelope.go:78`) cannot carry `Fields`, so the "direct envelope build" cannot carry `failure_reason`/`quarantine_path`. AC2-2 and AC1-5b are unsatisfiable under T2's exhaustive envelope edit set. Fix: enumerate a `runtimeErrorInfo` case (resource/true/static remediation) or an `ErrorInfo`+validated-`Fields` emitter, and pin it in T2/AC1-5b.

**[HIGH][statement-contradiction]** `quarantine_path` is missing from `envelopeFieldNames` (`envelope.go:556-706`) and the docs `envelope-fields` union (`docs/logging-contract.md:102-190`). `TestLoggingContractDocumentsExactEnvelopeFieldUnion` (`cmd/mcp-bridge/logging_contract_test.go:114`) compares them exactly, so adding the field to the authoritative union fails the suite; a prose-only sentence leaves the union incomplete, and `checkClosedFieldPaths` rejects the emitted field. T2 says "(c) nothing else changes" and lists only three doc additions. Fix: enumerate the registry insertion and the union-list edit.

**[HIGH][ticket-mirror-drift]** Ticket AC2-6 states "no `internal_error` wedge is emitted"; spec AC2-6 and R8 require at least one `startup_failed`/`internal_error` record. The ticket mirror also drops AC1-5b(b), AC2-7 byte assertions and AC2-8(b)'s "no quarantine file, empty result". Non-blocking class, but the engineering acceptance text is wrong.

**[MEDIUM][statement-contradiction]** T2 routes `ErrSnapshotUnrecoverable`→`stateDirectoryFailure`, but `startupFailureForError(err)` takes no path (`cmd/mcp-bridge/startup_failure.go:108-112`) while `stateDirectoryFailure` requires one (`:26-33`); §4 also says serve returns `stateDirectoryFailure(store.Dir())`. Risk: a pathless `state_directory` record failing AC2-4's `resource.path`. Fix: specify exactly one route.

**[MEDIUM][r-ac-coverage-gap]** `RetentionFailure *RetentionFailure` (singular) cannot report one record per affected family as R5 requires; no AC covers both families' prune failing. Fix: slice result plus an AC.

**[MEDIUM][r-ac-coverage-gap]** R1's "never in Reload, mid-run saves, or `ensureReplaceable`" has no oracle (R1 maps only to AC2-1). Add a negative reload/save transition test.

**[MEDIUM][r-ac-coverage-gap]** R7 "identical quarantine behavior" is covered only for rename/bytes (AC1-4). There is no prompts retention-bound, prompts warning-record, or prompts `read_error` oracle.

**[MEDIUM][lifecycle-hazard]** Prune ranks the just-created quarantine file; a backward clock or equal timestamp can sort it oldest and delete it, contradicting R2 "bytes are preserved exactly". Fix: never prune the file created in this recovery.

**[MEDIUM][statement-contradiction]** `snapshot_retention_failed` has no `eventMessage` case (`internal/observe/log.go:227-276`; default `:275`), so it emits `"mcp-bridge event"` while the AD13 row promises a fixed message. `log.go` is outside T2's write paths and AC1-5b does not assert message.

**[LOW]** (a) AC1-2's `read_error` row asserts only the reason, not rename/bytes preservation (R2) [r-ac-coverage-gap]. (b) A post-rename sync fatal skips retention, so R5's "at most 5" is not guaranteed on that path [lifecycle-hazard]. (c) §6.1 read paths omit `cmd/mcp-bridge/logging_contract_test.go`, which pins the envelope union [ledger-state-drift]. (d) AC1-5b's "record emitter" is undefined; the design emits inline in `serve.go` [undefined-term]. (e) "strict RFC3339Nano UTC" is not pinned to `Z`; `time.Parse` accepts `+02:00` family names, which may then be counted/pruned [r-ac-coverage-gap].

### Open Questions
- LOW: Does the retention grammar require a literal `Z`, or is any parseable offset conforming? Finding 14(e) depends on this.
- LOW: Must the "Event and operation pins" table (`docs/logging-contract.md:61-90`) list `snapshot_retention_failed`? That would be a fourth docs edit, contradicting the "exactly three additions" claim.

### Positive Observations
- §9 faithfully enumerates exactly H1-H5, M1-M7, L1-L2 with matching classes; H2, H4, M1-M7 and L1-L2 closures verify. H1/H3 remain incomplete per findings 1, 2 and 9.
- R9 is safe: `store_test.go:232-269` and `snapshot_edge_test.go:29` assert `errors.Is(err, ErrCorrupt)` only; no exact-text pins.
- Anchor corrections are accurate (`serve.go:46,96-98,213,340`; `store.go:295,309`; `startup_failure.go:26-33,108-112`).
- R8 default ON is owner-confirmed, `EqualFold` is specified, and AC2-6's field discrimination holds because `wiring.go:485-492` builds no `Resource`/`Fields`.
- AC2-7 accumulation, AC2-8's deterministic seams, the strict prune grammar, and §8's irreversibility disclosure are sound.
- §6.1 write scope matches §6 exactly.

### Recommendation
REQUEST CHANGES