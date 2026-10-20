# 02: HTTP wire contract verification and full-suite coverage

**What to build:** Verify the end-to-end HTTP JSON-RPC wire contract at `cmd/mcp-bridge/`. Assert that when an external client sends a JSON-RPC `tools/call` request for `search_tools` and `describe_tool` over `POST /mcp`, the response JSON-RPC `result` object contains a non-null `content` array with valid JSON text. Verify full suite passes, race detector passes, and the 90%+ source-line coverage gate is satisfied.

**Blocked by:** 01.

**Status:** resolved

Type: task

Source: Tool content compatibility specification (`.scratch/tool-content-compatibility/spec.md`).

- [x] In `cmd/mcp-bridge/`, update or add wire integration tests asserting that `tools/call` for `search_tools` returns a JSON-RPC payload with `result.content[0].type == "text"` and parseable JSON text containing `catalog_revision` and `results`.
- [x] In `cmd/mcp-bridge/`, assert that `tools/call` for `describe_tool` returns a JSON-RPC payload with `result.content[0].type == "text"` containing the expected tool description and input schema.
- [x] Run `go test -race ./...` to verify concurrent safety.
- [x] Run `go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./... && go run ./cmd/covcheck` to verify the >=90% source-line coverage gate.

## Answer

Wire-level integration contract verified by `cmd/mcp-bridge/serve_hybrid_coexistence_test.go:101-160` (search_tools wire content block) and `:215-225` (describe_tool wire content block) over loopback HTTP `POST /mcp`.

Full test suite passes:
- `go test ./...`: 11 packages ok
- `go test -race ./...`: 11 packages ok
- Full-suite source-line coverage: 91.6% (3799/4149), passing the >=90% gate via `go run ./cmd/covcheck`.
