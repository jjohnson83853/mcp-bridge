# 03: Route normalization and root POST discovery probe handling

**What to build:** Serve requests to both `/mcp` and `/mcp/` directly through the authenticated inbound handler without issuing HTTP 301/307 redirects, preventing HTTP clients from stripping the `Authorization: Bearer` header during Action Discovery. Route root discovery probes sent as `POST /` through the authenticated handler using Go ServeMux exact-path pattern `POST /{$}`, returning an authenticated HTTP 401 challenge rather than an unmatched 404 or redirect, while avoiding pattern collisions with `/mcp/`.

**Blocked by:** 01: RFC 9728 multi-path metadata discovery and path-aware resource advertisement, 02: Bidirectional audience tolerance and missing-credential safe diagnostics

**Status:** ready-for-agent

**Type:** task

## Acceptance criteria

- [ ] `POST /mcp` and `POST /mcp/` are handled directly by the authenticated inbound handler; neither path responds with an HTTP 301, 302, 307, or 308 redirect.
- [ ] Requests to `/mcp/` bearing valid credentials are admitted to the MCP protocol handler with intact headers.
- [ ] Exact root `POST /{$}` is routed through the authentication middleware, returning an HTTP 401 challenge with `WWW-Authenticate` metadata rather than an unauthenticated 404.
- [ ] Non-POST requests to unmatched paths (such as `GET /unknown` or `GET /`) continue to follow the fallback router behavior without breaking existing health endpoints or 404 expectations.
- [ ] Server initialization in `cmd/mcp-bridge` completes cleanly without Go `http.ServeMux` registration panics between `POST /{$}` and `/mcp/`.
- [ ] Wire tests in `cmd/mcp-bridge` verify that `/mcp`, `/mcp/`, and `POST /` respond with appropriate status codes and no redirect responses.
