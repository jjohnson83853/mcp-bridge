import { readFileSync, writeFileSync, renameSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { withReviewRegistryLock } from '/home/opencode/.omp/agent/workflow-policy/execution-review-sequence.mjs';
import { canonicalJson, sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = process.cwd();
const registryPath = path.join(root, '.omp', 'plans', 'review-sequence.json');
const cr = JSON.parse(readFileSync(path.join(root, '.omp/artifacts/header-transit-review-seed-cr1.json'), 'utf8'));
const ct = JSON.parse(readFileSync(path.join(root, '.omp/artifacts/header-transit-review-seed-ct1.json'), 'utf8'));
if (cr.instructions !== ct.instructions) throw new Error('instructions differ');
const rows = readFileSync(path.join(root, '.omp/artifacts/header-transit-sha.txt'), 'utf8').split('\n').filter(Boolean).map(line => line.split(/\s+/, 2));
const inputs = rows.map(([hash, rel]) => ({ path: root + '/' + rel, sha256: hash }));
const crReq = cr.workflowDispatch.requests[0];
const ctReq = ct.workflowDispatch.requests[0];
const candidate = {
  id: 'header-parameter-transit-spec-r1',
  revision: 'r1',
  authorIds: ['planner'],
  maxRounds: 4,
  inputs,
  targetRevision: 'r1',
  dispatches: [
    { name: crReq.name, agent: crReq.agent, reviewerId: crReq.agent, taskId: crReq.taskId,
      requestSha256: sha(canonicalJson(crReq)), targetCandidateId: crReq.targetCandidateId, targetRevision: crReq.targetRevision,
      taskSha256: sha(cr.tasks[0].task), instructionsSha256: sha(cr.instructions) },
    { name: ctReq.name, agent: ctReq.agent, reviewerId: ctReq.agent, taskId: ctReq.taskId,
      requestSha256: sha(canonicalJson(ctReq)), targetCandidateId: ctReq.targetCandidateId, targetRevision: ctReq.targetRevision,
      taskSha256: sha(ct.tasks[0].task), instructionsSha256: sha(ct.instructions) },
  ],
};
const result = withReviewRegistryLock(registryPath, {}, {}, () => {
  const registry = JSON.parse(readFileSync(registryPath, 'utf8'));
  if (registry.schemaVersion !== 1 || !Array.isArray(registry.candidates)) throw new Error('registry shape');
  registry.candidates = registry.candidates.filter(item => item?.id !== candidate.id).concat([candidate]);
  const temp = registryPath + '.' + randomUUID() + '.tmp';
  writeFileSync(temp, JSON.stringify(registry, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
  renameSync(temp, registryPath);
  return { candidates: registry.candidates.length };
});
console.log(JSON.stringify({ result, cr: candidate.dispatches[0], ct: candidate.dispatches[1] }, null, 2));
