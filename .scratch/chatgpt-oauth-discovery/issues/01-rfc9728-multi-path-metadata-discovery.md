# 01: RFC 9728 multi-path metadata discovery and path-aware resource advertisement

**What to build:** Expose OAuth 2.0 Protected Resource Metadata (RFC 9728) on all required discovery paths (`/.well-known/oauth-protected-resource`, `/.well-known/oauth-protected-resource/mcp`, and `/mcp/.well-known/oauth-protected-resource`). When queried on path-bearing endpoints (`/mcp`), the returned metadata `resource` property dynamically advertises the connector endpoint (`https://tool.bullprog.com/mcp`), ensuring ChatGPT binds and transmits the OAuth token to the correct resource identifier. Root queries continue advertising the base resource URL. The metadata endpoints bypass bearer authentication and are classified as registered (`metadata_target_registered: "yes"`) in challenge diagnostic records.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

**Type:** task

## Acceptance criteria

- [ ] Requests to `/.well-known/oauth-protected-resource`, `/.well-known/oauth-protected-resource/mcp`, and `/mcp/.well-known/oauth-protected-resource` return HTTP 200 with `Content-Type: application/json` without requiring an `Authorization` header.
- [ ] Responses to `/.well-known/oauth-protected-resource/mcp` and `/mcp/.well-known/oauth-protected-resource` advertise a `resource` field ending with `/mcp`, matching the endpoint URL expected by clients configuring an endpoint connector.
- [ ] Responses to root `/.well-known/oauth-protected-resource` advertise the configured `MCPResourceURL`.
- [ ] All three metadata paths are included in the authentication middleware's bypass list (`isBypassed`), ensuring they never trigger a 401 challenge or credential check.
- [ ] Challenge classification logic records `metadata_target_registered: "yes"` and `challenge_target: "path_appended_well_known"` when path-bearing resource URLs are configured.
- [ ] Unit tests in `internal/auth` assert payload generation across root and path-bearing URL variations.
- [ ] Wire tests in `cmd/mcp-bridge` assert real HTTP GET retrieval across all three paths on the composed listener.
