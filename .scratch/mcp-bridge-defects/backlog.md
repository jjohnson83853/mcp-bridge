# mcp-bridge defect backlog

Local-only. Read-only investigation; no fixes, no GitHub issues, no push.

Index:

| ID | Title | Severity | Status | Component |
|---|---|---|---|---|
| MB-001 | Header-bound tool parameters do not traverse the bridge | high | fixed (owner-attested; independent deployed verification not captured) | `internal/upstream` / upstream schema |
| MB-002 | Catalog serves keyword-only search with no client-visible signal | medium | open | `internal/search` / embedding provider |
| MB-003 | One upstream's failures pause the whole aggregated Tool server | high | open | client harness (Hermes) counter; bridge per-upstream breaker |
| MB-004 | Embedding timeout bounds the whole pass, so semantic search cannot activate on a large catalog | high | open | `cmd/mcp-bridge` embedder; `internal/search/provider`; `internal/config/bounds` |
| MB-005 | A failed embedding pass is completely silent | high | open | `internal/service/embedding`; catalog observability |
| MB-006 | One unreachable upstream blocks config reloads and degrades every refresh | high | open | `internal/service/reload`; `internal/config` servers schema |
| MB-007 | Invalid config crash-loops the gateway instead of falling back | high | open | `cmd/mcp-bridge` startup; `internal/config` validation |
| MB-008 | Second instance `mcp-bridge-network` is in a permanent restart loop | medium | open | deployment (`mcp-bridge-network` container) |
| MB-009 | `storage.directory: ./data` resolves against the config file, not the process CWD | low | open | `internal/config` storage resolution; startup observability |

Observation window (MB-001–MB-003): 2026-11-18 ~11:35–11:42 EST / 16:35–16:42 UTC, through the bridge's own Tool MCP server.
Observation window (MB-004–MB-009): 2026-11-21 18:30–19:35 EST / 23:30–00:35 UTC, live container `mcp-bridge` on `192.168.50.61` (`registry.bullprog.com:5000/mcp-bridge:latest`, config bind `/srv/mcp-grafana/mcp-bridge-config -> /app/configs`, state under `/app/configs/data`). Live telemetry and filesystem claims below were **not re-read at ticket time**; every code reference was re-verified in this worktree.
Live container: `mcp-bridge` (`cedc9b05…`, image `registry.bullprog.com:5000/mcp-bridge:latest`, created 2026-11-17T21:50Z).
Catalog revision at observation: `f898198b13f7b27e4ddf951562109455aa032bc00205935f738094647af51e80`.

---

## MB-001 — Header-bound tool parameters do not traverse the bridge

**Severity:** high
**Status:** fixed (owner-attested; independent deployed verification not captured)
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

**Cross-reference:** MB-004 — a pass-wide embedding timeout makes semantic search unactivatable on this catalog size; this ticket covers the missing client-visible signal.

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

---

## MB-004 — Embedding timeout bounds the whole pass, so semantic search cannot activate on a large catalog

**Severity:** high
**Status:** open
**First observed:** 2026-11-21 (observation window above)
**Component:** `cmd/mcp-bridge` embedder; `internal/search/provider`; `internal/config/bounds`

### Symptom (verbatim)

- Snapshot at `/app/configs/data/snapshot.json` (1.68 MB): `tools=974`, `embeddings=null`.
- Every `search_tools` response returns `"mode":"keyword"`, `"degraded_reasons":["vectors_absent"]`.

Live snapshot and responses **not re-read at ticket time**; reported from the observation window.

### Reproduction

1. Load a catalog of ~974 documents with `embedding.max_concurrency: 2` and `embedding.batch_size: 32`.
2. Trigger a refresh with no reusable vectors (`embeddings` empty/null).
3. Observe that the provider receives one deadline for the entire missing-vector pass.
4. Measure pass wall time; it exceeds the deadline on a large catalog.
5. Try `embedding.timeout: "120s"`: config validation rejects it (`duration_range`, shape `above_maximum`).

### Evidence

- `cmd/mcp-bridge/wiring.go:264-275` — `embedder()` creates **one** `context.WithTimeout(context.Background(), settings.Timeout)` and passes it to `provider.Embed(ctx, inputs)` for every missing document.
- `internal/search/provider.go:120-153` — `openAIProvider.Embed` fans all batches out under that single ctx at `p.concurrency` (sample `max_concurrency: 2`, `configs/config.example.yaml:62`); any batch error cancels the rest.
- `internal/config/config.go:655` — `embedding` allows `timeout max_concurrency batch_size max_response_bytes`; there is no separate pass/request budget.
- `internal/config/bounds.go:168-186` — a key literally named `timeout` is capped at 30s in every section (this is the rule that hits `embedding.timeout`); additionally any key whose name contains `search`/`embed` is capped at 30s. Rejection: `code=duration_range`, `property=range`, `shape=above_maximum`. `key.Value` is the leaf key, so the substring rule does **not** by itself explain `embedding.timeout`; the literal-`timeout` branch does.
- `internal/search/search.go:392-407` — zero valid vectors ⇒ `ModeKeyword` + `reasonVectorsAbsent`.
- `internal/service/embedding.go:23-26` — a failed/timed-out pass returns the snapshot unchanged.
- **Unverified at ticket time:** the live `snapshot.json` counts, the 10× `config_validation_failed` rows, and the claimed `config.yaml:46` line.

### Hypothesis (testable)

Wall time of one pass ≈ `ceil(documents / batch_size / max_concurrency)` round trips × request latency. At 974 documents and batch size 32 that is ≥31 batches at concurrency 2; any per-request latency above roughly 1s exceeds the hard-capped 30s pass deadline. The canceled pass then falls back to the reuse-only snapshot, so `embeddings` stays null and search can never enter hybrid mode.

### Acceptance

- Split `embedding.request_timeout` (1s..30s, per HTTP request) from `embedding.pass_timeout` (1s..10m, default ~5m).
- Keep `embedding.timeout` as a one-schema-version alias for `request_timeout`.
- Stop the `bounds.go` substring/literal-`timeout` heuristics from applying to the explicit pass budget.
- Fake-provider test: 40 × 50ms batches succeed at `pass_timeout=5s` and fail with a typed, loggable error at 1s.
- Config test: accepts 300s, rejects 11m.
- 974-document stub run yields a non-null `embeddings` array and `mode="hybrid"`.
- No ranking/RRF/weight changes.

---

## MB-005 — A failed embedding pass is completely silent

**Severity:** high
**Status:** open
**First observed:** 2026-11-21 (observation window above)
**Component:** `internal/service/embedding`; catalog observability

### Symptom (verbatim)

- `fillVectors()` returns the snapshot unchanged when `deps.Embed` errors or returns zero rows: no event, no counter.
- Search reports only the downstream `"degraded_reasons":["vectors_absent"]`, which cannot distinguish provider failure, empty response, or stale model digest.
- Hours of live logs contain zero embedding-related events while vectors are absent.

### Reproduction

1. Point `embedding.base_url` at a provider that returns an error (or an empty `data` array).
2. Trigger a refresh and let the pass fail.
3. Call `search_tools` and inspect `degraded_reasons`.
4. Search bridge logs for any per-pass embedding outcome — none exists.

### Evidence

- `internal/service/embedding.go:18-27` — on `err != nil || len(embedded) == 0` the function silently returns the input snapshot.
- `internal/search/search.go:392-407` — the only surfaced signal is the downstream `vectors_absent` reason.
- No `embedding_pass_*` event exists anywhere (`grep -r 'embedding_pass' internal cmd` → 0); refresh lifecycle events are `catalog_refresh_started` / `catalog_refresh_completed` / `catalog_refresh_failed` (`internal/service/refresh.go:234,305,341`), allowlisted in `internal/observe/log.go:258-264`.
- The degraded reason vocabulary is finite in `internal/search/search.go:293-300`; adding a sharper reason requires extending it.
- **Unverified at ticket time:** the live zero-event log scan.

### Hypothesis (testable)

The embedding outcome is discarded at the `fillVectors` boundary because the build path treats vectors as an optional enrichment. A zero-row response is indistinguishable from a reuse-only pass, so operators cannot see provider failure, dimension mismatch, or model-digest churn.

### Acceptance

- Emit `embedding_pass_completed` / `embedding_pass_failed` once per pass with `documents_missing`, `batches`, `reused_from_store`, `embedded`, `duration_ms`, `model`, `model_revision`, `dimensions`, and a typed error on failure — following the existing `catalog_refresh_*` envelope.
- Carry the outcome into the published generation and the ops payload.
- Sharpen the degraded reason (e.g. `embed_pass_timeout`) while keeping the keyword fallback.
- A zero-vector response is a reported failure, never silent.
- No `api_key`, base URL credential, or full vector appears in the serialized event.

---

## MB-006 — One unreachable upstream blocks config reloads and degrades every refresh

**Severity:** high
**Status:** open
**First observed:** 2026-11-21 (observation window above)
**Component:** `internal/service/reload`; `internal/config` servers schema

### Symptom (verbatim)

- 42× `upstream_discovery_failed` in one process lifetime: `legal` `dns_lookup_failed`, `mcp-netutils` `connect_failed`, `irs-taxpayer` `tools_list_failed` (that server runs and is healthy).
- 14× `catalog_refresh_failed` with `"publication_impact":"degraded_published"` and zero `catalog_refresh_completed`.
- The reload path rejects the whole candidate on any discovery failure, so SIGHUP cannot apply a config change while a single entry is broken; the only remedy is a container restart.

### Reproduction

1. Configure ≥2 servers where one is unreachable (bad DNS or closed port).
2. SIGHUP a valid change to another server.
3. Observe `config_reload_rejected` / preserved generation; the change does not apply.
4. Compare a periodic refresh pass: it publishes degraded state but logs the same `catalog_refresh_failed` event.

### Evidence

- `internal/service/reload.go:331-334` — build-mode contract: “a reload validates the whole membership and rejects the candidate on any failure, a refresh keeps every server's last known good catalog when its own discovery fails.”
- `internal/service/reload.go:475-496` — `Runtime.Reload` documents “On any failure it returns an error and leaves the published generation … exactly as they were.”
- `internal/service/refresh.go:44-47` — refresh intentionally degrades per server instead of rejecting.
- `internal/upstream/dedicated.go:238-247` — emits `upstream_discovery_failed` with `failure_stage` / `failure_reason` / scheduled `next_attempt_ms`.
- `internal/config/config.go:658` — allowed server keys are `id endpoint prompts_disabled credentials limits tool_metadata stateless`; there is no `optional`.
- `internal/service/refresh.go:270-272` — a discovery failure on a published pass is logged as `catalog_refresh_failed` with `publication_impact="degraded_published"`; the same event name is used for `candidate_rejected`. The observed 14×/0-completed pair is therefore consistent with published-but-degraded passes and does **not** by itself prove reload rejection; the reload contract above does. The conflated event name is part of this ticket.
- **Unverified at ticket time:** the live counts and per-server reasons.

### Hypothesis (testable)

Reload admission has no per-server tolerance: one discovery error anywhere fails the build. Because refresh uses the same event name for both a rejected candidate and a degraded publish, operators see a uniform failure stream and cannot tell which passes actually published.

### Acceptance

- Opt-in `servers[].optional` (default false; no behavior change for existing configs) that degrades that server instead of failing the pass.
- Reload tolerates optional-server failures and logs the skipped ids.
- Per-server status/reason/last-success stays visible to clients.
- Discovery retry/backoff unchanged.
- (a) Optional server failing ⇒ refresh completes and publishes with that backend marked unavailable; (b) non-optional failing ⇒ today's behavior; (c) reload with an optional failure publishes; (d) event names skipped ids in a bounded list.

---

## MB-007 — Invalid config crash-loops the gateway instead of falling back

**Severity:** high
**Status:** open
**First observed:** 2026-11-21 (observation window above)
**Component:** `cmd/mcp-bridge` startup; `internal/config` validation

### Symptom (verbatim)

- `embedding.timeout: "120s"` produced 10× `config_validation_failed` (`duration_range`, `shape=above_maximum`, `/app/configs/config.yaml` line 46).
- The container entered a `restart: always` loop (`status "restarting"`, `RestartCount` climbing), taking down every MCP tool the bridge fronts (portainer, grafana, ssh, second-brain, rag) because the bridge is the only path its own operators have.

### Reproduction

1. Set an out-of-range value (e.g. `embedding.timeout: "120s"`) in the active config.
2. Start the process under `restart: always`.
3. Observe one `config_validation_failed` per start, exit 1, then a restart cycle; no fallback config is served.

### Evidence

- `cmd/mcp-bridge/main.go:56-68` — `serve` error carrying `*config.Diagnostics` ⇒ `writeConfigValidationFailure`, return 1. There is no fallback path.
- `cmd/mcp-bridge/diagnostics_envelope.go:20` — `config_validation_failed` envelope vocabulary.
- `cmd/mcp-bridge/startup_failure.go:18-70` — `startup_failed` variants cover state dir, JWKS, embedding provider, tool surface, catalog loop, listener bind; no config-degraded fallback variant.
- `grep -r 'MCP_STRICT_CONFIG\|last-known-good.*config\|config_degraded' cmd internal` → 0 matches; “last known good” exists only for the catalog generation (`internal/service/refresh.go:34,184`; `internal/service/reload.go:628`).
- **Unverified at ticket time:** the live `config_validation_failed` count and restart counters.

### Hypothesis (testable)

Startup treats a config validation error as terminal and the container supervisor retries forever. A previously validated copy of the config, if retained, could serve the last-known-good generation until the operator fixes the file.

### Acceptance

- Invalid active config + validated fallback ⇒ serves, emits `config_degraded` naming file/line/property, never accepts the invalid value silently.
- Explicit strict mode `MCP_STRICT_CONFIG=true` always fails closed (today's behavior).
- Persist last-known-good on every successful load **without** overwriting the operator's file.
- Invalid with no fallback ⇒ `startup_failed`.
- If the exit path is kept, capped exponential backoff plus a `startup_failed` event naming the property.
- No secret written into the fallback file.

---

## MB-008 — Second instance `mcp-bridge-network` is in a permanent restart loop

**Severity:** medium
**Status:** open
**First observed:** 2026-11-21 (observation window above)
**Component:** deployment (`mcp-bridge-network` container)

### Symptom (verbatim)

- Container `mcp-bridge-network`, same image, its own config `/app/configs/config-network.yaml` (bind `/srv/mcp-grafana/mcp_bridge_network_data`), `State=restarting`, `RestartCount=82+`, cycling roughly every 20s, never serving traffic.

### Reproduction

1. Inspect the container's exit reason and the last stderr line of each cycle.
2. Read `/app/configs/config-network.yaml` from the bind mount.
3. Run the same image locally with that config and capture the single JSON diagnostic.
4. Compare with MB-004's duration rule to see whether any `*timeout`/`*_timeout` key is the trigger.

### Evidence

- Live container state and the config path **not re-verified at ticket time**; reported from the observation window.
- The diagnostics path that produces one JSON line and exit 1 is `cmd/mcp-bridge/main.go:56-68`; config bounds live in `internal/config/bounds.go:168-186`.

### Hypothesis (testable)

One value in `config-network.yaml` violates a validation bound, and the supervisor's `restart: always` policy turns a deterministic config error into an endless loop. The MB-004 duration heuristic is a candidate but must be confirmed against the actual file.

### Acceptance

- Diagnosis recorded in this ticket: which key, which validation stage, and whether the MB-004 duration rule is the cause.
- Reproduction with that config locally, quoting the diagnostic.
- Fix applied (config or code), with the service kept running — **not** resolved by deleting the service.
- After the fix, the container reaches `running` and serves traffic.

---

## MB-009 — `storage.directory: ./data` resolves against the config file, not the process CWD

**Severity:** low
**Status:** open
**First observed:** 2026-11-21 (observation window above)
**Component:** `internal/config` storage resolution; startup observability

### Symptom (verbatim)

- Container `WorkingDir=/app` with the config at `/app/configs/config.yaml`.
- `/app/data` is empty while `snapshot.json` (1.68 MB), `prompts.json`, and `.writer.lock` live in `/app/configs/data` — inside the bind-mounted config directory.

### Reproduction

1. Put `storage.directory: "./data"` in a config file and run the binary from a different working directory.
2. Observe the resolved path (state is created next to the config file, not the CWD).
3. Confirm the resolved absolute path is absent from startup output.

### Evidence

- `internal/config/config.go:380-381` — a relative `storage.directory` is cleaned/joined onto `filepath.Dir(configPath)`.
- `internal/config/config_test.go:304-312` — `TestParseResolvesRelativeStorageAgainstConfigDirectory` asserts `/etc/mcp-bridge/config.yaml` + `./state` ⇒ `/etc/mcp-bridge/state`. The config-dir base is therefore deliberate and tested, not an accident.
- `configs/config.example.yaml:10` documents that `{file:}` secret paths are relative to the config file; `:35` sets `directory: "./data"` without stating the base for this key.
- No `config_loaded` event exists (`grep -r 'config_loaded' cmd internal` → 0); startup emits only `config_validation_failed` / `startup_failed`.
- **Unverified at ticket time:** the live `/app/data` vs `/app/configs/data` listing and file sizes.

### Hypothesis (testable)

The base for `storage.directory` is config-file-relative and tested, but it is not stated where operators read it, and the resolved absolute path is never logged. With the config bind-mounted at `/app/configs`, state lands inside the config directory, where it is invisible to anyone assuming CWD-relative `./data`.

### Acceptance

- One documented base for relative resolution (CWD or config-file dir), chosen explicitly.
- Emit the resolved absolute path once in a startup `config_loaded` event (new or existing event), including the config path.
- Reject a `storage.directory` that resolves inside the config file's own directory unless explicitly allowed.
- Update `configs/config.example.yaml` and docs.
- Tests for both resolution bases, for the inside-config-dir rejection, and for the startup event carrying the absolute path.