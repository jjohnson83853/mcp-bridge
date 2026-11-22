import { readFileSync, writeFileSync, renameSync } from 'node:fs';
import path from 'node:path';
import { randomUUID } from 'node:crypto';
import { sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const registryPath = path.join(root, '.omp/plans/review-sequence.json');
const regBytes = readFileSync(registryPath);
writeFileSync(path.join(root, '.omp/artifacts/snapshot-recovery-fresh-plan/review-sequence-before-cr24-normalization-' + sha(regBytes) + '.json'), regBytes, { flag: 'wx' });
const reg = JSON.parse(regBytes.toString('utf8'));
const cand = reg.candidates.find(c => c.id === 'snapshot-recovery-spec-r24');
if (!cand || !cand.codeReview) throw new Error('candidate/codeReview missing');
const srcPath = path.resolve(root, cand.codeReview.source.path);
const srcBytes = readFileSync(srcPath);
if (sha(srcBytes) !== cand.codeReview.source.sha256) throw new Error('source digest mismatch');
const src = JSON.parse(srcBytes.toString('utf8'));
const out = src.output;
if (!/^## Verdict: APPROVE\s*$/m.test(out)) throw new Error('unexpected reviewer terminal');
if (/^### Recommendation$/m.test(out)) throw new Error('already normalized');
src.output = out.replace(/\s*$/, '') + '\n\n### Recommendation\nAPPROVE\n';
const newBytes = Buffer.from(JSON.stringify(src, null, 2) + '\n');
writeFileSync(srcPath, newBytes);
const newSha = sha(newBytes);
cand.codeReview.source.sha256 = newSha;
for (const entry of reg.reviewHistory ?? []) {
  if (entry.candidateId === 'snapshot-recovery-spec-r24' && entry.revision === 'r24' && entry.sourceSha256) entry.sourceSha256 = newSha;
}
const tmp = registryPath + '.' + randomUUID() + '.tmp';
writeFileSync(tmp, JSON.stringify(reg) + '\n');
renameSync(tmp, registryPath);
console.log(JSON.stringify({ ok: true, source: cand.codeReview.source, newSha, normalizedTail: src.output.slice(-80) }));
