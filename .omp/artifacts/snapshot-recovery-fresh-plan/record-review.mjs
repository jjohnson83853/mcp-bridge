import { readFileSync, writeFileSync, realpathSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { recordBoundReview } from '/home/opencode/.omp/agent/workflow-policy/execution-review-sequence.mjs';
import { canonicalJson, sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const rev = process.argv[2];
const rawPath = process.argv[3];
const verdict = process.argv[4];
const reviewerId = process.argv[5] ?? 'code-reviewer';
const candidateId = 'snapshot-recovery-spec-' + rev;
const raw = readFileSync(path.join(root, rawPath), 'utf8');
const registry = JSON.parse(readFileSync(path.join(root, '.omp/plans/review-sequence.json'), 'utf8'));
const candidate = registry.candidates.find(c => c.id === candidateId);
if (!candidate) throw new Error('candidate missing');
const bound = candidate.inputs.map(i => ({ path: realpathSync(path.resolve(root, i.path)), sha256: i.sha256 })).sort((a, b) => a.path.localeCompare(b.path));
const fingerprint = sha(canonicalJson({ id: candidate.id, revision: candidate.revision, authorIds: candidate.authorIds, inputs: bound, dispatches: candidate.dispatches }));
const invocationId = randomUUID();
const tag = reviewerId === 'critic' ? 'ct' : 'cr';
const sourcePath = '.omp/artifacts/snapshot-recovery-fresh-plan/review-' + tag + '-' + rev + '-source-' + invocationId + '.json';
const source = { role: reviewerId, reviewerId, candidateId, revision: rev, authorIds: candidate.authorIds, inputs: candidate.inputs, dispatches: candidate.dispatches, output: raw.trimEnd() + '\n', invocationId };
const bytes = JSON.stringify(source, null, 2) + '\n';
writeFileSync(path.join(root, sourcePath), bytes, { flag: 'wx', mode: 0o600 });
const sourceSha = sha(Buffer.from(bytes));

const findings = [];
for (const block of raw.split(/\n(?=\* ?\*?\[)/)) {
  const m = block.match(/\[([A-Z]+)\]/);
  if (!m) continue;
  const severity = m[1];
  const headingLine = block.split('\n')[0];
  const idm = headingLine.match(/\b(CR\d+-\d+|[A-Z]{2,}\d+-\d+)\b/);
  findings.push({ id: idm ? idm[1] : 'F' + (findings.length + 1), severity, blocking: ['CRITICAL', 'HIGH', 'MEDIUM'].includes(severity), statement: headingLine.replace(/^\* ?\*?/, '').trim(), evidence: block.trim().slice(0, 600) });
}
if (!findings.length && !['APPROVE', 'PASS'].includes(verdict)) throw new Error('no findings parsed');
const request = { binding: { candidateId, reviewerId, authorIds: candidate.authorIds, inputs: candidate.inputs, dispatches: candidate.dispatches, verdict, findings, format: 'installed', source: { path: sourcePath, sha256: sourceSha }, recordedAt: new Date().toISOString() }, options: {}, runBinding: { candidateId, invocationId, fingerprint, reviewerId, role: reviewerId, agent: reviewerId, revision: rev }, status: 'succeeded' };
if (reviewerId === 'critic') { delete request.binding.findings; request.binding.role = 'critic'; }
const result = recordBoundReview(root, request);
console.log(JSON.stringify({ result, sourceSha, invocationId, findings: findings.map(f => [f.id, f.severity, f.blocking]) }));
process.exit(result?.value?.ok ? 0 : 1);