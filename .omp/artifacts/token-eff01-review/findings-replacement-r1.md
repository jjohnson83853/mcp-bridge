Verdict: REQUEST CHANGES

**Scope:** specification mode, initial pass on the on-disk replacement (16,727-byte file, revision `r1-replacement`); inputs: issue 01 ACs, refinement R1–R12, packet K1–K10, grounding sources. Limit: no shell/Git, so I could not recompute the packet digest; the packet also still carries five stale predecessor candidate lines. Findings below judge only the replacement's sections.

### Findings

**F1 — [HIGH] Aggregate retention is declared advisory but asserted as a hard bound; overflow and isolation are undefined.**
Sections: R6 vs §6 `max_bytes_aggregate` ("advisory") vs §8 OD3. R6 claims a per-process aggregate bound covering every retained byte "so retention cannot grow unboundedly", yet §6 makes it advisory and AC7 only asserts accounting. Namespace count is unbounded, so 16 MiB per namespace × N namespaces is unbounded; R10's terminal list is exhaustive and has no aggregate outcome, so an enforced overflow has no defined delivery. No rule forbids one namespace evicting another's live handles (K4, refinement R9). Consequence: memory exhaustion under many sessions, or a fourth untyped terminal. Fix: either make the aggregate hard with a named admission outcome (add it to R10/R10's terminal set and AC11) or delete R6's causal claim; state the no-cross-namespace-eviction invariant and add its AC.

**F2 — [HIGH] Handle rejection is not required to be indistinguishable or content-free.**
Sections: R4, §3 retrieval failures, AC4. §3 groups unknown/forged/cross-namespace/retired under `invalid_handle`, but no requirement or AC asserts identical serialized shape, zero content, or absence of an existence signal; AC4 checks only the code for cross-namespace and post-epoch-bump. Consequence: a foreign/forged handle can return a distinguishable message or page residue and still pass (K3, refinement R7). Fix: state that every handle rejection is byte-identical (same code, zero content, no page, same timing class) and add an AC comparing full serialized rejections for unknown vs forged vs foreign vs retired.

**F3 — [HIGH] Page-boundary UTF-8 safety is not required for pages; only previews carry it.**
Sections: R5, AC6 (contrast R3). Pages are slices of the serialized JSON, which may split a multibyte rune; Go JSON string encoding replaces invalid bytes with U+FFFD, so R5's exact concatenation breaks. R3 mandates a UTF-8-safe prefix but R5 says nothing about rune alignment, and AC6 requires no boundary-spanning multibyte fixture. Consequence: multibyte payloads lose bytes silently while an ASCII-fixture AC6 passes (refinement R5/R12, K8). Fix: require each page to be a rune-aligned valid-UTF-8 slice, and add an AC fixture with a multibyte character spanning the boundary asserting byte-exact reconstruction and `utf8.Valid` per page.

**F4 — [MEDIUM] Evicted-handle rejection has no acceptance oracle.**
Sections: R6, §3 (`result_evicted`), AC5/AC7. AC5 covers expiry; AC7 stops at FIFO eviction and accounting. Nothing requires that retrieving an evicted handle returns `result_evicted` rather than an empty or partial page (intake O4, refinement R8). Fix: add an AC asserting exactly `result_evicted`, zero content, no page, with expired and evicted codes distinct.

**F5 — [MEDIUM] Surface uniformity and projection coverage are not falsifiable.**
Sections: §3 Surfaces, T3, AC1–AC14. The ACs never name `call_tool` vs directly invoked projected tools vs `search_tools`/`describe_tool`, and no fixture is a projection-bearing oversized result. A seam wired only into `call_tool` passes every AC while direct invocation and discovery stay unbounded (intake O7/O8, refinement R11, K7). Fix: add one parameterized AC (or per-surface AC) covering all four surfaces plus a projection-bearing oversized upstream result.

**F6 — [MEDIUM] Config relations beyond ceiling order have no acceptance oracle, and the minimum viable preview envelope is unvalidated.**
Sections: R11, §6, AC12, T1. AC12 tests only equal/inverted `preview_ceiling_bytes` vs `delivery_ceiling_bytes`; the documented `max_retained_result_bytes > delivery_ceiling_bytes`, `max_bytes_per_namespace ≥ max_retained_result_bytes`, and TTL rules are asserted by T1 but no AC fails on violation. Also no rule ensures `preview_ceiling_bytes` admits the mandatory handle/metadata envelope, so a legal config makes R3 unsatisfiable at runtime. Fix: state every relation as a requirement with its own AC, and add a minimum-feasibility check (preview envelope and page envelope must fit their ceilings).

**F7 — [MEDIUM] Terminal/page fields and out-of-range semantics are undefined, hence untestable.**
Sections: §3 Terminals, Page; R3/R5/R10; AC11. `retention` has no value domain, `execution_state: "unknown"` contradicts R8's preserved execution facts (the upstream did execute for entropy failure), `total_bytes` is never normatively defined (original measured vs retained bytes), page > `total_pages` and `invalid_arguments` have no required behavior (ACs never assert shape-invalid input), and the spec relies on a schema `pattern` whose enforcement for built-ins is not established by the cited path. Fix: enumerate `retention`, derive `execution_state` from the real fact, define `total_bytes`, and require handler-side argument/page validation with an AC.

**F8 — [MEDIUM] The public documentation acceptance is dropped.**
Sections: T4, §8 (not scoped out); refinement R6, K10. T4 only updates the built-in list; nothing requires README (and the tool description) to document the handle/page retrieval contract or to replace the "complete upstream `CallToolResult`" promise. Consequence: the documented contract change ships silently. Fix: add a documentation requirement and positive assertions in T4 on the new retrieval parameters and the replaced claim.

**F9 — [LOW] AC14 is owned by T2 but depends on T3's wiring.**
Sections: AC14, §7 ("T2 registers, T3 binds"), T2 GREEN ("no handler wiring"). Exact before/after deltas for `bridge_results_paged_total`/`bridge_result_pages_served_total` require the delivery seam, which T2 explicitly excludes. Fix: move AC14 to T3, or split it into registration-only (T2) and delta (T3).

### Positive observations
The six carried invariants are present and testable: independent duplicate-bearing measurement oracle (R2/AC2), tombstone lifetime plus clearing condition and accounting (R7/AC8, §6), upstream_code pinning (R9/AC10), finite enumerations with producer assertions (R12/AC13), per-terminal preview bounds (R10/AC11), and ceiling-order rejection (R11/AC12). Bridge dispatch failures are explicitly kept small and unpaged (R8), and the task chain T1→T2→T3→T4 is acyclic with per-task write roots.

REQUEST CHANGES