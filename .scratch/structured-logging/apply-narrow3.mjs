#!/usr/bin/env node
// AUTHORITY: owner authorized revision r8-pin-diagnostics-narrow3 on 2026-11-11 to close
// code-reviewer REQUEST CHANGES packet 8486fd04268f00b1f370600c5bedb10d7e8987794610d34ce2abaa74e7bb43a4
// residuals R1-R6, NARROWING ONLY: no R/AC/T id added or removed, no new AD, no new acceptance
// criterion, no widened envelope. The planner authored the delta
// (r8-narrowing3-delta.md) and edited nothing.
//
// This applier parses the delta itself - the replacement pairs and the append payloads are never
// retyped here - and refuses to write unless every guard holds. Dry run by default.
//
//   node .scratch/structured-logging/apply-narrow3.mjs           # dry run
//   node .scratch/structured-logging/apply-narrow3.mjs --apply   # write

import { readFileSync, writeFileSync, copyFileSync, readdirSync, existsSync } from "node:fs";
import { createHash } from "node:crypto";

const ROOT = "/home/opencode/projects/default/mcp-bridge";
const SC = `${ROOT}/.scratch/structured-logging`;
const APPLY = process.argv.includes("--apply");
const sha = (v) => createHash("sha256").update(v).digest("hex");

// Authority: exact bytes this plan was authored against.
const ORIGINAL = {
  "spec.md": "925246d9ce4231051c8ce63169b118e12b3da89451b8bfd514603f262a1d03a9",
  "issues/09-pin-blocker-detail.md": "08b98659dbdc68194522122eba51430c19b4f4e299d6561a8bda80d5b5ae5209",
  "issues/10-discovery-attribution-and-readiness-reason.md": "6a5b3c9c87783febcff3b63da0ec1ced1da1b75a1812091fab214f3a093f1b5b",
  "issues/11-path-parity-and-logger-fallback.md": "f982da9b65ccef3025b0fa89bc7c60b415f7a43565719cc51857696f023b6fca",
  "issues/17-r8-artifact-consistency-drift.md": "eda3565a9826e2347e22cb726e235070ff2595aee4cb4f1c6a07c4ece6fee5bc",
  "backlog.md": "e3400e2f948aed5182af6e90aa1d8d29819d1e0c91046f4d7949793789e61da4",
  "decisions.md": "c5d9b2d8b39f9f534f1fad2362cb78562f253f59e591886cba0ce2121333fe99",
};
const REV_FROM = "r8-pin-diagnostics-narrow2";
const REV_TO = "r8-pin-diagnostics-narrow3";

// ---- read the delta ---------------------------------------------------------
const delta = readFileSync(`${SC}/r8-narrowing3-delta.md`, "utf8");
const dlines = delta.split("\n");
const at = (re) => dlines.findIndex((l) => re.test(l));
const s1 = at(/^════ 1\./), s2 = at(/^════ 2\./), s3 = at(/^════ 3\./), s4 = at(/^════ 4\./), s5 = at(/^════ 5\./);
if (s1 < 0 || s2 < 0 || s3 < 0 || s4 < 0 || s5 < 0) throw new Error("delta sections not found");

// ---- parse the replacement pairs (sections 1-2) -----------------------------
const targetFor = (hint) => {
  const m = hint.match(/^(spec\.md|backlog\.md|decisions\.md|issues\/\d+)/);
  if (!m) throw new Error(`cannot resolve target from "${hint}"`);
  const key = m[1];
  if (!key.startsWith("issues/")) return key;
  const prefix = key.slice("issues/".length);
  const hit = readdirSync(`${SC}/issues`).filter((f) => f.startsWith(`${prefix}-`));
  if (hit.length !== 1) throw new Error(`issues/${prefix} matched ${hit.length} files`);
  return `issues/${hit[0]}`;
};

const pairs = [];
for (let i = s1; i < s3; i += 1) {
  const h = dlines[i].match(/^([A-Za-z0-9-]+) · (.+)$/) ?? dlines[i].match(/^((?:issues\/)?[\w.-]+\.md:\d+) \(/);
  if (!h) continue;
  const file = targetFor(h[2] ?? h[1]);
  const ci = dlines.findIndex((l, j) => j > i && j < s3 && l === "CURRENT:");
  const ri = dlines.findIndex((l, j) => j > ci && j < s3 && l === "REPLACEMENT:");
  const current = dlines[ci + 1];
  const replacement = dlines[ri + 1];
  if (!current || !replacement || current === "REPLACEMENT:") throw new Error(`empty block after ${h[0]}`);
  pairs.push({ label: h[0], hint: h[2], file, current, replacement });
}
if (pairs.length !== 11) throw new Error(`expected 11 replacement pairs, parsed ${pairs.length}`);

// ---- parse the append payloads (sections 3-4) -------------------------------
const backlogRow = dlines.slice(s3, s4).find((l) => l.startsWith("| 19 reload-leg runtime parity |"));
const i17 = dlines.findIndex((l, j) => j > s3 && j < s4 && l.startsWith("## narrow3 additions (2026-11-11)"));
const i17end = dlines.findIndex((l, j) => j > i17 && j < s4 && l.trim() === "");
const issues17Block = dlines.slice(i17, i17end).join("\n");
const decisionItem = dlines.slice(s4, s5).find((l) => l.startsWith("4. **narrow3 authorized.**"));
if (!backlogRow || i17 < 0 || !issues17Block || !decisionItem) throw new Error("append payloads not found");

// ---- load targets, assert they are the authorized bytes ---------------------
const files = [...new Set(["spec.md", ...pairs.map((p) => p.file), "issues/17-r8-artifact-consistency-drift.md", "backlog.md", "decisions.md"])];
const text = {};
for (const f of files) {
  const p = `${SC}/${f}`;
  if (!existsSync(p)) throw new Error(`missing ${f}`);
  text[f] = readFileSync(p, "utf8");
  if (sha(text[f]) !== ORIGINAL[f]) throw new Error(`${f} is not the authorized revision (sha ${sha(text[f])})`);
}
if (!text["spec.md"].split("\n")[0].includes(REV_FROM)) throw new Error("spec header is not the revision this plan narrows");

// ---- guards -----------------------------------------------------------------
const countIds = (s, re) => new Set([...s.matchAll(re)].map((m) => m[0])).size;
const idsOf = (s) => ({
  R: countIds(s, /R\d+/g), AC: countIds(s, /AC\d\d-\d+/g), T: countIds(s, /T\d+/g), AD: countIds(s, /AD\d+/g),
  specdd: (s.match(/specdd:/g) || []).length,
});
const before = idsOf(text["spec.md"]);
const next = { ...text };

const edits = [];
for (const p of pairs) {
  if (/specdd:/.test(p.current) || /specdd:/.test(p.replacement)) throw new Error(`${p.label} touches a specdd marker line`);
  const n = next[p.file].split(p.current).length - 1;
  if (n !== 1) throw new Error(`${p.label}: current text occurs ${n} times in ${p.file}`);
  next[p.file] = next[p.file].replace(p.current, p.replacement);
  edits.push(`${p.label.padEnd(7)} ${p.file.padEnd(48)} ${p.current.length}->${p.replacement.length} chars`);
}

const appendAfter = (file, anchor, payload) => {
  const lines = next[file].split("\n");
  const n = lines.filter((l) => l === anchor).length;
  if (n !== 1) throw new Error(`append anchor occurs ${n} times in ${file}`);
  lines.splice(lines.indexOf(anchor) + 1, 0, payload);
  next[file] = lines.join("\n");
  edits.push(`append  ${file.padEnd(48)} after "${anchor.slice(0, 40)}…"`);
};
appendAfter("backlog.md", "| 18 owner ruling: misleading-counter gap | grilling | Accept the counter gap (on what terms) or keep it pending? |", backlogRow);
const decAnchor = next["decisions.md"].split("\n").find((l) => l.startsWith("3. **Ruling 18 startup-parity claim SUPERSEDED"));
appendAfter("decisions.md", decAnchor, decisionItem);

const i17lines = next["issues/17-r8-artifact-consistency-drift.md"].replace(/\n*$/, "\n").split("\n");
i17lines.push(...issues17Block.split("\n"), "");
next["issues/17-r8-artifact-consistency-drift.md"] = i17lines.join("\n");
edits.push(`append  ${"issues/17-r8-artifact-consistency-drift.md".padEnd(48)} section "## narrow3 additions (2026-11-11)"`);

const after = idsOf(next["spec.md"]);
for (const k of ["R", "AC", "T", "AD", "specdd"]) {
  if (before[k] !== after[k]) throw new Error(`spec.md ${k} count changed ${before[k]} -> ${after[k]}`);
}
if (!next["spec.md"].includes(`${REV_TO}`)) throw new Error("revision bump missing");

console.log("");
console.log(edits.join("\n"));
console.log(`\nids unchanged: R ${after.R} AC ${after.AC} T ${after.T} AD ${after.AD} | specdd markers ${after.specdd}`);
console.log(APPLY ? "\nAPPLYING" : "\ndry run: add --apply to write");

if (APPLY) {
  copyFileSync(`${SC}/spec.md`, `${SC}/spec-r8-pin-diagnostics-narrow2.md`);
  for (const f of files) if (next[f] !== text[f]) writeFileSync(`${SC}/${f}`, next[f]);
  console.log(`wrote ${files.length} files; spec-r8-pin-diagnostics-narrow2.md preserved`);
  console.log(`spec.md sha256 now ${sha(readFileSync(`${SC}/spec.md`))}`);
}
