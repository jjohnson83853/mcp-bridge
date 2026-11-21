# 04 — Tool co-occurrence ranking boost

**Type:** task

**What to build:** After a session calls a prerequisite tool, discovery ranks its likely follow-ups higher. Learning `EndpointList → docker_proxy` or `list_datasources → query_loki_logs` from real usage means the next step is surfaced first instead of rediscovered.

**Blocked by:** 03.

**Status:** ready-for-agent

- [ ] Successful dispatched calls record a `previous tool → next tool` transition for the session, keyed by public tool name, in the durable store.
- [ ] Transition counts are bounded (per-key cap and a prune policy) and persist across restart; a cold store behaves exactly as today.
- [ ] `search_tools` boosts tools that follow the session's last successfully called tool; the boost is bounded so it cannot outrank a clearly stronger lexical or semantic match.
- [ ] Failed or rejected calls do not contribute transitions, and a tool called twice in a row neither inflates its own count unboundedly nor boosts itself.
- [ ] Storage read or write failure degrades to no boost and never fails the search.
- [ ] Tests cover the boost ordering, cold-store equivalence, failure exclusion, and persistence bound.