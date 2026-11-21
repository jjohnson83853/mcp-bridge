import { readFileSync, writeFileSync, renameSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { withStateLock } from '/home/opencode/.omp/agent/workflow-policy/execution-lock.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const statePath = path.join(root, '.omp/plans/execution-state.json');

const result = withStateLock(statePath, {}, { readFileSync, writeFileSync, renameSync }, () => {
  const state = JSON.parse(readFileSync(statePath, 'utf8'));
  if (state.schemaVersion !== 2 || state.feature !== 'header-parameter-transit' || state.revision !== 'r6') throw new Error('unexpected canonical state');
  const auth = state.authorization;
  if (!auth || auth.mode !== 'bounded' || auth.receipt?.sourceId !== 'src-owner-authorization-t1') throw new Error('owner authorization missing');
  if (state.reviews !== null) throw new Error('reviews already recorded');
  const reason = 'Owner accepted r6 residuals: CodeReviewer r6 APPROVE with three residual findings and Critic r6 REVISE with C1-C6 (C1/C3 HIGH). No passing Critic verdict exists; owner explicitly directed proceeding for T1.';
  const receipt = { sourceId: auth.receipt.sourceId, recordedAt: auth.receipt.recordedAt, response: { ...auth.receipt.response } };
  const next = { ...state, reviews: {
    specification: { verdict: 'waived', reason, receipt: { ...receipt, response: { ...receipt.response } } },
    tasks: { verdict: 'waived', reason, receipt: { ...receipt, response: { ...receipt.response } } },
  } };
  const temp = `${statePath}.${randomUUID()}.tmp`;
  writeFileSync(temp, JSON.stringify(next, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
  renameSync(temp, statePath);
  return { reviews: Object.keys(next.reviews), sourceId: auth.receipt.sourceId };
});
console.log(JSON.stringify(result, null, 2));
