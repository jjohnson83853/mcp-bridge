import { readFileSync } from 'node:fs';

const [deltaPath, targetPath] = process.argv.slice(2);
const delta = readFileSync(deltaPath, 'utf8').replace(/\r\n/g, '\n');
const spec = readFileSync(targetPath, 'utf8');
const blocks = [...delta.matchAll(/<<<BLOCK (\d+)>>>\n--- old ---\n([\s\S]*?)\n--- new ---\n([\s\S]*?)\n<<<END>>>/g)];
for (const [, n, oldText] of blocks) console.log(n, (spec.split(oldText).length - 1));
