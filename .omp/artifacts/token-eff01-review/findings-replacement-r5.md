## Code Review Summary

**Subject:** specification mode, initial pass. Candidate `token-efficient-tool-responses-ticket01-spec`, on-disk `spec.md` (`r5-replacement`), 33,881 bytes; manifest pins sha256 `adcc24a5…`. I reviewed the bytes as read (read-only; digest not recomputable with granted tools). Limit: no r4 ledger exists to check round-4 closures against (F5).

**Carried invariants:** all six present — independent byte oracle (R2/AC2/AC6), retired-marker lifetime + accounting (R7/R16/AC5/AC8/AC18), pinned `upstream_code` (R8/R9/AC9/AC10/AC29), finite labels with producers (R12/§7/AC13), per-terminal preview bounds (R10/AC11/AC26), ceiling-order validation (R11(a)/AC12/AC28). Three of them carry residual defects (F2–F4).
**Round-3 closures independently verified:** findings 1–4 closed (R4 now scopes `retrieve_result` as the only handle surface; `requestExposureEpoch` is the named epoch source and inbound.go:481/596/632/664 stamps/strips it; R13 + AC32 fix precedence; AC8/AC16 split store/handler). Findings 5–7 closed (T3 write roots name the count-assertion files — verified `assertAdvertisedBuiltinsAndPin` tools_test.go:517, `builtin_guidance_test.go:177`, `TestToolSurfaceAdvertisesExactlyItsThreeTools` tools_test.go:552; R2 golden literal; AC28 straddle).

---

**F1 [HIGH] — §6 CONFIGURATION + §5 T1: `retained_results` cannot be loaded and T1's write roots omit the files that would allow it.**

`internal/config/config.go:648-666` is a literal per-section key allowlist (`"root": set("schema_version http … pins")`), `config.go:773-788` emits `yaml_key_unknown` for anything else, and `config.go:146-147` decodes with `strict.KnownFields(true)`; `limits` admits only `max_concurrent_calls call_timeout`, so neither a top-level nor a per-server `retained_results` block parses. New diagnostic codes `ceiling_order`/`envelope_feasibility` must also join the closed `diagnosticCodes` set (`diagnostics.go:46-97`). §6 never states whether the section is global or per-server, yet the store and `max_bytes_aggregate` are process-wide. T1's roots are `internal/service/result_measure*.go`, `internal/config/bounds.go`, `internal/config/bounds_edge_test.go` — no `config.go`, no `diagnostics.go`. Consequence: T1's config fixtures fail with `yaml_key_unknown` and the R11 codes do not exist, so T1 cannot reach GREEN and R11's diagnostics are unimplementable as planned. Fix: state the section's schema location (global, matching the process-wide store), add `internal/config/config.go` and `internal/config/diagnostics.go` to T1's write roots, and name the code registration.

**F2 [MEDIUM] — R3, §3 Preview, AC3/AC11: the preview bound's measured form is undefined.**

R2 pins the delivery measure (SDK-marshaled complete `CallToolResult`) but nothing states the measure for `preview_ceiling_bytes`; R3 says "preview ≤ preview_ceiling_bytes", §3 says "`Content` … ≤ preview ceiling", and AC3/AC11 only compare to the limit with no literal-length fixture. Consequence: an implementation that measures only `Content`, or measures pre-escaping, satisfies AC3/AC11 while the delivered preview exceeds 1536 bytes — the K1 duplicate-copy risk is unpinned on the preview surface. Fix: state that preview and page bounds are measured on the same R2 serialization of the delivered result, and give AC3/AC11 a literal expected length at maximal metadata (AC28's fixture).

**F3 [MEDIUM] — R11(f), §6, AC34: `max_bytes_aggregate` has no positivity relation.**

R11 claims relations (a)–(g) are validated and §6 marks the key HARD, but no relation covers it; AC34 pins positivity only for `delivery_ceiling_bytes`, `preview_ceiling_bytes`, `max_upstream_code_bytes`. Consequence: `max_bytes_aggregate: 0` (or negative) loads cleanly and converts every over-ceiling result into `result_aggregate_capacity_exceeded`, silently disabling paging with no load-time diagnostic. Fix: add `max_bytes_aggregate > 0` (and decide `≥ max_bytes_per_namespace`) to R11 and an AC34 subtest.

**F4 [MEDIUM] — R11(g) vs R10/AC11/AC28: terminal-preview feasibility is not validated.**

R11(g) computes feasibility only for the retained-preview and page envelopes at maximal metadata. A terminal adds `retention` (longest code 36 chars) plus `execution_state`, which the feasibility arithmetic never includes and the spec never shows to be contained in the retained-preview envelope. Consequence: a load-valid maximal-metadata config can leave no room for R10's required non-empty prefix on a terminal, and AC11/AC28 would not detect it. Fix: include the maximal terminal envelope in R11(g), or state and assert its containment in the retained-preview envelope.

**F5 [LOW] — §1 STATUS: closure of "round-4 findings F1–F9" is unverifiable.**

`.omp/artifacts/token-eff01-review/` holds only `findings-replacement-r1.md`, `-r2.md`, `-r3.md`; no r4 ledger exists, and dispositions are said to "travel in the coordinator handoff" rather than in a citable artifact. Consequence: a reviewer cannot confirm the claimed closures did not regress. Fix: name the disposition artifact path, or inline the round-4 IDs and their resolutions.

**F6 [LOW] — R3 vs §3 Preview; AC19 Covers: preview metadata is duplicated, not single-sourced.**

R5 explicitly single-sources the page metadata set, but the preview set is enumerated twice (R3 and §3 Preview) with drift risk; AC19's Covers column names only R3 although it tests cross-surface uniformity (R11) and per-surface producers (R8). Fix: single-source the preview set the way R5 does, and correct AC19's Covers to R3/R8/R11.

**F7 [LOW] — R2 vs intake Notes: the measure excludes the bytes actually written.**

The intake says "measure the serialized bytes actually written to the client" and refinement R1 says "the exact serialized `tools/call` response"; R2 measures only the marshaled `CallToolResult`, omitting the JSON-RPC envelope. The excess is constant and bounded, so boundedness holds, but the documented ceiling is not the wire bound. Fix: state that the JSON-RPC envelope is excluded and why, or include it.

**Positive observations.** The replacement is dense and testable: every R1–R16 maps to an AC, tasks carry exact write roots and RED lists, page metadata is deliberately single-sourced, R13/R4 precedence closes a real existence-leak, and R16 makes expiry/eviction deterministic without sweep scheduling.

### Recommendation
REQUEST CHANGES