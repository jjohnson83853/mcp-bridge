import { readFileSync, writeFileSync, renameSync } from 'node:fs';
const p = '.omp/plans/review-sequence.json';
const reg = JSON.parse(readFileSync(p, 'utf8'));
const r1 = reg.candidates.find(c => c.id === 'header-parameter-transit-spec-r1');
if (!r1) throw new Error('r1 missing');
const inputs = r1.inputs.map(x => x.path.endsWith('/header-parameter-transit/spec.md')
  ? { path: x.path, sha256: 'fb2e471c9126f88bdacb4e9ce23ece9c91dd85b6a70b3187d35dfc6de2abc35d' } : x);
const d = { name: 'ReviewHeaderSpecR4', agent: 'code-reviewer', reviewerId: 'code-reviewer', taskId: 'CR1',
  requestSha256: 'd0cb668e45d1c20ba5897e9c039eb79e147bcfb52fdd81f3845c9853bc9a44f7',
  taskSha256: '4a596279deffe98b22c8053aa753f4b2e1b3c4bdf403f2749ddd98e836801ff4',
  instructionsSha256: 'bd82facae72052d5f21b105a2e4901cc09883f1d1a58308a70c3cea8fb4e1273',
  targetCandidateId: 'header-parameter-transit-spec-r4', targetRevision: 'r4' };
reg.candidates = reg.candidates.filter(c => c.id !== 'header-parameter-transit-spec-r4');
reg.candidates.push({ id: 'header-parameter-transit-spec-r4', revision: 'r4', authorIds: [...r1.authorIds], inputs, dispatches: [d] });
const t = p + '.tmp';
writeFileSync(t, JSON.stringify(reg, null, 2) + '\n');
renameSync(t, p);
console.log(JSON.stringify({ candidate: reg.candidates.at(-1).id, inputs: inputs.length }));
