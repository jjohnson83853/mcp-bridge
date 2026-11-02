# 03 — Upstream Adaptive Ephemeral Handshake for State-Enforcing Stubs

**What to build:** Transparent compatibility for upstream stubs that reject direct uninitialized tool calls. The bridge adaptively detects whether an upstream stub requires an initialization handshake during catalog discovery. When required, the runtime executes a transient `initialize` -> `tools/call` sequence within the request context and emits structured JSON observability logs (`upstream_ephemeral_handshake`).

**Blocked by:** 02 — Upstream Dedicated Runtime Stateless Execution & Pooled Keep-Alive.

**Status:** completed

- [X] Catalog discovery probes whether an upstream stub accepts direct calls or requires an `initialize` handshake, flagging backends that require initialization.
- [X] For flagged backends, tool invocations execute a transient `initialize` -> `tools/call` sequence within the caller's request context, disposing of the session immediately upon completion.
- [X] Tool execution emits an `info`-level structured JSON log with `event: "upstream_ephemeral_handshake"`, `backend_id`, `duration_ms`, and `protocol_version`.
- [X] Unit tests in `internal/upstream/dedicated_test.go` assert ephemeral handshake execution and log emission against a state-enforcing mock upstream server.
