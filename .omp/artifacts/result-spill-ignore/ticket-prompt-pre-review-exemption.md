# Continuation prompt — file harness ticket for pre-review owner exemptions

Harness Git repository root: /home/opencode/.omp
Harness runtime tree: /home/opencode/.omp/agent
Tracker lives in the gitignored local backlog: /home/opencode/.omp/agent/.scratch

Your only task is to file ONE draft backlog ticket in that local harness tracker. Do not implement a fix, dispatch planner/engineer/reviewer, or modify canonical execution state, review registries, runs, evidence or counters.

## Correct paths

- Backlog index: /home/opencode/.omp/agent/.scratch/BACKLOG.md
- Ticket directory: /home/opencode/.omp/agent/.scratch/mechanical-coordination/issues/
- Next free number: 33 (31 = owner-directed Critic findings waiver; 32 = cross-project backlog access)
- Git-tracked harness paths are repo-relative with the `agent/` prefix, for example:
  - agent/workflow-policy/execution-waiver.mjs
  - agent/workflow-policy/execution-waiver.test.mjs
  - agent/workflow-policy/execution-state.mjs
- Do not treat /home/opencode/.omp/agent as the Git root. Git operations, diffs and commits use /home/opencode/.omp.
- `.scratch/` is gitignored (`agent/.scratch/` in /home/opencode/.omp); the ticket itself is local input, not a committed artifact. The eventual harness fix commits only agent/workflow-policy code/tests.
- Match the shape of /home/opencode/.omp/agent/.scratch/mechanical-coordination/issues/31-owner-directed-critic-waiver-with-bug-reports.md and add one new row to the BACKLOG.md ranking table.

## Ticket subject

Owner-approved pre-review exemption is unrecordable: the waiver registry cannot represent a waived review that has no Critic observation, so a one-line maintenance task blocks at E_WAIVER.

## Observed evidence (2026-11-22)

Feature `result-spill-ignore`, revision `r1`, task `ENG1` (.gitignore-only maintenance under a single-writer owner maintenance exception).

Project state (evidence only; not part of the harness repo):
/home/opencode/projects/default/mcp-bridge/.omp/plans/execution-state.json

- `reviews.specification` and `reviews.tasks` are both `{"verdict":"waived", ... "receipt":{"sourceId":"src-owner-result-spill-ignore", ... "response":{"origin":"user", "explicit":true, "packetDigest":"5c4b3ea61a1dfb89cd4492dbfe687a25f5476870e5e09c2e7e1ba4a80adf91bf", "selectedBranchId":"result-spill-ignore-r1"}}}`.
- `/home/opencode/projects/default/mcp-bridge/.omp/plans/result-spill-ignore/waiver-registry.json` does not exist.
- No candidate/revision r1 exists in `/home/opencode/projects/default/mcp-bridge/.omp/plans/review-sequence.json` for `result-spill-ignore` (filter returned `[]`).
- `node /home/opencode/.omp/agent/workflow-policy/execution-state.mjs complete /home/opencode/projects/default/mcp-bridge/.omp/plans/execution-state.json` exits 1 with `status:"blocked"` and `E_WAIVER` — "state-side waiver has no registry entry" for both checkpoints. ENG1 stays pending.
- The supported writer cannot bridge the gap:
  - `record-waiver` without a request requires exactly one waived checkpoint (`requestForState`), but both are waived.
  - `record-waiver` with an explicit request reaches `observationFor` then `deriveWaiverTargets`; both require a recorded Critic observation with findings (`role:"critic"`, inputs, findings). None exists because the owner exempted reviews.
  - `waiverCheck` re-runs `completeDisposition`, which again requires `observationFor` and a matching derived entry.
- Diagnostic artifact (project-side evidence): /home/opencode/projects/default/mcp-bridge/.omp/artifacts/result-spill-ignore/waiver-repair-blocked-20261122T022400Z.json

Code anchors in the harness tree (read, do not edit):
- /home/opencode/.omp/agent/workflow-policy/execution-waiver.mjs — `waiverEntryShape`:28, `requestForState`:169, `observationFor`:190, `deriveWaiverTargets`:44, `completeDisposition`:282, `waiverCheck`:328, `bindWaivedReview`:365, `recordWaiver`:431
- /home/opencode/.omp/agent/workflow-policy/execution-state.mjs — `record-waiver` CLI:1062
- /home/opencode/.omp/agent/workflow-policy/execution-waiver.test.mjs and execution-contract.test.mjs cover current waiver behavior

## Expected vs actual

Expected: an owner maintenance approval that explicitly exempts a change from pre-review must be recordable as a zero-report registry disposition bound to the current owner receipt, packet digest, branch, feature, revision and checkpoint. The gate accepts that checkpoint; the engine task completes under the remaining prerequisites.

Actual: the only waiver producer conflates "owner accepts residual Critic findings" with "owner exempts this change from review" and requires a Critic observation and findings. There is no representation for a pre-review exemption, so the waiver contract is impossible to satisfy.

Impact: a trivial .gitignore-only maintenance change needs a full independent spec + Critic review chain solely to satisfy the waiver record, and otherwise deadlocks at completion. Harness contract defect, not application defect.

## Ticket required content

1. Title; `**Status:** draft-backlog - filed <date> from owner-reported one-line maintenance deadlock. No implementation or policy change is authorized by this filing.`
2. Symptom, exact commands/results, evidence paths.
3. Root cause: `execution-waiver.mjs` requires a Critic observation for every waiver; owner exemption has no observation; registry and gate therefore refuse. Distinguish from ticket 31 (post-review waiver of recorded MEDIUM/HIGH findings), which must keep requiring an observation.
4. Required behavior (draft; planner owns final design):
   - Two explicit bases: `owner-exemption` (no review observation; zero reports; binds owner receipt + packet digest + branch + feature/revision/checkpoint) and `finding-disposition` (existing behavior; requires Critic REVISE/BLOCKED observation and report set).
   - `record-waiver` can record multiple checkpoints without requiring exactly one waived checkpoint; idempotent replay.
   - `waiverCheck` accepts owner-exemption entries without `review-sequence.json` candidate/observation prerequisites, but still validates owner receipt, packet digest, selected branch, feature/revision/checkpoint and the state-side binding.
   - Existing finding-disposition behavior and all E_WAIVER/E_AUTHORIZATION/E_PATH_ESCAPE refusals unchanged.
   - No fabricated Critic output, findings, PASS or reviews; no gate weakening; no direct canonical-state edit.
5. Draft acceptance criteria (planner refines): owner-exemption records with no candidate/observation and completes; forged/stale/mismatched receipt still denied; finding-disposition without observation still denied; missing registry still denies; idempotent duplicate; exact byte/history preservation; focused RED/GREEN tests in `agent/workflow-policy/execution-waiver.test.mjs` plus completion/contract coverage; no changes to unrelated gates, counters or review runs.
6. Relationships: relates to 31 (findings waiver), 07/02; supersedes nothing; do not reopen 31.
7. Explicit non-authorization: filing only. Do not fix the live result-spill-ignore state, create registry entries, record ENG1 completion, commit or push.

## Applying the ticket later (context, not authorization now)

A future authorized harness session applies it at /home/opencode/.omp with these repo-relative targets:
- agent/workflow-policy/execution-waiver.mjs
- agent/workflow-policy/execution-waiver.test.mjs
- agent/workflow-policy/execution-state.mjs (only if the CLI/request contract must change)
- contract/completion test fixtures only where a changed waiver shape requires them

Snapshot each edited file first; RED/GREEN; run the narrow unfiltered suite with absolute paths, for example:
`node --test /home/opencode/.omp/agent/workflow-policy/execution-waiver.test.mjs /home/opencode/.omp/agent/workflow-policy/execution-contract.test.mjs`
and the completion/contract tests for any fixture change. Activation proof and live result-spill-ignore replay are not part of ticket filing.

## Done when

- Exactly one new ticket `/home/opencode/.omp/agent/.scratch/mechanical-coordination/issues/33-<slug>.md` exists with the above content.
- `/home/opencode/.omp/agent/.scratch/BACKLOG.md` has one new ranking row linking it (place after item 31 or at the most defensible prevention rank; state the placement is a recommendation).
- No other files changed.
- Report the two paths and confirm no implementation/state/registry mutation occurred.
