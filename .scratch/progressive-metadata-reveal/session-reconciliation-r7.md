# Session reconciliation r7

Planning-only provenance; no checkpoint approval or runtime verification.

Native source: `agent://ReconcileSessionContract/specification` (schema valid).
Full native specification spill: `artifact://881`, 83226 UTF-8 bytes.
Canonical SHA-256: `30b0738e55711d634101f8b32eec89d1d2a84101bdb34d70fa40109172118e6b`.
Canonical revision: `cursor-choice-reconciliation-7-session-admission`.

## Planner handoff

DRAFT r7-session-admission → complete nine-section replacement in this native result's `specification` field; runtime artifact persistence/reference must be verified by coordinator. No canonical files written, tasks issued or approvals granted.

R4-C2/B4: supplied current-POST RequestExtra.Header + actual SDK Session.ID suffice at design level. Strip every case-insensitive client carrier copy; stamp admitted record epoch only after existing auth/session admission; match actual ID/live incarnation after SDK gating. Lazy enrollment only for list/search/describe/direct calls; generic call_tool and other MCP traffic bypass exposure requirements. Removed pending map, absent-session exception, initialize-result promotion and write-failure interception. Ordinary registration, early-follow-up rejection, MaxSessions/idle/auth/leases unchanged. DELETE/lazy removal fences stale work.

Affected canonical anchors: `.scratch/progressive-metadata-reveal/spec.md` §1; §3 E3/E2p/A3/B4 and conclusion; §4.5/4.6/4.9 (old heading `Trusted pending association, promotion and terminal cleanup`); §5 PR-AC1/7/8/9; §6; §8 cutover; §9. PR-AC6 retained verbatim. §7/R4-C3 unchanged; no telemetry audit reopened. Metadata parity, pin precedence, pages/cursors, captured execution and runtime-acceptance sequencing retained.

Supplied source anchors: inbound.go:298-393,415-459,542-559; server.go:8-12; SDK streamable.go:1462-1464,1584-1589; shared.go:216-224,112-130,608-613,659-666; server.go:1606-1610,1851-1864,1878-1880,1988-2005,2038-2046. Secondary contract `.scratch/tool-metadata/spec.md` METADATA-r7 R4/AC4/provenance.

Verification NOT RUN: no commands/tests/source discovery/client probes. Four document reads only; no tool validation/execution failures. Remaining risks: unverified carrier sanitization/request isolation, live-epoch fencing/lock order, captured execution/runtime ordering and retained metadata/cursor behavior. All require later authorized AC proof; independent re-review/checkpoints remain. No exact missing source fact identified; no critic PASS claimed.
