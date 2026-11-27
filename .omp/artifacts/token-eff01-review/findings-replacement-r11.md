VERDICT: REVISER

Verdict: REQUEST CHANGES

**Scope.** Specification mode, initial pass on the on-disk `r11-replacement` bytes; judged against O1–O9, refinement R1–R12, §4 ACs and the cited seams. Candidate digest not recomputed (read-only tools); no implementation exists. Prior ledger `findings-replacement-r10.md` reviewed: its F2/F3/F4/F5 closures hold; the r10 F1 closure is incomplete (F1 below).

---

**F1 [MEDIUM] R16 vs AC18/AC39 — no-marker eviction contradicts the deterministic-code promise.**
Section: §2 R16, §4 AC18/AC39, §8 OD3.
R16 says a handle whose entry *or marker* is expired/evicted returns `result_expired`/`result_evicted` "regardless of sweep timing", but R16 and AC39 also require the aggregate-infeasible sweep to remove the entry "with no marker installed"; with neither entry nor marker there is no record from which the typed code can be produced, and AC18 correctly qualifies it "while entry or marker exists". OD3's `(max_sessions + 1) × 256` headroom is a session-count bound and does not bound marker churn, so the no-marker path is reachable under throughput. Observable consequence: the same eviction yields `result_evicted` or `invalid_handle` depending on unstated timing, and R16's own sentence is false on its fallback branch.
**Fix:** qualify R16's determinism sentence to "while the entry or its marker exists; where the marker could not be admitted under R6, retrieval returns the shared `invalid_handle`", matching AC18/AC39; replace "one per entry evicted in the current sweep" with a real per-namespace marker cap or state explicitly that the marker is best-effort.

**F2 [LOW] R13 vs AC25/AC32 — page-shape precedence over handle binding is undefined.**
Section: §2 R13, §4 AC24/AC25/AC32.
R13 orders "handle shape → resolution/binding → page bounds" yet says a shape-invalid *page* returns `invalid_arguments`; AC25 requires non-integer/<1 page → `invalid_arguments`; AC32 requires a foreign handle + out-of-range page → `invalid_handle`. For a well-formed foreign handle with page `0` or `1.5`, both codes are defensible, and AC24 defines "out of range" only as `page > total_pages`, so the case is uncovered.
**Fix:** state whether page shape is evaluated with handle shape (before binding) or as a page bound (after binding), and pin that combination in AC25/AC32.

**F3 [LOW] R8 vs AC9/AC10 — over-bound `upstream_code` truncation has no observing oracle.**
Section: §2 R8, §4 AC9/AC10/AC26.
R8 requires truncation to `max_upstream_code_bytes` with the `[truncated]` marker inside the bound and rune-safe cutting; AC9 only asserts `upstream_code` is "preserved" on an over-bound fixture, and AC10/AC29 assert pinning across reload. Nothing asserts the rendered value is ≤ the bound, ends with the marker, or is `utf8.Valid`; a full untruncated render passes those ACs and silently breaks R11(g)'s maximal-metadata envelope assumption.
**Fix:** add the literal expected rendered value to AC9's over-bound case and assert length ≤ bound, suffix, and `utf8.Valid`.

**F4 [LOW] T2 vs AC13 — "no `retrieve_result` reference" conflicts with the T2 vocabulary duty.**
Section: §5 T2, §4 AC13.
AC13 is a T2 oracle requiring `operationVocabulary` to contain `retrieve_result`; T2 owns `internal/observe/httpdiagnostics.go` and lists `TestLogLabelEnumeration`. Yet T2 GREEN says "no handler wiring and no `retrieve_result` reference". T2 cannot both be handler-free and the task adding the literal.
**Fix:** reword to "no retrieval handler; the two label values are added here", or move the vocabulary extension to T3 and split AC13 accordingly.

**F5 [LOW] T3/T5 — duplicate ownership of the `retrieve_result` description.**
Section: §5 T3/T5, §4 AC27.
T3 already owns `internal/catalog/schema.go` (schema and description) and its `builtin_guidance_test.go` exact-string map must carry the full R15 wording; T5 re-lists `schema.go` for "the `retrieve_result` description satisfying AC27". If T5 edits the wording, T3's exact-string assertion goes red and `builtin_guidance_test.go` is not a T5 write root; if it does not, T5's write root is a no-op.
**Fix:** assign the description and its exact-string test to one task (T3), and let T5 own README plus the new integration test only.

**F6 [LOW] R15 vs AC27 — the old promise's actual wording escapes the oracle.**
Section: §2 R15, §4 AC27.
`## Tool surface` currently promises `call_tool` "keeps its outer provenance envelope and complete upstream result"; AC27 forbids only an unqualified "complete `CallToolResult`" sentence. A README carrying both the qualified sentence and that older phrasing passes AC27 while R15 remains violated.
**Fix:** extend AC27's absence rule to the promise's actual phrasings at the named anchors ("complete upstream result", "complete original `CallToolResult`").

---

**Positive:** metadata sets are single-sourced and referenced rather than repeated; AC2/AC6/AC40 use genuinely independent golden literals; R11's six codes and R12's two vocabulary additions are grounded in the real registries; R13's precedence is anchored to `requestExposureEpoch`; r10 F2–F5 are closed.

**Review limits:** white-box store ACs (tombstone/marker accounting) can only be verified through package-level tests; no hash, runtime or Git access was used.

## Recommendation

REQUEST CHANGES