#!/usr/bin/env bash
# Owner-run. Two changes, both outside the coordinator's writable envelope:
#   1. revert the uncommitted execution-gate.ts settlement patch  -> guard + its test restored
#   2. declare `blocking: true` on the two reviewer agents        -> reviews settle inline
#
#   bash .scratch/structured-logging/apply-inline-reviewer-mode.sh
set -euo pipefail
REPO=/home/opencode/.omp
POLICY="$REPO/agent/workflow-policy"
AGENTS="$REPO/agent/agents"

echo "=== 1. settlement guard ==="
if git -C "$REPO" diff --quiet -- agent/workflow-policy/execution-gate.ts; then
  echo "gate already matches the committed revision - nothing to revert"
else
  echo "--- current uncommitted change being reverted ---"
  git -C "$REPO" diff -- agent/workflow-policy/execution-gate.ts | sed -n '1,20p'
  git -C "$REPO" checkout -- agent/workflow-policy/execution-gate.ts
  echo "reverted: settlement guard restored to the committed revision"
fi
grep -n "finishReview(binding, active" "$POLICY/execution-gate.ts" | head -4

echo
echo "=== 2. inline reviewer flow ==="
node -e '
const fs = require("fs");
for (const f of ["/home/opencode/.omp/agent/agents/code-reviewer.md", "/home/opencode/.omp/agent/agents/critic.md"]) {
  const s = fs.readFileSync(f, "utf8");
  if (/^blocking:\s*true\s*$/m.test(s)) { console.log("already declared:", f); continue; }
  const out = s.replace(/^(name:.*)$/m, "$1\nblocking: true");
  if (out === s) throw new Error("frontmatter anchor not found in " + f);
  fs.writeFileSync(f, out);
  console.log("added blocking: true to", f);
}
'
echo "--- frontmatter now ---"
for f in "$AGENTS/code-reviewer.md" "$AGENTS/critic.md"; do echo "== $f"; sed -n '1,9p' "$f"; done

echo
echo "=== 3. gate diff after revert (must be empty) ==="
git -C "$REPO" diff --stat -- agent/workflow-policy/execution-gate.ts || true

echo
echo "=== 4. harness settlement suite (proof) ==="
cd "$POLICY"
node --test execution-review-sequence.test.mjs 2>&1 | grep -E "^# (tests|pass|fail)|^✖ " | head -10
echo "(one pre-existing failure may remain: 'policy documents load with valid frontmatter and consistent CLI paths' - it also fails untouched at HEAD and is unrelated to reviews)"
echo "done"
