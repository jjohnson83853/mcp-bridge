# Feature Map: Configurable Stateless/Stateful Backend Modes and ChatGPT Downstream Transport (`stateless-and-backend-modes`)

- **Effort:** `stateless-and-backend-modes`
- **Owner:** Coordinator
- **Status:** completed
- **Revision:** r1

## Summary
Provide configurable stateless vs stateful execution modes per upstream MCP server backend (with default being stateless), while defaulting the downstream bridge transport to stateless mode so that ChatGPT Developer Mode action discovery succeeds out-of-the-box (MCP 2.0 / `2026-07-28` support).

## Tickets
1. `issues/01-upstream-server-config-stateless-stubs.md` (T1) — Upstream Server Configuration Schema & Fingerprinting (Default Stateless Stubs). Schema parsing in `internal/config/config.go` and fingerprinting in `internal/config/fingerprint.go`.
2. `issues/02-upstream-runtime-stateless-execution.md` (T2) — Upstream Dedicated Runtime Stateless Execution & Pooled Keep-Alive. Ephemeral execution and restart resilience in `internal/upstream/dedicated.go`.
3. `issues/03-upstream-adaptive-ephemeral-handshake.md` (T3) — Upstream Adaptive Ephemeral Handshake for State-Enforcing Stubs. Discovery probe, transient handshake execution, and structured logging in `internal/upstream/dedicated.go`.
4. `issues/04-downstream-stateless-streamable-transport.md` (T4) — Downstream Stateless Streamable Transport & Multi-Version Protocol Negotiation. Service options, `cmd/mcp-bridge/serve.go` stateless flag, and `2026-07-28` discovery test in `internal/service/initialize_wire_test.go`.
5. `issues/05-e2e-chatgpt-wire-verification-and-coverage.md` (T5) — End-to-End ChatGPT Wire Verification Across Mixed Stubs & Coverage Gate. Full loopback HTTP wire test in `cmd/mcp-bridge/serve_oauth_test.go` and workspace coverage gate (>= 90%).
## Decisions So Far
- `D1`: Upstream servers in `bridge.yaml` default to `stateless: true` when omitted. If explicitly set to `stateless: false`, the upstream client maintains a persistent `*mcp.ClientSession`.
- `D2`: In upstream stateless mode, `CallTool` and `Discover` execute ephemerally, ensuring resilience against upstream container restarts without leaking session IDs or stale transport handles.
- `D3`: Downstream bridge transport defaults to `Stateless: true`, advertising `2026-07-28` to ChatGPT and handling authenticated tool calls statelessly with standard Bearer tokens.
- `D4`: `MCP_STATELESS=false` allows reverting downstream transport to stateful mode if required by legacy clients.
- `D5`: Upstream stateless transport reuses TCP/TLS keep-alive connections via pooled `http.Transport` without caching `mcp.ClientSession`.
- `D6`: Upstream adaptive discovery probe flags backends needing an `initialize` handshake, executing a transparent ephemeral handshake with structured JSON logging (`upstream_ephemeral_handshake`).
- `D7`: Downstream stateless transport provides full multi-version support (advertising `2026-07-28` alongside all registered legacy versions).
- `D8`: Immediate eviction on hot-reload: in-flight requests on an evicted persistent session fail fast with HTTP 503 (`backend reloading`).
