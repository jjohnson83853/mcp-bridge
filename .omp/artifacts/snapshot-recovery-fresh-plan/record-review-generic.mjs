import { readFileSync, writeFileSync, realpathSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { recordBoundReview } from '/home/opencode/.omp/agent/workflow-policy/execution-review-sequence.mjs';
import { canonicalJson, sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const [rev, role, rawPath] = process.argv.slice(2);
if (!/^r\d+$/.test(rev ?? '')) throw new Error('usage: record-review-generic.mjs <rN> <code-reviewer|critic> <raw.md>');
if (!['code-reviewer', 'critic'].includes(role)) throw new Error('bad role');
const candidateId = 'snapshot-recovery-spec-' + rev;
const raw = readFileSync(path.resolve(root, rawPath), 'utf8');
const verdictMatch = /^#{1,3}\s*(?:Recommendation\s*\n\s*)?Verdict:\s*(APPROVE|REQUEST CHANGES|PASS|REVISE|BLOCKED)\s*$/mi.exec(raw)
  ?? /^#{1,3}\s*Recommendation\s*\n(APPROVE|REQUEST CHANGES|PASS|REVISE|BLOCKED)\s*$/mi.exec(raw)
  ?? /^#{1,3}\s*Verdict\s*\n\s*(APPROVE|REQUEST CHANGES|PASS|REVISE|BLOCKED)\s*$/mi.exec(raw);
if (!verdictMatch) throw new Error('unexpected verdict');
const verdict = verdictMatch[1];
const registry = JSON.parse(readFileSync(path.join(root, '.omp/plans/review-sequence.json'), 'utf8'));
const candidate = registry.candidates.find(c => c.id === candidateId);
if (!candidate) throw new Error('candidate missing');
const bound = candidate.inputs.map(i => ({ path: realpathSync(path.resolve(root, i.path)), sha256: i.sha256 })).sort((a, b) => a.path.localeCompare(b.path));
const fingerprint = sha(canonicalJson({ id: candidate.id, revision: candidate.revision, authorIds: candidate.authorIds, inputs: bound, dispatches: candidate.dispatches }));
const invocationId = randomUUID();
const sourcePath = '.omp/artifacts/snapshot-recovery-fresh-plan/review-' + (role === 'critic' ? 'ct' : 'cr') + rev + '-source-' + invocationId + '.json';
const source = { role, reviewerId: role, candidateId, revision: rev, authorIds: candidate.authorIds, inputs: candidate.inputs, dispatches: candidate.dispatches, output: raw.trimEnd() + '\n', invocationId };
const bytes = JSON.stringify(source, null, 2) + '\n';
writeFileSync(path.join(root, sourcePath), bytes, { flag: 'wx', mode: 0o600 });
const sourceSha = sha(Buffer.from(bytes));

const findings = [];
const findingRe = /^\s*(?:(?:[-*]|\d+\.)\s+|\*\*)(?:\*{0,2}\[(CRITICAL|HIGH|MEDIUM|LOW)\]|(?:[A-Z]\d+\s*—\s*)?(CRITICAL|HIGH|MEDIUM|LOW)\b)/;
for (const line of raw.split('\n')) {
  const m = findingRe.exec(line);
  if (!m) continue;
  const severity = m[1] ?? m[2];
  const statement = line.slice(m[0].length).replace(/^\*{0,2}\]?\s*/, '').replace(/^`\[[^\]]*\]`?\s*/, '').trim();
  findings.push({ id: 'F' + (findings.length + 1), severity, blocking: ['CRITICAL', 'HIGH', 'MEDIUM'].includes(severity), statement: statement.slice(0, 400), evidence: line.trim().slice(0, 600) });
}
if (!findings.length) throw new Error('no findings parsed');
const request = {
  binding: {
    candidateId, reviewerId: role, authorIds: candidate.authorIds, inputs: candidate.inputs,
    dispatches: candidate.dispatches, verdict, findings, format: 'installed',
    source: { path: sourcePath, sha256: sourceSha }, recordedAt: new Date().toISOString(),
  },
  options: {},
  runBinding: { candidateId, invocationId, fingerprint, reviewerId: role, agent: role, role, revision: rev },
  status: 'succeeded',
};
if (role === 'critic') {
  const b = request.binding;
  request.binding = { role: 'critic', candidateId: b.candidateId, reviewerId: b.reviewerId, authorIds: b.authorIds, inputs: b.inputs, dispatches: b.dispatches, verdict: b.verdict, format: b.format, source: b.source, recordedAt: b.recordedAt };
}
const result = recordBoundReview(root, request);
console.log(JSON.stringify({ result, sourceSha, invocationId, verdict, findings: findings.map(f => [f.id, f.severity, f.blocking, f.statement.slice(0, 80)]) }, null, 2));
process.exit(result?.value?.ok ? 0 : 1);
