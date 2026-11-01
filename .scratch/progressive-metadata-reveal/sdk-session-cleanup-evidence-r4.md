# Progressive metadata reveal — SDK session cleanup evidence r4

## Scope

Bounded read-only supplement. Go SDK v1.8.0 module-cache source was located through LSP and read at the exact ranges below. Bridge ownership/expiry facts are reused from `.scratch/progressive-metadata-reveal/session-writer-evidence.md:17-24`; no source/tests/workflow state/approvals changed. No runtime or client probe ran.

## Verified SDK facts

- `mcp/streamable.go:601-615`: stateful DELETE validates the session ID, calls `lookupSession`, then invokes `sessInfo.session.Close()` before writing HTTP 204.
- `mcp/server.go:870-879`: public `Server.Sessions()` clones the current session slice under the server mutex and returns that snapshot. Its docs explicitly do not guarantee that concurrent additions/removals are included.
- `mcp/server.go:2131-2164`: public `ServerSession.Close()` is documented idempotent and concurrency-safe; it cancels active listen requests, closes the connection, and invokes the close callback once. Its docs state graceful close waits for ongoing requests to return.
- SDK internal `StreamableHTTPHandler.onTransportDeletion` remains private (`mcp/streamable.go:686-697`, from the existing SDK evidence packet); no public callback for the transport's session-map removal was identified.

## Existing bridge boundary and consequence

The bridge currently removes its inbound session record after delegated DELETE and in lazy expiry; those records contain no SDK server/session pointer, and these paths do not call SDK close (`session-writer-evidence.md:18-20`). SDK DELETE itself closes the SDK session before returning. A future owner-specific cleanup can use public `Server.Sessions()` / `ServerSession.Close()` as candidate APIs, but enumeration is a snapshot and `Close()` can wait on in-flight requests. Therefore cleanup must retire the bridge exposure owner without holding inbound/publication locks across session close, and must handle concurrent close/removal without resurrecting exposure.

**B4 remains open.** These public APIs narrow the cleanup path; they do not prove authenticated owner association, safe lock order, prompt/resource coexistence, cleanup behavior at lazy expiry, or absence of retained references after close. No infeasibility verdict is established.