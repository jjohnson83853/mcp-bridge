# Spec: Configurable Stateless/Stateful Backend Modes and ChatGPT Streamable Transport

**Status:** completed

## Problem Statement

When connecting ChatGPT Developer Mode to the MCP bridge as a custom tool connector, ChatGPT fails during action discovery with the error: "Authentication succeeded, action discovery failed".

This failure occurs because of how the bridge manages protocol versions and session state on both ends of the bridge:

1. **Downstream Transport:** ChatGPT Developer Mode uses the MCP 2.0 draft protocol (`2026-07-28`). When ChatGPT probes the bridge with `server/discover`, the underlying Go MCP SDK inspects whether the Streamable HTTP transport is stateless. Because the bridge operates in stateful mode by default, the SDK actively removes `2026-07-28` from the advertised supported versions. When ChatGPT receives a discovery response omitting its requested protocol version, OpenAI's connector service aborts discovery with HTTP 424.
2. **Upstream Backend Sessions:** Upstream MCP servers aggregated behind the bridge currently maintain a single persistent stateful session cached across the entire bridge process lifetime. If an upstream container or microservice restarts or drops connections, its cached session ID becomes invalid, causing subsequent tool calls to fail with backend unavailable errors until the bridge itself is restarted. Furthermore, many modern upstream MCP servers are designed to be purely stateless.

The operator needs downstream connections (including ChatGPT) to default to stateless transport so action discovery succeeds immediately, while allowing upstream backend servers to be independently configurable as either stateless (the new default) or stateful (for backends that require persistent session state).

## Solution

1. **Downstream Transport (Bridge ↔ ChatGPT / Clients):**
   - The bridge's public MCP Streamable HTTP transport defaults to stateless mode.
   - In stateless mode, the bridge advertises protocol version `2026-07-28` alongside legacy versions (`2025-11-25`, `2025-06-18`, etc.), satisfying ChatGPT Developer Mode's action discovery probe.
   - Incoming requests carrying standard OAuth Bearer tokens execute statelessly without requiring or pinning `Mcp-Session-Id` headers.
   - A downstream environment flag permits operators to opt out and revert to stateful transport if a legacy client specifically requires it.

2. **Upstream Transport (Bridge ↔ Upstream MCP Server Backends):**
   - Each upstream server defined in the bridge configuration supports a `stateless` configuration flag that defaults to `true`.
   - When an upstream backend is stateless (default), the bridge executes tool discovery and tool invocations ephemerally without pinning a long-lived session handle. This makes upstream communication resilient against container restarts, dynamic IP changes, and upstream restarts.
   - When an upstream backend is explicitly configured with `stateless: false`, the bridge maintains a persistent session across the process lifetime, accommodating legacy stateful backends that require session continuity.

<!-- specdd:requirements:start -->
## User Stories & Requirements

### Requirements
- **R1 — Upstream Server Backend Stateless Mode Configuration**: Upstream servers defined in configuration support `stateless: true|false` defaulting to `true` when omitted.
- **R2 — Upstream Execution Lifecycle in Dedicated Runtime**: Upstream runtime executes tool calls and discovery statelessly using pooled HTTP keep-alive transport; executes transient ephemeral handshakes when required by upstream servers with structured logging.
- **R3 — Downstream Streamable Transport Default & Protocol Negotiation**: Downstream bridge transport defaults to `Stateless: true`, advertising `2026-07-28` to ChatGPT and handling authenticated tool calls statelessly with standard Bearer tokens.
- **R4 — Integrated ChatGPT Wire Verification & Coverage Gate**: Comprehensive wire tests verify action discovery, tool list, and tool calls across stateless and stateful backends, meeting the >= 90% coverage gate.

### User Stories
1. As an AI platform operator connecting ChatGPT to the MCP bridge, I want ChatGPT's action discovery probe (`server/discover`) to succeed without error, so that ChatGPT can discover and invoke the bridge's catalog of tools.
2. As an AI platform operator, I want incoming tool requests from ChatGPT to be authorized via standard OAuth Bearer tokens on each HTTP request, so that I don't suffer from session-state synchronization issues or lost session headers.
3. As an AI platform operator, I want the bridge to default to stateless operation across all upstream backends, so that upstream container restarts do not leave behind dead session IDs that break subsequent tool calls.
4. As an AI platform operator, I want to explicitly configure a specific upstream backend as stateful (`stateless: false`) in the bridge configuration, so that backends requiring persistent session lifecycle or subscriptions continue to work seamlessly.
5. As an AI platform operator, I want upstream backends configured without an explicit `stateless` field to automatically default to stateless execution, so that existing configuration files benefit from restart resilience without manual editing.
6. As an AI platform operator, I want configuration changes to a backend's `stateless` setting to invalidate the configuration fingerprint and trigger a clean hot-reload, so that I can switch a backend's mode without restarting the bridge.
7. As an AI platform operator, I want downstream transport to allow opting out of stateless mode via an environment variable (`MCP_STATELESS=false`), so that legacy clients that strictly depend on stateful session admission remain supported.
8. As a client interacting with a stateless upstream server, I want each tool call to execute cleanly regardless of how long the bridge has been running, so that idle timeouts do not cause subsequent calls to fail.
9. As a developer inspecting bridge logs, I want structured HTTP and tool execution diagnostic events to record the exact execution outcome and protocol version used, so that I have clear operational observability.
10. As a security engineer, I want stateless request evaluation to enforce the same strict OAuth token validation, issuer checks, and required scope checks as stateful requests, so that security posture is never weakened.
<!-- specdd:requirements:end -->
<!-- specdd:design:start -->
## Implementation Decisions

- **Downstream Transport Handler Variadic Options:**
  The streamable HTTP handler constructor in the service layer will be extended with optional streamable HTTP options. Callers passing no options retain backward-compatible behavior. The server entrypoint will pass `Stateless: true` by default unless explicitly disabled via environment configuration.
- **Protocol Version Advertisement:**
  Enabling stateless mode on the downstream transport allows the Go MCP SDK's version supporter to retain and advertise protocol version `2026-07-28` alongside all registered legacy versions (`2025-11-25`, `2025-06-18`, `2024-11-05`), serving clients statelessly while satisfying ChatGPT Developer Mode's action discovery probe.
- **Server Configuration Schema Extension:**
  The server configuration definition will add an optional boolean pointer field for `stateless`. If omitted or `nil`, the configuration query method returns `true` as the default. If explicitly provided as `false`, it returns `false`.
- **Configuration Fingerprint Inclusion:**
  The server projection used for configuration fingerprinting and hot-reload detection will include the effective stateless state of each server. Toggling this setting in configuration triggers an orderly reload of the upstream runtime.
- **Upstream Dedicated Runtime Dual-Mode Execution:**
  The upstream execution facade (`Dedicated`) will accept the server's stateless configuration setting upon construction:
  - When `stateless` is `true` (default):
    - Transport uses pooled HTTP keep-alive connections (`http.Transport`) to eliminate connection setup overhead while omitting `Mcp-Session-Id` and avoiding persistent session caching.
    - Adaptive discovery probe: during startup and reload catalog discovery, the bridge probes whether the upstream server accepts uninitialized calls or requires an `initialize` handshake. If initialization is required, the runtime flags the backend as requiring an ephemeral handshake.
    - Transparent Ephemeral Handshake: for backends requiring initialization, each tool call executes a transient `initialize` -> `tools/call` sequence within the caller's request context, disposing of the session immediately upon completion.
    - Ephemeral Handshake Observability: emits an `info`-level structured JSON log with `event: "upstream_ephemeral_handshake"`, `backend_id`, `duration_ms`, and `protocol_version`.
  - When `stateless` is `false`:
    - The runtime lazily establishes a persistent session and reuses it across calls for the lifetime of the runtime.
- **Hot-Reload Eviction & In-Flight Request Handling:**
  Toggling a backend from `stateless: false` to `stateless: true` on configuration reload immediately evicts the persistent session. Any in-flight call using the evicted session fails fast with HTTP 503 / JSON-RPC `-32000` (`backend reloading`).
- **OAuth and Session Admission Compatibility:**
  The existing session-bound OAuth admission logic in the authentication middleware remains fully intact for legacy stateful clients. In stateless mode, incoming requests carry standard Bearer authorization headers and are verified directly against the active JWKS key set and required scopes.
<!-- specdd:design:end -->

<!-- specdd:tasks:start -->
## Tasks Ledger

1. **T1 — Upstream Server Configuration Schema & Fingerprinting (Default Stateless Stubs)** (`issues/01-upstream-server-config-stateless-stubs.md`)
   - Schema updates in `internal/config/config.go` (`ServerStateless()` defaulting to `true`), and fingerprinting projection in `internal/config/fingerprint.go`.
   - Targets: R1.
2. **T2 — Upstream Dedicated Runtime Stateless Execution & Pooled Keep-Alive** (`issues/02-upstream-runtime-stateless-execution.md`)
   - Dedicated runtime dual-mode execution in `internal/upstream/dedicated.go`, pooled `http.Transport` keep-alive connections, restart resilience, and immediate session eviction on reload.
   - Targets: R1, R2.
3. **T3 — Upstream Adaptive Ephemeral Handshake for State-Enforcing Stubs** (`issues/03-upstream-adaptive-ephemeral-handshake.md`)
   - Adaptive discovery probe to flag backends requiring `initialize`, transient handshake execution, and structured JSON logging (`upstream_ephemeral_handshake`).
   - Targets: R2.
4. **T4 — Downstream Stateless Streamable Transport & Multi-Version Protocol Negotiation** (`issues/04-downstream-stateless-streamable-transport.md`)
   - Service constructor options in `internal/service/server.go`, `MCP_STATELESS=true` support in `cmd/mcp-bridge/serve.go`, and discovery test in `internal/service/initialize_wire_test.go` advertising `2026-07-28` to ChatGPT.
   - Targets: R3.
5. **T5 — End-to-End ChatGPT Wire Verification Across Mixed Stubs & Coverage Gate** (`issues/05-e2e-chatgpt-wire-verification-and-coverage.md`)
   - Full loopback HTTP wire test with ChatGPT discovery probe (`2026-07-28`), tool calls across default stateless stubs and stateful stubs, and workspace test coverage gate (>= 90%).
   - Targets: R4.
<!-- specdd:tasks:end -->

<!-- specdd:envelope:start -->
Allowed Envelope:
- `internal/config/config.go`
- `internal/config/fingerprint.go`
- `internal/config/config_test.go`
- `internal/config/fingerprint_test.go`
- `internal/upstream/dedicated.go`
- `internal/upstream/dedicated_test.go`
- `internal/service/server.go`
- `cmd/mcp-bridge/serve.go`
- `internal/service/initialize_wire_test.go`
- `cmd/mcp-bridge/serve_oauth_test.go`
- `coverage.out`
<!-- specdd:envelope:end -->
## Testing Decisions

- **What makes a good test:**
  Tests must exercise observable external behavior at protocol boundaries (HTTP request/response wire interactions, JSON-RPC envelopes, and configuration parsing outcomes), rather than internal struct fields or private state variables.
- **Tested Modules:**
  1. *Configuration Module:* Test that YAML documents with omitted `stateless` default to `true`, explicit `stateless: false` is honored, and the configuration fingerprint changes when the mode is modified.
  2. *Upstream Runtime Module:* Test that a stateless upstream backend executes tool calls successfully, survives simulated backend restarts without error, and does not hold open persistent session handles; test that a stateful backend preserves its persistent session.
  3. *Downstream Service Module:* Test that a `server/discover` probe with protocol version `2026-07-28` returns HTTP 200 with `supportedVersions` containing `2026-07-28` under default stateless transport.
  4. *Integrated Server Wire Loopback:* Full loopback HTTP wire test running the composed bridge with both a stateless and a stateful upstream backend, verifying that a simulated ChatGPT client can discover the service, list tools, and invoke tools on both backends using Bearer authorization.
- **Prior Art in Codebase:**
  - `internal/config/config_test.go` and `internal/config/fingerprint_test.go` for schema validation and fingerprinting.
  - `internal/upstream/dedicated_test.go` and `runtime_test.go` for upstream client and tool call execution.
  - `cmd/mcp-bridge/serve_oauth_test.go` (`TestChatGPTOAuthAndActionDiscoveryWire`) for full HTTP wire testing with simulated OAuth tokens and MCP JSON-RPC payloads.

## Out of Scope

- Client-side SSE (Server-Sent Events) streaming transport changes: the bridge continues to support Streamable HTTP.
- Reverse server-to-client sampling or roots requests: the bridge remains a forward proxy and gateway.
- Changes to PocketID or external OAuth identity provider configuration.

## Further Notes

- The default of stateless mode on both downstream and upstream aligns with the architectural direction of the Model Context Protocol (SEP-2575), eliminating session store exhaustion, container restart state invalidation, and tokenless session hijacking vectors.
