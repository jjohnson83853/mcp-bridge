# 01 - Upstream Server Stateless Wiring and Restart Resilience

Status: closed
Type: task

**What to build:**
In `cmd/mcp-bridge/wiring.go`, pass `Stateless: server.ServerStateless()` into `upstream.DedicatedOptions` when constructing the dedicated client in `newClient`. This activates the server's configured (and default) stateless mode so tool discovery and tool calls execute ephemerally over pooled keep-alive connections without pinning stale session handles. Add an integration test in `cmd/mcp-bridge/serve_upstream_restart_test.go` verifying that upstream backend server restarts do not cause subsequent tool calls to fail.

- [X] In `cmd/mcp-bridge/wiring.go`, update `newClient` to pass `Stateless: server.ServerStateless()` to `upstream.DedicatedOptions`.
- [X] Add integration test `TestUpstreamRestartSurvivesInDefaultStatelessMode` in `cmd/mcp-bridge/serve_upstream_restart_test.go` using a synchronized dynamic peer handler factory (`newPeerHandler`) with RWMutex swap, asserting generation `"after"` and `isError == false` after restart.
- [X] Add integration test `TestUpstreamPreservesSessionInExplicitStatefulMode` in `cmd/mcp-bridge/serve_upstream_restart_test.go` verifying that an upstream server explicitly configured with `stateless: false` in YAML performs exactly 1 initialize handshake across multiple calls.
- [X] Full repository test suite passes clean and meets coverage gate:
  `go test -race -p 1 -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./... && go run ./cmd/covcheck` (>= 90.0%).
