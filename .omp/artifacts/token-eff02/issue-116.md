# Issue #116: [token-efficient-tool-responses] 02 — Compact `search_tools` index

State: OPEN
Labels: status:ready-for-agent, type:ticket, migrated
Updated: 2026-11-26T14:32:22Z

# 02 — Compact `search_tools` index

**Type:** task
**Priority:** P0
**Status:** ready-for-agent
**Blocked by:** None — can start immediately.

**What to build:** `search_tools` answers like an index, not a catalog dump. Each hit carries only the callable name, a bounded one-line description, the score, and the minimum provenance already needed to invoke it. The response targets ≤2 KiB, with a small default result count and a small hard cap; when results are clipped the response reports the match count and that truncation occurred. Full schemas are obtainable only through `describe_tool`. The existing search filters and ranking behavior remain available, and existing session exposure behavior is out of scope here (ticket 03).

**Notes:** The current search payload carries full upstream descriptions and defaults to ten results with a cap of fifty; the reported incident showed response size growing roughly linearly with result count. Keep the root-level catalog revision and degradation reasons the bridge already relies on, but do not repeat per-result fields that are constant or derivable. Client-visible field renames must be documented in `README.md` and reflected in the tool description guidance.

- [ ] Each search hit contains only name, bounded one-line description, score, and the minimum fields required to invoke it; no schema content is present.
- [ ] One-line descriptions are truncated deterministically with a visible marker and a bounded character budget, so a verbose upstream cannot inflate a hit.
- [ ] The default result count is small, the caller-visible maximum is small, and a request above the maximum is rejected rather than silently clamped.
- [ ] A response whose results were clipped carries an explicit match count and truncation flag at the response root.
- [ ] A search returning the maximum results stays within the documented response-size target; a test asserts the serialized byte bound with long descriptions and long tool names.
- [ ] Ranking, filtering, degradation reporting, and catalog revision behavior are unchanged apart from the pruned fields.
- [ ] `describe_tool` remains the only path that returns input/output schemas.
- [ ] Tests cover: default count, cap rejection, truncation metadata, description bounding, and byte-bound enforcement.


Part of #157 (token-efficient-tool-responses specification).

---
_Migrated from `.scratch/token-efficient-tool-responses/issues/02-compact-search-index.md` (original status: ready-for-agent) on 2026-11-26. The local Markdown tracker is no longer authoritative; this issue is._
