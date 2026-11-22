import { readFileSync, writeFileSync, realpathSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { recordBoundReview } from '/home/opencode/.omp/agent/workflow-policy/execution-review-sequence.mjs';
import { canonicalJson, sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const candidateId = 'snapshot-recovery-spec-r17';
const revision = 'r17';
const raw = readFileSync(path.join(root, '.omp/artifacts/snapshot-recovery-fresh-plan/ct-r17-raw.md'), 'utf8');
if (!/### Verdict\s*REVISE\s*$/.test(raw.trimEnd())) throw new Error('unexpected verdict');
const registry = JSON.parse(readFileSync(path.join(root, '.omp/plans/review-sequence.json'), 'utf8'));
const candidate = registry.candidates.find(c => c.id === candidateId);
if (!candidate) throw new Error('candidate missing');
const bound = candidate.inputs.map(i => ({ path: realpathSync(path.resolve(root, i.path)), sha256: i.sha256 })).sort((a, b) => a.path.localeCompare(b.path));
const fingerprint = sha(canonicalJson({ id: candidate.id, revision: candidate.revision, authorIds: candidate.authorIds, inputs: bound, dispatches: candidate.dispatches }));
const invocationId = randomUUID();
const body = raw.replace(/(### Verdict\s*REVISE\s*)+$/, '').trimEnd();
const output = body + '\n\n### Verdict\nREVISE\n';
const sourcePath = '.omp/artifacts/snapshot-recovery-fresh-plan/review-ct17-source-' + invocationId + '.json';
const source = { role: 'critic', reviewerId: 'critic', candidateId, revision, authorIds: candidate.authorIds, inputs: candidate.inputs, dispatches: candidate.dispatches, output, invocationId };
const bytes = JSON.stringify(source, null, 2) + '\n';
writeFileSync(path.join(root, sourcePath), bytes, { flag: 'wx', mode: 0o600 });
const sourceSha = sha(Buffer.from(bytes));
const findings = [];
for (const line of raw.split('\n')) {
  const m = /^\*\*(C\d+) — (CRITICAL|HIGH|MEDIUM|LOW)\b/.exec(line);
  if (!m) continue;
  findings.push({ id: m[1], severity: m[2], blocking: true, statement: line.replace(/^\*\*|\*\*$/g, '').trim().slice(0, 400), evidence: line.trim().slice(0, 600) });
}
if (!findings.length) throw new Error('no findings parsed');
const request = { binding: { role: 'critic', candidateId, reviewerId: 'critic', authorIds: candidate.authorIds, inputs: candidate.inputs, dispatches: candidate.dispatches, verdict: 'REVISE', format: 'installed', source: { path: sourcePath, sha256: sourceSha }, recordedAt: new Date().toISOString() }, options: {}, runBinding: { candidateId, invocationId, fingerprint, reviewerId: 'critic', agent: 'critic', revision }, status: 'succeeded' };
const result = recordBoundReview(root, request);
console.log(JSON.stringify({ result, sourceSha, invocationId, findings: findings.map(f => [f.id, f.severity, f.statement.slice(0, 80)]) }, null, 2));
process.exit(result?.value?.ok ? 0 : 1);
