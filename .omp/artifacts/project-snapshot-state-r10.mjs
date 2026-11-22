import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { withJournal } from '/home/opencode/.omp/agent/workflow-policy/execution-lock.mjs';
import { canonicalJson, sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const statePath = path.join(root, '.omp/plans/execution-state.json');
const specRel = '.scratch/snapshot-recovery/spec.md';
const specPath = path.join(root, specRel);
const specBytes = readFileSync(specPath);
const spec = specBytes.toString('utf8');
const current = JSON.parse(readFileSync(statePath, 'utf8'));
if (current.revision !== 'r9' || current.feature !== 'snapshot-recovery') throw new Error('unexpected canonical state');

const sectionNames = ['requirements', 'design', 'tasks', 'envelope'];
const sectionBindings = {};
let last = -1;
for (const name of sectionNames) {
  const start = `<!-- specdd:${name}:start -->`;
  const end = `<!-- specdd:${name}:end -->`;
  const a = spec.indexOf(start);
  const b = spec.indexOf(end);
  if (a <= last || a < 0 || b < a + start.length) throw new Error(`missing/out-of-order section ${name}`);
  sectionBindings[name] = { revision: 'r10', sha256: sha(spec.slice(a + start.length, b)) };
  last = b + end.length - 1;
}

const reqStart = spec.indexOf('<!-- specdd:requirements:start -->');
const reqEnd = spec.indexOf('<!-- specdd:requirements:end -->');
const summaries = {};
for (const line of spec.slice(reqStart, reqEnd).split('\n')) {
  const m = /^- (R\d+)\s+(.+)$/.exec(line);
  if (m) summaries[m[1]] = m[2].trim();
}
if (Object.keys(summaries).length !== 9) throw new Error('requirement summaries incomplete');

const branch = {
  id: 'snapshot-recovery-r10',
  answerIds: [],
  taskIds: ['CR1', 'CT1', 'T1', 'T2', 'T3'],
  envelopeId: 'env-1',
  sectionBindings,
};
const branches = current.packet.branches.filter(item => item.id !== branch.id).concat([branch]);
const oldPacket = {
  feature: current.feature,
  projectRoot: current.projectRoot,
  sectionBindings: current.spec.sectionBindings,
  branches: current.packet.branches,
};
if (sha(canonicalJson(oldPacket)) !== current.packet.digest) throw new Error('existing packet digest does not reproduce');
const packetDigest = sha(canonicalJson({
  feature: current.feature,
  projectRoot: current.projectRoot,
  sectionBindings,
  branches,
}));

const acceptance = current.acceptance;
const t2Paths = [
  'cmd/mcp-bridge/serve.go',
  'cmd/mcp-bridge/snapshot_recovery_test.go',
  'cmd/mcp-bridge/serve_edge_test.go',
  'cmd/mcp-bridge/startup_failure_test.go',
  'cmd/mcp-bridge/config_envelope_test.go',
  'internal/observe/envelope.go',
  'internal/observe/log.go',
  'internal/observe/log_test.go',
  'internal/observe/edge_test.go',
  'docs/logging-contract.md',
];
const tasks = current.tasks.map(task => {
  if (task.id === 'T2') return { ...task, effects: { ...task.effects, paths: t2Paths } };
  if (task.id === 'T3') return { ...task, blockedBy: ['T1', 'T2'] };
  return task;
});
const next = {
  ...current,
  revision: 'r10',
  spec: { ...current.spec, sha256: sha(specBytes), sectionBindings },
  packet: { ...current.packet, digest: packetDigest, selectedBranchId: branch.id, branches },
  requirements: current.requirements.map(item => ({ ...item, summary: summaries[item.id] ?? item.summary })),
  acceptance,
  tasks,
};
const projectionPath = path.join(root, '.omp/artifacts/snapshot-recovery-state-r10-projection.json');
writeFileSync(projectionPath, JSON.stringify(next, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
const result = withJournal(statePath, { feature: 'snapshot-recovery', lockOptions: {} }, snapshot => {
  if (!snapshot.state.bytes) return { noMutation: true, value: { ok: false, detail: 'canonical state missing' } };
  const live = JSON.parse(snapshot.state.bytes.toString('utf8'));
  if (live.revision !== 'r9' || live.packet.digest !== current.packet.digest) {
    return { noMutation: true, value: { ok: false, detail: 'canonical state changed during projection' } };
  }
  return { stateBytes: JSON.stringify(next, null, 2) + '\n', value: { ok: true, revision: 'r10', packetDigest, projectionPath } };
});
console.log(JSON.stringify({ result, packetDigest, projectionPath, acceptance: acceptance.length, t2Paths: t2Paths.length }, null, 2));
