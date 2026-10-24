Status: ready-for-agent

## Problem Statement

When an operator updates the network endpoint of an existing upstream server in the configuration file—such as changing Home Assistant's port from `8000` to `8080`, migrating from HTTP to HTTPS, or pointing to a different IP or hostname—the MCP bridge crashes during startup reconciliation with the error:
`service: startup reconcile: storage: server endpoint identity changed: server "homeassistant"`

Because the bridge crashes on startup, the entire service becomes unavailable to client assistants and applications. The operator cannot resolve the failure simply by restarting the service; they are forced to manually intervene in the filesystem to delete or edit the internal state snapshot (`snapshot.json`), or invent a new synthetic server identifier (e.g. `homeassistant_v2`), which in turn permanently tombstones the original server name.

This behavior contradicts the declarative contract of configuration files: modifying an upstream server's address is a standard operational maintenance task and should automatically evict obsolete cached definitions and connect to the new endpoint, rather than halting the process.

## Solution

1. **Reconciliation on Endpoint Reconfiguration**:
   Allow the durable storage reconciliation cycle to accept a modified endpoint fingerprint for an actively configured server instead of rejecting the change with a fatal error. When the configured endpoint fingerprint differs from the binding in the previous snapshot, update the binding to the new endpoint fingerprint.

2. **Cache Eviction and Re-Discovery for Reconfigured Endpoints**:
   When an existing server's endpoint changes:
   - Evict previously cached tool definitions and embedding vectors associated with that server, guaranteeing that tools from the old target address are never exposed or invoked against the new endpoint.
   - At startup, initialize the reconfigured server in an unprobed status (`none`) with an empty tool set, exactly like a newly introduced server, allowing asynchronous post-startup discovery to probe the new endpoint and populate fresh tools.
   - During runtime signal reloads, establish a new dedicated client for the new address, execute discovery against the new endpoint, and atomic-swap the catalog only after successful discovery.
   - Keep all other unaffected upstream servers, their bindings, their cached tools, and their embedding vectors completely intact.

## User Stories

1. As a homelab operator, I want to change an upstream server's port in the configuration file, so that the MCP bridge connects to the new port without failing startup.
2. As a homelab operator, I want to change an upstream server's hostname or IP address, so that the MCP bridge tracks infrastructure moves seamlessly.
3. As a homelab operator, I want to switch an upstream server from HTTP to HTTPS, so that communication is encrypted without corrupting or breaking bridge state.
4. As an AI assistant user, I want the MCP bridge to start successfully after upstream port changes, so that my assistant does not experience service outages or connection drops.
5. As an AI assistant, I want stale tools from the old endpoint to be discarded immediately upon endpoint change, so that I do not attempt to invoke tools that no longer exist or belong to a different service.
6. As an AI assistant, I want newly discovered tools from the updated endpoint to be searchable and executable as soon as discovery completes.
7. As an operator, I want the bridge to avoid crashing into a restart loop when an endpoint address is updated.
8. As an operator, I want to update an upstream server's endpoint without needing shell or filesystem access to delete internal state snapshots.
9. As an operator, I want to retain the same human-meaningful server name (e.g., `homeassistant`) across network migrations, rather than having to rename it to avoid tombstone errors.
10. As an operator, I want other unchanged upstream servers (e.g., `portainer`, `github`) to retain their cached tools and embedding vectors across an unrelated server's endpoint change.
11. As an operator, I want startup to remain fast and resilient by admitting the reconfigured server with an empty catalog, while unaffected servers continue serving their warm cached catalogs immediately.
12. As an operator, I want post-startup discovery to asynchronously probe the new endpoint and publish its tools without blocking the bridge's initial availability.
13. As an operator, I want the durable snapshot on disk to be updated with the new endpoint fingerprint once reconciliation succeeds.
14. As an operator, I want sending a reload signal (SIGHUP) after editing an endpoint in the configuration to smoothly transition the connection to the new endpoint without restarting the bridge process.
15. As an operator, I want an in-flight reload to reject the candidate gracefully and retain the existing working catalog if the new endpoint is completely unreachable during a reload pass.
16. As an operator, I want the health and readiness reporting endpoints to accurately reflect that the reconfigured server is pending discovery immediately after startup.
17. As an operator, I want embedding vector search to remain accurate by purging old embedding records associated with the departed endpoint and indexing newly discovered tools.
18. As a developer, I want regression tests verifying that modifying an endpoint fingerprint in storage reconciliation produces an updated binding rather than an error.
19. As a developer, I want regression tests verifying that a cold startup with a reconfigured endpoint evicts obsolete tools and populates new tools upon discovery.
20. As a developer, I want regression tests verifying that concurrent or subsequent reloads maintain byte-stable snapshot canonicalization.

## Implementation Decisions

- **Storage Reconciliation Policy for Modified Endpoints**:
  The storage reconciliation logic will evaluate live configured servers whose endpoint fingerprint differs from their previous binding. Instead of returning an endpoint identity changed error, reconciliation will:
  - Update the binding's stored endpoint fingerprint to match the newly configured endpoint.
  - Mark the server's previous cached tool rows for eviction so they are omitted from the reconciled snapshot.
  - Omit embedding records whose tool identities belonged to the evicted tool definitions.
  - Reset the server's persisted catalog status to an uninitialized/unprobed status (`none`).

- **Startup Definition Restoration Safeguard**:
  The service startup admission logic must ensure that restoring cached tool definitions from a durable snapshot only applies to servers whose configured endpoint matches the durable snapshot's binding. If an endpoint change is detected for a server, the startup admission will restore zero tool definitions for that server, treating it as unprobed and allowing the background discovery loop to establish the initial catalog.

- **Reload Discovery Flow**:
  During a live configuration reload (e.g., via SIGHUP), the runtime will construct a dedicated client targeting the new endpoint and execute discovery. If discovery succeeds, the new definitions are registered in the candidate catalog, and reconciliation writes the new endpoint fingerprint and tool definitions to the durable snapshot.

- **Preservation of Departure Tombstones**:
  Servers that are completely removed from the configuration will continue to be retired and tombstoned according to existing policy. This spec specifically addresses reconfiguring the endpoint of an existing, actively configured server, not un-tombstoning deleted servers.

- **Zero Schema Breaking Changes**:
  The schema version and structural layout of the durable snapshot and prompt snapshot remain unchanged. The fields `server_id`, `endpoint_fingerprint`, `catalog_status`, and tool collections continue to follow the existing schema.

- **Dual-Credential Upstream Support (Bearer Gate + Header Pass-Through)**:
  Upstream MCP servers using FastMCP / HTTP per-user passthrough (such as `portainer-mcp`) require both a shared front-gate bearer token (`Authorization: Bearer <gate-token>`) to admit the connection and an API key header (e.g. `X-Portainer-API-Key: <key>`) to identify the user. The configuration schema supports supplying both `bearer:` and `headers:` concurrently under `credentials:`. Upstream request assembly propagates both without conflict, and runtime discovery/call dispatch sends the combined authentication set.
## Testing Decisions

- **Good Test Standard**:
  Tests must assert external observable behavior:
  - Starting the bridge with a modified server endpoint against an existing storage directory succeeds without error and reaches ready status.
  - Tools returned by discovery from the new endpoint become available for execution and search.
  - Tools that only existed on the old endpoint are no longer present in the catalog or search index.
  - Unaffected upstream servers retain their tool availability and search rankings across the restart.
  Tests must not assert internal intermediate map states or private pointer identities.

- **Primary Seams Tested**:
  1. **Bridge Service & HTTP Process Seam (`cmd/mcp-bridge` & `internal/service`)**:
     The highest integration seam. Runs the service with an initial server configuration, allows a snapshot to be persisted, stops the service, modifies the endpoint address/port in configuration, restarts the service, and verifies clean startup, readiness HTTP probes, and tool catalog transitions.
  2. **Storage Reconciliation Unit Seam (`internal/storage`)**:
     A focused algorithmic seam asserting that passing a modified endpoint fingerprint to the reconciliation function produces an updated snapshot with the new fingerprint, purged tool entries for the affected server, and no error.

- **Prior Art**:
  - `cmd/mcp-bridge/serve_test.go`: `TestBridgeRestartsWithDurableState` and multi-server reload workflows.
  - `internal/service/durable_test.go`: `TestStartupReconcilesAgainstDurableSnapshot`.
  - `internal/storage/reconcile_test.go`: `TestReconcileRejectsEndpointIdentityChange` (to be updated to assert successful reconciliation and eviction).

## Out of Scope

- Changing the policy for completely removed servers (tombstones on deleted server IDs remain intact).
- Automatic detection of port changes without configuration file updates.
- Changing secret or credential handling mechanisms.
- Altering the BM25 scoring parameters, vector dimensions, or embedding model algorithms.

## Further Notes

- Once this spec is implemented, homelab operators can freely update service ports, migrate containers, and update local DNS/IP configurations in `bridge.yaml` without needing to delete state files or manage database migrations.
