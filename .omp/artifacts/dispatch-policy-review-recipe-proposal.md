# Dispatch-policy review recipe proposal

Status: proposed documentation, not installed. Scope: review orchestration only; no new authority, gate changes, review retry or state mutation. Put the ordered recipe in `dispatch-policy`; keep only a pointer in `specdd-coordinator` so there is one source of truth.

## Findings that constrain the recipe

- The active native task tool exposes `i/context/tasks`; its declared task-item schema does not expose review target IDs. Local gate source accepts a broader set. Local acceptance of that broader set does not prove that a native call can express it.
- `execution-gate.ts:100-113` parses the WHOLE batch `context` as the JSON wrapper. Managed request metadata, including review targets, is extracted from `workflowDispatch.requests`.
- `execution-review-sequence.mjs:340-354` binds the matching managed request, task text and wrapper instructions. Its recording contract at lines 3-25 requires candidate registration and all planned dispatch descriptors before review; independent output is captured afterward.
- `execution-state.mjs:67-71,928-960` exposes observation binding and receipt recording. `bindWithReviewRound` (`execution-review-sequence.mjs:1000-1003`) requires a uniquely existing candidate. `bind` is NOT initial registration and must not be presented as a bootstrap command. `record-reviews` derives canonical receipts from verified existing observations, not desired verdicts.
- The inspected CLI help and writer-command routing enumerate no initial candidate/task registration command. This is a bounded finding about that CLI, NOT proof that every installed interface lacks a registration mechanism. The exact permitted bootstrap writer remains unestablished in this session.
- Direct review-registry insertion succeeded in this session; direct canonical execution-state editing was denied. Those are distinct permissions. Neither result grants permission to bypass the other.

## Proposed section: Staged review dispatch

### 1. Establish the lane and writers
Read the effective reviewer definitions, active native task schema, current canonical execution state, and W09/W09A/W09E in `workflow-policy/development-workflow.md`. Choose specification or implementation mode explicitly. Supply the same candidate-bound risk evidence to both reviewers.

Before promising review execution, identify the permitted producer for each required record: canonical planner-content persistence, canonical review-task registration, candidate registry registration, independent output capture, and post-review observation binding. Verify operation, input shape, affected paths and authority. Public CLI commands are listed in `execution-state.mjs --help`; follow referenced implementation contracts for exact record schemas.

Completion criterion: every required record already exists and is current, or its creation has a verified permitted writer. If a writer is unresolved or denied, report the exact record/path/operation and evidence gap. Do not manufacture placeholder specifications, role grants or review tasks as an ad-hoc gate repair; do not infer that one denied operation proves that no supported writer exists.

### 2. Freeze the candidate before review
Register a unique candidate ID/revision, planner author IDs, complete reviewed-input manifest with content hashes, and the intended CodeReviewer and Critic dispatch descriptors in `.omp/plans/review-sequence.json`. Preserve existing candidates and lineage history. Carry the actual round cap and applicable owner decisions; do not reset a lineage by renaming it.

Each descriptor identifies `name`, `agent`, independent `reviewerId`, `targetCandidateId`, `targetRevision`, `taskSha256`, and `instructionsSha256`. Managed descriptors also identify `taskId` and `requestSha256`. Follow the module's canonical-JSON hash rule rather than hashing arbitrary formatted JSON. Bind all reviewed files, not only the specification. A new decisive input requires reconciliation before counting a verdict as candidate-bound.

Completion criterion: registry identity, source hashes, dispatch descriptors, canonical spec binding and applicable canonical task/frontier all agree. Registration is metadata, not a review or approval.

### 3. Generate schema-safe managed requests
When the active native task-item schema lacks review target fields, carry them in `workflowDispatch.requests[]` inside the JSON-only batch context wrapper, together with the managed task identity and exact effects. Keep native items within the ACTIVE schema. The seed builder owns serialization; prose belongs in `instructions` and task text, not around the serialized wrapper.

Structural shape (placeholders are not an executable request):

```json
{
  "intent": "Review specification",
  "instructions": "Exact candidate-bound review instructions",
  "workflowDispatch": {
    "statePath": "/absolute/project/.omp/plans/execution-state.json",
    "schemaVersion": 1,
    "projectRoot": "/absolute/project",
    "feature": "current-feature",
    "revision": "current-revision",
    "requests": [{
      "name": "RegisteredReviewName",
      "taskId": "existing-eligible-review-task",
      "agent": "code-reviewer",
      "effects": {"kind": "read", "paths": []},
      "targetCandidateId": "registered-candidate",
      "targetRevision": "registered-revision"
    }]
  },
  "tasks": [{
    "name": "RegisteredReviewName",
    "agent": "code-reviewer",
    "task": "Exact hashed review task",
    "solutionSpace": "Bounded independent review scope"
  }]
}
```

The effect paths above are illustrative; use the actual canonical task's exact effects. A separate unmanaged preparation lane is usable only when policy permits it AND the active native schema can carry the required identity. Do not assume that a broader internal validator extends the exposed tool schema or switch lanes to evade a denial.

### 4. Preflight the exact CodeReviewer invocation
Use the prepare/check commands already documented in this skill. Retain the generated native payload. For corrected/retried dispatches, require both `prepare-dispatch` and `check-dispatch` to exit 0 with the exact matching request and native payload. Pass the object unchanged to native `task` through the declared interface. Local helper success alone is not full dispatch preflight or native admission.

Completion criterion: native admission and actual reviewer launch are observed. Preserve denied invocations and existing retry counters; follow the existing denial/recovery section rather than relabeling the error to obtain a retry.

### 5. Capture and bind actual CodeReviewer output
On completion, save the untouched final output in a fresh immutable observation artifact with the exact candidate/revision, author IDs, reviewer identity, reviewed inputs and dispatch descriptors. Use the installed verdict parser contract; do not translate an ambiguous result into PASS or edit the final response to satisfy a parser.

Run from `~/.omp/agent`:

```sh
node workflow-policy/execution-state.mjs bind <state.json> <binding.json>
```

The binding request has `{binding, runBinding, status, options?}`. Construct its exact fields from the installed `recordBoundReview` / `bindWithReviewRound` contracts; hashes, findings, source and run identity must describe the actual invocation. Do not use `status:"pending"` or a synthetic positive observation to fabricate a prerequisite. Preserve unsuccessful execution separately from a substantive REVISE verdict.

Completion criterion: the existing candidate has the verified independent observation and corresponding recorded run/round. A positive-looking paragraph, agent launch, todo status or raw source file alone is not sufficient.

### 6. Stage Critic, or repair and repeat
Only after the exact candidate/revision has a verified positive CodeReviewer observation, generate and preflight the Critic payload against that same candidate-bound packet. Dispatch Critic separately. On REVISE, follow the planner-owned repair loop and round/authorization rules; invalidate the changed candidate's positive binding, refresh input and dispatch hashes, and obtain CodeReviewer approval again before Critic. Do not rewrite a past receipt to cover new bytes.

After actual Critic completion, capture and bind its independent observation through the same installed transaction contract. Critic observations are separate from CodeReviewer receipts. Where canonical receipt recording is required, run:

```sh
node workflow-policy/execution-state.mjs record-reviews <state.json>
```

Completion criterion: both required verified positive observations apply to the same current candidate and inputs, and any required canonical receipts are recorded. This establishes reviews, not implementation authorization, runtime acceptance or delivery.

## Verification status and required scenario

Observed offline contract check, exit 0: built the saved managed review seed with the installed `buildDispatchPayload`, projected native items down to declared `name/agent/task/solutionSpace` fields, and used `extractDispatchMetadata` to assert that candidate ID/revision survived in the managed requests. Native-item lint also passed. No canonical state was changed and no review was dispatched by this check.

NOT VERIFIED: initial registration through an authorized writer, end-to-end native review admission, CodeReviewer output binding, subsequent Critic launch, and canonical receipt recording. An installed-runtime controlled fixture must exercise that entire sequence before claiming the recipe is operationally validated. Include missing/mismatched target, stale input hashes, reviewer/author identity collision, absent positive first-stage observation, and repaired-candidate invalidation. Never consume a live task's retry allowance merely to test failure handling.
