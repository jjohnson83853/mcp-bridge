import { readFileSync, writeFileSync, realpathSync } from 'node:fs';
import path from 'node:path';
import { randomUUID } from 'node:crypto';
import { recordBoundReview } from '/home/opencode/.omp/agent/workflow-policy/execution-review-sequence.mjs';
import { canonicalJson, sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const [candidateId, role, rawPath] = process.argv.slice(2);
if (!candidateId || !['code-reviewer', 'critic'].includes(role) || !rawPath) throw new Error('usage: record-review.mjs <candidateId> <code-reviewer|critic> <raw.txt>');
const rawAll = readFileSync(path.resolve(root, rawPath), 'utf8');
const m = /<output>\n([\s\S]*?)\n<\/output>/.exec(rawAll);
const raw = (m ? m[1] : rawAll).trimEnd() + '\n';
const registry = JSON.parse(readFileSync(path.join(root, '.omp/plans/review-sequence.json'), 'utf8'));
const candidate = registry.candidates.find(c => c.id === candidateId);
if (!candidate) throw new Error('candidate missing');
const verdictRe = role === 'critic'
  ? /^###\s*Verdict\s*\n\s*(PASS|REVISE|BLOCKED)\s*$/m
  : /^###\s*Recommendation\s*\n\s*(APPROVE|REQUEST CHANGES|COMMENT)\s*$/m;
const vm = verdictRe.exec(raw);
if (!vm) throw new Error('verdict not found');
const verdict = vm[1];
const inputs = candidate.inputs.map(i => ({ path: i.path, sha256: i.sha256 }));
const bound = inputs.map(i => ({ path: realpathSync(path.resolve(root, i.path)), sha256: i.sha256 })).sort((a, b) => a.path.localeCompare(b.path));
const fingerprint = sha(canonicalJson({ id: candidate.id, revision: candidate.revision, authorIds: candidate.authorIds, inputs: bound, dispatches: candidate.dispatches }));
const invocationId = randomUUID();
const sourcePath = `.omp/artifacts/config-admission-resilience/review-${role === 'critic' ? 'ct' : 'cr'}-source-${invocationId}.json`;
const source = { role, reviewerId: role, candidateId, revision: candidate.revision, authorIds: candidate.authorIds, inputs, dispatches: candidate.dispatches, output: raw, invocationId };
const bytes = JSON.stringify(source, null, 2) + '\n';
writeFileSync(path.join(root, sourcePath), bytes, { flag: 'wx', mode: 0o600 });
const findings = [];
const findingRe = /^\s*(?:[-*]|\d+\.)\s+\*{0,2}\[(CRITICAL|HIGH|MEDIUM|LOW)\]\*{0,2}(?:\[[^\]]*\])?\*{0,2}\s+(.*)$/;
for (const line of raw.split('\n')) {
  const f = findingRe.exec(line);
  if (!f) continue;
  findings.push({ id: 'F' + (findings.length + 1), severity: f[1], blocking: ['CRITICAL', 'HIGH', 'MEDIUM'].includes(f[1]), statement: f[2].slice(0, 400), evidence: line.trim().slice(0, 600) });
}
const binding = { candidateId, reviewerId: role, authorIds: candidate.authorIds, inputs, dispatches: candidate.dispatches, verdict, findings, format: 'installed', source: { path: sourcePath, sha256: sha(Buffer.from(bytes)) }, recordedAt: new Date().toISOString() };
const request = { binding: role === 'critic' ? { role: 'critic', candidateId, reviewerId: role, authorIds: binding.authorIds, inputs, dispatches: binding.dispatches, verdict, format: 'installed', source: binding.source, recordedAt: binding.recordedAt } : binding, options: {}, runBinding: { candidateId, invocationId, fingerprint, reviewerId: role, agent: role, role, revision: candidate.revision }, status: 'succeeded' };
const result = recordBoundReview(root, request);
console.log(JSON.stringify({ result, verdict, findings: findings.map(f => [f.id, f.severity, f.blocking]), sourceSha: binding.source.sha256, invocationId }, null, 2));
process.exit(result?.value?.ok ? 0 : 1);
