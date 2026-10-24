# 02: Service startup admission and dynamic reload for reconfigured endpoints

**What to build:** Update the service lifecycle engine so that process startup admission does not restore obsolete tool definitions from disk when an actively configured server's endpoint has changed. The reconfigured server enters in an unprobed status with an empty catalog so subsequent asynchronous discovery can probe the new address and populate fresh tools. Additionally, update runtime configuration reloads so that changing an upstream server's endpoint triggers creation of a new dedicated client for the new address, executes discovery against the new endpoint, and commits the reconciled generation.

**Blocked by:** 01: Storage reconciliation endpoint rebind and cache eviction

Status: resolved

Type: task

Source: Endpoint reconfiguration reconciliation specification (`.scratch/endpoint-reconfiguration-reconciliation/spec.md`).

- [x] Startup admission succeeds without error when an active server's configured endpoint differs from its persisted durable snapshot binding.
- [x] Startup admission does not restore tool definitions from the durable snapshot for any server whose endpoint has changed.
- [x] A reconfigured server enters startup with an unprobed catalog status and empty tool definitions.
- [x] Post-startup background discovery successfully probes the new endpoint address and publishes its newly discovered tools.
- [x] Unaffected upstream servers continue to restore their warm catalogs and vectors from disk at startup without waiting for discovery.
- [x] Runtime configuration reload (SIGHUP) builds a new client targeting the updated endpoint, discovers tools from the new endpoint, and updates the catalog.
- [x] Runtime reload fails cleanly without affecting the running generation if the reconfigured endpoint is unreachable or invalid during the reload pass.
- [x] Service lifecycle tests verify startup admission and dynamic reload behavior across endpoint changes.

## Answer

Implemented in `internal/service/reload.go:465` and `:636-654`. `definitionsFromSnapshot(snapshot, serverID, endpoint)` looks up the server's binding and validates `!binding.Retired && binding.EndpointFingerprint == endpointFingerprint(endpoint)`. If the endpoint has changed, it returns `nil` so stale tools are not restored, entering startup with `CatalogStatus: StatusNone` and 0 catalog tools, while unaffected servers retain their warm catalog.

Verified by `TestStartupRebindsServerWhenEndpointChanged` in `internal/service/durable_test.go` and `TestReloadRebindsEndpointIdentityChange` in `internal/service/reload_test.go`. Full suite passes with 91.7% source-line coverage.
