import { readFileSync, writeFileSync } from 'node:fs';
let s = readFileSync('.omp/artifacts/install-header-r5.mjs', 'utf8');
s = s.replaceAll('hpt-spec-r5', 'hpt-spec-r5b')
  .replaceAll("execution-state-r4-spec", "execution-state-r5pre-spec")
  .replaceAll("execution-state-r4-", "execution-state-r5pre-");
writeFileSync('.omp/artifacts/install-header-r5b.mjs', s);
console.log('ok');