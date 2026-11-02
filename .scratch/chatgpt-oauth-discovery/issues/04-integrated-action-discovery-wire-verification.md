# 04: Integrated action discovery wire verification and diagnostic provenance

**What to build:** Validate the complete end-to-end post-OAuth Action Discovery flow against the composed bridge harness: unauthenticated probe receiving the registered RFC 9728 challenge, client retrieving metadata with matching connector resource URI, submission of an authorized discovery request to `/mcp` or `/mcp/` with origin- or endpoint-scoped tokens without redirection, and successful MCP session initialization. Confirm diagnostic provenance records report `metadata_target_registered: "yes"` and that all affected test suites compile and pass clean.

**Blocked by:** 01: RFC 9728 multi-path metadata discovery and path-aware resource advertisement, 02: Bidirectional audience tolerance and missing-credential safe diagnostics, 03: Route normalization and root POST discovery probe handling

**Status:** ready-for-agent

**Type:** task

## Acceptance criteria

- [ ] An end-to-end wire test simulates the complete post-OAuth discovery sequence:
  1. Discovery probe to `/mcp` receives HTTP 401 with `WWW-Authenticate` challenge containing `resource_metadata`.
  2. Client follows `resource_metadata` to retrieve metadata with `resource` matching the endpoint URI.
  3. Client sends `POST /mcp` (and `POST /mcp/`) with an access token matching origin or endpoint audience; request is admitted with HTTP 200.
- [ ] Diagnostic completion records for path-bearing challenge responses confirm `challenge_target: "path_appended_well_known"` and `metadata_target_registered: "yes"`.
- [ ] Route outcomes for metadata paths report `route_outcome: "metadata"` and `failure_stage: "none"`.
- [ ] All unit and integration tests across `internal/auth/...` and `cmd/mcp-bridge/...` pass (`go test ./internal/auth/... ./cmd/mcp-bridge/...`).
- [ ] Project build succeeds cleanly with `go build ./...`.
