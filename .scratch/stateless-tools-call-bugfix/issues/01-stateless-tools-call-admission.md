# 01 - Stateless Tool Call and Preloaded Tool Execution Admission

**What to build:**
In `internal/service/tools.go` and `internal/service/projection.go`, allow stateless client requests (`req.GetSession() == nil || req.GetSession().ID() == ""`) to execute preloaded tools and built-in search tools using `t.publisherOwner`.

- [x] In `internal/service/tools.go`, `directRequestOwner` is updated to a method on `*ToolSurface` and returns `t.publisherOwner, true` for stateless requests.
- [x] Callsites in `callPinnedTool`, `searchTools`, and `describeTool` use `t.directRequestOwner(req)`.
- [x] In `internal/service/projection.go`, `discoveryOwnerValid` accepts `owner.kind == projectionPublisher`.
- [x] In `internal/service/projection.go`, `commitDiscovery` returns `nil` when `owner.kind == projectionPublisher`.
- [x] `TestChatGPTDeveloperModeStatelessDiscoveryWire` asserts `tools/call` for `search_tools` returns a non-error JSON-RPC response (`isError == false`), and a direct preloaded tool call succeeds.
- [x] Full repository test suite passes clean (`go test -race -p 1 ./...`).
- [x] Aggregate coverage gate passes (`go run ./cmd/covcheck` >= 90.0%).
