import { readFileSync, writeFileSync } from 'node:fs';
let s = readFileSync('.omp/artifacts/install-header-r4.mjs', 'utf8');
s = s.replaceAll('hpt-spec-r4', 'hpt-spec-r5')
  .replaceAll('hpt-state-r4', 'hpt-state-r5')
  .replaceAll('execution-state-r3-spec', 'execution-state-r4-spec')
  .replaceAll('execution-state-r3-', 'execution-state-r4-')
  .replace("state4.revision !== 'r4'", "state4.revision !== 'r5'")
  .replace("bindings(spec4.toString('utf8'), 'r4')", "bindings(spec4.toString('utf8'), 'r5')");
writeFileSync('.omp/artifacts/install-header-r5.mjs', s);
console.log('ok');