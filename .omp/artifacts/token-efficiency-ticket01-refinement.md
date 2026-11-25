# Ticket 01 refinement — bounded results with recoverable paging

Status: READY. Produced by read-only refinement (`scout`, role `issue-refiner` was not admissible on the read-only lane: `E_READONLY_NOT_EXEMPT`). Worktree unmodified.

## Authoritative intake

`.scratch/token-efficient-tool-responses/issues/01-bounded-results-with-paging.md` (unchanged; its 9 ACs are the acceptance contract).

## Refined wording supplied to planner

### What to build

No bridge tool response may deliver an unbounded payload. The ceiling is measured on the exact serialized `tools/call` response the bridge would write for that request — every content block plus `structuredContent`, after JSON escaping — not on the upstream payload and not on `upstream_defaults.max_response_bytes` (a transport cap, `internal/config/bounds.go:86`). A response of <=8192 bytes is delivered byte-identically to today. A larger response is delivered as a bounded preview plus an opaque bridge-issued handle; the whole preview delivery, duplicated text and structured forms included, is <=1536 bytes. The complete would-be response stays retrievable through a documented, bounded retrieval operation accepting the handle and a page selector. The ceiling applies to `call_tool`, directly invoked projected tools, `search_tools`, and `describe_tool`. `isError` and error code remain visible in preview and every page. Handles are crypto-random, content-independent, and bound to the issuing session incarnation (session ID plus exposure epoch), not to the ID string alone. Storage is bounded per session and in aggregate. This ticket only bounds; it adds no default projections (05) or deduplication (06).

### Acceptance criteria

- R1. Over-ceiling result: delivered as preview+handle; preview reports `total_bytes` (the unbounded serialized length) and `paging_required`; preview delivery <=1536 bytes.
- R2. <=8192-byte result is byte-identical to current output (no handle, no injected metadata); 8192 passes, 8193 pages.
- R3. Each retrieved page delivers <=8192 bytes and reports 1-based page index, total pages, remaining pages, total bytes, remaining bytes.
- R4. Concatenated pages reproduce the unbounded response byte-exactly; re-retrieval of the same handle/page returns identical bytes until expiry or eviction.
- R5. Page boundaries yield valid UTF-8 pages; escaping-inflated bytes are what the ceiling measures; multibyte content spanning a boundary round-trips.
- R6. The retrieval operation and its parameters are documented in `README.md`, appear in the tool description/schema, and are usable without the caller predicting result shape.
- R7. Handles are crypto-random and not derived from content; tampered, unknown, or other-incarnation retrieval fails with the same typed rejection, zero content, and no existence signal.
- R8. Expired and evicted handles fail with distinct typed error codes, never empty or partial success.
- R9. Per-session entry/byte bound plus documented aggregate cap; eviction is deterministic; one session cannot evict another's live handles; eviction of a handle yields the evicted error; concurrent retrieval returns a complete page or a typed error, never a partial page.
- R10. Oversized upstream `isError` results keep `isError` and error code in preview and every page; bridge dispatch failures keep their existing small typed shape and are never converted to a paged success.
- R11. Ceiling behavior is identical across `call_tool`, directly invoked projected tools, `search_tools`, and `describe_tool`; a projection or full-content request never exceeds the ceiling.
- R12. Tests cover R1-R11: pass-through at/below boundary, multi-page byte reconstruction, UTF-8 boundary, cross-session and cross-incarnation rejection, forged handle, expiry, eviction and isolation, error-result paging, both dispatch paths, discovery/describe, and serialized byte bounds.

## Mapping (original AC O1-O9)

O1->R1,R2; O2->R3,R4; O3->R7; O4->R8; O5->R9; O6->R10; O7->R11; O8->R11; O9->R12.
Gaps added, justified: G1 repeatable retrieval (R4, required by ticket 06); G2 post-escape/UTF-8 page integrity (R5); G3 retrieval surface was unspecified (R6); G4 incarnation binding and forged-handle rejection (R7, session rehydration reuses an ID); G5/G6 cross-session non-interference and concurrency (R9). One removed clause: paging bridge dispatch failures, because `DispatchError` carries no payload (`internal/core/core.go:77-82`) and `errorResult` emits a fixed small shape (`internal/service/tools.go:1037-1048`).

## Sibling interactions (named, not resolved)

- 02: its <=2 KiB target sits below this ceiling, so search paging should never fire after 02 lands; "truncated" means result-count clipping there versus byte paging here.
- 05: `full` collides. 05's `full` means unprojected fields; 01 must not define a competing `full`. The documented "complete upstream `CallToolResult`" promise (`README.md:470-476`) changes and must be documented.
- 06: depends on R4/R6 repeatable retrieval and needs paged responses machine-distinguishable so it does not nest.

## Anchors

`internal/service/tools.go:865-908` (duplicate envelope), `:457-471`, `:624-655` (discovery/describe marshal), `:315-321` + `internal/service/session_exposure.go:146-172` (session + epoch), `internal/service/projection.go:195-202`, `internal/service/direct_execution.go:29-52,59-63`, `internal/catalog/schema.go:74-89` (`additionalProperties:false` — retrieval needs a new surface or schema change), `internal/service/cursor.go:1-80` (signed-cursor precedent), `internal/config/bounds.go:86` (transport cap).
