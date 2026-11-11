#!/usr/bin/env bash
# Same-feature revision installation: structured-logging-pin-diagnostics
#   r8-pin-diagnostics -> r8-pin-diagnostics-narrow1
#
# `execution-state.mjs transition <feature>` cannot perform this bump: transitionFeature()
# returns ok/transitioned:false ("already current") when the target feature equals the
# current feature, so it would not install the draft. The revision bump therefore
# archives the outgoing canonical state and installs the validated draft directly,
# under assertions. Nothing is overwritten; the outgoing state is preserved first.
#
# usage: bash .scratch/structured-logging/install-narrow1-state.sh

set -euo pipefail

ROOT="/home/opencode/projects/default/mcp-bridge"
POLICY="/home/opencode/.omp/agent/workflow-policy"
FEATURE="structured-logging-pin-diagnostics"
REVISION="r8-pin-diagnostics-narrow1"
OUTGOING_REVISION="r8-pin-diagnostics"
DRAFT="$ROOT/.scratch/structured-logging/new-state-r8.json"
SPEC="$ROOT/.scratch/structured-logging/spec.md"
LIVE="$ROOT/.omp/plans/execution-state.json"
FEATURE_DIR="$ROOT/.omp/plans/$FEATURE"
EXPECTED_PACKET="15567c2bf834ec903608e884d4bcf0d56e76f00c1b5071ee8875600a51b31327"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"

cd "$ROOT"

# --- 0. assertions, including that the spec file on disk matches the draft's bindings ---
[ -f "$DRAFT" ] || { echo "FAIL: draft missing at $DRAFT"; exit 1; }
[ -f "$SPEC" ]  || { echo "FAIL: spec missing at $SPEC"; exit 1; }
[ -f "$LIVE" ]  || { echo "FAIL: live state missing at $LIVE"; exit 1; }

node --input-type=module -e '
import { readFileSync } from "node:fs";
const { canonicalJson, sha } = await import("/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs");
const [livePath, draftPath, specPath, feature, revision, outgoing, expectedPacket] = process.argv.slice(1);
const live = JSON.parse(readFileSync(livePath, "utf8"));
const draft = JSON.parse(readFileSync(draftPath, "utf8"));
const spec = readFileSync(specPath, "utf8");
const fail = (m) => { throw new Error(m); };

if (draft.feature !== feature) fail(`draft feature ${draft.feature} != ${feature}`);
if (draft.revision !== revision) fail(`draft revision ${draft.revision} != ${revision}`);
if (draft.packet.digest !== expectedPacket) fail(`draft packet digest ${draft.packet.digest} != authorized ${expectedPacket}`);
if (draft.authorization.packetDigest !== expectedPacket) fail("authorization binds a different packet digest");
if (draft.authorization.receipt.response.packetDigest !== expectedPacket) fail("receipt response binds a different packet digest");
if (live.feature !== feature) fail(`live feature ${live.feature} != ${feature}`);
if (live.revision !== outgoing) fail(`live revision ${live.revision} != ${outgoing}`);

// The installed spec file must reproduce the draft spec binding and every section hash.
if (sha(spec) !== draft.spec.sha256) fail(`spec.md sha256 ${sha(spec)} != state binding ${draft.spec.sha256}`);
let last = -1;
for (const name of ["requirements", "design", "tasks", "envelope"]) {
  const start = `<!-- specdd:${name}:start -->`, end = `<!-- specdd:${name}:end -->`;
  const a = spec.indexOf(start), b = spec.indexOf(end);
  if (!(a > last && a >= 0 && b > a && spec.indexOf(start, a + 1) === -1 && spec.indexOf(end, b + 1) === -1)) fail(`marker integrity failed for ${name}`);
  const got = sha(spec.slice(a + start.length, b));
  if (got !== draft.spec.sectionBindings[name].sha256) fail(`section ${name} hash ${got} != state binding ${draft.spec.sectionBindings[name].sha256}`);
  last = b + end.length - 1;
}
if (sha(canonicalJson({ feature: draft.feature, projectRoot: draft.projectRoot, sectionBindings: draft.spec.sectionBindings, branches: draft.packet.branches })) !== draft.packet.digest) fail("packet digest does not match the spec bindings on disk");

console.log(`assertions ok: ${live.feature}@${live.revision} -> ${draft.feature}@${draft.revision}`);
console.log(`draft: tasks=${draft.tasks.map(t => t.id).join(",")} acceptance=${draft.acceptance.length} writePaths=${draft.envelope.writePaths.length} reviews=${draft.reviews === null ? "null" : "set"}`);
' "$LIVE" "$DRAFT" "$SPEC" "$FEATURE" "$REVISION" "$OUTGOING_REVISION" "$EXPECTED_PACKET"

# --- 1. preserve the outgoing canonical state (never overwrite) ---------------
mkdir -p "$FEATURE_DIR"
if [ -f "$FEATURE_DIR/execution-state-prev-$STAMP.json" ]; then
  echo "FAIL: $FEATURE_DIR/execution-state-prev-$STAMP.json already exists; refusing to overwrite"
  exit 1
fi
cp "$LIVE" "$FEATURE_DIR/execution-state-prev-$STAMP.json"
echo "preserved outgoing -> $FEATURE_DIR/execution-state-prev-$STAMP.json"

# --- 2. install the validated draft ------------------------------------------
cp "$DRAFT" "$LIVE"
cp "$DRAFT" "$FEATURE_DIR/execution-state.json"
echo "installed $REVISION -> $LIVE (and $FEATURE_DIR/execution-state.json)"

# --- 3. verify ---------------------------------------------------------------
echo "--- canonical state ---"
node "$POLICY/execution-state.mjs" status "$LIVE" || true
echo "--- harness check ---"
node "$POLICY/execution-state.mjs" check "$LIVE" || true
echo "--- preserved states ---"
ls -l "$FEATURE_DIR"
echo "installation complete"
