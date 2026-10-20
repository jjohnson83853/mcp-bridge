# 01: Service-layer tool content fallback population

**What to build:** In `internal/service/tools.go`, update `searchTools` and `describeTool` handlers to serialize the payload into compact JSON and populate the `Content` field with a `TextContent` block alongside `StructuredContent`. In `internal/service/tools_test.go`, add assertions verifying that both meta-tools produce non-empty `Content` arrays with valid JSON matching `StructuredContent`.

**Blocked by:** None.

**Status:** resolved

Type: task

Source: Tool content compatibility specification (`.scratch/tool-content-compatibility/spec.md`).

- [x] In `internal/service/tools.go`, update `searchTools` to marshal `searchPayload(response)` to JSON and assign `Content: []mcp.Content{&mcp.TextContent{Text: string(rawJSON)}}` on `CallToolResult`.
- [x] In `internal/service/tools.go`, update `describeTool` to marshal `payload` to JSON and assign `Content: []mcp.Content{&mcp.TextContent{Text: string(rawJSON)}}` on `CallToolResult`.
- [x] Handle any marshaling error by returning an `errorResult(codeInternalError, ...)` with `isError: true`.
- [x] In `internal/service/tools_test.go`, add test cases asserting `result.Content` is non-nil, contains exactly one `TextContent` item, and its `Text` unmarshals into the same data fields as `result.StructuredContent`.
- [x] Ensure existing tests in `internal/service` continue to pass without regression.

## Answer

Implemented and committed in `main` at `2fbc290` (`internal/service/tools.go:424-432` for `searchTools`, `:512-519` for `describeTool`). Both meta-tools serialize the structured payload to compact JSON and assign `Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}` alongside `StructuredContent`.

Verified by `internal/service/tools_test.go:470-480` and `:565-575`.
