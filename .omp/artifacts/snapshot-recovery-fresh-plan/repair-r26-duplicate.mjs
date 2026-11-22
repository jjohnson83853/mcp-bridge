import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { withJournal } from '/home/opencode/.omp/agent/workflow-policy/execution-lock.mjs';
import { sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const statePath = path.join(root, '.omp/plans/execution-state.json');
const bytes = readFileSync(statePath);
const current = JSON.parse(bytes.toString('utf8'));
if (current.revision !== 'r26' || current.feature !== 'snapshot-recovery') throw new Error('unexpected state');
writeFileSync(path.join(root, `.omp/artifacts/snapshot-recovery-fresh-plan/archive-before-r26-repair-${sha(bytes)}.json`), bytes, { flag: 'wx' });
const order = [];
const byId = new Map();
for (const task of current.tasks) {
  if (!byId.has(task.id)) order.push(task.id);
  byId.set(task.id, task);
}
const tasks = order.map(id => byId.get(id));
const ids = tasks.map(t => t.id);
if (new Set(ids).size !== ids.length) throw new Error('duplicate survived');
if (!ids.includes('T4')) throw new Error('T4 missing');
const next = { ...current, tasks };
const result = withJournal(statePath, { feature: 'snapshot-recovery', lockOptions: {} }, snapshot => {
  const live = JSON.parse(snapshot.state.bytes.toString('utf8'));
  if (live.revision !== current.revision) return { noMutation: true, value: { ok: false, detail: 'changed' } };
  return { stateBytes: JSON.stringify(next, null, 2) + '\n', value: { ok: true, tasks: ids } };
});
console.log(JSON.stringify({ result }, null, 2));
