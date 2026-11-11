// Owner-run recorder for the r8 review verdicts. It does NOT ask you what the verdict
// was: it reads the reviewer's own report and refuses anything but a real pass bound to
// the installed packet digest. Dry run by default; add --apply to write.
//
//   node .scratch/structured-logging/record-review-r8.mjs specification <report.md> [--apply]
//   node .scratch/structured-logging/record-review-r8.mjs tasks         <report.md> [--apply]
//
// Requirements on the report file (the reviewer must state both):
//   REVIEWER: code-reviewer      (or critic, matching the stage)
//   PACKET: <64 hex chars>       (must equal the installed state's packet digest)
//   VERDICT: pass                (anything else aborts with no write)
import { readFileSync, writeFileSync, copyFileSync } from "node:fs";
import { createHash } from "node:crypto";

const ROOT = "/home/opencode/projects/default/mcp-bridge";
const SC = `${ROOT}/.scratch/structured-logging`;
const FEATURE = "structured-logging-pin-diagnostics";
const statePath = process.env.R8_STATE ?? `${ROOT}/.omp/plans/execution-state.json`;
const sha = (s) => createHash("sha256").update(s).digest("hex");

const [stage, reportPath, ...rest] = process.argv.slice(2);
const apply = rest.includes("--apply");
if (!["specification", "tasks"].includes(stage) || !reportPath) {
  console.error("usage: record-review-r8.mjs <specification|tasks> <reportPath> [--apply]");
  process.exit(2);
}
const expectedReviewer = stage === "specification" ? "code-reviewer" : "critic";

const state = JSON.parse(readFileSync(statePath, "utf8"));
if (state.feature !== FEATURE) throw new Error(`live state feature is "${state.feature}", expected "${FEATURE}"`);

const report = readFileSync(reportPath, "utf8");
const grab = (re, label) => {
  const m = report.match(re);
  if (!m) throw new Error(`report ${reportPath} does not state ${label}`);
  return m[1].trim();
};
const reviewer = grab(/^REVIEWER:\s*(\S+)\s*$/im, "REVIEWER:");
const packet = grab(/^PACKET:\s*([0-9a-f]{64})\s*$/im, "PACKET:");
const verdict = grab(/^VERDICT:\s*(\S+)\s*$/im, "VERDICT:");

if (reviewer !== expectedReviewer) throw new Error(`report is by "${reviewer}" but stage ${stage} requires "${expectedReviewer}"`);
if (packet !== state.packet.digest) throw new Error(`report binds packet ${packet} but the installed state is ${state.packet.digest}`);
if (verdict !== "pass") {
  console.error(`refusing to record: the report's verdict is "${verdict}", not "pass". Nothing written.`);
  process.exit(1);
}

const sourceId = `src-review-${stage}-r8`;
const summary = { stage, reviewer, packetDigest: packet, verdict: `"${verdict}"`, sourceId, reportSha256: sha(report), apply };
console.log(JSON.stringify(summary, null, 2));
if (!apply) {
  console.log("dry run: add --apply to write the source entry and the review record");
  process.exit(0);
}

const recordedAt = new Date().toISOString().replace(/\.\d{3}Z$/, ".000Z");
const kind = (state.sources.find((s) => s.id === "src-review-signoff") ?? {}).kind ?? "observation";
const sources = state.sources.filter((s) => s.id !== sourceId);
sources.push({
  id: sourceId,
  kind,
  artifact: { path: reportPath, sha256: sha(report) },
  locator: `independent ${expectedReviewer} report for spec revision ${state.revision}`,
  recordedAt,
});

const next = {
  ...state,
  sources,
  reviews: {
    ...state.reviews,
    [stage]: {
      verdict: "pass",
      reviewerId: expectedReviewer,
      packetDigest: packet,
      receipt: { sourceId, recordedAt, response: { origin: "reviewer", verdict: "pass" } },
    },
  },
};

copyFileSync(statePath, `${SC}/execution-state-before-review-${stage}.json`);
writeFileSync(statePath, `${JSON.stringify(next, null, 2)}\n`);
console.log(`wrote reviews.${stage} = pass (${expectedReviewer}) into ${statePath}`);
console.log(`previous state saved to ${SC}/execution-state-before-review-${stage}.json`);
