# PR-planning-recovery-2: bounded bridge evidence

## Status
Read-only planner assignment complete; B2/B4 remain BLOCKED. This is evidence, not a design amendment or approval.

## Verified source facts

- `internal/service/tools.go:115-173`: `Published` returns no error, holds `ToolSurface.mu`, prepares changed descriptors before mutation, and logs/rejects invalid metadata. `:174-181` then removes/adds tools individually and updates the pin map.
- `cmd/mcp-bridge/serve.go:122-136,182-205`: storage callbacks are supplied to `NewLoop`; publication callback ordering is prompts, tools, then operating observer (`:201-205`). Tool publication rejection cannot propagate through this void callback. These lines do not establish runtime prepare/persist/accept order.
- `internal/service/tools.go:211-224,251-259`: registered binding is read under its mutex, then `runtime.Current` is separately read with pin fallback; dispatch later leases that assembly. Single-generation binding/lease retention is not established.
- `internal/service/server.go:8-13` and `cmd/mcp-bridge/serve.go:112-118`: one server is created and prompts/tools coexist on it. This proves current shared-server coexistence only.
- `internal/service/inbound.go:46-53,313-355`: inbound records hold token, last-seen, and diagnostic reference; admission reaps, constant-time checks fingerprints, refreshes accepted activity, and rejects absent/mismatched sessions.
- `internal/service/inbound.go:393,415-445`: delegation precedes response-issued session-ID association. `:455-459` deletes the auth record after delegated DELETE. `:545-558` lazy expiry uses strict elapsed `>` and a sweep throttle; neither removal path retires a tool-exposure owner.

## Inferences and unknowns

- Existing local preflight does not prove accepted-generation/list atomicity. Post-return session association does not prove association before response headers or avoid races. Per-session retirement remains unproved.
- Supplied SDK v1.8.0 facts S1-S3 establish seams only. They do not prove notification ordering, pagination/cursor semantics, factory invocation/reuse, or result ownership.
- Gemini client re-list/cache/approval behavior remains NOT VERIFIED; dynamic per-tool exposure is the owner's assumption.

## Remaining primary evidence

- B2: runtime/loop prepare-persist-accept ordering and lease retirement; SDK list/result/cursor behavior; a supported post-commit notification ordering mechanism.
- B4: SDK factory invocation/reuse; inbound provisional-owner association before first response commit/Flush through actual wrappers; prompt lifecycle on per-session servers; owner retirement and late-request exclusion.

## Tool evidence correction

The planner handoff says `xd://lsp` was unavailable. In the coordinator's current main session, `xd://lsp` succeeded: references for `ToolSurface.Published` returned `cmd/mcp-bridge/serve.go:203` and test references; hover for `(*mcp.Server).AddTool` returned the SDK v1.8.0 add/replace contract. This does not expand the planner's source conclusions or establish other LSP capabilities.

## Verification

No tests, builds, runtime probes, client probes, or implementation were run. Graphify was not used because the worktree is dirty. No approval or implementation authorization is recorded.
