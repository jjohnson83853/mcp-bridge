import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const outputPath = '/home/opencode/.omp/agent/sessions/-projects-default-mcp-bridge/2026-11-21T04-00-17-335Z_01a1fc43-15b7-750a-82c7-f475df57d285/ReviewHeaderSpecR6.md';
const output = readFileSync(outputPath, 'utf8');
if (!output.trimEnd().endsWith('APPROVE')) throw new Error('unexpected review tail');
const registry = JSON.parse(readFileSync(path.join(root, '.omp/plans/review-sequence.json'), 'utf8'));
const candidate = registry.candidates.find(c => c.id === 'header-parameter-transit-spec-r6');
if (!candidate) throw new Error('candidate missing');
const runs = JSON.parse(readFileSync(path.join(root, '.omp/plans/review-runs.json'), 'utf8'));
const run = runs.runs.find(r => r.candidateId === candidate.id && (r.role ?? 'code-reviewer') === 'code-reviewer');
if (!run) throw new Error('run missing');
const recordedAt = new Date().toISOString();
const observation = {
  role: 'code-reviewer', reviewerId: 'code-reviewer', candidateId: candidate.id, revision: candidate.revision,
  authorIds: candidate.authorIds, inputs: candidate.inputs, dispatches: candidate.dispatches,
  output, invocationId: run.invocationId, recordedAt,
};
const obsName = 'review-' + candidate.id + '-code-reviewer-' + sha(output).slice(0, 12) + '-' + recordedAt.replace(/[-:.]/g, '') + '.json';
const obsPath = path.join(root, '.omp/artifacts', obsName);
writeFileSync(obsPath, JSON.stringify(observation, null, 2) + '\n', { flag: 'wx' });
const findings = [
  { id: 'F1-startup', class: 'lifecycle-hazard', severity: 'MEDIUM', confidence: 'HIGH', anchor: 'internal/service/reload.go:486-500', summary: 'Startup publishes the durable-snapshot catalog before discovery, so the header-bound index is empty while header-bound tools are published; I2 at-every-instant and R2 absolute are false at startup.', fix: 'Initialize the annotation index from the durable snapshot (or withhold header-bound catalog entries until discovery completes) so no published header-bound tool lacks an index entry at startup.' },
  { id: 'F1-over-inclusion', class: 'statement-contradiction', severity: 'MEDIUM', confidence: 'HIGH', anchor: '.scratch/header-parameter-transit/spec.md:103', summary: 'Union install never prunes, so a de-annotated tool keeps its index entry for the process lifetime and body-only calls pay a definitions pass, contradicting R3 and the stated cost bound; no AC covers de-annotation.', fix: 'Install union(header-bound names of the previously published catalog, new discovery set) to prune, or scope R3 and add a de-annotation oracle.' },
  { id: 'F3-new-anchors', class: 'stale-citation', severity: 'LOW', confidence: 'HIGH', anchor: 'internal/service/refresh_test.go:251-303', summary: 'Cited publication-order evidence is not the installation path; dedicated.go:377 is :374 and runtime.go:186 is :188.', fix: 'Correct the anchors to reload.go:618-747, dedicated.go:374 and runtime.go:188; the conclusion holds.' },
];
const binding = {
  binding: {
    candidateId: candidate.id, reviewerId: 'code-reviewer', authorIds: candidate.authorIds,
    inputs: candidate.inputs, dispatches: candidate.dispatches, verdict: 'APPROVE', findings,
    format: 'installed', source: { path: obsPath, sha256: sha(readFileSync(obsPath)) }, recordedAt,
  },
  runBinding: {
    candidateId: candidate.id, invocationId: run.invocationId, fingerprint: run.fingerprint,
    reviewerId: 'code-reviewer', agent: 'code-reviewer', revision: candidate.revision,
  },
  status: 'succeeded',
};
const bindPath = path.join(root, '.omp/artifacts/header-transit-bind-cr6.json');
writeFileSync(bindPath, JSON.stringify(binding, null, 2) + '\n', { flag: 'wx' });
console.log(JSON.stringify({ obsPath, obsSha: binding.binding.source.sha256, bindPath, verdict: 'APPROVE', findings: findings.length }, null, 2));
