import { readFileSync, writeFileSync, realpathSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { recordBoundReview } from '/home/opencode/.omp/agent/workflow-policy/execution-review-sequence.mjs';
import { canonicalJson, sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const candidateId = 'snapshot-recovery-spec-r17';
const revision = 'r17';
const raw = readFileSync(path.join(root, '.omp/artifacts/snapshot-recovery-fresh-plan/cr-r17-raw.md'), 'utf8');
if (!/^### Recommendation\nAPPROVE\s*$/m.test(raw)) throw new Error('unexpected verdict');
const registry = JSON.parse(readFileSync(path.join(root, '.omp/plans/review-sequence.json'), 'utf8'));
const candidate = registry.candidates.find(c => c.id === candidateId);
if (!candidate) throw new Error('candidate missing');
const bound = candidate.inputs.map(i => ({ path: realpathSync(path.resolve(root, i.path)), sha256: i.sha256 })).sort((a, b) => a.path.localeCompare(b.path));
const fingerprint = sha(canonicalJson({ id: candidate.id, revision: candidate.revision, authorIds: candidate.authorIds, inputs: bound, dispatches: candidate.dispatches }));
const invocationId = randomUUID();
const sourcePath = '.omp/artifacts/snapshot-recovery-fresh-plan/review-cr17-source-' + invocationId + '.json';
const source = { role: 'code-reviewer', reviewerId: 'code-reviewer', candidateId, revision, authorIds: candidate.authorIds, inputs: candidate.inputs, dispatches: candidate.dispatches, output: raw.trimEnd() + '\n', invocationId };
const bytes = JSON.stringify(source, null, 2) + '\n';
writeFileSync(path.join(root, sourcePath), bytes, { flag: 'wx', mode: 0o600 });
const sourceSha = sha(Buffer.from(bytes));

const findings = [];
for (const line of raw.split('\n')) {
  const m = /^\s*(?:[-*]\s*)?\*{0,2}\[(CRITICAL|HIGH|MEDIUM|LOW)\]/.exec(line);
  if (!m) continue;
  const severity = m[1];
  const statement = line.replace(/^\s*(?:[-*]\s*)?\*{0,2}\[[A-Z]+\]\s*/, '').replace(/\*\*$/, '').trim();
  findings.push({ id: 'F' + (findings.length + 1), severity, blocking: ['CRITICAL', 'HIGH', 'MEDIUM'].includes(severity), statement: statement.slice(0, 400), evidence: line.trim().slice(0, 600) });
}
if (!findings.length) throw new Error('no findings parsed');
const request = {
  binding: {
    candidateId, reviewerId: 'code-reviewer', authorIds: candidate.authorIds, inputs: candidate.inputs,
    dispatches: candidate.dispatches, verdict: 'APPROVE', findings, format: 'installed',
    source: { path: sourcePath, sha256: sourceSha }, recordedAt: new Date().toISOString(),
  },
  options: {},
  runBinding: { candidateId, invocationId, fingerprint, reviewerId: 'code-reviewer', role: 'code-reviewer', revision },
  status: 'succeeded',
};
const result = recordBoundReview(root, request);
console.log(JSON.stringify({ result, sourceSha, invocationId, findings: findings.map(f => [f.id, f.severity, f.blocking, f.statement.slice(0, 80)]) }, null, 2));
process.exit(result?.value?.ok ? 0 : 1);
