#!/usr/bin/env bash
# Owner-run installation of the validated r8 projection.
#
#   bash .scratch/structured-logging/install-r8-state.sh
#
# What it does, in order:
#   0. asserts the live state is the r7 `structured-logging` state and that the draft's
#      packet digest equals the digest the owner authorized;
#   1. preserves the older archive snapshot that already sits in the archive directory;
#   2. stages the validated draft where the sanctioned installer expects it;
#   3. runs the sanctioned installer, which archives the outgoing r7 state and installs
#      the target state as authored;
#   4. prints the canonical state's status and the harness check result.
#
# The coordinator cannot perform steps 1-3 itself: role rules refuse its writes to
# project repository paths, and delegates cannot be dispatched while the state is invalid.

set -euo pipefail

ROOT="/home/opencode/projects/default/mcp-bridge"
POLICY="/home/opencode/.omp/agent/workflow-policy"
FEATURE="structured-logging-pin-diagnostics"
DRAFT="$ROOT/.scratch/structured-logging/new-state-r8.json"
LIVE="$ROOT/.omp/plans/execution-state.json"
ARCHIVE_DIR="$ROOT/.omp/plans/structured-logging"
TARGET_DIR="$ROOT/.omp/plans/$FEATURE"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
EXPECTED_PACKET="15567c2bf834ec903608e884d4bcf0d56e76f00c1b5071ee8875600a51b31327"

cd "$ROOT"

# --- 0. assertions -----------------------------------------------------------
[ -f "$DRAFT" ] || { echo "FAIL: draft missing at $DRAFT (run: node .scratch/structured-logging/build-r8-state.mjs)"; exit 1; }
[ -f "$LIVE" ]  || { echo "FAIL: live state missing at $LIVE"; exit 1; }

node -e '
const fs = require("fs");
const [livePath, draftPath, feature, expectedPacket] = process.argv.slice(1);
const live = JSON.parse(fs.readFileSync(livePath, "utf8"));
const draft = JSON.parse(fs.readFileSync(draftPath, "utf8"));
if (!["structured-logging", "structured-logging-pin-diagnostics"].includes(live.feature)) throw new Error(`live state feature is "${live.feature}", expected the r7 "structured-logging" or the installed r8 "structured-logging-pin-diagnostics" state`);
if (live.feature === "structured-logging-pin-diagnostics" && live.revision !== "r8-pin-diagnostics") throw new Error(`live r8 state revision is "${live.revision}", expected "r8-pin-diagnostics"`);
if (draft.feature !== feature) throw new Error(`draft feature is "${draft.feature}", expected "${feature}"`);
if (draft.packet.digest !== expectedPacket) throw new Error(`draft packet digest ${draft.packet.digest} differs from the authorized ${expectedPacket}`);
if (draft.authorization.packetDigest !== expectedPacket) throw new Error(`authorization binds ${draft.authorization.packetDigest}, expected ${expectedPacket}`);
if (draft.authorization.receipt.response.packetDigest !== expectedPacket) throw new Error("receipt response binds a different packet digest");
console.log(`assertions ok: live=${live.feature}@${live.revision} -> draft=${draft.feature}@${draft.revision}`);
console.log(`draft: tasks=${draft.tasks.map(t => t.id).join(",")} acceptance=${draft.acceptance.length} evidence=${draft.evidence.length}`);
' "$LIVE" "$DRAFT" "$FEATURE" "$EXPECTED_PACKET"

# --- 1. preserve the older archive snapshot ---------------------------------
mkdir -p "$ARCHIVE_DIR"
if [ -f "$ARCHIVE_DIR/execution-state.json" ]; then
  if [ -f "$ARCHIVE_DIR/execution-state-prev-$STAMP.json" ]; then
    echo "FAIL: $ARCHIVE_DIR/execution-state-prev-$STAMP.json already exists; refusing to overwrite"
    exit 1
  fi
  cp "$ARCHIVE_DIR/execution-state.json" "$ARCHIVE_DIR/execution-state-prev-$STAMP.json"
  echo "preserved previous archive -> $ARCHIVE_DIR/execution-state-prev-$STAMP.json"
fi

# --- 2. stage the validated target state ------------------------------------
mkdir -p "$TARGET_DIR"
cp "$DRAFT" "$TARGET_DIR/execution-state.json"
echo "staged target -> $TARGET_DIR/execution-state.json"

# --- 3. sanctioned install (archives outgoing, installs target) -------------
node "$POLICY/execution-state.mjs" transition "$FEATURE" "$ROOT"

# --- 4. verify --------------------------------------------------------------
echo "--- canonical state ---"
node "$POLICY/execution-state.mjs" status "$LIVE" || true
echo "--- harness check ---"
node "$POLICY/execution-state.mjs" check "$LIVE" || true
echo "--- archive directory ---"
ls -l "$ARCHIVE_DIR"
echo "installation complete"
