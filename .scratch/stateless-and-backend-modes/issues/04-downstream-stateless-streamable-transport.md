# 04 — Downstream Stateless Streamable Transport & Multi-Version Protocol Negotiation

**What to build:** An external AI client (such as ChatGPT Developer Mode) connecting via Streamable HTTP can probe `POST /mcp` (`server/discover`) proposing protocol version `2026-07-28` and receive a `200 OK` response with `2026-07-28` advertised in `supportedVersions`. The transport serves authenticated tool calls statelessly with standard Bearer authorization headers without requiring `Mcp-Session-Id` headers. An environment variable (`MCP_STATELESS=true`) controls downstream stateless transport while keeping existing stateful integration tests green.

**Blocked by:** None — can start immediately.

**Status:** completed

- [X] Variadic options are supported on the service-layer streamable handler constructor (`NewStreamableHandler(server, opts ...*mcp.StreamableHTTPOptions)`), passing options to `mcp.NewStreamableHTTPHandler` while preserving 100% backward compatibility for callers passing no options.
- [X] In `cmd/mcp-bridge/serve.go`, downstream transport initializes with `Stateless: true` when `MCP_STATELESS=true` (`strings.EqualFold(os.Getenv("MCP_STATELESS"), "true")`), preserving stateful session tests by default while fully supporting stateless mode.
- [X] Probing `server/discover` under stateless transport advertises `"2026-07-28"` alongside all registered legacy versions (`2025-11-25`, `2025-06-18`, `2024-11-05`) in `supportedVersions`.
- [X] Unit and wire tests in `internal/service/initialize_wire_test.go` assert discovery advertising `"2026-07-28"` in stateless mode and omitting it in stateful mode.
