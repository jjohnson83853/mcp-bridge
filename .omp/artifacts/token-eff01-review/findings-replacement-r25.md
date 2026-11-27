Verdict: REQUEST CHANGES

Specification mode · initial review of `.scratch/token-efficient-tool-responses/spec.md` r25 (on-disk bytes), against the packet, intake O1–O9, refinement R1–R12, and live read-only anchors.

**F1 [MEDIUM] — Marker lifetime is still sweep-dependent, contradicting R16's own "logical state" guarantee.**
§2 R16 (retrieval-resolution paragraph, opening lifetime sentence); §4 AC5, AC18.
Defect: the typed-code window is declared independent of sweep timing, but its end is defined by *the marker's* lifetime, whose only stated clock is marker age from installation ("cleared when the sweep observes marker age > lifetime"). A first sweep arriving after the transition starts a fresh age, so `invalid_handle` arrives late; with no sweep it never arrives, while the same paragraph says the window exists "whether or not a sweep has yet materialised the marker". The marker that is supposed to bound the window may not exist.
Consequence: two conforming implementations return `result_expired`/`result_evicted` versus `invalid_handle` in one state; AC5's "invalid_handle only after that marker's lifetime ends" is underivable because no marker exists to carry a lifetime.
Fix: anchor the deadline at the transition (clear deadline = transition time + `tombstone_ttl`; a late sweep installs and clears immediately when already past; scheduling never changes a handle's code) and add the delayed-first-sweep oracle: evict at T, first sweep at T+10m, retrieve at T+14m → `result_evicted`, at T+16m → `invalid_handle`.

**F2 [MEDIUM] — §5 T2/T3 event-registry arithmetic is false against the tree; closing `eventMessage` as written drops a real startup event.**
§5 T2 (`eventMessage` registry), T3 (docs/logging-contract.md, logging_contract_test.go); §7 Events.
Defect: T2 closes `eventMessage` and adds twelve events; T3 asserts the pin map "23 → 35 … matching T2's closed `eventMessage` registry". `eventMessage` has **24** distinct names (internal/observe/log.go:230-277), including `snapshot_retention_failed`, emitted at cmd/mcp-bridge/serve.go:64 and documented at docs/logging-contract.md:73/516, but absent from the 23-entry pin map (cmd/mcp-bridge/logging_contract_test.go:701-751). Twelve additions therefore yield 36 registry names, not 35.
Consequence: an implementer closing the registry to the 35-entry map routes the real `snapshot_retention_failed` startup event through the `log_vocabulary_violation` path (record dropped, counter incremented), losing startup telemetry; §7's "registry equals the emitted set" and AC13's oracle become false.
Fix: state the 24-name basis, add the `snapshot_retention_failed` → `startup` row to docs and the pin map in T3's stated write roots, and shift the literals to 36 (or record the pre-existing pin-map gap and its resolution explicitly).

**F3 [MEDIUM] — Terminal `Content` sizing is not normatively stated, so AC11's literal oracle is not derivable.**
§2 R10; §3 Terminals; §4 AC11; §2 R5 (sizing rule), R11(g).
Defect: AC11 requires each terminal's `Content` to be "the longest prefix that fits under R5's pinned sizing rule", but R5's rule is written for the *preview* envelope and explicitly uses "preview metadata" (R3's set: handle, `paging_required`). A terminal's metadata set is R10's (no handle, adds `retention`), so the longest fitting prefix differs; no clause defines terminal sizing.
Consequence: AC11's literal expected prefix cannot be computed from the spec; two implementations produce different terminal `Content` for the same fixture while both satisfy R10.
Fix: add one sentence to R10 (referenced from §3): terminal `Content` is the longest rune-aligned prefix of the serialized result whose serialized terminal envelope, at R11(g)'s pinned terminal widths, is ≤ `preview_ceiling_bytes`; make AC11 cite that sentence.

**F4 [LOW] — R6's aggregate pre-refusal eviction set conflicts with R16's "sole marker-eviction exception".**
§2 R6 (aggregate-admission paragraph), R16 opening; §8 OD3; §4 AC15.
Defect: entry/tombstone commits are said to "evict the creating namespace's expired/evicted records first", but under R6/OD3 an expired/evicted entry's payload has already left accounting (the 256-byte reservation converts to the marker charge), leaving only a marker — which R16 says only the marker cap may evict. Either the step is vacuous or it evicts markers outside the cap, turning handles into `invalid_handle` with no stated rule.
Consequence: AC15 cannot pin what "expired/evicted records" are, and the pre-refusal reclaim set is ambiguous.
Fix: scope the first step to records whose payload/reservation is still charged (say at which instant payload leaves), or explicitly permit marker eviction there, name the resulting handle code, and cover it in AC15.

**F5 [LOW] — AC3's stated coverage and oracle disagree.**
§4 AC3 versus AC19/T4.
Defect: AC3 requires the preview contract "asserted per surface", but its oracle `TestPreviewBoundedAndMetadata` is T3, which wires `call_tool` only.
Fix: drop "per surface" from AC3 or cite AC19/T4 for the four-surface assertion.

**F6 [LOW] — "executed" is justified by upstream execution that does not occur on discovery surfaces.**
§2 R14, R12; §3 Surfaces/Terminals; §4 AC26, AC44; internal/core/core.go:69-73.
Defect: R14 justifies `execution_state: "executed"` for every terminal with "all five occur after upstream execution", yet §3 includes `search_tools`/`describe_tool` (bridge-generated per R8), where no upstream executes. A discovery result refused retention reports `executed`, contradicting the field's documented "no upstream execution" semantics.
Fix: define the terminal value as "the tool result was produced (including bridge-generated discovery)" in R14/R12, or restrict terminals to upstream-executing surfaces; state the choice in AC26/AC44.

### Positive Observations
All six carried invariants are present and testable: literal byte oracles (R1/R2, AC2, AC40), retired-record lifetime/accounting (R7/R16, AC8/AC39/AC43), pinned `upstream_code` (R8/R9, AC9/AC10/AC29), finite label registries with producers (R12, AC13, AC44), per-terminal bounds (R10, AC11, AC26), ceiling-order validation (R11(a), AC12). The r24 F1–F5 closures verify in text, and every task names exact write roots except where F2 requires expanding T3's docs scope.

Review limits: specification bytes and read-only anchors only; no implementation inspected and no tests executed.

### Recommendation
REQUEST CHANGES