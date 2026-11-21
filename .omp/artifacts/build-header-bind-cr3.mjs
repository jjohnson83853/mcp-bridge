import { readFileSync, writeFileSync, readdirSync } from 'node:fs';
import { createHash } from 'node:crypto';
import path from 'node:path';

const root = process.cwd();
const H = s => createHash('sha256').update(s).digest('hex');
const candId = 'header-parameter-transit-spec-r4';
const outName = 'ReviewHeaderSpecR4';
const runs = JSON.parse(readFileSync(path.join(root, '.omp/plans/review-runs.json'), 'utf8'));
const run = runs.runs.find(r => r.candidateId === candId && (r.role ?? 'code-reviewer') === 'code-reviewer');
if (!run) throw new Error('run missing');
const reg = JSON.parse(readFileSync(path.join(root, '.omp/plans/review-sequence.json'), 'utf8'));
const cand = reg.candidates.find(c => c.id === candId);
if (!cand) throw new Error('candidate missing');
const base = '/home/opencode/.omp/agent/sessions/-projects-default-mcp-bridge';
const sessionDir = readdirSync(base, { withFileTypes: true }).filter(e => e.isDirectory() && /^\d{4}-/.test(e.name)).map(e => e.name).sort().at(-1);
const output = readFileSync(path.join(base, sessionDir, outName + '.md'), 'utf8');
if (!output.trimEnd().endsWith('REQUEST CHANGES')) throw new Error('unexpected output tail');
const recordedAt = new Date().toISOString();
const observation = { role: 'code-reviewer', reviewerId: 'code-reviewer', candidateId: cand.id, revision: cand.revision, authorIds: cand.authorIds, inputs: cand.inputs, dispatches: cand.dispatches, output, invocationId: run.invocationId, recordedAt };
const obsPath = path.join(root, '.omp/artifacts/review-' + cand.id + '-code-reviewer-' + H(output).slice(0, 12) + '.json');
writeFileSync(obsPath, JSON.stringify(observation, null, 2) + '\n');
const findings = JSON.parse(readFileSync(path.join(root, '.omp/artifacts/findings-r4.json'), 'utf8'));
const binding = {
  binding: { candidateId: cand.id, reviewerId: 'code-reviewer', authorIds: cand.authorIds, inputs: cand.inputs, dispatches: cand.dispatches, verdict: 'REQUEST CHANGES', findings, format: 'installed', source: { path: obsPath, sha256: H(readFileSync(obsPath)) }, recordedAt },
  runBinding: { candidateId: cand.id, invocationId: run.invocationId, fingerprint: run.fingerprint, reviewerId: 'code-reviewer', agent: 'code-reviewer', revision: cand.revision },
  status: 'succeeded',
};
const bindPath = path.join(root, '.omp/artifacts/header-transit-bind-cr3.json');
writeFileSync(bindPath, JSON.stringify(binding, null, 2) + '\n');
console.log(JSON.stringify({ obsPath, obsSha: binding.binding.source.sha256, bindPath, invocationId: run.invocationId }, null, 2));
