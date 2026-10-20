# 04: Search coexistence and metadata alignment

**What to build:** The existing search, describe, and call meta-tools (`search_tools`, `describe_tool`, `call_tool`) remain fully functional alongside preloaded tools. Search results reflect canonical wire names and pinned flags; `describe_tool` and `call_tool` can operate on both preloaded and long-tail tools.

**Blocked by:** 03.

**Status:** complete

Type: task
Source: Hybrid tool discovery specification, revision HYBRID-DISCOVERY-r3. Coverage: User Stories 14–17; Implementation Decision 5.

- [X] Ensure `search_tools` indexes the complete upstream catalog (both preloaded and long-tail tools).
- [X] Align search result payloads so that preloaded tools report `"pinned": true` and expose their advertised wire name (`<server_id>__<tool_name>`).
- [X] Verify `describe_tool` accepts both canonical namespaced wire names and opaque hash names, returning the stored schema.
- [X] Verify `call_tool` continues to accept both preloaded tools and unpinned long-tail tools via the generic envelope, preserving backward compatibility for existing automated workflows.
- [X] Add integration tests verifying full coexistence: client discovers an unpinned tool via `search_tools`, inspects it via `describe_tool`, invokes it via `call_tool`, and directly invokes a preloaded tool.

24- Full suite: `go test ./cmd/mcp-bridge/... ./internal/... -count=1` -> 11 packages ok.
25- Coverage gate: `go run ./cmd/covcheck -profile=/tmp/coverage.out` -> 91.4% (3784/4141), gate 90%: true.
