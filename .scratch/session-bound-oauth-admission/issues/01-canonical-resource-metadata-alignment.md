# 01 — Canonical Endpoint Resource Metadata Alignment

**What to build:** End-to-end RFC 9728 discovery where any client querying `/.well-known/oauth-protected-resource` or path-bearing endpoints receives a metadata payload advertising the canonical `/mcp` endpoint resource URL. The deployment configuration sets `MCP_RESOURCE_URL=https://tool.bullprog.com/mcp`. Strict RFC 8707 clients observe a matching resource URI, eliminating path mismatch warnings during OAuth initiation.

**Blocked by:** None — can start immediately

**Status:** ready-for-agent

- [ ] Protected resource metadata responses served at root `/.well-known/oauth-protected-resource` and path-bearing discovery routes advertise a canonical `resource` URL ending in `/mcp`.
- [ ] Root discovery probes return metadata consistent with the `/mcp` endpoint path without triggering 401 challenge errors.
- [ ] Production deployment documentation and default configuration pin `MCP_RESOURCE_URL` to the path-bearing canonical endpoint URL.
- [ ] Bidirectional audience tolerance accepts tokens targeted at both the base origin and the canonical endpoint path.
- [ ] Unit and wire tests in `internal/auth/metadata_test.go` and `cmd/mcp-bridge/serve_oauth_test.go` assert canonical resource advertisement across all supported discovery paths.
