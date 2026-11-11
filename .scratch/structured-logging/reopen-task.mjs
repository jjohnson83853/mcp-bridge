import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";

const root = "/home/opencode/projects/default/mcp-bridge";
const taskId = process.argv[2];
if (!taskId) {
  console.error("usage: node reopen-task.mjs <taskId>");
  process.exit(2);
}

const statePath = path.join(root, ".omp", "plans", "execution-state.json");
const sha = p => crypto.createHash("sha256").update(fs.readFileSync(p)).digest("hex");
const state = JSON.parse(fs.readFileSync(statePath, "utf8"));

const task = state.tasks.find(t => t.id === taskId);
if (!task) throw new Error(`task ${taskId} missing`);

const removed = task.evidenceIds ?? [];
task.status = "pending";
task.evidenceIds = [];
state.evidence = state.evidence.filter(e => !removed.includes(e.id));

// Re-bind sources to current on-disk content.
for (const src of state.sources) {
  if (fs.existsSync(src.artifact.path)) src.artifact.sha256 = sha(src.artifact.path);
}

fs.writeFileSync(statePath, JSON.stringify(state, null, 2) + "\n", "utf8");
console.log(JSON.stringify({ ok: true, taskId, status: task.status, removedEvidence: removed }));
