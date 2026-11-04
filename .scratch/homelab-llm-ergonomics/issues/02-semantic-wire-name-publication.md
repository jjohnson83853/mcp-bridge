# 02 — Semantic Wire Name Publication

Status: ready-for-agent
Type: task
Blocked by: 01

**What to build:**
Publish tools using their canonical semantic wire name `<server>__<tool>` (`config.WireToolName(server, rawName)`) across search results and dynamic session projections.
If `<server>__<rawName>` exceeds 64 characters or contains characters outside `^[a-z][a-z0-9-]{0,31}__[a-zA-Z0-9_-]+$`, fall back deterministically to `core.PublicName` (`t_<base32>`). Preserve dual lookup in `resolveBinding` so both wire names and legacy/fallback hash names resolve cleanly. Update affected service and rebind regressions.

**Acceptance Criteria:**
- [ ] In `searchPayload` (`internal/service/tools.go`), tools whose wire name `server__raw_name` has length $\le 64$ characters and valid identifier characters are reported under `tool_name: "<server>__<raw_name>"`.
- [ ] Tools whose wire name exceeds 64 characters or contains invalid characters fall back deterministically to `t_<base32>`.
- [ ] In `preparePinProjection` (`internal/service/projection.go`), dynamically discovered tools committed to session projections use wire names when within bounds, or `t_<base32>` when exceeding bounds.
- [ ] `resolveBinding` continues to resolve both wire names and `t_<base32>` hashes to the catalog binding.
- [ ] Service progressive tests and `serve_endpoint_rebind_test.go` pass.

**Verification Commands:**
```sh
go test -v -count=1 ./internal/service/...
go test -v -count=1 ./cmd/mcp-bridge -run 'TestBridgeRebindsServerEndpointOnRestartAndReload'
```

**Envelope:**
- `internal/service/tools.go`
- `internal/service/projection.go`
- `internal/service/tools_test.go`
- `internal/service/progressive_discovery_test.go`
- `internal/service/progressive_session_test.go`
- `internal/service/progressive_projection_test.go`
- `internal/service/progressive_concurrency_lifecycle_test.go`
- `cmd/mcp-bridge/serve_endpoint_rebind_test.go`
- `.omp/artifacts/receipt_homelab_T2.json`
