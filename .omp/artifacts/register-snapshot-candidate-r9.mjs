import { readFileSync, writeFileSync, renameSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { withReviewRegistryLock } from '/home/opencode/.omp/agent/workflow-policy/execution-review-sequence.mjs';
import { canonicalJson, sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const statePath = path.join(root, '.omp/plans/execution-state.json');
const registryPath = path.join(root, '.omp/plans/review-sequence.json');
const candidateId = 'snapshot-recovery-spec-r9';
const revision = 'r9';
const stateRevision = 'r9';
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

Read the r9 specification at ${specPath} (including the four specdd regions and the 6.1 ENVELOPE subsection), the ticket, backlog, docs/logging-contract.md, go.mod, and the referenced code anchors. The candidate boundary is the exact hashed bytes in the review registry; do not treat later edits as reviewed. The r8 CodeReviewer review is evidence at .omp/artifacts/snapshot-review-code-review-r8-20261121T150000Z.md (sha256 50f681438593aeeabb16a913667eb4872263f385843afa32131a81bbcb313f12); treat it as evidence, not as reviewed r9 bytes.

This is a read-only independent review. Do not modify files, run implementation tests, implement, dispatch, or claim approval. Treat the dispatch context wrapper as metadata; this task text is authoritative.

Risk packet for both stages: (1) R8 MCP_SNAPSHOT_QUARANTINE=off semantics and default ON; (2) retention keep-5-newest per family, this-run exclusion and non-fatal prune failure; (3) failure_reason as an optional AD13 addition with schema_version 1; (4) errors.Is(err, ErrCorrupt) preservation; (5) single-writer startup fatal route; (6) exactly one startup_snapshot_rejected warning per quarantined file; (7) T1/T2/T3 file boundaries and the 6.1 envelope; (8) the r9 specdd markers and packet binding; (9) the r9 closure: deterministic O_EXCL reservation plus 1ns advance for quarantine naming, AC1-9(b) byte-identical collision, §8 unlink bound wording, ticket mirror AC1-5/AC1-9/AC2-2/AC2-5, §9 risk(3) seam oracles, schema-1 corrupt_json declaration.`;

const effects = { kind: 'read', paths: [specPath, ticketPath] };
const request = (name, taskId, agent) => ({ name, taskId, agent, effects, targetCandidateId: candidateId, targetRevision: revision });

const crTask = `# Target
Independently review snapshot-recovery specification candidate ${candidateId}@${revision} in specification mode. No implementation exists. The bound r8 CodeReviewer review was REQUEST CHANGES (1 HIGH, 1 MEDIUM, 2 LOW, 1 open question); verify this r9 revision closes it.

# Required checks
- Verify the HIGH is closed: §4 step 3 reserves the candidate with O_EXCL, advances the timestamp exactly 1ns on collision, retries deterministically up to 4096 attempts, never overwrites a preserved file, and AC1-9(b) seeds exactly one byte-identical pre-existing name and asserts both original byte sets survive at distinct paths; R2 and §8 additive semantics preserved; no new id or path.
- Verify the MEDIUM is closed: §8 states every pre-existing conforming file beyond the newest QuarantineKeep-1 is unlinked, at most QuarantineKeep-1 survive per pass.
- Verify the LOWs are closed: ticket mirror is r9 and includes AC1-5/AC1-9/AC2-2/AC2-5; §9 risk(3) names AC1-5/AC1-6 as seam oracles; schema-1 failures are declared corrupt_json, checksum_mismatch schema-2 only, T1 unchanged.
- Trace R1-R9 to AC1-1..AC1-10 and AC2-1..AC2-9 and T1-T3; identify missing boundary, error, precedence, state-transition or warning-count coverage.
- Verify the §9 ledger closes the exact r8 review; the r8 artifact is evidence, not reviewed r9 bytes.
- Verify the owner envelope is unchanged: no new/removed/renamed R/AC/T id; T1/T2/T3 file sets only approved paths; 6.1 ENVELOPE matches section 6.
- Verify symbols and error mappings against supplied anchors; apply W09E. Report every finding with class, severity, confidence, exact anchor, consequence and minimal fix. Do not claim unrun tests, git diff or LSP diagnostics.

# Output contract
Use the installed CodeReviewer format: Summary, Files Reviewed, By Severity, Issues, Open Questions, Positive Observations, then exactly '### Recommendation' followed by APPROVE or REQUEST CHANGES and nothing after it. APPROVE only if no blocking-class finding remains at high confidence. Do not modify any file.`;

const ctTask = `# Target
Independently gate snapshot-recovery specification candidate ${candidateId}@${revision} as the SpecDD/TDD Critic. The CodeReviewer round for this exact candidate and input set must be APPROVE before dispatch; verify that boundary but do not treat it as closure evidence.

# Required checks
- Review R1-R9, AC1-1..AC1-10, AC2-1..AC2-9, T1-T3 and 6.1 ENVELOPE against ticket scope and non-goals; verify R→AC→T traceability, deterministic oracles, coverage, dependencies and feasibility.
- Verify the §9 ledger closes the exact r8 CodeReviewer findings; the r8 review is .omp/artifacts/snapshot-review-code-review-r8-20261121T150000Z.md (sha256 50f681438593aeeabb16a913667eb4872263f385843afa32131a81bbcb313f12).
- Verify the owner envelope: no new/removed/renamed R/AC/T id; T1/T2/T3 boundaries; single-writer fatal route, four-location envelope insertion, QuarantineKeep-1 arithmetic, recovery-before-sync ordering, and deterministic non-overwriting quarantine naming preserve R1/R2/R4/R5/R7/R9.
- Apply W09E with the risk packet: R8 opt-out/default, retention/irreversible prune, failure_reason under AD13, errors.Is preservation, fatal non-regular/rename/sync paths, exact warning count, reviewer/authority boundaries, schema-1 reason declaration.
- Load skill://observability-contract and review section 7 against AC/T ids; missing decisive evidence is a finding. Spot-check the highest-risk assumptions narrowly.

# Output contract
Your final response must contain exactly one '### Verdict' heading, followed on the next nonblank line by exactly PASS, REVISE or BLOCKED. Then include 'Spec ID: SNAPREC-SPEC-r9', 'Revision: r9', and 'Findings by ID: <C1... or none>', plus verified references, coverage and limits, and exact amendments or missing evidence. Missing decisive evidence is BLOCKED; unresolved correctness/safety/acceptance/execution defect is REVISE; PASS only when coverage suffices and no blocking finding remains. At most 3072 UTF-8 bytes. Do not modify files, dispatch, or claim execution permission.`;

const solutionSpace = 'Independent specification review of one fixed candidate; verdict plus evidence-backed findings; no implementation or design widening.';
const makeSeed = (intent, req, task) => ({
  intent,
  instructions: sharedInstructions,
  workflowDispatch: { statePath, schemaVersion: 1, projectRoot: root, feature: 'snapshot-recovery', revision: stateRevision, requests: [req] },
  tasks: [{ name: req.name, agent: req.agent, task, solutionSpace }],
});

const crSeed = makeSeed('Review snapshot-recovery specification r9', request('SnapshotRecoverySpecR9CodeReview', 'CR1', 'code-reviewer'), crTask);
const ctSeed = makeSeed('Critic snapshot-recovery specification r9', request('SnapshotRecoverySpecR9Critic', 'CT1', 'critic'), ctTask);
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
for (const [name, seed] of [['cr9', crSeed], ['ct9', ctSeed]]) {
  writeFileSync(path.join(root, '.omp/artifacts/snapshot-review-seed-' + name + '.json'), JSON.stringify(seed, null, 2) + '\n');
  writeFileSync(path.join(root, '.omp/artifacts/snapshot-review-request-' + name + '.json'), JSON.stringify(seed.workflowDispatch, null, 2) + '\n');
}
console.log(JSON.stringify({ result, cr: candidate.dispatches[0], ct: candidate.dispatches[1] }, null, 2));
