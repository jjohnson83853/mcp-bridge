# Endpoint reconfiguration reconciliation — task frontier

Canonical specification: [Endpoint reconfiguration reconciliation](spec.md).

| Ticket | Blocked by | Current state |
| --- | --- | --- |
| [01 — Storage reconciliation endpoint rebind and cache eviction](issues/01-storage-reconciliation-endpoint-rebind.md) | None | resolved |
| [02 — Service startup admission and dynamic reload for reconfigured endpoints](issues/02-service-startup-and-reload-admission.md) | 01 | resolved |
| [03 — End-to-end CLI process and state persistence acceptance tests](issues/03-end-to-end-process-and-state-persistence.md) | 02 | resolved |

## Decisions so far
- Implement via TDD with `engineer` subagent.
- Ticket 01 resolved: `internal/storage/reconcile.go` rebinds modified endpoint fingerprints and purges obsolete tools/vectors while preserving unaffected servers.
- Ticket 01 unblocked and ready on the frontier.
- Ticket 02 resolved: `internal/service/reload.go` checks endpoint fingerprint matching before restoring snapshot definitions, resetting changed servers to unprobed with 0 tools while preserving unaffected servers.
- Ticket 03 resolved: `cmd/mcp-bridge/serve_endpoint_rebind_test.go` verifies cold restart and dynamic SIGHUP reload against real disk snapshots, HTTP readiness, tool catalog transitions, and durable state updates.
