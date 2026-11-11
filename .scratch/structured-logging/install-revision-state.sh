#!/usr/bin/env bash
# Same-feature revision installation: structured-logging-pin-diagnostics
#
# `execution-state.mjs transition <feature>` cannot perform a same-feature revision bump:
# transitionFeature() returns ok/transitioned:false ("already current") when the target feature equals
# the current feature. The bump therefore preserves the outgoing canonical state and installs the
# validated draft directly, under assertions.
#
# The narrowN spec text is applied in place by apply-delta.mjs --apply, which also preserves
# spec-<outgoing>.md; this script asserts that preservation (sha equality against the live binding)
# instead of performing it, and installs no spec bytes of its own.
#
# usage: bash install-revision-state.sh <revision> <outgoingRevision> <draftStatePath> <expectedPacketDigest>

set -euo pipefail

ROOT="/home/opencode/projects/default/mcp-bridge"
POLICY="/home/opencode/.omp/agent/workflow-policy"
FEATURE="structured-logging-pin-diagnostics"
REVISION="$1"
OUTGOING_REVISION="$2"
DRAFT="$3"
EXPECTED_PACKET="$4"
SC="$ROOT/.scratch/structured-logging"
SPEC="$SC/spec.md"
SPEC_PRESERVED="$SC/spec-$OUTGOING_REVISION.md"
LIVE="$ROOT/.omp/plans/execution-state.json"
FEATURE_DIR="$ROOT/.omp/plans/$FEATURE"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"

cd "$ROOT"

# --- 0. assertions: live state, draft and spec must agree on the authorized bytes -----------------
[ -f "$DRAFT" ]          || { echo "FAIL: draft missing at $DRAFT"; exit 1; }
[ -f "$SPEC" ]           || { echo "FAIL: spec missing at $SPEC"; exit 1; }
[ -f "$SPEC_PRESERVED" ] || { echo "FAIL: outgoing spec preservation $SPEC_PRESERVED missing"; exit 1; }
[ -f "$LIVE" ]           || { echo "FAIL: live state missing at $LIVE"; exit 1; }

node --input-type=module -e '
import { readFileSync } from "node:fs";
const { canonicalJson, sha } = await import("/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs");
const [livePath, draftPath, specPath, preservedPath, feature, revision, outgoing, expectedPacket] = process.argv.slice(1);
const live = JSON.parse(readFileSync(livePath, "utf8"));
const draft = JSON.parse(readFileSync(draftPath, "utf8"));
const spec = readFileSync(specPath, "utf8");
const preserved = readFileSync(preservedPath);
const fail = (m) => { throw new Error(m); };

if (draft.feature !== feature) fail(`draft feature ${draft.feature} != ${feature}`);
if (draft.revision !== revision) fail(`draft revision ${draft.revision} != ${revision}`);
if (draft.packet.digest !== expectedPacket) fail(`draft packet digest ${draft.packet.digest} != authorized ${expectedPacket}`);
if (draft.authorization.packetDigest !== expectedPacket) fail("authorization binds a different packet digest");
if (draft.authorization.receipt.response.packetDigest !== expectedPacket) fail("receipt response binds a different packet digest");
if (draft.authorization.receipt.response.origin !== "user" || draft.authorization.receipt.response.automatic === true) fail("authorization receipt is not an explicit user response");
if (live.feature !== feature) fail(`live feature ${live.feature} != ${feature}`);
if (live.revision !== outgoing) fail(`live revision ${live.revision} != ${outgoing}`);
if (draft.reviews !== null) fail("draft must carry reviews: null (nothing recorded yet)");

if (sha(preserved) !== live.spec.sha256) fail(`preserved outgoing spec sha256 ${sha(preserved)} != live binding ${live.spec.sha256}`);
if (sha(spec) !== draft.spec.sha256) fail(`spec sha256 ${sha(spec)} != state binding ${draft.spec.sha256}`);
let last = -1;
for (const name of ["requirements", "design", "tasks", "envelope"]) {
  const start = `<!-- specdd:${name}:start -->`, end = `<!-- specdd:${name}:end -->`;
  const a = spec.indexOf(start), b = spec.indexOf(end);
  if (!(a > last && a >= 0 && b > a && spec.indexOf(start, a + 1) === -1 && spec.indexOf(end, b + 1) === -1)) fail(`marker integrity failed for ${name}`);
  const got = sha(spec.slice(a + start.length, b));
  if (got !== draft.spec.sectionBindings[name].sha256) fail(`section ${name} hash ${got} != state binding ${draft.spec.sectionBindings[name].sha256}`);
  if (draft.spec.sectionBindings[name].revision !== revision) fail(`section ${name} binding revision != ${revision}`);
  last = b + end.length - 1;
}
if (sha(canonicalJson({ feature: draft.feature, projectRoot: draft.projectRoot, sectionBindings: draft.spec.sectionBindings, branches: draft.packet.branches })) !== draft.packet.digest) fail("packet digest does not match the candidate spec bindings");
if (draft.packet.branches[0].sectionBindings.requirements.revision !== revision) fail("branch bindings were not bumped to the new revision");

console.log(`assertions ok: ${live.feature}@${live.revision} -> ${draft.feature}@${draft.revision}`);
console.log(`draft: tasks=${draft.tasks.map(t => t.id).join(",")} acceptance=${draft.acceptance.length} writePaths=${draft.envelope.writePaths.length} reviews=${draft.reviews === null ? "null" : "set"}`);
console.log(`T10.blockedBy=${JSON.stringify(draft.tasks.find(t => t.id === "T10").blockedBy)} AC12-1.method=${draft.acceptance.find(a => a.id === "AC12-1").method} AC12-2.method=${draft.acceptance.find(a => a.id === "AC12-2").method}`);
' "$LIVE" "$DRAFT" "$SPEC" "$SPEC_PRESERVED" "$FEATURE" "$REVISION" "$OUTGOING_REVISION" "$EXPECTED_PACKET"

# --- 1. preserve the outgoing canonical state ----------------------------------------------------
mkdir -p "$FEATURE_DIR"
if [ -f "$FEATURE_DIR/execution-state-prev-$STAMP.json" ]; then
  echo "FAIL: $FEATURE_DIR/execution-state-prev-$STAMP.json already exists"
  exit 1
fi
cp "$LIVE" "$FEATURE_DIR/execution-state-prev-$STAMP.json"
echo "preserved outgoing state -> $FEATURE_DIR/execution-state-prev-$STAMP.json"

# --- 2. install the validated draft --------------------------------------------------------------
cp "$DRAFT" "$LIVE"
cp "$DRAFT" "$FEATURE_DIR/execution-state.json"
echo "installed $REVISION -> $LIVE (and $FEATURE_DIR/execution-state.json)"

# --- 3. verify -----------------------------------------------------------------------------------
echo "--- canonical state ---"
node "$POLICY/execution-state.mjs" status "$LIVE" || true
echo "--- harness check ---"
node "$POLICY/execution-state.mjs" check "$LIVE" || true
echo "installation complete"
