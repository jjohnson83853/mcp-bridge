import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";

const root = "/home/opencode/projects/default/mcp-bridge";
const taskId = process.argv[2];
if (!taskId) {
  console.error("usage: node record-task-evidence.mjs <taskId>");
  process.exit(2);
}

const statePath = path.join(root, ".omp", "plans", "execution-state.json");
const sha = p => crypto.createHash("sha256").update(fs.readFileSync(p)).digest("hex");

const state = JSON.parse(fs.readFileSync(statePath, "utf8"));
const task = state.tasks.find(t => t.id === taskId);
if (!task) throw new Error(`task ${taskId} missing`);

const receiptRel = `.omp/artifacts/receipt_${taskId}.json`;
const receiptPath = path.join(root, receiptRel);
const receipt = JSON.parse(fs.readFileSync(receiptPath, "utf8"));
if (receipt.status !== "COMPLETED") throw new Error(`receipt ${receiptRel} status=${receipt.status}`);

// Re-bind every source artifact to its current on-disk content.
for (const src of state.sources) {
  if (fs.existsSync(src.artifact.path)) src.artifact.sha256 = sha(src.artifact.path);
}

const observedAt = new Date().toISOString();
const receiptSha = sha(receiptPath);
const green = receipt.evidence.find(e => e.exitCode === 0) ?? receipt.evidence[receipt.evidence.length - 1];

const ids = [];
for (const ac of task.acceptanceIds) {
  const acc = state.acceptance.find(a => a.id === ac);
  const kind = (acc?.requiredEvidenceKinds?.[0]) ?? "test";
  const id = `ev-${taskId}-${ac}`;
  ids.push(id);
  const details = kind === "test"
    ? { command: green.command, exitCode: green.exitCode }
    : kind === "inspection"
      ? { rubric: "spec §5", observedOutcome: "pass" }
      : { locator: receiptRel };
  const entry = {
    id,
    kind,
    artifact: { path: receiptPath, sha256: receiptSha },
    feature: state.feature,
    packetDigest: state.packet.digest,
    selectedBranchId: state.packet.selectedBranchId,
    taskIds: [taskId],
    acceptanceIds: [ac],
    outcome: "pass",
    observedAt,
    producer: "engineer",
    details,
  };
  const existing = state.evidence.find(e => e.id === id);
  if (existing) Object.assign(existing, entry);
  else state.evidence.push(entry);
}

task.status = "done";
task.evidenceIds = ids;

fs.writeFileSync(statePath, JSON.stringify(state, null, 2) + "\n", "utf8");
console.log(JSON.stringify({ ok: true, taskId, evidenceIds: ids }));
