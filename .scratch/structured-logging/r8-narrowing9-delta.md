NARROWING DELTA - r8-pin-diagnostics-narrow8 -> r8-pin-diagnostics-narrow9
════ 1. SPEC.MD PER-LINE DELTAS ════
D0a · spec.md:1 (header, revision bump)
CURRENT:
# SPEC-STRUCTURED-LOGGING r8-pin-diagnostics-narrow8 — STATUS: DRAFT
REPLACEMENT:
# SPEC-STRUCTURED-LOGGING r8-pin-diagnostics-narrow9 — STATUS: DRAFT
D0b · spec.md:34 (§1 STATUS, revision bump; trailing 'Objective: …' sentence unchanged)
CURRENT:
SPEC-STRUCTURED-LOGGING revision r8-pin-diagnostics-narrow8. DRAFT (narrowing amendment narrow8 per owner decision 2026-11-11: mirrors the four spec fixes into tickets 09/10/11/12, completes the r8 coverage map with AC12-1..3 and records the counter gap as owner-accepted, narrowing only). Objective: r7 objective plus refresh-path pin diagnosability from logs alone.
REPLACEMENT:
SPEC-STRUCTURED-LOGGING revision r8-pin-diagnostics-narrow9. DRAFT (narrowing amendment narrow9 per owner decision 2026-11-11: closure round — reverts the false counter-gap acceptance claim, completes AC12-1's schema_version oracle and records the T8 dependency drift, narrowing only). Objective: r7 objective plus refresh-path pin diagnosability from logs alone.
[HIGH] · spec.md:315 (REVERT false acceptance claim on the misleading-counter gap — the only transcribed owner answer is pending, not accepted)
CURRENT:
- Prometheus: STILL no new metrics (r7 exception stands). Owner ruling 6 makes logs the diagnostic path, so failure visibility for rejected passes is carried by `catalog_refresh_failed` + readiness `Reason`, not counters. Consequence documented: the misleading `bridge_catalog_refresh_total{outcome="none"}` fallback on rejected passes REMAINS (known gap; owner-ACCEPTED as documented per decisions.md r8 checkpoint item 3, backlog ticket 18 — r8 changelog); operators must query logs, not this counter, for rejection state — stated in T12 docs.
REPLACEMENT:
- Prometheus: STILL no new metrics (r7 exception stands). Owner ruling 6 makes logs the diagnostic path, so failure visibility for rejected passes is carried by `catalog_refresh_failed` + readiness `Reason`, not counters. Consequence documented: the misleading `bridge_catalog_refresh_total{outcome="none"}` fallback on rejected passes REMAINS (known gap; acceptance pending owner ruling, backlog ticket 18 — r8 changelog); operators must query logs, not this counter, for rejection state — stated in T12 docs.
[MEDIUM] · spec.md:263 (AC12-1 — add the missing schema_version 1 assertion so the AD20 claim in the coverage line is actually asserted)
CURRENT:
- AC12-1 (12/AC-1; R18, AD20): docs list the added events and event-specific fields, each marked optional-addition vs schema-version change; the doc-drift test additionally asserts the extended `eventMessage` set (all 23 named events per AD19) and each r8 record's pinned operation — `refresh` for `catalog_refresh_started`/`catalog_refresh_completed`/`catalog_refresh_failed`/`upstream_discovery_failed`/`pin_blocker_detail`, `logging` for `logging_write_failed`, `reload` for `config_reload_applied`. test (doc-drift).
REPLACEMENT:
- AC12-1 (12/AC-1; R18, AD20): docs list the added events and event-specific fields, each marked optional-addition vs schema-version change; the doc-drift test additionally asserts the extended `eventMessage` set (all 23 named events per AD19) and each r8 record's pinned operation — `refresh` for `catalog_refresh_started`/`catalog_refresh_completed`/`catalog_refresh_failed`/`upstream_discovery_failed`/`pin_blocker_detail`, `logging` for `logging_write_failed`, `reload` for `config_reload_applied`, and that every emitted r8 record carries `schema_version` 1 (AD20). test (doc-drift).
════ 2. TICKET WHOLE-LINE REPLACEMENTS ════
NONE.
════ 3. APPEND LINES ════
[DRIFT-EVIDENCE] · issues/17-r8-artifact-consistency-drift.md (append after line 21 — last bullet of the existing evidence list; no existing bullet rewritten)
- `tasks[T8].blockedBy` is `[]` in the installed projection while the strict-serial ruling requires the T7 edge (decisions.md r8 checkpoint item 1: tickets 08-12 strictly serial in ticket order); `spec.md:165` already cites this gap as recorded here.
════ 4. DECISIONS.MD APPEND ════
NONE.
════ 5. REVISION ID AND ID INTEGRITY ════
Revision id advances from `r8-pin-diagnostics-narrow8` to `r8-pin-diagnostics-narrow9`. This is a closure narrowing: exactly three changes (two spec-line deltas, one ticket-17 evidence append), sourced from the round-10 code-reviewer report. No new R, AC or T id; no new AD; no acceptance criterion added or removed — the AC12-1 edit only completes the oracle for the already-claimed AD20 assertion inside the existing AC12-1 test; the spec.md:315 edit restores the prior pending-ruling wording without changing the gap's scope or the r7 exception; the ticket-17 bullet records drift that spec.md:165 already attributes to ticket 17, with no status or ruling change. The four specdd marker pairs are untouched; all section digests and the packet digest must be recomputed for narrow9 at persist time. decisions.md needs no append: checkpoint item 3 remains the recorded r8 owner decision, and the acceptance conflict stays reconciled via backlog ticket 18 (needs-info) per the pending transcribed answer in owner-authorization-r8.txt question 3.
