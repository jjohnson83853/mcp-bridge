import { readFileSync, writeFileSync, renameSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { canonicalJson, sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';
import { withStateLock } from '/home/opencode/.omp/agent/workflow-policy/execution-lock.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const statePath = path.join(root, '.omp/plans/execution-state.json');
const specPath = path.join(root, '.scratch/header-parameter-transit/spec.md');
const SECTIONS = ['requirements', 'design', 'tasks', 'envelope'];
const specBytes = readFileSync(specPath);
const specText = specBytes.toString('utf8');
const bindings = {};
let last = -1;
for (const name of SECTIONS) {
  const start = `<!-- specdd:${name}:start -->`;
  const end = `<!-- specdd:${name}:end -->`;
  const a = specText.indexOf(start);
  const b = specText.indexOf(end);
  if (a < 0 || b < a + start.length || a <= last) throw new Error('markers ' + name);
  bindings[name] = { revision: 'r6', sha256: sha(specText.slice(a + start.length, b)) };
  last = b + end.length - 1;
}

const result = withStateLock(statePath, {}, { readFileSync, writeFileSync, renameSync }, () => {
  const state = JSON.parse(readFileSync(statePath, 'utf8'));
  if (state.schemaVersion !== 2 || state.feature !== 'header-parameter-transit' || state.revision !== 'r5') throw new Error('unexpected canonical state');
  if (state.reviews !== null || state.authorization !== null || state.packet.selectedBranchId !== null) throw new Error('pending authority drifted');
  const branches = state.packet.branches.filter(branch => branch.id !== 'header-parameter-transit-r6');
  branches.push({
    id: 'header-parameter-transit-r6',
    answerIds: [],
    taskIds: state.packet.branches[0].taskIds.slice(),
    envelopeId: 'env-1',
    sectionBindings: JSON.parse(JSON.stringify(bindings)),
  });
  const next = {
    ...state,
    revision: 'r6',
    spec: { ...state.spec, sha256: sha(specBytes), sectionBindings: JSON.parse(JSON.stringify(bindings)) },
    packet: {
      ...state.packet,
      branches,
      digest: sha(canonicalJson({ feature: state.feature, projectRoot: state.projectRoot, sectionBindings: bindings, branches })),
    },
  };
  const temp = `${statePath}.${randomUUID()}.tmp`;
  writeFileSync(temp, JSON.stringify(next, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
  renameSync(temp, statePath);
  return { revision: next.revision, specSha: next.spec.sha256, packetDigest: next.packet.digest, branches: branches.map(b => b.id), bindings };
});
console.log(JSON.stringify(result, null, 2));
