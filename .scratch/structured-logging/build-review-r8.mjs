// Emits the r8 review dispatch seeds AND the matching review-sequence.json candidate
// fragment, so the hashes execution-review-sequence.mjs compares cannot drift.
//   node .scratch/structured-logging/build-review-r8.mjs [statePath]
// Then: owner merges review-candidate-r8.json into .omp/plans/review-sequence.json,
// and dispatches review-seed-spec.json (code-reviewer) then review-seed-critic.json (critic).
import { readFileSync, writeFileSync, existsSync } from "node:fs";
import { createHash } from "node:crypto";

const ROOT = "/home/opencode/projects/default/mcp-bridge";
const SC = `${ROOT}/.scratch/structured-logging`;
const FEATURE = "structured-logging-pin-diagnostics";
const REVISION = "r8-pin-diagnostics-narrow9";
const sha = (s) => createHash("sha256").update(s).digest("hex");
const fileSha = (p) => sha(readFileSync(p));

const statePath = process.argv[2] ?? `${ROOT}/.omp/plans/execution-state.json`;
const fallback = `${SC}/new-state-r8.json`;
const stateFile = existsSync(statePath) && JSON.parse(readFileSync(statePath, "utf8")).feature === FEATURE ? statePath : fallback;
const state = JSON.parse(readFileSync(stateFile, "utf8"));
if (state.feature !== FEATURE) throw new Error(`no ${FEATURE} state at ${statePath} or ${fallback}`);
const PACKET = state.packet.digest;

const COMMON_INSTRUCTIONS = `Independent review of spec revision ${REVISION} (feature ${FEATURE}) for the mcp-bridge structured-logging work. You are reviewing the merged specification and its r8 extension, not the implementation. Read-only: do not modify any file. Put your REVIEWER/PACKET/VERDICT block near the top of the report, before the closing recommendation block, which must be the last thing in your response.`;

const inputs = [
  `${SC}/spec.md`,
  `${SC}/decisions.md`,
  `${SC}/issues/08-refresh-pass-lifecycle-records.md`,
  `${SC}/issues/09-pin-blocker-detail.md`,
  `${SC}/issues/10-discovery-attribution-and-readiness-reason.md`,
  `${SC}/issues/11-path-parity-and-logger-fallback.md`,
  `${SC}/issues/12-contract-docs-and-verification-closeout.md`,
  stateFile,
  // The narrowing context: what changed relative to the revision the previous review
  // examined, and that review's own report, so closure can be judged rather than guessed.
  `${SC}/r8-narrowing9-delta.md`,
  `${SC}/review-code-reviewer-narrow8-final.md`,
].map((path) => ({ path, sha256: fileSha(path) }));

const EVIDENCE_ASK = `# What to check
This is a CLOSURE review of revision ${REVISION}. The narrowing delta and the previous review report are among your inputs.

1. Closure: for each finding the previous review raised, state whether this revision closes it, with the exact file and line. A finding the owner recorded as a deferred gap in the delta's residual-gap list and in the backlog is closed for review purposes; an unaddressed one is not.
2. Regressions: state whether the delta changed anything it did not claim to change.
3. New blocking defects: report a NEW finding only if it is HIGH or MEDIUM — a defect that would let a requirement be violated while every criterion passes, or that makes a criterion unfalsifiable. LOW findings (wording, stale citations, advisory notes) are recorded as comments and do NOT block.
4. Marker integrity: the four specdd marker pairs must be unique, correctly ordered and non-nested.

# Verdict rule
APPROVE when every previous finding is closed and no new HIGH/MEDIUM defect exists. REQUEST CHANGES only for an unclosed finding or a new HIGH/MEDIUM defect. Do not re-audit areas the delta did not touch.

# Required output
Report for each area: a verdict (pass / revise), the exact evidence (file and line), and for any revise, the smallest change that would fix it. State the packet digest you reviewed against: ${PACKET}. Do not report a pass for an area you could not verify from the files.`;

const specTask = `# Target
Independent code review of the r8 specification revision \`${REVISION}\` (feature \`${FEATURE}\`) at \`.scratch/structured-logging/spec.md\`, its decision record \`.scratch/structured-logging/decisions.md\`, tickets 08-12, and the r8 projection in \`.omp/plans/execution-state.json\`. Revision under review: r8 extension of SPEC-STRUCTURED-LOGGING.

# Scope
Review only. No edits, no test runs. Bounded: read the files listed in your context's inputs.

${EVIDENCE_ASK}

# Output contract
End your final response with the line \`### Recommendation\` followed, on its own line, by exactly one of \`APPROVE\`, \`REQUEST CHANGES\` or \`COMMENT\` — your own definition's recommendation block, and nothing may follow it. Also state in the report, each on its own line and in this order: your reviewer id as \`REVIEWER: code-reviewer\`, the packet digest as \`PACKET: <64 hex>\`, and your verdict as \`VERDICT: <pass|revise|fail>\` - one lowercase word, and \`pass\` only if your verdict is pass.

# Binding
Record the packet digest \`${PACKET}\` in your report. Your verdict words one of: \`pass\`, \`revise\`, \`fail\`.`;

const criticTask = `# Target
Independent adversarial critique (critic gate) of the r8 specification revision \`${REVISION}\` (feature \`${FEATURE}\`) at \`.scratch/structured-logging/spec.md\`, tickets 08-12, and the r8 projection in \`.omp/plans/execution-state.json\`.

# Method
Assume the specification is wrong and try to break it. Attack the requirements, the acceptance criteria, the observability contract, the failure-recovery path and the task decomposition. Look specifically for: criteria that pass while the requirement is violated, requirements with no criterion, criteria no test can falsify, task boundaries that let two tasks touch the same behaviour, and rulings that contradict each other.

${EVIDENCE_ASK}

# Output contract
State in the report, each on its own line and in this order: \`REVIEWER: critic\`, the packet digest as \`PACKET: <64 hex>\`, and your verdict as \`VERDICT: <pass|revise|fail>\` - one lowercase word, and \`pass\` only if the task ledger passes your critique. End your final response with the line \`### Recommendation\` followed, on its own line, by exactly one of \`APPROVE\`, \`REQUEST CHANGES\` or \`COMMENT\` — nothing may follow it.

# Binding
Record the packet digest \`${PACKET}\` in your report. Your verdict words one of: \`pass\`, \`revise\`, \`fail\`. A critique that finds nothing must say which attacks it ran.`;

const mkSeed = (name, agent, task, i) => ({
  i,
  context: JSON.stringify({
    instructions: COMMON_INSTRUCTIONS,
    inputs,
    packetDigest: PACKET,
    feature: FEATURE,
    revision: REVISION,
  }),
  tasks: [{ name, agent, task, solutionSpace: "One review of a fixed revision: the file set and the revision are given, so only the findings are open." }],
});

// Dispatch names must be unique across registry candidates: execution-review-sequence.mjs
// resolves a dispatched entry to exactly one candidate dispatch by name + agent and
// treats several matches as ambiguous. The r8 candidate already owns the
// ReviewR8PinDiagnosticsSpec/CritiqueR8PinDiagnosticsSpec names.
const specSeed = mkSeed("ReviewR8Narrow9Spec", "code-reviewer", specTask, "Review r8 pin-diagnostics narrow9 specification");
const criticSeed = mkSeed("CritiqueR8Narrow9Spec", "critic", criticTask, "Critique r8 pin-diagnostics narrow9 specification");

const candidate = {
  id: `structured-logging-${REVISION}`,
  revision: REVISION,
  authorIds: ["planner"],
  inputs,
  // These entries carry no taskId, so execution-review-sequence.mjs dispatchBinding()
  // treats them as non-managed: taskSha256 = sha(item.task) and
  // instructionsSha256 = sha(payload.context) - the entire context STRING, not its
  // .instructions field. Both values are taken from the seeds the coordinator dispatches.
  dispatches: [
    {
      name: specSeed.tasks[0].name,
      agent: "code-reviewer",
      reviewerId: "code-reviewer",
      taskSha256: sha(specSeed.tasks[0].task),
      instructionsSha256: sha(specSeed.context),
    },
    {
      name: criticSeed.tasks[0].name,
      agent: "critic",
      reviewerId: "critic",
      taskSha256: sha(criticSeed.tasks[0].task),
      instructionsSha256: sha(criticSeed.context),
    },
  ],
  codeReview: null,
};

writeFileSync(`${SC}/review-seed-spec.json`, `${JSON.stringify(specSeed, null, 2)}\n`);
writeFileSync(`${SC}/review-seed-critic.json`, `${JSON.stringify(criticSeed, null, 2)}\n`);
writeFileSync(`${SC}/review-candidate-r8.json`, `${JSON.stringify(candidate, null, 2)}\n`);
console.log(`packet digest        : ${PACKET}`);
console.log(`state used           : ${stateFile}`);
console.log(`spec.md sha256       : ${fileSha(`${SC}/spec.md`)}`);
console.log(`code-reviewer taskSha: ${sha(specTask)}`);
console.log(`critic taskSha256    : ${sha(criticTask)}`);
console.log("seeds  : review-seed-spec.json, review-seed-critic.json");
console.log("fragment: review-candidate-r8.json (merge into .omp/plans/review-sequence.json candidates[])");
