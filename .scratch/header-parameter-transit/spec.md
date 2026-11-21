# Header Parameter Transit — Header-Annotated Tool Parameters Through the Bridge (SpecDD, feature `header-parameter-transit`)

## 1. STATUS

Spec ID `header-parameter-transit`, revision r6. State: **DRAFT** (targeted repair of the r5 code-review findings F1–F6 plus the two r5 open questions; ready for staged independent review; no execution authorization). Objective: a `tools/call` for a tool whose published input schema annotates parameters with `x-mcp-header` must reach a modern upstream through the bridge's normal call path in the default `stateless: true` configuration, without pinning a session and without weakening restart resilience, while the explicit-stateful path and the legacy-protocol boundary are deterministic and named. Evidence scope: backlog MB-001, tickets `issues/01`–`issues/04`, the working tree and go-sdk v1.8.0 sources cited in §3.

**Revision history.** r1 (initial). r2 (CR1 REQUEST CHANGES). r3 (revised text; superseded unreviewed). r4 (full re-plan: every mechanism, cost statement, oracle and command re-derived from the tree and the SDK; r1/r2/r3 treated as evidence of prior failure, not a baseline to preserve). r5 (targeted CR3 repair; handshake/pagination/accounting/TTL/vocabulary oracles repaired). **r6 repairs the r5 code-review verdict (F1–F6: 2 MEDIUM/HIGH, 3 LOW/HIGH, 1 LOW/MEDIUM) and the two r5 open questions with targeted amendments; no R/AC/T id is added, removed, merged, renumbered or weakened, the envelope write set is unchanged, and no ticket criterion changes meaning.** F1 is closed by strengthening the design, not by narrowing R2: the annotation index is now installed as the *union* of the previous index and the new successful discovery's header-bound set, which makes the non-under-approximation property continuous — at every instant the index is a superset of the header-bound set of the published catalog — so R2's absolute last sentence stands verbatim and the r5 install-scoped I2 is superseded (AD11). F2: AC02-1 and AC03-1 each assert the peer records exactly one `tools/call` request on the success path. F3/F4: the stale anchors `cmd/mcp-bridge/wiring.go:142`→`wiring.go:177-179` and `go.mod:11`→`go.mod:8` are corrected (both re-verified in the tree). F5: the stateless call sequences now place connect before admission, matching `internal/upstream/dedicated.go:377` and `internal/upstream/runtime.go:190`. F6: the call-timeout claim is scoped to the post-admission window (the timeout context is created in `ServerClient.CallTool`), and the go-sdk anchors `client.go:1277-1300`→`client.go:1264-1277` (`put` call at `client.go:1276`, definition at `cache.go:45`) and `features.go:76-99`→`features.go:99-104` are corrected. Open question 1: AC03-2 now asserts the *observed negotiated version* `2025-11-25` rather than a wire error, because a legacy-pinned peer answers the new-protocol probe with HTTP 200 and a legacy `DiscoverResult` (discover is exempt from the stateless check, go-sdk `streamable.go:1520-1523`) and the `CodeUnsupportedProtocolVersion` fallback error is synthesized client-side (go-sdk `client.go:443-451`). Open question 2: the AC03-1 positive-TTL fixture pins `TTLMs` to a value strictly longer than the entire bounded test interval (60,000 ms against a test whose internal awaits are second-scale), so expiry cannot break the exact-zero list delta.

**r5→r6 plan-change continuity.** Three completed failed plan-change cycles exist (r2, r4, r5 REQUEST CHANGES); r3 was superseded unreviewed. r6 is the next meaningfully different repair: F1's fix strategy is new (union install superseding the snapshot-scoped I2, chosen because r5's textual scoping was the rejection ground), and F2's oracle addition is new evidence. No admission, retry counter, review identity or historical record is reset by this revision.

## 2. REQUIREMENTS
<!-- specdd:requirements:start -->
**R1 (default stateless transit).** With default configuration (`stateless` unset ⇒ true, `internal/config/config.go:48-52`), a `tools/call` for a tool whose last successful discovery published an input schema containing an `x-mcp-header` annotation must reach the upstream as exactly one execution whose outbound HTTP request carries the parameter values both in the JSON `arguments` object and in matching `Mcp-Param-<header-name>` request headers. No upstream session may be retained between calls to achieve this.

**R2 (single emitter, bridge ensures the definition).** The bridge must not construct, encode, re-encode or strip `Mcp-Param-*` headers. Emission must be the go-sdk client's, driven by the tool definition present in the executing session's `tools/list` cache. The bridge's only responsibility is to ensure that definition exists on the execution session for tools it knows to be header-bound; per-parameter selection and value encoding are the SDK's SEP-2243 rules. The bridge's knowledge must be derived from the same published schema the catalog serves and must never disagree with it in the under-inclusive direction for an already-published tool. *(r6: unchanged verbatim; the r5 requirement/design contradiction is resolved by the continuous union-install invariant in §4 I2, which makes this absolute sentence achievable in-envelope.)*

**R3 (control preservation).** A tool with no `x-mcp-header` annotation must behave exactly as today: no additional upstream RPC, no additional header, identical arguments bytes and identical result content.

**R4 (argument retention).** Header-bound values must remain in the JSON `arguments` object, byte-preserved through the existing decode/encode path; the headers are additive. Removing a value from `arguments` is prohibited because the SDK peer rejects a header whose body parameter is absent and rejects a mismatch when it is present.

**R5 (explicit stateful mode).** With `stateless: false`, a header-bound tool must succeed using definitions present on the runtime's single cached session; no extra session, no second execution. When the peer's advertised list TTL is positive and unexpired, the call must perform no definition RPC; when it is zero (or expired), the call may perform the definition pass before its single execution.

**R6 (legacy boundary).** When the executing session negotiated a protocol version lower than `2026-07-28` and the tool is header-bound, the bridge must answer with the named, payload-free failure code `parameter_header_unsupported` and execution state `not_started`, with zero upstream tool executions, zero `tools/list` warm request, and no breaker mutation. It must not answer with `tool_reported_error` or `upstream_protocol_error`, and it must not reject discovery (which would remove working non-header tools on the same server).

**R7 (documented boundary).** Repository documentation must state which protocol versions carry parameter headers, how the bridge behaves in stateless and stateful mode, the exact failure code for older upstreams, the operator action, and the cost of the definition pass.

**R8 (payload-free failure vocabulary).** Every failure path on the header call path must be payload-free (no tool names, argument values, header values, URLs or tokens in messages) and must use the existing safe code set, except for the single additive reason token in R6, which must be registered consistently in the closed runtime reason vocabulary, the client-facing dispatch code and the logging contract. No new log event name, field name, metric family or metric label is permitted.

**R9 (deterministic reproduction).** A test that drives the bridge's own Tool server against a controllable peer must reproduce the defect and record the negotiated protocol version, the presence or absence and value of each parameter header, and the outbound arguments body, in both default stateless and explicit stateful mode, with no dependency on the live github upstream.

**R10 (deployed verification).** The shipped build must be verified on the live deployment against the github upstream: the header-bound call returns content, the query-only control succeeds in the same window, and structured logs for the header call show no `tool_reported_error` and no `upstream_protocol_error`. This requirement is operator-gated and outside the code envelope.

**R11 (restart resilience).** The restart contract must be preserved: a stateless header call opens a fresh session, survives an upstream generation swap, and succeeds against the new generation on a subsequent call.

**Non-goals.** No configuration key, default change or `stateless` policy change; no session pinning, eviction or reconnect policy; no go-sdk upgrade or vendored patch (stays v1.8.0, `go.mod:8`); no change to `core.Bridge.Dispatch`, catalog publication, search/embedding, the breaker state machine, metrics, capacity or readiness; no new log event/field, metric family or label; no retry/replay of an upstream call; no bridge-side transport-header emitter, build tag or protocol-patch hack; no change to the upstream server's own protocol implementation. *(r6: only the `go.mod:8` citation corrected.)*
<!-- specdd:requirements:end -->

## 3. EVIDENCE AND UNCERTAINTY

### VERIFIED (source-anchored)

- **Default is stateless.** `Server.Stateless *bool` at `internal/config/config.go:41`; the predicate is `internal/config/config.go:48-52`; the upstream facades are built from it (`cmd/mcp-bridge/wiring.go:177-179`). *(r6: F3 anchor corrected and re-verified — `upstream.NewDedicated` is called at `wiring.go:177`.)*
- **Stateless discovery connects, lists, closes.** `Dedicated.Discover` at `internal/upstream/dedicated.go:71` opens an ephemeral session, paginates `ListTools`, records the negotiated version for the handshake log and the legacy flag, and closes the session; stateful discovery reuses `sessionFor` at `internal/upstream/dedicated.go:294`. The function already compares the session's `InitializeResult().ProtocolVersion` with `2026-07-28`, so the fact the legacy gate needs is already read on that path.
- **Stateless execution opens a fresh session and does not list tools.** `Dedicated.CallTool` at `internal/upstream/dedicated.go:359` connects (`d.client.Connect` at `internal/upstream/dedicated.go:377`, under the caller's request context), decodes the arguments and performs exactly one client call through `d.client.CallTool` at `internal/upstream/dedicated.go:401`; there is no `ListTools` on the execution session.
- **A modern session handshakes through `server/discover` and never sends `initialize`.** When the peer advertises a supported version at or above `2026-07-28`, `ClientSession.Connect` enters the discovery loop (go-sdk `client.go:323-381`), stores the discovery result as the session's `InitializeResult`, and returns; the legacy `initialize` request at go-sdk `client.go:390` runs only after the discovery loop breaks. `usesNewProtocol()` (go-sdk `client.go:517`) therefore reports the modern protocol for such a session even though its field is named for the initialize handshake, which is why the version gate reads `InitializeResult().ProtocolVersion` on both paths.
- **The SDK caches tool definitions only as a side effect of its own `ListTools`, and only under the new protocol.** `ClientSession` holds a per-session `toolsCache`; `ListTools` consults and stores it only when `usesNewProtocol()`; `filterValidTools` runs before caching, so a tool with an invalid annotation never reaches the bridge's catalog; `CallTool` resolves the definition with `lookupTool` (go-sdk `client.go:599`), which scans every stored page and ignores TTL, and injects it into the request context before the transport builds the request (go-sdk `client.go:1292`).
- **`usesNewProtocol()` is a string version comparison** of the initialize result's `ProtocolVersion` against `2026-07-28` (go-sdk `client.go:517`).
- **Header emission requires both the cached definition and a session negotiated at `2026-07-28` or later.** On the client transport, `setStandardHeaders` returns without setting any standard header when the negotiated version is below the threshold; at or above it, it sets `Mcp-Protocol-Version`, `Mcp-Method`, `Mcp-Name` where the method carries a name, and, for `tools/call`, the `Mcp-Param-<Header>` values generated from the definition in the request context (go-sdk `streamable_headers.go`, symbols `setStandardHeaders`, `generateParamHeaders`; go-sdk `streamable.go`).
- **Parameter encoding is SEP-2243 and primitive-only.** `generateParamHeaders` reads annotations from the definition's input schema and the values from `arguments`, skips absent and `null` values, and encodes strings (Base64 when not header-safe ASCII), booleans and safe-range integers; anything else yields no header (go-sdk `streamable_headers.go`, symbols `generateParamHeaders`, `primitiveToString`).
- **The SDK peer enforces header/body agreement.** `validateMcpHeaders`/`validateParamHeaders` require a matching parameter header for every annotated body value and reject a header whose body parameter is absent (go-sdk `streamable_headers.go`); the streamable server transport answers a mismatch with HTTP 400 and the JSON-RPC error `CodeHeaderMismatch` (go-sdk `streamable.go`, `shared.go`). This is why R4 retains the values in `arguments`.
- **A positive list TTL enables a `ListTools` cache hit; a zero TTL forces a refetch but never hides a definition from `CallTool`.** `ListTools` stores every successful page with `put` regardless of TTL (put call at go-sdk `client.go:1276`, definition at go-sdk `cache.go:45`, both re-verified; `ListTools` spans `client.go:1264-1277`); `cachedListResult` treats `ttlMs <= 0` as a miss and deletes the entry, while `lookupTool` scans the stored pages directly and ignores TTL (go-sdk `cache.go`, symbol `methodCache`; go-sdk `client.go:599`). Consequence: on a zero-TTL peer the definition pass costs one RPC per page to fill the cache and the immediately following call still finds the definition; on a positive-TTL peer the pass is a cache hit and costs zero RPCs. *(r6: F6 sub-anchor corrected.)*
- **`tools/list` is ordered by tool name, not by registration order.** The server stores tools in a `featureSet` keyed and sorted by unique ID, and `all()` iterates that sorted order (`sortKeys` at go-sdk `features.go:99-104`, re-verified); `Server.listTools` pages over `all()` (go-sdk `server.go:979`). A fixture that requires the target on a later page must therefore name it to sort after its peers; registration order is irrelevant. *(r6: F6 sub-anchor corrected.)*
- **A test peer can advertise a positive list TTL.** `ServerOptions.SetCacheable(ctx, req, *Cacheable)` at go-sdk `server.go:183` is invoked by `resolveCacheable` for every outgoing list result (go-sdk `server.go:1114`), so a peer can set `TTLMs` and `CacheScope` on `tools/list`. The advertised TTL lives for the peer's lifetime, so a fixture TTL strictly longer than the test's bounded interval guarantees the zero-delta assertion cannot be broken by expiry.
- **A peer can pin the protocol in either direction.** `ServerOptions.SupportedProtocolVersions` narrows the advertised list and rejects excluded new-protocol requests with `CodeUnsupportedProtocolVersion`; `Server.Connect` filters the advertised list through the transport's `SupportsProtocolVersion` (go-sdk `server.go`, symbols `ServerOptions`, `Connect`, `filterSupportedVersions`, `discover`).
- **A legacy pin rejects the probe client-side, not on the wire.** `server/discover` is exempt from the stateful-transport restriction (go-sdk `streamable.go:1520-1523`, re-verified), so a legacy-pinned peer answers the new-protocol probe with HTTP 200 and a legacy `DiscoverResult`; the client then synthesizes the fallback itself because the negotiated version is below `2026-07-28` (go-sdk `client.go:443-451`, re-verified), after which the legacy `initialize` negotiates the older version. *(r6: resolves open question 1.)*
- **A modern peer must be a stateless streamable transport.** `StreamableServerTransport.SupportsProtocolVersion` accepts `>= 2026-07-28` only when the transport is stateless (go-sdk `streamable.go`, symbol `SupportsProtocolVersion`); a stateful streamable handler therefore cannot negotiate `2026-07-28`, and the modern test peer must be built with `&mcp.StreamableHTTPOptions{Stateless: true}`.
- **The legacy fallback is source-verified and already exercised.** Pinning an older-only list (`[]string{"2025-11-25"}`) at `internal/upstream/upstream_test.go:154` makes the new-protocol probe fail (client-side, per the previous bullet), after which the client falls back to the legacy initialize and negotiates the older version, so `usesNewProtocol()` is false and the SDK caches nothing. The bridge's own legacy harness (`cmd/mcp-bridge/serve_upstream_restart_test.go:220`) asserts an observed version below `2026-07-28` for a legacy peer built by `newPeerHandler` at `cmd/mcp-bridge/serve_upstream_restart_test.go:29`.
- **Publication follows discovery.** The service refresher's pass calls the upstream's `Discover` and then installs the returned definitions into the runtime catalog; a server whose discovery fails has its catalog entry marked stale and `Acquire` fail-closes with a nil lease (observed in the existing refresh tests: one pass = one `Discover` followed by installation, `internal/service/refresh_test.go:251-303`; stale entry with fail-closed `Acquire`, `internal/service/refresh_test.go:230-243`). A catalog entry is therefore never served before the discovery that produced it has returned — the ordering the §4 I2 continuous invariant relies on. *(r6: new decisive evidence for F1 closure, read from existing tests.)*
- **The bridge has no header translation today.** A repository-wide search for `x-mcp-header` in production code returns nothing; the published schema is canonicalized at `internal/core/core.go:248` and published unchanged, and the call path forwards the client's raw arguments unchanged (`internal/service/tools.go:760-810`).
- **One upstream call path.** The bridge's Tool server dispatches through `core.Bridge.Dispatch` at `internal/core/core.go:269` from `ToolSurface.callTool` at `internal/service/tools.go:700`; the direct-execution surface calls the same dispatch. There is no second upstream invocation seam to keep in sync.
- **Admission precedes execution and the slot release is deferred; the call timeout starts there.** `ServerClient.CallTool` at `internal/upstream/runtime.go:184` takes the call context, creates the call-timeout context via `CallContext` (`internal/upstream/runtime.go:178`, consumed at `internal/upstream/runtime.go:186`), admits through `AdmitCall` at `internal/upstream/runtime.go:190`, and only then calls the session at `internal/upstream/runtime.go:204`; failure accounting is `RecordFailure` at `internal/upstream/runtime.go:266` / `RecordSuccess` at `internal/upstream/runtime.go:286`. `Dedicated.CallTool` performs the SDK call through this method (`internal/upstream/dedicated.go:401`), so the admission seam is the only place a definition pass may run after admission, and the stateless connect at `internal/upstream/dedicated.go:377` runs before that timeout exists. *(r6: F6 scoping corrected.)*
- **The failure vocabulary is closed and finite.** Reason tokens are listed at `internal/observe/log.go:160` and validated by `finiteReason` at `internal/observe/log.go:166`; `RuntimeErrorInfo` at `internal/observe/envelope.go:92` maps a token to `{code, kind, retryable, remediation}` and collapses unknown input to `unknown`; the client result shape and the fixed message table are `errorResult` at `internal/service/tools.go:1037` and `dispatchMessage` at `internal/service/tools.go:1051`.
- **The execution-state mapping has two named non-`not_started` branches.** `failureOutcome` at `internal/upstream/dedicated.go:431` maps `context.DeadlineExceeded` to `deadline_exceeded`/`unknown` and `context.Canceled` to `internal_error`/`unknown`; a `*CallError` keeps its own state (clamped to `unknown` when not `not_started`); every other error falls through to `backend_unavailable`/`not_started`.
- **The dispatch-error log path exists today.** On any dispatch error the service emits `backend_complete` and `mcp_complete` records carrying `observe.RuntimeErrorInfo(dispatchErr.Code)` before rendering the payload-free result (`internal/service/tools.go:760-810`), so a new terminal reason is visible without a new event name.
- **The logging contract is partly machine-checked, and the reason table is not.** `TestLoggingContractDocumentsExactEnvelopeFieldUnion` at `cmd/mcp-bridge/logging_contract_test.go:114` pins the envelope field union; the reason vocabulary appears as a documented table in `docs/logging-contract.md`, so extending it is a text edit that no test pins to an exact set.
- **Test harnesses exist and are reusable.** `newPeer` at `internal/upstream/dedicated_test.go:40`, `newDedicated` at `internal/upstream/dedicated_test.go:62`, `newDedicatedWithStateless` at `internal/upstream/dedicated_test.go:66`; `newPeerHandler` at `cmd/mcp-bridge/serve_upstream_restart_test.go:29`, `startUpstreamRestartBridge` at `cmd/mcp-bridge/serve_upstream_restart_test.go:52` (accepts any peer URL and the stateful flag), `TestUpstreamRestartSurvivesInDefaultStatelessMode` at `cmd/mcp-bridge/serve_upstream_restart_test.go:155`, `TestUpstreamPreservesSessionInExplicitStatefulMode` at `cmd/mcp-bridge/serve_upstream_restart_test.go:195`; the harness config writer pins a `5m` catalog refresh (`cmd/mcp-bridge/serve_test.go:125`, `cmd/mcp-bridge/serve_test.go:133`, `cmd/mcp-bridge/serve_test.go:153`). The existing handlers pass nil streamable options, so they are stateful transports and cannot negotiate `2026-07-28`; the modern peer must be built in the new enveloped test file.

### ASSUMPTIONS

- The bridge keeps its own client session alive for the duration of one `CallTool`; the SDK's definition lookup and the transport header generation run synchronously inside that call, so a definition inserted by the definition pass on the same session is visible to the immediately following call.
- The upstream's page cursors are self-contained and stateless-safe, as the github-style peer already relies on for discovery pagination.
- A stateful header call against a modern peer on the pre-change build succeeds when the discovery session previously stored a `tools/list` page, because `lookupTool` ignores TTL; the control oracle measures this rather than assuming it (§5 AC01-4).
- The publication-ordering observation of the refresh tests generalizes to all current publication paths (initial load, reload, refresh); r6's I2 proof depends on it and the reviewer is asked to probe it (§9). A future publication path that served definitions before their discovery returned would reintroduce a narrow under-inclusive window.

### UNKNOWN (each with consequence and resolver)

- **UNKNOWN-1 — the live github upstream's negotiated version and header requirement.** MB-001 shows an HTTP 200 tool error rather than a header-mismatch 400, which is consistent with either an older negotiation or a non-SDK server that requires the values functionally. Consequence: if the deployed upstream negotiates below `2026-07-28`, the fixed bridge answers `parameter_header_unsupported` and ticket 04 is not satisfied; the operator must upgrade or repoint the upstream. Resolver: the ticket 01 probe (deterministic peer) establishes the mechanism, and ticket 04 records the deployed version.
- **UNKNOWN-2 — the deployed image and its configuration.** Whether the running container is built from this commit, and whether the github upstream is configured stateful. Consequence: ticket 04 must name the image and container identity and state whether a restart or reload was used. Resolver: operator evidence in ticket 04.
- **UNKNOWN-3 — real upstream page size and list TTL versus the call timeout.** The definition pass costs up to every page of the list inside one `limits.call_timeout`. Consequence: a very large catalog on a slow upstream can exceed the default budget for header calls; the runbook names the cost and the operator knob. Resolver: observed request counts in the unit oracles plus the live revision recorded in ticket 04.
- **UNKNOWN-4 — a non-SDK upstream that requires parameter values but negotiates below `2026-07-28`.** Out of spec for the SDK's own emission; the named code is the correct, actionable answer there. Resolver: ticket 04.

### [AUTO-DECISION] entries (for coordinator persistence)

- **AD1 — mechanism: warm the execution session's tool cache with a paged `tools/list`, paginating until the tool appears.** Why: it is the only seam that leaves emission to the SDK's own SEP-2243 implementation, so byte-level encoding, absent/null handling, integer rules and header/body agreement cannot drift from the peer's validator; the SDK already sets the standard headers on the same request. Result: in stateless mode the call performs at most one definition pass; the worst case is the number of pages up to and including the page holding the tool (and no more than the upstream's whole list when the tool has vanished), each page a separate RPC, all inside the single call-timeout context, with duplicates suppressed by paging forward from cursor `""`.
- **AD2 — bound the cost with a discovery-maintained header-annotation index.** *(r6 amended: union install.)* Why: without it every stateless call would pay the definition pass. Result: an in-memory per-runtime set of raw tool names carrying at least one valid `x-mcp-header`, installed after a fully successful discovery as the **union of the previous index's name set and the new discovery's header-bound set**; a miss keeps today's path. A tool removed upstream keeps its index entry for one refresh interval and costs at most one wasted definition pass whose absent-name outcome is AD7's nil (or the tool's real absence upstream); it can never produce a missed header for a published tool.
- **AD3 — additive reason token `parameter_header_unsupported`.** Why: ticket 03 requires a specific, actionable code distinct from `tool_reported_error`/`upstream_protocol_error`; `unsupported_capability` would conflate a protocol boundary with upstream interaction semantics. Result: one new value in each of three closed places (runtime reason vocabulary, the runtime error-info map, the dispatch message), classified `configuration`/non-retryable with the existing configuration remediation string; no event or field name changes.
- **AD4 — the legacy rejection is per call, after session acquisition and before any `tools/list` or execution.** Why: discovery-time rejection would remove working non-header tools from the same server; a first-call-only check would be stateful and non-deterministic. Result: a stateless legacy header call pays one connect/initialize and then returns `parameter_header_unsupported` with `not_started`. A call rejected by admission (open breaker or saturated slot) returns its pre-existing admission code and never reaches the gate.
- **AD5 — arguments are retained and headers are additive.** Why: the peer rejects a header when the body parameter is absent and rejects a mismatch when present; stripping would break every compliant peer. Result: arguments bytes are unchanged; the bridge performs no filtering.
- **AD6 — the index is presence-based** (any non-empty string `x-mcp-header` under a `properties` object at any nesting depth, regardless of the declared type). Why: over-inclusion costs at most one wasted definition pass, while under-inclusion reproduces the defect; the SDK performs its own validation and already excludes tools with invalid annotations from the list. Result: a simple JSON walk, no duplicated validation logic.
- **AD7 — a tool absent from the session's fresh pages is not converted into a synthetic failure.** Why: a tool absent from the upstream's current list cannot produce a header/body mismatch, so the upstream's own answer is the truthful outcome. Result: the pass completes without the name, returns nil, and the single execution proceeds; no new code.
- **AD8 — admission ordering via a prepared-call seam in `ServerClient`.** Why: running the pass before admission would spend an RPC on every call rejected by an open breaker or a saturated slot, and would sit outside the per-call timeout; running it after admission keeps both invariants with a behavior-preserving refactor. Result: `CallTool` delegates to a prepared variant with a nil prepare function, so existing callers are unchanged; a prepare failure returns before any execution and before any breaker accounting, and its outcome is mapped by the existing `failureOutcome`.
- **AD9 — the vocabulary assertion lives in the enveloped upstream test file.** Why: the classification is part of the failure contract the upstream call path emits, and asserting it from `internal/upstream/header_params_test.go` (which may import `internal/observe`) needs no envelope widening. Result: no write path is added; the coordinator is informed that this placement is deliberate.
- **AD10 — failure-state exceptions are stated, not normalized.** Why: re-labelling a deadline/cancel prepare failure as `not_started` would add a second mapping beside `failureOutcome` and misreport a genuine timeout; the existing mapping already distinguishes pre-execution failures with a truthful state. Result: pass failures that are neither a deadline nor a cancellation are `not_started`; a pass deadline is `deadline_exceeded`/`unknown` and a pass cancellation is `internal_error`/`unknown`; zero execution and zero breaker accounting hold in all three cases. Consequence: the `not_started` invariant is scoped, never silently widened.
- **AD11 — *(r6, F1)* the index install is a union install, making the non-under-approximation continuous.** Why: r5 scoped the property to installation instants (old I2), which contradicted absolute R2 for an already-published tool whose removal had been discovered but not yet published. Union install — new index := previous index ∪ new discovery's header-bound set, installed after all pages succeed and before definitions return — together with the verified publication rule (a catalog entry is served only from a returned discovery's result; a failed discovery leaves index and catalog unchanged and marks the entry stale/fail-closed) proves the continuous invariant: at every instant, index ⊇ header-bound(published catalog). Alternatives rejected: making install and publication atomic requires touching catalog publication (prohibited non-goal, outside the write envelope); narrowing R2 to a snapshot-scoped guarantee weakens a safety requirement and was the r5 rejection ground; deriving header-boundness at call time requires widening `core.Upstream` (rejected alternative (d), `internal/core` outside the envelope). Result: no R/AC/T id, write-path or envelope change; the only cost is bounded over-inclusion (≤ one refresh interval), which is AD6's accepted cost class. Residual dependency: the publication-ordering rule; violation manifests as a payload-free peer rejection, never a missed header emission.
- **AD12 — *(r6, open question 1)* AC03-2 asserts the observed negotiated version, not a wire error.** Why: the legacy pin's probe rejection is synthesized client-side after a 200 legacy `DiscoverResult` (go-sdk `streamable.go:1520-1523`, `client.go:443-451`), so a wire-error assertion would be unfalsifiable in the wrong direction and the negotiated version is the behavior-level fact. Result: AC03-2's fixture asserts the recorded negotiated version is exactly `2025-11-25`; the `CodeUnsupportedProtocolVersion` reference stays as the client-side fallback mechanism, not as a wire observable.
- **AD13 — *(r6, open question 2)* the AC03-1 positive-TTL fixture pins a TTL strictly longer than the test interval.** Why: `TTLMs > 0` alone permits expiry inside the run, which would break the exact-zero list-delta assertion for timing reasons rather than behavior. Result: `TTLMs: 60_000` (60 s) against a test whose total bounded runtime (awaits plus default `go test` guard) is far below one minute; the TTL is asserted to exceed the measured test duration at the fixture level.

## 4. DESIGN
<!-- specdd:design:start -->

**Approach.** Keep the bridge ignorant of header encoding and make the SDK's emitter reachable on the execution session. Three small pieces: (1) a discovery-maintained annotation index that decides *whether* a tool needs the definition pass; (2) a paged definition pass that populates the executing session's cache; (3) a per-call legacy gate that fails before any pass or execution. A minimal `ServerClient` seam runs the pass after admission and inside the call timeout.

**Header-bound detection (`internal/upstream/dedicated.go`).** A new unexported helper takes a canonical input schema (`internal/core/core.go:248`) and reports whether any JSON object under a `properties` key at any nesting depth contains an `x-mcp-header` member whose value is a non-empty JSON string. It does not validate header names, types, required-ness or `additionalProperties`. This is sound because the SDK client's `filterValidTools` runs before the page is cached and before the bridge's discovery sees the tools, so a tool with an invalid annotation never enters the bridge catalog. The index is a `map[string]struct{}` of raw tool names on `Dedicated`, guarded by the existing mutex. On every fully successful discovery it is replaced, under the mutex, by the **union of the current index's name set and the new discovery's header-bound set**, and the union is installed before the discovery's definitions are returned to the caller; a failing discovery leaves the previous index intact, matching the existing catalog rule. Readers take the map under the mutex. The union install gives the continuous superset property of I2 (r6): because a catalog entry is served only from a returned discovery's result, and each install covers both the previously published set and the newly published set, the index is at every instant a superset of the header-bound set of the published catalog.

**Definition pass.** A new unexported helper takes the session and a raw name and calls `session.ListTools` with an explicit non-nil params object starting at cursor `""`; while the current page lacks the name and `NextCursor` differs from the previous cursor, it continues from that cursor. It returns nil when the name is found, nil when the list is exhausted or the cursor stops advancing, and the error when a `ListTools` call fails. Forward paging is required because `lookupTool` scans every stored page, so stopping at page 0 would miss a tool the upstream places later; the worst case is the whole list, which is the correctness cost AD2 accepts. A positive-TTL page is served from the SDK's cache and costs no RPC; a zero-TTL page is refetched and stored again.

**Call path (`internal/upstream/dedicated.go`, `internal/upstream/runtime.go`).** After the session is acquired and the arguments are decoded, `Dedicated.CallTool` selects a prepare function: for a body-only tool it is nil; for an indexed header-bound tool it first compares the session's `InitializeResult().ProtocolVersion` with `2026-07-28` (or treats a missing result as below) and returns the typed call error `parameter_header_unsupported` with `not_started` when the comparison fails, and only otherwise runs the definition pass. The call becomes the prepared variant of `ServerClient.CallTool`: the existing body, unchanged, plus a non-nil prepare invoked after a successful admission and before the session call, whose error is returned immediately. The prepare error path must not call failure or success accounting (a definitions read is not an execution outcome) and must not execute the tool; the deferred admission release still runs. Existing callers delegate with a nil prepare, so their behavior is byte-identical. Because admission precedes the gate, a call refused by an open breaker or a saturated slot returns its existing admission code and never reaches the gate; the named code is produced whenever admission succeeds.

**Why the gate is per call and reads the session's version.** The version is established by the handshake of *this* session, which is the authority for this request; a discovery snapshot is a different session (stateless mode) and could be stale. The check runs before the definition pass so a legacy remote pays no extra RPC, and before execution so no tool effect is attempted under a protocol that cannot carry the parameters. The `upstream_ephemeral_handshake` record remains the observability signal for the negotiated version and is not used as the gate's input.

**Cost, stated exactly.** Stateless header call: one connect (under the caller's request context, bounded by the connect timeout), one admission, one definition pass covering pages until the tool appears (worst case: every page of the list), one `tools/call`, one close. Stateful header call: no connect; the definition pass pages while the cache lacks the entry; on a positive-TTL peer the pass is a cache hit and costs zero definition RPCs, while on a zero-TTL peer the SDK deletes the entry on read, so each header call performs at least one definitions RPC and then one `tools/call`. The definition pass and the `tools/call` share the call-timeout deadline that `ServerClient.CallTool` creates via `CallContext` (`internal/upstream/runtime.go:178`, `:186`); the stateless connect and its handshake run *before* that context exists, under the caller's request context (`internal/upstream/dedicated.go:377`), and are bounded by the connect timeout instead. *(r6: F6 corrected.)* The plan adds no per-RPC timeout and no page cap, because a cap would risk the very false negative this feature exists to fix. Standard headers (`Mcp-Protocol-Version`, `Mcp-Method`, `Mcp-Name`) and parameter headers are claimed only for sessions negotiated at `2026-07-28` or later; below that, the transport emits none.

**Failure behavior.** Gate failure: `parameter_header_unsupported`, `not_started`, zero executions, zero breaker accounting. Definition-pass failure: the error is returned to `Dedicated.CallTool` and mapped by the existing `failureOutcome` (`internal/upstream/dedicated.go:431`). A pass that exceeds the call deadline maps to `deadline_exceeded`/`unknown`; a canceled context maps to `internal_error`/`unknown`; every other pass failure — including a `tools/list` RPC rejection, which falls through `failureOutcome`'s default branch — maps to `backend_unavailable` with `not_started`. No new code is introduced for a pass failure, and no breaker accounting occurs in any of these cases. Execution: unchanged, including `tool_reported_error` for tool-level errors. Messages stay fixed and payload-free; the new code has a fixed message.

**Sequences.** *(r6: F5 corrected — the connect precedes admission because `ServerClient.CallTool` admits only after `Dedicated.CallTool` has connected.)* Stateless body-only: connect (handshake under the request context) → admission → one `tools/call` → close. Stateless header-bound: connect → admission → index hit → version gate → definition pass (pages until found) → one `tools/call` with arguments and parameters → close. Stateful header-bound: cached session → admission → index hit → gate → pass (cache hit or refetch per TTL) → one `tools/call`. Legacy header-bound (stateless): connect → admission → index hit → gate fails → `parameter_header_unsupported`/`not_started`, no list, no call.

**Alternatives rejected.** *(a)* A bridge-side `RoundTripper` emitter: duplicates SEP-2243 encoding and the version gate, can disagree with the SDK's own emission when a stateful cache is already warm, and needs schema/version plumbing into the transport — rejected on correctness and maintenance. *(c)* Forcing `stateless: false` for affected servers: changes a documented default, pins sessions, and forfeits restart resilience — rejected. *(d)* Extending `core.Upstream` to carry the published definition: removes only the index, not the definition pass, and widens a public interface plus every fake — rejected. *(e)* Warm during discovery in stateless mode: the discovery session is closed, so the cache would be discarded — rejected as vacuous. *(f)* *(r6)* Restating R2 as a snapshot-scoped guarantee (the r5 reviewer's textual fix): weakens an absolute safety requirement to match a design limitation, when the union install closes the window in-envelope without any id, envelope or test change — rejected in favor of AD11.

**Invariants.** (I1) No bridge code writes a parameter header or alters arguments; emission is the SDK's. (I2) *(r6, continuous form superseding the r5 installation-scoped form)* At every instant, the index contains every tool of the currently published catalog whose published input schema carries a valid `x-mcp-header` annotation. Proof obligations, all resting on verified existing behavior: (i) a catalog entry is served only from a discovery result after that discovery returned, and the union index was installed immediately before the return (`internal/service/refresh_test.go:251-303`); (ii) a failed discovery leaves both the previous index and the previous catalog unchanged, and the entry is stale/fail-closed (`internal/service/refresh_test.go:230-243`); (iii) each union install covers the previously published set and the newly published set, so no publication boundary can drop a published header-bound name from the index. Consequence: an upstream schema removal is covered for one extra refresh interval (bounded over-inclusion, at most one wasted definition pass per call on the removed name, with AD7's nil outcome); the under-inclusive direction is closed, so a tool the catalog publishes can never take the body-only path for index reasons. (I3) A body-only call has no prepare: zero extra RPCs. (I4) A legacy header call performs zero lists and zero executions. (I5) A prepare failure performs zero executions and zero breaker accounting; it is `not_started` except for the deadline/cancel branches of `failureOutcome`, which keep their existing `deadline_exceeded`/`internal_error` codes and the `unknown` state.
<!-- specdd:design:end -->

## 5. ACCEPTANCE CRITERIA

Every ticket checkbox maps 1:1 to one id of the form `AC<ticket>-<criterion>`: the ticket number is the first component and the criterion position in that ticket's checklist is the second. No ticket criterion is merged, split, widened or weakened. The fifth criterion of ticket 01 and the third criterion of ticket 03 are inspections; every other criterion is backed by a named test or an operator run.

| ID | R | Ticket checkbox | Observable pass/fail | Oracle |
|----|---|-----------------|----------------------|--------|
| AC01-1 | R9 | ticket 01 #1 | A test in `cmd/mcp-bridge` drives the bridge's own Tool server against a modern peer built in the new file as `mcp.NewServer` with `ServerOptions.SupportedProtocolVersions` pinned to `2026-07-28` and served by a `StreamableHTTPHandler` constructed with `&mcp.StreamableHTTPOptions{Stateless: true}`; the peer registers one tool with `owner` and `repo` annotated `x-mcp-header` and one body-only control tool, records each request's method (from the JSON-RPC body, so `server/discover` and `initialize` are both classified), `Mcp-Protocol-Version`, `Mcp-Param-owner`, `Mcp-Param-repo` and body, and the test asserts the observed negotiated version is exactly `2026-07-28`; the test is independent of any live upstream. | `TestParameterHeaderTransitWireFacts`, run by the anchored command in T1. |
| AC01-2 | R9 | ticket 01 #2 | In default stateless mode the header-bound call returns a non-error result while the control call succeeds in the same window; on the pre-change commit the same assertion collapses because the header call's result carries `isError=true` and the peer's captured response is a header-mismatch rejection (HTTP status and JSON-RPC error code recorded), so the failure is an assertion, never a compile error. | The pre-change and post-change runs of the probe named by AC01-1; the pre-change failure records the error code and the absent headers. |
| AC01-3 | R9 | ticket 01 #3 | For the header call the recorded evidence states the negotiated version, that `Mcp-Param-owner` and `Mcp-Param-repo` are present with values equal to the argument values, and that the outbound body still contains both argument values. | The capture assertions printed by the probe named by AC01-1 as one labelled evidence line per mode. |
| AC01-4 | R9 | ticket 01 #4 | The same modern peer runs against a bridge configured `stateless: false` and the header call succeeds with the parameter headers present; for the whole run the peer records exactly one handshake — exactly one `server/discover` request and zero `initialize` requests — because a session negotiated at `2026-07-28` completes through discovery and the discovery session is reused. That separates `absent definition cache` from `protocol cannot carry headers`. This is a control: it must already pass on the pre-change commit (discovery's page is stored even at TTL 0 and `lookupTool` ignores TTL). | `TestParameterHeaderStatefulControlIsAchievable` — its own modern-handshake control on the header-params modern peer: the stateful call succeeds with `Mcp-Param-owner`/`Mcp-Param-repo`, the peer records exactly one `server/discover` and zero `initialize`, and it does not use the restart fixture. |
| AC01-5 | R9 | ticket 01 #5 | The task receipt states, per mode, which mechanism was missing — absent definition cache, insufficient protocol version, or neither — each backed by the recorded wire facts; no inference is presented as observation. | Inspection of the T1 receipt evidence table by the code reviewer; the spec forbids converting a derived attribution into an observed fact. |
| AC02-1 | R1, R2, R4 | ticket 02 #1 | In default configuration the header-bound tool succeeds end to end, the peer receives `Mcp-Param-owner` and `Mcp-Param-repo` equal to the argument values, and the `tools/call` body still contains both values; the peer records **exactly one `tools/call` request** for the header-bound call on the success path, so a duplicate execution (including a transport retry) fails this criterion; the same test covers a single-page peer (definition-pass delta after discovery = exactly one list request) and a `PageSize: 1` peer with three tools whose names are chosen so the header-bound tool sorts last in `tools/list` (`aaa_control_tool`, `mmm_body_tool`, `zzz_header_tool`; the transport orders by tool name, not registration), so the definition pass issues exactly three list requests in its delta, proving pagination to the target page. | `TestDedicatedParamHeadersReachPeerInStatelessMode` (two subtests), with request counts — including the `tools/call` count — taken under the peer's lock after discovery and again after the call. *(r6: F2 exactly-one oracle added; no other change.)* |
| AC02-2 | R3 | ticket 02 #2 | The body-only control tool's call carries no `Mcp-Param-*` header, the peer records zero additional `tools/list` requests between the post-discovery snapshot and the call's completion (no refresh loop runs in a unit test), and the result content equals the pre-change expectation. | `TestBodyOnlyControlAddsNoExtraListCall`. |
| AC02-3 | R11 | ticket 02 #3 | No session is retained: after an upstream generation swap a stateless header call succeeds against the new generation and its capture shows the new generation's echo and the parameter headers; the pre-existing stateless restart contract is additionally verified at the pre-change baseline and, when green, remains in the delivery gate. If that baseline is red on the pre-change commit, T1 records it as an explicit baseline exclusion and a blocker instead of attributing it to this feature. | `TestParameterHeaderRestartSurvivesInStatelessMode` (upstream generation swap); the pre-existing restart test appears only as the baseline record in T1 and is not an oracle of this AC. |
| AC02-4 | R1, R3, R11 | ticket 02 #4 | Regression tests cover the header call, the control call and the restart case; on the pre-change commit the header and restart-header oracles fail as assertions while the control oracle passes, and all pass after the change. | The pre-change and post-change runs of the oracles named by AC02-1, AC02-2 and AC02-3, both recorded in the task receipts. |
| AC02-5 | R8 | ticket 02 #5 | The header path's failure cases return safe, payload-free outcomes: a modern peer whose `tools/list` starts failing with an immediate application error after discovery yields `backend_unavailable` with `not_started` (the `failureOutcome` default branch; a delay-based fixture would instead yield `deadline_exceeded`/`unknown` per I5 and is therefore not used here) and the peer records zero `tools/call` requests; the additive token is registered such that `observe.RuntimeErrorInfo("parameter_header_unsupported")` returns code equal to the token, kind `configuration` and retryable false; no returned message contains an argument value, header value, tool name, URL or token. | `TestParamHeaderFailurePathsUseSafeCodes` (two subtests in the test file, which imports `internal/observe`). |
| AC03-1 | R5 | ticket 03 #1 | With `stateless: false` the header-bound tool succeeds end to end and the peer receives the parameter values on one session acquired by discovery: for the whole run the peer records exactly one `server/discover` request and zero `initialize` requests, so no second session is created, and the peer records **exactly one `tools/call` request** for the header-bound call on the success path; with a peer that advertises a positive `tools/list` TTL via `ServerOptions.SetCacheable` with `TTLMs: 60_000` — a value asserted strictly longer than the test's entire bounded runtime, so expiry cannot occur inside the run — the call adds exactly zero `tools/list` requests after the post-discovery snapshot. | `TestStatefulParamHeadersReuseDiscoveryCache`, two subtests: `positive-ttl` (success, headers, exactly one `tools/call`, zero list delta, one handshake) and `zero-ttl` (success, headers, exactly one `tools/call`, one handshake; the pass refetch is permitted and not counted). *(r6: F2 exactly-one oracle and AD13 TTL bound added; subtests unchanged.)* |
| AC03-2 | R6 | ticket 03 #2 | Against a legacy peer pinned with `ServerOptions.SupportedProtocolVersions: []string{"2025-11-25"}`, the peer answers the client's new-protocol probe with HTTP 200 and a legacy `DiscoverResult` (discover is exempt from the stateless check, go-sdk `streamable.go:1520-1523`) and the `CodeUnsupportedProtocolVersion` fallback is synthesized client-side (go-sdk `client.go:443-451`), after which the legacy `initialize` negotiates the older version; the test asserts the **observed negotiated version is exactly `2025-11-25`** (recorded handshake/version evidence, not a wire error code), the header-bound call's structured result carries code `parameter_header_unsupported` and `execution_state=not_started`, the peer records zero `tools/call` requests and zero `tools/list` requests after discovery, and the bridge's captured stdout contains a `backend_complete` record whose `error.code` equals the token. | `TestLegacyParamHeaderCallFailsWithNamedCode`. *(r6: negotiated-version assertion replaces the wire-rejection reading; oracle name unchanged.)* |
| AC03-3 | R7 | ticket 03 #3 | `docs/runbooks/header-parameters.md` contains the exact headings `## Protocol versions`, `## Mechanism`, `## Failure code and operator action` and `## Cost`, and under them states the header-carrying versions (`2026-07-28` and later), the stateless and stateful mechanisms, the failure code for older upstreams with the operator action, and the definition-pass cost; `docs/logging-contract.md`'s reason vocabulary table lists the new token. | Documentation checks in T3 assert six greps exit 0 — each of the four headings in the runbook, `parameter_header_unsupported` in the runbook, and `parameter_header_unsupported` in `docs/logging-contract.md` — and the code reviewer inspects the content under each heading for R7's mechanism, action and cost. |
| AC03-4 | R5, R6 | ticket 03 #4 | The tests cover the explicit-stateful success path and the legacy branch, and the legacy assertion reads the structured code field itself, never the human-readable message text. | The code-field assertions of the oracles named by AC03-1 and AC03-2. |
| AC04-1 | R10 | ticket 04 #1 | On the deployed build, `github__get_file_contents(owner=jjohnson83853, repo=mcp-bridge, path=README.md)` returns file content, not a tool error and not a connection closure. | Operator-run live call; evidence recorded in the ticket-04 report. |
| AC04-2 | R10 | ticket 04 #2 | `github__search_repositories(query=mcp-bridge)` returns a successful result in the same observation window as the AC04-1 call. | The same operator window. |
| AC04-3 | R10 | ticket 04 #3 | Bridge structured logs for the header call contain no `tool_reported_error` and no `upstream_protocol_error`, and the report records the catalog revision used. | Loki query on the deployed container plus the recorded revision. |
| AC04-4 | R10 | ticket 04 #4 | The report records the deployed image reference and the container identity and states whether a restart, reload or fresh rollout was used. | Operator report fields. |

Coverage: R1→AC02-1/AC02-4; R2→AC02-1; R3→AC02-2/AC02-4; R4→AC02-1; R5→AC03-1/AC03-4; R6→AC03-2/AC03-4; R7→AC03-3; R8→AC02-5; R9→AC01-1/AC01-2/AC01-3/AC01-4/AC01-5; R10→AC04-1/AC04-2/AC04-3/AC04-4; R11→AC02-3/AC02-4

AC01-* describes the reproduction; AC02-* and AC03-* describe the fix and its boundary; AC04-* is the operator-gated live proof. The RED/GREEN pair is defined by the pre-change and post-change runs, not by two different tests: the same assertion must fail before and pass after. No criterion is weakened to obtain GREEN, and no test asserts message text for the named code. AC01-4 and AC03-1's `positive-ttl` subtest are controls that pass on both builds; AC03-1's `zero-ttl` subtest is a regression guard whose post-change list delta is allowed to be one. AC01-1's method recorder classifies by JSON-RPC body method so that the modern `server/discover` handshake is counted and not silently read as an absent `initialize`. The r6 exactly-one-`tools/call` assertions (AC02-1, AC03-1) are counted under the peer's lock in the same post-call snapshot as the list delta, so a transport-level duplicate execution cannot pass while the result assertions pass.

## 6. TASK LEDGER
<!-- specdd:tasks:start -->

**T0 — Planning and refinement (read-only; this specification).** Status: produced by this planner dispatch. **Deps: none.** Deliverable: sections 1–10. Scope: no production, test, config, doc or observe file is written by T0. AC IDs: none executable.

**T1 — Deterministic reproduction and control harness (ticket 01; tests only). Deps: none.**

- **Objective.** Add the harness that reproduces MB-001 against a controllable modern peer, records the wire facts for both modes, and adds the unit-level oracles that T2 must turn from RED to GREEN. T1 must also establish the baseline of the pre-existing restart test before any later task is allowed to gate on it.
- **Required inputs/evidence.** §3; the harnesses at `internal/upstream/dedicated_test.go:40`, `internal/upstream/dedicated_test.go:62`, `internal/upstream/dedicated_test.go:66`, `cmd/mcp-bridge/serve_upstream_restart_test.go:29`, `cmd/mcp-bridge/serve_upstream_restart_test.go:52`; the config writer and its pinned 5m refresh at `cmd/mcp-bridge/serve_test.go:125`, `cmd/mcp-bridge/serve_test.go:133`, `cmd/mcp-bridge/serve_test.go:153`; the legacy fixture precedent at `internal/upstream/upstream_test.go:154`; the client-side legacy fallback semantics at go-sdk `client.go:443-451` and the discover exemption at go-sdk `streamable.go:1520-1523`.
- **Write set.** `internal/upstream/header_params_test.go` (new); `cmd/mcp-bridge/serve_header_params_test.go` (new). Existing test files are not edited.
- **Allowed scope.** Register a header-annotated tool (`owner`, `repo`) and a body-only control tool; build the modern peer with `ServerOptions.SupportedProtocolVersions: []string{"2026-07-28"}` and `&mcp.StreamableHTTPOptions{Stateless: true}`; build the legacy peer with `ServerOptions.SupportedProtocolVersions: []string{"2025-11-25"}` and assert the negotiated version `2025-11-25` from the recorded handshake evidence; capture headers, methods, bodies, HTTP status and the JSON-RPC error code of rejected requests in a request-recording wrapper that forwards the body unchanged; classify requests by JSON-RPC body method (so `server/discover` and `initialize` are counted separately); set the AC03-1 positive-TTL fixture's `tools/list` TTL via `ServerOptions.SetCacheable` to `TTLMs: 60_000`, asserting at fixture level that the TTL exceeds the test's measured bounded runtime; name the reordered pagination fixture's tools so the header-bound tool sorts last (`aaa_control_tool`, `mmm_body_tool`, `zzz_header_tool`); count requests — including `tools/call` — under a lock, snapshotted after discovery and again after the call; reuse `startUpstreamRestartBridge` for the process-level runs, including its stdout capture.
- **Prohibited scope.** No production or config edit; no change to existing tests' assertions; no live network, no github access, no deployment; no enclosure write outside the two new files.
- **RED step and expected failure.** On the unmodified production tree: `TestParameterHeaderTransitWireFacts` fails at the “header call is not an error” assertion (the peer rejects the missing headers); `TestParameterHeaderRestartSurvivesInStatelessMode` fails the same way after the swap; `TestDedicatedParamHeadersReachPeerInStatelessMode` fails at the header-presence assertion; `TestParamHeaderFailurePathsUseSafeCodes` fails because the pre-change call is dispatched as a `tools/call` (rejected by the peer) instead of returning a safe code before any execution, and because the token is not yet classified; `TestLegacyParamHeaderCallFailsWithNamedCode` fails because the legacy call still succeeds instead of returning the named code. Controls that must pass on the same tree: `TestParameterHeaderStatefulControlIsAchievable`, `TestBodyOnlyControlAddsNoExtraListCall`, `TestStatefulParamHeadersReuseDiscoveryCache` (both subtests). Compile errors are not acceptable RED. The baseline run of `TestUpstreamRestartSurvivesInDefaultStatelessMode` is recorded; a red baseline is an explicit blocker, never a feature attribution.
- **Deliverable.** The two files plus a receipt evidence table per mode: negotiated version, per-header presence/value, whether arguments retained the values, request counts (server/discover, initialize, tools/list, tools/call), rejection HTTP status and JSON-RPC error code, and which mechanism was missing, each fact labelled observed or derived.
- **AC IDs.** AC01-1..AC01-5; T1 also introduces the failing oracles whose acceptance AC02-1..AC02-5, AC03-1, AC03-2 and AC03-4 own after T2.
- **Validation commands.** Baseline: `go test ./cmd/mcp-bridge -run '^TestUpstreamRestartSurvivesInDefaultStatelessMode$' -count=1`. Feature oracles: `go test ./internal/upstream -run '^(TestDedicatedParamHeadersReachPeerInStatelessMode|TestBodyOnlyControlAddsNoExtraListCall|TestStatefulParamHeadersReuseDiscoveryCache|TestParamHeaderFailurePathsUseSafeCodes)$' -count=1` and `go test ./cmd/mcp-bridge -run '^(TestParameterHeaderTransitWireFacts|TestParameterHeaderStatefulControlIsAchievable|TestParameterHeaderRestartSurvivesInStatelessMode|TestLegacyParamHeaderCallFailsWithNamedCode)$' -count=1`; plus `go vet ./...`. On T1 alone the RED oracles must fail for the stated reason; after T2 the same commands must exit 0.
- **Decision log.** Coordinator-owned task log/decisions file (skill://specdd-coordinator); T1 appends the baseline result and the RED/GREEN attribution.
- **Failed/blocked reporting.** A compile failure, a missing harness affordance, or a red baseline that cannot be excluded is reported as BLOCKED with the exact command, exit status and first failing assertion.

**T2 — Fix: definition pass, union-install index, legacy gate, vocabulary (tickets 02 and 03 production scope). Deps: T1.**

- **Objective.** Implement AD1–AD13 so T1's RED oracles turn GREEN without changing the arguments body, the body-only path or the restart contract.
- **Required inputs/evidence.** T1's receipt and RED/GREEN evidence; the anchors in §3; §4 contracts, including the continuous I2 union-install invariant and its publication-ordering proof obligations.
- **Write set.** `internal/upstream/dedicated.go`, `internal/upstream/runtime.go`, `internal/observe/log.go`, `internal/observe/envelope.go`, `internal/service/tools.go`, and (only if the harness needs a correction that does not weaken an assertion) `internal/upstream/header_params_test.go`.
- **Allowed scope.** The union-install index and schema walk; the paged definition pass; the prepared-call seam; the per-call version gate; the additive reason token in the three closed vocabularies; the fixed dispatch message. No signature change to `core.Upstream` or `core.Bridge.Dispatch`.
- **Prohibited scope.** No go.mod/go.sum change; no config or default change; no service-inbound, storage, search, breaker-state, metrics, readiness or wiring change; no header writing; no change to an existing test's assertions; no catalog-publication change (the I2 proof consumes the existing publication ordering, it does not modify it).
- **GREEN step.** Re-run T1's two feature commands; every RED oracle must pass, including the exactly-one-`tools/call` assertions; the three control oracles keep passing, including AC03-1's zero-list delta under the 60 s TTL. The pre-existing restart test remains green (or is the recorded baseline blocker). `go vet ./...` exits 0.
- **Deliverable.** The production change plus a receipt listing each criterion's pass/fail and the exact commands.
- **AC IDs.** AC02-1..AC02-5, AC03-1, AC03-2, AC03-4.
- **Validation commands.** The two anchored commands above, then `go test ./internal/upstream ./cmd/mcp-bridge -count=1` restricted to the package-level run only after the feature oracles are green; a failure outside the feature oracles is recorded as a blocker, not fixed out of envelope.
- **Decision log.** Coordinator-owned; record any local design resolution inside the approved design.
- **Failed/blocked reporting.** If the SDK's behavior contradicts §3 (for example the definition is not visible to the next call on the same session), report BLOCKED with the failing oracle and the captured wire facts; do not edit the SDK or add a second emitter.

**T3 — Documentation and delivery gate (ticket 03 #3 plus repository gates). Deps: T2.**

- **Objective.** Publish the operator-facing boundary and the vocabulary row, then run the repository-wide gates.
- **Required inputs/evidence.** T2's receipt; the documentation criterion in §5; §8.
- **Write set.** `docs/runbooks/header-parameters.md` (new); `docs/logging-contract.md`.
- **Allowed scope.** Documentation only; the runbook uses the four headings named by AC03-3 and under them names the versions, the mechanisms, the named code with the operator action and the cost; the contract table gains one row.
- **Prohibited scope.** No source or test change; no changelog/README requirement is introduced.
- **GREEN step.** The six documentation greps in the AC03-3 row exit 0; the content inspection passes; the delivery gate commands exit 0 or are reported as blocked.
- **Deliverable.** The two documents plus a gate receipt: `go test ./... -count=1`, `go test -race ./... -count=1`, `go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...` with `go run ./cmd/covcheck`, and `git diff --stat go.mod go.sum` showing no change.
- **AC IDs.** AC03-3.
- **Validation commands.** As above; a pre-existing failure outside the feature's files is reported as a blocker, never silently excluded from the gate.
- **Decision log.** Coordinator-owned.
- **Failed/blocked reporting.** Report the exact command, exit status and failing package; do not weaken the coverage gate or delete a test.

**T4 — Deployed verification (ticket 04; operator-gated, read-only from the plan's perspective). Deps: T3.**

- **Objective.** Prove the original symptom is gone on the live deployment and record the identity of what was verified.
- **Required inputs/evidence.** The shipped image, the deployment access and the operator authorization; §8.
- **Write set.** None.
- **Allowed scope.** Read-only live calls and log queries; recording evidence in the ticket-04 report.
- **Prohibited scope.** No deploy, restart, registry write, credential change or configuration mutation by the implementation pipeline; the operator performs the deployment step.
- **GREEN step.** AC04-1..AC04-4 all hold in one report: header call content, control success in the same window, no `tool_reported_error`/`upstream_protocol_error` in the header call's logs, the catalog revision, the image reference, the container identity and the restart/reload statement.
- **Deliverable.** The ticket-04 report.
- **AC IDs.** AC04-1..AC04-4.
- **Validation commands.** Operator-run; the exact tool calls and the log query are recorded in the report.
- **Decision log.** Coordinator-owned.
- **Failed/blocked reporting.** If the deployed upstream negotiates below `2026-07-28`, the fix is correct but the ticket is unsatisfied; report the observed version and the required operator action (upgrade or repoint the upstream), not a code change.
<!-- specdd:tasks:end -->

## 7. OBSERVABILITY

The observability contract was read before this section. The change adds one *value* to closed vocabularies and adds no event, field, metric family, label or dashboard. Handshake counts, `tools/call` counts and list-request deltas exist only in tests; they are not exported as metrics.

| Component | Logging | Metrics | Verification | Exception reason |
|---|---|---|---|---|
| Upstream call path (header transit) | Existing `backend_complete` and `mcp_complete` warning records carry the terminal code through `observe.RuntimeErrorInfo` (`internal/service/tools.go:760-810`); the new token appears as `error.code=parameter_header_unsupported` with the fixed message. No new event or field. | No new series or label; `bridge_requests_total`'s bounded `outcome` dimension already marks the request unhealthy. A per-code label is rejected as unbounded. | `TestParamHeaderFailurePathsUseSafeCodes` asserts code, kind and retryability; `TestLoggingContractDocumentsExactEnvelopeFieldUnion` at `cmd/mcp-bridge/logging_contract_test.go:114` pins that no envelope field was added and must stay green. | A per-code metric label would create unbounded cardinality; per-code counts remain available from the structured logs (Loki), which is the documented query surface. |
| Log delivery and retention | The existing JSON-lines/stderr pipeline is unchanged; operators query `error.code` as a field, never a label. | None. | The contract table row in `docs/logging-contract.md`; the existing logging-contract suite must remain green. | — |
| Discovery and catalog | Unchanged; the existing discovery failure records and `bridge_catalog_refresh_total` remain the signals. | Unchanged. | The existing discovery tests. | — |
| Readiness and health | Unchanged; the fix adds no readiness or health dependency. | `bridge_readiness` unchanged. | The existing readiness tests. | — |
| Tracing/context | The definition pass runs inside the call's existing request-correlated context, so its failures join the call's request/operation identities; no new identity, span or context is created. | None. | The call-path tests assert terminal records are emitted for the gate failure (AC03-2). | — |
| Dashboards/alerts | No change is required to make the failure visible: the named code is a log field and the request outcome is already a bounded metric. | — | Reviewed by inspection; no new alert is introduced because header-bound legacy upstreams are a configuration condition, not a transient fault. | — |

Operational signals: a legacy header-bound upstream is visible as repeated `parameter_header_unsupported` records with a `configuration` classification; a slow definition pass is visible as a `deadline_exceeded` record on a header call (state `unknown`, per I5) and is resolvable through the operator's call-timeout knob (§8). The definition pass performs upstream reads only and emits no extra log line of its own, so a successful call's record count is unchanged.

## 8. RELEASE AND RECOVERY

**Order.** Merge to the main branch after review → build and push the image (operator/registry-owned; this pipeline must not write `registry.localdomain:5000`) → deploy through the operator's Portainer flow → wait for readiness → record the image reference, the container identity and the catalog revision → run T4.

**Rollback.** Redeploy the previous image digest. There is no schema, state or data migration; no session is pinned and no configuration changed, so rollback is a plain image swap, after which header calls return to the pre-change behavior (a payload-free upstream rejection in the same scenario). The catalog is republished by discovery on the next refresh.

**Irreversible steps.** None.

**What operators must know.** (1) `2026-07-28` and later carry parameter headers; below that the bridge now answers a header-bound call with `parameter_header_unsupported`/`not_started` instead of letting the SDK silently omit the headers — this is intentional fail-closed behavior, and ticket 04's upstream must negotiate the newer version or be upgraded/repointed. (2) A header-bound call pays one definition pass (up to every page of the upstream's tool list) inside the single `limits.call_timeout`; on a zero-TTL peer the same is true in stateful mode, so raise the call timeout for large catalogs. A peer that advertises a positive `tools/list` TTL removes that cost in stateful mode. (3) A body-only tool pays nothing and keeps its behavior. (4) The failure is logged with the existing fields and appears in Loki as `error.code=parameter_header_unsupported`.

## 9. CRITIC HANDOFF

**Finding ledger (r5 review → r6 disposition).** Classes per `workflow-policy/review-findings-taxonomy.md`.

| ID | Class / severity | r5 anchor | r6 disposition |
|----|------------------|-----------|----------------|
| F1 | `statement-contradiction` / MEDIUM (blocking HIGH) | spec.md:13 R2 vs :95, :111 I2 | **Closed by design strengthening (AD11, I2 continuous form).** Union install makes index ⊇ header-bound(published catalog) at every instant; R2 stands verbatim, absolute, unweakened. No owner semantic decision required. Supersedes the reviewer's proposed snapshot-scoped R2 restatement (rejected alternative (f)). |
| F2 | `ac-not-falsifiable` / MEDIUM (blocking HIGH) | spec.md:11, :19, :125, :130, :156 | **Closed.** AC02-1 and AC03-1 now assert the peer records exactly one `tools/call` request on each R1/R5 success path; counts taken under the peer's lock in the post-call snapshot. |
| F3 | `stale-citation` / LOW (blocking HIGH) | spec.md:40 | **Closed.** `cmd/mcp-bridge/wiring.go:177-179` (re-verified: `upstream.NewDedicated` at :177). |
| F4 | `stale-citation` / LOW (blocking HIGH) | spec.md:33, :254 | **Closed.** `go.mod:8` (re-verified: go-sdk v1.8.0 at line 8). |
| F5 | `statement-contradiction` / LOW (blocking HIGH) | spec.md:107 | **Closed.** Sequences reordered: connect (request context) → admission → gate/pass → `tools/call`, matching `dedicated.go:377` and `runtime.go:190`. |
| F6 | `stale-citation` / LOW (blocking MEDIUM) | spec.md:103, :49, :50 | **Closed.** Call-timeout scoped to post-admission RPCs (`runtime.go:178`, `:186`); connect bounded by connect timeout (`dedicated.go:377`); `put` at `client.go:1276`/`cache.go:45`; `sortKeys` at `features.go:99-104`. |
| OQ1 | open question / LOW | r5 review open questions | **Closed (AD12).** AC03-2 asserts the observed negotiated version `2025-11-25`; probe rejection is client-side (`client.go:443-451`), discover exempt (`streamable.go:1520-1523`). |
| OQ2 | open question / LOW | r5 review open questions | **Closed (AD13).** Positive-TTL fixture pins `TTLMs: 60_000`, asserted strictly longer than the test's bounded runtime. |

**Highest-risk assumptions and decisions.** (a) The definition inserted by the pass on the execution session is visible to the immediately following call — test-verified by AC02-1 rather than assumed at runtime; if it fails, T2 reports BLOCKED. (b) *(r6)* The continuous I2 invariant rests on the publication-ordering rule: a catalog entry is served only from a returned discovery's result, and a failed discovery keeps both index and catalog unchanged (stale/fail-closed). Evidence is the existing refresh tests (`internal/service/refresh_test.go:230-243`, `:251-303`); this is the reviewer's highest-priority probe, because a future publication path that served definitions before their discovery returned would reintroduce a narrow under-inclusive window — manifesting as a payload-free peer rejection, never a missed header. (c) On a zero-TTL peer a concurrent stateful header call can lose the cache entry between the pass and the call and fail safely with an existing code; this is a `dependency`-class availability limitation, never a data-corruption or execution-replay risk. (d) The legacy gate is per call and reads the executing session's version; a discovery snapshot is deliberately not used, and admission precedes the gate by design.

**Coverage gaps the reviewer should probe.** (i) The pagination oracle pins an exact delta for a `PageSize: 1`, three-tool fixture; confirm the fixture's *names* sort the target last (the transport orders by name, `features.go:99-104`) and that the counts — now including exactly-one `tools/call` — are taken after discovery, not cumulatively. (ii) AC01-4 and AC03-1's `positive-ttl` subtest are controls and must not be counted as RED evidence; confirm the receipt separates controls from failures and that the fixture asserts the 60 s TTL exceeds the measured test duration. (iii) AC02-3 depends on a pre-existing test; confirm T1's baseline is recorded and that a red baseline yields a blocker, not an attributed failure. (iv) The vocabulary test lives in the upstream package by design (AD9); confirm that placement does not hide a drift between the dispatch message and the runtime map. (v) The handshake oracle counts `server/discover` instead of `initialize`; confirm the recorder classifies by body method so neither count is fabricated. (vi) *(r6)* AC03-2's negotiated-version assertion must read the recorded handshake/version evidence, not narrate it; confirm the `2025-11-25` value is asserted from the recorded evidence and that the `CodeUnsupportedProtocolVersion` reference is treated as the client-side mechanism, not a wire observable.

**Review targets with finding classes.**
- `ac-not-falsifiable`: AC01-3's evidence must be asserted by the test, not narrated; AC02-4 is a process property whose two runs must both exist in the receipt; AC01-4's one-`server/discover` count and the new exactly-one-`tools/call` counts must be assertions, not comments.
- `r-ac-coverage-gap`: R4 is covered only by AC02-1 — verify the body assertion is present and non-vacuous; R5's zero-RPC clause is carried only by AC03-1's positive-TTL subtest.
- `shared-oracle`: AC01-2/AC01-3/AC02-4 reference the AC01-1/AC02-1/AC02-3 probes by id; confirm the spec treats them as sub-assertions of one criterion rather than pretending to distinct oracles.
- `stale-citation`: the r6-corrected anchors to spot-check are `cmd/mcp-bridge/wiring.go:177`, `go.mod:8`, go-sdk `client.go:1276`, go-sdk `cache.go:45`, go-sdk `features.go:99-104`, go-sdk `streamable.go:1520-1523`, go-sdk `client.go:443-451`, `internal/upstream/runtime.go:178/186/190/204`, `internal/upstream/dedicated.go:377/401/431`, and the r5 additions `go-sdk client.go:323/390/517/599/1292`, `go-sdk server.go:183/979/1114`.
- `undefined-term`: `definition pass`, `header-bound`, `modern peer`, `legacy peer`, `control`, `handshake count`, `named code`, `union install` are defined in §2/§3/§4/§5.
- `statement-contradiction` (r6-specific): confirm no residual clause still claims the r5 installation-scoped I2 (§1 history and §9(b) reference it only as superseded history).

**W09E risk matrix (evidence + counterexample).**
- *Lifecycle*: stateless execution sessions are per call; the pass must run on that session (`internal/upstream/dedicated.go:359`). Counterexample rejected: warming a discovery session caches nothing after it closes. Evidence: the discovery close path at `internal/upstream/dedicated.go:71`.
- *Shared state*: the index is per runtime, read by concurrent calls and written only by successful discoveries; the union swap under the existing mutex mirrors the catalog rule. Counterexample considered and closed: install/publish skew — union install makes each install cover both the previously and newly published sets, and publication-after-discovery plus fail-closed stale entries (`internal/service/refresh_test.go:230-243`, `:251-303`) close the under-inclusive direction continuously; residual risk is bounded over-inclusion ≤ one refresh interval, costing at most one wasted definition pass (AD7 nil outcome). Counterexample rejected: rebuilding the index per call would add RPCs and could disagree with the published catalog.
- *Contract*: `x-mcp-header` presence is the only signal; invalid annotations are invisible because the SDK's filter runs before caching. Counterexample: an annotation value that is a number is dropped by the SDK, so the bridge cannot publish or call that tool — an existing SDK behavior, not this fix's regression. R2's absolute sentence and I2's continuous invariant now state the same property; counterexample walk (discovery removes a header-bound tool → install N+1 keeps the name → publish N+1 drops it from the catalog) finds no instant where a published header-bound tool lacks an index entry.
- *Authority*: planner authors this specification; the engineer implements T1–T3; T4 is operator-gated; the live deployment is not executable from the pipeline. No R/AC/T id, envelope path or ticket criterion changed in r6; no owner checkpoint is consumed by this repair.
- *Observability*: one reason value in three closed places; no unbounded label; the failure is queryable by `error.code` and the request outcome. Counterexample rejected: a metric label carrying per-code values.
- *Repair closure*: closure requires the recorded pre-change failure and the post-change pass for the same assertion, plus the delivery-gate receipt; a claim of “passes after” without the RED run is not closure. F1's closure is design-level and is exercised by the existing oracle set (no new test); the reviewer verifies the invariant argument, not a new GREEN.

## 10. ENVELOPE
<!-- specdd:envelope:start -->

**Envelope id `env-1`, root `/home/opencode/projects/default/mcp-bridge`.** Allowed write paths: `internal/upstream/dedicated.go`, `internal/upstream/runtime.go`, `internal/upstream/header_params_test.go`, `cmd/mcp-bridge/serve_header_params_test.go`, `internal/observe/log.go`, `internal/observe/envelope.go`, `internal/service/tools.go`, `docs/logging-contract.md`, `docs/runbooks/header-parameters.md`. Allowed effects: read, write, test. Forbidden effects: network (other than loopback test peers), publish, deploy, schedule, credentials, infrastructure. **Prohibited subsystems:** `go.mod`/`go.sum` and the module cache; `internal/config`; the inbound service and transport (`internal/service/inbound*`, `internal/http*`); storage and search; catalog publication; the breaker state machine; metrics/readiness definitions; `cmd/mcp-bridge/wiring.go`, `serve.go` and the harness files (read-only); deployment manifests and registries; the `.scratch` tracker files and `.omp/plans` state (coordinator/owner only). No new dependency, build tag or vendored patch; go-sdk stays at v1.8.0 (`go.mod:8`). *(r6: unchanged except the `go.mod:8` citation.)*
<!-- specdd:envelope:end -->

## APPENDIX — Planner-owned schema-2 state projection (r6)

Derived exactly from the current canonical state `.omp/plans/execution-state.json` (r5, schemaVersion 2). All `R6_*_PLACEHOLDER` values are for mechanical coordinator computation after the r6 specification is persisted; no digest below is asserted or fabricated. Task roles, effects, IDs, dependencies, acceptance bindings, `authorization: null`, `reviews: null`, `selectedBranchId: null`, capabilities, decisions, evidence, blockers, gate and the historical `sources` entry are preserved verbatim; the r5 packet branch is preserved as history and the r6 candidate branch is appended per the new registration.

```json
{
  "schemaVersion": 2,
  "feature": "header-parameter-transit",
  "revision": "r6",
  "projectRoot": "/home/opencode/projects/default/mcp-bridge",
  "spec": {
    "path": "/home/opencode/projects/default/mcp-bridge/.scratch/header-parameter-transit/spec.md",
    "sha256": "R6_SPEC_SHA256_PLACEHOLDER",
    "sectionBindings": {
      "requirements": { "revision": "r6", "sha256": "R6_SECTION_REQUIREMENTS_SHA256_PLACEHOLDER" },
      "design": { "revision": "r6", "sha256": "R6_SECTION_DESIGN_SHA256_PLACEHOLDER" },
      "tasks": { "revision": "r6", "sha256": "R6_SECTION_TASKS_SHA256_PLACEHOLDER" },
      "envelope": { "revision": "r6", "sha256": "R6_SECTION_ENVELOPE_SHA256_PLACEHOLDER" }
    }
  },
  "packet": {
    "digest": "R6_PACKET_DIGEST_PLACEHOLDER",
    "selectedBranchId": null,
    "branches": [
      {
        "id": "header-parameter-transit-r5",
        "answerIds": [],
        "taskIds": ["CR1", "CT1", "T1", "T2", "T3", "T4"],
        "envelopeId": "env-1",
        "sectionBindings": {
          "requirements": { "revision": "r5", "sha256": "e10e12b2491d7b2c2785be8588ddf1de0cb5dbe138d1148b280b5b37b651c26b" },
          "design": { "revision": "r5", "sha256": "a343d840acc08b3cd616ac55de24dc67c2dd6862feddd4854551bf186ecf06d4" },
          "tasks": { "revision": "r5", "sha256": "e58aa100be420a25f4d71f3175a098b1416f1c2eb37c1772a80f68053579509b" },
          "envelope": { "revision": "r5", "sha256": "793dc3c5951de85a2056febd23249e9002d631b0b8a347d41f0e8902ceac5de8" }
        }
      },
      {
        "id": "header-parameter-transit-r6",
        "answerIds": [],
        "taskIds": ["CR1", "CT1", "T1", "T2", "T3", "T4"],
        "envelopeId": "env-1",
        "sectionBindings": {
          "requirements": { "revision": "r6", "sha256": "R6_SECTION_REQUIREMENTS_SHA256_PLACEHOLDER" },
          "design": { "revision": "r6", "sha256": "R6_SECTION_DESIGN_SHA256_PLACEHOLDER" },
          "tasks": { "revision": "r6", "sha256": "R6_SECTION_TASKS_SHA256_PLACEHOLDER" },
          "envelope": { "revision": "r6", "sha256": "R6_SECTION_ENVELOPE_SHA256_PLACEHOLDER" }
        }
      }
    ]
  },
  "actors": { "plannerId": "planner", "implementerIds": ["engineer"] },
  "requirements": [
    { "id": "R1", "summary": "Header-bound calls transit in default stateless mode without a retained session" },
    { "id": "R2", "summary": "The SDK is the only emitter; the bridge ensures the execution session holds the tool definition" },
    { "id": "R3", "summary": "Body-only tools keep today's behavior, with no extra upstream RPC or header" },
    { "id": "R4", "summary": "Header-bound values stay in the arguments body; parameter headers are additive" },
    { "id": "R5", "summary": "Explicit stateful mode succeeds using cached definitions, refetching only when the TTL requires it" },
    { "id": "R6", "summary": "A legacy negotiated protocol yields parameter_header_unsupported with not_started and zero executions" },
    { "id": "R7", "summary": "The protocol boundary, operator handling and definition-pass cost are documented" },
    { "id": "R8", "summary": "Failure paths stay payload-free in the closed vocabulary plus one additive reason token" },
    { "id": "R9", "summary": "A deterministic peer-based reproduction records wire facts in both modes" },
    { "id": "R10", "summary": "The deployed github upstream is verified end to end under operator gate" },
    { "id": "R11", "summary": "Stateless restart resilience is preserved" }
  ],
  "authorization": null,
  "reviews": null,
  "sources": [
    {
      "id": "src-spec-r1",
      "kind": "repository",
      "artifact": {
        "path": "/home/opencode/projects/default/mcp-bridge/.omp/artifacts/execution-state-r1-spec-8054d28f24cd.md",
        "sha256": "8054d28f24cdf82475a27df8c0ad3e73ed5bf36da91db299be709e2e147b8347"
      },
      "locator": "planner specification r1 for header-parameter-transit (superseded by r2, r3 and r4; retained as historical evidence)",
      "recordedAt": "2026-11-20T00:00:00.000Z"
    }
  ],
  "decisions": [],
  "envelope": {
    "id": "env-1",
    "root": "/home/opencode/projects/default/mcp-bridge",
    "allowedTaskIds": ["CR1", "CT1", "T1", "T2", "T3", "T4"],
    "readPaths": [
      ".scratch/header-parameter-transit/spec.md",
      ".scratch/header-parameter-transit/issues/01-header-parameter-wire-reproduction.md",
      ".scratch/header-parameter-transit/issues/02-header-parameters-traverse-stateless-calls.md",
      ".scratch/header-parameter-transit/issues/03-stateful-and-legacy-protocol-boundary.md",
      ".scratch/header-parameter-transit/issues/04-deployed-github-verification.md",
      ".scratch/mcp-bridge-defects/backlog.md",
      "docs/agents/domain.md",
      "docs/agents/issue-tracker.md",
      "docs/logging-contract.md",
      "go.mod",
      "internal/config/config.go",
      "internal/core/core.go",
      "internal/observe/envelope.go",
      "internal/observe/log.go",
      "internal/observe/metrics.go",
      "internal/service/direct_execution.go",
      "internal/service/tools.go",
      "internal/upstream/client.go",
      "internal/upstream/dedicated.go",
      "internal/upstream/dedicated_test.go",
      "internal/upstream/runtime.go",
      "internal/upstream/upstream_test.go",
      "cmd/mcp-bridge/logging_contract_test.go",
      "cmd/mcp-bridge/serve_test.go",
      "cmd/mcp-bridge/serve_upstream_restart_test.go",
      "cmd/mcp-bridge/wiring.go"
    ],
    "writePaths": [
      "internal/upstream/dedicated.go",
      "internal/upstream/runtime.go",
      "internal/upstream/header_params_test.go",
      "cmd/mcp-bridge/serve_header_params_test.go",
      "internal/observe/log.go",
      "internal/observe/envelope.go",
      "internal/service/tools.go",
      "docs/logging-contract.md",
      "docs/runbooks/header-parameters.md"
    ],
    "allowedEffects": ["read", "write", "test"],
    "forbiddenEffects": ["network", "publish", "deploy", "schedule", "credentials", "infrastructure"],
    "decisionPolicy": { "technicalDecisionIds": [], "ownerCategories": [] },
    "retryPolicy": { "maxAttempts": 1, "resetOn": ["owner-signal", "material-evidence"] }
  },
  "tasks": [
    {
      "id": "CR1", "stage": "research", "status": "pending", "blockedBy": [], "requires": [],
      "effects": { "kind": "read", "paths": [
        ".scratch/header-parameter-transit/spec.md",
        ".scratch/header-parameter-transit/issues/01-header-parameter-wire-reproduction.md",
        ".scratch/header-parameter-transit/issues/02-header-parameters-traverse-stateless-calls.md",
        ".scratch/header-parameter-transit/issues/03-stateful-and-legacy-protocol-boundary.md",
        ".scratch/header-parameter-transit/issues/04-deployed-github-verification.md"
      ] },
      "acceptanceIds": [], "evidenceIds": [], "agentRoles": ["code-reviewer"]
    },
    {
      "id": "CT1", "stage": "research", "status": "pending", "blockedBy": [], "requires": [],
      "effects": { "kind": "read", "paths": [
        ".scratch/header-parameter-transit/spec.md",
        ".scratch/header-parameter-transit/issues/01-header-parameter-wire-reproduction.md",
        ".scratch/header-parameter-transit/issues/02-header-parameters-traverse-stateless-calls.md",
        ".scratch/header-parameter-transit/issues/03-stateful-and-legacy-protocol-boundary.md",
        ".scratch/header-parameter-transit/issues/04-deployed-github-verification.md"
      ] },
      "acceptanceIds": [], "evidenceIds": [], "agentRoles": ["critic"]
    },
    {
      "id": "T1", "stage": "implementation", "status": "pending", "blockedBy": [], "requires": [],
      "effects": { "kind": "write", "paths": ["internal/upstream/header_params_test.go", "cmd/mcp-bridge/serve_header_params_test.go"] },
      "acceptanceIds": ["AC01-1", "AC01-2", "AC01-3", "AC01-4", "AC01-5"], "evidenceIds": [], "agentRoles": ["engineer"]
    },
    {
      "id": "T2", "stage": "implementation", "status": "pending", "blockedBy": ["T1"], "requires": [],
      "effects": { "kind": "write", "paths": [
        "internal/upstream/dedicated.go", "internal/upstream/runtime.go", "internal/upstream/header_params_test.go",
        "internal/observe/log.go", "internal/observe/envelope.go", "internal/service/tools.go"
      ] },
      "acceptanceIds": ["AC02-1", "AC02-2", "AC02-3", "AC02-4", "AC02-5", "AC03-1", "AC03-2", "AC03-4"], "evidenceIds": [], "agentRoles": ["engineer"]
    },
    {
      "id": "T3", "stage": "verification", "status": "pending", "blockedBy": ["T2"], "requires": [],
      "effects": { "kind": "write", "paths": ["docs/logging-contract.md", "docs/runbooks/header-parameters.md"] },
      "acceptanceIds": ["AC03-3"], "evidenceIds": [], "agentRoles": ["engineer"]
    },
    {
      "id": "T4", "stage": "external", "status": "pending", "blockedBy": ["T3"], "requires": [],
      "effects": { "kind": "read", "paths": ["docs/runbooks/header-parameters.md"] },
      "acceptanceIds": ["AC04-1", "AC04-2", "AC04-3", "AC04-4"], "evidenceIds": [], "agentRoles": []
    }
  ],
  "capabilities": [],
  "acceptance": [
    { "id": "AC01-1", "requirementIds": ["R9"], "ownerTaskIds": ["T1"], "method": "test", "requiredEvidenceKinds": ["test"] },
    { "id": "AC01-2", "requirementIds": ["R9"], "ownerTaskIds": ["T1"], "method": "test", "requiredEvidenceKinds": ["test"] },
    { "id": "AC01-3", "requirementIds": ["R9"], "ownerTaskIds": ["T1"], "method": "test", "requiredEvidenceKinds": ["test"] },
    { "id": "AC01-4", "requirementIds": ["R9"], "ownerTaskIds": ["T1"], "method": "test", "requiredEvidenceKinds": ["test"] },
    { "id": "AC01-5", "requirementIds": ["R9"], "ownerTaskIds": ["T1"], "method": "inspection", "requiredEvidenceKinds": ["inspection"] },
    { "id": "AC02-1", "requirementIds": ["R1", "R2", "R4"], "ownerTaskIds": ["T2"], "method": "test", "requiredEvidenceKinds": ["test"] },
    { "id": "AC02-2", "requirementIds": ["R3"], "ownerTaskIds": ["T2"], "method": "test", "requiredEvidenceKinds": ["test"] },
    { "id": "AC02-3", "requirementIds": ["R11"], "ownerTaskIds": ["T2"], "method": "test", "requiredEvidenceKinds": ["test"] },
    { "id": "AC02-4", "requirementIds": ["R1", "R3", "R11"], "ownerTaskIds": ["T2"], "method": "test", "requiredEvidenceKinds": ["test"] },
    { "id": "AC02-5", "requirementIds": ["R8"], "ownerTaskIds": ["T2"], "method": "test", "requiredEvidenceKinds": ["test"] },
    { "id": "AC03-1", "requirementIds": ["R5"], "ownerTaskIds": ["T2"], "method": "test", "requiredEvidenceKinds": ["test"] },
    { "id": "AC03-2", "requirementIds": ["R6"], "ownerTaskIds": ["T2"], "method": "test", "requiredEvidenceKinds": ["test"] },
    { "id": "AC03-3", "requirementIds": ["R7"], "ownerTaskIds": ["T3"], "method": "inspection", "requiredEvidenceKinds": ["inspection"] },
    { "id": "AC03-4", "requirementIds": ["R5", "R6"], "ownerTaskIds": ["T2"], "method": "test", "requiredEvidenceKinds": ["test"] },
    { "id": "AC04-1", "requirementIds": ["R10"], "ownerTaskIds": ["T4"], "method": "external", "requiredEvidenceKinds": ["external"] },
    { "id": "AC04-2", "requirementIds": ["R10"], "ownerTaskIds": ["T4"], "method": "external", "requiredEvidenceKinds": ["external"] },
    { "id": "AC04-3", "requirementIds": ["R10"], "ownerTaskIds": ["T4"], "method": "external", "requiredEvidenceKinds": ["external"] },
    { "id": "AC04-4", "requirementIds": ["R10"], "ownerTaskIds": ["T4"], "method": "external", "requiredEvidenceKinds": ["external"] }
  ],
  "evidence": [],
  "blockers": [],
  "gate": {
    "markerPath": "/home/opencode/projects/default/mcp-bridge/.superpowers/sdd/GATE_OPEN",
    "activationEvidenceIds": []
  }
}
```

**Ticket mirror replacements: none required.** The r6 amendments change only specification prose and AC observable text; every ticket checkbox in `issues/01`–`04` is unchanged in meaning and text, the §5 `Ticket checkbox` column references remain accurate, and no criterion was merged, split or weakened. The ticket files are coordinator/owner-owned (`.scratch`, outside the env-1 write set), so no mirror edit is specified.

**Planning-lane notes.** (1) `workflow-policy/spec-lint.mjs` was not executed: the read-only planner does not persist the specification file, and r6 exists only as this structured planning sink; the coordinator MUST run `spec-lint --spec <persisted r6> --tickets .scratch/header-parameter-transit/issues --state .omp/plans/execution-state.json --root /home/opencode/projects/default/mcp-bridge` after persistence (non-zero exit is a self-repair, not a review round). Mechanical stale-citation checks were performed manually against the tree and go-sdk v1.8.0 sources instead; every anchor changed or added in r6 was re-read and verified. (2) Semantic search tools were unavailable to this read-only role; all retrieval was bounded native read/grep. (3) One read outside the assignment's listed seams was required and performed: `internal/service/refresh_test.go:230-303` (publication-ordering evidence decisive for F1's continuous invariant); read-only, no runtime probe was executed. (4) No behavior was verified at runtime; all §3 VERIFIED entries are source-anchored reads, and the planned RED/GREEN runs remain the engineer's execution evidence.