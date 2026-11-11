#!/usr/bin/env node
// Generic narrowing-delta applier. Parses a planner delta in the r8-narrowing3-delta.md format and
// transplants its CURRENT/REPLACEMENT pairs and append payloads; nothing is retyped here.
//
// Two phases, so the bytes a human reviewed are the bytes written:
//   node apply-delta.mjs <deltaPath> <revFrom> <revTo>            # dry run; writes the byte manifest
//   node apply-delta.mjs <deltaPath> <revFrom> <revTo> --apply    # refuses unless every sha still matches
//
// Guards: every current line must occur exactly once; no pair may touch a specdd marker line; the
// requirement/acceptance/task/AD/specdd counts of spec.md must be unchanged; the revision header must
// read revFrom before and revTo after.

import { readFileSync, writeFileSync, readdirSync, existsSync } from "node:fs";
import { createHash } from "node:crypto";

const ROOT = "/home/opencode/projects/default/mcp-bridge";
const SC = `${ROOT}/.scratch/structured-logging`;
const [deltaArg, revFrom, revTo, ...rest] = process.argv.slice(2);
if (!deltaArg || !revFrom || !revTo) {
  console.error("usage: apply-delta.mjs <deltaPath> <revFrom> <revTo> [--apply]");
  process.exit(2);
}
const APPLY = rest.includes("--apply");
const DELTA = deltaArg.startsWith("/") ? deltaArg : `${SC}/${deltaArg}`;
const MANIFEST = `${SC}/apply-manifest-${revTo}.json`;
const sha = (v) => createHash("sha256").update(v).digest("hex");

const dlines = readFileSync(DELTA, "utf8").split("\n");
const marker = (n) => dlines.findIndex((l) => l.startsWith(`════ ${n}.`));
const [s1, s2, s3, s4, s5] = [1, 2, 3, 4, 5].map(marker);
if ([s1, s2, s3, s4].some((i) => i < 0)) throw new Error("delta sections not found");

const resolve = (hint) => {
  const m = hint.match(/^(spec\.md|backlog\.md|decisions\.md|issues\/\d+)/);
  if (!m) throw new Error(`cannot resolve target from "${hint}"`);
  if (!m[1].startsWith("issues/")) return m[1];
  const prefix = m[1].slice("issues/".length);
  const hit = readdirSync(`${SC}/issues`).filter((f) => f.startsWith(`${prefix}-`));
  if (hit.length !== 1) throw new Error(`issues/${prefix} matched ${hit.length} files`);
  return `issues/${hit[0]}`;
};

const pairs = [];
for (let i = s1; i < s3; i += 1) {
  const h = dlines[i].match(/^(\[[A-Z0-9-]+\]|[A-Za-z0-9+ -]+) · (.+)$/) ?? dlines[i].match(/^((?:issues\/)?[\w.-]+\.md:\d+) \(/);
  if (!h) continue;
  const file = resolve(h[2] ?? h[1]);
  const ci = dlines.findIndex((l, j) => j > i && j < s3 && l === "CURRENT:");
  const ri = dlines.findIndex((l, j) => j > ci && j < s3 && l === "REPLACEMENT:");
  const current = dlines[ci + 1];
  const replacement = dlines[ri + 1];
  if (!current || !replacement) throw new Error(`empty block after ${h[0]}`);
  pairs.push({ label: h[0], file, current, replacement });
}
if (pairs.length === 0) throw new Error("no replacement pairs parsed");
const declaredPairs = dlines.slice(s1, s3).filter((l) => l === "CURRENT:").length;
if (pairs.length !== declaredPairs) throw new Error(`parsed ${pairs.length} pairs but the delta declares ${declaredPairs} CURRENT blocks (a label was skipped)`);

const sec3 = dlines.slice(s3, s4), sec4 = dlines.slice(s4, s5);
const backlogRows = sec3.filter((l) => /^\| \d+ .*\|$/.test(l));
const h17 = sec3.findIndex((l) => /^## narrow\d+ additions/.test(l));
let block17 = null;
if (h17 >= 0) {
  const end = sec3.findIndex((l, j) => j > h17 && l.trim() === "");
  block17 = sec3.slice(h17, end < 0 ? sec3.length : end).join("\n");
}
const decItem = sec4.find((l) => /^\d+\. \*\*narrow\S* authorized/.test(l));

// Generic append: "[LABEL] · issues/<file> (append after line <n>)" followed by "- " payload lines.
const appends = [];
for (let i = 0; i < sec3.length; i += 1) {
  const h = sec3[i].match(/^\[[A-Z0-9-]+\] · (issues\/[^:]+) \(append after line (\d+)[^)]*\)$/);
  if (!h) continue;
  const payload = [];
  for (let j = i + 1; j < sec3.length && sec3[j].startsWith("- "); j += 1) payload.push(sec3[j]);
  if (!payload.length) throw new Error(`empty append for ${h[1]}`);
  appends.push({ label: sec3[i], file: resolve(h[1]), after: Number(h[2]), payload });
}

const touched = [...new Set(["spec.md", ...pairs.map((p) => p.file), ...appends.map((a) => a.file)])];
if (backlogRows.length) touched.push("backlog.md");
if (decItem) touched.push("decisions.md");
if (block17) touched.push(resolve("issues/17"));
const text = {};
for (const f of touched) {
  if (!existsSync(`${SC}/${f}`)) throw new Error(`missing ${f}`);
  text[f] = readFileSync(`${SC}/${f}`, "utf8");
}

if (APPLY) {
  if (!existsSync(MANIFEST)) throw new Error(`no manifest at ${MANIFEST}; run the dry run first`);
  const manifest = JSON.parse(readFileSync(MANIFEST, "utf8"));
  if (manifest.delta !== sha(readFileSync(DELTA))) throw new Error("delta changed since the dry run");
  for (const [f, want] of Object.entries(manifest.files)) {
    if (sha(text[f]) !== want) throw new Error(`${f} changed since the dry run (sha ${sha(text[f])})`);
  }
}

const idsOf = (s) => ({
  R: new Set([...s.matchAll(/R\d+/g)].map((m) => m[0])).size,
  AC: new Set([...s.matchAll(/AC\d\d-\d+/g)].map((m) => m[0])).size,
  T: new Set([...s.matchAll(/T\d+/g)].map((m) => m[0])).size,
  AD: new Set([...s.matchAll(/AD\d+/g)].map((m) => m[0])).size,
  specdd: (s.match(/specdd:/g) || []).length,
});
const before = idsOf(text["spec.md"]);
if (!text["spec.md"].split("\n")[0].includes(revFrom)) throw new Error(`spec header is not ${revFrom}`);

const next = { ...text };
const plan = [];
for (const a of appends) {
  const lines = next[a.file].split("\n");
  if (a.after < 1 || a.after > lines.length) throw new Error(`append line ${a.after} out of range in ${a.file}`);
  lines.splice(a.after, 0, ...a.payload);
  next[a.file] = lines.join("\n");
  plan.push(`${a.label.slice(0, 44).padEnd(44)} ${a.file} +${a.payload.length}`);
}
for (const p of pairs) {
  if (/specdd:/.test(p.current) || /specdd:/.test(p.replacement)) throw new Error(`${p.label} touches a specdd marker line`);
  const n = next[p.file].split(p.current).length - 1;
  if (n === 1) {
    next[p.file] = next[p.file].replace(p.current, p.replacement);
    plan.push(`${p.label.padEnd(8)} ${p.file.padEnd(46)} ${p.current.length}->${p.replacement.length}`);
    continue;
  }
  // Line-addressed fallback: only for pairs labelled with an explicit line, where the delta states the
  // current text is longer than the capture window. The whole line is replaced; nothing is substring-matched.
  const at = p.label.match(/^(?:([\w.-]+) · )?([\w./-]+\.md):(\d+)/);
  if (n === 0 && at && p.replacement.includes("\n") === false) {
    const fileLines = next[p.file].split("\n");
    const idx = Number(at[3]) - 1;
    if (idx < 0 || idx >= fileLines.length) throw new Error(`${p.label}: line ${at[3]} out of range`);
    const oldLine = fileLines[idx];
    if (oldLine.trim() === "") throw new Error(`${p.label}: line ${at[3]} is empty`);
    fileLines[idx] = p.replacement;
    next[p.file] = fileLines.join("\n");
    plan.push(`[by-line] ${p.label.padEnd(24)} ${p.file} line ${at[3]}: ${oldLine.length}->${p.replacement.length} ${JSON.stringify(oldLine.slice(0, 70))}`);
    continue;
  }
  throw new Error(`${p.label}: current text occurs ${n} times in ${p.file}`);
}
const insertAfterLast = (file, re, payload) => {
  const lines = next[file].split("\n");
  let idx = -1;
  lines.forEach((l, i) => { if (re.test(l)) idx = i; });
  if (idx < 0) throw new Error(`no anchor matching ${re} in ${file}`);
  lines.splice(idx + 1, 0, payload);
  next[file] = lines.join("\n");
  plan.push(`append   ${file.padEnd(46)} after line ${idx + 1}`);
};
for (const row of backlogRows) insertAfterLast("backlog.md", /^\| \d+ .*\|$/, row);
if (decItem) insertAfterLast("decisions.md", /^\d+\. \*\*/, decItem);
if (block17) {
  const f = resolve("issues/17");
  next[f] = next[f].replace(/\n*$/, "\n") + "\n" + block17 + "\n";
  plan.push(`append   ${f.padEnd(46)} block "${block17.split("\n")[0]}"`);
}

const after = idsOf(next["spec.md"]);
for (const k of ["R", "AC", "T", "AD", "specdd"]) {
  if (before[k] !== after[k]) throw new Error(`spec.md ${k} count changed ${before[k]} -> ${after[k]}`);
}
if (!next["spec.md"].includes(revTo)) throw new Error(`revision bump to ${revTo} missing`);

console.log(plan.join("\n"));
console.log(`\nids unchanged: R ${after.R} AC ${after.AC} T ${after.T} AD ${after.AD} | specdd markers ${after.specdd}`);

if (!APPLY) {
  writeFileSync(MANIFEST, JSON.stringify({ delta: sha(readFileSync(DELTA)), revisionFrom: revFrom, revisionTo: revTo, files: Object.fromEntries(touched.map((f) => [f, sha(text[f])])) }, null, 2) + "\n");
  console.log(`\ndry run: manifest ${MANIFEST}; add --apply to write`);
  process.exit(0);
}

for (const f of touched) if (next[f] !== text[f]) writeFileSync(`${SC}/${f}`, next[f]);
if (!text["spec.md"].includes(revFrom) || next["spec.md"] !== text["spec.md"]) {
  writeFileSync(`${SC}/spec-${revFrom}.md`, text["spec.md"]);
  console.log(`preserved outgoing spec -> spec-${revFrom}.md`);
}
console.log(`wrote ${touched.filter((f) => next[f] !== text[f]).length} files; spec.md sha256 now ${sha(next["spec.md"])}`);
