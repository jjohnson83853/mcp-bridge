# 01: Storage reconciliation endpoint rebind and cache eviction

**What to build:** Allow the storage reconciliation engine to accept an updated endpoint fingerprint for an actively configured server rather than rejecting the change with a fatal endpoint identity error. The reconciled snapshot updates the server's binding to the new endpoint fingerprint, evicts obsolete tool definitions and embedding records associated with that server, and resets the server's health status to uninitialized, while preserving all unaffected servers, bindings, tools, and embeddings byte-identically.

**Blocked by:** None (can start immediately)

Status: resolved

Type: task

Source: Endpoint reconfiguration reconciliation specification (`.scratch/endpoint-reconfiguration-reconciliation/spec.md`).

- [x] When a live configured server has an endpoint fingerprint different from its previous snapshot binding, reconciliation succeeds without returning an endpoint changed error.
- [x] The updated snapshot reflects the new endpoint fingerprint in the server's binding.
- [x] Pre-existing tool definitions for the reconfigured server are excluded from the reconciled snapshot tools collection.
- [x] Pre-existing embedding records whose tool identities belonged to the reconfigured server are excluded from the reconciled snapshot embeddings collection.
- [x] The server's health row in the reconciled snapshot is set to catalog status `none` without stale failure reasons.
- [x] All other configured servers whose endpoints were not modified retain their exact tool definitions, embedding vectors, and bindings.
- [x] Departed and retired servers continue to be tombstoned per existing retirement rules.
- [x] Reconciled collections maintain byte-stable canonical ordering across inputs.
- [x] Storage unit tests verify that endpoint fingerprint changes produce clean rebinds and evictions.

## Answer

Implemented in `internal/storage/reconcile.go:81-88`. When `stillConfigured && binding.EndpointFingerprint != server.EndpointFingerprint`, reconciliation now sets `purged[binding.ServerID] = true` and appends a live binding with `server.EndpointFingerprint`. The purge flag resets the server's health row to `CatalogStatus: StatusNone` and evicts its tool definitions and embedding records, leaving unaffected servers intact.

Verified by `TestReconcileRebindsEndpointIdentityChange` in `internal/storage/reconcile_test.go` and `TestReloadRebindsEndpointIdentityChange` in `internal/service/reload_test.go`. Full test suite PASS with source-line coverage 91.7% (3928/4282), passing the >=90% gate.
