// Build the upstream-breaker-recovery r5 candidate state from the planner r5
// artifact. Writes only .scratch files; canonical installation happens through
// `execution-state.mjs transition`.
import { readFileSync, writeFileSync, renameSync, copyFileSync } from 'node:fs';
import { createHash, randomUUID } from 'node:crypto';
import path from 'node:path';

const root = '/home/opencode/projects/default/mcp-bridge';
const src = path.join(root, '.omp/artifacts/breaker-planner-r5-412b6f9a6cd2-20261120T030413662Z.md');
const specPath = path.join(root, '.scratch/upstream-breaker-recovery/spec.md');
const templatePath = path.join(root, '.scratch/upstream-breaker-recovery/new-state-r4.json');
const outPath = path.join(root, '.scratch/upstream-breaker-recovery/new-state-r5.json');

const sha = buf => createHash('sha256').update(buf).digest('hex');
const canonicalJson = value => {
  if (Array.isArray(value)) return `[${value.map(canonicalJson).join(',')}]`;
  if (value && typeof value === 'object') {
    return `{${Object.keys(value).sort().map(k => `${JSON.stringify(k)}:${canonicalJson(value[k])}`).join(',')}}`;
  }
  return JSON.stringify(value);
};

const specBytes = readFileSync(src);
const specText = specBytes.toString('utf8');
const SECTIONS = ['requirements', 'design', 'tasks', 'envelope'];
const sectionBindings = {};
for (const name of SECTIONS) {
  const start = `<!-- specdd:${name}:start -->`;
  const end = `<!-- specdd:${name}:end -->`;
  const a = specText.indexOf(start);
  const b = specText.indexOf(end);
  if (a < 0 || b < a) throw new Error(`missing section ${name}`);
  sectionBindings[name] = { revision: 'r5', sha256: sha(Buffer.from(specText.slice(a + start.length, b), 'utf8')) };
}

// Persist the planner-authored revision; atomic replace.
const tmpSpec = `${specPath}.${randomUUID()}.tmp`;
writeFileSync(tmpSpec, specBytes, { flag: 'wx', mode: 0o644 });
renameSync(tmpSpec, specPath);

const state = JSON.parse(readFileSync(templatePath, 'utf8'));
const branchId = 'upstream-breaker-recovery-r5';
state.revision = 'r5';
state.spec = { ...state.spec, path: specPath, sha256: sha(readFileSync(specPath)), sectionBindings };
state.packet.branches = state.packet.branches.map(branch => ({
  ...branch,
  id: branchId,
  sectionBindings,
}));
state.packet.selectedBranchId = branchId;
state.packet.digest = sha(canonicalJson({
  feature: state.feature,
  projectRoot: state.projectRoot,
  sectionBindings: state.spec.sectionBindings,
  branches: state.packet.branches,
}));
state.authorization = null;
state.reviews = null;
state.envelope.readPaths = [
  ...new Set([
    ...state.envelope.readPaths.filter(p => p !== '.omp/artifacts/upstream-breaker-recovery-planner-r1.md'),
    '.omp/artifacts/breaker-planner-r5-412b6f9a6cd2-20261120T030413662Z.md',
  ]),
];
for (const task of state.tasks) task.status = 'pending';

const tmpState = `${outPath}.${randomUUID()}.tmp`;
writeFileSync(tmpState, JSON.stringify(state, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
renameSync(tmpState, outPath);

console.log(JSON.stringify({
  spec: specPath,
  specSha256: state.spec.sha256,
  sectionBindings,
  branchId,
  packetDigest: state.packet.digest,
  state: outPath,
}, null, 2));
