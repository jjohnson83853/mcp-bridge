// Register the r25 review candidate (CodeReviewer + Critic) in .omp/plans/review-sequence.json.
import { readFileSync, writeFileSync, renameSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { withReviewRegistryLock } from '/home/opencode/.omp/agent/workflow-policy/execution-review-sequence.mjs';
import { canonicalJson, sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const statePath = path.join(root, '.omp/plans/execution-state.json');
const registryPath = path.join(root, '.omp/plans/review-sequence.json');
const candidateId = 'snapshot-recovery-spec-r25';
const revision = 'r25';
const stateRevision = 'r25';
const specPath = '.scratch/snapshot-recovery/spec.md';
const ticketPath = '.scratch/snapshot-recovery/issues/01-untrusted-snapshot-self-recovery.md';
const inputPaths = [specPath, ticketPath, 'docs/logging-contract.md', 'go.mod', 'internal/observe/envelope.go', 'internal/observe/log.go', 'internal/service/loop.go', 'internal/storage/store.go', 'internal/storage/snapshot_v1.go', 'cmd/mcp-bridge/serve.go', 'cmd/mcp-bridge/startup_failure.go', 'cmd/mcp-bridge/wiring.go'];

const sharedInstructions = `Candidate: ${candidateId} @ ${revision}. Author: planner. Review mode: specification only; no implementation exists.

Reviewed inputs:\n${inputPaths.map(p => '- ' + p).join('\n')}

Read the r25 specification at ${specPath} (the four specdd regions plus 6.1 ENVELOPE), the ticket mirror, docs/logging-contract.md and the referenced code anchors. The candidate boundary is the exact hashed bytes in the registry. Prior evidence: .omp/artifacts/snapshot-recovery-fresh-plan/ct-r24-raw.md (REVISE: C5 HIGH, C8 LOW) and cr-r24-raw.md (APPROVE with 2 LOW). Treat them as evidence, not reviewed bytes. The ticket mirror is intentionally stale (section 9.7); it is not the candidate contract.

Read-only independent review. Do not modify files, do not run tests, do not claim runtime verification. Report findings with severity, anchor and confidence; do not invent ids.`;

const effects = { kind: 'read', paths: [specPath, ticketPath] };
const request = (name, taskId, agent) => ({ name, taskId, agent, effects, targetCandidateId: candidateId, targetRevision: revision });

const crTask = `# Target\nReview snapshot-recovery specification candidate ${candidateId}@${revision} (specification only; no implementation). This round makes an owner-authorized plan change and re-applies two carried findings.\n\n# Required checks\n1. **CT24-C5 closure (HIGH, lifecycle hazard).** Verify the D12 moved-entry set is keyed by ORIGINAL CONFIGURED PATH + pre-move entry identity, that ordinary (iii)/(iv) and mutual (m-iii)/(m-iv) partner-versus-self is decided from recorded chain-entry paths and never a bare inode match, that the hard-linked-symlink counterexample is stated, that the inode-only alternative is explicitly rejected, and that the AC2-9 mutual hard-linked variant is executable (exactly [snapshot read_error, prompts orphaned_link], Readlink assertion, fresh regular prompts.json accepted by direct SavePrompts, second-run no-op, counting discrimination: both-moved/self-moved ZERO, clean pair exactly ONE). Check the re-key is echoed consistently in R2, §4 contract, §4(3), §4 alternatives, §8(2), T3, §7, §9 and that D10/D11 protection is untouched.\n2. **Four-ticket decomposition (owner-authorized).** Verify §6 T1-T4 is a complete, disjoint partition: every R id and every AC id owned by exactly one task; dependency edges T1→T2,T3; T2→T3; T1,T2,T3→T4 consistent with the task text; the §6.1 write set unchanged at the same 14 files with T4 added as a task id only; no live (non-historical) stale reference to the old T1-T3 meanings; historical round text preserved and scoped.\n3. **CT24-C8** wording: fixtures described as remaining required planned oracles, no implied executed GREEN; §9.6 unchanged.\n4. **CR24 LOWs**: §9.21 documents \u0060orphaned_label\u0060 → \u0060orphaned_link\u0060, AC1-10(b-dangle) cites (m-ii)+(m-iii). The quoted old token must remain as a quotation where the change is described; only the live reason value must be \u0060orphaned_link\u0060.\n\nCheck regressions: no added/removed/renumbered R/AC id, no envelope path change, no new dependency beyond std-lib, no contradiction with D1-D12 or the ACs, no false review/approval/test claim, C4 and CR23 closures intact.\n\nReturn Findings as a Markdown list, each with severity (CRITICAL/HIGH/MEDIUM/LOW), file:line anchor and confidence, and a one-line recommended fix, followed by a verdict heading on its own line:\n## Verdict: APPROVE, REQUEST CHANGES, or BLOCKED. Do not edit any file.`;

const ctTask = `# Target\nApply the SpecDD pass policy to snapshot-recovery candidate ${candidateId}@${revision} (specification only; no implementation). This round is an owner-authorized plan change plus two carried findings.\n\n# Required checks\n- Verify CT24-C5 is closed at the invariant level, not by another fixture patch: the identity/moved-entry key must be incapable of conflating distinct hard-linked directory entries, partner/self must be decidable from recorded paths, and the stated counterexample must not survive under the new text. Reconstruct the counterexample; report any residual self-healing contradiction or unreachable oracle.\n- Assess the T1-T4 decomposition as a plan: complete ownership of every R/AC id, no orphan or duplicated acceptance, dependency edges sufficient for incremental implementation and integration, and the 14-file envelope unchanged. Report any task whose acceptance cannot be verified independently, and any AC that now has no owning task oracle.\n- Verify CT24-C8 and the two CR24 LOWs with exact text.\n- Confirm every open item remains open and unclaimed: retention-unlink owner checkpoint, originally-dangling non-mutual carve-out, requested non-OpenAI escalation. Reject any implied approval, owner confirmation beyond the decomposition authorization, or executed test result.\n\nReturn Findings with severity (CRITICAL/HIGH/MEDIUM/LOW), anchor, confidence and one-line fix, then a verdict heading on its own line:\n## Verdict: PASS, REVISE, or BLOCKED. Do not edit any file.`;

const makeSeed = (intent, req, task, solutionSpace) => ({ intent, instructions: sharedInstructions, workflowDispatch: { statePath, schemaVersion: 1, projectRoot: root, feature: 'snapshot-recovery', revision: stateRevision, requests: [req] }, tasks: [{ name: req.name, agent: req.agent, task, solutionSpace }] });
const crSeed = makeSeed('Review snapshot-recovery specification r25', request('SnapshotRecoverySpecR25CodeReview', 'CR1', 'code-reviewer'), crTask, 'one review: C5 re-key, decomposition integrity, two LOWs, regression on a fixed id set');
const ctSeed = makeSeed('Critic snapshot-recovery specification r25', request('SnapshotRecoverySpecR25Critic', 'CT1', 'critic'), ctTask, 'one verdict: invariant-level closure plus a plan-change assessment');

const descriptor = (seed, agent, reviewerId) => { const req = seed.workflowDispatch.requests[0]; return { name: req.name, agent: req.agent, reviewerId, taskId: req.taskId, requestSha256: sha(canonicalJson(req)), targetCandidateId: req.targetCandidateId, targetRevision: req.targetRevision, taskSha256: sha(seed.tasks[0].task), instructionsSha256: sha(seed.instructions) }; };

const inputs = inputPaths.map(rel => ({ path: rel, sha256: sha(readFileSync(path.join(root, rel))) }));
const candidate = { id: candidateId, revision, authorIds: ['planner'], maxRounds: 5, inputs, targetRevision: revision, dispatches: [descriptor(crSeed, 'code-reviewer', 'code-reviewer'), descriptor(ctSeed, 'critic', 'critic')] };

const result = withReviewRegistryLock(registryPath, {}, { readFileSync, writeFileSync, renameSync }, () => {
  const registry = JSON.parse(readFileSync(registryPath, 'utf8'));
  if (registry.schemaVersion !== 1 || !Array.isArray(registry.candidates)) throw new Error('registry shape');
  const existing = registry.candidates.find(item => item.id === candidateId);
  if (existing) {
    const rounds = (registry.reviewHistory ?? []).filter(r => r.candidateId === candidateId);
    if (existing.codeReview || rounds.length) throw new Error('already reviewed');
  }
  registry.candidates = registry.candidates.filter(item => item.id !== candidateId).concat([candidate]);
  const temp = registryPath + '.' + randomUUID() + '.tmp';
  writeFileSync(temp, JSON.stringify(registry, null, 2) + '\n');
  renameSync(temp, registryPath);
  return { ok: true, candidate: candidateId, inputs: inputs.length };
});

for (const [name, seed] of [['cr25', crSeed], ['ct25', ctSeed]]) {
  writeFileSync(path.join(root, '.omp/artifacts/snapshot-recovery-fresh-plan/review-seed-' + name + '.json'), JSON.stringify(seed, null, 2) + '\n');
  writeFileSync(path.join(root, '.omp/artifacts/snapshot-recovery-fresh-plan/review-request-' + name + '.json'), JSON.stringify(seed.workflowDispatch, null, 2) + '\n');
}
console.log(JSON.stringify({ result, specSha: sha(readFileSync(path.join(root, specPath))), cr: candidate.dispatches[0].taskSha256, ct: candidate.dispatches[1].taskSha256 }, null, 2));
