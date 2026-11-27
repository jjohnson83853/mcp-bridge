# Ticket 01 spec — review round 8 findings ledger

Candidate revision r8 (`ticket01-bounded-results-v8`), verdict REQUEST CHANGES (bound).
Observation: `.omp/artifacts/token-eff01-review/observation-code-reviewer-r8-3bad151ce23d.json`.
Review budget (owner allocation 6): **3 of 6 used**, 3 remaining.

## Verified adequate (do not regress)

Complete `CallToolResult` marshal accounting with per-occurrence duplicates and escape expansion; preview < delivery by validation and page feasibility; `retrieve_result` never paged; 32-byte crypto handles with owner+epoch binding, one `invalid_handle`, retired-epoch rejection, three retirement seams, explicit publisher namespace; per-namespace hard bounds with soft aggregate, FIFO, atomic create, serialized reads; `is_error`/`upstream_code` extraction path; small `errorResult`; `additionalProperties:false`; discovery seam placement; built-in inventory; T1–T7 dependencies.

## Open findings

| ID | Class | Sev | Anchor | Defect |
| --- | --- | --- | --- | --- |
| H1 | statement-contradiction | HIGH | §4.5, R1, AC13, T2 | `max_bytes_per_session` (8 MiB) is a hard per-namespace bound, but `max_retained_result_bytes` derives from the transport cap inflated by duplicated content/structured forms plus escaping, so one reachable result exceeds the hard bound; creation "cannot overshoot" and no terminal outcome is defined. R1/AC13 "every over-ceiling result yields a handle" is then false with no fixture. |
| M1 | r-ac-coverage-gap | MEDIUM | AC11, AC13, T4/T5 | AC13 names one oracle owned by both T4 and T5 (duplicate symbol, or the discovery/publisher half stays unowned); T4's `TestCeilingAcrossAllSurfaces` cannot pass before T5's seam exists. |
| M2 | ledger-state-drift | MEDIUM | §8, AC18, T2, §7 | Collectors are claimed registered in the ops registry, but the process `observe.Metrics` reaches the surface only through the `ToolSurfaceConfig` literal at `cmd/mcp-bridge/serve.go:164`, outside every write root; §8 also promises per-surface private registries that no ops handler scrapes, and says four collectors where §8/AC18/T2 say five. |
| M3 | statement-contradiction | MEDIUM | R9, §4.5, AC9, AC17 | All session-less callers resolve to the single `publisherOwner`, so cross-caller eviction isolation is false there and AC9 cannot distinguish; AC17 overstates "its own publisher-bound handles". |
| M4 | ac-not-falsifiable | MEDIUM | T6, §6 | T6's checks are jointly unsatisfiable: a positive grep for `result_too_large`, which the spec removes, plus negative greps forbidding strings §6 keeps in qualified form. |
| M5 | ac-not-falsifiable | MEDIUM | §4.1, T5, AC11 | "Seam after `commitDiscovery`" is a placement instruction with no behavioural oracle; a seam before it passes every AC while breaking progressive reveal. |
| L1 | statement-contradiction | LOW | R1, AC13, §4.4 | "The only terminal case" is false: entropy failure and envelope infeasibility are documented fail-closed `internal_error` outcomes with no handle. |
| L2 | ac-not-falsifiable | LOW | R1, AC1/AC13, §4.2 | No AC asserts the preview text block carries a non-empty UTF-8-safe prefix; a metadata-only preview passes. |
| L3 | statement-contradiction | LOW | R8, AC8, §4.5 | Tombstone overflow evicts oldest tombstones first, so the expired/evicted classification is lost before `tombstone_ttl`; determinism is overstated. |
