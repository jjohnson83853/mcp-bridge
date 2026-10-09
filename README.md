# mcp-bridge

A headless bridge that fronts several MCP upstream servers behind exactly one
Streamable HTTP endpoint, `/mcp`. It discovers each configured server's tools,
serves them under stable server-derived names, answers tool discovery through
hybrid keyword/vector search, and forwards executions to the owning upstream
with provenance attached.

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
  version list and zero execution.

## Testing

```sh
go test ./...                          # full suite
go test -race ./...
go vet ./...

# project coverage gate (source-line, >=90%)
go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...
go run ./cmd/covcheck
```

`cmd/mcp-bridge/serve_test.go` is the process-level acceptance test: it starts
the real composition root against `httptest` upstreams and a private temp store,
then exercises readiness, catalog search, a bridged call, a SIGHUP membership
change, a stale-revision refusal, and a rejected candidate.