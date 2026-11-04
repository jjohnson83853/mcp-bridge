# mcp-bridge

A headless bridge that fronts several MCP upstream servers behind exactly one
Streamable HTTP endpoint, `/mcp`. It discovers each configured server's tools
and prompts, serves them under stable server-derived names, answers tool
discovery through hybrid keyword/vector search, and forwards tool executions and
prompt reads to the owning upstream with provenance attached.

There is no namespace layer, no per-server route, and no admin UI: server ID is
the provenance, and membership is the flat `servers:` list of one YAML document.

## Build and run

```sh
go build ./cmd/mcp-bridge

# validate a document locally (no connectivity checks)
./mcp-bridge --check-config --config configs/config.example.yaml

# serve
./mcp-bridge --config configs/config.example.yaml
```

`--config` defaults to `configs/config.yaml`. `SIGHUP` reloads it; `SIGINT`/
`SIGTERM` drains and exits.

The sample names the credentials it needs instead of carrying them: `{env: NAME}`
reads one environment variable, and `{file: PATH}` reads a regular, non-symlink,
operator-owned file that is not group- or world-readable (a relative `PATH`
resolves against the document's directory). `--check-config` resolves those
references locally — never connecting anywhere — so it succeeds only once they
resolve, and it never prints a secret value, only the reference names. For
example, with the sample unchanged:

```sh
mkdir -p configs/secrets
printf '%s\n' "$(openssl rand -hex 32)" > configs/secrets/developer-api-key
chmod 600 configs/secrets/developer-api-key
BRIDGE_EMBEDDING_KEY=... BRIDGE_CLIENT_TOKEN=... \
DEVELOPER_UPSTREAM_TOKEN=... PORTAINER_UPSTREAM_TOKEN=... \
  ./mcp-bridge --check-config --config configs/config.example.yaml
```

## Authentication (OAuth 2.1 via Pocket ID & RFC 9728)

The bridge enforces OAuth 2.1 Bearer authentication via our Pocket ID identity provider in accordance with the Model Context Protocol specification and RFC 9728 (OAuth 2.0 Protected Resource Metadata).

### Protected Resource Metadata (RFC 9728)

The bridge serves unauthenticated metadata at:

```http
GET /.well-known/oauth-protected-resource
```

Returning HTTP 200 with JSON:

```json
{
  "resource": "https://mcp.bullprog.com",
  "authorization_servers": ["https://id.bullprog.com"],
  "scopes_supported": ["openid", "email", "profile", "mcp:connect"],
  "bearer_methods_supported": ["header"]
}
```

Standard MCP clients (Claude Desktop, LibreChat, and custom agent runtimes) query this endpoint to automatically discover the required authorization server and supported scopes.

### Pocket ID Identity Provider Setup

To configure Pocket ID for this bridge:

1. **Create an Application / API Resource**:
   - In your Pocket ID admin console, register a new client application for the MCP bridge.
   - Set the **Resource Identifier / Audience** to match `MCP_RESOURCE_URL` (default: `https://mcp.bullprog.com`).
2. **Configure Scopes**:
   - Ensure the application permits scopes: `openid`, `email`, `profile`, and `mcp:connect`.
3. **Grant Types & Client Authentication**:
   - For interactive user agents (e.g. desktop clients), configure Authorization Code grant with PKCE.
   - For automated headless agents, configure Client Credentials grant.

### Environment Variables & Defaults

All OAuth 2.1 endpoints and validation settings are configurable via environment variables with safe defaults:

| Variable | Default Value | Description |
|---|---|---|
| `POCKET_ID_ISSUER` | `https://id.bullprog.com` | Base URL of the Pocket ID instance. The token's `iss` claim must match this exactly. |
| `POCKET_ID_JWKS_URL` | `https://id.bullprog.com/.well-known/jwks.json` | Remote JSON Web Key Set endpoint. The bridge caches keys and auto-refreshes them in the background. |
| `MCP_RESOURCE_URL` | `https://mcp.bullprog.com` | Canonical resource identifier for this bridge. The token's `aud` claim must equal or contain this URL (trailing slashes normalized). |
| `MCP_REQUIRED_SCOPE` | `""` | Optional required scope (e.g. `mcp:connect`). When configured, tokens lacking this scope receive HTTP 403 Forbidden. |

### Running with Pocket ID

Pass the environment variables directly:

```sh
POCKET_ID_ISSUER="https://id.bullprog.com" \
POCKET_ID_JWKS_URL="https://id.bullprog.com/.well-known/jwks.json" \
MCP_RESOURCE_URL="https://mcp.bullprog.com" \
MCP_REQUIRED_SCOPE="mcp:connect" \
./mcp-bridge --config configs/config.yaml
```

Or in Docker Compose:

```yaml
services:
  mcp-bridge:
    image: mcp-bridge:latest
    environment:
      POCKET_ID_ISSUER: "https://id.bullprog.com"
      POCKET_ID_JWKS_URL: "https://id.bullprog.com/.well-known/jwks.json"
      MCP_RESOURCE_URL: "https://mcp.bullprog.com"
      MCP_REQUIRED_SCOPE: "mcp:connect"
```

### Protocol Challenges & Error Responses

- **Missing, invalid, or expired token**:
  - Responds with `HTTP 401 Unauthorized`
  - Challenge header:
    ```http
    WWW-Authenticate: Bearer realm="mcp", resource_metadata="https://mcp.bullprog.com/.well-known/oauth-protected-resource"
    ```
- **Token missing required scope (`MCP_REQUIRED_SCOPE`)**:
  - Responds with `HTTP 403 Forbidden`
  - Challenge header:
    ```http
    WWW-Authenticate: Bearer error="insufficient_scope", scope="mcp:connect"
    ```
- **Clock Skew Tolerance**:
  - A 60-second clock skew grace period is permitted on `exp` validation to absorb NTP drift between hosts.
- **Context Identity**:
  - Validated claims (`sub`, `email`, etc.) are injected into the request context for downstream tool provenance and audit logs.

### Dual-Mode & Health Check Bypass

- **Dual-Mode Fallback**: Non-JWT opaque tokens continue to be verified against the static tokens configured in `client_auth.bearer_tokens` (YAML document), preserving compatibility for legacy scripts and integration tests.
- **Bypassed Endpoints**: `/.well-known/oauth-protected-resource`, `/livez`, `/readyz`, and `/healthz` bypass authentication without credentials.

## Configuration

`configs/config.example.yaml` is a complete document. The shape is flattened —
top-level `servers:` plus top-level `pins:`:

```yaml
schema_version: 1
http: { ... }            # one inbound listener, host/origin allowlists, bounds
operations: { ... }      # liveness/readiness/metrics listener + its rate limit
storage: { ... }         # private state directory and snapshot ceiling
catalog: { ... }         # refresh interval, staleness horizon, catalog ceilings
search: { ... }          # top-k bounds, hybrid weights, query cache, timeouts
embedding: { ... }       # OpenAI-compatible provider and model identity
client_auth: { ... }     # optional static bearer tokens (dual-mode fallback)
upstream_defaults: { ... } # connect/call/refresh limits, breaker
servers:
  - id: developer-api
    endpoint: "https://developer-upstream.invalid/mcp"
    prompts_disabled: false  # optional per-server prompt opt-out (default false)
    credentials:
      bearer: {env: DEVELOPER_UPSTREAM_TOKEN}
      headers:
        X-API-Key: {file: "/run/secrets/developer-api-key"}
pins: []
```

Rules that matter operationally:

- **Secrets are references only** (`{env: NAME}` or `{file: PATH}`), never
  literals. A file reference must be a regular, operator-owned, non-symlink file
  at mode `0600`-class privacy; the value is never logged, persisted, or echoed.
- **Restart-only settings** are bound once and cannot be changed by a reload:
  the two listeners, the state directory, the snapshot ceiling, the refresh
  interval, the operations rate limit, and the whole `search:`/`embedding:`
  identity. A reload that changes any of them is rejected as a whole.
- **Endpoint identity is permanent**: a server ID keeps its normalized endpoint
  fingerprint. Pointing an existing ID at a different endpoint is rejected;
  rotating that endpoint's credentials is allowed.
- `pins:` names exact `[server_id, raw_name]` pairs that must stay discoverable.

### Embedding model and base URL from the environment

`embedding.model` and `embedding.base_url` accept either a literal YAML string
or a native, unquoted environment reference — the same one-key mapping syntax
the secret fields already use:

```yaml
embedding:
  model: {env: EMBEDDINGS_LLM_MODEL}       # unquoted mapping: resolves the variable
  base_url: {env: EMBEDDINGS_LLM_BASE_URL} # unquoted mapping: resolves the variable
  api_key: {env: LITELLM_API_KEY}          # existing reference grammar, unchanged
```

The mapping is the intended syntax, not text to be repaired: there is no
autoquoting, no `${...}`/`!env` interpolation, no templating or preprocessing
step, and no fallback evaluation. Quoting changes the meaning —
`model: "{env: EMBEDDINGS_LLM_MODEL}"` is the literal string `{env:
EMBEDDINGS_LLM_MODEL}`, never a reference. A quoted map-looking string in
`base_url` therefore fails ordinary URL validation (it is not a URL), while a
quoted map-looking `model` is accepted as that literal model name. Plain legal
literals (`model: nomic-embed-text`, a plain URL) never need quotes; quote only
when YAML syntax itself requires it. These two fields take `{env: NAME}` only —
no `{file: PATH}` and no other key. `api_key` keeps its existing
`{env: NAME}`/`{file: PATH}` rules.

Resolved values are captured per config load and byte-preserved: no trimming,
no newline stripping, no re-quoting. A nonempty model that contains spaces,
quotes, `${...}` or an embedded newline is used exactly as-is (existing model
semantics); `base_url` must be an absolute HTTP(S) URL and rejects ASCII
whitespace/control characters, including CR/LF, as `url_invalid` rather than
trimming them. An unset variable and a variable set to exactly the empty string
fail startup (or the whole reload candidate) with distinct, value-free
diagnostics on the named field — `code=reference_unresolved` with shape
`missing` vs `empty` and distinct static reasons, never the value.

Because the raw reference (not the resolved value) feeds the public config
fingerprint, rotating only the variable's value leaves the fingerprint stable
— but the effective embedding identity changes, so a restart rebuilds or
rejects stale vectors as usual. Changing the *reference itself* (literal ↔
mapping, or a different variable name) changes the fingerprint, as expected.

### Docker: environment layering

Host `.env` values and Compose `${...}` substitution only affect the Compose
file itself — they do not by themselves put variables in the container. Each
`{env: NAME}` reference in the mounted document must be satisfied by the
container's own process environment, so pass them explicitly under the
service's `environment:` (or an `env_file`):

```yaml
    # Documentation only — docker-compose.yaml is unchanged; adapt to your service.
    environment:
      EMBEDDINGS_LLM_MODEL: ${EMBEDDINGS_LLM_MODEL:?set it in ./.env}
      EMBEDDINGS_LLM_BASE_URL: ${EMBEDDINGS_LLM_BASE_URL:?set it in ./.env}
      LITELLM_API_KEY: ${LITELLM_API_KEY:?set it in ./.env}
```

The `:?` guard rejects variables that are unset or empty; a whitespace-only
value passes it (the bridge's own validation still applies downstream).

`SIGHUP` re-reads the mounted document against the *existing* process
environment. It cannot import a changed host `.env` or a changed Compose
`environment:` block. Resolved embedding settings are restart-only: to apply a
changed variable value or a changed Compose environment, recreate the container
with the new environment (e.g. `docker compose up -d --force-recreate
mcp-bridge`); restarting the old container without it does not acquire the
change, and a reload that sees different effective embedding settings is
rejected as `restart_required` with the live runtime unchanged. Failure
diagnostics surface through the existing local channels only — startup stderr,
`--check-config` output, and reload JSON/metric records — with no promise of
external log-shipping ingestion.

## Configuration diagnostics

Every rejected document is attributed as one typed, single-line record: the
supplied config path, the complete known location, the owner and reference
identity, a finite code/property/shape class, and a static reason. Records
never contain a secret value, a preview, a hash, a fingerprint, an exact
length, an offset, or raw YAML/URL/OS text — a multi-failure collection is
pipe-separated and bounded:

```text
invalid configuration: diagnostics=5: config="configs/config.example.yaml" key="embedding.api_key" ref="env:BRIDGE_EMBEDDING_KEY" code=secret_unset property=value shape=unset: secret environment variable is unset; set the named reference without printing its value | ... | config="configs/config.example.yaml" key="servers[1].credentials.bearer" server="portainer-api" ref="env:PORTAINER_UPSTREAM_TOKEN" code=secret_unset property=value shape=unset: secret environment variable is unset; set the named reference without printing its value; remaining_stages=not_checked
```

Safe repair path: edit the file named by `config=` at the `key=` location named
by the record. The code is a closed vocabulary (`secret_unset`, `secret_empty`,
`secret_too_large`, `secret_cr`/`secret_lf`/`secret_nul`, `secret_file_missing`,
`secret_file_policy`, `bound_range`, `duration_*`, `url_invalid`, …) and the
reason is a static instruction; neither ever echoes the value. `ref=` names the
configured reference identity (`env:NAME` or `file:PATH` as written); the
config-dir resolution of a relative file path stays internal and is never
rendered.

Channels:

- `--check-config`: rejections print `invalid configuration: <records>` to
  stderr with stdout empty and exit status 1; a valid document keeps the
  existing `configuration valid: <summary>` line on stdout. This is the
  intended validation step before `docker compose up`, and it can also run
  inside the deployed image:
  `docker compose run --rm mcp-bridge --check-config --config /app/configs/config.mcp-grafana-stack.yaml`.
  In Compose the same record surfaces as a container log line prefixed
  `mcp-bridge |`.
- Serve startup: rejections keep the existing `mcp-bridge: configuration:`
  stderr prefix followed by the same records; stdout shape is unchanged.
- Reload: a rejected candidate is reported as an operator-log JSON entry
  (`event: reload`, `outcome: failure`) carrying the typed manifest —
  `config_path`, `location`, `line`, `column`, `code`, `property`, `shape`,
  `ref_kind`, `ref_name`, `pin_server`, `pin_tool`, `related_location`,
  `related_ref_kind`, `related_ref_name`, `diagnostic_index`,
  `diagnostics_incomplete`, `remaining_stages`, `server`. Disallowed metadata
  (`config_key`, `secret_ref`, header/URL/endpoint identities, freeform error
  text) is dropped by the logger allowlist and never reflected; the unrelated
  credential redaction stays intact.

Line-ending policy: file secret values must be UTF-8 with no CR, LF, or NUL.
Reads strip exactly one final LF, so a file with a single trailing newline is
fine; a second trailing LF is a `secret_lf` rejection. Environment values are
never newline-stripped.

A collection reports at most 32 records. Beyond that the collection is marked
incomplete (`diagnostics_incomplete: true` in the logger manifest, footer once
on the rendered line), and a secret-stage failure skips the remaining stages
(`remaining_stages: not_checked`) rather than guessing what they would have
reported.

## Tool surface

One MCP server, three tools:

| Tool | Purpose |
| --- | --- |
| `search_tools` | hybrid keyword/vector search over the current catalogs, optionally narrowed with `servers: []` |
| `describe_tool` | exact schema for one tool name, with validation status |
| `call_tool` | one forwarded execution, carrying the expected `catalog_revision` |

All three declare standard MCP tool annotations (advisory hints — never
authorization and never a promise of zero confirmation prompts):

| Tool | `readOnlyHint` | `destructiveHint` | `idempotentHint` | `openWorldHint` |
| --- | --- | --- | --- | --- |
| `search_tools` | true | unset | true | true |
| `describe_tool` | true | unset | true | true |
| `call_tool` | false | true | false | true |

`search_tools`/`describe_tool` are read-only, idempotent discovery over the
bridge's own catalog and dispatch nothing upstream. The generic `call_tool`
forwards execution to an arbitrary upstream tool: it may mutate, may be
destructive, is not idempotent, and interacts with the open world of configured
upstream servers. The annotations carry no guarantee that a client will skip
its permission prompts — clients decide independently — and they never grant
execution authority or bypass any policy.

Names are server-derived and stable: `t_` plus unpadded lowercase base32 of
`SHA-256([server_id, raw_name])` (RFC 4648 §6). A schema change moves the catalog
revision without moving the public name.

The catalog revision is **global**: it hashes the sorted bound server identities
and their exact definitions (schemas compared whitespace-insensitively), plus
each definition's upstream metadata (title, icons, annotations, `_meta`). A
metadata-only upstream change therefore moves the revision without moving the
public name. Search
returns it with every result, and `describe_tool`/`call_tool` require it. A
mismatch — including one caused by an unrelated server's definition changing — is
`catalog_changed` with zero dispatch; refresh explicitly rather than relying on
implicit rerouting.

### Server metadata defaults

Any configured server may own an optional `tool_metadata` mapping whose
`defaults` patch is applied to **that server's configured pins only**:

```yaml
servers:
  - id: alpha
    endpoint: https://alpha.internal/mcp
    tool_metadata:
      defaults:
        title: Corrected read
        description: Advisory read description
        annotations:
          readOnlyHint: true
          destructiveHint: false
        icons:
          - {source: "https://example.internal/read.svg", mime_type: "image/svg+xml"}
        _meta:
          policy: strict
```

The supported patch members are `description`, `title`, `icons`,
`annotations` (the SDK annotation inventory: `title` plus
`readOnlyHint`/`destructiveHint`/`idempotentHint`/`openWorldHint`) and
`_meta`. Object members merge recursively; arrays and scalars replace
outright; explicit `false`, empty members and nested `null` values are
retained as values, never treated as deletions. Unknown members, wrong value
shapes and duplicate mapping keys reject the whole candidate with a typed,
redacted configuration diagnostic before anything is published.

Defaults change the public configuration fingerprint, so a config-only edit
reloads; they never change credential identity. Effective values ride the
same publication path as any definition change: `tools/list` serves the
effective descriptor for pinned tools, and `describe_tool` reports the
source view as `upstream_metadata`, the effective view as `metadata`, and the
winning field paths with `layer: "server_default"` in
`metadata_override_fields` (an empty list means no correction). Only source
metadata is persisted; corrections never become upstream truth, and removing
a defaults member (or the whole `tool_metadata` block) on reload reverts the
same session to the current source values.

Defaults apply to the owning server's pins and to that server's unpinned tools
when progressively revealed through `search_tools` or `describe_tool`; their
winning paths report `layer: "server_default"`. Another server sharing the
same raw tool name, the bridge built-ins and generic `call_tool` are
unaffected. A server with no pins may still declare defaults; they apply if
an unpinned tool from that server is revealed. **Risk:** a server-wide
assertion such as `readOnlyHint: true` misclassifies every pinned tool of a
mixed read/write server, and clients may suppress their own warnings on the
strength of it. Defaults are advisory claims, not authorization, and a
blanket read-only default is never installed automatically.

### Exact-tool exceptions

A server's `tool_metadata` mapping may also carry a `tools` mapping of exact
raw tool names to patches. Each entry is layered **above** that server's
defaults for that one pinned tool only — precedence is upstream source →
server defaults → exact-tool exception, highest wins:

```yaml
    tool_metadata:
      defaults:
        annotations:
          readOnlyHint: true
      tools:
        write:
          annotations:
            readOnlyHint: false
            destructiveHint: true
            idempotentHint: false
```

There are no wildcard or cross-server selectors: a `tools` entry may name any
valid raw tool on the same server, including an unpinned one. An unpinned
entry is accepted without auto-revealing the tool; when that tool is
progressively discovered, its patch is layered above defaults and winning
paths report `layer: "tool"`. Unknown tool names reject the whole candidate
with a typed, redacted diagnostic (`tool_metadata_tool_unknown`), as do
invalid tool names, unsupported patch members and duplicate entries. The
shared merge semantics
apply to exception patches exactly as to defaults: object members merge
recursively, arrays and scalars replace outright, and explicit `false`, empty
members and nested `null` values are retained. A path both layers touch
records the exception as the winner; `describe_tool` provenance then reports
that path with `layer: "tool"` while paths won only by the defaults keep
`layer: "server_default"`.

Exceptions change the public configuration fingerprint (never the credential
identity) and publish through the same reload path. Removing a member or the
entire tool patch on reload reverts that tool to the current defaults (or, if
no defaults remain, to the source values); removing the defaults while a
tool patch remains keeps the patch in force, and upstream source changes
never overwrite an explicit patch. Corrections are never persisted as
upstream truth. **Risk:** the same mixed-service caveat applies — an
exception is a per-tool advisory claim, not authorization, and overriding a
read-only default back to `false` for a write tool restores honest client
warnings, while the inverse direction can suppress them.


### Progressive metadata reveal and session projections

New sessions initially expose exactly the three bridge built-ins
(`search_tools`, `describe_tool`, and `call_tool`) plus configured pins.

Successful `search_tools` and `describe_tool` discovery commits discovered
unpinned tools to that session's projection before returning success, so an
immediate `tools/list` re-list or direct call in the same session succeeds.
Projections are session-local: a tool discovered in session A is not visible
in session B, even when both sessions use the same bearer token.

`tools/list` supports pagination with a default page size of 1000 or a
configured positive page size. Its URL-safe base64 cursors are 57 bytes and
are signed with process-transient HMAC-SHA256. Stale or tampered cursors,
including those used after a catalog reload or process restart, return JSON-RPC
error -32602 (`invalid cursor`).

HTTP `DELETE /mcp` or idle timeout retires the session projection and its
union references. In-flight direct-call leases survive `DELETE` until
execution completes.

Metadata annotations are advisory claims, not authorization. Clients receive
best-effort asynchronous `tools/list_changed` notifications and must
explicitly re-list; no zero-prompt behavior is promised for any client
(Gemini CLI 0.18.13.x is user-reported).

Executions return the upstream result unaltered inside a provenance envelope:
`structuredContent.upstream_result` is the complete original `CallToolResult`
(content blocks, structured content, `isError`, metadata) and the outer envelope
adds `server`, `catalog_revision`, `catalog_status`, `availability`, `tool_name`,
and `request_id`.

## Prompt surface

Every configured server's native prompts are passed through the same endpoint.
The bridge serves `prompts/list`, `prompts/get`, and `prompts/list_changed`; it
does not synthesize prompts, translate arguments, or fan a read out.

Public prompt names are as opaque and stable as tool names: `p_` plus unpadded
lowercase base32 of `SHA-256([server_id, raw_name])`. Two upstreams may publish
the same raw prompt name and the public names still differ; a `prompts/get`
reaches exactly one owning upstream and is never retried against its sibling.
Arguments are forwarded unchanged.

A served prompt describes its origin in its description (`… (via <server id>)`,
or just `via <server id>` when the upstream description is empty). The raw
upstream name stays internal, and the description is provenance for humans only
— routing never depends on it. Failures are sanitized: an upstream error never
reaches the client verbatim, a failing prompt is reported as `prompt
unavailable`, and an unknown public name is JSON-RPC `invalid params`.

Prompts are opt-out per server: everything is discovered unless that server sets
`prompts_disabled: true`, which excludes it from prompt discovery and exposure
without touching its tools. The key is reload-safe.

Published prompts survive a restart. The state directory keeps `prompts.json`
beside the tool snapshot, a server that cannot be reached keeps its complete
last-known-good prompts and is marked stale exactly like its tools, and a
prompt-only change never moves the tool catalog revision, so a client holding a
current revision keeps using it. `GET /metrics` exposes the published count as
the unlabelled `bridge_catalog_prompts` gauge.

## Operations

On the `operations.listen` address:

- `GET /livez` — the process event loop only.
- `GET /readyz` — per-server readiness (ready / degraded / unavailable) plus the
  aggregate.
- `GET /metrics` — Prometheus exposition, rate limited by
  `operations.requests_per_second`/`burst`.

Metric labels stay inside a finite vocabulary: `server` is always a configured
ID (or the reserved `_bridge` for work the bridge does for itself), never a tool
name, query, URL, or token.

### HTTP request diagnostics

Every HTTP request reaching the inbound listener produces one terminal
structured JSON record (`event: http_complete`) on standard output. The
completion is owned by one private outer observer composed around the entire
public mux, so it also covers early OAuth rejections, metadata and health
responses, and unmatched routes — not only the protected MCP path. It is
attempted on normal handler return or panic unwind. One terminal write is
attempted per request; abrupt process loss or a broken/short stdout write can
lose the record, and broken sinks do not self-report:

- **Base envelope:** `time` (UTC RFC3339Nano), `level` (`info` for successes,
  `warn` for rejections/client errors, `error` for internal errors), `service`
  (`mcp-bridge`), `environment` (`unspecified`), `version` (`unspecified`),
  and fixed `message: "http request completed"`. Actual typed tool errors
  under an observed 200 promote the HTTP completion to `warn`, and an
  unavailable response promotes `info` to `warn`; existing `warn`/`error`
  severities are never downgraded and no duplicate failure stream exists.
- **Transport fields:** `http_method` (`GET`, `POST`, `DELETE`, `HEAD`,
  `OPTIONS`, `PUT`, `PATCH`, `OTHER`), `route` (`mcp`, `not_found`),
  `operation` (inbound method/transport classification: `initialize`,
  `initialized`, `tools_list`, `tools_call`, `resources_list`, `resources_read`,
  `ping`, `prompts_list`, `prompts_get`, `sse_stream`, or fallback `request`),
  `client_family` (closed client classification derived by strict priority substring
  match from `User-Agent`: `openai`, `claude`, `cursor`, `omp`, `browser`, `other`,
  or `absent`; raw User-Agent headers are never logged),
  `http_status` (observed integer `100`–`599`, or explicit JSON `null` when no
  response evidence exists), `http_outcome` (`success`, `failure`, `incomplete`,
  `unknown`), `http_bytes` (exact response-body byte count written, including
  `0` for zero-byte writes and observed body bytes before an error on partial writes),
  `http_bytes_bucket` (`empty`, `le_1k`, `le_16k`, `le_256k`, `le_1m`, `gt_1m`,
  `unavailable`), `exposed_tools` (exact count of exposed tools returned on
  successful `tools/list` response pages; omitted on write failures, list errors,
  and non-`tools/list` operations), and `duration_ms` (signed elapsed
  milliseconds from outer-owner entry — before routing, OAuth evaluation, and
  correlation-ID generation — through handler return/unwind; this is wider
  than the former inner-handler interval). A separate bounded
  `diag_duration_ms` field (`0`–`86400000`, with `duration_capped: true` when
  clamped, including negative clock deltas clamped to `0`) measures the same
  interval within the semantic bounds. Inbound request counters
  (`bridge_requests_total`) partition requests by this operation enum. Raw
  paths, queries, and arbitrary verbs are never reflected.
- **Internal correlation:** `request_id` is an internal lowercase 32-hex ID
  minted independently of caller-supplied identifiers; `correlation_state`
  reports `available` or `unavailable`. The same owned ID is returned to the
  caller as a `Mcp-Diagnostic-Id` response header, installed before routing
  and any write — including 401/403 responses and metadata/health when
  available. Callers cannot choose this value; caller-supplied public,
  internal, or diagnostic headers never supply identity. When request entropy
  fails, `correlation_state` becomes `unavailable`, `request_id` is absent,
  and the diagnostic header is omitted — no caller/time/counter/hash
  fallback or retry. Public `X-Request-ID` response behavior remains unchanged
  where the inner handler already echoes it, including on request-entropy
  failure; it is never an internal logging identity or trusted diagnostic join.
- **Finite admission stages and reasons:**
  - `client_auth` / `client_auth_failed`: missing or invalid client Bearer token.
  - `session_admission` / `session_auth_failed`: an absent session entry or
    existing bearer-binding mismatch; returns 404 with no `Bearer` challenge
    and no session ID, so clients treat the session as lost/unusable,
    reinitialize, and retry. Omitting the session header alone is
    not an owned rejection, and fresh initialization can be legal.
  - `session_admission` / `session_capacity`: active sessions exceed `max_sessions`.
  - `host_admission` / `host_rejected`: Host header not allowed by `allowed_hosts`.
  - `origin_admission` / `origin_rejected`: Origin header not allowed by
    `allowed_origins` (Host rejection decisive before Origin).
  - `request_limits` / `request_too_large`: body exceeds `max_request_body_bytes`
    (returns 413 with native transport close-on-limit).
  - `http_route` / `route_not_found`: unmatched path or unusual verb.
- **Outer OAuth rejection reasons (typed):** Early OAuth failures are mapped
  to authoritative finite reasons — missing credentials to
  `credentials_missing`, a wrong Bearer scheme to `bearer_malformed`,
  explicitly unusable keyset to `jwt_key_unavailable`, explicit verified
  audience denial to `jwt_audience_invalid`, and scope denial to
  `scope_insufficient` (403 with its existing challenge). Parser failures,
  including those without an authoritative specific typed classification, use
  `jwt_invalid_other`; a generic JWT outcome proves neither expiry nor a
  particular signature/key failure. Any
  outer denial records the session as `not_evaluated` and the upstream as
  `not_attempted`. The first actual decisive rejection is immutable — later
  operation or delivery evidence cannot overwrite it.
- **Outer rejection examples:** The rows below are synthetic, abbreviated
  (core envelope fields such as `time`, `message`, `service`, `environment`,
  `version`, and `generation` elided, along with conditional and
  always-present typed slots not shown, e.g. `duration_capped`, `path_class`,
  `route_outcome`, `status_state`, `challenge_*`, `process_*`) and
  illustrative only; the accepted
  severity policy is unchanged (`info` for accepted requests, `warn` for
  expected 4xx rejections, `error` for actual dependency/internal 5xx, `warn`
  when no response evidence is available). A rejected request has no
  downstream side effect: `session_state` stays `not_evaluated` and
  `upstream_state` stays `not_attempted`, and the existing status, challenge,
  and response body are unchanged by the diagnostics.

  Missing credentials (no Bearer token; 401 resource-metadata challenge):

  ```json
  {"event":"http_complete","level":"warn","operation":"unknown","outcome":"failure","reason":"unknown","stage":"unknown","failure_stage":"outer_oauth","failure_reason":"credentials_missing","http_status":401,"http_method":"POST","route":"mcp","http_outcome":"failure","duration_ms":3,"diag_duration_ms":1,"credential_state":"missing","scope_state":"not_evaluated","session_state":"not_evaluated","upstream_state":"not_attempted","auth_mode":"oauth","request_id":"0123456789abcdef0123456789abcdef","correlation_state":"available","diag_schema":"oauth_diag_v1"}
  ```

  Malformed credentials (wrong Bearer scheme; 401 challenge unchanged):

  ```json
  {"event":"http_complete","level":"warn","operation":"unknown","outcome":"failure","reason":"unknown","stage":"unknown","failure_stage":"outer_oauth","failure_reason":"bearer_malformed","http_status":401,"http_method":"POST","route":"mcp","http_outcome":"failure","duration_ms":2,"diag_duration_ms":1,"credential_state":"rejected","scope_state":"not_evaluated","session_state":"not_evaluated","upstream_state":"not_attempted","auth_mode":"oauth","request_id":"a1b2c3d4e5f60102030405060708090a","correlation_state":"available","diag_schema":"oauth_diag_v1"}
  ```

  Authoritative JWT failure classes (all 401; generic parser rejections stay
  `jwt_invalid_other`, which proves neither expiry nor a specific signature
  or key failure):

  ```json
  {"event":"http_complete","level":"warn","operation":"unknown","outcome":"failure","reason":"unknown","stage":"unknown","failure_stage":"outer_oauth","failure_reason":"jwt_key_unavailable","http_status":401,"http_method":"POST","route":"mcp","http_outcome":"failure","duration_ms":5,"diag_duration_ms":1,"credential_state":"rejected","scope_state":"not_evaluated","session_state":"not_evaluated","upstream_state":"not_attempted","auth_mode":"oauth","request_id":"b7e0f11a2233445566778899aabbccdd","correlation_state":"available","diag_schema":"oauth_diag_v1"}
  ```

  ```json
  {"event":"http_complete","level":"warn","operation":"unknown","outcome":"failure","reason":"unknown","stage":"unknown","failure_stage":"outer_oauth","failure_reason":"jwt_audience_invalid","http_status":401,"http_method":"POST","route":"mcp","http_outcome":"failure","duration_ms":4,"diag_duration_ms":1,"credential_state":"rejected","scope_state":"not_evaluated","session_state":"not_evaluated","upstream_state":"not_attempted","auth_mode":"oauth","request_id":"c9d8e7f6050403020110aabbccddeeff","correlation_state":"available","diag_schema":"oauth_diag_v1"}
  ```

  Scope denial (credentials accepted, scope rejected; 403 with the existing
  insufficient-scope challenge — no session or downstream evaluation):

  ```json
  {"event":"http_complete","level":"warn","operation":"unknown","outcome":"failure","reason":"unknown","stage":"unknown","failure_stage":"outer_oauth","failure_reason":"scope_insufficient","http_status":403,"http_method":"POST","route":"mcp","http_outcome":"failure","duration_ms":6,"diag_duration_ms":2,"credential_state":"accepted","scope_state":"rejected","session_state":"not_evaluated","upstream_state":"not_attempted","auth_mode":"oauth","request_id":"ddeeff00112233445566778899aabb01","correlation_state":"available","diag_schema":"oauth_diag_v1"}
  ```

- **Session outcomes (`failure_stage: session`, both 404 with no challenge,
  unchanged diagnostics).** The inner session admission distinguishes
  an absent session entry from an existing-entry bearer-binding mismatch —
  independently of the HTTP status:

  Missing/reaped entry: valid credentials plus a session header that has no
  stored entry (`session_absent`; the comparator never ran and the session
  reference was deleted with the session):

  ```json
  {"event":"http_complete","level":"warn","operation":"unknown","outcome":"failure","reason":"session_auth_failed","stage":"session_admission","failure_stage":"session","failure_reason":"session_absent","http_status":404,"http_method":"POST","route":"mcp","http_outcome":"failure","duration_ms":4,"diag_duration_ms":2,"credential_state":"accepted","scope_state":"accepted","session_header":"present","session_state":"absent","binding_relation":"not_evaluated","session_ref_state":"not_applicable","upstream_state":"not_attempted","auth_mode":"oauth","request_id":"11ff22ee33dd44cc55bb66aa7799cc00","correlation_state":"available","diag_schema":"oauth_diag_v1"}
  ```

  Existing entry, different bearer binding (comparator mismatch; the live
  `session_ref` appears in the server record only, never in any response):

  ```json
  {"event":"http_complete","level":"warn","operation":"unknown","outcome":"failure","reason":"session_auth_failed","stage":"session_admission","failure_stage":"session","failure_reason":"session_fingerprint_mismatch","http_status":404,"http_method":"POST","route":"mcp","http_outcome":"failure","duration_ms":3,"diag_duration_ms":1,"credential_state":"accepted","scope_state":"accepted","session_header":"present","session_state":"existing","binding_relation":"mismatch","session_ref_state":"available","session_ref":"7f3a9c1e552b40d6a8c1e93b7742d015","upstream_state":"not_attempted","auth_mode":"oauth","request_id":"aabb0011ccdd22ee33ff445566778899","correlation_state":"available","diag_schema":"oauth_diag_v1"}
  ```

  Successful session creation (initialize without a header; legal, never a
  rejection) and ordinary sessionless traffic:

  ```json
  {"event":"http_complete","level":"info","operation":"unknown","outcome":"success","reason":"none","stage":"none","failure_stage":"none","failure_reason":"none","http_status":200,"http_method":"POST","route":"mcp","http_outcome":"success","duration_ms":12,"diag_duration_ms":11,"credential_state":"accepted","scope_state":"accepted","session_header":"absent","session_state":"created","binding_relation":"not_evaluated","session_ref_state":"available","session_ref":"0d47a2c9b1e6f83d55a04c2e91b7f660","upstream_state":"not_attempted","auth_mode":"oauth","request_id":"5566778899aabbccddeeff0011223344","correlation_state":"available","diag_schema":"oauth_diag_v1"}
  ```

  A request that proceeds without any supplied or issued session records
  `session_state: not_supplied` with `session_ref_state: not_applicable`. A
  `session_capacity` rejection runs no session lookup or comparator, so its
  record reports `session_state: not_evaluated` and
  `binding_relation: not_evaluated`.

  Successful use of a retained session (`binding_relation: match`; the same
  reference the session was created with, in the server record only):

  ```json
  {"event":"http_complete","level":"info","operation":"unknown","outcome":"success","reason":"none","stage":"none","failure_stage":"none","failure_reason":"none","http_status":200,"http_method":"POST","route":"mcp","http_outcome":"success","duration_ms":9,"diag_duration_ms":8,"credential_state":"accepted","scope_state":"accepted","session_header":"present","session_state":"existing","binding_relation":"match","session_ref_state":"available","session_ref":"0d47a2c9b1e6f83d55a04c2e91b7f660","upstream_state":"not_attempted","auth_mode":"oauth","request_id":"8899aabbccddeeff0011223344556677","correlation_state":"available","diag_schema":"oauth_diag_v1"}
  ```

- **Session reference (`session_ref`) handling.** When an admitted live session
  is created, the server draws one independent cryptographically random
  128-bit reference (32 lowercase hex) and retains it only with that session;
  it is deleted with the session (DELETE or idle reaping). If the upstream
  issues a new session ID while an existing matched session is still live,
  both sessions and their distinct references remain until each is deleted
  or reaped; no replacement deletion occurs. Large records may omit it for
  size, recording
  `session_ref_state: omitted_for_budget`; records never drop the session
  state or reason to fit the budget. Treat it
  as a stable pseudonym for one server-side session incarnation:
  - Include `session_ref` in operator examples or incident bundles only when
    it is necessary to pair control requests with session observations;
    otherwise omit it from shared material and mark the omission explicitly
    `omitted_for_privacy` in operator notes (a shared-artifact annotation, not
    a record value — not `unavailable`, which is the entropy-failure state the
    record itself uses).
  - A changed reference establishes changed server session state only.
    Absence or change alone cannot distinguish token expiry, process loss,
    eviction, or unknown client input. A reconnect may replace both the
    credentials and the session.
  - The reference is not a session credential, not a lookup key, and never
    appears in a response. No raw session ID, credential/session digest,
    tombstone, or per-request correlation map is retained.
- **Missing session headers.** Header absence alone is not a rejection and
  never produces `session_header_missing`: that reason is reserved for actual
  session-required branches with an authoritative protocol cause. The pinned
  SDK's pre-dispatch initialization rejection is not yet observable through a
  supported typed path, so no missing-header reason is emitted on that path;
  no status or error text is inferred to manufacture one.
- **Not established by session observations:** Real OMP/Gemini smoke is NOT
  RUN; all examples above are synthetic. These records separate observations
  (a rejected binding, an absent entry, a created session) from refresh and
  expiry hypotheses: no record proves why a client's credentials or session
  changed, and nothing here labels identical real identities or proves actual
  client renewal.
- **Runtime provenance and safe route context.** Each record carries three
  distinct identities that must never be conflated:
  - `process_id` — the process incarnation: one independent cryptographically
    random 128-bit value (32 lowercase hex) drawn once per process and stable
    across configuration reloads. It changes on every restart. **A changed
    process incarnation alone is not incident-cause proof and is not a
    recommendation to restart**; it only lets you tell records from different
    process lifetimes apart.
  - `source_revision` / `build_state` — the verified immutable source revision
    of the running binary (40–64 lowercase hex from its VCS stamp), separate
    from the process identity and from mutable image tags. `build_state` is
    `verified` only when a clean, stamped revision was read; a locally
    modified (unpushed) build reports `unavailable` and emits no revision —
    unpushed local diagnostics never claim to be deployed, and a mutable
    tag or arbitrary hash-shaped value is never accepted as provenance.
  - `effective_policy_revision` / `effective_policy_revision_state` — an
    attested nonsecret policy-content artifact identifier, `verified` only
    when such an attestation exists; otherwise explicitly `unavailable`. The
    fixed `diagnostic_policy_revision` (`oauth-diag-policy-v1`) identifies the
    diagnostic contract only and never proves that authorization policy
    content is unchanged. No secret-bearing configuration is hashed, and no
    hidden issuer, principal, client or audience equality is inferred.
  The safe nonsecret policy tuple is `policy_source` (`defaults`,
  `environment`, `file`, `mixed`, `unavailable`), `oauth_enabled`,
  `static_configured`, `scope_required` (booleans or explicit `null`), and
  `clock_skew_seconds` (`60` when the verified validation tolerance applies,
  otherwise absent). `resource_path_mode` classifies the resource URL as
  `origin_only` or `path_bearing` without emitting any URL. This tuple is a
  startup snapshot built once from the then-current environment and
  configuration: a `SIGHUP` reload that adds or removes client token hashes
  or other policy inputs is not reflected in it until the next restart.
  Safe route and challenge context is finite classification only — never raw
  paths, queries, URLs, realms, scopes or headers: `path_class`
  (`mcp`/`metadata_root`/`health`/`other`), `route_outcome` by actual
  dispatch (`oauth_protected`, `inbound_direct`, `metadata`, `health`,
  `not_found`, `other`, `unavailable`), `challenge_kind` and
  `challenge_target` from what was actually selected and written
  (`root_well_known` versus `path_appended_well_known`), and
  `metadata_target_registered` (`yes`/`no`/`not_applicable`/`unavailable`)
  from an exact registration comparison — which is **not** proof of public
  reachability.
  Operation linkage: `operation_relation` (`none`/`single`/`multiple`/
  `unavailable`) counts only server-owned operation identities actually
  observed; a single eligible `operation_id` (32 lowercase hex) is emitted
  only for a single linked operation, several operations keep their own
  per-operation records (no unbounded terminal ID list), caller-supplied or
  JSON-RPC identifiers are never trusted, and unsafe IDs stay
  `unavailable` rather than being rewritten into certainty.
  `upstream_state` reports the actual downstream classes observed
  (`not_attempted`, `succeeded`, `transport_failed`, `auth_failed`,
  `tool_error`, `multiple`, `unavailable`) — a tool-error envelope under HTTP
  200 records `tool_error` with the completion promoted to `warn`, staying
  distinct from admission rejection; an HTTP 200 never invents downstream
  success. A tool-error example (synthetic):

  ```json
  {"event":"http_complete","level":"warn","outcome":"success","reason":"none","stage":"none","failure_stage":"none","failure_reason":"none","http_status":200,"http_method":"POST","route":"mcp","http_outcome":"success","duration_ms":210,"diag_duration_ms":209,"credential_state":"accepted","scope_state":"accepted","session_header":"absent","session_state":"not_supplied","binding_relation":"not_evaluated","session_ref_state":"not_applicable","upstream_state":"tool_error","operation_relation":"single","operation_id_state":"available","operation_id":"4f1d2c9b55a04c2e91b7f6600d47a2c9","auth_mode":"oauth","process_state":"available","process_id":"8c2f4e6a1b3d5f7e9a0c2e4b6d8f0a2c","source_revision":"e621622397298bb4eb44a9b0f5ff1a00d6f16300aa11bb22cc33dd44ee55ff66","build_state":"verified","effective_policy_revision_state":"unavailable","policy_source":"environment","oauth_enabled":true,"static_configured":true,"scope_required":true,"clock_skew_seconds":60,"resource_path_mode":"origin_only","path_class":"mcp","route_outcome":"oauth_protected","request_id":"99aabbcdddeeff001122334455667700","correlation_state":"available","diag_schema":"oauth_diag_v1"}
  ```

- **Privacy and byte bounds:** Each record is a closed typed allowlist with no
  arbitrary maps or extras: no credentials, JWTs, claims, identities, raw
  session IDs, header/path/query/URL/payload values, or error prose appear.
  Core output is bounded at 3584 bytes; the complete record including the
  trailing newline is bounded at 4096 bytes. One sink write is attempted —
  failed or short writes are not retried, and no recursive logging occurs.
- **Collection and delivery boundary:** Records are emitted as single-line JSON
  to stdout. Standard container/service stdout collection is the shipping boundary.
  Stdout write failures are nonfatal and do not fail requests, retry, or recurse;
  no durable buffer or delivery guarantee is provided. When routing to Loki or
  similar systems, index only low-cardinality metadata (`level`, `event`,
  `service`, `http_method`, `route`, `stage`, `reason`); keep high-cardinality
  identifiers such as `request_id` out of label indexes; raw session IDs and
  client addresses are not part of the typed record at all.
- **Correlating responses with log records:** An exact join parses the
  response `Mcp-Diagnostic-Id` (32 lowercase hex), finds the parsed
  `http_complete` record with the same `request_id` from the same service and
  `oauth_diag_v1` schema, and requires exact equality — arbitrary header text,
  labels, or timestamps never substitute. A hidden header (origin absence,
  transit stripping, CORS invisibility, wrapper hiding) cannot be joined by
  time coincidence or wrapper traces; classify it as unobserved rather than
  guessing. There are no CORS or proxy changes to make the header readable to
  browsers or wrappers, and no guarantee it survives them.
- **Not established by this diagnostics:** The cause of any past session
  failure, real client renewal behavior, and actual deployed collector/query
  behavior are not proven by this logging foundation; the provenance fields
  report what the running process can verify about itself and never prove
  what is actually deployed or why an incident happened. Absence of a
  diagnostic field is absence of
  evidence, never a positive diagnosis (e.g. a missing header is not proof of
  expiry).
- **Metrics repartitioning:** Inbound request counter
  `bridge_requests_total{server="_bridge", operation="<operation>", outcome="<outcome>"}`
  partitions incoming traffic into the finite operation enum (`initialize`,
  `initialized`, `tools_list`, `tools_call`, `resources_list`, `resources_read`,
  `ping`, `prompts_list`, `prompts_get`, `sse_stream`, `request`). Label names
  and server scopes remain strictly bounded; total traffic aggregations over all
  operations remain continuous while panels or alerts filtering strictly on
  `operation="request"` reflect the partitioned operations.

## Behaviour worth knowing

- **Reload is atomic.** Parse, resolve, validate, then publish one new pointer.
  A candidate that fails at any step is reported and the previous configuration,
  credentials, routes, catalog, and clients keep serving unchanged.
- **Last known good catalogs.** A refresh that cannot reach one upstream keeps
  that server's complete previous catalog and marks it stale; the remaining
  servers advance. A cold store starts empty rather than trusting an unreadable
  snapshot.
- **One execution, never a replay.** A `tools/call` is forwarded at most once.
  An upstream `InputRequired`/elicitation response becomes
  `unsupported_capability` with execution state `unknown`; the bridge never
  re-dispatches, never augments the client request, and all MRTR middleware is
  installed in its explicitly disabled form.
- **Protocol versions.** The inbound edge accepts the four legacy call versions;
  a client offering the newest version is served after negotiating down to
  `2025-11-25`, and the newest `tools/call` is rejected `400` with the supported
  version list and zero execution. `prompts/list` and `prompts/get` are served on
  all four accepted versions. Change notifications need a client that offers one
  of them: a client that offers the newest version is negotiated down, but the
  pinned `go-sdk` still classifies that session as post-SEP-2575 and delivers
  list-changed notifications only on a `subscriptions/listen` stream that the
  legacy handshake never opened, so such a client sees no `prompts/list_changed`
  and must re-list instead.

## Testing

```sh
go test ./...                          # full suite
go test -race ./...
go vet ./...

# project coverage gate (source-line, >=90%)
go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...
go run ./cmd/covcheck
```

GitHub Actions runs the race suite, vet and the instrumented full-suite coverage
gate on pushes to `main`, pull requests and manual dispatch. The instrumented run
also supplies ordinary full-suite verification; no image build or deployment runs
in this workflow.

`internal/config` carries the typed-diagnostic contract tests: every rejected
document names its config path, document location, owner/reference identity,
code, property, shape and static reason, file classification keeps its
exact-case matrix (`bounds_edge_test.go`, `private_file_test.go`,
`secret_validation_edge_test.go`), and `internal/observe` pins the exact
18-field diagnostic logger manifest in `TestConfigDiagnosticFieldManifest`.

`cmd/mcp-bridge/serve_test.go` is the process-level acceptance test: it starts
the real composition root against `httptest` upstreams and a private temp store,
then exercises readiness, catalog search, a bridged call, a SIGHUP membership
change, a stale-revision refusal, and a rejected candidate.

`cmd/mcp-bridge/prompts_acceptance_test.go` is the prompt-surface acceptance
test: one live bridge against three upstreams with two colliding raw prompt
names, covering advertised capability, publication, opaque names, routing,
provenance, sanitized errors, removal notifications, prompt-only revision
isolation, and restart/outage last-known-good behaviour. Its client pins an
accepted protocol version, because change notifications are unobservable on the
negotiated-down session described under "Protocol versions".