import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { withJournal } from '/home/opencode/.omp/agent/workflow-policy/execution-lock.mjs';
import { canonicalJson, sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const newRev = process.argv[2];
if (!/^r\d+$/.test(newRev ?? '')) throw new Error('usage: project-snapshot-state.mjs <rN>');
const statePath = path.join(root, '.omp/plans/execution-state.json');
const specRel = '.scratch/snapshot-recovery/spec.md';
const specBytes = readFileSync(path.join(root, specRel));
const spec = specBytes.toString('utf8');
const currentBytes = readFileSync(statePath);
const current = JSON.parse(currentBytes.toString('utf8'));
if (current.feature !== 'snapshot-recovery') throw new Error('unexpected feature');
writeFileSync(path.join(root, `.omp/artifacts/snapshot-recovery-fresh-plan/archive-before-review-${newRev}-${sha(currentBytes)}.json`), currentBytes, { flag: 'wx' });
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
const next = { ...current, revision: newRev, spec: { ...current.spec, sha256: sha(specBytes), sectionBindings }, packet: { ...current.packet, digest: packetDigest, selectedBranchId: current.packet.selectedBranchId, branches } };
const projectionPath = path.join(root, `.omp/artifacts/snapshot-recovery-fresh-plan/snapshot-recovery-state-${newRev}-review-projection.json`);
writeFileSync(projectionPath, JSON.stringify(next, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
const result = withJournal(statePath, { feature: 'snapshot-recovery', lockOptions: {} }, snapshot => {
  if (!snapshot.state.bytes) return { noMutation: true, value: { ok: false, detail: 'canonical state missing' } };
  const live = JSON.parse(snapshot.state.bytes.toString('utf8'));
  if (live.revision !== current.revision || live.packet.digest !== current.packet.digest) return { noMutation: true, value: { ok: false, detail: 'canonical state changed during projection' } };
  return { stateBytes: JSON.stringify(next, null, 2) + '\n', value: { ok: true, revision: newRev, packetDigest } };
});
console.log(JSON.stringify({ result, packetDigest, projectionPath }, null, 2));
