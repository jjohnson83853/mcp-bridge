# Changelog

All notable changes to this project are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- **Outer OAuth rejection diagnostics (`oauth_diag_v1`).** Early OAuth
  rejections now carry authoritative typed reasons in the terminal
  `http_complete` log record: `credentials_missing` (401 resource challenge),
  `bearer_malformed` (401), `jwt_key_unavailable` and `jwt_audience_invalid`
  (401), `scope_insufficient` (403 with the existing insufficient-scope
  challenge), and `jwt_invalid_other` for generic parser rejections (proving
  neither expiry nor a particular signature/key failure). Rejected requests
  record `failure_stage: outer_oauth`, `session_state: not_evaluated`, and
  `upstream_state: not_attempted`; the existing status, challenge, and response
  body are unchanged, there is no downstream side effect, and the first
  decisive rejection is immutable. See the README "HTTP request diagnostics"
  section for example records and operator guidance.

### Changed

- **`duration_ms` measurement boundary.** The latency-log field now measures
  outer-owner entry — before routing, OAuth evaluation, and correlation-ID
  generation — through handler return/unwind, not the former inner-handler
  interval. Signed milliseconds are retained, including truthful negative
  injected-clock deltas. A separate bounded `diag_duration_ms` field
  (`0`–`86400000`, clamped and flagged with `duration_capped: true`) measures
  the same request interval as `duration_ms` within those semantic bounds,
  never the diagnostic record generation itself. Existing Prometheus metrics and
  counts are unchanged and are not widened with the log interval; latency-log
  consumers must treat `duration_ms` under its new, wider semantics.

### Unchanged

- Severity policy: accepted requests log `info`, expected 4xx rejections log
  `warn`, actual dependency/internal 5xx log `error`, and unavailable
  responses log `warn`. Single failure stream, no duplicates.
- The `Mcp-Diagnostic-Id` response header remains server-owned cryptographic
  128-bit correlation (32 lowercase hex), set on every response before any
  write including 401/403. It is not caller-supplied, not a lookup credential,
  and absent when request entropy fails. It reveals no credential, session, or
  raw request data; server-internal headers remain scrubbed from records.
- Stdout is the shipping boundary: the terminal record is a single write
  attempt at transport completion, not retried; abrupt process loss may lose
  it and no crash-log delivery guarantee exists.
