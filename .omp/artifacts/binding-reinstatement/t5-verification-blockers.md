# T5 verification blockers (attempt 2)

Date: 2026-11-28. HEAD fa832293 + 9-path T5 working tree. Owner review round: binding-reinstatement r17 (CR APPROVE, Critic PASS).

## Green gates (engineer-reported, exit 0)
Commands 1–6, 9, 10 of §6 T5. RED observed first: `storage: tombstoned server cannot be rebound` for both fingerprint sub-cases.

## Blocker 1 — data race on Store.closed (PRE-EXISTING)
- Command: `go test -race -p 1 ./...` → FAIL.
- Test: `TestStartupListenerServeFailureClassified` (10.03 s, drain timeout).
- Race: write `internal/storage/store.go:66` (Close) vs read `internal/storage/store.go:101` (LoadPrompts/SavePrompts trust check).
- Writer stack: test goroutine → runContext → serve → deferred `Store.Close` (`cmd/mcp-bridge/serve.go:138`).
- Reader stack: `Loop.Start` worker (`loop.go:178/180`) → `Refresher.Run` (`refresh.go:198`) → `runPassFor` (`:266`) → `Runtime.Refresh` (`:57`) → build (`reload.go:778`) → persistence closure (`serve.go:196`) → SavePrompts.
- Root cause (debugger, high confidence): serve closes the store before cancelling/joining the loop; the drain timeout does not stop writers; `Store.Close` does not take `commitMu`.
- Baseline proof: reproduced in a clean `git archive fa832293` tree at `/tmp/mcp-head` with the same test (`FAIL ... 10.058s`, race detected). The T5 change does not introduce it; it exposes it.
- Fix location: `cmd/mcp-bridge/serve.go` lifecycle ordering (+ lifecycle regression test). Outside the 9-path envelope.

## Blocker 2 — coverage
- `go run ./cmd/covcheck` on the stale committed `coverage.out` reported 89.0% (7907/8889) — not a measurement of the T5 tree.
- Regenerated profile (`go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...`) while `internal/service` had one instrumented flake (`TestToolSurfaceSearchToolsUsesWireBoundaryAndFallbackNames`, passes alone on HEAD and current): covcheck reported 90.3% (8030/8889), gate true. Treat as provisional until a fully green profile run.

## Flake note
`TestToolSurfaceSearchToolsUsesWireBoundaryAndFallbackNames` failed once under `-coverpkg` instrumentation with `invalid exposure request`; passes standalone on HEAD and on the T5 tree, and the package suite passes standalone on both. Not attributable to T5; needs a separate look if it recurs under coverage instrumentation.
