# 03 — Search must not fan out matched schemas into later prompts

**Type:** task
**Priority:** P0
**Status:** ready-for-agent
**Blocked by:** None — can start immediately.

**What to build:** Searching no longer mutates what the session sees in later `tools/list` pages. Match sets returned by `search_tools` do not add tool declarations; exposure of a tool’s full schema becomes an explicit, caller-requested action, and configured pins keep behaving as today. A compact search response must not be followed by a silent expansion of every matched tool’s schema, which currently re-sends the catalog on subsequent turns.

**Notes:** Today the search handler commits every discovered target into the session projection, so a compact response can still cost a full schema dump on the next turn. Decide and document the explicit exposure request; it must preserve the existing invariant that only tools the session legitimately knows can be called directly. Do not weaken session ownership, projection epoch, or catalog-revision checks, and do not auto-expose on describe either unless the ticket’s design says so explicitly.

- [ ] After `search_tools`, a subsequent `tools/list` for the same session contains no tool added solely because it matched the query.
- [ ] The caller can explicitly request exposure of a chosen tool set, bounded in size, with deterministic success or rejection.
- [ ] Configured pins continue to appear as before for publisher and session projections.
- [ ] Direct invocation of a tool that was never pinned or explicitly exposed is still rejected exactly as today.
- [ ] Explicitly exposed tools appear only in the requesting session, and other sessions’ projections are unaffected.
- [ ] Catalog revision changes still invalidate projections and preserve the existing owner/epoch checks.
- [ ] Tests cover: no-implicit-exposure after search, explicit exposure success, exposure bound rejection, pin preservation, and cross-session isolation.
