import { readFileSync, writeFileSync, copyFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import path from 'node:path';

const root = process.cwd();
const H = b => createHash('sha256').update(b).digest('hex');
const canonical = v => Array.isArray(v) ? '[' + v.map(canonical).join(',') + ']'
  : v && typeof v === 'object' ? '{' + Object.keys(v).sort().map(k => JSON.stringify(k) + ':' + canonical(v[k])).join(',') + '}' : JSON.stringify(v);
const SECTIONS = ['requirements', 'design', 'tasks', 'envelope'];
const bindings = (text, rev) => {
  const out = {};
  let last = -1;
  for (const name of SECTIONS) {
    const start = `<!-- specdd:${name}:start -->`, end = `<!-- specdd:${name}:end -->`;
    const a = text.indexOf(start), b = text.indexOf(end);
    if (a < 0 || b < a + start.length || a <= last) throw new Error('markers ' + name);
    out[name] = { revision: rev, sha256: H(text.slice(a + start.length, b)) };
    last = b + end.length - 1;
  }
  return out;
};

const dir = '/home/opencode/.omp/agent/sessions/-projects-default-mcp-bridge/2026-11-20T23-05-10-099Z_01a1fb34-e4d3-70ab-8f55-d5acfd2fa6d0/local';
const specPath = path.join(root, '.scratch/header-parameter-transit/spec.md');
const statePath = path.join(root, '.omp/plans/execution-state.json');
const oldSpec = readFileSync(specPath);
const oldState = readFileSync(statePath);
const archivedSpec = path.join(root, '.omp/artifacts/execution-state-r5pre-spec-' + H(oldSpec).slice(0, 12) + '.md');
copyFileSync(specPath, archivedSpec);
copyFileSync(statePath, path.join(root, '.omp/artifacts/execution-state-r5pre-' + H(oldState).slice(0, 12) + '.json'));

const spec4 = readFileSync(path.join(dir, 'hpt-spec-r5b.md'));
const state4 = JSON.parse(readFileSync(path.join(dir, 'hpt-state-r5.json'), 'utf8'));
if (state4.revision !== 'r5') throw new Error('revision');
if (state4.packet.selectedBranchId !== null || state4.authorization !== null || state4.reviews !== null) throw new Error('draft authority');
const b4 = bindings(spec4.toString('utf8'), 'r5');
state4.spec.path = specPath;
state4.spec.sha256 = H(spec4);
state4.spec.sectionBindings = b4;
if (!Array.isArray(state4.packet.branches) || state4.packet.branches.length !== 1) throw new Error('branches');
state4.packet.branches[0].sectionBindings = JSON.parse(JSON.stringify(b4));
state4.packet.digest = H(canonical({ feature: state4.feature, projectRoot: state4.projectRoot, sectionBindings: b4, branches: state4.packet.branches }));

const src = state4.sources[0];
const archived = {
  '8054d28f24cdf82475a27df8c0ad3e73ed5bf36da91db299be709e2e147b8347': path.join(root, '.omp/artifacts/execution-state-r1-spec-8054d28f24cd.md'),
  [H(oldSpec)]: archivedSpec,
};
if (src?.artifact && archived[src.artifact.sha256]) src.artifact.path = archived[src.artifact.sha256];

writeFileSync(specPath, spec4);
writeFileSync(statePath, JSON.stringify(state4, null, 2) + '\n');
console.log(JSON.stringify({ specSha: state4.spec.sha256, packetDigest: state4.packet.digest, branch: state4.packet.branches[0].id, source: src?.artifact, tasks: state4.tasks.map(t => t.id), requirements: state4.requirements.length, acceptance: state4.acceptance.length, archivedOldSpec: H(oldSpec) }, null, 2));
