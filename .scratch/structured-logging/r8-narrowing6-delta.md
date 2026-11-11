NARROWING DELTA - r8-pin-diagnostics-narrow5 -> r8-pin-diagnostics-narrow6
════ 1. SPEC.MD PER-LINE DELTAS ════
D0a · spec.md:1 (header, revision bump)
CURRENT:
# SPEC-STRUCTURED-LOGGING r8-pin-diagnostics-narrow5 — STATUS: DRAFT
REPLACEMENT:
# SPEC-STRUCTURED-LOGGING r8-pin-diagnostics-narrow6 — STATUS: DRAFT
D0b · spec.md:34 (§1 STATUS, revision bump; trailing 'Objective: …' sentence unchanged)
CURRENT:
SPEC-STRUCTURED-LOGGING revision r8-pin-diagnostics-narrow5. DRAFT (narrowing amendment narrow5 per owner decision 2026-11-11: closes the round-6 residuals — AC09-1/AC10-1 level assertions, T9 debug clause and AddPins scope, induced-failure wording, AD19 count, AD21 fallback term, AC12-3 oracle, AC10-5 hook — narrowing only; ready for critic review; execution requires the workflow's explicit user checkpoints). Objective: r7 objective plus refresh-path pin diagnosability from logs alone.
REPLACEMENT:
SPEC-STRUCTURED-LOGGING revision r8-pin-diagnostics-narrow6. DRAFT (narrowing amendment narrow6 per owner decision 2026-11-11: mirrors the narrow5 AC repairs into tickets 09/10/12, corrects the AC10-5 hook-site wording and completes the R25 coverage mapping, narrowing only). Objective: r7 objective plus refresh-path pin diagnosability from logs alone.
AC10-5 HOOK SITE WORDING · spec.md:259 (corrects "failed-pass branch": refresh.go:211 is the success guard; the failed pass falls through to runtime.Current())
CURRENT:
- AC10-5 (10/AC-5; R23): after a rejected pass the affected server's readiness `Reason` is non-empty and identifies the failure, distinct from not-yet-probed; this criterion depends on the hook site `internal/service/refresh.go:211` (the failed-pass branch of `report(assembly, err)` whose `err` is currently unread) reached via the T10 `publishObserver` readiness update. test.
REPLACEMENT:
- AC10-5 (10/AC-5; R23): after a rejected pass the affected server's readiness `Reason` is non-empty and identifies the failure, distinct from not-yet-probed; this criterion depends on the hook site `internal/service/refresh.go:211` (the success guard of `report(assembly, err)` whose `err` is currently unread; a failed pass falls through to `runtime.Current()`) reached via the T10 `publishObserver` readiness update. test.
COVERAGE R25 MAPPING · spec.md:267 (adds the two missing ACs to the R25 mapping)
CURRENT:
Coverage (r8): R20→AC08-1/AC09-7; R21→AC08-6/AC09-1..6,8; R22→AC08-1..6; R23→AC10-1..5; R24→AC08-2/AC11-1..2; R25→AC08-1,7/AC11-3. AD20 ruling (no schema_version change) is itself asserted by AC12-1.
REPLACEMENT:
Coverage (r8): R20→AC08-1/AC09-7; R21→AC08-6/AC09-1..6,8; R22→AC08-1..6; R23→AC10-1..5; R24→AC08-2/AC11-1..2; R25→AC08-1,7/AC09-1/AC10-1/AC11-3. AD20 ruling (no schema_version change) is itself asserted by AC12-1.
════ 2. TICKET WHOLE-LINE REPLACEMENTS ════
TICKET 09 AC-1 · issues/09-pin-blocker-detail.md:14 (mirrors spec.md:247 content, including the `level` `warn` obligation per AD18/ruling 16; ticket checkbox format preserved)
CURRENT:
- [ ] AC-1: A candidate with several unresolved mandatory pins reports all of them, each with server and tool identity, not only the first.
REPLACEMENT:
- [ ] AC09-1 (09/AC-1; R21): several unresolved mandatory pins → ALL reported via `pin_blocker_detail` (AD18), each with server and tool identity, not only the first; every such record carries `level` `warn` per AD18/ruling 16 — a `pin_blocker_detail` emitted at any other level (in particular `debug`) fails this criterion. test.
TICKET 10 AC-1 · issues/10-discovery-attribution-and-readiness-reason.md:14 (mirrors spec.md:255 content, including the AD-pinned levels; ticket checkbox format preserved)
CURRENT:
- [ ] AC-1: A discovery failure names the failing stage and a bounded cause category, distinguishing at minimum an authentication failure during initialize from a tool-listing timeout.
REPLACEMENT:
- [ ] AC10-1 (10/AC-1; R23): a discovery failure names `failure_stage` and a bounded `failure_reason`, distinguishing auth-during-initialize from tools-list timeout; the emitted records carry the AD-pinned levels — `upstream_discovery_failed` at `warn` (AD17) and, per the pass outcome, the accompanying `catalog_refresh_failed` at `warn` with `retained_published` or `error` with `none_available` (AC08-1/AC08-7) — a diagnostic at any other level (in particular `debug`) fails this criterion. test.
TICKET 12 AC-3 · issues/12-contract-docs-and-verification-closeout.md:14 (mirrors spec.md:265 content: go test ./... is the own oracle; doc-drift half belongs to AC12-1; ticket checkbox format preserved)
CURRENT:
- [ ] AC-3: The full suite passes and no doc drift remains — the AC-1 doc-drift test is the mechanical oracle for documented-contract-matches-emission.
REPLACEMENT:
- [ ] AC12-3 (12/AC-3; R20–R25): the full suite passes — the `go test ./...` exit-0 run is this criterion's OWN oracle (not AC12-1's); the documented-contract-matches-emission half is FOLDED into AC12-1's doc-drift test and is not separately asserted here. test (`go test ./...`).
════ 3. APPEND LINES ════
NONE.
════ 4. DECISIONS.MD APPEND ════
NONE.
════ 5. REVISION ID AND ID INTEGRITY ════
Revision advances r8-pin-diagnostics-narrow5 -> r8-pin-diagnostics-narrow6. No R, AC or T id is added or removed; AD1–AD21 are untouched. The Coverage line (spec.md:267) is updated only by adding the two missing ACs to the R25 mapping (R25→AC08-1,7/AC09-1/AC10-1/AC11-3); all other mappings are unchanged. The three ticket AC lines mirror the spec AC lines they correspond to (spec.md:247/:255/:265) with the ticket checkbox prefix preserved, so ticket and spec agree 1:1 after application; the only spec-side prose change is the AC10-5 hook-site wording correction backed by refresh.go:210-241 (err compared only at the :211 success guard; a failed pass falls through to runtime.Current()).
