# 03 — Initialize Handshake Lifecycle Evidence & Stateless Representation

**What to build:**
Captures truthful session establishment evidence during the `initialize` handshake in stateful mode, recording session creation and pseudonym references directly into the single terminal completion record without emitting duplicate log events or leaking credentials. In stateless mode, explicitly records that session state was not supplied and session binding is unavailable.

**Blocked by:** 01 — Typed Session Admission Seam & Outcome Classification

**Status:** closed

- [x] Successful stateful `initialize` response sets `SessionState: "created"`, `BindingRelation: "not_evaluated"`, and truthful `RefState` in the request's terminal diagnostic record.
- [x] Exactly one completion record is emitted per handshake without secondary log entries.
- [x] Stateless `initialize` handshake records `SessionState: "not_supplied"` and `SessionBinding: "unavailable"`.
- [x] Simulated reference generation failure gracefully sets `RefState: "unavailable"` without failing the request.
- [x] Unit tests in `internal/service/inbound_test.go` verify handshake diagnostic contributions across stateful and stateless modes.
