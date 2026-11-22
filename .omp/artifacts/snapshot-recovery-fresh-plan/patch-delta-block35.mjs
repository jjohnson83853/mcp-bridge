// Replace Block 35 in the delta with the correct §9 anchor (the prior anchor copied the §3 variant).
import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';

const root = '/home/opencode/projects/default/mcp-bridge';
const deltaPath = path.join(root, '.omp/artifacts/snapshot-recovery-fresh-plan/r25-delta.md');
const text = readFileSync(deltaPath, 'utf8');

const oldAnchor = 'r24 likewise reads no new source — every r24 amendment is design/AC/decision text level (the CR23-F1/F2 predicate correction, F3 oracles, F4 key definition, F5 citation, and F6 spelling resolve by spec-text reasoning over the r23 rules)**';
const newText = 'r24 likewise reads no new source — every r24 amendment is design/AC/decision text level (the CR23-F1/F2 predicate correction, F3 oracles, F4 key definition, F5 citation, and F6 spelling resolve by spec-text reasoning over the r23 rules); r25 likewise reads no new source — every r25 amendment is design/AC/decision/task text level (the CT24-C5 re-key and counterexample resolve by spec-text reasoning over the r24 D12 rules and the `link(2)` hard-link capability cited from the r22/r23 critic evidence, and the four-ticket decomposition applies the owner-authorized proposal `.omp/artifacts/snapshot-recovery-ticket-decomposition-r25.md` (owner authorization 2026-11-21) verbatim in mapping)**';

const block = ['**Block 35 — §9 intro, r25 disclosure**', '- section: §9 CRITIC HANDOFF intro parenthetical, tail', '- oldAnchor:', '```', oldAnchor, '```', '- newText:', '```', newText, '```', ''].join('\n');

const start = text.indexOf('**Block 35 — §9 intro, r25 disclosure**');
const end = text.indexOf('**Block 36 — §9.2, blocked-condition re-point**');
if (start < 0 || end < 0 || end < start) { console.log('block bounds not found'); process.exit(1); }

const next = text.slice(0, start) + block + text.slice(end);
writeFileSync(deltaPath, next);
console.log(JSON.stringify({ replaced: true, bytes: Buffer.byteLength(next) }));
