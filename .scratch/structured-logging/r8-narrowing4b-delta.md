NARROWING DELTA — r8-pin-diagnostics-narrow4 → r8-pin-diagnostics-narrow4b
Owner decision 2026-11-11 (narrow4b): completes the two spec rows the narrow4 transcription dropped (spec.md:284/:285; the planner's verbatim text below) and the row-M1 dependency alignment the narrow4 review found unrecorded (spec.md:284/:292, issues/10:4) plus the issues/08:20 AC-7 mirror. NARROWING ONLY — no R/AC/T id added or removed, no new AD, no new acceptance criterion, no widened envelope. Whole-line replacements only; no marker line is touched (40/67, 169/204, 271/287, 289/303 stay unique, ordered, non-nested).

════ 1. SPEC.MD PER-LINE DELTAS ════

D0a · spec.md:1 (header, revision bump)
CURRENT:
# SPEC-STRUCTURED-LOGGING r8-pin-diagnostics-narrow4 — STATUS: DRAFT
REPLACEMENT:
# SPEC-STRUCTURED-LOGGING r8-pin-diagnostics-narrow4b — STATUS: DRAFT

D0b · spec.md:34 (§1 STATUS, revision bump; trailing 'Objective: …' sentence unchanged)
CURRENT:
SPEC-STRUCTURED-LOGGING revision r8-pin-diagnostics-narrow4. DRAFT (narrowing amendment narrow4 per owner decision 2026-11-11: closes review packet db7551a1… rows H1–H2, M1–M3, L1–L3, narrowing only; ready for critic review; execution requires the workflow's explicit user checkpoints). Objective: r7 objective plus refresh-path pin diagnosability from logs alone.
REPLACEMENT:
SPEC-STRUCTURED-LOGGING revision r8-pin-diagnostics-narrow4b. DRAFT (narrowing amendment narrow4b per owner decision 2026-11-11: completes narrow4's transcribed rows H1/L1 at :284/:285 and the row-M1 dependency alignment, narrowing only; ready for critic review; execution requires the workflow's explicit user checkpoints). Objective: r7 objective plus refresh-path pin diagnosability from logs alone.

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

M1 · spec.md:284 (dependency alignment: T10 runs after T9 per the state edges, ticket 10 and :273)
CURRENT:
Deps: T8. Inputs: ticket 10
REPLACEMENT:
Deps: T9. Inputs: ticket 10

M1 · spec.md:292 (envelope edges aligned to the strict-serial ledger)
CURRENT:
r8 edges: T9,T10 after T8;
REPLACEMENT:
r8 edges: T9 after T8; T10 after T9;

════ 2. TICKET WHOLE-LINE REPLACEMENTS ════

issues/10-discovery-attribution-and-readiness-reason.md:4 (M1 — Blocked-by mirrors the T10 dependency)
CURRENT:
Blocked-by: 08
REPLACEMENT:
Blocked-by: 09

issues/08-refresh-pass-lifecycle-records.md:20 (M2 — AC-7 mirrors the narrowed spec AC08-7 at :246)
CURRENT:
- [ ] AC-7: No failure diagnostic is emitted only at debug level.
REPLACEMENT:
- [ ] AC-7: Every `catalog_refresh_failed` record carries `level` `warn` or `error` exactly as pinned by AC-1 — positively asserted on each emitted record (`warn` with `retained_published`, `error` with `none_available`).

════ 3. APPEND LINES ════

NONE.

════ 4. DECISIONS.MD APPEND ════

YES — one numbered append as item 6, after item 5:
6. **narrow4b authorized.** The owner authorized revision `r8-pin-diagnostics-narrow4b` (2026-11-11, in answer to the narrow4 review report `review-code-reviewer-narrow4-final.md`): apply the two spec rows the narrow4 transcription dropped — the planner's verbatim :284/:285 replacements — plus the row-M1 dependency alignment (:284 `Deps: T9`, :292 edges, issues/10:4 `Blocked-by: 09`) and the issues/08:20 AC-7 mirror; narrowing only — no R/AC/T id added or removed, no new AD, no new acceptance criterion, no widened envelope; one review round, then the critic, then T8–T12; if that review returns revise the coordinator stops and reports.

════ 5. REVISION ID AND ID INTEGRITY ════

Revision id: `r8-pin-diagnostics-narrow4b`. NO requirement, acceptance or task id added or removed: R1–R25, AC01-1…AC12-3, T1–T12, AD1–AD21 keep their ids; Coverage (r8) :267 unchanged; the only semantic edits are the two rows narrow4 already authorized (`T10`/`T11` ledger lines) and the dependency alignment the narrow4 review's fix sentence specifies.