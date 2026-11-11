# 10 - Discovery Attribution and Readiness Reason

Status: ready-for-agent
Blocked-by: 09

## What to build
An upstream discovery failure identifies where it failed, not merely that it failed. The record names the failing stage - DNS, connect, TLS, authentication, initialize, or tool listing - with a bounded cause category and the HTTP status or timeout where applicable, so "401 during initialize" is distinguishable from "tool listing timed out". The configured endpoint is identified in sanitized form only: scheme, host, port and path. Credentials, headers, query strings and response bodies never appear.

The record also states the recovery expectation: whether an automatic retry is scheduled or the configuration or credentials must be corrected, with the next attempt time present only when an automatic retry is scheduled. Repeating on the refresh interval is not evidence that an unresolvable pin or a bad credential will recover unattended.

Finally, a rejected pass updates the readiness reason for the affected server instead of leaving it empty, so a server that failed is distinguishable from a server that has not been probed yet.

## Acceptance Criteria
- [ ] AC10-1 (10/AC-1; R23, R25): a discovery failure names `failure_stage` and a bounded `failure_reason`, distinguishing auth-during-initialize from tools-list timeout; the emitted records carry the AD-pinned levels — `upstream_discovery_failed` at `warn` (AD17) and, per the pass outcome, the accompanying `catalog_refresh_failed` at `warn` with `retained_published` or `error` with `none_available` (AC08-1/AC08-7) — a diagnostic at any other level (in particular `debug`) fails this criterion. test.
- [ ] AC-2: The HTTP status or timeout value is reported where applicable.
- [ ] AC-3: No credential, header, query string or response body appears; the endpoint is reduced to scheme, host, port and path.
- [ ] AC-4: Each failure states automatic-retry-scheduled or human-correction-required, with the next attempt time present only when an automatic retry is scheduled (per AD17).
- [ ] AC-5: After a rejected pass, the affected server's readiness reason is non-empty and identifies the failure, distinct from a not-yet-probed server.
