Status: ready-for-agent
Revision: HYBRID-DISCOVERY-r3 (2026-10-19)

## Problem Statement

When an AI client (such as Perplexity, Claude, or LibreChat) connects to the MCP bridge, the bridge advertises only three meta-tools: `search_tools`, `describe_tool`, and `call_tool`. Even when an operator configures `pins` in `config.yaml` for high-frequency tools (such as `wiki_search_notes` from `second-brain` or `search_documents` and `query_context` from `rag`), those tools are not present in the MCP `tools/list` advertisement.

Because the tools are absent from `tools/list`:
1. The AI client cannot call them directly upon being prompted (e.g. "search rag for a lease").
2. The client is forced into a multi-turn discovery loop (`search_tools` -> `describe_tool` -> `call_tool`), which adds 1.5s to 3s of latency per interaction.
3. Client models frequently hallucinate or fail when instructed to use a specific tool name, because their prompt-level tool catalog does not include that tool.
4. Operators who define `pins` in the configuration expect those tools to be preloaded, but the current system only uses pins as a startup validation check and a search ranking flag.

## Solution

Implement Composio-style hybrid tool discovery in the MCP bridge. The bridge will eagerly preload selected pinned tools directly into the MCP `tools/list` response with their complete, validated input schemas, while keeping the rest of the upstream tool catalog discoverable on demand via `search_tools` and executable via `call_tool`.

From the user's and client's perspective:
1. When connecting to the bridge, the client's `tools/list` call returns both the meta-tools (`search_tools`, `describe_tool`, `call_tool`) and the preloaded pinned tools with descriptive names and complete schemas.
2. The client can invoke preloaded tools directly in a single turn without intermediate discovery hops or envelope wrapping.
3. The bridge validates arguments against the tool's cached schema, dispatches directly to the owning upstream backend, and returns standard MCP tool call results with provenance metadata.
4. Long-tail, unpinned tools remain fully discoverable through search and callable through `call_tool`.
5. Configuration reloads (such as SIGHUP) update the preloaded tool set atomically without dropping active connections, and broadcast tool-change notifications to clients that support them.

## Contract Amendments

1. **Tool Name Schema Pattern Amendment**:
   The input schemas for `catalog.CallToolInputSchema()` and `catalog.DescribeToolInputSchema()` currently restrict `tool_name` to the opaque base32 hash format `^t_[a-z2-7]{52}$`. This specification formally amends that contract to a single pattern string with regular expression alternation supporting both the opaque hash and the preloaded namespaced wire format:
   ```regex
   ^(t_[a-z2-7]{52}|[a-z][a-z0-9-]{0,31}__[a-zA-Z0-9_-]{1,94})$
   ```
2. **`tools/list` Exact Count Assertion Amendment**:
   The existing integration test assertion that `tools/list` returns *exactly* 3 tools is amended to assert `3 + len(active_pins)`, where `active_pins` is defined as the set of successfully resolved preloaded tools in the active generation.

## User Stories

1. As an operator, I want to designate specific upstream tools in `config.yaml` to be preloaded, so that high-priority tools are immediately available to connected AI clients.
2. As an operator, I want preloaded tools to be validated against upstream server catalogs at bridge startup, so that misconfigured tool names fail closed before traffic is accepted.
3. As an operator, I want to pin tools from multiple different upstream servers, so that I can assemble a unified hot-path tool set across heterogeneous backends.
4. As an operator, I want preloaded tool names to follow a deterministic namespacing convention, so that tool name collisions between different upstream servers are impossible.
5. As an operator, I want preloaded tool configurations to enforce a strict upper bound of at most 64 tools, so that the client's context window cannot be overwhelmed.
6. As an operator, I want preloaded tool configurations to enforce a maximum schema byte size of 64 KiB per tool, so that malformed or massive upstream schemas do not bloat discovery payloads.
7. As an AI client, I want `tools/list` to include preloaded tools alongside the bridge meta-tools, so that I know what hot-path capabilities exist without running a search query.
8. As an AI client, I want each preloaded tool in `tools/list` to carry its full `inputSchema`, so that I can generate valid invocation arguments on the first turn.
9. As an AI client, I want each preloaded tool to include a clear description indicating its capability and upstream origin, so that I choose the correct tool for the user prompt.
10. As an AI client, I want to invoke a preloaded tool directly by its advertised name, so that I do not need to wrap execution inside a generic `call_tool` envelope.
11. As an AI client, I want argument validation errors on preloaded tools to return descriptive MCP errors with `isError: true`, so that I can correct my arguments in a follow-up turn.
12. As an AI client, I want preloaded tool execution results to carry the upstream's exact content payload, so that tool execution fidelity is identical to direct upstream communication.
13. As an AI client, I want execution failures from upstream servers to be cleanly reported as MCP tool errors, so that I can recover or inform the user.
14. As an AI client, I want to use `search_tools` to find tools that are not preloaded, so that I retain access to the entire long-tail catalog.
15. As an AI client, I want `search_tools` to indicate which tools are already preloaded, so that I do not attempt redundant discovery or schema lookups.
16. As an AI client, I want `describe_tool` to remain available for on-demand tools, so that I can inspect schemas for tools discovered through search.
17. As an AI client, I want `call_tool` to continue working for all valid upstream tools (both preloaded and on-demand), so that existing client workflows and generic tool invocation agents do not break.
18. As an operator, I want preloaded tool calls to inherit upstream timeout and circuit breaker defaults, so that hanging upstreams cannot stall the bridge or client indefinitely.
19. As an operator, I want preloaded tool calls to respect per-server concurrency limits, so that hot-path tool calls do not overwhelm fragile backend services.
20. As an operator, I want preloaded tool calls to emit structured diagnostic logs, so that I can monitor latency, status, and failures with the same observability as meta-tool calls.
21. As an operator, I want credentials for upstream servers to be injected securely into preloaded calls without ever exposing them to the client or in log output, so that zero-trust boundaries are maintained.
22. As an operator, I want to trigger a configuration reload (SIGHUP) that adds or removes preloaded tools, so that I can adjust the hot-path tool set without restarting the bridge process.
23. As an AI client supporting MCP list change notifications, I want to receive `notifications/tools/list_changed` when preloaded tools are modified during a reload, so that I can refresh my active tool definitions.
24. As an AI client that does not support list change notifications, I want `tools/list` on my next turn to reflect the current generation's preloaded tools, so that state changes are eventually consistent.
25. As an operator, I want bridge startup to fail closed if a configured pin references an unknown tool, but runtime SIGHUP reloads to preserve Last-Known-Good (LKG) catalog state if an already-pinned tool's upstream is temporarily unreachable, so that temporary backend outages do not block configuration reload.
26. As an operator, I want the public configuration fingerprint to change whenever the preloaded tool list changes, so that reloads can be tracked deterministically.
27. As an operator, I want preloaded tool schemas to be cached in memory, so that repeated `tools/list` requests do not incur upstream network overhead.
28. As an operator, I want backend connection errors during a preloaded tool call to report whether the backend was unavailable, timed out, or rejected authentication, so that operational troubleshooting is unambiguous.
29. As an operator, I want preloaded tool calls to adhere to HTTP request and response byte caps, so that memory consumption remains bounded.
30. As an API consumer, I want JSON-RPC protocol compliance preserved across all preloaded tool operations, so that standard MCP SDKs and clients interoperate without custom adapters.

## Implementation Decisions

1. **Preload Specification via Existing `pins` Configuration**:
   The existing `pins` list in `config.yaml` is utilized as the canonical declaration for preloaded tools. Each pin explicitly binds an upstream server ID to an upstream tool name. Pin validation continues to enforce exact `server` and `tool` keys, server existence in `servers:`, non-empty strings, and rejection of duplicates.

2. **Deterministic Wire Tool Naming & Bounds**:
   Preloaded tools exposed in `tools/list` will use a deterministic double-underscore namespace format: `<server_id>__<upstream_tool_name>` (for example, `rag__search_documents`, `rag__query_context`, `mcp-second-brain__wiki_search_notes`). Upstream tool names must conform to standard identifier characters `^[a-zA-Z0-9_-]+$`. The maximum total wire name length is capped at 128 characters (`len(server_id) + 2 + len(upstream_tool_name) <= 128`). Tool names containing invalid characters or exceeding 128 characters are rejected during configuration validation with a typed diagnostic.

3. **Hybrid Tool Catalog Registration**:
   The tool surface will register the three bridge meta-tools (`search_tools`, `describe_tool`, `call_tool`) along with all resolved preloaded tools.
   - For `tools/list`, the response combines the static meta-tools with the dynamic list of preloaded tools from the current assembly generation.
   - Each preloaded tool includes its namespaced name, its upstream description, and its cached `inputSchema` extracted from the upstream catalog.

4. **Direct Preloaded Tool Dispatch**:
   When an incoming `tools/call` request specifies a preloaded tool name (e.g. `rag__search_documents`):
   - The bridge resolves the binding directly from the generation's preloaded tool registry.
   - The bridge validates the incoming arguments against the tool's stored `inputSchema`.
   - If arguments fail schema validation, the bridge returns an MCP tool error result with `isError: true` without forwarding to upstream.
   - The bridge acquires an upstream execution lease (enforcing concurrency limits, timeouts, and circuit breaker checks).
   - The request is forwarded directly to the upstream server's `tools/call` endpoint with the raw upstream tool name.
   - The result is unpacked, logged, and returned to the client as a standard MCP `CallToolResult`.
   - The client is NOT required to wrap arguments in a `call_tool` envelope.
5. **Coexistence with On-Demand Discovery**:
   - `search_tools` remains active and continues to index all tools across all connected upstreams.
   - Search results continue to flag preloaded tools with `"pinned": true` and provide their wire names.
   - `describe_tool` can describe both preloaded and non-preloaded tools.
   - `call_tool` remains capable of executing both preloaded and non-preloaded tools, ensuring backward compatibility with automated agents that exclusively use the meta-tool calling convention.

6. **Generation Lifecycle, Startup Validation & LKG Reloads**:
   - **Startup:** During bridge startup, the assembly resolves all configured pins against freshly fetched upstream catalogs. If any pinned tool is missing, startup fails closed to catch misconfigurations immediately.
   - **Runtime Reload (SIGHUP):**
     - *Upstream Outage:* If a previously pinned tool's upstream is temporarily unreachable or fails refresh, the bridge preserves the Last-Known-Good (LKG) catalog definitions for that tool, marking its health as degraded/unavailable. Degraded LKG tools remain advertised in `tools/list` with their cached schemas; direct execution of a degraded tool returns `isError: true` with reason `backend_unavailable`.
     - *Upstream Deletion:* If an upstream server is reachable but an existing pinned tool was deleted from its catalog, the reload candidate is rejected atomically with a typed diagnostic, preserving the running generation.
     - *New Pin Invalid:* If a candidate configuration introduces a *newly added pin* for an unknown tool or server, the reload candidate is rejected atomically and the running generation is preserved.
   - Once a new generation is published, the preloaded tool map is updated atomically under the runtime lock.
   - Active client sessions receive a `notifications/tools/list_changed` notification if the transport supports notifications.
7. **Normative Guardrails & Memory Bounds**:
   - Enforce a strict upper bound of at most 64 preloaded tools per configuration (`max_preloaded_tools = 64`).
   - Enforce a maximum wire name length of 128 characters.
   - Enforce a maximum `inputSchema` byte size of 64 KiB per preloaded tool.
   - Violating any bound fails configuration validation immediately with a typed diagnostic.

## Testing Decisions

### What Makes a Good Test
Tests must verify observable external behavior at the highest available system boundary without coupling to unexported internal structures or intermediate helper functions:
- Test that an MCP client sending `tools/list` receives the expected meta-tools plus the exact preloaded tool names, descriptions, and schemas.
- Test that calling a preloaded tool directly with valid arguments executes the upstream tool and returns the expected payload.
- Test that calling a preloaded tool with invalid arguments returns an MCP tool error result with `isError: true` without hitting the upstream server.
- Test that unknown tool calls produce standard MCP errors.
- Test that SIGHUP reload with modified pins updates `tools/list` and handles execution under the new tool set.
- Test that SIGHUP reload preserves LKG preloaded tools during temporary upstream outages.
- Test that SIGHUP reload with a newly introduced invalid pin fails closed and leaves the running tool set untouched.

### Test Seam
The primary test seam is the **public HTTP JSON-RPC endpoint** (`POST /mcp`), hosted by the real composition root and backed by local fake MCP upstreams. This exercises the complete HTTP inbound stack, authentication, JSON-RPC decoding, tool routing, upstream leasing, response encoding, and diagnostic logging.

### Prior Art
- `cmd/mcp-bridge/serve_test.go`: Tests the full HTTP bridge server lifecycle, tool registration, search, and execution.
- `cmd/mcp-bridge/serve_logging_execution_test.go`: Tests real MCP `tools/call` dispatch through fake upstream HTTP endpoints with sync capture.
- `internal/service/reload_test.go`: Tests generation assembly, catalog refresh, and pin resolution failures.

## Out of Scope

1. **Client-Side Dynamic Tool Registration**: Allowing connected LLM clients to dynamically push new tool definitions into the bridge via custom MCP messages is out of scope; all preloaded tools must be defined in the bridge configuration.
2. **Schema Mutation or Synthesis**: Synthesizing or altering upstream parameter schemas beyond standard normalization and validation is out of scope.
3. **Automatic Preload Heuristics**: Automatically selecting tools to preload based on historical query frequency or machine learning is out of scope; preloading is explicitly operator-configured via `pins`.
4. **Non-HTTP Transports**: Supporting stdio or raw SSE-only transport changes is out of scope; the bridge continues to operate over streamable HTTP.

## Further Notes

- **Token Budget Impact**: Exposing full schemas directly in `tools/list` increases the baseline token consumption of every client request by the size of the preloaded schemas (typically 200–500 tokens per tool). Operators should pin only genuine hot-path tools (typically 2 to 10 tools) and leave the long tail to `search_tools`.
- **Client Caching**: Some MCP clients cache the output of `tools/list` for the duration of a session. Broadcasting `notifications/tools/list_changed` ensures compliant clients invalidate their cache upon bridge reload.
