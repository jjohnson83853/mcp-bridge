# Session rehydration — decisions

## D1 (2026-11-01) — Owner-authorized acceptance amendment (standalone defect fix)

[OWNER-DIRECTIVE] The owner supplied an explicit, detailed chat directive to change mcp-bridge session
admission so container restarts do not break stateful MCP clients: an authenticated POST/GET/DELETE on
`/mcp` carrying an unknown (absent) but syntactically valid `Mcp-Session-Id` must be auto-vivified instead
of rejected with `404 session_absent`; DELETE on an absent session is an idempotent teardown (200/204); an
existing session bound to a different principal stays rejected; observability records
`session_state: rehydrated`.

[AUTO-DECISION] Route: standalone owner-authorized defect fix (owner selected this option explicitly).
Rationale: the change reverses an approved acceptance criterion of the completed `progressive-metadata-reveal`
feature (spec `.scratch/progressive-metadata-reveal/spec.md` §4.9 line 181 and PR-AC1 line 208 both require
"absent-session rejection"), so it cannot be executed as a within-ledger edit; and the managed ledger has no
runnable frontier (`execution-state.mjs status` -> `status: blocked`, `runnable_frontier: []`, PR-T1..PR-T9 all
`done`). The owner directive is the authoritative acceptance amendment; no planner/critic round is inserted.
Consequence: PR-AC1's "absent-session rejection" clause is superseded by this fix for authenticated,
non-cross-principal requests; the progressive exposure clauses are preserved (rehydrated sessions enrol a
live owner so `tools/list` keeps working).

[AUTO-DECISION] Implementation design: because the go-sdk v1.8.0 `StreamableHTTPHandler` keeps its own
in-memory session table and answers unknown IDs with `404 session not found` (verified probe), the bridge
alone is insufficient. The bridge `InboundHandler` will (a) record the client-visible ID, (b) obtain a fresh SDK
session by issuing an internal `initialize` through `h.next` (the SDK handler), and (c) forward the original
request under the SDK-issued ID, registering the live exposure owner under that SDK ID so the receiving
middleware's `(actual SDK session id, epoch)` check passes.

Source: this file; owner chat message of 2026-11-01.

## D2 (2026-11-01) — Independent review of the rehydration implementation: FAIL, six findings

Independent review (code-reviewer, PR-T3 scope against the D1 design) returned FAIL / REQUEST CHANGES.
Each finding was re-verified against the working tree and the go-sdk v1.8.0 source before it was accepted;
the recorded evidence is the SDK fact next to each item.

- F1 (HIGH) unknown-session `initialize` is executed twice. `mcp/streamable.go:1693` emits
  `Mcp-Session-Id` on ANY `initialize` response, and `mcp/server.go:2095-2097` answers a second
  `initialize` with JSON-RPC error `duplicate "initialize" received`; the post-forward block therefore
  also registers the internal SDK id as a second bridge session. Consequence: a client that re-sends
  `initialize` after a restart is broken even though HTTP is 200.
- F2 (HIGH) SDK setup can fail without producing 503: `mcp/streamable.go:733-737` closes a session whose
  `InitializeParams()` is nil AFTER the response already carried HTTP 200 plus a session header, so
  `200 + header` is not proof of a successful bootstrap.
- F3 (HIGH) an SDK-only disappearance leaves an unrecoverable stale mapping, because each forwarded
  request refreshes `lastSeen` and no path drops the mapping; bridge-side retirement never releases the
  SDK session either.
- F4 (MEDIUM) the two nested SDK round-trips run while `h.mu` is held, serialising every
  session-bearing request behind a bootstrap.
- F5 (LOW) the rehydration branch is not restricted to POST/GET, so PUT/PATCH/OPTIONS with an unknown id
  allocate a bridge and an SDK session before the SDK answers 405.
- F6 (LOW) `serve_logging_test.go` pins an incidental diagnostic byte bucket.

[AUTO-DECISION] Amendment: the D1 design is refined, not replaced. Unchanged: the owner acceptance
criteria (a)-(d), the security invariants (no rehydration after a failed outer OAuth check; an existing
session bound to a different principal stays 404; no cross-principal hijack), the diagnostics contract and
the exposure-epoch protocol. Changed: an unknown-session request that is itself `initialize` MUST be
forwarded without a synthetic bootstrap (the SDK creates the session and its id is mapped to the client's
pre-restart id); the synthetic bootstrap MUST prove a successful JSON-RPC result before publishing;
a definitively missing SDK session MUST drop the mapping instead of being forwarded forever; the nested
round-trips MUST run outside `h.mu`; and only POST/GET may rehydrate. Rationale: `mcp/server.go:192`
documents that the legacy initialize handshake never rejects, so the duplicate-initialize error is a real
client-visible defect in the restart recovery path, which is exactly the path this fix exists to repair.
Consequence: PR-T3 remains `pending` (re-opened by the owner amendment) until the fixes and their
permanent tests pass; the defect fix is not complete before that.

Source: this file; independent review verdict FAIL of 2026-11-01; verified go-sdk v1.8.0 facts above.
