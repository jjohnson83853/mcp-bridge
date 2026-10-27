# Approved diagnostics branch inventory

Baseline: 7a40ed5246eb9afd1af1001f70b58e71138aa25d (main; only repository CI/docs changed after retained e621 source baseline). Authenticated bridge discovery and bounded source reads succeeded in this continuation against catalog 6021d87ff9798be6f78e659b565ca34d429717aae2b4b223212ca8d66bc26e0c. Earlier 401 observations remain historical, not current access evidence or incident diagnosis.

## Actual owned branch inventory

- internal/auth/middleware.go:94–109: bypass first; absent Authorization or wrong scheme returns401. Scheme check is case-insensitive literal `bearer `; whitespace-only suffix reaches token-shape handling after trim, not the absent-header branch.
- :112–137: token dot count other than2 delegates only when static fallback enabled, otherwise401. Two-dot input obtains KeySet then parses JWT; provider errors and Parse errors return401 without static fallback. Preserve generic jwt_invalid_other unless an authoritative returned typed outcome supports a specific approved classification. Retained pinned jwx2.1.7 representative runtime predicates are not exhaustive taxonomy.
- :139–163: verified audience denial returns401; scope denial after accepted credential returns403; success attaches existing claims and delegates. All outer401 branches retain existing resource-metadata challenge/body;403 retains insufficient_scope challenge/body.
- internal/service/inbound.go:603–620: inner authentication requires exactly one Authorization value, two Bearer fields, then claims-context acceptance or existing constant-time static comparison. :317–371: inner denial401/bare Bearer; then absent session entry and existing fingerprint mismatch each401/bare Bearer. Empty session header does not itself reject; capacity may503 before SDK delegation. Preserve actual admission order and statuses.
- :372–390: after delegation, body limit precedes write failure, then status>=400 selects existing mcp_dispatch. This is not a typed SDK cause and cannot establish a missing-header rejection under200.
- Existing actual upstream/MCP outcomes remain the source-contract.md:29–31 interfaces and existing typed dispatch outcomes. Do not infer backend auth failure from admission rejection.

## Supported and unavailable interfaces

DiagnosticBranchClosure used ten bounded source/LSP/installed-public-API probes, no tests, edits, external SDK source reads or live fault probes. Installed SDK1.8.0 StreamableHTTPOptions, ServerOptions and StreamableHTTPHandler public contracts expose no documented typed protocol-admission rejection observer. Logger/initialized/session-ID/getServer callbacks are not such observers; ServeHTTP returns no error. internal/service/server.go:11–13 passes nil options to the default handler.

The inventory establishes which facts are available and which are unsupported. It does NOT establish the actual embedded SDK decisive missing-session cause. No session_header_missing reason may be emitted from a header, HTTP status, error prose, or the isolated SDK probe. That missing authoritative cause/observation path remains a blocker for affected ticket03/exhaustive AC-D2 claims, not an outer-only ticket02 branch prerequisite; DiagnosticBranchClosure explicitly reached that scope distinction. Existing requirements and acceptance remain unchanged.

## Implementation boundary

Ticket01 research closure is the completed inventory with the above explicit limitation, not an assertion that every SDK rejection cause is known or that future runtime acceptance passed. Ticket02 may implement the approved outer OAuth foundation using the authoritative owned branches. Ticket03 must not claim unsupported SDK specificity or exhaustive AC-D2 closure. External05 collection/capture authorization remains separate. No new feature design, approval, auth-policy, wire, metadata, SDK, deployment or credential change is authorized by this evidence.

Required r6 requirements/design and r6-tasks-a1 task approvals remain the unchanged review.md:64–114 receipts. Missing project gate-extension/load evidence means hard runtime enforcement is not verified; it does not authenticate or replace those approvals. A feature-bound local marker records the approved implementation scope only and is not independently shippable.

## Authorized pinned-SDK source supplement

User explicitly approved “Allow bounded SDK inspection”: read-only pinned Go SDK v1.8.0 session rejection branch and observation hooks only, no dependency edits/live requests. LSP resolved the imported handler into /home/opencode/go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v1.8.0/mcp/streamable.go; direct bounded reads are the authorized out-of-MCP-mount fallback. This exception does not authorize other projects/dependencies.

- streamable.go:540–552 dispatches stateful GET/POST/DELETE. :579–588 checks GET Accept before missing session and returns400 for absent Mcp-Session-Id; DELETE has its own absent-header400 branch. These are SDK branches, not bridge-owned preflight decisions.
- :621–645 checks POST Content-Type and Accept before session lookup. :647–739 creates a fresh stateful SDK session for a POST without a session header, connects it, delegates transport handling and closes it if initialization never succeeded. No absent-header POST rejection exists at that branch.
- server.go:1942–2000 is the actual session initialization guard. It reads initialized state, validates per-request protocol metadata and handles allowed initialization/ping/notification methods. An ordinary call while uninitialized under the old protocol returns an untyped fmt.Errorf before dispatch. A missing-header tools/list POST can therefore reach a new, uninitialized session; the cause is not a typed session_header_missing sentinel.
- server.go:2038 invokes handleReceive only after that guard. AddReceivingMiddleware at :1851–1864 wraps the receiving method handler. shared.go:206–224 unmarshals/checks the request and invokes that handler. Therefore this public middleware cannot observe a rejection returned before handleReceive; using it cannot supply the missing authoritative guard observation.

The exact source path is now established; a supported typed bridge observation path for that pre-dispatch SDK rejection is not. No error-text matching, status inference, unverified metadata decoding, SDK patch, new protocol policy or diagnostic-only body read is authorized. This supplement does not reproduce the original OMP incident, alter protocol behavior, or establish exact wire JSON-RPC error codes. The remaining limitation is observation capability, not absent source access or a guessed SDK cause.

Planner clarification (SDKObservationContract, read-only, no contract revision): existing spec.md:160/209/229/280 permits ticket03's bridge-owned observations/control/lifecycle implementation after02, with unsupported SDK specificity generic/unavailable. It does not permit03/exhaustive AC-D2 acceptance to close while that observation remains unsupported. Ticket02 is unaffected. This preserves, rather than changes, the approved acceptance boundary.
