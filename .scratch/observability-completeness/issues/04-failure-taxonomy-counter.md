# 04 — Failure taxonomy counter (`bridge_errors_total`)

**Type:** task

**What to build:** Every classified failure — including the HTTP 200 that carries a JSON-RPC error or `CallToolResult.isError == true` — increments one counter that says whether it was client misuse, an upstream fault, or an application-level tool failure. Alerting on upstream and storage stages then catches broken containers immediately.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] `bridge_errors_total{server, stage, reason}` is exported; `stage` is the closed set `auth`, `session`, `admission`, `schema_validation`, `upstream_transport`, `upstream_protocol`, `tool_is_error`, `storage`.
- [ ] `reason` comes from the existing observe failure-reason vocabulary; a reason outside it creates no series.
- [ ] Incremented exactly once per classified failure: a tool result with `isError == true` increments `tool_is_error` while the request itself still counts as a successful outbound request; a transport timeout increments `upstream_transport`; local validation rejects increment `schema_validation` or `admission`.
- [ ] Client misuse, upstream fault, and application-level failure never collapse into the same series, and no series is double-counted by the existing request counter.
- [ ] `server` uses the bounded server set with the aggregate label where unattributable.
- [ ] Tests cover `isError` result, upstream timeout, local schema rejection, and reason-label refusal.