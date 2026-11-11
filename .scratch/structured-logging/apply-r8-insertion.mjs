#!/usr/bin/env node
// Mechanical applier for the r8 insertion plan produced by `PlanR8Insertion`.
// Verifies the base digest, every anchor's uniqueness, marker integrity and the
// survival of named r7 exemplars BEFORE writing anything. Refuses on any doubt.
//
// usage: node apply-r8-insertion.mjs <plan.json> <base.md> <out.md>

import { readFileSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";

const [planPath, basePath, outPath] = process.argv.slice(2);
if (!planPath || !basePath || !outPath) {
  console.error("usage: node apply-r8-insertion.mjs <plan.json> <base.md> <out.md>");
  process.exit(2);
}

const sha = (value) => createHash("sha256").update(value).digest("hex");
const countOf = (haystack, needle) => haystack.split(needle).length - 1;
const fail = (detail) => {
  console.error(JSON.stringify({ ok: false, error: detail }));
  process.exit(1);
};

const plan = JSON.parse(readFileSync(planPath, "utf8"));
const base = readFileSync(basePath, "utf8");
const baseDigest = sha(base);

if (!plan.base || plan.base.sha256 !== baseDigest) {
  fail(`base digest mismatch: plan says ${plan.base?.sha256}, file is ${baseDigest}`);
}
if (!Array.isArray(plan.operations) || plan.operations.length === 0) fail("plan has no operations");

// Pre-flight: every anchor and removes string must be present exactly once in the base.
for (const op of plan.operations) {
  if (!op.id || !op.anchor || typeof op.content !== "string") fail(`operation is missing id/anchor/content: ${JSON.stringify(op.id ?? op)}`);
  const anchors = countOf(base, op.anchor);
  if (anchors !== 1) fail(`${op.id}: anchor occurs ${anchors} times in the base, must be 1`);
  if (op.kind === "replace") {
    const removes = countOf(base, op.removes ?? "");
    if (removes !== 1) fail(`${op.id}: removes occurs ${removes} times in the base, must be 1`);
  } else if (op.kind !== "insert-before" && op.kind !== "insert-after") {
    fail(`${op.id}: unknown kind ${op.kind}`);
  }
}

let out = base;
for (const op of plan.operations) {
  const at = out.indexOf(op.anchor);
  if (at < 0) fail(`${op.id}: anchor is absent after earlier operations`);
  if (op.kind === "insert-before") {
    out = out.slice(0, at) + op.content + out.slice(at);
  } else if (op.kind === "insert-after") {
    const end = at + op.anchor.length;
    out = out.slice(0, end) + op.content + out.slice(end);
  } else {
    const from = out.indexOf(op.removes);
    if (from < 0) fail(`${op.id}: removes string is absent after earlier operations`);
    out = out.slice(0, from) + op.content + out.slice(from + op.removes.length);
  }
}

// Structural integrity, using the same rule the workflow harness applies.
const SECTIONS = ["requirements", "design", "tasks", "envelope"];
let last = -1;
for (const name of SECTIONS) {
  const start = `<!-- specdd:${name}:start -->`;
  const end = `<!-- specdd:${name}:end -->`;
  const a = out.indexOf(start);
  const b = out.indexOf(end);
  if (a <= last || a < 0 || b < a + start.length || out.indexOf(start, a + 1) !== -1 || out.indexOf(end, b + 1) !== -1) {
    fail(`marker integrity broken at ${name} (start=${a} end=${b} previousEnd=${last})`);
  }
  last = b + end.length - 1;
}
for (const exemplar of plan.verification?.expectExemplars ?? []) {
  if (!out.includes(exemplar)) fail(`r7 exemplar did not survive the merge: ${JSON.stringify(exemplar)}`);
}
if (out.length <= base.length) fail("merged document did not grow; refusing to write");

writeFileSync(outPath, out);
console.log(JSON.stringify({
  ok: true,
  out: outPath,
  baseBytes: base.length,
  bytes: out.length,
  sha256: sha(out),
  operations: plan.operations.length,
  markers: SECTIONS,
}, null, 1));
