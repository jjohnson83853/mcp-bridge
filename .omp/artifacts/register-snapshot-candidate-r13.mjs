import { readFileSync, writeFileSync, renameSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { withReviewRegistryLock } from '/home/opencode/.omp/agent/workflow-policy/execution-review-sequence.mjs';
import { canonicalJson, sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const statePath = path.join(root, '.omp/plans/execution-state.json');
const registryPath = path.join(root, '.omp/plans/review-sequence.json');
const candidateId = 'snapshot-recovery-spec-r13';
const revision = 'r13';
const stateRevision = 'r13';
const specPath = '.scratch/snapshot-recovery/spec.md';
const ticketPath = '.scratch/snapshot-recovery/issues/01-untrusted-snapshot-self-recovery.md';
const inputPaths = [
  specPath, ticketPath, '.scratch/mcp-bridge-defects/backlog.md', 'docs/agents/domain.md',
  'docs/agents/issue-tracker.md', 'docs/logging-contract.md', 'go.mod', 'internal/config/config.go',
  'internal/observe/envelope.go', 'internal/observe/log.go', 'internal/observe/log_test.go',
  'internal/observe/edge_test.go', 'internal/observe/metrics.go', 'internal/service/loop.go',
  'internal/service/reload.go', 'internal/storage/store.go', 'internal/storage/snapshot_v1.go',
  'internal/storage/store_test.go', 'internal/storage/store_edge_test.go',
  'internal/storage/snapshot_edge_test.go', 'cmd/mcp-bridge/runtime_edge_test.go',
  'cmd/mcp-bridge/serve.go', 'cmd/mcp-bridge/serve_edge_test.go', 'cmd/mcp-bridge/startup_failure.go',
  'cmd/mcp-bridge/startup_failure_test.go', 'cmd/mcp-bridge/logging_contract_test.go',
  'cmd/mcp-bridge/wiring.go', 'cmd/mcp-bridge/config_envelope_test.go',
];

const sharedInstructions = `Candidate: ${candidateId} @ ${revision}. Author: planner. Review mode: specification only; no implementation exists.

Reviewed inputs (all candidate-bound and hashed):
${inputPaths.map(p => '- ' + p).join('\n')}

Read the r13 specification at ${specPath} (including the four specdd regions and the 6.1 ENVELOPE subsection), the ticket mirror, backlog, docs/logging-contract.md, go.mod, and the referenced code anchors. The candidate boundary is the exact hashed bytes in the review registry; do not treat later edits as reviewed.

Prior evidence: the r12 CodeReviewer round was REQUEST CHANGES (SR12-01..05) and the r12 Critic was REVISE (C1..C4), artifacts .omp/artifacts/snapshot-recovery-fresh-plan/cr-r12-raw.md and ct-r12-raw.md; treat them as evidence, not as reviewed r13 bytes. r13 claims to close all of them in place with no R/AC/T id or envelope change.

This is a read-only independent review. Do not modify files, run implementation tests, implement, dispatch, or claim approval. Treat the dispatch context wrapper as metadata; this task text is authoritative.`;

const effects = { kind: 'read', paths: [specPath, ticketPath] };
const request = (name, taskId, agent) => ({ name, taskId, agent, effects, targetCandidateId: candidateId, targetRevision: revision });

const crTask = `# Target
Independently review snapshot-recovery specification candidate ${candidateId}@${revision} in specification mode. No implementation exists. The r12 review requested changes; verify r13 actually closes the nine recorded findings and introduces no new blocking defect.

# Required checks
- C1/SR12-01: the fatal path must preserve the real cause. Verify R3 and section 4 describe an unexported serve.go error type carrying the classification plus the original cause, that main.go:56-66 errors.As/Is behavior still matches, and that startup_failure.go stays outside the envelope; AC2-4 and AC2-8(a) assert cause preservation. Flag any claim that code outside the 14-file envelope must change.
- C2/SR12-02: a successful Stat of a FIFO, socket, device or symlink to one must be defined: never opened, never read, never renamed; specify per-family classification and a no-block oracle (syscall.Mkfifo subcase, completion inside the deadline).
- C3/SR12-03: prompts-family taxonomy mapping must be per family: decode/trailing corrupt_json, Validate validation_failure, checksum sentinel store.go:103-107 checksum_mismatch, and unsupported_schema not producible by prompts (PromptSchemaVersion). Verify the cited anchors match the code.
- SR12-04: retention must pin total <= QuarantineKeep including future-dated pre-existing names, with deterministic ordering and a future-dated test.
- SR12-05: startup oracles must carry numeric deadlines and intervals (5s / 25ms) consistent with AC2-1 and AC2-9.
- C4: alert coverage must be a verified exception with cited queries (logging-contract.md:607-614) or an explicit non-goal; no unverified alert-behavior claim in R7/ACs.
- Reviewer notes: anchors envelope.go:103 (runtimeErrorInfo) and :244 (addEnvelopeFields) refreshed; section 9.7 corrected.
- Trace R1-R9 to AC1-1..AC1-10, AC2-1..AC2-9 and T1-T3; check missing boundary, precedence, state-transition, warning-count coverage.
- Verify the owner envelope is unchanged: no new/removed/renamed R/AC/T id; T1/T2/T3 file sets only approved paths; 6.1 ENVELOPE matches section 6.
- Apply W09E. Report every finding with class, severity, confidence, exact anchor, consequence and minimal fix. Do not claim unrun tests, git diff or LSP diagnostics.

# Output contract
Use the installed CodeReviewer format: Summary, Files Reviewed, By Severity, Issues, Open Questions, Positive Observations, then exactly '### Recommendation' followed by APPROVE or REQUEST CHANGES and nothing after it. APPROVE only if no blocking-class finding remains at high confidence. Do not modify any file.`;

const ctTask = `# Target
Independently apply the SpecDD/TDD pass policy to snapshot-recovery candidate ${candidateId}@${revision}. No implementation exists; judge only the specification against R1-R9, AC1-1..AC2-9, T1-T3 and the owner envelope.

# Required checks
- Verify the r13 revision closes the r12 findings (cr SR12-01..05, ct C1..C4) with evidence-backed text, not assertions; the r12 artifacts are evidence, not reviewed bytes.
- Judge: single-intake contract consistency, AC traceability (each R has an AC; each AC names requirement ids; T1/T2/T3 own the ACs), envelope containment (14 files only; executor restrictions stay mbtest-only), retention/atomicity invariants, warning record counts, and the section 9 handoff quality.
- Flag owner-decision-required items only for envelope or R/AC changes; none are expected.
- Report blocking findings with class, severity, confidence and the exact anchor plus minimal fix. Do not invent runs, receipts or approvals.

# Output contract
Use the installed Critic format: end with exactly '### Verdict' followed by PASS, REVISE or BLOCKED and nothing after it. REVISE for any blocking high-confidence specification defect; BLOCKED only for missing decisive evidence.`;

const makeSeed = (intent, req, task, solutionSpace) => ({
  intent, instructions: sharedInstructions,
  workflowDispatch: { statePath, schemaVersion: 1, projectRoot: root, feature: 'snapshot-recovery', revision: stateRevision, requests: [req] },
  tasks: [{ name: req.name, agent: req.agent, task, solutionSpace }],
});

const crSeed = makeSeed('Review snapshot-recovery specification r13', request('SnapshotRecoverySpecR13CodeReview', 'CR1', 'code-reviewer'), crTask, 'one review: revise request on a fixed id set; new defects open');
const ctSeed = makeSeed('Critic snapshot-recovery specification r13', request('SnapshotRecoverySpecR13Critic', 'CT1', 'critic'), ctTask, 'one verdict: pass policy on a fixed id set; new defects open');
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
  id: candidateId, revision, authorIds: ['planner'], maxRounds: 4, inputs, targetRevision: revision,
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
  const temp = registryPath + '.' + randomUUID() + '.tmp';
  writeFileSync(temp, JSON.stringify(registry, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
  renameSync(temp, registryPath);
  return { candidates: registry.candidates.length, candidateId, inputs: candidate.inputs.length };
});
for (const [name, seed] of [['cr13', crSeed], ['ct13', ctSeed]]) {
  writeFileSync(path.join(root, '.omp/artifacts/snapshot-recovery-fresh-plan/review-seed-' + name + '.json'), JSON.stringify(seed, null, 2) + '\n');
  writeFileSync(path.join(root, '.omp/artifacts/snapshot-recovery-fresh-plan/review-request-' + name + '.json'), JSON.stringify(seed.workflowDispatch, null, 2) + '\n');
}
console.log(JSON.stringify({ result, cr: candidate.dispatches[0], ct: candidate.dispatches[1] }, null, 2));
