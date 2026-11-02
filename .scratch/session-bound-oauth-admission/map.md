# Session-Bound OAuth Admission and RFC 9728 Canonical Resource Alignment — Task Frontier

Canonical specification: [Session-Bound OAuth Admission and RFC 9728 Canonical Resource Alignment](spec.md).

## Task Frontier

| Ticket | Blocked by | Triage state |
| --- | --- | --- |
| [01 — Canonical Endpoint Resource Metadata Alignment](issues/01-canonical-resource-metadata-alignment.md) | None | ready-for-agent |
| [02 — Session Admission Seam & Tokenless Post-Init Discovery Burst](issues/02-session-admission-seam-and-discovery-burst.md) | None | ready-for-agent |
| [03 — Explicit Credential Precedence & Dynamic Token Renewal](issues/03-explicit-credential-precedence-and-token-renewal.md) | 02 | ready-for-agent |
| [04 — Authorization Ceiling & Scope Policy Enforcement](issues/04-authorization-ceiling-and-scope-enforcement.md) | 02, 03 | ready-for-agent |
| [05 — Integrated Action Discovery Wire Verification](issues/05-integrated-action-discovery-wire-verification.md) | 01, 02, 03, 04 | ready-for-agent |

## Execution Frontier

The current execution frontier consists of **01** and **02**, which have no blockers and can be claimed immediately:
- `01`: Unblocked, ready for implementation.
- `02`: Unblocked, ready for implementation.
- `03`: Blocked by `02`.
- `04`: Blocked by `02`, `03`.
- `05`: Blocked by `01`, `02`, `03`, `04`.
