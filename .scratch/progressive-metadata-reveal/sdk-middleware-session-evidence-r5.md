# Progressive metadata reveal — session-aware middleware evidence r5

## Scope

Bounded read-only Go SDK v1.8.0 source check. Exact module-cache source was read at `mcp/server.go:1871-1879,1931-1935` and `mcp/shared.go:112-137,196-224,313-325,606-627`. No code, tests, workflow state or approvals changed; no runtime probe ran.

## Verified facts

- `serverMethodInfos` maps both `tools/list` and `tools/call` to the SDK's `Server.listTools` and `Server.callTool` handlers (`mcp/server.go:1878-1879`).
- `handleReceive` validates the method and unmarshals parameters, creates the typed `Request`, obtains the session's current receiving `MethodHandler`, and invokes it before returning the result (`mcp/shared.go:206-224`). The session handler is read from the owning server under its mutex (`mcp/server.go:1931-1935`).
- `Request.GetSession()` is public; `Session.ID()` returns the MCP session ID (`mcp/shared.go:117-127,606-614`). For server methods, `newServerMethodInfo` constructs `ServerRequest` with the actual `*ServerSession` (`:313-325`); `ServerSession.ID()` returns the transport session ID (`mcp/server.go:1606-1610`, previously verified in `sdk-evidence-r3.md`).
- The public `Middleware` wraps a generic `MethodHandler` (`mcp/shared.go:112-137`). Thus a receiving middleware can distinguish an accepted session's `tools/list` and `tools/call` requests before those SDK handlers execute. This is a candidate seam, not existing bridge behavior.

## Design consequence (inference)

Per-session visibility does not inherently require one SDK `Server` instance per session: a middleware over the current shared server could filter `tools/list` results and gate direct `tools/call` names using `Request.GetSession().ID()`, provided publication, list results and route/binding lookup share a generation barrier. Pagination semantics and global SDK registry mutation/notification effects still need design and behavioral proof.

This does **not** close B4's first-request race. The existing inbound auth/session gate runs before SDK middleware, while its issued-session record is added after the delegated initialize handler returns (`session-writer-evidence.md:18-20`). Therefore the middleware cannot rescue a follow-up rejected by inbound admission before reaching it. Early issued-ID registration or a proven protocol/lifecycle guarantee remains necessary. It also does not prove expiry cleanup or lock ordering.

## Remaining limits

- B2 remains open: middleware coverage is verified at the SDK method-dispatch seam, but no bridge publication transaction coordinates accepted runtime, registry mutations, route bindings, cursor generation, session exposure, and notifications.
- B4 is narrowed: session identity is available after the inbound gate; per-session server selection is not inherently required. Early inbound ID association and safe owner retirement remain open.
- No tests, builds, race checks, wire probes, or Gemini/client checks were run.