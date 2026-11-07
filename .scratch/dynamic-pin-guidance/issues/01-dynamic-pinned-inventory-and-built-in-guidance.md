# 01 - Dynamic Pinned Inventory in search_tools and Guidance on Gateway Built-ins

Status: ready-for-agent
Blocked-by: none

## What to build
Update the descriptions of `search_tools`, `describe_tool`, and `call_tool` in `internal/service/tools.go` to provide model-visible guidance steering callers toward directly declared tools:
1. `search_tools`: Dynamically incorporates the complete, exact, alphabetically sorted list of actually published configured pins from the active assembly (e.g. `rag__fetch_chunks`, `second-brain__wiki_read_note`). It instructs models to prefer directly declared tools, while noting that unpinned capabilities remain discoverable.
2. `describe_tool`: Advises avoiding redundant input-schema discovery for already visible tools, while explicitly allowing metadata and provenance inspection.
3. `call_tool`: Advises preferring directly declared tools, while retaining generic dispatch as a fallback.
4. If no configured pins are published, `search_tools` cleanly omits the inventory section while retaining baseline direct-first guidance.
5. All safety hints, schemas, and dispatch semantics remain unchanged.

## Acceptance Criteria
- [ ] AC-1: In `tools/list`, `search_tools.Description` dynamically lists the exact, sorted wire names of all published configured pins.
- [ ] AC-2: In `tools/list` with zero configured pins, `search_tools.Description` omits the pin inventory and provides baseline direct-first guidance.
- [ ] AC-3: `describe_tool.Description` advises against redundant schema discovery while allowing metadata inspection.
- [ ] AC-4: `call_tool.Description` advises preferring direct tools and preserving generic dispatch as fallback.
- [ ] AC-5: Real HTTP JSON-RPC integration wire tests in `cmd/mcp-bridge` and unit tests in `internal/service` assert the exact descriptions and inventory formatting.
