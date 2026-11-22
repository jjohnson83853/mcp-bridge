import { readFileSync, writeFileSync, renameSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { canonicalJson, sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';
import { withStateLock } from '/home/opencode/.omp/agent/workflow-policy/execution-lock.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const statePath = path.join(root, '.omp/plans/execution-state.json');
const directive = path.join(root, '.omp/artifacts/owner-authorization-snapshot-recovery-t1-20261121.md');
const BRANCH = 'snapshot-recovery-r30';
const TASK_IDS = ['T1', 'T2', 'T3', 'T4'];
const SOURCE_ID = 'src-owner-authorization-snapshot-recovery-t1';
const AUTH_ID = 'owner-bounded-snapshot-recovery-t1';
const directiveBytes = readFileSync(directive);

const result = withStateLock(statePath, {}, { readFileSync, writeFileSync, renameSync }, () => {
  const state = JSON.parse(readFileSync(statePath, 'utf8'));
  if (state.schemaVersion !== 2 || state.feature !== 'snapshot-recovery' || state.revision !== 'r30') throw new Error('unexpected canonical state');
  if (state.authorization !== null || state.packet.selectedBranchId !== null) throw new Error('authority already recorded');
  const branchDef = state.packet.branches.find(item => item.id === BRANCH);
  if (!branchDef) throw new Error('target branch missing');
  const branches = state.packet.branches.map(item => item.id === BRANCH ? { ...item, taskIds: TASK_IDS } : item);
  const packetDigest = sha(canonicalJson({ feature: state.feature, projectRoot: state.projectRoot, sectionBindings: state.spec.sectionBindings, branches }));
  const recordedAt = new Date().toISOString();
  const next = {
    ...state,
    packet: { ...state.packet, digest: packetDigest, branches, selectedBranchId: BRANCH },
    sources: [...state.sources, { id: SOURCE_ID, kind: 'user-message', artifact: { path: directive, sha256: sha(directiveBytes) }, locator: 'owner directive: select the reviewed branch and dispatch engineer for T1', recordedAt }],
    authorization: {
      mode: 'bounded',
      id: AUTH_ID,
      packetDigest,
      selectedBranchId: BRANCH,
      receipt: { sourceId: SOURCE_ID, recordedAt, response: { origin: 'user', explicit: true, timedOut: false, automatic: false, packetDigest, selectedBranchId: BRANCH } },
    },
  };
  const temp = `${statePath}.${randomUUID()}.tmp`;
  writeFileSync(temp, JSON.stringify(next, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
  renameSync(temp, statePath);
  return { branch: BRANCH, authId: AUTH_ID, sourceId: SOURCE_ID, recordedAt, packetDigest };
});
console.log(JSON.stringify(result, null, 2));
