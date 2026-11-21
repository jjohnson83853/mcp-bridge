# 03 — `resources/read` forwarding and content fidelity

**Type:** task

**What to build:** A client reads discovered content through `resources/read` by bridge URI; the bridge decodes the owning server, forwards the exact raw URI upstream, and returns upstream contents and errors without inventing or leaking cross-server detail.

**Blocked by:** 01.

**Status:** ready-for-agent

- [ ] Only bridge-issued URIs are accepted. A raw upstream URI, an unknown server id, a malformed namespace, or a URI from a server that is no longer configured is rejected with a clear MCP error and is never forwarded upstream.
- [ ] The exact upstream raw URI is forwarded byte-for-byte; the response preserves the contents array including text blocks, blob blocks, and per-block `mimeType`, plus response-level fields the upstream provided.
- [ ] Upstream error responses (not found, access denied, protocol error) map to MCP errors with upstream semantics intact and no other server's identity or content in the message.
- [ ] Response size is bounded and an oversized read fails deterministically without truncating silently.
- [ ] Authentication, admission, and session semantics match `tools/call`: an unauthenticated or unadmitted client is rejected before any upstream contact.
- [ ] Tests cover text content, blob content, not-found, foreign URI rejection, and unreachable server.