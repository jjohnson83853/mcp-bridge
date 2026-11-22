// Bind the emitted r25 dispatch payload hashes into the review registry.
import { readFileSync, writeFileSync, renameSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { withReviewRegistryLock } from '/home/opencode/.omp/agent/workflow-policy/execution-review-sequence.mjs';
import { sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const registryPath = path.join(root, '.omp/plans/review-sequence.json');
const pairs = [
  ['cr', 'SnapshotRecoverySpecR25CodeReview'],
  ['ct', 'SnapshotRecoverySpecR25Critic'],
];
const prepared = Object.fromEntries(pairs.map(([key, name]) => {
  const file = path.join(root, `.omp/artifacts/snapshot-recovery-fresh-plan/review-prepared-${key}-r25.json`);
  const emitted = JSON.parse(readFileSync(file, 'utf8'));
  if (emitted.ok !== true || emitted.dispatch_allowed !== true) throw new Error(`prepare not allowed for ${key}`);
  const wrapper = JSON.parse(emitted.payload.context);
  const item = emitted.payload.tasks.find(t => t.name === name);
  if (!item) throw new Error(`missing task ${name}`);
  return [name, { taskSha256: sha(item.task), instructionsSha256: sha(wrapper.instructions) }];
}));

const result = withReviewRegistryLock(registryPath, {}, { readFileSync, writeFileSync, renameSync }, () => {
  const registry = JSON.parse(readFileSync(registryPath, 'utf8'));
  const candidate = registry.candidates.find(c => c.id === 'snapshot-recovery-spec-r25');
  if (!candidate || (registry.reviewHistory ?? []).some(r => r.candidateId === candidate.id)) throw new Error('candidate missing or reviewed');
  candidate.dispatches = candidate.dispatches.map(d => {
    const hashes = prepared[d.name];
    if (!hashes) throw new Error(`unexpected dispatch ${d.name}`);
    return { ...d, ...hashes };
  });
  const temp = `${registryPath}.${randomUUID()}.tmp`;
  writeFileSync(temp, JSON.stringify(registry, null, 2) + '\n');
  renameSync(temp, registryPath);
  return candidate.dispatches.map(d => ({ name: d.name, taskSha256: d.taskSha256, instructionsSha256: d.instructionsSha256 }));
});
console.log(JSON.stringify({ result }, null, 2));
