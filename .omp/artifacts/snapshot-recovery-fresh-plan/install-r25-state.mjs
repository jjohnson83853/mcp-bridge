// Install snapshot-recovery r25 into canonical execution state (owner-authorized, envelope write set unchanged).
import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { withJournal } from '/home/opencode/.omp/agent/workflow-policy/execution-lock.mjs';
import { canonicalJson, sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const newRev = process.argv[2];
const expectedPrev = process.argv[3];
if (!/^r\d+$/.test(newRev ?? '') || !/^r\d+$/.test(expectedPrev ?? '')) throw new Error('usage: install-r25-state.mjs <newRev> <expectedPrev>');
const statePath = path.join(root, '.omp/plans/execution-state.json');
const specBytes = readFileSync(path.join(root, '.scratch/snapshot-recovery/spec.md'));
const spec = specBytes.toString('utf8');
const currentBytes = readFileSync(statePath);
const current = JSON.parse(currentBytes.toString('utf8'));
if (current.feature !== 'snapshot-recovery' || current.revision !== expectedPrev) throw new Error('unexpected canonical state');

// 1. Byte-identical preservation of the outgoing state (never overwrite).
const archive = path.join(root, `.omp/artifacts/snapshot-recovery-fresh-plan/archive-before-${newRev}-state-${sha(currentBytes)}.json`);
try { writeFileSync(archive, currentBytes, { flag: 'wx' }); } catch (error) { if (error?.code !== 'EEXIST') throw error; }
if (sha(readFileSync(archive)) !== sha(currentBytes)) throw new Error('archive verification failed');

// 2. Section bindings.
const names = ['requirements', 'design', 'tasks', 'envelope'];
const sectionBindings = {};
let last = -1;
for (const name of names) {
  const start = `<!-- specdd:${name}:start -->`, end = `<!-- specdd:${name}:end -->`;
  const a = spec.indexOf(start), b = spec.indexOf(end);
  if (a <= last || a < 0 || b < a + start.length) throw new Error(`missing/out-of-order section ${name}`);
  sectionBindings[name] = { revision: newRev, sha256: sha(spec.slice(a + start.length, b)) };
  last = b + end.length - 1;
}

const oldPacket = { feature: current.feature, projectRoot: current.projectRoot, sectionBindings: current.spec.sectionBindings, branches: current.packet.branches };
if (sha(canonicalJson(oldPacket)) !== current.packet.digest) throw new Error('existing packet digest does not reproduce');
const oldBranch = current.packet.branches.find(b => b.id === `snapshot-recovery-${current.revision}`) ?? current.packet.branches[0];
if (!oldBranch) throw new Error('no existing branch');
const branch = { id: `snapshot-recovery-${newRev}`, answerIds: oldBranch.answerIds, taskIds: oldBranch.taskIds, envelopeId: oldBranch.envelopeId, sectionBindings };
const branches = [branch];
const packetDigest = sha(canonicalJson({ feature: current.feature, projectRoot: current.projectRoot, sectionBindings, branches }));

// 3. Task and acceptance ownership (r25 four-ticket decomposition).
const byId = new Map(current.tasks.map(t => [t.id, t]));
for (const id of ['T1', 'T2', 'T3']) if (!byId.has(id)) throw new Error(`missing ${id}`);
const effect = (paths, kind = 'write') => ({ kind, paths });
const replacements = {
  T1: { ...byId.get('T1'), effects: effect(['internal/storage/recovery.go', 'internal/storage/recovery_test.go', 'internal/storage/store.go', 'internal/storage/store_edge_test.go']), acceptanceIds: ['AC1-1', 'AC1-2', 'AC1-3', 'AC1-4', 'AC1-7'] },
  T2: { ...byId.get('T2'), blockedBy: ['T1'], effects: effect(['internal/storage/recovery.go', 'internal/storage/recovery_test.go', 'internal/storage/store_edge_test.go']), acceptanceIds: ['AC1-5', 'AC1-6', 'AC1-8', 'AC1-9', 'AC1-10'] },
  T3: { ...byId.get('T3'), blockedBy: ['T1', 'T2'], effects: effect(['internal/storage/recovery.go', 'internal/storage/recovery_test.go', 'internal/observe/envelope.go', 'internal/observe/log.go', 'internal/observe/log_test.go', 'internal/observe/edge_test.go', 'docs/logging-contract.md', 'cmd/mcp-bridge/config_envelope_test.go']), acceptanceIds: ['AC2-9'] },
};
const tasks = [
  ...current.tasks.map(t => replacements[t.id] ?? t),
  { id: 'T4', stage: 'implementation', status: 'pending', blockedBy: ['T1', 'T2', 'T3'], requires: [], effects: effect(['cmd/mcp-bridge/serve.go', 'cmd/mcp-bridge/snapshot_recovery_test.go', 'cmd/mcp-bridge/serve_edge_test.go', 'cmd/mcp-bridge/startup_failure_test.go', 'internal/storage/recovery_test.go']), acceptanceIds: ['AC2-1', 'AC2-2', 'AC2-3', 'AC2-4', 'AC2-5', 'AC2-6', 'AC2-7', 'AC2-8'], evidenceIds: [], agentRoles: ['engineer'] },
];
{ const seen = new Set(); const unique = tasks.filter(t => !seen.has(t.id) && seen.add(t.id)); tasks.length = 0; tasks.push(...unique); }

const acceptanceOwners = new Map([['AC1-1', 'T1'], ['AC1-2', 'T1'], ['AC1-3', 'T1'], ['AC1-4', 'T1'], ['AC1-7', 'T1'], ['AC1-5', 'T2'], ['AC1-6', 'T2'], ['AC1-8', 'T2'], ['AC1-9', 'T2'], ['AC1-10', 'T2'], ['AC2-9', 'T3'], ...['AC2-1', 'AC2-2', 'AC2-3', 'AC2-4', 'AC2-5', 'AC2-6', 'AC2-7', 'AC2-8'].map(id => [id, 'T4'])]);
const acceptance = current.acceptance.map(a => ({ ...a, ownerTaskIds: [acceptanceOwners.get(a.id) ?? a.ownerTaskIds?.[0] ?? 'T1'] }));

// 4. Envelope: same read/write paths; T4 added as a task id only.
const envelope = { ...current.envelope, allowedTaskIds: [...new Set([...current.envelope.allowedTaskIds, 'T4'])] };
for (const task of tasks.filter(t => t.effects.kind === 'write')) for (const p of task.effects.paths) if (!envelope.writePaths.includes(p)) throw new Error(`task path outside envelope: ${p}`);
const covered = new Set(tasks.filter(t => t.effects.kind === 'write').flatMap(t => t.effects.paths));
for (const p of envelope.writePaths) if (!covered.has(p)) throw new Error(`envelope path uncovered: ${p}`);

const next = { ...current, revision: newRev, tasks, acceptance, envelope, spec: { ...current.spec, sha256: sha(specBytes), sectionBindings }, packet: { ...current.packet, digest: packetDigest, branches, selectedBranchId: null }, authorization: null };
const projectionPath = path.join(root, `.omp/artifacts/snapshot-recovery-fresh-plan/snapshot-recovery-state-${newRev}-review-projection.json`);
writeFileSync(projectionPath, JSON.stringify(next, null, 2) + '\n', { flag: 'wx', mode: 0o600 });

const result = withJournal(statePath, { feature: 'snapshot-recovery', lockOptions: {} }, snapshot => {
  if (!snapshot.state.bytes) return { noMutation: true, value: { ok: false, detail: 'canonical state missing' } };
  const live = JSON.parse(snapshot.state.bytes.toString('utf8'));
  if (live.revision !== current.revision || live.packet.digest !== current.packet.digest) return { noMutation: true, value: { ok: false, detail: 'canonical state changed during projection' } };
  return { stateBytes: JSON.stringify(next, null, 2) + '\n', value: { ok: true, revision: newRev, packetDigest, archive } };
});
console.log(JSON.stringify({ result, packetDigest, projectionPath, archive, sectionBindings }, null, 2));
