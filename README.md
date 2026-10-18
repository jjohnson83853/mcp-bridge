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
client_auth: { ... }     # bearer tokens this bridge accepts
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

Names are server-derived and stable: `t_` plus unpadded lowercase base32 of
`SHA-256([server_id, raw_name])` (RFC 4648 §6). A schema change moves the catalog
revision without moving the public name.

The catalog revision is **global**: it hashes the sorted bound server identities
and their exact definitions (schemas compared whitespace-insensitively). Search
returns it with every result, and `describe_tool`/`call_tool` require it. A
mismatch — including one caused by an unrelated server's definition changing — is
`catalog_changed` with zero dispatch; refresh explicitly rather than relying on
implicit rerouting.

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