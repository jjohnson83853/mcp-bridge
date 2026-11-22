import { readFileSync, writeFileSync } from 'node:fs';

const [deltaPath, targetPath] = process.argv.slice(2);
if (!deltaPath || !targetPath) throw new Error('usage: apply-delta.mjs <delta.md> <target.md>');
const delta = readFileSync(deltaPath, 'utf8').replace(/\r\n/g, '\n');
const original = readFileSync(targetPath);
let spec = original.toString('utf8');
const blocks = [...delta.matchAll(/<<<BLOCK (\d+)>>>\n--- old ---\n([\s\S]*?)\n--- new ---\n([\s\S]*?)\n<<<END>>>/g)];
if (blocks.length === 0) throw new Error('no blocks parsed');
for (const [, n, oldText, newText] of blocks) {
  const count = spec.split(oldText).length - 1;
  if (count !== 1) throw new Error(`block ${n}: old occurs ${count} times`);
  spec = spec.replace(oldText, newText);
}
const { createHash } = await import('node:crypto');
const sha256 = (b) => createHash('sha256').update(b).digest('hex');
writeFileSync(`/home/opencode/projects/default/mcp-bridge/.omp/artifacts/snapshot-recovery-fresh-plan/archive-before-apply-${sha256(original)}.md`, original, { flag: 'wx' });
writeFileSync(targetPath, spec);
console.log(JSON.stringify({ blocks: blocks.length, before: sha256(original), after: sha256(Buffer.from(spec)) }, null, 2));
