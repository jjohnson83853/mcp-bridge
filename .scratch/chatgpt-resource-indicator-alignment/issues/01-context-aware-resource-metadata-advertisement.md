# 01 — Context-Aware Resource Metadata Advertisement

Status: resolved
Type: task
Blocked by: None
Canonical State: Governed by `.omp/plans/execution-state.json`

## Description

Update the OAuth 2.0 Protected Resource metadata handler so that it serves the origin resource identifier (`https://<origin>`) on root discovery queries (`/.well-known/oauth-protected-resource`) when configured with an origin resource URL, and serves the endpoint-specific resource identifier (`https://<origin>/mcp`) on path-bearing discovery routes (`/mcp/.well-known/oauth-protected-resource` and canonical RFC 9728 route `/.well-known/oauth-protected-resource/mcp`). When configured explicitly with a path-bearing resource URL, all discovery paths advertise that canonical URL. Trailing slashes in `cfg.MCPResourceURL` MUST be trimmed consistently (`cleanResource := strings.TrimRight(cfg.MCPResourceURL, "/")`) before configuration-mode classification and suffix checking, preventing double slashes (such as `//mcp`). Update existing assertion in `cmd/mcp-bridge/serve_oauth_test.go` to keep integration tests green.

## Acceptance Criteria
- [x] **AC-1**: When `cfg.MCPResourceURL` is an origin URL (with or without trailing slash, e.g. `https://api.example.org` or `https://api.example.org/`), `GET /.well-known/oauth-protected-resource` returns HTTP 200 with JSON payload containing `"resource": "https://api.example.org"`, `"authorization_servers": [cfg.PocketIDIssuer]`, and supported scopes/bearer methods.
- [x] **AC-2**: When `cfg.MCPResourceURL` is an origin URL (with or without trailing slash), both canonical endpoint discovery `GET /.well-known/oauth-protected-resource/mcp` and compatibility alias `GET /mcp/.well-known/oauth-protected-resource` return HTTP 200 with JSON payload containing `"resource": "https://api.example.org/mcp"`.
- [x] **AC-3**: When `cfg.MCPResourceURL` is a path-bearing URL (ending in `/mcp` or `/mcp/`), all three discovery routes (`/.well-known/oauth-protected-resource`, `/.well-known/oauth-protected-resource/mcp`, and `/mcp/.well-known/oauth-protected-resource`) return HTTP 200 with `"resource": "https://api.example.org/mcp"`.
- [x] **AC-4**: In `cmd/mcp-bridge/serve_oauth_test.go`, `TestServeOAuthWire` line 410 asserts that root discovery under origin `oauthResource` returns `metadata.Resource == oauthResource`.

## Deterministic TDD Plan
- **RED:** In `internal/auth/metadata_test.go`:
  1. Update root discovery expectation under origin configuration to expect origin URL `https://api.example.org` (leaving production code unchanged).
  2. Add test cases for origin URL with trailing slash (`https://api.example.org/`) and path-bearing URL with trailing slash (`https://api.example.org/mcp/`) to the test matrix.
  3. Run `go test -v -count=1 ./internal/auth -run TestProtectedResourceHandler`.
  4. Observe failure (RED): `Resource = "https://api.example.org/mcp"; want "https://api.example.org"`.
- **GREEN:** Implement context-aware route selection in `internal/auth/metadata.go` using `cleanResource := strings.TrimRight(cfg.MCPResourceURL, "/")`. Update line 410 of `cmd/mcp-bridge/serve_oauth_test.go` to assert `metadata.Resource == oauthResource`. Re-run:
  - `go test -v -count=1 ./internal/auth -run TestProtectedResourceHandler` -> PASS (GREEN).
  - `go test -v -count=1 ./cmd/mcp-bridge -run TestServeOAuthWire` -> PASS (GREEN).

## Answer

Implemented context-aware resource metadata advertisement in `internal/auth/metadata.go`. Trailing slashes are trimmed, origin configuration serves the root origin resource on `/.well-known/oauth-protected-resource` and the endpoint resource on path-bearing discovery routes (`/mcp/.well-known/oauth-protected-resource` and `/.well-known/oauth-protected-resource/mcp`), while endpoint configuration serves the `/mcp` endpoint resource uniformly across all discovery paths.

Verified via:
- `go test -v -count=1 ./internal/auth -run TestProtectedResourceHandler` -> PASS
- `go test -v -count=1 ./cmd/mcp-bridge -run TestServeOAuthWire` -> PASS
