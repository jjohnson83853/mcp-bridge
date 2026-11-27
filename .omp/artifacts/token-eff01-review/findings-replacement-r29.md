VERDICT: REQUEST CHANGES

**F1 [HIGH] — Retrieval-rejection shape is self-contradictory.**
Affected: §3 *Retrieval failures*, R4, R13, AC18, AC24, AC25, AC32.
Defect: §3 says every rejection is an “`errorResult`-shaped tool result”, and `errorResult` always carries one `TextContent` message, while R4, R13, AC18 and AC24 require “zero `Content`”; the spec never pins the rejection payload’s `Content`, `message`, or `execution_state`, so AC16’s “byte-identical” requirement has no expected literal and the retrieval ACs cannot all pass.
Observable consequence: following §3 produces a non-empty `Content`; following R4 produces an empty one; AC24/AC16 reject one of the two, and AC44’s `execution_state` producer set omits retrieval rejections if they are not `errorResult`.
Fix: define the rejection value once — either exact `errorResult` (fixed message in `Content`; change R4/R13/AC18/AC24 to “no page content”) or an empty-`Content` shape with `code`/`message` in `StructuredContent` (correct §3 and add it to AC44’s producers) — and give AC16 a literal expected payload.

**F2 [MEDIUM] — R12’s “every envelope construction” enforcement does not cover all construction paths.**
Affected: §2 R12; §7 *Events*; AC13(b).
Defect: R12 states every envelope validates `operation` in the `addEnvelopeFields` path, but `addEnvelopeFields` is reached only through `Logger.Log`→`buildEnvelope`; `NewEnvelope` + `WriteJSONLine` (`internal/observe/envelope.go:80`, `internal/observe/log.go:43`) marshal an `Envelope` directly, bypassing it, and the HTTP path validates separately.
Observable consequence: an unregistered operation emitted through the diagnostics-envelope path is silently written; AC13(b), which injects through the handler-free envelope path, cannot detect it, so the “never silently emitted” claim is false.
Fix: require the registry check at the single shared serialization boundary used by both `WriteJSONLine` and `Logger.Log`, or enumerate both paths and add an injection oracle for each; if only `Logger.Log` is in scope, remove the universal claim.

**F3 [MEDIUM] — `upstream_code` truncation is not pinned to a deterministic prefix.**
Affected: §2 R8, R9; AC9.
Defect: R8 says a long value is “rune-safe truncated to that bound”, but never says *which* prefix; AC9 only checks rune validity, the `[truncated]` marker, and ≤ `max_upstream_code_bytes`, so a one-byte prefix passes.
Observable consequence: two conforming implementations render different client-visible `upstream_code` values, and R9’s “rendered value fixed at retention” is not testable; the error code can be destroyed while AC9 stays green.
Fix: pin truncation to the longest rune-aligned prefix whose escaped serialized form plus the marker is ≤ `max_upstream_code_bytes`, state whether the bound includes the JSON quotes, and assert the exact literal in AC9.

**F4 [MEDIUM] — A sweep’s born-dead marker can evict a live marker and make retrieval depend on sweep timing.**
Affected: §2 R16; R6; AC5, AC18, AC39.
Defect: R16 says a sweep “install[s] and immediately clear[s] any marker whose deadline the clock has reached”, while the cap says “beyond the cap the oldest marker in that namespace is evicted first”; installing a born-dead marker can therefore evict a live marker and turn its handle into `invalid_handle`.
Observable consequence: handle resolution — currently defined as a function of logical state, “never sweep progress”, with ACs that “pin the clock; none depends on sweep scheduling” — changes when a sweep runs.
Fix: state that a born-dead marker is discarded without entering the live-marker set or invoking cap eviction (and does not consume a marker slot); add an AC with an already-dead transition and a full marker cap.

**F5 [LOW] — The maximal envelope’s `is_error` literal is not pinned.**
Affected: §2 R11(g); AC23.
Defect: R11(g) requires “every fixed metadata field at its literal value (`paging_required`, `is_error`, `execution_state`)”, but `is_error` has two serializations (`true`/`false`) of different lengths; AC23’s “exactly the negation” boundary therefore depends on an unpinned byte.
Consequence: the load-time feasibility test can be off by one byte between spec and implementation at the rejection boundary.
Fix: pin the maximal envelope to the longer literal (`false`) and state that AC23 constructs its config from that pinned envelope.

**Verified (not findings):** the six carried invariants are present and oracled — independent byte oracle (R1/R2, AC1/AC2/AC6), retired-marker lifetime and accounting (R7/R16, AC5/AC8/AC18/AC39/AC43), pinned `upstream_code` (R8/R9, AC9/AC10/AC29 — subject to F3), finite label registries with producers (R12, AC13/AC44 — subject to F2), per-terminal preview bounds (R10, AC11/AC26), and ceiling-order validation (R11(a)/(g), AC12/AC20–AC23/AC28/AC35/AC42 — subject to F5). The r28 ledger’s F1–F4 are visibly closed in this revision. Task write roots are exact files and the staging matches the owner ruling, apart from the rejection-shape ambiguity above.

REQUEST CHANGES