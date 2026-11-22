import { readFileSync, writeFileSync, renameSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import { withReviewRegistryLock } from '/home/opencode/.omp/agent/workflow-policy/execution-review-sequence.mjs';
import { sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const rev = process.argv[2];
const kind = process.argv[3];
const d = root + '/.omp/artifacts/snapshot-recovery-fresh-plan/';
const prepared = JSON.parse(readFileSync(d + 'review-prepared-' + kind + '-' + rev + '.json', 'utf8'));
const wrapper = JSON.parse(prepared.payload.context);
const item = prepared.payload.tasks[0];
const registryPath = root + '/.omp/plans/review-sequence.json';
withReviewRegistryLock(registryPath, {}, { readFileSync, writeFileSync, renameSync }, () => {
  const registry = JSON.parse(readFileSync(registryPath, 'utf8'));
  const candidate = registry.candidates.find(c => c.id === 'snapshot-recovery-spec-' + rev);
  if (!candidate) throw new Error('candidate missing');
  candidate.dispatches = candidate.dispatches.map(x => x.name === item.name ? { ...x, taskSha256: sha(item.task), instructionsSha256: sha(wrapper.instructions) } : x);
  const temp = registryPath + '.' + randomUUID() + '.tmp';
  writeFileSync(temp, JSON.stringify(registry, null, 2) + '\n');
  renameSync(temp, registryPath);
  return { ok: true };
});
console.log('synced ' + item.name);
