import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { withJournal } from '/home/opencode/.omp/agent/workflow-policy/execution-lock.mjs';
import { validateShape } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-schema.mjs';
import { evaluateContract } from '/home/opencode/.omp/agent/workflow-policy/execution-contract.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const statePath = path.join(root, '.omp/plans/execution-state.json');
const artifactRel = '.omp/artifacts/startup-diagnostics/owner-authorization-r8.json';
const artifactPath = path.join(root, artifactRel);
const sha = createHash('sha256').update(readFileSync(artifactPath)).digest('hex');
const state = JSON.parse(readFileSync(statePath, 'utf8'));
const artifact = JSON.parse(readFileSync(artifactPath, 'utf8'));
if (artifact.packetDigest !== state.packet.digest || artifact.selectedBranchId !== state.packet.selectedBranchId) throw new Error('artifact does not match canonical packet');
const recordedAt = new Date().toISOString();
const sourceId = 'src-owner-startup-diagnostics';
const source = {
  id: sourceId,
  kind: 'user-message',
  artifact: { path: artifactPath, sha256: sha },
  locator: 'chat: simplify the plan and start with startup diagnostics; retries; Go with A; critic',
  recordedAt,
};
const authorization = {
  mode: 'milestone',
  id: 'owner-authorize-startup-diagnostics-r8',
  packetDigest: state.packet.digest,
  selectedBranchId: state.packet.selectedBranchId,
  receipt: {
    sourceId,
    recordedAt,
    response: {
      origin: 'user',
      explicit: true,
      timedOut: false,
      automatic: false,
      packetDigest: state.packet.digest,
      selectedBranchId: state.packet.selectedBranchId,
      milestone: true,
    },
  },
};
const next = { ...state, sources: [...state.sources.filter(s => s.id !== sourceId), source], authorization };
const errors = validateShape(next);
if (errors.length) throw new Error('schema: ' + JSON.stringify(errors));
const report = evaluateContract(next, { readFileSync });
if (report.status === 'invalid') throw new Error('contract: ' + JSON.stringify(report.reasons));
const bytes = Buffer.from(`${JSON.stringify(next, null, 2)}\n`);
const result = withJournal(statePath, { feature: state.feature, lockOptions: {} }, snapshot => {
  const live = JSON.parse(snapshot.state.bytes.toString('utf8'));
  if (live.packet.digest !== state.packet.digest) return { noMutation: true, value: { ok: false, detail: 'packet changed' } };
  return { stateBytes: bytes, value: { ok: true, sourceId, artifactSha256: sha } };
});
console.log(JSON.stringify({ result, status: report.status, authority_ready: report.authority_ready }, null, 2));
