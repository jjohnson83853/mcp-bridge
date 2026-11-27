VERDICT: REVISER

Specification mode, initial pass on the on-disk `r13-replacement` bytes (§1 STATUS). No implementation exists; I judged requirements, oracles, task ledgers and the six carried invariants, and checked r12 ledger closures for regression.

## Findings

**REV-1 [MEDIUM] — AC37 and AC40 have no owning task's executable test set.**
Section: §4 AC37, AC40 vs §5 T3.
Defect: both ACs name `(T3)`, but T3's RED list ("tests for AC1, AC3, AC6, AC9–AC11, AC13 (producer half), AC15, AC16 (handler half), AC17, AC24–AC26, AC28 (runtime half), AC29–AC32 and `TestPagingMetricsDeltas`") and GREEN omit `TestEscapingHeavyPagesWithinCeiling` and `TestCeilingPlusOneBoundary`; no other task claims them.
Consequence: T3 can exit 0 and T5's `go test ./...` pass while the escaping-within-ceiling and ceiling+1 boundaries never produce a failing-first test — two R5/R1-R3 guarantees ship unproven. AC37's "concatenation of all pages is byte-exact" also names no reference target (retained bytes vs golden literal), so its oracle is not determined.
Fix: add both tests to T3's RED and GREEN, and pin AC37's concatenation target to the golden fixture's literal expected bytes.

**REV-2 [MEDIUM] — `max_markers_per_namespace` load validation is asserted but has no relation and no AC.**
Section: §2 R16, §2 R11, §6 key table, §4.
Defect: R16 and §6 call it "a `retained_results` key validated at load like its siblings … ≥ 1 (existing `bound_range` diagnostic)", while R11 enumerates validated relations (a)–(h) without it and no AC exercises its rejection (AC33 covers only `max_entries_per_namespace`; AC39 tests the runtime cap, not load validation).
Consequence: a cap of 0 (or non-integer) loads, and every eviction then takes R16's no-marker branch: handles resolve to `invalid_handle` instead of `result_evicted`, silently changing AC18/R16 behaviour with no failing test. This is the only incomplete part of r12 F1's closure.
Fix: add the relation to R11's (a)–(h) list with its code and add an AC (extend AC33 or add AC41) asserting rejection.

**REV-3 [MEDIUM] — Two eviction orders govern one marker commit.**
Section: §2 R6 vs R16; §4 AC7 vs AC39.
Defect: R6's pre-reject rule frees "expired/evicted records first, then that namespace's FIFO-oldest **entries**"; R16's marker-admission rule frees "expired/evicted **markers**, then the oldest **markers** … and only if the namespace holds no other markers … evict the entry WITHOUT a marker". R16 never says it supersedes R6, so when a marker commit needs space and the namespace holds both entries and markers, two normative orders name different victims.
Consequence: two conforming implementations destroy different records; which handle later returns `result_evicted` vs `invalid_handle` differs, yet AC7 (accounting only) and AC39 (marker path) can both pass.
Fix: state in R6 that R16's order applies to marker commits (or fold both into one ordered rule), and have AC7/AC39 cite that single order.

**REV-4 [LOW] — Undefined identifier `(F1)` in oracles.**
Section: §4 AC18, AC39.
Defect: both cite "(F1)"; no F1 is defined anywhere in the document (§1's ledger filenames are the only cross-reference).
Fix: define F1 in §1 as the no-other-markers eviction case or drop the token.

**REV-5 [LOW] — AC32's exclusivity clause contradicts R13's shape-first precedence.**
Section: §4 AC32 vs §2 R13; AC25.
Defect: AC32 says "never `invalid_arguments` on an unresolvable handle", but R13 (and AC25) require a shape-invalid handle — necessarily unresolvable — to return `invalid_arguments`.
Fix: qualify AC32 to "well-formed but unresolvable handle".

**REV-6 [LOW] — R11(g)'s fixed 4-byte headroom is not derived from the escaping rule it must guarantee.**
Section: §2 R11(g), R5, §3 Page; §4 AC23/AC28.
Defect: pages are sized against the re-escaped serialized envelope, but the spec never pins the serializer's escaping mode; a raw one-byte `<` becomes 6 bytes under HTML-escaping (JSON-RPC transports often disable it), so "≥4 bytes last-rune headroom" does not establish the per-page ceiling for all admitted configs.
Consequence: a config passing AC23/AC28 could still be unable to emit a conforming page containing such a byte.
Fix: pin the serialization/escaping mode and state the worst-case single-rune escaped expansion that justifies the headroom constant (or compute it).

## Regression check / closures

r12 F2 (AC27 now rejects both README phrasings, anchored at README.md:325, :468), F3 (T5 roots are now README plus integration test only; T3 owns the description literal), F5 (§3 Preview now bounds the serialized preview value), F6 (§6 defers to R11(g)) are closed. F1 is closed for the runtime cap but leaves REV-2 open; F4 is closed ("emission is T3's"). No prior invariant regressed.

## Positive observations

The six carried invariants are present and oracled: independent byte literals (R1/R2; AC2, AC6, AC40), retired-record lifetime/accounting (R7/R16; AC8, AC18, AC39), pinned `upstream_code` (R9; AC10, AC29), closed label enumerations with producers (R12; AC13), per-terminal preview bounds (R10; AC11, AC26), ceiling-order validation (R11(a)–(h); AC12, AC20–AC35). Grounding claims verified: `diagnosticCodes` is a closed array (diagnostics.go:37) with `bound_range` present; `operationVocabulary` at httpdiagnostics.go:660 lacks `search_tools`; `requestExposureEpoch` at session_exposure.go:146 parses one header; `ToolSurface.Register` at tools.go:171 called from serve.go:175; README anchors at 277/441. Oracles are implementation-independent (golden literals, no re-marshaling).

## Review limits

Read-only review of specification mode only; no tests or scenarios were executed. The packet digest was not recomputed (no hash tool in this profile); I reviewed the on-disk bytes as read. Several accounting ACs (AC7, AC8, AC39) are only checkable by package-internal tests — stated, not a defect.

### Recommendation

REQUEST CHANGES