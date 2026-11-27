# Ticket 02 refinement — compact `search_tools` index (#116)

Status: **READY**.

## Source identity

- Tracker: GitHub `jjohnson83853/mcp-bridge#116`, OPEN, labels `status:ready-for-agent`, `type:ticket`, `migrated`.
- Issue updatedAt: 2026-11-26T14:32:22Z.
- Supplied body sha256: `ffdf8986f121d53af965d963b9510e7170521cf21f8814c9bfda33bdee27ade7`.
- Parent: #157 (`token-efficient-tool-responses` specification).

## Invocations

1. `RefineTicket116` (agent `issue-refiner`), read-only, 2026-11-27 — **BLOCKED**: named five contract gaps (fixed small cap vs configured limits; minimum invocation provenance; pre/post-clip count; byte-bound domain; AC7 scope vs session exposure).
2. `RefineTicket116b` (agent `issue-refiner`), read-only, 2026-11-27 — **READY** after coordinator supplied verified source evidence resolving all five gaps as planner-resolvable.

## Assessment (second pass, verbatim)

READY — jjohnson83853/mcp-bridge#116, updated 2026-11-26T14:32:22Z; supplied body sha256 ffdf8986f121d53af965d963b9510e7170521cf21f8814c9bfda33bdee27ade7. Live issue read; code facts below use supplied verified evidence.

Five gap dispositions — all (i), planner-resolvable:

1. Small limits/rejection (AC3): search/search.go:140-181 already rejects invalid top_k. Planner chooses small numeric limits and aligns advertised/effective defaults and maxima. A bridge cap can preserve config validity; lowering operator bounds risks existing max_top_k=50 deployments. That avoidable compatibility break is not required by this ticket.
2. Invocation minimum (AC1): service/tools.go:573-602 requires tool_name, optional catalog_revision, arguments. Retain tool_name; server/upstream_name are unnecessary even for opaque names. Keep internal identity data for discovery validation, not redundant wire fields.
3. Count/clipping (AC4): search/search.go:284-293,350-449 exposes pre-TopK population. match_count means matches before count clipping; truncated means returned count < match_count. No byte-paging mechanism is required.
4. Byte domain (AC5): service/result_measure.go:9-14 defines len(json.Marshal(*mcp.CallToolResult)), including text/structuredContent duplication. Use that domain for the maximum-count long-name/description test.
5. Schema boundary (AC7): tools.go:507-538 and projection.go:367-430 use internal identity for exposure. Schema prohibition applies to discovery responses, not subsequent session tools/list. Ticket03 owns exposure changes.

Sizing/dependencies: one coherent response-contract change, coupling projection, count/description bounds, guidance and tests. Key risks are stale schema limits, duplicate serialization bytes and accidentally pruning internal discovery identity. No declared blocker; ticket03 is a boundary, not a prerequisite. No review outcome is promised.

Complete proposed request (AC1-AC8 remain authoritative by reference):

- R1. Make search_tools a compact index: each hit has only callable tool_name, bounded one-line description, score and any demonstrably necessary invocation provenance; no schemas or redundant fields. Bound descriptions by characters, truncate deterministically with a visible marker.
- R2. Use a small default and hard caller-visible maximum; reject above-maximum requests, never silently clamp them.
- R3. At the root retain catalog revision and degradation reasons; report pre-count-clip match_count and truncated when clipped. Preserve ranking, filters and session exposure.
- R4. Document the ≤2048-byte complete serialized CallToolResult target; test maximum-count responses with long names/descriptions, default count, cap rejection, clipping metadata and description bounds.
- R5. Keep describe_tool as the schema-returning discovery path. Document client-visible renames in README.md and live tool guidance (tools.go:103-105; builtin_guidance_test.go:15-83).

Bidirectional equivalence (original ↔ rewrite; no weaker/stronger contract):

- AC1 ↔ R1: same invocation-only hit fields/no schemas.
- AC2 ↔ R1: same character budget, deterministic marker.
- AC3 ↔ R2: same small limits/rejection, not clamping.
- AC4 ↔ R3: same root count/flag; count-clipping interpretation.
- AC5 ↔ R4: same maximum-count byte target/test; existing measurement domain.
- AC6 ↔ R3: same ranking/filter/degradation/revision preservation.
- AC7 ↔ R1,R5: same schema discovery boundary; exposure unchanged.
- AC8 ↔ R4: all five required test categories retained.
- R5 documentation ↔ issue Notes; R3 exposure exclusion ↔ What to build. No mandatory outcome removed or added.

Unresolved owner decisions/missing facts: none. Numeric budgets and compatible cap mechanics remain planner design work, not new scope. No writes, dispatch, tests or approval claims.

## Handoff (verbatim)

READY for planning #116 at supplied digest. All five gaps are planner-resolvable; assessment contains rewrite and AC1-AC8 equivalence. Preserve config validity/internal discovery identity; measure complete CallToolResult bytes. Not approval.

## Coordinator evidence supplied to the second pass

- `internal/search/search.go:140-181` decodes strictly, rejects out-of-range `top_k` (no clamping).
- `internal/search/search.go:20-28` pins `DefaultTopK=10`, `DefaultTopKMax=50`.
- `internal/config/bounds.go:86` admits `default_top_k`/`max_top_k` in 1..50; `configs/config.example.yaml:45-47` and `configs/config.mcp-grafana-stack.yaml:30-32` set 10/50.
- `internal/catalog/schema.go:24-40` advertises the effective default/max.
- `internal/service/tools.go:573-602` call_tool requires only `tool_name` (+ optional revision, arguments); projected tools invoke by public name.
- `internal/service/tools.go:987` shows semantic vs opaque (`t_`) wire names.
- `internal/service/tools.go:499-505` search result already passes through bounded delivery; `result_measure.go:9-14` measures the complete serialized `mcp.CallToolResult` (text + structured duplicates).
- `internal/service/tools.go:507-538` discovery-target validation uses internal `search.Response` identity; `projection.go:367-430` commits exposure. Neither depends on pruned payload fields.
- `internal/service/tools.go:103-105` live search guidance; `builtin_guidance_test.go:15-83` asserts it. `README.md:275-340` documents the discovery contract.
