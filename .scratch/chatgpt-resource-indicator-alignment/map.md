# ChatGPT Resource Indicator Alignment and RFC 9728 Discovery

- **Canonical Specification:** `spec.md` (r15)
- **Status:** resolved
- **Children:**
  - `issues/01-context-aware-resource-metadata-advertisement.md` (AC-1, AC-2, AC-3, AC-4; T1)
  - `issues/02-integrated-chatgpt-wire-verification.md` (AC-5, AC-6; T2)
- **Execution Frontier:**
  - Both child tickets (01, 02) are resolved.

## Decisions-so-far

- **Ticket 01 (`issues/01-context-aware-resource-metadata-advertisement.md`)**: Context-aware Protected Resource metadata handler implemented in `internal/auth/metadata.go`. Trims trailing slashes, routes origin queries on `/.well-known/oauth-protected-resource` to origin payload, and routes `/mcp` path queries to `/mcp` endpoint payload.
- **Ticket 02 (`issues/02-integrated-chatgpt-wire-verification.md`)**: Complete 6-step loopback wire test `TestChatGPTOAuthAndActionDiscoveryWire` added in `cmd/mcp-bridge/serve_oauth_test.go` and passing. Targeted edge tests added in `progressive_session_test.go`, `store_test.go`, and `serve_oauth_test.go`. Full repository verification gate passed: `go test -race -count=1 ./...` (PASS across all 12 packages), `go vet ./...` (PASS), and `go run ./cmd/covcheck` passed at 90.1% (6,135 / 6,807 lines >= 90.0%).
