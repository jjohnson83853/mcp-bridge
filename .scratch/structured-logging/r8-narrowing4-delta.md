NARROWING DELTA — r8-pin-diagnostics-narrow3 → r8-pin-diagnostics-narrow4
Owner decision 2026-11-11 (narrow4): close code-reviewer REQUEST CHANGES packet db7551a1… (sha256 db7551a19d53ee518e0aaf975f4796e44cc9b7beaaeabb7361a57f2a6692d503) rows H1–H2, M1–M3, L1–L3, NARROWING ONLY — no R/AC/T id added or removed, no new AD, no new acceptance criterion, no widened envelope. M3 (AD21 AC coverage) is DECLINED and recorded in decisions.md, not implemented. All spec changes are whole-line replacements; none touches a marker line, so the four specdd marker pairs (40/67, 169/204, 271/287, 289/303) stay unique, ordered, non-nested, one per line (edited lines: 1, 34, 63, 64, 240, 246, 260, 261, 273, 284, 285 — none is a marker line).

════ 1. SPEC.MD PER-LINE DELTAS ════

D0a · spec.md:1 (header, revision bump)
CURRENT:
# SPEC-STRUCTURED-LOGGING r8-pin-diagnostics-narrow3 — STATUS: DRAFT
REPLACEMENT:
# SPEC-STRUCTURED-LOGGING r8-pin-diagnostics-narrow4 — STATUS: DRAFT

D0b · spec.md:34 (§1 STATUS, revision bump; trailing 'Objective: …' sentence unchanged)
CURRENT:
SPEC-STRUCTURED-LOGGING revision r8-pin-diagnostics-narrow3. DRAFT (narrowing amendment narrow3 per owner decision 2026-11-11: closes review packet 8486fd04… residuals R1–R6, narrowing only; ready for critic review; execution requires the workflow's explicit user checkpoints). Objective: r7 objective plus refresh-path pin diagnosability from logs alone.
REPLACEMENT:
SPEC-STRUCTURED-LOGGING revision r8-pin-diagnostics-narrow4. DRAFT (narrowing amendment narrow4 per owner decision 2026-11-11: closes review packet db7551a1… rows H1–H2, M1–M3, L1–L3, narrowing only; ready for critic review; execution requires the workflow's explicit user checkpoints). Objective: r7 objective plus refresh-path pin diagnosability from logs alone.

L2 · spec.md:63 (R23) — qualify the unconditional "with the next attempt time" with AD17's already-approved iff; pure narrowing of an over-broad sentence.
CURRENT:
- R23 (owner rulings 8, 9): An upstream discovery failure names the failing stage — closed set `dns|connect|tls|auth|initialize|tools_list` — with a bounded cause category and the HTTP status or timeout value where applicable. The endpoint appears sanitized as scheme/host/port/path only; never credentials, headers, query strings, or bodies (R8). Each failure states its recovery expectation — automatic retry scheduled vs configuration/credentials require correction — with the next attempt time, and after a rejected pass the affected server's readiness reason is non-empty and identifies the failure, distinct from not-yet-probed.
REPLACEMENT:
- R23 (owner rulings 8, 9): An upstream discovery failure names the failing stage — closed set `dns|connect|tls|auth|initialize|tools_list` — with a bounded cause category and the HTTP status or timeout value where applicable. The endpoint appears sanitized as scheme/host/port/path only; never credentials, headers, query strings, or bodies (R8). Each failure states its recovery expectation — automatic retry scheduled vs configuration/credentials require correction — with the next attempt time present only when an automatic retry is scheduled (per AD17), and after a rejected pass the affected server's readiness reason is non-empty and identifies the failure, distinct from not-yet-probed.

H1a · spec.md:64 (R24) — parity sentence retargeted to the reachable config-validation pin class; the reload-leg runtime parity claim (unreachable: `reload_rejected` carries no `config` object) is deleted and stays deferred to backlog ticket 19.
CURRENT:
- R24 (owner rulings 10, 18): The log shows a configuration edit reaching the running process: refresh records name the configuration revision and trigger they used. A pin rejection on SIGHUP reload and on periodic refresh reports the same core pin diagnostic detail — pin identity (server, tool) and source location (`config.property_path`); discovery evidence and retry guidance are refresh-path records (AD16–AD18) and are not asserted on the reload path; no path is the only way to obtain the useful error.
REPLACEMENT:
- R24 (owner rulings 10, 18): The log shows a configuration edit reaching the running process: refresh records name the configuration revision and trigger they used. A config-validation pin rejection reports the same core pin diagnostic detail on the SIGHUP reload path (`config_reload_rejected`) and the configuration-validation path (`config_validation_failed`): `config.property_path` with position, plus `config.pin_server`/`config.pin_tool` only where the diagnostic class carries validated pin identity (absent for the `pin_fields`/`pin_server_missing`/`pin_tool_missing` classes); a runtime pin rejection on the reload leg is deferred (backlog ticket 19); no path is the only way to obtain the useful error.

M1a · spec.md:240 (AC08-1) — the "with debug disabled" clause is deleted: `internal/observe/log.go` has no level threshold, so a debug-disabled mode cannot exist and the clause is vacuous (same vacuity as AC08-7).
CURRENT:
- AC08-1 (08/AC-1; R20, R22, R25): a refresh pass whose candidate is rejected emits exactly one pass-level failure record, `catalog_refresh_failed` (per-pin `pin_blocker_detail` records are additional and not counted by this oracle), at the AD16-pinned level — warn when `catalog_state` is `retained_published`, error when `none_available` — with debug disabled. test.
REPLACEMENT:
- AC08-1 (08/AC-1; R20, R22, R25): a refresh pass whose candidate is rejected emits exactly one pass-level failure record, `catalog_refresh_failed` (per-pin `pin_blocker_detail` records are additional and not counted by this oracle), at the AD16-pinned level — warn when `catalog_state` is `retained_published`, error when `none_available`. test.

M1b · spec.md:246 (AC08-7) — restated as a positive level assertion; the old "not debug-only" form cannot fail because no debug-only mode exists.
CURRENT:
- AC08-7 (08/AC-7; R25): no failure diagnostic is emitted only at debug level. test.
REPLACEMENT:
- AC08-7 (08/AC-7; R25): every `catalog_refresh_failed` record carries `level` `warn` or `error` exactly as pinned by AC08-1 — positively asserted on each emitted record (`warn` with `retained_published`, `error` with `none_available`). test.

H1b · spec.md:260 (AC11-1) — retargeted to the reachable config-validation pin class, naming the carrier records and compared key forms; the unreachable reload↔refresh runtime-parity set is deleted, not softened. NOTE: the CURRENT line is longer than the review capture window; its full text is the verbatim REPLACEMENT that r8-narrowing3-delta.md §1 item R1b applied at :260 (prefix verified this session through "…remain refresh-path re"). Replace the WHOLE line by number; do not textual-match.
CURRENT:
[entire line at :260 = r8-narrowing3-delta.md §1 R1b REPLACEMENT, applied verbatim; begins: "- AC11-1 (11/AC-1; R24): a pin rejection on SIGHUP reload and on periodic refresh reports the same core detail — pin identity (server, tool) and source location (`config.property_path`); `publication_impact` is no longer part of the compared set (the rejection event itself states the impact; refresh-side `publication_impact` stays asserted by AC09-7). Compared carriers and key forms — reload: `config_reload_rejected` (AD7) carrying the Diagnostics-renderer pin attribution (AD3/AD4) as `config.pin_server`, `config.pin_tool`, `config.property_path`; refresh: `pin_blocker_detail` (AD18) carrying typed `server`, `config.pin_tool`, `config.property_path`. Discovery evidence (chunked `discovered_tools`) and retry guidance (`next_attempt_ms`) remain refresh-path re…" and ends "…and are not asserted on the reload path. test."]
REPLACEMENT:
- AC11-1 (11/AC-1; R24): a config-validation pin rejection — the same malformed-pin document — reports the same core detail on the SIGHUP reload path and on the configuration-validation path. Carriers: reload → `config_reload_rejected` (AD7; the `*config.Diagnostics` branch of `operatorLog`, wiring.go:428-438, first record per AD4); configuration validation → `config_validation_failed` (AD7/AD14, operation `load_config`) — both rendered by the ONE shared AD3 renderer. Compared key forms: `config.property_path` with one-based `config.line`/`config.column` on every compared pin class; `config.pin_server`/`config.pin_tool` on both sides only where the diagnostic class carries validated pin identity — absent for the `pin_fields`/`pin_server_missing`/`pin_tool_missing` classes (config.go:222-322). A RUNTIME pinned-catalog rejection on the reload leg emits `reload_rejected` (internal_error, no `config` object, wiring.go:461-467); parity for it is DEFERRED to backlog ticket 19, not asserted here. test.

H1c · spec.md:261 (AC11-2) — aligned to the same class.
CURRENT:
- AC11-2 (11/AC-2; R24): neither surviving path's pin failure is obtainable only through SIGHUP — the periodic refresh record alone carries the full AC11-1 detail set. test.
REPLACEMENT:
- AC11-2 (11/AC-2; R24): neither surviving path's pin failure is obtainable only through SIGHUP — the configuration-validation record (`config_validation_failed`, operation `load_config`) alone carries the full AC11-1 detail set. test.

L3 · spec.md:273 (r8 ticket-mapping prose) — made to match the strictly serial ledger (T10 runs after T9).
CURRENT:
Ticket mapping (r8 extension): T8↔ticket 08, T9↔09, T10↔10, T11↔11, T12↔12. Dependencies: T9 and T10 after T8; T11 after T9+T10; T12 after T9+T10+T11. Dispatch STRICTLY SERIAL in ticket order, continuing the r7 rule: T8→T9→T10→T11→T12 (after T7). One engineer owns production code and permanent tests per task through RED/GREEN.
REPLACEMENT:
Ticket mapping (r8 extension): T8↔ticket 08, T9↔09, T10↔10, T11↔11, T12↔12. Dependencies: T9 after T8; T10 after T9; T11 after T9+T10; T12 after T9+T10+T11. Dispatch STRICTLY SERIAL in ticket order, continuing the r7 rule: T8→T9→T10→T11→T12 (after T7). One engineer owns production code and permanent tests per task through RED/GREEN.

L1a+H1d · spec.md:284 (T10 prohibited list) — envelope-wide clauses added; nothing else on the line changes.
CURRENT:
- T10 — Discovery attribution and readiness reason. Deps: T8. Inputs: ticket 10, rulings 8, 9, AD17, defect sites wiring.go `publishObserver` (readiness only on publish) and ops.go empty `Reason`. Files: `internal/upstream/` discovery-site (exact file located from evidence; expected `internal/upstream/dedicated.go` — else BLOCKED-with-evidence), `internal/service/refresh.go` (retryable/next attempt), `internal/service/ops.go` (finite Reason classes populated), `cmd/mcp-bridge/wiring.go` (`publishObserver` updates readiness on rejection), upstream discovery test (location per site), `cmd/mcp-bridge/serve_edge_test.go` (readiness assertions). RED: discovery-failure test asserts stage/status/endpoint attribution — expected failure. Deliverable: AC10-1..5. Verify: `go test ./internal/service ./internal/upstream ./cmd/mcp-bridge -run 'Discovery|Readiness|Refresh'`, `go vet ./...`, `gofmt -l` changed files. Prohibited: no credentials/headers/query/body in any field; endpoint sanitizer emits scheme/host/port/path only; no metric changes.
REPLACEMENT:
- T10 — Discovery attribution and readiness reason. Deps: T8. Inputs: ticket 10, rulings 8, 9, AD17, defect sites wiring.go `publishObserver` (readiness only on publish) and ops.go empty `Reason`. Files: `internal/upstream/` discovery-site (exact file located from evidence; expected `internal/upstream/dedicated.go` — else BLOCKED-with-evidence), `internal/service/refresh.go` (retryable/next attempt), `internal/service/ops.go` (finite Reason classes populated), `cmd/mcp-bridge/wiring.go` (`publishObserver` updates readiness on rejection), upstream discovery test (location per site), `cmd/mcp-bridge/serve_edge_test.go` (readiness assertions). RED: discovery-failure test asserts stage/status/endpoint attribution — expected failure. Deliverable: AC10-1..5. Verify: `go test ./internal/service ./internal/upstream ./cmd/mcp-bridge -run 'Discovery|Readiness|Refresh'`, `go vet ./...`, `gofmt -l` changed files. Prohibited: no credentials/headers/query/body in any field; endpoint sanitizer emits scheme/host/port/path only; no metric changes; no schema_version increment; no debug-only diagnostics; no induced-failure test infrastructure.

L1b+H1e · spec.md:285 (T11 RED realigned to the config-validation pin class; prohibited list gains the envelope-wide clauses)
CURRENT:
- T11 — Path parity and logger fallback. Deps: T9+T10. Inputs: ticket 11, rulings 10, 16–18, AD21, trigger sources (loop.go watch, Refresher tick/Trigger). Files: `internal/service/loop.go` (reload revision/trigger attribution), `cmd/mcp-bridge/wiring.go` (`config_reload_rejected` +`config_revision`/`trigger`; `config_reload_applied` emission), `internal/observe/log.go`/`envelope.go` (+2 events, `logging` operation, fallback path), `internal/observe` fallback test (exact existing file located in T11 evidence; new `internal/observe/fallback_test.go` if none), `internal/service/parity_test.go` (new) + `loop_test.go` extensions. RED: parity test drives the SAME pinned-catalog rejection through the SIGHUP reload and periodic refresh paths and asserts equal detail — expected failure (the paths' detail differs today); fallback test asserts encode-failure surfaces `logging_write_failed` — expected failure. Deliverable: AC11-1..3. Verify: `go test ./internal/service ./internal/observe ./cmd/mcp-bridge -run 'Parity|Reload|Fallback'`, `go vet ./...`, `gofmt -l` changed files. Prohibited: no alteration of r7 record assertions (additive fields only, AD21); no recursion in the fallback (single attempt).
REPLACEMENT:
- T11 — Path parity and logger fallback. Deps: T9+T10. Inputs: ticket 11, rulings 10, 16–18, AD21, trigger sources (loop.go watch, Refresher tick/Trigger). Files: `internal/service/loop.go` (reload revision/trigger attribution), `cmd/mcp-bridge/wiring.go` (`config_reload_rejected` +`config_revision`/`trigger`; `config_reload_applied` emission), `internal/observe/log.go`/`envelope.go` (+2 events, `logging` operation, fallback path), `internal/observe` fallback test (exact existing file located in T11 evidence; new `internal/observe/fallback_test.go` if none), `internal/service/parity_test.go` (new) + `loop_test.go` extensions. RED: parity test drives the SAME config-validation pin rejection document through the SIGHUP reload path and the configuration-validation path (`--check-config`) and asserts equal AC11-1 key forms plus `config_revision`/`trigger` on the reload record — expected failure (`config_reload_rejected` carries no `config_revision`/`trigger` today and the parity test does not exist); fallback test asserts encode-failure surfaces `logging_write_failed` — expected failure. Deliverable: AC11-1..3. Verify: `go test ./internal/service ./internal/observe ./cmd/mcp-bridge -run 'Parity|Reload|Fallback'`, `go vet ./...`, `gofmt -l` changed files. Prohibited: no alteration of r7 record assertions (additive fields only, AD21); no recursion in the fallback (single attempt); no metric additions or changes (`bridge_catalog_refresh_total` untouched); no schema_version increment; no debug-only diagnostics; no induced-failure test infrastructure.

════ 2. TICKET WHOLE-LINE REPLACEMENTS ════

issues/10-discovery-attribution-and-readiness-reason.md:17 (H2 — AC-4 checklist line mirrors AD17's iff; only the prose at :9 was fixed by narrow3)
CURRENT:
- [ ] AC-4: Each failure states automatic-retry-scheduled or human-correction-required, with the next attempt time.
REPLACEMENT:
- [ ] AC-4: Each failure states automatic-retry-scheduled or human-correction-required, with the next attempt time present only when an automatic retry is scheduled (per AD17).

issues/08-refresh-pass-lifecycle-records.md:14 (M2 — AC-1 mirrors narrowed spec:240 incl. the M1 vacuous-clause deletion)
CURRENT:
- [ ] AC-1: A refresh pass whose candidate is rejected emits one failure record (`catalog_refresh_failed`) at the AD16-pinned level — warn when the retained catalog keeps serving, error when serving zero upstream tools — with debug disabled.
REPLACEMENT:
- [ ] AC-1: A refresh pass whose candidate is rejected emits exactly one pass-level failure record (`catalog_refresh_failed`); per-pin `pin_blocker_detail` records are additional and not counted by this oracle — warn when the retained catalog keeps serving, error when serving zero upstream tools.

issues/09-pin-blocker-detail.md:18 (M2 — AC-5 mirrors narrowed spec:251's own-oracle ≤64 bound)
CURRENT:
- [ ] AC-5: For each server with a missing pin, the actual discovered tool names are reported; large lists are split across records carrying a shared refresh ID, server ID and chunk index/count with no silent truncation.
REPLACEMENT:
- [ ] AC-5: For each server with a missing pin, the actual discovered tool names are reported; large lists are split across records carrying a shared refresh ID, server ID and chunk index/count, each chunk record carrying at most 64 names (this AC's own oracle bound, asserted per chunk record) so `chunk_count` ≥ 2 whenever a listing exceeds 64, with no silent truncation.

issues/12-contract-docs-and-verification-closeout.md:14 (M2 — AC-3 mirrors narrowed spec:265)
CURRENT:
- [ ] AC-3: The full suite passes.
REPLACEMENT:
- [ ] AC-3: The full suite passes and no doc drift remains — the AC-1 doc-drift test is the mechanical oracle for documented-contract-matches-emission.

issues/11-path-parity-and-logger-fallback.md:7 (H1 mirror — prose mirrors retargeted R24:64)
CURRENT:
SIGHUP reload and periodic refresh report the same core pin diagnostic detail: identities and configuration source locations (`pins[i]` property path); chunked discovery evidence and retry guidance are refresh-path records. No path may be the only way to obtain the useful error, so an operator never needs to send a signal to a running process to learn what a pinned tool rejection means.
REPLACEMENT:
A config-validation pin rejection reports the same core pin diagnostic detail on the SIGHUP reload path (`config_reload_rejected`) and the configuration-validation path (`config_validation_failed`): the `pins[i]` property path with position, plus pin identity only where the diagnostic class carries validated identity; a runtime pin rejection on the reload leg is deferred to backlog ticket 19. No path may be the only way to obtain the useful error, so an operator never needs to send a signal to a running process to learn what a pinned tool rejection means.

issues/11-path-parity-and-logger-fallback.md:12 (H1 mirror — AC-1 mirrors retargeted AC11-1:260)
CURRENT:
- [ ] AC-1: A pin rejection on a SIGHUP reload and on a periodic refresh reports the same core detail — pin identity and source location (`pins[i]` property path); discovery evidence and retry guidance remain refresh-path records.
REPLACEMENT:
- [ ] AC-1: A config-validation pin rejection (the same malformed-pin document) reports the same core detail on the SIGHUP reload path (`config_reload_rejected`) and the configuration-validation path (`config_validation_failed`): the `pins[i]` property path with position, plus pin identity only where the diagnostic class carries validated identity (absent for `pin_fields`/`pin_server_missing`/`pin_tool_missing`); runtime reload parity is deferred to backlog ticket 19.

issues/11-path-parity-and-logger-fallback.md:13 (H1 mirror — AC-2 mirrors retargeted AC11-2:261)
CURRENT:
- [ ] AC-2: Neither surviving path's pin failure is obtainable only through a SIGHUP reload — the periodic refresh record alone carries the full detail set.
REPLACEMENT:
- [ ] AC-2: Neither surviving path's pin failure is obtainable only through a SIGHUP reload — the configuration-validation record (`config_validation_failed`) alone carries the full AC-1 detail set.

════ 3. APPEND LINES ════

NONE. backlog.md needs no row: the runtime reload-leg parity deferred by H1 is already carried by the existing ticket-19 row, whose text remains accurate (it asks which AD licenses reload-side detail and what record carries it — exactly the new-AD question H1 forbids opening). issues/17 needs no note: this delta adds no new artifact drift, and the M2 ticket-lag drift is closed by §2 rather than recorded.

════ 4. DECISIONS.MD APPEND ════

YES — one numbered append, following the narrow1/narrow2/narrow3 precedent in the existing "## r8 narrowing — owner decisions (2026-11-11)" section, directly after item 4. Exact line to append as item 5:
5. **narrow4 authorized; M3 declined.** The owner authorized revision `r8-pin-diagnostics-narrow4` (2026-11-11): close code-reviewer REQUEST CHANGES packet `db7551a19d53ee518e0aaf975f4796e44cc9b7beaaeabb7361a57f2a6692d503` rows H1–H2, M1–M3, L1–L3, narrowing only — no R/AC/T id added or removed, no new AD, no new acceptance criterion, no widened envelope; if the revision still fails review the coordinator stops and reports. **M3 DECLINED, recorded not implemented:** the reviewer asked for an AC covering AD21's `config_reload_applied` and the `config_revision`/`trigger` fields on `config_reload_rejected` (review §2: "AD21's `config_reload_applied` and revision/trigger on `config_reload_rejected` (:203) have no AC"); an AC per AD would add a new acceptance-criterion id, outside the narrowing authorization, so the decline is recorded here and the open question stays with backlog ticket 15 ("Which AC asserts `config_reload_applied` and the additive rejected-half fields?").
As with narrow3, NO `src-decisions` digest re-bind is claimed for this append (the state has no `src-decisions` source entry; see issues/17 narrow3 additions — ticket-17 scope).

════ 5. REVISION ID AND ID INTEGRITY ════

Revision id to record: r8-pin-diagnostics-narrow4 (set at spec.md:1 and :34, and in the state's revision + sectionBindings by the coordinator, per the same sanctioned suffix-amendment path as r7→r7-closure1 and r8→narrow1→narrow2→narrow3).
Confirmation: NO requirement, acceptance or task id was added or removed. R1–R25 keep their ids (R23, R24 reworded in place); AC01-1…AC12-3 keep their ids (AC08-1, AC08-7, AC11-1, AC11-2 narrowed in place); T1–T12 keep their ids (T11's RED reworded in place; T10/T11 prohibited clauses extended; :273 dependency prose aligned with the existing strict-serial rule — no edge widened); AD1–AD21 untouched; the Coverage (r8) line :267 needs no edit (R24→AC08-2/AC11-1..2 still holds). Backlog is unchanged — ticket 19 already carries the H1 deferral; tickets 13–18 untouched; ticket 15 remains the M3 home. Evidence anchors used in the replacements (wiring.go:428-438, wiring.go:461-467, config.go:222-322, reload.go AddPins wrap) are VERIFIED facts from review packet db7551a1… §2b/§6; no new evidence was gathered and no test was run.