# Spec — token-efficient-tool-responses / Ticket 01: bounded tool results with recoverable paging

## 1. STATUS

- **Spec ID / revision:** `ticket01-bounded-results-v5` (r5 repair; supersedes v4)
- **Status:** DRAFT (final authorized review unit; planner does not approve)
- **Objective:** Guarantee that no bridge tool response writes an unbounded payload to the client — when the fully serialized result exceeds the configured delivery ceiling (default 8 KiB) the caller receives a bounded preview plus a bridge-issued, session-bound handle and can recover the complete result through bounded paged retrieval — while preserving session isolation, projection ownership/epoch invariants, catalog-revision checks, and the existing error taxonomy.

**Problem and intent (tied to observed token cost):** bridge discovery and generic dispatch consumed ~44% of billed prompt tokens in the reviewed session. The bridge today re-serializes tool results twice (once into a text content block and once into `structuredContent`, and for generic `call_tool` a third copy inside `structuredContent.upstream_result`) and delivers discovery/describe results unbounded. Bounding the *serialized* output — not the upstream payload and not the transport `max_response_bytes` cap — puts a hard ceiling on the worst case a single tool response can cost, without hiding execution facts: the preview keeps `isError` and execution outcome visible, and the complete result stays recoverable through bounded pages. This ticket only bounds the worst case; it does not add default projections (ticket 05) or deduplication (ticket 06).

## 2. REQUIREMENTS

<!-- specdd:requirements:start -->
All requirements are **enforced** (falsifiable acceptance contract) unless marked otherwise. IDs are stable; the original ticket ACs O1–O9 map as O1→R1,R2; O2→R3,R4; O3→R7; O4→R8; O5→R9; O6→R10; O7→R11; O8→R11; O9→R12.

- **R1 (enforced).** Any tool result whose fully serialized form exceeds the configured delivery ceiling (`delivery_ceiling_bytes`, default 8192) is delivered as a bounded preview. When the result is retainable, the preview also carries an opaque handle; the preview always reports `total_bytes` (the unbounded serialized length) and `paging_required` (true iff a handle is present), and always reports the upstream error class `is_error` (+ `upstream_code` when the upstream supplied one). The whole preview delivery — text block and structured form included — is ≤ the configured preview ceiling (`preview_ceiling_bytes`, default 1536) serialized bytes. The explicit exception is an unretainable result, which has exactly three causes with this precedence: **`no_session`** (request has no session owner) → **`too_large`** (single result exceeds `max_retained_result_bytes`) → **`capacity`** (aggregate cap cannot be satisfied by evicting the creating session's own handles). An unretainable result is a bounded preview with `retained:false`, `paging_required:false`, no handle, `retention:"result_too_large"`, and `retention_reason` naming the first cause in precedence order; its upstream error class is still preserved.
- **R2 (enforced).** A result ≤ the configured delivery ceiling (default 8192) is delivered byte-identically to today (no handle, no injected metadata). Falsifiable default-boundary fixtures: exactly 8192 bytes is delivered unchanged; 8193 bytes pages.
- **R3 (enforced).** Each retrieved page is ≤ the configured delivery ceiling (default 8192) serialized bytes and reports: 1-based `page`, `total_pages`, `remaining_pages`, `total_bytes`, `remaining_bytes`.
- **R4 (enforced).** Concatenating the page content (text blocks) in order reproduces the unbounded serialized result byte-exactly; re-retrieval of the same handle/page returns identical bytes until expiry or eviction.
- **R5 (enforced).** Every page is valid UTF-8; a multibyte rune spanning a page boundary round-trips; the ceiling measures post-escape serialized bytes, not the raw payload length.
- **R6 (enforced).** The retrieval operation, its parameters, and the preview/page contract are documented in `README.md`; the operation, its parameters, and the preview/page contract appear in the tool description; the parameters appear in the stored input schema. Usable without the caller predicting the result shape. *(Advisory sub-part: the prose that steers clients away from redundant discovery — matching the existing `search_tools`/`describe_tool` guidance style.)*
- **R7 (enforced).** Handles are crypto-random and not derived from content or session identifiers. A tampered, unknown, other-session, or other-incarnation handle fails with the same typed rejection, zero retained-result and page bytes (no upstream payload leaked), and no existence signal. A malformed handle (wrong length/alphabet) or an out-of-range page selector is rejected by handler-side shape validation as `invalid_arguments` (shape before store access; range after lookup).
- **R8 (enforced).** Expired and evicted handles fail with distinct typed error codes — never an empty page and never a partial success.
- **R9 (enforced).** Retained-result storage has a configured per-session entry bound and byte bound plus a documented aggregate byte cap; eviction is deterministic; one session cannot evict another session's live handles; evicting a handle makes retrieval return the evicted error; concurrent retrieval of the same handle/page returns a complete page or a typed error, never a partial page.
- **R10 (enforced).** An oversized upstream `isError` result keeps `is_error` (and the upstream's own top-level structured `upstream_code` when supplied) visible in the preview and every page — on every branch, retainable or unretainable; `upstream_code` is never silently truncated (any truncation carries a visible marker, and the full code remains byte-exact in the retained result). Bridge-originated dispatch failures keep their existing small typed `errorResult` shape and are never converted into a paged success.
- **R11 (enforced).** The ceiling applies identically to `call_tool`, directly invoked projected tools, `search_tools`, and `describe_tool`; paging never depends on the caller predicting the output shape (the preview supplies `handle` and `total_bytes`, pages supply `total_pages`/`remaining_*`). *(Forward compatibility: a future ticket-05 projection or `full` request remains subject to this ceiling; this ticket adds no such request and asserts no such fixture.)*
- **R12 (enforced).** Durable behavioral tests cover R1–R11, including: pass-through at/below the 8192 boundary, multi-page byte reconstruction, UTF-8 boundary round-trip, cross-session and cross-incarnation rejection, forged-handle rejection, expiry, eviction and eviction isolation, error-result paging (retainable and unretainable), both dispatch paths, discovery/describe, and serialized byte bounds.
<!-- specdd:requirements:end -->

## 3. EVIDENCE AND UNCERTAINTY

### Verified facts (read-only inspection)

- `call_tool` (generic dispatch) builds an outer provenance payload `{server, tool_name, request_id, upstream_result}` and returns it as `StructuredContent` while also returning the upstream `Content` blocks, so the payload is serialized twice (plus `upstream_result` repeats upstream content). Anchor: `internal/service/tools.go:865-908`.
- **`call_tool` resolves no owner identity** (it admits against the catalog assembly directly, `internal/service/tools.go:700-863`), while `directRequestOwner(req)` exists and is used only by the direct-execution path (`tools.go:225`) and by `search_tools` (`tools.go:439`) and `describe_tool` (`tools.go:562`). This is the F1 gap the owner-resolution rule (§4.0) closes.
- `search_tools` and `describe_tool` marshal their payload into a text block (`Content` = `string(raw)`) **and** set the same payload as `StructuredContent`. Anchors: `internal/service/tools.go:457-471` (`search_tools`) and `:624-655` (`describe_tool`).
- Directly invoked projected tools forward upstream `Content` and `StructuredContent` unchanged (`internal/service/direct_execution.go:59-63`).
- Sessions carry a trusted SDK session ID plus an `X-Mcp-Bridge-Exposure-Epoch` incarnation value; the projection owner is `{kind, sessionID, epoch}` and reused session IDs cannot inherit an earlier projection (`internal/service/projection.go:1-40`, `:195-202`; `internal/service/session_exposure.go:146-172`). `directRequestOwner` returns `publisherOwner` for a session-less request and `{kind:session, sessionID, epoch}` only for a session-ful request with a valid epoch header (`internal/service/tools.go:315-324`).
- **`admitExposureRequest` admits session-less requests to the publisher projection** (returns `coordinator.projections[t.publisherOwner]` when `request.GetSession()` is nil/empty; `internal/service/session_exposure.go:105-120`). So a session-less `retrieve_result` is admitted, not rejected; its observable is the empty publisher namespace → `invalid_handle` (H2).
- **Owner retirement is a single funnel with four named seams:** the idle reaper (`internal/service/inbound.go:1120-1145` → `cleanupRetiredOwners`), explicit `DELETE /mcp`, SDK-session-loss detection (`internal/service/inbound.go:800-816` → `retireLiveOwnerAtInboundLock` + `cleanupRetiredOwner`), and rehydration that mints a new epoch (`internal/service/inbound.go:850` / `:921` → `newLiveOwner`). A purge hook wired to `cleanupRetiredOwner` therefore covers all four (R7 finding).
- A signed-cursor implementation already exists with an HMAC-over-epoch+generation+offset pattern and canonical re-encode checks (`internal/service/cursor.go`).
- Stored tool input schemas validate with `additionalProperties:false` (`internal/catalog/schema.go:74-89`), so a retrieval surface must be a first-class tool (or an explicit schema change), not an accidental extra parameter on an existing tool.
- **The SDK does not enforce input-schema `pattern`/`minimum`; it delegates argument validation to the caller (`go-sdk mcp/server.go:306-308`), and existing handlers only `decodeStrict` (`internal/service/tools.go:1074-1081`), which rejects unknown fields and non-objects but not pattern/minimum.** Therefore built-in handler shape validation must be handler-side (§4.3).
- `upstream_defaults.max_response_bytes` is a transport cap (`internal/config/bounds.go:86`) with a permitted range up to 8388608; it is a connection-level limit and must not be reused as the delivery ceiling.
- Bridge tool-layer error codes are `catalog_changed`, `unknown_tool`, `backend_unavailable`, `unsupported_schema`, `internal_error` (`internal/service/tools.go:993-999`); `errorResult` emits a fixed small shape `{code, message, retryable, execution_state}` with one TextContent (`internal/service/tools.go:1037-1048`). Core dispatch codes are `catalog_changed | unknown_tool | unsupported_capability` plus transport codes (`deadline_exceeded`, `backend_busy`, `upstream_protocol_error`, …) (`internal/core/core.go:77-82`).
- `requiresExposure` returns `false` for `catalog.ToolCallTool` and `true` only for `search_tools`/`describe_tool` and projected names (`internal/service/session_exposure.go:60-83`); a fourth built-in needs its own `requiresExposure` case to be exposure-relevant.
- Observability uses structured `observe.Entry` JSON logs and a private Prometheus registry with bounded label vocabularies (`internal/observe/metrics.go:35-52`); the structured-logging vocabulary is documented in `docs/logging-contract.md` (661 lines), which must gain the new events.

### Built-in-surface assertion inventory (R1 finding, coordinator-verified — authoritative)

Adding `retrieve_result` falsifies every count/set assertion below. **Rule:** every exact-count / exact-name-set / `exposed_tools` assertion a surface change affects is a write root of the owning task (T3); presence-only name loops are unaffected and stay out.

| File | Assertion |
| --- | --- |
| `internal/service/tools_test.go:512-548` | exact built-in descriptor set |
| `internal/service/progressive_session_test.go:180-183` | wanted tool names include the three built-ins |
| `internal/service/progressive_concurrency_lifecycle_test.go:697-731` | exact surface in lifecycle schedule |
| `internal/service/tool_metadata_test.go:108-120` | `want` map of three names, `len(got)!=3` |
| `internal/service/tools_publish_atomic_test.go:50-57` | `want` map of three names plus `alpha__good`, `len(got)!=4` |
| `cmd/mcp-bridge/serve_hybrid_exposure_test.go:201-204` | `tools/list length == 4`, three-name meta list |
| `cmd/mcp-bridge/serve_hybrid_reload_test.go:217,226,238,262` | 4/5/4 across reload phases |
| `cmd/mcp-bridge/serve_hybrid_progressive_reveal_test.go:352-390` | exact name sets |
| `cmd/mcp-bridge/serve_hybrid_progressive_reveal_lifecycle_test.go:278-286,492` | exact name sets |
| `cmd/mcp-bridge/serve_oauth_test.go:449,500` | `exposed_tools == 3` |
| `cmd/mcp-bridge/serve_test.go:618` | `len(listed.Tools) != 3` |

Presence-only assertions (`cmd/mcp-bridge/serve_hybrid_metadata_test.go:143`, `cmd/mcp-bridge/serve_oauth_test.go:1734`) are unaffected and need no change. With a fourth built-in the numbers move as: internal `3`→`4` (`tools_test.go`, `progressive_session_test.go` name map), `3`→`4`/`53`→`54`/`1053`→`1054` (`progressive_concurrency_lifecycle_test.go`), `3`→`4` (`tool_metadata_test.go`), `4`→`5` (`tools_publish_atomic_test.go`); `cmd/mcp-bridge` `4`→`5`/`5`→`6`/`4`→`5` (exposure + reload), `exposed_tools 3`→`4`, `len(listed.Tools) 3`→`4`, and every name map gains `retrieve_result`.

### README changed-claim anchors (H3)

`README.md:279` ("One MCP server, three tools"), `:286-296` + `:288` ("All three declare standard MCP tool annotations"), `:322-325` (discovery/describe pruning + "`call_tool` keeps its outer provenance envelope and complete upstream result"), `:337` ("`search_tools` includes the revision at the response root"), `:443` ("exactly the three bridge built-ins"), `:468-476` ("`structuredContent.upstream_result` is the complete original `CallToolResult`").

### Assumptions

- The MCP transport serializes `CallToolResult` with the standard JSON keys `content`, `structuredContent`, `isError`, using `encoding/json` with HTML escaping enabled. The accounting oracle is defined against the SDK's own marshaling path; see the K1 caution in §4.4.
- The existing `projectionOwner` (session+epoch, or the publisher owner for session-less callers) is the correct identity to bind handles to; retention is restricted to session owners per §4.0.

### `[AUTO-DECISION]` entries (planner-chosen, for coordinator persistence in the task log/decisions file)

1. **Byte-accounting oracle.** `measureResult(result)` = `len(serialize(result))` where `serialize` marshals the `CallToolResult` value through the SDK's own `MarshalJSON` path (not a hand-built field count), so SDK-added fields (`_meta`, `resultType`, …) are included; the constant JSON-RPC wrapper is excluded (§4.4). Consequence: `total_bytes` can exceed the raw payload length; tests must build escaping-inflated fixtures.
2. **Page model.** The retained value is the complete serialized `CallToolResult` bytes; a page is a UTF-8-safe contiguous byte slice of those bytes; concatenation == retained bytes.
3. **Page partition algorithm class.** Pages are a maximal-fill, metadata-aware partition: given the page's own metadata envelope size (bounded by `max_metadata_envelope_bytes`, §4.4), choose the largest UTF-8-safe prefix of the remaining retained bytes such that the whole page delivery ≤ the configured delivery ceiling.
4. **Error-code vocabulary.** New tool-layer codes: `invalid_handle` (structurally valid but unknown/tampered/foreign-session/other-incarnation/session-less — one fixed shape, no existence signal), `result_expired`, `result_evicted`, `result_too_large` (retention classification in the `retention` field, `retention_reason` ∈ {`no_session`, `too_large`, `capacity`}). Malformed/pattern-violating handles and out-of-range page selectors are `invalid_arguments` (shape before store, range after lookup).
5. **Binding identity.** Handles bind only to session owners (`{kind:session, sessionID, epoch}`). Session-less (publisher) requests never receive a handle; the publisher namespace is always empty, so session-less retrieval yields `invalid_handle` (H2).
6. **Error-class vs retention classification.** The upstream error class (`is_error` + optional `upstream_code`) and the bridge retention classification (`retention` + `retention_reason`) are two independent fields; `upstream_code` is never overwritten by the retention failure.
7. **Remaining-arithmetic convention.** `remaining_*` counts what comes strictly after the current page; see §4.2 (L1).
8. **`upstream_code` bounding.** `upstream_code` is copied into metadata only up to `max_upstream_code_bytes` (default 128), UTF-8-safe, with a trailing `…` (U+2026) marker when truncated; the full code remains byte-exact in the retained result (R10 truthfulness).

### Inaccessible prerequisites

- None that block planning. The only genuinely owner-owned values (exact storage bounds; retrieval surface shape) are captured as open decisions in §11 with recommendations; independent tasks remain plannable.

### W09E evidence-packet assessment

Compact anchored risk evidence and counterexamples are placed in §10 (W09E matrix). Decisive gaps: none — the owner decisions in §11 are value choices, not unknown facts.

## 4. DESIGN

<!-- specdd:design:start -->
### 4.0 Owner resolution (general rule)

Every result-producing surface resolves its handle owner through the same `directRequestOwner(req)` seam before the ceiling decision. The four surfaces are:

| Surface | Owner resolution |
| --- | --- |
| `call_tool` (generic) | **changed** to call `directRequestOwner(req)`; today it resolves no owner (`internal/service/tools.go:700-863`) |
| directly invoked projected tool | already resolves `owner` via `directRequestOwner` (`tools.go:225`) |
| `search_tools` | already resolves via `directRequestOwner` (`tools.go:439`) |
| `describe_tool` | already resolves via `directRequestOwner` (`tools.go:562`) |

`directRequestOwner(req)` yields exactly one of: `{kind:session, sessionID, epoch}` (session-ful request with a valid epoch header), the single `publisherOwner` (session-less request), or no owner (`ownerOK=false`, session-ful with missing/invalid epoch). **Handles are issued only when the resolved owner is a session owner.** A session-less (publisher) over-ceiling result, or a request whose owner does not resolve, is delivered as a bounded preview with `retained:false`, `paging_required:false`, no handle, `retention:"result_too_large"`, and `retention_reason:"no_session"` (§4.2). This makes R7 true by construction: every handle is keyed by exactly one `{sessionID, epoch}`, and retrieval re-resolves the caller's owner through the same seam and matches it exactly. Effect on write roots: `call_tool` (`internal/service/tools.go`) gains the owner-resolution call inside its existing T4 write root; no new file.

### 4.1 Selected approach

Introduce a single measurement/decision seam at the point where each tool handler returns its `*mcp.CallToolResult`, applied to **all four** result-producing surfaces (generic `call_tool`, direct projected execution, `search_tools`, `describe_tool`): measure → if ≤ the configured ceiling return byte-identically; else retain the full serialized result (session owners only) and return a ≤ preview-ceiling preview + handle. Add a retained-result handle store keyed by the session owner, and a fourth built-in tool `retrieve_result` that pages the retained bytes. The store and the retrieval tool are the only new state; the existing error taxonomy, exposure admission, and catalog-revision checks are reused unchanged. For `search_tools`/`describe_tool`, the seam runs **after** `commitDiscovery`, so discovered-tool exposure still commits even when the result pages (K7).

### 4.2 Caller-visible contract

**Under-ceiling (≤ the configured delivery ceiling).** Byte-identical to today's output for every surface: `call_tool` keeps its provenance envelope, direct projected tools forward upstream content unchanged, discovery/describe keep their text+structured forms. No `handle`, no injected metadata.

**Over-ceiling, retainable (session owner + within `max_retained_result_bytes` + aggregate admitted).** The response is replaced by a preview with:
- MCP `isError` = the original's `isError` (a failure is never mistaken for truncation).
- `Content`: one text block = a UTF-8-safe byte prefix of the retained serialized result, sized so the whole preview fits ≤ the configured preview ceiling (default 1536).
- `StructuredContent`: `{ "paging_required": true, "handle": "<opaque>", "total_bytes": <int>, "retained": true, "is_error": <bool>, "upstream_code": <string|null> }`. `upstream_code` is present only when the original is an error and its own structured content carries a top-level string `code`; otherwise omitted.

**Over-ceiling, unretainable (R6 finding).** Bounded preview with the **same content prefix and `isError` mirroring as the retainable preview**:
- MCP `isError` = the original's `isError` (never false for an unretainable upstream error).
- `Content`: one text block = a UTF-8-safe byte prefix of the full serialized result (the bytes that would have been retained), sized so the whole preview fits ≤ the configured preview ceiling. The preview is never metadata-only.
- `StructuredContent`: `{ "paging_required": false, "retained": false, "total_bytes": <int>, "is_error": <bool>, "upstream_code": <string|null>, "retention": "result_too_large", "retention_reason": "no_session"|"too_large"|"capacity" }` and no `handle`. The upstream error class (`is_error` + `upstream_code`) is preserved independently of `retention`; `retention_reason` names the first cause in precedence order `no_session` → `too_large` → `capacity` (§4.5).

**Each page (via `retrieve_result`).**
- MCP `isError` = the original's `isError`.
- `Content`: one text block = the page's UTF-8-safe byte slice of the retained result.
- `StructuredContent`: `{ "handle": "<opaque>", "page": <1-based int>, "total_pages": <int>, "remaining_pages": <int>, "total_bytes": <int>, "remaining_bytes": <int>, "is_error": <bool>, "upstream_code": <string|null> }`. `upstream_code` is present on every page exactly as on the preview (same bounded form, §4.4), so error classification survives paging.
- **Remaining-arithmetic convention (L1):** the retained result is partitioned into `total_pages` pages. `total_bytes` = `measureResult(full)` and is constant across all pages. `remaining_pages = total_pages - page` (number of pages strictly after the current page). `remaining_bytes` = the sum of retained-slice byte lengths of pages strictly after the current page. Both are `0` on the last page.
- Concatenation of all pages' text blocks, in order, equals the retained (full serialized) result byte-for-byte.

**Error-code source rule (F2).** `is_error` is the original result's MCP `isError`. `upstream_code` is the string `code` at the top level of the **upstream result's own** structured content when that result `isError` and carries a top-level string `code`: for direct projected tools this is the original `structuredContent.code`; for generic `call_tool` it is the decoded `upstream_result.structuredContent.code` (NOT the bridge provenance envelope `{server, tool_name, request_id, upstream_result}`); for discovery/describe it is always absent. The store persists `{is_error, upstream_code}` alongside the retained bytes so pages reproduce the same metadata.

**`upstream_code` bounding (R2 finding).** `upstream_code` is copied into metadata only up to `max_upstream_code_bytes` (default 128), UTF-8-safe, with a trailing `…` (U+2026) marker when and only when truncated. The full code is never silently truncated: it remains byte-exact in the retained result. `max_upstream_code_bytes` is config-validated (`0 < max_upstream_code_bytes ≤ 512`).

**Discriminator vs ticket 02 (F14).** This ticket signals paging with `paging_required` plus a `handle`; it does **not** emit a `truncated` flag. Ticket 02's count clipping uses `truncated` (no handle). The two are mutually distinguishable by `paging_required`+`handle` (paging) versus `truncated` without a handle (count clipping).

### 4.3 Retrieval surface (fourth tool)

`retrieve_result` is a first-class built-in tool, added beside `search_tools`/`describe_tool`/`call_tool` in `renderBuiltinDescriptors`, registered in `registerBuiltinTools`, and made exposure-relevant by a new `requiresExposure` case (`case catalog.ToolRetrieveResult: return true, false, ""`), so it follows the same admission path as `describe_tool`. Its stored input schema uses `additionalProperties:false` and declares exactly:

```json
{"type":"object","additionalProperties":false,"required":["handle"],"properties":{
  "handle":{"type":"string","pattern":"^[A-Za-z0-9_-]{43}$"},
  "page":{"type":"integer","minimum":1}
}}
```

`handle` is 32 random bytes base64url-encoded without padding (43 chars). `page` is optional and defaults to 1. The tool annotations are `readOnlyHint: true`, `destructiveHint: false`, `idempotentHint: true`, `openWorldHint: false`. The tool description documents the preview/page contract (R5 finding: the description text is owned by T3 and asserted in AC6). The caller never predicts the result shape: the preview already supplies `handle` and `total_bytes`, and page metadata supplies `total_pages`/`remaining_*`.

**Handler-side validation, split (R-03 + R3 finding).** The SDK does not enforce the schema `pattern`/`minimum`; the schema is the caller-visible contract and the handler is the enforcer. The retrieval handler validates in two phases:
1. **Pre-store shape validation** (before any store access): `decodeStrict` into `struct{ Handle string; Page *int }` (rejects unknown fields and non-object arguments); validate `handle` is present, exactly 43 chars, base64url alphabet `[A-Za-z0-9_-]`; validate `page` (if present) is a JSON integer ≥1. A violation → `invalid_arguments`, zero store access.
2. **Post-lookup range validation** (after the owner-namespaced lookup resolves the entry and its `total_pages`): `page > total_pages` → `invalid_arguments`.

Between the two phases: owner-namespaced store lookup → `invalid_handle` (unknown/tampered/foreign/other-incarnation/session-less), `result_expired`, or `result_evicted`. **Ordering observable:** an out-of-range `page` on an *invalid* handle yields `invalid_handle` (never `invalid_arguments`), proving the handle lookup precedes the range check; an out-of-range `page` on a *valid* handle yields `invalid_arguments`.

**Session-less observable (H2).** `admitExposureRequest` admits session-less requests to the publisher projection (it does not reject them; `internal/service/session_exposure.go:105-120`). The handler resolves the session-less owner as `publisherOwner` via `directRequestOwner`. Because handles are never issued to the publisher owner (§4.0), the publisher namespace is always empty, so a session-less `retrieve_result` with any structurally valid handle returns `invalid_handle` — never another session's bytes and never `result_expired`/`result_evicted`. This is the observable AC17 asserts, not an admission rejection.

**`invalid_handle` scope (F16).** `invalid_handle` covers only structurally valid (43-char base64url) but unrecognized handles: unknown, tampered, foreign-session, foreign-incarnation, or session-less (empty publisher namespace) — all with one fixed shape, zero retained-result/page bytes, and no existence signal. A handle that fails shape validation is `invalid_arguments` before the handler runs the store; this reveals only the submitted value's format, never any handle's existence.

**Built-in-surface inventory (R1 finding).** Adding any built-in tool falsifies every exact-count / exact-name-set / `exposed_tools` assertion across `internal/service` and `cmd/mcp-bridge`. The complete current inventory is enumerated in §3 and owned by T3's write roots. **A future fifth built-in tool must repeat this inventory step** (re-grep for exact built-in counts, exact name sets, and `exposed_tools` totals across `internal/service`, `cmd/mcp-bridge`, and any other package) and add every hit to that task's write roots before dispatch. Presence-only name loops are unaffected and are excluded.

**Decision note:** this is open decision OD1 in §11 (fourth tool vs. schema change to an existing tool). Recommendation: fourth tool.

### 4.4 Byte-accounting definition and metadata envelope bound

- **Measured form (F12):** the complete `CallToolResult` **value** — the `content` array (each block fully serialized, including its `{"type":"text","text":…}` envelope and any per-block fields), the `structuredContent` object, and the `isError` field — serialized with the same `encoding/json` codec the transport uses (HTML escaping on).
- **K1 caution:** the oracle marshals the `CallToolResult` through the SDK's own `MarshalJSON` path (never a hand-built field enumeration), so any SDK-added fields (`_meta`, `resultType`, or future additions) are included in the measured bytes and cannot escape the measurement.
- **Excluded wrapper constant (F12):** the ceiling excludes the constant JSON-RPC response envelope around the result (`jsonrpc`, `id`, and the `result` wrapper braces) and any HTTP/SSE framing, because those are per-request constants that do not scale with tool output; the ceiling bounds the result payload, which is what scales. The excluded constant is never measured.
- **What counts once:** each distinct serialized form counts once per occurrence. A value that appears in both a text content block and `structuredContent` is counted **in each form separately** (and for `call_tool`, the `upstream_result` copy counts a third time). This is intentional: the client pays tokens for every serialized copy, and ticket 06 (deduplication) is a separate later ticket that must not change this ticket's accounting.
- **Escaping:** JSON escaping expansion counts toward the ceiling (e.g., `<`, `>`, `&` → `\u003c`-family, U+2028/U+2029, quote/backslash/control escapes). Multibyte UTF-8 in string values is emitted raw and counted as its raw byte length.
- **`total_bytes`** reported to the caller equals `measureResult(originalResult)` for the complete unbounded result.
- **Metadata envelope bound (R2 finding).** Every page/preview metadata field is bounded: `handle` (43 bytes), `page`/`total_pages` (decimal digits of integers ≤ a bound derived from `max_retained_result_bytes`), `total_bytes`/`remaining_bytes` (decimal digits), `is_error` (bool), `upstream_code` (≤ `max_upstream_code_bytes` + the `…` marker). The sum is a constant `max_metadata_envelope_bytes`. **Page-feasibility invariant:** config validation enforces `max_metadata_envelope_bytes < delivery_ceiling_bytes`, so every page admits a non-empty content prefix. If the invariant is ever violated at runtime (only reachable via misconfiguration or a future metadata growth), retrieval returns the typed `internal_error` "page metadata exceeds the delivery ceiling" — never an empty page, never a silent truncation, never a loop.
- **Boundaries (H4):** the delivery boundary is the configured `delivery_ceiling_bytes` (default 8192: 8192 → pass-through, 8193 → paged); the preview bound is the configured `preview_ceiling_bytes` (default 1536); each page delivery is ≤ the configured `delivery_ceiling_bytes`; all measured by the same oracle. The 8192/8193 boundary fixtures use the default values.

### 4.5 Handle lifecycle

- **Generation (F13):** 32 bytes from `crypto/rand`, base64url-encoded (no padding), independent of content and of session/owner identifiers. Non-derivation is not separately asserted (unfalsifiable for a CSPRNG); the observable property is uniqueness (§5 AC14).
- **Binding (F1):** each retained entry is keyed by the session owner `{sessionID, epoch}` (§4.0). Retrieval re-resolves the caller's owner through `directRequestOwner` and looks up **only within that owner's namespace**; a handle not present there → `invalid_handle`.
- **Owner verification ordering (F8):** the store is namespaced by owner, so existence, expiry, and eviction are evaluated only inside the caller's own namespace after the owner resolves. A foreign handle can therefore never elicit `result_expired`/`result_evicted`; it is indistinguishable from an unknown handle (`invalid_handle`).
- **Storage bounds (F15):** configured `max_entries_per_session`, `max_bytes_per_session`, `max_bytes_aggregate`, `max_retained_result_bytes`, `handle_ttl`, `max_upstream_code_bytes` (default 128), and the two ceiling keys `delivery_ceiling_bytes` (default **8192**) and `preview_ceiling_bytes` (default **1536**). Config-time validation enforces `0 < preview_ceiling_bytes < delivery_ceiling_bytes`, `0 < max_retained_result_bytes ≤ max_bytes_per_session ≤ max_bytes_aggregate`, per-session entry bound ≥1, **`handle_ttl > 0` (L3)**, **`0 < max_upstream_code_bytes ≤ 512` (R2)**, and the page-feasibility invariant `max_metadata_envelope_bytes < delivery_ceiling_bytes` (§4.4). R2's 8192/8193 fixture is the **default** boundary; a separate test asserts the configured-boundary behavior. Defaults and the exact bound numbers are open decision OD2 (§11).
- **Eviction (deterministic):** within a session, eviction is oldest-created-first (FIFO by creation sequence) when that session's own entry/byte bound is exceeded. When the aggregate cap would be exceeded by a new handle, evict the **creating** session's own oldest handles until it fits; if that is insufficient (or the creating session has no handles), the new result is delivered as a preview with `retained:false`, no handle, `retention:"result_too_large"`, and `retention_reason:"capacity"`. This preserves R9's "one session cannot evict another's live handles" strictly. A handle that is evicted returns `result_evicted` on retrieval.
- **Unretainable-cause precedence (R-02):** the three `result_too_large` causes are evaluated in this fixed order and exactly one is reported: **`no_session`** (no session owner) before **`too_large`** (`retainedSize > max_retained_result_bytes`) before **`capacity`** (aggregate admission cannot be satisfied by evicting the creating session's own handles). A single oversized result therefore reports `too_large` even if the aggregate is also full.
- **Creation serialization (F9):** handle creation — including the aggregate-cap admission and any eviction it triggers — is one atomic critical section (reserve-then-insert under one lock), so concurrent creates cannot overshoot `max_bytes_aggregate`.
- **Expiry (F10):** each handle has a TTL (`handle_ttl`) from creation. Expiry is enforced lazily on retrieval **and** by a periodic store sweep keyed on the handle's own TTL, independent of session lifecycle, so expired entries are reclaimed even for live idle sessions and never force premature eviction. Expired → `result_expired`.
- **Owner-retirement purge (one rule, R7 finding):** every path that retires a session owner — (1) the idle reaper, (2) explicit `DELETE /mcp`, (3) SDK-session-loss detection (`internal/service/inbound.go:800-816`), and (4) rehydration that mints a new epoch (`internal/service/inbound.go:850`/`:921`) — reaches the single `cleanupRetiredOwner` funnel. The store registers one purge callback on that funnel, so all four seams purge the retired owner's handles. A handle whose owner is retired becomes unreadable (`invalid_handle`) and is purged. **T2 owns the purge wiring.**
- **Catalog reload (R-06):** retained handles are catalog-revision-independent after creation. The retained bytes are the immutable result of an already-executed call; retrieval reads only the store and never consults the catalog, so a catalog reload neither invalidates nor re-admits handles. The session owner's `{sessionID, epoch}` persists across reload (the projection is re-published at the new runtime generation with the same epoch), so the handle binding stays valid. Handles are invalidated only by TTL, eviction, owner retirement (any of the four seams), or process restart.
- **Concurrency:** retained bytes are immutable after creation; a page read returns the complete page or a typed error (`result_evicted`/`result_expired`/`invalid_handle`), never a partial page. Reads/evict/expire are serialized on the store; a read that begins before eviction completes with the full page.
- **Failure modes:** typed codes only — `invalid_arguments` (malformed handle / bad page, handler-side: shape before store, range after lookup), `invalid_handle` (well-formed unrecognized, store-side, zero retained/page bytes), `result_expired`, `result_evicted`, `result_too_large` (unretainable, with `retention_reason`), and `internal_error` (page metadata exceeds ceiling). `retrieve_result` is itself never paged (its own delivery is ≤ the delivery ceiling by construction), so there is no recursive paging.

### 4.6 Invariants that must not change

- Session isolation: a handle is unreadable by another session, another incarnation of the same session, or a session-less (publisher) caller.
- Projection ownership/epoch: no new identity; handle binding reuses `projectionOwner` (session owners only).
- Catalog-revision checks: `catalog_changed` admission behavior is untouched for dispatch; paging does not bypass it. `retrieve_result` is catalog-independent by design (§4.5 catalog reload) and never re-admits against the catalog.
- Error classification: `isError` and `upstream_code` are preserved on every branch (retainable and unretainable) per R10; `upstream_code` truncation is always marked and the full code stays in the retained result; `errorResult` small shapes are never paged.
- `additionalProperties:false` stored schemas: no accidental extra parameter on existing tools; retrieval is its own tool.

### 4.7 Alternatives considered (meaningful tradeoffs only)

- **Signed self-contained handle (HMAC over owner+id, like `cursor.go`) vs. store lookup key.** Signed handles would be verifiable without a store but still require a store to hold the retained bytes and enforce expiry/eviction/bounds, so they add complexity with no isolation gain. Store-lookup with owner namespacing is simpler and reuses the existing ownership check.
- **Per-page deterministic slice offset vs. byte-range re-derivation.** Byte-range slicing of the retained serialized result is the only model that satisfies R4's byte-exact reconstruction with a single retained artifact.
- **Publisher-bound handles for session-less callers.** Rejected: a shared, long-lived publisher handle namespace lets one session-less caller read another's retained result and violates R7. Instead session-less over-ceiling results are `result_too_large` with `retention_reason:no_session`, and session-less retrieval is `invalid_handle` (§4.0, §4.3).
<!-- specdd:design:end -->

## 5. ACCEPTANCE CRITERIA

Each AC names an observable and an expected value (or a test that can fail), plus its oracle (test name in a task `-run`). R mapping is listed.

- **AC1 (R1).** Given a retainable fixture result with `measureResult > 8192`, the returned preview has `paging_required:true`, `retained:true`, a non-empty 43-char `handle`, `total_bytes == measureResult(full)`, and `measureResult(preview) ≤ preview_ceiling_bytes` (default 1536). Oracle: `TestPreviewBoundedAndMetadata` (T4).
- **AC2 (R2).** Given fixtures of exactly 8192 and 8193 bytes (default ceiling), the 8192 result is byte-identical to the un-bounded output (no handle/metadata) and the 8193 result is a preview. Oracles: `TestCeilingBoundary8192vs8193` (T4, execution paths) and `TestDiscoveryCeilingBoundary` (T5, discovery/describe paths).
- **AC3 (R3).** For a multi-page fixture, every page has `measureResult(page) ≤ delivery_ceiling_bytes` (default 8192) and reports `page`, `total_pages`, `remaining_pages`, `total_bytes`, `remaining_bytes` obeying the §4.2 remaining-arithmetic convention (`remaining_pages == total_pages - page`; `remaining_bytes` = sum of later page slice lengths; both 0 on the last page). Oracle: `TestPageMetadataArithmetic` (T3).
- **AC4 (R4).** Concatenating page text blocks reproduces the retained bytes exactly; two retrievals of the same handle/page return identical bytes. Oracle: `TestPageConcatenationByteExact` (T3).
- **AC5 (R5).** A fixture whose content includes a multibyte rune positioned to cross a page boundary round-trips (concatenation is byte-exact and every page is `utf8.Valid`); an escaping-inflated fixture (e.g., many `<`/`&`) is measured at its post-escape size. Oracles: `TestResultMeasure` (T1, escaping accounting) and `TestUTF8BoundaryAndEscapingAccounting` (T3, page UTF-8 + escaping round-trip).
- **AC6 (R6).** `retrieve_result` appears in `renderBuiltinDescriptors` with a description that documents the preview/page contract; its stored **input** schema has `additionalProperties:false` and exactly the §4.3 properties (`handle`, `page`); and `README.md` documents the retrieval operation and the preview/page output shapes. Oracle: `TestRetrieveResultSchema` (T3, schema + description half) + T6 docs checks (README half).
- **AC7 (R7).** Forged, unknown, other-session, other-incarnation, and session-less handles all return the identical `invalid_handle` shape (same code, same fixed message, zero retained-result and page bytes). Oracle: `TestInvalidHandleNoExistenceSignal` (T3, table-driven).
- **AC8 (R8).** An expired handle returns `result_expired`; an evicted handle returns `result_evicted`; neither returns retained/page bytes or a success flag. Oracle: `TestExpiredAndEvictedDistinct` (T2).
- **AC9 (R9).** Exceeding a session's entry/byte bound evicts that session's oldest handles only; aggregate overflow evicts the creating session's own handles only (never another session's live handles); concurrent retrievals return complete pages or typed errors, never partial. Oracle: `TestEvictionIsolationAndConcurrency` (T2).
- **AC10 (R10).** An oversized upstream `isError` result keeps `is_error:true` and its own top-level `upstream_code` in the preview and every page (for `call_tool`, extracted from `upstream_result.structuredContent.code`, not the bridge envelope); a bridge dispatch failure keeps its exact small `errorResult` shape and is never paged. Oracle: `TestErrorClassificationPreserved` (T4).
- **AC11 (R11).** The same ceiling decision applies to generic `call_tool`, direct projected execution, `search_tools`, and `describe_tool`; an oversized result on each surface is bounded identically (preview+handle for session owners). Oracles: `TestCeilingAcrossAllSurfaces` (T4, call_tool + direct) and `TestDiscoveryBounded` (T5, search + describe).
- **AC12 (R12).** The full suite passes and the aggregate production source-line coverage gate (W06 ≥90%) holds. Oracle: `go test ./... -count=1 -coverprofile=coverage-ticket01.out` + coverage gate, run by T7.
- **AC13 (R1).** An unretainable result — each of the three causes `no_session`, `too_large`, `capacity` — is delivered as a bounded preview ≤1536 bytes with a **non-empty `Content` prefix of the result bytes (not metadata-only)**, MCP `isError` mirroring the original, `paging_required:false`, `retained:false`, no `handle`, `retention:"result_too_large"`, the correct `retention_reason` (first cause in precedence order), and `total_bytes` present. Oracle: `TestUnretainableResultBranch` (T4, table-driven over three causes).
- **AC14 (R7).** Issuing many handles yields pairwise-distinct, 43-char base64url values (uniqueness). Oracle: `TestHandleUniqueness` (T2).
- **AC15 (R7).** Two-phase validation: (1) pre-store — a malformed handle (wrong length/alphabet) or a non-integer/`<1` `page` is rejected as `invalid_arguments` with zero store access; (2) post-lookup — a valid handle with `page > total_pages` is rejected as `invalid_arguments`. Ordering: an out-of-range `page` on an *invalid* handle yields `invalid_handle`, not `invalid_arguments`. Oracle: `TestRetrieveHandlerShapeValidation` (T3).
- **AC16 (R10).** An unretainable upstream `isError` result keeps `is_error:true` and its own `upstream_code` in the preview (MCP `isError` mirrors the original), with the retention failure in a separate `retention`/`retention_reason` field that does not overwrite `upstream_code`. Oracle: `TestUnretainableErrorClassPreserved` (T4).
- **AC17 (R7).** `retrieve_result` is exposure-relevant (registered in `requiresExposure`); a session-less request is admitted to the publisher projection (not rejected) and, because the publisher namespace never holds handles, returns `invalid_handle` for any structurally valid handle — never another session's bytes and never `result_expired`/`result_evicted`. Oracle: `TestRetrieveResultRequiresExposure` (T3).

## 6. TASK LEDGER

<!-- specdd:tasks:start -->
Dependencies reference T IDs only. Every task's successful termination requires the listed commands to pass with exit code 0; every task that touches `internal/service`, `internal/catalog`, `internal/observe`, or `cmd/mcp-bridge` also runs the existing-suite regression `go test ./internal/service/ ./cmd/mcp-bridge/ -count=1` (exit 0). Blocked tasks are unsuccessful, never "conversationally complete." Effects are the write roots each task may touch; scope outside a task's envelope is prohibited. Decision-log location for all tasks: `.omp/plans/decisions.md` (coordinator-persisted); receipts in `.omp/artifacts/receipt_<task_id>.json`.

### T1 — Byte-accounting oracle and ceiling constants
- **Objective:** introduce the `measureResult` seam and the `delivery_ceiling_bytes=8192` / `preview_ceiling_bytes=1536` / `max_upstream_code_bytes=128` constants (validated config with these defaults).
- **Deps:** none.
- **Inputs:** §4.4 definition; `internal/service/tools.go:865-908` (result construction); `internal/config/bounds.go` (bounds validation pattern).
- **Expected files:** `internal/service/result_measure.go` (new), `internal/config/` ceiling/code-bytes config keys + validation, `internal/service/result_measure_test.go` (new).
- **Allowed scope:** the accounting function and its unit tests; config key registration and bound validation (including the page-feasibility invariant). **Prohibited:** wiring it into any handler.
- **RED step / expected failure:** a test asserting `measureResult` of a duplicated-forms fixture equals the SDK-marshaled length of `content`+`structuredContent`+`isError` fails to compile/run because `measureResult` does not exist.
- **Deliverable:** `measureResult` correct against the SDK's own `MarshalJSON` path (including any `_meta`/`resultType` fields; excluding the JSON-RPC wrapper constant); boundary constants and validated defaults; unit tests for escaping inflation, duplicated-form double counting, and the 8192/8193 classification split.
- **AC IDs:** AC5 (escaping-accounting half).
- **Validation:** `go test ./internal/service/ -run 'TestResultMeasure' -count=1`; `go test ./internal/config/ -count=1`; existing-suite regression `go test ./internal/service/ ./cmd/mcp-bridge/ -count=1`.
- **Failed/blocked reporting:** failure of the accounting test against the SDK's actual serializer is BLOCKED (cannot proceed to T2 without a truthful byte count).

### T2 — Retained-result handle store (and metric registration)
- **Objective:** implement the handle store (crypto-random generation, session-owner binding, owner-namespaced lookup, per-session entry/byte bounds, aggregate cap, FIFO eviction with isolation, TTL expiry with a periodic sweep, atomic creation+admission, catalog-reload independence, owner-retirement purge across all four seams, concurrency-safe immutable reads), its config section `retained_results`, and the four Prometheus collectors in `internal/observe/metrics.go`.
- **Deps:** T1 (uses `measureResult` to compute retained byte size).
- **Inputs:** §4.5; `internal/service/projection.go:195-202` (owner identity); `internal/service/cursor.go` (crypto/encoding pattern); `internal/service/inbound.go:800-816,850,921,1120-1145` (retirement seams + reaper); `internal/config/bounds.go` (validation pattern); `internal/observe/metrics.go:35-52` (bounded-label registry pattern).
- **Expected files:** `internal/service/result_handle.go` (new), `internal/service/result_handle_test.go` (new), `internal/config/` `retained_results` keys + validation, `internal/observe/metrics.go` (four collectors + bounded labels + increment helpers), reaper/`cleanupRetiredOwner` purge hook + periodic TTL sweep.
- **Allowed scope:** store internals and tests; config keys; metric collectors; the single `cleanupRetiredOwner` purge hook; periodic sweep. **Prohibited:** any HTTP/JSON-RPC surface; handler wiring.
- **RED step / expected failure:** a test asserting cross-session rejection and FIFO eviction fails to compile because the store type does not exist.
- **Deliverable:** store with unique unguessable handles, session-owner binding, owner-namespaced lookup (owner-match precondition), bounds+isolation eviction, TTL expiry (lazy + periodic sweep), atomic creation/admission, catalog-reload independence, owner-retirement purge wired to `cleanupRetiredOwner` (all four seams), concurrency-safe page reads; the four metric collectors registered with bounded labels; unit tests for each R7/R8/R9 property and metric scraping.
- **AC IDs:** AC8, AC9, AC14.
- **Validation:** `go test ./internal/service/ -run 'TestResultHandle|TestExpiredAndEvictedDistinct|TestEvictionIsolationAndConcurrency|TestHandleUniqueness' -count=1`; `go test ./internal/config/ -count=1`; existing-suite regression `go test ./internal/service/ ./cmd/mcp-bridge/ -count=1`.
- **Failed/blocked reporting:** an eviction schedule that violates isolation, a partial-page concurrency result, an omitted retirement seam, or a metric that misuses an unbounded label is BLOCKED (lifecycle hazard).

### T3 — `retrieve_result` retrieval surface
- **Objective:** add the fourth built-in tool with the §4.3 schema, tool description, handler-side two-phase validation, exposure-admission registration, and the page/typed-error responses; update **every** built-in-surface assertion in the §3 inventory.
- **Deps:** T2.
- **Inputs:** §4.3, §4.2 page contract; `internal/catalog/schema.go:74-89` (schema pattern); `internal/service/tools.go:44-175` (descriptor registration) and `:1074-1081` (`decodeStrict`); `internal/service/session_exposure.go:60-83` (`requiresExposure`).
- **Expected files:** `internal/catalog/schema.go` (tool constant + schema), `internal/service/tools.go` (descriptor + description + handler + two-phase validation), `internal/service/session_exposure.go` (new `requiresExposure` case), `internal/service/retrieve_result_test.go` (new), and the complete §3 built-in-surface assertion inventory — `internal/service/tools_test.go:512-548`, `internal/service/progressive_session_test.go:180-183`, `internal/service/progressive_concurrency_lifecycle_test.go:697-731`, `internal/service/tool_metadata_test.go:108-120`, `internal/service/tools_publish_atomic_test.go:50-57`, `cmd/mcp-bridge/serve_hybrid_exposure_test.go:201-204`, `cmd/mcp-bridge/serve_hybrid_reload_test.go:217,226,238,262`, `cmd/mcp-bridge/serve_hybrid_progressive_reveal_test.go:352-390`, `cmd/mcp-bridge/serve_hybrid_progressive_reveal_lifecycle_test.go:278-286,492`, `cmd/mcp-bridge/serve_oauth_test.go:449,500`, `cmd/mcp-bridge/serve_test.go:618`.
- **Allowed scope:** the new tool, its schema, its description text, its handler-side two-phase validation, its `requiresExposure` registration, its tests, and updating every §3 assertion (add `retrieve_result` to name maps; bump internal `3`→`4`/`53`→`54`/`1053`→`1054`/`3`→`4`/`4`→`5`; `cmd/mcp-bridge` `4`→`5`/`5`→`6`/`4`→`5`/`exposed_tools 3`→`4`/`len(listed.Tools) 3`→`4`). Presence-only name loops stay unchanged. **Prohibited:** changing existing tool schemas or the envelope of other tools.
- **RED step / expected failure:** a wire test calling `retrieve_result` fails with "unknown tool" because the tool is not registered.
- **Deliverable:** working `retrieve_result` with two-phase validation (`invalid_arguments` for malformed handle/bad page shape before store; `invalid_arguments` for `page > total_pages` after lookup), exposure-admission registration, correct page metadata (including bounded `upstream_code` and the remaining-arithmetic convention), byte slices, and the typed error codes; every §3 assertion updated and green.
- **AC IDs:** AC3, AC4, AC5 (UTF-8 page half), AC6 (schema + description half), AC7, AC15, AC17.
- **Validation:** `go test ./internal/service/ -run 'TestRetrieveResult|TestPageMetadataArithmetic|TestPageConcatenationByteExact|TestUTF8BoundaryAndEscapingAccounting|TestRetrieveResultSchema|TestInvalidHandleNoExistenceSignal|TestRetrieveHandlerShapeValidation|TestRetrieveResultRequiresExposure' -count=1`; `go test ./internal/catalog/ -count=1`; existing-suite regression `go test ./internal/service/ ./cmd/mcp-bridge/ -count=1`.
- **Failed/blocked reporting:** schema with `additionalProperties:false` mismatch, any empty-page/partial-page outcome, a handler that reaches the store with a malformed handle or checks `page > total_pages` before lookup, a missing `requiresExposure` case, a missing description, or any §3 assertion left stale is BLOCKED.

### T4 — Ceiling enforcement on execution dispatch paths
- **Objective:** apply the measure→decide→preview+handle (or pass-through / unretainable) seam to generic `call_tool` and to directly invoked projected tools, preserving error classification (including the unretainable content prefix and `isError` mirroring) and adding `call_tool` owner resolution.
- **Deps:** T1, T2, T3.
- **Inputs:** §4.2, §4.0; `internal/service/tools.go:865-908` (generic callTool) and `:315-324` (`directRequestOwner`); `internal/service/direct_execution.go:29-63` (direct execution); `internal/service/tools.go:1037-1048` (`errorResult`).
- **Expected files:** `internal/service/bounded_delivery.go` (new shared wrapper), `internal/service/tools.go` (generic `call_tool` owner resolution + seam), `internal/service/direct_execution.go`, `internal/service/bounded_delivery_test.go` (new).
- **Allowed scope:** the two dispatch return paths and the shared wrapper; `call_tool` owner resolution via `directRequestOwner`; error-result preview metadata (retainable and unretainable, with the content prefix and `isError` mirroring); the unretainable branch with all three causes. **Prohibited:** touching discovery/describe handlers (T5), catalog, or session admission.
- **RED step / expected failure:** a test asserting an 8193-byte direct-projected result returns a preview+handle fails (current code returns the full result unchanged).
- **Deliverable:** over-ceiling results on both paths become preview+handle (session owners) or `result_too_large` with the correct `retention_reason` and a non-empty content prefix (publisher/session-less/too_large/capacity); under-ceiling remain byte-identical; `is_error`/`upstream_code` (bounded) preserved on every branch; bridge failures stay small; `call_tool` binds handles to the resolved session owner.
- **AC IDs:** AC1, AC2 (execution boundary), AC10, AC11 (call_tool + direct projected halves), AC13, AC16.
- **Validation:** `go test ./internal/service/ -run 'TestBoundedDelivery|TestPreviewBoundedAndMetadata|TestCeilingBoundary8192vs8193|TestErrorClassificationPreserved|TestCeilingAcrossAllSurfaces|TestUnretainableResultBranch|TestUnretainableErrorClassPreserved' -count=1`; existing-suite regression `go test ./internal/service/ ./cmd/mcp-bridge/ -count=1`.
- **Failed/blocked reporting:** any path that returns the full over-ceiling body, drops `isError`/`upstream_code`, returns a metadata-only unretainable preview, or lets a publisher/session-less caller obtain a handle is BLOCKED.

### T5 — Ceiling enforcement on discovery/describe
- **Objective:** apply the same seam to `search_tools` and `describe_tool`, placed after `commitDiscovery`.
- **Deps:** T1, T2, T3, T4 (reuses the shared wrapper).
- **Inputs:** §4.2; `internal/service/tools.go:457-471`, `:624-655`; `commitDiscovery` at `internal/service/tools.go:457-471` / `:624-655`.
- **Expected files:** `internal/service/tools.go` (both handlers), `internal/service/discovery_bounded_test.go` (new).
- **Allowed scope:** the two discovery/describe return paths. **Prohibited:** changing catalog-revision or exposure-admission behavior; moving the seam before `commitDiscovery`.
- **RED step / expected failure:** a test asserting an oversized `describe_tool` response pages fails (current code returns the full body).
- **Deliverable:** oversized discovery/describe results become preview+handle (session owners) or `result_too_large` (publisher) with a non-empty content prefix, placed after `commitDiscovery`; retrieved via `retrieve_result`.
- **AC IDs:** AC2 (discovery boundary), AC11 (search_tools + describe_tool halves), AC13 (reuses wrapper).
- **Validation:** `go test ./internal/service/ -run 'TestDiscoveryBounded|TestDiscoveryCeilingBoundary' -count=1`; existing-suite regression `go test ./internal/service/ ./cmd/mcp-bridge/ -count=1`.
- **Failed/blocked reporting:** a discovery path that bypasses the ceiling or a seam placed before `commitDiscovery` is BLOCKED.

### T6 — Documentation and changelog (docs-only)
- **Objective:** update `README.md` contract and tool descriptions; update `docs/logging-contract.md` with the new log events; record which README claims change and which remain true; add a changelog entry. Docs-only: no Go test written in this task.
- **Deps:** T3, T4, T5.
- **Inputs:** `README.md:279`, `:286-296`, `:288`, `:322-325`, `:337`, `:443`, `:468-476`; `docs/logging-contract.md`; `CHANGELOG.md`.
- **Expected files:** `README.md`, `docs/logging-contract.md`, `CHANGELOG.md`.
- **Allowed scope:** docs and changelog only. **Prohibited:** production/test code.
- **RED step / expected failure:** `grep -q 'retrieve_result' README.md` exits non-zero (the retrieval surface is absent).
- **Deliverable:** README documents `retrieve_result`, preview/page shapes (including `paging_required`+`handle` vs ticket 02 `truncated`), typed error codes, and the byte-accounting definition; updates every §6 README changed claim with qualified-at-or-below-ceiling truths; `docs/logging-contract.md` gains the new log events; changelog entry.
- **AC IDs:** AC6 (README half).
- **Validation (docs-only):** positive greps `grep -q 'retrieve_result' README.md`, `grep -q 'paging_required' README.md`, `grep -q 'result_too_large' README.md`; positive qualified-truth greps `grep -q 'at or below the ceiling' README.md` (or an equivalent qualification); negative unqualified-falsehood greps `! grep -q 'One MCP server, three tools' README.md`, `! grep -q 'All three declare' README.md`, `! grep -q 'complete upstream result' README.md`, `! grep -q 'is the complete original' README.md`, `! grep -q 'exactly the three bridge built-ins' README.md`; `grep -q 'result_paged' docs/logging-contract.md`; `git diff --check`. No `go test` run in this task.
- **Failed/blocked reporting:** docs contradicting the implemented contract (e.g., still claiming "three tools", "All three declare", or an inline "complete upstream result") is BLOCKED.

### T7 — Integration and coverage gate
- **Objective:** run the full suite and the aggregate coverage gate so AC12 is attached to an executable task.
- **Deps:** T2, T3, T4, T5 (implementation complete; T6 not required for the gate).
- **Inputs:** §5 AC12; W06 ≥90% aggregate production source-line coverage.
- **Expected files:** `.omp/artifacts/coverage-ticket01.out` (disposable coverage artifact; removed after capture per W10/W14).
- **Allowed scope:** run `go test ./...` and write the coverage artifact; remove the artifact after capture. **Prohibited:** production/test source edits.
- **RED step / expected failure:** `go test ./... -count=1` exits non-zero or coverage falls below 90%.
- **Deliverable:** full-suite pass and coverage ≥90%, recorded in the coordinator ledger; coverage artifact removed after capture.
- **AC IDs:** AC12.
- **Validation:** `go test ./... -count=1 -coverprofile=coverage-ticket01.out` (exit 0) + aggregate coverage ≥90%; then remove `coverage-ticket01.out`.
- **Failed/blocked reporting:** a failing full suite or sub-90% coverage is BLOCKED.

**Effects and exclusions (whole ticket).**
- **Write roots:** `internal/service/result_measure.go`, `result_handle.go`, `bounded_delivery.go`, `result_measure_test.go`, `result_handle_test.go`, `retrieve_result_test.go`, `bounded_delivery_test.go`, `discovery_bounded_test.go`; edits to `internal/service/tools.go`, `internal/service/direct_execution.go`, `internal/service/session_exposure.go` (admission switch + `requiresExposure` case only), `internal/service/inbound.go` (purge hook + periodic sweep only), the five `internal/service` built-in-surface tests, the six `cmd/mcp-bridge` built-in-surface tests (§3 inventory), `internal/observe/metrics.go`, `internal/catalog/schema.go`, `internal/config/`; `README.md`, `docs/logging-contract.md`, `CHANGELOG.md`; disposable `.omp/artifacts/coverage-ticket01.out` (T7 only, removed after capture).
- **Explicit non-goals:** no default field projections (ticket 05); no `full` projection knob; no duplicate-result reuse/dedup (ticket 06); no ranking/filtering changes; no upstream capability removal; no change to `upstream_defaults.max_response_bytes`; no change to catalog-revision or exposure-admission semantics; no recursive paging.
- **README claims (H3 + R8, complete):** **change** — `README.md:279` "One MCP server, three tools" → four tools (add `retrieve_result` row); `README.md:286-296` + `:288` "All three declare standard MCP tool annotations" → four (add `retrieve_result` row with `readOnlyHint:true`, `destructiveHint:false`, `idempotentHint:true`, `openWorldHint:false`); `README.md:322-325` discovery/describe pruning paragraph ending "`call_tool` keeps its outer provenance envelope and complete upstream result" → qualified: true at or below the ceiling, over-ceiling it is a preview and the complete result is retrievable via `retrieve_result`; `README.md:337` "`search_tools` includes the revision at the response root" → qualified: true at or below the ceiling, over-ceiling search delivers a preview (revision inside the retained bytes); `README.md:443` "exactly the three bridge built-ins" → four bridge built-ins; `README.md:468-476` "`structuredContent.upstream_result` is the complete original `CallToolResult`" → qualified: true at or below the ceiling. **Remain true** (qualified): catalog-revision semantics, tool-name derivation, error taxonomy codes, and session/epoch ownership rules. The provenance envelope fields (`server`, `tool_name`, `request_id`, `upstream_result`) remain true only for ≤ ceiling `call_tool` results; over-ceiling, the immediate `structuredContent` is the preview shape and the envelope is inside the retained bytes.
<!-- specdd:tasks:end -->

## 7. ENVELOPE

<!-- specdd:envelope:start -->
- **Write roots (tasks may touch only these):** new files `internal/service/result_measure.go`, `internal/service/result_handle.go`, `internal/service/bounded_delivery.go`, `internal/service/result_measure_test.go`, `internal/service/result_handle_test.go`, `internal/service/retrieve_result_test.go`, `internal/service/bounded_delivery_test.go`, `internal/service/discovery_bounded_test.go`; edits to `internal/service/tools.go`, `internal/service/direct_execution.go`, `internal/service/session_exposure.go` (exposure-admission switch + `requiresExposure` case only), `internal/service/inbound.go` (purge hook + periodic sweep only), `internal/service/tools_test.go`, `internal/service/progressive_session_test.go`, `internal/service/progressive_concurrency_lifecycle_test.go`, `internal/service/tool_metadata_test.go`, `internal/service/tools_publish_atomic_test.go`, `cmd/mcp-bridge/serve_hybrid_exposure_test.go`, `cmd/mcp-bridge/serve_hybrid_reload_test.go`, `cmd/mcp-bridge/serve_hybrid_progressive_reveal_test.go`, `cmd/mcp-bridge/serve_hybrid_progressive_reveal_lifecycle_test.go`, `cmd/mcp-bridge/serve_oauth_test.go`, `cmd/mcp-bridge/serve_test.go`, `internal/observe/metrics.go`, `internal/catalog/schema.go`, `internal/config/`; `README.md`, `docs/logging-contract.md`, `CHANGELOG.md`; disposable `.omp/artifacts/coverage-ticket01.out` (T7 only, removed after capture).
- **Read paths tasks need:** `internal/service/tools.go`, `internal/service/direct_execution.go`, `internal/service/session_exposure.go`, `internal/service/projection.go`, `internal/service/cursor.go`, `internal/service/inbound.go`, `internal/service/reload.go`, `internal/catalog/schema.go`, `internal/config/bounds.go` + `internal/config/config.go`, `internal/core/core.go`, `internal/observe` (logger + metrics), `README.md`, `docs/logging-contract.md`, `CHANGELOG.md`, and the ticket/map/sibling files under `.scratch/token-efficient-tool-responses/` plus `.omp/artifacts/token-efficiency-ticket01-refinement.md`.
- **Allowed effect kinds:** read (source, docs, config, evidence), write (only the write roots above), test (run `go test` / `go build` against the project's own package suites).
- **Forbidden effect kinds:** network (no live upstream or host-service calls in deterministic tests), publish, deploy, schedule (no cron), credentials/secrets handling, infrastructure changes, and any write outside the write roots.
- **Allowed task ids:** T1, T2, T3, T4, T5, T6, T7.
- **Explicit non-goals:** no default field projections (ticket 05); no `full` projection knob; no duplicate-result reuse/deduplication (ticket 06); no ranking/filtering changes; no upstream capability removal; no change to `upstream_defaults.max_response_bytes`; no change to catalog-revision or exposure-admission semantics; no recursive paging.
<!-- specdd:envelope:end -->

## 8. OBSERVABILITY

Applied per the observability-contract skill, scoped to what this change touches (four result surfaces + the new store + the new tool). Existing Loki/Prometheus collection (`internal/observe`) is reused; no new stack.

### Logging (per affected component)
- **Shipping path:** existing `observe.Logger` structured JSON → stdout/stderr (unchanged).
- **Schema:** existing `observe.Entry` envelope (`level`, `event`, `request_id`, `operation`, `duration`, `outcome`, `error`, `fields`) — no new schema; new `event` values reuse it and are added to `docs/logging-contract.md` (owned by T6).
- **Severity policy:** `info` for a bounded decision (`event: result_paged`, `operation: call_tool|search_tools|describe_tool`); `info` for successful retrieval (`event: result_page_served`, `operation: retrieve_result`); `warn` for `result_expired`/`result_evicted`/`invalid_handle` retrievals and for `result_too_large` retention rejection (`event: result_retention_rejected`); never log payload bytes or content, only byte counts and the handle-*existence* outcome class (never the handle value).
- **Actionable events:** `result_paged` (fields: `total_bytes`), `result_page_served` (fields: `page`, `total_pages`), `result_expired`/`result_evicted`/`invalid_handle` (fields: reason class only), `result_retention_rejected` (fields: `total_bytes`, `max_retained_result_bytes`, `retention_reason`), `result_evicted_due_to_bound` (store-side).
- **Required fields:** all entries carry UTC timestamp, `level`, `service`, `environment`, `version`, `event`, `message`, and relevant `request_id`/`operation` — inherited from the existing envelope; no new fields added.

### Prometheus
- **Scrape path:** existing `internal/observe/metrics.go` registry and ops handler (unchanged); the four new collectors are registered there (owned by T2).
- **Metrics to add (names/types/units, bounded labels):**
  - `bridge_result_paged_total` (Counter) — labels `{operation}` (bounded to `call_tool|search_tools|describe_tool`; direct projected counts as `call_tool`).
  - `bridge_result_pages_served_total` (Counter) — labels `{outcome}` (bounded to `success|expired|evicted|invalid_handle|invalid_arguments`).
  - `bridge_result_retained_bytes` (Gauge, bytes) — current retained bytes across the store.
  - `bridge_result_retained_entries` (Gauge, count) — current retained entries.
- **Bounded labels:** only the finite vocabularies above; never session ID, handle, tool name, or any unbounded value as a label.
- **Latency histogram:** not warranted — page slicing is a bounded local read; reuse existing request-duration accounting rather than a new histogram.
- **Throughput/error/saturation coverage:** the counters and gauges above plus the existing `bridge_requests_total` (operation `tools_call`) cover throughput, retrieval error mix, and store saturation.

### Operations and verification
- **Dashboards/alerts:** add the two gauges (`retained_bytes`, `retained_entries`) to the existing ops dashboard; alert when `bridge_result_retained_bytes` approaches the aggregate cap (e.g., >90%) — actionable saturation signal; alert on a rising `result_retention_rejected` count.
- **Verification tests (each tied to AC/T):**
  - Valid JSON log emission: assert the new `result_paged`/`result_expired` entries marshal under the existing envelope and carry no payload content — **AC10, T4/T3**.
  - Metric scraping: expose the registry and assert the new counters/gauges appear with bounded label values — **AC9, T2** (T2 owns `metrics.go`).
  - Log delivery path: reuse the existing logger delivery test with a new event — **T4**.
  - Failure visibility: an expired/evicted/invalid retrieval increments the correct bounded-outcome counter — **AC8, T3**.
  - Alert/dashboard behavior: gauge reflects post-eviction retained bytes; retention-rejection counter increments on `result_too_large` — **AC9, T2**.

**Implementing T IDs:** T1–T5 (instrumentation woven into each slice; metric registration in T2, log-event doc in T6); verification AC IDs as listed.

## 9. RELEASE AND RECOVERY

- **Deployment/migration order:** no data migration; the new store is process-local and empty at start. Deploy order: config (optional `retained_results`/ceiling/code-bytes keys) then binary. Rolling restart is safe because handles are process-local and never persisted; a restart drops all handles and callers receive `invalid_handle` on stale handles — documented, and consistent with the existing session-projection process-local lifetime.
- **Rollback:** revert to the previous binary restores byte-identical unbounded delivery; no persisted state to reconcile.
- **Irreversible steps:** none.
- **Config validation:** an invalid `retained_results` bound (e.g., `max_bytes_per_session > max_bytes_aggregate`), an invalid ceiling pair (e.g., `preview_ceiling_bytes ≥ delivery_ceiling_bytes`), a non-positive `handle_ttl`, an out-of-range `max_upstream_code_bytes`, or a violation of the page-feasibility invariant (`max_metadata_envelope_bytes ≥ delivery_ceiling_bytes`) fails at config load, matching the existing bounds-rejection behavior.

## 10. CRITIC HANDOFF

### Highest-risk assumptions/decisions to review
1. The byte-accounting oracle (§4.4) is the single most load-bearing contract: if it does not match the transport's actual serializer, the ceiling and `total_bytes` are wrong. Review target: T1's oracle test against the SDK's own `MarshalJSON` path (K1 caution).
2. The page partition (metadata-aware maximal fill, now bounded by `max_metadata_envelope_bytes`) must satisfy R3+R4+R5 simultaneously; a reviewer should check the boundary where metadata digit counts change page content sizes and confirm the page-feasibility invariant holds.
3. Handle binding at session-owner granularity and the session-less `result_too_large:no_session` + `invalid_handle` rules (§4.0, §4.3) must not create an existence oracle for other sessions; review T2/T3 for any timing or error-shape distinction.
4. Eviction isolation (aggregate overflow evicts only the creating session) can, under adversarial load, strand a new large result as `result_too_large:capacity`; confirm this is acceptable and documented.
5. README claim change (§6 effects) must be exact: the "complete upstream result" promise narrows only for over-ceiling results, and the qualified-truth/negative-falsehood grep split in T6 proves the old unqualified wording is gone without blocking the qualified truths.

### Missing coverage to target
- The `result_too_large:too_large` path (single result exceeding `max_retained_result_bytes`) — explicit fixture required even though it is unreachable under default config.
- Publisher-owner (session-less) handle binding round-trip — asserts `result_too_large:no_session` on issuance and `invalid_handle` on retrieval, not a handle.
- A live HTTP wire test asserting the exact serialized byte count at the 8192/8193 boundary (not just the in-process oracle).
- `upstream_code` exceeding `max_upstream_code_bytes` — assert the marker is present and the full code is byte-exact in the retained result.

### W09E shared review-risk matrix

| Risk (surface) | Class | Affected requirement | Verification evidence | Counterexample considered |
| --- | --- | --- | --- | --- |
| Handle create→read→expire→evict→retire across bootstrap, ordinary, failure, restart, catalog reload | Lifecycle | R7, R8, R9 | `TestExpiredAndEvictedDistinct` + `TestEvictionIsolationAndConcurrency` + periodic-sweep + owner-retirement purge + reload-independence test; restart drops handles → `invalid_handle` | Session deleted mid-page-read must yield `invalid_handle`, not partial page |
| Store writers: creating handler, eviction (same-session + aggregate), expiry sweep, owner-retirement purge (four seams) | Shared state | R9 | `TestEvictionIsolationAndConcurrency` walks a competing schedule; purge-hook test covers all four seams | Aggregate overflow while creating session has no own handles → `result_too_large:capacity`, never evict a foreign live handle |
| Owner resolution across four surfaces + session-less publisher → `result_too_large:no_session` / `invalid_handle` | Contract | R7, R11 | `TestCeilingAcrossAllSurfaces` + `TestUnretainableResultBranch` + `TestInvalidHandleNoExistenceSignal` + `TestRetrieveResultRequiresExposure` | `call_tool` publisher fallback must never issue a handle; session-less retrieval must be `invalid_handle`, not a rejection |
| Upstream error class vs retention classification; `upstream_code` bounding with marker | Contract | R1, R10 | `TestUnretainableErrorClassPreserved` + `TestErrorClassificationPreserved` + code-bound test | `result_too_large` must never overwrite `upstream_code`; truncation must be marked |
| Preview/page/typed-error shapes; two-phase validation; `additionalProperties:false` schema; README contract | Contract | R1–R8, R10, R11 | `TestPreviewBoundedAndMetadata`, `TestPageMetadataArithmetic`, `TestRetrieveHandlerShapeValidation`, `TestRetrieveResultSchema`, README docs checks | A page whose metadata digits grow must still fit ≤8192; a malformed handle must not reach the store |
| Built-in-surface assertion inventory (exact counts/name sets/exposed_tools) | Contract/ledger | R11, R12 | T3 updates all 11 §3 files; T7 `go test ./...` + `go test ./cmd/mcp-bridge/` green | Any omitted assertion leaves `cmd/mcp-bridge` failing |
| Owner stops/scope/approval provenance; no schema change to existing tools; no `full` knob | Authority | R6, R11 | Design review: effects list in §7 restricts writes; ticket 05 boundary recorded | No `full` field in any new or existing schema |
| Emission→collection→delivery of `result_paged`/`result_expired`/retained gauges | Observability | R10, R9 | §8 verification tests (valid JSON, scrape, failure visibility, saturation gauge) tied to AC/T; T2 owns `metrics.go`, T6 owns `docs/logging-contract.md` | Unbounded label injection (session/handle) must be rejected |
| Root invariant = byte-exact reconstruction + no-existence-signal rejection; adjacent fixtures (error results, UTF-8 boundary, publisher owner) | Repair closure | R4, R5, R7, R10 | `TestPageConcatenationByteExact`, `TestUTF8BoundaryAndEscapingAccounting`, `TestErrorClassificationPreserved`, `TestInvalidHandleNoExistenceSignal` | Error-result pages and UTF-8-crossing pages exercise the same slicing invariant |

### Findings resolution (round 4, R1–R8)

Round-1/2/3 closures are preserved and must not regress. Round-4 findings are closed as follows:

| ID | Class | Root fix | Section |
| --- | --- | --- | --- |
| R1 | ledger-state-drift | coordinator-verified 11-file inventory adopted; `tool_metadata_test.go:108-120`, `tools_publish_atomic_test.go:50-57`, `serve_test.go:618` added to T3 + envelope; presence-only loops excluded | §3, §4.3, §6 T3, §7 |
| R2 | statement-contradiction | `max_upstream_code_bytes` (default 128) with UTF-8-safe truncation + `…` marker; page-feasibility invariant with typed `internal_error` outcome | §4.2, §4.4, §4.5, §9 |
| R3 | statement-contradiction | two-phase validation split (pre-store shape vs post-lookup range) with ordering observable | §4.3, §5 AC15 |
| R4 | ledger-state-drift | `internal/observe/metrics.go` (T2) and `docs/logging-contract.md` (T6) added to write roots/expected files | §6 T2/T6, §7, §8 |
| R5 | ac-not-falsifiable | description text owned by T3 and asserted in AC6 (`TestRetrieveResultSchema`) | §4.3, §6 T3, §5 AC6 |
| R6 | statement-contradiction | unretainable preview specifies `Content` prefix and MCP `isError` mirroring; AC13/AC16 extended | §4.2, §5 AC13/AC16, §6 T4 |
| R7 | statement-contradiction | one purge rule over the four retirement seams (reaper, DELETE, SDK-session-loss, rehydration), owned by T2 | §4.5, §6 T2 |
| R8 | statement-contradiction | T6 split into positive qualified-truth greps + negative unqualified-falsehood greps | §6 T6, §6 README claims |

No finding is factually wrong. R1's 11-file inventory and R7's retirement seams are confirmed against `internal/service` and `cmd/mcp-bridge` sources. Reviewer-adequacy items K1–K10 remain intact. R/AC/T IDs are stable; AC17 was added (not renumbered) and the R↔AC map and task ledger reference it.

## 11. OPEN OWNER DECISIONS

### OD1 — Retrieval surface shape
- **Option A (recommended):** a fourth first-class built-in tool `retrieve_result`. Tradeoff: a new public tool name; cleanly sidesteps `additionalProperties:false` (no existing schema changes), gives the clearest caller-visible contract, and lets its schema stay minimal.
- **Option B:** add an optional `handle`/`page` parameter to `describe_tool` or `call_tool`. Tradeoff: no new tool, but it changes an existing stored schema (violates the "retrieval is its own surface" principle) and overloads `call_tool` semantics.
- **Option C:** an MCP resource or a non-tool JSON-RPC method. Tradeoff: avoids a tool entirely but breaks the existing three-tool surface convention and its annotations/discovery behavior.

### OD2 — Retained-result storage bounds (exact values)
- **Option A (recommended):** `max_entries_per_session: 16`, `max_bytes_per_session: 8 MiB`, `max_bytes_aggregate: 64 MiB`, `max_retained_result_bytes: 8 MiB` (= transport `max_response_bytes` default), `handle_ttl: 1h`. Tradeoff: headroom for the worst-case single result; modest memory; deterministic.
- **Option B:** tighter (`4 entries` / `2 MiB` / `16 MiB`). Tradeoff: lower memory, but a single large upstream result may hit `result_too_large` more often.
- **Option C:** looser (`64 entries` / `32 MiB` / `256 MiB`). Tradeoff: higher memory ceiling; more retained data than the token problem warrants.

### OD3 — Handle expiry policy
- **Option A (recommended):** fixed `handle_ttl` (default 1h) plus purge on session idle-retire/delete and a periodic store sweep. Tradeoff: predictable storage bound; a long-lived session must re-request after TTL (its own cost).
- **Option B:** tie handle lifetime to the session idle lifetime (`session_idle_timeout`) with no independent TTL. Tradeoff: simpler mental model, but a busy session can retain indefinitely and the storage bound becomes the only limit.
- **Option C:** no TTL, eviction-only. Tradeoff: violates the ticket's explicit expiry requirement — rejected.

**Recommendation:** OD1-A, OD2-A, OD3-A. These preserve the ticket's "expiring, session-scoped, bounded" contract with the smallest config surface and the least risk of `result_too_large` under defaults.