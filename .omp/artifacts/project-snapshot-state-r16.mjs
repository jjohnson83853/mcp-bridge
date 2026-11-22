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
const currentBytes = readFileSync(statePath);
const current = JSON.parse(currentBytes.toString('utf8'));
if (current.revision !== 'r16' || current.feature !== 'snapshot-recovery') throw new Error('unexpected canonical state');

writeFileSync(path.join(root, `.omp/artifacts/snapshot-recovery-fresh-plan/archive-before-review-r16-${sha(currentBytes)}.json`), currentBytes, { flag: 'wx' });

const sectionNames = ['requirements', 'design', 'tasks', 'envelope'];
const sectionBindings = {};
let last = -1;
for (const name of sectionNames) {
  const start = `<!-- specdd:${name}:start -->`;
  const end = `<!-- specdd:${name}:end -->`;
  const a = spec.indexOf(start), b = spec.indexOf(end);
  if (a <= last || a < 0 || b < a + start.length) throw new Error(`missing/out-of-order section ${name}`);
  sectionBindings[name] = { revision: 'r16', sha256: sha(spec.slice(a + start.length, b)) };
  last = b + end.length - 1;
}

const oldPacket = { feature: current.feature, projectRoot: current.projectRoot, sectionBindings: current.spec.sectionBindings, branches: current.packet.branches };
if (sha(canonicalJson(oldPacket)) !== current.packet.digest) throw new Error('existing packet digest does not reproduce');

const oldBranch = current.packet.branches.find(b => b.id === 'snapshot-recovery-r15') ?? current.packet.branches[0];
if (!oldBranch) throw new Error('no existing branch');
const branch = { id: 'snapshot-recovery-r16', answerIds: oldBranch.answerIds, taskIds: oldBranch.taskIds, envelopeId: oldBranch.envelopeId, sectionBindings };
const branches = [branch];
const packetDigest = sha(canonicalJson({ feature: current.feature, projectRoot: current.projectRoot, sectionBindings, branches }));

const next = { ...current, revision: 'r16', spec: { ...current.spec, sha256: sha(specBytes), sectionBindings }, packet: { ...current.packet, digest: packetDigest, selectedBranchId: current.packet.selectedBranchId, branches } };
const projectionPath = path.join(root, '.omp/artifacts/snapshot-recovery-fresh-plan/snapshot-recovery-state-r16-review-projection.json');
writeFileSync(projectionPath, JSON.stringify(next, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
const result = withJournal(statePath, { feature: 'snapshot-recovery', lockOptions: {} }, snapshot => {
  if (!snapshot.state.bytes) return { noMutation: true, value: { ok: false, detail: 'canonical state missing' } };
  const live = JSON.parse(snapshot.state.bytes.toString('utf8'));
  if (live.revision !== 'r16' || live.packet.digest !== current.packet.digest) return { noMutation: true, value: { ok: false, detail: 'canonical state changed during projection' } };
  return { stateBytes: JSON.stringify(next, null, 2) + '\n', value: { ok: true, revision: 'r16', packetDigest } };
});
console.log(JSON.stringify({ result, packetDigest, projectionPath, sectionBindings }, null, 2));
