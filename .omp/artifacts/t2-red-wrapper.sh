#!/usr/bin/env bash
# Reproduces T2 expected-RED on a clean HEAD worktree with the current test files.
set +e
BASE=/home/opencode/projects/default/mcp-bridge
WT=/tmp/mcp-t2-red
rm -rf "$WT"
git -C "$BASE" worktree add --detach "$WT" HEAD >/dev/null 2>&1 || { echo "worktree add failed"; exit 1; }
cp "$BASE/internal/upstream/header_params_test.go" "$WT/internal/upstream/"
cp "$BASE/cmd/mcp-bridge/serve_header_params_test.go" "$WT/cmd/mcp-bridge/"
out1=$(cd "$WT" && go test ./internal/upstream -run '^(TestDedicatedParamHeadersReachPeerInStatelessMode|TestBodyOnlyControlAddsNoExtraListCall|TestStatefulParamHeadersReuseDiscoveryCache|TestParamHeaderFailurePathsUseSafeCodes)$' -count=1 2>&1); c1=$?
out2=$(cd "$WT" && go test ./cmd/mcp-bridge -run '^(TestParameterHeaderTransitWireFacts|TestParameterHeaderStatefulControlIsAchievable|TestParameterHeaderRestartSurvivesInStatelessMode|TestLegacyParamHeaderCallFailsWithNamedCode)$' -count=1 2>&1); c2=$?
git -C "$BASE" worktree remove --force "$WT" >/dev/null 2>&1
printf '== pre-change baseline (HEAD checkout + current tests): upstream oracle, exit=%s ==\n%s\n' "$c1" "$out1"
printf '== pre-change baseline (HEAD checkout + current tests): bridge oracle, exit=%s ==\n%s\n' "$c2" "$out2"
if [ "$c1" -eq 1 ] && [ "$c2" -eq 1 ] && printf '%s' "$out1" | grep -q 'FAIL: TestDedicatedParamHeadersReachPeerInStatelessMode' && ! printf '%s' "$out1" | grep -q 'FAIL: TestBodyOnlyControlAddsNoExtraListCall' && printf '%s' "$out2" | grep -q 'FAIL: TestParameterHeaderRestartSurvivesInStatelessMode'; then
  echo "expected-RED verified: header-bound and restart oracles fail as assertions; body-only control passes"
  exit 0
fi
echo "expected-RED check failed"
exit 1
