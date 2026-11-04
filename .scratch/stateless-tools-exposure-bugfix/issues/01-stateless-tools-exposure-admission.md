# 01 — Stateless Tool Exposure Admission and Wire Verification

**What to build:**
In `internal/service/session_exposure.go`, update `admitExposureRequest` so that when `request.GetSession() == nil` (stateless mode), it resolves the publisher projection `t.publisherOwner` instead of rejecting with `invalidExposureRequest()`. In `cmd/mcp-bridge/serve_oauth_test.go`, update `TestChatGPTDeveloperModeStatelessDiscoveryWire` to parse and assert the `tools/list` JSON-RPC response body, verifying that `error == nil` and `result.tools` contains the catalog. Verify full race suite passes and coverage meets >=90%.

**Blocked by:** None
- [x] In `internal/service/session_exposure.go`, `admitExposureRequest` returns the publisher projection `t.publisherOwner` when `request.GetSession() == nil || request.GetSession().ID() == ""`.
- [x] Stateful requests continue to validate epoch and live session owner.
- [x] `TestChatGPTDeveloperModeStatelessDiscoveryWire` in `cmd/mcp-bridge/serve_oauth_test.go` asserts `jsonrpc: "2.0"`, response ID 2, `error == nil`, and a non-empty `tools` array over live loopback HTTP wire.
- [x] Full repository test suite passes clean (`go test -race -p 1 ./...`).
- [x] Aggregate coverage gate passes (`go run ./cmd/covcheck` >= 90.0%).
