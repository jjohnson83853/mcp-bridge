import { readFileSync, writeFileSync, renameSync } from 'node:fs';
import { createHash } from 'node:crypto';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const H = b => createHash('sha256').update(b).digest('hex');
const canonical = v => Array.isArray(v) ? '[' + v.map(canonical).join(',') + ']'
  : v && typeof v === 'object' ? '{' + Object.keys(v).sort().map(k => JSON.stringify(k) + ':' + canonical(v[k])).join(',') + '}' : JSON.stringify(v);

const root = process.cwd();
const regPath = path.join(root, '.omp/plans/review-sequence.json');
const specPath = path.join(root, '.scratch/header-parameter-transit/spec.md');
const seed = JSON.parse(readFileSync(path.join(root, '.omp/artifacts/header-transit-review-seed-cr2.json'), 'utf8'));
const req = seed.workflowDispatch.requests[0];
const dispatch = {
  name: req.name, agent: req.agent, reviewerId: 'code-reviewer', taskId: req.taskId,
  requestSha256: H(canonical(req)), taskSha256: H(seed.tasks[0].task),
  instructionsSha256: H(seed.instructions),
  targetCandidateId: 'header-parameter-transit-spec-r2', targetRevision: 'r2',
};
const mod = await import(pathToFileURL('/home/opencode/.omp/agent/workflow-policy/execution-review-sequence.mjs').href);
const result = mod.withReviewRegistryLock(regPath, {}, { readFileSync, writeFileSync, renameSync }, () => {
  const reg = JSON.parse(readFileSync(regPath, 'utf8'));
  const r1 = reg.candidates.find(c => c.id === 'header-parameter-transit-spec-r1');
  if (!r1) throw new Error('r1 candidate missing');
  const existing = reg.candidates.find(c => c.id === 'header-parameter-transit-spec-r2');
  const inputs = r1.inputs.map(rec => rec.path === specPath ? { path: specPath, sha256: H(readFileSync(specPath)) } : rec);
  if (existing) { existing.dispatches = [dispatch]; }
  else reg.candidates.push({ id: 'header-parameter-transit-spec-r2', revision: 'r2', authorIds: [...r1.authorIds], inputs, dispatches: [dispatch] });
  const tmp = regPath + '.tmp';
  writeFileSync(tmp, JSON.stringify(reg, null, 2) + '\n');
  renameSync(tmp, regPath);
  return { candidateId: 'header-parameter-transit-spec-r2', specSha: inputs[0].sha256 };
});
console.log(JSON.stringify(result, null, 2));
