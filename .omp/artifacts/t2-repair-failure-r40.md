# T2 repair attempt 2 — failure record and escalation (r40)

Outcome: **BLOCKED after the second engineer attempt. Retries halted (MAX_FAILURES=2).**
Owner must explicitly approve a third attempt; none is launched.

## Attempt facts

| Field | Value |
|---|---|
| Job | `RepairTicket01T2` (engineer) |
| Duration | 6m31s (transcript 21:17–21:23) |
| Transcript | `history://RepairTicket01T2`, 160 records |
| Read/search ops | 57 `read` + 2 `grep` + 1 `glob` + 2 `lsp` = 62 total; 39 before the first edit (under P04's 42) |
| Edits | 5: 2 applied to `result_handle_test.go`; 3 rejected on `internal/observe/metrics.go` |
| Bash / write | 3 / 1 (receipt only) |
| Receipt | `.omp/artifacts/receipt_T2.json`, `status=BLOCKED`, `error_code=E_EDIT_REJECTED`, sha256 `c5fa2f45…`; `receipt-validator check-receipt` exit 0 |

## Root cause (evidence-backed)

The engineer consumed its one-correction budget on a malformed edit bundle, not on the
spec: three consecutive hashline patches to `internal/observe/metrics.go` at
`PUT 37.=52` were each rejected with *"file parses now but would not after this edit as
written (first syntax error near line 357). Nothing was applied."* After the third
rejection it stopped per T04 and wrote a truthful BLOCKED receipt. `result_handle.go`
was never created, so the observe changes were never exercised.

## Tree after the attempt

| Path | State |
|---|---|
| `internal/service/result_handle_test.go` | **repaired**: parses, exactly 9 test functions, splice markers gone, sha256 `03d3ae7a…` (15,312 B) — the one durable gain |
| `internal/service/result_handle.go` | **absent** |
| `internal/observe/metrics.go` | untouched (mtime 2026-11-03); five methods still undefined |
| `internal/observe/log.go` | untouched (2026-11-21); 12 events unregistered |
| `internal/observe/envelope.go` | untouched (2026-11-22) |
| `internal/observe/httpdiagnostics.go` | untouched (2026-11-10); vocabulary still 18, not 31 |
| `docs/logging-contract.md` | only T1's config block; no retention field vocabulary |

Post-attempt checks:

- `go vet ./internal/service/` → exit 1; undefined `resultStore`, `resultNamespace`,
  `resultOwnerResolver`, `resultCommit`, `resultLookup`. (Syntax error is gone.)
- `go test ./internal/observe/ -run TestPagingMetricsRegistered` → exit 1; the five
  `*Metrics` methods are still undefined.

AC4/5/7/8/16/18/36/38/39 therefore still fail on missing implementation; AC13/14 still
build-fail. T3 remains withheld — no production store seam exists.

## Tooling gap

`execution-state.mjs record-failure` exits 2 `E_STATE_INVALID` on this state:
`recordTaskFailureLocked` requires `state.tasks` as a task-keyed object
(`execution-transition.mjs:446-452`), while the canonical schema-2 state stores
`tasks` as an array. The failure is recorded here instead; no state file was hand-edited.

## Surgical redispatch prompt (requires explicit owner approval)

> Retry T2 once, bounded, root `/home/opencode/projects/default/mcp-bridge`, spec r40
> sha256 `37dd21ce…`. Write only the nine declared paths. Keep the existing
> `internal/service/result_handle_test.go` (sha256 `03d3ae7a…`) as the fixed contract —
> do not regenerate it. Order the work and verify after each file:
> (1) create `internal/service/result_handle.go` to the seam those nine tests declare and
> R4/R5/R6/R7/R9/R10/R16; run `go vet ./internal/service/` (expect only observe-package
> errors, none in service).
> (2) `internal/observe/metrics.go`: add the five methods and the five-code closed
> `bridge_retention_rejections_total{retention=…}` set using exact-string edits or a
> whole-file write — **no line-number hashline patches**, one change at a time, checking
> `gofmt -e` after each.
> (3) `internal/observe/log.go`: the 12 event messages. (4) `internal/observe/httpdiagnostics.go`:
> the exact 31-value `operationVocabulary`. (5) `docs/logging-contract.md`: retention log
> field vocabulary only.
> Then run and report verbatim: `go build ./...`; `go test ./internal/service/ -run
> 'TestHandle|TestRetention|TestMarker|TestEviction|TestRetired|TestRetirement' -count=1`;
> `go test ./internal/observe/ -count=1`; `gofmt -l` on all written files. Overwrite
> `.omp/artifacts/receipt_T2.json` with a real receipt. Stop at the first failing named
> command; no third design.
