// Register the r5 review candidate and its planned descriptors in the
// project review registry. Metadata only: preserves existing candidates and
// history; does not record a verdict or observation.
import { readFileSync, writeFileSync, copyFileSync, renameSync, existsSync } from 'node:fs';
import { createHash, randomUUID } from 'node:crypto';
import path from 'node:path';

const root = '/home/opencode/projects/default/mcp-bridge';
const registryPath = path.join(root, '.omp/plans/review-sequence.json');
const crSeedPath = path.join(root, '.omp/artifacts/breaker-managed-code-review-seed-r5.json');
const ctSeedPath = path.join(root, '.omp/artifacts/breaker-managed-critic-seed-r5.json');
const candidateId = 'upstream-breaker-recovery-spec-r5';
const revision = 'r5';

const sha = value => createHash('sha256').update(value).digest('hex');
const canonicalJson = value => {
  if (Array.isArray(value)) return `[${value.map(canonicalJson).join(',')}]`;
  if (value && typeof value === 'object') {
    return `{${Object.keys(value).sort().map(k => `${JSON.stringify(k)}:${canonicalJson(value[k])}`).join(',')}}`;
  }
  return JSON.stringify(value);
};
const digestFile = file => sha(readFileSync(file));

// Freeze the planner artifact under its canonical name.
const plannerSrc = path.join(root, '.omp/artifacts/breaker-planner-r5-412b6f9a6cd2-20261120T030413662Z.md');
const plannerDst = path.join(root, '.omp/artifacts/upstream-breaker-recovery-planner-r5.md');
if (!existsSync(plannerDst)) copyFileSync(plannerSrc, plannerDst);

const inputPaths = [
  '.scratch/upstream-breaker-recovery/spec.md',
  'docs/agents/domain.md',
  'docs/agents/issue-tracker.md',
  '.scratch/upstream-breaker-recovery/issues/01-breaker-discovery-probe-recovery.md',
  '.scratch/upstream-breaker-recovery/backlog.md',
  '.scratch/upstream-breaker-recovery/incident-2026-11-14.md',
  'internal/upstream/runtime.go',
  'internal/upstream/runtime_test.go',
  'internal/upstream/runtime_edge_test.go',
  'internal/upstream/dedicated.go',
  'internal/upstream/dedicated_test.go',
  'internal/service/refresh.go',
  'internal/service/reload.go',
  'cmd/mcp-bridge/wiring.go',
  'internal/observe/log.go',
  'internal/observe/envelope.go',
  'internal/observe/metrics.go',
  '.omp/artifacts/upstream-breaker-recovery-planner-r5.md',
];
const inputs = inputPaths.map(rel => {
  const abs = path.join(root, rel);
  return { path: abs, sha256: digestFile(abs) };
});

const cr = JSON.parse(readFileSync(crSeedPath, 'utf8'));
const ct = JSON.parse(readFileSync(ctSeedPath, 'utf8'));
const descriptor = (seed, reviewerId, taskId) => {
  const request = seed.workflowDispatch.requests[0];
  return {
    name: request.name,
    agent: request.agent,
    reviewerId,
    taskId,
    requestSha256: sha(canonicalJson(request)),
    targetCandidateId: candidateId,
    targetRevision: revision,
    taskSha256: sha(seed.tasks[0].task),
    instructionsSha256: sha(seed.instructions),
  };
};

const registry = JSON.parse(readFileSync(registryPath, 'utf8'));
if (registry.candidates.some(item => item.id === candidateId)) throw new Error('candidate already registered');
registry.candidates.push({
  id: candidateId,
  revision,
  authorIds: ['planner'],
  maxRounds: 4,
  inputs,
  dispatches: [descriptor(cr, 'code-reviewer', 'CR1'), descriptor(ct, 'critic', 'CT1')],
  targetRevision: revision,
});

const tmp = `${registryPath}.${randomUUID()}.tmp`;
writeFileSync(tmp, JSON.stringify(registry, null, 2) + '\n', { flag: 'wx', mode: 0o664 });
renameSync(tmp, registryPath);

console.log(JSON.stringify({
  candidateId,
  revision,
  inputCount: inputs.length,
  dispatches: registry.candidates.find(c => c.id === candidateId).dispatches,
}, null, 2));
