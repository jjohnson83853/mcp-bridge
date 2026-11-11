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

## Logging

The bridge writes application logs and CLI failures as versioned JSON lines,
with typed configuration attribution, stable error classes, and explicit
stdout/stderr routing. The full field union, event and error classifications,
HTTP diagnostics, reload behavior, collector boundary, and query examples are
documented in the [structured logging contract](docs/logging-contract.md).

R8 adds refresh lifecycle and discovery records, per-pin blocker details,
successful reload attribution, and a minimal logger-write fallback. These are
optional event-specific additions; `schema_version` remains 1.

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
rejected as `restart_required` with the live runtime unchanged.
Failure diagnostics use the structured JSON contract in
[Logging](#logging) for startup, `--check-config`, reload, and lifecycle events.
The collector/shipping path is unchanged and is not guaranteed by this repository.

## Configuration diagnostics

Configuration validation emits one JSON envelope with the first deterministic
diagnostic, safe source attribution, and a static remediation. Startup rejection
and `--check-config` use the same `config_validation_failed` contract; a rejected
reload uses the warning-level reload event and leaves the active configuration
serving. The 128-pin ceiling identifies the first excess entry as `pins[128]`
without exposing unrelated pin values. See
[the logging contract](docs/logging-contract.md) for the schema, attribution
rules, failure codes, streams, and examples.

## Tool surface

One MCP server, three tools:

| Tool | Purpose |
| --- | --- |
| `search_tools` | hybrid keyword/vector search over current catalogs; optionally narrowed with `servers: []` |
| `describe_tool` | exact schema for a tool name; `catalog_revision` is optional |
| `call_tool` | one forwarded execution; `catalog_revision` is optional |

All three declare standard MCP tool annotations (advisory hints — never
authorization and never a promise of zero confirmation prompts):

| Tool | `readOnlyHint` | `destructiveHint` | `idempotentHint` | `openWorldHint` |
| --- | --- | --- | --- | --- |
| `search_tools` | true | false | true | false |
| `describe_tool` | true | false | true | false |
| `call_tool` | false | true | false | true |

`search_tools`/`describe_tool` are read-only, idempotent discovery over the
bridge's own catalog and dispatch nothing upstream. The generic `call_tool`
forwards execution to an arbitrary upstream tool: it may mutate, may be
destructive, is not idempotent, and interacts with the open world of configured
upstream servers. The annotations carry no guarantee that a client will skip
its permission prompts — clients decide independently — and they never grant
execution authority or bypass any policy.

The built-in descriptions steer clients toward tools already declared in their
active list. `search_tools` includes the exact, alphabetically sorted wire names
of published configured pins; with no published pins it uses a baseline
direct-first description without an empty inventory. `describe_tool` discourages
redundant schema discovery, and `call_tool` reserves generic dispatch for
unpinned tools. These are advisory descriptions: unpinned tools remain
discoverable, and schemas, annotations, dispatch behavior, and upstream
metadata are unchanged.

Unpinned tools returned by `search_tools` and revealed in a session projection
use semantic wire names of the form `<server>__<tool>` when the name is a valid
identifier and at most 64 characters long. If it is longer or contains invalid
identifier characters, the bridge uses the deterministic fallback `t_` plus
unpadded lowercase base32 of `SHA-256([server_id, raw_name])` (RFC 4648 §6).
A schema change moves the catalog revision without moving the public name.

Discovery responses are intentionally pruned: `search_tools` returns the
catalog revision once at the root and keeps per-tool `availability`, but omits
the `backends` cluster-health dump and redundant per-result health/revision
fields. `describe_tool` also retains `availability` while omitting
`last_success_at` and `catalog_status`. `call_tool` keeps its outer provenance
envelope and complete upstream result without duplicating discovery-health
fields.

The catalog revision is **global**: it hashes the sorted bound server identities
and their exact definitions (schemas compared whitespace-insensitively), plus
each definition's upstream metadata (title, icons, annotations, `_meta`). A
metadata-only upstream change therefore moves the revision without moving the
public name. `search_tools` includes the revision at the response root.
`catalog_revision` is optional for `describe_tool` and `call_tool`: omitting it
or passing `""` uses the current published revision. A non-empty value must be
exactly 64 lowercase hexadecimal characters and match the current revision;
malformed values return `invalid_arguments`, while stale revisions return
`catalog_changed` without upstream dispatch. Refresh discovery before retrying
with a current revision.


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
(content blocks, structured content, `isError`, metadata). The outer
`call_tool` envelope contains `server`, `tool_name`, `request_id`, and
`upstream_result`; discovery revision and health fields are not duplicated.

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

Each observed inbound HTTP request produces one terminal `http_complete`
diagnostic in the same versioned JSON-line format. The fixed HTTP lane has its
own closed 60-slot map, shares the level-to-stream routing and collector
boundary, and records only finite route, transport, outcome, and correlation
classes. It does not serialize raw paths, query strings, headers, payloads,
credentials, or arbitrary error text. The complete slot map, nested error
shape, provenance boundary, and Loki examples are in the
[structured logging contract](docs/logging-contract.md).

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

`internal/config` and `internal/observe` exercise the structured diagnostic
contract, including configuration attribution, typed error objects, HTTP
records, and the closed field vocabulary. The full contract and its tests are
linked above.

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