// Review observations bind bytes and actor claims, not consent or identity.
// Both managed CLI dispatch and native read-only preparation use this rule.
// Recording contract (producer: coordinator after an actual independent review):
// .omp/plans/review-sequence.json = {schemaVersion:1,candidates:[{
//   id,revision,authorIds,inputs:[{path,sha256}],dispatches:[{
//     name,agent,reviewerId,taskId?,requestSha256?,taskSha256,instructionsSha256
//   }],codeReview:null|{role:"code-reviewer",reviewerId,candidateId,revision,
//     authorIds,inputs,verdict,format:"installed"|"policy",source:{path,sha256}}}],
//   criticObservations?:[{seq,role:"critic",reviewerId,candidateId,revision,
//     verdict,inputsSha256,source:{path,sha256},recordedAt}]}
// Optional reviewHistory?: [{seq,kind,lineageKey,candidateId?,revision?,verdict?,findings?,from?,to?,authorityRef?,maxRounds?}].
// CodeReviewer receipts live in `codeReview`; critic observations never replace them.
// The source artifact is a JSON observation captured for that review:
// {role,reviewerId,candidateId,revision,authorIds,inputs,dispatches,
//  output:<untouched actual final response>}. It is not an approval assertion.
// Hash requestSha256 from canonicalJson(exact managed request entry), taskSha256
// from native item.task (flat managed: wrapper.instructions), instructionsSha256
// from wrapper.instructions (unbound batch: exact context; unbound flat: "").
// Include ALL reviewed files and all planned dispatch descriptors in the review
// manifest BEFORE review. Capture immutable source bytes only AFTER completion.
// After each completed CodeReviewer review, append exactly one `kind:"round"`
// outcome with its binding. Critic observations append separately by candidate revision.
// Append each alias under withReviewRegistryLock on a fresh read, carrying a known cap in maxRounds.
// Clear codeReview before rerun/repair; bump revision, rehash inputs/descriptors,
// obtain a new independent review, then replace the observation/source binding.
// Established schema1/2 state receipts remain untouched and cannot substitute.
// Neither hashes nor this parser authenticate actors or prove input completeness.
// Preparation/checks are read-only; only the native handler knows live children.
import { readFileSync, realpathSync, statSync, lstatSync, writeFileSync, renameSync } from 'node:fs';
import path from 'node:path';
import { canonicalJson, sha } from './execution-contract-binding.mjs';
import { randomUUID } from 'node:crypto';
import { withJournal, withStateLock } from './execution-lock.mjs';

const registryLockFailures = new Set(['E_LOCKED', 'E_LOCK_IO', 'E_LOCK_REENTRANT']);
export function withReviewRegistryLock(file, options = {}, dependencies = {}, fn) {
  const result = withStateLock(file, options, dependencies, fn);
  return result?.ok === false && registryLockFailures.has(result.code)
    ? { ...result, code: 'E_REVIEW_REGISTRY_CHANGED', lockCode: result.code }
    : result;
}

const text = v => typeof v === 'string' && v.trim().length > 0;
const object = v => v !== null && typeof v === 'object' && !Array.isArray(v);
const digest = v => typeof v === 'string' && /^[a-f0-9]{64}$/.test(v);
const inside = (root, file) => file.startsWith(root + path.sep);
const fail = () => [{ code: 'E_REVIEW', location: 'reviewSequence', detail: 'Critic requires a current independent positive CodeReviewer observation for its exact candidate and inputs; dependent review batches are prohibited.' }];
const roles = new Set(['critic', 'code-reviewer']);

const targetValue = value => (value === undefined ? '<absent>' : String(value))
  .replace(/[\u0000-\u001f\u007f]/g, '').slice(0, 512);
const targetIssue = (entry, candidate, dispatch) => {
  const requestCandidateId = targetValue(entry.targetCandidateId);
  const requestRevision = targetValue(entry.targetRevision);
  const descriptorCandidateId = targetValue(dispatch.targetCandidateId);
  const descriptorRevision = targetValue(dispatch.targetRevision);
  if (!text(entry.targetCandidateId) || !text(entry.targetRevision)) {
    return { code: 'E_REVIEW_TARGET', location: 'reviewSequence',
      detail: `absent pair: request=targetCandidateId:${requestCandidateId},targetRevision:${requestRevision}; descriptor=targetCandidateId:${descriptorCandidateId},targetRevision:${descriptorRevision}` };
  }
  for (const [fact, descriptor, identity] of [
    ['targetCandidateId', dispatch.targetCandidateId, candidate.id],
    ['targetRevision', dispatch.targetRevision, candidate.revision],
  ]) {
    if (descriptor !== identity) {
      return { code: 'E_REVIEW_TARGET', location: 'reviewSequence',
        detail: `${fact}: descriptor=${targetValue(descriptor)} candidate=${targetValue(identity)}` };
    }
  }
  for (const [fact, request, descriptor] of [
    ['targetCandidateId', entry.targetCandidateId, dispatch.targetCandidateId],
    ['targetRevision', entry.targetRevision, dispatch.targetRevision],
  ]) {
    if (request !== descriptor) {
      return { code: 'E_REVIEW_TARGET', location: 'reviewSequence',
        detail: `${fact}: request=${targetValue(request)} candidate/descriptor=${targetValue(descriptor)}` };
    }
  }
  return null;
};

function artifact(record, root, deps) {
  if (!object(record) || !text(record.path) || !digest(record.sha256)) throw Error('artifact');
  const file = deps.realpathSync(path.resolve(root, record.path));
  if (!inside(root, file) || !deps.statSync(file).isFile()) throw Error('artifact containment');
  const bytes = deps.readFileSync(file);
  if (sha(bytes) !== record.sha256) throw Error('artifact digest');
  return { path: file, sha256: record.sha256, bytes };
}

function inputs(records, root, deps) {
  if (!Array.isArray(records) || records.length === 0) throw Error('inputs');
  const bound = records.map(record => artifact(record, root, deps)).map(({ path, sha256 }) => ({ path, sha256 })).sort((a, b) => a.path.localeCompare(b.path));
  if (new Set(bound.map(record => record.path)).size !== bound.length) throw Error('duplicate inputs');
  return bound;
}

function verdict(source, format) {
  if (format !== 'installed' && format !== 'policy') return null;
  const lines = source.toString('utf8').replace(/\r\n/g, '\n').split('\n');
  let headingIndex = -1;
  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index].trim();
    const matches = format === 'installed'
      ? line === '### Recommendation' || line === '## Recommendation' || line === '**Recommendation**'
      : line === '### Verdict' || line === '## Verdict' || line === '**Verdict**';
    if (!matches) continue;
    if (headingIndex !== -1) return null;
    headingIndex = index;
  }
  if (headingIndex === -1) return null;
  const tail = lines.slice(headingIndex + 1).join('\n').trim();
  const allowed = format === 'installed' ? ['APPROVE', 'REQUEST CHANGES', 'COMMENT'] : ['PASS', 'REVISE', 'BLOCKED'];
  return allowed.includes(tail) ? tail : null;
}
const criticVerdicts = new Set(['PASS', 'REVISE', 'BLOCKED']);
function criticVerdict(output) {
  if (typeof output !== 'string') return null;
  const lines = output.replace(/\r\n/g, '\n').split('\n');
  const headings = lines.flatMap((line, index) => line.trim() === '### Verdict' ? [index] : []);
  if (headings.length !== 1) return null;
  let index = headings[0] + 1;
  while (index < lines.length && !lines[index].trim()) index += 1;
  const value = lines[index]?.trim();
  return criticVerdicts.has(value) ? value : null;
}

const validCriticRegistryObservation = record => hasExactFields(record, [
  'seq', 'role', 'reviewerId', 'candidateId', 'revision', 'authorIds', 'inputs',
  'dispatches', 'verdict', 'format', 'inputsSha256', 'source', 'recordedAt',
]) && Number.isSafeInteger(record.seq) && record.seq >= 1
  && record.role === 'critic' && text(record.reviewerId) && text(record.candidateId)
  && text(record.revision) && Array.isArray(record.authorIds) && record.authorIds.every(text)
  && Array.isArray(record.inputs) && record.inputs.length > 0
  && record.inputs.every(input => hasExactFields(input, ['path', 'sha256'])
    && text(input.path) && digest(input.sha256))
  && Array.isArray(record.dispatches) && record.dispatches.every(object)
  && criticVerdicts.has(record.verdict) && record.format === 'installed'
  && digest(record.inputsSha256) && hasExactFields(record.source, ['path', 'sha256'])
  && text(record.source.path) && digest(record.source.sha256) && text(record.recordedAt);
const validCriticRunObservation = record => object(record)
  && hasExactFields(record, [
    'seq', 'role', 'reviewerId', 'candidateId', 'revision', 'authorIds', 'inputs',
    'dispatches', 'verdict', 'format', 'inputsSha256', 'source', 'recordedAt', 'receipt',
  ])
  && validCriticRegistryObservation({
    seq: record.seq,
    role: record.role,
    reviewerId: record.reviewerId,
    candidateId: record.candidateId,
    revision: record.revision,
    authorIds: record.authorIds,
    inputs: record.inputs,
    dispatches: record.dispatches,
    verdict: record.verdict,
    format: record.format,
    inputsSha256: record.inputsSha256,
    source: record.source,
    recordedAt: record.recordedAt,
  })
  && (record.verdict === 'PASS'
    ? sameCanonicalJson(record.receipt, {
      sourceId: record.source.path,
      recordedAt: record.recordedAt,
      response: { origin: 'reviewer', verdict: 'pass' },
    })
    : record.receipt === null);

function criticObservationEntries(registry) {
  const entries = [];
  const sequences = new Set();
  const candidates = new Set();
  for (const candidate of registry.candidates) {
    const observations = candidate?.criticObservations;
    if (observations === undefined) continue;
    if (!Array.isArray(observations)) throw Error('critic observation history');
    let previousSeq = 0;
    for (const record of observations) {
      const pair = JSON.stringify([record?.candidateId, record?.revision]);
      if (!validCriticRegistryObservation(record) || record.candidateId !== candidate.id
        || record.seq <= previousSeq || sequences.has(record.seq) || candidates.has(pair)) {
        throw Error('critic observation history');
      }
      previousSeq = record.seq;
      sequences.add(record.seq);
      candidates.add(pair);
      entries.push(record);
    }
  }
  return entries.sort((left, right) => left.seq - right.seq);
}

function boundCriticObservation(root, binding, deps, registryOverride, runsOverride) {
  if (!object(binding) || binding.agent !== 'critic' || !text(binding.candidateId)
    || !text(binding.revision) || !text(binding.reviewerId) || !digest(binding.fingerprint)
    || !text(binding.invocationId)) throw Error('critic observation binding');
  const projectRoot = deps.realpathSync(root);
  const directory = path.join(projectRoot, '.omp', 'plans');
  if (deps.realpathSync(directory) !== directory) throw Error('critic observation directory');
  const registryPath = path.join(directory, 'review-sequence.json');
  let registry = registryOverride;
  if (registry === undefined) {
    const stat = deps.lstatSync(registryPath);
    if (stat.isSymbolicLink() || !stat.isFile()) throw Error('critic observation registry');
    registry = JSON.parse(deps.readFileSync(registryPath, 'utf8'));
  }
  if (registry?.schemaVersion !== 1 || !Array.isArray(registry.candidates)) {
    throw Error('critic observation history');
  }
  const observations = criticObservationEntries(registry);
  const candidates = registry.candidates.filter(candidate => candidate?.id === binding.candidateId);
  if (candidates.length !== 1) throw Error('critic observation candidate');
  const candidate = candidates[0];
  if (candidate.revision !== binding.revision || !Array.isArray(candidate.authorIds)
    || !Array.isArray(candidate.dispatches)) throw Error('critic observation revision');
  const critics = candidate.dispatches.filter(dispatch => dispatch?.agent === 'critic'
    && dispatch.reviewerId === binding.reviewerId);
  if (critics.length === 0 || candidate.authorIds.includes(binding.reviewerId)) {
    throw Error('critic observation reviewer');
  }
  const boundInputs = inputs(candidate.inputs, projectRoot, deps);
  const fingerprint = sha(canonicalJson({
    id: candidate.id, revision: candidate.revision, authorIds: candidate.authorIds,
    inputs: boundInputs, dispatches: candidate.dispatches,
  }));
  if (binding.fingerprint !== fingerprint || !positive(candidate, critics[0], boundInputs, projectRoot, deps, runsOverride)) {
    throw Error('critic observation prerequisite');
  }
  const matches = (candidate.criticObservations ?? []).filter(record =>
    record.revision === candidate.revision && record.reviewerId === binding.reviewerId);
  if (matches.length !== 1 || !observations.includes(matches[0])) {
    throw Error('critic observation missing or ambiguous');
  }
  const record = matches[0];
  if (record.format !== 'installed'
    || canonicalJson(record.authorIds) !== canonicalJson(candidate.authorIds)
    || canonicalJson(record.inputs) !== canonicalJson(candidate.inputs)
    || canonicalJson(record.dispatches) !== canonicalJson(candidate.dispatches)
    || record.inputsSha256 !== sha(canonicalJson(candidate.inputs))) throw Error('critic observation inputs');
  const sourceArtifact = artifact(record.source, projectRoot, deps);
  const source = JSON.parse(sourceArtifact.bytes.toString('utf8'));
  if (!object(source) || source.role !== 'critic' || source.reviewerId !== record.reviewerId
    || source.candidateId !== candidate.id || source.revision !== candidate.revision
    || canonicalJson(source.authorIds) !== canonicalJson(candidate.authorIds)
    || canonicalJson(source.inputs) !== canonicalJson(candidate.inputs)
    || canonicalJson(source.dispatches) !== canonicalJson(candidate.dispatches)
    || source.invocationId !== binding.invocationId
    || criticVerdict(source.output) !== record.verdict) throw Error('critic observation source mismatch');
  return { ...record, source: { path: sourceArtifact.path, sha256: sourceArtifact.sha256 } };
}

const runRole = run => run.role ?? 'code-reviewer';

function validateRunStore(file, data) {
  const observations = data?.observations === undefined ? [] : data.observations;
  if (data?.schemaVersion !== 1 || !Array.isArray(data.runs)
    || data.runs.some(run => !text(run?.candidateId) || !text(run.invocationId)
      || !digest(run.fingerprint) || !text(run.reviewerId) || !['pending', 'succeeded', 'failed'].includes(run.status)
      || (run.role !== undefined && !roles.has(run.role))
      || (run.revision !== undefined && !text(run.revision)))
    || new Set(data.runs.map(run => JSON.stringify([run.candidateId, runRole(run)]))).size !== data.runs.length
    || !Array.isArray(observations) || observations.some(record => !validCriticRunObservation(record))
    || new Set(observations.map(record => JSON.stringify([record.candidateId, record.revision]))).size !== observations.length) {
    throw Error('invalid runs');
  }
  return { file, data: { ...data, observations } };
}

function runStore(root, deps, bytes) {
  const directory = path.join(root, '.omp', 'plans');
  if (deps.realpathSync(directory) !== directory) throw Error('redirected run directory');
  const file = path.join(directory, 'review-runs.json');
  if (bytes !== undefined) {
    return validateRunStore(file, bytes === null
      ? { schemaVersion: 1, runs: [] }
      : JSON.parse(bytes.toString('utf8')));
  }
  try { deps.lstatSync(file); } catch (error) {
    if (error.code === 'ENOENT') return validateRunStore(file, { schemaVersion: 1, runs: [] });
    throw error;
  }
  const stat = deps.lstatSync(file);
  if (stat.isSymbolicLink() || !stat.isFile()) throw Error('invalid run file');
  return validateRunStore(file, JSON.parse(deps.readFileSync(file, 'utf8')));
}

function applyReviewRun(root, data, binding, status, deps, registryOverride) {
  if (!object(binding) || !['pending', 'succeeded', 'failed'].includes(status)) throw Error('review run');
  const role = binding.agent ?? binding.role ?? 'code-reviewer';
  if (!roles.has(role)) throw Error('review run role');
  const { data: current } = validateRunStore(path.join(root, '.omp', 'plans', 'review-runs.json'), data);
  const existing = current.runs.find(run => run.candidateId === binding.candidateId && runRole(run) === role);
  if (status !== 'pending' && existing?.invocationId !== binding.invocationId) {
    if (role === 'critic') throw Error('critic observation run mismatch');
    return current;
  }
  let observations = current.observations;
  if (role === 'critic' && status === 'pending'
    && observations.some(record => record.candidateId === binding.candidateId
      && record.revision === binding.revision)) throw Error('critic observation cannot be rewritten');
  if (role === 'critic' && status === 'succeeded') {
    const observation = boundCriticObservation(root, binding, deps, registryOverride, current);
    if (observations.some(record => record.candidateId === observation.candidateId
      && record.revision === observation.revision)) throw Error('critic observation already recorded');
    const receipt = observation.verdict === 'PASS' ? {
      sourceId: observation.source.path,
      recordedAt: observation.recordedAt,
      response: { origin: 'reviewer', verdict: 'pass' },
    } : null;
    observations = [...observations, { ...observation, receipt }];
  }
  const run = { candidateId: binding.candidateId, invocationId: binding.invocationId,
    fingerprint: binding.fingerprint, reviewerId: binding.reviewerId, status, role,
    ...(text(binding.revision) ? { revision: binding.revision } : {}) };
  return {
    ...current,
    observations,
    runs: [...current.runs.filter(run => run.candidateId !== binding.candidateId || runRole(run) !== role), run],
  };
}

/** Record run lifecycle; critic receipt data requires a verified bound source observation. */
export function recordReviewRun(root, binding, status, dependencies = {}) {
  const deps = { readFileSync, realpathSync, statSync, lstatSync, writeFileSync, renameSync, ...dependencies };
  const registryPath = path.join(path.resolve(root), '.omp', 'plans', 'review-sequence.json');
  const result = withReviewRegistryLock(registryPath, dependencies.lockOptions, deps, () => {
    const { file, data } = runStore(root, deps);
    const updated = applyReviewRun(root, data, binding, status, deps);
    const temporary = `${file}.${randomUUID()}.tmp`;
    deps.writeFileSync(temporary, JSON.stringify(updated) + '\n', { flag: 'wx', mode: 0o600 });
    deps.renameSync(temporary, file);
    const role = binding.agent ?? binding.role ?? 'code-reviewer';
    return updated.runs.find(run => run.candidateId === binding.candidateId && runRole(run) === role);
  });
  if (result?.ok === false) throw Error(`review registry lock failed: ${result.code}`);
  return result;
}


function dispatchBinding(entry, dispatch, payload) {
  if (!object(payload)) return false;
  const items = payload.tasks ?? [payload];
  const item = items.find(item => item.name === entry.name && item.agent === entry.agent);
  if (!item) return false;
  let wrapper;
  try { wrapper = JSON.parse(payload.tasks ? payload.context : payload.task); } catch {}
  const managed = entry.taskId !== undefined;
  if (managed && canonicalJson(wrapper?.workflowDispatch?.requests?.find(request => request.name === entry.name)) !== canonicalJson(entry)) return false;
  const instructions = managed ? wrapper?.instructions : payload.context ?? '';
  const task = managed && !payload.tasks ? wrapper?.instructions : item.task;
  return text(task) && text(dispatch.reviewerId)
    && dispatch.taskSha256 === sha(task) && dispatch.instructionsSha256 === sha(instructions)
    && (entry.taskId === undefined || dispatch.requestSha256 === sha(canonicalJson(entry)));
}


function positive(candidate, dispatch, bound, root, deps, runsOverride) {
  const review = candidate.codeReview;
  if (!object(review) || review.role !== 'code-reviewer' || !text(review.reviewerId)
    || review.reviewerId === dispatch.reviewerId || candidate.authorIds.includes(review.reviewerId)
    || candidate.authorIds.includes(dispatch.reviewerId) || !text(dispatch.reviewerId)) return false;
  if (review.candidateId !== candidate.id || review.revision !== candidate.revision
    || canonicalJson(review.authorIds) !== canonicalJson(candidate.authorIds)
    || canonicalJson(inputs(review.inputs, root, deps)) !== canonicalJson(bound)) return false;
  const source = JSON.parse(artifact(review.source, root, deps).bytes.toString('utf8'));
  if (!object(source) || source.role !== review.role || source.reviewerId !== review.reviewerId
    || source.candidateId !== candidate.id || source.revision !== candidate.revision
    || canonicalJson(source.authorIds) !== canonicalJson(candidate.authorIds)
    || canonicalJson(inputs(source.inputs, root, deps)) !== canonicalJson(bound) || !text(source.output)) return false;
  if (canonicalJson(source.dispatches) !== canonicalJson(candidate.dispatches)) return false;
  const run = (runsOverride ?? runStore(root, deps).data).runs.find(run => run.candidateId === candidate.id
    && runRole(run) === 'code-reviewer');
  const fingerprint = sha(canonicalJson({ id: candidate.id, revision: candidate.revision,
    authorIds: candidate.authorIds, inputs: bound, dispatches: candidate.dispatches }));
  if (run && (run.status !== 'succeeded' || run.fingerprint !== fingerprint
    || source.invocationId !== run.invocationId || source.reviewerId !== run.reviewerId)) return false;
  const observed = verdict(source.output, review.format);
  if (review.verdict !== observed) return false;
  if (review.format === 'installed' ? observed === 'APPROVE' : observed === 'PASS') {
    if (!deps.reviewRegistry) return true;
    const history = lineageReviewHistory(deps.reviewRegistry, reviewLineageKey(candidate.id));
    if (history.issues.length) return false;
    const index = history.rounds.findIndex(round => round.candidateId === candidate.id
      && round.revision === candidate.revision);
    return index < 0 || blockingFindings({ ...history.rounds[index], round: index + 1 }).length === 0;
  }
  if (!['REQUEST CHANGES', 'COMMENT', 'REVISE', 'BLOCKED'].includes(observed)) return false;
  const history = lineageReviewHistory(deps.reviewRegistry, reviewLineageKey(candidate.id));
  if (history.issues.length) return false;
  const index = history.rounds.findIndex(round => round.candidateId === candidate.id
    && round.revision === candidate.revision && round.verdict === observed
    && round.reviewerId === review.reviewerId && round.sourceSha256 === review.source.sha256
    && round.inputsSha256 === sha(canonicalJson(candidate.inputs)));
  return index >= 0 && blockingFindings({ ...history.rounds[index], round: index + 1 }).length === 0;
}

/** Commit one bound observation and its run record through the review journal. */
export function recordBoundReview(root, request, dependencies = {}) {
  const projectRoot = path.resolve(root);
  const plans = path.join(projectRoot, '.omp', 'plans');
  const registryPath = path.join(plans, 'review-sequence.json');
  const runPath = path.join(plans, 'review-runs.json');
  const deps = { readFileSync, realpathSync, statSync, lstatSync, writeFileSync, renameSync, ...dependencies };
  return withJournal(registryPath, {
    feature: 'review-sequence',
    registryPath: runPath,
    journalPath: path.join(plans, 'evidence-continuity', `${sha(registryPath)}.review-journal.json`),
    lockOptions: dependencies.lockOptions ?? {},
    ...(dependencies.onSeam ? { onSeam: dependencies.onSeam } : {}),
  }, snapshot => {
    if (!snapshot.state.bytes) {
      return { noMutation: true, value: { ok: false, code: 'E_REVIEW', detail: 'review registry is missing' } };
    }
    const registry = JSON.parse(snapshot.state.bytes.toString('utf8'));
    const appended = bindWithReviewRound(registry, request.binding, request.options);
    if (!appended.ok) return { noMutation: true, value: appended };
    let runs = runStore(projectRoot, deps, snapshot.registry.bytes).data;
    runs = applyReviewRun(projectRoot, runs, request.runBinding, 'pending', deps, appended.registry);
    if (request.status !== 'pending') {
      runs = applyReviewRun(projectRoot, runs, request.runBinding, request.status, deps, appended.registry);
    }
    return {
      stateBytes: `${JSON.stringify(appended.registry)}\n`,
      registryBytes: `${JSON.stringify(runs)}\n`,
      value: { ok: true, candidateId: appended.candidate.id },
    };
  }, deps);
}

function requiredReviewInputs(state, root, deps) {
  const specPath = deps.realpathSync(path.resolve(root, state.spec.path));
  if (!inside(root, specPath) || specPath === root || !deps.statSync(specPath).isFile()) {
    throw Error('reviewed specification changed');
  }
  const specBytes = deps.readFileSync(specPath);
  if (sha(specBytes.toString('utf8').replace(/\r\n/g, '\n')) !== state.spec.sha256) {
    throw Error('reviewed specification changed');
  }
  const specInputSha = sha(specBytes);
  const required = new Map([[specPath, specInputSha]]);
  for (const task of state.tasks) {
    for (const input of task.effects.paths) {
      let file;
      try { file = deps.realpathSync(path.resolve(root, input)); }
      catch (error) {
        if (error.code === 'ENOENT' || error.code === 'ENOTDIR') continue;
        throw error;
      }
      if (!inside(root, file)) throw Error('review input containment');
      if (deps.statSync(file).isFile()) required.set(file, sha(deps.readFileSync(file)));
    }
  }
  return { specPath, specInputSha, required };
}

function candidateForState(state, registry, root, deps) {
  if (registry?.schemaVersion !== 1 || !Array.isArray(registry.candidates)) throw Error('review registry');
  const { specPath, specInputSha, required } = requiredReviewInputs(state, root, deps);
  const matches = [];
  for (const candidate of registry.candidates) {
    if (candidate?.revision !== state.revision || !Array.isArray(candidate.inputs)
      || !candidate.inputs.some(input => object(input)
        && path.resolve(root, input.path) === specPath && input.sha256 === specInputSha)) continue;
    const bound = inputs(candidate.inputs, root, deps);
    if ([...required].every(([file, digest]) => bound.some(input => input.path === file && input.sha256 === digest))) {
      matches.push({ candidate, bound });
    }
  }
  if (matches.length !== 1) throw Error('review candidate missing or ambiguous');
  return matches[0];
}

function codeReviewReceiptEvidence(state, registry, candidate, bound, runs, root, deps) {
  const codeReview = candidate.codeReview;
  const codeDispatches = candidate.dispatches.filter(dispatch => dispatch?.agent === 'code-reviewer');
  const criticDispatches = candidate.dispatches.filter(dispatch => dispatch?.agent === 'critic');
  if (codeDispatches.length !== 1 || criticDispatches.length !== 1
    || !object(codeReview) || codeReview.reviewerId !== codeDispatches[0].reviewerId
    || codeReview.reviewerId === criticDispatches[0].reviewerId
    || [state.actors.plannerId, ...state.actors.implementerIds].includes(codeReview.reviewerId)
    || !positive(candidate, criticDispatches[0], bound, root, deps, runs)) throw Error('CodeReviewer observation invalid');
  const rounds = (registry.reviewHistory ?? []).filter(round => round?.kind === 'round'
    && round.candidateId === candidate.id && round.revision === candidate.revision
    && round.reviewerId === codeReview.reviewerId && round.verdict === codeReview.verdict
    && round.inputsSha256 === sha(canonicalJson(candidate.inputs))
    && round.sourceSha256 === codeReview.source.sha256 && text(round.recordedAt));
  if (rounds.length !== 1) throw Error('CodeReviewer review history missing or ambiguous');
  const source = artifact(codeReview.source, root, deps);
  return { reviewerId: codeReview.reviewerId, source, recordedAt: rounds[0].recordedAt,
    criticDispatch: criticDispatches[0] };
}

function criticReceiptEvidence(state, candidate, bound, code, runs, registry, root, deps) {
  const entries = criticObservationEntries(registry);
  const current = (candidate.criticObservations ?? []).filter(record =>
    record.candidateId === candidate.id && record.revision === candidate.revision);
  if (current.length !== 1 || current[0].verdict !== 'PASS') throw Error('passing Critic observation missing');
  const record = current[0];
  const criticRuns = runs.runs.filter(run => run.candidateId === candidate.id && runRole(run) === 'critic');
  const fingerprint = sha(canonicalJson({ id: candidate.id, revision: candidate.revision,
    authorIds: candidate.authorIds, inputs: bound, dispatches: candidate.dispatches }));
  if (!entries.includes(record) || criticRuns.length !== 1 || criticRuns[0].status !== 'succeeded'
    || criticRuns[0].revision !== candidate.revision || criticRuns[0].reviewerId !== record.reviewerId
    || criticRuns[0].fingerprint !== fingerprint
    || [state.actors.plannerId, ...state.actors.implementerIds].includes(record.reviewerId)
    || record.reviewerId === code.reviewerId) throw Error('Critic run binding invalid');
  const verified = boundCriticObservation(root, {
    agent: 'critic', candidateId: candidate.id, revision: candidate.revision,
    reviewerId: criticRuns[0].reviewerId, fingerprint, invocationId: criticRuns[0].invocationId,
  }, deps, registry, runs);
  const normalized = { ...record, source: { path: verified.source.path, sha256: record.source.sha256 } };
  const runObservation = { ...verified, receipt: {
    sourceId: verified.source.path,
    recordedAt: verified.recordedAt,
    response: { origin: 'reviewer', verdict: 'pass' },
  } };
  const mirrored = runs.observations.filter(item => item.candidateId === candidate.id
    && item.revision === candidate.revision);
  if (!sameCanonicalJson(normalized, verified) || mirrored.length !== 1
    || !sameCanonicalJson(mirrored[0], runObservation)) throw Error('Critic registry observations differ');
  return { reviewerId: verified.reviewerId, source: artifact(verified.source, root, deps),
    recordedAt: verified.recordedAt };
}

function deriveCandidateReviewEvidence(state, registry, runData, dependencies = {}) {
  const deps = { readFileSync, realpathSync, statSync, lstatSync, ...dependencies };
  const root = path.resolve(state.projectRoot);
  if (deps.realpathSync(root) !== root || !Array.isArray(state.actors?.implementerIds)
    || !Array.isArray(state.tasks)) throw Error('review state binding');
  const { candidate, bound } = candidateForState(state, registry, root, deps);
  if (!text(candidate.id) || !Array.isArray(candidate.authorIds)
    || !candidate.authorIds.every(text) || !Array.isArray(candidate.dispatches)) throw Error('review candidate');
  const runs = validateRunStore(path.join(root, '.omp', 'plans', 'review-runs.json'), runData);
  const code = codeReviewReceiptEvidence(state, registry, candidate, bound, runs.data, root, deps);
  const critic = criticReceiptEvidence(state, candidate, bound, code, runs.data, registry, root, deps);
  return {
    candidateId: candidate.id,
    revision: candidate.revision,
    specification: { reviewerId: code.reviewerId, source: code.source, recordedAt: code.recordedAt },
    tasks: { reviewerId: critic.reviewerId, source: critic.source, recordedAt: critic.recordedAt },
  };
}

function sourceId(key, evidence, candidateId, revision) {
  return `review-${key}-${sha(canonicalJson({
    candidateId, revision, reviewerId: evidence.reviewerId, sourceSha256: evidence.source.sha256,
  }))}`;
}

function recordReceipts(state, evidence) {
  const reviews = state.reviews === null ? { specification: null, tasks: null } : { ...state.reviews };
  const sources = [...state.sources];
  let changed = state.reviews === null;
  for (const key of ['specification', 'tasks']) {
    if (reviews[key]?.verdict === 'waived') continue;
    const reviewEvidence = evidence[key];
    const id = sourceId(key, reviewEvidence, evidence.candidateId, evidence.revision);
    const source = {
      id, kind: 'observation',
      artifact: { path: reviewEvidence.source.path, sha256: reviewEvidence.source.sha256 },
      locator: `${key} review observation for ${evidence.candidateId}@${evidence.revision}`,
      recordedAt: reviewEvidence.recordedAt,
    };
    const existingSource = sources.find(item => item.id === id);
    if (existingSource && !sameCanonicalJson(existingSource, source)) throw Error('review source id collision');
    const review = {
      verdict: 'pass',
      reviewerId: reviewEvidence.reviewerId,
      packetDigest: state.packet.digest,
      receipt: { sourceId: id, recordedAt: reviewEvidence.recordedAt,
        response: { origin: 'reviewer', verdict: 'pass' } },
    };
    if (reviews[key] !== null) {
      if (!sameCanonicalJson(reviews[key], review) || !existingSource) throw Error('review already bound differently');
      continue;
    }
    if (!existingSource) sources.push(source);
    reviews[key] = review;
    changed = true;
  }
  return changed ? { state: { ...state, reviews, sources }, changed } : { state, changed };
}

/** Record only observations verified against the current candidate and run registries. */
export function recordCanonicalReviews(statePath, dependencies = {}) {
  const deps = { readFileSync, realpathSync, statSync, lstatSync, writeFileSync, renameSync, ...dependencies };
  const stateFile = path.resolve(statePath);
  const originalBytes = deps.readFileSync(stateFile);
  const initialState = JSON.parse(originalBytes.toString('utf8'));
  const root = path.resolve(initialState.projectRoot);
  const plans = path.join(root, '.omp', 'plans');
  const registryPath = path.join(plans, 'review-sequence.json');
  const journalPath = path.join(plans, 'evidence-continuity', `${sha(stateFile)}.review-journal.json`);
  const runsPath = path.join(plans, 'review-runs.json');
  return withJournal(stateFile, {
    feature: initialState.feature,
    registryPath,
    markerPath: runsPath,
    journalPath,
    expectedStateBytes: originalBytes,
    lockOptions: dependencies.lockOptions ?? {},
    ...(dependencies.onSeam ? { onSeam: dependencies.onSeam } : {}),
  }, snapshot => {
    const state = JSON.parse(snapshot.state.bytes.toString('utf8'));
    const shapeIssues = dependencies.validateState?.(state) ?? [];
    if (shapeIssues.length) {
      return { noMutation: true, value: { ok: false, code: 'E_STATE', location: shapeIssues[0].location } };
    }
    try {
      const runs = runStore(root, deps, snapshot.marker.bytes).data;
      const evidence = deriveCandidateReviewEvidence(state,
        JSON.parse(snapshot.registry.bytes.toString('utf8')), runs, deps);
      const recorded = recordReceipts(state, evidence);
      if (!recorded.changed) return { noMutation: true, value: { ok: true, alreadyRecorded: true } };
      const issues = dependencies.validateState?.(recorded.state) ?? [];
      if (issues.length) {
        return { noMutation: true, value: { ok: false, code: 'E_STATE', location: issues[0].location } };
      }
      return {
        stateBytes: `${JSON.stringify(recorded.state, null, 2)}\n`,
        value: { ok: true, candidateId: evidence.candidateId, revision: evidence.revision },
      };
    } catch {
      return { noMutation: true, value: { ok: false, code: 'E_REVIEW', location: 'reviewSequence' } };
    }
  }, deps);
}

/** Class and completed-review ordinal decide blocking; severity only orders reports. */
export function severityGate(findings, round = 1) {
  const blocking = blockingFindings({ round, findings });
  const substantive = new Set(blocking);
  const advisory = findings.filter(finding => !substantive.has(finding));
  return { blocking, advisory, pass: blocking.length === 0 };
}

const lineageIssue = detail => ({ code: 'E_REVIEW', location: 'reviewSequence', detail: `lineage-accounting: ${detail}` });
const validAliasCap = entry => !Object.hasOwn(entry, 'maxRounds')
  || (Number.isInteger(entry.maxRounds) && entry.maxRounds >= 1);
const reviewHistoryShapeIssues = registry => {
  if (registry.reviewHistory === undefined) return [];
  if (!Array.isArray(registry.reviewHistory)) return [lineageIssue('reviewHistory must be an array.')];
  const issues = [];
  let previousSeq = -Infinity;
  for (const [index, entry] of registry.reviewHistory.entries()) {
    if (!object(entry)) {
      issues.push(lineageIssue(`reviewHistory[${index}] must be an object.`));
      continue;
    }
    if (!Number.isInteger(entry.seq) || entry.seq <= previousSeq) {
      issues.push(lineageIssue(`reviewHistory[${index}].seq must be an increasing integer.`));
    }
    if (Number.isInteger(entry.seq)) previousSeq = entry.seq;
    if (!text(entry.lineageKey)) issues.push(lineageIssue(`reviewHistory[${index}].lineageKey is required.`));
    if (!['round', 'legacy-unobserved', 'alias'].includes(entry.kind)) {
      issues.push(lineageIssue(`reviewHistory[${index}].kind is unknown.`));
      continue;
    }
    if (entry.kind === 'round' && (!text(entry.candidateId) || !text(entry.revision) || !text(entry.verdict)
      || !Array.isArray(entry.findings)
      || (entry.priorFindings !== undefined && !Array.isArray(entry.priorFindings))
      || (entry.inputsSha256 !== undefined && !digest(entry.inputsSha256))
      || (entry.sourceSha256 !== undefined && entry.sourceSha256 !== null && !digest(entry.sourceSha256))
      || (entry.recordedAt !== undefined && entry.recordedAt !== null && !text(entry.recordedAt)))) {
      issues.push(lineageIssue(`reviewHistory[${index}] round fields are incomplete or malformed.`));
    } else if (entry.kind === 'legacy-unobserved' && (!text(entry.candidateId) || !text(entry.revision))) {
      issues.push(lineageIssue(`reviewHistory[${index}] legacy-unobserved fields are incomplete.`));
    } else if (entry.kind === 'alias' && (!text(entry.from) || !text(entry.to) || !text(entry.authorityRef)
      || !validAliasCap(entry))) {
      issues.push(lineageIssue(`reviewHistory[${index}] alias fields are incomplete or malformed.`));
    }
  }
  return issues;
};
const reviewAliasTopology = registry => {
  const history = Array.isArray(registry.reviewHistory) ? registry.reviewHistory : [];
  const aliases = history.filter(entry => object(entry) && entry.kind === 'alias'
    && text(entry.from) && text(entry.to));
  const knownLineages = new Set();
  for (const candidate of registry.candidates) {
    if (text(candidate?.id)) knownLineages.add(reviewLineageKey(candidate.id));
  }
  for (const entry of history) {
    if (object(entry) && entry.kind !== 'alias' && text(entry.lineageKey)) knownLineages.add(entry.lineageKey);
    if (object(entry) && entry.kind === 'alias' && text(entry.to)) knownLineages.add(entry.to);
  }
  const issues = [];
  const next = new Map();
  const previous = new Map();
  for (const alias of aliases) {
    if (!knownLineages.has(alias.from)) issues.push(lineageIssue('alias from target is unresolved.'));
    if (next.has(alias.from) && next.get(alias.from) !== alias.to) {
      issues.push(lineageIssue('alias lineage branches to multiple targets.'));
    } else next.set(alias.from, alias.to);
    if (previous.has(alias.to) && previous.get(alias.to) !== alias.from) {
      issues.push(lineageIssue('alias lineage merges multiple sources.'));
    } else previous.set(alias.to, alias.from);
  }
  const complete = new Set();
  for (const start of next.keys()) {
    const visiting = new Set();
    let current = start;
    while (next.has(current)) {
      if (visiting.has(current)) {
        issues.push(lineageIssue('alias lineage contains a cycle.'));
        break;
      }
      if (complete.has(current)) break;
      visiting.add(current);
      current = next.get(current);
    }
    for (const item of visiting) complete.add(item);
  }
  return { aliases, next, previous, issues };
};
const reviewHistoryOutcomeIssues = registry => {
  const outcomes = new Map();
  const issues = [];
  const history = Array.isArray(registry.reviewHistory) ? registry.reviewHistory : [];
  for (const entry of history) {
    if (!object(entry) || entry.kind !== 'round' || !text(entry.candidateId)
      || !text(entry.revision) || !text(entry.verdict)) continue;
    const key = JSON.stringify([entry.candidateId, entry.revision]);
    const previous = outcomes.get(key);
    if (previous !== undefined && previous !== entry.verdict) {
      issues.push(lineageIssue('recorded outcomes conflict for one candidate revision.'));
    } else outcomes.set(key, entry.verdict);
  }
  return issues;
};
const reviewHistoryAccountingIssues = registry => [
  ...reviewHistoryShapeIssues(registry),
  ...reviewHistoryOutcomeIssues(registry),
  ...reviewAliasTopology(registry).issues,
];
const reviewPair = (candidateId, revision) => JSON.stringify([candidateId, revision]);
export function lineageReviewHistory(registry, lineageKey) {
  const issues = [];
  if (!object(registry) || !Array.isArray(registry.candidates)) {
    return { rounds: [], unobserved: [], aliasChain: [], issues: [lineageIssue('registry candidates are malformed.')] };
  }
  if (!text(lineageKey)) issues.push(lineageIssue('lineage key is required.'));
  issues.push(...reviewHistoryAccountingIssues(registry));
  const topology = reviewAliasTopology(registry);
  let first = text(lineageKey) ? lineageKey : '';
  const backward = new Set();
  while (topology.previous.has(first) && !backward.has(first)) {
    backward.add(first);
    first = topology.previous.get(first);
  }
  const aliasChain = first ? [first] : [];
  const lineages = new Set(aliasChain);
  let current = first;
  while (topology.next.has(current) && !lineages.has(topology.next.get(current))) {
    current = topology.next.get(current);
    aliasChain.push(current);
    lineages.add(current);
  }
  if (text(lineageKey) && !lineages.has(lineageKey)) {
    aliasChain.push(lineageKey);
    lineages.add(lineageKey);
  }
  const history = Array.isArray(registry.reviewHistory) ? registry.reviewHistory : [];
  const recorded = history.filter(entry => object(entry) && lineages.has(entry.lineageKey)
    && ['round', 'legacy-unobserved'].includes(entry.kind));
  const rounds = recorded.filter(entry => entry.kind === 'round');
  const unobserved = recorded.filter(entry => entry.kind === 'legacy-unobserved');
  const recordedByPair = new Map();
  for (const entry of recorded) {
    const key = reviewPair(entry.candidateId, entry.revision);
    const entries = recordedByPair.get(key) ?? [];
    entries.push(entry);
    recordedByPair.set(key, entries);
  }
  const derivedRounds = new Map();
  const derivedUnobserved = new Set(unobserved.map(entry => reviewPair(entry.candidateId, entry.revision)));
  for (const candidate of registry.candidates) {
    if (!object(candidate) || !text(candidate.id) || !text(candidate.revision)) continue;
    const candidateLineage = reviewLineageKey(candidate.id);
    if (!lineages.has(candidateLineage)) continue;
    const currentPair = reviewPair(candidate.id, candidate.revision);
    const review = candidate.codeReview;
    if (object(review) && object(review.source)) {
      const candidateId = text(review.candidateId) ? review.candidateId : candidate.id;
      const revision = text(review.revision) ? review.revision : candidate.revision;
      if (reviewLineageKey(candidateId) !== candidateLineage || !text(review.verdict)) {
        issues.push(lineageIssue('bound legacy observation has incomplete candidate, revision, or verdict fields.'));
      } else {
        const key = reviewPair(candidateId, revision);
        const prior = recordedByPair.get(key) ?? [];
        const recordedRounds = prior.filter(entry => entry.kind === 'round');
        if (recordedRounds.length) {
          if (recordedRounds.some(entry => entry.verdict !== review.verdict)) {
            issues.push(lineageIssue('recorded and derived verdicts conflict for one candidate revision.'));
          }
        } else if (prior.length > 0) {
          issues.push(lineageIssue('recorded legacy-unobserved marker conflicts with a bound observation.'));
        } else {
          const duplicate = derivedRounds.get(key);
          if (duplicate && duplicate.verdict !== review.verdict) {
            issues.push(lineageIssue('legacy observations conflict for one candidate revision.'));
          } else if (!duplicate) {
            const round = { kind: 'round', lineageKey: candidateLineage, candidateId, revision,
              verdict: review.verdict, findings: [] };
            derivedRounds.set(key, round);
            rounds.push(round);
          }
        }
      }
    }
    if (!recordedByPair.has(currentPair) && !derivedRounds.has(currentPair)
      && !derivedUnobserved.has(currentPair)) {
      unobserved.push({ kind: 'legacy-unobserved', lineageKey: candidateLineage,
        candidateId: candidate.id, revision: candidate.revision });
      derivedUnobserved.add(currentPair);
    }
  }
  return { rounds, unobserved, aliasChain, issues };
}
export function reviewRoundCount(registry, lineageKey) {
  return lineageReviewHistory(registry, lineageKey).rounds.length;
}

const substantiveClasses = new Set([
  'ac-not-falsifiable', 'r-ac-coverage-gap', 'statement-contradiction', 'lifecycle-hazard',
]);

/** Round is the one-based completed review ordinal, never a retry counter. */
export function blockingFindings(round) {
  if (!object(round) || !Number.isSafeInteger(round.round) || round.round < 1
    || !Array.isArray(round.findings)) throw Error('review round findings');
  return round.round >= 3 ? [] : round.findings.filter(finding => substantiveClasses.has(finding?.class));
}

export function residualRegister(registry, lineageKey) {
  const history = lineageReviewHistory(registry, lineageKey);
  if (history.issues.length) throw Error('review lineage accounting');
  return history.rounds.slice(2).flatMap(round => round.findings.map(
    ({ class: findingClass, severity, anchor, fix }) => ({ class: findingClass, severity, anchor, fix })));
}
/**
 * Mechanical round cap. Explicit candidate caps win; recorded alias caps carry
 * the limit after predecessor removal, with surviving predecessors retained
 * as the legacy fallback. Completed lineage outcomes consume the cap.
 */
const aliasCapFor = (registry, lineages) => {
  let cap;
  for (const entry of registry.reviewHistory ?? []) {
    if (entry.kind === 'alias' && lineages.has(entry.from) && lineages.has(entry.to)
      && Object.hasOwn(entry, 'maxRounds')) cap = entry.maxRounds;
  }
  return cap;
};
export function roundCapIssues(candidate, registry, ownerOverride = false) {
  if (!object(candidate) || !object(registry) || !Array.isArray(registry.candidates)) throw Error('round cap input');
  let lineageKey;
  try { lineageKey = reviewLineageKey(candidate.id); } catch {
    return [lineageIssue('candidate lineage cannot be resolved.')];
  }
  const history = lineageReviewHistory(registry, lineageKey);
  if (history.issues.length) return history.issues;
  const chain = history.aliasChain.length ? history.aliasChain : [lineageKey];
  const currentLineage = reviewLineageKey(candidate.id);
  const currentIndex = chain.indexOf(currentLineage);
  const lineagePrefix = new Set(currentIndex < 0 ? [currentLineage] : chain.slice(0, currentIndex + 1));
  const predecessorLineages = new Set(currentIndex < 0 ? chain.slice(0, -1) : chain.slice(0, currentIndex));
  const predecessorCaps = registry.candidates.flatMap(item => {
    if (!object(item) || !text(item.id) || !predecessorLineages.has(reviewLineageKey(item.id))
      || !Number.isInteger(item.maxRounds) || item.maxRounds < 1) return [];
    return [item.maxRounds];
  });
  const aliasCap = aliasCapFor(registry, lineagePrefix);
  const hasCandidateCap = Object.hasOwn(candidate, 'maxRounds');
  const inheritedCap = predecessorCaps[0] ?? aliasCap;
  if (inheritedCap !== undefined) {
    const mismatch = predecessorCaps.some(cap => cap !== predecessorCaps[0])
      || (aliasCap !== undefined && aliasCap !== predecessorCaps[0] && predecessorCaps.length > 0)
      || (hasCandidateCap && candidate.maxRounds !== inheritedCap)
      || (predecessorCaps.length > 0 && !hasCandidateCap);
    if (mismatch) {
      return ownerOverride ? [] : [{ code: 'E_ROUND_CAP', location: 'reviewSequence',
        detail: `cap-continuity: candidate lineage ${lineageKey} must retain predecessor maxRounds=${predecessorCaps[0] ?? aliasCap}; an explicit owner override is required.` }];
    }
  }
  if (!hasCandidateCap && aliasCap === undefined) return [];
  const maxRounds = hasCandidateCap ? candidate.maxRounds : aliasCap;
  if (!Number.isInteger(maxRounds) || maxRounds < 1) {
    return [{ code: 'E_ROUND_CAP', location: 'reviewSequence',
      detail: `cap-continuity: candidate lineage ${lineageKey} has an invalid maxRounds value.` }];
  }
  // Exhaustion is not a continuity defect. The class schedule governs findings.
  return [];
}
const roundAppendIssue = detail => ({
  code: 'E_ROUND_APPEND', location: 'reviewSequence', detail: `round-append: ${detail}`,
});
const duplicateRoundIssues = registry => {
  const seen = new Set();
  const issues = [];
  for (const entry of registry.reviewHistory ?? []) {
    if (entry.kind !== 'round') continue;
    const key = reviewPair(entry.candidateId, entry.revision);
    if (seen.has(key)) issues.push(lineageIssue('recorded rounds duplicate one candidate revision.'));
    else seen.add(key);
  }
  return issues;
};
const validRoundFields = round => object(round) && text(round.verdict)
  && Array.isArray(round.findings) && digest(round.inputsSha256)
  && (round.sourceSha256 === null || digest(round.sourceSha256))
  && (round.recordedAt === null || text(round.recordedAt));
const roundBindingFields = ['reviewerId', 'verdict', 'findings', 'inputsSha256', 'sourceSha256', 'recordedAt'];
const reviewBindingFields = [
  'candidateId', 'reviewerId', 'authorIds', 'inputs', 'dispatches', 'verdict',
  'findings', 'format', 'source', 'recordedAt',
];
const hasExactFields = (value, fields) => object(value) && Reflect.ownKeys(value).length === fields.length
  && Reflect.ownKeys(value).every(field => typeof field === 'string' && fields.includes(field));
const sameCanonicalJson = (left, right) => {
  try { return canonicalJson(left) === canonicalJson(right); } catch { return false; }
};
const criticBindingFields = ['role', 'candidateId', 'reviewerId', 'authorIds', 'inputs',
  'dispatches', 'verdict', 'format', 'source', 'recordedAt'];

function bindCriticObservation(registry, candidate, binding, reject) {
  let observations;
  try { observations = criticObservationEntries(registry); }
  catch { return reject(lineageIssue('critic observation history is malformed.')); }
  if (observations.some(record => record.candidateId === candidate.id && record.revision === candidate.revision)) {
    return reject(lineageIssue('a critic observation already exists for this candidate revision.'));
  }
  const candidateObservations = candidate.criticObservations ?? [];
  const critics = Array.isArray(candidate.dispatches)
    ? candidate.dispatches.filter(dispatch => dispatch?.agent === 'critic'
      && dispatch.reviewerId === binding.reviewerId) : [];
  const reviewers = Array.isArray(candidate.dispatches)
    ? candidate.dispatches.filter(dispatch => dispatch?.agent === 'code-reviewer') : [];
  const codeReview = candidate.codeReview;
  const validFirstStage = reviewers.length === 1 && object(codeReview)
    && codeReview.role === 'code-reviewer' && codeReview.reviewerId === reviewers[0].reviewerId
    && codeReview.candidateId === candidate.id && codeReview.revision === candidate.revision
    && ['APPROVE', 'PASS'].includes(codeReview.verdict)
    && ['installed', 'policy'].includes(codeReview.format)
    && sameCanonicalJson(codeReview.authorIds, candidate.authorIds)
    && sameCanonicalJson(codeReview.inputs, candidate.inputs)
    && hasExactFields(codeReview.source, ['path', 'sha256'])
    && text(codeReview.source.path) && digest(codeReview.source.sha256);
  const validAuthors = Array.isArray(candidate.authorIds) && candidate.authorIds.length > 0
    && candidate.authorIds.every(text);
  const validInputs = Array.isArray(binding.inputs) && binding.inputs.length > 0
    && binding.inputs.every(input => hasExactFields(input, ['path', 'sha256'])
      && text(input.path) && digest(input.sha256));
  const validDispatches = Array.isArray(candidate.dispatches) && candidate.dispatches.every(object)
    && Array.isArray(binding.dispatches) && sameCanonicalJson(binding.dispatches, candidate.dispatches);
  const validSource = hasExactFields(binding.source, ['path', 'sha256'])
    && text(binding.source.path) && digest(binding.source.sha256);
  const validBinding = hasExactFields(binding, criticBindingFields)
    && binding.role === 'critic' && binding.candidateId === candidate.id
    && text(binding.reviewerId) && critics.length > 0
    && validAuthors && !candidate.authorIds.includes(binding.reviewerId)
    && validInputs && sameCanonicalJson(binding.authorIds, candidate.authorIds)
    && sameCanonicalJson(binding.inputs, candidate.inputs)
    && validDispatches && binding.format === 'installed'
    && criticVerdicts.has(binding.verdict) && validSource
    && text(binding.recordedAt);
  if (!validFirstStage || !validBinding) {
    return reject(lineageIssue('critic observation or first-stage review binding is invalid.'));
  }
  const seq = (observations.at(-1)?.seq ?? 0) + 1;
  if (!Number.isSafeInteger(seq)) return reject(lineageIssue('critic observation sequence is not increasing.'));
  const criticObservation = {
    seq,
    role: 'critic',
    reviewerId: binding.reviewerId,
    candidateId: candidate.id,
    revision: candidate.revision,
    authorIds: [...candidate.authorIds],
    inputs: candidate.inputs.map(input => ({ ...input })),
    dispatches: candidate.dispatches.map(dispatch => ({ ...dispatch })),
    verdict: binding.verdict,
    format: 'installed',
    inputsSha256: sha(canonicalJson(binding.inputs)),
    source: { ...binding.source },
    recordedAt: binding.recordedAt,
  };
  const updatedCandidate = { ...candidate, criticObservations: [...candidateObservations, criticObservation] };
  const candidates = registry.candidates.map(item => item === candidate ? updatedCandidate : item);
  return {
    ok: true,
    registry: { ...registry, candidates },
    candidate: updatedCandidate,
    criticObservation,
  };
}

/**
 * Pure producer: bind a CodeReviewer round or append a separate Critic observation.
 * Critic observations never modify codeReview and yield receipt data only after
 * recordReviewRun verifies the persisted source bytes.
 */
export function bindWithReviewRound(registry, binding, options = {}) {
  const reject = issue => ({ ok: false, issues: [issue] });
  const malformedCandidate = candidate => !object(candidate) || !text(candidate.id)
    || !text(candidate.revision) || (candidate.codeReview !== undefined && candidate.codeReview !== null
      && (!object(candidate.codeReview) || !object(candidate.codeReview.source)
        || !text(candidate.codeReview.source.path) || !digest(candidate.codeReview.source.sha256)
        || !text(candidate.codeReview.verdict)))
    || (candidate.criticObservations !== undefined
      && (!Array.isArray(candidate.criticObservations)
        || candidate.criticObservations.some(record => !validCriticRegistryObservation(record))));
  if (!object(options) || (options.ownerOverride !== undefined && typeof options.ownerOverride !== 'boolean')
    || !object(registry) || registry.schemaVersion !== 1 || !Array.isArray(registry.candidates)
    || registry.candidates.some(malformedCandidate)
    || new Set(registry.candidates.map(candidate => candidate.id)).size !== registry.candidates.length) {
    return reject(lineageIssue('registry or producer options are malformed.'));
  }
  try { criticObservationEntries(registry); }
  catch { return reject(lineageIssue('critic observation history is malformed.')); }
  const accountingIssues = reviewHistoryAccountingIssues(registry);
  if (accountingIssues.length) return { ok: false, issues: accountingIssues };
  const duplicates = duplicateRoundIssues(registry);
  if (duplicates.length) return { ok: false, issues: duplicates };
  if (!object(binding) || !text(binding.candidateId)) return reject(lineageIssue('binding candidateId is required.'));
  const matches = registry.candidates.filter(candidate => candidate.id === binding.candidateId);
  if (matches.length !== 1) return reject(lineageIssue('binding candidate does not exist uniquely.'));
  const candidate = matches[0];
  if (binding.role === 'critic') return bindCriticObservation(registry, candidate, binding, reject);
  const lineageKey = reviewLineageKey(candidate.id);
  const history = lineageReviewHistory(registry, lineageKey);
  if (history.issues.length) return { ok: false, issues: history.issues };
  const lineages = new Set(history.aliasChain);
  const recorded = registry.reviewHistory ?? [];
  const recordedRounds = recorded.filter(entry => entry.kind === 'round' && lineages.has(entry.lineageKey));
  const legacyRounds = history.rounds.filter(round => !Number.isInteger(round.seq));
  if (legacyRounds.length > 1) return reject(lineageIssue('multiple unrecorded bound observations cannot be captured by one legacyRound.'));
  if (legacyRounds.length && options.legacyRound === undefined) {
    return reject(roundAppendIssue('a bound legacy observation must be captured before rebinding.'));
  }
  const bindingKeys = Reflect.ownKeys(binding);
  const bindOnly = bindingKeys.every(field => field === 'candidateId'
    || (typeof field === 'string' && roundBindingFields.includes(field)));
  if (bindOnly) {
    return recordedRounds.length
      ? reject(roundAppendIssue('a recorded lineage requires an appendable observation round.'))
      : reject(lineageIssue('binding observation fields are required.'));
  }
  if (!hasExactFields(binding, reviewBindingFields)) {
    return reject(lineageIssue('binding observation fields are invalid.'));
  }
  const validAuthors = Array.isArray(candidate.authorIds) && candidate.authorIds.length > 0
    && candidate.authorIds.every(text);
  const validInputs = Array.isArray(binding.inputs) && binding.inputs.length > 0
    && binding.inputs.every(input => hasExactFields(input, ['path', 'sha256'])
      && text(input.path) && digest(input.sha256));
  const validDispatches = Array.isArray(candidate.dispatches)
    && candidate.dispatches.every(object)
    && Array.isArray(binding.dispatches)
    && sameCanonicalJson(binding.dispatches, candidate.dispatches);
  const source = binding.source;
  const dependentCritics = Array.isArray(candidate.dispatches)
    ? candidate.dispatches.filter(dispatch => dispatch?.agent === 'critic') : [];
  const independent = text(binding.reviewerId) && validAuthors
    && !candidate.authorIds.includes(binding.reviewerId)
    && dependentCritics.every(dispatch => text(dispatch.reviewerId)
      && dispatch.reviewerId !== binding.reviewerId
      && !candidate.authorIds.includes(dispatch.reviewerId));
  const inputsMatch = validInputs && sameCanonicalJson(binding.authorIds, candidate.authorIds)
    && sameCanonicalJson(binding.inputs, candidate.inputs);
  const validObservation = binding.candidateId === candidate.id && independent && inputsMatch
    && validDispatches && binding.format === 'installed'
    && ['APPROVE', 'REQUEST CHANGES', 'COMMENT'].includes(binding.verdict)
    && Array.isArray(binding.findings)
    && hasExactFields(source, ['path', 'sha256']) && text(source.path) && digest(source.sha256);
  if (!validObservation) return reject(lineageIssue('binding observation or reviewer independence is invalid.'));
  const roundFields = {
    reviewerId: binding.reviewerId,
    verdict: binding.verdict,
    findings: binding.findings,
    inputsSha256: sha(canonicalJson(binding.inputs)),
    sourceSha256: binding.source.sha256,
    recordedAt: binding.recordedAt,
  };
  if (!validRoundFields(roundFields)) {
    return reject(lineageIssue('binding observation or findings are invalid.'));
  }
  const newPair = reviewPair(candidate.id, candidate.revision);
  if (history.rounds.some(round => reviewPair(round.candidateId, round.revision) === newPair)) {
    return reject(lineageIssue('a round already exists for the bound candidate revision.'));
  }
  let legacy = null;
  if (legacyRounds.length) {
    const round = options.legacyRound;
    const prior = legacyRounds[0];
    const observation = registry.candidates.find(item => {
      const review = item.codeReview;
      return object(review) && object(review.source)
        && (text(review.candidateId) ? review.candidateId : item.id) === prior.candidateId
        && (text(review.revision) ? review.revision : item.revision) === prior.revision;
    });
    if (!validRoundFields(round) || round.verdict !== prior.verdict
      || !observation || round.sourceSha256 !== observation.codeReview.source.sha256) {
      return reject(lineageIssue('legacyRound does not match the bound observation.'));
    }
    legacy = { ...round, candidateId: prior.candidateId, revision: prior.revision, lineageKey: prior.lineageKey };
  }
  const capIssues = roundCapIssues(candidate, registry, options.ownerOverride);
  if (capIssues.length) return { ok: false, issues: capIssues };
  let seq = (recorded.at(-1)?.seq ?? 0) + 1;
  const nextSeq = () => {
    if (!Number.isSafeInteger(seq) || (recorded.length && seq <= recorded.at(-1).seq)) return null;
    return seq++;
  };
  const appended = [];
  if (legacy) {
    const legacySeq = nextSeq();
    if (legacySeq === null) return reject(lineageIssue('constructed reviewHistory seq is not increasing.'));
    appended.push({ seq: legacySeq, kind: 'round', lineageKey: legacy.lineageKey,
      candidateId: legacy.candidateId, revision: legacy.revision, verdict: legacy.verdict,
      findings: [...legacy.findings], inputsSha256: legacy.inputsSha256,
      sourceSha256: legacy.sourceSha256, recordedAt: legacy.recordedAt });
  }
  const entrySeq = nextSeq();
  if (entrySeq === null) return reject(lineageIssue('constructed reviewHistory seq is not increasing.'));
  const entry = { seq: entrySeq, kind: 'round', lineageKey, candidateId: candidate.id,
    revision: candidate.revision, reviewerId: roundFields.reviewerId, verdict: roundFields.verdict,
    findings: [...roundFields.findings], inputsSha256: roundFields.inputsSha256,
    sourceSha256: roundFields.sourceSha256, recordedAt: roundFields.recordedAt };
  appended.push(entry);
  const updatedCandidate = {
    ...candidate,
    codeReview: {
      role: 'code-reviewer',
      reviewerId: binding.reviewerId,
      candidateId: candidate.id,
      revision: candidate.revision,
      authorIds: binding.authorIds,
      inputs: binding.inputs,
      dispatches: binding.dispatches,
      verdict: binding.verdict,
      format: binding.format,
      source: binding.source,
    },
  };
  const candidates = registry.candidates.map(item => item === candidate ? updatedCandidate : item);
  return {
    ok: true,
    registry: { ...registry, candidates, reviewHistory: [...recorded, ...appended] },
    candidate: updatedCandidate,
    entry,
  };
}

/** No writes, approvals, dispatches or schema migration occur here. */
export function reviewSequenceIssues(projectRoot, entries, dependencies = {}, context = {}) {
  if (!Array.isArray(entries)) return fail();
  if (!entries.some(entry => roles.has(entry?.agent))) return [];
  const deps = { readFileSync, realpathSync, statSync, lstatSync, ...dependencies };
  try {
    const root = deps.realpathSync(projectRoot);
    const registryPath = deps.realpathSync(path.join(root, '.omp/plans/review-sequence.json'));
    if (!inside(root, registryPath)) return fail();
    const registry = JSON.parse(deps.readFileSync(registryPath, 'utf8'));
    if (registry?.schemaVersion !== 1 || !Array.isArray(registry.candidates)) return fail();
    const accountingIssues = reviewHistoryAccountingIssues(registry);
    if (accountingIssues.length) return accountingIssues;
    const ids = registry.candidates.map(candidate => candidate?.id);
    if (new Set(ids).size !== ids.length) {
      const duplicates = [...new Set(ids.filter((id, index) => ids.indexOf(id) !== index))].map(targetValue);
      return [{ code: 'E_REVIEW', location: 'reviewSequence',
        detail: `duplicate candidate ids: ${duplicates.join(', ')}`.slice(0, 512) }];
    }
    for (const candidate of registry.candidates) {
      const cap = roundCapIssues(candidate, registry, context.ownerRoundOverride === true);
      if (cap.length > 0) return cap;
    }
    for (const candidate of registry.candidates) {
      const review = candidate?.codeReview;
      if (!object(review) || !object(review.source) || !['APPROVE', 'PASS'].includes(review.verdict)) continue;
      const candidateId = text(review.candidateId) ? review.candidateId : candidate.id;
      const revision = text(review.revision) ? review.revision : candidate.revision;
      const history = lineageReviewHistory(registry, reviewLineageKey(candidate.id));
      const recordedHistory = Array.isArray(registry.reviewHistory) ? registry.reviewHistory : [];
      const hasRecordedLineage = recordedHistory.some(entry => object(entry)
        && (entry.kind === 'alias'
          ? history.aliasChain.includes(entry.from) || history.aliasChain.includes(entry.to)
          : history.aliasChain.includes(entry.lineageKey)));
      const matchingRound = hasRecordedLineage
        ? recordedHistory.some(round => object(round) && round.kind === 'round'
          && history.aliasChain.includes(round.lineageKey)
          && round.candidateId === candidateId && round.revision === revision)
        : history.rounds.some(round => round.candidateId === candidateId && round.revision === revision);
      if (history.issues.length) return history.issues;
      if (!matchingRound) {
        return [{ code: 'E_REVIEW', location: 'reviewSequence',
          detail: `no recorded review round for candidate ${targetValue(candidateId)}@${targetValue(revision)}; record the round before dispatching a dependent review.`.slice(0, 512) }];
      }
    }
    const matchedEntries = entries.filter(entry => roles.has(entry.agent)).map(entry => {
      const matches = registry.candidates.flatMap(candidate => {
        if (!object(candidate) || !Array.isArray(candidate.dispatches)) throw Error('candidate');
        return candidate.dispatches.filter(d => d?.name === entry.name && d.agent === entry.agent
          && d.taskId === entry.taskId).map(dispatch => ({ candidate, dispatch }));
      });
      if (matches.length !== 1) throw Error('ambiguous or missing dispatch');
      return { entry, ...matches[0] };
    });
    const targetIssues = matchedEntries.map(({ entry, candidate, dispatch }) =>
      targetIssue(entry, candidate, dispatch)).filter(Boolean);
    if (targetIssues.length) return targetIssues;
    const boundEntries = matchedEntries.map(({ entry, candidate, dispatch }) => {
      if (!text(candidate.id) || !text(candidate.revision) || !Array.isArray(candidate.authorIds)
        || candidate.authorIds.length === 0 || !candidate.authorIds.every(text)
        || (context.revision !== undefined && candidate.revision !== context.revision)) throw Error('identity');
      if (!dispatchBinding(entry, dispatch, context.payload)) throw Error('changed dispatch inputs');
      const bound = inputs(candidate.inputs, root, deps);
      if (context.spec) {
        const specPath = deps.realpathSync(context.spec.path);
        const normalized = deps.readFileSync(specPath, 'utf8').replace(/\r\n/g, '\n');
        if (!bound.some(input => input.path === specPath) || sha(normalized) !== context.spec.sha256) throw Error('spec not reviewed');
      }
      if (entry.effects?.kind === 'read' && entry.effects.paths.some(file => !bound.some(input => input.path === deps.realpathSync(path.resolve(root, file))))) throw Error('unreviewed dispatch input');
      if (entry.agent === 'critic' && !positive(candidate, dispatch, bound, root, { ...deps, reviewRegistry: registry })
        && context.reviewPauseResume?.candidateId !== candidate.id) {
        const history = lineageReviewHistory(registry, reviewLineageKey(candidate.id));
        const index = history.rounds.findIndex(round => round.candidateId === candidate.id
          && round.revision === candidate.revision);
        const blockers = index < 0 ? [] : blockingFindings({ ...history.rounds[index], round: index + 1 });
        if (blockers.length) throw Error(blockers.map(finding => `${finding.class} at ${finding.anchor}`).join('; '));
        throw Error('prerequisite');
      }
      return { entry, candidate, bound };
    });
    for (const critic of boundEntries.filter(item => item.entry.agent === 'critic')) {
      for (const reviewer of boundEntries.filter(item => item.entry.agent === 'code-reviewer')) {
        if (critic.candidate.id === reviewer.candidate.id || critic.bound.some(a => reviewer.bound.some(b => a.path === b.path))) return fail();
      }
    }
    context.bindings?.push(...boundEntries.map(({ entry, candidate, bound }) => ({
      key: `${root}:${candidate.id}`, candidateId: candidate.id, revision: candidate.revision,
      fingerprint: sha(canonicalJson({ id: candidate.id, revision: candidate.revision,
        authorIds: candidate.authorIds, inputs: bound, dispatches: candidate.dispatches })),
      reviewerId: candidate.dispatches.find(d => d.name === entry.name && d.agent === entry.agent).reviewerId,
      agent: entry.agent, name: entry.name,
      paths: bound.map(input => input.path), sourceSha256: candidate.codeReview?.source?.sha256 ?? null,
    })));
    context.residualRegister = boundEntries.flatMap(({ candidate }) =>
      residualRegister(registry, reviewLineageKey(candidate.id)));
    return [];
  } catch (error) { return [{ code: 'E_REVIEW', location: 'reviewSequence',
    detail: `review-sequence check failed: ${targetValue(error?.message)}`.slice(0, 512) }]; }
}
const reviewLineageKey = id => {
  if (!text(id)) throw Error('candidate id');
  return id.replace(/-r\d+$/, '') || id;
};

const reviewId = review => {
  const id = review?.candidateId ?? review?.id;
  return text(id) ? id : null;
};

const blockingFinding = finding => finding?.blocking === true
  || finding?.severity === 'blocking' || finding?.priority === 'blocking';

const evidenceClasses = finding => {
  const values = Array.isArray(finding?.evidenceClasses) ? finding.evidenceClasses
    : text(finding?.evidenceClass) ? [finding.evidenceClass] : [];
  return values.filter(text);
};

/** Compare reviewer finding ledgers; input hashes alone are never material. */
export function reviewMateriality(priorReview, nextReview) {
  if (!object(priorReview) || !object(nextReview)) throw Error('review materiality input');
  const priorFindings = Array.isArray(priorReview.findings) ? priorReview.findings : [];
  const nextFindings = Array.isArray(nextReview.findings) ? nextReview.findings : [];
  const resolvedLedger = Array.isArray(nextReview.priorFindings) ? nextReview.priorFindings : nextFindings;
  const priorBlocking = new Map(priorFindings.filter(blockingFinding).map(finding => [finding.id, finding]));
  const resolvedIds = resolvedLedger.filter(finding => {
    const prior = priorBlocking.get(finding?.id);
    return prior && finding.status === 'RESOLVED' && text(prior.anchor)
      && text(finding.anchor) && prior.anchor !== finding.anchor;
  }).map(finding => finding.id);
  const priorClasses = new Set(priorFindings.flatMap(evidenceClasses));
  const newEvidenceClasses = [...new Set(nextFindings.filter(blockingFinding)
    .flatMap(evidenceClasses).filter(value => !priorClasses.has(value)))];
  return {
    material: resolvedIds.length > 0 || newEvidenceClasses.length > 0,
    resolvedIds,
    newEvidenceClasses,
  };
}

/** Derive lineage counters from persisted registry review observations only. */
export function reviewStallSignal(candidate, registry, reviewsHistory) {
  if (!object(candidate) || !object(registry) || !Array.isArray(registry.candidates)
    || !Array.isArray(reviewsHistory)) throw Error('review history');
  const candidateId = candidate.id ?? candidate.candidateId;
  const lineageKey = reviewLineageKey(candidateId);
  const lineageKeys = new Set(Array.isArray(candidate.reviewLineages) ? candidate.reviewLineages : [lineageKey]);
  const rounds = reviewsHistory.map((review, index) => {
    const id = reviewId(review);
    if (!id || !text(review.revision) || !text(review.verdict)
      || !Array.isArray(review.findings)) throw Error('review record');
    return {
      review, index, id,
      lineageKey: text(review.lineageKey) ? review.lineageKey : reviewLineageKey(id),
    };
  }).filter(round => lineageKeys.has(round.lineageKey));
  const revisionNumber = id => Number(id.match(/-r(\d+)$/)?.[1] ?? NaN);
  rounds.sort((left, right) => {
    const leftSeq = Number.isInteger(left.review.seq) ? left.review.seq : undefined;
    const rightSeq = Number.isInteger(right.review.seq) ? right.review.seq : undefined;
    if (leftSeq !== undefined || rightSeq !== undefined) {
      if (leftSeq === undefined) return 1;
      if (rightSeq === undefined) return -1;
      return rightSeq - leftSeq;
    }
    const leftRevision = revisionNumber(left.id);
    const rightRevision = revisionNumber(right.id);
    return Number.isFinite(leftRevision) && Number.isFinite(rightRevision)
      ? rightRevision - leftRevision : right.index - left.index;
  });
  let nonPassStreak = 0;
  for (const { review } of rounds) {
    if (review.verdict === 'PASS') break;
    nonPassStreak++;
  }
  let noProgressCount = 0;
  for (let index = 0; index + 1 < rounds.length; index++) {
    if (!reviewMateriality(rounds[index + 1].review, rounds[index].review).material) noProgressCount++;
  }
  const latest = rounds[0]?.review;
  const priorFindings = Array.isArray(latest?.priorFindings) ? latest.priorFindings : [];
  const latestFindings = latest?.findings ?? [];
  const carriedOpenIds = priorFindings.filter(finding => finding?.status === 'OPEN'
    && (finding.severity === 'blocking' || finding.severity === 'advisory'
      || finding.blocking === true || finding.advisory === true))
    .map(finding => finding.id).filter(text);
  const priorIds = new Set(priorFindings.map(finding => finding?.id));
  const newBlockingIds = latestFindings.filter(finding => blockingFinding(finding)
    && text(finding.id) && !priorIds.has(finding.id)).map(finding => finding.id);
  return {
    lineageKey,
    nonPassStreak,
    noProgressCount,
    carriedOpenIds,
    newBlockingIds,
    stalled: (nonPassStreak >= 2 && noProgressCount >= 1) || nonPassStreak >= 3,
  };
}

/** Split findings in priority order: precise repairs first, owner decisions second. */
export function classifyReviewFindings(findings, ownerDecisionIds) {
  if (!Array.isArray(findings) || !(Array.isArray(ownerDecisionIds) || ownerDecisionIds instanceof Set)) {
    throw Error('review findings');
  }
  const settledIds = new Set(ownerDecisionIds);
  const isOwnerDecision = finding => {
    const contradicted = finding?.decisionId && settledIds.has(finding.decisionId);
    const contradictedSet = Array.isArray(finding?.contradictsDecisionIds)
      && finding.contradictsDecisionIds.some(id => settledIds.has(id));
    return contradicted || contradictedSet || finding?.ownerDecision === true
      || ['owner-decision', 'acceptance-contract', 'gate-integrity', 'iterate-vs-accept'].includes(finding?.kind)
      || finding?.acceptanceContractChange === true || finding?.gateIntegrity === true
      || finding?.iterateVsAccept === true;
  };
  const exactRepair = finding => text(finding?.path) && text(finding?.locator)
    && text(finding?.requiredChange ?? finding?.implementableChange)
    && (finding.withinEnvelope === true || finding.withinApprovedEnvelope === true);
  if (findings.length === 0 || findings.every(exactRepair)) {
    return { ownerDecision: [], technicalRepair: [...findings], demoted: [] };
  }
  const ownerDecision = [];
  const technicalRepair = [];
  const demoted = [];
  for (const finding of findings) {
    if (isOwnerDecision(finding)) ownerDecision.push(finding);
    else if (finding?.repoAnswerable === true) {
      technicalRepair.push(finding);
      demoted.push(finding);
    } else technicalRepair.push(finding);
  }
  return { ownerDecision, technicalRepair, demoted };
}
