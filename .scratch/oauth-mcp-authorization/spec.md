Status: ready-for-agent

## Problem Statement

When Model Context Protocol (MCP) clients connect to the MCP bridge over HTTP or SSE, the bridge currently requires either an unauthenticated connection or static pre-shared bearer tokens configured via the operator's YAML document. This approach does not scale across enterprise environments, violates the MCP Authorization specification (which mandates OAuth 2.1 authentication), and prevents integration with centralized identity providers such as Pocket ID.

Furthermore, clients adhering to RFC 9728 (OAuth 2.0 Protected Resource Metadata) cannot automatically discover authorization servers, supported scopes, or metadata for the MCP resource, resulting in failed connections, opaque authorization errors, or missing challenge headers when connecting through automated agent runtimes (such as Claude Desktop, LibreChat, or custom agents). Without standard OAuth 2.1 Bearer authentication and Protected Resource Metadata, the bridge cannot verify user identity, enforce granular permission scopes, or securely authenticate inbound MCP calls against Pocket ID.

## Solution

Enforce OAuth 2.1 Bearer token authentication via Pocket ID in compliance with the MCP Authorization specification and RFC 9728.

1. **Protected Resource Metadata (RFC 9728)**: Expose an unauthenticated discovery endpoint at `GET /.well-known/oauth-protected-resource` returning the resource identifier, authorization server URLs, supported scopes, and supported bearer token methods.
2. **HTTP Authentication Middleware**: Wrap MCP transport handlers with middleware that enforces Bearer token validation on all protected routes while bypassing metadata and health check endpoints.
3. **Standards-Compliant Challenge Headers**: On missing, invalid, or expired tokens, respond with `HTTP 401 Unauthorized` and the RFC 9728 `WWW-Authenticate` header pointing clients directly to the resource metadata URL.
4. **Cryptographic Validation & Background Keyset Refresh**: Validate incoming JWT signatures against Pocket ID's JSON Web Key Set (JWKS), using asynchronous keyset caching and background refresh to avoid per-request outbound network calls. Validate standard claims (`iss`, `aud`, `exp` with clock skew tolerance).
5. **Granular Scope Enforcement**: When a required scope is configured, verify that the token's `scope` claim contains the required scope; reject insufficient scopes with `HTTP 403 Forbidden` and the RFC 6750 `insufficient_scope` challenge header.
6. **Context Propagation & Dual-Mode Compatibility**: Inject validated claims and caller identity into the request context for downstream auditing and session handling, while providing dual-mode fallback for configured static bearer tokens to ensure zero disruption to existing integration suites.

## User Stories

1. As an MCP client connecting without credentials, I want to receive an `HTTP 401 Unauthorized` response with a standard `WWW-Authenticate: Bearer realm="mcp", resource_metadata="<MCP_RESOURCE_URL>/.well-known/oauth-protected-resource"` header, so that I can automatically discover where to obtain an OAuth token.
2. As an MCP client, I want to query `GET /.well-known/oauth-protected-resource` without authentication, so that I can discover the resource URL, authorization servers, and supported scopes defined by RFC 9728.
3. As an MCP client with a valid Pocket ID JWT, I want to access `/mcp` successfully, so that I can initialize MCP sessions and execute tools seamlessly.
4. As an MCP client, I want my token's `iss` claim verified against the configured Pocket ID issuer, so that tokens issued by untrusted or rogue identity providers are rejected.
5. As an MCP client, I want my token's `aud` claim verified against the MCP resource URL, so that access tokens intended for other applications or audiences cannot be replayed against this bridge.
6. As an MCP client, I want audience validation to accommodate both single-string and array-of-strings `aud` claims, so that minor variations in IdP token serialization do not cause false rejections.
7. As an MCP client, I want audience validation to normalize trailing URL slashes, so that `https://mcp.bullprog.com` matches `https://mcp.bullprog.com/` without authentication failure.
8. As an MCP client presenting an expired JWT, I want to receive `HTTP 401 Unauthorized` with the standard RFC 9728 challenge header, so that my agent runtime knows it must refresh its access token.
9. As an MCP client experiencing slight clock drift, I want a 60-second clock skew tolerance on token expiration checks, so that network or container NTP variances do not cause premature token rejections.
10. As an MCP client with a malformed or tamper-corrupted JWT, I want an immediate `HTTP 401 Unauthorized` response with clear challenge headers, so that debugging token corruption is straightforward.
11. As an operator configuring a required scope, I want tokens missing the required scope to receive an `HTTP 403 Forbidden` response, so that unauthorized callers cannot access MCP tools.
12. As an MCP client receiving an `HTTP 403 Forbidden` due to missing scope, I want the response to include `WWW-Authenticate: Bearer error="insufficient_scope", scope="<MCP_REQUIRED_SCOPE>"`, so that I know exactly which scope must be requested from Pocket ID.
13. As an MCP client, I want scope validation to parse both space-delimited string claims and JSON string arrays, so that the bridge interoperates smoothly with any standard JWT scope representation.
14. As an MCP client using an established session, I want my session affinity (`Mcp-Session-Id`) to bind reliably to my token identity, so that other clients or callers cannot hijack or cross-contaminate my session state.
15. As a monitoring system or load balancer, I want health check endpoints (`/livez`, `/readyz`, `/healthz`) to be exempted from authentication, so that probe requests succeed without credentials.
16. As an operator running legacy automation with static pre-shared bearer tokens, I want the bridge to accept configured static tokens when non-JWT bearer tokens are supplied, so that existing scripts and test environments continue functioning without immediate token migration.
17. As an operator, I want the bridge to refresh JWKS public keys in the background, so that key rotations in Pocket ID take effect automatically without bridge restarts.
18. As an operator, I want the bridge not to perform an outbound HTTP call to Pocket ID on every request, so that MCP tool calls maintain sub-millisecond local routing latency and do not fail if the IdP experiences transient network latency.
19. As an operator, I want the bridge to start up and bind its HTTP listeners immediately even if Pocket ID is momentarily unreachable or when running offline tests, so that dependency outages do not crash the service.
20. As an operator, I want to configure the Pocket ID issuer, JWKS URL, resource URL, and required scope via standard environment variables with safe defaults, so that deployment in containerized environments requires zero code changes.
21. As a downstream tool handler or audit logger, I want the authenticated subject and email claims injected into the request context, so that tool executions can record caller identity for provenance and security tracking.
22. As a test engineer, I want all tests to execute against in-process mock JWKS endpoints without external network calls, so that CI test runs remain deterministic, fast, and completely offline.

## Implementation Decisions

- **Single Maintained Cryptographic Engine**:
  Adopt `github.com/lestrrat-go/jwx/v2` for JWT decoding, signature verification, and JWKS management. This provides production-grade key caching, multi-algorithm support (RSA, ECDSA, EdDSA), and native claim validation in a single dependency without custom cryptographic glue code.

- **Outer Multiplexer Routing Seam**:
  Mount an outer HTTP router on the inbound listener:
  - `GET /.well-known/oauth-protected-resource`: Dispatches to the unauthenticated metadata handler.
  - `/mcp`: Dispatches to the MCP transport handler wrapped by `AuthMiddleware`.
  - Health checks (`/livez`, `/readyz`, `/healthz`): Bypass authentication if routed to this listener.
  This preserves the core MCP transport handler's focus on protocol messages while handling metadata and auth wrapping cleanly at the edge.

- **Protected Resource Metadata Schema (RFC 9728)**:
  Serve `application/json` at `GET /.well-known/oauth-protected-resource` with HTTP 200:
  ```json
  {
    "resource": "<MCP_RESOURCE_URL>",
    "authorization_servers": ["<POCKET_ID_ISSUER>"],
    "scopes_supported": ["openid", "email", "profile", "mcp:connect"],
    "bearer_methods_supported": ["header"]
  }
  ```
  `authorization_servers` dynamically reflects the configured issuer.

- **Standard Challenge Responses (RFC 9728 & RFC 6750)**:
  - Missing, invalid, expired, or malformed credentials:
    `HTTP 401 Unauthorized` with header:
    `WWW-Authenticate: Bearer realm="mcp", resource_metadata="<MCP_RESOURCE_URL>/.well-known/oauth-protected-resource"`
  - Valid token with missing required scope:
    `HTTP 403 Forbidden` with header:
    `WWW-Authenticate: Bearer error="insufficient_scope", scope="<MCP_REQUIRED_SCOPE>"`

- **Claim Validation Rules**:
  - `iss`: Exact match against configured `POCKET_ID_ISSUER`.
  - `aud`: Token `aud` (single string or slice of strings) must match or contain `MCP_RESOURCE_URL` (normalizing trailing slashes).
  - `exp`: Token must not be expired, with an acceptable clock skew tolerance of 60 seconds (`time.Minute`).
  - `scope`: If `MCP_REQUIRED_SCOPE` is non-empty, parse token's `scope` claim (supporting both space-delimited string and string array) and verify presence of required scope.

- **Dual-Mode Authentication & Fast-Fail Logic**:
  - If the token exhibits standard JWT structure (`header.payload.signature`): evaluate exclusively via OAuth 2.1 rules. Fail fast with HTTP 401 and challenge header on invalid signature, expiration, issuer, or audience mismatch.
  - If the token is an opaque string (non-JWT) and static bearer tokens are configured: fall back to matching static token SHA-256 hashes.

- **Context Claims Model**:
  Define a strongly-typed `Claims` model containing `Subject`, `Email`, `Issuer`, `Audience`, `Scopes []string`, and the raw token. Provide context helpers (`ClaimsFromContext`, `SubjectFromContext`) to expose validated identity to downstream handlers.

- **Context-Aware Transport Integration**:
  Update the MCP inbound handler to check for validated claims in the request context. If claims are present, bypass internal static token checks while using the SHA-256 hash of the Bearer token for session affinity mapping, preserving session isolation and timeout reaping.

- **Non-Blocking Background Keyset Refresh**:
  Initialize the JWKS cache with background auto-refresh asynchronously during startup. If Pocket ID is temporarily slow or offline, listener binding proceeds without fatal crashes.

- **Environment-Driven Configuration with Safe Defaults**:
  Load configuration on startup as restart-only values:
  - `POCKET_ID_ISSUER`: Default `"https://id.bullprog.com"`
  - `POCKET_ID_JWKS_URL`: Default `"https://id.bullprog.com/.well-known/jwks.json"`
  - `MCP_RESOURCE_URL`: Default `"https://mcp.bullprog.com"`
  - `MCP_REQUIRED_SCOPE`: Default `""`

## Testing Decisions

- **Highest-Seam HTTP Wire Testing**:
  A good test verifies observable HTTP wire behavior rather than mocking internal helper functions. Tests will spin up an `httptest.Server` running the real composed inbound listener (`inboundServer.Handler`) and issue real HTTP requests over loopback connections.
- **In-Process Mock JWKS Server**:
  Tests will run an ephemeral `httptest.Server` serving a dynamically generated JWKS public keyset from a test RSA keypair. Test tokens will be signed with the matching private key, ensuring zero outbound internet calls and deterministic execution in all environments.
- **Test Scenarios**:
  1. `GET /.well-known/oauth-protected-resource`: Verify 200 OK, JSON content-type, correct resource URL, dynamic authorization server list, and supported scopes/methods.
  2. Unauthenticated request to `/mcp`: Verify 401 Unauthorized and exact `WWW-Authenticate: Bearer realm="mcp", resource_metadata="..."` challenge header.
  3. Expired token request to `/mcp`: Verify 401 Unauthorized and challenge header.
  4. Token with invalid signature or wrong issuer: Verify 401 Unauthorized.
  5. Token with wrong audience: Verify 401 Unauthorized.
  6. Clock skew tolerance: Verify token expiring 30 seconds ago is accepted under 60-second clock skew tolerance.
  7. Missing scope request when `MCP_REQUIRED_SCOPE` is active: Verify 403 Forbidden and `WWW-Authenticate: Bearer error="insufficient_scope", scope="..."` header.
  8. Valid token request: Verify 200 OK JSON-RPC response, session header affinity, and claims available in context.
  9. Dual-mode fallback: Verify opaque static token matches configured YAML bearer token with 200 OK.
  10. Health check bypass: Verify `/livez` and `/readyz` bypass authentication without credentials.
- **Prior Art**:
  Follows the integration testing patterns in `cmd/mcp-bridge/serve_test.go` and `internal/service/inbound_test.go`, maintaining the project-wide source-line test coverage gate of $\ge 90\%$.

## Out of Scope

- Implementing an OAuth authorization server or token issuance endpoint (delegated entirely to Pocket ID).
- Dynamic OAuth client registration (RFC 7591).
- Browser-based interactive login redirects or PKCE flows (handled by client runtimes before contacting the bridge).
- Modifying upstream MCP server connection authentication (bridges connect to upstreams using existing upstream credentials).

## Further Notes

- The metadata endpoint URL path `/.well-known/oauth-protected-resource` is explicitly specified by RFC 9728 Section 3.
- Normalization ensures that URLs with or without trailing slashes do not break audience or metadata URL matching.
- The dual-mode fallback ensures 100% backward compatibility with existing integration test suites in the repository while enforcing OAuth 2.1 in production.
