# Logging observability — task frontier

Canonical specification: [LOG-OBS-r5](spec.md); ticket source r4 remains historical provenance. The r5 amendment changes design/testing prerequisites, not requirements, acceptance text or the graph.
Reviews and explicit approvals: [checkpoint evidence](checkpoints.md).

The graph below expresses ticket dependencies. The dependency frontier is 01, now claimed after the three explicit logging checkpoints and independent reviews. The coordinator is appending a logging-specific gate record for canonical r5; older gate entries are not the authorization source. No implementation or acceptance result is claimed by opening the gate.

| Ticket | Blocked by | Current state |
| --- | --- | --- |
| [01 — HTTP request diagnostics](issues/01-http-request-diagnostics.md) | None | complete |
| [02 — Correlated MCP/backend execution](issues/02-correlated-mcp-backend-execution.md) | 01 | complete |
| [03 — Protocol/tool/backend failures](issues/03-protocol-tool-backend-failures.md) | 02 | complete |
| [04 — Upstream/client response shapes](issues/04-upstream-client-response-shapes.md) | 02 | complete |
| [05 — Streaming/cancellation diagnostics](issues/05-stream-timeout-cancellation-diagnostics.md) | 03 | complete |
| [06 — Concurrent/reload isolation](issues/06-concurrent-reload-correlation-isolation.md) | 04 | complete |

## Decisions so far

- The user approved the existing real HTTP/MCP composition-root harness and publication of these six tickets.
- The user separately selected **Approve requirements r4**, **Approve design r5**, and **Approve tasks r5 and implement**, each after its required independent review. r5 leaves requirements/AC text unchanged. Exact artifact hashes and review/approval linkage are recorded in the checkpoint evidence.
- The user selected **engineer** for actual implementation. Repository policy requires direct-to-main work with one integration owner for overlapping source changes; no integration/task branches or implementation worktrees.
- Only ticket 01 is claimed; no ticket is implemented or resolved. Actual RED/GREEN, feature smoke, full-suite source-line coverage and code review evidence must precede resolution. A completed ticket receives its own Answer section and a corresponding decision/answer pointer here.
- Source-only readiness, proposal security review and SDK-only streaming probe are preparatory evidence, not app acceptance. Existing public/auth/session/network/metrics/LibreChat behavior remains unchanged.
