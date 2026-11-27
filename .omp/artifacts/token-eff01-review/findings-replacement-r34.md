VERDICT: REVISER

**Mode:** specification, candidate `token-efficient-tool-responses-ticket01-spec`, `r34-replacement` (on-disk bytes). Prior ledger inspected: `findings-replacement-r33.md`.

**Closure evidence (r33 ledger):** R33-F1 closed — AC39 now asserts the clock-driven 256-byte release at the half-open deadline plus admission of a commit needing those bytes before any sweep. R33-F3 closed — AC38 now asserts the observable `invalid_handle` at `retrieve_result` for an un-finalized retirement. R33-F4 closed — §7 and AC13 both forbid handle/presence emission, with no `handle_present` producer. R33-F2 is **not** closed (F3 below).

### Findings

**F1 [MEDIUM] R16/AC39 — the born-dead-marker case has no oracle.**
§2 R16 states: "the AC39 oracle covers the residual case: with the marker cap full, an already-dead transition is discarded and every live marker and every live handle's retrieval code is unchanged, asserted before/after." AC39 contains no such assertion; it covers only reservation-funded installation, deadline-charge release, and `max_markers_per_namespace`+1 live installations.
Consequence: an implementation that installs a born-dead marker and cap-evicts the oldest live marker makes a live handle resolve to `invalid_handle` as a function of sweep progress — precisely the dependence R16 forbids — and passes every AC.
Fix: add the full-cap born-dead case (before/after live-marker set, per-handle retrieval code, aggregate delta) to AC39 or AC18 and name its test; or delete R16's claim that AC39 covers it.

**F2 [MEDIUM] AC39 — self-contradiction on marker eviction.**
§4 AC39 says "a marker's admission never refuses, **never evicts another record**", then requires that a 17th installation evicts the oldest marker. R6 and R16 permit exactly one eviction type: another (oldest) marker, never an entry or tombstone.
Consequence: the named oracle cannot be satisfied as written — implementing the first clause fails the AC's own cap clause and vice versa — and an implementer cannot tell which marker-eviction rule to build.
Fix: state "never evicts an entry or tombstone; under the cap it evicts only the namespace's oldest marker (R6/R16)".

**F3 [MEDIUM] AC43 — tombstone installation is still asserted unconditionally (r33-F2).**
§4 AC43: "after any trigger, … the tombstone is counted in accounting per R7", and "a commit landing on the retired namespace is delivered as `result_namespace_retired`". §2 R7 and §4 AC38 require an aggregate-infeasible retirement to install nothing, persist nothing, and be retried by a later sweep. The only qualification r34 added is in AC38 ("the completing path AC43 asserts unconditionally"), which describes AC43 rather than changing it.
Consequence: under aggregate pressure AC43 is unsatisfiable; a correct deferred retirement fails the oracle, and forcing immediate installation violates R7 and the hard aggregate bound.
Fix: qualify AC43 itself — "when the tombstone is admissible per R7; otherwise the retirement stays un-finalized per AC38 and is retried by a sweep" — or restrict its trigger assertions explicitly to the completing path.

**F4 [LOW] R9/§6 — TTLs are not pinned at record creation.**
§2 R9 pins `upstream_code` and page/preview sizing against reload but says nothing about `handle_ttl`/`tombstone_ttl`; AC29 reloads only the ceilings. If a reload is applied lazily, entry/handle deadlines (R4: "a handle never outlives a retrievable entry") and marker/tombstone deadlines (R16) can invert, and nothing tests it.
Fix: pin each entry's `handle_ttl` deadline and each marker/tombstone's `tombstone_ttl` at creation, state that a reload does not move existing deadlines, and extend AC29 or add a TTL-reload oracle.

**F5 [LOW] AC1 — no per-surface byte literal.**
§4 AC1 requires "an at/below-ceiling golden fixture … delivered byte-identically on each surface", but the four surfaces have different pre-change result envelopes, so one fixture cannot supply the literal for all four.
Fix: require one golden literal (expected byte string + length) per surface, per R2's sole oracle form.

### Positive observations
All six carried invariants are present and testable: independent golden-literal oracles (R1/R2, AC1/2/6/37/40), retired-marker lifetime and aggregate accounting (R7/R16, AC5/8/18/39), pinned `upstream_code` (R8/R9, AC9/10/29), finite label registries with producers (R12, AC13/44 — the 18+11+2=31 arithmetic and the grounded `eventMessage`/`reasonVocabulary`/`diagnosticCodes` sets check out), per-terminal bounds (R10, AC11/26), and ceiling-order validation (R11(a)–(k), AC12/20–23/35/42). Write roots are exact files and task staging follows the owner ruling.

### Recommendation
REQUEST CHANGES