#!/usr/bin/env node
// Applies the planner's narrow2 delta (findings 1-6, narrowing only) to a CANDIDATE copy
// of spec.md and derives the revision's bindings. Never writes spec.md or the live state.
//
// Self-check: against the UNMODIFIED (narrow1) spec it must reproduce the installed section
// hashes and the installed packet digest 15567c2b...; only then are the narrowed hashes
// trustworthy.
//
// usage: node apply-narrow2.mjs [--write]

import { readFileSync, writeFileSync } from "node:fs";
const { canonicalJson, sha } = await import(
  "/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs"
);

const ROOT = "/home/opencode/projects/default/mcp-bridge";
const SRC = `${ROOT}/.scratch/structured-logging/spec.md`;
const OUT = `${ROOT}/.scratch/structured-logging/spec-narrow2-candidate.md`;
const STATE = `${ROOT}/.omp/plans/execution-state.json`;
const REV = "r8-pin-diagnostics-narrow2";
const FEATURE = "structured-logging-pin-diagnostics";
const SECTIONS = ["requirements", "design", "tasks", "envelope"];

const src = readFileSync(SRC, "utf8");
const state = JSON.parse(readFileSync(STATE, "utf8"));

// substring pairs; each must match exactly once
const OPS = [
  ["F2a", "# SPEC-STRUCTURED-LOGGING r8-pin-diagnostics-narrow1 \u2014 STATUS: DRAFT",
    `# SPEC-STRUCTURED-LOGGING ${REV} \u2014 STATUS: DRAFT`],
  ["F2b", "SPEC-STRUCTURED-LOGGING revision r8-pin-diagnostics-narrow1. DRAFT (narrowing amendment per owner decision 2026-11-11, option 2; ready for critic review;",
    `SPEC-STRUCTURED-LOGGING revision ${REV}. DRAFT (narrowing amendment narrow2 per owner decision 2026-11-11: closes review packet 15567c2b\u2026 findings 1\u20136, narrowing only; ready for critic review;`],
  ["F3", "`chunk_index`/`chunk_count`, no silent truncation. test.",
    "`chunk_index`/`chunk_count`; each chunk record carries at most 64 `discovered_tools` names (the design's existing \u226464 bound), so `chunk_count` \u2265 2 whenever a server's listing exceeds 64; no silent truncation. test."],
  ["F4", "(`error.retryable` + `next_attempt_ms`). test.",
    "(`error.retryable`), with `next_attempt_ms` present iff an automatic retry is scheduled (per AD17). test."],
  ["F2c", "reports the same detail set: identity, source location, discovery evidence, publication impact, retry guidance. test.",
    "reports the same core detail \u2014 pin identity (server, tool), source location (`config.property_path`), publication impact (`publication_impact`); discovery evidence (chunked `discovered_tools`) and retry guidance (`next_attempt_ms`) remain refresh-path records (AD18/AD16/AD17) and are not asserted on the reload path. test."],
  ["F5a", "`chunk_count`, `failure_stage`, `endpoint`); each task that adds keys updates the INDEPENDENT hard-coded expected list",
    "`chunk_count`, `endpoint`) \u2014 `failure_stage` is NOT a new key: it already exists in the r7 109-name union (AD2 slot 27), AD17 only reuses it with unchanged semantics; each task that adds keys updates the INDEPENDENT hard-coded expected list"],
];

// the one insertion: after the "UNKNOWN (resolution named, not blocking)" line, outside all markers
const DEFERRAL_ANCHOR = "UNKNOWN (resolution named, not blocking): unchanged from r3";
const DEFERRAL_LINE = "Owner-deferred gap (ticket 17, NOT in narrow2 scope): the known artifact-consistency drift \u2014 state T10 effect paths `internal/upstream/runtime.go`, `cmd/mcp-bridge/serve.go`, `internal/upstream/dedicated_test.go`, the `spec.md:295` `service_edge_test.go` authorization, `tasks[T8].blockedBy` missing its T7 prerequisite, and the decisions/state packet-digest mismatch \u2014 is recorded in `.scratch/structured-logging/issues/17-r8-artifact-consistency-drift.md` and owner-deferred; a re-review must treat it as a deferred, recorded gap, not an unaddressed review finding.";

function sectionsOf(text, revision) {
  const out = {};
  let last = -1;
  for (const name of SECTIONS) {
    const start = `<!-- specdd:${name}:start -->`, end = `<!-- specdd:${name}:end -->`;
    const a = text.indexOf(start), b = text.indexOf(end);
    const ok = a > last && a >= 0 && b >= a + start.length &&
      text.indexOf(start, a + 1) === -1 && text.indexOf(end, b + 1) === -1;
    if (!ok) throw new Error(`marker integrity failed for ${name}`);
    out[name] = { revision, sha256: sha(text.slice(a + start.length, b)) };
    last = b + end.length - 1;
  }
  return out;
}
const packetDigest = (bindings) => sha(canonicalJson({
  feature: FEATURE, projectRoot: ROOT, sectionBindings: bindings,
  branches: [{ id: "approved-structured-logging", answerIds: [], taskIds: ["T8", "T9", "T10", "T11", "T12"], envelopeId: "env-1", sectionBindings: bindings }],
}));

// ---- baseline self-check against the UNMODIFIED (narrow1) spec ----
const baseBindings = sectionsOf(src, "r8-pin-diagnostics-narrow1");
let baseOk = true;
for (const n of SECTIONS) {
  const got = baseBindings[n].sha256;
  const want = state.spec.sectionBindings[n].sha256;
  const ok = got === want;
  if (!ok) baseOk = false;
  console.log(`baseline ${n.padEnd(13)} ${ok ? "MATCH" : "MISMATCH"} ${got.slice(0, 16)}`);
}
const baseDigest = packetDigest(baseBindings);
console.log(`baseline packet   ${baseDigest === state.packet.digest ? "MATCH" : "MISMATCH"} ${baseDigest.slice(0, 16)} (installed ${state.packet.digest.slice(0, 16)})`);
if (!baseOk || baseDigest !== state.packet.digest) {
  console.error("baseline self-check FAILED; refusing to derive new hashes");
  process.exit(1);
}

// ---- apply deltas (each must match exactly once) ----
let text = src;
const fails = [];
for (const [id, from, to] of OPS) {
  const n = text.split(from).length - 1;
  if (n !== 1) { fails.push(`${id}(${n} matches)`); continue; }
  text = text.replace(from, to);
}
const lines = text.split("\n");
const hits = lines.map((l, i) => [l, i]).filter(([l]) => l.includes(DEFERRAL_ANCHOR));
if (hits.length !== 1) fails.push(`deferral-anchor(${hits.length} matches)`);
else lines.splice(hits[0][1] + 1, 0, DEFERRAL_LINE);
text = lines.join("\n");

console.log(`\ndeltas applied: ${OPS.length - fails.filter(f => !f.startsWith("deferral")).length}/${OPS.length}${fails.length ? ` FAILED -> ${fails.join(", ")}` : ""}`);
if (fails.length) { console.error("refusing to write the candidate"); process.exit(1); }

const before = src.split("\n"), after = text.split("\n");
const changed = [];
for (let i = 0; i < Math.max(before.length, after.length); i++) if (before[i] !== after[i]) changed.push(i + 1);
console.log(`changed lines: ${changed.join(",")} (count ${changed.length})`);

const candBindings = sectionsOf(text, REV);
console.log(`\n=== ${REV} ===`);
console.log("spec sha256        :", sha(text));
for (const n of SECTIONS) console.log(`section ${n.padEnd(13)}: ${candBindings[n].sha256}`);
const candDigest = packetDigest(candBindings);
console.log("packet digest      :", candDigest);

if (process.argv.includes("--write")) {
  writeFileSync(OUT, text);
  console.log(`wrote ${OUT}`);
}
