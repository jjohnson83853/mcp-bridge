import { readFileSync, writeFileSync, renameSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { withReviewRegistryLock } from '/home/opencode/.omp/agent/workflow-policy/execution-review-sequence.mjs';
import { canonicalJson, sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const statePath = path.join(root, '.omp/plans/execution-state.json');
const registryPath = path.join(root, '.omp/plans/review-sequence.json');
const candidateId = 'snapshot-recovery-spec-r6';
const revision = 'r6';
const stateRevision = 'r6';
const specPath = '.scratch/snapshot-recovery/spec.md';
const ticketPath = '.scratch/snapshot-recovery/issues/01-untrusted-snapshot-self-recovery.md';
const inputPaths = [
  specPath,
  ticketPath,
  '.scratch/mcp-bridge-defects/backlog.md',
  'docs/agents/domain.md',
  'docs/agents/issue-tracker.md',
  'docs/logging-contract.md',
  'go.mod',
  'internal/config/config.go',
  'internal/observe/envelope.go',
  'internal/observe/log.go',
  'internal/observe/log_test.go',
  'internal/observe/edge_test.go',
  'internal/observe/metrics.go',
  'internal/service/loop.go',
  'internal/service/reload.go',
  'internal/storage/store.go',
  'internal/storage/store_test.go',
  'internal/storage/store_edge_test.go',
  'internal/storage/snapshot_edge_test.go',
  'cmd/mcp-bridge/runtime_edge_test.go',
  'cmd/mcp-bridge/serve.go',
  'cmd/mcp-bridge/serve_edge_test.go',
  'cmd/mcp-bridge/startup_failure.go',
  'cmd/mcp-bridge/startup_failure_test.go',
  'cmd/mcp-bridge/logging_contract_test.go',
  'cmd/mcp-bridge/wiring.go',
];

const sharedInstructions = `Candidate: ${candidateId} @ ${revision}. Author: planner. Review mode: specification only; no implementation exists.

Reviewed inputs (all candidate-bound and hashed):
${inputPaths.map(p => '- ' + p).join('\n')}

Read the r6 specification at ${specPath} (including the four specdd regions and the 6.1 ENVELOPE subsection), the ticket, backlog, docs/logging-contract.md, go.mod, and the referenced code anchors. The candidate boundary is the exact hashed bytes in the review registry; do not treat later edits as reviewed.

This is a read-only independent review. Do not modify files, run implementation tests, implement, dispatch, or claim approval. Treat the dispatch context wrapper as metadata; this task text is authoritative.

Risk packet for both stages: (1) R8 MCP_SNAPSHOT_QUARANTINE=off semantics and the owner-confirmed default ON; (2) retention keep-5-newest per family and non-fatal prune failure; (3) failure_reason as an optional event-specific addition under AD13 with schema_version 1; (4) errors.Is(err, ErrCorrupt) preservation for existing callers; (5) fatal non-regular snapshot path, rename failure and directory-sync failure; (6) exactly one startup_snapshot_rejected warning per quarantined file and no startup_failed/internal_error for quarantinable conditions; (7) T1/T2/T3 file boundaries and the 6.1 envelope; (8) the r6 specdd markers and packet binding are required by the state contract; (9) owner-authorized r6 envelope emitter/registry scope (runtimeErrorInfo, envelopeFieldNames/docs union, emitRecoveryRecords, log.go eventMessage) and the T1 RetentionFailures plural change.`;

const effects = { kind: 'read', paths: [specPath, ticketPath] };
const request = (name, taskId, agent) => ({ name, taskId, agent, effects, targetCandidateId: candidateId, targetRevision: revision });

const crTask = `# Target
Independently review the snapshot-recovery specification candidate ${candidateId}@${revision} in specification mode against the supplied r6 spec, ticket, backlog and code anchors. No implementation exists; absent implementation tests are not a defect at this stage.

# Required checks
- Trace R1-R9 to AC1-1..AC1-10 and AC2-1..AC2-9 and to the T1/T2/T3 planned tests/oracles. Identify any missing boundary, error, precedence or state-transition coverage, including second-restart, valid/absent snapshot, prompt family, fatal non-regular path, rename/sync failure, opt-out, retention and warning-count behavior.
- Verify the §9 finding ledger closes the exact 2026-11-21 r5 CodeReviewer findings (3 HIGH, 6 MEDIUM, 5 LOW); the frozen review is .omp/artifacts/snapshot-review-code-review-r5-20261121T130345Z.md (sha256 176021454ee09bc91f3a74832e29ffb4497774d01705235cf9d19fafabd3160e). Treat it as evidence, not as reviewed r6 bytes.
- Verify the H1/H2 fix: the r6 T2 exact edit set adds runtimeErrorInfo for snapshot_unavailable, envelopeFieldNames plus the docs envelope-fields and log_test independent literal, and defines the emitRecoveryRecords Entry/Fields path; confirm NewEnvelope is not used for these records and the emitters remain field-equal to wiring.go:485-492 and the AD13 row.
- Verify the owner-authorized r6 scope: T2 envelope edit set, log.go eventMessage if required, and the coverage ACs (AC1-6..AC1-10, AC2-9); flag any widening beyond the supplied authorization, T2/T1 boundary or 6.1 ENVELOPE.
- Verify each acceptance oracle is observable and falsifiable, and that the 6.1 ENVELOPE matches the section 6 task scope without widening.
- Verify referenced symbols, signatures, error sentinels and error-code mappings against the supplied source anchors; a matching name alone is not evidence.
- Apply W09E to the exact packet: lifecycle, shared state, contract, authority and repair closure. Check the R8 owner-confirmed default, retention prune irreversibility, the optional failure_reason field against the closed AD13 table, errors.Is preservation, and the no-startup_failed requirement.
- Report full discovery findings with class (specification-contradiction, r-ac-coverage-gap, lifecycle-hazard, state-iteration, ac-not-falsifiable, or editorial), severity (CRITICAL/HIGH/MEDIUM/LOW), confidence, exact file:line anchor or spec section, concrete failure consequence and minimal fix. Do not pre-filter low-severity or uncertain findings into silence.
- Do not claim unrun tests, git diff, or language-server diagnostics: your tools are read-only and those capabilities are absent. State review limits instead.

# Output contract
Use the installed CodeReviewer format: Summary, Files Reviewed, By Severity, Issues, Open Questions, Positive Observations, then a final section with the exact heading "### Recommendation" followed by exactly APPROVE or REQUEST CHANGES and nothing after it. APPROVE only if no blocking-class finding remains at high confidence. Do not modify any file.`;

const ctTask = `# Target
Independently gate the snapshot-recovery specification candidate ${candidateId}@${revision} as the SpecDD/TDD Critic. The CodeReviewer round for this exact candidate and input set is APPROVE; verify that boundary but do not treat it as closure evidence or narrow this review.

# Required checks
- Review R1-R9, AC1-1..AC1-10 and AC2-1..AC2-9, T1-T3 and the 6.1 ENVELOPE against the ticket/user scope and non-goals; verify R to AC to T traceability, deterministic test oracles, negative/integration coverage, task dependencies and feasibility.
- Verify the §9 finding ledger closes the exact 2026-11-21 r5 CodeReviewer findings; the frozen review is .omp/artifacts/snapshot-review-code-review-r5-20261121T130345Z.md (sha256 176021454ee09bc91f3a74832e29ffb4497774d01705235cf9d19fafabd3160e).
- Verify the owner-authorized r6 T2 edit set, the emitRecoveryRecords validation path, the three field-union lists, and the T1 RetentionFailures change preserve the R1/R2/R4/R5/R7/R9 invariants without widening the envelope.
- Apply W09E with the same risk packet: R8 opt-out/default, retention and irreversible prune, failure_reason under AD13, errors.Is preservation, fatal non-regular/rename/sync paths, exact warning count, and reviewer/authority boundaries.
- Load skill://observability-contract and review the spec's section 7 coverage against AC/T IDs; missing decisive evidence or an unexplained gap is a finding.
- Independently spot-check the highest-risk assumptions and supplied evidence; retrieve only missing facts narrowly. Do not invent findings or demand enterprise extras.

# Output contract
Your final response must contain exactly one "### Verdict" heading, followed on the next nonblank line by exactly PASS, REVISE or BLOCKED. Then include the lines "Spec ID: SNAPREC-SPEC-r6", "Revision: r6", and "Findings by ID: <C1... or none>", plus verified references, coverage and limits, and any exact amendments or missing evidence. Missing decisive evidence means BLOCKED; an unresolved correctness, safety, acceptance or execution defect means REVISE; PASS only when coverage suffices and no blocking finding remains. Keep the final response at most 3072 UTF-8 bytes. Do not modify files, dispatch, or claim execution permission.`;

const solutionSpace = 'Independent specification review of one fixed candidate; verdict plus evidence-backed findings; no implementation or design widening.';
const makeSeed = (intent, req, task) => ({
  intent,
  instructions: sharedInstructions,
  workflowDispatch: { statePath, schemaVersion: 1, projectRoot: root, feature: 'snapshot-recovery', revision: stateRevision, requests: [req] },
  tasks: [{ name: req.name, agent: req.agent, task, solutionSpace }],
});

const crSeed = makeSeed('Review snapshot-recovery specification r6', request('SnapshotRecoverySpecR6CodeReview', 'CR1', 'code-reviewer'), crTask);
const ctSeed = makeSeed('Critic snapshot-recovery specification r6', request('SnapshotRecoverySpecR6Critic', 'CT1', 'critic'), ctTask);
const descriptor = (seed, agent, reviewerId) => {
  const req = seed.workflowDispatch.requests[0];
  return {
    name: req.name, agent: req.agent, reviewerId, taskId: req.taskId,
    requestSha256: sha(canonicalJson(req)), targetCandidateId: req.targetCandidateId, targetRevision: req.targetRevision,
    taskSha256: sha(seed.tasks[0].task), instructionsSha256: sha(seed.instructions),
  };
};
const inputs = inputPaths.map(rel => ({ path: rel, sha256: sha(readFileSync(path.join(root, rel))) }));
const candidate = {
  id: candidateId,
  revision,
  authorIds: ['planner'],
  maxRounds: 4,
  inputs,
  targetRevision: revision,
  dispatches: [descriptor(crSeed, 'code-reviewer', 'code-reviewer'), descriptor(ctSeed, 'critic', 'critic')],
};
const result = withReviewRegistryLock(registryPath, {}, { readFileSync, writeFileSync, renameSync }, () => {
  const registry = JSON.parse(readFileSync(registryPath, 'utf8'));
  if (registry.schemaVersion !== 1 || !Array.isArray(registry.candidates)) throw new Error('registry shape');
  const existing = registry.candidates.find(item => item.id === candidateId);
  if (existing) {
    const rounds = (registry.reviewHistory ?? []).filter(r => r.candidateId === candidateId);
    if (existing.codeReview || rounds.length) throw new Error('candidate already reviewed; refusing overwrite');
  }
  registry.candidates = registry.candidates.filter(item => item.id !== candidateId).concat([candidate]);
  const temp = `${registryPath}.${randomUUID()}.tmp`;
  writeFileSync(temp, JSON.stringify(registry, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
  renameSync(temp, registryPath);
  return { candidates: registry.candidates.length, candidateId, inputs: candidate.inputs.length };
});
for (const [name, seed] of [['cr6', crSeed], ['ct6', ctSeed]]) {
  writeFileSync(path.join(root, '.omp/artifacts/snapshot-review-seed-' + name + '.json'), JSON.stringify(seed, null, 2) + '\n');
  writeFileSync(path.join(root, '.omp/artifacts/snapshot-review-request-' + name + '.json'), JSON.stringify(seed.workflowDispatch, null, 2) + '\n');
}
console.log(JSON.stringify({ result, cr: candidate.dispatches[0], ct: candidate.dispatches[1] }, null, 2));
