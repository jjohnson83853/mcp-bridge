# Bounded bridge evidence

Planning investigation only; no application/test edits, tests, runtime probe, implementation authorization or clean critic pass.

## Observed source facts

- B2/C2: `internal/service/tools.go:112-182` prepares and validates descriptors before mutation; `:174-180` then calls `RemoveTools` and `AddTool` separately. `t.mu` protects bridge reconciliation/maps (`:119-120`); no batch SDK registry replacement appears in this inspected method. This proves prevalidation, not atomic tools/list/route/generation publication. Do not weaken the acceptance contract or claim the existing seam meets it.
- Direct dispatch separately locks `pinnedTools` then reads `runtime.Current()` (`internal/service/tools.go:209-234`), with fallback to assembly pins. This is not evidence that registry and assembly share a single atomic accepted-state pointer.
- B4/C4: `cmd/mcp-bridge/serve.go:112-118` constructs one MCP server and registers tool/prompt surfaces. `:138-155` passes that same server through the streamable/inbound handler; refresh callback `:201-205` invokes both surfaces. Shared runtime/server ownership is supported by these construction anchors; no per-client registry proposal is necessary. Session teardown behavior was not exercised.
- C6: `internal/config/tool_metadata.go:134-169` requires annotations to be a mapping, annotation title to be string, and every supported hint to be boolean; null containers/null hints/wrong types reject at this parse-time boundary. `_meta` requires a mapping (`:170-177`). Nested arbitrary `_meta` values are not annotation hints. `:36-59` lossless conversion comments do not override this validation.
- SDK conversion: `internal/service/tool_metadata.go:21-55` validates metadata, creates annotation object only when normalized annotation bytes are present, and unmarshals into `mcp.ToolAnnotations`. Remaining merge/pointer/source-normalization details were not independently checked in this packet.
- Existing exact override boundary is pinned-only: `internal/config/tool_metadata.go:61-82`. Owner-selected full parity requires an explicit approved amendment, not an accidental bypass.

## Semantic evidence and tools

Bounded mounted `codebase-searcher` search/read_range, project `mcp-bridge`; internal/service/tools.go, internal/service/tool_metadata.go, internal/config/tool_metadata.go and cmd/mcp-bridge/serve.go only. LSP references for `ToolSurface.Published` resolved production call at serve.go:203 and existing tests; LSP hover identified `(*mcp.Server).AddTool` in go-sdk v1.8.0 as single-name add/replace, not batch publication. SDK documentation alone does not prove an alternative atomic API absent. Graphify skipped: dirty working tree. No vendor source inspection or unrelated project access.

## Remaining blockers

B2/C2 needs planner-owned feasible atomic publication design against actual SDK/read-dispatch seams; current separate registry operations are insufficient evidence. B3/C3 needs the actual installed Gemini CLI executable/bundle identity and loaded re-list/cache/invocation/approval implementation; prior OMP strings are not Gemini evidence. No supported installation path has been established in this session; do not substitute website/OMP or perform unbounded filesystem searches. B4/C4 is source-supported but not runtime-exercised. C6 parse-time null/type rules are source-supported; merge/SDK normalization completeness remains unverified. C1 is reconciled to explicit owner choice, C5 full canonical restoration complete, independent re-review still required. All implementation and three checkpoint approvals remain absent.
