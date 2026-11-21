# 05 — Auth rejection and session lifecycle counters

**Type:** task

**What to build:** Outer HTTP rejections and session churn stop being invisible. Expired-JWT spikes from clock skew, invalid-token spikes from JWKS rotation, and eviction loops from `MaxSessions` each become a distinct, alertable series.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] `bridge_auth_rejections_total{reason}` is exported with the closed set `missing_token`, `jwt_expired`, `jwt_invalid`, `scope_denied`, `unknown_session`, `host_origin_rejected`, and is incremented for HTTP-level rejections that never reach MCP dispatch.
- [ ] `bridge_session_events_total{event}` is exported with the closed set `created`, `rehydrated`, `expired`, `evicted`, `rejected`, incremented exactly once per event on the path that performs it.
- [ ] Session auto-vivification, idle expiry, and `MaxSessions` eviction are distinguishable from each other and from rejection.
- [ ] Neither counter can be created by free-text input: an unknown reason or event is refused without a series.
- [ ] Tests cover expired JWT, invalid JWT, unknown session, eviction at the session cap, and label refusal.