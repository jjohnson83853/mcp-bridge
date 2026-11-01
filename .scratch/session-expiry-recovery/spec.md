# Expired-session recovery for long-running MCP clients

Status: ready-for-agent

## Problem Statement

Long-running MCP clients — including omp sessions that stay connected for hours — intermittently fail every bridge tool call with `401 Unauthorized` after the bridge has expired or forgotten their `Mcp-Session-Id`. The client still holds a valid bearer token and keeps sending its old session id; the bridge rejects the request as an authentication failure before dispatch, and the client surfaces `MCP failure … retryable: no … Check the MCP endpoint and authentication configuration.` The only recovery the operator has today is to manually reconnect the tool, which discards the agent's working session state.

The bridge conflates two different conditions: "this credential is not valid" and "this session no longer exists". MCP defines the latter as a recoverable condition, and omp already implements the recovery path for it: omp 18.13.2 classifies HTTP `404` (and 502/503) from an MCP server as retryable, tears down the transport, reconnects, and performs a fresh `initialize` — creating a new session without operator action. It does not do that for `401`, which it treats as a terminal authentication failure.

## Solution

Return `404 Not Found` (without `WWW-Authenticate`) when a request carries a valid bearer credential but a session id the bridge cannot use, and reserve `401 Unauthorized` for missing or invalid credentials. An omp client then recovers automatically through its existing reconnect-and-reinitialize path; any spec-compliant MCP client can do the same. No custom "rebuild session" endpoint, persisted session store, or client change is needed.

A session id is unusable when it is absent from the bridge's live-session table (idle-expired, never issued, or lost on restart) or when it is bound to a different accepted credential fingerprint. In both cases no session state is read or modified and no tool is dispatched; the caller only learns that it must start a new session, which still requires a valid bearer.

## User Stories

1. As an omp user with a long-running connection, I want an expired bridge session to trigger the client's automatic reconnect, so that my next tool call succeeds without manually reconnecting the tool.
2. As an omp user, I want a session expiry to stop being reported as an authentication failure, so that I do not waste time checking tokens and auth configuration that are actually fine.
3. As an omp user, I want the retry to create a fresh session transparently, so that I can keep working in the same agent session.
4. As an MCP client author, I want the bridge to follow the MCP session-expiry contract, so that the standard 404-and-reinitialize behavior works without bridge-specific handling.
5. As a client author, I want `404` to be distinguishable from `401`, so that I can treat an unusable session as recoverable and a bad credential as terminal.
6. As a client author, I want the `404` response to be free of `Mcp-Session-Id` and `WWW-Authenticate` headers, so that I do not mistakenly treat it as a challenge or adopt a session the bridge did not issue.
7. As an operator, I want an idle-expired session to produce the same recoverable signal as a session lost on restart, so that both cases self-heal the same way.
8. As an operator, I want a credential that was rotated or is no longer accepted to still produce `401`, so that genuine authentication failures remain visible and actionable.
9. As an operator, I want a valid credential presenting another credential's session id to get `404` rather than session access, so that session hijacking remains impossible while the legitimate holder can recover.
10. As an operator, I want the session cap to keep returning its existing `503`, so that capacity pressure is not disguised as expiry.
11. As an operator, I want the bridge to keep expiring idle sessions, so that memory stays bounded and abandoned sessions do not accumulate.
12. As an operator, I want diagnostics to keep recording the distinct `session_absent` and `session_fingerprint_mismatch` reasons, so that post-incident analysis can tell expiry, restart, token rotation, and credential attacks apart even though the client sees the same recovery signal.
13. As an operator, I want the HTTP completion record to show the actual `404` status, so that I can measure how often clients need to reinitialize.
14. As a security reviewer, I want the new `404` path to perform no session reads or writes and no dispatch, so that it cannot become an authorization bypass.
15. As a security reviewer, I want the response body and headers to reveal nothing about whether the session id ever existed, so that the endpoint is not a session oracle.
16. As a bridge maintainer, I want the status decision to live in one place in the inbound edge, so that POST and DELETE (and future methods) cannot drift apart.
17. As a bridge maintainer, I want the change to leave credential checks, session binding, request limits, body limits, origin policy, and protocol version untouched, so that only session-expiry semantics change.
18. As a bridge maintainer, I want deterministic tests for expiry, so that the behavior is verified without sleeping in the suite.
19. As a bridge maintainer, I want one real `/mcp` wire test proving initialize → expire → 404 → reinitialize → success, so that the omp-compatible recovery path is proven end to end.
20. As an operator reading the docs, I want the recovery behavior and the client expectation documented, so that future 404s are not misdiagnosed as outages.
21. As an omp user, I want my surviving SSE stream to keep working while idle-expiry is handled on the request path, so that recovery does not disrupt unrelated live traffic.

## Implementation Decisions

- The inbound edge owns the session-status decision. It runs after bearer authentication and after the live-session lookup, before any delegation to the downstream MCP handler. The delegated handler is never invoked on an unusable-session rejection.
- Status mapping:
  - Missing, malformed, or unaccepted bearer credential → `401 Unauthorized` with the existing `WWW-Authenticate: Bearer` header.
  - Accepted bearer credential, no `Mcp-Session-Id` header → existing behavior (the request may create a session).
  - Accepted bearer credential, session id absent from the live-session table → `404 Not Found`, no `WWW-Authenticate`, no `Mcp-Session-Id`.
  - Accepted bearer credential, session id present but bound to a different accepted credential fingerprint → `404 Not Found`, same headers and body. The session record is not read beyond the fingerprint comparison, not refreshed, and not deleted.
  - Session cap reached for a request without a session id → existing `503 Service Unavailable`.
- The `404` body is a fixed, non-reflective string (for example "session not found"); it never includes the submitted session id, a token prefix, or timing-derived hints.
- POST and DELETE share the same mapping; no method-specific exception is introduced.
- Diagnostics keep the current reason vocabulary and evidence slots (`session_absent` with `SessionState: absent`, `session_fingerprint_mismatch` with `SessionState: existing`, stage `session`). The only observable diagnostic change is the recorded HTTP status (`404`) on the completion record. No new telemetry field, metric name, or log line is added.
- Idle expiry semantics stay as they are: the idle timeout remains a bounded session lifetime (currently `session_idle_timeout`, minimum 1s, maximum 1h), sessions are swept on request, and an accepted request refreshes `lastSeen`.
- No tombstone, resurrection, or re-admission of an old session id. Recovery always creates a new session through the client's normal `initialize`.
- No new endpoint, header, query parameter, or bridge-specific "rebuild" verb. The wire contract is deliberately the MCP-standard one: an unusable session is `404`, and the client starts a new session.
- Client compatibility note for the docs: omp 18.13.2 treats server `404` as retryable, closes the transport, reconnects with a new transport, and sends `initialize`; `401` is terminal. The spec relies on that behavior rather than adding client-side changes.
- Unchanged surfaces: static bearer authentication, credential fingerprinting, session binding, request/body limits, origin policy, allowed hosts, protocol version negotiation, dispatch, tool metadata, and result envelopes.

## Testing Decisions

A good test asserts externally observable behavior — HTTP status, headers, response body shape, whether the downstream handler was invoked, the protocol-level ability to initialize a new session, and the recorded diagnostic reason/status — rather than poking the session map or asserting internal control flow. Expiry must be driven by an injected clock in unit tests; the wire test may use the minimum `session_idle_timeout` only where a real process is required.

Two seams are used, both existing:

1. **Real `/mcp` wire seam (primary).** Extend the existing `cmd/mcp-bridge` harness (`writeBridgeConfig`, `startHybridBridgeWithUpstream`, `mcpSend`, `probeBody`/`waitFor`, `serveClientToken`) with a session-expiry end-to-end case:
   - initialize and capture `Mcp-Session-Id`;
   - expire it (short `session_idle_timeout` plus a bounded wait, or the harness's clock control if present);
   - send `tools/list` with the stale id and assert `404`, no `WWW-Authenticate`, no new session header, and that no tool dispatch occurred;
   - send a fresh `initialize` without a session id, then `tools/list`, and assert success on the new session;
   - repeat the stale-id assertion for `DELETE /mcp`.
   Prior art: `cmd/mcp-bridge/serve_hybrid_metadata_source_test.go`, `serve_hybrid_exposure_test.go`, and the existing reload tests.

2. **`InboundHandler` unit seam (secondary, for determinism).** Extend `internal/service/inbound_test.go` using the existing `InboundOptions.Now` clock and `SessionIdle` bounds:
   - advance past the idle timeout and assert `404` for POST and DELETE;
   - assert `401` still applies to a missing or wrong bearer with an unknown session id;
   - assert a valid credential with another credential's session id gets `404` and does not reach the downstream handler;
   - assert an unexpired session still reaches the handler and refreshes its idle deadline;
   - assert the diagnostic reason/state pairs remain `session_absent` / `absent` and `session_fingerprint_mismatch` / `existing`.

TDD: each new assertion must fail on the current code (which returns `401`), then pass after the status mapping changes. No assertion may depend on the response time or on a specific downstream handler body.

## Out of Scope

- Changes to omp or any other MCP client; this spec relies on the client's standards-based 404 recovery.
- Token refresh, OAuth authorization-server flows, dynamic client registration, and JWT validation.
- Persisting sessions across bridge restarts, sharing sessions between processes, or moving session state out of memory.
- Changing the default or allowed range of `session_idle_timeout`, or adding new configuration keys.
- A bridge-specific reconnect endpoint, a "resume session" API, or accepting an old session id with a new transport.
- Changing tool metadata (`METADATA-r7`), including its release gate and review findings.
- Deployment, image builds, registry operations, or committing/pushing this planning artifact alone.

## Further Notes

- Key evidence at HEAD `e4f3ac1`: `internal/service/inbound.go` maps both an absent session and a fingerprint mismatch to `401` with `WWW-Authenticate`; `session_idle_timeout` is bounded to 1s–1h and reaped lazily on request; the completion record already carries `session_absent` / `session_fingerprint_mismatch`.
- omp 18.13.2 behavior was read from the installed binary's bundled client sources (`packages/coding-agent/src/mcp/errors.ts`, `mcp/transports/http.ts`, `mcp/manager.ts`): HTTP `404`/`502`/`503` are retryable; a retryable tool failure schedules `reconnectServer`, which builds a new transport and sends `initialize`; `401` is non-retryable and surfaces the auth-configuration hint.
- Acceptance summary for implementation: `404` for unusable sessions with a valid credential; `401` only for credential failures; no session access or dispatch on the rejection path; diagnostics preserved; POST and DELETE agree; a real client can reinitialize after the `404`; full suite and source-line coverage gates still pass.
- This spec is independent of the paused METADATA-r7 release. That change set is blocked by review findings (schema-1 trailing-byte fail-open and related defects) and must not ship as part of this work.
