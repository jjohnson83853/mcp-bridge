# 04 — Integrated Wire Verification and Coverage Gate

Status: ready-for-agent
Type: task
Blocked by: 01, 02, 03

**What to build:**
Add and update loopback HTTP wire verification tests in `cmd/mcp-bridge` exercising the changes across the live transport and verify full repository test suite and coverage gate:
1. `describe_tool` and `call_tool` over live wire succeed with omitted and empty `catalog_revision`.
2. `describe_tool` and `call_tool` with mismatched valid 64-hex revision return `catalog_changed` and emit expected `mcp_complete` diagnostics.
3. `describe_tool` and `call_tool` with malformed revision (e.g. invalid non-hex string) return `invalid_arguments` and emit expected `mcp_complete` diagnostics.
4. `search_tools` over live wire returns semantic wire names, includes `"availability"`, and omits `"backends"`.
5. Newly discovered dynamic tools appear under wire names in `tools/list` following search.
6. Diagnostic log correlation confirms `http_complete` and `mcp_complete` share `request_id`, and `http_complete` reports `upstream_state: "not_attempted"` for `describe_tool`.
7. Update `README.md` to document optional `catalog_revision`, semantic wire naming, and streamlined payloads.
8. Generate fresh `coverage.out` and confirm source-line coverage >= 90.0%.

**Acceptance Criteria:**
- [ ] Wire tests in `cmd/mcp-bridge/serve_test.go` or `serve_oauth_test.go` verify omitted and empty `catalog_revision` for describe and call over HTTP.
- [ ] Wire tests verify rejection on mismatched revision with `catalog_changed`.
- [ ] Wire tests verify rejection on malformed revision with `invalid_arguments`.
- [ ] Wire tests verify pruned search payload with wire names and availability.
- [ ] Wire tests verify dynamic reveal in `tools/list` under `<server>__<tool>`.
- [ ] Wire execution logs emit `http_complete` and `mcp_complete` with matching `request_id` and correct admission reason (`catalog_changed` on stale revision, `invalid_arguments` on malformed revision, `none` on success), and `http_complete` reports `upstream_state: "not_attempted"` for `describe_tool`.
- [ ] `README.md` updated to accurately document optional revision, wire names, and pruned payloads.
- [ ] `go test -race ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `go run ./cmd/covcheck` verifies source-line coverage >= 90.0% against fresh `coverage.out`.

**Verification Commands:**
```sh
go test -race ./...
go vet ./...
go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...
go run ./cmd/covcheck
```

**Envelope:**
- `cmd/mcp-bridge/serve_test.go`
- `cmd/mcp-bridge/serve_oauth_test.go`
- `README.md`
- `coverage.out`
- `.omp/artifacts/receipt_homelab_T4.json`
