import { readFileSync, writeFileSync, renameSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { sha, gateMarkerForState } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';
import { withStateLock } from '/home/opencode/.omp/agent/workflow-policy/execution-lock.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const statePath = path.join(root, '.omp/plans/execution-state.json');
const directive = path.join(root, '.omp/artifacts/owner-authorization-header-transit-ticket03-20261121.md');
const BRANCH = 'header-parameter-transit-r6';
const SOURCE_ID = 'src-owner-authorization-t3';
const AUTH_ID = 'owner-bounded-header-parameter-transit-t3';
const bytes = readFileSync(directive);

const result = withStateLock(statePath, {}, { readFileSync, writeFileSync, renameSync }, () => {
  const state = JSON.parse(readFileSync(statePath, 'utf8'));
  if (state.schemaVersion !== 2 || state.feature !== 'header-parameter-transit' || state.revision !== 'r6') throw new Error('unexpected canonical state');
  if (!state.authorization || state.authorization.id !== 'owner-bounded-header-parameter-transit-t2') throw new Error('unexpected prior authorization');
  if (state.packet.selectedBranchId !== BRANCH) throw new Error('unexpected selected branch');
  if (state.sources.some(source => source.id === SOURCE_ID)) throw new Error('source already recorded');
  const recordedAt = new Date().toISOString();
  const next = {
    ...state,
    sources: [...state.sources, {
      id: SOURCE_ID,
      kind: 'user-message',
      artifact: { path: directive, sha256: sha(bytes) },
      locator: 'owner directive: authorize ticket 03 and dispatch it',
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
  const marker = gateMarkerForState(next);
  writeFileSync(path.join(root, '.superpowers/sdd/GATE_OPEN'), JSON.stringify(marker, null, 2) + '\n');
  return { authId: AUTH_ID, sourceId: SOURCE_ID, recordedAt, marker };
});
console.log(JSON.stringify(result, null, 2));
