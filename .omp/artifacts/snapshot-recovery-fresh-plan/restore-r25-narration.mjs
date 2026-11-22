// Restore the quotations the blanket postfix rewrote; the normative token stays `orphaned_link`.
import { readFileSync, writeFileSync } from 'node:fs';
import { createHash } from 'node:crypto';

const p = '/home/opencode/projects/default/mcp-bridge/.scratch/snapshot-recovery/spec.md';
let t = readFileSync(p, 'utf8');
const before = createHash('sha256').update(t).digest('hex');

const fixes = [
  [`the r22→r23 change log's "remaining required planned oracles under the stricter key"`, `the r22→r23 change log's "still passing under the stricter key"`],
  [`The r22→r23 change log's "unchanged and remaining required planned oracles under the stricter key"`, `The r22→r23 change log's "unchanged and still passing under the stricter key"`],
  ['§9.21\'s `orphaned_link` token is fixed to `orphaned_link`', '§9.21\'s `orphaned_label` token is fixed to `orphaned_link`'],
  ['— token corrected r25, CR24-LOW: R6\'s closed set has no `orphaned_link` value', '— token corrected r25, CR24-LOW: R6\'s closed set has no `orphaned_label` value'],
];

const report = [];
for (const [from, to] of fixes) {
  const n = t.split(from).length - 1;
  report.push({ from: from.slice(0, 60), count: n });
  if (n !== 1) { console.log('unexpected count', n, from); process.exit(1); }
  t = t.split(from).join(to);
}

writeFileSync(p, t);
console.log(JSON.stringify({ report, before, after: createHash('sha256').update(t).digest('hex'), bytes: Buffer.byteLength(t) }, null, 1));
