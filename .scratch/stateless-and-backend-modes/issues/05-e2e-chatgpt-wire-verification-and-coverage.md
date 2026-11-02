# 05 — End-to-End ChatGPT Wire Verification Across Mixed Stubs & Coverage Gate

**What to build:** Complete end-to-end loopback wire verification: ChatGPT Developer Mode connects to the bridge, performs protocol discovery, lists tools, and executes tool calls across a mixed catalog of default stateless stubs and stateful stubs with Bearer authentication. Verifies full workspace test suite (`go test -race ./...`, `go vet ./...`) and passes the `>= 90.0%` test coverage gate.

**Blocked by:** 01 — Upstream Server Configuration Schema & Fingerprinting (Default Stateless Stubs), 02 — Upstream Dedicated Runtime Stateless Execution & Pooled Keep-Alive, 03 — Upstream Adaptive Ephemeral Handshake for State-Enforcing Stubs, 04 — Downstream Stateless Streamable Transport & Multi-Version Protocol Negotiation.

**Status:** completed

- [X] Loopback wire test simulating ChatGPT discovery burst (`server/discover` with `2026-07-28`, `tools/list`, and `tools/call`) with Bearer token against a bridge aggregating both default stateless stubs and stateful stubs succeeds with `200 OK`.
- [X] Existing stateful session admission wire tests (`TestChatGPTOAuthAndActionDiscoveryWire`) continue to pass.
- [X] Full workspace test verification gate (`go test -race ./...`, `go vet ./...`, `covcheck >= 90%`) passes.
