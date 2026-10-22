# 05: Server composition wiring and HTTP wire test suite

**What to build:** In `cmd/mcp-bridge/serve.go` (and `main.go`), wire `auth.Config` and the outer multiplexer routing `GET /.well-known/oauth-protected-resource` and wrapping `/mcp` with `AuthMiddleware`. Add end-to-end HTTP wire tests asserting RFC 9728 discovery, 401 challenges, 403 scope rejection, and successful authenticated MCP sessions using an in-process mock JWKS server. Ensure full-suite test coverage remains $\ge 90\%$.

**Blocked by:** 02, 04.

Status: resolved

Type: task

Source: OAuth 2.1 MCP Authorization Specification (`.scratch/oauth-mcp-authorization/spec.md`).

- [x] In `cmd/mcp-bridge/serve.go`:
  - Initialize `auth.NewConfigFromEnv()`.
  - Create the JWKS cache with background refresh for `cfg.PocketIDJWKSURL`.
  - Construct the outer `http.ServeMux` for the inbound server:
    - Route `/.well-known/oauth-protected-resource` to `auth.NewProtectedResourceHandler(authCfg)`.
    - Route `/mcp` through `authMiddleware(inbound)`.
  - Assign the outer mux to `inboundServer.Handler`.
- [x] In `cmd/mcp-bridge/serve_oauth_test.go`:
  - Spin up an ephemeral `httptest.Server` serving a test JWKS endpoint with an RSA keypair.
  - Test `GET /.well-known/oauth-protected-resource` returns 200 OK with correct RFC 9728 metadata.
  - Test unauthenticated `POST /mcp` returns 401 Unauthorized with exact `WWW-Authenticate: Bearer realm="mcp", resource_metadata="<MCP_RESOURCE_URL>/.well-known/oauth-protected-resource"`.
  - Test invalid and expired JWT returns 401 Unauthorized with the challenge header.
  - Test valid token missing required scope returns 403 Forbidden with `WWW-Authenticate: Bearer error="insufficient_scope", scope="<scope>"`.
  - Test valid JWT signed with test key returns 200 OK on MCP initialize/tools list.
  - Test static bearer token fallback returns 200 OK when configured.
- [x] Run `go test -coverpkg=./... -covermode=atomic -coverprofile=/tmp/coverage.out ./...` and `go run ./cmd/covcheck -profile=/tmp/coverage.out` to verify coverage meets or exceeds the mandatory 90% threshold.

## Answer

Implemented in `cmd/mcp-bridge/serve.go` (lines 82–85 and 149–164). Verified by end-to-end wire tests in `cmd/mcp-bridge/serve_oauth_test.go`. Full test suite passes and project source-line coverage stands at 91.8% (gate $\ge 90\%$ passed).
