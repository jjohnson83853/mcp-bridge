# Progressive metadata reveal — bounded SDK evidence r3

## Scope and status

Read-only evidence supplement for `.scratch/progressive-metadata-reveal/spec.md`, revision `cursor-choice-reconciliation-2`. No spec text, workflow state, implementation, tests, or approvals changed. No Gemini/client research was performed. Graphify was not used because the worktree is dirty.

Source-access note: the project-scoped codebase searcher cannot retrieve SDK module-cache source. LSP located the exact symbols in Go SDK v1.8.0; only the corresponding bounded ranges were read from `/home/opencode/go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v1.8.0/mcp/`. This is dependency evidence, not a claim of runtime behavior.

## Verified Go SDK v1.8.0 facts

### tools/list and pagination

- `mcp/server.go:979-995`: `Server.listTools` holds `Server.mu` while calling `paginateList`, copies the selected tool pointers into `ListToolsResult`, and returns the SDK cacheable result. This protects one result construction, not a sequence of pages or a multi-call bridge transaction.
- `mcp/server.go:2179-2208,2215-2250`: SDK cursors encode only `pageToken.LastUID`; malformed Base64/Gob input becomes `jsonrpc2.ErrInvalidParams` at `:2220-2225`. A valid cursor continues with `featureSet.above(LastUID)` against the current set. The SDK cursor has no owner or generation binding; membership changes do not themselves invalidate it.
- `mcp/server.go:1878,1914-1918`; `mcp/shared.go:206-224`: `tools/list` is a registered receiving method. The SDK unmarshals its wire parameters into a typed request, invokes the installed receiving `MethodHandler`, and returns a typed `Result`. `AddReceivingMiddleware` is a public wrapper point (`server.go:1851-1864`; middleware types at `shared.go:112-137`). This supports an adapter that validates/wraps cursors without changing the MCP method or response shape; bridge integration remains unimplemented.

### list-change notification ordering

- `mcp/server.go:747-765`: each registry mutation takes the SDK mutex, changes the feature set, and schedules/debounces an asynchronous notification after 10 ms.
- `mcp/server.go:775-824`: notification work collects sessions while holding `Server.mu`, explicitly unlocks before sending, then sends to legacy sessions and subscribed sessions.
- `mcp/server.go:1845-1849`; `mcp/shared.go:179-182`: the public sending middleware wraps the session send handler, and `handleNotify` uses that handler for notifications. **[INFERENCE]** A bridge middleware could wait on a publication-commit signal before forwarding `notifications/tools/list_changed`, without holding the SDK registry mutex across network I/O. No code or concurrency probe establishes that the bridge can safely implement this for all sessions/generations. The 10 ms timer alone is not a commit barrier.

### Session selection and lifecycle

- `mcp/streamable.go:228-246,340-364`: the public factory accepts `func(*http.Request) *Server`; it is documented as creating/looking up servers for new sessions and may return the same server multiple times. The SDK calls it during the protocol-version precheck for every request (`:349-353`). A stateful POST without a session ID calls it again to create the session (`:647-650`); an existing-session POST goes through `lookupSession` (`:635-644`).
- `mcp/server.go:175`: `ServerOptions.GetSessionID` has no request/context parameter. The SDK calls it after selecting the server (`streamable.go:647-656`), stores the session in its internal map before serving the transport (`:727-739`), and sets the session header before stream delivery (`:1683-1699`). **[INFERENCE]** A request-context provisional owner plus a per-owner `GetSessionID` closure appears able to select consistently across both factory calls and bind the generated ID before response bytes. Authenticated context propagation, failed-initialization cleanup, racing follow-ups, and compatibility with existing inbound capacity/lifecycle behavior remain unproved.
- `mcp/server.go:874-879,1606-1611,2136-2164`: public `Server.Sessions`, `ServerSession.ID`, and `ServerSession.Close` APIs exist. Closing disconnects the session and invokes SDK cleanup; streamable transport's `onTransportDeletion` callback itself remains private (`streamable.go:686-697`). **[INFERENCE]** The bridge may be able to close SDK sessions at its existing DELETE/lazy-expiry owner-retirement point without enabling SDK idle timeouts or using the private callback. Correct per-owner association and safe lock ordering are not verified.

## Remaining blockers and checkpoint

- **B2 is still open** for accepted-runtime/surface/route atomicity and persistence failure semantics: the existing bridge stores a candidate before its void `Published` observer; see `runtime-acceptance-evidence.md:19-29`. Middleware and SDK list locking do not create one transaction across runtime, route admission, descriptor publication, new-session enrollment, and durable persistence. The store's post-rename directory-sync error behavior remains as recorded at `runtime-acceptance-evidence.md:25`.
- **B4 is still open** for authenticated provisional-owner association, failed-init/late-response handling, and cleanup under existing bridge lifecycle. SDK APIs provide candidate seams, not proof of an end-to-end adapter. See `session-writer-evidence.md:17-28`.
- No independent critic pass or applicable user design checkpoint is recorded. **The design checkpoint remains blocked**; no implementation task or implementation authorization follows from this supplement.

Verification: exact source ranges above were read through LSP-located SDK v1.8.0 symbols. No tests, builds, wire/runtime/client probes, or code changes were run.