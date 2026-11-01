## 1. STATUS

**ID:** progressive-metadata-reveal/session-writer. **Revision:** PR-planning-recovery-2/SW-1. **Status: BLOCKED for complete adapter feasibility; assigned bounded investigation COMPLETE.** Objective: establish bridge ownership, lifecycle and streaming constraints without choosing or implementing a replacement architecture.

This is a scoped evidence supplement, not a replacement of the feature specification or an implementation authorization. Existing PR-R/PR-AC IDs and pending checkpoints remain unchanged.

## 2. REQUIREMENTS

Preserve PR-R1/R6/R7/R8/R9: session-local listing AND direct invocation; cleanup on existing DELETE/lazy expiry; no resurrection; atomic old-or-complete-new lists; unchanged execution/auth/session/result contracts. Preserve PR-R10/R12 scope and evidence discipline. These requirements are recorded at `.scratch/progressive-metadata-reveal/spec.md:22-35`; binding isolation/atomicity choices are at `.scratch/progressive-metadata-reveal/owner-choice.md:14-23`, and internal-wiring planning scope at `:27-33`.

Non-goals: implementation, preferred architecture selection, new admission/expiry rules, external SDK research, client-behavior verification, telemetry additions, or running checks.

## 3. EVIDENCE AND UNCERTAINTY

**VERIFIED — current bridge implementation:**

- **One shared MCP server, not a bridge-created server per session.** Startup creates one `server`, registers tool and prompt surfaces on it (`cmd/mcp-bridge/serve.go:112-118`), then passes it to the inbound transport (`:138-155`). `NewStreamableHandler` returns that identical pointer from every factory invocation and supplies nil transport options (`internal/service/server.go:8-13`). `NewMCPServer` advertises prompts (`:8-9`). This proves bridge server ownership, not SDK factory invocation frequency or internal session allocation.
- **Concrete inbound ownership:** `InboundHandler` owns an `http.Handler` delegate and `map[string]sessionRecord`; records contain only token fingerprint, lastSeen and diagnostic ref, not an MCP server/session/exposure owner (`internal/service/inbound.go:25-53`). Existing IDs are admitted against that map, with constant-time fingerprint comparison and accepted-request lastSeen refresh (`:313-325`); absent/mismatched bindings return 404 (`:326-356`). Session capacity uses the map after lazy reaping (`:365-380`).
- **Delegation precedes association.** A cloned request preserves context and replaces the internal request-ID header before delegation (`internal/service/inbound.go:384-393`). Only after delegation returns does a response-issued session ID insert/replace a map record (`:415-445`). That path has no retired-owner/epoch check (`:415-434`).
- **DELETE/idle release bridge admission records only at the shown sites.** After delegated DELETE, the supplied session ID is removed without a response-status condition (`internal/service/inbound.go:455-459`). Lazy reaping deletes expired map entries, including their diagnostic references, with idle<=0 disabling reaping, strict elapsed>idle, and the existing throttle exception at 64 records (`:542-558`). Neither removal invokes SDK close or exposure cleanup. The record itself owns no SDK pointer (`:46-53`). This is not proof that delegated SDK DELETE performs no additional cleanup.
- **Writer composition:** the listener uses `diagnostics.Wrap(inboundMux)` (`cmd/mcp-bridge/serve.go:223`), with `/mcp` routed through auth middleware to inbound (`:172-174`). Outer diagnostics claim once, delegate and defer completion (`internal/observe/httpdiagnostics.go:169-197`). Nested Claim preserves an existing writer/context owner rather than wrapping again (`:205-219`); inbound claims that owner (`internal/service/inbound.go:196-205`).
- **Immediate forwarding, not whole-response buffering:** Header returns the underlying map; WriteHeader forwards the first observed status; Write forwards immediately, records implicit 200, actual bytes and failure; first ordinary Flush sends 200 if needed then delegates (`internal/observe/httpwriter.go:178-218`). Error-aware flushing delegates first, records success/failure, falls back to ordinary flushing or ErrNotSupported (`:220-241`); this helper alone does not prove which SDK path invokes it.
- **Method sets matter:** optional capabilities are probed once (`internal/observe/httpwriter.go:44-80`); generated non-Flusher and Flusher facades have distinct method sets and both expose Unwrap (`internal/observe/httpwriter_facades_generated.go:19-40`). The all-capability facade also delegates Hijack, Push, CloseNotify, ReadFrom and WriteString (`:903-913`). Factory selection uses the capability mask (`:915-934`). ReadFrom and WriteString delegate independently of Write (`internal/observe/httpwriter.go:262-286`). NativeHTTPWriter intentionally exposes the original pre-observer writer for request-size handling (`internal/observe/httpdiagnostics.go:295-303`).
- **Publication/coexistence integration:** the current Published callback updates prompts, tools and operating views together at one startup-wired callback (`cmd/mcp-bridge/serve.go:201-205`). This proves the registration/update integration points, not safe multi-server registration/fan-out internals.

**ASSUMPTION / INFERENCE:** a short association/publication action before bytes/headers escape can coexist with forwarding streaming writers; the shown diagnostic layer does not itself require buffering. A lock held until ServeHTTP returns would instead cover potentially long streaming work. Neither statement proves a complete adapter.

**UNKNOWN / precise coordinator evidence requests:** B4 still needs public SDK factory invocation/reuse conditions; request→ServerSession/server association; SDK DELETE/close/map-retention behavior; all header/flush/SSE response paths; and prompt/resource registration/publication ownership. Supply bounded SDK factory/session/DELETE/write-path excerpts and prompt-surface registration/publication anchors, because these are outside this assignment's allowlist. B2 still needs SDK notification-send ordering, list/cursor/result ownership and runtime acceptance evidence; this slice cannot close it.

Tools: bounded MCP search_codebase/read_range only for repository retrieval. Graphify skipped as forbidden on supplied dirty state. LSP is absent from the mounted device inventory, so no semantic reference/type claims are made. No direct repository reads, shell commands or verification ran. HEAD was supplied, not independently checked.

## 4. DESIGN

**Constraints, not a selected implementation:**

1. The current constant shared-server factory does not establish per-session descriptor isolation. Any changed integration must enforce both list and direct-call visibility independently for same-bearer sessions; bearer identity alone is insufficient. This is required new behavior, not present ownership (`owner-choice.md:16-19`; current ownership evidence above).
2. Session association cannot depend solely on the post-return path when an issued ID may already be usable by a streaming client. The future seam must establish the correct owner before exposure of that ID, without delaying the whole response. The existing proposal recognizes this at `.scratch/progressive-metadata-reveal/spec.md:164-168`; SDK timing remains supplied rather than independently verified here.
3. A commit interception design must account for explicit headers, implicit Write, first Flush, optimized ReadFrom/WriteString, optional-interface presence/absence, Unwrap/controller paths and native-writer bypasses. Merely embedding http.ResponseWriter can hide Flusher; universally adding Flush can falsely advertise it. Do not modify diagnostic completion semantics to create a second observer.
4. Per-request identity and session lifetime are distinct. Required session exposure must survive ordinary request completion, retire at the existing removal points, and resist late discovery/publication/response-header reattachment. Current unconditional post-return insertion is a concrete race boundary, not a retirement fence (`internal/service/inbound.go:415-434`). Early binding must also be assessed against existing capacity timing rather than silently adding reservations.
5. Bridge-record cleanup is not SDK eviction. Claims about releasing transport/session storage require SDK evidence. Required exposure-reference cleanup must remain valid even if SDK retains a server/session; no new SDK timeout or expiry mechanism may be introduced to avoid that proof (`.scratch/progressive-metadata-reveal/spec.md:170-180`).
6. Atomic list snapshots are a publication/read-ownership contract, not a response-buffering contract. Transport Write/Flush interception alone does not prove immutable old/new descriptor sets, route-generation coherence, cursor behavior or postcommit notification ordering. No lock may be held across SSE delivery/upstream I/O merely to approximate atomicity (`.scratch/progressive-metadata-reveal/spec.md:124-158`).
7. Any integration change must preserve existing prompt capabilities, registration and accepted-update propagation, not create tool-only sessions (`internal/service/server.go:8-9`; `cmd/mcp-bridge/serve.go:112-118,201-205`). Multi-server fan-out feasibility is still unknown.

## 5. ACCEPTANCE CRITERIA

All methods below are **planned, NOT RUN**; existing criteria are retained without weakening.

- **PR-AC1/7/10 → PR-R1/6/9:** two initialized sessions using the same bearer; discover only in A; A lists/invokes returned descriptors while B neither lists nor directly invokes them. Generic execution remains unchanged. RED for new behavior: A's revealed descriptor/direct route is missing; unchanged test must pass GREEN.
- **PR-AC8 → PR-R7:** injected-clock lazy expiry and delegated DELETE retire only the affected session's exposure; immediate/new initialization starts empty; late completion cannot restore exposure. Retain exact current throttle/predicate, lastSeen and admission behavior. New retirement test RED: obsolete exposure remains usable or reappears; current auth/session checks are regressions, not invented REDs.
- **PR-AC9 → PR-R8:** real HTTP /mcp initialize with first-header visibility barrier, immediately issue a same-session follow-up before initialization handler return, and verify owner association without buffering SSE. Cover explicit WriteHeader, implicit Write, first Flush, no-ID/rejected initialization, cancellation/write failure and lifecycle races. Exercise optional-capability presence AND absence; optimized writer methods must not bypass association. RED: follow-up lacks owner, streaming stalls, or capability behavior changes.
- **PR-AC6/9 → PR-R5/8:** barrier-controlled publication/list/direct-call/notification scenarios yield complete old/new sets and matching bindings, including real supported cursor behavior, without blocking on open SSE delivery. Notification ordering cannot be claimed from a writer-only test. Exact SDK seam remains blocked.
- **PR-AC7/9/12 → PR-R6/8/11/12:** retain auth/status/envelope/prompt/coexistence and single truthful diagnostic completion regressions. No current test names or passing results are asserted.

**Bounded wire seam:** compose the same diagnostics→mux/auth→InboundHandler→NewStreamableHandler chain anchored at `cmd/mcp-bridge/serve.go:138-174,223`, using controlled HTTP clients, upstream fixtures, clocks and synchronization barriers. Observe wire headers/SSE before handler completion; use request/response assertions rather than private implementation copies. No live client/model probe is necessary.

## 6. TASK LEDGER

**resolve-session-writer:** authorized read-only task COMPLETE; inputs were the eight allowlisted files. Deliverable is this runtime-owned structured evidence supplement. No source/tests/config/infra changes or verification were permitted or performed.

Implementation ledger remains deferred exactly as recorded at `.scratch/progressive-metadata-reveal/spec.md:212-220`. No executable T IDs or worker assignments are issued. Dependency: coordinator supplies the precise B4/B2 facts above → planner reconciles the complete design → independent review and explicit applicable checkpoints. A compatibility-critical missing SDK/lifecycle fact blocks its affected implementation, not the completed static investigation.

## 7. OBSERVABILITY

The observability-contract skill was loaded before this section. For the affected inbound/writer integration, preserve the single diagnostic owner/completion (`internal/observe/httpdiagnostics.go:169-219`) and actual status/byte/typed-write-failure observations (`internal/observe/httpwriter.go:180-241`; `internal/service/inbound.go:394-412`). Planned PR-AC7/9 regression checks must show truthful failures and no duplicate terminal record; PR-AC12 retains documentation of limitations. Never introduce raw session IDs, bearer fingerprints, credentials or payloads into logs/labels.

No new observability component is designed or implemented. Full JSON field/severity schema, logger shipping, Loki labels/retention/ingestion handling, Prometheus names/types/units/buckets/labels/scrape path, tracing exporters, alerts and dashboards cannot be established from these files. Existing exceptions remain at `.scratch/progressive-metadata-reveal/spec.md:224-255`: reason is unchanged observability infrastructure and bounded no-new-telemetry scope; consequence is no claim of delivery/scrape/alert/dashboard verification or complete checklist-field compliance. Existing health and operations wiring is visible at `cmd/mcp-bridge/serve.go:159-173,224-230`; preserve rather than expand it. New spans/health states/signals are not justified by a read-only ownership investigation. Concrete implementation T bindings remain deferred with the ledger, not falsely marked satisfied.

## 8. RELEASE AND RECOVERY

Not applicable to this read-only supplement: no implementation, deployment, migration or irreversible action occurred. Existing feature release/recovery obligations remain at `.scratch/progressive-metadata-reveal/spec.md:257-267`. No approvals or gates changed; dirty user work was preserved.

## 9. CRITIC HANDOFF

B4 is **partially evidenced, not closed**: shared bridge server, post-return association gap, bridge cleanup scope and capability-preserving streaming writer behavior are established. SDK concrete session retention, association timing, notification/SSE interaction and prompt fan-out remain unproved. C4 is therefore not closed. C2/B2 is unchanged; no runtime-acceptance conclusion is imported from out-of-scope files. No other finding or PR-R/AC obligation is dropped.

Highest-risk question: can pre-wire association and terminal retirement coexist with existing admission timing, all writer capabilities and SDK session retention without protocol/auth/session changes? Second: can publication/list/notification atomicity be proven separately from HTTP write observation?

Self-evaluation: accuracy 4/5 (SDK object lifetime unavailable); completeness 4/5 (all allowed seams traced, external ownership necessarily unresolved); clarity 4/5 (separate existing/required/inferred behavior, but wrapper paths are intricate); actionability 4/5 (exact missing SDK/prompt targets identified, no execution gate); conciseness 4/5 (nine-section role format is longer than the compressed report). Overall 4.0/5; improvement priority is bounded SDK lifecycle evidence, then prompt fan-out evidence. User agreement is unconfirmed.

Artifact: this dispatch's native structured result, `specification` field. Actual persisted path must be retrieved/verified by coordinator; no canonical or standalone file was written.
