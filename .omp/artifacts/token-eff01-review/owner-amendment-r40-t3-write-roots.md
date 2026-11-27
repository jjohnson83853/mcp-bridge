# Owner amendment — r40 T3 write roots (inventory pins)

Owner answer to the coordinator's amendment form (`amendment=extend-t3`), 2026-11-27:

> Authorize an r40 amendment adding the three files to T3's effects; engineer refreshes the pins, T3 completes and T4 proceeds.

Scope authorized (test-only inventory refresh for the fourth built-in, no semantic change):

1. cmd/mcp-bridge/serve_hybrid_progressive_reveal_lifecycle_test.go — assertProgressiveRevealNameSet expected set gains retrieve_result.
2. cmd/mcp-bridge/serve_hybrid_progressive_reveal_test.go — traversal expected map at ~352 gains retrieve_result.
3. cmd/mcp-bridge/serve_oauth_test.go — exposed_tools 3 -> 4 at lines 450 and 501.

Context: spec T3 write roots enumerate nine other count-assertion files but omit these three; no task in the r40 packet owns them, so `check-dispatch` denies an engineer dispatch that edits them. Pre-existing flake `TestBridgeRebindsServerEndpointOnRestartAndReload` (`invalid exposure request`, 1 failure in 5 full runs, passes alone) is out of scope.
