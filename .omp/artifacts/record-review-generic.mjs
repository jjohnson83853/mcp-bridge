import { readFileSync, writeFileSync, realpathSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { recordBoundReview } from '/home/opencode/.omp/agent/workflow-policy/execution-review-sequence.mjs';
import { canonicalJson, sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const [role, rev, rawRel] = process.argv.slice(2);
if (!['cr', 'ct'].includes(role) || !/^r\d+$/.test(rev ?? '')) throw new Error('usage: record-review-generic.mjs <cr|ct> <rN> <raw.md>');
const root = '/home/opencode/projects/default/mcp-bridge';
const candidateId = 'snapshot-recovery-spec-' + rev;
const raw = readFileSync(path.join(root, rawRel), 'utf8');
let verdict;
if (role === 'cr') { const m = /### Recommendation\s*\n\s*(APPROVE|REQUEST CHANGES)\s*$/m.exec(raw); if (!m) throw new Error('unexpected CR verdict'); verdict = m[1]; if (!['APPROVE', 'REQUEST CHANGES'].includes(verdict)) throw new Error('verdict ' + verdict); }
else { const m = /### Verdict\s*\n\s*(PASS|REVISE|BLOCKED)\s*$/m.exec(raw); if (!m) throw new Error('unexpected CT verdict'); verdict = m[1]; }
const registry = JSON.parse(readFileSync(path.join(root, '.omp/plans/review-sequence.json'), 'utf8'));
const candidate = registry.candidates.find(c => c.id === candidateId);
if (!candidate) throw new Error('candidate missing');
const reviewerId = role === 'cr' ? 'code-reviewer' : 'critic';
const dispatch = candidate.dispatches.find(d => d.reviewerId === reviewerId);
if (!dispatch) throw new Error('dispatch missing');
const bound = candidate.inputs.map(i => ({ path: realpathSync(path.resolve(root, i.path)), sha256: i.sha256 })).sort((a, b) => a.path.localeCompare(b.path));
const fingerprint = sha(canonicalJson({ id: candidate.id, revision: candidate.revision, authorIds: candidate.authorIds, inputs: bound, dispatches: candidate.dispatches }));
const invocationId = randomUUID();
const findings = [];
for (const line of raw.split('\n')) {
  let sev = null, id = null;
  const a = /^\s*\*{0,2}(F\d+)\s*\[(CRITICAL|HIGH|MEDIUM|LOW)\]/.exec(line);
  const b = /^\s*\*{0,2}(C\d+)\s*[—-]\s*(CRITICAL|HIGH|MEDIUM|LOW)\b/.exec(line);
  const c = /^\s*[-*]\s*\*{0,2}\[(CRITICAL|HIGH|MEDIUM|LOW)\]/.exec(line);
  if (a) { id = a[1]; sev = a[2]; } else if (b) { id = b[1]; sev = b[2]; } else if (c) { sev = c[1]; id = 'F' + (findings.length + 1); }
  if (!sev) continue;
  const blocking = ['CRITICAL', 'HIGH', 'MEDIUM'].includes(sev);
  findings.push({ id, severity: sev, blocking, statement: line.replace(/^\s*(?:[-*]\s*)?\*{0,2}/, '').replace(/\*+$/, '').trim().slice(0, 400), evidence: line.trim().slice(0, 600) });
}
if (role === 'cr' && !findings.length && verdict === 'REQUEST CHANGES') throw new Error('no findings parsed');
let output = raw.trimEnd() + '\n';
if (role === 'ct') { const body = raw.replace(/(### Verdict\s*(PASS|REVISE|BLOCKED)\s*)+$/, '').trimEnd(); output = body + '\n\n### Verdict\n' + verdict + '\n'; }
const sourcePath = '.omp/artifacts/snapshot-recovery-fresh-plan/review-' + role + '-' + rev + '-source-' + invocationId + '.json';
const source = { role: reviewerId, reviewerId, candidateId, revision: rev, authorIds: candidate.authorIds, inputs: candidate.inputs, dispatches: candidate.dispatches, output, invocationId };
const bytes = JSON.stringify(source, null, 2) + '\n';
writeFileSync(path.join(root, sourcePath), bytes, { flag: 'wx', mode: 0o600 });
const sourceSha = sha(Buffer.from(bytes));
const binding = { ...(role === "ct" ? { role: "critic" } : {}), candidateId, reviewerId, authorIds: candidate.authorIds, inputs: candidate.inputs, dispatches: candidate.dispatches, verdict, format: 'installed', source: { path: sourcePath, sha256: sourceSha }, recordedAt: new Date().toISOString() };
if (role === 'cr') binding.findings = findings;
const request = { binding, options: {}, runBinding: { candidateId, invocationId, fingerprint, reviewerId, revision: rev, ...(role === 'ct' ? { agent: 'critic' } : { role: 'code-reviewer' }) }, status: 'succeeded' };
const result = recordBoundReview(root, request);
console.log(JSON.stringify({ result, sourceSha, invocationId, verdict, findings: findings.map(f => [f.id, f.severity, f.blocking, f.statement.slice(0, 70)]) }, null, 2));
process.exit(result?.value?.ok ? 0 : 1);
