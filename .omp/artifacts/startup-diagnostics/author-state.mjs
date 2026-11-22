import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';

const root = path.resolve(process.argv[2] ?? process.cwd());
const outPath = process.argv[3] ?? path.join(root, '.omp/plans/startup-diagnostics/execution-state.json');
const planPath = process.argv[4] ?? path.join(root, '.omp/artifacts/startup-diagnostics/plan.json');
const sha = value => createHash('sha256').update(value).digest('hex');
const canonicalValue = value => {
  if (Array.isArray(value)) return `[${value.map(canonicalValue).join(',')}]`;
  if (value !== null && typeof value === 'object') return `{${Object.keys(value).sort().map(key => `${JSON.stringify(key)}:${canonicalValue(value[key])}`).join(',')}}`;
  return JSON.stringify(value);
};

const feature = 'startup-diagnostics';
const revision = process.argv[5] ?? 'r1';
const specPath = path.join(root, '.scratch/startup-diagnostics/spec.md');
const content = readFileSync(specPath, 'utf8');
const specSha = sha(readFileSync(specPath));
const sectionBindings = {};
for (const name of ['requirements', 'design', 'tasks', 'envelope']) {
  const start = `<!-- specdd:${name}:start -->`;
  const end = `<!-- specdd:${name}:end -->`;
  const a = content.indexOf(start);
  const b = content.indexOf(end);
  if (a < 0 || b < a) throw new Error(`section ${name} markers missing`);
  sectionBindings[name] = { revision, sha256: sha(content.slice(a + start.length, b)) };
}

const plan = JSON.parse(readFileSync(planPath, 'utf8'));
const reviewTasks = [
  { id: 'CR1', stage: 'research', blockedBy: [], effects: { kind: 'read', paths: [path.join(root, '.scratch/startup-diagnostics/spec.md')] }, acceptanceIds: [], agentRoles: ['code-reviewer'] },
  { id: 'CT1', stage: 'research', blockedBy: [], effects: { kind: 'read', paths: [path.join(root, '.scratch/startup-diagnostics/spec.md')] }, acceptanceIds: [], agentRoles: ['critic'] },
];
const allTasks = [...reviewTasks, ...plan.tasks];
const branch = {
  id: `${feature}-${revision}`,
  answerIds: [],
  taskIds: allTasks.map(t => t.id),
  envelopeId: 'env-1',
  sectionBindings,
};
const packet = { feature, projectRoot: root, sectionBindings, branches: [branch] };
const state = {
  schemaVersion: 2,
  feature,
  revision,
  projectRoot: root,
  spec: { path: specPath, sha256: specSha, sectionBindings },
  packet: { digest: sha(canonicalValue(packet)), branches: [branch], selectedBranchId: branch.id },
  actors: { plannerId: 'planner', implementerIds: ['engineer'] },
  requirements: plan.requirements,
  authorization: null,
  reviews: null,
  sources: [],
  decisions: [],
  envelope: plan.envelope,
  tasks: allTasks.map(t => ({ status: 'pending', evidenceIds: [], requires: [], acceptanceIds: [], agentRoles: [], ...t })),
  capabilities: [],
  acceptance: plan.acceptance,
  evidence: [],
  blockers: [],
  gate: {
    markerPath: path.join(root, '.omp/artifacts/startup-diagnostics/GATE_OPEN.json'),
    activationEvidenceIds: [],
  },
};
mkdirSync(path.dirname(outPath), { recursive: true });
writeFileSync(outPath, JSON.stringify(state, null, 2) + '\n');
console.log(JSON.stringify({ outPath, specSha, packetDigest: state.packet.digest, sectionBindings }));
