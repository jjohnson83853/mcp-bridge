import { readFileSync, writeFileSync, readdirSync } from 'node:fs';
import { createHash } from 'node:crypto';
import path from 'node:path';

const root = process.cwd();
const H = s => createHash('sha256').update(s).digest('hex');
const runs = JSON.parse(readFileSync(path.join(root, '.omp/plans/review-runs.json'), 'utf8'));
const run = runs.runs.find(r => r.candidateId === 'header-parameter-transit-spec-r1' && (r.role ?? 'code-reviewer') === 'code-reviewer');
if (!run) throw new Error('run missing');
const reg = JSON.parse(readFileSync(path.join(root, '.omp/plans/review-sequence.json'), 'utf8'));
const cand = reg.candidates.find(c => c.id === 'header-parameter-transit-spec-r1');
if (!cand) throw new Error('candidate missing');
const sessionDir = readdirSync('/home/opencode/.omp/agent/sessions/-projects-default-mcp-bridge', { withFileTypes: true }).filter(e => e.isDirectory() && /^\d{4}-/.test(e.name)).map(e => e.name).sort().at(-1);
const output = readFileSync('/home/opencode/.omp/agent/sessions/-projects-default-mcp-bridge/' + sessionDir + '/ReviewHeaderSpecR1.md', 'utf8');
if (!output.trimEnd().endsWith('REQUEST CHANGES')) throw new Error('unexpected output tail: ' + output.slice(-40));
const observation = {
  role: 'code-reviewer',
  reviewerId: 'code-reviewer',
  candidateId: cand.id,
  revision: cand.revision,
  authorIds: cand.authorIds,
  inputs: cand.inputs,
  dispatches: cand.dispatches,
  output,
  invocationId: run.invocationId,
  recordedAt: new Date().toISOString(),
};
const obsPath = path.join(root, '.omp/artifacts/review-header-transit-spec-r1-code-reviewer-' + H(output).slice(0, 12) + '.json');
writeFileSync(obsPath, JSON.stringify(observation, null, 2) + '\n');
const findings = [
  { id: 'F1', severity: 'HIGH', summary: 'Legacy peer must advertise older-only versions; default options negotiate 2026-07-28 and the legacy gate never fires.' },
  { id: 'F2', severity: 'HIGH', summary: 'T2 validation embeds a plausibly-red out-of-envelope pre-existing test; bound the command or state a verified-green precondition.' },
  { id: 'F3', severity: 'MEDIUM', summary: 'AC02-2/AC03-2 zero-tools/list oracles need a pinned counting window.' },
  { id: 'F4', severity: 'LOW', summary: 'Section 3 overstates Mcp-Method coverage; standard headers are new-protocol-only.' },
  { id: 'F5', severity: 'LOW', summary: 'Anchor and wording nits: config field range, logging-contract table range, refresh completion sentence.' },
  { id: 'F6', severity: 'LOW', summary: 'Warm cost claim must state the paginated worst case.' },
];
const binding = {
  binding: {
    candidateId: cand.id,
    reviewerId: 'code-reviewer',
    authorIds: cand.authorIds,
    inputs: cand.inputs,
    dispatches: cand.dispatches,
    verdict: 'REQUEST CHANGES',
    findings,
    format: 'installed',
    source: { path: obsPath, sha256: H(readFileSync(obsPath)) },
    recordedAt: observation.recordedAt,
  },
  runBinding: {
    candidateId: cand.id,
    invocationId: run.invocationId,
    fingerprint: run.fingerprint,
    reviewerId: 'code-reviewer',
    agent: 'code-reviewer',
    revision: cand.revision,
  },
  status: 'succeeded',
};
const bindPath = path.join(root, '.omp/artifacts/header-transit-bind-cr1.json');
writeFileSync(bindPath, JSON.stringify(binding, null, 2) + '\n');
console.log(JSON.stringify({ obsPath, obsSha: binding.binding.source.sha256, bindPath, invocationId: run.invocationId }, null, 2));
