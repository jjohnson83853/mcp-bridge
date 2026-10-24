# 03: End-to-end CLI process and state persistence acceptance tests

**What to build:** Implement end-to-end integration and wire tests asserting that when the bridge is started against an existing state directory after modifying a server's port or URL in the configuration file, the process starts cleanly without crashing, reports ready on health endpoints, populates newly discovered tools from the updated port, drops obsolete tools from the old port, and writes the updated snapshot to disk. Also verify dynamic SIGHUP configuration reload transitions the upstream connection without dropping the bridge process.

**Blocked by:** 02: Service startup admission and dynamic reload for reconfigured endpoints

Status: resolved

Type: task

Source: Endpoint reconfiguration reconciliation specification (`.scratch/endpoint-reconfiguration-reconciliation/spec.md`).

- [x] End-to-end integration test boots the bridge against an existing state snapshot after changing a server's port (e.g. 8000 to 8080) in configuration.
- [x] Process exits startup cleanly and achieves HTTP 200 on the `/readyz` probe.
- [x] Tools from the new port are discoverable and callable via the JSON-RPC `/mcp` endpoint.
- [x] Tools unique to the old port are no longer exposed in the tool list or searchable via hybrid search.
- [x] Other configured upstream servers in the multi-server setup retain their cached tool lists and search functionality.
- [x] The updated endpoint fingerprint is atomically persisted to `snapshot.json` on disk.
- [x] A dynamic SIGHUP reload with an endpoint modification updates upstream routing and catalog without restarting the bridge daemon.
- [x] Full repository test suite passes with source line coverage maintaining at least 90%.

## Answer

Implemented in `cmd/mcp-bridge/serve_endpoint_rebind_test.go` with `TestBridgeRebindsServerEndpointOnRestartAndReload`. The test validates the complete lifecycle:
1. Bridge boots with upstream servers `alpha` (port 1) and `beta`, serves tools, and writes durable `snapshot.json`.
2. Bridge is cleanly stopped, and configuration is rewritten pointing `alpha` to a new port (`alpha2`).
3. Bridge restarts against the existing storage directory without error, reaches HTTP 200 on `/readyz`, serves `read_v2`, evicts `read_v1`, and updates `snapshot.json`.
4. SIGHUP reload rebinds `alpha` to `alpha3` dynamically without dropping the bridge process.

Verified by `go test -v ./cmd/mcp-bridge -run TestBridgeRebindsServerEndpointOnRestartAndReload` (10 consecutive passes) and full suite coverage gate: 91.7% (3936/4290 source lines), passing the >=90% threshold.
