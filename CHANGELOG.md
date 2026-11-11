# Changelog

All notable changes to this project are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added
- **Structured logging contract.** Application logs and CLI failures now use
  schema-versioned JSON lines with typed configuration attribution, stable
  error classifications, unified reload/startup events, and bounded HTTP
  diagnostics. The README and [logging contract](docs/logging-contract.md)
  document the application field union, collector boundary, and query examples.

- **Structured logging r8 refresh diagnostics.** Refresh lifecycle and discovery
  records, per-pin blocker details, successful reload attribution, and a
  minimal opposite-lane logger-write fallback were added as optional
  event-specific additions; `schema_version` remains 1. The pin ceiling is 128,
  with the first excess at `pins[128]`; tool-list chunks remain capped at 64
  names. See the [logging contract](docs/logging-contract.md) for event fields
  and queries.

- **Dynamic direct-first guidance for gateway built-ins.** `search_tools`
  descriptions list the exact, sorted wire names of published configured pins
  (with clean baseline guidance when none are published), while `describe_tool`
  and `call_tool` steer clients away from redundant discovery and multiplexing.
  Unpinned tools remain discoverable; schemas, safety annotations, dispatch, and
  upstream metadata are unchanged.
- **Inbound observability enrichment and operation-partitioned request metrics.**
  Incoming requests are classified by inbound method/transport into a closed
  finite operation enum (`initialize`, `initialized`, `tools_list`, `tools_call`,
  `resources_list`, `resources_read`, `ping`, `prompts_list`, `prompts_get`,
  `sse_stream`, or fallback `request`).
  Prometheus counter `bridge_requests_total` partitions requests by this enum,
  and terminal `http_complete` diagnostic logs emit `operation`, exact response-body
  byte count (`http_bytes`),
  `client_family` (`openai`, `claude`, `cursor`, `omp`, `browser`, `other`, `absent`),
  and `exposed_tools` count on successful `tools/list` requests.


- **Progressive metadata reveal and session-scoped projections.** Initial
  `tools/list` exposure serves built-ins and configured pins. Calls to
  `search_tools` and `describe_tool` dynamically commit unpinned tools to the
  calling session's local projection with full metadata defaults and exact
  exception parity. Includes URL-safe 57-byte HMAC-SHA256 pagination cursors
  (-32602 on stale or invalid cursors) and session lifecycle retirement on
  HTTP DELETE or idle expiry with in-flight direct execution lease
  preservation.

- **Exact-tool metadata exceptions (`tool_metadata.tools`).** A server's
  `tool_metadata` mapping may carry a `tools` mapping of exact raw tool names
  to patches, layered above that server's defaults with precedence upstream
  source → server defaults → exact-tool exception. Entries must name tools
  pinned on the same server (no wildcard/cross-server selectors); unknown
  targets, invalid names, unsupported members and duplicate entries reject
  the whole candidate atomically through the typed, redacted diagnostics.
  Exceptions join the public configuration fingerprint (never the credential
  identity), publish through the existing reload path, report
  `layer: "tool"` provenance in `describe_tool`'s
  `metadata_override_fields`, and removing a patch on reload reverts that
  tool to the current defaults or source. Corrections are never persisted as
  upstream truth. See "Exact-tool exceptions" in the README for the
  mixed-service risk and client-policy boundary.
- **Server-scoped metadata defaults (`tool_metadata.defaults`).** Any
  configured server may declare a `defaults` patch (`description`, `title`,
  `icons`, `annotations`, `_meta`) that is applied — after the upstream's
  SDK-normalized source values — to that server's configured pins only.
  Object members merge recursively, arrays and scalars replace outright, and
  explicit `false`/empty/`null` members are retained as values. Invalid or
  unknown members reject the whole candidate atomically through the typed,
  redacted config diagnostics. Defaults change the public configuration
  fingerprint (never the credential identity), publish through the existing
  candidate/reload path (`tools/list` serves the effective descriptor;
  `describe_tool` adds `metadata` and `metadata_override_fields` with
  `server_default` provenance while `upstream_metadata` stays the source
  view), and removing them on reload reverts the same session to the current
  source values. Corrections are never persisted as upstream truth. See
  "Server metadata defaults" in the README for the mixed-service
  misclassification risk.
- **Upstream tool metadata preservation (schema 2).** Discovery now converts
  each upstream tool's SDK-normalized metadata — title, icons, the annotation
  object (title plus the four standard hints, with explicit `false` pointer
  hints preserved as false rather than dropped), and supported `_meta` keys —
  into the catalog definition. Pinned tools publish it on the wire
  (`tools/list` carries annotations, title and icons; `describe_tool` reports
  `upstream_metadata`), and a metadata-only upstream change moves the
  definition digest and catalog revision, triggering republish while
  metadata-absent definitions keep their frozen legacy digests. Metadata is
  advisory claims from the upstream, never authorization.
- **Snapshot schema 2 with a strict schema-1-or-2 reader.** The durable
  snapshot writer publishes `schema_version: 2` with optional per-tool source
  metadata; the reader selects the decoder by document version and strictly
  verifies schema 1 through its frozen legacy representation (unchanged
  checksum semantics) and schema 2 through the canonical representation,
  rejecting unknown fields, trailing data, checksum mismatches, tampered
  metadata and unsupported versions. Loading legacy state never rewrites it;
  the next accepted save publishes schema 2. Rollback remains restore, not
  reverse migration: stop the new process, retain the newer state separately,
  and restore a validated pre-upgrade schema-1 snapshot with its matching
  config before starting an old binary.

- **Built-in tool annotations.** The three bridge-owned tools now declare
  standard MCP tool annotations on the wire: `search_tools` and `describe_tool`
  are read-only, idempotent, open-world discovery (no destructive hint);
  `call_tool` remains potentially mutating, potentially destructive,
  non-idempotent, and open-world. Annotations are advisory client hints — not
  authorization, not a prompt-suppression guarantee, and never a trust bypass.
  No name, schema, auth, dispatch, or initialize behavior changed.

- **Runtime provenance and safe route context (`oauth_diag_v1`).** The
  terminal `http_complete` record now carries the process incarnation
  (`process_id`/`process_state`: one independent cryptographically random
  128-bit value per process, stable across reloads, explicitly unavailable on
  entropy failure with no derived fallback), the verified immutable source
  revision (`source_revision`/`build_state`: 40–64 lowercase hex from the
  binary's VCS stamp only when clean and stamped; a locally modified build
  reports `unavailable` and never claims deployment), the effective policy
  provenance (`effective_policy_revision`/`effective_policy_revision_state`:
  an attested nonsecret artifact identifier when one exists, otherwise
  explicitly unavailable — no secret-bearing configuration is hashed; the
  fixed `diagnostic_policy_revision` identifies the diagnostic contract only),
  the nonsecret policy tuple (`policy_source`, `oauth_enabled`,
  `static_configured`, `scope_required`, `clock_skew_seconds`,
  `resource_path_mode`), finite route and challenge context
  (`path_class`, `route_outcome` by actual dispatch, `challenge_kind`,
  `challenge_target`, `metadata_target_registered`) with no path, query, URL,
  realm, scope or header values, and truthful operation linkage
  (`operation_relation` none/single/multiple/unavailable with one eligible
  server-owned `operation_id` only for a single linked operation, plus the
  actual `upstream_state` classes — a tool-error envelope under HTTP 200
  records `tool_error` and stays distinct from admission rejection). Statuses,
  challenges, response bodies, metrics, label names and cardinality are
  unchanged; the record is still a closed typed allowlist within the same
  byte budgets. See the README "Runtime provenance and safe route context"
  section for operator guidance. Real OMP/Gemini smoke NOT RUN.
- **Session outcome diagnostics (`oauth_diag_v1`).** Authenticated unknown
  session IDs now initialize a fresh SDK session and map its SDK-issued ID to
  the preserved client ID before forwarding, so pre-restart clients can
  continue on the ordinary MCP handling path. Successful recovery records
  `session_state: rehydrated` with `binding_relation: not_evaluated` and an
  independent diagnostic reference; SDK initialization failure returns 503.
  An unknown-session DELETE returns 204 without forwarding. Existing-session
  bearer-binding mismatches retain `session_fingerprint_mismatch`,
  `session_state: existing`, `binding_relation: mismatch`, and the live
  reference in the server record only.
  Legal headerless initialization records `session_state: created` with an
  independent cryptographically random 128-bit `session_ref` (32 lowercase
  hex) retained only with the session and deleted with it; ordinary
  sessionless traffic records `session_state: not_supplied`; an entropy
  failure records `session_ref_state: unavailable` with no retry or derived
  fallback. Missing-session-header reasons are reserved for actual
  session-required branches with an authoritative cause and are not emitted
  from header absence alone. Authentication failures and existing-session
  mismatch statuses/challenges remain unchanged; references never appear in
  responses, and no raw session ID, digest or correlation map is retained.
  Real OMP/Gemini smoke NOT RUN.
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

- **Explicit discovery-tool advisory hints.** `search_tools` and
  `describe_tool` now declare `destructiveHint: false` and
  `openWorldHint: false`, matching their read-only, closed-world catalog behavior.

- **Metadata defaults and exceptions apply to revealed tools.**
  `tool_metadata.defaults` and `tool_metadata.tools` exception patches apply
  to unpinned tools discovered via `search_tools` or `describe_tool`, matching
  pinned-tool metadata parity and provenance reporting
  (`server_default`, `tool`).

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
