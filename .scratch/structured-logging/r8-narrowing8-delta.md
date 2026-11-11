NARROWING DELTA - r8-pin-diagnostics-narrow7 -> r8-pin-diagnostics-narrow8
════ 1. SPEC.MD PER-LINE DELTAS ════
D0a · spec.md:1 (header, revision bump)
CURRENT:
# SPEC-STRUCTURED-LOGGING r8-pin-diagnostics-narrow7 — STATUS: DRAFT
REPLACEMENT:
# SPEC-STRUCTURED-LOGGING r8-pin-diagnostics-narrow8 — STATUS: DRAFT
D0b · spec.md:34 (§1 STATUS, revision bump; trailing 'Objective: …' sentence unchanged)
CURRENT:
SPEC-STRUCTURED-LOGGING revision r8-pin-diagnostics-narrow7. DRAFT (narrowing amendment narrow7 per owner decision 2026-11-11: declares the D5 fallback-lane exception, asserts ruling-12 candidate counts, pins the eventMessage/operation assertions, re-tags AC09-1/AC10-1 with R25 and makes next_attempt_ms conditional, narrowing only). Objective: r7 objective plus refresh-path pin diagnosability from logs alone.
REPLACEMENT:
SPEC-STRUCTURED-LOGGING revision r8-pin-diagnostics-narrow8. DRAFT (narrowing amendment narrow8 per owner decision 2026-11-11: mirrors the four spec fixes into tickets 09/10/11/12, completes the r8 coverage map with AC12-1..3 and records the counter gap as owner-accepted, narrowing only). Objective: r7 objective plus refresh-path pin diagnosability from logs alone.
COVERAGE · spec.md:267 (add AC12-1..3 to the r8 coverage mapping; state tags AC12-1 R18, AC12-2 R5, AC12-3 R20-R25)
CURRENT:
Coverage (r8): R20→AC08-1/AC09-7; R21→AC08-6/AC09-1..6,8; R22→AC08-1..6; R23→AC10-1..5; R24→AC08-2/AC11-1..2; R25→AC08-1,7/AC09-1/AC10-1/AC11-3. AD20 ruling (no schema_version change) is itself asserted by AC12-1.
REPLACEMENT:
Coverage (r8): R20→AC08-1/AC09-7/AC12-3; R21→AC08-6/AC09-1..6,8/AC12-3; R22→AC08-1..6/AC12-3; R23→AC10-1..5/AC12-3; R24→AC08-2/AC11-1..2/AC12-3; R25→AC08-1,7/AC09-1/AC10-1/AC11-3/AC12-3; R18→AC12-1; R5→AC12-2. AD20 ruling (no schema_version change) is itself asserted by AC12-1.
PROMETHEUS-STALE-STATUS · spec.md:315 (counter gap: acceptance pending -> owner-ACCEPTED per decisions.md r8 checkpoint item 3)
CURRENT:
- Prometheus: STILL no new metrics (r7 exception stands). Owner ruling 6 makes logs the diagnostic path, so failure visibility for rejected passes is carried by `catalog_refresh_failed` + readiness `Reason`, not counters. Consequence documented: the misleading `bridge_catalog_refresh_total{outcome="none"}` fallback on rejected passes REMAINS (known gap; acceptance pending owner ruling, backlog ticket 18 — r8 changelog); operators must query logs, not this counter, for rejection state — stated in T12 docs.
REPLACEMENT:
- Prometheus: STILL no new metrics (r7 exception stands). Owner ruling 6 makes logs the diagnostic path, so failure visibility for rejected passes is carried by `catalog_refresh_failed` + readiness `Reason`, not counters. Consequence documented: the misleading `bridge_catalog_refresh_total{outcome="none"}` fallback on rejected passes REMAINS (known gap; owner-ACCEPTED as documented per decisions.md r8 checkpoint item 3, backlog ticket 18 — r8 changelog); operators must query logs, not this counter, for rejection state — stated in T12 docs.
════ 2. TICKET WHOLE-LINE REPLACEMENTS ════
AC11-3-OPPOSITE-LANE · issues/11-path-parity-and-logger-fallback.md:14 (mirror spec:262: opposite-lane fallback clause, D5 exception, AD21)
CURRENT:
- [ ] AC-3: A failed log encode or write surfaces a minimal credential-free diagnostic instead of failing silently.
REPLACEMENT:
- [ ] AC11-3 (11/AC-3; R25): a failed log encode or write surfaces `logging_write_failed` — minimal, credential-free — instead of failing silently, written as one JSON line to the lane OPPOSITE the one the failed record routed to (the declared D5 exception, AD21: failed stderr warn/error write → warn-level fallback on stdout; failed stdout debug/info write → fallback on stderr). test.
AC12-1-DOC-DRIFT · issues/12-contract-docs-and-verification-closeout.md:12 (mirror spec:263: extended eventMessage set and pinned r8 operations)
CURRENT:
- [ ] AC-1: Documentation lists the added events, their event-specific fields, and for each states optional-addition versus schema-version change.
REPLACEMENT:
- [ ] AC12-1 (12/AC-1; R18, AD20): docs list the added events and event-specific fields, each marked optional-addition vs schema-version change; the doc-drift test additionally asserts the extended `eventMessage` set (all 23 named events per AD19) and each r8 record's pinned operation — `refresh` for `catalog_refresh_started`/`catalog_refresh_completed`/`catalog_refresh_failed`/`upstream_discovery_failed`/`pin_blocker_detail`, `logging` for `logging_write_failed`, `reload` for `config_reload_applied`. test (doc-drift).
AC09-1-R25-TAG · issues/09-pin-blocker-detail.md:14 (add R25 tag, mirror spec:247)
CURRENT:
- [ ] AC09-1 (09/AC-1; R21): several unresolved mandatory pins → ALL reported via `pin_blocker_detail` (AD18), each with server and tool identity, not only the first; every such record carries `level` `warn` per AD18/ruling 16 — a `pin_blocker_detail` emitted at any other level (in particular `debug`) fails this criterion. test.
REPLACEMENT:
- [ ] AC09-1 (09/AC-1; R21, R25): several unresolved mandatory pins → ALL reported via `pin_blocker_detail` (AD18), each with server and tool identity, not only the first; every such record carries `level` `warn` per AD18/ruling 16 — a `pin_blocker_detail` emitted at any other level (in particular `debug`) fails this criterion. test.
AC10-1-R25-TAG · issues/10-discovery-attribution-and-readiness-reason.md:14 (add R25 tag, mirror spec:255)
CURRENT:
- [ ] AC10-1 (10/AC-1; R23): a discovery failure names `failure_stage` and a bounded `failure_reason`, distinguishing auth-during-initialize from tools-list timeout; the emitted records carry the AD-pinned levels — `upstream_discovery_failed` at `warn` (AD17) and, per the pass outcome, the accompanying `catalog_refresh_failed` at `warn` with `retained_published` or `error` with `none_available` (AC08-1/AC08-7) — a diagnostic at any other level (in particular `debug`) fails this criterion. test.
REPLACEMENT:
- [ ] AC10-1 (10/AC-1; R23, R25): a discovery failure names `failure_stage` and a bounded `failure_reason`, distinguishing auth-during-initialize from tools-list timeout; the emitted records carry the AD-pinned levels — `upstream_discovery_failed` at `warn` (AD17) and, per the pass outcome, the accompanying `catalog_refresh_failed` at `warn` with `retained_published` or `error` with `none_available` (AC08-1/AC08-7) — a diagnostic at any other level (in particular `debug`) fails this criterion. test.
T10-BLOCKEDBY-REFRESH · issues/17-r8-artifact-consistency-drift.md:20 (stale evidence: projection now carries ["T9"]; strict-serial edge present)
CURRENT:
- T10's `blockedBy` is `["T8"]` in the projection; the strict-serial ruling requires `["T9"]`.
REPLACEMENT:
- T10's `blockedBy` is now `["T9"]` in the installed projection (verified 2026-11-11 against `.omp/plans/execution-state.json`); the strict-serial edge the strict-serial ruling requires is present, so this drift item is resolved and retained for the audit trail only.
════ 3. APPEND LINES ════
NONE.
════ 4. DECISIONS.MD APPEND ════
NONE.
════ 5. REVISION ID AND ID INTEGRITY ════
Revision bumps r8-pin-diagnostics-narrow7 → r8-pin-diagnostics-narrow8 as a pure ticket-mirror pass: no R, AC or T id is added or removed, no AD entry is created or amended (AD19–AD21 untouched), and the envelope is unchanged. The spec already carries all four fixes (AC09-1 at :247, AC10-1 at :255, AC11-3 at :262, AC12-1 at :263); this round mirrors them into tickets 09/10/11/12 and corrects the two stale lines only — :267 extends the r8 coverage map so every AC12 criterion is R-anchored (AC12-1→R18, AC12-2→R5, AC12-3→R20–R25, with AC12-3 appended to each R20–R25 entry), making r8 coverage complete with no uncovered requirement, and :315 records the refresh-counter gap as owner-ACCEPTED per decisions.md r8 checkpoint item 3 instead of acceptance-pending. issues/17:20 is refreshed to the verified installed projection state (T10 blockedBy = ["T9"]).
