# Metadata prerequisite evidence

Recorded 2026-10-26. Scope: mcp-bridge only; read-only investigation of U2/U3/U5 for METADATA-r4. No application/test/config/gate changes.

## Access and method

The user's latest request authorized another source-discovery attempt. It succeeded: the mounted bridge reported codebase-searcher available/fresh, last success 2026-10-26T12:53:29.962893882Z, catalog revision 6021d87ff9798be6f78e659b565ca34d429717aae2b4b223212ca8d66bc26e0c. Subsequent bounded source calls succeeded. The previous HTTP401 blocker is cleared for this investigation; no credentials or global configuration were accessed.

Used bounded MCP read_range/search_codebase scoped to project mcp-bridge, and gopls definition lookups. Dependency source was read only at the installed SDK definition path resolved by gopls, outside the source-searcher's project mount. Graphify skipped: no verified graph path and local planning changes make clean-worktree gating unavailable.

## U2 — storage compatibility: additive old-reader compatibility disproved

- The existing flat tool snapshot uses schema version 1, with explicit Tool fields and a canonical JSON checksum: internal/storage/snapshot.go:19-21,39-78,90-98,153-164.
- Load uses DisallowUnknownFields, rejects trailing data, verifies the checksum computed from the decoded typed snapshot, then validates schema/identities: internal/storage/store.go:143-173; internal/storage/snapshot.go:251-330.
- Therefore an old binary will reject a new tool-row metadata field; silently appending fields cannot provide old-reader compatibility. Even ignoring an added field would not preserve the current typed checksum computation. This is a source-proven incompatibility, not a tested migration implementation.
- Save validates and refuses replacement of untrusted state, computes canonical checksum, then publishes via private temporary file, fsync, rename and directory sync: internal/storage/store.go:176-232. Preserve integrity and atomic admission rather than relaxing the reader.
- Existing schema validation accepts only its exact version. A new-reader/legacy-reader compatibility strategy and rollback contract require a planner revision and critic review before design approval. No migration has been selected or implemented by this investigation. Do not delete or reinterpret an invalid snapshot to force compatibility.

## U3 — lifecycle, configuration and wire evidence

- Inbound MCP handler is one shared server with nil StreamableHTTPOptions: internal/service/server.go:8-12. gopls resolved the SDK constructor to go-sdk v1.8.0/mcp/streamable.go:231-246; nil options preserve zero-valued stateful mode, SSE response mode, and no SDK idle-session expiry. SDK option documentation: streamable.go:140-172. Do not change these settings or MCPGODEBUG.
- The separate inbound admission layer receives the configured SessionIdle and MaxSessions; it removes records whose idle time exceeds the configured duration: cmd/mcp-bridge/serve.go:131-147; internal/service/inbound.go:421-437. SDK session timeout and the admission-layer lifetime are distinct.
- Config.Command requires the values from configuration; it does not install session/refresh defaults: internal/config/command.go:77-105. Example configuration supplies session_idle_timeout 10m and refresh_interval 5m (configs/config.example.yaml:10,19); these are example values, not implicit code defaults. Bounds are not runtime defaults.
- Dedicated upstream sessions are lazily connected and reused for their runtime lifetime, not reinitialized per request: internal/upstream/dedicated.go:54-70. Reload/refresh reuses matching clients; refresh cannot create/rotate clients: internal/service/reload.go:434-451.
- Cold admission restores durable definitions without contacting upstream; pins are deferred. Loop.Start publishes the cold generation then triggers live refresh immediately: internal/service/reload.go:460-490,510-527; internal/service/loop.go:116-158.
- A reload discovery error rejects the whole candidate; refresh retains the failed server's complete previous definitions and reason while successful peers advance. Persistence runs before acceptance and failure rejects the candidate: internal/service/reload.go:472-495,529-573. Refresh publication callback fires only on successful assembly: internal/service/refresh.go:196-223.
- SIGHUP is wired to reload signals; accepted publication calls the tool surface without replacing the inbound handler: cmd/mcp-bridge/serve.go:166-197; internal/service/loop.go:94-113,179-193. Preserve existing SDK signaling, explicit re-list visibility, and existing subscription caveat; no universal notification-delivery guarantee was established.
- Executed real bridge/controlled-upstream wire harness: `go test ./cmd/mcp-bridge -run '^TestServeHybrid(PreloadedToolsListExposure|Coexistence|SIGHUPReload)$' -count=1` — PASS, exit 0, package runtime 2.128s. This exercises the existing HTTP MCP serving composition and reload path, not the unimplemented metadata feature. No new tests or mutable real-upstream calls were introduced.

## U5 — existing reporting boundaries

- JSON logger is created with stdout: cmd/mcp-bridge/serve.go:38. Entry schema, finite event/operation/outcome/reason vocabulary and identity fields are enforced in internal/observe/log.go:13-75,95-158,225-239.
- Log delivery is synchronous encoder output under a mutex. Failed encoding recreates the encoder for a future record; no replay or durable retention is provided by this logger: internal/observe/log.go:205-223. External stdout collection/retention is not established and must not be promised.
- Recursive key redaction covers token/key/secret/password/auth/session/arguments/results/query/schema/description/payload/url/endpoint/header/stack markers: internal/observe/log.go:275-299. Metadata contents must not be added to logs.
- Existing operator reporting emits warn/reload/failure/invalid_config, projecting typed config diagnostics or a finite generic bridge record; it intentionally does not output raw error text: cmd/mcp-bridge/wiring.go:408-462. Preserve this policy instead of claiming distinct persistence/discovery log severities or inventing telemetry.
- Refresh health remains visible through source snapshot status/reason, readiness and metrics; publishObserver updates readiness/catalog metrics, not a separate per-refresh JSON event: cmd/mcp-bridge/wiring.go:369-405; internal/service/refresh.go:204-223. Rejected pass metric reporting uses the current assembly; do not claim newly stored stale state on a rejected generation.
- Persistence failure marks local storage readiness false and propagates an error: cmd/mcp-bridge/serve.go:115-129. Loop reload failures report metrics and the existing operator log path: internal/service/loop.go:94-113,179-216.

## Consequences

U3/U5 have bounded source/runtime evidence sufficient for a planner to state the existing constraints without new defaults, delivery guarantees or telemetry. U2 is resolved as an incompatibility requiring a planner-selected version/legacy-read/rollback contract. Requirements are not waived; design remains unapproved. Metadata tests, full suite/race/vet/coverage, new-format compatibility probes and Gemini smoke have NOT RUN.

## Later engineer-handoff collection blocker

Recorded 2026-10-26T14:13:41.918567Z. The successful prerequisite retrieval above remains historical evidence; it is not a claim that connector access stayed available. A later coordinator `call_tool` request for public-surface source returned non-retryable HTTP 401 Unauthorized, trace `b7f7c64ff2f58c98d91af8c9599161f4`. No further connector attempts were made after inspecting that failure.

Read-only scout `MetadataConfigAnchors` reported three failed bridge attempts followed by local source reads/grep/LSP as a fallback (52.3 seconds; reported counts: three failed bridge attempts, four source windows, one LSP-symbols request, two greps). Access denial does not authorize that fallback. Its source-map findings are rejected as inadmissible, were not supplied to the planner, and cannot satisfy the exact engineer contract. The scout reported no writes/tests; no application changes were dispatched. Preserve this safety failure rather than treating its COMPLETE label as acceptance.

METADATA-r4 requirements and METADATA-r5 design have each received independent PASS and separate explicit user approval. The remaining task-checkpoint preparation needs exact source declarations and conversion/parser/publication/test-harness anchors. It may use the earlier authorized evidence but must not invent the missing contract, bypass the renewed denial, or infer task/implementation approval. Source restoration needs a new user signal before retry; no credential search or global configuration change is authorized.
