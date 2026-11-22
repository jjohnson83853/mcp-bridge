import { readFileSync, writeFileSync, renameSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { withReviewRegistryLock } from '/home/opencode/.omp/agent/workflow-policy/execution-review-sequence.mjs';
import { canonicalJson, sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const statePath = path.join(root, '.omp/plans/execution-state.json');
const registryPath = path.join(root, '.omp/plans/review-sequence.json');
const candidateId = 'snapshot-recovery-spec-r7';
const revision = 'r7';
const stateRevision = 'r7';
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
  'cmd/mcp-bridge/config_envelope_test.go',
];

const sharedInstructions = `Candidate: ${candidateId} @ ${revision}. Author: planner. Review mode: specification only; no implementation exists.

Reviewed inputs (all candidate-bound and hashed):
${inputPaths.map(p => '- ' + p).join('\n')}

Read the r7 specification at ${specPath} (including the four specdd regions and the 6.1 ENVELOPE subsection), the ticket, backlog, docs/logging-contract.md, go.mod, and the referenced code anchors. The candidate boundary is the exact hashed bytes in the review registry; do not treat later edits as reviewed. The r6 CodeReviewer review is evidence at .omp/artifacts/snapshot-review-code-review-r6-20261121T133415Z.md (sha256 8b0c179502a214d7fe238d8f8b992cd31ce2f4e8f9e15a5b4c790ffe925f2dc8); treat it as evidence, not as reviewed r7 bytes.

This is a read-only independent review. Do not modify files, run implementation tests, implement, dispatch, or claim approval. Treat the dispatch context wrapper as metadata; this task text is authoritative.

Risk packet for both stages: (1) R8 MCP_SNAPSHOT_QUARANTINE=off semantics and the owner-confirmed default ON; (2) retention keep-5-newest per family, this-run exclusion and non-fatal prune failure; (3) failure_reason as an optional event-specific addition under AD13 with schema_version 1; (4) errors.Is(err, ErrCorrupt) preservation for existing callers; (5) single-writer startup fatal route and no startup_failed/internal_error for quarantinable conditions; (6) exactly one startup_snapshot_rejected warning per quarantined file; (7) T1/T2/T3 file boundaries and the 6.1 envelope; (8) the r7 specdd markers and packet binding are required by the state contract; (9) the r6 closure: config_envelope_test.go added to T2/§6.1 with pin 129→130, quarantine_path after publication_impact, serve returns stateDirectoryFailure for main to write, prune QuarantineKeep-1, Recovery before sync, AC1-8 RFC3339 offset; no new R/AC/T ids.`;

const effects = { kind: 'read', paths: [specPath, ticketPath] };
const request = (name, taskId, agent) => ({ name, taskId, agent, effects, targetCandidateId: candidateId, targetRevision: revision });

const crTask = `# Target
Independently review the snapshot-recovery specification candidate ${candidateId}@${revision} in specification mode. No implementation exists; absent implementation tests are not a defect at this stage. The bound r6 CodeReviewer review was REQUEST CHANGES (3 HIGH, 2 MEDIUM, 5 LOW); verify this r7 revision closes it.

# Required checks
- Verify every r6 finding is closed by the reviewer's stated fix: (H1) config_envelope_test.go added to T2/§6.1, pin 129→130, quarantine_path after publication_impact in envelope source/docs union/log_test literal; (H2) serve emits accumulated warnings only and returns stateDirectoryFailure(store.Dir()), main remains sole startup_failed writer, AC2-4/AC2-7 one-record semantics hold; (H3) prune excludes this run's file by exact path then unlinks beyond QuarantineKeep-1, leaving 5; (M1) Recovery appended post-rename pre-sync; (M2) AC1-8 offset example is RFC3339; all LOWs: AC2-9 trigger named/polled, §9 risk(4), ticket example, R8 enabled/1 arm, AC1-1 no-retention observable.
- Trace R1-R9 to AC1-1..AC1-10 and AC2-1..AC2-9 and T1-T3; identify missing boundary, error, precedence, state-transition or warning-count coverage.
- Verify the §9 finding ledger closes the exact r6 review; the r6 artifact is evidence, not reviewed r7 bytes.
- Verify the owner-approved envelope is unchanged: no new/removed/renamed R/AC/T id; T1/T2/T3 file sets contain only the approved paths, with config_envelope_test.go as approved-AC coverage; the 6.1 ENVELOPE matches section 6.
- Verify referenced symbols, signatures and error mappings against the supplied anchors; apply W09E to lifecycle, shared state, contract, authority and repair closure.
- Report discovery findings with class, severity, confidence, exact anchor, consequence and minimal fix; do not pre-filter low/uncertain findings.
- Do not claim unrun tests, git diff or LSP diagnostics; state review limits.

# Output contract
Use the installed CodeReviewer format: Summary, Files Reviewed, By Severity, Issues, Open Questions, Positive Observations, then exactly "### Recommendation" followed by APPROVE or REQUEST CHANGES and nothing after it. APPROVE only if no blocking-class finding remains at high confidence. Do not modify any file.`;

const ctTask = `# Target
Independently gate the snapshot-recovery specification candidate ${candidateId}@${revision} as the SpecDD/TDD Critic. The CodeReviewer round for this exact candidate and input set must be APPROVE before dispatch; verify that recorded boundary but do not treat it as closure evidence or narrow this review.

# Required checks
- Review R1-R9, AC1-1..AC1-10 and AC2-1..AC2-9, T1-T3 and the 6.1 ENVELOPE against the ticket/user scope and non-goals; verify R→AC→T traceability, deterministic oracles, negative/integration coverage, dependencies and feasibility.
- Verify the §9 ledger closes the exact r6 CodeReviewer findings; the r6 review is .omp/artifacts/snapshot-review-code-review-r6-20261121T133415Z.md (sha256 8b0c179502a214d7fe238d8f8b992cd31ce2f4e8f9e15a5b4c790ffe925f2dc8).
- Verify the owner-approved envelope: no new/removed/renamed R/AC/T id; T1/T2/T3 boundaries; the single-writer fatal route, four-location envelope insertion, QuarantineKeep-1 arithmetic and Recovery-before-sync ordering preserve R1/R2/R4/R5/R7/R9.
- Apply W09E with the same risk packet: R8 opt-out/default, retention/irreversible prune, failure_reason under AD13, errors.Is preservation, fatal non-regular/rename/sync paths, exact warning count, reviewer/authority boundaries.
- Load skill://observability-contract and review section 7 against AC/T ids; missing decisive evidence is a finding.
- Independently spot-check the highest-risk assumptions; retrieve only missing facts narrowly.

# Output contract
Your final response must contain exactly one "### Verdict" heading, followed on the next nonblank line by exactly PASS, REVISE or BLOCKED. Then include "Spec ID: SNAPREC-SPEC-r7", "Revision: r7", and "Findings by ID: <C1... or none>", plus verified references, coverage and limits, and exact amendments or missing evidence. Missing decisive evidence is BLOCKED; unresolved correctness/safety/acceptance/execution defect is REVISE; PASS only when coverage suffices and no blocking finding remains. At most 3072 UTF-8 bytes. Do not modify files, dispatch, or claim execution permission.`;

const solutionSpace = 'Independent specification review of one fixed candidate; verdict plus evidence-backed findings; no implementation or design widening.';
const makeSeed = (intent, req, task) => ({
  intent,
  instructions: sharedInstructions,
  workflowDispatch: { statePath, schemaVersion: 1, projectRoot: root, feature: 'snapshot-recovery', revision: stateRevision, requests: [req] },
  tasks: [{ name: req.name, agent: req.agent, task, solutionSpace }],
});

const crSeed = makeSeed('Review snapshot-recovery specification r7', request('SnapshotRecoverySpecR7CodeReview', 'CR1', 'code-reviewer'), crTask);
const ctSeed = makeSeed('Critic snapshot-recovery specification r7', request('SnapshotRecoverySpecR7Critic', 'CT1', 'critic'), ctTask);
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
for (const [name, seed] of [['cr7', crSeed], ['ct7', ctSeed]]) {
  writeFileSync(path.join(root, '.omp/artifacts/snapshot-review-seed-' + name + '.json'), JSON.stringify(seed, null, 2) + '\n');
  writeFileSync(path.join(root, '.omp/artifacts/snapshot-review-request-' + name + '.json'), JSON.stringify(seed.workflowDispatch, null, 2) + '\n');
}
console.log(JSON.stringify({ result, cr: candidate.dispatches[0], ct: candidate.dispatches[1] }, null, 2));
