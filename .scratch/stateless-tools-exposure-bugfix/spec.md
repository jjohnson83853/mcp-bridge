# Stateless Tools Exposure Bugfix Specification

Status: ready-for-agent

<!-- specdd:requirements:start -->
### 1. Requirements
- R1: In stateless transport mode (`Stateless: true`, `MCP_STATELESS=true`), requests arriving without a session or with an ephemeral session (`request.GetSession() == nil || request.GetSession().ID() == ""`) must not be rejected with `invalid exposure request` (`-32600`).
- R2: In stateless mode, `admitExposureRequest` must resolve the publisher projection (`coordinator.projections[t.publisherOwner]`). If no publisher projection exists, it must return a sanitized JSON-RPC internal error (`CodeInternalError`, `"exposure unavailable"`).
- R3: Stateful request admission (`request.GetSession() != nil && request.GetSession().ID() != ""`) must preserve existing behavior verbatim: epoch header validation, live owner lookup, and exact error sanitization.
- R4: `TestChatGPTDeveloperModeStatelessDiscoveryWire` in `cmd/mcp-bridge/serve_oauth_test.go` must assert that `tools/list` over live HTTP wire returns `jsonrpc: "2.0"`, ID `2`, `error == nil`, and a non-empty `tools` array.
- R5: Full test suite passes clean under `go test -race -p 1 -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...` and meets the aggregate coverage gate (`go run ./cmd/covcheck` >= 90.0%).
<!-- specdd:requirements:end -->

<!-- specdd:design:start -->
### 2. Design
In `internal/service/session_exposure.go`:
Update `admitExposureRequest` to check both nil session and ephemeral session (`session.ID() == ""`):

```go
func (t *ToolSurface) admitExposureRequest(request mcp.Request) (*toolProjection, error) {
	if request == nil {
		return nil, invalidExposureRequest()
	}
	if request.GetSession() == nil || request.GetSession().ID() == "" {
		coordinator := t.projectionCoordinator()
		coordinator.barrier.Lock()
		projection := coordinator.projections[t.publisherOwner]
		coordinator.barrier.Unlock()
		if projection == nil {
			return nil, &jsonrpc.Error{
				Code:    jsonrpc.CodeInternalError,
				Message: "exposure unavailable",
			}
		}
		return projection, nil
	}

	epoch, valid := requestExposureEpoch(request)
	if !valid {
		return nil, invalidExposureRequest()
	}
	sessionID := request.GetSession().ID()
	owner := projectionOwner{kind: projectionSession, sessionID: sessionID, epoch: epoch}
	coordinator := t.projectionCoordinator()
	coordinator.barrier.Lock()
	_, live := coordinator.liveOwners[owner]
	coordinator.barrier.Unlock()
	if !live {
		return nil, invalidExposureRequest()
	}
	projection, err := t.ensureOwnerProjection(owner)
	if err != nil {
		if rpcErr, ok := err.(*jsonrpc.Error); ok {
			return nil, rpcErr
		}
		return nil, &jsonrpc.Error{
			Code:    jsonrpc.CodeInternalError,
			Message: "exposure unavailable",
		}
	}
	return projection, nil
}
```

In `cmd/mcp-bridge/serve_oauth_test.go`:
In `TestChatGPTDeveloperModeStatelessDiscoveryWire`:
Ensure JSON-RPC envelope assertions on `toolList`:
```go
toolListBody := oauthJSONBody(t, toolList)
if toolListBody["jsonrpc"] != "2.0" {
	t.Fatalf("tools/list jsonrpc = %v; want 2.0", toolListBody["jsonrpc"])
}
if id, ok := toolListBody["id"].(float64); !ok || int(id) != 2 {
	t.Fatalf("tools/list id = %v; want 2", toolListBody["id"])
}
if toolListBody["error"] != nil {
	t.Fatalf("tools/list error = %v; want nil", toolListBody["error"])
}
result, ok := toolListBody["result"].(map[string]any)
if !ok {
	t.Fatalf("tools/list result missing or not object: %v", toolListBody)
}
tools, ok := result["tools"].([]any)
if !ok || len(tools) == 0 {
	t.Fatalf("tools/list tools missing or empty: %v", result)
}
```
<!-- specdd:design:end -->

<!-- specdd:tasks:start -->
### 3. Tasks & Acceptance Criteria
- Tasks:
  - T1: Stateless Tool Exposure Admission and Wire Verification
    - Files: `internal/service/session_exposure.go`, `cmd/mcp-bridge/serve_oauth_test.go`, `coverage.out`
    - Acceptance: AC1, AC2, AC3, AC4
- Acceptance Criteria:
  - AC1: `admitExposureRequest` returns publisher projection when `request.GetSession() == nil || request.GetSession().ID() == ""`.
  - AC2: Stateful requests preserve epoch header validation, live owner checks, and error mapping.
  - AC3: `TestChatGPTDeveloperModeStatelessDiscoveryWire` asserts `tools/list` returns `jsonrpc: "2.0"`, ID `2`, `error == nil`, and non-empty `tools` list.
  - AC4: Full test suite passes under race detector with fresh aggregate coverage meeting >= 90.0%.
<!-- specdd:tasks:end -->

<!-- specdd:envelope:start -->
### 4. Envelope & Verification
- Allowed files:
  - `internal/service/session_exposure.go`
  - `cmd/mcp-bridge/serve_oauth_test.go`
  - `coverage.out`
- Prohibited scope: auth middleware, router configuration, config schemas, storage schemas.
- RED/GREEN sequence:
  - RED: Verified in previous run (`tools/list error = map[code:-32600 message:invalid exposure request]`).
  - GREEN: Update `admitExposureRequest` in `internal/service/session_exposure.go` to include `request.GetSession().ID() == ""`; observe test passing.
- Full verification command:
  `go test -race -p 1 -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./... && go run ./cmd/covcheck`
<!-- specdd:envelope:end -->
