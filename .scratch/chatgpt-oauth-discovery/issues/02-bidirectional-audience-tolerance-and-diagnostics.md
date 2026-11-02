# 02: Bidirectional audience tolerance and missing-credential safe diagnostics

**What to build:** Allow bearer tokens minted for either the origin audience (`https://tool.bullprog.com`) or endpoint audience (`https://tool.bullprog.com/mcp`) to authenticate successfully against the bridge, tolerating trailing slashes and the `/mcp` suffix in either direction. When an unauthenticated client initiates Action Discovery without an `Authorization` header, emit safe, non-sensitive request diagnostic logs (`path`, `host`, `user_agent`, `has_x_forwarded_uri`, `has_origin`) without leaking sensitive tokens or cookies, while recording `credentials_missing` in structured diagnostics and returning an HTTP 401 challenge pointing to the metadata URL.

**Blocked by:** 01: RFC 9728 multi-path metadata discovery and path-aware resource advertisement

**Status:** ready-for-agent

**Type:** task

## Acceptance criteria

- [ ] Audience validation accepts tokens where the token audience equals the resource URL, or where one has a `/mcp` suffix relative to the other (origin token vs endpoint target, or endpoint token vs origin target).
- [ ] Trailing slashes on either the token audience or target resource URL do not cause audience validation failures.
- [ ] Tokens with completely distinct audiences (unrelated domains or paths) continue to be rejected with `jwt_audience_invalid` and HTTP 401.
- [ ] When a request reaches protected endpoints without an `Authorization` header, the bridge emits structured log output capturing `path`, `host`, `user_agent`, `has_x_forwarded_uri`, and `has_origin`.
- [ ] Diagnostic logs never record token bytes, authorization headers, cookies, query parameters, or request bodies.
- [ ] Unauthenticated requests to protected endpoints return HTTP 401 with `WWW-Authenticate: Bearer realm="mcp", resource_metadata="..."` pointing to the registered metadata URL.
- [ ] Unit tests in `internal/auth` verify audience matching permutations and diagnostic logging hooks.
- [ ] Wire tests in `cmd/mcp-bridge` verify audience acceptance for origin and endpoint tokens on the composed listener.
