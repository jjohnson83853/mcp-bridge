# PR-T5 focused test observations

Feature: `progressive-metadata-reveal`
Revision: `cursor-choice-reconciliation-7-session-admission`
Selected branch: `approved-progressive-metadata-ledger`
Task scope: PR-T5 progressive discovery reveal slice only.

## Acceptance mapping

- PR-AC1: `TestToolSurfaceAdvertisesExactlyItsThreeTools` and `TestToolSurfaceSurvivesReloadWithoutDuplicatingTools` verify that each new session baseline advertises exactly the three built-ins plus configured pins without duplication across reloads.
- PR-AC2/3/4/5/10: `TestServeHybridProgressiveRevealDiscovery` and `TestProgressiveMetadataDiscoveryCommit` verify that unpinned tools revealed via search/describe return effective descriptions, metadata overrides, and provenance fields matching the projection descriptors, while preserving separate source `upstream_metadata`.
- PR-AC6/7: `TestServeHybridProgressiveRevealDiscovery` and `TestProgressiveMetadataDiscoveryFailuresDoNotCommit` verify same-session immediate discovery-to-list/direct execution and distinct-bearer isolation.
- PR-AC8: `TestProgressiveMetadataDiscoveryFailuresDoNotCommit` verifies that stale catalog revisions reject without mutating the session's projection, union, or interests.
- PR-AC9: `TestProgressiveMetadataDiscoveryCommit` verifies that concurrent searches under the same owner retain both discovered tools and interest sets in that owner's projection upon concurrent completion.

These results cover PR-T5's scope only. Dynamic pagination (PR-T6), full lifecycle (PR-T7), documentation (PR-T8), and final feature verification (PR-T9) remain assigned to their later owners.

## Actual commands and results

1. `go test ./internal/service -run '^TestProgressiveMetadataDiscovery' -count=1` — exit 0; package `ok`, 0.024s.
2. `go test -race ./internal/service -run '^TestProgressiveMetadataDiscovery' -count=1` — exit 0; package `ok`, 1.111s.
3. `go test ./cmd/mcp-bridge -run '^TestServeHybridProgressiveRevealDiscovery$' -count=1` — exit 0; package `ok`, 0.040s.
4. `go test ./cmd/mcp-bridge -run '^(TestServeHybridMetadata.*|TestServeHybridPreloadedToolsListExposure|TestServeHybridCoexistence|TestServeHybridSIGHUPReload)$' -count=1` — exit 0; package `ok`, 2.307s.
5. `go test ./internal/service -count=1` — exit 0; package `ok`, 0.946s.

Evidence captured: `2026-10-31T20:45:00Z`.
