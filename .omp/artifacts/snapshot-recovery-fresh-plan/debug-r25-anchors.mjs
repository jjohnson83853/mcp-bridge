import { readFileSync } from 'node:fs';

const root = '/home/opencode/projects/default/mcp-bridge';
const delta = readFileSync(root + '/.omp/artifacts/snapshot-recovery-fresh-plan/r25-delta.md', 'utf8');
const spec = readFileSync(root + '/.scratch/snapshot-recovery/spec.md', 'utf8');

function parse(text) {
  const lines = text.split('\n');
  const blocks = [];
  for (let i = 0; i < lines.length; i++) {
    const m = /^\*\*Block (\d+) — (.*)\*\*$/.exec(lines[i]);
    if (!m) continue;
    let j = i + 1, oa = null, nt = null;
    while (j < lines.length && !/^\*\*Block \d+ — /.test(lines[j])) {
      if (lines[j].trim() === '- oldAnchor:') {
        let k = j + 1;
        while (k < lines.length && lines[k].trim() !== '```') k++;
        const body = []; k++;
        while (k < lines.length && lines[k].trim() !== '```') { body.push(lines[k]); k++; }
        oa = body.join('\n'); j = k + 1; continue;
      }
      if (lines[j].trim() === '- newText:') {
        let k = j + 1;
        while (k < lines.length && lines[k].trim() !== '```') k++;
        const body = []; k++;
        while (k < lines.length && lines[k].trim() !== '```') { body.push(lines[k]); k++; }
        nt = body.join('\n'); j = k + 1; continue;
      }
      j++;
    }
    blocks.push({ id: Number(m[1]), label: m[2], oldAnchor: oa, newText: nt });
    i = j - 1;
  }
  return blocks;
}

const A = '**r24 likewise reads no new source: every r24 amendment is design/AC/decision text level — the CR23-F1/F2 predicate correction, the F3 oracles, the F4 key definition, the F5 citation, and the F6 spelling resolve by spec-text reasoning over the r23 reconciliation and resolver rules):**';
const blocks = parse(delta);
console.log('blocks', blocks.length, 'anchor in spec', spec.indexOf(A));
let t = spec;
for (const b of blocks) {
  const before = t.split(A).length - 1;
  const matches = t.split(b.oldAnchor).length - 1;
  if (matches === 1) t = t.replace(b.oldAnchor, b.newText);
  const after = t.split(A).length - 1;
  if (before !== after) console.log('block', b.id, b.label, 'anchorCount', before, '->', after, 'matches', matches);
}
