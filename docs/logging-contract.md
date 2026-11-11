# Structured logging contract

## Scope and wire envelope

Every application log record and CLI error is one newline-delimited JSON object
with `schema_version: 1` and `service: "mcp-bridge"`. The field union below is
the complete application-owned vocabulary, not a list of required-on-every-record
keys: event-specific fields may be absent, but an emitted key must belong to the
union. Nested paths name JSON object members.

The common envelope is:

```json
{
  "schema_version": 1,
  "level": "error",
  "service": "mcp-bridge",
  "event": "config_validation_failed",
  "message": "configuration contains more pins than supported",
  "operation": "load_config",
  "outcome": "failure",
  "error": {
    "code": "pin_ceiling",
    "kind": "configuration",
    "retryable": false,
    "remediation": "remove pins at index 128 or later"
  },
  "config": {
    "path": "/app/configs/config.yaml",
    "property_path": "pins[128]",
    "line": 134,
    "column": 5,
    "pin_server": "alpha",
    "pin_tool": "tool-128",
    "actual_count": 129,
    "max_count": 128,
    "first_excess_index": 128
  }
}
```

`message` is for people; alert and query on `event`, `error.code`, or
`error.kind`, not prose. Messages, codes, kinds, outcomes, and remediation text
are package-owned finite values. Arbitrary Go error text is never a log field.
A record is encoded and its severity-lane write is attempted once, with one
trailing newline; a short or failed write is not retried. On encode or write
failure, the logger emits one fixed, credential-free JSON fallback record to
the opposite severity lane. The fallback is not recursively logged or retried.

### Severity, outcome, and stream

The shared envelope accepts levels `debug`, `info`, `warn`, and `error`;
`debug`/`info` go to stdout and `warn`/`error` go to stderr. The shared `outcome`
vocabulary is `unknown`, `success`, `failure`, `error`, `denied`, `busy`,
`timeout`, `fallback`, and `degraded`. The HTTP lane's `http_outcome` uses
`success`, `failure`, `incomplete`, or `unknown`; its top-level `outcome` uses
the same four values. Per-event pins are listed below. An error object is used
for a failure classification; successful configuration validation has no error
object.

### Event and operation pins

| Event | Operation | Pinned level and outcome |
|---|---|---|
| `config_validation_failed` | `load_config` for serve and `--check-config` | `error` / `failure` |
| `config_validation_succeeded` | `check_config` | `info` / `success` |
| `startup_failed` | `startup` | `error` / `failure` |
| `startup_publication_failed` | `startup` | `error` / `failure` |
| `cli_usage_failed` | `cli` | `error` / `failure` |
| `config_reload_rejected` | `reload` | `warn` / `failure` |
| `reload_publication_failed` | `publish` | `warn` / `failure` |
| `startup_snapshot_rejected` | `startup` | `warn` / `degraded` |
| `prompt_publication_failed` | `publish` | `warn` / `failure` |
| `reload_rejected` | `reload` | `warn` / `failure` |
| `internal_warning` | `reload` | `warn` / `failure` |
| `upstream_ephemeral_handshake` | `handshake` | `info` / `success` |
| `config_reload_applied` | `reload` | `info` / `success` |
| `catalog_refresh_started` | `refresh` | `info` / `unknown` |
| `catalog_refresh_completed` | `refresh` | `info` / `success` |
| `catalog_refresh_failed` | `refresh` | `warn` or `error` / `failure` |
| `upstream_discovery_failed` | `refresh` | `warn` / `failure` |
| `pin_blocker_detail` | `refresh` | `warn` / `failure` |
| `logging_write_failed` | `logging` | `warn` / `failure` |
| `http_complete` | finite inbound-method/transport classification | level/outcome reflect the observed request |
| `mcp_complete` | decoded MCP operation | level/outcome reflect that operation |
| `backend_complete` | `call_tool` | level/outcome reflect the upstream call |

The HTTP operation classification is finite: `initialize`, `initialized`,
`tools_list`, `tools_call`, `resources_list`, `resources_read`, `ping`,
`prompts_list`, `prompts_get`, `sse_stream`, or fallback `request`. MCP and
backend records carry their typed event-specific values; for example, a tool
call has operation `call_tool`.

## Complete application field union

This list is sorted and deduplicated exactly as
`observe.EnvelopeFieldNames()`. That function returns a fresh copy. It includes
the common envelope, event-specific shared-lane fields, all nested object paths,
and the complete HTTP diagnostic lane. No timestamp or collector enrichment is
part of this application field set.

<!-- envelope-fields:start -->
```text
auth_mode
 backend_call_id
 backend_id
 binding_relation
 blocker_class
 build_state
 candidate_servers
 candidate_tools
 catalog_state
 catalog_tools
 challenge_kind
 challenge_target
 chunk_count
 chunk_index
 client_family
 clock_skew_seconds
 config
 config.actual_count
 config.column
 config.diagnostics_incomplete
 config.first_excess_index
 config.line
 config.max_count
 config.path
 config.pin_server
 config.pin_tool
 config.property_path
 config.ref_kind
 config.ref_name
 config.related.column
 config.related.line
 config.related.property_path
 config.related.ref_kind
 config.related.ref_name
 config.related.server
 config.remaining_stages
 config.server
 config.shape
 config_revision
 correlation_state
 count
 credential_state
 diag_duration_ms
 diag_schema
 diagnostic_policy_revision
 discovered_tools
 duration_capped
 duration_ms
 effective_policy_revision
 effective_policy_revision_state
 endpoint
 error
 error.code
 error.kind
 error.remediation
 error.retryable
 error_detail
 error_detail_state
 event
 exposed_tools
 failure_reason
 failure_stage
 final_content_count
 final_shape
 final_structured_presence
 final_text_presence
 generation
 http_bytes
 http_bytes_bucket
 http_method
 http_outcome
 http_status
 level
 mcp_message_kind
 mcp_method
 message
 metadata_target_registered
 mode
 next_attempt_ms
 oauth_enabled
 operation
 operation_id
 operation_id_state
 operation_relation
 outcome
 path_class
 pins_resolved
 pins_total
 policy_source
 process_id
 process_state
 protocol_version
 publication_impact
 refresh_id
 request_id
 resource
 resource.address
 resource.listener
 resource.path
 resource_path_mode
 retained_age_ms
 retained_generation
 route
 route_outcome
 schema_version
 scope_required
 scope_state
 server
 servers_discovered
 service
 session_binding
 session_header
 session_ref
 session_ref_state
 session_state
 slots
 stage
 static_configured
 status_state
 timing_scope
 tool
 tools_discovered
 tools_published
 trigger
 upstream_content_count
 upstream_shape
 upstream_state
 upstream_structured_presence
 upstream_text_presence
```
<!-- envelope-fields:end -->

`schema_version`, `level`, `service`, `event`, `message`, `operation`, and
`outcome` identify the record. `service` is always `mcp-bridge`. Typed shared
fields include `request_id` (a validated 32-character lowercase hexadecimal
correlation ID), `generation`, `duration_ms`, and safe `server` identity.

- The nested `error` object is exactly `{code, kind, retryable, remediation}`.
  `kind` is one of `configuration`, `resource`, `dependency`, `usage`, or
  `internal`. The code and kind tables below pin the class and retryability;
  remediation is static.
- `config` contains `path`, `property_path`, optional one-based `line` and
  `column`, validated `server`, `pin_server`, `pin_tool`, `ref_kind`,
  `ref_name`, optional `shape`, and optional `related` attribution. The related
  object can contain `property_path`, `line`, `column`, `ref_kind`, `ref_name`,
  and `server`. Pin-ceiling records may additionally carry `actual_count`,
  `max_count`, and `first_excess_index`. The single-record completeness fields
  are `diagnostics_incomplete` and `remaining_stages`.
- `resource` carries only the configured `path`, `listener` (`inbound` or
  `operations`), and configured `address` when applicable.
- Event-specific shared-lane values include counts and shapes (`count`, `slots`,
  `catalog_tools`, content counts, and upstream/final shapes), the four
  `*_presence` values (`unknown`, `present`, or `absent`), validated 32-hex
  operation/backend-call IDs, `correlation_state` (`valid` or `unavailable`),
  finite MCP metadata, search `mode` (`hybrid`, `hybrid_partial`, or `keyword`),
  validated `backend_id`, and validated `protocol_version`.
- The HTTP-only `error_detail` object is a separate optional summary with
  `class`, `code`, and `retry`; its classification is closed. It is not freeform
  error text.

### HTTP diagnostic lane

Each observed inbound HTTP request has one `http_complete` diagnostic record.
It uses the same versioned JSON-line envelope and level-to-stream policy. Its
fixed, ordered 60-slot map is:

```text
schema_version, level, event, message, service, generation, duration_ms,
http_outcome, stage, http_status, http_method, route, http_bytes_bucket,
timing_scope, correlation_state, request_id, diag_schema, build_state,
process_state, process_id, operation_relation, operation_id_state, path_class,
route_outcome, status_state, diag_duration_ms, duration_capped, failure_stage,
failure_reason, auth_mode, credential_state, scope_state, session_header,
session_state, binding_relation, session_ref_state, challenge_kind,
challenge_target, metadata_target_registered, diagnostic_policy_revision,
effective_policy_revision_state, effective_policy_revision, policy_source,
oauth_enabled, static_configured, scope_required, resource_path_mode,
session_binding, clock_skew_seconds, upstream_state, error_detail_state,
error_detail, operation_id, session_ref, outcome, error, operation, client_family,
http_bytes, exposed_tools
```

`http_status` is present only when observed; `status_state` distinguishes observed
from unavailable. `http_bytes` and `http_bytes_bucket` describe response bytes,
not request contents. Methods, routes, stages, failure reasons, auth/session
states, challenge classes, and provenance states are finite classifications;
raw paths, query strings, URLs, headers, payloads, credentials, session IDs, and
arbitrary error prose are not emitted. The nested `error` object, when present,
uses the same four-member shape as other records and is limited to correlation-
unavailable or delivery-error cases. Optional `error_detail` is a closed
`{class, code, retry}` summary.

`process_id` is an opaque per-process instance-correlation token paired with
`process_state`. It is a random 128-bit value, stable across configuration
reloads. It is never derived from host, container, environment, or version identifiers.
`build_state` is only a finite state, not deployment identity.

## Error classification

`error.code` is the stable query key; `message` remains human-only. For shared
runtime records, the closed mappings are:

| Codes | `error.kind` | `error.retryable` | Static `error.remediation` |
|---|---|---:|---|
| `host_rejected`, `origin_rejected`, `client_auth_failed`, `session_auth_failed`, `request_too_large`, `route_not_found`, `unsupported_capability`, `unsupported_schema`, `invalid_arguments`, `unknown_tool`, `malformed_request`, `invalid_request`, `method_not_found`, `client_disconnected` | `usage` | `false` | `correct the request and retry` |
| `request_cancelled` | `usage` | `true` | `retry the request` |
| `session_capacity` | `resource` | `true` | `retry when capacity is available` |
| `backend_unavailable`, `backend_auth_failed`, `backend_busy`, `deadline_exceeded`, `upstream_protocol_error`, `catalog_changed` | `dependency` | `true` | `verify the upstream service and retry` |
| `tool_reported_error` | `dependency` | `false` | `correct the upstream tool behavior` |
| `invalid_config` | `configuration` | `false` | `correct the configuration` |
| `internal_error`, `correlation_unavailable`, `unknown` | `internal` | `false` | `inspect the bridge logs and configuration` |

`none` has no error object. An unrecognized runtime reason collapses safely to
`unknown`; it is never serialized as a new code. `error.retryable` describes
whether retrying that class is allowed by the classification, not a guarantee
that a retry succeeds.

### Startup and CLI events

Startup and CLI errors use a typed `error` object and static remediation:

| Event | Level | Operation | Outcome | Code | Kind | Retryable | Remediation | Static message |
|---|---|---|---|---|---|---:|---|---|
| `startup_failed` | `error` | `startup` | `failure` | `state_directory` | `resource` | `false` | `create the configured state directory or fix its permissions` | `application startup failed` |
| `startup_failed` | `error` | `startup` | `failure` | `jwks_cache` | `dependency` | `true` | `verify the identity provider URL and network reachability, then restart` | `application startup failed` |
| `startup_failed` | `error` | `startup` | `failure` | `embedding_provider` | `dependency` | `false` | `fix the embedding provider configuration or credentials` | `application startup failed` |
| `startup_failed` | `error` | `startup` | `failure` | `tool_surface` | `internal` | `false` | `report this failure; the tool surface failed to initialize` | `application startup failed` |
| `startup_failed` | `error` | `startup` | `failure` | `catalog_loop` | `internal` | `false` | `report this failure; the catalog loop failed to start` | `application startup failed` |
| `startup_failed` | `error` | `startup` | `failure` | `listener_bind` | `resource` | `true` | `free the configured address or change the listen setting` | `application startup failed` |
| `startup_failed` | `error` | `startup` | `failure` | `internal_error` | `internal` | `false` | `report this failure; no specific class matched` | `application startup failed` |
| `startup_publication_failed` | `error` | `startup` | `failure` | `publication_failed` | `internal` | `true` | `verify the startup catalog and restart the service` | `startup publication failed` |
| `cli_usage_failed` | `error` | `cli` | `failure` | `cli_usage` | `usage` | `false` | `use valid flags and arguments; run mcp-bridge --help for usage` | `command-line arguments are invalid` |

The `resource` object is included only for state-directory path or listener
and configured-address attribution. `--help` is intentionally human-readable
and is not a JSON record.

### AD3 configuration attribution and classification (AD14 `load_config`)

Configuration failures use `config_validation_failed`, operation `load_config`
for both serve startup and `--check-config`, level `error`, outcome `failure`,
kind `configuration`, and retryable `false`. The envelope contains exactly the
first diagnostic in deterministic document order; it does not emit a detail
record plus a summary. The diagnostic's finite `code` is the closed sorted set
from `config.DiagnosticCodes()`:

If a structured diagnostic is unavailable, the envelope uses fallback code
`config_invalid`, kind `configuration`, retryable `false`, and the same generic
remediation. Diagnostic-backed records use their registered code and the
producer-owned static reason for the first diagnostic.


<!-- config-codes:start -->
```text
bound_range
config_read
config_too_large
credential_overlap
duration_positive
duration_range
duration_type
header_duplicate
header_forbidden
header_mapping
header_name_invalid
pin_ceiling
pin_duplicate
pin_fields
pin_server_missing
pin_server_unknown
pin_tool_missing
pin_tool_name
reference_env_name
reference_invalid
reference_literal_required
reference_required
reference_shape
reference_unresolved
schema_version
secret_cr
secret_empty
secret_file_missing
secret_file_policy
secret_file_read
secret_file_stat
secret_lf
secret_nul
secret_too_large
secret_unset
server_count
server_id_duplicate
server_id_invalid
server_unknown
setting_duration
setting_internal
setting_missing
setting_type
tool_metadata_tool_name
tool_metadata_tool_unknown
tool_metadata_unknown_member
tool_metadata_value
url_invalid
url_secret_parameter
weight_invalid
weights_invalid
yaml_alias
yaml_documents
yaml_key_duplicate
yaml_key_invalid
yaml_key_unknown
yaml_number
yaml_root_type
yaml_syntax
yaml_type
```
<!-- config-codes:end -->

Each configuration code has kind `configuration` and retryable `false`. Unless
a specific action applies, remediation is `correct the configuration and rerun
validation`; `config_read` instead advises checking the file path and
permissions. `pin_ceiling` specifically advises removing pins at index 128 or
later. Message text is the diagnostic's static reason, not raw YAML, a URL, a
secret, or a decoder/OS error. `config.shape` is an optional diagnostic
sub-class; it is not a new top-level field.

Attribution uses the offending value/key's YAML node and one-based line/column
when that node exists. Duplicate keys/references report their own location and
`config.related` for the earlier occurrence. The node sources are:

- Structure validation: schema version, server count and IDs, endpoint URL
  classes, pin validation/ceiling, duplicate YAML keys, and duplicate headers
  use the offending document element or key node.
- Credential overlap: the current occurrence is the own location; the earlier
  occurrence is related.
- Embedding references and embedding URL checks: the reference node is the own
  location. Secret references use their `{env: ...}` or `{file: ...}` node.
- Tool metadata and ordinary bounds: the producing validator's inspected node
  supplies the own location. Weight validation uses the scalar value node;
  cross-weight conflicts also carry the counterpart in `config.related`.

Line/column are omitted only when there is no source node: decoded-Config
accessors, post-parse `restart_required`, document failures without a parser
position, or a required missing weight key. For these, `config.path` and the
known `config.property_path` remain available.

#### Pin ceiling (AD9)

The 129-pin case fails at index 128 after entries 0–127 have been checked, so an
invalid earlier pin wins over the ceiling. It names the `pins[128]` property path
as `config.property_path`, gives the excess entry's one-based line and column,
and reports `actual_count: 129`, `max_count: 128`, and
`first_excess_index: 128`. It includes `pin_server` and `pin_tool` only if the
excess entry passes the same identity validation as ordinary entries. Only the
first excess identity is considered; no other pin identities or values are
logged.

`config.diagnostics_incomplete` and `config.remaining_stages` preserve whether
the first diagnostic stopped later validation stages inside the same envelope.

## Reload and lifecycle events (AD7, AD10, AD13, AD14)

A rejected reload candidate is discarded and the active configuration keeps
serving. A configuration-diagnostic rejection is level `warn`, event
`config_reload_rejected`, operation `reload`, outcome `failure`; it uses the
same first-record `config` attribution and configuration error classification
as startup validation. A `restart_required` candidate remains rejected without
invented source positions.

### AD13 lifecycle classification

| Event | Level | Operation | Outcome | `error.code` | Kind | Retryable | Remediation | Static message |
|---|---|---|---|---|---|---:|---|---|
| `startup_snapshot_rejected` | `warn` | `startup` | `degraded` | `snapshot_unavailable` | `resource` | `true` | `verify the durable snapshot and storage permissions, then retry` | `startup snapshot unavailable` |
| `prompt_publication_failed` | `warn` | `publish` | `failure` | `internal_error` | `internal` | `false` | `inspect the bridge logs and configuration` | `prompt publication failed` |
| `reload_rejected` | `warn` | `reload` | `failure` | `internal_error` | `internal` | `false` | `inspect the bridge logs and configuration` | `configuration reload rejected` |
| `reload_publication_failed` | `warn` | `publish` | `failure` | `internal_error` | `internal` | `false` | `inspect the bridge logs and configuration` | `configuration reload publication failed` |
| `internal_warning` | `warn` | `reload` | `failure` | `internal_error` | `internal` | `false` | `inspect the bridge logs and configuration` | `bridge encountered an internal warning` |

All these messages are fixed event messages; raw error strings are dropped. A
publication failure after accepting a reload has exactly one emitter,
`operatorLog`; startup publication failure is a separate `startup_publication_failed`
record and is emitted once. No legacy fallback record is emitted. An
`internal_warning` indicates an unclassified lifecycle producer and is a defect
to report against this table, not a normal operator action.

### AD7 validation-success record

Configuration validation success is a distinct `config_validation_succeeded`
record: level `info`, operation `check_config`, outcome `success`, message
`configuration validated`, no `error` object and no configuration contents. It
is written to stdout and exits 0. Configuration failure exits 1.

## R8 event-specific additions: refresh and logger events (AD16–AD21)

Every r8 addition below is an optional event-specific addition under the
existing evolution rule: none removes, renames, retypes, or changes the meaning
of an existing field, so there is no schema-version change and
`schema_version` remains 1.

| Event | Operation | Event-specific fields | Schema evolution |
|---|---|---|---|
| `catalog_refresh_started` | `refresh` | `refresh_id`, `config_revision`, `trigger`, `next_attempt_ms` | optional event-specific addition; no schema-version change (`schema_version` remains 1) |
| `catalog_refresh_completed` | `refresh` | `refresh_id`, `config_revision`, `trigger`, `servers_discovered`, `tools_discovered`, `tools_published`, `pins_resolved`, `pins_total` | optional event-specific addition; no schema-version change (`schema_version` remains 1) |
| `catalog_refresh_failed` | `refresh` | `refresh_id`, `config_revision`, `trigger`, optional `next_attempt_ms`, `catalog_state`, `retained_generation`, `catalog_tools`, `retained_age_ms`, `candidate_servers`, `candidate_tools`, `publication_impact`, `pins_resolved`, `pins_total`, `error` | optional event-specific addition; no schema-version change (`schema_version` remains 1) |
| `upstream_discovery_failed` | `refresh` | `server`, `failure_stage`, `failure_reason`, applicable `http_status`, sanitized `endpoint`, `error`, scheduled `next_attempt_ms` | optional event-specific addition; no schema-version change (`schema_version` remains 1) |
| `pin_blocker_detail` | `refresh` | per-pin: `refresh_id`, `server`, `config.pin_tool`, `config.property_path`, `blocker_class`, applicable `failure_reason`, `publication_impact`; tool-list chunk: `refresh_id`, `server`, `blocker_class`, `discovered_tools`, `chunk_index`, `chunk_count`, `publication_impact` | optional event-specific addition; no schema-version change (`schema_version` remains 1) |
| `config_reload_applied` | `reload` | `config_revision`, `trigger` | optional event-specific addition; no schema-version change (`schema_version` remains 1) |
| `logging_write_failed` | `logging` | none beyond the common envelope | optional event-specific addition; no schema-version change (`schema_version` remains 1) |

The three `catalog_refresh_*` events describe one pass: `started` includes the
correlation ID, configuration revision, trigger, and next-attempt interval;
`completed` reports only aggregate discovery/publication and pin counts, never
per-server or per-tool names. The single `catalog_refresh_failed` pass record
uses `warn` while a retained catalog with tools remains available and `error`
when no upstream tools are available. Its `catalog_state` is
`retained_published` or `none_available`; retained-generation, tool-count, and
age fields are present only for a retained catalog. Candidate and pin counts,
publication impact, and the finite nested error classification identify the
rejected or degraded pass. `next_attempt_ms` appears only when an automatic
retry is scheduled.

`upstream_discovery_failed` attributes a failed server probe with a finite
`failure_stage` and `failure_reason`, a sanitized endpoint, and a nested finite
error classification. `http_status` appears when applicable and
`next_attempt_ms` only when an automatic retry is scheduled. `pin_blocker_detail`
emits one record per blocking pin; when tool-list details are present, it emits
bounded per-server chunks with at most 64 validated tool names per record and
zero-based `chunk_index`/`chunk_count`. A discovery-failed server is classified
as `server_unverified`, not `tool_absent`.

`config_reload_applied` records the successful reload's `config_revision` and
`trigger`. `logging_write_failed` is the fixed, credential-free fallback
record: it carries no fields beyond the common envelope and is written as one
JSON line to the severity lane opposite the failed record's lane.

### Loki query examples for refresh diagnostics

The rejected-pass query uses `event` and the nested error classification; the
pin query finds blocker records by event and blocker class. Neither uses message
text.

<!-- query-refresh-rejections -->
```logql
sum by (error_code, error_kind) (count_over_time({service="mcp-bridge"} | json event="event", error_code="error.code", error_kind="error.kind" | event="catalog_refresh_failed" [5m]))
```

<!-- query-unresolved-pins -->
```logql
sum by (blocker_class) (count_over_time({service="mcp-bridge"} | json event="event", blocker_class="blocker_class" | event="pin_blocker_detail" [5m]))
```

All added names extend the field union below. Reused fields such as
`failure_stage`, `failure_reason`, `http_status`, `server`, `catalog_tools`,
and `config.*` retain their existing meanings.


## Embedding environment references

`embedding.model` and `embedding.base_url` accept either a literal YAML string
or an unquoted one-key `{env: NAME}` reference. These two fields do not accept a
file reference. References are resolved against the bridge process environment
at config load time; a reload reuses that process environment and cannot import
changed host `.env` or Compose settings. Resolved embedding identity is
restart-only. An unset and an exactly empty variable have distinct, value-free
configuration diagnostics. Values are preserved as supplied; `base_url` must
be an absolute HTTP(S) URL and invalid whitespace/control bytes are rejected.
No resolved value is included in a log record.

### Collector-owned fields

The collector supplies `timestamp`, `environment`, and `version`, plus host,
subsystem, container identity, and other deployment enrichment. These are not
application fields: the bridge emits no timestamp and does not emit collector-
owned identity. Enrichment happens after stdout/stderr reaches the site
collector. In Loki, use only bounded `service` and `level` as labels; query
`event`, `operation`, `error.code`, and `error.kind` as fields. Keep
operator-supplied `config.path` out of label indexes. The deployment's
shipping pipeline is outside this repository, so local verification proves
emitted lines and stream selection, not collector delivery.

Removed, renamed, retyped, or semantically changed fields increment `schema_version`;
optional event-specific additions do not. The current schema starts at version 1.
The Docker Compose `mcp-bridge |` prefix is orchestrator output; the bridge does
not work around it. Panics retain Go's normal crash behavior; this contract adds
no panic recovery or raw-cause diagnostic mode.

## Loki query examples

These examples parse JSON and filter on fields in the application vocabulary;
`error_code` and `error_kind` are local parser aliases sourced from the nested
`error.code` and `error.kind` values. The messages are never query keys.

<!-- alert-debug -->
```logql
sum(count_over_time({service="mcp-bridge", level="debug"} | json [5m]))
```

<!-- alert-info -->
```logql
sum(count_over_time({service="mcp-bridge", level="info"} | json event="event" | event="config_validation_succeeded" [5m]))
```

<!-- alert-warn -->
```logql
sum(count_over_time({service="mcp-bridge", level="warn"} | json event="event", error_code="error.code" | event="config_reload_rejected" [5m]))
```

<!-- alert-error -->
```logql
sum(count_over_time({service="mcp-bridge", level="error"} | json event="event", error_code="error.code", error_kind="error.kind" | event="config_validation_failed" | error_code="pin_ceiling" | error_kind="configuration" [5m]))
```
