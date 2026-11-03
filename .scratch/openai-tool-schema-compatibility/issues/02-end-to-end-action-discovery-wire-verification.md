# 02 — End-to-End Action Discovery Wire Verification

**What to build:**
End-to-end HTTP loopback integration tests asserting that `POST /mcp` requests for `tools/list` return the updated tool definitions over live HTTP wire. Verifies that the discovery payload served to connecting clients (such as ChatGPT Custom GPT actions) contains tool schemas that conform to OpenAI function-calling compiler constraints without breaking compatibility for Claude, Cursor, or Gemini clients. Wire tests verify that `search_tools` execution over HTTP admits `servers: []` (matching omitted filter) and up to 32 valid configured servers, while 33 servers returns an MCP tool failure (`isError: true`, `code: "invalid_arguments"`). Full test suite passes clean under `go test -race ./...` and meets the aggregate coverage gate (`go run ./cmd/covcheck` >= 90.0%).

**Blocked by:** 01 — Tool Input Schema Alignment and Bounds Normalization

**Status:** closed

- [x] Loopback HTTP wire test executes `POST /mcp` with `tools/list` on an authenticated session and parses the returned JSON-RPC result.
- [x] Returned tool schemas for `search_tools`, `describe_tool`, and `call_tool` verify absence of `$schema`, absence of `minItems`/`maxItems`, and valid object definition for `arguments` with description.
- [x] Wire test verifies that invoking `search_tools` over HTTP with `servers: []` succeeds (`isError: false`) with results identical to omitting `servers`.
- [x] Wire test verifies that invoking `search_tools` over HTTP with up to 32 valid configured servers succeeds (`isError: false`).
- [x] Wire test verifies that invoking `search_tools` over HTTP with 33 servers returns an MCP tool failure with `isError: true` and `code: "invalid_arguments"`.
- [x] Full repository test suite passes clean (`go test -race ./...`).
- [x] Aggregate repository source coverage meets or exceeds 90.0% (`go run ./cmd/covcheck`).
