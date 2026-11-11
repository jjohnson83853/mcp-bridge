NARROWING DELTA — r8-pin-diagnostics-narrow1 → r8-pin-diagnostics-narrow2
Owner decision 2026-11-11 (narrow2): fix code-reviewer REQUEST CHANGES packet 15567c2b… findings 1–6, NARROWING ONLY — no R/AC/T id added or removed, no new AD, no new acceptance criterion, no widened envelope. Ticket-17 drift is explicitly OUT of scope (see §5 sentence). All spec changes are whole-line replacements strictly inside existing marker sections (or one appended §3 line outside all markers), so the four specdd marker pairs stay unique, ordered, non-nested, each on its own line. Line numbers cite narrow1; the single §3 insertion shifts all later line numbers by +1.

════ 1. SPEC.MD PER-LINE DELTAS ════

F2a · spec.md:1 (header)
CURRENT: "# SPEC-STRUCTURED-LOGGING r8-pin-diagnostics-narrow1 — STATUS: DRAFT"
REPLACEMENT: "# SPEC-STRUCTURED-LOGGING r8-pin-diagnostics-narrow2 — STATUS: DRAFT"

F2b · spec.md:34 (§1 STATUS)
CURRENT fragment: "SPEC-STRUCTURED-LOGGING revision r8-pin-diagnostics-narrow1. DRAFT (narrowing amendment per owner decision 2026-11-11, option 2; ready for critic review; execution requires the workflow's explicit user checkpoints)."
REPLACEMENT: "SPEC-STRUCTURED-LOGGING revision r8-pin-diagnostics-narrow2. DRAFT (narrowing amendment narrow2 per owner decision 2026-11-11: closes review packet 15567c2b… findings 1–6, narrowing only; ready for critic review; execution requires the workflow's explicit user checkpoints)."

F3 · spec.md:250 (AC09-5) — finding 3: pin the chunk bound so a single chunk_count=1 record can no longer pass.
CURRENT: "- AC09-5 (09/AC-5; R21): for each server with a missing pin, `discovered_tools` listed; large lists split across records sharing `refresh_id`, `server`, `chunk_index`/`chunk_count`, no silent truncation. test."
REPLACEMENT: "- AC09-5 (09/AC-5; R21): for each server with a missing pin, `discovered_tools` listed; large lists split across records sharing `refresh_id`, `server`, `chunk_index`/`chunk_count`; each chunk record carries at most 64 `discovered_tools` names (the design's existing ≤64 bound), so `chunk_count` ≥ 2 whenever a server's listing exceeds 64; no silent truncation. test."
NOTE: the bound is stated in the AC only — amending AD18 would be a design change; 64 reuses the only bounded constant the design already carries (pin ceiling). No AD added.

F4 · spec.md:257 (AC10-4) — finding 4: resolve the contradiction with AD17's conditional.
CURRENT: "- AC10-4 (10/AC-4; R23): each failure states retry-scheduled vs correction-required (`error.retryable` + `next_attempt_ms`). test."
REPLACEMENT: "- AC10-4 (10/AC-4; R23): each failure states retry-scheduled vs correction-required (`error.retryable`), with `next_attempt_ms` present iff an automatic retry is scheduled (per AD17). test."

F2 · spec.md:259 (AC11-1) — finding 2. CHOSEN: narrow the parity claim to what the reload path can actually emit. WHY: AD18 pins the only full-detail record (`pin_blocker_detail`) to operation `refresh`; the design defines NO reload-path record carrying discovery evidence (chunked `discovered_tools`) or retry guidance (`next_attempt_ms`), so binding AC11-1 to a shared record would require widening AD18 — a new design element, which narrow2 prohibits. The reload path's design-defined pin facts are T9's structured identity (reload.go), `config.property_path` and `publication_impact`.
CURRENT: "- AC11-1 (11/AC-1; R24): a pin rejection on SIGHUP reload and on periodic refresh reports the same detail set: identity, source location, discovery evidence, publication impact, retry guidance. test."
REPLACEMENT: "- AC11-1 (11/AC-1; R24): a pin rejection on SIGHUP reload and on periodic refresh reports the same core detail — pin identity (server, tool), source location (`config.property_path`), publication impact (`publication_impact`); discovery evidence (chunked `discovered_tools`) and retry guidance (`next_attempt_ms`) remain refresh-path records (AD18/AD16/AD17) and are not asserted on the reload path. test."
AC11-2 (:260) UNCHANGED (not among findings 1–6).

F5a · spec.md:200 (AD20) — finding 5: `failure_stage` is not new vocabulary.
CURRENT fragment: "…`blocker_class`, `discovered_tools`, `chunk_index`, `chunk_count`, `failure_stage`, `endpoint`); each task that adds keys updates the INDEPENDENT hard-coded expected list…"
REPLACEMENT fragment: "…`blocker_class`, `discovered_tools`, `chunk_index`, `chunk_count`, `endpoint`) — `failure_stage` is NOT a new key: it already exists in the r7 109-name union (AD2 slot 27), AD17 only reuses it with unchanged semantics; each task that adds keys updates the INDEPENDENT hard-coded expected list…" (rest of AD20 unchanged).

F1-note · spec.md:207 (§5 preamble 1:1 claim) — NO EDIT. Finding 1 is closed by mirroring the narrowing into the tickets (§2 below); once tickets 08/09/11 carry the narrowed ACs, the existing qualified 1:1 claim is again accurate.

════ 2. TICKET LINE REPLACEMENTS (whole lines) ════

issues/08-refresh-pass-lifecycle-records.md:14 (AC-1)
CURRENT: "- [ ] AC-1: A refresh pass whose candidate is rejected emits one failure record at warn or error with debug disabled."
REPLACEMENT: "- [ ] AC-1: A refresh pass whose candidate is rejected emits one failure record (`catalog_refresh_failed`) at the AD16-pinned level — warn when the retained catalog keeps serving, error when serving zero upstream tools — with debug disabled."

issues/08-refresh-pass-lifecycle-records.md:15 (AC-2)
CURRENT: "- [ ] AC-2: Start and outcome records carry a shared correlation ID, the configuration revision, the trigger, and the next scheduled attempt."
REPLACEMENT: "- [ ] AC-2: Start and outcome records carry a shared correlation ID, the configuration revision, and the trigger; the start record carries the next scheduled attempt."

issues/09-pin-blocker-detail.md:15 (AC-2)
CURRENT: "- [ ] AC-2: Each reported pin carries its configuration source location: file, property path, one-based line, one-based column."
REPLACEMENT: "- [ ] AC-2: Each reported pin carries its configuration source location: the pin's `pins[i]` property path."

issues/11-path-parity-and-logger-fallback.md:7 (What-to-build prose)
CURRENT: "Startup, SIGHUP reload and periodic refresh report the same pin diagnostic detail: identities, configuration source locations, discovery evidence, publication impact and retry guidance. No path may be the only way to obtain the useful error, so an operator never needs to send a signal to a running process to learn what a pinned tool rejection means."
REPLACEMENT: "SIGHUP reload and periodic refresh report the same core pin diagnostic detail: identities, configuration source locations and publication impact; chunked discovery evidence and retry guidance are refresh-path records. No path may be the only way to obtain the useful error, so an operator never needs to send a signal to a running process to learn what a pinned tool rejection means."

issues/11-path-parity-and-logger-fallback.md:12 (AC-1)
CURRENT: "- [ ] AC-1: A pin rejection at startup, on a SIGHUP reload and on a periodic refresh reports the same detail set: identity, source location, discovery evidence, publication impact, retry guidance."
REPLACEMENT: "- [ ] AC-1: A pin rejection on a SIGHUP reload and on a periodic refresh reports the same core detail — pin identity, source location (`pins[i]` property path), publication impact; discovery evidence and retry guidance remain refresh-path records."

issues/11-path-parity-and-logger-fallback.md:13 (AC-2)
CURRENT: "- [ ] AC-2: No path's pin failure is obtainable only through a SIGHUP reload."
REPLACEMENT: "- [ ] AC-2: Neither surviving path's pin failure is obtainable only through a SIGHUP reload — the periodic refresh record alone carries the full detail set."

════ 3. DECISIONS.MD APPEND (lines ready to append; then re-bind src-decisions sha256 per the recorded digest-refresh convention) ════

## r8 narrowing — owner decisions (2026-11-11)

1. **narrow1 authorized (option 2).** The owner authorized revision `r8-pin-diagnostics-narrow1`: narrow r8 to what its design can assert; defer the remainder to backlog tickets 13–18. No requirement, acceptance or task id was added or removed; the four specdd marker pairs are untouched.
2. **narrow2 authorized.** The owner authorized revision `r8-pin-diagnostics-narrow2` (2026-11-11): close code-reviewer REQUEST CHANGES packet `15567c2bf834ec903608e884d4bcf0d56e76f00c1b5071ee8875600a51b31327` findings 1–6, narrowing only — no new R/AC/T ids, no new AD, no new acceptance criterion, no widened envelope. The remaining ticket-17 artifact drift is explicitly NOT in scope and stays an owner-deferred, recorded gap (backlog ticket 17).
3. **Ruling 18 startup-parity claim SUPERSEDED (narrow1, mirrored narrow2).** Startup never evaluates pins (`internal/service/reload.go:554-560` sets `next.pins = nil` before publication), so ruling 18's "startup, SIGHUP reload and periodic refresh report identical pin detail" is superseded as written: the parity claim narrows to SIGHUP reload ↔ periodic refresh (AC11-1), the startup leg is deferred to backlog ticket 13, and narrow2 further narrows AC11-1's parity set to what the reload path can actually emit — pin identity, `config.property_path` source location and `publication_impact`; chunked discovery evidence and retry guidance remain refresh-path records (AD16–AD18).

════ 4. EXECUTION-STATE CORRECTIONS (.omp/plans/execution-state.json) ════

Changed fields — exactly three:
- tasks[T10].blockedBy: ["T8"] → ["T9"] (state :381-383). Restores strict serial dispatch T8→T9→T10→T11→T12 (spec.md:272/291); T9 and T10 can no longer be dispatched in parallel over shared `internal/service/refresh.go`, `internal/observe/log.go`, `internal/observe/envelope.go`, `refresh_lifecycle_test.go`.
- acceptances[AC12-1].method: "inspection" → "test"; requiredEvidenceKinds: ["inspection"] → ["test"] (state :789 region). Matches spec.md:262 "test (doc-drift)" and the existing test-AC shape (e.g. AC09-6/AC10-1: method "test", requiredEvidenceKinds ["test"]).
- acceptances[AC12-2].method: "inspection" → "test"; requiredEvidenceKinds: ["inspection"] → ["test"] (state :802 region). Matches spec.md:263.
Plus the mechanical re-binds: revision → "r8-pin-diagnostics-narrow2", all four sectionBindings' digests, the spec sha256, packet.digest, and — after the §3 append — the src-decisions artifact.sha256.

Explicitly NOT changed (ticket 17, per owner instruction):
- tasks[T10].effects.paths stays as installed, including "internal/upstream/runtime.go", "cmd/mcp-bridge/serve.go", "internal/upstream/dedicated_test.go" — ticket-17 drift, out of scope.
- tasks[T8].blockedBy: [] stays (its "T7 complete" prerequisite omission is ticket-17 drift, out of scope).
- All other effect lists (T8, T9, T11, T12) and T8's T7 prerequisite stay as ticket 17.
- tasks[T11].blockedBy ["T9","T10"], T12 edges: unchanged (already serial).

════ 5. TICKET-17 DEFERRAL SENTENCE (exact line to APPEND in spec §3, immediately after the "UNKNOWN (resolution named, not blocking): unchanged from r3 …" line; outside all specdd markers) ════

"Owner-deferred gap (ticket 17, NOT in narrow2 scope): the known artifact-consistency drift — state T10 effect paths `internal/upstream/runtime.go`, `cmd/mcp-bridge/serve.go`, `internal/upstream/dedicated_test.go`, the `spec.md:295` `service_edge_test.go` authorization, `tasks[T8].blockedBy` missing its T7 prerequisite, and the decisions/state packet-digest mismatch — is recorded in `.scratch/structured-logging/issues/17-r8-artifact-consistency-drift.md` and owner-deferred; a re-review must treat it as a deferred, recorded gap, not an unaddressed review finding."

════ 6. REVISION ID AND ID INTEGRITY ════

Revision id to record: **r8-pin-diagnostics-narrow2** (same sanctioned suffix-amendment path as r7→r7-closure1 and r8→narrow1; set at spec.md:1 and :34 and in the state's revision + bindings).
Confirmation: NO requirement, acceptance or task id was added or removed. R1–R25, AC01-1…AC12-3 (26 r8-slice ACs intact), T1–T12, AD16–AD21 all keep their existing ids; the AC09-5/AC10-4/AC11-1 amendments narrow assertions in place under their existing ids; AD20's amendment only corrects the new-key list; the Coverage (r8) line needs no edit. Marker integrity: every spec change is a whole-line replacement inside an existing marker section, except the one §3 deferral line appended outside all markers — the four specdd marker pairs remain unique, ordered, non-nested, each on its own line.
