import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { withJournal } from '/home/opencode/.omp/agent/workflow-policy/execution-lock.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const newRev = process.argv[2];
if (!/^r\d+$/.test(newRev ?? '')) throw new Error('usage: project-r2.mjs <rN>');
const statePath = path.join(root, '.omp/plans/execution-state.json');
const nextPath = path.join(root, '.omp/plans/config-admission-resilience/execution-state.json');
const nextBytes = readFileSync(nextPath);
const currentBytes = readFileSync(statePath);
const current = JSON.parse(currentBytes.toString('utf8'));
const next = JSON.parse(nextBytes.toString('utf8'));
if (current.feature !== 'config-admission-resilience' || current.revision === newRev) throw new Error('unexpected canonical state');
if (next.feature !== current.feature || next.revision !== newRev) throw new Error('projected state does not match requested revision');
const result = withJournal(statePath, { feature: current.feature, lockOptions: {} }, snapshot => {
  if (!snapshot.state.bytes) return { noMutation: true, value: { ok: false, detail: 'canonical state missing' } };
  const live = JSON.parse(snapshot.state.bytes.toString('utf8'));
  if (live.revision !== current.revision || live.packet.digest !== current.packet.digest) return { noMutation: true, value: { ok: false, detail: 'canonical state changed during projection' } };
  return { stateBytes: nextBytes, value: { ok: true, revision: newRev } };
});
writeFileSync(path.join(root, '.omp/artifacts/config-admission-resilience/archive-before-' + newRev + '-' + current.packet.digest + '.json'), currentBytes, { flag: 'wx', mode: 0o600 });
console.log(JSON.stringify(result, null, 2));
