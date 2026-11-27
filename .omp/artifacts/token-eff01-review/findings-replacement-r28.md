VERDICT: REVISE

**F1 [HIGH] — Maximal-envelope feasibility undercounts the first content item**
Affected: R11(g), AC23/AC28, R5, R10; T1.
Anchor: §2 R11(g) — “The maximal-metadata envelope is computed with an empty or absent `Content` — no content bytes are counted in it”; the 6-byte headroom is justified only for re-escaping one rune. §3/§2 never pin the serialized shape of `Content` on a preview, page, or terminal.
Defect: if `Content` is “absent” (e.g. `"content":[]`), the envelope excludes the content item’s structural bytes (a `TextContent` block is ~26 bytes). A config meeting maximal-envelope(absent)+6 ≤ ceiling can then have no rune-aligned prefix that fits, so R3/R5/R10’s *non-empty* guarantee fails at runtime while load-time validation passed; AC23’s negation test inherits the same undercount, and the golden literals in AC3/AC11 cannot be derived from the spec because the block encoding is unspecified.
Fix: pin the `Content` block representation used by R2/R5/R10/R11(g) and require the maximal envelope to contain that item present with zero text bytes; state that the pinned 6 bytes cover re-escaping only, with item structural bytes inside the inequality.

**F2 [HIGH] — Tombstone/marker lifetime is stated under two incompatible rules**
Affected: §6 `tombstone_ttl` row vs R7, R16; AC5, AC8, AC18, AC39.
Anchor: §6 says tombstones and markers are “cleared when the sweep observes age > ttl” (strict `>`, sweep-dependent), while R7/R16 pin “live while now < transition+`tombstone_ttl`; at now ≥ … cleared” (half-open), and R16 says logical state is “evaluated from timestamps, never sweep progress.” AC8 adds a third phrasing: “on the first sweep whose clock is at or past … the charge is released.”
Defect: at exactly `transition+tombstone_ttl` the spec simultaneously says the record is live (§6) and cleared (R16/AC5/AC18/AC39); the 256-byte aggregate charge is likewise released at the deadline (R16) or at the next sweep (AC8). No implementation and no pair of ACs can satisfy all three.
Fix: delete `age > ttl` and the sweep-observed phrasing from §6 and reference R16’s half-open rule verbatim; pin whether accounting release is clock-evaluated at the deadline (then AC8 must assert the pre-sweep admission) or sweep-gated (then R16 must stop saying the charge is released at `now ≥` the deadline).

**F3 [MEDIUM] — `upstream_code` truncation vs the retained-byte domain**
Affected: R8, R9, R14, R5; AC6, AC9.
Anchor: R8 — “truncated to that bound at retention”; R14 — `total_bytes` is “the original measured bytes … never retained or truncated bytes”; R5 — `remaining_bytes` is measured on “the retained serialized bytes, the same byte domain as `total_bytes`”; AC6 asserts concatenated page length = `total_bytes`.
Defect: the spec does not say whether truncation edits the stored serialized result or only the pinned `upstream_code` metadata. If it edits the payload, retained bytes ≠ measured bytes and AC6/AC9’s byte-exact literals diverge from page reconstruction; if it does not, “at retention” is misleading. This is exactly the independent byte-oracle invariant.
Fix: state that retention stores the unmodified serialized result and the bound/truncation applies only to the per-entry pinned `upstream_code` rendering; keep `total_bytes`/`remaining_bytes`/AC6 in the unmodified domain.

**F4 [MEDIUM] — An unrecoverable truncation can be delivered as a success**
Affected: R10, R8, §3 Terminals; AC26.
Anchor: AC26 pins a terminal’s `is_error` to R8’s preserved value; terminals carry no handle (R10) and only a prefix of `Content`.
Defect: a successful over-ceiling result that cannot be retained is delivered with `isError:false` and truncated content; an MCP client keying on the canonical error flag sees a complete success, and the absent bytes are unrecoverable. The ticket requires errors never to be mistaken for truncations; the converse — a truncation mistaken for success — is left unstated.
Fix: name the truncation-visibility invariant for terminals: either pin terminal `isError:true` (and scope R8/AC26 accordingly), or state explicitly that `retention` in `StructuredContent` is the sole and sufficient unrecoverable-truncation signal.

**Verified (not findings):** All six carried invariants are present and oracled — independent byte oracle (R1/R2, AC1/AC2), retired-marker lifetime/accounting (R7/R16, AC5/AC8/AC18/AC39/AC43), pinned `upstream_code` (R8/R9, AC9/AC10/AC29), finite label registries with producers (R12, AC13/AC44), per-terminal preview bounds (R10, AC11/AC26), ceiling-order validation (R11(a),(g), AC12/AC23/AC28). The 31-value operation arithmetic (18+11+2) checks out; the README anchors and the `requestExposureEpoch`/`operationVocabulary`/`execution_state` anchors exist; task write roots are exact files, T3/T4 staging matches the owner ruling, and every AC is assigned to a task and a named oracle.

REQUEST CHANGES