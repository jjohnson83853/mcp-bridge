NARROWING DELTA — r8-pin-diagnostics → r8-pin-diagnostics-narrow1
Owner decision 2026-11-11 (option 2): narrow r8 to what it can assert; defer the rest to backlog tickets 13–18. No new R/AC/T IDs. Every delta is a whole-line replacement; the four specdd marker pairs (spec.md requirements 67 / design 203 / tasks 286 / envelope 302) are not touched, so they remain unique, ordered, non-nested, each on its own line.

════════ 1. PER-LINE DELTAS (spec.md) ════════

— §2 REQUIREMENTS —

D1 · R21 (spec.md:61)
CURRENT: "- R21 (owner rulings 2, 3, 4, 13): A failed pin check reports EVERY unresolved pin — never only the first — each with its safe server and tool identity (R8-validated) and its configuration source location (file, property path, one-based line, one-based column)."
REPLACEMENT: "- R21 (owner rulings 2, 3, 4, 13): A failed pin check reports EVERY unresolved pin — never only the first — each with its safe server and tool identity (R8-validated) and its configuration source location (property path)."
REST OF LINE (discovery-failed → UNVERIFIABLE with shared cause; discovered-tools chunking with refresh_id/server/chunk_index/chunk_count; every failed pass repeats complete details) UNCHANGED.
REASON: refresh-time pins are `[]map[string]string` (config.go:66); file/line/column are unobtainable without the ticket-16 envelope widening. Property path (`pins[i]`) is derivable from the pin index.

D2 · R22 (spec.md:62) — two trims in one line
CURRENT (clauses): "…stating the configuration revision, the trigger, and the next scheduled attempt. A failed pass distinguishes the retained catalog (generation, tool count, age) from no-usable-catalog/zero-upstream-tools, and distinguishes candidate counts from retained counts. …"
REPLACEMENT: "…stating the configuration revision and the trigger, and — on the start record — the next scheduled attempt. A failed pass distinguishes the retained catalog (generation, tool count) from no-usable-catalog/zero-upstream-tools. …"
ALL OTHER CLAUSES OF R22 UNCHANGED (logs-alone head clause; success counts-only; stalled-vs-waiting).
REASON: AD16's `catalog_refresh_completed` carries no `next_attempt_ms` (review §2); `retained_age_ms` and candidate counts are designed (AD16) but asserted by no AC (AC08-3 asserts catalog_state + retained_generation + catalog_tools only). Ticket 14 rules on extending the ACs; ticket 18 rules the counter-gap context.

D3 · R24 (spec.md:64)
CURRENT: "- R24 (owner rulings 10, 18): The log shows a configuration edit reaching the running process: reload attempts identify the configuration revision and trigger and whether the candidate was accepted or rejected; refresh records name the revision they used. Startup, SIGHUP reload and periodic refresh report IDENTICAL pin diagnostic detail — identity, source location, discovery evidence, publication impact, retry guidance; no path is the only way to obtain the useful error."
REPLACEMENT: "- R24 (owner rulings 10, 18): The log shows a configuration edit reaching the running process: refresh records name the configuration revision and trigger they used. SIGHUP reload and periodic refresh report IDENTICAL pin diagnostic detail — identity, source location, discovery evidence, publication impact, retry guidance; no path is the only way to obtain the useful error."
REASON: startup performs NO pin evaluation (reload.go:554-560 sets `next.pins = nil`) — ticket 13; the reload accepted/rejected halves (`config_reload_applied` + additive rejected fields) are asserted by no AC — ticket 15. AD21 keeps both designed (see G2); T11 keeps emitting them.

— §4 DESIGN —

D4 · AD18 (spec.md:196), per-pin field list
CURRENT: "…per-pin record carries `refresh_id`, typed `server`, `config.pin_tool`, `config.property_path`, `config.line`, `config.column` (REUSED R6 keys, unchanged semantics — the pin's configuration source), `blocker_class` …"
REPLACEMENT: "…per-pin record carries `refresh_id`, typed `server`, `config.pin_tool`, `config.property_path` (the pin's configuration source, as the `pins[i]` index the loader retains), `blocker_class` …"
REST OF AD18 (blocker_class ∈ {tool_absent, server_unverified}; chunk records with discovered_tools/chunk_index/chunk_count, no silent truncation) UNCHANGED.
REASON: same ticket-16 evidence as D1 — the fields are unimplementable inside the r8 write envelope (internal/config absent from it). `config.line/column` are reused R6 keys, so AD20's new-vocabulary list is unaffected.
AD16, AD17, AD19, AD20, AD21: NO CHANGES (see §3 for the deliberately kept unasserted design fields).

— §5 ACCEPTANCE CRITERIA —

D5 · AC08-1 (spec.md:239)
CURRENT: "- AC08-1 (08/AC-1; R20, R22, R25): a refresh pass whose candidate is rejected emits one failure record (`catalog_refresh_failed`) at warn or error with debug disabled. test."
REPLACEMENT: "- AC08-1 (08/AC-1; R20, R22, R25): a refresh pass whose candidate is rejected emits one failure record (`catalog_refresh_failed`) at the AD16-pinned level — warn when `catalog_state` is `retained_published`, error when `none_available` — with debug disabled. test."
REASON: review §3 — "warn or error" can never fail; AD16 already pins level by catalog_state.

D6 · AC08-2 (spec.md:240)
CURRENT: "- AC08-2 (08/AC-2; R22, R24): start and outcome records share `refresh_id` and each carries `config_revision`, `trigger`, `next_attempt_ms`. test."
REPLACEMENT: "- AC08-2 (08/AC-2; R22, R24): start and outcome records share `refresh_id` and each carries `config_revision`, `trigger`; the start record carries `next_attempt_ms`. test."
REASON: AD16 defines `next_attempt_ms` on start and failed records, not on the success summary; extending it is ticket 14's design question.

D7 · AC09-2 (spec.md:249)
CURRENT: "- AC09-2 (09/AC-2; R21, R6): each reported pin carries file, `config.property_path`, one-based `config.line`, one-based `config.column`. test."
REPLACEMENT: "- AC09-2 (09/AC-2; R21, R6): each reported pin carries `config.property_path` (the pin's `pins[i]` path). test."
R6 binding KEPT (property_path is an R6 key).

D8 · AC09-8 (spec.md:253)
CURRENT: "- AC09-8 (09/AC-8; R21): all evidence in one failure correlates to one `refresh_id` and one `config_revision`; no result mixes catalog generations. test."
REPLACEMENT: "- AC09-8 (09/AC-8; R21): all evidence in one failure correlates to one `refresh_id` and, through the shared `catalog_refresh_failed` record, to that pass's `config_revision`; no result mixes catalog generations. test."
REASON: AD18 does not carry `config_revision` on per-pin records; correlation runs via the pass record AD16 defines (its config_revision is asserted by AC08-2).

D9 · AC10-2 (spec.md:258)
CURRENT: "- AC10-2 (10/AC-2; R23): `http_status` or the timeout value reported where applicable. test."
REPLACEMENT: "- AC10-2 (10/AC-2; R23): `http_status` is present exactly when the failing stage produced an HTTP status; timeout failures instead name the timeout class and value in `failure_reason` (per AD17). test."
REASON: review §3 — "where applicable" left the oracle open. AD17 prose keeps "where applicable"; the AC oracle governs.

D10 · AC11-1 (spec.md:259)
CURRENT: "- AC11-1 (11/AC-1; R24): a pin rejection at startup, on SIGHUP reload and on periodic refresh reports the same detail set: identity, source location, discovery evidence, publication impact, retry guidance. test."
REPLACEMENT: "- AC11-1 (11/AC-1; R24): a pin rejection on SIGHUP reload and on periodic refresh reports the same detail set: identity, source location, discovery evidence, publication impact, retry guidance. test."

D11 · AC11-2 (spec.md:260)
CURRENT: "- AC11-2 (11/AC-2; R24): no path's pin failure is obtainable only through SIGHUP. test."
REPLACEMENT: "- AC11-2 (11/AC-2; R24): neither surviving path's pin failure is obtainable only through SIGHUP — the periodic refresh record alone carries the full AC11-1 detail set. test."
REASON D10/D11: ticket 13 — startup never evaluates pins; the parity set is reload↔refresh. AC11-2 stays as the independent obtainability oracle, not a restatement.

D12 · AC12-3 (spec.md:267)
CURRENT: "- AC12-3 (12/AC-3; R20–R25): the full suite passes and documented contract matches producer emission. test (`go test ./...`)."
REPLACEMENT: "- AC12-3 (12/AC-3; R20–R25): the full suite passes and no doc drift remains — the AC12-1 doc-drift test is the mechanical oracle for documented-contract-matches-emission. test (`go test ./...`)."

— §6 TASK LEDGER —

D13 · T8 (spec.md:281) — deliverable clause
CURRENT fragment: "Deliverable: AD16 records; completion summary counts-only."
REPLACEMENT: "Deliverable: AC08-1..7 (AD16 records; completion summary counts-only)."
Everything else on the T8 line unchanged.

D14 · T9 (spec.md:282) — RED wording + deliverable clause
CURRENT fragment: "RED: multi-unresolved-pin test asserts ALL pins reported with source locations — expected failure (first-only/text-only today). Verify: …"
REPLACEMENT: "RED: multi-unresolved-pin test asserts ALL pins reported with `config.property_path` identity — expected failure (first-only/text-only today). Deliverable: AC09-1..8 (AC09-2 asserting `config.property_path` only, per the narrowed revision). Verify: …"
T9 files/scope otherwise UNCHANGED.

D15 · T10 (spec.md:283) — deliverable clause
CURRENT fragment: "RED: discovery-failure test asserts stage/status/endpoint attribution — expected failure. Verify: …"
REPLACEMENT: "RED: discovery-failure test asserts stage/status/endpoint attribution — expected failure. Deliverable: AC10-1..5. Verify: …"
T10 scope otherwise UNCHANGED (its state-envelope drift — runtime.go, serve.go, dedicated_test.go — is ticket 17 and is NOT touched here).

D16 · T11 (spec.md:284) — startup-leg shrink + deliverable
CURRENT fragment (files): "…Files: `internal/service/loop.go` (reload revision/trigger attribution), `cmd/mcp-bridge/serve.go` (startup-path pin detail parity), `cmd/mcp-bridge/wiring.go` …"
REPLACEMENT (files): "…Files: `internal/service/loop.go` (reload revision/trigger attribution), `cmd/mcp-bridge/wiring.go` …" — the serve.go entry is DELETED.
CURRENT fragment (RED): "RED: parity test drives the SAME pinned-catalog rejection through startup and reload paths and asserts equal detail — expected failure (startup path lacks detail); fallback test asserts encode-failure surfaces `logging_write_failed` — expected failure. Verify: …"
REPLACEMENT (RED): "RED: parity test drives the SAME pinned-catalog rejection through the SIGHUP reload and periodic refresh paths and asserts equal detail — expected failure (the paths' detail differs today); fallback test asserts encode-failure surfaces `logging_write_failed` — expected failure. Deliverable: AC11-1..3. Verify: …"
T11 keeps: loop.go/wiring.go/log.go/envelope.go attribution + `config_reload_applied` emission (AD21, unasserted — G2), fallback path, parity_test.go, loop_test.go. T11 deps and Prohibited unchanged.

— HEADER / STATUS / §7 —

D17 · spec.md:1 title and spec.md:34 §1 STATUS
CURRENT :1: "# SPEC-STRUCTURED-LOGGING r7-closure1 — STATUS: DRAFT"
REPLACEMENT :1: "# SPEC-STRUCTURED-LOGGING r8-pin-diagnostics-narrow1 — STATUS: DRAFT"
CURRENT :34 fragment: "SPEC-STRUCTURED-LOGGING revision r8-pin-diagnostics. DRAFT (ready for critic review; execution requires the workflow's explicit user checkpoints)."
REPLACEMENT :34: "SPEC-STRUCTURED-LOGGING revision r8-pin-diagnostics-narrow1. DRAFT (narrowing amendment per owner decision 2026-11-11, option 2; ready for critic review; execution requires the workflow's explicit user checkpoints)." This also closes the review's LOW title/§1 mismatch, which the bump forces anyway.

D18 · §7 Prometheus bullet ("Prometheus: STILL no new metrics …")
CURRENT fragment: "…the misleading `bridge_catalog_refresh_total{outcome="none"}` fallback on rejected passes REMAINS (known accepted gap, r8 changelog); operators must query logs, not this counter, for rejection state — stated in T12 docs."
REPLACEMENT fragment: "…the misleading `bridge_catalog_refresh_total{outcome="none"}` fallback on rejected passes REMAINS (known gap; acceptance pending owner ruling, backlog ticket 18 — r8 changelog); operators must query logs, not this counter, for rejection state — stated in T12 docs."
REASON: decisions.md:162 says ACCEPTED, owner-authorization-r8.txt Q3 says pending; the spec cannot assert an unrecorded acceptance.

NO OTHER LINES CHANGE. R20, R23, R25, AD16, AD17, AD19, AD20, AD21, AC08-3..7, AC09-1/3..7, AC10-1/3..5, AC11-3, AC12-1/2, T12, both Coverage lines, the scope-envelope lines, §8 and §9 stand as written.

════════ 2. COVERAGE MAP AFTER DELTAS ════════

The "Coverage (r8)" line (spec.md:266 region) remains ID-accurate with NO edit needed:
R20→AC08-1/AC09-7 · R21→AC08-6/AC09-1..6,8 · R22→AC08-1..6 · R23→AC10-1..5 · R24→AC08-2/AC11-1..2 · R25→AC08-1,7/AC11-3 · (AD20 no-schema_version-change ruling asserted by AC12-1).
Clause-level meaning after narrowing: R21 source location = property_path only; R22 next-attempt = start record only, retained = generation+count (no age, no candidate counts); R24 revision/trigger = refresh records only, parity = reload↔refresh.

════════ 3. RESIDUAL COVERAGE GAPS (owner ruling required — not narrowed away, none invented) ════════

G1 · AD16 `catalog_refresh_failed` fields `candidate_servers`, `candidate_tools`, `retained_age_ms` (and `catalog_refresh_completed` lacking `next_attempt_ms`). Clause origin: R22 rulings 12/7 (trimmed in D2, but the design still emits these fields). Why no AC asserts them today: AC08-3/AC08-2 were left unextended — extending them is exactly ticket 14's question; deleting the AD16 fields instead would be a design removal the owner has not ruled. Needed: ticket 14 ruling (extend ACs vs drop fields), then planner-authored AC/AD text.
G2 · AD21 `config_reload_applied` and the additive `config_revision`/`trigger` fields on `config_reload_rejected` — designed and emitted by T11, asserted by no AC (the review's suggested AC11-4 would be a NEW AC, which the owner withheld). Needed: ticket 15 ruling (which AC owns it; T11 vs separate ticket; `config_revision` vs `revision` naming).
G3 · Misleading-counter acceptance status (decisions.md:162 vs owner-authorization-r8.txt Q3). The spec text is trimmed to "pending" (D18), but only the owner can affirm acceptance and its terms. Needed: ticket 18 ruling; ticket 14's R22 restoration depends on it.
G4 · Startup pin-evaluation leg (R24 clause trimmed in D3). Needed: ticket 13 ruling — (a) startup leg = first post-startup pass narrated by AD16 records, or (b) explicit startup pin check in internal/service/reload.go with envelope entry + own RED test. Until ruled, no startup-parity assertion exists.
(Ticket 16 is consumed by D1/D4/D7/D8 — the narrowing implements its option (b); option (a), envelope widening into internal/config, remains available if the owner later prefers it.)

════════ 4. REVISION ID + LEDGER/EFFECT CHANGES ════════

Revision id to record: **`r8-pin-diagnostics-narrow1`** — the same sanctioned bump path as r7→r7-closure1 (suffix amendment on the pinned base, DRAFT until reviews pass). Set in spec.md:1 and :34 (D17) and in execution-state.json `revision`, with all four sectionBindings and the spec sha256 re-bound and packet.digest recomputed.

Spec ledger lines that change: exactly D13 (T8, spec.md:281), D14 (T9, :282), D15 (T10, :283), D16 (T11, :284). T12 (spec.md:285) unchanged.
Effect-path changes: exactly ONE — execution-state.json tasks[T11].effects.paths must drop `"cmd/mcp-bridge/serve.go"` (second entry in that array, ~line 404) to mirror D16. All other T8–T12 effect paths unchanged: T8 {refresh.go, log.go, envelope.go, refresh_test.go, refresh_lifecycle_test.go, receipt}, T9 {reload.go, refresh.go, log.go, envelope.go, pin_blocker_test.go, refresh_lifecycle_test.go, receipt}, T10 unchanged (its drift is ticket 17, not forced by any delta), T12 unchanged. The spec scope-envelope lines need NO change (serve.go remains in the r7 list for T3; parity/fallback test files unchanged). State acceptance entries need NO change (no AC id added/removed; AC11-1/2/3 keep R24/R24/R25). Not forced, therefore NOT included (ticket 17): T10 blockedBy ["T8"]→["T9"], AC12-1/2 method inspection→test, spec.md:76 state-revision claim, decisions.md:160,166 packet-digest lines.

════════ 5. MINIMAL ARTIFACT SET FOR RE-REVIEW ════════

Rebuild only: (1) spec.md with D1–D18 applied → recompute all four section digests + file sha256; (2) execution-state.json → revision, spec binding + sectionBindings, T11 effect-path removal, packet.digest; (3) decisions.md → append the r8-narrow owner decision + re-bind `src-decisions` sha256 (digest-refresh convention recorded there); (4) a fresh reviewer packet/binding for the code-reviewer re-run (review-runs.json gains a new run; nothing from the succeeded r8 run is reused or waived). Not rebuilt: backlog.md and issues/13–18 (stay deferred, untouched), owner-authorization-r8.txt (coordinator appends the narrowing authorization as an addendum only), spec-r7-closure1.md and spec-r8-additions-condensed.md (historical, byte-frozen).

Marker integrity note: D1–D18 replace whole lines strictly inside existing marker sections; no marker line is added, moved, or duplicated — the four pairs stay unique, correctly ordered, non-nested, one per line.