# 01 — Typed Session Admission Seam & Outcome Classification

**What to build:**
A typed session admission boundary between the inbound session store and the authentication middleware that accurately reports why a session request succeeded or failed (`AdmissionAdmitted`, `AdmissionNotFound`, `AdmissionUnauthenticated`, `AdmissionExpired`), with a strict deadline check where requests at or after the token expiry deadline fail. Retains `SessionClaims` so existing consumers continue functioning without breaking changes.

**Blocked by:** None — can start immediately.

**Status:** closed

- [x] Session admission lookup reports `AdmissionAdmitted` when session ID exists, holds valid claims, and current time is before the auth deadline.
- [x] Session admission lookup reports `AdmissionNotFound` when session ID does not exist in the store.
- [x] Session admission lookup reports `AdmissionUnauthenticated` when session ID exists but has no OAuth claims bound.
- [x] Session admission lookup reports `AdmissionExpired` when current time is at or after the auth deadline (`!now.Before(deadline)`).
- [x] Unit tests verify deadline boundary with controlled clock before the deadline, at the exact deadline, and after the deadline.
- [x] Backward-compatible `SessionClaims` method on the inbound handler is preserved and verified across all existing callers.
