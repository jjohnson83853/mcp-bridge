// Native tool_call policy boundary. The shared evaluator owns feature authority;
// this adapter owns native identity, live cwd, discovery and project composition.
// Not a shell/eval sandbox. No module-global session state.
import { existsSync, lstatSync, mkdirSync, readFileSync, readdirSync, realpathSync, statSync, unlinkSync, writeFileSync } from 'node:fs';
import * as fs from 'node:fs';
import path from 'node:path';
import { EFFECTS } from './execution-contract-schema.mjs';
import { evaluateState } from './execution-state.mjs';
import { canonicalJson, canonicalPath, sha } from './execution-contract-binding.mjs';
import { ownerAmendmentToken } from './execution-amendment.mjs';
import { deliveryComplete, verifyToolAccess } from './execution-transition.mjs';
import { reviewSequenceIssues, recordReviewRun, reviewStallSignal, lineageReviewHistory, classifyReviewFindings } from './execution-review-sequence.mjs';
import { randomUUID } from 'node:crypto';
import { withJournal, withStateLock } from './execution-lock.mjs';

export const READONLY_CANDIDATE_AGENTS = Object.freeze(['scout', 'analyst', 'planner', 'code-reviewer', 'critic', 'architect', 'debugger', 'triage', 'telemetry-scout']);
const READ_ONLY_TOOLS: Readonly<Record<string, true>> = { read: true, grep: true, glob: true, find: true, web_search: true, yield: true, ask: true, todo: true };
const READ_ONLY_MCP_DIAGNOSTIC_TOOLS: Readonly<Record<string, true>> = {
  mcp__tool_call_tool: true,
  mcp__tool_grafana_list_datasources: true,
  mcp__tool_grafana_list_loki_label_names: true,
  mcp__tool_grafana_list_loki_label_values: true,
  mcp__tool_grafana_query_loki_logs: true,
};
const ALTERNATES: Readonly<Record<string, true>> = { subagent: true, subagent_send: true, subagent_resume: true };
const CODES = Object.fromEntries(['E_SCHEMA', 'E_AUTHORIZATION', 'E_REVIEW', 'E_REVIEW_TARGET', 'E_REVIEW_PAUSE', 'E_REVIEW_REGISTRY_CHANGED', 'E_PROVENANCE', 'E_VIEW_BINDING', 'E_TASK_NOT_FRONTIER', 'E_TASK_BLOCKED', 'E_EFFECT_OUTSIDE_ENVELOPE', 'E_PATH_ESCAPE', 'E_DECISION_UNRESOLVED', 'E_RETRY_NOT_AUTHORIZED', 'E_ROLE_NOT_PERMITTED', 'E_DISPATCH_IDENTITY', 'E_DISPATCH_METADATA', 'E_DISPATCH_MISMATCH', 'E_LOCKED', 'E_LOCK_IO', 'E_LOCK_ORDER', 'E_LOCK_REENTRANT', 'E_LOCK_RELEASE', 'E_COORDINATOR_EDIT_FORBIDDEN', 'E_USER_INTERRUPT_ACTIVE', 'E_PROJECT_GATE', 'E_READONLY_NOT_EXEMPT', 'E_ALTERNATE_ROUTE', 'E_INTERNAL', 'E_INVALID_INPUT', 'E_DIRTY_WORKTREE', 'E_NOT_A_REPO', 'E_GIT_STATUS', 'E_RUNTIME_INCOMPATIBLE', 'E_STATE_SOURCE', 'E_CONCURRENCY_LIMIT', 'E_PAYLOAD_TOO_LARGE', 'E_INVALID_SOLUTION_SPACE'].map(code => [code, true]));
const object = value => value !== null && typeof value === 'object' && !Array.isArray(value);
const text = value => typeof value === 'string' && value.trim().length > 0;
const keys = (value, expected, optional = []) => object(value)
  && Object.keys(value).sort().join(',') === [...expected, ...optional.filter(key => Object.hasOwn(value, key))].sort().join(',');
const inside = (root, target) => target === root || target.startsWith(root + path.sep);
function scopesOverlap(left, right) {
  return left.some(p => right.some(q => inside(p, q) || inside(q, p)));
}
const canonicalState = root => path.join(root, '.omp', 'plans', 'execution-state.json');
const badMetadata = () => ({ kind: 'invalid', code: 'E_DISPATCH_METADATA', detail: 'Provide one complete JSON workflowDispatch wrapper with nonempty instructions and exact request fields.' });
const DENIAL_GUIDANCE = Object.freeze({
  E_DISPATCH_METADATA: 'Provide one exact JSON workflowDispatch wrapper with documented keys, nonempty instructions, a valid effect kind, and one request per task; omit surrounding prose.',
  E_CONCURRENCY_LIMIT: 'Limit each native batch to three tasks.',
  E_PAYLOAD_TOO_LARGE: 'Keep each task and wrapper instructions at or below 32,768 characters; place supporting material in an artifact.',
  E_INVALID_SOLUTION_SPACE: 'Provide a non-empty, concrete solutionSpace description of at most 1,000 characters.',
  E_DISPATCH_MISMATCH: 'Match each wrapper request to a native task by exact name and agent; do not add or omit native tasks.',
  E_COORDINATOR_EDIT_FORBIDDEN: 'Coordinator direct edit and write tool calls targeting production repository paths are prohibited. Delegate changes to engineer.',
  E_USER_INTERRUPT_ACTIVE: 'Dispatch rejected: user interrupt lockfile is active in .omp/state/interrupt.lock. Clear the lock to resume.',
  E_STATE_SOURCE: 'Use the project’s current canonical execution-state source. Inspect it before correcting statePath; do not create replacement state or approvals.',
  E_AUTHORIZATION: 'Required owner authorization is missing from the current state; request that specific approval instead of retrying dispatch.',
  E_REVIEW: 'A required passing independent review is missing; obtain it before retrying dispatch.',
  E_REVIEW_REGISTRY_CHANGED: 'The review registry changed after it was read; retry only from a fresh review snapshot.',
  E_REVIEW_TARGET: 'Match each critic or code-reviewer request to the registered candidate ID and revision; register a new candidate revision when the target changes.',
  E_RETRY_NOT_AUTHORIZED: 'Resolve the task-local blocker or apply the explicit owner-directed amendment, reconcile attempt sources, and preflight the amended request; unchanged retries remain bounded.',
  E_EFFECT_OUTSIDE_ENVELOPE: 'Ask once for the exact missing effect path unless the owner already directed it; record that amendment without a new critic receipt, then preflight the exact task/request effects.',
  E_PROJECT_GATE: 'Required project marker or evidence is absent; verify the approved feature and revision before retrying.',
  E_READONLY_NOT_EXEMPT: 'Read-only preparation requires an effective read-only candidate profile; managed read-only work must also be on the runnable research frontier.',
  E_DIRTY_WORKTREE: 'Graphify requires a clean Git worktree; use narrow search instead. Do not stash or commit.',
  E_NOT_A_REPO: 'Graphify requires a verified Git repository; use narrow search instead.',
  E_GIT_STATUS: 'Git status could not be verified, so Graphify is blocked; use narrow search instead.',
  E_INVALID_INPUT: 'Graphify access checks require a valid project root; do not invoke Graphify until it is verified.',
});
const MAX_DIAGNOSTIC_ITEMS = 8;
const MAX_NATIVE_BATCH_ITEMS = 3;
const MAX_NATIVE_TASK_SIZE = 32_768;
const MAX_NATIVE_SOLUTION_SPACE_SIZE = 1_000;
const RESOLVER_CODES = Object.freeze({ E_READONLY_UNRESOLVED: true, E_READONLY_WRITE_CAPABLE: true, E_READONLY_AMBIGUOUS: true, E_READONLY_SHADOWED: true });
const PROFILE_CAUSES = Object.freeze({ 'noncandidate': true, 'discovery-unavailable': true, 'not-found': true, 'ambiguous': true, 'shadowed': true, 'missing-tools': true, 'disallowed-tools': true, 'spawning': true, 'disabled': true, 'frontier': true });
const diagnosticPath = value => typeof value === 'string' ? value.replace(/[\u0000-\u001f\u007f]/g, '').slice(0, 512) : null;
const diagnosticTool = value => typeof value === 'string' && /^[A-Za-z0-9_.:-]{1,64}$/.test(value) ? value : '<invalid>';
function profileRole(name) {
  return READONLY_CANDIDATE_AGENTS.includes(name) || name === 'engineer' ? name : 'noncandidate';
}
function readonlyFailure(name, code, details = {}) {
  return { resolved: false, name: profileRole(name), code, ...details };
}
function readonlyMetadata(profile) {
  if (profile?.cause === 'frontier') return { role: null, resolverCode: null, definitionPath: null, disallowedTools: [], causes: ['frontier'], conflictingDeclarationPaths: [], cause: 'frontier' };
  const causes = [...new Set((profile.causes ?? (profile.cause ? [profile.cause] : [])).filter(cause => PROFILE_CAUSES[cause] === true))].slice(0, MAX_DIAGNOSTIC_ITEMS);
  return {
    role: profileRole(profile.name),
    resolverCode: RESOLVER_CODES[profile.code] === true ? profile.code : 'E_READONLY_UNRESOLVED',
    definitionPath: diagnosticPath(profile.definitionPath),
    disallowedTools: [...new Set((profile.disallowedTools ?? []).map(diagnosticTool))].slice(0, MAX_DIAGNOSTIC_ITEMS),
    causes,
    conflictingDeclarationPaths: [...new Set((profile.conflictingDeclarationPaths ?? []).map(diagnosticPath).filter(Boolean))].sort().slice(0, MAX_DIAGNOSTIC_ITEMS),
  };
}

function profileAdvice(profile) {
  if (profile.cause === 'frontier') return 'The request is not an approved runnable research read; correct the task/effect frontier. The role profile was not evaluated.';
  const role = profile.role ?? profile.name;
  if (role === 'engineer') return 'Engineer is an intentionally mutating noncandidate and cannot use read-only preparation; use the authorized managed dispatch flow.';
  if (role === 'noncandidate' || profile.causes?.includes('noncandidate')) return 'This role is not a read-only candidate; use an authorized read-only candidate for preparation.';
  const issues = profile.causes?.join(', ') || 'profile resolution failed';
  const tools = profile.disallowedTools?.length ? ` Disallowed tools: ${profile.disallowedTools.join(', ')}.` : '';
  const paths = profile.conflictingDeclarationPaths?.length ? ` Conflicting declarations: ${profile.conflictingDeclarationPaths.join(', ')}.` : '';
  const selected = profile.definitionPath ? ` Effective definition: ${profile.definitionPath}.` : '';
  return `The effective ${role ?? 'candidate'} profile failed read-only resolution (${profile.code ?? 'E_READONLY_UNRESOLVED'}; ${issues}). Review installed definition tools, disable/spawn settings and precedence.${tools}${selected}${paths}`;
}

class LegacyMarkerDenial extends Error {}

/** Entire native context/task must be the JSON wrapper, not a brace substring. */
export function extractDispatchMetadata(input) {
  if (!object(input)) return { kind: 'unmanaged' };
  const shape = Array.isArray(input.tasks) ? 'batch' : 'flat';
  const body = shape === 'batch' ? input.context : input.task;
  if (typeof body !== 'string') return { kind: 'unmanaged' };
  let wrapper;
  try { wrapper = JSON.parse(body); }
  catch { return body.trimStart().startsWith('{') ? badMetadata() : { kind: 'unmanaged' }; }
  if (!object(wrapper) || !Object.hasOwn(wrapper, 'workflowDispatch')) return { kind: 'unmanaged' };
  if (wrapperError(wrapper)) return badMetadata();
  const value = wrapper.workflowDispatch;
  const { statePath, ...request } = value;
  return { kind: 'managed', shape, statePath, request };
}

const payloadFailure = (code, field, detail) => ({ code, field, detail });

function nativePayloadFailure(input, shape) {
  if (!object(input)) {
    return payloadFailure('E_DISPATCH_METADATA', 'payload', 'Provide a native dispatch payload object.');
  }
  const items = shape === 'batch' ? (Array.isArray(input.tasks) ? input.tasks : []) : [input];
  if (shape === 'batch' && items.length > MAX_NATIVE_BATCH_ITEMS) {
    return payloadFailure('E_CONCURRENCY_LIMIT', 'tasks.concurrency', 'A native batch may contain at most three tasks.');
  }
  const body = shape === 'batch' ? input.context : input.task;
  let wrapper = null;
  if (typeof body === 'string') {
    try { wrapper = JSON.parse(body); }
    catch {}
  }
  if (object(wrapper) && typeof wrapper.instructions === 'string') {
    if (wrapper.instructions.length > MAX_NATIVE_TASK_SIZE) {
      return payloadFailure('E_PAYLOAD_TOO_LARGE', 'context.instructions.size', 'Wrapper instructions exceed the 32,768-character limit.');
    }
  }

  for (const item of items) {
    if (!object(item)) continue;
    if (typeof item.task === 'string') {
      if (item.task.length > MAX_NATIVE_TASK_SIZE) {
        return payloadFailure('E_PAYLOAD_TOO_LARGE', 'task.size', 'A task prompt exceeds the 32,768-character limit.');
      }
    }
    if (typeof item.solutionSpace !== 'string') {
      return payloadFailure('E_INVALID_SOLUTION_SPACE', 'task.solutionSpace', 'solutionSpace must be a non-empty description of at most 1,000 characters.');
    }
    const solutionSpace = item.solutionSpace.trim();
    if (solutionSpace.length === 0 || solutionSpace.length > MAX_NATIVE_SOLUTION_SPACE_SIZE) {
      return payloadFailure('E_INVALID_SOLUTION_SPACE', 'task.solutionSpace', 'solutionSpace must be a non-empty description of at most 1,000 characters.');
    }
  }
  return null;
}
function nativeItems(input, shape, validatePayload = true) {
  if (!object(input)) return null;
  const top = shape === 'batch' ? ['i', 'context', 'tasks'] : ['i', 'name', 'agent', 'task', 'solutionSpace', 'effort', 'outputSchema', 'schemaMode', 'isolated', 'targetCandidateId', 'targetRevision'];
  if (Object.keys(input).some(key => !top.includes(key)) || (Object.hasOwn(input, 'i') && !text(input.i))) return null;
  if (shape === 'batch' && !text(input.context)) return null;
  const items = shape === 'batch' ? input.tasks : [input];
  if (!Array.isArray(items) || items.length === 0 || (validatePayload && nativePayloadFailure(input, shape))) return null;
  const seen = new Set();
  for (const item of items) {
    if (!object(item) || !['name', 'agent', 'task', 'solutionSpace'].every(key => text(item[key]))) return null;
    if (Object.keys(item).some(key => !['i', 'name', 'agent', 'task', 'solutionSpace', 'effort', 'outputSchema', 'schemaMode', 'isolated', 'targetCandidateId', 'targetRevision'].includes(key))) return null;
    if ((Object.hasOwn(item, 'effort') && !['lo', 'med', 'hi'].includes(item.effort))
      || (Object.hasOwn(item, 'schemaMode') && !['permissive', 'strict'].includes(item.schemaMode))
      || (Object.hasOwn(item, 'isolated') && typeof item.isolated !== 'boolean')
      || (Object.hasOwn(item, 'outputSchema') && item.outputSchema !== null
        && !object(item.outputSchema) && !['boolean', 'string'].includes(typeof item.outputSchema))) return null;
    if (seen.has(item.name.toLowerCase())) return null;
    seen.add(item.name.toLowerCase());
  }
  return items;
}
export function lintUnmanagedPayload(input) {
  const shape = Array.isArray(input?.tasks) ? 'batch' : 'flat';
  const lintFailure = nativePayloadFailure(input, shape);
  return { shape, lintFailure, items: lintFailure ? null : nativeItems(input, shape, false) };
}
function nativeMapping(parsed, items) {
  return items !== null && items.length === parsed.request.requests.length
    && items.every(item => parsed.request.requests.some(entry => entry.name === item.name && entry.agent === item.agent));
}
class ReviewRegistryChangedError extends Error {}
function reviewPauseResult(details, message) {
  return { block: true, code: 'E_REVIEW_PAUSE',
    reason: `workflow dispatch gate: E_REVIEW_PAUSE: ${message}`, ...details };
}
function loadReviewRegistry(root, fsApi) {
  const plans = path.join(root, '.omp', 'plans');
  if (fsApi.realpathSync(plans) !== plans) throw Error('redirected review registry');
  const file = path.join(plans, 'review-sequence.json');
  const stat = fsApi.lstatSync(file);
  if (stat.isSymbolicLink() || !stat.isFile() || fsApi.realpathSync(file) !== file) throw Error('invalid review registry');
  const sourceText = fsApi.readFileSync(file, 'utf8');
  const registry = JSON.parse(sourceText);
  if (registry?.schemaVersion !== 1 || !Array.isArray(registry.candidates)) throw Error('invalid review registry');
  return { file, registry, sourceBytes: Buffer.from(sourceText, 'utf8') };
}
function matchingReviewCandidates(registry, entries) {
  const reviewEntries = entries.filter(entry => ['critic', 'code-reviewer'].includes(entry?.agent));
  if (!reviewEntries.length) return [];
  return registry.candidates.filter(candidate => {
    if (!object(candidate) || !Array.isArray(candidate.dispatches)) throw Error('invalid review candidate');
    return candidate.dispatches.some(dispatch => reviewEntries.some(entry =>
      dispatch?.name === entry.name && dispatch.agent === entry.agent && dispatch.taskId === entry.taskId));
  });
}
function candidateReviewHistory(candidate, registry, lineages) {
  const sources = [candidate.reviewsHistory, candidate.reviewHistory, registry.reviewsHistory, registry.reviewHistory];
  const merged = new Map();
  const unkeyed = [];
  for (const source of sources) {
    if (source === undefined) continue;
    if (!Array.isArray(source)) throw Error('invalid review history');
    for (const entry of source) {
      if (!text(entry?.verdict) || !Array.isArray(entry.findings)
        || (entry.kind !== undefined && entry.kind !== 'round')) continue;
      const id = entry.candidateId ?? entry.id;
      const entryLineage = text(entry.lineageKey) ? entry.lineageKey
        : text(id) ? id.replace(/-r\d+$/, '') || id : null;
      if (!entryLineage || !lineages.has(entryLineage)) continue;
      if (!text(id) || !text(entry.revision)) {
        unkeyed.push(entry);
        continue;
      }
      const key = JSON.stringify([id, entry.revision]);
      const previous = merged.get(key);
      if (previous && previous.verdict !== entry.verdict) {
        throw Error('conflicting review verdicts for one candidate revision');
      }
      const entrySeq = Number.isInteger(entry.seq) ? entry.seq : undefined;
      const previousSeq = Number.isInteger(previous?.seq) ? previous.seq : undefined;
      if (!previous || (entrySeq !== undefined && (previousSeq === undefined || entrySeq >= previousSeq))
        || (entrySeq === undefined && previousSeq === undefined)) merged.set(key, entry);
    }
  }
  return [...merged.values(), ...unkeyed];
}
function resolvedReviewLineage(registry, lineageKey) {
  const history = lineageReviewHistory(registry, lineageKey);
  if (history.issues.length) throw Error('invalid review lineage');
  return history.aliasChain.at(-1) ?? lineageKey;
}
function candidateReviewLineage(candidate, registry) {
  const candidateId = candidate.id ?? candidate.candidateId;
  if (!text(candidateId)) throw Error('review candidate id');
  const candidateLineage = candidateId.replace(/-r\d+$/, '') || candidateId;
  const lineage = lineageReviewHistory(registry, candidateLineage);
  if (lineage.issues.length) throw Error('invalid review lineage');
  const aliasChain = lineage.aliasChain.length ? lineage.aliasChain : [candidateLineage];
  const lineages = new Set(aliasChain);
  const history = candidateReviewHistory(candidate, registry, lineages);
  return { candidateLineage, aliasChain, history };
}
function signalForReviewCandidate(candidate, registry) {
  const { candidateLineage, aliasChain, history } = candidateReviewLineage(candidate, registry);
  const signalCandidate = aliasChain.length > 1
    ? { ...candidate, id: aliasChain[0], reviewLineages: aliasChain } : candidate;
  return {
    ...reviewStallSignal(signalCandidate, registry, history),
    lineageKey: aliasChain.at(-1) ?? candidateLineage,
  };
}
function matchesReviewPause(candidate, registry, pause) {
  try {
    return signalForReviewCandidate(candidate, registry).lineageKey
      === resolvedReviewLineage(registry, pause.lineageKey);
  } catch {
    return false;
  }
}
function validReviewPause(pause) {
  return object(pause) && text(pause.pauseId) && text(pause.lineageKey) && text(pause.revision)
    && Array.isArray(pause.questionIds) && pause.questionIds.length > 0
    && text(pause.askedAt) && /^[a-f0-9]{64}$/.test(pause.packetDigest);
}
export function reviewPauseDenial(root, entries, state, deps = { fs }) {
  const pause = state?.reviewPause;
  if (!validReviewPause(pause)) return null;
  let matches;
  try {
    const { registry } = loadReviewRegistry(root, deps.fs);
    matches = matchingReviewCandidates(registry, entries).filter(candidate =>
      matchesReviewPause(candidate, registry, pause));
  } catch {
    return null;
  }
  return matches.length
    ? reviewPauseResult({ reviewPause: pause }, 'This review lineage is paused for owner input.')
    : null;
}

function validateWorkflowDispatch(value) {
  if (!keys(value, ['statePath', 'schemaVersion', 'projectRoot', 'feature', 'revision', 'requests'])
    || !['statePath', 'projectRoot', 'feature', 'revision'].every(key => text(value[key]))
    || value.schemaVersion !== 1 || !Array.isArray(value.requests) || value.requests.length === 0) {
    return 'context.workflowDispatch';
  }
  const names = new Set();
  const ids = new Set();
  for (const [index, entry] of value.requests.entries()) {
    if (!keys(entry, ['name', 'taskId', 'agent', 'effects'], ['targetCandidateId', 'targetRevision'])
      || !['name', 'taskId', 'agent'].every(key => text(entry[key]))) return `context.workflowDispatch.requests[${index}]`;
    if ((Object.hasOwn(entry, 'targetCandidateId') && !text(entry.targetCandidateId))
      || (Object.hasOwn(entry, 'targetRevision') && !text(entry.targetRevision))) return `context.workflowDispatch.requests[${index}]`;
    if (!keys(entry.effects, ['kind', 'paths'])) return `context.workflowDispatch.requests[${index}].effects`;
    if (!EFFECTS.includes(entry.effects.kind)) return `context.workflowDispatch.requests[${index}].effects.kind`;
    if (!Array.isArray(entry.effects.paths) || !entry.effects.paths.every(text)
      || new Set(entry.effects.paths).size !== entry.effects.paths.length) return `context.workflowDispatch.requests[${index}].effects.paths`;
    if (names.has(entry.name.toLowerCase()) || ids.has(entry.taskId)) return 'context.workflowDispatch.requests';
    names.add(entry.name.toLowerCase());
    ids.add(entry.taskId);
  }
  return null;
}

function wrapperError(wrapper) {
  if (!object(wrapper) || !Object.hasOwn(wrapper, 'workflowDispatch')
    || Object.keys(wrapper).some(key => !['workflowDispatch', 'instructions'].includes(key))) return 'context';
  if (!text(wrapper.instructions)) return 'context.instructions';
  return validateWorkflowDispatch(wrapper.workflowDispatch);
}

/** Same complete native contract for builder, local preflight and live admission. */
export function validateDispatchPayload(payload) {
  if (!object(payload)) {
    return { ok: false, ...payloadFailure('E_DISPATCH_METADATA', 'payload', 'Provide a native dispatch payload object.') };
  }
  const shape = Array.isArray(payload.tasks) ? 'batch' : 'flat';
  const lintFailure = nativePayloadFailure(payload, shape);
  if (lintFailure) return { ok: false, ...lintFailure };
  const body = shape === 'batch' ? payload.context : payload.task;
  let wrapper;
  try { wrapper = JSON.parse(body); }
  catch {
    return { ok: false, ...payloadFailure('E_DISPATCH_METADATA', 'context', 'Provide a complete JSON workflowDispatch wrapper.') };
  }
  const field = wrapperError(wrapper);
  if (field) {
    return { ok: false, ...payloadFailure('E_DISPATCH_METADATA', field, `Correct the workflowDispatch wrapper field: ${field}.`) };
  }
  const parsed = { shape, request: wrapper.workflowDispatch };
  const items = nativeItems(payload, shape, false);
  if (!items || items.length !== parsed.request.requests.length) {
    return { ok: false, ...payloadFailure('E_DISPATCH_MISMATCH', 'tasks', 'Provide one valid native task for each workflowDispatch request.') };
  }
  if (!nativeMapping(parsed, items)) {
    return { ok: false, ...payloadFailure('E_DISPATCH_MISMATCH', 'tasks[].name', 'Match each workflowDispatch request to a native task by exact name and agent.') };
  }
  return { ok: true, shape };
}

function dispatchScopes(request, dependencies = {}, activePaths = []) {
  const deps = { realpathSync, lstatSync, ...dependencies };
  const scopes = [];
  const previous = [...activePaths];
  try {
    const root = canonicalPath(request.projectRoot, process.cwd(), deps);
    for (const [index, entry] of request.requests.entries()) {
      if (entry.effects.kind === 'read') continue;
      const paths = entry.effects.paths.map(p => canonicalPath(p, root, deps));
      if (paths.some(p => !inside(root, p))) return { error: 'E_PATH_ESCAPE' };
      if (paths.some(p => previous.some(q => inside(p, q) || inside(q, p)))) return { error: 'E_EFFECT_OUTSIDE_ENVELOPE' };
      scopes.push({ name: entry.name, index, paths });
      previous.push(...paths);
    }
  } catch { return { error: 'E_PATH_ESCAPE' }; }
  return { scopes, error: null };
}

/** Local scope check; the live gate also reserves against active native calls. */
export function dispatchScopeError(request, dependencies = {}) {
  return dispatchScopes(request, dependencies).error;
}

/** Build an immutable native batch payload from validated dispatch inputs. */
export function buildDispatchPayload(seed) {
  if (!keys(seed, ['intent', 'instructions', 'workflowDispatch', 'tasks'])) {
    return { ok: false, ...payloadFailure('E_DISPATCH_METADATA', 'input', 'Provide intent, instructions, workflowDispatch, and tasks.') };
  }
  if (!text(seed.intent)) {
    return { ok: false, ...payloadFailure('E_DISPATCH_METADATA', 'intent', 'Provide non-empty dispatch intent.') };
  }
  if (!text(seed.instructions)) {
    return { ok: false, ...payloadFailure('E_DISPATCH_METADATA', 'instructions', 'Provide non-empty wrapper instructions.') };
  }
  let payload;
  try {
    payload = {
      i: seed.intent,
      context: JSON.stringify({ instructions: seed.instructions, workflowDispatch: seed.workflowDispatch }),
      tasks: structuredClone(seed.tasks),
    };
  } catch {
    return { ok: false, ...payloadFailure('E_DISPATCH_METADATA', 'input', 'Provide JSON-serializable dispatch inputs.') };
  }
  const validation = validateDispatchPayload(payload);
  return validation.ok ? { ok: true, payload } : validation;
}

const MAX_DENIED_DISPATCH_ATTEMPTS = 2;
const MAX_RETRY_TRACKS = 128;
const RESUME_CONFLICT_GUIDANCE = 'This settled child owns mutating scopes that overlap an active writer reservation; wait for that writer to settle before resuming the child.';
const RETRYABLE_CORRECTIONS = Object.freeze({ E_DISPATCH_METADATA: true, E_DISPATCH_MISMATCH: true });
const OWNER_RECOVERABLE: Readonly<Record<string, true>> = { E_SCHEMA: true, E_STATE_SOURCE: true, E_DISPATCH_METADATA: true, E_DISPATCH_MISMATCH: true, E_EFFECT_OUTSIDE_ENVELOPE: true, E_TASK_NOT_FRONTIER: true, E_TASK_BLOCKED: true, E_RETRY_NOT_AUTHORIZED: true, E_READONLY_NOT_EXEMPT: true };
function retryScope(input) {
  if (!object(input)) return sha('invalid-input');
  const bodyKey = Array.isArray(input.tasks) ? 'context' : 'task';
  let dispatch;
  if (typeof input[bodyKey] === 'string') {
    try { dispatch = JSON.parse(input[bodyKey])?.workflowDispatch; }
    catch {}
  }
  const nativeTasks = (Array.isArray(input.tasks) ? input.tasks : [input]).map(task => {
    const taskId = object(task) && text(task.taskId) ? task.taskId : null;
    if (taskId) return `id:${taskId}`;
    const agent = object(task) && text(task.agent) ? task.agent : '';
    const name = object(task) && text(task.name) ? task.name : '';
    return `native:${agent}\0${name}`;
  }).sort();
  const requests = object(dispatch) && Array.isArray(dispatch.requests)
    ? dispatch.requests.map(request => {
      const taskId = object(request) && text(request.taskId) ? request.taskId : null;
      if (taskId) return `id:${taskId}`;
      const agent = object(request) && text(request.agent) ? request.agent : '';
      const name = object(request) && text(request.name) ? request.name : '';
      return `native:${agent}\0${name}`;
    }).sort()
    : nativeTasks;
  return sha(canonicalJson({
    projectRoot: object(dispatch) && text(dispatch.projectRoot) ? dispatch.projectRoot : null,
    feature: object(dispatch) && text(dispatch.feature) ? dispatch.feature : null,
    revision: object(dispatch) && text(dispatch.revision) ? dispatch.revision : null,
    requests,
  }));
}

function retryKey(ctx, fs, scope) {
  if (!text(ctx?.cwd) || !path.isAbsolute(ctx.cwd)) return null;
  let root;
  try { root = fs.realpathSync(ctx.cwd); }
  catch { return null; }
  const agentId = object(ctx.agent) && text(ctx.agent.id) ? ctx.agent.id : '';
  return `${root}\0${agentId}\0${scope}`;
}

function retryIdentity(input) {
  if (!object(input)) return null;
  const normalized = { ...input };
  const bodyKey = Array.isArray(input.tasks) ? 'context' : 'task';
  if (typeof input[bodyKey] === 'string') {
    try { normalized[bodyKey] = JSON.parse(input[bodyKey]); }
    catch {}
  }
  return sha(canonicalJson(normalized));
}

function isRequestShapeFailure(code, location) {
  if (code === 'E_DISPATCH_METADATA' || code === 'E_DISPATCH_MISMATCH') return true;
  return code === 'E_SCHEMA' && typeof location === 'string'
    && (location === 'request' || location.startsWith('request.'));
}

function rememberDeniedDispatch(history, key, code, identity, amendment, requestShapeOnly = false) {
  const previous = history.get(key);
  if (code === 'E_STATE_SOURCE' && previous?.code === code && previous.identity !== identity) return false;
  const correctionDenial = previous?.correctionUsed === true && previous.attempts === MAX_DENIED_DISPATCH_ATTEMPTS;
  if ((previous?.attempts ?? 0) >= MAX_DENIED_DISPATCH_ATTEMPTS && !correctionDenial) return false;
  history.delete(key);
  history.set(key, {
    attempts: (previous?.attempts ?? 0) + 1,
    code,
    identity,
    amendment,
    requestShapeOnlyFailures: requestShapeOnly ? (previous?.requestShapeOnlyFailures ?? 0) + 1 : 0,
    correctionUsed: previous?.correctionUsed ?? false,
  });
  if (history.size > MAX_RETRY_TRACKS) history.delete(history.keys().next().value);
  return true;
}

function allTargetsPendingAndUnblocked(state, request) {
  if (!Array.isArray(state?.tasks) || !Array.isArray(state.blockers)
    || !Array.isArray(request?.requests) || request.requests.length === 0) return false;
  return request.requests.every(entry => {
    const task = state.tasks.find(candidate => candidate.id === entry.taskId);
    return task?.status === 'pending'
      && !state.blockers.some(blocker => blocker.taskId === entry.taskId);
  });
}
function ownerReopens(previous, amendment) {
  return previous && amendment && amendment !== previous.amendment && OWNER_RECOVERABLE[previous.code] === true;
}

const denialCodes = new WeakMap();

/** Runtime discovery is injected from the actual installed SDK, never emulated.
 * Discovery returns its effective first-wins definitions plus searchedDirs.
 * Inspect declaration collisions conservatively, without selecting another role.
 */
export async function resolveReadOnlyAgent(name: string, { cwd, discoverAgents, deps = { readFileSync, readdirSync } }: { cwd?: string; discoverAgents?: (cwd?: string) => Promise<{ agents?: Array<{ name: string; filePath?: string; tools?: string[]; spawns?: string[]; disable?: boolean }>; searchedDirs?: string[] }>; deps?: Record<string, unknown> } = {}) {
  if (!READONLY_CANDIDATE_AGENTS.includes(name)) return readonlyFailure(name, 'E_READONLY_UNRESOLVED', { cause: 'noncandidate' });
  if (typeof discoverAgents !== 'function') return readonlyFailure(name, 'E_READONLY_UNRESOLVED', { cause: 'discovery-unavailable' });
  const discovery = await discoverAgents(cwd);
  const definitions = discovery?.agents?.filter(agent => agent.name === name) ?? [];
  if (definitions.length !== 1) {
    const conflictingDeclarationPaths = definitions.map(definition => diagnosticPath(definition.filePath)).filter(Boolean);
    return readonlyFailure(name, 'E_READONLY_UNRESOLVED', {
      cause: definitions.length ? 'ambiguous' : 'not-found',
      definitionPath: null,
      conflictingDeclarationPaths,
    });
  }
  // Native discovery deduplicates names; inspect its declared search precedence
  // without choosing a different definition when a collision is found.
  const definition = definitions[0];
  for (const dir of discovery.searchedDirs ?? []) {
    let files;
    try { files = deps.readdirSync(dir, { withFileTypes: true }); }
    catch (error) { if (error?.code === 'ENOENT') continue; throw error; }
    const claims = files.filter(file => file.isFile() && file.name.endsWith('.md')).filter(file => {
      const content = deps.readFileSync(path.join(dir, file.name), 'utf8');
      const claim = content.match(/^name:\s*["']?([^\r\n"']+)["']?\s*$/m)?.[1]?.trim();
      return claim === name || file.name === `${name}.md`;
    });
    const conflictingDeclarationPaths = claims.map(file => path.join(dir, file.name));
    if (claims.length > 1) return readonlyFailure(name, 'E_READONLY_AMBIGUOUS', { cause: 'ambiguous', definitionPath: definition.filePath, conflictingDeclarationPaths });
    if (claims.length && definition.filePath !== conflictingDeclarationPaths[0]) {
      return readonlyFailure(name, 'E_READONLY_SHADOWED', { cause: 'shadowed', definitionPath: definition.filePath, conflictingDeclarationPaths: [...conflictingDeclarationPaths, definition.filePath] });
    }
    if (claims.length) break;
  }
  const missingTools = !Array.isArray(definition.tools);
  const disallowedTools = missingTools ? [] : definition.tools.filter(tool => {
    if (READ_ONLY_TOOLS[tool] === true) return false;
    if (name === 'telemetry-scout' && READ_ONLY_MCP_DIAGNOSTIC_TOOLS[tool] === true) return false;
    return true;
  });
  const causes = [
    ...(missingTools ? ['missing-tools'] : []),
    ...(disallowedTools.length ? ['disallowed-tools'] : []),
    ...((definition.spawns?.length ?? 0) > 0 ? ['spawning'] : []),
    ...(definition.disable === true ? ['disabled'] : []),
  ];
  if (causes.length) return readonlyFailure(name, 'E_READONLY_WRITE_CAPABLE', { definitionPath: definition.filePath, disallowedTools, causes });
  return { resolved: true, name, tools: [...definition.tools] };
}

function exists(file, fs) {
  try { return fs.lstatSync(file); }
  catch (error) { if (error?.code === 'ENOENT' || error?.code === 'ENOTDIR') return null; throw error; }
}

export function loadLocalHarnessState(cwd, fs = { readFileSync, existsSync }) {
  const p = path.join(cwd, '.omp', 'state.json');
  try {
    const hasFile = typeof fs.existsSync === 'function' ? fs.existsSync(p) : exists(p, fs);
    if (hasFile) return JSON.parse(fs.readFileSync(p, 'utf8'));
  } catch {}
  return { tasks: {} };
}

export function saveLocalHarnessState(cwd, state, fs = { writeFileSync, mkdirSync }) {
  const p = path.join(cwd, '.omp', 'state.json');
  try {
    const dir = path.dirname(p);
    if (typeof fs.mkdirSync === 'function') fs.mkdirSync(dir, { recursive: true });
    if (typeof fs.writeFileSync === 'function') fs.writeFileSync(p, JSON.stringify(state, null, 2) + '\n', 'utf8');
  } catch {}
}

export function checkQuarantinedTasks(cwd, taskIds, fs = { readFileSync, existsSync }) {
  const state = loadLocalHarnessState(cwd, fs);
  for (const id of taskIds) {
    if (state.tasks?.[id]?.status === 'QUARANTINED') return id;
  }
  return null;
}

export function recordTaskAttempt(cwd: string, taskId: string, status: string = 'FAILED_REVIEW', errorCode: string | null = null, fs: Record<string, unknown> = { readFileSync, writeFileSync, mkdirSync, existsSync }) {
  const state = loadLocalHarnessState(cwd, fs);
  state.tasks = state.tasks ?? {};
  const entry = state.tasks[taskId] ?? { attempt_count: 0, status: 'pending' };
  entry.attempt_count = (entry.attempt_count || 0) + 1;
  entry.last_status = status;
  entry.last_error = errorCode;
  let quarantined = false;
  if (entry.attempt_count >= 2 && (status === 'FAILED_REVIEW' || status === 'BLOCKED')) {
    entry.status = 'QUARANTINED';
    entry.lock_released = true;
    quarantined = true;
  }
  state.tasks[taskId] = entry;
  saveLocalHarnessState(cwd, state, fs);
  return { quarantined, attempt_count: entry.attempt_count, status: entry.status };
}
// Canonicalize existing ancestors too: lexical containment does not stop links
// to a different feature/project or a new file below an escaping symlink.
function canonical(file, fs) {
  if (!path.isAbsolute(file)) throw Error('Canonical identity must be absolute');
  const full = path.resolve(file);
  if (exists(full, fs)) return fs.realpathSync(full);
  const parent = path.dirname(full);
  if (parent === full) throw Error('Unresolvable path');
  return path.join(canonical(parent, fs), path.basename(full));
}

function workspaceState(root, fs) {
  const plans = path.join(root, '.omp', 'plans');
  const stat = exists(plans, fs);
  if (!stat) return null;
  if (!stat.isDirectory() || stat.isSymbolicLink() || !inside(root, canonical(plans, fs))) throw Error('Invalid plan root');
  // Only the current plan directory, never parent/sibling repositories.
  const file = canonicalState(root);
  if (exists(file, fs)) return file;
  const entries = fs.readdirSync(plans, { withFileTypes: true });
  return entries.some(entry => entry.name.endsWith('.md') || entry.name.endsWith('.json')) ? plans : null;
}

function readStateSource(parsed, root, active, fs) {
  if (canonical(parsed.request.projectRoot, fs) !== root) throw Error('Foreign project');
  const source = path.resolve(root, parsed.statePath);
  const sourceReal = canonical(source, fs);
  if (!inside(root, source) || !inside(root, sourceReal)) throw Error('Foreign state');
  const sourceStat = fs.lstatSync(source);
  if (!sourceStat.isFile() || sourceStat.isSymbolicLink()) throw Error('Invalid state file');
  const current = canonicalState(root);
  const currentStat = source === current ? sourceStat : exists(current, fs);
  if (!currentStat?.isFile() || currentStat.isSymbolicLink()) throw Error('Invalid current state file');
  const currentReal = source === current ? sourceReal : canonical(current, fs);
  if (!inside(root, currentReal)) throw Error('Foreign current state');
  const currentText = fs.readFileSync(current, 'utf8');
  const currentState = JSON.parse(currentText);
  if (currentState?.schemaVersion === 2) {
    if (currentReal !== sourceReal) throw Error('Noncurrent state');
  } else if (currentState?.schemaVersion === 1) {
    const feature = parsed.request.feature;
    if (/^[A-Za-z0-9][A-Za-z0-9._-]*$/.exec(feature)?.[0] !== feature || source !== path.join(root, '.omp', 'plans', feature, 'execution-state.json')) throw Error('Invalid feature sidecar');
    const report = fs.evaluateState(current, fs);
    if (object(report) && report.status === 'invalid'
      && parsed.request.requests.some(entry => entry.effects.kind !== 'read' && EFFECTS.includes(entry.effects.kind))
      && Array.isArray(report.reasons) && report.reasons.length > 0
      && report.reasons.every(reason => object(reason) && reason.code === 'E_EVIDENCE_MISSING' && reason.location === 'gate.markerPath')) {
      throw new LegacyMarkerDenial('Missing legacy gate marker');
    }
    if (!object(report) || !['ready', 'blocked'].includes(report.status)) throw Error('Invalid legacy current state');
  } else {
    throw Error('Unknown current state');
  }
  if (active && active !== source) throw Error('Changed current state');
  const sourceText = sourceReal === currentReal ? currentText : fs.readFileSync(source, 'utf8');
  const sourceBytes = Buffer.from(sourceText, 'utf8');
  const state = sourceReal === currentReal ? currentState : JSON.parse(sourceText);
  if (state.schemaVersion !== 2 || canonical(state.projectRoot, fs) !== root || state.feature !== parsed.request.feature || state.revision !== parsed.request.revision) throw Error('Foreign state identity');
  const spec = canonical(path.resolve(path.dirname(source), state.spec.path), fs);
  if (!inside(root, spec)) throw Error('Foreign canonical spec');
  return { source, state, sourceBytes };
}

/** Preparation uses live canonical-source binding, not standalone compatibility. */
export function validateDispatchSource(payload, dependencies = {}) {
  const fs = { lstatSync, readFileSync, readdirSync, realpathSync, statSync, evaluateState, ...dependencies };
  try {
    const parsed = extractDispatchMetadata(payload);
    if (parsed.kind !== 'managed') throw Error('Invalid metadata');
    const root = fs.realpathSync(parsed.request.projectRoot);
    for (const ancestor of [path.join(root, '.omp'), path.join(root, '.omp', 'plans')]) {
      const stat = fs.lstatSync(ancestor);
      if (!stat.isDirectory() || stat.isSymbolicLink()) throw Error('Invalid canonical ancestor');
    }
    workspaceState(root, fs);
    return { ok: true, statePath: readStateSource(parsed, root, null, fs).source };
  } catch (error) {
    return { ok: false, code: error instanceof LegacyMarkerDenial ? 'E_PROJECT_GATE' : 'E_STATE_SOURCE' };
  }
}

function projectProtection(request, source, root, fs) {
  if (request.requests.every(entry => entry.effects.kind === 'read')) return true;
  const legacyPath = canonicalState(root);
  if (!exists(legacyPath, fs)) return true;
  if (fs.lstatSync(legacyPath).isSymbolicLink() || !inside(root, canonical(legacyPath, fs))) return false;
  const legacy = JSON.parse(fs.readFileSync(legacyPath, 'utf8'));
  if (legacy.schemaVersion !== 1) {
    if (legacy.schemaVersion === 2) {
      if (legacy.feature !== request.feature) return false;
      const protectedOtherRoots: string[] = [];
      const plansDir = path.join(root, '.omp', 'plans');
      if (exists(plansDir, fs)) {
        try {
          const entries = fs.readdirSync(plansDir, { withFileTypes: true });
          for (const entry of entries) {
            if (entry.isDirectory() && entry.name !== request.feature && !entry.name.startsWith('.')) {
              protectedOtherRoots.push(canonical(path.join(plansDir, entry.name), fs));
            }
          }
        } catch {}
      }
      const sddDir = path.join(root, '.superpowers', 'sdd');
      if (exists(sddDir, fs)) {
        try {
          const entries = fs.readdirSync(sddDir, { withFileTypes: true });
          for (const entry of entries) {
            if (entry.isDirectory() && entry.name !== request.feature && !entry.name.startsWith('.')) {
              protectedOtherRoots.push(canonical(path.join(sddDir, entry.name), fs));
            }
          }
        } catch {}
      }
      if (protectedOtherRoots.length > 0) {
        let envelopePaths: string[] = [];
        try {
          const state = JSON.parse(fs.readFileSync(source, 'utf8'));
          envelopePaths = state.envelope?.writePaths ?? [];
        } catch {}
        const candidates = [...envelopePaths, ...request.requests.flatMap(entry => entry.effects.paths)];
        if (candidates.some(file => protectedOtherRoots.some(pRoot => {
          const target = canonical(path.resolve(root, file), fs);
          return inside(pRoot, target) || inside(target, pRoot);
        }))) return false;
      }
    }
    return true;
  }
  const base = path.dirname(legacyPath);
  const protectedPaths = [legacyPath, ...['markerPath', 'extensionPath', 'loadEvidencePath'].map(key => path.resolve(base, legacy.gate[key]))];
  if (!inside(root, canonical(protectedPaths[1], fs))) return false;
  // State, receipts, marker and their containing evidence directory are never
  // borrowed by another feature, even when its envelope claims those paths.
  const protectedRoots = [...protectedPaths, path.dirname(protectedPaths[1])].map(file => canonical(file, fs));
  if (legacy.feature !== request.feature) {
    const state = JSON.parse(fs.readFileSync(source, 'utf8'));
    const candidates = [...state.envelope.writePaths, ...request.requests.flatMap(entry => entry.effects.paths)];
    if (candidates.some(file => protectedRoots.some(protectedRoot => {
      const target = canonical(path.resolve(root, file), fs);
      return inside(protectedRoot, target) || inside(target, protectedRoot);
    }))) return false;
  }
  // Require the project's historical marker/receipts independently of feature.
  // Unrelated feature task IDs do NOT need to appear on the old frontier.
  const report = fs.evaluateState(legacyPath);
  if (!object(report) || !Array.isArray(report.reasons)) return false;
  if (legacy.feature === request.feature) {
    return report.status === 'ready' && report.reasons.length === 0
      && Array.isArray(report.runnable_frontier)
      && request.requests.every(entry => report.runnable_frontier.includes(entry.taskId));
  }
  return (report.status === 'ready' && report.reasons.length === 0)
    || (report.status === 'blocked' && report.reasons.length === 1
      && object(report.reasons[0]) && report.reasons[0].code === 'E_NO_RUNNABLE_FRONTIER');
}

async function bounded(run, milliseconds) {
  const { promise, reject } = Promise.withResolvers();
  const timer = setTimeout(() => reject(Error('Gate timeout')), milliseconds);
  try { return await Promise.race([Promise.resolve().then(run), promise]); }
  finally { clearTimeout(timer); }
}

export function createToolCallGate(deps = {}) {
  if (typeof deps.evaluateDispatch !== 'function') throw Error('Shared evaluateDispatch is required');
  const fs = { existsSync, lstatSync, mkdirSync, readFileSync, readdirSync, realpathSync, statSync, unlinkSync, writeFileSync, evaluateState, ...deps };
  const log = deps.log ?? (entry => process.stderr.write(JSON.stringify({ service: 'workflow-policy', version: 'dispatch-shape-r5', ...entry }) + '\n'));
  let activeStatePath = null;
  let activeFeature = null;
  let activeRevision = null;
  let lifecycleEpoch = 0;
  const retryHistory = new Map();
  const activeMutators = new Map();
  const settledMutators = new Map();
  const toolCallGenerations = new Map();
  const activeReviews = new Map();
  const reviewPauseAdmissions = new WeakMap();
  function latestCandidateReview(candidate, registry) {
    const history = candidateReviewLineage(candidate, registry).history;
    const sequenced = history.filter(review => Number.isInteger(review.seq));
    if (!sequenced.length) return history.at(-1);
    return sequenced.reduce((latest, review) => review.seq > latest.seq ? review : latest);
  }
  function reviewDecisionIds(state) {
    return (Array.isArray(state.decisions) ? state.decisions : [])
      .filter(decision => decision?.kind === 'owner' && decision.status === 'resolved')
      .map(decision => decision.id).filter(text);
  }
  function makeReviewPausePayload(candidate, signal, classified, pauseId) {
    const questions = classified.ownerDecision.map((finding, index) => {
      const evidence = (Array.isArray(finding.evidence) ? finding.evidence
        : text(finding.path) && text(finding.anchor) ? [{ path: finding.path, anchor: finding.anchor }] : [])
        .filter(item => text(item?.path) && text(item?.anchor))
        .map(item => ({ path: item.path, anchor: item.anchor }));
      if (!text(finding.question) || !evidence.length) throw Error('unaskable owner finding');
      const options = Array.isArray(finding.options) ? finding.options.filter(text) : [];
      return {
        id: `Q${index + 1}`,
        question: finding.question,
        evidence,
        answerKind: options.length ? 'choice' : 'freeform',
        options,
      };
    });
    if (!questions.length) throw Error('empty owner question set');
    const excluded = classified.technicalRepair.map(finding =>
      String(finding.summary ?? finding.title ?? finding.id ?? 'Technical repair item').replace(/[\r\n]+/g, ' ').slice(0, 300));
    return {
      pauseId,
      kind: 'review_pause',
      lineageKey: signal.lineageKey,
      revision: candidate.revision,
      classifier: 'owner-decision',
      context: {
        nonPassStreak: signal.nonPassStreak,
        noProgressCount: signal.noProgressCount,
        carriedBlockingIds: signal.carriedOpenIds,
        newBlockingIds: signal.newBlockingIds,
      },
      questions,
      resume: 'reply referencing pauseId + question ids',
      excluded,
    };
  }
  async function evaluateDispatchIgnoringReviewPause(source, request, payload) {
    const state = source.state;
    if (!Object.hasOwn(state, 'reviewPause') && !Object.hasOwn(state, 'reviewPauseEvents')) {
      return fs.evaluateDispatch(source.source, request, { reviewPayload: payload });
    }
    const evaluationState = { ...state };
    delete evaluationState.reviewPause;
    delete evaluationState.reviewPauseEvents;
    const temporary = `${source.source}.${randomUUID()}.tmp`;
    fs.writeFileSync(temporary, JSON.stringify(evaluationState) + '\n', { flag: 'wx', mode: 0o600 });
    try {
      return await fs.evaluateDispatch(temporary, request, { reviewPayload: payload });
    } finally {
      fs.unlinkSync(temporary);
    }
  }
  function pauseEvent(state, registry, event) {
    for (const target of [state, registry]) {
      if (target.reviewPauseEvents !== undefined && !Array.isArray(target.reviewPauseEvents)) throw Error('invalid pause events');
      target.reviewPauseEvents = [...(target.reviewPauseEvents ?? []), event];
    }
  }
  function persistReviewPause(root, source, registryRecord, candidateSnapshot) {
    const decisions = path.join(root, '.omp', 'plans', source.state.feature, 'decisions.md');
    if (!inside(root, path.resolve(decisions))) throw Error('decision log containment');
    const result = withJournal(source.source, {
      feature: source.state.feature,
      registryPath: registryRecord.file,
      auditPath: decisions,
      expectedStateBytes: source.sourceBytes,
      expectedRegistryBytes: registryRecord.sourceBytes,
    }, snapshot => {
      const stateBefore = snapshot.stateObject;
      const registry = snapshot.registryObject;
      if (!stateBefore || registry?.schemaVersion !== 1 || !Array.isArray(registry.candidates)) throw Error('invalid review transaction snapshot');
      const candidate = registry.candidates.find(item => item?.id === candidateSnapshot.id);
      if (!candidate) return { noMutation: true, value: null };
      const signal = signalForReviewCandidate(candidate, registry);
      const latest = latestCandidateReview(candidate, registry);
      const classified = classifyReviewFindings(latest?.findings ?? [], reviewDecisionIds(stateBefore));
      if (!signal.stalled || classified.ownerDecision.length === 0) return { noMutation: true, value: null };
      if (registry.reviewPauseHistory !== undefined && !Array.isArray(registry.reviewPauseHistory)) throw Error('invalid pause history');
      const history = registry.reviewPauseHistory ?? [];
      const lineage = lineageReviewHistory(registry, signal.lineageKey);
      if (lineage.issues.length) throw Error('invalid review lineage');
      const priorLineages = new Set(lineage.aliasChain);
      const priorCount = history.filter(record => priorLineages.has(record?.lineageKey)).length;
      candidate.lineageKey = signal.lineageKey;
      candidate.nonPassStreak = signal.nonPassStreak;
      candidate.noProgressCount = signal.noProgressCount;
      candidate.lastRevision = candidate.revision;
      candidate.lastInputHashes = Array.isArray(candidate.inputs)
        ? candidate.inputs.map(input => ({ path: input.path, sha256: input.sha256 })) : [];
      if (priorCount >= 2) return {
        stateBytes: snapshot.state.bytes,
        registryBytes: `${JSON.stringify(registry)}\n`,
        value: reviewPauseResult({
          blocked: { kind: 'review_pause_cap', lineageKey: signal.lineageKey, pauseCount: priorCount, cap: 2 },
        }, 'The owner-question pause cap is exhausted for this review lineage.'),
      };

      const askedAt = new Date().toISOString();
      const pauseId = randomUUID();
      const payload = makeReviewPausePayload(candidate, signal, classified, pauseId);
      const packetDigest = sha(canonicalJson(payload));
      const pause = {
        pauseId, lineageKey: signal.lineageKey, revision: candidate.revision,
        questionIds: payload.questions.map(question => question.id), askedAt, packetDigest,
      };
      const event = {
        event: 'review_pause_opened', timestamp: askedAt, lineageKey: signal.lineageKey,
        revision: candidate.revision, pauseId, nonPassStreak: signal.nonPassStreak,
        noProgressCount: signal.noProgressCount,
      };
      const state = { ...stateBefore, reviewPause: pause };
      pauseEvent(state, registry, event);
      registry.reviewPauseHistory = [...history, { ...event, status: 'open' }];
      candidate.pauses = [...(Array.isArray(candidate.pauses) ? candidate.pauses : []), { ...event, status: 'open' }];
      return {
        stateBytes: `${JSON.stringify(state)}\n`,
        registryBytes: `${JSON.stringify(registry)}\n`,
        auditLine: `[AUTO-DECISION] review pause opened: ${pauseId} lineage=${signal.lineageKey} revision=${candidate.revision} nonPassStreak=${signal.nonPassStreak} noProgressCount=${signal.noProgressCount} txnId=${snapshot.txnId}\n`,
        value: reviewPauseResult({ pause: payload }, 'Owner input is required before this review lineage can continue.'),
      };
    }, fs);
    if (result?.ok === false) {
      const code = result.lockRole === 'registry' ? 'E_REVIEW_REGISTRY_CHANGED' : result.code;
      return deny(code ?? 'E_STALE');
    }
    return result?.ok === true && Object.hasOwn(result, 'value') ? result.value : result;
  }
  function acceptReviewPauseReceipt(event, pause, candidate, source, registryRecord, root) {
    const receipt = event?.reviewPauseReceipt;
    if (!object(receipt) || !keys(receipt, ['pauseId', 'questionId', 'answer', 'answerDigest'])
      || receipt.pauseId !== pause.pauseId || !pause.questionIds.includes(receipt.questionId)
      || !text(receipt.answer)
      || receipt.answerDigest !== sha(canonicalJson({
        pauseId: receipt.pauseId, questionId: receipt.questionId, answer: receipt.answer, packetDigest: pause.packetDigest,
      }))) return null;
    return { candidateId: candidate.id, pauseId: pause.pauseId, receipt, source, registryRecord, root };
  }
  function persistReviewPauseResume(admission) {
    const { source, registryRecord, pauseId, receipt, candidateId } = admission;
    const result = withJournal(source.source, {
      feature: source.state.feature,
      registryPath: registryRecord.file,
      expectedStateBytes: source.sourceBytes,
    }, snapshot => {
      const stateBefore = snapshot.stateObject;
      const registry = snapshot.registryObject;
      const pause = stateBefore?.reviewPause;
      if (!validReviewPause(pause) || pause.pauseId !== pauseId) throw Error('stale review pause');
      if (!registry || registry.schemaVersion !== 1 || !Array.isArray(registry.candidates)) throw Error('invalid review registry');
      const candidate = registry.candidates.find(item => item?.id === candidateId);
      if (!candidate) throw Error('stale review candidate');
      const resumedAt = new Date().toISOString();
      const eventFields = {
        timestamp: resumedAt, lineageKey: pause.lineageKey, revision: pause.revision, pauseId,
        nonPassStreak: candidate.nonPassStreak ?? 0, noProgressCount: candidate.noProgressCount ?? 0,
      };
      const state = { ...stateBefore };
      delete state.reviewPause;
      pauseEvent(state, registry, { event: 'review_pause_resumed', ...eventFields });
      pauseEvent(state, registry, { event: 'review_pause_cleared', ...eventFields });
      if (registry.reviewPauseHistory !== undefined && !Array.isArray(registry.reviewPauseHistory)) throw Error('invalid pause history');
      registry.reviewPauseHistory = (registry.reviewPauseHistory ?? []).map(record =>
        record.pauseId === pauseId ? { ...record, status: 'resumed', resumedAt } : record);
      if (Array.isArray(candidate.pauses)) {
        candidate.pauses = candidate.pauses.map(record =>
          record.pauseId === pauseId ? { ...record, status: 'resumed', resumedAt } : record);
      }
      if (registry.reviewPauseReceipts !== undefined && !Array.isArray(registry.reviewPauseReceipts)) throw Error('invalid pause receipts');
      registry.reviewPauseReceipts = [...(registry.reviewPauseReceipts ?? []), {
        pauseId, questionId: receipt.questionId, answerDigest: receipt.answerDigest, answeredAt: resumedAt,
      }];
      return {
        stateBytes: `${JSON.stringify(state)}\n`,
        registryBytes: `${JSON.stringify(registry)}\n`,
        value: true,
      };
    }, fs);
    if (result?.ok === false) {
      if (result.code === 'E_REVIEW_REGISTRY_CHANGED' || result.lockRole === 'registry') throw new ReviewRegistryChangedError();
      const error = new Error('Review pause transaction did not commit');
      error.code = result.code;
      throw error;
    }
  }
  function inspectReviewPause(root, entries, source, event) {
    const receipt = event?.reviewPauseReceipt;
    const state = source?.state ?? source;
    const pause = state?.reviewPause;
    if (pause && !validReviewPause(pause)) return { admission: null, denial: null };
    if (!receipt) return { admission: null, denial: reviewPauseDenial(root, entries, state, { fs }) };
    if (!pause) return {
      admission: null,
      denial: reviewPauseResult({}, 'No matching review pause is open for this answer receipt.'),
    };
    let record;
    let matches;
    try {
      record = loadReviewRegistry(root, fs);
      matches = matchingReviewCandidates(record.registry, entries).filter(candidate =>
        matchesReviewPause(candidate, record.registry, pause));
    } catch {
      return { admission: null, denial: null };
    }
    if (!matches.length) return {
      admission: null,
      denial: reviewPauseResult({ reviewPause: pause }, 'The answer receipt does not match this review lineage.'),
    };
    const admission = matches.length === 1 && entries.length === 1
      ? acceptReviewPauseReceipt(event, pause, matches[0], source, record, root) : null;
    return admission
      ? { admission, denial: null }
      : { admission: null, denial: reviewPauseResult({ reviewPause: pause }, 'The answer receipt is invalid or bound to a different pause question.') };
  }
  function maybeOpenReviewPause(root, source, entries, reviewIssues) {
    if (!reviewIssues.length || source.state.reviewPause) return null;
    try {
      const registryPath = path.join(root, '.omp', 'plans', 'review-sequence.json');
      const record = withStateLock(registryPath, {}, fs, () => loadReviewRegistry(root, fs));
      if (!record || record.ok === false) return null;
      const critics = entries.filter(entry => entry?.agent === 'critic');
      for (const candidate of matchingReviewCandidates(record.registry, critics)) {
        const signal = signalForReviewCandidate(candidate, record.registry);
        if (!signal.stalled) continue;
        const latest = latestCandidateReview(candidate, record.registry);
        const classified = classifyReviewFindings(latest?.findings ?? [], reviewDecisionIds(source.state));
        if (classified.ownerDecision.length) return persistReviewPause(root, source, record, candidate);
      }
    } catch {}
    return null;
  }
  const settledReviews = new Map();
  function finishReview(binding, active, succeeded = null) {
    if (binding.agent === 'code-reviewer' && succeeded !== null) recordReviewRun(binding.root, binding, succeeded ? 'succeeded' : 'failed', fs);
    if (text(binding.childId)) settledReviews.set(binding.childId, { ...binding, owner: active.owner,
      callId: active.callId, pendingTerminal: succeeded === null });
  }
  function reserveReviewBindings(bindings, event, ctx, payload) {
    if (!bindings.length) return null;
    if (!text(event.toolCallId)) return 'E_REVIEW';
    for (const binding of bindings) {
      if ([...activeReviews.values()].some(active => active.bindings.some(other =>
        binding.key === other.key || binding.paths.some(p => other.paths.includes(p))))) return 'E_REVIEW';
    }
    const items = payload.tasks ?? [payload];
    const reserved = bindings.map(binding => ({ ...binding, root: fs.realpathSync(ctx.cwd),
      payload: structuredClone(payload), owner: ctx?.agent?.id ?? null,
      invocationId: randomUUID(), index: items.findIndex(item => item.name === binding.name),
      childId: binding.childId ?? null, pendingStart: Boolean(binding.childId) }));
    for (const binding of reserved) {
      if (binding.agent === 'code-reviewer') recordReviewRun(binding.root, binding, 'pending', fs);
    }
    activeReviews.set(Symbol('review-dispatch'), { callId: event.toolCallId, owner: ctx?.agent?.id ?? null, bindings: reserved });
    return null;
  }
  function reserveReviews(event, ctx, parsed, resumeAdmission = null) {
    const bindings = [];
    const entries = parsed.kind === 'managed' ? parsed.request.requests : (event.input.tasks ?? [event.input]);
    const context = {
      payload: event.input, bindings,
      ...(resumeAdmission ? { reviewPauseResume: { pauseId: resumeAdmission.pauseId, candidateId: resumeAdmission.candidateId } } : {}),
    };
    const errors = reviewSequenceIssues(ctx.cwd, entries, fs, context);
    return errors.length ? 'E_REVIEW' : reserveReviewBindings(bindings, event, ctx, event.input);
  }
  function resumeReviews(target, event, ctx) {
    const active = [...activeReviews.values()].flatMap(review => review.bindings);
    if (active.some(binding => target === 'all' || binding.childId === target)) return 'E_REVIEW';
    const entries = [...settledReviews.entries()].filter(([childId]) => target === 'all' || childId === target);
    const bindings = [];
    for (const [childId, previous] of entries) {
      if (previous.owner !== (ctx?.agent?.id ?? null) || previous.root !== fs.realpathSync(ctx.cwd)) return 'E_REVIEW';
      const parsed = extractDispatchMetadata(previous.payload);
      const requests = parsed.kind === 'managed' ? parsed.request.requests : (previous.payload.tasks ?? [previous.payload]);
      const current = [];
      if (reviewSequenceIssues(ctx.cwd, requests, fs, { payload: previous.payload, bindings: current }).length) return 'E_REVIEW';
      const binding = current.find(binding => binding.name === previous.name && binding.agent === previous.agent);
      if (!binding || binding.fingerprint !== previous.fingerprint || binding.key !== previous.key) return 'E_REVIEW';
      bindings.push({ ...binding, childId, payload: previous.payload });
    }
    if (bindings.some((a, i) => bindings.slice(i + 1).some(b => a.key === b.key || scopesOverlap(a.paths, b.paths)))) return 'E_REVIEW';
    // Validate the whole broadcast before starting any invocation.
    for (const binding of bindings) {
      const error = reserveReviewBindings([binding], event, ctx, binding.payload);
      if (error) return error;
    }
    return null;
  }
  const clearLifecycleHistory = () => {
    settledMutators.clear();
    toolCallGenerations.clear();
    lifecycleEpoch++;
  };
  const rememberSettledMutators = (scopes, defaultOwner = null) => {
    for (const scope of scopes) {
      if (!text(scope.childId)) continue;
      const existing = settledMutators.get(scope.childId);
      const paths = [...new Set([...(existing?.paths ?? []), ...scope.paths])];
      const owner = scope.owner ?? defaultOwner ?? existing?.owner ?? null;
      const generation = Math.max(existing?.generation ?? 0, scope.generation ?? 0);
      settledMutators.delete(scope.childId);
      settledMutators.set(scope.childId, { paths, owner, generation });
    }
  };
  const conflictsWithActiveReservation = p => [...activeMutators.values()].some(reservation =>
    reservation.scopes.some(scope => scope.paths.some(q => inside(p, q) || inside(q, p))));
  const releaseObservedChildren = children => {
    if (!Array.isArray(children)) return;
    for (const [token, active] of activeReviews) {
      active.bindings = active.bindings.filter(binding => {
        const child = binding.childId && children.find(child => child?.id === binding.childId);
        if (child?.status === 'running') binding.pendingStart = false;
        const settled = child && (active.owner === null || child.parentId === active.owner)
          && (child.status === 'aborted' || (!binding.pendingStart && ['idle', 'parked'].includes(child.status)));
        if (settled) finishReview(binding, active, child.status === 'aborted' ? false : null);
        return !settled;
      });
      if (!active.bindings.length) activeReviews.delete(token);
    }
    for (const [token, reservation] of activeMutators) {
      const released = [];
      reservation.scopes = reservation.scopes.filter(scope => {
        const child = Boolean(scope.childId) && children.find(c => c?.id === scope.childId);
        if (child && child.status === 'running') {
          scope.pendingStart = false;
        }
        const effectiveOwner = scope.owner ?? reservation.owner;
        const matchesOwner = Boolean(child) && (effectiveOwner === null || child.parentId === effectiveOwner);
        if (matchesOwner && child.status === 'aborted') {
          released.push(scope);
          return false;
        }
        if (scope.pendingStart) {
          return true;
        }
        const settled = matchesOwner && ['idle', 'parked'].includes(child.status);
        if (settled) released.push(scope);
        return !settled;
      });
      if (reservation.lifecycleEpoch === lifecycleEpoch) rememberSettledMutators(released, reservation.owner);
      if (!reservation.scopes.length) activeMutators.delete(token);
    }
  };
  const handleToolResult = (event, ctx) => {
    if (!['task', 'write'].includes(event?.toolName)) return;
    for (const binding of settledReviews.values()) {
      if (!binding.pendingTerminal || event.toolCallId !== binding.callId || binding.owner !== (ctx?.agent?.id ?? null)) continue;
      const result = event.details?.results?.find(item => item?.id === binding.childId);
      if (result && (Number.isFinite(result.exitCode) || result.aborted === true)) {
        finishReview(binding, { callId: binding.callId, owner: binding.owner },
          result.exitCode === 0 && result.aborted !== true && event.isError !== true);
      }
    }
    for (const [token, active] of activeReviews) {
      const matches = text(event.toolCallId) && event.toolCallId === active.callId && active.owner === (ctx?.agent?.id ?? null);
      active.bindings = active.bindings.filter(binding => {
        const results = Array.isArray(event.details?.results) ? event.details.results : [];
        const progress = Array.isArray(event.details?.progress) ? event.details.progress : [];
        const result = matches ? results.find(item => binding.childId ? item?.id === binding.childId : item?.index === binding.index) : null;
        const identity = result ?? (matches ? progress.find(item => item?.index === binding.index) : null);
        if (text(identity?.id)) binding.childId = identity.id;
        if (identity) binding.pendingStart = false;
        const terminal = result && (Number.isFinite(result.exitCode) || result.aborted === true);
        if (terminal || (matches && event.isError === true)) finishReview(binding, active,
          terminal && result.exitCode === 0 && result.aborted !== true && event.isError !== true);
        return !terminal && !(matches && event.isError === true);
      });
      if (!active.bindings.length) activeReviews.delete(token);
    }
    if (event.toolName === 'write') {
      if (event.isError === true && text(event.toolCallId)) {
        for (const [token, reservation] of activeMutators) {
          if (reservation.callId === event.toolCallId && reservation.owner === (ctx?.agent?.id ?? null)) activeMutators.delete(token);
        }
      }
      return;
    }
    for (const [token, reservation] of activeMutators) {
      const results = Array.isArray(event.details?.results) ? event.details.results : [];
      const progress = Array.isArray(event.details?.progress) ? event.details.progress : [];
      const matchesCall = text(event.toolCallId) && reservation.callId === event.toolCallId && reservation.owner === (ctx?.agent?.id ?? null);
      const settled = [];
      reservation.scopes = reservation.scopes.filter(scope => {
        const result = matchesCall
          ? results.find(item => item?.index === scope.index)
          : (scope.childId ? results.find(item => item?.id === scope.childId) : null);
        const identity = matchesCall
          ? (result ?? progress.find(item => item?.index === scope.index))
          : null;
        const currentGeneration = scope.generation ?? (scope.childId ? settledMutators.get(scope.childId)?.generation : 0) ?? 0;
        const resultGeneration = Number.isSafeInteger(result?.generation) ? result.generation
          : Number.isSafeInteger(event.generation) ? event.generation
            : text(event.toolCallId) && reservation.lifecycleEpoch === lifecycleEpoch
              ? toolCallGenerations.get(event.toolCallId)?.get(scope.childId) : undefined;
        const terminal = Boolean(result && (Number.isFinite(result.exitCode) || result.aborted === true));
        if (scope.generation > 0 && terminal
          && (!text(event.toolCallId) || resultGeneration !== scope.generation)) return true;
        if (!matchesCall && resultGeneration !== undefined && resultGeneration !== currentGeneration) return true;
        if (text(identity?.id)) {
          scope.childId = identity.id;
          scope.generation ??= settledMutators.get(identity.id)?.generation ?? 0;
        }
        const childIdentityMatches = text(scope.childId)
          && (result?.id === scope.childId || progress.some(item => item?.id === scope.childId));
        if (text(event.toolCallId) && childIdentityMatches && reservation.lifecycleEpoch === lifecycleEpoch) {
          const generations = toolCallGenerations.get(event.toolCallId) ?? new Map();
          generations.set(scope.childId, scope.generation ?? currentGeneration);
          toolCallGenerations.set(event.toolCallId, generations);
        }
        if (result || progress.some(item => item?.id === scope.childId || (matchesCall && item?.index === scope.index))) {
          scope.pendingStart = false;
        }
        if (terminal) settled.push(scope);
        return !terminal;
      });
      if (reservation.lifecycleEpoch === lifecycleEpoch) rememberSettledMutators(settled, reservation.owner);
      if (!reservation.scopes.length) activeMutators.delete(token);
    }
    try { releaseObservedChildren(deps.getAgentRegistry?.()); } catch {}
  };
  const deny = (code, requestShapeOnly = false, profileFailure = null, guidanceOverride = null, denialDetail = undefined) => {
    const safe = Object.hasOwn(CODES, code) ? code : 'E_DISPATCH_MISMATCH';
    const diagnostic = profileFailure ? readonlyMetadata(profileFailure) : null;
    const profileFields = diagnostic
      ? diagnostic.cause === 'frontier' ? { diagnostic } : { readonlyProfile: diagnostic }
      : {};
    const detailFields = denialDetail === undefined ? {} : { detail: denialDetail };
    log({ event: 'dispatch_denied', code: safe, ...profileFields, ...detailFields });
    const guidance = guidanceOverride ?? (diagnostic?.cause === 'frontier'
      ? 'Managed read-only dispatch is limited to approved runnable research reads.'
      : DENIAL_GUIDANCE[safe] ?? 'Inspect the canonical state and exact native workflowDispatch request before taking another action.');
    const advice = diagnostic ? ` ${profileAdvice({ ...diagnostic, code: diagnostic.resolverCode })}` : '';
    const result = {
      block: true,
      reason: `workflow dispatch gate: ${safe}: ${guidance}${advice}${denialDetail === undefined ? '' : ` ${denialDetail}`}`,
      ...(diagnostic ? { diagnostic } : {}),
      ...detailFields,
    };
    denialCodes.set(result, { code: safe, requestShapeOnly });
    return result;
  };
  log({ event: 'gate_registered' });

  async function decide(event, ctx, checkRetry, consumeCorrection, reserve, reserveResumed) {
    if (!object(event) || !['task', 'write', 'edit'].includes(event.toolName)) return undefined;
    if (!text(ctx?.cwd) || !path.isAbsolute(ctx.cwd)) return deny('E_RUNTIME_INCOMPATIBLE');
    const root = fs.realpathSync(ctx.cwd);
    const managed = workspaceState(root, fs) !== null || activeStatePath !== null;
    if (managed && ctx?.agent?.kind === 'main' && (event.toolName === 'edit' || event.toolName === 'write')) {
      const inputPath = event.input?.path;
      if (typeof inputPath === 'string' && inputPath.length > 0 && !/^[a-z][a-z0-9+.-]*:\/\//i.test(inputPath)) {
        const targetPath = path.normalize(path.isAbsolute(inputPath) ? inputPath : path.join(root, inputPath));
        const strictDenial = targetPath === canonicalState(root)
          || targetPath === path.join(root, '.omp', 'state', 'interrupt.lock')
          || targetPath === path.join(root, '.omp', 'state.json');
        if (strictDenial) return deny('E_COORDINATOR_EDIT_FORBIDDEN');
        const reviewObservationPath = path.join(root, '.omp', 'plans', 'review-sequence.json');
        let writableReviewObservation = false;
        if (targetPath === reviewObservationPath) {
          try { writableReviewObservation = canonical(targetPath, fs) === reviewObservationPath; } catch {}
        }
        const isGlobalRoot = root === '/home/opencode/.omp/agent';
        const cutoverArtifacts = '/home/opencode/projects/default/mcp-bridge/.omp/artifacts/review-schedule-cutover';
        let writableCutoverArtifact = false;
        if (isGlobalRoot && inside(cutoverArtifacts, targetPath) && targetPath !== cutoverArtifacts) {
          try {
            writableCutoverArtifact = fs.realpathSync(cutoverArtifacts) === cutoverArtifacts
              && inside(cutoverArtifacts, canonical(targetPath, fs));
          } catch {}
        }
        const allowed = targetPath === '/home/opencode/.omp/agent/workflow-policy/noninteractive-decisions.md'
          || writableReviewObservation
          || writableCutoverArtifact
          || (isGlobalRoot && inside(path.join(root, 'workflow-policy'), targetPath))
          || [
            path.join(root, '.scratch'),
            '/tmp',
            path.join(root, '.omp', 'artifacts'),
            path.join(root, '.omp', 'state'),
          ].some(prefix => inside(prefix, targetPath));
        if (!allowed) return deny('E_COORDINATOR_EDIT_FORBIDDEN');
      }
    }
    if (event.toolName === 'write') {
      const isProgress = typeof event.input?.path === 'string' && event.input.path.startsWith('agent://') && /[?&]progress(?:[&=#]|$)/.test(event.input.path);
      const target = !isProgress && typeof event.input?.path === 'string' ? event.input.path.match(/^agent:\/\/([^/?#]+)(?:[/?#]|$)/)?.[1] : null;
      const resumedEntries = target === 'all'
        ? [...settledMutators.entries()].map(([childId, entry]) => ({ childId, paths: entry.paths, owner: entry.owner, generation: entry.generation ?? 0 }))
        : target && settledMutators.has(target)
          ? [{ childId: target, paths: settledMutators.get(target).paths, owner: settledMutators.get(target).owner, generation: settledMutators.get(target).generation ?? 0 }]
          : [];
      for (let i = 0; i < resumedEntries.length; i++) {
        for (let j = i + 1; j < resumedEntries.length; j++) {
          if (scopesOverlap(resumedEntries[i].paths, resumedEntries[j].paths)) {
            return deny('E_EFFECT_OUTSIDE_ENVELOPE', false, null, RESUME_CONFLICT_GUIDANCE);
          }
        }
      }
      const resumedPaths = resumedEntries.flatMap(entry => entry.paths);
      if (resumedPaths.some(conflictsWithActiveReservation)) return deny('E_EFFECT_OUTSIDE_ENVELOPE', false, null, RESUME_CONFLICT_GUIDANCE);
      if (target) {
        const reviewError = resumeReviews(target, event, ctx);
        if (reviewError) return deny(reviewError);
      }
      if (resumedEntries.length > 0) {
        reserveResumed(resumedEntries, event, ctx);
      }
    }
    if (event.toolName === 'edit') return undefined;
    const device = typeof event.input?.path === 'string' ? event.input.path.match(/^xd:\/\/([^/?#]+)(?:[/?#]|$)/)?.[1] : null;
    if (event.toolName === 'write' && ALTERNATES[device] !== true) return undefined;
    if (event.toolName === 'write') return managed ? deny('E_ALTERNATE_ROUTE') : undefined;
    const parsed = extractDispatchMetadata(event.input);
    if (parsed.kind === 'invalid') return deny(parsed.code, true);
    if (parsed.kind === 'unmanaged') {
      const unboundItems = Array.isArray(event.input?.tasks) ? event.input.tasks : [event.input];
      try {
        const stateFile = canonicalState(root);
        if (exists(stateFile, fs)) {
          const stateStat = fs.lstatSync(stateFile);
          if (!stateStat.isSymbolicLink() && stateStat.isFile()) {
            const state = JSON.parse(fs.readFileSync(stateFile, 'utf8'));
            const pauseCheck = inspectReviewPause(root, unboundItems, state, event);
            if (pauseCheck.denial) return pauseCheck.denial;
            if (pauseCheck.admission) return reviewPauseResult({ reviewPause: state.reviewPause }, 'A bound resume receipt requires one managed review dispatch.');
          }
        }
      } catch {}
      const reviewContext = { payload: event.input, residualRegister: undefined };
      const reviewIssues = reviewSequenceIssues(root, unboundItems, fs, reviewContext);
      if (reviewIssues.length) {
        const issue = reviewIssues[0];
        return issue?.code === 'E_REVIEW_TARGET'
          ? deny(issue.code, false, null, null, issue.detail)
          : deny('E_REVIEW', false, null, null, issue?.detail);
      }
      if (reviewContext.residualRegister) {
        event.residualRegister = reviewContext.residualRegister;
        log({ event: 'dispatch_review_residual', residualRegister: reviewContext.residualRegister });
      }
      if (!managed) return undefined;
      // Preparation must be possible before a planner has authored a task
      // ledger. Trust only actual installed read-only definitions, never the
      // caller's role name, requested tools, or a claimed read effect.
      const { lintFailure, items } = lintUnmanagedPayload(event.input);
      if (lintFailure) return deny(lintFailure.code, true);
      if (!items) return deny('E_DISPATCH_METADATA', true);
      const stateFile = canonicalState(root);
      if (exists(stateFile, fs) && items.every(item => item.agent === 'git-master')) {
        try {
          const report = fs.evaluateState(stateFile);
          if (deliveryComplete(report)) {
            log({ event: 'dispatch_allowed_git_delivery' });
            return undefined;
          }
        } catch {}
      }
      for (const item of items) {
        const role = await resolveReadOnlyAgent(item.agent, { cwd: root, discoverAgents: cwd => deps.discoverAgents?.(cwd, ctx), deps: fs });
        if (!role.resolved) return deny('E_READONLY_NOT_EXEMPT', false, role);
      }
      log({ event: 'dispatch_allowed_readonly_preparation' });
      return undefined;
    }
    const validation = validateDispatchPayload(event.input);
    if (!validation.ok) return deny(validation.code, true);
    const items = nativeItems(event.input, parsed.shape, false);
    if (!items || !nativeMapping(parsed, items)) return deny('E_DISPATCH_MISMATCH', true);
    if (!event?.reviewPauseReceipt) consumeCorrection();
    let source;
    try { source = readStateSource(parsed, root, activeStatePath, fs); }
    catch (error) { return deny(error instanceof LegacyMarkerDenial ? 'E_PROJECT_GATE' : 'E_STATE_SOURCE'); }
    // Retain child provenance only within one validated feature revision.
    if (activeStatePath !== null
      && (activeStatePath !== source.source || activeFeature !== source.state.feature || activeRevision !== source.state.revision)) {
      clearLifecycleHistory();
    }
    activeStatePath = source.source;
    activeFeature = source.state.feature;
    activeRevision = source.state.revision;
    const admissionEpoch = lifecycleEpoch;
    const pauseCheck = inspectReviewPause(root, parsed.request.requests, source, event);
    if (pauseCheck.denial) return pauseCheck.denial;
    const resumeAdmission = pauseCheck.admission;
    if (resumeAdmission) reviewPauseAdmissions.set(event, resumeAdmission);
    const retryDenial = resumeAdmission ? undefined : checkRetry(source.state, parsed.request);
    if (retryDenial) return retryDenial;
    let report = await evaluateDispatchIgnoringReviewPause(source, parsed.request, event.input);
    if (!object(report) || typeof report.allowed !== 'boolean' || typeof report.authority_ready !== 'boolean') return deny('E_INTERNAL');
    let firstTargetIssue;
    if (!report.allowed) {
      const reasons = [...(report.reasons ?? []), ...(report.entries ?? []).flatMap(entry => entry.reasons ?? [])];
      firstTargetIssue = reasons.find(issue => issue?.code === 'E_REVIEW_TARGET');
      const requestShapeOnly = reasons.length > 0
        && reasons.every(reason => isRequestShapeFailure(reason.code, reason.location));
      const reviewOnly = reasons.length > 0 && reasons.every(issue => issue?.code === 'E_REVIEW');
      if (reviewOnly && resumeAdmission) report = { ...report, allowed: true };
      else if (reviewOnly) {
        const reviewIssues = reviewSequenceIssues(root, parsed.request.requests, fs, {
          revision: source.state.revision, payload: event.input,
          ...(source.state.spec ? { spec: { ...source.state.spec, path: path.resolve(path.dirname(source.source), source.state.spec.path) } } : {}),
        });
        firstTargetIssue ??= reviewIssues.find(issue => issue?.code === 'E_REVIEW_TARGET');
        if (!firstTargetIssue) {
          const pauseResult = maybeOpenReviewPause(root, source, parsed.request.requests, reviewIssues);
          if (pauseResult) return pauseResult;
        }
      }
      if (!report.allowed) {
        const issue = firstTargetIssue ?? reasons[0];
        return deny(issue?.code, requestShapeOnly, null, null,
          ['E_REVIEW_TARGET', 'E_REVIEW'].includes(issue?.code) ? issue.detail : undefined);
      }
    }
    const resume = reviewPauseAdmissions.get(event);
    const reviewContext = {
      revision: source.state.revision, payload: event.input, residualRegister: undefined,
      ...(resume ? { reviewPauseResume: { pauseId: resume.pauseId, candidateId: resume.candidateId } } : {}),
      ...(source.state.spec ? { spec: { ...source.state.spec, path: path.resolve(path.dirname(source.source), source.state.spec.path) } } : {}),
    };
    const reviewIssues = reviewSequenceIssues(root, parsed.request.requests, fs, reviewContext);
    if (reviewIssues.length) {
      const pauseResult = resume ? null : maybeOpenReviewPause(root, source, parsed.request.requests, reviewIssues);
      if (pauseResult) return pauseResult;
      const issue = reviewIssues[0];
      return deny(issue?.code ?? 'E_REVIEW', false, null, null, issue?.detail);
    }
    if (reviewContext.residualRegister) {
      event.residualRegister = reviewContext.residualRegister;
      log({ event: 'dispatch_review_residual', residualRegister: reviewContext.residualRegister });
    }
    const scopeError = dispatchScopeError(parsed.request, fs);
    if (scopeError) return deny(scopeError);
    if (!report.authority_ready) {
      const researchFrontier = new Set(report.task_detail?.filter(task => task.runnable === true && task.stage === 'research').map(task => task.id) ?? []);
      if (!parsed.request.requests.every(entry => entry.effects.kind === 'read' && researchFrontier.has(entry.taskId))) return deny('E_READONLY_NOT_EXEMPT', false, { cause: 'frontier' });
      for (const entry of parsed.request.requests) {
        const role = await resolveReadOnlyAgent(entry.agent, { cwd: root, discoverAgents: cwd => deps.discoverAgents?.(cwd, ctx), deps: fs });
        if (!role.resolved) return deny('E_READONLY_NOT_EXEMPT', false, role);
      }
      log({ event: 'dispatch_allowed_readonly' });
      return undefined;
    }
    const roles = await deps.discoverAgents?.(root, ctx);
    if (parsed.request.requests.some(entry => roles?.agents?.filter(role => role.name === entry.agent && role.disable !== true).length !== 1)) return deny('E_ROLE_NOT_PERMITTED');
    if (!projectProtection(parsed.request, source.source, root, fs)) return deny('E_PROJECT_GATE');
    const reservationError = reserve(parsed, event, ctx, admissionEpoch);
    if (reservationError) return deny(reservationError);
    log({ event: 'dispatch_allowed' });
    return undefined;
  }

  async function checkReadOnlyProfiles(cwd, ctx) {
    const unavailable = { checked: false, code: 'E_READONLY_UNRESOLVED', incompatible: [] };
    if (!text(cwd) || !path.isAbsolute(cwd) || typeof deps.discoverAgents !== 'function') return unavailable;
    try {
      const root = fs.realpathSync(cwd);
      const discovery = await bounded(() => deps.discoverAgents(root, ctx), deps.profileCheckTimeoutMs ?? Math.min(deps.timeoutMs ?? 5000, 1000));
      if (!object(discovery) || !Array.isArray(discovery.agents)) return unavailable;
      const incompatible = [];
      for (const name of READONLY_CANDIDATE_AGENTS) {
        const profile = await resolveReadOnlyAgent(name, { cwd: root, discoverAgents: async () => discovery, deps: fs });
        if (!profile.resolved) incompatible.push(readonlyMetadata(profile));
      }
      return { checked: true, incompatible };
    } catch {
      return unavailable;
    }
  }

  return {
    clearLifecycleHistory,
    checkReadOnlyProfiles,
    handleToolResult,
    onAgentRegistryChange: releaseObservedChildren,
    async handleToolCall(event, ctx) {
      const invocation = { open: true, token: Symbol('native-dispatch') };
      const reserve = (parsed, nativeEvent, context, admissionEpoch) => {
        if (!invocation.open) return 'E_INTERNAL';
        if (lifecycleEpoch !== admissionEpoch) return 'E_STATE_SOURCE';
        let children = null;
        try {
          children = deps.getAgentRegistry?.();
          releaseObservedChildren(children);
        } catch {}
        const activePaths = [...activeMutators.values()].flatMap(value => value.scopes.flatMap(scope => scope.paths));
        const checked = dispatchScopes(parsed.request, fs, activePaths);
        if (checked.error) return checked.error;
        if (checked.scopes.length) {
          const items = nativeItems(nativeEvent.input, parsed.shape);
          activeMutators.set(invocation.token, {
            owner: context?.agent?.id ?? null,
            callId: nativeEvent.toolCallId,
            lifecycleEpoch: admissionEpoch,
            scopes: checked.scopes.map(scope => ({ ...scope, childId: null, generation: 0,
              index: items.findIndex(item => item.name === scope.name) })),
          });
        }
        return null;
      };
      const reserveResumed = (entries, nativeEvent, context) => {
        if (!invocation.open) return;
        let currentRegistry = null;
        try {
          currentRegistry = deps.getAgentRegistry?.();
          releaseObservedChildren(currentRegistry);
        } catch {}
        const registryList = Array.isArray(currentRegistry) ? currentRegistry : [];
        const scopes = entries.map(entry => {
          const child = registryList.find(c => c?.id === entry.childId);
          const isRunning = Boolean(child && child.status === 'running');
          const generation = (entry.generation ?? 0) + 1;
          settledMutators.set(entry.childId, { ...entry, generation });
          return {
            name: entry.childId,
            childId: entry.childId,
            paths: entry.paths,
            owner: entry.owner ?? null,
            generation,
            pendingStart: !isRunning,
            index: -1,
          };
        });
        activeMutators.set(invocation.token, {
          owner: context?.agent?.id ?? null,
          callId: nativeEvent?.toolCallId ?? null,
          lifecycleEpoch,
          scopes,
        });
      };
      try {
        const toolName = event?.toolName;
        const dispatchDevice = toolName === 'write' && typeof event?.input?.path === 'string'
          ? event.input.path.match(/^xd:\/\/([^/?#]+)(?:[/?#]|$)/)?.[1]
          : null;
        const dispatchTool = toolName === 'task' || ALTERNATES[dispatchDevice] === true;
        if (dispatchTool && text(ctx?.cwd) && path.isAbsolute(ctx.cwd)
          && exists(path.join(ctx.cwd, '.omp', 'state', 'interrupt.lock'), fs)) {
          invocation.open = false;
          return deny('E_USER_INTERRUPT_ACTIVE');
        }
        if (typeof toolName === 'string' && toolName.toLowerCase().includes('graphify')) {
          const access = verifyToolAccess(toolName, ctx?.cwd, deps);
          if (!access.ok) {
            invocation.open = false;
            const code = access.code ?? 'E_GIT_STATUS';
            const guidance = DENIAL_GUIDANCE[code] ?? 'Graphify access could not be verified; use narrow search instead.';
            return deny(code, false, null, guidance);
          }
        }
        const key = event?.toolName === 'task' ? retryKey(ctx, fs, retryScope(event?.input)) : null;
        const sourceKey = event?.toolName === 'task' ? retryKey(ctx, fs, 'state-source') : null;
        const identity = retryIdentity(event?.input);
        let previous = key ? retryHistory.get(key) : null;
        const previousSourceFailure = sourceKey ? retryHistory.get(sourceKey) : null;
        let amendment = null;
        let correctionUsedForCall = false;
        const consumeCorrection = () => {
          const correctionEligible = Boolean(previous
            && previous.requestShapeOnlyFailures >= MAX_DENIED_DISPATCH_ATTEMPTS
            && previous.correctionUsed !== true
            && identity !== previous.identity);
          if (!correctionEligible) return;
          previous = { ...previous, correctionUsed: true };
          retryHistory.set(key, previous);
          correctionUsedForCall = true;
        };
        const reviewFingerprint = () => {
          if (event?.toolName !== 'task') return null;
          const metadata = extractDispatchMetadata(event.input);
          const entries = metadata.kind === 'managed' ? metadata.request.requests : (event.input?.tasks ?? [event.input]);
          const bindings = [];
          const errors = reviewSequenceIssues(ctx?.cwd, entries, fs, { payload: event.input, bindings });
          const critics = bindings.filter(binding => binding.agent === 'critic');
          return errors.length || !critics.length ? null
            : sha(canonicalJson(critics.map(({ key, sourceSha256 }) => ({ key, sourceSha256 }))));
        };
        const checkRetry = (state = null, request = null) => {
          if (previous?.code === 'E_REVIEW') {
            const fingerprint = reviewFingerprint();
            if (fingerprint !== null && fingerprint !== previous.reviewFingerprint) {
              retryHistory.delete(key);
              previous = null;
            }
          }
          if (state && request && previous
            && ['E_TASK_BLOCKED', 'E_RETRY_NOT_AUTHORIZED'].includes(previous.code)
            && allTargetsPendingAndUnblocked(state, request)) {
            retryHistory.delete(key);
            previous = null;
          }
          amendment = state ? ownerAmendmentToken(state, request.requests.map(entry => entry.taskId)) : null;
          const reopened = ownerReopens(previous, amendment);
          const reopenedSource = ownerReopens(previousSourceFailure, amendment);
          if ((!reopened && !correctionUsedForCall && (previous?.attempts >= MAX_DENIED_DISPATCH_ATTEMPTS
            || (RETRYABLE_CORRECTIONS[previous?.code] === true && previous.identity === identity)))
            || (!reopenedSource && previousSourceFailure?.attempts >= MAX_DENIED_DISPATCH_ATTEMPTS)
          ) return deny('E_RETRY_NOT_AUTHORIZED');
          if (reopened) retryHistory.delete(key);
          if (reopenedSource) retryHistory.delete(sourceKey);
          return undefined;
        };
        const parsed = extractDispatchMetadata(event?.input);
        if (parsed.kind === 'managed' && Array.isArray(parsed.requests)) {
          const taskIds = parsed.requests.map(r => r.taskId).filter(Boolean);
          const quarantinedId = checkQuarantinedTasks(ctx?.cwd ?? process.cwd(), taskIds, fs);
          if (quarantinedId) {
            return {
              block: true,
              reason: `workflow dispatch gate: E_TASK_BLOCKED: Task "${quarantinedId}" is QUARANTINED after breaching the 2-strike failure limit. Locks released; do not retry this quarantined branch. Proceed to unlock independent graph branches or halt for human review.`,
            };
          }
        }
        if (parsed.kind !== 'managed' || !nativeMapping(parsed, nativeItems(event?.input, parsed.shape, false))) {
          const retryDenial = checkRetry();
          if (retryDenial) return retryDenial;
        }
        // Read current identity once, then check owner recovery before the old
        // retry ceiling. All authority/effect/role/project checks still execute.
        let result = await bounded(() => decide(event, ctx, checkRetry, consumeCorrection, reserve, reserveResumed), deps.timeoutMs ?? 5000);
        invocation.open = false;
        if (result !== undefined) reviewPauseAdmissions.delete(event);
        if (result === undefined && event?.toolName === 'task') {
          const resumeAdmission = reviewPauseAdmissions.get(event);
          const reviewError = reserveReviews(event, ctx, parsed, resumeAdmission);
          if (reviewError) {
            reviewPauseAdmissions.delete(event);
            activeMutators.delete(invocation.token);
            result = deny(reviewError);
          } else if (resumeAdmission) {
            try {
              persistReviewPauseResume(resumeAdmission);
            } catch (error) {
              activeMutators.delete(invocation.token);
              for (const [token, active] of activeReviews) {
                if (active.callId === event.toolCallId && active.owner === (ctx?.agent?.id ?? null)) activeReviews.delete(token);
              }
              const pause = resumeAdmission?.source?.state?.reviewPause;
              result = error instanceof ReviewRegistryChangedError ? deny('E_REVIEW_REGISTRY_CHANGED')
                : reviewPauseResult(pause ? { reviewPause: pause } : {},
                  'The owner answer could not be durably recorded; the review pause remains active.');
            }
            reviewPauseAdmissions.delete(event);
          }
        }
        if (result === undefined) {
          if (extractDispatchMetadata(event?.input).kind === 'managed') {
            if (!event?.reviewPauseReceipt) {
              if (key) retryHistory.delete(key);
              if (sourceKey) retryHistory.delete(sourceKey);
            }
          }
          return result;
        }
        const denial = object(result) ? denialCodes.get(result) : null;
        const historyKey = denial?.code === 'E_STATE_SOURCE' ? sourceKey : key;
        if (!denial || !historyKey) return result;
        const remembered = rememberDeniedDispatch(retryHistory, historyKey, denial.code, identity, amendment, denial.requestShapeOnly);
        if (denial.code === 'E_REVIEW') retryHistory.get(historyKey).reviewFingerprint = reviewFingerprint();
        return remembered ? result : deny('E_RETRY_NOT_AUTHORIZED');
      } catch {
        invocation.open = false;
        activeMutators.delete(invocation.token);
        // Logging itself may have failed. Never recurse through the failing sink
        // or expose thrown payloads; native denial remains visible.
        return { block: true, reason: 'workflow dispatch gate: E_INTERNAL: Gate evaluation failed closed.' };
      }
    },
  };
}
