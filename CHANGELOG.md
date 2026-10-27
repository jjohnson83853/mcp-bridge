# Changelog

All notable changes to this project are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- **Session outcome diagnostics (`oauth_diag_v1`).** The inner session
  admission now distinguishes absent session entries from existing-session
  bearer-binding mismatches in the terminal `http_complete` record:
  `failure_stage: session` with `session_absent` (401, `session_state: absent`,
  `binding_relation: not_evaluated`, no session reference) versus
  `session_fingerprint_mismatch` (401, `session_state: existing`,
  `binding_relation: mismatch`, the live reference in the server record only).
  Legal headerless initialization records `session_state: created` with an
  independent cryptographically random 128-bit `session_ref` (32 lowercase
  hex) retained only with the session and deleted with it; ordinary
  sessionless traffic records `session_state: not_supplied`; an entropy
  failure records `session_ref_state: unavailable` with no retry or derived
  fallback. Missing-session-header reasons are reserved for actual
  session-required branches with an authoritative cause and are not emitted
  from header absence alone. Statuses, challenges and response bodies are
  unchanged; the reference never appears in a response and no raw session ID,
  digest or correlation map is retained. Real OMP/Gemini smoke NOT RUN.
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

- **Session state at the inner credential gate.** An inner static-credential
  rejection now records `session_state: not_evaluated` truthfully for a
  supplied session header (previously `existing`), because the stored-entry
  lookup and comparator run after credential acceptance; header presence
  remains its own independent `session_header` slot.
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
