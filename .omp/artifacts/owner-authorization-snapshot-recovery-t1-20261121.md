# Owner authorization - snapshot-recovery T1 (bounded)

Date: 2026-11-21
Owner message (verbatim): "reviews pass; select the reviewed branch and dispatch the engineer for T1"

Scope granted by this directive:
- Select the reviewed packet branch `snapshot-recovery-r29` (spec sha256 33fdf021d3a88f16b7fde1eb0993041a402c2dc03d83732cb892683d9bfdebfc; CR APPROVE, CT PASS).
- Authorize the bounded branch's implementation tasks T1-T4 within the existing env-1 envelope and the unchanged 14-file write set.
- Dispatch `engineer` for T1 only; T2-T4 remain gated by their depends-on chain.

Not granted: R/AC/T id changes, envelope path changes, review/approval claims, delivery, or changes outside the reviewed spec.
