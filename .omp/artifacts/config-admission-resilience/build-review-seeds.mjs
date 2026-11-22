import { readFileSync, writeFileSync, statSync, renameSync } from 'node:fs';
import path from 'node:path';
import { randomUUID } from 'node:crypto';
import { withReviewRegistryLock } from '/home/opencode/.omp/agent/workflow-policy/execution-review-sequence.mjs';
import { canonicalJson, sha } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const statePath = path.join(root, '.omp/plans/execution-state.json');
const state = JSON.parse(readFileSync(statePath, 'utf8'));
const revision = state.revision;
const candidateId = `${state.feature}-spec-${revision}`;
const specRel = path.relative(root, state.spec.path);

const rel = p => path.relative(root, path.resolve(root, p));
const files = [...new Set([state.spec.path, ...state.tasks.flatMap(t => (t.effects?.paths ?? []))].map(rel))]
  .filter(f => { try { return statSync(path.join(root, f)).isFile(); } catch { return false; } })
  .sort();
const inputs = files.map(f => ({ path: f, sha256: sha(readFileSync(path.join(root, f))) }));

const instructions = [
  `Feature config-admission-resilience, revision ${revision}, candidate ${candidateId}.`,
  `You are an independent read-only reviewer. Do not modify any file, state, plan or artifact.`,
  `Review target: the specification at ${specRel} (sha256 ${state.spec.sha256}), which is bound to the canonical execution state and packet digest.`,
  `Supporting evidence: .scratch/config-admission-resilience/issues/01..04 (owner-confirmed tickets); .scratch/mcp-bridge-defects/backlog.md MB-007/MB-008/MB-009; and the source anchors the spec cites.`,
  `Live stability context: the primary bridge is healthy and serving; the second instance mcp-bridge-network restarts every ~60s with only startup_failed/internal_error (exit 1), and no config_validation_failed event exists for it. The spec must make that class diagnosable and prevent config-admission outage loops.`,
  `Risk packet (W09E): (1) load() runs twice before admission (serve.go:107-131 and Loop.Start) plus once per SIGHUP - exactly-one config_loaded/config_degraded per start is the highest-risk invariant; (2) startup-vs-reload discriminator uses holder.publishedSettings()==nil at load entry; (3) raw-byte fallback copy secret-safety rests on validation rejecting inline credentials; (4) the typed cause must never carry runtime error text; (5) T2's fix files are diagnosis-dependent and its deploy/verification effect is not granted by this envelope.`,
  `Output contract: list findings as bullets starting with [CRITICAL], [HIGH], [MEDIUM] or [LOW], each with file:line evidence. End with a heading '### Recommendation' followed on the next non-empty line by exactly one of APPROVE, REQUEST CHANGES, or COMMENT. Do not invent findings; approve only when the specification is implementable as written.`,
].join('\n\n');

const crTask = [
  'Independently review the specification ./' + specRel + ' for feature config-admission-resilience ' + revision + ' as a specification feasibility and correctness review (not implementation review).',
  '',
  'Check: (a) every owner-confirmed ticket acceptance item in .scratch/config-admission-resilience/issues/01..04 maps to a requirement and an observable AC; (b) each AC has a concrete verification command or evidence method that a verifier can run without reading intent; (c) the design resolves the real startup paths in cmd/mcp-bridge/serve.go, cmd/mcp-bridge/startup_failure.go, internal/observe/envelope.go, internal/config/config.go and bounds.go without contradicting existing contracts; (d) the envelope write paths and task effects actually permit the specified work and nothing wider, and T2 diagnosis-dependent widening is handled honestly; (e) secret hygiene and exactly-one-event semantics are testable; (f) non-goals do not silently drop a ticket requirement.',
  '',
  'Read the cited source files and verify each claim rather than trusting the prose. Report specification defects only; do not report implementation quality. Use severity labels with file:line evidence.',
  '',
  ...(revision === 'r1' ? [] : ['', 'This is a repair review of a revised candidate. Verify each finding in the prior CodeReviewer ledger .omp/artifacts/config-admission-resilience/review-cr-r' + (Number(revision.slice(1)) - 1) + '.txt is actually closed by the revised text, and do not re-raise a resolved finding absent new evidence.']),
  'End with the verdict contract from the wrapper instructions.',
].join('\n');

const ctTask = [
  'Act as the independent SpecDD gate for feature config-admission-resilience ' + revision + '.',
  '',
  'Assess the specification ./' + specRel + ' against the tickets, the canonical requirements/AC set, and the project development workflow. Decide whether implementation may start: are the requirements complete, unambiguous and non-contradictory; do ACs prove them; are the tasks and envelope sufficient and bounded; are secret-safety, compatibility and no-silent-fallback conditions explicit; does the design avoid inventing authority (especially T2 deploy and diagnosis-dependent paths)?',
  '',
  'Report blocking findings first with severity and evidence. End with a heading exactly `### Verdict` followed on the next non-empty line by exactly PASS, REVISE, or BLOCKED. Use BLOCKED only when decisive evidence is missing and you cannot reach a judgement from the cited material.',
].join('\n');

const request = (name, taskId, agent) => ({
  name, taskId, agent, targetCandidateId: candidateId, targetRevision: revision,
  effects: { kind: 'read', paths: [specRel] },
});
const crReq = request('ReviewConfigAdmissionSpecCR' + revision.slice(1), 'CR1', 'code-reviewer');
const ctReq = request('ReviewConfigAdmissionSpecCT' + revision.slice(1), 'CT1', 'critic');
const desc = (q, task) => ({
  name: q.name, agent: q.agent, reviewerId: q.agent, taskId: q.taskId,
  requestSha256: sha(canonicalJson(q)), targetCandidateId: candidateId, targetRevision: revision,
  taskSha256: sha(task), instructionsSha256: sha(instructions),
});
const candidate = {
  id: candidateId, revision, authorIds: ['planner'], maxRounds: 5, inputs, targetRevision: revision,
  dispatches: [desc(crReq, crTask), desc(ctReq, ctTask)],
};
const registryPath = path.join(root, '.omp/plans/review-sequence.json');
withReviewRegistryLock(registryPath, {}, { readFileSync, writeFileSync, renameSync }, () => {
  const registry = JSON.parse(readFileSync(registryPath, 'utf8'));
  registry.candidates = registry.candidates.filter(c => c.id !== candidate.id).concat([candidate]);
  const temp = registryPath + '.' + randomUUID() + '.tmp';
  writeFileSync(temp, JSON.stringify(registry, null, 2) + '\n');
  renameSync(temp, registryPath);
  return { ok: true };
});

const workflowDispatch = q => ({
  statePath, schemaVersion: 1, projectRoot: root, feature: state.feature, revision, requests: [q],
});
const seed = (q, task, solutionSpace) => ({
  intent: q.name,
  instructions,
  workflowDispatch: workflowDispatch(q),
  tasks: [{ name: q.name, agent: q.agent, task, solutionSpace }],
});
const out = '.omp/artifacts/config-admission-resilience/';
writeFileSync(path.join(root, out + 'seed-cr-' + revision + '.json'), JSON.stringify(seed(crReq, crTask, 'independent specification feasibility and correctness review'), null, 2) + '\n');
writeFileSync(path.join(root, out + 'seed-ct-' + revision + '.json'), JSON.stringify(seed(ctReq, ctTask, 'independent SpecDD gate review of specification completeness and implementability'), null, 2) + '\n');
const strip = q => { const { statePath: _s, ...rest } = workflowDispatch(q); return rest; };
writeFileSync(path.join(root, out + 'request-cr-' + revision + '.json'), JSON.stringify(strip(crReq), null, 2) + '\n');
writeFileSync(path.join(root, out + 'request-ct-' + revision + '.json'), JSON.stringify(strip(ctReq), null, 2) + '\n');
console.log(JSON.stringify({
  candidateId, revision, specSha: state.spec.sha256, inputs: inputs.length,
  cr: { requestSha256: candidate.dispatches[0].requestSha256, taskSha256: candidate.dispatches[0].taskSha256 },
  ct: { requestSha256: candidate.dispatches[1].requestSha256, taskSha256: candidate.dispatches[1].taskSha256 },
  instructionsSha256: sha(instructions),
}, null, 2));
