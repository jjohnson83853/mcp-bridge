# Hybrid tool discovery — task frontier

Canonical specification: [HYBRID-DISCOVERY-r3](spec.md).
Reviews and explicit approvals: [checkpoints.md](checkpoints.md).

The graph below expresses ticket dependencies. The dependency frontier is 01, ready for checkpoint progression. No implementation or acceptance result is claimed by ticket creation.

| Ticket | Blocked by | Current state |
| --- | --- | --- |
| [01 — Preloaded tool configuration bounds and wire naming](issues/01-preloaded-tool-configuration-and-naming.md) | None | complete |
| [02 — Assembly catalog and tools/list preloaded exposure](issues/02-assembly-catalog-and-tools-list-exposure.md) | 01 | complete |
| [03 — Direct preloaded tool execution dispatch](issues/03-direct-preloaded-tool-execution-dispatch.md) | 02 | complete |
| [04 — Search coexistence and metadata alignment](issues/04-search-coexistence-and-metadata-alignment.md) | 03 | complete |
| [05 — Dynamic SIGHUP reload and tool list change notifications](issues/05-dynamic-reload-and-list-changed-notifications.md) | 04 | complete |

## Decisions so far

- The user approved the single HTTP integration seam (`POST /mcp`) using the real composition root and local fake upstreams.
- Implementation will strictly follow the newly added `AGENTS.md` guardrails: zero open-ended research dispatches, all application and test modifications delegated to `engineer` and `test-engineer` subagents, and no direct coordinator implementation.
- All 5 tickets published under `.scratch/hybrid-tool-discovery/issues/` with status `ready-for-agent`.
