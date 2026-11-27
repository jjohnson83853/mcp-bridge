VERDICT: REQUEST CHANGES

Packet vocabulary: `VERDICT: REVISE`. Specification mode, initial pass on the on-disk `r12-replacement` bytes (STATUS §1). No implementation exists; I judged requirements, AC oracles, task ledgers, and the six carried invariants against O1–O9 and refinement R1–R12. Prior ledger `findings-replacement-r11.md` checked for regression.

## Findings

**F1 [MEDIUM] — R16/AC39: the "live marker" bound is not a bound, so AC39's assertion is unfalsifiable.**
Section: §2 R16 (line 27), §4 AC39 (line 105), §8 OD3.
R16 caps markers "to one per entry evicted in the current sweep", but markers persist for `tombstone_ttl`, so live markers per namespace accumulate across successive sweeps inside that window and are bounded only by total evictions, not by `max_entries_per_namespace`. AC39 repeats the same phrasing ("live markers per namespace never exceed one per entry evicted in the current sweep"), which is tautologically true of the installation rule and never fails. OD3's documented headroom `(max_sessions + 1) × 256` counts tombstones only, so marker churn is unaccounted.
Observable consequence: sustained eviction consumes `max_bytes_aggregate`, and otherwise-admissible over-ceiling results are refused with `result_aggregate_capacity_exceeded` for reasons the spec's own capacity model does not predict.
Fix: state a real per-namespace live-marker cap (e.g. `≤ max_entries_per_namespace`, oldest-marker eviction beyond it) or declare markers best-effort and extend OD3's headroom statement to marker churn; make AC39 assert the stated cap, not the installation rule.

**F2 [MEDIUM] — R15/AC27: the superseded promise at the `## Tool surface` anchor survives the oracle.**
Section: §2 R15, §4 AC27 (line 93); README.md:325.
`## Tool surface` currently says `call_tool` "keeps its outer provenance envelope and complete upstream result". AC27 forbids only "an unqualified complete-`CallToolResult` sentence" and otherwise requires positive assertions. A README that adds the four-tool inventory and the qualified promise while retaining line 325 passes every named assertion, yet R15's replacement goal is unmet and the document contradicts itself. (The second anchor's phrase at README.md:468 is caught; this one is not.)
Fix: extend AC27's absence rule to the anchor's actual phrasings — "complete upstream result" and "complete original `CallToolResult`" — at both named anchors.

**F3 [MEDIUM] — T3/T5 both own the `retrieve_result` description, with a locked literal in a T3-only test.**
Section: §5 T3 write roots (line 117), §5 T5 write roots (line 121); `builtin_guidance_test.go` exact-string map.
T3 owns `internal/catalog/schema.go` "(fourth built-in schema and description)" and must carry the full literal wording in `builtin_guidance_test.go`, which is a T3 write root only. T5 re-lists `schema.go` "(retrieve_result description satisfying AC27)". If T5 edits the wording, T3's exact-string assertion goes red and T5 cannot repair that test; if it does not, T5's write root is a no-op.
Fix: assign the description and its exact-string test to one task (T3), and limit T5 to README plus the integration test.

**F4 [LOW] — §7 Events assigns `result_expired`/`result_evicted` to T2, but T2 is handler-free and the producer clause is T3.**
Section: §7 Events (line 148); §5 T2 GREEN; R12/AC13.
Event records carry the transport request id and a paging `operation` label, which only exist at the handler; §7 nonetheless maps these two events to `AC5/T2, AC18/T2`, whose oracles are store-level and assert return codes, not emitted records. Following §7 literally forces T2 either to emit handler-shaped logs (contradicting "store-only and handler-free") or to leave the events unproduced.
Fix: attribute these events' emission to T3 and keep T2's oracle to returned codes.

**F5 [LOW] — §3 Preview wording weakens R3's measuring rule.**
Section: §3 Preview; §2 R3; §4 AC3.
§3 says "`Content` = UTF-8-safe byte prefix ≤ preview ceiling", which reads as bounding the `Content` string; R3 requires the bound to be the marshaled preview value, "not by the length of its `Content` string alone", and §3's Page bullet correctly states the serialized form. An implementer sizing `Content` to the ceiling produces an over-ceiling preview.
Fix: say the serialized preview value is ≤ `preview_ceiling_bytes` and `Content` is sized to fit after escaping, mirroring the Page bullet.

**F6 [LOW] — R11(g) is restated in §6 (line 141).**
Two normative copies of the feasibility rule can drift; only §6's is read by implementers of the config table.
Fix: replace §6's final paragraph with a pointer to R11(g).

## Verified sound

The six carried invariants are present and oracled: independent byte literals (R1/R2; AC2, AC6, AC40), retired-record lifetime and accounting (R7/R16; AC8, AC18, AC39), pinned `upstream_code` (R9; AC10, AC29), closed label enumerations with producers (R12; AC13 — `search_tools` is genuinely absent from `operationVocabulary` at httpdiagnostics.go:660-664, so "exactly two new values" holds), per-terminal preview bounds (R10; AC11, AC26), ceiling-order validation (R11(a)-(h); AC12, AC20-AC35 — directions are consistent with §6). O1–O9 and refinement R1–R12 all map to ACs without weakening. Prior r11 F1a, F2, F3, F4 closures hold; F1b, F5, F6 remain open above.

## Review limits

Specification mode only; no implementation to trace. The packet's digest was not recomputed (no shell/hash tool in this profile) — I reviewed the on-disk `r12-replacement` bytes as read. Several accounting ACs (AC7, AC8, AC39) are only checkable through package-internal tests; that is stated, not a defect.

### Recommendation

REQUEST CHANGES