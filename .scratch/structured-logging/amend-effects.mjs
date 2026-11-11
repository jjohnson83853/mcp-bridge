import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";

const root = "/home/opencode/projects/default/mcp-bridge";
const [taskId, ...addPaths] = process.argv.slice(2);
if (!taskId || addPaths.length === 0) {
  console.error("usage: node amend-effects.mjs <taskId> <path> [path...]");
  process.exit(2);
}

const statePath = path.join(root, ".omp", "plans", "execution-state.json");
const sha = p => crypto.createHash("sha256").update(fs.readFileSync(p)).digest("hex");
const state = JSON.parse(fs.readFileSync(statePath, "utf8"));

const task = state.tasks.find(t => t.id === taskId);
if (!task) throw new Error(`task ${taskId} missing`);

const added = [];
for (const p of addPaths) {
  if (!task.effects.paths.includes(p)) {
    task.effects.paths.push(p);
    added.push(p);
  }
}

for (const src of state.sources) {
  if (fs.existsSync(src.artifact.path)) src.artifact.sha256 = sha(src.artifact.path);
}

fs.writeFileSync(statePath, JSON.stringify(state, null, 2) + "\n", "utf8");
console.log(JSON.stringify({ ok: true, taskId, added, total: task.effects.paths.length }));
