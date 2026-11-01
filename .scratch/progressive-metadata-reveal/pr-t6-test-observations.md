# PR-T6 focused test observations

Feature: `progressive-metadata-reveal`  
Revision: `cursor-choice-reconciliation-7-session-admission`  
Selected branch: `approved-progressive-metadata-ledger`  
Task scope: PR-T6 immutable session pages and authenticated generation cursors slice only.

## Acceptance mapping

- PR-AC8: `TestServeHybridProgressiveRevealPages` verifies that continuation cursors are invalidated across between-page publications/reloads, rejecting with JSON-RPC -32602 `invalid cursor` without partial data.
- PR-AC9: `TestProgressiveMetadataCursorValidation` and `TestServeHybridProgressiveRevealPages` verify:
  1. Complete version-1 encoding (57 bytes decoded, 76 characters raw URL base64, 1 version byte, three big-endian uint64 values, 32-byte HMAC-SHA256 signature).
  2. Transient cryptographically random 32-byte process secret generated at service initialization.
  3. Rejection of malformed/noncanonical base64, wrong length/size/version, invalid HMAC, offset=0, out-of-range offset, non-boundary offset, stale generation, same-bearer cross-owner epoch, and retired/replaced incarnation with code -32602 (`jsonrpc.CodeInvalidParams`) and message `invalid cursor`.
  4. Multi-page traversal with controlled page size reaching every tool in sorted order exactly once without duplication or omission, with final page `NextCursor == ""`.

These results cover PR-T6's scope only. Controlled concurrency/lifecycle (PR-T7), documentation (PR-T8), and final feature verification (PR-T9) remain assigned to their later owners.

## Actual commands and results

1. `go test ./internal/service -run '^TestProgressiveMetadataCursorValidation$' -count=1` — exit 0; package `ok`, 0.005s.
2. `go test -race ./internal/service -run '^TestProgressiveMetadataCursorValidation$' -count=1` — exit 0; package `ok`, 1.027s.
3. `go test ./cmd/mcp-bridge -run '^TestServeHybridProgressiveRevealPages$' -count=1` — exit 0; package `ok`, 0.038s.
4. `go test -race ./cmd/mcp-bridge -run '^TestServeHybridProgressiveRevealPages$' -count=1` — exit 0; package `ok`, 1.099s.
5. `go test ./cmd/mcp-bridge -run '^(TestServeHybridMetadata.*|TestServeHybridPreloadedToolsListExposure|TestServeHybridCoexistence|TestServeHybridSIGHUPReload|TestServeHybridProgressiveRevealDiscovery|TestServeHybridProgressiveRevealPages)$' -count=1` — exit 0; package `ok`, 2.372s.
6. `go test ./internal/service -count=1` — exit 0; package `ok`, 0.855s.
7. `go vet ./...` — exit 0, clean.
8. `go run ./cmd/covcheck` — exit 0; source-line coverage 90.1% (5760/6393 source lines; gate 90%: true).

Evidence captured: `2026-10-31T23:59:00Z`.
