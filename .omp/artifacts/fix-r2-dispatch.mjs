import { readFileSync, writeFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
const H = b => createHash('sha256').update(b).digest('hex');
const canonical = v => Array.isArray(v) ? '[' + v.map(canonical).join(',') + ']'
  : v && typeof v === 'object' ? '{' + Object.keys(v).sort().map(k => JSON.stringify(k) + ':' + canonical(v[k])).join(',') + '}' : JSON.stringify(v);
const seed1 = JSON.parse(readFileSync('.omp/artifacts/header-transit-review-seed-cr1.json', 'utf8'));
const reg = JSON.parse(readFileSync('.omp/plans/review-sequence.json', 'utf8'));
const r1 = reg.candidates.find(c => c.id === 'header-parameter-transit-spec-r1');
const d1 = r1.dispatches.find(d => d.name === 'ReviewHeaderSpecR1');
const reqEntry = seed1.workflowDispatch.requests[0];
const v = {
  request: H(canonical(reqEntry)) === d1.requestSha256,
  task: H(seed1.tasks[0].task) === d1.taskSha256,
  instructions: H(seed1.instructions) === d1.instructionsSha256,
  candidate: H(canonical({...r1.inputs})) === H(canonical(r1.inputs)),
};
console.log(JSON.stringify(v));
if (!v.request || !v.task || !v.instructions) throw new Error('algorithm mismatch');
const seed2 = JSON.parse(readFileSync('.omp/artifacts/header-transit-review-seed-cr2.json', 'utf8'));
const req2 = seed2.workflowDispatch.requests[0];
const d = {
  name: req2.name, agent: req2.agent, reviewerId: 'code-reviewer', taskId: req2.taskId,
  requestSha256: H(canonical(req2)), taskSha256: H(seed2.tasks[0].task),
  instructionsSha256: H(seed2.instructions),
  targetCandidateId: 'header-parameter-transit-spec-r2', targetRevision: 'r2',
};
console.log(JSON.stringify(d, null, 2));
const mod = await import('./workflow-policy/execution-review-sequence.mjs').catch(() => import('file:///home/opencode/.omp/agent/workflow-policy/execution-review-sequence.mjs'));
const { writeFileSync: wfs, renameSync } = await import('node:fs');
const path = (await import('node:path')).default;
const regPath = path.resolve('.omp/plans/review-sequence.json');
mod.withReviewRegistryLock(regPath, {}, { readFileSync, writeFileSync: wfs, renameSync }, () => {
  const cur = JSON.parse(readFileSync(regPath, 'utf8'));
  const c = cur.candidates.find(x => x.id === 'header-parameter-transit-spec-r2');
  c.dispatches = [d];
  const tmp = regPath + '.tmp';
  writeFileSync(tmp, JSON.stringify(cur, null, 2) + '\n');
  return () => { renameSync(tmp, regPath); return c.id; };
});
console.log('dispatches updated');
