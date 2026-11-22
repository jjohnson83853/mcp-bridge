import { readFileSync, writeFileSync, renameSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { withStateLock } from '/home/opencode/.omp/agent/workflow-policy/execution-lock.mjs';

const statePath = path.join('/home/opencode/projects/default/mcp-bridge', '.omp/plans/execution-state.json');
const result = withStateLock(statePath, {}, { readFileSync, writeFileSync, renameSync }, () => {
  const state = JSON.parse(readFileSync(statePath, 'utf8'));
  const seen = new Set();
  const sources = state.sources.filter(source => !seen.has(source.id) && seen.add(source.id));
  const removed = state.sources.length - sources.length;
  if (!removed) return { removed: 0 };
  const temp = statePath + '.' + randomUUID() + '.tmp';
  writeFileSync(temp, JSON.stringify({ ...state, sources }, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
  renameSync(temp, statePath);
  return { removed };
});
console.log(JSON.stringify(result));