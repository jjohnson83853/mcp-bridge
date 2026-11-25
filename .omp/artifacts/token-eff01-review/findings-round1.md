# Ticket 01 spec — review round 1 findings ledger

Candidate `token-efficient-tool-responses-ticket01-spec` r1, verdict REQUEST CHANGES (bound).
Source observation: `.omp/artifacts/token-eff01-review/observation-code-reviewer-r1-89e4041210f0.json`.
Reviewer: `code-reviewer` (independent of planner/engineer).

| ID | Class | Sev | Anchor | Defect |
| --- | --- | --- | --- | --- |
| F1 | statement-contradiction | HIGH | `internal/service/tools.go:700-910` | `call_tool` resolves no owner identity; `directRequestOwner` is used only at `tools.go:225/435/562` and `boundDirectExecution` carries none. Handle binding therefore either falls back to the publisher — letting one caller read another's handles, defeating R7 — or requires a new identity the spec does not define. |
| F2 | ac-not-falsifiable | HIGH | spec §4.2 | AC10 cannot pass: pages omit `code`, and the preview reads the original's top-level `structuredContent.code`, which for `call_tool` is the bridge envelope, not the upstream code inside `structuredContent.upstream_result`. |
| F3 | statement-contradiction | MEDIUM | spec §4.5 | `result_too_large` returns an over-ceiling result with no handle and no `paging_required`, contradicting R1, with no AC fixture for the branch. |
| F4 | ledger-state-drift | MEDIUM | spec §6 T3 | A fourth built-in breaks existing tests (`tools_test.go:512-548`, `progressive_session_test.go:180-183`) that are neither write roots nor T3 files, so T3 cannot exit 0 inside its envelope. |
| F5 | r-ac-coverage-gap | MEDIUM | spec §6 | No task runs the existing `./internal/service` or `./cmd/mcp-bridge` suites; AC12's full-suite gate is unattached to any exit condition. |
| F6 | ledger-state-drift | MEDIUM | spec §6 T6 | T6 is docs-only yet its RED step and validation require a Go test it may not write. |
| F7 | statement-contradiction | MEDIUM | spec §6 | README claim change omits `README.md:279` ("three tools"), the annotations table, and `README.md:325` completeness claim, and calls provenance fields still true over-ceiling. |
| F8 | lifecycle-hazard | MEDIUM | spec §4.5 | Owner verification is not ordered before existence, expiry and eviction evaluation, so a foreign handle can answer `result_expired`/`result_evicted` and reveal existence. |
| F9 | lifecycle-hazard | MEDIUM | spec §4.5 | Creation plus aggregate admission is not serialized; concurrent creates can overshoot `max_bytes_aggregate`. |
| F10 | lifecycle-hazard | MEDIUM | spec §4.5 | TTL is reclaimed only on retrieval and session retirement, never for live idle sessions; expired entries consume budget and force premature eviction. |
| F11 | ac-not-falsifiable | LOW | spec §4.2 | `preview_bytes` and `page_size` are undefined and unasserted by AC1/AC3. |
| F12 | statement-contradiction | LOW | spec §4.4 | The measured form is the `CallToolResult` value, not the JSON-RPC wrapper; the excluded constant is unstated. |
| F13 | ac-not-falsifiable | LOW | spec §4.5 | Handle randomness and non-derivation have no observable oracle beyond rejection tests. |
| F14 | statement-contradiction | LOW | spec §4.2 | The preview sets `truncated`, which ticket 02 uses for count clipping; only `paging_required` plus a handle distinguishes them. |
| F15 | statement-contradiction | LOW | spec §6 T1 | 8192/1536 are fixed in requirements but configurable in T1; R2's boundary is false under a lowered ceiling. |
| F16 | statement-contradiction | LOW | spec §4.3 | `invalid_handle` is said to cover malformed handles, but the schema pattern rejects them before the handler. |

## Review budget

S15 default: 5 completed CodeReviewer reviews per planning task. Used: 1 (this round). Remaining: 4.

## Adequacy recorded by the reviewer (do not regress)

K1 byte-accounting single form; K2 no nesting/termination; K3 crypto handle + epoch binding; K4 bounded store with foreign-eviction ban; K5 dispatch failures not paged; K6 first-class tool without schema change or `full`; K7 seam after `commitDiscovery`; K8 UTF-8 slices and byte-exact concatenation; K9 acyclic ledger; K10 `upstream_result` claim named.
