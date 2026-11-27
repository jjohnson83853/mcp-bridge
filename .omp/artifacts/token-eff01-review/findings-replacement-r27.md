Verdict: REQUEST CHANGES

**Candidate:** `.scratch/token-efficient-tool-responses/spec.md`, on-disk bytes, `r27-replacement` (the file's STATUS). Specification mode, initial pass on this replacement. Review limits: read-only, no builds/tests run; evidence is the spec text plus the source anchors it cites (I did not re-hash the candidate).

The six carried invariants are all present, each with an independent oracle: byte-golden pass-through/measurement (R1/R2; AC1/AC2/AC6/AC40), retired-marker lifetime and accounting (R7/R16; AC5/AC18/AC39/AC43), pinned `upstream_code` (R8/R9; AC9/AC10/AC29), closed label registries with producers (R12; AC13/AC44), per-terminal preview bounds (R10; AC11/AC26), and ceiling-order validation (R11; AC12/AC20–AC23/AC33–AC35/AC41/AC42). The r26 findings are closed: AC11 now cites R10's terminal sizing; R6 scopes marker eviction outside retirement admission and R7/AC38 name the released live-marker set; R16 pins the half-open rule applied by AC5/AC18/AC39. Three defects remain.

### Findings

**1. [HIGH] Aggregate pre-reject eviction set is contradictory.**
*Sections:* §2 R6 (aggregate admission paragraph); §4 AC15; §5 T2 objective; §6 `max_bytes_aggregate` row.
*Defect:* R6 authorizes an entry/tombstone commit to evict only that namespace's "FIFO-oldest **live** entries", states an expired or evicted entry "holds no aggregate charge to release", and forbids evicting a marker outside the marker cap. AC15, §6 and T2 all require the same commit to evict the namespace's "expired/evicted **records** first, then its FIFO-oldest entries". If "records" means entries, the first step releases zero aggregate bytes; if it means markers, R6 forbids it. The ordered pre-refusal eviction set is therefore unspecified.
*Consequence:* `TestAggregateAdmissionHard` (AC15) cannot distinguish a compliant implementation from one that reclaims markers (turning surviving handles into `invalid_handle`) or from one that evicts charge-free entries (changing entry counts); the hard aggregate oracle is not falsifiable.
*Fix:* State one ordered eviction set in R6 and make AC15/§6/T2 cite it. If markers/tombstones may be reclaimed before refusal, scope R6's "only the marker cap evicts a marker" to non-retirement admission and state the resulting handle code; otherwise delete the expired/evicted step from AC15/§6/T2.

**2. [MEDIUM] Retirement precedence is asserted over a state retirement itself removes.**
*Sections:* §2 R7; §3 "Retrieval failures"; §4 AC18 and AC32.
*Defect:* R7's retirement commit releases the namespace's live markers, evicts its expired/evicted records and FIFO-oldest entries, and installs the tombstone in the same commit; R6 says retirement releases every record it held. No entry or marker can coexist with a completed retirement. Yet §3, AC18 and AC32 assert `invalid_handle` from a retired namespace "even when its entry or marker is also expired or evicted" — a combined state the stated commit cannot produce (unless a concurrent read is intended, which the spec does not define or give an oracle for).
*Consequence:* The precedence clause is untestable, and a store-level test may pass two different implementations.
*Fix:* Either state that retirement atomically removes all entry/marker records so only the tombstone survives (and delete the "even when…" clause), or define retirement as a namespace flag leaving records reachable only by precedence, with R7/R6's release rule updated. AC18/AC32 must assert only reachable states.

**3. [MEDIUM] AC13(b)'s counter assertion conflicts with T2's declared scope.**
*Sections:* §4 AC13(b); §5 T2 GREEN; §4 AC14.
*Defect:* AC13(b) asserts `bridge_log_operation_violations_total` increments, but T2's GREEN restricts metric assertions to registration only ("deltas belong to T3"), and AC14 assigns before/after deltas to T3. AC13 is nevertheless in T2's RED list (`TestLogLabelEnumeration`); T3's RED list does not include the increment assertion.
*Consequence:* The only observable signal that unlisted operations are rejected can be left unasserted or asserted by a task whose stated scope forbids it.
*Fix:* Move AC13(b)'s increment assertion to T3 (with AC13's producer clause), or amend T2's GREEN to permit that single counter assertion. One owner.

### Task/write-root check

T1–T5 name exact files and a coherent T1→T2→T3→T4→T5 staging; no production source is assigned to T5, and `cmd/mcp-bridge/serve.go` is correctly excluded. One advisory: T3's tool-count instruction offers a "PREFERRED" derived-count rewrite plus a literal fallback (4→5, 5→6, 4→5); pin one oracle so the expected values are unique. This is a planning ambiguity, not a separate blocking defect.

### Positive observations

The replacement is genuinely single-sourced: metadata sets live in R3/R5/R10 and §3 references rather than restates them; the page cut-point rule is escaping-aware and deterministic; golden literals are used as independent oracles ("not a retained artifact, not re-marshaled"); and the marker/tombstone half-open rule is pinned once and referenced by every affected AC.

### Recommendation

REQUEST CHANGES