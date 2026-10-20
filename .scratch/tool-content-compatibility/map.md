# Tool content compatibility — task frontier

Canonical specification: [Tool content compatibility specification](spec.md).

The graph below expresses ticket dependencies. The dependency frontier is 01, unblocked and ready for agent implementation.

| Ticket | Blocked by | Current state |
| --- | --- | --- |
| [01 — Service-layer tool content fallback population](issues/01-service-tool-content-population.md) | None | resolved |
| [02 — HTTP wire contract verification and full-suite coverage](issues/02-http-wire-contract-verification.md) | 01 | resolved |

## Decisions so far

- The user approved the single HTTP integration seam (`POST /mcp`) using the real composition root and loopback client sessions.
- In accordance with repository rules in `AGENTS.md`, coordinator will delegate implementation dispatches to `engineer` / `test-engineer` subagents using TDD (deterministic RED before GREEN).
- Both tickets are resolved: the service layer and HTTP wire test implementations are present and verified in `main` at `2fbc290` (and `origin/main`). Full-suite source-line coverage stands at 91.6% (gate >=90% passed).
- Rebuilding the bridge Docker image from current `main` (`2fbc290`) delivers the fix to the running environment.
