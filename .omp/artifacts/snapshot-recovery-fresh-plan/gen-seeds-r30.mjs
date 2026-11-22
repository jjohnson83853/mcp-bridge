import { readFileSync, writeFileSync, statSync } from 'node:fs';
import path from 'node:path';
const root = '/home/opencode/projects/default/mcp-bridge';
const d = root + '/.omp/artifacts/snapshot-recovery-fresh-plan/';
const state = JSON.parse(readFileSync(root + '/.omp/plans/execution-state.json', 'utf8'));
const abs = p => p.startsWith('/') ? p : root + '/' + p;
const rel = p => path.relative(root, abs(p));
const inputs = [...new Set([rel(state.spec.path), ...state.tasks.flatMap(t => t.effects.paths).filter(p => { try { return statSync(abs(p)).isFile(); } catch { return false; } })])];
const instructions = 'Candidate: snapshot-recovery-spec-r30 @ r30. Author: planner. Review mode: specification only; no implementation exists.\n\nReviewed inputs (exact hashed set):\n' + inputs.map(p => '- ' + p).join('\n') + '\n\nRead the r30 specification, the ticket mirror, docs/logging-contract.md and the referenced code anchors. The candidate boundary is the exact hashed bytes in the registry. Read-only; do not edit files; do not run tests.';
for (const k of ['cr', 'ct']) { const s = JSON.parse(readFileSync(d + 'review-seed-' + k + '-r29.json', 'utf8')); s.instructions = instructions; s.workflowDispatch.revision = 'r30'; const q = s.workflowDispatch.requests[0]; q.name = q.name.replace('R29', 'R30'); q.targetCandidateId = 'snapshot-recovery-spec-r30'; q.targetRevision = 'r30'; s.tasks[0].name = q.name; s.tasks[0].task = s.tasks[0].task.replaceAll('r29', 'r30').replaceAll('R29', 'R30'); writeFileSync(d + 'review-seed-' + k + '-r30.json', JSON.stringify(s, null, 2) + '\n'); }
console.log('seeds written', inputs.length);