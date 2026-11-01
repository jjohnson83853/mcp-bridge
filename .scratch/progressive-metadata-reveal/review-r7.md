# Independent requirements/design review r7

Native reviewer: `ReviewSessionReconciliation` (critic; distinct from planner).
Reviewed canonical SHA-256: `30b0738e55711d634101f8b32eec89d1d2a84101bdb34d70fa40109172118e6b`.
Packet digest: `7464742031b90bbe1b4770027694d1b5a3e2d04c2880990a7d141b32e1e82544`.
No task-ledger verdict or owner approval.

**Verdict: PASS** (technical/design consistency only; no implementation approval)

**Reviewed:** `.scratch/progressive-metadata-reveal/spec.md`, revision `cursor-choice-reconciliation-7-session-admission`, SHA256 `30b0738e55711d634101f8b32eec89d1d2a84101bdb34d70fa40109172118e6b`.

**Scope/limits:** Requirements and design only; deferred task ledger not reviewed. Read-only. Three document probes; no source reopening, build, tests, MCP/LSP/Graphify calls or client probes. Supplied E3/E2p facts treated as coordinator-verified, not runtime proof.

**Blocking findings:** None.

**Verified design/contract checks**
- **Header trust/request isolation (§4.9):** Dedicated `X-Mcp-Bridge-Exposure-Epoch` is stripped case-insensitively from the cloned Header on every delegated route and stamped only after ordinary HTTP auth/session admission; middleware compares actual SDK `Session.ID()` plus epoch against the live admitted-record incarnation. Header claims alone are insufficient. AC9 explicitly covers stripping, duplicate/mixed-case copies, forged carriers, missing/malformed carriers and no HTTP-context-propagation assumption. Consistent with supplied E3/E2p seams; no amendment needed.
- **Lifecycle/retirement ordering (§4.9):** Preserves ordinary post-response registration, early absent-session rejection and MaxSessions/idle/lastSeen semantics. DELETE/lazy expiry retires the matching epoch under the coordination barrier before releasing the inbound lock; sole lock order is inbound-auth mutex → projection/admission barrier; registry cleanup is detached outside both locks; no lock spans I/O, SSE or notification. AC8/9 cover stale-recreation, same-bearer isolation and in-flight leases. Consistent.
- **Preserved built-ins/generic access (§4.9):** Exposure handling is scoped to `tools/list`, `search_tools`, `describe_tool` and direct pin/dynamic calls. Generic `call_tool`, prompts/resources, initialize/initialized and other methods pass unchanged without exposure prerequisites. AC5/7 cover exactly three built-ins and generic independence. Consistent.
- **Metadata/pin/list/cursor invariants (§4.3/4.4/4.7):** Source→same-server defaults→exact layering, `server_default`/`tool` provenance, annotation presence/serialization and protected schemas are retained. Logical targets, pin precedence and per-session interests remain intact. Pages are immutable; cursors are HMAC-authenticated, generation-bound and reject with -32602 `invalid cursor`. AC2–6/9 cover. Consistent.
- **Accepted-runtime/captured execution (§4.2/4.5/4.6):** Direct admission captures one accepted Assembly/generation, binding/schema/lease; no runtime/map recapture or second SDK lookup; direct-pinned vs generic error mapping stays separate. Commit-before-success, stale-generation rejection and immutable projection swaps are specified. AC6/7/9 cover pause-after-capture and removal races. Consistent.
- **Traceability/approvals:** PR-R1..12→PR-AC1..12 mappings are complete. All requirement/design/task approval values remain null; §8 grants no release/commit authority. No invented approval found. §9 invalidates prior pending-initialize/promotion design and dependent work; no approvals are invalidated.
- **Observability (§7):** Documented owner-directed deferral with consequences: existing logging/redaction/metrics/health behavior preserved; no new telemetry; no audit claims. AC12 retains existing regression coverage. Acceptable under the assignment; not blocking. Prior expanded audit remains in `observability-deferred.md`.

**Remaining risk:** Runtime header isolation, live-incarnation fencing and lock ordering are NOT VERIFIED; later authorized PR-AC1/7/8/9 checks must prove them. The spec's stop condition is appropriate. No design defect requiring amendment identified.
