# 01: HTTP request diagnostics and precise admission failures

**What to build:** Operators can explain an HTTP request's transport outcome and the precise reason it was rejected, without inspecting request contents. Deliver the real inbound HTTP-to-structured-log path, including production composition wiring and captured-log verification. No logger-only scaffold or authentication-policy change.

**Blocked by:** None (can start immediately).

**Status:** complete

Type: task

Source: Logging observability specification, revision LOG-OBS-r4 (supersedes the original r1 source reference; approved ticket scope and dependencies unchanged). Coverage: AC1, AC3; HTTP portions of AC8, AC14–AC21 and AC24. Ticket publication approval does not authorize implementation; applicable project review and user-approval gates still apply.

- [x] Each request reaching the inbound handler, including an unmatched path, produces exactly one terminal HTTP record. It contains the finite HTTP method/route categories, observed status, transport-scoped duration, bounded actual-write byte bucket, and internal request correlation defined by the spec. Implicit successful writes/flushes report 200; missing response evidence is explicitly unavailable, never status zero or invented success. Arbitrary paths, queries and unusual verbs are not echoed.
- [x] Bad credentials, invalid session binding, disallowed Host, disallowed Origin, oversized requests and session capacity have distinct finite stages/reasons. When both Host and Origin are disallowed, Host remains the first decisive rejection. Client/session authentication failures are never backend_auth_failed. No rejected request is represented as a backend attempt.
- [x] Statuses, challenges, public request-ID acceptance/return behavior, request/session limits and admission ordering remain unchanged. Internal request correlation is generated independently of caller identifiers, with the spec's honest unavailable state on entropy failure.
- [x] Extend the existing logger allowlist and finite-value checks only for this slice. Invalid field types, out-of-vocabulary values, arbitrary nested metadata and nonfinite numeric values cannot create new log data channels. Existing typed configuration diagnostics and their redaction contract remain unchanged.
- [x] The existing real HTTP/MCP composition-root harness proves successful, implicit-status, route, verb and rejection cases through public requests and captured JSON logs. Synthetic secrets in credentials, cookies, session/header values, request IDs, paths, queries and hostile logger fields are absent from the complete emitted bytes. Do not replace integration evidence with mock forwarding assertions.
- [x] Existing metrics scrape policy and representative series/labels/counts remain compatible; new logging classifications do not change metric labels. A failing log writer remains nonfatal without retries, recursion or response changes. Startup, reload and shutdown still work through the production composition root.
- [x] Update existing operator documentation/changelog for the HTTP fields, rejection meanings, stdout collection boundary, delivery/retention limitations and unchanged metric policy. No collector, deployment, dashboard, alert or Loki delivery guarantee is added; any documented Loki indexing guidance keeps high-cardinality IDs and identities out of labels.
- [x] Retain deterministic failing-before/passing-after evidence for the missing status and incorrect rejection logs, then exercise the actual local server path. Run the required focused and full-suite checks, measure at least 90% full-suite source-line coverage using the repository's coverage convention, and resolve applicable code/Go/security review findings. Existing passing safety checks remain regression evidence, not invented RED results. No public response, authentication, session, networking or LibreChat compatibility change is permitted.
