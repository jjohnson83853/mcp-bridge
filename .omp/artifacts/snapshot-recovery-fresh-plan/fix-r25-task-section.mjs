// Move the r25 four-task ledger inside the specdd:tasks section, replacing the stale T1-T3 block.
import { readFileSync, writeFileSync } from 'node:fs';
import { createHash } from 'node:crypto';

const specPath = '/home/opencode/projects/default/mcp-bridge/.scratch/snapshot-recovery/spec.md';
const sha256 = (bytes) => createHash('sha256').update(bytes).digest('hex');
const original = readFileSync(specPath);
writeFileSync('/home/opencode/projects/default/mcp-bridge/.omp/artifacts/snapshot-recovery-fresh-plan/archive-before-r25-tasks-fix-' + sha256(original) + '.md', original, { flag: 'wx' });

const spec = original.toString('utf8');
const startMark = '<!-- specdd:tasks:start -->';
const endMark = '<!-- specdd:tasks:end -->';
const a = spec.indexOf(startMark);
const b = spec.indexOf(endMark, a);
const t1 = spec.indexOf('- **T1 — quarantine core');
if (a < 0 || b < 0 || t1 < 0 || t1 > a) throw new Error('unexpected layout');
const entries = spec.slice(t1, a).replace(/\s+$/, '');
if (!entries.includes('- **T4 — startup integration')) throw new Error('T4 missing from new ledger');
if ((entries.match(/^- \*\*T[1-4] — /gm) ?? []).length !== 4) throw new Error('expected four task entries');
const oldSection = spec.slice(a + startMark.length, b);
if (!oldSection.includes('T1 — storage recovery engine')) throw new Error('old section not the stale block');

const head = spec.slice(0, t1);
const tail = spec.slice(b + endMark.length);
const next = head + startMark + '\n' + entries + '\n\n' + endMark + tail;

const count = (s, needle) => s.split(needle).length - 1;
if (count(next, '- **T1 — ') !== 1 || count(next, '- **T4 — ') !== 1) throw new Error('duplicate entries after splice');
if (next.includes('T1 — storage recovery engine')) throw new Error('stale block survived');
const na = next.indexOf(startMark), nb = next.indexOf(endMark, na);
if (next.indexOf('- **T1 — ', na) > nb) throw new Error('entries still outside section');
if (next.indexOf('- **T4 — ') > nb) throw new Error('T4 outside section');
process.stdout.write(JSON.stringify({ before: sha256(original), after: sha256(Buffer.from(next)), sectionBefore: sha256(oldSection), sectionAfter: sha256(next.slice(na + startMark.length, nb)) }, null, 2) + '\n');
writeFileSync(specPath, next);
