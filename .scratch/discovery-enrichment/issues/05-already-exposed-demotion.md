# 05 — Already-exposed demotion in search results

**Type:** task

**What to build:** Search stops spending its top slots on tools the model already sees in its own schema. Results mark which entries are already visible in this session's `tools/list` (pinned or progressively revealed) and rank comparable-score undiscovered tools first.

**Blocked by:** 03.

**Status:** ready-for-agent

- [ ] Each result entry reports `already_exposed` when that tool is currently visible to the session through pinning or progressive reveal.
- [ ] Already-exposed entries are demoted below unexposed entries of comparable score; the demotion is deterministic, bounded, and does not reorder clearly stronger matches.
- [ ] When every eligible match is already exposed, results are returned in their normal score order rather than an empty or degraded list.
- [ ] Sessions with no pin or reveal state return exactly today's ordering.
- [ ] Tests cover revealed-tool demotion, all-exposed fallback, and no-state equivalence.