# T2 acceptance validation — token-efficient-tool-responses r40

Verdict: **PASS**. T2's eleven acceptance ids are backed by passing oracles, the
receipt validates, and the workspace is vet/build/gofmt clean.

Spec anchor: `.scratch/token-efficient-tool-responses/spec.md`
sha256 `37dd21cee4cebdd8c5e4771741e05e594feb01292e43269ace89ee11d35ad686` (r40 binding).
Acceptance log: `.omp/artifacts/t2-acceptance/acceptance.log`
sha256 `e23074af9af28b5e9577c81a36123ea25ebb5ab2bb0cdc5ae7674af71ba906da`.

## 1. Receipt and attempts

| Item | Fact |
|---|---|
| `.omp/artifacts/receipt_T2.json` | `status=COMPLETED`, sha256 `9505145cd6df5bcf4197953136e92120ecdd83cb162e222fb7642a578fbc83ab` |
| `receipt-validator check-receipt` | `valid: true`, exit 0 |
| Attempt 2 (`RepairTicket01T2`) | 6m31s, 62 read/search ops, 3 rejected `metrics.go` patches → BLOCKED |
| Attempt 3 (`ImplementTicket01T2Attempt3`) | 8m26s, 31 tool calls, 47 reads, 2 searches; store landed, 2 oracles failed |
| Attempt 4 (+2 resumes) | fixtures repaired, observe lane completed, `logHTTP` empty-label guard, gofmt, receipt |

## 2. Verification (re-run by the coordinator after the last edit)

| Command | Exit | Output |
|---|---|---|
| `go build ./...` | 0 | — |
| `go vet ./internal/service/ ./internal/observe/` | 0 | — |
| `gofmt -l <six T2 files>` | 0 | no output |
| `go test ./internal/observe/ -count=1` | 0 | `ok … 0.134s` |
| `go test ./internal/service/ -run 'TestHandle\|TestRetention\|TestMarker\|TestEviction\|TestRetired\|TestRetirement' -count=1` | 0 | `ok … 0.024s` |

## 3. AC verdicts (T2's eleven ids)

| AC | Oracle | Verdict |
|---|---|---|
| AC4 | `TestHandleOwnerEpochBinding` | PASS |
| AC5 | `TestHandleExpiry` | PASS |
| AC7 | `TestRetentionBounded` (entry-bound FIFO + 100-byte byte-bound FIFO subcase) | PASS |
| AC8 | `TestRetiredNamespaceTombstone` | PASS |
| AC13 | `TestLogLabelEnumeration` (31-value registry, 12 events, violation path both funnel points) | PASS |
| AC14 | `TestPagingMetricsRegistered` | PASS |
| AC16 | `TestHandleRejectionUniformStore` | PASS |
| AC18 | `TestEvictedHandleRetrieval` | PASS |
| AC36 | `TestEvictionNamespaceIsolation` | PASS |
| AC38 | `TestRetirementAdmissionUnconditional` | PASS |
| AC39 | `TestMarkerAdmissionBound` (aggregate-neutral cap swap) | PASS |

## 4. Oracle defects found and repaired during T2 (test-only, R6-conformant)

1. `TestRetentionBounded` set `MaxBytesPerNamespace=100` while requiring two retained
   60-byte entries (120 payload bytes) and asserting `namespacePayloadBytes <= 100` —
   unsatisfiable for any implementation under R6. Repaired to a 200-byte entry bound
   plus a second store with a 100-byte cap and 40-byte payloads, which exercises the
   byte-overflow FIFO path (120 > 100 → oldest evicted, 80 retained).
2. `TestMarkerAdmissionBound` asserted a −256 aggregate delta for a marker-cap swap.
   Under R6 an entry holds payload + a durable 256-byte reservation that converts into
   its marker's charge, so the swap is aggregate-neutral (576 → 576); the test's own
   `entries==1 && markers==1` companions already implied that. Repaired to assert
   equality; the cap-eviction observables (`invalid_handle` for the evicted oldest
   marker, `result_evicted` for the newest) are unchanged.

3. `internal/observe/log.go` `logHTTP` treated an empty `HTTP.Operation` as an unlisted
   label because `logVocabularyViolation` maps `""` to `!validOperation("")`. Repaired
   by checking the vocabulary only for non-empty event/operation values, restoring the
   pre-existing invalid-core `logging_write_failed` fallback (`buildEnvelope` already
   layered the structural gate first).

## 5. Store seam T2 lands (T3's dependency)

```
newResultStore(bounds config.RetainedResultBounds, logger *observe.Logger,
               metrics *observe.Metrics, now func() time.Time, entropy io.Reader) *resultStore
resultNamespace{sessionID, epoch string; publisher bool}
resultOwnerResolver func(resultNamespace) (resultNamespace, bool)
store.commit(owner resultNamespace, resolve resultOwnerResolver, payload []byte,
             upstreamCode string, isError bool) resultCommit      // {handle, retention}
store.lookup(handle string, owner resultNamespace, resolve resultOwnerResolver, page int) resultLookup
store.retire(resultNamespace) | store.sweep() | store.installMarker(entry, code, transition)
store.stats() // aggregateBytes, entries, tombstones, markers, namespacePayloadBytes
```

`lookup` already applies R13 precedence (shape → owner resolution/binding → page) and
returns shared `invalidHandleLookup()` / `invalidArgumentsLookup()` values.

## 6a. Harness backlog ticket

Filed `HM-03` — https://github.com/jjohnson83853/firecrawl-router/issues/33
(`status/backlog`, `area/harness-maintenance`): no sanctioned writer for
implementation-task completion in schema-2 state; `record-failure` rejects schema 2;
`check-dispatch` refuses a non-canonical state source. Booking the gap grants no
authority; the owner install command above still stands.

## 6. State publication gap (blocks T3 launch)

`.omp/plans/execution-state.json` still records `tasks[T2].status = "pending"` with
`evidenceIds: []`, so `check-dispatch` reports T3 not on the frontier
(`E_TASK_NOT_FRONTIER`, "dependency lacks valid terminal observations").

The harness has **no writer** for implementation-task completion: `complete` is
feature-level and acceptance-backed; `authorize` (execution-amendment) only amends
effects and clears blockers and explicitly refuses a `done` task; `record-failure`
refuses this state with `E_STATE_INVALID` because it accepts only the schema-1 task map.
T1's completion was recorded in the state document itself (evidence record + status +
evidenceIds), so the same publication is the only available path for T2.

Proposed T2 publication (mirrors T1's record shape exactly):

```json
{"id":"ev-t2-tests-20261127","kind":"test",
 "artifact":{"path":"/home/opencode/projects/default/mcp-bridge/.omp/artifacts/t2-acceptance/acceptance.log",
             "sha256":"e23074af9af28b5e9577c81a36123ea25ebb5ab2bb0cdc5ae7674af71ba906da"},
 "feature":"token-efficient-tool-responses",
 "packetDigest":"53a6908b6e97798a07907f95185e939bfe9c5a2830be1d0dc104135a308309d3",
 "selectedBranchId":"token-efficient-tool-responses-ticket01-r40",
 "taskIds":["T2"],
 "acceptanceIds":["AC4","AC5","AC7","AC8","AC13","AC14","AC16","AC18","AC36","AC38","AC39"],
 "outcome":"pass","observedAt":"<now>",
 "producer":"engineer:T2 (dispatch ImplementTicket01T2Attempt4 + resumes), verified by coordinator",
 "details":{"command":"go build ./... && go vet ./internal/service/ ./internal/observe/ && go test ./internal/observe/ -count=1 && go test ./internal/service/ -run '<T2 oracles>' -count=1","exitCode":0},
 "contractAnchor":"<state packet contract anchor>"}
```

plus `tasks[T2].status = "done"` and `tasks[T2].evidenceIds = ["ev-t2-tests-20261127"]`.

Owner authorized publication. The coordinator's write is then refused by the dispatch
gate itself: `edit .omp/plans/execution-state.json` → `E_COORDINATOR_EDIT_FORBIDDEN`
("Coordinator direct edit and write tool calls targeting production repository paths
are prohibited. Delegate changes to engineer."), and no CLI writer exists to delegate
to. Candidate document therefore prepared and validated but NOT installed:

- `.omp/artifacts/execution-state.t2-done.json` — current state plus the
  `ev-t2-tests-20261127` record and `tasks[T2] = {status: done, evidenceIds: [ev-t2-tests-20261127]}`.
- `node workflow-policy/execution-state.mjs check <candidate>` → exit 0,
  `{"ok":true,"status":"ready","runnable_frontier":["CR1","CT1","T3"]}`.
- T3 payload: `check-payload` → `{"ok":true,"shape":"batch"}`.
- `check-dispatch` against the candidate is refused `E_STATE_SOURCE` because the
  request must name the canonical state path; T3 preflight can only run after install.

Install (owner action, one command):

```
cp .omp/artifacts/execution-state.t2-done.json .omp/plans/execution-state.json
```

Then `prepare-dispatch` on `.omp/artifacts/token-eff01-dispatch-t3-seed.json` and launch
the emitted payload. Until then T3 stays blocked; its payload is authored and
lint-clean.

## 7. T3 dispatch (authored, frontier-blocked)

Task `T3`, role `engineer`, 19 declared effect paths, acceptance ids AC3, AC6,
AC9–AC11, AC14–AC17, AC24–AC32, AC37, AC40, AC43, AC44. Seed, request and native
payload at `.omp/artifacts/token-eff01-dispatch-t3-{seed,request,payload}.json`.
Objective and write roots are spec §5 T3 verbatim; the store seam is §5 above.
