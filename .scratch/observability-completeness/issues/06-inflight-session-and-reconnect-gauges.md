# 06 — Inflight, active-session, and reconnect gauges

**Type:** task

**What to build:** Leaked upstream streams and stuck sessions become directly visible: an inflight gauge that stays above zero while request totals are flat indicates a missing cancellation or unreleased stream, and reconnect counts distinguish a flaky upstream from a slow one.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] `bridge_inflight_requests{server, operation}` increments before an upstream call and decrements on every exit path, including panic, context cancellation, and timeout, so it returns to zero after any single call.
- [ ] `bridge_active_sessions{mode}` reports live sessions separated by `stateful` and `stateless` mode and decrements on expiry, eviction, DELETE, and teardown.
- [ ] `bridge_upstream_reconnects_total{server, reason}` is exported with the closed set `eof`, `timeout`, `protocol_reset`, `session_expired`, counted once per reconnection attempt with its classified cause.
- [ ] All three use the bounded server set and closed vocabularies; no client-supplied text becomes a label.
- [ ] Tests cover gauge return to zero after a cancelled upstream call, decrement on eviction and DELETE, and reconnect reason classification.