#!/usr/bin/env node
// Applies the planner's narrowing delta (D1-D18) to a CANDIDATE copy of spec.md and
// derives the revision's bindings. Never writes spec.md or the live state.
//
// Self-check: against the UNMODIFIED spec it must reproduce the installed section
// hashes and the installed packet digest 358ee090...; only then are the narrowed
// hashes trustworthy.
//
// usage: node apply-narrow1.mjs [--write]

import { readFileSync, writeFileSync } from "node:fs";
const { canonicalJson, sha } = await import(
  "/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs"
);

const ROOT = "/home/opencode/projects/default/mcp-bridge";
const SRC = `${ROOT}/.scratch/structured-logging/spec.md`;
const OUT = `${ROOT}/.scratch/structured-logging/spec-narrow1-candidate.md`;
const STATE = `${ROOT}/.omp/plans/execution-state.json`;
const REV = "r8-pin-diagnostics-narrow1";
const FEATURE = "structured-logging-pin-diagnostics";
const SECTIONS = ["requirements", "design", "tasks", "envelope"];

const src = readFileSync(SRC, "utf8");
const state = JSON.parse(readFileSync(STATE, "utf8"));

const OPS = [
  ["D1", "(file, property path, one-based line, one-based column)", "(property path)"],
  ["D2a", "stating the configuration revision, the trigger, and the next scheduled attempt.",
    "stating the configuration revision and the trigger, and \u2014 on the start record \u2014 the next scheduled attempt."],
  ["D2b", "distinguishes the retained catalog (generation, tool count, age) from no-usable-catalog/zero-upstream-tools, and distinguishes candidate counts from retained counts.",
    "distinguishes the retained catalog (generation, tool count) from no-usable-catalog/zero-upstream-tools."],
  ["D3", "reload attempts identify the configuration revision and trigger and whether the candidate was accepted or rejected; refresh records name the revision they used. Startup, SIGHUP reload and periodic refresh report IDENTICAL",
    "refresh records name the configuration revision and trigger they used. SIGHUP reload and periodic refresh report IDENTICAL"],
  ["D4", "`config.property_path`, `config.line`, `config.column` (REUSED R6 keys, unchanged semantics \u2014 the pin's configuration source), `blocker_class`",
    "`config.property_path` (the pin's configuration source, as the `pins[i]` index the loader retains), `blocker_class`"],
  ["D5", "at warn or error with debug disabled.",
    "at the AD16-pinned level \u2014 warn when `catalog_state` is `retained_published`, error when `none_available` \u2014 with debug disabled."],
  ["D6", "and each carries `config_revision`, `trigger`, `next_attempt_ms`. test.",
    "and each carries `config_revision`, `trigger`; the start record carries `next_attempt_ms`. test."],
  ["D7", "carries file, `config.property_path`, one-based `config.line`, one-based `config.column`. test.",
    "carries `config.property_path` (the pin's `pins[i]` path). test."],
  ["D8", "all evidence in one failure correlates to one `refresh_id` and one `config_revision`; no result mixes catalog generations.",
    "all evidence in one failure correlates to one `refresh_id` and, through the shared `catalog_refresh_failed` record, to that pass's `config_revision`; no result mixes catalog generations."],
  ["D9", "`http_status` or the timeout value reported where applicable. test.",
    "`http_status` is present exactly when the failing stage produced an HTTP status; timeout failures instead name the timeout class and value in `failure_reason` (per AD17). test."],
  ["D10", "a pin rejection at startup, on SIGHUP reload and on periodic refresh reports the same detail set",
    "a pin rejection on SIGHUP reload and on periodic refresh reports the same detail set"],
  ["D11", "no path's pin failure is obtainable only through SIGHUP. test.",
    "neither surviving path's pin failure is obtainable only through SIGHUP \u2014 the periodic refresh record alone carries the full AC11-1 detail set. test."],
  ["D12", "the full suite passes and documented contract matches producer emission. test (`go test ./...`).",
    "the full suite passes and no doc drift remains \u2014 the AC12-1 doc-drift test is the mechanical oracle for documented-contract-matches-emission. test (`go test ./...`)."],
  ["D13", "Deliverable: AD16 records; completion summary counts-only.",
    "Deliverable: AC08-1..7 (AD16 records; completion summary counts-only)."],
  ["D14", "RED: multi-unresolved-pin test asserts ALL pins reported with source locations \u2014 expected failure (first-only/text-only today).",
    "RED: multi-unresolved-pin test asserts ALL pins reported with `config.property_path` identity \u2014 expected failure (first-only/text-only today). Deliverable: AC09-1..8 (AC09-2 asserting `config.property_path` only, per the narrowed revision)."],
  ["D15", "RED: discovery-failure test asserts stage/status/endpoint attribution \u2014 expected failure.",
    "RED: discovery-failure test asserts stage/status/endpoint attribution \u2014 expected failure. Deliverable: AC10-1..5."],
  ["D16a", "Files: `internal/service/loop.go` (reload revision/trigger attribution), `cmd/mcp-bridge/serve.go` (startup-path pin detail parity), `cmd/mcp-bridge/wiring.go`",
    "Files: `internal/service/loop.go` (reload revision/trigger attribution), `cmd/mcp-bridge/wiring.go`"],
  ["D16b", "RED: parity test drives the SAME pinned-catalog rejection through startup and reload paths and asserts equal detail \u2014 expected failure (startup path lacks detail); fallback test asserts encode-failure surfaces `logging_write_failed` \u2014 expected failure.",
    "RED: parity test drives the SAME pinned-catalog rejection through the SIGHUP reload and periodic refresh paths and asserts equal detail \u2014 expected failure (the paths' detail differs today); fallback test asserts encode-failure surfaces `logging_write_failed` \u2014 expected failure. Deliverable: AC11-1..3."],
  ["D17a", "# SPEC-STRUCTURED-LOGGING r7-closure1 \u2014 STATUS: DRAFT",
    `# SPEC-STRUCTURED-LOGGING ${REV} \u2014 STATUS: DRAFT`],
  ["D17b", "SPEC-STRUCTURED-LOGGING revision r8-pin-diagnostics. DRAFT (ready for critic review; execution requires the workflow's explicit user checkpoints).",
    `SPEC-STRUCTURED-LOGGING revision ${REV}. DRAFT (narrowing amendment per owner decision 2026-11-11, option 2; ready for critic review; execution requires the workflow's explicit user checkpoints).`],
  ["D18", "(known accepted gap, r8 changelog)",
    "(known gap; acceptance pending owner ruling, backlog ticket 18 \u2014 r8 changelog)"],
];

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

// ---- baseline self-check against the UNMODIFIED spec ----
const baseBindings = sectionsOf(src, "r8-pin-diagnostics");
let baseOk = true;
for (const n of SECTIONS) {
  const installed = state.spec.sectionBindings[n].sha256, derived = baseBindings[n].sha256;
  const same = installed === derived;
  baseOk &&= same;
  console.log(`baseline ${n.padEnd(13)} ${same ? "MATCH" : "MISMATCH"} ${derived.slice(0, 16)}`);
}
const baseDigest = packetDigest(baseBindings);
console.log(`baseline packet   ${baseDigest === state.packet.digest ? "MATCH" : "MISMATCH"} ${baseDigest.slice(0, 16)} (installed ${state.packet.digest.slice(0, 16)})`);
if (!baseOk || baseDigest !== state.packet.digest) {
  console.error("REFUSING: derivation does not reproduce the installed bindings; candidate hashes would be untrustworthy.");
  process.exit(1);
}

// ---- apply deltas ----
let text = src;
const fails = [];
for (const [id, from, to] of OPS) {
  const n = text.split(from).length - 1;
  if (n !== 1) { fails.push(`${id}: ${n} match(es)`); continue; }
  text = text.replace(from, to);
}
console.log(`\ndeltas applied: ${OPS.length - fails.length}/${OPS.length}${fails.length ? ` FAILED -> ${fails.join(", ")}` : ""}`);

const before = src.split("\n"), after = text.split("\n");
const changed = [];
for (let i = 0; i < Math.max(before.length, after.length); i++) if (before[i] !== after[i]) changed.push(i + 1);
console.log(`changed lines: ${changed.join(",")} (count ${changed.length})`);

const candBindings = sectionsOf(text, REV);
console.log("\n=== r8-pin-diagnostics-narrow1 ===");
console.log("spec sha256        :", sha(text));
for (const n of SECTIONS) console.log(`section ${n.padEnd(13)}: ${candBindings[n].sha256}`);
console.log("packet digest      :", packetDigest(candBindings));

if (process.argv.includes("--write") && fails.length === 0) {
  writeFileSync(OUT, text);
  console.log(`\nwrote ${OUT} (${Buffer.byteLength(text)} bytes)`);
}
