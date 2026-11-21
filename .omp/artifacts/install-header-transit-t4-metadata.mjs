import { readFileSync, writeFileSync, renameSync, existsSync } from 'node:fs';
import { randomUUID, createHash } from 'node:crypto';
import path from 'node:path';
import { sha, canonicalJson, gateMarkerForState } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';
import { withStateLock } from '/home/opencode/.omp/agent/workflow-policy/execution-lock.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const statePath = path.join(root, '.omp/plans/execution-state.json');
const directive = path.join(root, '.omp/artifacts/owner-authorization-header-transit-ticket04-20261121.md');
const RUNBOOK = 'docs/runbooks/header-parameters.md';
const BRANCH = 'header-parameter-transit-r6';
const SOURCE_ID = 'src-owner-authorization-t4';
const AUTH_ID = 'owner-bounded-header-parameter-transit-t4';
const bytes = readFileSync(directive);

const result = withStateLock(statePath, {}, { readFileSync, writeFileSync, renameSync }, () => {
  const original = readFileSync(statePath);
  const originalHash = createHash('sha256').update(original).digest('hex');
  const snapshot = path.join(root, `.omp/artifacts/hpt-state-r6-pre-t4-${originalHash}.json`);
  if (!existsSync(snapshot)) writeFileSync(snapshot, original, { flag: 'wx', mode: 0o600 });
  const state = JSON.parse(original.toString('utf8'));
  if (state.schemaVersion !== 2 || state.feature !== 'header-parameter-transit' || state.revision !== 'r6') throw new Error('unexpected canonical state');
  if (!state.authorization || state.authorization.id !== 'owner-bounded-header-parameter-transit-t3') throw new Error('unexpected prior authorization');
  if (state.packet.selectedBranchId !== BRANCH) throw new Error('unexpected selected branch');
  if (state.sources.some(source => source.id === SOURCE_ID)) throw new Error('source already recorded');
  const recordedAt = new Date().toISOString();
  const decisionId = `owner-metadata-exception:T4:${sha(canonicalJson({ sourceId: SOURCE_ID, artifact: sha(bytes) }))}`;
  const next = {
    ...state,
    sources: [...state.sources, {
      id: SOURCE_ID,
      kind: 'user-message',
      artifact: { path: directive, sha256: sha(bytes) },
      locator: 'owner directives: authorize ticket 04 and dispatch it; owner-metadata-exception',
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
    envelope: {
      ...state.envelope,
      readPaths: [...new Set([...state.envelope.readPaths, RUNBOOK])],
    },
    tasks: state.tasks.map(task => task.id === 'T4'
      ? { ...task, agentRoles: ['operator'] }
      : task),
    decisions: [...state.decisions.filter(decision => decision.id !== decisionId), {
      id: decisionId,
      kind: 'owner',
      status: 'resolved',
      question: 'Correct T4 agent role and read path so the owner-authorized deployed verification can be dispatched?',
      answer: 'Owner selected the one-time metadata exception: T4 agentRoles [operator], runbook added to envelope.readPaths.',
      rationale: 'Installed authorize amendment accepts only write/test tasks and transition refuses same-feature replacement; owner explicitly authorized this bounded metadata correction.',
      sourceIds: [SOURCE_ID],
      affects: ['T4'],
    }],
  };
  const temp = `${statePath}.${randomUUID()}.tmp`;
  writeFileSync(temp, JSON.stringify(next, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
  renameSync(temp, statePath);
  const marker = gateMarkerForState(next);
  writeFileSync(path.join(root, '.superpowers/sdd/GATE_OPEN'), JSON.stringify(marker, null, 2) + '\n');
  return { authId: AUTH_ID, decisionId, snapshot, originalHash, recordedAt, marker };
});
console.log(JSON.stringify(result, null, 2));
