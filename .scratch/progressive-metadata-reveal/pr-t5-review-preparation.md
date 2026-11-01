# PR-T5 review preparation — not dispatched

## Baseline and scope

- Repository: `/home/opencode/projects/default/mcp-bridge`.
- Git HEAD: `c44f56dc101de5a6cc4095ebd2a509dc6ae36451`; the worktree was already dirty before PR-T5.
- Pre-T5 `pr-t5-baseline.txt` SHA-256: `baac51df208d2de53cbe8c9d67d4b3d75b93770bbbd5627330fb6f37ab4ddbf8`. It records status only, not source snapshots.
- The PR-T4 review artifact `/tmp/mcp-bridge-PR-T4-review.md` (SHA-256 `703c2415f6a6e552abafee9fc8a4b52e205ddc8684e88af4882bdf4482e93a8f`) records pre-T5 `tools.go` SHA-256 `e78cd5d9b44e3f81c33e1f5a6d6cbbc0d07fc68e880193e791a712d49523c361`. Current `tools.go` SHA-256: `cb693e93f1f9cfd54eddaeed694a19daa78f2ad0124a804313edb978e376d521`.
- `projection.go` was already untracked at the PR-T5 baseline, but no pre-T5 content hash/snapshot was recorded. Current SHA-256: `78ea79d595634da0b66a92f273f95ce361f6a190e3c476b9da472cb2f7e491c7`.
- Exact byte-for-byte PR-T5 production diff cannot be reconstructed from the recorded baseline. Review only the T5-specific anchors below; exclude all other dirty hunks.

## Included PR-T5 anchors

- Spec: `.scratch/progressive-metadata-reveal/spec.md:312-324`.
- `internal/service/tools.go` SHA-256 `32f97226b68e69901272fdca920dbaa4c0dbb0db3cc2942ef6af0c906a57df4b`: `ToolSurface.searchTools` `:368-414`; `searchDiscoveryTargets` `:416-445`; `ToolSurface.describeTool` `:468-540`.
- `internal/service/projection.go` SHA-256 `78ea79d595634da0b66a92f273f95ce361f6a190e3c476b9da472cb2f7e491c7`: `ToolSurface.commitDiscovery` `:355-452`; staged reconciliation `:455-515`; replacement/reconciliation helpers around `:628-743`; descriptor validation and publication/registry paths around `:770-915`. These paths were already dirty/untracked; do not infer all lines are new in T5.
- `internal/service/progressive_discovery_test.go` (new in T5; SHA-256 `99ac8488ad8c91c3cc95076cf643182fe15e33e34aa8e260510a5dd8655b9027`): `TestProgressiveMetadataDiscoveryCommit`, `TestProgressiveMetadataDiscoveryFailuresDoNotCommit`, `TestProgressiveMetadataDiscoveryCancellationKeepsCommit`, snapshot/assertion helpers.
- `cmd/mcp-bridge/serve_hybrid_progressive_reveal_test.go` (new in T5; SHA-256 `52bae42e681fed5486afa91ab6f7a9fca1f357731eac8d611a55b839388ba7dc`): `TestServeHybridProgressiveRevealDiscovery`.
- PR-T5 test-fix scope: `internal/service/tools_test.go` SHA-256 `fa0fd197098ba92985dbbbc07cafb9d5ef7152932ed71d9e17ef2759a09e4b1f` (`assertAdvertisedBuiltinsAndPin`, initial/reload list tests); `internal/service/service_edge_test.go` SHA-256 `2fef62453585adeb04d46d5a2deff674691b389bde79d05c28f1e7f51ae0a94f` (`TestDescribeToolReturnsStoredOutputSchemaAndUnknownTool`).
- Exclude existing PR-T1–T4 changes and every unrelated path in `pr-t5-baseline.txt`.

## Verification evidence

- PASS: `go test ./internal/service -run '^(TestToolSurfaceAdvertisesExactlyItsThreeTools|TestToolSurfaceSurvivesReloadWithoutDuplicatingTools|TestDescribeToolReturnsStoredOutputSchemaAndUnknownTool)$' -count=1` (exit 0, 0.017s).
- PASS: `go test ./internal/service -count=1` (exit 0, 0.898s); `go test -race ./internal/service -count=1` (exit 0, 2.550s).
- PASS: `go test ./cmd/mcp-bridge -run '^TestServeHybridProgressiveRevealDiscovery$' -count=1` (exit 0, 0.042s; engineer), and rerun after final changes (exit 0, 0.044s; coordinator).
- PASS: `go test ./cmd/mcp-bridge -run '^(TestServeHybridMetadata.*|TestServeHybridPreloadedToolsListExposure|TestServeHybridCoexistence|TestServeHybridSIGHUPReload)$' -count=1` (exit 0, 2.301s).
- PASS: `go vet ./...` (exit 0). `go run ./cmd/covcheck` reports 90.0% source-line coverage (5685/6316), gate true.
- FAIL: `go test ./...`, `go test -race ./...`, and `go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...` each fail only at `internal/storage/store_test.go:179`, checksum `f7eb24a8ab754ad9eaf7449afcfab644ffe6a6c7048283eb42292f307684ddc0`. Storage is outside PR-T5 effects; no edits made.
- `gofmt` and LSP diagnostics pass for the repair files. Full command outputs: `artifact://584`, `artifact://586`, `artifact://589`; engineer repair evidence: `/tmp/mcp-bridge-pr-t5-list-contract-repair.md`.
- PR-T5's stale-revision negative case now passes with the corrected raw JSON fixture; no remaining PR-T5 focused service test failure is observed. Postcommit notification-failure simulation remains unexercised (no existing deterministic injection seam recorded).
- PR-T5 full acceptance still pending: independent PR-AC11/code review not dispatched (role absent); feature documentation/changelog and downstream feature gates not complete.

## Reviewer instructions — draft only

- Review the listed PR-T5 anchors against spec lines 312-324. Check complete response preparation before commit; accepted-generation and owner validity; atomic projection/union/interest updates; same-generation merge; registry reconciliation before success; search limits/payload preservation; failed/empty/invalid target/stale revision/preparation non-mutation; cancellation after commit; same/different bearer isolation and immediate list/direct behavior. Confirm each new session's baseline has the three built-ins plus current pins, and discovery reveals only its owner's targets before success. The stale-revision test now passes using a raw JSON arguments object; stale-revision non-mutation remains part of the review. Report file:line evidence, severity, and unmet criteria. Read-only; no source/state changes, no Go reviewer dispatch.

## Gate

No review dispatched. Canonical `PR-T5` remains pending/runnable with `agentRoles:["engineer"]`; PR-AC11 inspection is not authorized. No execution-state or policy changes were made.