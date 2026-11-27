## Code Review Summary

**Mode:** specification · **Pass:** initial (replacement document, `r9-replacement`) · **Files reviewed:** 1 (spec) + grounding sources

Verdict: **REQUEST CHANGES**

The six carried invariants are all present and oracled: independent golden-literal byte oracle (R2/AC2, AC6), tombstone/marker lifetime + accounting (R7/R16/§6), pinned `upstream_code` (R9/AC10), closed label enumerations (R12 — verified against `internal/observe/log.go:156-160`, `httpdiagnostics.go:660`), per-terminal preview bounds (R10/R11(g)/AC11/AC26), and ceiling-order validation (R11(a)(b)(c)(h)/AC12/AC20/AC21/AC35). Findings below are what remains.

### Issues

**F1 [HIGH] Aggregate "HARD" bound is violated by the spec's own record creation**
Section: R6, R7, R16, §8 OD3; AC7/AC15/AC18. Defect: tombstones and sweep markers are counted against `max_bytes_aggregate` at 256 B (R7/R16/§6) but are installed by retirement/expiry/eviction, which R6 never admission-gates; OD3's required headroom, `(max_sessions + 1) × 256`, omits per-entry markers created whenever an entry expires or is evicted, whose live count is bounded by eviction rate × `tombstone_ttl`, not session count. Consequence: the total can exceed a bound the spec calls HARD, admission can terminally refuse while the documented headroom says otherwise, and no AC exercises it. Fix: state that marker/tombstone installation is aggregate-checked with a defined outcome when the reserve is exhausted (or cap live markers per namespace), state that only the sweep reduces the total, and replace OD3's formula with a correct relation.

**F2 [MEDIUM] Four-surface claim has producers for only two surfaces**
Section: R8 ("Producer per surface"), AC19. Defect: R8 names producers only for `call_tool` and directly invoked projected tools, yet AC19 requires "R8's per-surface `upstream_code` producer" on `search_tools` and `describe_tool`, whose results have no upstream execution. Consequence: the expected `is_error`/`upstream_code` metadata for two of four surfaces is unimplementable-as-written; AC19 cannot assert it. Fix: state that bridge-generated discovery/describe previews and pages carry `is_error: false` and omit `upstream_code` (no producer), and mirror that in AC19.

**F3 [MEDIUM] Five config relations have no named diagnostic code**
Section: R11(b)–(f), §6 table; AC20/AC21/AC22/AC33/AC34. Defect: `diagnosticCodes` is a closed set and T1 registers only `ceiling_order`, `envelope_feasibility`, `aggregate_order`, yet the entry-cap, namespace-cap, TTL, entries and positivity relations say only "typed diagnostic" while ACs 23/35 do name codes. Consequence: a test cannot deterministically assert which code is emitted; two conforming implementations pass different oracles. Fix: name the code per relation (or one shared `retention_bound`), or state explicitly that these reuse `bound_range`/`duration_positive`.

**F4 [MEDIUM] Pass-through byte-identity is oracled on `call_tool` only**
Section: R1, §3 "Surfaces"; AC1, AC19. Defect: the seam is mandatory on four surfaces, but AC1/AC6 run at T3 on `call_tool`; AC19 asserts only the over-ceiling preview contract, so a re-marshal regression on `search_tools`, `describe_tool` or direct invocation passes every AC. AC1/AC2 also never assert that ceiling+1 pages (refinement R2's "8192 passes, 8193 pages" boundary). Fix: extend AC19 (or add an AC) to assert golden-literal, at/below-ceiling byte-identical pass-through per surface, and to assert the ceiling+1 delivery decision.

**F5 [MEDIUM] Aggregate admission's eviction ordering is unspecified; AC15 is under-determined**
Section: R6; AC15. Defect: "within it after the commit" leaves open whether aggregate pressure may or must trigger FIFO eviction inside the admitting namespace; AC15 asserts a terminal while forbidding eviction only of *other* namespaces' entries. Consequence: whether AC15 passes depends on an unstated implementation choice, and the operator-visible meaning of "HARD" differs. Fix: state that aggregate pressure alone triggers no eviction, or that within-namespace FIFO eviction runs first and the terminal occurs only when it cannot free enough — then align AC15's precondition.

**F6 [LOW] `operation` gloss is false**
Section: R12; AC13. Defect: "all existing operation values are unchanged, so `operation` ∈ the four tool surfaces" — the retained vocabulary still contains `search`, `describe`, and others. Fix: say paging events log only those four values.

**F7 [LOW] Precedence omits expired/evicted handles with an out-of-range page**
Section: R13; AC32. Defect: the precedence chain names only `invalid_handle` at resolution, leaving `result_expired`/`result_evicted` vs `invalid_arguments` undefined for a live-shaped but expired handle with `page` > `total_pages`. Fix: extend R13 and AC32 to pin these two combinations.

**F8 [LOW] T5 grant is not an exact write root**
Section: T5. Defect: "write roots (exact files only)" is immediately followed by a conditional grant to edit `cmd/mcp-bridge/serve.go`, outside the named set. Fix: keep `serve_test.go` as the only assertion surface; route any proven `serve.go` change to a separately authorized task.

### Positive Observations
- R2's restriction to golden literals, and AC6's "not a retained artifact, not re-marshaled", give genuinely independent oracles.
- Grounded claims check out: `call_tool` is un-gated (`session_exposure.go:70-75`), inbound stamps only when epoch≠0 (`inbound.go:596`), so AC31 is reachable; README anchors exist (`:277`, `:441`) and the promise to qualify is `README.md:468`.
- Metadata sets are single-sourced (R3/R5/R10) and referenced, so the document avoids the predecessor's per-surface repetition.

### Review Limits
Specification mode only: no implementation exists to execute; all findings are textual or grounded in cited source.

### Recommendation
REQUEST CHANGES