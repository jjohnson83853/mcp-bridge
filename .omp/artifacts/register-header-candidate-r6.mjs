import { readFileSync, writeFileSync, renameSync, existsSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { withReviewRegistryLock } from '/home/opencode/.omp/agent/workflow-policy/execution-review-sequence.mjs';
import { canonicalJson, sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const config = '.omp/plans/review-sequence.json';
const specPath = path.join(root, '.scratch/header-parameter-transit/spec.md');
const cr = JSON.parse(readFileSync(path.join(root, '.omp/artifacts/header-transit-review-seed-cr6.json'), 'utf8'));
const ct = JSON.parse(readFileSync(path.join(root, '.omp/artifacts/header-transit-review-seed-ct6.json'), 'utf8'));
if (cr.instructions !== ct.instructions) throw new Error('shared instructions differ');

const descriptor = (seed, agent, reviewerId) => {
  const req = seed.workflowDispatch.requests[0];
  return {
    name: req.name, agent: req.agent, reviewerId, taskId: req.taskId,
    requestSha256: sha(canonicalJson(req)),
    targetCandidateId: req.targetCandidateId, targetRevision: req.targetRevision,
    taskSha256: sha(seed.tasks[0].task),
    instructionsSha256: sha(seed.instructions),
  };
};

const readInputs = () => {
  const registry = JSON.parse(readFileSync(path.join(root, config), 'utf8'));
  const r5 = registry.candidates.find(c => c.id === 'header-parameter-transit-spec-r5');
  if (!r5) throw new Error('r5 candidate missing');
  const inputs = r5.inputs.map(rec => ({ path: rec.path, sha256: rec.path === specPath ? sha(readFileSync(specPath)) : rec.sha256 }));
  const refresh = path.join(root, 'internal/service/refresh_test.go');
  if (!inputs.some(i => i.path === refresh)) inputs.push({ path: refresh, sha256: sha(readFileSync(refresh)) });
  return inputs;
};

const candidate = {
  id: 'header-parameter-transit-spec-r6',
  revision: 'r6',
  authorIds: ['planner'],
  maxRounds: 4,
  inputs: readInputs(),
  targetRevision: 'r6',
  dispatches: [descriptor(cr, 'code-reviewer', 'code-reviewer'), descriptor(ct, 'critic', 'critic')],
};

const result = withReviewRegistryLock(path.join(root, config), {}, { readFileSync, writeFileSync, renameSync }, () => {
  const registry = JSON.parse(readFileSync(path.join(root, config), 'utf8'));
  if (registry.schemaVersion !== 1 || !Array.isArray(registry.candidates)) throw new Error('registry shape');
  const existing = registry.candidates.find(item => item.id === candidate.id);
  if (existing) {
    const rounds = (registry.reviewHistory ?? []).filter(r => r.candidateId === candidate.id);
    if (existing.codeReview || rounds.length) throw new Error('candidate already reviewed; refusing overwrite');
  }
  registry.candidates = registry.candidates.filter(item => item.id !== candidate.id).concat([candidate]);
  const temp = `${path.join(root, config)}.${randomUUID()}.tmp`;
  writeFileSync(temp, JSON.stringify(registry, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
  renameSync(temp, path.join(root, config));
  return { candidates: registry.candidates.length, candidateId: candidate.id, inputs: candidate.inputs.length };
});

for (const [name, seed] of [['cr6', cr], ['ct6', ct]]) {
  const file = path.join(root, '.omp/artifacts/header-transit-request-' + name + '.json');
  writeFileSync(file, JSON.stringify(seed.workflowDispatch, null, 2) + '\n');
}
console.log(JSON.stringify({ result, cr: candidate.dispatches[0], ct: candidate.dispatches[1] }, null, 2));
