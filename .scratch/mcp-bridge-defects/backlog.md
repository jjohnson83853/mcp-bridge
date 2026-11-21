# mcp-bridge defect backlog

Local-only. Read-only investigation; no fixes, no GitHub issues, no push.

Index:

| ID | Title | Severity | Status | Component |
|---|---|---|---|---|
| MB-001 | Header-bound tool parameters do not traverse the bridge | high | open | `internal/upstream` / upstream schema |
| MB-002 | Catalog serves keyword-only search with no client-visible signal | medium | open | `internal/search` / embedding provider |
| MB-003 | One upstream's failures pause the whole aggregated Tool server | high | open | client harness (Hermes) counter; bridge per-upstream breaker |

Observation window: 2026-11-18 ~11:35–11:42 EST / 16:35–16:42 UTC, through the bridge's own Tool MCP server.
Live container: `mcp-bridge` (`cedc9b05…`, image `registry.bullprog.com:5000/mcp-bridge:latest`, created 2026-11-17T21:50Z).
Catalog revision at observation: `f898198b13f7b27e4ddf951562109455aa032bc00205935f738094647af51e80`.

---

## MB-001 — Header-bound tool parameters do not traverse the bridge

**Severity:** high
**Status:** open
**First observed:** 2026-11-18
**Component:** `internal/upstream` call path; `github-mcp-server` upstream; bridge/upstream MCP protocol handshake

### Symptom (verbatim)

- `Error calling tool 'get_file_contents': Connection closed`
- `the upstream call did not complete`
- Live run: client `upstream_protocol_error`; log `code=tool_reported_error`, `upstream_state=tool_error`, HTTP 200.

`github__get_file_contents` publishes `owner` and `repo` as `"x-mcp-header"` properties. Calls with those arguments fail; same-window calls to `github__search_repositories` (query only) and `github__search_code` succeed.

### Reproduction

1. `describe_tool github__get_file_contents` — confirm `owner`/`repo` carry `"x-mcp-header"` and `path`/`fields` are body parameters.
2. Call `github__get_file_contents` with `owner=jjohnson83853`, `repo=mcp-bridge`, `path=README.md`.
3. Within a few seconds, call `github__search_repositories` with `query=mcp-bridge`. No other calls between.
4. Correlate both request IDs in bridge structured logs (`http_complete`, `backend_complete`, `mcp_complete`, `upstream_discovery_failed`).

### Evidence

- Failing call 2026-11-18T16:39:21Z, rid `cec8bb6ff3b692cdc848c7d17727e010`:
  - `http_complete`: `"http_status":"200","http_outcome":"success","upstream_state":"tool_error","duration_ms":"722"`
  - `backend_complete`: `"server":"github","tool":"t_effo7fw2vfllva2cyqyzn4gtwtpnnyiu33norql3dfag3q7nbjea","duration_ms":"720","error_code":"tool_reported_error","upstream_content_count":"1","upstream_structured_presence":"absent"`
  - `mcp_complete`: `code=tool_reported_error stage=backend_response dur=721`
- Control seconds later: `github__search_repositories(query=mcp-bridge)` succeeded, `total_count=7295`; client rid `d4e9af3c` (no Loki row matched that short rid).
- `grep -r 'x-mcp-header' internal/` → 0 matches. The bridge itself has no translation code; the input schema is preserved and published (`internal/upstream/dedicated.go:108-135,472-499`; `internal/service/tools.go:614-619`).
- Call path keeps arguments in the JSON `arguments` field: `internal/service/tools.go:718-720,755-790` → `internal/core/core.go:269-280` → `internal/upstream/dedicated.go:397-401` → `internal/upstream/runtime.go:204`.
- go-sdk v1.8.0 does implement `x-mcp-header` → `Mcp-Param-*` (`mcp/streamable_headers.go:222-256`), but only for protocol ≥ `2026-07-28` and only when the tool definition is present in the request context (`:202-218`). That definition comes from the session-local `ListTools` cache (`mcp/client.go:595-609,1274-1276,1292-1294`).
- With `stateless` defaulting true (`internal/config/config.go:48-52`), discovery closes its session (`internal/upstream/dedicated.go:76-89`) and execution opens another without `ListTools` (`:366-401`).

### Hypothesis (testable) and discriminating test

**Hypothesis:** execution runs without the schema definition that marks `owner`/`repo` as header parameters, so those values stay in the JSON body and no `Mcp-Param-*` headers are emitted; the upstream then returns an error tool result.

**Discriminating test run:** header call failed (rid `cec8bb6f`) while the non-header control succeeded seconds later (rid `d4e9af3c`). This **kills the wedged-session hypothesis** for this pair.

**Not yet run:** capture outbound request headers for the failing call. The result proves the header tool fails while a control works; it does not prove which header was absent. The upstream returned HTTP 200 with a tool error, so closure at the transport layer is not established either.

**Next check:** capture one execution POST's headers/body (test bridge or `httputil.DumpRequestOut`) and confirm presence/absence of `Mcp-Param-owner`, `Mcp-Param-repo`, and the negotiated `MCP-Protocol-Version`; repeat with a stateful session or a server on protocol ≥ 2026-07-28.

### Blast radius

Every upstream tool whose schema marks parameters `x-mcp-header` fails through normal `call_tool` execution. Query-only tools on the same server continue to work, so the fault looks tool-specific. Clients cannot distinguish this from an upstream outage (`upstream_protocol_error` / `tool_reported_error`).

---

## MB-002 — Catalog serves keyword-only search with no client-visible signal

**Severity:** medium
**Status:** open
**First observed:** 2026-11-18 (onset not derivable from logs)
**Component:** `internal/search`, embedding provider/configuration, catalog observability

### Symptom (verbatim)

`search_tools` returns:

```json
"mode":"keyword",
"degraded_reasons":["vectors_absent"],
"catalog_revision":"f898198b13f7b27e4ddf951562109455aa032bc00205935f738094647af51e80"
```

Tools still list and call; ranking is silently keyword-only.

### Reproduction

1. Call `search_tools` with any query at 16:34Z and again at 16:51Z.
2. Read `mode`, `degraded_reasons`, and `catalog_revision`.
3. Query bridge logs for a matching event carrying the same fields.

### Evidence

- Both calls returned `mode="keyword"`, `degraded_reasons=["vectors_absent"]`, same catalog revision.
- No bridge log event carries `mode` or `degraded_reasons`; `|= "degraded"` matches only `catalog_refresh_failed` rows with `"publication_impact":"degraded_published"` (older container `4ec0209…`, 2026-11-15; different cause).
- Producer: `internal/search/search.go:392-407` sets keyword mode when a provider exists but zero eligible vectors validate.
- Dimension mismatch discards the batch: `internal/search/provider.go:234-235` (`embedded vectors != model dims` pattern).
- Embedding failure silently retains reuse-only vectors: `internal/service/embedding.go:18-27`.
- Snapshot publication excludes model/content/dimension mismatches: `internal/service/tools.go:378-401`.
- Exposure is search payload only: `internal/service/tools.go:982-989`; readiness ignores vector state: `internal/service/ops.go:80-101`.
- Config sample `configs/config.mcp-grafana-stack.yaml:41-44`: `provider=openai-compatible`, `model=fictitious-embedding-model-v1`, `model_revision=fictitious-revision-v1`, `dimensions=1536`, `base_url=https://litellm.bullprog.com/v1`. Deployed `/srv/mcp-grafana` config was unreadable; live values are `[INFERENCE]`.

### Hypothesis (testable) and discriminating test

**Hypothesis:** the configured embedding model either is a placeholder or returns vectors whose dimension differs from the configured `1536`, so every batch is rejected and the catalog falls back to keyword scoring.

**Run:** live `search_tools` confirmed the degraded state twice (16:34Z, 16:51Z).

**Not yet run:** inspect returned vector widths/model identity from the embedding endpoint, and read the deployed config. Logs cannot date the onset because degradation is not logged.

**Next check:** query the embedding endpoint for one vector and compare its length/model with the configured values; if mismatched, record the model response; if correct, inspect the reuse-only path (`embedding.go:18-27`) for the failure reason that is currently discarded.

### Blast radius

All discovery traffic through `search_tools`: results remain usable but ranking quality silently drops to keyword BM25. No tool becomes unavailable. Clients that do not read `degraded_reasons` (including most MCP clients) cannot detect the degradation; readiness and health probes stay green.

---

## MB-003 — One upstream's failures pause the whole aggregated Tool server

**Severity:** high
**Status:** open
**First observed:** 2026-11-18
**Component:** client harness failure counter (Hermes); bridge per-upstream breaker is a separate mechanism

### Symptom (verbatim)

`MCP server 'Tool' rejected the last N calls ... Paused for ~59s`

After five failed upstream calls, unrelated calls including `describe_tool` were refused for about a minute.

### Reproduction

1. Issue five failing upstream calls through the aggregated `Tool` MCP server in one window.
2. Immediately call an unrelated tool and `describe_tool`.
3. Record the rejection text, the count `N`, the pause duration, and which calls were refused.
4. Correlate the five failing calls and the pause in both bridge and client logs.

### Evidence

- Failing calls (bridge `mcp_complete` rows, 16:38–16:50Z):
  - `bae19f532834a225ecf19d069f802864` — `tool_reported_error`, 137 ms, 16:38:50
  - `cec8bb6ff3b692cdc848c7d17727e010` — `tool_reported_error`, 721 ms, 16:39:21
  - `d20ce656eedbcb9e9acf59677065bf24` — `upstream_protocol_error`, stage `backend_dispatch`, 448 ms, 16:39:23
  - `13179f339a549d14cea1102a4d7b1b86` — `upstream_protocol_error`, 580 ms, 16:39:23
  - `94b4b7213109da90285e5d587525ce1c` — `upstream_protocol_error`, 581 ms, 16:40:38
  - `69a618b02fd8cca183e98c3e7dfafc67` — `upstream_protocol_error`, 430 ms, 16:49:47
- Bridge logs 16:30–17:00Z contain **zero** `backend_unavailable`/`backend_busy` admission rows and no `Paused`/`rejected the last` text. The bridge did not emit the pause.
- Exact pause text is found in Hermes `tools/mcp_tool_handlers.py:306-308` (GitHub main; deployed version unknown). Counter is per Hermes server connection — the aggregated `Tool` server, not an individual bridge upstream. Error payloads increment; success resets (`:352-360`; `tools/mcp_tool.py:560-578`). Admission precedes every RPC, which explains `describe_tool` refusal (`:807-813`). Main uses 3 failures/60 s (`tools/mcp_tool.py:542`), not the observed five.
- Bridge breaker is separate and per upstream `ServerClient` (`internal/upstream/runtime.go:266-310`): `consecutive_failures=3`, counts transport/malformed/deadline, `open_duration=30s`, one half-open probe; admission while open returns `backend_unavailable`. Deployed breaker values are `[INFERENCE]` (config unreadable).

### Hypothesis (testable) and discriminating test

**Hypothesis:** the Hermes client counts bridge tool-error results at the aggregated server-connection level, so one upstream's failure series trips a single global pause that blocks unrelated tools.

**Run:** log reconstruction confirms the bridge emitted no global pause; the pause text exists only in Hermes. A direct 5-failure reproduction with Hermes logs was **not deliberately run** (read-only constraint; would pause the client).

**Not yet run:** live reproduction with client-side log capture; confirm the deployed Hermes threshold and whether it counts JSON-RPC tool results (`isError`) or transport errors, and why the observed count is five vs the 3-failure constant.

**Next check:** reproduce in a controlled client session, capture Hermes counter increments/reset, and identify whether `describe_tool` is refused before RPC dispatch. If confirmed, the bridge-side mitigation would be to avoid returning generic application errors for upstream failures (or expose a machine-readable error code the client can exempt), not to change the per-upstream breaker.

### Blast radius

While paused, every tool on the aggregated `Tool` server is unavailable to that client, including tools from healthy upstreams and bridge-native `describe_tool`/`search_tools`. One flaky upstream can therefore stall unrelated work for every client instance that shares the same Hermes server connection. The bridge remains up; other clients and the per-upstream breaker are unaffected.