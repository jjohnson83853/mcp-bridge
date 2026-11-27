# Ticket 01 spec — review round 11 findings ledger

Candidate revision r11 (`ticket01-bounded-results-v11`), verdict REQUEST CHANGES (bound).
Observation: `.omp/artifacts/token-eff01-review/observation-code-reviewer-r11-586ac68b6648.json`.
Review budget: the 6-round allocation was consumed here; the owner raised it to **8** and chose **typed terminal** for oversize results. Consumed: 6 of 8.

## Owner resolution

Enforcing the upstream body cap is removed from this ticket (decision log: "Oversize results: typed terminal (owner)"). Total retention holds up to the per-entry cap; a larger result returns `result_capacity_exceeded`.

## Verified adequate (do not regress)

Whole-`CallToolResult` marshal accounting with duplicates per copy; `total_bytes == len(S)`; 8192/8193 and preview ≤1536 on that form; termination with pinned pages and no nesting; handles bound to bridge-stamped `{sessionID,epoch}`, pairwise distinct, one `invalid_handle`, two-phase validation; per-namespace hard bound, FIFO, own-only eviction, atomic creation, immutable reads; `isError`/`upstream_code` on preview and pages; small `errorResult`; `additionalProperties:false`; seam after `commitDiscovery`; T1–T7 dependencies.

## Open findings

| ID | Class | Sev | Anchor | Defect |
| --- | --- | --- | --- | --- |
| H1 | statement-contradiction | HIGH | §4.5, §7, T2 | The enforced cap needs `Command.MaxResponseBytes` in `NewClient` via `cmd/mcp-bridge/wiring.go:159` and `internal/upstream/runtime.go:154`, neither a write root nor a T2 expected file. **Resolved by removal of enforcement.** |
| H2 | statement-contradiction | HIGH | R11, R9 | A cap on the shared `HTTPClient` bounds every upstream body, including discovery lists and the long-lived SSE GET, whose supported volumes exceed 8 MiB. **Resolved by removal of enforcement.** |
| M1 | statement-contradiction | MEDIUM | R1 | An over-cap body yields a dispatch error before the seam, contradicting the universal preview+handle claim. |
| M2 | statement-contradiction | MEDIUM | R1, R9, AC9 | `C_envelope = 256` undercounts the bridge envelope (`server` ≤32, wire `tool_name` ≤128, `request_id` 32, keys and wrapper ≥ ~300 B). |
| M3 | ac-not-falsifiable | MEDIUM | AC13, T2 | `TestEnvelopeInfeasibility` has no inducible seam; config-load validation makes the terminal unreachable. |
| M4 | ac-not-falsifiable | MEDIUM | R7, AC14 | Pairwise distinctness passes for a counter or a content-derived handle, so unguessability has no oracle. |
| L1 | ledger-state-drift | LOW | §11 OD2 | `decisions.md` records 16 MiB while the spec uses 100663552 bytes with a 384 GiB worst-case exposure; the change is unrecorded. |
| OQ | statement-contradiction | LOW | R1, T4 | `call_tool` has no owner-rejection path and inbound stamps no epoch when `sessionEpoch == 0`; epoch-0 sessions may be rejected by a rule the spec assumes exists. |
