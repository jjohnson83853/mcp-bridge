import { readFileSync, writeFileSync, renameSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';
import { withStateLock } from '/home/opencode/.omp/agent/workflow-policy/execution-lock.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const statePath = path.join(root, '.omp/plans/execution-state.json');
const directive = path.join(root, '.omp/artifacts/owner-authorization-header-transit-ticket01-20261121.md');
const BRANCH = 'header-parameter-transit-r6';
const SOURCE_ID = 'src-owner-authorization-t1';
const AUTH_ID = 'owner-bounded-header-parameter-transit-t1';
const directiveBytes = readFileSync(directive);

const result = withStateLock(statePath, {}, { readFileSync, writeFileSync, renameSync }, () => {
  const state = JSON.parse(readFileSync(statePath, 'utf8'));
  if (state.schemaVersion !== 2 || state.feature !== 'header-parameter-transit' || state.revision !== 'r6') throw new Error('unexpected canonical state');
  if (state.authorization !== null || state.packet.selectedBranchId !== null) throw new Error('authority already recorded');
  const branch = state.packet.branches.find(item => item.id === BRANCH);
  if (!branch) throw new Error('target branch missing');
  const recordedAt = new Date().toISOString();
  const next = {
    ...state,
    packet: { ...state.packet, selectedBranchId: BRANCH },
    sources: [...state.sources, {
      id: SOURCE_ID,
      kind: 'user-message',
      artifact: { path: directive, sha256: sha(directiveBytes) },
      locator: 'owner directive: accept the residuals and authorize ticket 01',
      recordedAt,
    }],
    authorization: {
      mode: 'bounded',
      id: AUTH_ID,
      packetDigest: state.packet.digest,
      selectedBranchId: BRANCH,
      receipt: {
        sourceId: SOURCE_ID,
        recordedAt,
        response: {
          origin: 'user', explicit: true, timedOut: false, automatic: false,
          packetDigest: state.packet.digest, selectedBranchId: BRANCH,
        },
      },
    },
  };
  const temp = `${statePath}.${randomUUID()}.tmp`;
  writeFileSync(temp, JSON.stringify(next, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
  renameSync(temp, statePath);
  return { branch: BRANCH, authId: AUTH_ID, sourceId: SOURCE_ID, recordedAt, packetDigest: state.packet.digest };
});
console.log(JSON.stringify(result, null, 2));
