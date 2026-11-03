# 01 — Tool Input Schema Alignment and Bounds Normalization

**What to build:**
Align the bridge's three built-in tool input schemas (`search_tools`, `describe_tool`, and `call_tool`) to standard Model Context Protocol parameter conventions. All tool schemas must omit the root `$schema` property. The `servers` parameter on `search_tools` must remove `minItems` and `maxItems` while preserving items regex pattern and `uniqueItems: true`. The `arguments` parameter on `call_tool` must be defined as `{"type": "object", "additionalProperties": true}` with description `"Arguments for the upstream tool invocation"`. The Go search request decoder must tolerate an empty `servers: []` slice by treating it as an omitted filter (returning results equivalent to omitting `servers`), admit 1 to 32 servers, and strictly reject any array with more than 32 servers with `invalidArguments` returning an MCP tool execution failure (`isError: true`, `code: "invalid_arguments"`).

**Blocked by:** None — can start immediately.

**Status:** closed

- [x] `SearchToolsInputSchema`, `DescribeToolInputSchema`, and `CallToolInputSchema` omit the root `$schema` property (verified via comma-ok check).
- [x] `search_tools` input schema defines `servers` as an array without `minItems` or `maxItems` keys, retaining item pattern and `uniqueItems: true`.
- [x] `call_tool` input schema defines `arguments` as `{"type": "object", "additionalProperties": true, "description": "Arguments for the upstream tool invocation"}`.
- [x] Parameter optionality is preserved: `query` is required in `search_tools`; `top_k` and `servers` are optional.
- [x] String patterns and length constraints (`minLength`, `maxLength`, `pattern`) are preserved across all tool schemas.
- [x] Go decoder treats an empty `servers: []` array as an omitted filter (`request.Servers == nil`) without error.
- [x] Go decoder admits populated `servers` arrays up to 32 items.
- [x] Go decoder strictly rejects requests where `servers` exceeds 32 items with `*search.Failure` returning `isError: true` and `code: "invalid_arguments"`.
- [x] Unit tests in `internal/catalog/catalog_test.go` assert absence of `$schema`, absence of `minItems`/`maxItems`, presence of `additionalProperties: true` on `arguments`, and required description.
- [x] Unit tests in `internal/search/search_test.go` verify empty servers list acceptance, 32-server acceptance, and 33-server rejection.
