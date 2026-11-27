# Spec — token-efficient-tool-responses / Ticket 01: bounded tool results with recoverable paging

## 1. STATUS

- **Spec ID / revision:** `ticket01-bounded-results-v30` (r30 measurement-oracle repair; supersedes v29)
- **Status:** DRAFT (Critic gate repair; planner does not approve)
- **Objective:** Guarantee that no bridge tool response writes an unbounded payload to the client — when the fully serialized result exceeds the configured delivery ceiling (default 8 KiB) the caller receives a bounded preview plus a bridge-issued, owner-bound handle and can recover the complete result through bounded paged retrieval — while preserving session isolation, projection ownership/epoch invariants, catalog-revision checks, and the existing error taxonomy.

**Problem and intent (tied to observed token cost):** bridge discovery and generic dispatch consumed ~44% of billed prompt tokens. The bridge re-serializes tool results twice (text block + `structuredContent`, plus `upstream_result` for `call_tool`) and delivers discovery/describe unbounded. Bounding the *serialized* output caps the worst case a single tool response can cost, without hiding execution facts: the preview keeps `isError` visible and the complete result stays recoverable through bounded pages. This ticket only bounds the worst case; it does not add default projections (ticket 05) or deduplication (ticket 06).

## 2. REQUIREMENTS

<!-- specdd:requirements:start -->
All requirements are **enforced** (falsifiable acceptance contract) unless marked otherwise. IDs are stable; the original ticket ACs O1–O9 map as O1→R1,R2; O2→R3,R4; O3→R7; O4→R8; O5→R9; O6→R10; O7→R11; O8→R11; O9→R12.

- **R1 (enforced, delivery-scoped, total retention up to the per-entry cap).** Any tool result whose fully serialized form exceeds the configured **delivery** ceiling (`delivery_ceiling_bytes`, default 8192) is delivered as a bounded preview plus an opaque handle — **total retention up to the per-entry cap** (owner decision). The preview reports `total_bytes` and `paging_required`, and the upstream error class `is_error` (+ `upstream_code` when applicable); its `Content` is a non-empty, UTF-8-safe byte prefix. The whole preview is ≤ the configured preview ceiling (default 1536), which is **strictly less than** the delivery ceiling (config-validated, F1). **Unretainable outcomes (each a typed terminal, never a silent or empty success, and each a bounded preview with a non-empty `Content` prefix, no handle, and the upstream error class preserved):** (a) `result_capacity_exceeded` — the serialized form exceeds `max_retained_result_bytes` (default 8388608); (b) `result_namespace_retired` — the owner namespace was retired before the retention commit (C2); (c) `result_entropy_unavailable` — `crypto/rand` failed at handle generation after the upstream already executed (F3). R1 governs the dispatch-time delivery ceiling only; retrieved pages are bounded by the handle's pinned ceiling and are exempt from the current configured ceiling (F2). **Terminal precedence:** (1) no resolvable owner — a session-ful request with a non-valid epoch is rejected before the seam per §4.0: `call_tool` (which bypasses admission) returns the handler `invalid_epoch` (`errorResult(codeInvalidEpoch, "the caller's session epoch is not valid", ExecutionNotStarted)`); the exposure-gated surfaces return the admission middleware JSON-RPC −32600 `"invalid exposure request"`; (2) `result_capacity_exceeded` at the seam (size check); (3) `result_namespace_retired` at the commit (liveness check); (4) `result_entropy_unavailable` at handle generation. Envelope infeasibility is a config-load rejection, not a runtime terminal (M3).
- **R2 (enforced).** A result ≤ the configured delivery ceiling (default 8192) **whose owner resolves** is delivered byte-identically; when the owner does not resolve, the per-surface `ownerOK=false` rejection of §4.0 is returned (handler `invalid_epoch` for `call_tool`; middleware JSON-RPC −32600 for the exposure-gated surfaces). Falsifiable boundary: 8192 bytes unchanged; 8193 pages.
- **R3 (enforced).** Each retrieved page is ≤ its handle's **pinned** delivery ceiling serialized bytes and reports: 1-based `page`, `total_pages`, `remaining_pages`, `total_bytes`, `remaining_bytes`.
- **R4 (enforced).** Concatenating the page content reproduces the unbounded serialized result byte-exactly; re-retrieval of the same handle/page returns identical bytes until expiry or eviction — including across a `delivery_ceiling_bytes` reload. The entry's `C_envelope` is pinned with the entry at retention time, exactly as the delivery ceiling is pinned, so a reload cannot make a retained partition infeasible.
- **R5 (enforced).** Every page is valid UTF-8; a multibyte rune spanning a page boundary round-trips; the ceiling measures post-escape serialized bytes.
- **R6 (enforced).** The retrieval operation, parameters, and preview/page contract are documented in `README.md`; the operation, parameters, and contract appear in the tool description (`handle`, `page`, a paging term); the parameters appear in the stored input schema. *(Advisory sub-part: discovery-steering prose.)*
- **R7 (enforced).** Handles are cryptographically unpredictable: the generator **must** use `crypto/rand` (or an equivalent OS CSPRNG) — a **mandatory implementation property verified by inspection evidence recorded in the task receipt** (owner decision, C1); pairwise distinctness is **necessary, not sufficient**. Enforced behavioural checks: identical result content yields distinct handles and the handle bytes are independent of the result content and the session identifier (AC14); unknown/cross-namespace handles return the identical `invalid_handle` with zero retained/page bytes and no existence signal (AC7); an entropy failure is the typed `result_entropy_unavailable` retention terminal, never a guessable handle (AC13). A malformed handle or out-of-range page is `invalid_arguments` (shape before store; range after lookup). **Monotonic non-reuse (L1):** `(sessionID, epoch)` is never reused — `nextEpoch` is monotonically increasing across all owners.
- **R8 (enforced).** Expired and evicted handles fail with distinct codes — `result_expired`/`result_evicted` — deterministically **while their bounded tombstone is retained**; after tombstone overflow or `tombstone_ttl`, the handle returns `invalid_handle`. Never an empty page or partial success.
- **R9 (enforced).** Retained-result storage has a hard per-namespace byte bound `max_bytes_per_session` (default 16777216) and entry bound `max_entries_per_session` (16), plus an **advisory** aggregate byte target (default 67108864, best-effort evict-own-then-admit). **Retirement-versus-retention (C2):** retention commits only when the owner namespace is live at commit, checked under the same lock that admits the entry; a racing retirement either wins (no retention, `result_namespace_retired`) or loses (retention under a live namespace). Cross-namespace eviction isolation holds; within the shared publisher namespace all session-less callers are one owner. Concurrent retrieval returns a complete page or a typed error.
- **R10 (enforced).** An oversized upstream `isError` result keeps `is_error` and `upstream_code` visible in the preview and every page. `upstream_code` is the upstream result's own top-level structured `code` **when it is a string** — `call_tool` from decoded `upstream_result.structuredContent.code`, direct projected from `structuredContent.code` — present as a bounded string only when `is_error`, omitted if absent or non-string, with a `…` marker iff truncated rune-safely. The full code (and full structured error content) is byte-exact in the retained result. Bridge dispatch failures keep their small typed shape and are never paged. The upstream error class is preserved on every branch including the unretainable terminals (no R10 exception is needed — the class is available at the seam).
- **R11 (enforced).** The ceiling applies identically to `call_tool`, directly invoked projected tools, `search_tools`, and `describe_tool`; paging never depends on the caller predicting the output shape. The discovery seam runs after `commitDiscovery`. *(Forward compatibility: a future ticket-05 projection or `full` request remains subject to this ceiling.)*
- **R12 (enforced).** Durable behavioral tests cover R1–R11: pass-through at/below the 8192 boundary, multi-page byte reconstruction (incl. escape-dense fill), UTF-8 boundary round-trip, cross-namespace rejection, reused-session-ID rejection, forged-handle rejection, invalid-epoch rejection, expiry/eviction (with tombstones and overflow), error-result paging, over-cap and namespace-retired terminals, both dispatch paths, discovery/describe (commit preserved), and serialized byte bounds.
<!-- specdd:requirements:end -->

## 3. EVIDENCE AND UNCERTAINTY

### Verified facts (read-only inspection)

- `call_tool` builds `{server, tool_name, request_id, upstream_result}` as `StructuredContent` and returns the upstream `Content` (`internal/service/tools.go:865-908`). `call_tool` resolves no owner (`internal/service/tools.go:700-863`); `directRequestOwner` is used by direct (`internal/service/tools.go:225`), `search_tools` (`internal/service/tools.go:439`), `describe_tool` (`internal/service/tools.go:562`).
- **No upstream body cap is enforced** (`internal/upstream/client.go` `NewClient` installs only `CheckRedirect`); oversize is the typed terminal, no shared-transport change (owner decision).
- Sessions carry session ID + `X-Mcp-Bridge-Exposure-Epoch`; owner `{kind, sessionID, epoch}` (`internal/service/projection.go:1-40,195-202`). `nextEpoch` is one monotonic counter across all owners (`internal/service/projection.go:190-202`).
- **`requestExposureEpoch` collapses absent/zero/invalid into `valid=false` (`internal/service/session_exposure.go:154-172`); `directRequestOwner` keys the publisher fallback on session-lessness (`internal/service/tools.go:315-324`).**
- **Per-surface invalid-epoch rejection (C5 + F2, r26-F1):** `call_tool` (bypasses admission) → `errorResult(codeInvalidEpoch, "the caller's session epoch is not valid", ExecutionNotStarted)`; `search_tools`/`describe_tool`/direct-projected (exposure-gated) → admission middleware JSON-RPC −32600 `"invalid exposure request"` (`internal/service/session_exposure.go:162-166`).
- `admitExposureRequest` admits session-less to the publisher projection (`internal/service/session_exposure.go:105-120`); invalid epoch → `jsonrpc.CodeInvalidRequest` "invalid exposure request" (`internal/service/session_exposure.go:162-166`).
- Owner retirement: three seams (idle reaper `internal/service/inbound.go:1120-1145`, `DELETE /mcp`, SDK-session-loss `internal/service/inbound.go:800-816`); rehydration is not a seam.
- **Inbound epoch invariant:** the inbound handler strips any client-supplied `X-Mcp-Bridge-Exposure-Epoch` and stamps only the admitted server-issued epoch, so incarnations cannot be forged by a client. Stamping sites: the idle reaper (`internal/service/inbound.go:1120-1145`) and the SDK-session-loss seam (`internal/service/inbound.go:800-816`); `newLiveOwner` is invoked only when epoch is 0.
- Stored schemas `additionalProperties:false` (`internal/catalog/schema.go:74-89`); the SDK delegates tool-argument validation to the caller (see `mcp.Server.AddTool`); handlers only `decodeStrict` (`internal/service/tools.go:1074-1081`).
- `upstream_defaults.max_response_bytes` = 8388608 (`internal/config/bounds.go:86`), read but not enforced. The serialized form duplicates content+structured and escapes up to 6× per byte.
- **`internal/observe/envelope.go` rejects unknown `Entry.Fields` keys** (`addEnvelopeFields`, `internal/observe/envelope.go:252-488`): the new event fields `total_bytes`, `page`, `total_pages`, `max_retained_result_bytes` are **not** in the current vocabulary and would fall back to a rejected record (C3). The exact-union fixtures `internal/observe/log_test.go:46-80` and `cmd/mcp-bridge/logging_contract_test.go:114-118` pin this vocabulary.
- **`cmd/covcheck` reads `coverage.out` by default (`cmd/covcheck/main.go:22-42`)**; a differently-named profile would be ignored (C4).
- Bridge error codes (`internal/service/tools.go:993-999`); `errorResult` (`internal/service/tools.go:1037-1048`); core dispatch codes (`internal/core/core.go:77-82`). `requiresExposure` (`internal/service/session_exposure.go:60-83`); no `ToolSurface.Close`; reaper on the inbound admission path (`internal/service/inbound.go:1120-1145`).
- The process `observe.Metrics` is built once by `NewMetrics` and served by `Metrics.Handler()`. **The `ToolSurfaceConfig` at `cmd/mcp-bridge/serve.go:164` does not yet carry a `Metrics` field; adding that field and wiring it is work T2 performs (F5).** The coverage gate is in `.github/workflows/verify.yml`.

### Built-in-surface assertion inventory (coordinator-verified)

| File | Assertion |
| --- | --- |
| `internal/service/tools_test.go:512-548` | exact built-in descriptor set |
| `internal/service/progressive_session_test.go:180-183` | wanted names include the three built-ins |
| `internal/service/progressive_concurrency_lifecycle_test.go:697-731` | exact surface in lifecycle schedule |
| `internal/service/tool_metadata_test.go:108-120` | `want` map of three names, `len(got)!=3` |
| `internal/service/tools_publish_atomic_test.go:50-57` | `want` map + `alpha__good`, `len(got)!=4` |
| `cmd/mcp-bridge/serve_hybrid_exposure_test.go:201-204` | `tools/list length == 4` |
| `cmd/mcp-bridge/serve_hybrid_reload_test.go:217,226,238,262` | 4/5/4 across reload phases |
| `cmd/mcp-bridge/serve_hybrid_progressive_reveal_test.go:352-390` | exact name sets |
| `cmd/mcp-bridge/serve_hybrid_progressive_reveal_lifecycle_test.go:278-286,492` | exact name sets |
| `cmd/mcp-bridge/serve_oauth_test.go:449,500` | `exposed_tools == 3` |
| `cmd/mcp-bridge/serve_test.go:618` | `len(listed.Tools) != 3` |

Presence-only (`cmd/mcp-bridge/serve_hybrid_metadata_test.go:143`, `cmd/mcp-bridge/serve_oauth_test.go:1734`) unaffected. Numbers: internal `3`→`4`/`53`→`54`/`1053`→`1054`/`3`→`4`/`4`→`5`; `cmd/mcp-bridge` `4`→`5`/`5`→`6`/`4`→`5`/`exposed_tools 3`→`4`/`len 3`→`4`; name maps gain `retrieve_result`.

### README changed-claim anchors (H3 + F8 + M2 + F6)

`README.md:279` ("One MCP server, three tools"), `:287` ("All three declare standard MCP tool annotations"), `:322-325` (discovery/describe pruning + "complete upstream result without duplicating"), `:332` ("`search_tools` includes the revision at the response root"), `:443` ("exactly the three bridge built-ins"), `:467` ("Executions return the upstream result unaltered inside a provenance envelope"), `:468` ("is the complete original `CallToolResult`").

### Assumptions

- The transport serializes `CallToolResult` with `encoding/json` HTML escaping; the oracle marshals via the SDK `MarshalJSON` path (K1).
- Oversize is handled by the per-entry cap terminal, not enforcement (owner decision).

### `[AUTO-DECISION]` entries

1. **Byte-accounting oracle.** Invariant: the measured length equals the byte length of the SDK-marshaled complete CallToolResult; every occurrence contributes its own bytes and no deduplication, interning, or structural sharing may reduce it. Independent oracle: a duplicate-bearing fixture whose text block and structuredContent carry identical payload bytes, with expected serialized length computed without `measureResult` and compared against the delivered bytes. The fixture must cross the 8192 boundary, so a deduplicating build ships a preview at half the real size and fails.
2. **Page model.** Retained value = serialized form S; page = UTF-8-safe slice; concatenation == S.
3. **Page partition.** Maximal-fill against the pinned ceiling, measured on the escaped wire size.
4. **Error-code vocabulary.** `invalid_handle`, `result_expired`, `result_evicted` (tombstoned), `invalid_arguments`, `invalid_epoch` (session-epoch rejection, F2), `internal_error` (general), `result_capacity_exceeded`, `result_namespace_retired`, `result_entropy_unavailable` (F3). `result_too_large`/`no_session`/`too_large`/`capacity` removed.
5. **Binding identity.** Handles bind to the resolved owner (session namespaces, or the shared publisher namespace).
6. **Error class.** `is_error` + `upstream_code` preserved on every branch including the unretainable terminals.
7. **Remaining arithmetic.** `remaining_*` counts strictly after the current page.
8. **`upstream_code` form.** String only when `is_error` and the upstream's top-level `code` is a string; omitted otherwise; never null; bounded `max_upstream_code_bytes` (default 128, configurable ≤512) rune-safe with `…` marker iff truncated; full code byte-exact in the retained result.
9. **`total_bytes` width.** `max_total_bytes_digits = 20`.
10. **Pinned ceiling.** `pinned_delivery_ceiling_bytes` per entry.
11. **Typed terminals.** Total retention up to the per-entry cap; over-cap → `result_capacity_exceeded`; retired namespace → `result_namespace_retired`; entropy failure → `result_entropy_unavailable` (`execution_state` unknown).
12. **Quantified bounds (L1/F4).** `max_retained_result_bytes = 8388608`; `max_bytes_per_session = 16777216`; `max_entries_per_session = 16`; `tombstone_bytes_per_session = 1048576`; `max_bytes_aggregate = 67108864` (advisory); `max_sessions` (1–4096); `preview_ceiling_bytes < delivery_ceiling_bytes` (F1); aggregate exposure = `(max_sessions + 1) × (max_bytes_per_session + tombstone_bytes_per_session)`.
13. **Tombstones.** 16 entries / 1 MiB / 10m TTL; overflow evicts oldest → `invalid_handle`.
14. **Collectors.** Five collectors on the process registry; T2 adds the `Metrics` field to `ToolSurfaceConfig` and wires it at `cmd/mcp-bridge/serve.go:164`.
15. **Owner resolution (F1).** Publisher fallback keyed on session-lessness; a session-ful non-valid epoch is rejected per §4.0 (handler `invalid_epoch` for `call_tool`; middleware −32600 for the exposure-gated surfaces).
16. **CSPRNG mandatory (C1).** `crypto/rand` (or equivalent OS CSPRNG) is mandatory, verified by inspection evidence in the receipt.
17. **Retirement-versus-retention (C2).** Retention commits only under a live namespace, checked under the admission lock.
18. **Field-union co-ownership (C3).** The emitter, the field registry, the exact-union fixtures, and the documented vocabulary change together in T2.
19. **Paging-counter binding (C6/F1).** Counters bound to committed outcomes with exact deltas; ownership per the surface that introduces the behavior.
20. **Entropy-failure truthfulness (F3).** Entropy failure occurs after upstream execution, so the terminal reports `execution_state: "unknown"`, never `"not_started"`.

### Inaccessible prerequisites

- None that block planning. Owner decisions are in `.scratch/token-efficient-tool-responses/decisions.md`.

### W09E evidence-packet assessment

Compact anchored risk evidence and counterexamples are placed in §10. Decisive gaps: none.

## 4. DESIGN

<!-- specdd:design:start -->
### 4.0 Owner resolution and per-surface rejection matrix

Every surface resolves its handle owner through `directRequestOwner(req)` **before the ceiling decision**; `call_tool` is changed to do so before dispatch. `directRequestOwner(req)` yields `publisherOwner` (session-less: `req.GetSession() == nil || req.GetSession().ID() == ""`), `{kind:session, sessionID, epoch}` (session-ful with a valid epoch), or no owner (`ownerOK=false`, session-ful with a non-valid epoch — absent/zero/invalid, collapsed by `requestExposureEpoch`). **The publisher fallback is keyed on session-lessness, never on the header.** **Monotonic non-reuse:** `nextEpoch` is one monotonic counter across all owners.

**Per-surface `ownerOK=false` rejection (C5 + F2, r26-F1).** `call_tool` bypasses exposure admission, so only its handler returns the typed `invalid_epoch` (`errorResult(codeInvalidEpoch, "the caller's session epoch is not valid", ExecutionNotStarted)`, `not_started`, no retention). `search_tools`, `describe_tool`, and directly invoked projected tools are exposure-gated and are rejected first by the admission middleware — JSON-RPC −32600 (`jsonrpc.CodeInvalidRequest`), message `"invalid exposure request"`, no tool result. `catalog_changed` keeps its exact meaning (actual catalog change) and is **not** reused for epoch rejection. Oracles: `TestInvalidEpochRejectedBeforeSeam` (T4, `call_tool`); `TestInvalidEpochMiddlewareRejection` (T4, the three exposure-gated surfaces).

**Terminal set (four, each typed, never a handle-less preview for a within-cap live result):** (1) no-owner → the per-surface rejection above; (2) `result_capacity_exceeded` (seam size check); (3) `result_namespace_retired` (commit liveness check); (4) `result_entropy_unavailable` (handle generation). Envelope infeasibility is config-load only (M3).

### 4.1 Selected approach

A single measure/decide seam at each of the four result-producing surfaces: measure → if ≤ delivery ceiling return byte-identically; else retain the serialized form (session or publisher namespace) up to the per-entry cap and return a ≤ preview-ceiling preview + handle (or the capacity/namespace-retired/entropy terminal). A fourth built-in tool `retrieve_result` pages the retained form. For `search_tools`/`describe_tool`, the seam runs **after** `commitDiscovery` (K7; oracle M5).

### 4.2 Caller-visible contract

**Under-ceiling.** Byte-identical. No handle, no metadata.

**Over-ceiling, within the per-entry cap (total retention).** Preview: MCP `isError` = original; `Content` = non-empty UTF-8-safe byte prefix of S sized ≤ preview ceiling; `StructuredContent` = `{ paging_required: true, handle, total_bytes, is_error, upstream_code }`.

**Over-ceiling, unretainable (typed terminal).** Bounded preview with MCP `isError` = original, `Content` = non-empty UTF-8-safe byte prefix of S, and `StructuredContent` = `{ paging_required: false, total_bytes, is_error, upstream_code, retention: "result_capacity_exceeded" | "result_namespace_retired" | "result_entropy_unavailable", execution_state: "unknown" }` — no handle; the upstream error class is preserved. `retention` names the cause; `execution_state` is always `"unknown"` (never `"not_started"`) because the upstream already executed and a retry could duplicate a mutating call (F3).

**Each page.** MCP `isError` = original; `Content` = the page's UTF-8-safe slice of S; `StructuredContent` = `{ handle, page, total_pages, remaining_pages, total_bytes, remaining_bytes, is_error, upstream_code }`. **Remaining arithmetic:** `remaining_pages = total_pages - page`; `remaining_bytes` = sum of later slice lengths; both 0 on last page. Concatenation == S. The per-entry `upstream_code` bound and rendered value are pinned with the entry at retention time, so reloading `max_upstream_code_bytes` cannot change a re-retrieved page's bytes; byte-identity of the same handle/page across such a reload is asserted. No new config key.

**Serialization and page fill.** S is the retained serialized form; a page's wire size (content-block wrapper + JSON-re-escaped slice + metadata + `isError`) is what the pinned ceiling bounds (escape-dense-aware).

**Error-code source rule (H5 + L3).** `upstream_code` populated only when the upstream's top-level `code` is a string; non-string/absent → omitted; full structured error content byte-exact in the retained result.

**`upstream_code` bounding.** String only when `is_error` and code is a string; omitted otherwise; never null; bounded `max_upstream_code_bytes` (default 128, configurable ≤512) rune-safe with `…` marker iff truncated.

**Discriminator vs ticket 02.** Paging = `paging_required` + `handle`; ticket 02 count clipping = `truncated` without a handle; the terminals use `retention`.

### 4.3 Retrieval surface (fourth tool)

`retrieve_result` is a fourth built-in tool, exposure-relevant via a new `requiresExposure` case. Input schema `additionalProperties:false`: `{handle: ^[A-Za-z0-9_-]{43}$, page: integer >=1}`. `handle` = 32 random bytes base64url; `page` optional default 1. Annotations `readOnlyHint:true`, `destructiveHint:false`, `idempotentHint:true`, `openWorldHint:false`. Description contains `handle`, `page`, `paging`.

**Two-phase validation.** Pre-store: `decodeStrict` + 43-char base64url + integer page ≥1 → `invalid_arguments` zero store access (a shape-invalid handle — wrong length/alphabet — or a non-integer/`<1` page is rejected here, pre-store). Post-lookup: `page > total_pages` → `invalid_arguments`. Between: owner-namespaced lookup → `invalid_handle`/`result_expired`/`result_evicted` (a well-formed-but-forged/unknown/cross-namespace handle is rejected here, post-lookup).

**Owner-namespaced retrieval (M3 + F1 + F2).** Session-less (no session ID) → publisher namespace (shared): retrieves publisher-bound, `invalid_handle` for session-bound; session caller `invalid_handle` for publisher-bound/other-session; a session-ful non-valid epoch → admission rejection (`invalid exposure request`, the middleware, or `invalid_epoch` handler-side); retired-epoch caller → admission rejection; live-epoch-with-retired-epoch handle → `invalid_handle`; reused session ID → new epoch cannot read old epoch's handles.

**`invalid_handle` scope.** Unknown, tampered, cross-namespace, live-epoch-with-retired-epoch, reused-session-ID, tombstone-elapsed/overflow — one fixed shape, zero retained/page bytes.

**Built-in-surface inventory.** §3 inventory owned by T3; a future fifth tool repeats the step.

**Decision note:** OD1-A adopted.

### 4.4 Byte-accounting definition and envelope feasibility

- **Measured form:** complete `CallToolResult` value via SDK `MarshalJSON` (HTML escaping); JSON-RPC wrapper excluded.
- **What counts once:** each distinct serialized form per occurrence; duplicated forms counted per copy.
- **Escaping:** escape expansion counts; multibyte UTF-8 emitted raw.
- **`total_bytes`** = `measureResult(V)` = `len(S)`.
- **Ceiling ordering (F1).** `preview_ceiling_bytes < delivery_ceiling_bytes` is validated at config load; a configuration where `preview_ceiling_bytes ≥ delivery_ceiling_bytes` is rejected with the typed `ceiling_order` record (`{code: "ceiling_order", property: "range", reason: "preview_ceiling_bytes must be strictly less than delivery_ceiling_bytes"}`) and startup fails closed before serving.
- **Complete-serialized envelope feasibility.** One envelope budget `C_envelope` = content-block wrapper + `isError` + JSON structural + the preview/page metadata keys (`paging_required`, `handle`, `total_bytes`, `is_error`, `upstream_code`, `retention`, `execution_state`, `page`, `total_pages`, `remaining_pages`, `remaining_bytes`), with bounded widths (`handle` 43, numerics ≤ 20, `retention` ≤ 24, `execution_state` ≤ 7, `upstream_code` ≤ `max_upstream_code_bytes` + marker — the configured value, not a hardcoded 128). `C_envelope` is derived from the configured `max_upstream_code_bytes` (≤512). Worst-case single-byte reserve `max_single_byte_escape_bytes = 6`. Invariants: `C_envelope + 6 < preview_ceiling_bytes`; `C_envelope + 6 < delivery_ceiling_bytes` — config-load validation only (M3); a configuration whose derived envelope leaves no room for the non-empty prefix is rejected at load (AC20).
- **Boundaries:** delivery = 8192, preview = 1536 (defaults); page ≤ pinned ceiling.

### 4.5 Handle lifecycle

- **Generation (C1 + F3).** 32 bytes from `crypto/rand` (mandatory CSPRNG; inspection evidence in the receipt). **Entropy failure:** a `crypto/rand` read failure at handle generation — which occurs after the upstream execution completed — is the `result_entropy_unavailable` retention terminal (no handle, `execution_state: "unknown"`, upstream error class preserved, **zero retained entries** — no entry is committed, so it can never later evict a live handle), never a bare `not_started` `errorResult`.
- **Binding + monotonic non-reuse.** Keyed by resolved owner; `nextEpoch` monotonic across all owners.
- **Owner resolution (F1 + F2).** `call_tool` resolves owner via `directRequestOwner` before dispatch: session-less → publisher; valid epoch → session namespace; non-valid epoch → `ownerOK=false` → per-surface rejection §4.0 (`invalid_epoch` handler-side for `call_tool`; middleware −32600 for the exposure-gated surfaces). Every session-ful result is bound to `(sessionID, epoch)` or rejected — never silently to the publisher namespace.
- **Pinned ceiling (F2).** `pinned_delivery_ceiling_bytes` and `C_envelope` are pinned per entry at retention time; reload does not re-partition or re-derive the envelope.
- **Per-entry cap (typed terminal).** `max_retained_result_bytes = 8388608` bounds S; `len(S) > max_retained_result_bytes` → `result_capacity_exceeded`. No upstream body-cap enforcement (shared-transport preserved).
- **Retirement-versus-retention (C2).** Retention commits **only** when the owner namespace is live at commit, checked under the **same lock that admits the entry** (the store admission mutex, which also serializes eviction and the retirement purge). A racing retirement either wins — the commit observes the retired namespace → `result_namespace_retired`, no retention, no retained-state growth — or loses — the commit observes a live namespace → retention under that live namespace. No retired namespace is resurrected. **Controlled competing-operation test:** `TestRetentionRetirementRace` (T2) pauses an admitted call, retires and purges its owner, then resumes the commit and asserts the typed terminal and zero retained entries (store-side; the seam-observable preview shape is T4/T5, F2).
- **Quantified bounds (L1/F4/F1).** `max_retained_result_bytes = 8388608`; `max_bytes_per_session = 16777216` (hard, validated `≥ max_retained_result_bytes`); `max_entries_per_session = 16`; `tombstone_bytes_per_session = 1048576`; `max_bytes_aggregate = 67108864` (advisory); `max_sessions` (`http.max_sessions`, validated 1–4096); **`preview_ceiling_bytes < delivery_ceiling_bytes` (validated; violation is the typed `ceiling_order` rejection)**. **Maximum aggregate exposure** = `(max_sessions + 1) × (max_bytes_per_session + tombstone_bytes_per_session)` = `(4096 + 1) × (16777216 + 1048576) = 4097 × 17825792 = 73032269824` bytes (≈ 68 GiB) — bounded by the session cap, the per-namespace live bound, and the per-namespace tombstone allowance, not by the advisory aggregate. Config validation enforces `max_bytes_per_session ≥ max_retained_result_bytes`, `max_entries_per_session ≥ 1`, `1 ≤ max_sessions ≤ 4096`, `preview_ceiling_bytes < delivery_ceiling_bytes`, `handle_ttl > 0`, `tombstone_ttl > 0`, `0 < max_upstream_code_bytes ≤ 512`, feasibility invariants, and positive tombstone bounds.
- **Eviction.** FIFO within a namespace; advisory aggregate best-effort evict-own-then-admit. Eviction creates an `evicted` tombstone.
- **Creation serialization.** Creation + admission + eviction + liveness check atomic under one lock.
- **Expiry + tombstones.** `handle_ttl` (lazy + sweep). Tombstones `{reason, tombstoned_at}`; bounds (16 entries / 1 MiB, oldest evicted first) and `tombstone_ttl` (10m). `result_expired`/`result_evicted` while the tombstone lives; `invalid_handle` after overflow/TTL.
- **Sweep lifecycle (H3).** `SweepExpired(now)` from the inbound admission path (throttled, injectable clock).
- **Owner-retirement purge (F1).** The retirement seams call T2's purge seam — it marks the result namespace retired and releases its retained bytes under the admission lock, liveness check and purge serialized, after no further commit can land.
- **Catalog reload.** Handles catalog-independent.
- **Concurrency.** Immutable retained bytes; complete page or typed error.
- **Failure modes.** `invalid_arguments` (a shape-invalid handle/page, pre-store), `invalid_handle` (a well-formed but forged/unknown/cross-namespace handle, post-lookup), `invalid_epoch` (session-epoch rejection), `result_expired`, `result_evicted`, `result_capacity_exceeded`, `result_namespace_retired`, `result_entropy_unavailable` — retrieval failures are tool results with `isError: true` (`errorResult` shape), never JSON-RPC errors. `retrieve_result` never paged.

### 4.6 Invariants that must not change

- Cross-namespace isolation; publisher namespace shared (M3); `(sessionID, epoch)` monotonic non-reuse (L1); session-ful results never silently bound to the publisher (F1); retention only under a live namespace (C2).
- **Inbound epoch-stripping:** the inbound handler strips any client-supplied `X-Mcp-Bridge-Exposure-Epoch` and stamps only the admitted server-issued epoch, so a client cannot forge an incarnation.
- Catalog-revision checks untouched; `catalog_changed` keeps its exact meaning (actual catalog change); the session-epoch rejection uses the distinct `invalid_epoch` (F2); `retrieve_result` catalog-independent.
- Error classification preserved on every branch including the unretainable terminals; `upstream_code` string-only, rune-safe; `errorResult` never paged.
- `additionalProperties:false` schemas; retrieval is its own tool.

### 4.7 Alternatives considered

- **Typed terminal vs upstream body-cap enforcement.** Enforcement would cap the shared client (discovery/SSE); the owner resolved typed terminals.
- **`invalid_epoch` vs reusing `catalog_changed`.** Reusing `catalog_changed` misreported the catalog state; a distinct `invalid_epoch` preserves `catalog_changed`'s exact meaning (F2).
- **Hard vs advisory aggregate.** Hard aggregate re-introduces capacity rejection; the owner resolved advisory.
- **`result_entropy_unavailable` vs a bare `internal_error`.** A bare `internal_error` with `not_started` would falsely claim the upstream never ran; the retention terminal reports `unknown` (F3).
- **CSPRNG inspection vs behavioural oracle.** Behavioural distinctness is necessary-not-sufficient; the owner made `crypto/rand` mandatory via inspection evidence.
<!-- specdd:design:end -->

## 5. ACCEPTANCE CRITERIA

- **AC1 (R1).** Retainable `measureResult > 8192` and ≤ cap: `paging_required:true`, non-empty 43-char handle, `total_bytes == measureResult(full)`, `measureResult(preview) ≤ preview_ceiling_bytes`, non-empty `utf8.Valid` prefix. Oracle: `TestPreviewBoundedAndMetadata` (T4).
- **AC2 (R2).** 8192 byte-identical; 8193 previews. Oracles: `TestCeilingBoundary8192vs8193` (T4) + `TestDiscoveryCeilingBoundary` (T5).
- **AC3 (R3).** Every page ≤ pinned ceiling; metadata obeys §4.2; emits `result_page_served`. Oracle: `TestPageMetadataArithmetic` (T3).
- **AC4 (R4).** Concatenation byte-exact; re-retrieval after ceiling reload identical, including the pinned `C_envelope` across a `max_upstream_code_bytes` reload. Oracle: `TestPageConcatenationByteExact` (T3).
- **AC5 (R5).** Multibyte boundary round-trip; escaping post-escape; escape-dense page fill to pinned ceiling. Oracles: `TestResultMeasure` (T1) + `TestUTF8BoundaryAndEscapingAccounting` (T3) + `TestEscapeDensePageFill` (T3).
- **AC6 (R6).** Description contains `handle`/`page`/`paging`; schema `additionalProperties:false`; README documents operation/shapes. Oracle: `TestRetrieveResultSchema` (T3) + T6 docs checks.
- **AC7 (R7).** A well-formed forged/unknown/cross-namespace/reused-session-ID handle → identical `invalid_handle` (tool result, `isError: true`) + emit `invalid_handle`, with zero retained/page bytes and no existence signal. Oracle: `TestInvalidHandleNoExistenceSignal` (T3).
- **AC8 (R8).** `result_expired`/`result_evicted` for a live tombstone; after `tombstone_ttl` or overflow → `invalid_handle`. Oracles: `TestExpiredAndEvictedDistinct` (T3) + `TestTombstoneOverflow` (T2).
- **AC9 (R9).** Hard per-namespace eviction; advisory aggregate evict-own-then-admit; publisher shared; concurrent complete-or-typed-error; eviction emits `result_evicted_due_to_bound`, overshoot emits `result_aggregate_soft_cap_exceeded`; sweep tombstones; **retirement-versus-retention race: no retired-namespace resurrection, no retained-state growth; after retirement: retained bytes zero**. The retired-namespace marker is store-API-only with a stated lifetime — the race window — and is removed from wire-observable retrieval failures: a live-epoch caller presenting a retired handle gets `invalid_handle` at the wire. AC9 asserts the store-level outcome: retained bytes zero after purge, and commit rejected with the retired terminal only during the race window. Oracles: `TestEvictionIsolationAndConcurrency` (T2) + `TestTTLSweep` (T2) + `TestRetentionRetirementRace` (T2).
- **AC10 (R10).** Oversized upstream `isError` keeps `is_error:true` + `upstream_code` (call_tool from `upstream_result.structuredContent.code`) on preview and pages; `…` marker iff truncated rune-safely; full code byte-exact; bridge failure never paged. Oracles: `TestErrorClassificationPreserved` (T4) + `TestUpstreamCodeForm` (T4).
- **AC11 (R11).** Ceiling identical across four surfaces; emits `result_paged`; discovery seam after `commitDiscovery`. Oracles: `TestCeilingExecutionSurfaces` (T4) + `TestCeilingDiscoverySurfaces` (T5) + `TestDiscoveryCommitBeforeCeiling` (T5).
- **AC12 (R12).** Full suite + coverage gate (W06 ≥90%). Oracle (C4): `go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...` then `go run ./cmd/covcheck` (reads `coverage.out`), run by T7.
- **AC13 (R1).** **Total retention up to the per-entry cap + typed terminals (branch-ownership F2):** every over-ceiling result ≤ cap (all four surfaces, session and publisher) yields a preview + handle (`TestTotalRetentionExecution` T4, `TestTotalRetentionDiscovery` T5); the `result_capacity_exceeded` terminal preview (≤ preview ceiling, non-empty UTF-8-safe prefix, no handle, preserved `is_error` + `upstream_code`) is proven by `TestCapacityTerminal` (T4, call_tool + direct; T5 reuses the shared wrapper); the `result_namespace_retired` terminal preview (≤ preview ceiling) is proven by `TestNamespaceRetiredPreviewShape` (T4, call_tool + direct; T5 reuses the shared wrapper), while T2's `TestRetentionRetirementRace` proves only the store-side liveness (zero retained entries, no resurrection); the `result_entropy_unavailable` terminal (≤ preview ceiling, no handle, `execution_state: "unknown"`, preserved class, non-empty prefix, **zero retained entries** — no entry is committed, so it can never later evict a live handle) is proven by `TestEntropyTerminal` (T4), while T2's `TestEntropyFailure` proves only that a failing entropy source makes handle generation fail with no handle and commits nothing; session-ful non-valid epoch → per-surface rejection, retains nothing (`TestInvalidEpochRejectedBeforeSeam` T4). Oracles: `TestTotalRetentionExecution` (T4) + `TestTotalRetentionDiscovery` (T5) + `TestCapacityTerminal` (T4) + `TestNamespaceRetiredPreviewShape` (T4) + `TestRetentionRetirementRace` (T2) + `TestEntropyTerminal` (T4) + `TestEntropyFailure` (T2) + `TestInvalidEpochRejectedBeforeSeam` (T4).
- **AC14 (R7).** Identical result content + same owner yields pairwise-distinct 43-char base64url handles, independent of content and session id (necessary-not-sufficient); **CSPRNG use (`crypto/rand`) is verified by inspection evidence in the receipt** (C1). Oracle: `TestHandleUnguessability` (T2) + inspection evidence.
- **AC15 (R7).** Two-phase validation with ordering observable: a shape-invalid handle (wrong length/alphabet) or a non-integer/`<1` page returns `invalid_arguments` pre-store (zero store access); a well-formed handle returns `invalid_handle`/`result_expired`/`result_evicted` post-lookup — all tool results with `isError: true`. Oracle: `TestRetrieveHandlerShapeValidation` (T3).
- **AC16 (R10).** Over-ceiling upstream `isError` (within cap) yields a handle; every page preserves `is_error:true` + `upstream_code`; full payload reconstructs. Oracle: `TestRetainedErrorResult` (T4).
- **AC17 (R7).** `retrieve_result` exposure-relevant; session-less retrieves publisher-bound, `invalid_handle` for session-bound; session `invalid_handle` for publisher-bound; session-ful non-valid epoch → admission rejection; retired-epoch → admission rejection. Oracle: `TestRetrieveResultRequiresExposure` (T3).
- **AC18 (R12).** The five Prometheus collectors register/scrape with bounded labels; **the new log events emit to the sink with the required fields (`total_bytes`, `page`, `total_pages`, `max_retained_result_bytes`) and the failure-visibility path falls back correctly** (C3). The two paging counters are bound to committed outcomes with exact before/after deltas read through the process scrape path (C6), owned by the surface that introduces the behavior (F1): **T2** registers both counters with help text (no delta assertion); **T3** owns the retrieval deltas — successful retrieval of one page increments `bridge_result_pages_served_total` by exactly 1 and leaves `bridge_result_paged_total` unchanged (re-retrieving the same handle/page counts again as a served page), and retrieval rejection (`invalid_handle`, expired, evicted, foreign-session) changes neither; **T4** (`call_tool` + direct projected) and **T5** (`search_tools` + `describe_tool`) own the dispatch deltas — dispatch-time paged delivery increments `bridge_result_paged_total` by exactly 1 and leaves `bridge_result_pages_served_total` unchanged, pass-through under the ceiling changes neither, unretainable terminals (`result_capacity_exceeded`, `result_namespace_retired`, `result_entropy_unavailable`) change neither and issue no handle; each counter increments only after its outcome is committed, never before, so a failed commit produces no phantom count. Oracles: `TestObservabilityDeliverables` (T2) + `TestRetrievalCounterDeltas` (T3) + `TestDispatchCounterDeltas` (T4) + `TestDiscoveryDispatchCounterDeltas` (T5).
- **AC19 (R1).** Config validation enforces `preview_ceiling_bytes < delivery_ceiling_bytes`: a configuration with equal ceilings is rejected at load with the typed `ceiling_order` error (startup fails closed before serving); `preview_ceiling_bytes = delivery_ceiling_bytes − 1` is accepted (F1). Oracle: `TestCeilingOrderValidation` (T1).
- **AC20 (R1).** Config validation derives `C_envelope` from the configured `max_upstream_code_bytes` (≤512) and enforces the feasibility invariants: a configuration where `C_envelope + 6 ≥ preview_ceiling_bytes` or `C_envelope + 6 ≥ delivery_ceiling_bytes` is rejected at load with the typed `ceiling_order` record (property `feasibility`) and startup fails closed before serving; the largest configured `max_upstream_code_bytes` that still leaves room for a non-empty prefix is accepted (F1). Oracle: `TestEnvelopeFeasibilityValidation` (T1).

## 6. TASK LEDGER

<!-- specdd:tasks:start -->
Dependencies reference T IDs only. Every task's successful termination requires the listed commands to pass with exit code 0; tasks touching `internal/service`, `internal/catalog`, `internal/observe`, or `cmd/mcp-bridge` also run `go test ./internal/service/ ./internal/observe/ ./cmd/mcp-bridge/ -count=1` (exit 0). Decision-log: `.scratch/token-efficient-tool-responses/decisions.md`; receipts in `.omp/artifacts/receipt_<task_id>.json`.

### T1 — Byte-accounting oracle and ceiling constants
- **Objective:** `measureResult` + `delivery_ceiling_bytes=8192`/`preview_ceiling_bytes=1536`/`max_upstream_code_bytes` (default 128, configurable ≤512) + the ceiling-order and envelope-feasibility invariants.
- **Deps:** none. **Inputs:** §4.4; `internal/service/tools.go:865-908`; `internal/config/bounds.go`.
- **Expected files:** `internal/service/result_measure.go` (new), `internal/config/` keys + validation (incl. `ceiling_order`), `internal/service/result_measure_test.go` (new).
- **Allowed scope:** accounting, config keys, feasibility + ceiling-order predicates, tests. **Prohibited:** handler wiring.
- **RED:** `measureResult` test fails to compile.
- **Deliverable:** `measureResult` via SDK `MarshalJSON`; `max_total_bytes_digits=20`, `max_single_byte_escape_bytes=6`; validated defaults incl. `preview_ceiling_bytes < delivery_ceiling_bytes`; escaping/duplicate/boundary/feasibility (envelope derived from the configured `max_upstream_code_bytes`)/ceiling-order tests.
- **AC IDs:** AC5 (escaping half), AC19, AC20. **Validation:** `go test ./internal/service/ -run 'TestResultMeasure' -count=1`; `go test ./internal/config/ -run 'TestCeilingOrderValidation' -count=1`; `go test ./internal/config/ -run 'TestEnvelopeFeasibilityValidation' -count=1`; `go test ./internal/config/ -count=1`; existing-suite regression.
- **Failed/blocked:** accounting mismatch vs SDK, or a config where `preview_ceiling_bytes ≥ delivery_ceiling_bytes` loads, is BLOCKED.

### T2 — Retained-result handle store, envelope vocabulary, and metric registration
- **Objective:** the store (serialized-form retention up to the per-entry cap, owner/publisher namespaces, hard per-namespace + advisory aggregate, FIFO, tombstones + overflow, TTL sweep-from-inbound, atomic admission + retirement-versus-retention liveness check, pinned ceiling, monotonic non-reuse, three-seam purge, concurrency-safe reads, inducible entropy failure, CSPRNG via `crypto/rand`), config `retained_results`, the envelope field vocabulary, the logging-contract vocabulary documentation, and the five collectors.
- **Deps:** T1. **Inputs:** §4.5; `internal/service/projection.go:190-202`; `internal/service/cursor.go`; `internal/service/inbound.go:800-816,1120-1145`; `internal/config/bounds.go`; `internal/observe/metrics.go:35-52`; `internal/observe/envelope.go:252-488`; `cmd/mcp-bridge/serve.go:164`.
- **Expected files:** `internal/service/result_handle.go` (new, `SweepExpired(now)` + tombstones + injectable entropy/clock + liveness-checked commit), `internal/service/result_handle_test.go` (new), `internal/config/` `retained_results` keys + validation, `internal/observe/metrics.go` (five collectors + helpers), `internal/observe/envelope.go` (add `total_bytes`, `page`, `total_pages`, `max_retained_result_bytes` to the field vocabulary), `internal/observe/log_test.go` (update the exact-union fixture), `cmd/mcp-bridge/logging_contract_test.go` (update the registry-union assertion), `docs/logging-contract.md` (document the new fields/events), `internal/service/inbound.go` (sweep trigger + purge seam called by the retirement hooks), `cmd/mcp-bridge/serve.go` (add the `Metrics` field to the `ToolSurfaceConfig` literal), `internal/service` (add the `Metrics` field to `ToolSurfaceConfig`).
- **Allowed scope:** store/tests; config keys; collectors + injection; envelope field vocabulary + exact-union fixtures + logging-contract doc; sweep trigger; purge seam; injectable entropy/clock seams. **Prohibited:** handler wiring; no goroutine/`Close()`; no upstream body-cap enforcement.
- **RED:** cross-namespace rejection + FIFO eviction test fails to compile.
- **Deliverable:** store with serialized-form retention (up to cap), owner/publisher namespaces, hard per-namespace + advisory aggregate, tombstoned eviction/expiry (+ overflow), owned sweep, atomic admission + liveness-checked commit (no retired-namespace resurrection), pinned ceiling, monotonic non-reuse, three-seam purge, concurrency-safe reads; `crypto/rand` CSPRNG with inspection evidence; five collectors injected (the two paging counters registered with help text stating exactly what they count); envelope field vocabulary extended; bounds validated at load; a failing entropy source makes handle generation fail with no handle (store-side). **Field-union invariant (C3):** the emitter, the complete field registry (`internal/observe/envelope.go`), every exact-union fixture (`internal/observe/log_test.go`, `cmd/mcp-bridge/logging_contract_test.go`), and the documented vocabulary (`docs/logging-contract.md`) change together in T2, so no gate can pass on a registry/vocabulary mismatch.
- **AC IDs:** AC8, AC9, AC13 (entropy store-side half + namespace-retired store-side half), AC14, AC18 (registration + help text half). **Validation:** `go test ./internal/service/ -run 'TestResultHandle|TestEvictionIsolationAndConcurrency|TestTTLSweep|TestTombstoneOverflow|TestRetentionRetirementRace|TestHandleUnguessability|TestEntropyFailure|TestObservabilityDeliverables' -count=1`; `go test ./internal/observe/ -count=1`; `go test ./internal/config/ -count=1`; existing-suite regression `go test ./internal/service/ ./internal/observe/ ./cmd/mcp-bridge/ -count=1`.
- **Failed/blocked:** eviction-isolation violation, partial page, missing retirement seam, unbounded label, a sweep with no observable, a liveness-check omission (retained under a dead namespace), a registry/vocabulary mismatch (exact-union fixture or `logging_contract_test.go` failing), a missing paging-counter help text, or a config where `max_bytes_per_session < max_retained_result_bytes` loads is BLOCKED.

### T3 — `retrieve_result` retrieval surface
- **Objective:** the fourth built-in tool (schema, description, two-phase validation, exposure registration, page/typed-error responses incl. tombstoned expired/evicted); update every §3 assertion.
- **Deps:** T2. **Inputs:** §4.3, §4.2; `internal/catalog/schema.go:74-89`; `internal/service/tools.go:44-175,1074-1081`; `internal/service/session_exposure.go:60-83`.
- **Expected files:** `internal/catalog/schema.go`, `internal/service/tools.go`, `internal/service/session_exposure.go`, `internal/service/retrieve_result_test.go` (new), and the complete §3 inventory.
- **Allowed scope:** new tool, schema, description, two-phase validation, `requiresExposure`, tests, §3 updates. **Prohibited:** changing existing tool schemas/envelopes.
- **RED:** wire test fails with "unknown tool".
- **Deliverable:** working `retrieve_result` (two-phase validation, exposure registration, description tokens, page metadata, byte slices, typed codes); the retrieval counter-delta assertions (`TestRetrievalCounterDeltas`, the AC18 retrieval-delta rows) pass; §3 green.
- **AC IDs:** AC3, AC4, AC5 (UTF-8 + escape-dense half), AC6 (schema+description half), AC7, AC8, AC15, AC17, AC18 (retrieval-delta half). **Validation:** `go test ./internal/service/ -run 'TestRetrieveResult|TestPageMetadataArithmetic|TestPageConcatenationByteExact|TestUTF8BoundaryAndEscapingAccounting|TestEscapeDensePageFill|TestRetrieveResultSchema|TestInvalidHandleNoExistenceSignal|TestExpiredAndEvictedDistinct|TestRetrieveHandlerShapeValidation|TestRetrieveResultRequiresExposure|TestRetrievalCounterDeltas' -count=1`; `go test ./internal/catalog/ -count=1`; existing-suite regression.
- **Failed/blocked:** schema mismatch, empty/partial page, store access with malformed handle, range-before-lookup, missing `requiresExposure`, missing description tokens, a retrieval counter delta that disagrees with the AC18 retrieval rows, or a stale §3 assertion is BLOCKED.

### T4 — Ceiling enforcement on execution dispatch paths
- **Objective:** the seam on `call_tool` + direct projected tools, error classification, `call_tool` owner resolution (per-surface matrix with `invalid_epoch`), the capacity/namespace-retired/entropy terminal preview shapes, and the dispatch counter deltas.
- **Deps:** T1, T2, T3. **Inputs:** §4.2, §4.0; `internal/service/tools.go:865-908,315-324`; `internal/service/direct_execution.go:29-63`; `internal/service/tools.go:1037-1048`.
- **Expected files:** `internal/service/bounded_delivery.go` (new), `internal/service/tools.go`, `internal/service/direct_execution.go`, `internal/service/bounded_delivery_test.go` (new).
- **Allowed scope:** the two dispatch return paths, shared wrapper, `call_tool` owner resolution (per-surface matrix), error-result metadata, the capacity/namespace-retired/entropy terminal preview shapes, the dispatch counter increments. **Prohibited:** discovery/describe handlers, catalog, session admission.
- **RED:** an 8193-byte direct result fails to preview+handle.
- **Deliverable:** over-ceiling preview+handle (up to cap) or the capacity/namespace-retired/entropy terminal on both execution paths; handler-level `invalid_epoch` for `call_tool` and middleware JSON-RPC −32600 for the directly invoked projected tool; under-ceiling byte-identical; `is_error`/`upstream_code` preserved; bridge failures small; the terminal preview shapes (`TestCapacityTerminal`, `TestNamespaceRetiredPreviewShape`, `TestEntropyTerminal`) and the dispatch counter deltas (`TestDispatchCounterDeltas`) pass.
- **AC IDs:** AC1, AC2 (execution boundary), AC10, AC11 (`TestCeilingExecutionSurfaces`), AC13 (`TestTotalRetentionExecution` + `TestCapacityTerminal` + `TestNamespaceRetiredPreviewShape` + `TestEntropyTerminal` + `TestInvalidEpochRejectedBeforeSeam` + `TestInvalidEpochMiddlewareRejection`), AC16, AC18 (dispatch-execution-delta half). **Validation:** `go test ./internal/service/ -run 'TestBoundedDelivery|TestPreviewBoundedAndMetadata|TestCeilingBoundary8192vs8193|TestErrorClassificationPreserved|TestUpstreamCodeForm|TestCeilingExecutionSurfaces|TestTotalRetentionExecution|TestCapacityTerminal|TestNamespaceRetiredPreviewShape|TestEntropyTerminal|TestInvalidEpochRejectedBeforeSeam|TestInvalidEpochMiddlewareRejection|TestRetainedErrorResult|TestDispatchCounterDeltas' -count=1`; existing-suite regression.
- **Failed/blocked:** full over-ceiling body, dropped `isError`/`upstream_code`, non-string `code` surfaced, over-cap retaining a handle, a handle-less preview for a within-cap result, a terminal preview that omits `execution_state: "unknown"`, a dispatch counter delta that disagrees with the AC18 dispatch rows, or a session-ful `call_tool` with a non-valid epoch retaining a handle is BLOCKED.

### T5 — Ceiling enforcement on discovery/describe
- **Objective:** the seam on `search_tools` + `describe_tool`, after `commitDiscovery`, plus the dispatch counter deltas.
- **Deps:** T1, T2, T3, T4. **Inputs:** §4.2; `internal/service/tools.go:457-471,624-655`.
- **Expected files:** `internal/service/tools.go`, `internal/service/discovery_bounded_test.go` (new).
- **Allowed scope:** the two discovery/describe return paths, the dispatch counter increments. **Prohibited:** catalog-revision/exposure-admission changes; seam before `commitDiscovery`.
- **RED:** an oversized `describe_tool` response fails to page.
- **Deliverable:** oversized discovery/describe preview+handle (up to cap) or the capacity/namespace-retired/entropy terminal (via the shared wrapper), after `commitDiscovery`; discovery commit preserved; the discovery dispatch counter deltas (`TestDiscoveryDispatchCounterDeltas`, the AC18 dispatch rows) pass.
- **AC IDs:** AC2 (discovery boundary), AC11 (`TestCeilingDiscoverySurfaces` + `TestDiscoveryCommitBeforeCeiling`), AC13 (`TestTotalRetentionDiscovery`), AC18 (dispatch-discovery-delta half). **Validation:** `go test ./internal/service/ -run 'TestDiscoveryBounded|TestDiscoveryCeilingBoundary|TestCeilingDiscoverySurfaces|TestDiscoveryCommitBeforeCeiling|TestTotalRetentionDiscovery|TestDiscoveryDispatchCounterDeltas' -count=1`; existing-suite regression.
- **Failed/blocked:** a discovery path bypassing the ceiling, a seam before `commitDiscovery`, or a discovery dispatch counter delta that disagrees with the AC18 dispatch rows is BLOCKED.

### T6 — Documentation and changelog (docs-only)
- **Objective:** `README.md`, changelog. Docs-only.
- **Deps:** T3, T4, T5. **Inputs:** `README.md:279,287,322-325,332,443,467,468`; `CHANGELOG.md`.
- **Expected files:** `README.md`, `CHANGELOG.md`.
- **Allowed scope:** docs only. **Prohibited:** production/test code.
- **RED:** `grep -q 'retrieve_result' README.md` exits non-zero.
- **Deliverable:** README documents the retrieval surface, preview/page shapes (incl. `result_capacity_exceeded`/`result_namespace_retired`/`result_entropy_unavailable`), typed codes (incl. `invalid_epoch` and its meaning "the caller's session epoch is not valid"), byte-accounting, total retention up to the cap, and the invalid-epoch contract — `call_tool` handler `invalid_epoch`; `search_tools`/`describe_tool`/directly invoked projected tools middleware JSON-RPC −32600 `"invalid exposure request"` (no tool result); the `search_tools`/`describe_tool` `availability` sentences must say full response carries `availability`, paged response only paging metadata in `structuredContent`; every changed completion claim qualified at-or-below-ceiling; changelog.
- **AC IDs:** AC6 (README half). **Validation (docs-only, F6):** positive `grep -q 'retrieve_result' README.md`, `grep -q 'paging_required' README.md`, `grep -q 'result_capacity_exceeded' README.md`, `grep -q 'invalid_epoch' README.md`, `grep -q 'invalid exposure request' README.md`; per-claim qualified-sentence positives `grep -q 'complete upstream result at or below the delivery ceiling' README.md`, `grep -q 'unaltered inside a provenance envelope at or below the delivery ceiling' README.md`, `grep -q 'is the complete original `CallToolResult` at or below the delivery ceiling' README.md`, `grep -q 'includes the revision at the response root at or below the delivery ceiling' README.md`; per-claim negative (single-line eliminated falsehoods only) `! grep -q 'One MCP server, three tools' README.md`, `! grep -q 'All three declare' README.md`, `! grep -q 'exactly the three bridge built-ins' README.md`, `! grep -q 'keeps per-tool `availability`\|retains `availability`' README.md`; `git diff --check`. No `go test`.
- **Failed/blocked:** docs contradicting the implemented contract is BLOCKED.

### T7 — Integration and coverage gate
- **Objective:** full instrumented suite + repository coverage gate.
- **Deps:** T2, T3, T4, T5. **Inputs:** §5 AC12; the coverage gate is in `.github/workflows/verify.yml`; `cmd/covcheck/main.go:22-42`; W06 ≥90%.
- **Expected files:** `coverage.out` (disposable; removed after capture).
- **Allowed scope:** run `go test` + coverage gate; write/remove `coverage.out`. **Prohibited:** source edits.
- **RED:** `go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...` non-zero or `go run ./cmd/covcheck` <90%.
- **Deliverable:** full pass + `covcheck` ≥90%; `coverage.out` removed after capture.
- **AC IDs:** AC12. **Validation (C4):** `go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...` (exit 0) then `go run ./cmd/covcheck` (reads `coverage.out`, exit 0); then `rm coverage.out`.
- **Failed/blocked:** failing suite or sub-90% `covcheck` is BLOCKED.

**Effects and exclusions (whole ticket).**
- **Write roots:** `internal/service/result_measure.go`, `result_handle.go`, `bounded_delivery.go`, `result_measure_test.go`, `result_handle_test.go`, `retrieve_result_test.go`, `bounded_delivery_test.go`, `discovery_bounded_test.go`; edits to `internal/service/tools.go`, `direct_execution.go`, `session_exposure.go` (admission switch + `requiresExposure` case only), `inbound.go` (sweep trigger + purge seam only, called by the retirement hooks), the five `internal/service` and six `cmd/mcp-bridge` built-in-surface tests (§3), `internal/observe/metrics.go`, `internal/observe/envelope.go` (field vocabulary only), `internal/observe/log_test.go` (exact-union fixture), `cmd/mcp-bridge/logging_contract_test.go` (registry-union assertion), `internal/catalog/schema.go`, `internal/config/`, `cmd/mcp-bridge/serve.go` (`ToolSurfaceConfig.Metrics` literal only); `README.md`, `docs/logging-contract.md`, `CHANGELOG.md`; disposable `coverage.out` (T7). **No `internal/upstream/client.go`, `cmd/mcp-bridge/wiring.go`, or `internal/upstream/runtime.go` edits.**
- **Explicit non-goals:** no default field projections (ticket 05); no `full` knob; no duplicate-result reuse/dedup (ticket 06); no ranking/filtering changes; no upstream capability removal; no change to the shared transport/discovery/SSE; no change to catalog-revision or exposure-admission semantics; no recursive paging.
- **README claims (H3 + R8 + F8 + M2 + F6, complete):** **change** — `:279` "three tools" → four; `:287` "All three declare" → four (add `retrieve_result` row); `:322-325` "complete upstream result" → qualified at-or-below-ceiling and names `retrieve_result` as the carrier (preview carries only paging metadata); `:332` "includes the revision at the response root" → qualified; `:443` "exactly the three bridge built-ins" → four; `:467` "unaltered inside a provenance envelope" → qualified; `:468` "is the complete original `CallToolResult`" → qualified; plus the new `invalid_epoch` and `result_entropy_unavailable` codes. **Remain true** (qualified): catalog-revision semantics, tool-name derivation, error taxonomy codes, session/epoch ownership rules.
<!-- specdd:tasks:end -->

## 7. ENVELOPE

<!-- specdd:envelope:start -->
- **Write roots (tasks may touch only these):** new files `internal/service/result_measure.go`, `result_handle.go`, `bounded_delivery.go`, `result_measure_test.go`, `result_handle_test.go`, `retrieve_result_test.go`, `bounded_delivery_test.go`, `discovery_bounded_test.go`; edits to `internal/service/tools.go`, `direct_execution.go`, `session_exposure.go` (admission switch + `requiresExposure` case only), `inbound.go` (sweep trigger + purge seam only, called by the retirement hooks), `internal/service/tools_test.go`, `progressive_session_test.go`, `progressive_concurrency_lifecycle_test.go`, `tool_metadata_test.go`, `tools_publish_atomic_test.go`, `cmd/mcp-bridge/serve_hybrid_exposure_test.go`, `serve_hybrid_reload_test.go`, `serve_hybrid_progressive_reveal_test.go`, `serve_hybrid_progressive_reveal_lifecycle_test.go`, `serve_oauth_test.go`, `serve_test.go`, `cmd/mcp-bridge/serve.go` (`ToolSurfaceConfig.Metrics` literal only), `internal/observe/metrics.go`, `internal/observe/envelope.go` (field vocabulary only), `internal/observe/log_test.go` (exact-union fixture), `cmd/mcp-bridge/logging_contract_test.go` (registry-union assertion), `internal/catalog/schema.go`, `internal/config/`; `README.md`, `docs/logging-contract.md`, `CHANGELOG.md`; disposable `coverage.out` (T7). **Explicitly excluded:** `internal/upstream/client.go`, `cmd/mcp-bridge/wiring.go`, `internal/upstream/runtime.go`.
- **Read paths tasks need:** `internal/service/tools.go`, `internal/service/direct_execution.go`, `internal/service/session_exposure.go`, `internal/service/projection.go`, `internal/service/cursor.go`, `internal/service/inbound.go`, `internal/service/reload.go`, `internal/config/command.go` + `internal/config/bounds.go` + `internal/config/config.go`, `internal/catalog/schema.go`, `internal/core/core.go`, `internal/observe/metrics.go`, `internal/observe/envelope.go`, `internal/observe/log.go`, `cmd/mcp-bridge/serve.go`, `cmd/covcheck/main.go`, `README.md`, `docs/logging-contract.md`, `CHANGELOG.md`, `.github/workflows/verify.yml`, the ticket/map/sibling files under `.scratch/token-efficient-tool-responses/`, `.omp/artifacts/token-efficiency-ticket01-refinement.md`, `.scratch/token-efficient-tool-responses/decisions.md`.
- **Allowed effect kinds:** read (source/docs/config/evidence), write (only the write roots above), test (run `go test` / `go build` / `go run ./cmd/covcheck`).
- **Forbidden effect kinds:** network, publish, deploy, schedule (no cron), credentials/secrets handling, infrastructure changes, and any write outside the write roots.
- **Allowed task ids:** T1, T2, T3, T4, T5, T6, T7.
- **Explicit non-goals:** no default field projections (ticket 05); no `full` knob; no duplicate-result reuse/deduplication (ticket 06); no ranking/filtering changes; no upstream capability removal; no change to the shared transport/discovery/SSE (no body-cap enforcement); no change to catalog-revision or exposure-admission semantics; no recursive paging.
<!-- specdd:envelope:end -->

## 8. OBSERVABILITY

Applied per the observability-contract skill. Existing collection reused. **Every deliverable is falsifiable by an AC with an owning task.**

### Logging
- **Shipping path:** existing `observe.Logger` structured JSON → sink (unchanged).
- **Schema:** existing `observe.Entry` envelope; **the new field vocabulary (`total_bytes`, `page`, `total_pages`, `max_retained_result_bytes`) is added to `internal/observe/envelope.go` (T2)**, and the exact-union fixtures (`internal/observe/log_test.go`, `cmd/mcp-bridge/logging_contract_test.go`) and `docs/logging-contract.md` change together in T2 (C3).
- **Severity:** `info` for `result_paged`/`result_page_served`; `warn` for `result_expired`/`result_evicted`/`invalid_handle`, `result_capacity_exceeded`, `result_namespace_retired`, `result_entropy_unavailable`, and store-side `result_evicted_due_to_bound`/`result_aggregate_soft_cap_exceeded`; never log payload content or handle values.
- **Events, fields, and verifying ACs:** `result_paged` (`total_bytes`) — **AC11 (T4/T5)**; `result_page_served` (`page`, `total_pages`) — **AC3 (T3)**; `result_expired`/`result_evicted` — **AC8 (T3)**; `invalid_handle` — **AC7 (T3)**; `result_capacity_exceeded` (`total_bytes`, `max_retained_result_bytes`) — **AC13 (T4)**; `result_namespace_retired` (`total_bytes`) — **AC13 (T2/T4)**; `result_entropy_unavailable` (`total_bytes`) — **AC13 (T4)**; `result_evicted_due_to_bound` — **AC9 (T2)**; `result_aggregate_soft_cap_exceeded` — **AC9 (T2)**.

### Prometheus
- **Scrape path:** process `observe.Metrics` registry (`NewMetrics`) served by `Metrics.Handler()`; five collectors registered there (T2/AC18).
- **Metrics (bounded labels, verifying AC):** `bridge_result_paged_total` `{operation}` — help text: counts dispatch-time paged deliveries, incremented by exactly 1 only after a preview+handle is committed; `bridge_result_pages_served_total` `{outcome}` — help text: counts retrieval pages served, incremented by exactly 1 only after a page is committed; `bridge_result_retained_bytes` (Gauge); `bridge_result_retained_entries` (Gauge; live + tombstones); `bridge_result_tombstone_entries` (Gauge) — all **AC18 (T2)**; deltas asserted by **AC18 (T3/T4/T5)**; sweep/eviction gauges asserted by **AC9 (T2)**. `{operation}` is the finite dispatch-surface enumeration {`call_tool`, `direct`, `search_tools`, `describe_tool`}; `{outcome}` is the finite retrieval enumeration {`success`, `failure`}. Any value outside these enumerations is refused at record time and fails the test — no free-form strings.
- **Bounded labels:** finite vocabularies; never session ID, handle, tool name.
- **Latency histogram:** not warranted (advisory).
- **Collector injection (M2/F5).** Five collectors on the process registry built by `NewMetrics`; **T2 adds the `Metrics` field to `ToolSurfaceConfig` and wires it at `cmd/mcp-bridge/serve.go:164`**; the surface does not build a private registry.

### Operations and verification
- **Dashboards/alerts (advisory only):** retained gauges informational; the aggregate cap is advisory (no "approaching the cap" alert).
- **Verification tests (C3/C6):** AC18 `TestObservabilityDeliverables` (T2) asserts the five collectors register/scrape with bounded labels AND that every event in the Events list above **emit to a real sink** with the required fields and that the failure-visibility path (an out-of-vocabulary field) is rejected/fallback, not silently dropped; AC18 `TestRetrievalCounterDeltas` (T3) and `TestDispatchCounterDeltas` (T4)/`TestDiscoveryDispatchCounterDeltas` (T5) assert the exact before/after paging-counter deltas through the process scrape path; the exact-union fixtures (`internal/observe/log_test.go`, `cmd/mcp-bridge/logging_contract_test.go`) assert the registry and `docs/logging-contract.md` match; AC3/AC7/AC8/AC9/AC11/AC13 assert their surface events.

**Implementing T IDs:** T1–T5 (metric registration + envelope vocabulary + logging-contract doc T2; counter deltas T3/T4/T5); verification AC IDs as listed.

## 9. RELEASE AND RECOVERY

- **Deployment/migration order:** no data migration; process-local store empty at start. Deploy order: config then binary. Rolling restart drops handles/tombstones; stale handles return `invalid_handle`.
- **Rollback:** revert restores byte-identical unbounded delivery; no persisted state.
- **Irreversible steps:** none.
- **Config validation (L1/F4/F1):** `max_bytes_per_session ≥ max_retained_result_bytes` (16777216 ≥ 8388608), `max_entries_per_session ≥ 1`, `1 ≤ max_sessions ≤ 4096`, **`preview_ceiling_bytes < delivery_ceiling_bytes` (violation → typed `ceiling_order` rejection, startup fails closed before serving)**, `handle_ttl > 0`, `tombstone_ttl > 0`, `0 < max_upstream_code_bytes ≤ 512`, or a feasibility-invariant violation all fail at config load. The aggregate cap is advisory and not validated as a hard bound.

## 10. CRITIC HANDOFF

### Highest-risk assumptions/decisions to review
1. The byte-accounting oracle (§4.4) must match the SDK `MarshalJSON` path (K1).
2. The per-entry cap (§4.5) `max_retained_result_bytes = 8388608` on the serialized form.
3. The retirement-versus-retention liveness check under the admission lock (§4.5, C2) and the retirement purge seam (r27-F1).
4. The per-surface rejection matrix (§4.0, C5/F2) with `invalid_epoch` and the terminal-preview branch ownership (F2).
5. The entropy-failure truthfulness (`execution_state: "unknown"`, F3), the ceiling-order invariant (F1), and the envelope-feasibility derivation from the configured `max_upstream_code_bytes` (r25-F1).

### Missing coverage to target
- The `result_capacity_exceeded`, `result_namespace_retired`, and `result_entropy_unavailable` terminals.
- The invalid-epoch `call_tool` rejection (per-surface matrix) and its README documentation.
- Publisher-namespace retention + cross-namespace isolation round-trip.
- Live HTTP wire test at the 8192/8193 boundary.
- `upstream_code` over `max_upstream_code_bytes` (rune-safe marker) and non-string `code`.
- Tombstone overflow/TTL → `invalid_handle`; reused session ID; retirement-retention race; the `ceiling_order` boundary; the envelope-feasibility boundary (`C_envelope + 6 ≥ preview_ceiling_bytes`).

### W09E shared review-risk matrix

| Risk (surface) | Class | Affected requirement | Verification evidence | Counterexample considered |
| --- | --- | --- | --- | --- |
| Handle create→read→expire→evict→tombstone→retire→commit-race across bootstrap, ordinary, failure, restart, reload | Lifecycle | R4, R7, R8, R9 | `TestExpiredAndEvictedDistinct` + `TestEvictionIsolationAndConcurrency` + `TestTTLSweep` + `TestTombstoneOverflow` + `TestRetentionRetirementRace` + purge/reload tests | A racing retirement must not resurrect a dead namespace or grow retained state |
| Store writers: creating handler, eviction, tombstone sweep, owner-retirement purge, liveness-checked commit | Shared state | R9 | `TestEvictionIsolationAndConcurrency` + `TestTTLSweep` + `TestTombstoneOverflow` + `TestRetentionRetirementRace` | Soft aggregate evicts only the creating namespace's own handles |
| Owner resolution (per-surface matrix + `invalid_epoch`) + monotonic non-reuse + four terminals | Contract | R1, R7, R11 | `TestTotalRetentionExecution`/`Discovery` + `TestInvalidEpochRejectedBeforeSeam` + `TestInvalidEpochMiddlewareRejection` + `TestCapacityTerminal` + `TestNamespaceRetiredPreviewShape` + `TestEntropyTerminal` + `TestRetentionRetirementRace` + `TestRetrieveResultRequiresExposure` | Per-surface code+state preserved; `catalog_changed` never misreports an epoch fault |
| Error class + `upstream_code` string-only + rune-safe truncation | Contract | R10 | `TestErrorClassificationPreserved` + `TestUpstreamCodeForm` + `TestRetainedErrorResult` | Non-string `code` omitted; multibyte truncation rune-safe |
| Preview/page shapes; two-phase validation; schema; README | Contract | R1–R8, R10, R11 | `TestPreviewBoundedAndMetadata`, `TestPageMetadataArithmetic`, `TestRetrieveHandlerShapeValidation`, `TestRetrieveResultSchema`, README checks | Preview non-empty UTF-8 prefix; malformed handle never reaches store |
| Discovery seam placement (after `commitDiscovery`) | Contract | R11 | `TestDiscoveryCommitBeforeCeiling` (T5) | A seam before `commitDiscovery` drops exposure |
| Built-in-surface assertion inventory | Contract/ledger | R11, R12 | T3 updates all 11 §3 files; T7 green | Any omitted assertion fails `cmd/mcp-bridge` |
| Ceiling-order validation (preview < delivery) | Contract | R1 | `TestCeilingOrderValidation` (T1) | A preview ceiling ≥ delivery ceiling would force a paged preview |
| Envelope-feasibility derivation (configured `max_upstream_code_bytes`) | Contract | R1 | `TestEnvelopeFeasibilityValidation` (T1) | A hardcoded 128 bound would let a legal 512 config leave no room for the prefix |
| Authority/scope; no `full`; no shared-transport change; CSPRNG mandatory | Authority | R6, R7, R9, R11 | §7 effects; ticket 05 boundary; `crypto/rand` inspection evidence | No `full` field; no `internal/upstream` edits; CSPRNG verified |
| Observability emission→collection→delivery; envelope vocabulary + fixtures + doc co-owned by T2; five collectors; counter deltas split T2/T3/T4/T5; coverage artifact | Observability | R12 | AC18 (`TestObservabilityDeliverables` + `TestRetrievalCounterDeltas` + `TestDispatchCounterDeltas` + `TestDiscoveryDispatchCounterDeltas`) + exact-union fixtures + AC3/AC7/AC8/AC9/AC11/AC13; T7 `coverage.out` | Registry/vocabulary mismatch fails the fixture; counters bound to committed outcomes; `covcheck` reads the fresh profile |
| Root invariant = byte-exact reconstruction + no-existence-signal rejection | Repair closure | R4, R5, R7, R10 | `TestPageConcatenationByteExact`, `TestUTF8BoundaryAndEscapingAccounting`, `TestEscapeDensePageFill`, `TestErrorClassificationPreserved`, `TestInvalidHandleNoExistenceSignal` | Error/UTF-8/escape-dense/publisher fixtures |

### Findings resolution (Critic round 1, C1–C6 + CodeReviewer r23 F1–F6 + CodeReviewer r24 F1–F3 + CodeReviewer r25 F1–F6 + carried LOW-1/LOW-2 + CodeReviewer r26 F1–F6 + CodeReviewer r27 F1–F5)

| ID | Class | Root fix | Section |
| --- | --- | --- | --- |
| C1 | ac-not-falsifiable | R7 CSPRNG mandatory; uniqueness necessary-not-sufficient; entropy typed | §2 R7, §4.5, §5 AC14 |
| C2 | lifecycle-hazard | retirement-versus-retention invariant; `result_namespace_retired`; `TestRetentionRetirementRace` | §2 R9, §4.5, §5 AC9/AC13, §6 T2 |
| C3 | ledger-state-drift | envelope.go + log_test.go + logging_contract_test.go + logging-contract.md co-owned by T2; AC18 real emission | §3, §7, §6 T2, §5 AC18, §8 |
| C4 | ledger-state-drift | one artifact `coverage.out`; exact command + cleanup | §5 AC12, §6 T7, §9 |
| C5 | statement-contradiction | per-surface rejection matrix (code + state + zero retention) | §4.0, §2 R1, §5 AC13, §6 T4 |
| C6 | ac-not-falsifiable | counters bound to committed outcomes with exact deltas | §5 AC18, §6 T2/T3/T4/T5, §8 |
| F1 | r-ac-coverage-gap | delta ownership split (T2 registration, T3 retrieval, T4/T5 dispatch) | §5 AC18, §6 T2/T3/T4/T5 |
| F2 | statement-contradiction | `invalid_epoch` replaces `catalog_changed` for session-epoch rejection; documented | §4.0, §2 R1, §4.5, §4.6, §6 T6 |
| F3 | ac-not-falsifiable | AC13 asserts each terminal preview ≤ ceiling, non-empty UTF-8 prefix, no handle, preserved class | §5 AC13 |
| F4 | statement-contradiction | aggregate exposure formula incl. tombstone allowance; `max_sessions` validated | §4.5, §9, §11 OD2 |
| F5 | ledger-state-drift | `Metrics` field described as T2 work | §3, §8, AUTO-DECISION 14 |
| F6 | ac-not-falsifiable | README anchors corrected; negative greps reduced to single-line eliminated falsehoods | §3, §6 T6 |
| r24-F1 | statement-contradiction | ceiling-order invariant (`preview < delivery`) + AC19 + `TestCeilingOrderValidation` | §4.4, §4.5, §9, §5 AC19, §6 T1 |
| r24-F2 | r-ac-coverage-gap | AC13 terminal-preview branch ownership (T2 store-side only; T4/T5 seam shapes) | §5 AC13, §6 T2/T4/T5 |
| r24-F3 | statement-contradiction | entropy failure → `result_entropy_unavailable` with `execution_state: "unknown"` and preserved class | §2 R1/R7, §4.2, §4.5, §5 AC13, §6 T4 |
| r25-F1 | statement-contradiction | envelope budget derives from configured `max_upstream_code_bytes` (≤512), not hardcoded 128; AC20 + `TestEnvelopeFeasibilityValidation` | §4.4, §4.2, §5 AC20, §6 T1, AUTO-DECISION 8 |
| r25-F2 | ac-not-falsifiable | T6 per-claim qualified-sentence greps (complete upstream result / unaltered / complete original `CallToolResult` / includes the revision) | §6 T6 |
| r25-F3 | statement-contradiction | R2 scoped to owner-resolves; `ownerOK=false` rejection named (`invalid_epoch`/`unknown_tool`) | §2 R2 |
| r25-F4 | lifecycle-hazard | `result_entropy_unavailable` leaves zero retained entries (no entry committed, so it can never later evict a live handle) | §4.5, §5 AC13 |
| r25-F5 | r-ac-coverage-gap | AC20 own oracle `TestEnvelopeFeasibilityValidation`; T1 AC list AC5,AC19,AC20 + validation command | §5 AC20, §6 T1 |
| r25-F6 | statement-contradiction | handle-validation boundary: shape-invalid → `invalid_arguments` (pre-store); well-formed forged → `invalid_handle` (post-lookup); AC7/AC15 consistent | §4.3, §4.5, §5 AC7/AC15 |
| r25-LOW-1 | ledger-state-drift | README anchors corrected to `:287`/`:332`/`:468` | §3, §6 T6 |
| r25-LOW-2 | statement-contradiction | inbound strips client-supplied epoch and stamps server-issued epoch (incarnation forgery impossible) | §3, §4.6 |
| r26-F1 | statement-contradiction | invalid-epoch per-surface layer | §4.0, §2 R1/R2 |
| r26-F2 | ac-not-falsifiable | terminal previews ≤ preview ceiling | §4.4, §5 AC13 |
| r26-F3 | statement-contradiction | envelope pinned at retention | §2 R4, §4.5, §5 AC4 |
| r26-F4 | ac-not-falsifiable | real-sink emission for all events | §8 |
| r26-F5 | ledger-state-drift | purge-hook ownership named | §4.5, §6 T2, §7 |
| r26-F6 | statement-contradiction | single `C_envelope` | §4.4, §4.5, §5 AC20 |
| r27-F5 | statement-contradiction | `availability` sentences qualified (full vs paged) | §6 T6 |

No finding is factually wrong. Reviewer-adequacy items K1–K10 remain intact. R/AC/T IDs stable; AC18/AC19/AC20 added (not renumbered).

## 11. OPEN OWNER DECISIONS

### OD1 — Retrieval surface shape
- **Adopted OD1-A:** a fourth first-class built-in tool `retrieve_result`.

### OD2 — Retained-result storage bounds
- **Final (F4):** `max_retained_result_bytes = 8388608` (8 MiB, per-entry cap); `max_bytes_per_session = 16777216` (16 MiB, hard); `max_entries_per_session = 16`; `tombstone_bytes_per_session = 1048576` (1 MiB); `max_bytes_aggregate = 67108864` (64 MiB, advisory); `handle_ttl = 1h`. Aggregate exposure = `(max_sessions + 1) × (max_bytes_per_session + tombstone_bytes_per_session)` = `4097 × 17825792 = 73032269824` bytes (≈ 68 GiB).

### OD3 — Handle expiry policy
- **Adopted OD3-A:** fixed `handle_ttl` (1h) plus purge at session retirement and a sweep driven from the inbound request path (§4.5).

### OD4 — Invalid epoch (owner, F2)
- **Adopted:** a present-but-invalid epoch header is rejected before the seam with the distinct `invalid_epoch` code (per-surface matrix); only a genuinely absent epoch (session-less) uses the publisher namespace; `catalog_changed` keeps its exact meaning.

### OD5 — CSPRNG mandatory (owner)
- **Adopted:** `crypto/rand` (or equivalent OS CSPRNG) is mandatory, verified by inspection evidence in the receipt.

**Recommendation:** OD1-A, OD2 (final), OD3-A, OD4, OD5. These preserve the ticket's "expiring, session-scoped, bounded" contract under total retention up to the per-entry cap, without shared-transport changes and without ever binding a session-ful result to the publisher namespace.