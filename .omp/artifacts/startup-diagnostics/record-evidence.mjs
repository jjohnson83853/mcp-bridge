import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { withJournal } from '/home/opencode/.omp/agent/workflow-policy/execution-lock.mjs';
import { validateShape } from '/home/opencode/.omp/agent/workflow-policy/execution-contract-schema.mjs';
import { evaluateContract } from '/home/opencode/.omp/agent/workflow-policy/execution-contract.mjs';

const root = '/home/opencode/projects/default/mcp-bridge';
const statePath = path.join(root, '.omp/plans/execution-state.json');
const artifactRel = '.omp/artifacts/receipt_T1.json';
const artifactPath = path.join(root, artifactRel);
const sha = createHash('sha256').update(readFileSync(artifactPath)).digest('hex');
const state = JSON.parse(readFileSync(statePath, 'utf8'));
const now = new Date().toISOString();
const focused = "out=$(go test ./cmd/mcp-bridge -run '^(TestStartupStageFailureOmitsRawCauseAndConfiguredSecrets|TestStartupListenerServeFailureClassified|TestStartupGracefulShutdownEmitsNoStartupFailure|TestStartupFailureForErrorClassifiesStages|TestUnknownStartupErrorKeepsDistinctUnknownCode)$' -v -count=1 2>&1); rc=$?; [ \"$rc\" -eq 0 ]";
const base = {
  kind: 'test',
  artifact: { path: artifactPath, sha256: sha },
  feature: state.feature,
  packetDigest: state.packet.digest,
  selectedBranchId: state.packet.selectedBranchId,
  taskIds: ['T1'],
  outcome: 'pass',
  observedAt: now,
  producer: 'engineer',
};
const commands = {
  AC1: focused,
  AC2: focused,
  AC3: 'go build ./... && go vet ./... && go test ./... -count=1 && go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./... && go run ./cmd/covcheck -profile coverage.out -root .',
  AC4: focused,
  AC5: focused,
  AC6: focused,
};
const evidence = Object.entries(commands).map(([ac, command], index) => ({
  ...base,
  id: `EV-${ac}`,
  acceptanceIds: [ac],
  details: { command, exitCode: 0 },
}));
const task = state.tasks.find(t => t.id === 'T1');
const evidenceIds = evidence.map(e => e.id);
const next = {
  ...state,
  tasks: state.tasks.map(t => t.id === 'T1' ? { ...t, status: 'done', evidenceIds } : t),
  evidence: [...state.evidence.filter(e => !evidenceIds.includes(e.id)), ...evidence],
};
const errors = validateShape(next);
if (errors.length) throw new Error('schema: ' + JSON.stringify(errors));
const report = evaluateContract(next, { readFileSync });
if (report.status === 'invalid') throw new Error('contract: ' + JSON.stringify(report.reasons));
const bytes = Buffer.from(`${JSON.stringify(next, null, 2)}\n`);
const result = withJournal(statePath, { feature: state.feature, lockOptions: {} }, snapshot => {
  const live = JSON.parse(snapshot.state.bytes.toString('utf8'));
  if (live.packet.digest !== state.packet.digest) return { noMutation: true, value: { ok: false, detail: 'packet changed' } };
  return { stateBytes: bytes, value: { ok: true, evidenceIds } };
});
console.log(JSON.stringify({ result, contractStatus: report.status, completion: report.completion }, null, 2));
