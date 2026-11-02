# ChatGPT Action Discovery and RFC 9728 OAuth Protected Resource Routing — Task Frontier

Canonical specification: [ChatGPT Action Discovery and RFC 9728 OAuth Protected Resource Routing](spec.md).

## Task Frontier

| Ticket | Blocked by | Triage state |
| --- | --- | --- |
| [01 — RFC 9728 multi-path metadata discovery and path-aware resource advertisement](issues/01-rfc9728-multi-path-metadata-discovery.md) | None | ready-for-agent |
| [02 — Bidirectional audience tolerance and missing-credential safe diagnostics](issues/02-bidirectional-audience-tolerance-and-diagnostics.md) | 01 | ready-for-agent |
| [03 — Route normalization and root POST discovery probe handling](issues/03-route-normalization-and-root-probe-handling.md) | 01, 02 | ready-for-agent |
| [04 — Integrated action discovery wire verification and diagnostic provenance](issues/04-integrated-action-discovery-wire-verification.md) | 01, 02, 03 | ready-for-agent |

## Execution Frontier

The current execution frontier is ticket **01**, which has no blockers and can be claimed immediately:
- `01`: Unblocked, ready for implementation.
- `02`: Blocked by `01`.
- `03`: Blocked by `01`, `02`.
- `04`: Blocked by `01`, `02`, `03`.
