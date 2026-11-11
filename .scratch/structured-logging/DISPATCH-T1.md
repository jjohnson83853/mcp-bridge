# Dispatch card — structured-logging T1

State is `ready`; `T1` is the only runnable task. The managed payload is already built and checked.

## Paste this into a fresh session rooted at /home/opencode/projects/default/mcp-bridge

> Dispatch T1 now. Read `.scratch/structured-logging/payload-t1.json` and call the `task` tool with exactly its `i`, `context`, and `tasks` fields — do not edit, re-serialize, or re-run `prepare-dispatch`. Then report the engineer's result and the receipt path.

## Why a fresh session

The running session loaded the workflow-policy extension before the harness change, so its in-memory schema rejects `reviews.tasks.verdict: "waived"` with `E_SCHEMA`. A new process loads the current policy. Do not "fix" the state to satisfy the stale schema.

## Preflight (optional, fresh process)

```bash
node /home/opencode/.omp/agent/workflow-policy/execution-state.mjs check-dispatch \
  .omp/plans/execution-state.json \
  .scratch/structured-logging/dispatch-request-t1-nostate.json \
  .scratch/structured-logging/payload-t1.json
```
Exit 0 = admissible. The request file must NOT contain `statePath`.

## What the dispatch does

- Task `T1`, agent `engineer`, effect kind `write`, 14 exact paths (13 sources/tests + `.omp/artifacts/receipt_T1.json`).
- Scope: envelope v1 builder + `EnvelopeFieldNames()`, ordered pin-loop ceiling attribution, `--check-config` single JSON line, AC01-1..AC01-8 tests.
- The engineer writes `.omp/artifacts/receipt_T1.json`; T2..T7 stay blocked until T1 has a valid terminal observation.

## If the state changes

Any edit to `.omp/plans/execution-state.json` invalidates the payload. Rebuild:

```bash
node .scratch/structured-logging/build-dispatch-t1.mjs
node /home/opencode/.omp/agent/workflow-policy/execution-state.mjs prepare-dispatch \
  .scratch/structured-logging/dispatch-seed-t1.json > .scratch/structured-logging/prepared-t1.json
```
Then re-extract `payload` from `prepared-t1.json` and re-run `check-dispatch`.
