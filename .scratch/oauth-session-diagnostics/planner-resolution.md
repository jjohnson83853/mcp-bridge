# OAUTH-DIAG-RESOLUTION-r2 — proposed ticket01 resolution

## 1. STATUS

**DRAFT; implementation BLOCKED.** Resolve ticket01's local ownership, correlation, schema/redaction, reference and byte-budget prerequisites without rebuilding the six completed logging tickets. This revision addresses independent r1 critic C1–C5 and security M1/L1; it does not declare those findings accepted/closed by their reviewers.

Only this assigned planning artifact is changed. Canonical OAUTH-DIAG-r4, tickets, gates, source/tests/config/infrastructure remain unchanged. R-D1–R-D9, AC-D1–AC-D10 and T-D0–T-D6 are preserved. Coordinator reconciliation, independent specification/task re-review and explicit affected requirements/design/tasks checkpoints remain necessary. Publication/breakdown approval and “Use the proposed seams” are not implementation or response-contract approval. r1 technical verdict was REJECT; security was APPROVE-WITH-CHANGES, with no proven current vulnerability. r2 is proposed for renewed review.

## 2. REQUIREMENTS

| ID | Required behavior/constraint and proposed delta |
|---|---|
| R-D1 | One whole-listener terminal completion attempt before routing/OAuth, contribution-only nested/inner handlers and standalone fallback; preserve auth/wire/admission/metrics/streaming/panic behavior, except separately reviewed diagnostic additions. |
| R-D2 | Actual first decisive rejection and separately evaluated credential/scope/session/upstream states. Generic evidence stays generic; no status/error-string/unverified-claim diagnosis. |
| R-D3 | Move/reuse existing owned internal HTTP ID before dispatch; preserve caller-controlled public X-Request-ID. Propose one distinct response header carrying that same ID, not another generated ID. |
| R-D4 | Finite method/route/challenge and honest process/source/nonsecret-policy provenance. User-confirmed GitHub HEAD is deployed; runtime process/policy facts remain separate. |
| R-D5 | Distinguish absent entry and existing exact-bearer mismatch without changing either current401, affinity or session lifetime. Independent random live-session pseudonyms only. |
| R-D6 | Strict typed complete-byte serialization, delete-first collision handling, independent refs, no blanket redaction weakening; nonfatal single attempt/no retry; core<=3584, optional<=512, full<=4096 including LF. |
| R-D7 | Actual deployed stdout/capture/collector/normalizer/queried-Loki preservation, not local JSON or mocked forwarding. |
| R-D8 | Deterministic composed-HTTP/captured-log TDD, local JWKS, controlled clocks, independent generic profiles, privacy/failure/capability/lifecycle/metric regression; full-suite PASS and >=90% source-line coverage. |
| R-D9 | Bounded allowlisted operator evidence, explicit owners/unknowns/checkpoints; no client implementation, actual renewal, incident-cause or restart-remedy claim. |

Non-goals: corrective issues06–08; changing auth/scope/binding/status/challenge/body/order/skew/reaping/retry/metric policy; collector/proxy/CORS/client/deployment edits; new trace/log/lookup/index/retention/queue/dashboard/alert stack; rebuilding implemented MCP/backend/configuration logging. Required HTTP severity additions are logging behavior changes, **not** auth/wire changes or claims of preserved current severity.

## 3. EVIDENCE AND UNCERTAINTY

### VERIFIED evidence and provenance

Primary packets: `source-contract.md`, `auth-seam-evidence.md`, OAUTH-DIAG-r4 `spec.md`, existing owner tickets01–05. Remote/local HEAD **e621622397298bb4eb44a9b0f5ff1a00d6f16300**; user confirms HEAD deployed. Planning changes are local-only. Source access is restored; older `delta-evidence.md` access-blocked prose is stale, not current blockage or incident explanation.

| Source anchor | Established fact |
|---|---|
| `cmd/mcp-bridge/serve.go:131–164`; `internal/auth/middleware.go:86–164` | Public mux metadata/health bypass inner completion; `/mcp` wraps inbound with OAuth; fallback invokes inbound. Auth constructor is `NewAuthMiddleware(cfg Config, keys KeySetProvider, opts ...Option) func(http.Handler) http.Handler`; staticFallback is existing option. Credential/key/parse/audience failures401, scope403. |
| `internal/service/inbound.go:139–156,249–287,362–371` | Inner owns timer/generator/statusWriter/defer, emits http_complete; public X-Request-ID normalized/echoed independently; owned internal header scrubbed then supplied on downstream clone. Current HTTP generation=0; no server/backend/tool identity in terminal. |
| `inbound.go:288–360` | Order: host, origin, declared size, inner auth, session/capacity. Host/origin403, declared size413, inner credentials401/bare Bearer, absent/mismatch401/bare Bearer, capacity503. Missing header is not rejected here; initialization can be legal. |
| `inbound.go:331–335,393–401,424–437` | Accepted lookup replaces entire record. Response-issued session header insertion and DELETE removal occur after delegation without added success-status predicates. Reap is request-triggered, strict idle comparison, no tombstone. **393–401 is lifecycle;424–437 is reaping.** |
| `inbound.go:139–207,440–457,579–600`; auth-seam §2 | Clock/generator execute before finalizer registration: initial panic window unobserved. Installed defer unwinds without recover or synthetic500. First WriteHeader wins; Write/Flush observe implicit200; zero status→null/unknown. Always advertises Flush, no Hijack, Unwrap is not full capability transparency. |
| `inbound.go:152–193,249–274`; r1 critic C1 | **Current HTTP completion for tool error under HTTP200/default stage remains info.** Existing operation/tool records are separate. r4-required HTTP warn for actual tool_error under200 is a behavior change, not preservation. Known admission503/delivery severity exceptions already exist. |
| `internal/observe/log.go:13–44,47–103,163–238,241–322` | Entry/Logger signatures, uint64 generation, time.Duration, exact finite old fields, fixed message/null status, redaction after projection, no total ceiling, encoder reset for next independent event—not retry. |
| `internal/service/tools.go:209–315` | Implemented request/operation/backend correlation, actual dispatch/tool-reported failures and summary records. Retain; exports still need complete-byte review. |
| `serve_oauth_test.go:37–56,76–129,167–216,258–395`; auth-seam §2–3 | Real runContext loopback harness/captured output/local JWKS/fake backend. Token timestamp helper is not verifier clock. InboundOptions.Now controls reap/duration; composed entropy/verifier-clock/process/session-ID seams absent. Follow-up helper lacks session/protocol header, not retained-session proof. |
| `internal/service/server.go:11–13`; auth-seam §3 | SDK v1.8.0 nil options. Exact SDK missing-header non-init branch/status unverified; normative suggested400 is not implementation proof. |
| `go.mod:7`; auth-seam §1 | jwx/v2 v2.1.7 public expiry/nbf/issuer errors.Is sentinels exist, but exhaustive Parse wrapping/compound-error taxonomy unproved. Verification-family predicate is broader than invalid-signature proof. Unsupported distinctions stay jwt_invalid_other. |
| README:447–456; `internal/observe/metrics.go:35–43,117–172`; README:373 | Exact final commands below, operations /metrics, seven collectors, no histogram/buckets; newly logged outer boundaries must not increment existing request metrics. |

Retained completed logging baseline: all six tickets complete, suite/race PASS, **92.0% source-line coverage**, code/security PASS. Do not reopen because of stale map prose. Older OAuth91.8% is a different historical result. Parent's post-reconnection focused command `go test ./cmd/mcp-bridge -run '^(TestServeOAuthWire|TestServeLoggingExecution|TestServeLoggingFailures|TestServeLoggingConcurrent)$' -count=1` PASS, package0.397s, is supplied baseline, not new diagnostic acceptance.

Parent research receipts: (1) exact §4.6 pure-data evaluator PASS2048/200/2248 bytes; (2) installed exact-version Go docs PASS0.260s: `jwt.WithClock(v Clock) ValidateOption`, `Clock interface { Now() time.Time }`, `ClockFunc func() time.Time` implements Now; WithClock covers exp/iat/nbf; Parse forwards validation options. Selected adapter `jwt.WithClock(jwt.ClockFunc(now))` is API-evidenced, **not runtime injection proof**. Context7 unavailable; installed API docs were bounded fallback, not vendor source.

This planner's r1 source work was limited authenticated MCP read_range at packet logger/inbound anchors. **r2 performs no new source/API research or verification.** Graphify skipped on dirty tree; packet LSP evidence reused; no bypass/local-source fallback. Repo-local domain docs searched per docs/agents/domain.md; glossary/ADR absent, none created. No application builds/tests/lint/formatting/runtime/Loki probes run by planner. Arithmetic is parent-executed, not a fabricated planner test result.

### UNKNOWN / affected blockers

- U-D1: selected new interfaces/entropy/serializer/capability behavior require implementation/tests; current optional-interface transparency is not established. Optional operation reuse needs ownership-path evidence, not just hex grammar, otherwise unavailable.
- U-D6: keep generic Parse rejection jwt_invalid_other until actual returned typed outcomes are mapped/reviewed; supported public sentinel documentation is not exhaustive runtime wrapping evidence. No unverified-claim decoding/string matching. Exact SDK missing-header cause/status unresolved; do not emit owned session_header_missing from status. Legal no-header initialize remains legal; exhaustive unsupported branches stay blocked.
- U-D2/U-D4: actual collection map/path/labels/retention/access/failure monitor and origin/proxy/client visibility UNKNOWN, named owners UNKNOWN.05 owns external proof; external AC-D3/D5/D6/D8 unwaived, but do not block independently approved local slices.
- U-D3: HEAD deployment relationship resolved; process/replica/stdout/policy and authoritative runtime provenance inputs separate. Verify build and nonsecret effective-policy artifact identity or emit unavailable; no env-hash guess or secret-config hashing.04 resolves instrumentation inputs without implied build-plumbing authorization.
- U-D5: active OMP/Gemini behavior/build/refresh and historical401 cause UNKNOWN; smoke NOT RUN. **Long-lived TCP, MCP session and access-token lifetimes are independent.** Fresh TCP does not prove fresh session; reconnect changing credential+session cannot discriminate stale A rejection from valid B+retained S_A mismatch. No new capture scope.
- Arithmetic does not establish implemented enforcement, maximal valid runtime fixtures, wire/collection behavior or approval. Proposed method-set restoration and severity additions need reviewed amendments, not invented preservation claims.

## 4. DESIGN

### 4.1 Concrete completion ownership and inner cutover (r1 C3)

Retain auth constructor, InboundHandler.ServeHTTP, NewLogger(io.Writer) and Logger.Log(Entry) signatures. Selected additive interfaces:

- `HTTPDiagnosticsOptions { Now func() time.Time; NewID func() string; Process HTTPProcessContext }`; `observe.NewHTTPDiagnostics(logger *Logger, opts HTTPDiagnosticsOptions) *HTTPDiagnostics`.
- `(*HTTPDiagnostics).Wrap(next http.Handler) http.Handler`, around **whole existing public mux** before routing/OAuth, with unchanged mux registrations.
- **r2 replaces r1's returned closure with a small ownership lease:** `(*HTTPDiagnostics).Claim(w http.ResponseWriter,r *http.Request) (http.ResponseWriter,*http.Request,HTTPOwnership)`. `HTTPOwnership` has private owner pointer/claimant boolean; `OwnsCompletion() bool` and `Finish()` methods. Passed by value; no per-request finalizer closure allocation. Only claimant lease may finalize; owner has finished guard, no double sink attempt.
- Private package `observe` types: `httpCompletionContextKey struct{}`, `httpOwner` (bounded diagnostic state, sole observer, native writer, timing/generator data, claimed/finished flags), and private carrier `interface { completionOwner() *httpOwner }`. Every factory facade promotes that private method. External packages cannot manufacture the key/carrier's unexported method. Owner/context/state never come from a request header.
- Claim detection order: typed context lookup for `*httpOwner`; otherwise private writer carrier lookup, reinstalling the context marker if necessary; otherwise allocate/initialize one facade+embedded owner, generate one ID/install header, attach marker, return claimant lease. Existing owner => incoming writer remains unchanged (do not strip intervening wrappers), same state and nonclaimant lease; **no observer/timer/generator/header/finalizer replacement**. Both marker/carrier presence are tested, including context loss with carrier intact. Trusted impossible owner mismatch is a diagnostic defect, never a reason to run auth again or claim a second owner.

**Exactly how current inner defer changes:** add `InboundOptions.Diagnostics *observe.HTTPDiagnostics`; constructor defaults to same implementation/current logger, Now and private generator conventions when absent. Public ServeHTTP calls Claim first; if claimant, immediately `defer lease.Finish()`. Keep existing captured-body/limit variables and current branch order. Replace local statusWriter field reads/writes with the **same owner observation**, using a read-only `observe.HTTPResponseObservation(ctx)` API returning status/bytes/typed write-failure evidence. This observation API is not the serialized HTTPDiagnostic and exposes no caller strings. Existing delivery classification may inspect its retained error internally but never serialize raw error text.

Split old defer responsibilities, not a second wrapper: keep the current inner metrics/count condition and typed legacy-result contribution in an inner defer registered **after** claimant Finish. Thus LIFO runs inner metrics/contribution first, then direct fallback Finish; with an outer owner, inner has no Finish, and outer Wrap finishes after all contributors. Remove old inner terminal logger call, second timer/generation and statusWriter creation. Current body-limit/post-dispatch classifiers remain, adapted to the shared observation; each actual admission branch contributes at the decision, so later delivery/legacy summarization cannot overwrite first rejection. Metrics keep original operation/count logic and must not count metadata/health/outer OAuth/early404.

`Wrap` likewise calls Claim, conditionally defers lease.Finish, then invokes next. **Finish intentionally uses the same logic on normal return and panic unwinding**, so no normal-return flag, recover or exception inspection is needed. It snapshots actual observed state, attempts one record, and lets the panic propagate. Claim's initial Now/NewID work before caller can install the defer preserves the existing unobserved early-panic window; do not promise otherwise. Nested wrappers' nonclaimant leases cannot finish. Zero-write normal return stays null/unavailable (later net/http default200 was not observed); no synthetic500/panic value. Handler-return/unwind timing, not per frame/connection lifetime; process death may lose the record.

Contributions: `observe.ContributeHTTP(ctx,HTTPDecision)` and typed session/operation/legacy-result functions accept closed enum/boolean/owned-reference evidence only, no arbitrary Fields/header maps/claims/error prose. Missing marker contributions are no-ops; supported direct entry establishes fallback first. First decisive rejection immutable; aggregate actual upstream outcome separate. Opaque static pass-through is not static acceptance; inner comparator proves it.

Add `Entry.HTTP *HTTPDiagnostic`, a closed concrete finalized snapshot, no maps/slices/interfaces/custom marshaling. Log HTTP path uses existing sink/mutex, not parallel stream; incompatible non-http_complete use is invalid. Existing ordinary MCP/backend/config producers and manifest remain unchanged.

**Required composed seams:** package-private trusted lifecycle-context `withDiagnosticDeps(context.Context,diagnosticDeps) context.Context`, `diagnosticDeps { Now func() time.Time; NewID func() string; LogWriter io.Writer }`; runContext reads startup hooks only, not inbound context/config/headers. Production defaults time.Now/crypto/stdout. Pass clock to diagnostics/InboundOptions.Now and new auth `WithNow(func() time.Time) Option`, whose adapter is verified `jwt.WithClock(jwt.ClockFunc(now))`; skew/validation policy unchanged. Generator covers independent request/process/session draws; writer feeds established logger. Log timestamp may remain actual UTC; token-helper time is not verifier time. These seams are **new work** requiring composed tests.

Create process context once per process startup and share across reloads/handlers; standalone default is one process-scoped context, not incarnation per constructor. Explicit contexts permit two-process tests.

### 4.2 Concrete capability factory, allocation policy and tests (r1 C2)

**Selected maintainable construction: checked-in offline-generated Go facades**, not64 manually maintained implementations, runtime generation/reflection, external package, new goroutine or body buffering. Bounded proposed paths under existing observe package: `internal/observe/httpwriter.go` (shared observer/delegates/factory entry), `internal/observe/httpwriter_facades_generated.go` (64 small method-set types/static switch), `internal/observe/httpwriter_test.go` plus generated test fixtures, and retained `internal/observe/internal/genhttpwriter/main.go` (small standard-library-only deterministic maintainer tool). Exact filenames are proposed new locations, not existing-file claims. Generator is maintained source supporting a necessary method-set matrix, not a throwaway scaffold. Normal build/test uses checked-in output and **does not execute generator**; no new runtime dependency or build-time code generation. T-D1 owns these, not another workstream.

One six-entry method descriptor table and one facade template emit masks0..63. Bits fixed as F=Flusher,H=Hijacker,P=Pusher,C=CloseNotifier,R=ReaderFrom,S=StringWriter. A type contains the shared `httpOwner` **by value** and only the optional methods selected by mask, forwarding to six shared observer helpers. Factory probes underlying supported interfaces once, caches delegates, uses generated static switch, allocates the chosen concrete facade, initializes embedded owner in place, and returns its writer interface plus pointer to embedded owner. No separate base-wrapper/state allocation, per-request method map/table, reflective call, duplicated large owner copy or allocated callback is permitted. Context attachment/owned ID storage remain necessary existing feature costs; do not promise zero total HTTP allocations. No request-time generator. The default base implements Header/WriteHeader/Write/Unwrap and private carrier, not the six optional methods. This preserves **both presence and absence**; r1's prohibition on any offline generator is explicitly superseded by this reviewed r2 selection.

**FlushError/ResponseController:** base additionally provides an intentional control adapter `FlushError() error`; it is not the six-bit legacy capability. If underlying supports FlushError, call that once and observe success/error; otherwise use actual Flusher; if neither, return `http.ErrNotSupported` without status/bytes/side effect. Never advertise Flusher solely because this adapter exists. For error-aware delegate failure without observable status, retain unavailable; do not write an invented200 before a failed native operation. Successful supported flush establishes implicit200 if none observed. Direct Flusher facade preserves current successful flush semantics and commits/observes its actual200 without duplicate writes. ResponseController traverses Unwrap for deadlines/full-duplex while FlushError remains intercepted. Tests cover error-aware-only delegate (no Flusher), Flusher-only, both and neither, plus failures and call precedence. This explicit control adapter exception is documented; no unsupported fake no-op capability is introduced.

Hijack/Pusher/CloseNotify forward exact values/errors/channels without goroutine. Successful hijack without observed response leaves null/unavailable; bytes written directly on connection are unobservable, bucket unavailable. ReaderFrom delegates exactly once, counts returned n/error (no io.Copy fallback/extra buffer); n>0 establishes implicit200, n=0 without other commitment stays unavailable. StringWriter delegates and observes actual count/error. Write tracks actual count/first write failure and implicit200. No per-frame events. First-WriteHeader-wins remains current policy, not incidental informational-response repair.

**MaxBytesReader:** expose trusted `observe.NativeHTTPWriter(ctx,w) http.ResponseWriter` returning owner's original pre-observer writer (or supplied w without owner). Replace only the existing MaxBytesReader writer argument with that native handle; retain existing body reader/limit logic. Do not pass a generated facade to net/http's native close-on-limit signaling. This explicitly preserves `inbound.go:310–314`, not an assumption that Unwrap alone does so.

**Single parameterized capability matrix:** independent test loop masks0..63, generated test-only fake writer factory composed from six tiny recording facets and base writer. No64 hand-written tests. Before wrapping, independently assert the fake's six public assertions equal expected mask; after wrapping assert the same six presence **and absence** bit-by-bit, using test constants/assertions independent of production factory. This catches a shared generator descriptor mistake instead of trusting generated mask names. Invoke each supported operation, check one delegate invocation, exact return/error/channel, status/byte/error evidence; unsupported operation assertions must be false, and unsupported ResponseController flush must return ErrNotSupported with zero effects. Exercise Write/WriteHeader/Unwrap and wrapper ownership for every mask. Separate focused cases cover FlushError precedence/error, ReadFrom0/n/error, StringWriter error, hijack no-status, streaming flush, native body-limit signaling/connection behavior, and no duplicate header/finalizer.

Generator test writes output only to t.TempDir and byte-compares checked-in production/test generated files, testing deterministic generation without modifying tree. Maintainer regeneration is explicit `go generate`/documented generator invocation; main's normal Go suite includes deterministic-output/matrix tests but does not regenerate files in place. Standard compiler/type checking plus benchmark/allocation assertions tied to the single-owner design must reject extra request-time facade/base/state/callback allocation; do not exempt compiled code from allocation policy or assert a numeric baseline before measurement. No application code/generator/tests are written by this planner.

### 4.3 Public ID versus owned ID; selected proposed exposure

| Candidate | Current policy | Selection |
|---|---|---|
| X-Request-ID | CorrelationID(header) accepts caller-valid32-lower-hex, otherwise generates; inner echo after route, not early OAuth. | Ineligible trusted identity. Preserve policy/echo placement, never log/export caller value. |
| Owned internal request ID | h.generator→CorrelationID(""); logger validates/no-remint; cloned downstream X-Mcp-Bridge-Internal-Request-Id scrubbed/owned. | Eligible generation policy: move **single** generation to claimant before mux/OAuth; reuse for inner/terminal/downstream. |

**Propose one response `Mcp-Diagnostic-Id`**, equal to moved owned ID; distinct header justified by unsafe public-ID reuse, **not another ID** or response alias for internal transport header. Owner context wins; caller/public IDs never participate. Scrub caller diagnostic/internal headers, trusted clone uses context ID; set/overwrite one diagnostic response value before any routing/auth/write. Contributor never regenerates/resets it. Entropy/format failure removes diagnostic header/request_id, sets unavailable; no caller/time/counter/hash fallback/retry. Public X-Request-ID remains independent and unchanged.

Ordinary origin responses including401/403/404/metadata/health expose it when available; no credential/capability/lookup API, CORS/expose-header/proxy change or promised wrapper readability.02 proves local equality/spoof/failure;05 real header=origin=Loki and absence/stripping/CORS/wrapper/unknown distinctions. Header contract remains unapproved pending affected checkpoint.

### 4.4 Complete physical-to-semantic mapping and severity correction (r1 C1/C5)

Literal r4 conflicts with preservation; selected amendments require reviewed canonical revision, not silent aliases/deletion:

| Physical legacy field | Selected mapping and change boundary |
|---|---|
| time | Semantic timestamp, UTC RFC3339Nano max30ASCII; no duplicate timestamp. |
| event,message | Preserve http_complete / `http request completed` (22ASCII); not http_request_complete or second stream. Other events/messages unchanged. |
| level | Retain existing owned admission503/delivery exceptions and existing success/4xx/internal behavior. **New r4-required change:** actual tool_error under HTTP200 changes HTTP terminal info→warn, based only on existing typed operation outcome. Unavailable response also gains warn per r4. Do not describe these as preserved behavior. |
| service,environment,version | Preserve mcp-bridge/unspecified/unspecified. Add diag_environment safe<=32 `[a-z0-9_-]` or unavailable, source_revision verified40–64 lower-hex or unavailable with build_state; no silent repurposing. |
| generation | Current HTTP producer fixed0; generic Entry uint64 other events unchanged. No policy-generation inference. |
| operation | Preserve request/discover/initialize/tools_list/tools_call/call_tool/search/describe/health/describe_tool/prompts_list/prompts_get/unknown; outer without capture unknown, no new body read. |
| duration_ms | Preserve Entry.Duration.Milliseconds() signed integer representation, owner-entry-before-generation through return/unwind; widened outer measurement is explicit design change. Add diag_duration_ms0..86400000/duration_capped for bounded semantic duration (negative clamp0/capped=true), not silently clamp old field/metrics. |
| outcome | Preserve unknown/success/failure/error/denied/busy/timeout/fallback/degraded; HTTP incomplete→overall failure. No HTTP200⇒tool success. |
| reason | Preserve exact legacy reasonVocabulary below. New outer code maps old unknown unless existing code actually applies; failure_reason separate. |
| stage | Preserve none/http_route/host_admission/origin_admission/client_auth/session_admission/request_limits/mcp_decode/mcp_dispatch/tool_admission/backend_dispatch/backend_response/response_delivery/unknown. Outer legacy unknown; new failure_stage outer_oauth. |
| http_status | Preserve integer100..599 **or explicit null**, status_state observed/unavailable. Null iff unavailable, never0; revise r4 omitted-status wording. |
| http_method | Semantic method preserves GET/POST/DELETE/HEAD/OPTIONS/PUT/PATCH/OTHER (max7); no narrowed replacement/duplicate method. |
| route | Preserve mcp/not_found exact /mcp classification. path_class/route_outcome separately supply metadata/health/auth/direct finite dispatch; never raw path/query. |
| http_outcome | Preserve success/failure/incomplete/unknown, actual observer evidence separate from upstream. |
| timing_scope,http_bytes_bucket | Keep http; empty/le_1k/le_16k/le_256k/le_1m/gt_1m/unavailable, actual successful-write thresholds0/1024/16384/262144/1048576; unobserved hijacked bytes unavailable. |
| request_id,correlation_state | Moved owned ID, available/unavailable/no remint. Generic Fields acceptance of valid is not current HTTP producer; typed path has no override/fallback. |
| canonical error.class | Core closed correlation_unavailable/client_disconnected/unknown when actual legacy rule selects it; not optional error_detail. |
| execution/config extras | HTTP producer has no server/backend/tool identity/arbitrary extras. Keep existing MCP/backend/config summaries/contracts elsewhere, not in HTTP record; export still reviewed. |

Legacy reasonVocabulary (`log.go:229–238`) preserved exactly: none, unknown, host_rejected, origin_rejected, client_auth_failed, session_auth_failed, session_capacity, request_too_large, route_not_found, backend_unavailable, backend_auth_failed, backend_busy, deadline_exceeded, upstream_protocol_error, unsupported_capability, invalid_config, client_disconnected, tool_reported_error, unknown_tool, invalid_arguments, catalog_changed, unsupported_schema, malformed_request, invalid_request, method_not_found, request_cancelled, internal_error, correlation_unavailable.

Every r4 semantic field retained, with existing finite sets/bounds: diag_schema oauth_diag_v1; build_state; process_state/process_id; operation_relation/operation_id_state/operation_id; path_class/route_outcome/status_state; failure_stage/failure_reason; auth_mode/credential_state/scope_state; session_header/session_state/binding_relation; session_ref_state/session_ref; challenge_kind/challenge_target/metadata_target_registered; diagnostic_policy_revision oauth-diag-policy-v1; effective_policy_revision_state/effective_policy_revision; policy_source/oauth_enabled/static_configured/scope_required/resource_path_mode/session_binding/clock_skew_seconds; upstream_state; error_detail_state/error_detail. Timestamp/event/message/method/request/bucket bind old physical fields; environment/source→diag_environment/source_revision; bounded duration→diag_duration_ms/duration_capped. **session_binding=bearer_sha256 is only a fixed mechanism label, never a digest value**; stored token fingerprint is never emitted (security L1).

First-cause mapping: observed outer absence→outer_oauth/credentials_missing; observed malformed bearer→bearer_malformed; generic Parse failure→jwt_invalid_other; explicit unusable keyset→jwt_key_unavailable; verified explicit audience denial→jwt_audience_invalid; scope→scope_insufficient. Host/origin/size→host/host_denied,origin/origin_denied,body_limit/request_too_large. Actual selected static denial→inner_credential/static_invalid, otherwise unavailable, not guessed mode. Absent lookup→session/session_absent; existing mismatch→session/session_fingerprint_mismatch; capacity→capacity/session_capacity; fallback→route/route_not_found. Protocol/upstream only actual typed outcomes, no new header rejection. First decisive rejection immutable; legacy delivery classification separate; none/none means no observed rejection, not operation success. Wire/order unchanged.

**Severity implementation/test decision:** compute existing owned-cause severity first, preserving its known admission/delivery handling. Only promote info to warn when actual aggregate upstream_state=tool_error under observed200, or response evidence unavailable per r4; never downgrade existing warn/error or infer tool error from status/payload text. Existing backend/MCP completion severity is not rewritten. Ticket04 owns actual typed outcome contribution and targeted HTTP severity change;02 owns shared severity projection. RED: existing composed fake backend returns IsError with HTTP200 and admitted auth; assert exactly one HTTP completion with actual upstream_state=tool_error and level=warn, while status/body/challenge and existing operation/backend records remain frozen. Current info fails level assertion (and missing contribution may separately fail); same assertions GREEN. Include successful HTTP200→info and capacity503→warn/client-disconnect exception regressions, proving no status-only overcorrection. AC-D2/D5/D9 cover it.

### 4.5 Delete-first typed serialization (r1 C4, security M1)

Generic sensitive()/redact(), full substring set, config manifest and non-HTTP protections unchanged. No blanket exemption for maps named diag/schema/session/header. Current producer Fields are trusted internal but must still be collision-tested; no claim of a proven present exploit.

**Normative delete-first contract:** the HTTP schema owns the complete62-key universe in §4.6 (59core+3optional, including shared/base names), not just keys containing sensitive substrings. At the serialization boundary unconditionally remove/clear **every owned key** from any legacy/redacted candidate before inserting validated data—even keys whose typed value is omitted, unavailable or invalid. This includes request_id/operation_id/process_id/build_state/auth_mode as well as diag_schema/session_header/session_ref and their states. A redacted placeholder is not a valid typed value or a fallback. Also drop all non-schema extras on the typed HTTP path. No stale Entry.Fields/base value may survive because validation chose not to emit its typed counterpart.

Concrete allocation-aware implementation: use a fixed-slot HTTP projection/encoder under existing logger mutex, not a second per-request overlay map. Reset all owned slots to absent first (`clearOwnedSlots` is unconditional logical deletion of the whole schema); legacy/redacted candidate projection cannot populate an owned slot from Entry.Fields. Reconstruct required legacy fields once from authoritative closed legacy projection sources and actual owner snapshot, then insert independently validated typed fields into their fixed slots. Shared request/status/canonical-error fields come only from that snapshot, never Fields. If adapting the existing map-based projection temporarily, it must literally delete all owned keys and reject non-schema extras before insert; that is a correctness description, **not permission for an additional overlay map/allocation or recomputation in the shipped path**. Implement the selected fixed-slot path directly; ordinary non-HTTP Log remains unchanged. Fixed schema defaults are inserted only as schema-defined constants or truthful unavailable states, never copied placeholders.

Optional/conditional invalid or absent typed value leaves its cleared slot absent; required invalid core makes the record invalid and fails acceptance, not fallback. Explicit null status is freshly inserted for unavailable snapshot. Canonical error.class is recomputed by its closed legacy rule after clearing, not copied arbitrary error. Allowed typed enums/references may survive generic redaction only through their dedicated validator. No interface/map/slice/custom marshaler, arbitrary errors/claims/headers/paths/queries/URLs/payload. Numbers finite ints/bools/documented null; strict safe alphabets. error_detail closed class/code/retry enums as r4, no causes/prose.

Security RED/GREEN collisions (02/T-D1/T-D3, AC-D3/D5/D6): hostile same-name Fields for **all owned keys**, individually and combined; request/process/session entropy unavailable; operation none/multiple/unavailable/budget omission; build/effective-policy unavailable; invalid typed enum/ref; literal `[REDACTED]`/redacted placeholders; nested error/custom-marshaler injection. Assert complete emitted bytes have no hostile content and values/states only from owner/schema; omitted IDs/revisions/ref/error_detail must stay absent, status null stays explicit. Include unredacted-name request_id/process_id/auth_mode and substring-redacted diag_schema/session_ref/header cases. Existing generic redaction tests must still pass. Invalid core cannot become valid by using supplied fallback.

Encode once into bounded<=4096 storage including LF, measure actual encoded size and make one sink write attempt. No duplicate encoding just to size, unbounded copies or per-request generic map on typed path. Optional omit order error_detail→operation_id→session_ref with truthful omitted_for_budget; core remains. Overflow/invalid core blocks release; runtime stays nonfatal to auth/response and suppresses unsafe output rather than emitting valid-looking truncation, no retry/recursion. Sink error/short write one attempt; encoder reset only next independent event. Runtime enforcement unimplemented.

### 4.6 Exact retained executable sizing evidence

**Schema/key/value bounds and exact evaluator unchanged from r1**, including fixed HTTP generation0, full legacy enums and canonical error. r2 changes ownership/facade/projection implementation and corrects severity attribution only; info/warn/error were already covered. Parent need not rerun arithmetic for these changes. If later schema/alphabet/legacy field bounds change, parent must rerun; never delete fields/weaken ceilings silently.

Parent pure-data eval **PASS**, compact UTF-8 bytes including LF: core **2048**/3584 (headroom1536), optional incremental **200**/512 (312), full **2248**/4096 (1848),59core/62full fields; coherent mismatch core1825/full1973, unavailable-request envelope2000. No source/app/test/config inputs, network, writes or runtime execution. Both assertions passed. This proves proposed-map arithmetic only: serializer/max-valid fixtures/normalized bytes/Loki remain NOT RUN.

Envelope maximizes independent fields and deliberately includes mutually exclusive states/IDs/error: conservative upper bound, **not valid emitted record**. Includes all actual HTTP legacy fields, longest finite enums, null4bytes, minimum signed legacy millisecond value,64-char revisions,32-char refs, actual fixed message and generation0. Coherent mismatch fixture also computed. Exact maximal valid implementation-state fixtures remain required TDD before AC-D5 closes.

Allowed content excludes quotes/backslash/control/non-ASCII/`<>&`/U+2028/U+2029; timestamp/message fixed safe ASCII. Accepted-value escape expansion is zero even with Go HTML escaping; quoting, delimiters, braces, LF are counted. Six-byte escaped arbitrary input is rejected, not silently truncated. Complete-byte hostile-input privacy tests remain required.

```python
import json
J = lambda x: json.dumps(x, ensure_ascii=True, separators=(',', ':'), sort_keys=True)
B = lambda x: len((J(x) + '\n').encode('utf-8'))
L = lambda s: max(s.split('|'), key=lambda v: (len(J(v)), v))
failure_reasons = 'none|credentials_missing|bearer_malformed|jwt_malformed|jwt_signature_invalid|jwt_key_unavailable|jwt_expired|jwt_not_yet_valid|jwt_issuer_invalid|jwt_audience_invalid|jwt_invalid_other|scope_insufficient|static_invalid|auth_mode_unavailable|session_header_missing|session_absent|session_fingerprint_mismatch|host_denied|origin_denied|request_too_large|session_capacity|route_not_found|protocol_rejected|upstream_transport_failed|upstream_auth_failed|tool_error|unavailable'
legacy_reasons = 'none|unknown|host_rejected|origin_rejected|client_auth_failed|session_auth_failed|session_capacity|request_too_large|route_not_found|backend_unavailable|backend_auth_failed|backend_busy|deadline_exceeded|upstream_protocol_error|unsupported_capability|invalid_config|client_disconnected|tool_reported_error|unknown_tool|invalid_arguments|catalog_changed|unsupported_schema|malformed_request|invalid_request|method_not_found|request_cancelled|internal_error|correlation_unavailable'
legacy_stages = 'none|http_route|host_admission|origin_admission|client_auth|session_admission|request_limits|mcp_decode|mcp_dispatch|tool_admission|backend_dispatch|backend_response|response_delivery|unknown'
core = {
 'time':'2000-01-01T00:00:00.123456789Z', 'level':L('info|warn|error'),
 'event':'http_complete','message':'http request completed','service':'mcp-bridge',
 'environment':'unspecified','version':'unspecified','generation':0,
 'operation':L('unknown|request|discover|initialize|tools_list|tools_call|call_tool|search|describe|health|describe_tool|prompts_list|prompts_get'),
 'duration_ms':-9223372036854,'outcome':L('unknown|success|failure|error|denied|busy|timeout|fallback|degraded'),
 'reason':L(legacy_reasons),'stage':L(legacy_stages),
 'http_status':None,'http_method':L('GET|POST|DELETE|HEAD|OPTIONS|PUT|PATCH|OTHER'),
 'route':L('mcp|not_found'),'http_outcome':L('success|failure|incomplete|unknown'),
 'timing_scope':'http','http_bytes_bucket':L('empty|le_1k|le_16k|le_256k|le_1m|gt_1m|unavailable'),
 'request_id':'a'*32,'correlation_state':'unavailable',
 'error':{'class':L('correlation_unavailable|client_disconnected|unknown')},
 'diag_schema':'oauth_diag_v1','diag_environment':'a'*32,'source_revision':'a'*64,
 'build_state':L('verified|unavailable'),'process_state':L('available|unavailable'),'process_id':'b'*32,
 'operation_relation':L('none|single|multiple|unavailable'),
 'operation_id_state':L('available|not_applicable|unavailable|omitted_for_budget'),
 'path_class':L('mcp|metadata_root|health|other'),
 'route_outcome':L('oauth_protected|inbound_direct|metadata|health|not_found|other|unavailable'),
 'status_state':L('observed|unavailable'),'diag_duration_ms':86400000,'duration_capped':False,
 'failure_stage':L('none|route|outer_oauth|inner_credential|host|origin|session|body_limit|capacity|protocol|upstream|unavailable'),
 'failure_reason':L(failure_reasons),'auth_mode':L('oauth|static|none|bypass|unavailable'),
 'credential_state':L('not_evaluated|missing|accepted|rejected|unavailable'),
 'scope_state':L('not_evaluated|not_required|accepted|rejected|unavailable'),
 'session_header':L('absent|present'),'session_state':L('not_evaluated|not_supplied|absent|existing|created|unavailable'),
 'binding_relation':L('not_evaluated|match|mismatch|unavailable'),
 'session_ref_state':L('available|unavailable|not_applicable|omitted_for_budget'),
 'challenge_kind':L('none|resource_metadata|insufficient_scope|bearer_only|other|unavailable'),
 'challenge_target':L('none|root_well_known|path_appended_well_known|other|unavailable'),
 'metadata_target_registered':L('yes|no|not_applicable|unavailable'),
 'diagnostic_policy_revision':'oauth-diag-policy-v1',
 'effective_policy_revision_state':L('verified|unavailable'),'effective_policy_revision':'c'*64,
 'policy_source':L('defaults|environment|file|mixed|unavailable'),
 'oauth_enabled':'unavailable','static_configured':'unavailable','scope_required':'unavailable',
 'resource_path_mode':L('origin_only|path_bearing|unavailable'),
 'session_binding':L('bearer_sha256|unavailable'),'clock_skew_seconds':60,
 'upstream_state':L('not_attempted|succeeded|transport_failed|auth_failed|tool_error|multiple|unavailable'),
 'error_detail_state':L('included|not_applicable|omitted_for_budget')
}
optional = {'operation_id':'d'*32,'session_ref':'e'*32,
 'error_detail':{'class':L('validation|dependency|admission|protocol|tool|internal|unavailable'),
                 'code':L(failure_reasons),'retry':L('not_attempted|observed|unavailable')}}
full = dict(core, **optional)
# Conservative optional charge includes commas, not a second brace pair or LF.
optional_bytes = B(full)-B(core)
# Coherent maximum-string mismatch fixture: no unavailable correlation error,
# accepted OAuth+scope, existing mismatched session, zero downstream attempts.
fixture = dict(full)
fixture.pop('error')
fixture.update(level='warn',operation='request',duration_ms=86400000,outcome='failure',
 reason='session_auth_failed',stage='session_admission',http_status=401,http_method='DELETE',
 route='mcp',http_outcome='failure',http_bytes_bucket='le_1k',correlation_state='available',
 build_state='verified',process_state='available',operation_relation='none',
 operation_id_state='not_applicable',path_class='mcp',route_outcome='oauth_protected',
 status_state='observed',duration_capped=False,failure_stage='session',
 failure_reason='session_fingerprint_mismatch',auth_mode='oauth',credential_state='accepted',
 scope_state='accepted',session_header='present',session_state='existing',binding_relation='mismatch',
 session_ref_state='available',challenge_kind='bearer_only',challenge_target='none',
 metadata_target_registered='not_applicable',effective_policy_revision_state='verified',
 policy_source='environment',oauth_enabled=True,static_configured=False,scope_required=True,
 resource_path_mode='path_bearing',upstream_state='not_attempted',error_detail_state='included')
fixture.pop('operation_id')
fixture['error_detail']={'class':'admission','code':'session_fingerprint_mismatch','retry':'not_attempted'}
fixture_core={k:v for k,v in fixture.items() if k not in optional}
# Presence-state validation examples: unavailable request removes ID but keeps
# canonical error; no core ever disappears just to pass the budget.
unavailable=dict(core)
unavailable.pop('request_id')
print(J({'core_upper':B(core),'optional_upper':optional_bytes,'full_upper':B(full),
 'core_fields':len(core),'full_fields':len(full),'coherent_mismatch_core':B(fixture_core),
 'coherent_mismatch_full':B(fixture),'unavailable_envelope':B(unavailable),
 'core_headroom':3584-B(core),'optional_headroom':512-optional_bytes,'full_headroom':4096-B(full),
 'longest_legacy_reason':L(legacy_reasons),'longest_legacy_stage':L(legacy_stages),
 'longest_failure_reason':L(failure_reasons)}))
print('per_field_charge=' + J({k:len(J(k))+1+len(J(v))+1 for k,v in core.items()}))
assert B(core)<=3584 and optional_bytes<=512 and B(full)<=4096
assert B(fixture_core)<=3584 and B(fixture)<=4096
```

Field charge = encoded key+colon+encoded value+comma; sum+2=B(core) (braces+LF−last comma). Optional=B(full)−B(core), no duplicated delimiter/newline. Generation0 is selected HTTP producer contract; other events' uint64 max does not apply. No omission needed to fit.

## 5. ACCEPTANCE CRITERIA AND FULL DELTA MAPPING

All IDs preserved. Source research complete for supplied contracts; new behavior not implemented. Owner numbers refer existing tickets, not dispatch authority.

| AC / R coverage | Existing→missing delta; local owner/method | External verification / remaining gap |
|---|---|---|
| AC-D1 / R-D1,D4 | Inner completion→02 whole-mux owner/private marker+carrier/lease/direct fallback and exact inner defer cutover; generated64-mask presence+absence/delegation/native-limit/FlushError/streaming/panic tests; real lifecycle, no added metric counts. |05 actual composition/traversal; no crash guarantee. Capability/panic runtime proof remains NOT RUN. |
| AC-D2 / R-D2,D5 | Old finite/generic/merged401→02 actual outer states;03 absent/mismatch/legal no-header;04 actual route/upstream plus tool-error HTTP200 warn change. Generic/unavailable for unverified causes. |05 preservation; U-D6 exhaustive Parse/SDK cause map still blocked, not inferred. |
| AC-D3 / R-D3,D4,D6 | Eligible owned ID/ineligible caller public→02 pre-OAuth Mcp-Diagnostic-Id same owned ID, spoof/entropy/delete-first collision tests;03 refs;04 process/provenance/op ownership. |05 real header=origin=Loki; U-D2/D4 map/capture and runtime policy unknown; no new HEAD deployment attestation. |
| AC-D4 / R-D2,D5,D8 | Isolation baseline not retained-token renewal proof→03 A initialize+initialized/tools-list using negotiated protocol/session headers; valid B+retained S_A401 then A control still works; fresh B success; P/Q/static/principal/invalid/scope/reap negatives. |05 controlled traversal; U-D5 clients/TCP/session/token hypothesis remains unknown, not scope expansion. |
| AC-D5 / R-D4,D6 | Existing allowlist/redaction/null/message→02–04 fixed typed/delete-first schema, actual cap/max-valid-state/optional/complete-byte tests;01 arithmetic PASS only.04 targeted severity RED/GREEN, mechanism label not digest. |05 all normalized/exported bytes/equality and retention/access; arithmetic cannot close runtime or external portions. |
| AC-D6 / R-D1,D3,D6 | Sink one-attempt→02–04 independent/simultaneous request/process/session entropy failure, hostile fallback collision, short/broken sink, no-status/hijack/panic, counted attempts/no retries/recursion/auth change. |05 actual failure monitor/recovery; U-D2/D3/D4 unknown. Broken sink cannot promise self-report. |
| AC-D7 / R-D3,D5,D9 | Operator baseline→02–04 incremental docs/changelog and two-reviewer fixture rubric: outer vs session, absent vs mismatch, generic JWT, process/policy, hidden concurrency; TCP≠session≠token. |05 executable reviewed actual-map query/visibility,2–10min/50rows-query/200bundle; exact capture U-D4 may block join. |
| AC-D8 / R-D7,D6,D9 | Previous stdout acceptance/historical Loki status-message only→local origin fixtures, no mock substitute. |05 actual stdout/capture/normalizer/queried-Loki outer/scope/absent/mismatch/success+relevant protocol/upstream; every applicable typed core equal, labels/retention/access/monitor; named U-D2/D3/D4 owners unknown. |
| AC-D9 / R-D1,D6,D8 | Historical completed suite/race92.0%/reviews→Main exact final full/race/vet/coverage>=90%, capability/generator/lifecycle/metric/severity/privacy/code/security scope regressions after approved changes. |05 all-AC gate; no no-new-failures substitute or local-only deployed claim. |
| AC-D10 / R-D9,D1 | Breakdown/seam approval only→01 reconcile r2, independent spec/task/security finding disposition and explicit affected requirements/design/tasks checkpoints. |05 separate authorized release/probe/capture; no approval asserted here. |

Complete requirement index: R-D1→AC-D1/D6/D9/D10 [01 facts/gates,02 owner,03/04 inherited,05 final]; R-D2→D2/D4 [02 outer,03 session,04 actual outcomes/severity,05 fields;U-D6]; R-D3→D3/D6/D7 [02 ID,03 refs,04 process/ops,05 U-D2/D4 capture]; R-D4→D1/D3/D5 [02 schema,04 provenance,05 U-D3 runtime]; R-D5→D2/D4/D7 [03,05 controls;U-D5 optional renewal]; R-D6→D3/D5/D6/D8/D9 [02–04 local,05 normalized/failure]; R-D7→D8 [05]; R-D8→D4/D9 [02–04 embedded TDD,05 Main final]; R-D9→D7/D8/D10 [01 approvals,02–04 docs,05 operators/owners]. None waived.

**Ticket01 closure distinction:** established research includes HEAD/deployment rule, six-ticket reuse, logger/owner/ID/metric facts, APIs/available-vs-absent seams, arithmetic sub-result and bounded external-role/operator handoff. Source research need not restart. Selected header/ownership/generated-facade/delete-first/severity/seam/schema/ref amendments still need independent review/reconciliation/checkpoints. Method-set/runtime/max-valid fixture/Loki proof is later work; exhaustive Parse/SDK cause facts remain affected unresolved acceptance. Existing JWKS/capture/inner Now confirmed; new composed entropy/verifier clock only proposed. Do not check all “reviewed” ticket01 criteria merely because r2 exists; coordinator records exact evidence versus proposed amendment status.

## 6. TASK LEDGER — bounded proposals, implementation still gated

| ID / owner / dependencies | Inputs and allowed deliverable | RED/GREEN, validation and stop condition |
|---|---|---|
| T-D0 /01/none | Source/auth-seam+r4+r2+independent r1 findings; coordinator planning reconciliation preserves R/AC/T IDs, records explicit compatibility changes/finding disposition. No source research restart. |No research tests. Re-review spec/tasks and affected explicit user checkpoints required; unproved detailed Parse/SDK map blocks affected exhaustive claim. Conflict/budget change returns planner, not weaker AC. |
| T-D1 /02 foundation;03 refs;04 provenance /T-D0+gates | Existing observe Entry/Log/inbound/options/serve; concrete marker/carrier/lease/defer cutover; generated facade factory/generator/fixtures bounded §4.2; fixed-slot delete-first encoder/shared seams. No generic redactor/manifest rewrite/extra stream. |Engineer RED missing outer/duplicate/fallback/ID/capability/collision/core observation; same GREEN. Include all mask absences/native limit/FlushError; main checks generator determinism/allocation structure. Stop wire/metric/core/privacy or unresolved selected-interface defect. |
| T-D2 /02 outer;03 session;04 routes/outcome/severity /T-D1 | Actual existing auth/inner/lifecycle/tools boundaries, record-copy refs, safe provenance;04 owns new actual tool_error200 info→warn, not backend rewrite. |Targeted §4.4 severity RED/GREEN plus branch/ref/order tests with frozen401/403/413/503. Unknown type→generic/unavailable, exhaustive gap still blocked; no status-derived missing-header or auth repair. |
| T-D3 /embedded02–04 /RED before corresponding T-D1/D2; final GREEN after | Existing loopback/local JWKS/fake backend plus new trusted hooks. Parameterized64-mask/ownership/defer/collision/entropy/size/privacy/lifecycle/profile tests; complete raw/exportable op bytes. |Unchanged behavioral assertions GREEN, old regressions unchanged. Test generated fixture itself independently before wrap. Raw/encoded sentinel/hash scans plus allowlist; no real issuer/sleeps/client probe/sensitive evidence export. AC weakening/privacy failure stops release. |
| T-D4 /docs02–04;final05 /T-D0+corresponding verified examples | Existing operator logging/auth docs/changelog; bounded docs discovery resolves exact existing files. Document tool-error severity change, generated method-set maintenance, header/map/lifetime/limits. |Rubric RED indistinguishable/false exact join/client-cause claim; GREEN two independent reviewers classify fixtures and TCP/session/token unknown correctly. Actual query awaits actual reviewed map. |
| T-D5 /05 /03+04→02, owners+separate live authorization | Real capture/collector/Loki and header boundary evidence, pushed-vs-local baseline; no infrastructure edits. |Actual matrix typed equality/complete-byte privacy/retention/access/known failure monitor/recovery. Local/mocked data cannot close; missing external evidence blocks external ACs, not waived. |
| T-D6 /Main final05 /T-D1–T-D5+reviews | Full focused/suite/race/vet/coverage/lifecycle/metrics/generator/capability/severity/security/code/spec/task evidence once changes settle. |Full PASS>=90% full-suite source-line coverage, not package averages/historical/no-new-failures. Main verifies once; no competing specialist suites. Required local/external gap prevents final-complete verdict. |

Exact planned existing commands, not executed by planner:

- `go test ./cmd/mcp-bridge -run '^(TestServeOAuthWire|TestServeLoggingExecution|TestServeLoggingFailures|TestServeLoggingConcurrent)$' -count=1`
- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...`
- `go run ./cmd/covcheck`

New focused names anchored after tests exist, not invented today. Standard suite covers parameterized generated-facade/deterministic-generator/security/severity tests. Lifecycle procedure extends real runContext startup, reload success/failure, shutdown/cancellation with diagnostics; compare frozen wire/metric/lifecycle behavior. Clock tests use verified new adapter for expired beyond60s and within30s controls without sleeps, separate InboundOptions.Now for reap/duration. Metric scrape procedure uses existing operations /metrics/access/rate controls before/after identical admitted operations and outer401/metadata/health/not-found; compare seven names/types/bounded label sets/increments and no new request counts. No new scrape route.

## 7. OBSERVABILITY

T-D1/T-D2 implementation, T-D3 local proofs, T-D4 operators, T-D5 external, T-D6 final; AC-D1/D2/D3/D5/D6/D7/D8/D9 measurable. Mux/OAuth/inner credential/session/route and observed operation aggregates contribute one established stdout JSON completion; lifecycle/MCP/backend/config streams retained. UTC time/level/service/environment/version/event/message and request/process/optional op/session/duration/error use complete §4 mapping. No raw credentials/claims/payload/error or hash values; bearer_sha256 only mechanism label.

Shipping stdout→existing runtime capture→collector/normalizer→queried Loki; deployed config/map/component names U-D2 UNKNOWN until05. Retain known admission/delivery severity exceptions. **New r4-required HTTP logging behavior** is observed tool_error200 info→warn, separately tested, plus unavailable-response warn; no assertion this already exists or changing operation/backend severity. Expected denials not duplicate streams; no per-frame/lifecycle-noise/new retry events. Broken sink cannot promise self-report; process death can lose record.

Loki: existing bounded approved labels only; IDs/revisions/users/paths/errors remain JSON fields, never new labels/high-cardinality metadata index. Retention/access unchanged but actual policy unknown until05. Existing ingestion monitor/runbook must show known failure/test and recovery; missing signal blocks external AC-D6/D8. No new queue/stack/retry.

Prometheus unchanged-contract exception: existing operations /metrics, private registry; counters bridge_requests_total{server,operation,outcome}, bridge_catalog_refresh_total{server,outcome,reason}, bridge_config_reload_total{outcome,reason} in events; gauges bridge_catalog_tools{server}, bridge_catalog_prompts unlabelled in entries, bridge_catalog_age_seconds{server} in seconds, bridge_readiness{server} state. Seven collectors, no histograms/buckets. No new throughput/error/saturation/queue/dependency series or ID/reason label. T-D3/T-D6 scrape/increments→AC-D9 including uncounted outer boundaries remain uncounted.

Dashboard/alerts scope exception: T-D4 bounded queries/rehearsal, not fictitious deployed panel; T-D5 identifies actual exploration/ingestion-health monitor/runbook and proves signal/recovery, no new infra/threshold. Valid JSON/local failure T-D3→AC-D5/D6; deployed delivery/equality/retention/monitor T-D5→AC-D8; scrape T-D3/T-D6→AC-D9; operator interpretation T-D4/T-D5→AC-D7. External unknown is blocked, not N/A.

Operator bounds: start2min UTC window, reasoned expansion<=10min,<=50rows/query,<=200/bundle; validated owned32-lower-hex exact same-service/schema parsed-field join only after reviewed real map. No arbitrary public-text interpolation/cross-service search. Header absence/stripping/CORS/wrapper/unknown require observed boundary evidence; hidden concurrent matches AMBIGUOUS, not exact join. Minimize session pseudonyms, evidence omitted_for_privacy distinct from server entropy. TCP≠MCP session≠access token; stale A outer rejection and valid B+retained S_A mismatch are distinct controls, neither proves OMP/Gemini implementation/history. No new capture authorization. Export only allowlisted reviewed terminal/op rows, no raw traces/header/challenges/errors/config. External collection/runtime/proxy-client roles required, named owners UNKNOWN; real renewal voluntary optional U-D5, not local prerequisite.

## 8. RELEASE AND RECOVERY

No release/migration here. After r2 reconciliation/review/affected checkpoints:02 foundation then independent03/04, Main local TDD/privacy/capability/generator/severity/lifecycle/metrics/full-suite evidence; separately authorized release/live capture;05 actual traversal/all-AC closure. User HEAD rule preserved; unpushed changes not called deployed. External unknowns block external acceptance, not separately approved local work.

No persistent migration/token rotation/irreversible auth change. Session refs die with live state, process change may discard sessions and is not remedy. Owner rollback uses known-good app artifact/process, removes diagnostic header/fields and loses observability without weakening isolation; no collector/proxy/deployment rollback implementation here. Emitted records follow verified existing retention/access, no identifier index/tombstone. Privacy leak stops release/security escalation.

## 9. CRITIC HANDOFF AND r1 FINDING DISPOSITION

New finding IDs below are **r1 review C1–C5**, distinct from preserved historical r4 C1–C8:

| Finding | r2 amendment / evidence; affected tasks/AC |
|---|---|
| r1 C1 MAJOR | Accepted source correction: current HTTP tool_error200 info; proposed warn is r4-required change, not preservation. §3/§4.4/§7 corrected; targeted one-HTTP-event RED/GREEN plus successful200 and admission/delivery regressions. T-D2/T-D3 ticket04/shared02, AC-D2/D5/D9. |
| r1 C2 MAJOR | §4.2 selects retained standard-Go offline generator, checked-in64 facades/static switch, single embedded owner allocation/no callbacks or runtime reflection/generation, bounded files; independent parameterized presence+absence/delegation matrix, generator determinism, FlushError and native MaxBytesReader tests. T-D1/T-D3, AC-D1/D6/D9. Reviewer re-review required, not claimed closed. |
| r1 C3 MAJOR | §4.1 private key/state/carrier detection, small claimant lease, marker/carrier reuse, exact inner defer LIFO/metrics/observer/log removal/direct fallback; no same-request second owner, normal/panic finalization and initial uncovered window. T-D1/T-D3, AC-D1/D6. |
| r1 C4 MINOR / security M1 MEDIUM | §4.5 unconditional clear/delete every owned key before validated insert, no fallback on absent/invalid/unavailable/budget values, no redacted placeholder; all-key hostile collision/entropy tests and generic redaction regressions. Fixed slots avoid extra map. T-D1/T-D3, AC-D3/D5/D6. |
| security L1 LOW | §4.4/§7 bearer_sha256 explicitly fixed mechanism label only, never credential digest. Complete-byte exclusions unchanged. T-D4, AC-D5/D7. |
| r1 C5 AMENDMENT | Full existing compatibility changes preserved in §4 mappings and below; literal r4 not implementation-ready. Reconcile canonical revision, independent spec/task reviews, affected user checkpoints before02–04. |

Historical r4 findings retained: C1 external blockers/unknown owners;C2 suite-wide T-D0 evidence;C3 owner/contribution/direct fallback;C4 diagnostic vs effective-policy provenance;C5 real lifecycle;C6 public-ID planner selection;C7 full PASS>=90%;C8 core/optional budgets/precedence. Historical SEC-LOW-1..4 pseudonym minimization/core non-displacement/op-export privacy/header-visibility remain. No requirement/AC/finding silently dropped.

Explicit reviewed amendments: time/http_complete/full http_method semantic bindings; null status; old environment/version plus typed provenance; legacy duration plus bounded diagnostic duration; **new tool_error200 warn and unavailable-response severity** while retaining exceptions; generic JWT/unknown SDK limits; generated capability restoration/FlushError/native signaling; private ownership lease/inner cutover; delete-first typed serialization; header addition; composed clock/entropy/writer seams. R2 supersedes r1's closure-return/no-offline-generator details; both were unapproved proposals. No prior implementation approval exists to invalidate; r1 technical review rejected, r2 needs renewed review. Literal-r4 readiness assumption invalidated, not historical OAuth/logging approvals.02–04 remain gated.

Reviewer questions: Does generated method-set factory/test independence avoid correlated mask errors and extra owner allocation? Are native limit/FlushError/status/panic/defer behaviors concrete and nonintrusive? Does clear-first projection prevent every fallback including harmless-looking unredacted keys? Is tool-error HTTP severity accurately classified as new and covered by targeted unchanged GREEN? Does unchanged arithmetic still cover all fields while runtime/Loki/JWT/SDK proofs remain explicitly blocked? Is source research completion distinguished from proposed amendment review/checkpoint permission? TCP/client hypothesis remains unknown without new scope.
