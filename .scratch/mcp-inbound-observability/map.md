# MCP Inbound Observability Enrichment — Task Frontier

Canonical specification: [MCP Inbound Observability Enrichment](spec.md).

## Task Frontier

| Ticket | Blocked by | Triage state |
| --- | --- | --- |
| [01 — Inbound Method Operation Classification and Unified Prometheus Metrics](issues/01-inbound-method-operation-classification-and-metrics.md) | None | ready-for-agent |
| [02 — Client Family Classification and Exact Response Bytes](issues/02-client-family-classification-and-exact-bytes.md) | None | ready-for-agent |
| [03 — Exposed Tool Count and Wire Observability Verification](issues/03-exposed-tool-count-and-wire-verification.md) | 01, 02 | ready-for-agent |

## Execution Frontier

The current execution frontier consists of **01** and **02**, which have no blockers and can be claimed immediately in parallel or sequence:
- `01`: Unblocked, ready for implementation.
- `02`: Unblocked, ready for implementation.
- `03`: Blocked by `01`, `02`.
