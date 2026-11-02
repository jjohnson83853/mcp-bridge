# 02 — Integrated ChatGPT Wire Verification

Status: resolved
Type: task
Blocked by: 01
Canonical State: Governed by `.omp/plans/execution-state.json`

## Description

Verify end-to-end integration of the ChatGPT OAuth and discovery lifecycle in `cmd/mcp-bridge/serve_oauth_test.go` under `MCP_RESOURCE_URL="https://mcp.bullprog.com"` and `MCP_SESSION_ADMISSION_ENABLED="true"`. The test exercises an unauthenticated probe receiving the 401 challenge, fetching the root protected resource metadata document, issuing an OAuth token scoped to the origin resource, completing `initialize`, and executing subsequent discovery requests (`notifications/initialized` and `tools/list`) without an `Authorization` header using the issued `Mcp-Session-Id`.

## Acceptance Criteria
- [x] **AC-5**: Loopback wire test `TestChatGPTOAuthAndActionDiscoveryWire` in `cmd/mcp-bridge/serve_oauth_test.go` exercises the exact ChatGPT flow (R2):
  1. Initial unauthenticated probe to `POST /mcp` receives HTTP 401 Unauthorized with `WWW-Authenticate: Bearer realm="mcp", resource_metadata="https://mcp.bullprog.com/.well-known/oauth-protected-resource"`.
  2. `GET /.well-known/oauth-protected-resource` returns the origin resource URL (`https://mcp.bullprog.com`).
  3. `POST /mcp` (`initialize`) with Bearer token scoped to origin resource succeeds (`200 OK`) and returns `Mcp-Session-Id`.
  4. Follow-up `POST /mcp` `notifications/initialized` with `Mcp-Session-Id` and omitted `Authorization` header returns HTTP 202 Accepted with an empty body.
  5. Follow-up `POST /mcp` `tools/list` with `Mcp-Session-Id` and omitted `Authorization` header returns HTTP 200 OK with valid JSON-RPC tool list.
  6. Follow-up `POST /mcp` with invalid or unknown `Mcp-Session-Id` without `Authorization` header returns HTTP 401 Unauthorized; the response `mcp-diagnostic-id` header correlates with JSON field `request_id` in exactly one stdout diagnostic record asserting `failure_stage: "outer_oauth"` and `failure_reason: "credentials_missing"`.
- [x] **AC-6**: Repository verification gate:
  - Focused wire test: `go test -v -count=1 ./cmd/mcp-bridge -run TestChatGPTOAuthAndActionDiscoveryWire` (exit 0)
  - Package vet: `go vet ./...` (exit 0)
  - Full race test: `go test -race -count=1 ./...` (exit 0)
  - Full coverage check: `go test -count=1 -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./... && go run ./cmd/covcheck` -> PASSED at 90.1% (6,135 / 6,807 lines >= 90.0%).

## Verification Plan
1. Add `TestChatGPTOAuthAndActionDiscoveryWire` to `cmd/mcp-bridge/serve_oauth_test.go` after T1 is complete.
2. Execute: `go test -v -count=1 ./cmd/mcp-bridge -run TestChatGPTOAuthAndActionDiscoveryWire` -> PASS (GREEN).
3. Execute complete repository verification gate:
   - `go test -race ./...` -> exit 0
   - `go vet ./...` -> exit 0
   - `go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...` -> exit 0
   - `go run ./cmd/covcheck` -> exit 0 (aggregate coverage ≥ 90.0%).

## Answer

Implemented `TestChatGPTOAuthAndActionDiscoveryWire` in `cmd/mcp-bridge/serve_oauth_test.go` covering all 6 steps of the ChatGPT lifecycle. Verified passing via `go test -v -count=1 ./cmd/mcp-bridge -run TestChatGPTOAuthAndActionDiscoveryWire` (PASS), `go vet ./...` (PASS), full repository `go test -race -count=1 ./...` (PASS across all 12 packages), and `go run ./cmd/covcheck` (PASS at 90.1%). AC-5 and AC-6 fully satisfied.
