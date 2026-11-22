import { readFileSync, writeFileSync } from 'node:fs';

const root = '/home/opencode/projects/default/mcp-bridge';
const task = readFileSync(root + '/.omp/artifacts/snapshot-recovery-fresh-plan/round-17-task.txt', 'utf8');
const instructions = 'Round 17 (r27) closing revision for feature snapshot-recovery. Specification-only; no implementation exists. The three findings in the task are the bound r26 CodeReviewer output. Textual corrections only: no R/AC/T id add/remove/rename, no envelope path change, no new dependency. Do not edit files or run tests. Return numbered delta blocks with exact old/new text plus a short handoff.';
const seed = {
  intent: 'Snapshot-recovery specification round 17 (r27)',
  instructions,
  workflowDispatch: { statePath: root + '/.omp/plans/execution-state.json', schemaVersion: 1, projectRoot: root, feature: 'snapshot-recovery', revision: 'r26', requests: [{ name: 'SnapshotRecoveryRound17', taskId: 'PLAN1', agent: 'planner', effects: { kind: 'read', paths: [] } }] },
  tasks: [{ name: 'SnapshotRecoveryRound17', agent: 'planner', task, solutionSpace: 'close three residual wording findings in a fixed spec; no id, no envelope change' }],
};
writeFileSync(root + '/.omp/artifacts/snapshot-recovery-fresh-plan/review-seed-plan17.json', JSON.stringify(seed, null, 2) + '\n');
console.log('seed written', task.length);
