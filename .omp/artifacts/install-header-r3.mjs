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
const archivedR2Spec = path.join(root, '.omp/artifacts/execution-state-r2-spec-' + H(oldSpec).slice(0, 12) + '.md');
copyFileSync(specPath, archivedR2Spec);
copyFileSync(statePath, path.join(root, '.omp/artifacts/execution-state-r2-' + H(oldState).slice(0, 12) + '.json'));

const spec3 = readFileSync(path.join(dir, 'hpt-spec-r3.md'));
const state3 = JSON.parse(readFileSync(path.join(dir, 'hpt-state-r3.json'), 'utf8'));
if (state3.revision !== 'r3') throw new Error('revision');
if (state3.packet.selectedBranchId !== null || state3.authorization !== null || state3.reviews !== null) throw new Error('draft authority');
const b3 = bindings(spec3.toString('utf8'), 'r3');
state3.spec.path = specPath;
state3.spec.sha256 = H(spec3);
state3.spec.sectionBindings = b3;
if (state3.packet.branches.length !== 1) throw new Error('branches');
state3.packet.branches[0].sectionBindings = JSON.parse(JSON.stringify(b3));
state3.packet.digest = H(canonical({ feature: state3.feature, projectRoot: state3.projectRoot, sectionBindings: b3, branches: state3.packet.branches }));

const src = state3.sources[0];
const r1Sha = '8054d28f24cdf82475a27df8c0ad3e73ed5bf36da91db299be709e2e147b8347';
const r2Sha = H(oldSpec);
if (src.artifact.sha256 === r1Sha) src.artifact.path = path.join(root, '.omp/artifacts/execution-state-r1-spec-' + r1Sha.slice(0, 12) + '.md');
else if (src.artifact.sha256 === r2Sha) src.artifact.path = archivedR2Spec;
else throw new Error('unexpected source artifact sha ' + src.artifact.sha256);

writeFileSync(specPath, spec3);
writeFileSync(statePath, JSON.stringify(state3, null, 2) + '\n');
console.log(JSON.stringify({ specSha: state3.spec.sha256, packetDigest: state3.packet.digest, sourceArtifact: src.artifact, archivedR2Spec, archivedR2State: H(oldState), branch: state3.packet.branches[0].id }, null, 2));
