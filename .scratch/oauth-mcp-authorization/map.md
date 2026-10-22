# OAuth 2.1 MCP Authorization (RFC 9728) — Task Frontier

Canonical specification: [OAuth 2.1 MCP Authorization Specification](spec.md).

The graph below expresses ticket dependencies. The dependency frontier is 01, unblocked and ready for agent implementation.

| Ticket | Blocked by | Current state |
| --- | --- | --- |
| [01 — Add JWX dependency and internal auth package](issues/01-jwx-dependency-and-auth-package.md) | None | resolved |
| [02 — Protected resource metadata handler](issues/02-protected-resource-metadata-handler.md) | 01 | resolved |
| [03 — HTTP authentication middleware and claim validation](issues/03-http-auth-middleware-and-claim-validation.md) | 01 | resolved |
| [04 — Inbound transport context integration and session binding](issues/04-inbound-transport-context-integration.md) | 03 | resolved |
| [05 — Server composition wiring and HTTP wire test suite](issues/05-server-composition-wiring-and-wire-tests.md) | 02, 04 | resolved |

## Decisions so far

- **Grilling Round 1**: User approved dual-mode fallback, `github.com/lestrrat-go/jwx/v2` engine, outer `http.ServeMux` routing seam, environment variable configuration with defaults, and Bearer token SHA-256 session binding.
- **Grilling Round 2**: User approved fast failure on malformed/expired JWTs with 401 challenge, context-driven claims bypass in `InboundHandler`, non-blocking JWKS initialization, dynamic `authorization_servers` metadata, and RFC 6750 `error="insufficient_scope"` header on 403 Forbidden.
- **Grilling Round 3**: User approved 60-second clock skew tolerance, typed `Claims` context model, trailing slash audience normalization, multi-format scope parsing (space-delimited string & array), and in-process mock JWKS testing with zero external network calls.
- **Testing Seam**: The highest seam—loopback HTTP requests against `inboundServer.Handler` using an ephemeral mock JWKS server—is the primary testing seam for observable wire verification.

- Ticket 01 resolved: `github.com/lestrrat-go/jwx/v2` added to `go.mod`, `internal/auth/config.go` and `internal/auth/claims.go` implemented and verified with 100% test coverage.
- Ticket 02 resolved: `internal/auth/metadata.go` implemented and verified in `internal/auth/metadata_test.go` with 100% test coverage.
- Ticket 03 resolved: `internal/auth/jwks.go` and `internal/auth/middleware.go` implemented and verified with 99% test coverage.
- Ticket 04 resolved: `internal/service/inbound.go` updated to recognize `auth.ClaimsFromContext` while retaining session affinity.
- Ticket 05 resolved: Outer multiplexer wired in `cmd/mcp-bridge/serve.go`, end-to-end wire tests verified in `cmd/mcp-bridge/serve_oauth_test.go`, and full-suite coverage measured at 91.8% (gate >=90% passed).