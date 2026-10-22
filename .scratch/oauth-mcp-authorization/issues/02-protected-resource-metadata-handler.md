# 02: Protected resource metadata handler (RFC 9728)

**What to build:** In `internal/auth/metadata.go`, implement the HTTP handler for `GET /.well-known/oauth-protected-resource`. It returns HTTP 200 with JSON content type and the RFC 9728 schema:
```json
{
  "resource": "<MCP_RESOURCE_URL>",
  "authorization_servers": ["<POCKET_ID_ISSUER>"],
  "scopes_supported": ["openid", "email", "profile", "mcp:connect"],
  "bearer_methods_supported": ["header"]
}
```

**Blocked by:** 01.
Status: resolved

Type: task

Source: OAuth 2.1 MCP Authorization Specification (`.scratch/oauth-mcp-authorization/spec.md`).

- [x] In `internal/auth/metadata.go`, define `Metadata` struct with JSON tags for `resource`, `authorization_servers`, `scopes_supported`, and `bearer_methods_supported`.
- [x] In `internal/auth/metadata.go`, implement `NewProtectedResourceHandler(cfg Config) http.Handler` serving GET requests with HTTP 200 and JSON body.
- [x] Ensure non-GET methods return `405 Method Not Allowed`.
- [x] In `internal/auth/metadata_test.go`, add unit tests asserting HTTP 200, Content-Type `application/json`, correct JSON payload fields, and dynamic population of `authorization_servers` matching `cfg.PocketIDIssuer`.

## Answer

Implemented in `internal/auth/metadata.go` and verified in `internal/auth/metadata_test.go` with 100% statement coverage.
