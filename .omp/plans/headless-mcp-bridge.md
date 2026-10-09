# Headless MCP Bridge — SpecDD v1

## 1. STATUS, PURPOSE AND SCOPE

**Spec ID:** HMB-v1 · **Revision:** 8 · **Date:** 2026-10-06 · **Status:** DRAFT incorporating the user-confirmed single-endpoint scope amendment.

**Objective:** Expose ONE authenticated MCP endpoint, `/mcp`, fronting all configured upstream servers, with bounded discovery and validated execution, without advertising their entire catalog.

**Artifact:** `/home/opencode/projects/default/mcp-bridge/.omp/plans/headless-mcp-bridge.md`. Only this planning artifact is writable. Proposed files, schemas and commands are plans, not implementation or verification evidence.

**Views:** Requirements = §2 and §5; Design = §4; Tasks = §6 and §7. The coordinator reports explicit user approval of ALL THREE r4 checkpoints in this conversation, accompanied by a scope amendment that supersedes the namespace model. This revision implements that confirmed amendment. It does not invent approval of the resulting r5 artifact or grant execution permission; the coordinator reconciles changed contracts/checkpoints. Stable R-/AC-/T- IDs are preserved. T-10 remains BLOCKED on real-environment prerequisites.

**Scope change:** No namespace layer, `/mcp/<id>` routes, namespace configuration, namespace input field, namespace selector or namespace metric label. Server ID is provenance. Search retains `servers[]`; identity tuple becomes `[server_id, raw_name]`; global catalog revision replaces per-namespace revisions. U-06 is RESOLVED, not an open user decision. All prior critic-confirmed protocol, authentication, no-replay, isolation, LKG, reload, coverage and test obligations remain.

**Protocol boundary retained:** Stateful call-based inbound versions are the four pre-2026-07-28 versions, enforced by a default-options server on a stateful StreamableHTTPHandler; NO SupportedProtocolVersions narrowing. Latest server/discover is answered with supportedVersions exactly the four legacy versions; initialize offering latest negotiates down to 2025-11-25; latest tools/call is rejected with HTTP400/CodeUnsupportedProtocolVersion advertising the legacy list and zero execution. No400 for discover or latest initialize offers. Upstream targets all five SDK versions. Every upstream client explicitly sets `MultiRoundTrip: &MultiRoundTripOptions{Disabled:true}`; ALL original-operation re-dispatch middleware, including urlElicitationMiddleware, is prohibited.

**Non-goals:** UI, stdio, legacy HTTP+SSE transport, OAuth, RBAC/per-user/per-tool policy, built-in TLS, distributed storage, vector database, automatic prerequisites/retries/interactive re-dispatch, arbitrary URL proxy, resources/prompts/sampling/elicitation/roots/tasks proxy, new observability infrastructure, inbound latest call-based execution or dual-mode handler, and any namespace abstraction. SDK discovery/negotiate-down is not latest execution support. No speculative namespace extension is planned.

## 2. REQUIREMENTS VIEW

All 22 stable IDs remain covered. The confirmed scope change rewords R-07/R-08/R-10 and translates dependent contracts without dropping any critical-test group.

| ID | Required behavior | Acceptance | Tasks |
|---|---|---|---|
| R-01 | Streamable HTTP only inbound/upstream, including JSON and SSE responses; no stdio or legacy HTTP+SSE fallback. Version support in §4.5. | AC-01,AC-02 | T-01,T-03,T-04,T-09 |
| R-02 | Strict YAML configuration; headless, no UI. | AC-03 | T-02,T-09 |
| R-03 | Static bearer authentication on every MCP request; no OAuth. | AC-04 | T-02,T-03,T-09 |
| R-04 | No per-user/per-tool permissions or RBAC; all valid tokens have identical access to configured servers through the one endpoint. | AC-04,AC-21 | T-03,T-09 |
| R-05 | Independently configured upstream bearer and/or API-key headers; never forward client bearer upstream. | AC-05,AC-16 | T-02,T-04,T-08,T-09 |
| R-06 | Plain HTTP deployment; no built-in TLS termination. | AC-01,AC-21 | T-03,T-09 |
| R-07 | ONE MCP endpoint `/mcp` aggregates every configured upstream server; no namespace layer or namespace routes. Responses expose server ID/status/catalog revision as provenance. | AC-06 | T-02,T-03,T-04,T-05,T-09 |
| R-08 | Stable server-derived tool names, deterministic routing/collision detection across restart/refresh: t_+base32(SHA256(canonical JSON [server_id,raw_name])). | AC-07,AC-16 | T-05,T-07,T-08,T-09 |
| R-09 | Initially advertise search_tools,describe_tool,call_tool plus explicit pins only, not full upstream catalog. | AC-08,AC-12 | T-03,T-05,T-09,T-10 |
| R-10 | Hybrid name/description keyword+embedding search, configurable provider/model/base URL/credentials, bounded top_k and servers[] filter. No namespace field/selector; single-endpoint scope by construction. | AC-09 | T-02,T-06,T-09,T-10 |
| R-11 | Exact schema from describe; validate arguments before forwarding. | AC-10,AC-11 | T-01,T-04,T-05,T-09,T-10 |
| R-12 | Optional explicitly pinned prerequisite tools. | AC-08,AC-11 | T-02,T-05,T-09 |
| R-13 | Persist catalogs/embeddings; content hash and embedding-model identity invalidate vectors. | AC-13 | T-06,T-07,T-09 |
| R-14 | Explicit keyword fallback on embedding failure; never full-catalog fallback exposure. | AC-12 | T-06,T-09 |
| R-15 | Independent connections, deadlines, concurrency limits and breakers; failed server cannot block healthy peers; no execution retry/re-dispatch. | AC-14,AC-15 | T-01,T-04,T-05,T-09 |
| R-16 | Pagination, periodic refresh and supported notifications; retain LKG with explicit stale/unavailable status. | AC-15 | T-04,T-07,T-09 |
| R-17 | Validate YAML/secrets; atomic reload; failed reload preserves working configuration. | AC-03,AC-16,AC-17 | T-02,T-08,T-09 |
| R-18 | Loki-ready JSON logs, credential/payload redaction, correlation, backend/search/cache metrics, Prometheus, separate live/ready health with documented degradation. | AC-18,AC-19 | T-03,T-04,T-06,T-08,T-09 |
| R-19 | No UI/OAuth/RBAC/distributed storage/vector database absent demonstrated need. | AC-21 | T-01,T-09 |
| R-20 | Six critical test groups: initial advertisement; real Portainer/developer discovery/describe/read-only execution; single-endpoint configured-server membership / stable server-derived names; isolated outages/malformed/expired-auth/embedding failures; refresh/reload route/credential safety; failed-reload preservation. | AC-05–AC-20 | T-03–T-10 |
| R-21 | Unit, integration, fault-injection and real omp agent-session E2E test strategy; no invented upstream names/URLs. | AC-20,AC-22 | T-01,T-09,T-10 |
| R-22 | Harness reuse; grounded language/SDK/storage; architecture/YAML/tool schemas/routing/isolation; protocol/capability matrix; phases/ACs/test matrix; separate unresolved decisions. | AC-02,AC-22 | T-01,T-09,T-10 |

R-20 and R-21 retain their test obligations. R-20's original “namespace/name stability” wording is historical only: under the user-confirmed override, CT-03 verifies **single-endpoint configured-server membership and stable server-derived names**. It does not authorize restoring namespaces or weakening any of the six tests.

## 3. EVIDENCE AND UNCERTAINTY

### 3.1 VERIFIED supplied evidence

Supplied coordinator/critic findings are reused, not rerun by this planner.

| Anchor | Fact / use |
|---|---|
| Coordinator initial repository inventory | LICENSE, empty README.md, standard Go .gitignore, initial commit; no implementation/tests. Source locations below are proposed. |
| mcp-codebase-searcher/go.mod, supplied inventory | Go1.25, SDKv1.8.0, jsonschema-gov0.4.3, x/{sys,sync,time,oauth2}; grounded stack choice, not OAuth permission. |
| internal/service/server.go:76 in mcp-codebase-searcher | SDK StreamableHTTPHandler on mux; harness stateless usage does not force bridge inbound statelessness. |
| internal/service/server_test.go:28-90 in mcp-codebase-searcher | Real SDK client session against httptest MCP handler; reuse loopback pattern. |
| internal/coveragecheck in mcp-codebase-searcher | Source-line profile checker/gate; adapt >=90%; discover actual invocation in T-01. |
| internal/observe/log.go; secretMarkers; ScrubCredentials | JSON-lines and credential scrub convention; complete payload omission remains necessary. |
| SDK mcp/streamable.go:1966; mcp/event.go | Transport fields and JSON/SSE. GET/DELETE/Last-Event-ID lifecycle only pre-latest. |
| SDK mcp/shared.go:44-64; capabilities_test.go; examples/server/proxy/main.go | Five dated versions, supported list changes, credential-injecting transport precedent; no actual upstream compatibility proof. |
| SDK mcp/server.go:1952-1966,1977-1985; coordinator verified v1.8.0 source for C8 | ServerSession.handle checks protocolVersion metadata against ServerOptions.SupportedProtocolVersions BEFORE server/discover. Narrowing rejects discover too, with {supported: narrowed list, requested: 2026-07-28}; no server-level discover exemption. |
| SDK mcp/streamable.go:1520-1522,1055-1062; coordinator verified C8 | Transport-level discover exemption does not bypass the earlier server-level exclusion; exclusion error becomes HTTP400. Default server options are required, not a narrowed list. |
| SDK mcp/streamable_test.go:3958 TestStreamableStateful_AcceptsDiscover; coordinator verified C8 | Uses DEFAULT server options (all five); stateful DiscoverResult.SupportedVersions excludes 2026-07-28, leaving exactly the four legacy versions. Prior r3 critic evidence did not test a narrowed server. |
| SDK mcp/shared.go:72-98; coordinator verified C8 | Initialize offering 2026-07-28 negotiates down to 2025-11-25; version offer is not rejected. |
| SDK mcp/streamable.go:1529-1548; coordinator verified C8 | Stateful transport rejects call-based >=2026-07-28 with HTTP400/CodeUnsupportedProtocolVersion advertising the legacy list. |
| SDK mcp/mrtr.go:18-31; ClientOptions.MultiRoundTrip pointer; default retry10 | Explicit nonnil `MultiRoundTrip: &MultiRoundTripOptions{Disabled:true}` prevents default InputRequired re-dispatch. Transport retry control alone insufficient. |
| SDK mcp/client.go:764-864; urlElicitationMiddleware | Unexported, opt-in via AddSendingMiddleware; retries original operation. Ban all upstream re-dispatching middleware. |
| SDK NewInMemoryTransports supplied fact | In-process protocol unit tests. |
| Supplied mcp-rag embedding/index evidence | Provider/model identity, text hash/schema-version invalidation, LKG/stale conventions; conceptual reuse only. |
| Main coordinator's current user-decision handoff | All r4 checkpoints explicitly approved, with namespace removal/single endpoint/flat servers/server provenance/two-element name tuple amendment. U-06 resolved by user in this conversation. No implementation approval is inferred. |

### 3.2 Probe/access record

Cumulative bounded probes: r1 precedent glob, sink existence read, precedent documentation `mcp-rag/.omp/plans/index-process-observability.md:1-100`, unavailable edit-device lookup; r2 affected-contract grep; r3 protocol/middleware grep; r4 editorial-symbol grep; r5 superseded-namespace-contract grep. **8/10 total.** Each probe answered the named question in its tool intent. The extra r4/r5 probes update the historical r3 count; no runtime checks were performed.

Only this artifact was written. No upstream/network probes, source re-derivation, Graphify, LSP, tests, builds, linters, formatters, git actions or implementation changes. Direct reads limited to documentation/authorized sink; source facts reused. Graphify/LSP unnecessary for an empty target and supplied evidence.

### 3.3 Assumptions/consequences

- **[INFERENCE]** Bounded single-process catalog fits exact local search/snapshots; hard limits fail affected refresh, not truncate or add vector DB.
- **[INFERENCE]** Real toolsets expose compatible endpoints; mounted availability proves no URL/auth/version. U-01 remains hard T-10 prerequisite.
- **[INFERENCE]** Validator/SDK preserve schema/number semantics; T-01 characterizes, fail closed on unsupported validation.
- Plain HTTP needs trusted network/loopback; external TLS operator-owned. Catalog metadata can be sensitive; private storage and untrusted-data treatment.
- The identity tuple change intentionally changes draft-era tool IDs. No bridge code/storage exists per supplied inventory; no legacy name/config compatibility shim is planned.

## 4. DESIGN VIEW

### 4.1 Components and stack

Go1.25/SDKv1.8.0/jsonschema-gov0.4.3, cmd/mcp-bridge/main.go and internal/{config,core,observe,service,coveragecheck}, with upstream/catalog/search/storage packages. Proposed strict YAML dependency e.g. gopkg.in/yaml.v3, pinned/characterized T-01; embedding standard net/http/JSON.

Private versioned JSON snapshots, not SQLite: bounded single writer needs atomic replacement without new DB/CGO. Tradeoff whole-snapshot writes/exact O(N×dimensions) search, measured within caps. No vector/distributed store absent reviewed need.

Flow: `/mcp` auth -> one immutable runtime generation -> local global-catalog search/describe or schema router -> dedicated server client -> fixed server endpoint/credentials. Workers independent; persist then publish, no global network lock. Exactly ONE MCP route. `/mcp/<anything>` is404, not alternate routing. Operational endpoints use separate listener and are not extra MCP endpoints.

Default-options server on a stateful StreamableHTTPHandler; NO ServerOptions.SupportedProtocolVersions narrowing. Transport enforces four inbound call versions. Latest authenticated discover returns exactly four legacy supportedVersions; initialize offering latest negotiates down to 2025-11-25; latest tools/call rejected400/CodeUnsupportedProtocolVersion with legacy list/zero execution. No400 for discover/latest initialize offers. --config/--check-config/SIGHUP; no dual mode/admin UI/legacy transport.

### 4.2 Flat YAML schema/example

Synthetic URLs/model/dimensions only; no real upstream/tool compatibility claimed.

```yaml
schema_version: 1
http:
  listen: "127.0.0.1:8080"
  allowed_hosts: ["127.0.0.1:8080", "localhost:8080"]
  allowed_origins: []
  max_request_bytes: 1048576
  requests_per_second: 50
  burst: 100
  max_sessions: 128
  session_idle_timeout: "10m"
operations:
  listen: "127.0.0.1:9090"
  requests_per_second: 10
  burst: 20
storage:
  directory: "./data"
  max_snapshot_bytes: 268435456
catalog:
  refresh_interval: "5m"
  stale_after: "15m"
  max_tools_per_server: 10000
  max_tools_total: 20000
  max_pages_per_server: 1000
  max_schema_bytes: 262144
  max_description_bytes: 32768
search:
  default_top_k: 10
  max_top_k: 50
  timeout: "5s"
  query_cache_entries: 256
  query_cache_ttl: "5m"
  keyword_weight: 0.5
  embedding_weight: 0.5
  rrf_constant: 60
embedding:
  provider: "openai-compatible"
  model: "OPERATOR_SELECTED_MODEL"
  model_revision: "OPERATOR_SELECTED_REVISION"
  base_url: "https://embeddings.invalid/v1"
  dimensions: 1536
  api_key: {env: BRIDGE_EMBEDDING_KEY}
  timeout: "3s"
  max_concurrency: 2
  batch_size: 32
  max_response_bytes: 4194304
client_auth:
  bearer_tokens:
    - {env: BRIDGE_CLIENT_TOKEN}
upstream_defaults:
  connect_timeout: "5s"
  call_timeout: "30s"
  refresh_timeout: "15s"
  max_concurrent_calls: 8
  max_event_bytes: 4194304
  max_response_bytes: 8388608
  breaker:
    consecutive_failures: 3
    open_duration: "30s"
    half_open_probes: 1
servers:
  - id: developer-api
    endpoint: "https://developer-upstream.invalid/mcp"
    credentials:
      bearer: {env: DEVELOPER_UPSTREAM_TOKEN}
      headers:
        X-API-Key: {file: "/run/secrets/developer-api-key"}
  - id: portainer-api
    endpoint: "https://portainer-upstream.invalid/mcp"
    credentials:
      bearer: {env: PORTAINER_UPSTREAM_TOKEN}
    limits:
      max_concurrent_calls: 4
      call_timeout: "20s"
pins: []
# Optional pin: {server: portainer-api, tool: "<EXACT_DISCOVERED_NAME>"}
```

One <=1MiB YAML document, version1, strict unknown/duplicate/alias/merge/coercion/nonfinite rejection. `servers` nonempty flat list; globally unique IDs `[a-z][a-z0-9-]{0,31}`. Maximum1024 server identities preserves the previous total hard ceiling (32×32); no grouping layer. Filter array may select up to32 IDs; omission searches all. Pins top-level unique exact server/tool referencing configured server. Old `namespaces` key rejects; no translation/shim. MCP route fixed `/mcp`, no path selector.

Positive limits: connect/call/refresh<=120s; search/embed<=30s; top_k<=50; slots<=128; inbound sessions<=4096; catalog/snapshot/page/tool/schema/description ceilings as example, increases require revision. Embed dimensions<=4096,batch<=128,concurrency<=32,cache<=4096entries. Weights nonnegative/sum1/keyword>0; positive RRF constant. Relative storage resolved against config directory. Listener/host/origin/storage path/schema restart-only; attempted change rejects entire reload. Check-config local, not connectivity. Protocol/middleware invariants not weakening switches.

Secrets exactly env/file ref; no literals/interpolation/commands/URL secrets. Nonempty UTF-8<=8KiB/noCRLFNUL; strip one terminal file newline. Regular/nonsymlink/operator-owned/private file. Resolve once candidate/reload rotation; client refs/values disjoint upstream/embed; never persist/log. URLs absoluteHTTP(S)/host/no userinfo-fragment-query secrets; normalize scheme/host preserve path/query. Redirects disabled. Unique headers case-insensitively, forbid Authorization(use bearer),Host,Cookie,hop/proxy,MCPsession/version,Origin,X-Request-ID.

### 4.3 Server identity, routing, revision and pins

One global configured-server set. `servers[]` narrows search only, not permissions. Unknown IDs invalid_filter; omitted means all. No namespace parameter is accepted. Every valid client has identical server access through `/mcp`.

Stable public name `t_` + lowercase unpadded base32 of FULL SHA256 over compact canonical UTF-8 JSON **[server_id,raw_name]**. No whitespace/HTML escaping or name case-folding/normalization.54ASCII characters, raw name/server returned separately. Schema/description/order/credentials/generation do not affect identity. Same raw name on different servers produces different public names. Duplicate raw name within one server or digest collision rejects affected refresh and retains LKG; tests inject collision, production full digest.

Globally permanent server_id->normalized endpoint fingerprint with persisted retired tombstones. Different endpoint needs new ID; rebind rejects reload/startup; same endpoint credential rotation permitted. A global catalog revision hashes sorted bound server identities and exact definitions, excluding timestamps/availability/secrets. Search returns revision globally and on each result; describe/call require expected revision. Mismatch catalog_changed/zero dispatch; schema change revision, public name stable. Other-server catalog changes can invalidate an expected global revision; explicit safe refresh, never reroute silently.

Pins exact global server/rawname, no fuzzy match/autoexecution/order/policy. Resolved stable wrapper/exact schema/shared validator/router; missing definition omitted with safe health reason; LKG pin can remain stale/unavailable and safe-fail. Supported tools/list_changed for resolved pin changes only, never advertise other tools. Direct pins capture one active schema/server snapshot.

### 4.4 Tool schemas and provenance

Explicit schemas, no generic derivation; object roots/additionalPropertiesfalse. There is NO namespace field. Passing the removed field is invalid-params, not a compatibility selector.

**search_tools inputSchema:**
```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object", "additionalProperties": false,
  "required": ["query"],
  "properties": {
    "query": {"type": "string", "minLength": 1, "maxLength": 2048},
    "top_k": {"type": "integer", "minimum": 1, "maximum": 50, "default": 10},
    "servers": {
      "type": "array", "minItems": 1, "maxItems": 32, "uniqueItems": true,
      "items": {"type": "string", "pattern": "^[a-z][a-z0-9-]{0,31}$"}
    }
  }
}
```
Effective default/max from config; whitespace/overlimit/unknown server fails, no clamp. structuredContent: catalog_revision,mode(hybrid|hybrid_partial|keyword),finite degraded_reasons,results,backends. Each result explicitly tool_name,**server**,upstream_name,description,score,**catalog_revision,catalog_status,availability**,last_success_at,pinned. Each backend server/status/availability/reason/time/revision.<=top_k/no schema/full catalog. Empty matches distinct from unavailable catalogs.

**describe_tool inputSchema:**
```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object", "additionalProperties": false,
  "required": ["tool_name", "catalog_revision"],
  "properties": {
    "tool_name": {"type": "string", "pattern": "^t_[a-z2-7]{52}$"},
    "catalog_revision": {"type": "string", "pattern": "^[a-f0-9]{64}$"}
  }
}
```
structuredContent: **server,catalog_revision,catalog_status,availability**,tool_name,upstream_name,description,exact inputSchema/outputSchema(ifpresent),untrusted annotations/meta,validation_status(supported|unsupported+safe reason),last_success_at,pinned. Exact JSON/numeric semantics including refs/defs/required/enums/extensions/additionalProperties; whitespace need not match. Raw schema retained; no unsupported rewriting.

**call_tool inputSchema:**
```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object", "additionalProperties": false,
  "required": ["tool_name", "catalog_revision", "arguments"],
  "properties": {
    "tool_name": {"type": "string", "pattern": "^t_[a-z2-7]{52}$"},
    "catalog_revision": {"type": "string", "pattern": "^[a-f0-9]{64}$"},
    "arguments": {"type": "object"}
  }
}
```
Validate captured exact schema BEFORE execution admission. No coercion/default insertion/property deletion/external-ref fetching. Only soundly supported refs/dialects/keywords; unsupported zero dispatch but searchable/describable. Raw/number-aware precision characterized T-01; no poisoning others.

**Execution provenance envelope:** outer CallToolResult preserves upstream content blocks and isError. Its structuredContent contains `server,catalog_revision,catalog_status,availability,tool_name,request_id,upstream_result`. `upstream_result` is the complete original CallToolResult, with original content/structuredContent/isError/metadata preserved unchanged inside it. This explicit envelope supplies user-required provenance without overwriting conflicting upstream fields; upstream structuredContent is compared at `upstream_result.structuredContent`, never coerced/reinterpreted. Ordinary business error remains original, never retry. Bridge metadata may be added in its own envelope; original metadata is retained inside upstream_result. Describe's outputSchema is exact upstream schema, not a claim that the bridge envelope has that schema. Pinned wrappers use the same preservation/provenance contract.

Bridge failures on a resolved tool expose same server/status/revision. Unresolved tool errors set server=null and status=unknown rather than invent provenance; revision is active global revision. Validation detail Pointer/keyword only/no rejected value. Upstream sensitive results go only to authenticated caller, never logs/storage. No resource fetch/content instructions.

**No-replay invariant:** ClientOptions.MultiRoundTrip is POINTER. EVERY factory sets `MultiRoundTrip: &MultiRoundTripOptions{Disabled:true}` (mcp qualification in code). Omitted/nil/zero/nonpointer options fail contract. Ban ALL original-operation re-dispatch middleware, MRTR and opt-in **urlElicitationMiddleware** (unexported, AddSendingMiddleware) and future interaction retries. No callbacks/options enabling it, including new/rotated clients.

InputRequired/NeedsInput/elicitation requests ->unsupported_capability,retryablefalse, exactly ONE original tools/call. After dispatch execution_state=unknown (possible partial effects); no augmentation/client interaction/secondarytool/resend. Error structuredContent safe code/message/request_id/retryable/execution_state plus server/status/revision; no interaction payload.

SDK malformed/method JSON-RPC; bad bridge params invalid-params. Safe error codes invalid_filter,unknown_tool,catalog_changed,unsupported_schema,invalid_arguments,backend_unavailable,backend_auth_failed,backend_busy,deadline_exceeded,upstream_protocol_error,unsupported_capability,internal_error. Execution not_started/unknown,retryablefalse. HTTPauth401/Bearerchallenge,hostorigin403,body413,rate429,unknownroute404. Protocol400 only §4.5 method/header cases, not blanket discover/offer rejection.

### 4.5 Protocol/capability matrix — corrected mechanism, unchanged behavior

Use a DEFAULT-options server on a stateful StreamableHTTPHandler. **NO ServerOptions.SupportedProtocolVersions narrowing.** The SDK server default supports all five versions; stateful transport enforces the four legacy inbound call versions and reports those four in discovery. A narrowed server rejects latest protocolVersion metadata BEFORE the discover case, breaking the required observable contract (C8).

| MCP version | Inbound `/mcp` | Upstream | Behavior |
|---|---|---|---|
| 2024-11-05 | Supported | Supported | Actual Streamable HTTP peers only; older initialize/session/JSON/SSE/GET/DELETE. |
| 2025-03-26 | Supported | Supported | Older stateful/keepalive/GET/DELETE/resume. |
| 2025-06-18 | Supported | Supported | Version-aware older lifecycle. |
| 2025-11-25 | Supported; preferred inbound | Supported | Highest negotiated inbound call version. |
| 2026-07-28 | server/discover ANSWERED; initialize offer DOWN; latest tools/call NOT supported | Supported; preferred if peer supports | Default-options server plus stateful transport: discover reports four legacy versions, initialize negotiates down; stateless latest upstream, no old initialize/GET-SSE/DELETE/Last-Event-ID, no re-dispatch middleware. |
| Other | Compatible offer may negotiate older, otherwise unsupported call/header | Unsupported if no listed agreement | No active-version rewriting/legacy fallback; backend unavailable. |

Authenticated server/discover at2026-07-28 MUST return DiscoverResult with supportedVersions SET exactly the four legacy versions {2024-11-05,2025-03-26,2025-06-18,2025-11-25}, excluding2026-07-28; order = SDK Server.protocolVersions order (newest-first [2025-11-25,2025-06-18,2025-03-26,2024-11-05]), not an oldest-first sequence assertion, and not full tools/catalog. Initialize offering2026-07-28 MUST negotiate down to2025-11-25 via SDK negotiatedVersion; version offer is not rejected in the legacy initialize handshake (discover→fallback initialize@2025-11-25, negotiatedVersion cap). tools/call at2026-07-28 MUST reject HTTP400/CodeUnsupportedProtocolVersion advertising the legacy list, with ZEROexecution. Discover and latest initialize offers through that legacy handshake are POSITIVE fixtures, latest tools/call NEGATIVE. “No400 for latest initialize offers” refers to this legacy handshake, NOT a raw initialize carrying Mcp-Protocol-Version:2026-07-28 on a non-discover path: that raw request IS statefully rejected400. HTTP400 assertions remain method/version-header scoped, never a blanket discover/offer rejection. T-01/T-03 preserve these same observed behaviors with the corrected default-server mechanism.

Latest upstream discover/stateless, no old wire session lifecycle. GET/DELETE/keepalive/resume only pre-latest; POST JSON/SSE where allowed. Inbound tools/listChanged where supported, no resources/prompts/tasks. Upstream no roots/sampling/elicitation/no replay middleware; unsupported interaction safely isolated. Advisory annotations, supported notification delivery only else polling, no invented latest GET/OAuth/task execution.

### 4.6 Auth and credential/session isolation

EVERY inbound method including discover/initialize authenticates same captured generation with constant-time hashed-token compare; duplicate/nonBearer reject. Uniform access. Stateful inbound session binds token fingerprint and the ONE endpoint, never a namespace; different token cannot adopt, revoked token cannot bypass rotation.

Dedicated transport/HTTP client/SDK handle per globally unique serverID/endpoint/credential generation. Older sessions separate/latest no invented session ID. Clone requests/discard inbound headers/set only backend protocol/session(if any), configured creds and safe correlation. NEVER client bearer upstream. Embed separate client/key; no shared cookiejar/header mutation/sessionID crossing.

ALL clients nonnil disabledMRTRpointer; ban all re-dispatch middleware including urlElicitationMiddleware (client.go:764-864), no opt-in callbacks; all versions/initial/new/reloaded/rotated. MaxRetries/HTTP recovery separately characterized. No redirects/OAuthHandler; upstream401/403 backend_auth_failed not client401. Best-effort cancel/no resend; close own handles/wireDELETEonlyolder. Host/Origin/no wildcard cookies; headlessUI/CSRF N/A. No secrets/session/payload/query/URL observability/persistence.

### 4.7 Reliability, refresh, LKG

Per server dedicated handle/call semaphore/breaker/one connect-refresh worker. Local search reads immutable aggregate; server outage never blocks healthy server. Refresh/embedding independent admission; no lock across network. Deadlines min(client/config); whole refresh one pagination budget. Body/event/response bounded; no global write timeout cutting valid SSE; old idle/session bounds only.

No unbounded queue: busy immediate error. Breaker counts connect/transport5xx/malformed/deadline, excludes args/cancel/overload/business/unsupported interaction.401/403 immediate auth-open; thresholdcooldown; halfopen ONE NONEXECUTING oldinitialize/latestdiscover+catalog probe, never tools/call; monotonic fake clock.

No replay any layer. Explicit pointer/middleware ban independent MaxRetries/HTTP/session recovery. InputRequired/elicitation one dispatch/error; oldresume only original event retrieval/noPOST; latest no Last-Event-ID. Count all uncertain faults; failure blocks T-04. Scheduled/coalesced background read-only probes not execution retries.

State catalog_status fresh|stale|none;availability available|unavailable|probing. Failed refresh retains COMPLETE LKG/stale/reason/age; coldcache unavailable until samebound server version-correct connects/discovers. Stale describe/search allowed; call only available+expectedrevision+status. Last-success COMPLETE accepted catalog not TCP/discover. Pagination allpages/loopduplicate/malformed/limits reject candidate, retainLKG; successful empty clears. Poll stagger, notifications coalesce oneactive+onepending through supported channel; healthy publish independently/no all-server startup barrier.

### 4.8 Search/provider/invalidation/cache

Context-bound batch contract -> indexed finite vectors. OpenAI-compatible POST{model,input}base+/embeddings ownkey; validate count/index/dimensions/finite/nonzero/size. Unknown provider reject/no presumed Gemini; new adapter reviewed.

Formatterv1 rawname+newline+description contenthash withformatversion; exact definition hash separatelyrevision. Text/name reembed/schema-only neednot. Modelidentity provider/normalizedbase/model/revision/dimensions/formatversion,nosecret; aliasrevision explicit; content/identity mismatch invalidates, credentialrotatealone reuse. Embedding records keyed by server-derived tool identity+content+model.

Filter configured server set before BM25 Unicodecasefoldnamesdescription k1=1.2,b=.75/exactnormalizedcosine/RRF weight/(constant+rank),stable public name tie/notconfidence. Omittedservers all, no namespaces; <=top_k/noempty-queryallcatalog. Providerfailure keyword/reason/counter,partialvectors hybrid_partial/keywordwholeeligible. Bounded batches independent calls. LRU key query/normalizedservers[]/GLOBALcatalog_revision/modelidentity,TTLentries; vectors/IDs not payloads; current status. No stale removedserver/tool/model revival.

### 4.9 Durable storage

Private directory0700/files0600, exclusive single writer, snapshot.json schema1, RFC3339NanoUTC.

| Collection | Fields |
|---|---|
| Envelope | schema_version,generation_sequence,created_at,config_public_fingerprint,catalog_revision,snapshot_checksum |
| Bindings/tombstones | server_id,endpoint_fingerprint,retired |
| Servers | server_id/fingerprint,negotiated_version,last_success_at,last_attempt_at,catalog_status,safe reason |
| Tools | server_id/rawname/publicname,rawexactdefinition/inputSchema/outputSchema,description,definition_hash,embedding_content_hash |
| Embeddings | toolidentity/contenthash,nonsecretprovider/modelidentitydigest,dimensions,format_version,finitevector |

No namespace columns/keys/config or migration shim. No plaintextendpoint/secretsrefs/session/argsresults/querycache. Publicfingerprintnosecrets/notauth; canonicalchecksum excludingitself; validate bounds/version/checksum/identity/schemas/vectors, corrupt ignored/no destructiveoverwrite/cache_invalid/refresh, no untrustedroute.

Completeimmutable ->samefilesystemtempwrite/fsync ->atomicrename ->dirfsync ->pointer; failure oldtrusted disk/memory, healthoverlay; crash completeold/new, never mixture. Requests pointer not deepcopies. Current flatconfig membership/identity/model reconciliation; removedservers excluded/cache unreachable, tombstones reject rebind. Derivedcatalogvector rebuild, bindingerase explicitrisk. No deployed namespace-era storage exists per evidence; new schema is flat from first implementation.

### 4.10 Atomic reload

1. Serialize publication/reloads; parse/resolve/validate flatservers/pins/secrets/restartconstraints/bindings; immutable auth/router/globalcatalog.
2. Reuse only matchingserveridentity/endpoint/credentials; new/rotated dedicatedclients pointerdisabled/no middleware, never activeheader mutation; remote outage isolated.
3. CompatibleLKG only, removedservers excluded, modelchangevectors invalid, missingpins explicit.
4. Persist candidate; anyfailure closes candidateonly/preserves EXACT config/auth/routes/catalog. Lastgood memory/startupinvalidfatal/notcacheauth.
5. ONEpointer at admission auth+config+serverrouter+catalog+clientobjects; oldcall retains exactserver/schema/creds/deadline, no replay; nextrequest newpointer.
6. Drain oldleases/versionclose/revokedtokencheck/cacheinvalidate/pinnotify. Endpoint stays `/mcp`; membership update never creates routes.

Crashpersistbeforepointer safe validatedconfig reconciliation; remote postreload degradation not mixedrollback. Operator oldvalidYAML explicit.

### 4.11 Observability, server labels and readiness

JSONstdoutLoki: UTC,level,finite event/reason/outcome,request_id,generation,server where resolved,operation,duration/counts. No namespace field. Lifecycle/auth/admission/call/search/fallback/refresh/breaker/reload/ready; omit headers/secrets/sessionIDs/URLs/query/args/results/schema-description/exceptionstack; keyURLscrub defense in depth. Intended result only caller. Interaction/version reasons finite/payload-free.

Correlation32lowerhexelse random128hex/header/envelope/outgoing; childownID+optionalparent,not metric/auth/session. Metrics use finite **server** label, never namespace. Backend labels actualconfigured ID. Aggregate search/cache/request/embedding work uses reserved `_bridge` (not valid configured server ID) or a single configured ID when uniquely attributable; never arbitrary filter combinations as labels. No tool/query/URL/request/token labels.

| Metric | Type/labels |
|---|---|
| bridge_requests_total | counter server,operation,outcome |
| bridge_backend_requests_total | counter server,operation,outcome,reason |
| bridge_backend_request_duration_seconds | histogram server,operation, failuresincluded |
| bridge_backend_inflight | gauge server, slots |
| bridge_backend_breaker_state | gauge server,closed0/open1/halfopen2 |
| bridge_catalog_tools | gauge server,acceptedcount |
| bridge_catalog_age_seconds | gauge server,complete-success age; status/availability separates none |
| bridge_catalog_refresh_total | counter server,outcome,reason |
| bridge_search_duration_seconds | histogram server,mode,includingfallback |
| bridge_search_cache_requests_total | counter server,cachekind,hit/miss |
| bridge_embedding_requests_total | counter server,document/query,outcome,reason |
| bridge_embedding_coverage_ratio | gauge server,validvectors/tools;empty1 |
| bridge_config_reload_total | counter outcome,reason |
| bridge_readiness | gauge unavailable0/degraded1/ready2 |

Bucketsseconds .005,.01,.025,.05,.1,.25,.5,1,2.5,5,10,30,120. live200eventloop/503fatallocalstartupshutdown, notbackendfailure/nohealthnetwork. Ready localvalidconfig+lock+writestore+router AND>=1 availableconnectedserver with complete trustedcatalog(emptyvalidcounts)->200. Ready allservers/pinsfreshavailable/embedusable, else degraded explicit PER-SERVERstatus/reasons.0usableservers orlocalinvalid503; coldcache notready; storagewritefailready503/live200/oldsaferoutes,recoverdurable.200 notallhealthy. Opsloopback/rate/firewall; existing LokiPrometheus/no dashboardsdeploy; README alerts sustainedunready/stale/auth/breaker/errors/fallback. Validate actualmetrics/redaction/healthtransitions.

## 5. ACCEPTANCE CRITERIA

U=unit,I=realSDKloopback,F=controlledfault,E=realomp. Planned notpassed.

| ID | Observable pass/fail | R | Verification/tasks |
|---|---|---|---|
| AC-01 | Single `/mcp` HTTPJSON/SSE;oldGETDELETE;latestupstreamnoremovedlifecycle;no legacy/stdio/TLS. | R-01,R-06 | I/review;T-01,T-03,T-04,T-09 |
| AC-02 | Default-optionsserver/statefulStreamableHTTPHandler;NO SupportedProtocolVersions narrowing. Latestdiscover answered/supportedVersions SET exactly the four legacy versions, order = SDK Server.protocolVersions order, excludes2026-07-28;latestinitializeofferDOWNto2025-11-25;latesttools-call400/CodeUnsupportedProtocolVersion advertisinglegacylist/zeroexecution. No400discover/latestoffers;method/header-scopednegative. FIVEupstreams correctlifecycle. | R-01,R-22 | U/I/F;T-01,T-03,T-04,T-09 |
| AC-03 | FlatYAML valid; retired `namespaces` key rejected as unknown; duplicateglobalIDs/invalidkeysboundsURLrefssecrets rejectbeforeactivation. | R-02,R-17 | U/checkconfig;T-02,T-09 |
| AC-04 | Bad auth401 ALL methods/discover;uniformaccess/tokenboundsession, no OAuthRBAC. | R-03,R-04 | U/I;T-02,T-03,T-09 |
| AC-05 | Clienttoken neverupstream/embed; intendedservercreds only/no redirect/session/crossserverleak. | R-05,R-20 | I/F;T-02,T-04,T-08,T-09 |
| AC-06 | Exactly ONE `/mcp` aggregates ALLandONLY configuredservers;removed/unconfigured excluded;`/mcp/<id>`404;responses server/status/revision correct;no namespaceparameteraccepted. | R-07,R-20 | U/I/F;T-02,T-03,T-04,T-05,T-09 |
| AC-07 | Names equal two-elementserver/rawname formula;same rawname acrossservers distinct;reorder/restart/schema-description edits stable;digestduplicate/rebind reject. | R-08,R-20 | U/I/F;T-05,T-07,T-08,T-09 |
| AC-08 | Initialthree+explicitresolvedpins ONLY;pinboundcalls/changesnotify correct,no fullcatalog. | R-09,R-12,R-20 | I/E;T-03,T-05,T-09,T-10 |
| AC-09 | Hybridfixtures/ties/configprovider/bounds;omittedservers searchesall,filteronlyknownIDs;unknownreject;NO namespacefield/schema/selector. | R-10 | U/I/E;T-02,T-06,T-09,T-10 |
| AC-10 | Describeexactschema/numbers/refs/extensions;unsupported marked/notrewritten, server/revision/status accurate. | R-11,R-20 | U/I/E;T-01,T-05,T-09,T-10 |
| AC-11 | Invalid/unsupported/stalerevision zero;valid/pin once unchangedargs;originalresult preserved inupstream_result inclstructuredContent/meta/content/isError;outerserverstatusrevision accurate. InputRequired/elicitation no redispatch/allfactorynonnilDisabledpointer/no retrymiddleware. | R-11,R-12,R-20 | U/I/F/E;T-01,T-04,T-05,T-09,T-10 |
| AC-12 | Embedfaults boundedkeyword/reason/no fullcatalog. | R-09,R-14,R-20 | U/I/F;T-05,T-06,T-09 |
| AC-13 | Flatcatalog/vector restartreuse/invalidation/schema-onlyreuse;corruptnevertrusted/no secretsor executionstored. | R-13 | U/I/F;T-06,T-07,T-09 |
| AC-14 | Healthyserverwhilefaulty/limits;countoneEOF5xxoldexpiryresume/latestInputRequired+elicitation;pointer+allmiddlewareban independenttransportproof. | R-15,R-20 | I/F;T-01,T-04,T-05,T-09 |
| AC-15 | Completepagesonce/partialLKGexplicit/pollsupportednotify/recovery/nonexecutingversionprobe. | R-15,R-16,R-20 | U/I/F;T-04,T-07,T-09 |
| AC-16 | Concurrentflatmembershiprefresh/reload/call retainsONEschema/server/creds generation;removedstalecache neverwrongtarget/leak. | R-05,R-08,R-17,R-20 | I/F/race;T-05,T-07,T-08,T-09 |
| AC-17 | FAILEDreload preservesconfigauth/globalcatalog/workingcall atSAMEserver/creds;syntaxsecretimmutablepersistfaults;validswapdrainno replay. | R-17,R-20 | Dedicated§6.3U/I/F;T-02,T-08,T-09 |
| AC-18 | JSONparse/correlate/no payloadsentinels;metrics serverlabels only/finite_actualbackendsearchcachechanges. | R-18 | U/I/F;T-03,T-04,T-06,T-08,T-09 |
| AC-19 | Backendoutage live200;anyhealthyserver ready200/degraded;zero/storage503;recover/per-serverdocs/no healthnetwork. | R-18 | I/F;T-03,T-04,T-08,T-09 |
| AC-20 | Realomp THROUGHsinglebridge endpoint BOTHactualtoolsets/search/exactschema/verifiedread-onlysuccess;realprovenance/evidence/no inventednamesURLs. | R-20,R-21 | E;T-10 |
| AC-21 | No prohibitedstack/namespaceabstraction;docs trustcaps/protocolrestrictionsdiscoverexception. | R-04,R-06,R-19 | Review;T-01,T-09 |
| AC-22 | Fullmapping/views/contracts/matrix/observability/openfacts/4tiers/>=90line/6criticalgroups;scopeuserdecision recorded,nocriterionweakened. | R-21,R-22 | Critic/evidence;T-01,T-09,T-10 |

## 6. TASKS VIEW AND TEST STRATEGY

### 6.1 Common contract

Proposedsourcepaths only. Tdd-guide directsengineertests+code; each target/assertion/exercisedRED/unchangedGREEN/exitevidence. IntentionalmissingAPIcompileREDallowed; unrelatedsetupnot. No existingbridge regressions; retainnewtests/docsnofakeRED. Commandsvalidationintent notpermission; assignedRED/GREENauthority/mainconsolidated checks.

Root /home/opencode/projects/default/mcp-bridge; searcher mcp-bridge/precedentmcp-codebase-searcher. BoundedMCP/findfirstunknown/Graphifycorrectverifiedprojectfreshcleanimmediatelyeachneveredited/LSPsymbols/narrowexplainedfallback/freshanchors. Noverifiedgraphsymbolnow; propagatepermittedhandoffs/plannerdispatchnone. Afterapprovals bridgecode/tests/deps/sampleREADME/evidence only; no upstreammutations/secretprinting/globalharness/infra/unrelated/fullcatalog/gateexceptions.

### 6.2 Phased ledger

#### Phase A — grounded local contracts/config

**T-01 Dependency/protocol/no replay**
- Dependsnone. R R-01,R-11,R-15,R-19,R-21,R-22; AC-01,02,10,11,14,21,22.
- Inputs suppliedSDK/harnessanchors and C8 correctedmechanism; proposedgo.mod/sum/internalcoveragecheck/serviceprotocol/upstreamtransport/catalogschema tests.
- RED version/schema/numberintendedassertions,executioncounter>1oldloss/latestInputRequired/elicitation,under90checkerfailure.
- Deliverablepinlibs/default-optionsserveronstatefulStreamableHTTPHandler/NO SupportedProtocolVersions narrowing;latestdiscoverpositive/exactFOURlegacysupportedVersions+latestinitializeofferDOWNto2025-11-25+latesttoolscall400CodeUnsupportedProtocolVersionlegacylist/zeroexecution/header-vsoffer;allclientnonnilDisabledpointer/nomiddleware/transportcontrols/schema-numbermatrix;discoveractualcoverageAPIboundedMCP/LSP.
- Planned go test ./internal/service ./internal/upstream ./internal/catalog ./internal/coveragecheck.
- Stopwronglatestexecution->T-03/replay->T-04/precision->T-05blockedreviewnotweaken.

**T-02 Flatconfig/secrets**
- DependsT-01. R R-02,R-03,R-05,R-07,R-10,R-12,R-17; AC-03,04,05,06,09,17.
- Inputs§4.2flatservers/pins; internalconfig/command/sampleREADMEtests.
- RED globalduplicateIDaccepted,oldnamespaceskeyaccepted,missingsecretoverlapaccepted,validmissing; no invalidactivation.
- Deliverableimmutableflatparser/refs/bounds/checkconfig/nosecretexample; no oldmodelshim.
- Planned go test ./internal/config; private synthetic go run ./cmd/mcp-bridge --check-config --config <synthetic-test-config> smoke/no secretoutput.
- Stopparserunsoundrevise notaccept.

#### Phase B — single endpoint/isolation

**T-03 ONE inbound endpoint/auth/operations**
- DependsT-01,T-02. R R-01,R-03,R-04,R-06,R-07,R-09,R-18,R-20; AC-01,02,04,06,08,18,19.
- Inputs§4.1/4.5/4.6/4.11 and C8; internalservice/observe/commandtests.
- RED `/mcp/<id>` admitted or`/mcp`missing;wronglist/auth/sessionownership/leak/live;latestcallaccepted/discover400/offerfailsDOWN.
- DeliverableONEstatefulSDKhandlerwithdefault-optionsserver/NO SupportedProtocolVersions narrowing/tokenbinding/noroutelayer;allupstreamaggregationviarouter;authenticatedlatestdiscover/exactfourlegacy supportedVersions/latestinitializeofferDOWNto2025-11-25/latesttools-call400CodeUnsupportedProtocolVersionlegacylist/zeroexecution;admissionlogshealth. Productionnostubs/noops/mocks/partialrelease;testtools onlytest,completehandlersT-05/T-06.
- Planned go test ./internal/service ./internal/observe;routeexactness/fourversions/positiveDiscoverDOWN/negativeTOOLS-CALL,notnegative discover.
- Stoplatestexecution/wrongdefault-servermechanism/wrongroute/auth;no dualmodeor namespaceabstraction.

**T-04 Dedicatedserverclient/no replay**
- DependsT-01,T-02. R R-01,R-05,R-07,R-11,R-15,R-16,R-18,R-20; AC-01,02,05,06,11,14,15,18,19.
- Inputscriticconfirmedpointer/middleware/version; internalupstream/metrics/faultpeers.
- RED execution>1EOFoldlosslatestInputRequired/elicitation,defaultnilpointer/optinretry/tokenleak/healthypeerblockinglimits.
- DeliverablePERserverIDclient/endpoint/credgeneration/nonnilMRTRdisable/ALLredispatchmiddlewarebaninclurlElicitationMiddleware/callbackpath/versionlifecycle/cancel/probes.
- Planned go test ./internal/upstream;factoryoptions/countfaults/fiveversions/rawHTTPbarriers/oldresumeonly.
- Stoponecallisolationunproven->BLOCKEDreviewedtransport,MaxRetriesaloneinsufficient.

#### Phase C — globalcatalog/search/storage

**T-05 Globalserveridentity/describe/call/pins**
- DependsT-01–T-04. R R-07,R-08,R-09,R-11,R-12,R-14,R-15,R-20; AC-06,07,08,10,11,12,14,16.
- Inputs§4.3/4.4two-elementtuple/globalrevision/provenanceenvelope; internalcatalog/core/registrationtests.
- RED wrongdigest/identicalrawnamescollapse/acrossserverwrongroute/membershipleak/collisionoverwrite/invalidstaledispatch/schema-number/resultenvelopedataaltered/pinwrong/repeat.
- DeliverableflatglobalIDbindings/router/stagedcatalog/revision/exactdescribe/soundvalidate/oneexecution/provenancewithunalteredupstream_result/pins.
- Planned go test ./internal/catalog ./internal/core ./internal/service;two+serversSAMErawname,distincttoolnames,globalrevision/provenance,schemasprecision/contentmeta fixtures.
- Stopprecision/route/leak/resultloss;unsupportedrealvalidationcannotpassE.

**T-06 Globalhybridsearch/filter/cache**
- DependsT-02,T-05; U-06 resolvednochoicegate. R R-10,R-13,R-14,R-18,R-20; AC-09,12,13,18.
- Inputs§4.8/globalcatalog/servers[]deterministiccorpus; internalsearch/provider/cache/metrics.
- RED omittedfilterfailsdiscoveroneserver/filterreturnsunselected/namespacefieldaccepted/overlimit/fullfallback/semanticabsent/stalemodel.
- Deliverableallconfiguredsearchorserverfilter/BM25cosineRRF/provider/currentvectors/bounds/globalrevisioncache/provenance/no grouping.
- Planned go test ./internal/search;modelURLkey/indexfinite/dimension/filter/cache/faultfixtures.
- StoprealproviderU-04,notfabricatedcompatible; namespaceadditionoutofscope.

**T-07 Flatdurablecatalog/refresh**
- DependsT-04,T-05,T-06. R R-08,R-13,R-16,R-20; AC-07,13,15,16.
- Inputs§4.7/4.9globallyboundserverID; internalstorage/catalogrefresh/restarttests.
- RED partialdeletes/wrongservercache/rebindrestart/removedserverrevival/mixedcrash/notificationstorm/latestGET.
- Deliverableflatsnapshotglobalrevision/tombstones/privateatomiclock/reconcileconfiguredmembership/independentpoll-supportednotify/persistpointer/status.
- Planned go test ./internal/storage ./internal/catalog;two-serverrestart/reorder/remove/schemachange/hash/crashdiskfsyncrenamelocklimits.
- Stopfilesystemguaranteemissingdeploymentblocked/approvedalternative;no legacy namespace migration.

#### Phase D — reload/local gates

**T-08 Atomicflatmembershipreload/drain**
- DependsT-02–T-07. R R-05,R-08,R-17,R-18,R-20; AC-05,07,16,17,18,19.
- Inputsgeneration/flatservermembership/safety; coreconfigservicecommandtests.
- RED FAILEDreloadlosesworkingconfigauthserverroutecatalogcall;removeaddmix/wrongcreds/cache/replay/revokedbypass.
- DeliverableSIGHUPcandidatepersistONEpointer/explicitservermembership/isolatedrotatedclients/drain/cachehealth/ONErouteunchanged/allmiddlewarebanpreserved.
- Planned go test ./internal/config ./internal/core ./internal/service;SIGHUP/barriers/dedicatedsyntax-secret-immutable-persistfailure andremove/add/rotate.
- Stopwrongrouteleakretainlastgoodfix,notrelax.

**T-09 Consolidatedgate/docs**
- DependsT-01–T-08. R R-01–R-22local; AC-01–19,21,22.
- Inputsunchangedregressions/§6.3;package/integration/fault/checker/README/sample.
- RED crossfaultsrepeat/leak/FAILEDreload/protocol;singleendpointold-route404/no namespaceinput/provenanceandserverlabels failures;docsno fakeRED.
- Deliverablefullsuite/>=90source-line/racevetbuild/review/docsflattenedconfig/serverprovenance/statusglobalrevision/noshamcompat;criticconfirmedprotocolallmiddleware preserved.
- Planned go test ./...;go test -race ./...;go vet ./...;go test -covermode=atomic -coverprofile=coverage.out ./... +exactcheckerdiscoveredT-01;go build ./cmd/mcp-bridge.
- StopACsecurityrace<90/missingevidence;no criticalskips/coverageexclusions/smokesubstitute;localnotE.

#### Phase E — real omp

**T-10 RealPortainer/developer — BLOCKED**
- DependsT-09/U-01–05asapplicable/authorizedrealaccess. R R-09,R-10,R-11,R-20,R-21,R-22; AC-08,09,10,11,20,22.
- InputactualURLsauthversionscapabilities/safereadonly/catalog/embed/tasklocalomp/evidence; U-01 HARDprecondition. Agent mustnegotiateFOURcallversions(latestofferDOWNokay/latestcallonlynot);discoveryaloneinsufficient.
- Actualbridge`/mcp`frontsBOTHrealservers;agent THROUGHthisONEendpoint;privateevidence/noglobalconfig.
- DeterministicorchestrationREDlocal;agentmodelsemanticobservednotfakeREDGREEN.
- Deliverablelist/searchwithserverfilters/exactdescribe/read-onlyboth/serverprovenance/globalrevision/stablenames/membershipthroughbridge,nodirectmountedsubstitute.
- Coordinatoractualompdocs/permittedmount/start/actualcommandsessioninvocations;no inventedflagsnamesURLs.
- Stopunknown/incompatibleaccess/schema/provider/safety/mount/inconclusive->BLOCKEDnot skippedpass;productionfaultsprohibited/localpeersT-09.

### 6.3 Four-tier matrix and six critical tests

| Tier | Method | Required cases |
|---|---|---|
| Unit | Test-firstGo/inmemoryMCP/fakeclockproviderdigestfs | FlatYAML/globalIDs/secrets/twoelementhash/collisions/schemasnumbersrefs/filter/RRF/bounds/models/cache/breaker/log/snapshot/ready/factorydisabledpointer/no middleware. |
| Integration | SDKbridge+independenthttptest MCP/embed/private tempstore/process | EXACT `/mcp`only/oldpaths404/allandonlyservers/same rawnameprovenance/filter/no namespacefield;default-optionsstatefulserver/NO SupportedProtocolVersions narrowing/fourcallversions/latestdiscoverexactFOURlegacy+DOWNto2025-11-25positive/latesttools-call400CodeUnsupportedProtocolVersionlegacylistzeroexecutionnegative;fiveupstreams/auth/session/listpins/results/provenance/credcapture/refreshreloadrestart/metricsserverlabels. |
| Fault | RawHTTP/SSE/barriers/clock/fs/crashes | Hanglimitsdeadline/expired401403/5xxEOFafterdispatch/oldexpiryresume;malformedoversizepartialcursors;embedNaNdimensionsindexauth timeout;diskchecksumversionfsyncrenamelock;concurrentserveraddremovecredentialrotate/wrongrouteleaks/healthyserverisolation. |
| Eomp | Actualagent THROUGHsingleendpointtoBOTHactualservers | Initialthree+pins/keywordparaphrase/serverfilter/exactschema/safereadonly/provenance/globalrevision/stablenamesmembership/compatiblecallprotocol;missingU-01blocked. |
| ExplicitF latestInputRequired | LatestpeerreturnsInputRequiredaftercountedcall/allnonnilDisabledpointers | EXACTLYone/unsupportedcapability/retryablefalse/unknown/no augmentationresend/leak/healthyserver;AC-11/14,T-01,T-04,T-05,T-09. |
| ExplicitU/I/F elicitationban | No opt-in urlElicitationMiddleware;peerattemptsSDKinteractionretryfixture | No callback/re-dispatch,originalcall<=1/safeunsupported;initialrotatedclientfactoryassertions;AC-11/14,T-01,T-04,T-09. |
| ExplicitU/I/F FAILEDreload | Syntaxunknownkeys/missingsecret/immutableedit/persistfailure/SIGHUPbarrier | Exactoldauthconfigcatalog/serverID/endpointcreds/workingcall;no candidateleak/replay/safe logmetric;AC-17,T-02,T-08,T-09. |

| Critical test | AC | Tier | Tasks |
|---|---|---|---|
| CT-01 Initialthree+explicitpins | AC-08 | I,E | T-03,T-05,T-09,T-10 |
| CT-02 Realboth discovery/describe/read-only | AC-09,AC-10,AC-11,AC-20 | E;localfixturesseparate | T-06,T-09,T-10 |
| CT-03 SINGLE-ENDPOINT configured-server membership/stable server-derived names | AC-06,AC-07 | U,I,F,E | T-02,T-03,T-04,T-05,T-07,T-08,T-09,T-10 |
| CT-04 Outage/malformed/expiredauth/embed isolation | AC-12,AC-14,AC-15,AC-19 | I,F | T-04,T-06,T-07,T-09 |
| CT-05 Refreshreload nowrongserver/credentialleak | AC-05,AC-16 | I,F/race | T-04,T-05,T-07,T-08,T-09 |
| CT-06 FAILEDreloadworkingconfigretained | AC-17/dedicatedrow | U,I,F | T-02,T-08,T-09 |

Emanifest BEFOREoutcome capturedactualcatalog>=keyword+meaningfulparaphraseEACHserver(>=4queries),expectedactualIDs/top_k/serversfilters/verifiedsafeargs/resultproperties. Recordmode/rankedexpectedwithin_k/serverprovenance/membership. HealthyHYBRIDparaphrase required,fallbacknotsemanticpass. Exactdescribe andsuccessfulverifiedreadonlyeachserver. Originalcallresultcomparedinupstream_result;bridgeprovenanceconsistent. Agent/modeloutcomesobserved/schemaassertionsdeterministic;no posthoc easierfixturewithoutapprovedrevision. Retainallnewregressions/focusedunchangedGREEN+fullsuite/race/vet/build/>=90line,Eseparate. InputRequired/elicitationmandatoryevenrealserversolder;no criticalskips/weakening.

## 7. RELEASE AND RECOVERY

Planningonly/noimplementationdeploycommitgate. Futureaftercoordinatorreconciliation:localgates/reviews/actualU-01–05/r5evidence/callcompatibility. Operatorprivatestore/separatesecrets/checkconfig/trustedHTTP/opsfirewall;noglobalharnessorregistry.localdomain:5000writes. ONEprocess/writer/endpoint,readinessper-serveraggregate. RealapprovedompE BOTHthrough`/mcp` beforecutover;blockedAC-20notrelease. AtomicYAML/SIGHUPfailureoldretained;binaryformatstopprivatebackupcompatible/no distributedmigration.

RollbackpriorvalidFLATYAML orbinary/compatibleprivatebindingsnapshot;secretsoperator/notcache,unsupportedformatnooverwrite/derivedrebuild/bindingsnoterase/startupinvalidfatal. No obsolete namespace routes/config/names aliases. The user-changed identity tuple intentionally invalidates draft-era client manifests/fixtures; there is no implemented prior model to migrate. Latestcall-onlyclientincompatible;discover/DOWNnotcallproof;dualexecutionneedsnewcritic/checkpoints.

Calls maymutate/partiallyexecute beforeInputRequired/elicitation/lostresult;unknown/no automatic repeat,configrollbacknotundo. Realacceptanceonlyverifiedreadonly/pinsneverauto/canceldrainbest effortnotdistributedtransaction.

## 8. CRITIC HANDOFF, RISKS AND OPEN ITEMS

### 8.1 Unresolved Decisions / Facts

**Unresolved count: FIVE** (U-01–U-05). U-06 ID retained separately as RESOLVED, not listed as an open checkpoint question.

| ID | Missing fact / concrete resolution | Affected |
|---|---|---|
| U-01 | ActualserverURLs/auth/version/capabilities ANDompnegotiatedcallversion;coordinatorauthorizedmetadata/versioncorrectinitialize(old)/discover(latest)+list+actualcompatiblecall. | HARDT-10precondition(C7);mounted/discoveraloneinsufficient. |
| U-02 | Actualpins/safereadonlynamesargs/assertions;catalogprimaryguidance/independentsafety,emptypinsvalid. | RealE/pinconfig. |
| U-03 | SDKenforcement/schema/number/no replaycharacterization;known default-optionsstatefulmechanism/nonnilMRTRpointer/nomiddleware. | T-03protocol/T-04replay/T-05validation stop gates. |
| U-04 | RealembeddingcontractmodelrevisionURLkeydimensions/approvedfinitevectorprobe. | RealhybridE/no assumedGemini. |
| U-05 | Filesystemprivacyatomicity/networkresources/task-localomp/evidencesink;coordinatoractualverification/measurement. | Deployment/E/cap-storagechangesrequire reviewedrevision. |

**U-06 — RESOLVED, USER-CONFIRMED in this conversation:** drop namespace layer entirely; ONE `/mcp` fronts all flat configured servers; server ID/status/globalcatalog revision provenance, servers[] narrowing; name tuple [server_id,rawname]; top-level servers and pins. Resolution supersedes earlier endpoint-local-vs-cross-namespace question. No further user choice about namespace filtering remains. Coordinator reports r4allthreecheckpoints approvedWITHthischange; r5affectedcontracts reconciled bycoordinator, not automaticplannerapproval.

### 8.2 Risks and critic questions

- Two-elementhash is deliberate breaking draftidentity change;stablewithinr5 thereafter. GlobalserverIDs must be unique;rebindtombstones preventoldnamestargetingdifferentendpoint.
- Globalcatalogrevision mayinvalidatecallswhenunrelatedserverdefinitionchanges;safe catalog_changed,zero dispatch,explicitrefresh,neverimplicitwrongroute.
- Provenance envelope mustretainoriginalresult intact atupstream_result;ensurecontent/numbers/metadata/structuredContent assertionscovercolliding upstreamfieldnames.
- ONEendpoint notsharedheaders/sessions/slots/breakers: all remainPERserver. Failedpeer/readinessdegradedmustnotblockhealthy.
- Latestdiscover/DOWNnotlatestexecution;U-01hard. MRTRpointer/opt-inurlElicitationMiddleware/defaultHTTP recovery counted separately.
- Exactschemasfailclosed/privateatomiccrash/no payloadlogs/serverlabelbounds/200degradedwith503zero-storage/actualsafeE unchanged.

### 8.3 Critic history and r5 scope disposition

| Finding/decision | Retained or amended contract |
|---|---|
| C1/C5 behavior retained; mechanism corrected by C8 | FOURstatefulcallversions/FIVEupstream;DEFAULT-optionsserver/statefultransport/NO SupportedProtocolVersions narrowing;latestdiscoverexactfourpositive/initializeDOWN/toolscallnegative;method-header-scoped400. |
| C2/C6 resolved | ExactClientOptions nonnil MultiRoundTrip:&MultiRoundTripOptions{Disabled:true};ALLre-dispatchmiddlewareban;InputRequired/elicitation counted. |
| C3 resolved | SixcriticalACtierTmatrix/dedicatedFAILEDreloadAC-17. |
| C4/U-06 superseded by explicituserdecision | FilteringquestionRESOLVED by NO namespace layer;serverfilteronlyONEendpoint. |
| r4 editorial confirmed | Correct urlElicitationMiddleware,client.go:764-864/unexported/opt-in AddSendingMiddleware;no semanticrollback. |
| C7 unchanged | U-01hardrealacceptanceprecondition. |
| r5 user scope amendment | R-07,R-08,R-10 changed;dependentR-04/R-22 wording translated;R-20/R-21testobligationsretained. FlatYAML/serveridentity/provenance/schema/filter/storage/cache/metric/ready/task/CT-03contracts updated. |
| C8 mechanism-only correction in r7 | r3 critic discovery evidence used a DEFAULT server; prior SupportedProtocolVersions narrowing mechanism was incorrect. server.go:1952-1966 exclusion precedes1977-1985discover,streamable.go:1055-1062makes400;TestStreamableStateful_AcceptsDiscover:3958/defaultoptions andstreamable.go:1520-1522,1529-1548/shared.go:72-98 groundcorrectmechanism. Observable discover/offer-down/call-rejection contract UNCHANGED;stableIDs/no weakened or strengthened acceptance/no new approval. |
| C9 applied in r8 from BridgeCritic5 confirmation | Discover supportedVersions is exactly the four legacy versions as a SET; sequence follows SDK Server.protocolVersions newest-first, excludes2026-07-28, no oldest-first assertion. No400 for latest initialize offers means legacy discover→fallbackinitialize@2025-11-25/negotiatedVersion cap; raw non-discover initialize with Mcp-Protocol-Version:2026-07-28 IS rejected400 under method/version-header scope. Clarifications only;stableIDs/behaviorunchanged/no new approval. |

Changed ACs: AC-01,AC-03,AC-04,AC-06,AC-07,AC-09,AC-10,AC-11,AC-13,AC-16,AC-17,AC-18,AC-19,AC-20,AC-21,AC-22 (scope/provenance only;protocol/no replay/coverageguarantees unchanged). Tasks T-02,T-03,T-04,T-05,T-06,T-07,T-08,T-09,T-10 and dependentfixtures/docs need r5 flatmodel; T-01safetycharacterization unchanged. All IDs preserved/no droppedacceptance/no hiddencompatshim.

r4 explicitapprovals are recorded as supplied user decision, notfabricated. Its superseded namespace contracts cannot authorize restoring that design;coordinatorreviews/persistsr5and reconcilesaffectedcheckpoints. Thisplannerneitheropensimplementationgate norclaimsnewr5criticapproval.

### 8.4 Self-evaluation (not approval)

Accuracy4/5: supplieduserdecision/criticAPI grounded,implementationcharacterization notrun. Completeness4/5:22R/fullmapping/flatcontracts/4tiers/6criticalgroups;realfactsblocked. Clarity4/5:ONEendpoint/serverprovenanceexplicit;densecontractcriticreviewneeded. Actionability4/5:boundedtasksREDcommandsstops,U-06resolved/U-01hard. Conciseness4/5: centralizedcontracts/traceabilityrepetition. Overall4.0/5. NextfactsU-01 actualcall/T-01schema-safety/U-04provider;no implementedorpassedbehaviorclaimed.
