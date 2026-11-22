import { readFileSync, writeFileSync, renameSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { withReviewRegistryLock } from '/home/opencode/.omp/agent/workflow-policy/execution-review-sequence.mjs';
import { canonicalJson, sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const statePath = path.join(root, '.omp/plans/execution-state.json');
const registryPath = path.join(root, '.omp/plans/review-sequence.json');
const candidateId = 'snapshot-recovery-spec-r10';
const revision = 'r10';
const stateRevision = 'r10';
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
  'internal/storage/snapshot_v1.go',
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

Read the r10 specification at ${specPath} (including the four specdd regions and the 6.1 ENVELOPE subsection), the ticket, backlog, docs/logging-contract.md, go.mod, and the referenced code anchors. The candidate boundary is the exact hashed bytes in the review registry; do not treat later edits as reviewed. The r9 CodeReviewer review is evidence at .omp/artifacts/snapshot-review-code-review-r9-20261121T153000Z.md (sha256 0137848d5a2b0b7042b0540b397323a683fc83ffddb590f1e47d2fb47f559c5a); treat it as evidence, not as reviewed r10 bytes.

This is a read-only independent review. Do not modify files, run implementation tests, implement, dispatch, or claim approval. Treat the dispatch context wrapper as metadata; this task text is authoritative.

Risk packet for both stages: (1) R8 MCP_SNAPSHOT_QUARANTINE=off semantics and default ON; (2) retention keep-5-newest per family, this-run exclusion and non-fatal prune failure; (3) failure_reason as an optional AD13 addition with schema_version 1; (4) errors.Is(err, ErrCorrupt) preservation; (5) single-writer startup fatal route; (6) exactly one startup_snapshot_rejected warning per quarantined file; (7) T1/T2/T3 file boundaries and the 6.1 envelope; (8) the r10 specdd markers and packet binding; (9) r10 closure: AC2-8(c) reservation-exhaustion oracle, Stat/symlink lifecycle remedy, ticket mirror byte-equality, re-derived store.go anchors, snapshot_v1.go read-path binding, and the owner-authorized r10/r11 planning budget.`;

const effects = { kind: 'read', paths: [specPath, ticketPath] };
const request = (name, taskId, agent) => ({ name, taskId, agent, effects, targetCandidateId: candidateId, targetRevision: revision });

const crTask = `# Target
Independently review snapshot-recovery specification candidate ${candidateId}@${revision} in specification mode. No implementation exists. The bound r9 CodeReviewer review was REQUEST CHANGES (2 MEDIUM, 4 LOW, 3 open questions); verify this r10 revision closes it.

# Required checks
- Verify MEDIUM r-ac-coverage-gap is closed: AC2-8(c) exists in place with no new id; a fixed clock, exactly 4096 pre-seeded conforming names at now+0..now+4095ns plus a corrupt snapshot with known bytes; asserts errors.Is(err, ErrSnapshotUnrecoverable), empty Recoveries, original bytes intact, and family count 4096; T3 has the execution schedule; the §3 arithmetic is corrected.
- Verify MEDIUM lifecycle-hazard is closed: §4 step 1 uses os.Stat on the resolved target; R3/R6 state the regular-file precondition and symlink behavior; a symlinked regular target is admitted; AC1-1(c) and AC2-4's symlink subcase are no new ids; the old Lstat-first fatal is gone.
- Verify the LOWs: ticket mirror is r10 with AC1-1/AC2-4/AC2-8 and schema-1/AC1-9(b)(iv) completed; store.go anchors re-derived; snapshot_v1.go is a §6.1 read path only; statement arithmetic corrected.
- Verify the 3 open questions are closed by explicit ruling or accepted residual without widening the envelope.
- Trace R1-R9 to AC1-1..AC1-10 and AC2-1..AC2-9 and T1-T3; identify missing boundary, error, precedence, state-transition or warning-count coverage.
- Verify the §9 ledger closes the exact r9 review; the r9 artifact is evidence, not reviewed r10 bytes.
- Verify the owner envelope is unchanged: no new/removed/renamed R/AC/T id; T1/T2/T3 file sets only approved paths; 6.1 ENVELOPE matches section 6.
- Apply W09E. Report every finding with class, severity, confidence, exact anchor, consequence and minimal fix. Do not claim unrun tests, git diff or LSP diagnostics.

# Output contract
Use the installed CodeReviewer format: Summary, Files Reviewed, By Severity, Issues, Open Questions, Positive Observations, then exactly '### Recommendation' followed by APPROVE or REQUEST CHANGES and nothing after it. APPROVE only if no blocking-class finding remains at high confidence. Do not modify any file.`;

const ctTask = `# Target
Independently gate snapshot-recovery specification candidate ${candidateId}@${revision} as the SpecDD/TDD Critic after an APPROVE CodeReviewer observation for this exact candidate and input set. Do not treat the CodeReviewer result as closure evidence.

# Required checks
- Review R1-R9, AC1-1..AC1-10, AC2-1..AC2-9, T1-T3 and 6.1 ENVELOPE against ticket scope and non-goals; verify R→AC→T traceability, deterministic oracles, coverage, dependencies and feasibility.
- Verify the §9 ledger closes the exact r9 CodeReviewer findings; the r9 review is .omp/artifacts/snapshot-review-code-review-r9-20261121T153000Z.md (sha256 0137848d5a2b0b7042b0540b397323a683fc83ffddb590f1e47d2fb47f559c5a).
- Verify the owner envelope: no new/removed/renamed R/AC/T id; T1/T2/T3 boundaries; single-writer fatal route, four-location envelope insertion, QuarantineKeep-1 arithmetic, recovery-before-sync ordering, and deterministic non-overwriting quarantine naming preserve R1/R2/R4/R5/R7/R9.
- Apply W09E with the risk packet: R8 opt-out/default, retention/irreversible prune, failure_reason under AD13, errors.Is preservation, fatal non-regular/rename/sync paths, exact warning count, reviewer/authority boundaries, schema-1 reason declaration, AC2-8(c) exhaustion oracle, and Stat/symlink admission.
- Load skill://observability-contract and review section 7 against AC/T ids; missing decisive evidence is a finding. Spot-check the highest-risk assumptions narrowly.

# Output contract
Your final response must contain exactly one '### Verdict' heading, followed on the next nonblank line by exactly PASS, REVISE or BLOCKED. Then include 'Spec ID: SNAPREC-SPEC-r10', 'Revision: r10', and 'Findings by ID: <C1... or none>', plus verified references, coverage and limits, and exact amendments or missing evidence. Missing decisive evidence is BLOCKED; unresolved correctness/safety/acceptance/execution defect is REVISE; PASS only when coverage suffices and no blocking finding remains. At most 3072 UTF-8 bytes. Do not modify files, dispatch, or claim execution permission.`;

const solutionSpace = 'Independent specification review of one fixed candidate; verdict plus evidence-backed findings; no implementation or design widening.';
const makeSeed = (intent, req, task) => ({
  intent,
  instructions: sharedInstructions,
  workflowDispatch: { statePath, schemaVersion: 1, projectRoot: root, feature: 'snapshot-recovery', revision: stateRevision, requests: [req] },
  tasks: [{ name: req.name, agent: req.agent, task, solutionSpace }],
});

const crSeed = makeSeed('Review snapshot-recovery specification r10', request('SnapshotRecoverySpecR10CodeReview', 'CR1', 'code-reviewer'), crTask);
const ctSeed = makeSeed('Critic snapshot-recovery specification r10', request('SnapshotRecoverySpecR10Critic', 'CT1', 'critic'), ctTask);
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
for (const [name, seed] of [['cr10', crSeed], ['ct10', ctSeed]]) {
  writeFileSync(path.join(root, '.omp/artifacts/snapshot-review-seed-' + name + '.json'), JSON.stringify(seed, null, 2) + '\n');
  writeFileSync(path.join(root, '.omp/artifacts/snapshot-review-request-' + name + '.json'), JSON.stringify(seed.workflowDispatch, null, 2) + '\n');
}
console.log(JSON.stringify({ result, cr: candidate.dispatches[0], ct: candidate.dispatches[1] }, null, 2));
