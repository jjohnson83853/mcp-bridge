# 01 — Bounded tool results with recoverable paging

**Type:** task
**Priority:** P0
**Status:** ready-for-agent
**Blocked by:** None — can start immediately.

**What to build:** No bridge tool response can deliver an unbounded payload. When a serialized tool result exceeds the delivery ceiling (initial target: 8 KiB), the caller receives a short preview (≤1.5 KiB) plus an opaque, bridge-issued handle and explicit outcome metadata, and can retrieve the retained result through bounded paged retrieval that reports pages consumed and pages remaining. The same ceiling applies to generic `call_tool` results, directly invoked projected tools, and oversized discovery/describe responses. `full=true` or equivalent requests more content but still returns pages, never an unlimited body. Tool-reported error status and execution outcome remain visible in the preview so a failure is never mistaken for a truncation.

**Notes:** Measure the serialized bytes actually written to the client, including duplicated content-block/structured forms and escaping, not just the underlying payload. Handles require caller isolation, expiry, bounded storage, and explicit expired/evicted errors rather than silent empty pages. This ticket bounds the worst case; it does not add default field projections (ticket 05) or deduplication (ticket 06).

- [ ] A result whose serialized form exceeds the ceiling is delivered as preview plus handle, and the preview reports the original byte size and that paging is required.
- [ ] Paged retrieval returns at most the ceiling per page, reports total/remaining pages, and reconstructs the complete original result across pages.
- [ ] Handles are unguessable, scoped to the session that received the result, and unusable by another session or owner.
- [ ] Expired or evicted handles fail with a distinct typed error rather than an empty or partial success.
- [ ] Retained-result storage has a configured byte/entry bound; exceeding it evicts deterministically without corrupting live handles.
- [ ] Upstream `isError` results and dispatch failures keep their error classification in both preview and paged retrieval.
- [ ] The ceiling applies identically to `call_tool` and to directly invoked projected tools.
- [ ] A large result reached through a request that already specified a projection is still bounded; paging does not depend on the caller predicting output shape.
- [ ] Tests cover: under-ceiling pass-through, multi-page reconstruction, cross-session handle rejection, expiry, eviction, error results, and byte-level ceiling enforcement.
