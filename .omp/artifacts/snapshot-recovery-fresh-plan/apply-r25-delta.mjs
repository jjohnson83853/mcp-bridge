// Deterministic applier for the planner's r24 -> r25 delta.
// Usage: node apply-r25-delta.mjs [--write]
import { readFileSync, writeFileSync, copyFileSync, existsSync } from 'node:fs';
import { createHash } from 'node:crypto';
import path from 'node:path';

const root = '/home/opencode/projects/default/mcp-bridge';
const deltaPath = path.join(root, '.omp/artifacts/snapshot-recovery-fresh-plan/r25-delta.md');
const specPath = path.join(root, '.scratch/snapshot-recovery/spec.md');
const write = process.argv.includes('--write');

const sha = t => createHash('sha256').update(t).digest('hex');
const delta = readFileSync(deltaPath, 'utf8');
const original = readFileSync(specPath, 'utf8');

function parseBlocks(text) {
  const lines = text.split('\n');
  const blocks = [];
  for (let i = 0; i < lines.length; i++) {
    const m = /^\*\*Block (\d+) — (.*)\*\*$/.exec(lines[i]);
    if (!m) continue;
    let j = i + 1;
    let section = '';
    let oldAnchor = null;
    let newText = null;
    while (j < lines.length && !/^\*\*Block \d+ — /.test(lines[j])) {
      if (lines[j].startsWith('- section:')) section = lines[j].slice(10).trim();
      if (lines[j].trim() === '- oldAnchor:') { const r = readFence(lines, j + 1); oldAnchor = r.text; j = r.next; continue; }
      if (lines[j].trim() === '- newText:') { const r = readFence(lines, j + 1); newText = r.text; j = r.next; continue; }
      j++;
    }
    if (oldAnchor === null || newText === null) throw new Error(`Block ${m[1]} missing oldAnchor/newText`);
    blocks.push({ id: Number(m[1]), label: m[2], section, oldAnchor, newText });
    i = j - 1;
  }
  return blocks;
}
function readFence(lines, start) {
  let k = start;
  while (k < lines.length && lines[k].trim() !== '```') k++;
  if (k >= lines.length) throw new Error('unterminated fence');
  const body = [];
  k++;
  while (k < lines.length && lines[k].trim() !== '```') { body.push(lines[k]); k++; }
  if (k >= lines.length) throw new Error('unterminated fence');
  return { text: body.join('\n'), next: k + 1 };
}

const blocks = parseBlocks(delta);
console.log(`parsed blocks: ${blocks.length}`);

let text = original;
const failures = [];
const applied = [];
for (const b of blocks) {
  const count = text.split(b.oldAnchor).length - 1;
  if (count !== 1) { failures.push({ id: b.id, label: b.label, count }); continue; }
  text = text.replace(b.oldAnchor, b.newText);
  applied.push(b.id);
}

console.log(`applied: ${applied.length} [${applied.join(',')}]`);
if (failures.length) {
  console.log('FAILED ANCHORS:');
  for (const f of failures) console.log(` block ${f.id} (${f.label}) matches=${f.count}`);
  process.exit(1);
}

// Post-fixes: the planner's substitute text for the same defect retains the old token in the
// newly inserted D12 sentence and in the r22 change log. Apply the planner's own replacements.
for (const [from, to] of [
  ['orphaned_label', 'orphaned_link'],
  ['still passing under the stricter key', 'remaining required planned oracles under the stricter key'],
]) {
  const n = text.split(from).length - 1;
  text = text.split(from).join(to);
  console.log(`postfix ${JSON.stringify(from)} -> ${JSON.stringify(to)} count=${n}`);
}

// Post-conditions
const checks = {
  header_r25: text.startsWith('# SNAPREC-SPEC-r25 — snapshot-recovery (revision, plan round 15)'),
  no_orphaned_label: !text.includes('orphaned_label'),
  no_still_passing: !text.includes('still passing under the stricter key'),
  tasks_t1_t4: /T4/.test(text) && text.includes('<!-- specdd:tasks:end -->'),
  change_log_r25: text.includes('CHANGE LOG — r24 → r25'),
};
console.log(JSON.stringify(checks));
if (!Object.values(checks).every(Boolean)) { console.log('post-condition failed'); process.exit(1); }

const before = sha(original);
const after = sha(text);
console.log(JSON.stringify({ before, after, beforeBytes: Buffer.byteLength(original), afterBytes: Buffer.byteLength(text) }));

if (write) {
  const snap = path.join(root, `.omp/artifacts/snapshot-recovery-fresh-plan/archive-before-r25-${before}.md`);
  if (existsSync(snap)) { console.log('snapshot already exists; not overwriting'); process.exit(1); }
  copyFileSync(specPath, snap);
  writeFileSync(specPath, text);
  console.log(JSON.stringify({ wrote: specPath, snapshot: snap, verified: sha(readFileSync(specPath, 'utf8')) === after }));
} else {
  console.log('dry run (no --write)');
}
