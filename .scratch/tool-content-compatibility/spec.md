Status: ready-for-agent

## Problem Statement

When users ask an AI assistant inside LibreChat (or other standard Model Context Protocol clients) to find or execute tools indexed by the bridge—such as tools in a second-brain system, document stores, or developer utilities—the assistant calls the `search_tools` meta-tool, but LibreChat displays `(No response)`. The underlying LLM receives an empty string for the tool call execution, causing it to hallucinate that the bridge is offline, unreachable, or has no indexed tools registered.

In reality, the bridge receives the HTTP requests, executes the search against the published catalog, and successfully returns HTTP 200 responses containing up to hundreds of kilobytes of matched tool data. However, the bridge populates only the optional `structuredContent` field of the MCP `CallToolResult`, leaving the primary `content` array null.

Standard MCP clients (including LibreChat and client libraries derived from the core TypeScript/JavaScript and Python MCP SDKs) do not parse standalone `structuredContent` unless serialized text is also provided in `content`. According to the MCP specification and SEP-2106 compatibility standards, `content` is required and must contain text representations of structured responses. Because `content` is null, clients treat the tool call as having produced zero output, completely breaking tool discovery and execution workflows for end users. The identical defect affects `describe_tool`.

## Solution

Update the bridge's meta-tool surface so that `search_tools` and `describe_tool` populate both `content` (as a standard `TextContent` block holding the JSON-serialized payload) and `structuredContent` (the structured map) on every successful execution.

This dual-payload response adheres to the MCP specification and SEP-2106 backward compatibility requirements:
1. Standard MCP clients (like LibreChat, Claude Desktop, and CLI runners) extract the text content from `content[0].text`, pass it to the language model, and render the results in the user interface without reporting `(No response)`.
2. Advanced clients supporting SEP-2106 structured tool output continue to receive the strongly-typed `structuredContent` object without regression.
3. Upstream execution results forwarded through `call_tool` and error results from `errorResult` already populate `content` and will remain preserved and consistent with this standard.

## User Stories

1. As a LibreChat user, I want `search_tools` to return readable text content, so that LibreChat does not display `(No response)` when searching for tools.
2. As a LibreChat user, I want the LLM to receive the list of discovered tools, so that it can find notes, documents, and actions in my second-brain tools instead of falsely claiming they do not exist.
3. As an AI assistant, I want `search_tools` to return complete JSON-formatted text in its content array, so that I can parse the available tools, their descriptions, and their wire names in a single turn.
4. As an AI assistant, I want `describe_tool` to return the complete JSON schema inside its content array, so that I can inspect the required arguments and parameter types without receiving an empty response.
5. As an AI assistant, I want search queries that return zero matches to return an explicit JSON payload with an empty results list in its content array, so that I can tell the user no tools matched rather than assuming the bridge failed.
6. As a modern MCP client supporting SEP-2106, I want `structuredContent` to remain present alongside `content`, so that I can directly consume typed objects without re-parsing JSON text.
7. As a legacy MCP client, I want `content` to always contain at least one `TextContent` block on success, so that my tool call parser does not encounter schema validation errors or null pointer exceptions.
8. As an operator, I want error responses from `search_tools` and `describe_tool` to continue providing clear error messages in `content`, so that clients can display actionable failure reasons.
9. As an operator, I want the serialized text in `content` to be identical in data content to `structuredContent`, so that clients reading either representation see identical catalog revisions, server IDs, scores, and tool schemas.
10. As an operator, I want `search_tools` to preserve its hybrid keyword and embedding ranking information in the text content, so that the AI assistant can choose the most relevant tools.
11. As an operator, I want `search_tools` to preserve backend health and availability metadata in the text content, so that the AI assistant knows whether an upstream server is currently reachable before attempting to call it.
12. As an operator, I want `search_tools` to preserve the `pinned` status and canonical wire names in the text content, so that the client knows whether a tool can be called directly or requires the `call_tool` envelope.
13. As an operator, I want `describe_tool` to preserve validation status and validation reasons in the text content, so that the AI assistant understands if an upstream tool schema is supported.
14. As an operator, I want HTTP request diagnostics and observability logs to continue recording correct HTTP status, operation metrics, and byte counts without being corrupted by the addition of text content.
15. As a developer maintaining the bridge, I want all existing tests expecting `structuredContent` to continue passing unchanged, so that backward compatibility is preserved across the codebase.
16. As a developer maintaining the bridge, I want tests to assert that `content` is populated and valid on all meta-tool responses, so that future regressions cannot silently reintroduce `null` content arrays.

## Implementation Decisions

- **Dual-Representation Tool Results**:
  Both `search_tools` and `describe_tool` handlers will construct `CallToolResult` with both `Content` (containing a single `TextContent` element whose `Text` field is the compact JSON serialization of the payload) and `StructuredContent` (containing the existing unmarshaled map).

- **Strict Error Handling During Serialization**:
  If JSON marshaling of the payload fails for any unforeseen reason, the handlers will return an internal error via the standard error result constructor with `isError: true` and appropriate text content, rather than returning a partial or malformed result.

- **No Schema Changes to Inputs**:
  The input schemas for `search_tools`, `describe_tool`, and `call_tool` remain completely unchanged. This fix is strictly an output serialization and protocol compliance adjustment.

- **Consistency Across All Bridge Meta-Tools**:
  - `search_tools`: returns JSON text in `content` and structured map in `structuredContent`.
  - `describe_tool`: returns JSON text in `content` and structured map in `structuredContent`.
  - `call_tool`: continues to return upstream content in `content`, provenance envelope in `structuredContent`, and forwarded `isError`.
  - `errorResult`: continues to return error message in `content`, structured error map in `structuredContent`, and `isError: true`.

- **Standard MCP Compliance**:
  Adheres to the official Model Context Protocol specification where `CallToolResult.content` is a required array, and follows the official reference implementation behavior where unstructured text mirrors structured output for pre-SEP-2106 clients.

## Testing Decisions

- **Behavior-Driven Test Standard**:
  Tests must exercise actual client-facing wire behavior and verify protocol compliance:
  1. `CallToolResult` must have a non-nil `Content` array containing exactly one `TextContent` element.
  2. The `TextContent.Text` field must be valid JSON that unmarshals into the expected response fields.
  3. The `StructuredContent` field must match the decoded text representation.
  4. Tests must not inspect internal handler state, private structs, or unexported variables.

- **Modules and Layers Tested**:
  1. **Tool Surface Unit/Integration Tests**: Verify direct tool execution of `search_tools` and `describe_tool` against published assemblies, checking that `result.Content` is populated with valid JSON text matching `result.StructuredContent`.
  2. **End-to-End Composition Root Tests (`cmd/mcp-bridge`)**: Verify full JSON-RPC over HTTP (`POST /mcp`) using real client sessions, asserting that the JSON-RPC wire envelope carries non-null `content` arrays with valid text for `search_tools` and `describe_tool`.
  3. **Regression / Negative Path Tests**: Verify that invalid search queries, unconfigured server filters, unknown tools in describe, and stale catalog revisions return proper text content and structured error maps with `isError: true`.

- **Prior Art**:
  - `internal/service/tools_test.go`: existing `TestToolSurfaceSearchToolsReturnsRankedProvenance` and describe tests.
  - `cmd/mcp-bridge/serve_logging_execution_test.go`: existing multi-phase HTTP session test driving `search_tools` and `call_tool` over real HTTP.

## Out of Scope

- Changing how `call_tool` forwards upstream content (it already passes `upstream.Content`).
- Changing the schema or fields of `search_tools` or `describe_tool` input arguments.
- Altering the hybrid search ranking algorithm (keyword + RRF embedding).
- Preloading pinned tools into `tools/list` (handled separately under the hybrid-tool-discovery specification).
- Adding custom UI formatters or non-JSON markdown templates to the text content (compact JSON is the standard for structured content text fallbacks).

## Further Notes

- The observed log entries showing HTTP 200 with sizes `le_16k` and `le_256k` confirm that upstream indexing, search execution, and catalog generation are fully functional. The failure was entirely caused by the client ignoring responses where `content` was serialized as `null`.
- Once implemented, LibreChat will immediately be able to display search results and pass the discovered second-brain tool definitions to the LLM.
