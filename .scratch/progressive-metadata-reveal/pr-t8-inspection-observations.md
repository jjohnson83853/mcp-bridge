# PR-T8 operator documentation and changelog inspection observations

Feature: `progressive-metadata-reveal`
Revision: `cursor-choice-reconciliation-7-session-admission`
Selected branch: `approved-progressive-metadata-ledger`
Task scope: PR-T8 documentation and changelog review.

## Verification Checklist per spec.md §6.2 PR-T8 and PR-R11:

1. [x] Built-ins and current-pin baseline described: new sessions expose exactly 3 built-ins (`search_tools`, `describe_tool`, `call_tool`) plus configured pins.
2. [x] Session-local progressive discovery: `search_tools` and `describe_tool` commit discovered unpinned tools to calling session's local projection before returning success; immediate re-list / direct call succeeds.
3. [x] Session isolation: dynamic reveal in session A is not visible in session B, even when using the same bearer token.
4. [x] Metadata defaults and exceptions parity: unpinned tools revealed via discovery receive applicable `tool_metadata.defaults` (`server_default` layer) and `tool_metadata.tools` exceptions (`tool` layer).
5. [x] Annotations presence and scalar normalization: advisory claims, never authorization; no blanket read-only default installed automatically.
6. [x] Mixed-service risks: broad defaults caveat documented; explicit `false` overrides write tools back to honest warnings.
7. [x] Immutable session pages and signed cursors: 1000 default or configured size; 57-byte URL-safe base64 cursors signed with HMAC-SHA256; stale or post-reload cursors return JSON-RPC -32602 `invalid cursor`.
8. [x] Session lifecycle: HTTP DELETE `/mcp` or idle timeout retires session projections; in-flight direct call leases survive DELETE until completion.
9. [x] Client advisory boundary: best-effort asynchronous `tools/list_changed` notifications; explicit re-list required; no zero-prompt promise; Gemini CLI 0.18.13.x cited as user-reported client.
10. [x] CHANGELOG.md [Unreleased]: entries added under `### Added` and `### Changed` reflecting progressive reveal, session-scoped projections, HMAC cursors, and metadata parity.

Artifact hashes:
- `README.md`: `45528687f728596470e78b674d529dfe35f6ce859a655f16672bc4ee01fb628d`
- `CHANGELOG.md`: `0328dcf130124843b4c2ecd588dcffc0729eb2d387c4614a6530643cd493d2e4`

Outcome: PASS. All PR-R11 / PR-T8 requirements satisfied without code or config modifications.
