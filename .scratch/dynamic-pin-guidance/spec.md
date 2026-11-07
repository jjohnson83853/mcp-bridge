# Specification: Dynamic Pinned Tool Guidance on Gateway Built-ins

Status: ready-for-agent

## Problem Statement

When AI agent clients (such as Gemini CLI) connect to `mcp-bridge`, they frequently bypass directly exposed pinned tools (e.g., `rag__search_documents`, `second-brain__wiki_read_note`) and immediately call `search_tools` or generic `call_tool`. Because LLM function-calling attention mechanisms prioritize plain-text tool descriptions over custom system instructions or markdown skills, generic tool descriptions like `"Search the bridge's indexed tools"` are interpreted as mandatory discovery rituals. This causes unnecessary roundtrips, redundant discovery latency, and interactive safety confirmation dialogs when calls fall back to generic multiplexers.

## Solution

Dynamically enrich the plain-text tool descriptions of the three gateway built-in tools (`search_tools`, `describe_tool`, and `call_tool`) to provide clear, model-visible guidance that directs the agent toward directly declared tools:
1. `search_tools` dynamically incorporates the complete, exact, alphabetically sorted inventory of actually published configured pins. It explicitly instructs the model to invoke suitable directly declared tools first, while retaining discovery capability for unpinned tools.
2. `describe_tool` instructs the model not to perform redundant schema discovery for tools already visible in the catalog, while explicitly allowing metadata and provenance inspection.
3. `call_tool` instructs the model to prefer suitable directly declared tools and retain generic dispatch as a fallback.
4. All safety hints, schemas, dispatch semantics, and upstream metadata remain strictly intact.

<!-- specdd:requirements:start -->
## User Stories

1. As an autonomous AI agent, I want `search_tools` to list the exact wire names of directly published configured tools, so that I know which capabilities can be invoked immediately without a discovery roundtrip.
2. As an autonomous AI agent, I want `search_tools` to clearly instruct me to prefer directly declared tools, so that my function-calling selector does not waste turns searching for tools already visible in my active catalog.
3. As an operator configuring a subset of tools from a server (e.g. pinning only 2 out of 5 `rag` tools), I want `search_tools` to permit discovering the remaining unpinned tools from that server, so that partial pinning does not artificially block discovery.
4. As an autonomous AI agent, I want `describe_tool` to advise against redundant input-schema discovery while permitting metadata inspection, so that I do not re-query schemas I already have but can still inspect tool provenance when needed.
5. As an autonomous AI agent, I want `call_tool` to advise preferring directly declared tools, so that I do not trigger generic gateway multiplexer safety prompts for capabilities that have dedicated direct endpoints.
6. As an operator running `mcp-bridge` with no pinned tools, I want the gateway built-ins to maintain clean, baseline direct-first guidance without displaying an empty or confusing inventory list.
7. As an operator reconfiguring tool pins at runtime, I want configuration reloads to update the dynamic pin inventory in `search_tools` immediately, so that the advertised guidance matches the active catalog generation.
<!-- specdd:requirements:end -->

<!-- specdd:design:start -->
## Implementation Decisions

1. **Assembly-Level Generation**:
   The pinned tool inventory is derived from the actually published configured pins of the active `Assembly` at startup and configuration reload. It is not recomputed per-session on progressive dynamic reveals.
2. **Exact Wire Names & Deterministic Sorting**:
   The inventory lists exact wire names (e.g. `rag__fetch_chunks`, `second-brain__wiki_read_note`) sorted alphabetically. There is no truncation budget or wildcarding.
3. **Inventory Placement**:
   The dynamic pin list is included exclusively in `search_tools.Description`. `describe_tool` and `call_tool` receive general direct-first guidance without repeating the pin inventory.
4. **Zero-Pin Fallback**:
   When no configured pins are published, `search_tools.Description` omits the inventory section and provides baseline guidance encouraging direct execution of any declared tools.
5. **Preserved Semantics**:
   - `call_tool` annotations (`destructiveHint: true`, `idempotentHint: false`, `openWorldHint: true`) and arguments remain unchanged.
   - Upstream tool descriptions, schemas, annotations, and `_meta` are preserved without modification.
   - Built-in tool input schemas (`catalog.SearchToolsInputSchema()`, etc.) remain unchanged.
<!-- specdd:design:end -->

<!-- specdd:tasks:start -->
## Testing Decisions

1. **Good Test Standard**:
   Tests must exercise observable wire-level protocol behavior through real `/mcp` `tools/list` requests. Tests must never inspect internal unexported fields or mock catalog state.
2. **Modules Tested**:
   - `internal/service`: Unit and assembly-level tests for dynamic description formatting.
   - `cmd/mcp-bridge`: Full integration wire tests for `tools/list` payloads, zero-pin baseline behavior, and reload consistency.
3. **Prior Art**:
   - `cmd/mcp-bridge/serve_hybrid_metadata_test.go`
   - `internal/service/progressive_projection_test.go`
4. **Regression Safety**:
   - Full test suite passes: `go test -race ./...`
   - Coverage gate threshold: `go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./... && go run ./cmd/covcheck` passes with $\ge 90\%$ statement coverage.
<!-- specdd:tasks:end -->

<!-- specdd:envelope:start -->
## Out of Scope

- Client-side execution gate implementations or prompt-policy modifications in third-party harnesses.
- Guaranteeing model selection behavior in LLM clients (model choice remains probabilistic).
- Modifying `call_tool` safety annotations, argument schemas, or execution paths.
- Altering upstream tool definitions, descriptions, or tags.
<!-- specdd:envelope:end -->

## Further Notes

Metadata annotations and descriptions are advisory hints defined by the MCP specification. They do not grant execution authority or bypass any server-side authorization or validation rules.
