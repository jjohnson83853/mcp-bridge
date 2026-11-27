VERDICT: REQUEST CHANGES

**Subject:** specification mode, initial pass. Candidate `.scratch/token-efficient-tool-responses/spec.md`, on-disk bytes (self-identified `r3-replacement`). Review limit: the packet's sha256 cannot be recomputed with the granted read-only toolset; I reviewed the bytes as read. Read-only throughout.

**Carried invariants:** all six are present and testable — independent byte oracle (R2/AC2), retired-marker lifetime and accounting (R7/R16, AC5/AC18), pinned `upstream_code` (R9/AC10), finite labels with producers (R12/AC13), per-terminal preview bounds (R10/AC11), ceiling-order validation (R11/AC12/AC23/AC28). Round-2 findings R-01, R-02, R-03, R-05, R-06, R-07, R-08, R-09 are independently verified closed against R8/R10/R13/R14, AC9/AC11/AC24/AC25/AC27/AC29 and T5's full-suite gate.

---

**1. HIGH — R4, §3 "Owner binding", AC16: the `call_tool` handle-rejection surface does not exist.**
R4 says the rejection contract "applies identically through `call_tool` and through `retrieve_result`", and AC16 exercises both. But `call_tool` takes only `{tool_name, catalog_revision, arguments}` (`internal/service/tools.go:705-711`) and validates `arguments` against the upstream bound schema with `additionalProperties:false`; there is no handle parameter on any surface except `retrieve_result`. Consequence: AC16's `call_tool` half cannot be written without inventing an argument surface the ticket forbids, so R4's uniformity is untestable. Fix: scope R4 and AC16 to `retrieve_result` only, and delete the `call_tool` clause, or name the exact carrier and its schema exception.

**2. HIGH — §3 "Owner binding"/"Retrieval failures", R4: `invalid_epoch` is claimed as existing `call_tool` behaviour and has no producer, no AC, no owner.**
`callTool` performs no session or epoch admission today (`internal/service/tools.go:699-790`: no `directRequestOwner`, no `requestExposureEpoch`), and the string `invalid_epoch` appears nowhere in `internal/`. So "validates … exactly as `call_tool` does" is false, and T3's objective/RED list never adds epoch admission to `call_tool`. R4 binds handles to "session + admitted epoch" without stating what happens when a session-ful call arrives with no valid epoch; the natural fallback (publisher namespace) makes a session's result readable by a session-less owner. Fix: state that the epoch source is the post-inbound stamped header via the existing `requestExposureEpoch` helper, that a session-ful call with missing/invalid epoch is `invalid_epoch`, that it is never a publisher fallback, add an AC, and add the code to §7's outcome enumeration (contradicting "existing error taxonomy" as an unchanged set).

**3. HIGH — R13 vs R4/AC16: validation precedence is unspecified and leaks handle existence.**
R13 requires `page > total_pages` → `invalid_arguments`. If that check runs before owner resolution, a foreign or retired-namespace handle submitted with a large `page` yields a cause-distinguishable `invalid_arguments`, disclosing existence and `total_pages`, defeating R4's byte-identical `invalid_handle`. Fix: state that handle resolution and owner/binding check precede page-bound validation, and add an AC: foreign handle + out-of-range page → `invalid_handle`.

**4. MEDIUM — AC8 and AC16 assigned to T2, whose GREEN forbids handler wiring.**
AC8's oracle is "retrieval through the `retrieve_result` handler"; AC16's is both handlers. T2 is store-only and `retrieve_result` registers in T3. Both ACs are unsatisfiable where they are assigned. Fix: move the handler-level halves to T3 and split store-level rejection oracles for T2, or restate T2's GREEN.

**5. MEDIUM — §5 T3 write roots omit three inventory assertions, so T3 cannot reach "suite green".**
A fourth built-in breaks `internal/service/tools_test.go:515-563,956` (`assertAdvertisedBuiltinsAndPin` with exactly three schemas; `TestToolSurfaceAdvertisesExactlyItsThreeTools`), `internal/service/builtin_guidance_test.go:24-36` (description map of exactly three), and `internal/service/progressive_session_test.go:180` (strict three-name list). None is a write root, and the guidance test additionally needs the new description's wording. Fix: add these three files to T3's write roots and name the description obligation, or explain why they are updated elsewhere.

**6. MEDIUM — R2/AC2: the "independent oracle" is not pinned to a serialization route.**
"Recomputes the length without the measuring function" permits an oracle that re-enters the same marshal path, so a wrong-form measure (measuring the pre-marshal value, excluding `_meta`, deduplicating duplicated content blocks) still passes — exactly round 2's caution. Fix: require the oracle to marshal an independently constructed `mcp.CallToolResult` via `encoding/json` (or a golden fixture with literal expected length), asserting lengths as literals on duplicate-bearing fixtures, one above and one below the ceiling.

**7. MEDIUM — R11(e)/AC28: the preview rune fixture is vacuous and preview-boundary safety is untested.**
`Content` is a prefix of the serialized result, which always begins with `{`; "a result beginning with a 4-byte rune" cannot exercise a split, so `TestPreviewFourByteRune` passes for any implementation. AC17 covers only page boundaries. Fix: re-specify the fixture as content whose 4-byte rune straddles `preview_ceiling_bytes`, assert `utf8.Valid` and prefix-hood on that preview, and restate the 4-byte headroom as last-rune headroom.

**8. MEDIUM — R11's "all documented relations" is false.**
§6 also documents `max_entries_per_namespace ≥ 1`, `max_upstream_code_bytes > 0`, `delivery_ceiling_bytes > 0`, preview `> 0`, and aggregate hardness; none is in R11(a)-(e) or an AC. Fix: enumerate them, or narrow R11's claim to the listed relations.

**9. LOW — falsifiability/traceability leftovers.**
`§3` terminals render `execution_state: "executed""` (unparsable literal vs R14). `§7` metric labels are "bounded" with no closed value set, unlike R12's log enumerations. R4's "no cause-distinguishable … timing difference" has no oracle. `§1 STATUS` cites "second-review findings F1–F10", but round 2's ledger IDs are `R-01…R-09`. Fix each in place.

### Recommendation
REQUEST CHANGES