VERDICT: REQUEST CHANGES

**Reviewed:** `.scratch/token-efficient-tool-responses/spec.md`, on-disk `r17-replacement` (55,553 B per packet; digest not independently re-hashed — no shell tool). Specification mode, initial pass on this document. No implementation exists; no runtime checks claimed. Anchors I verified in-tree: `operationVocabulary` (`internal/observe/httpdiagnostics.go:660-664`), `diagnosticCodes` (`internal/config/diagnostics.go:37`), `requestExposureEpoch` (`session_exposure.go:146`), `ToolSurface.Register` (`tools.go:171`), the tool-count pins (`serve_hybrid_reload_test.go:225-226,238,262`), the 23-event pin map (`logging_contract_test.go:701-752`), README anchors (`README.md:277,441`).

The six carried invariants are present and mostly oracled: AC2/AC6/AC37/AC40 use literal-byte golden oracles; AC5/AC18/AC39 pin marker lifetimes; AC9/AC10/AC29 pin `upstream_code`; AC11/AC26 bound every terminal; AC12/AC20-23/AC33-35/AC41-42 validate ceiling order; R16's marker rule is now correctly qualified (r16 F3 closed), AC43 closes r16 F2, and T5's duplicated RED is gone (r16 F4 closed). Two of the six still fail, below.

### F1 — HIGH — The closed operation registry omits four production-emitted values, and AC13's scan cannot see them
Affects §2 R12, §7 "Finite labels", AC13, T2.
Defect: R12 pins "EXACTLY these 27 values" and asserts it is "the 18 values present today, plus the seven operation literals the non-test tree already emits outside the vocabulary". Production also emits `load_config` (`cmd/mcp-bridge/diagnostics_envelope.go:22`), `check_config` (`cmd/mcp-bridge/main.go:79`), `publish` (`cmd/mcp-bridge/wiring.go:472,498`), and `logging` (`internal/observe/log.go:35`) — 31 values, not 27. Worse, AC13's oracle parses only literal `Operation:`/`operation:` assignments; the first three reach the envelope through positional arguments to `observe.NewEnvelope(level, event, message, operation, outcome)` (`envelope.go:80`) and `internalOperatorEnvelope`, and `logging` is emitted inside a raw JSON line. The scan therefore passes while production writes unregistered operation values, so R12's "registry equals the literal-emitted set" and §7's "closed by registration" are both false and untestable as written.
Consequence: the finite-label invariant the owner carried stays unproven; a new undocumented label can ship green. This recurs r16 F1 in a new form.
Fix: enumerate and register all production values (`load_config`, `check_config`, `publish`, `logging` included; state the count as 31), and replace the source-syntax scan with an oracle that observes every operation value actually written to a log record (drive each emitter or enforce registry membership at the envelope construction point), asserting registry ⊇ emitted and equality against the stated set.

### F2 — HIGH — `upstream_code` is bounded on its decoded value but accounted in the serialized envelope, so an admitted config can exceed the preview ceiling
Affects §2 R8, R11(g), AC9, AC23/AC28.
Defect: R8 truncates so "the rendered value including the marker never exceeds `max_upstream_code_bytes`", and AC9 asserts the *rendered* byte length. R11(g) then pins `upstream_code` "at its full rendered `max_upstream_code_bytes` value" inside a *serialized* maximal envelope. Escaping is unbounded by that rule: a 512-byte code string of control bytes serializes to ≈3072 bytes (> the 1536 default preview ceiling). R3/R5 already size `Content` after escaping; `upstream_code` receives no equivalent rule.
Consequence: a config validated feasible can still deliver a preview or terminal whose serialized form exceeds `preview_ceiling_bytes`, falsifying R3/R10's core bound; AC23's "computable from the loaded file alone" claim fails.
Fix: state that the `upstream_code` bound is enforced on its escaped serialized contribution (`[truncated]` marker inside that bound), and that R11(g)'s pinned width is a serialized width; make AC9 assert the serialized length, plus an escaping-heavy code fixture.

### F3 — MEDIUM — AC8's tombstone-clearing half has no observable oracle
Affects AC8, R7, §6 `tombstone_ttl`.
Defect: retrieval returns the byte-identical `invalid_handle` both before and after tombstone clearing (R7, AC38), so "cleared by its stated condition" cannot be observed through lookup. The only observable is the 256-byte aggregate charge being released, which AC8 never names. R7 also points at "an explicit clearing condition" without stating it; it lives only in §6.
Consequence: an implementation that never clears tombstones passes AC8 and permanently consumes aggregate headroom, eventually refusing admissions the bound should allow.
Fix: state the clearing observable in AC8 — after a sweep past `tombstone_ttl`, a subsequent entry commit that needs the freed 256 bytes is admitted (accounting exact per AC7), while retrieval still yields `invalid_handle`.

### F4 — LOW — R11(g)'s "serialized envelope" does not say whether `Content` is included
Affects R11(g), AC23/AC28.
Defect: the inequality is "maximal-metadata serialized envelope plus 6 bytes of last-rune headroom"; if the envelope includes `Content` the +6 headroom is meaningless, and if it excludes it the text never says so, so AC23's fixture is ambiguous.
Fix: state that the envelope is computed with an empty (or absent) `Content`, the 6 bytes being the room for at least one escaped rune.

Not reopened: §8's `full`/deduplication/non-goals; process-local retention is consistent with handles resolving `invalid_handle` across restart.

### Recommendation
REQUEST CHANGES