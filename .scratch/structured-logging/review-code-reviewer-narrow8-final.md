## Review — SPEC-STRUCTURED-LOGGING r8-pin-diagnostics-narrow8
Spec-only, read-only. Findings: HIGH 1, MEDIUM 2, LOW 4.

REVIEWER: code-reviewer
PACKET: 23a6e42fe8a4a60fad498f64c140e7028e02370329354e5dbc5d41de69a7a838
VERDICT: revise

**1 MARKERS — pass.** 8 tokens / 4 pairs, own lines, unique, ordered, non-nested: `spec.md:40/:67`, `:169/:204`, `:271/:287`, `:289/:303`. Header `:1` and §1 `:34` both narrow8; state binds this exact spec digest.

**2 COVERAGE — pass.** R20–R25 (`:56-62`) all map to ACs; r8 map `:267` now includes AC12-1..3; every AC tag `:240-265` names exactly one producer; state `ownerTaskIds` are one-per-AC and disjoint (T8→AC08-1..7, T9→AC09-1..8, T10→AC10-1..5, T11→AC11-1..3, T12→AC12-1..3).

**2b NARROWING CLOSURE — revise.** Closed: issues/11:14 opposite-lane (matches `:262`), /12:12 eventMessage+operations (matches `:263`), /09:14 and /10:14 R25 tags, `:267` AC12 anchors, issues/17:20. **[HIGH]** `:315` closes the prior LOW by asserting "owner-ACCEPTED … per decisions.md r8 checkpoint item 3, backlog ticket 18"; the only transcribed owner answer on this gap says otherwise — `owner-authorization-r8.txt` Q3 "Decision pending; no acceptance and no scope extension is authorized yet" — and `issues/18` (needs-info) exists to reconcile that conflict, so `:315` cites the ticket that contradicts it. Fix: restore "acceptance pending owner ruling (backlog ticket 18)"; nothing else changes.

**3 ACs — revise [MEDIUM].** AC08-1..AC12-3 are falsifiable and producer-named; AC09-5 carries its own ≤64 oracle; AC10-5 names the real hook (`refresh.go:211`). Defect: `:267` claims "AD20 ruling (no schema_version change) is itself asserted by AC12-1", but AC12-1's oracle is docs text + eventMessage names + operations (`:263`), and AC04-1 (`:223`) asserts schema_version *presence* only — an emitted `schema_version: 2` passes every oracle. Fix (id-neutral): add to AC12-1 "every emitted r8 record carries `schema_version` 1". LOW: AC08-7 (`:246`) restates AC08-1's level clause; AC12-2 (`:264`) lacks AC07-2's "keys ∈ `EnvelopeFieldNames()`" check; AC08-6/AC09-8 tail clauses are only positively observable.

**4 RULINGS — pass.** T8 `:282` no metric / reload.go-wiring / schema_version + R8 BLOCKED rule; T9 `:283` no raw text, no debug-only; T10 `:284` no metric, no schema_version, no debug, no induced-failure, endpoint scheme/host/port/path only; T11 `:285` additive-only per AD21, `bridge_catalog_refresh_total` untouched; envelope `:300` mirrors. AD20 `:196` keeps 1; AD21 `:203` additive. Matches decisions rulings 6–9, 16–17, 19.

**5 ENVELOPE/STATE — pass [MEDIUM/LOW].** `state.spec.sha256` = reviewed digest, all four sections narrow8, packet 23a6e42f…; chain T8→T9→T10→T11→T12 is total ⇒ strictly serial. Effects match `:283-286` except the recorded drift (T10 adds `internal/upstream/runtime.go`; spec `:296` `service_edge_test.go` has no effect) — issue 17, deferred. **[MEDIUM]** `:165` says issue 17 records "`tasks[T8].blockedBy` missing its T7 prerequisite"; issue 17 does not (only T10's, now resolved), so that deferral record is incomplete. Fix: add the bullet to issues/17 or drop it from `:165`. LOW: issue 17's other bullets are stale (AC12-1/2 now `method: test`; state binds 23a6e42f, not 358ee090).

**6 DEFECT SITES — pass.** `refresh.go:195-205` (runPass emits nothing); `:210-224` (`err` read only in the `err == nil` guard; failure falls through to `runtime.Current()` at `:218-221`); `reload.go:562-564` text-only `%w` identity with `core.go:311` first-only (T9 scope note accurate, core untouched); `wiring.go:373-406` readiness only on publish; `ops.go:19` `reason,omitempty`; `dedicated.go:122-168` raw `%w`, no stage; `log.go:186-219` 16 names, no catalog/pin event. The prior "one line off" citation is now exact.

Positive: ticket mirrors are near-verbatim; several ACs own their oracles; section bindings are honest.

### Recommendation

REQUEST CHANGES
