# 02 — Upstream Dedicated Runtime Stateless Execution & Pooled Keep-Alive

**What to build:** Upstream tool discovery and tool calls execute statelessly over pooled HTTP keep-alive connections without pinning or caching long-lived MCP session handles. Upstream container restarts or network drops no longer cause tool calls to fail with stale session errors. When an upstream stub is configured as `stateless: false`, the runtime retains its persistent session across calls.

**Blocked by:** 01 — Upstream Server Configuration Schema & Fingerprinting (Default Stateless Stubs).

**Status:** completed

- [X] Upstream `Dedicated` execution facade accepts the server's stateless configuration setting upon construction.
- [X] When `stateless: true` (default), tool discovery (`Discover`) and tool calls (`CallTool`) execute ephemerally over pooled HTTP keep-alive transport (`http.Transport`) without retaining or caching `*mcp.ClientSession` across calls.
- [X] In stateless mode, tool calls succeed cleanly before and after a simulated upstream server restart, with zero session invalidation errors.
- [X] When `stateless: false`, persistent session caching across calls is preserved.
- [X] Toggling a server from stateful to stateless on reload immediately evicts the persistent session; in-flight calls on the evicted session fail fast with HTTP 503 (`backend reloading`).
- [X] Unit and mock server tests in `internal/upstream/dedicated_test.go` assert stateless execution, restart resilience, and stateful session preservation.
