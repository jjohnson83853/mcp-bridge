# Local backlog migration to GitHub issues

Migrated 130 tickets and 34 feature specifications from the local Markdown tracker
(`.scratch/<feature>/`) to GitHub issues in this repository on 2026-11-26.

GitHub issues are now the authoritative tracker. The files under `.scratch/` are retained as the
migration source: they are not updated further, and where they disagree with an issue, the issue wins.

Mapping rules:

- One feature specification → one parent issue (`type:spec`), listed with its ticket checklist.
- One file in `.scratch/<feature>/issues/` → one issue (`type:ticket`).
- Feature → milestone.
- The original `Status:` line → a `status:*` label; resolved, complete, closed and wontfix tickets were closed.
- `Blocked by:` numbers → `#` references to the blocking issue within the same feature.
- Specifications larger than the issue body limit stay in the tree; their parent issue carries a section index instead.

## Feature specifications

| Feature | Issue | Tickets | Spec file |
| --- | --- | --- | --- |
| chatgpt-oauth-discovery | [#131](https://github.com/jjohnson83853/mcp-bridge/issues/131) | 4 | `.scratch/chatgpt-oauth-discovery/spec.md` |
| chatgpt-resource-indicator-alignment | [#132](https://github.com/jjohnson83853/mcp-bridge/issues/132) | 2 | `.scratch/chatgpt-resource-indicator-alignment/spec.md` |
| chatgpt-session-admission-diagnostics | [#133](https://github.com/jjohnson83853/mcp-bridge/issues/133) | 4 | `.scratch/chatgpt-session-admission-diagnostics/spec.md` |
| config-admission-resilience | [#134](https://github.com/jjohnson83853/mcp-bridge/issues/134) | 3 | `.scratch/config-admission-resilience/spec.md` |
| discovery-enrichment | [#135](https://github.com/jjohnson83853/mcp-bridge/issues/135) | 7 | `.scratch/discovery-enrichment/spec.md` |
| dynamic-pin-guidance | [#136](https://github.com/jjohnson83853/mcp-bridge/issues/136) | 2 | `.scratch/dynamic-pin-guidance/spec.md` |
| endpoint-reconfiguration-reconciliation | [#137](https://github.com/jjohnson83853/mcp-bridge/issues/137) | 3 | `.scratch/endpoint-reconfiguration-reconciliation/spec.md` |
| gateway-tool-annotations | [#138](https://github.com/jjohnson83853/mcp-bridge/issues/138) | 2 | `.scratch/gateway-tool-annotations/spec.md` |
| header-parameter-transit | [#139](https://github.com/jjohnson83853/mcp-bridge/issues/139) | 4 | `.scratch/header-parameter-transit/spec.md` |
| homelab-llm-ergonomics | [#140](https://github.com/jjohnson83853/mcp-bridge/issues/140) | 4 | `.scratch/homelab-llm-ergonomics/spec.md` |
| hybrid-tool-discovery | [#141](https://github.com/jjohnson83853/mcp-bridge/issues/141) | 5 | `.scratch/hybrid-tool-discovery/spec.md` |
| logging-observability | [#142](https://github.com/jjohnson83853/mcp-bridge/issues/142) | 6 | `.scratch/logging-observability/spec.md` |
| mcp-inbound-observability | [#143](https://github.com/jjohnson83853/mcp-bridge/issues/143) | 3 | `.scratch/mcp-inbound-observability/spec.md` |
| oauth-mcp-authorization | [#144](https://github.com/jjohnson83853/mcp-bridge/issues/144) | 8 | `.scratch/oauth-mcp-authorization/spec.md` |
| oauth-session-diagnostics | [#145](https://github.com/jjohnson83853/mcp-bridge/issues/145) | 5 | `.scratch/oauth-session-diagnostics/spec.md` |
| observability-completeness | [#146](https://github.com/jjohnson83853/mcp-bridge/issues/146) | 9 | `.scratch/observability-completeness/spec.md` |
| openai-tool-schema-compatibility | [#147](https://github.com/jjohnson83853/mcp-bridge/issues/147) | 2 | `.scratch/openai-tool-schema-compatibility/spec.md` |
| progressive-metadata-reveal | [#162](https://github.com/jjohnson83853/mcp-bridge/issues/162) | 0 | `.scratch/progressive-metadata-reveal/spec.md` |
| resource-passthrough | [#148](https://github.com/jjohnson83853/mcp-bridge/issues/148) | 7 | `.scratch/resource-passthrough/spec.md` |
| result-spill-ignore | [#163](https://github.com/jjohnson83853/mcp-bridge/issues/163) | 0 | `.scratch/result-spill-ignore/spec.md` |
| search-filter-and-lock | [#149](https://github.com/jjohnson83853/mcp-bridge/issues/149) | 2 | `.scratch/search-filter-and-lock/spec.md` |
| session-bound-oauth-admission | [#150](https://github.com/jjohnson83853/mcp-bridge/issues/150) | 5 | `.scratch/session-bound-oauth-admission/spec.md` |
| session-expiry-recovery | [#164](https://github.com/jjohnson83853/mcp-bridge/issues/164) | 0 | `.scratch/session-expiry-recovery/spec.md` |
| snapshot-recovery | [#151](https://github.com/jjohnson83853/mcp-bridge/issues/151) | 1 | `.scratch/snapshot-recovery/spec.md` |
| startup-diagnostics | [#152](https://github.com/jjohnson83853/mcp-bridge/issues/152) | 1 | `.scratch/startup-diagnostics/spec.md` |
| stateless-and-backend-modes | [#153](https://github.com/jjohnson83853/mcp-bridge/issues/153) | 5 | `.scratch/stateless-and-backend-modes/spec.md` |
| stateless-tools-call-bugfix | [#154](https://github.com/jjohnson83853/mcp-bridge/issues/154) | 1 | `.scratch/stateless-tools-call-bugfix/spec.md` |
| stateless-tools-exposure-bugfix | [#155](https://github.com/jjohnson83853/mcp-bridge/issues/155) | 1 | `.scratch/stateless-tools-exposure-bugfix/spec.md` |
| structured-logging | [#156](https://github.com/jjohnson83853/mcp-bridge/issues/156) | 18 | `.scratch/structured-logging/spec.md` |
| token-efficient-tool-responses | [#157](https://github.com/jjohnson83853/mcp-bridge/issues/157) | 6 | `.scratch/token-efficient-tool-responses/spec.md` |
| tool-content-compatibility | [#158](https://github.com/jjohnson83853/mcp-bridge/issues/158) | 2 | `.scratch/tool-content-compatibility/spec.md` |
| tool-metadata | [#159](https://github.com/jjohnson83853/mcp-bridge/issues/159) | 6 | `.scratch/tool-metadata/spec.md` |
| upstream-breaker-recovery | [#160](https://github.com/jjohnson83853/mcp-bridge/issues/160) | 1 | `.scratch/upstream-breaker-recovery/spec.md` |
| upstream-stateless-wiring-bugfix | [#161](https://github.com/jjohnson83853/mcp-bridge/issues/161) | 1 | `.scratch/upstream-stateless-wiring-bugfix/spec.md` |

## Tickets

| Ticket file | Issue | Original status | Title |
| --- | --- | --- | --- |
| `.scratch/chatgpt-oauth-discovery/issues/01-rfc9728-multi-path-metadata-discovery.md` | [#1](https://github.com/jjohnson83853/mcp-bridge/issues/1) | ready-for-agent | [chatgpt-oauth-discovery] 01: RFC 9728 multi-path metadata discovery and path-aware resource advertisement |
| `.scratch/chatgpt-oauth-discovery/issues/02-bidirectional-audience-tolerance-and-diagnostics.md` | [#2](https://github.com/jjohnson83853/mcp-bridge/issues/2) | ready-for-agent | [chatgpt-oauth-discovery] 02: Bidirectional audience tolerance and missing-credential safe diagnostics |
| `.scratch/chatgpt-oauth-discovery/issues/03-route-normalization-and-root-probe-handling.md` | [#3](https://github.com/jjohnson83853/mcp-bridge/issues/3) | ready-for-agent | [chatgpt-oauth-discovery] 03: Route normalization and root POST discovery probe handling |
| `.scratch/chatgpt-oauth-discovery/issues/04-integrated-action-discovery-wire-verification.md` | [#4](https://github.com/jjohnson83853/mcp-bridge/issues/4) | ready-for-agent | [chatgpt-oauth-discovery] 04: Integrated action discovery wire verification and diagnostic provenance |
| `.scratch/chatgpt-resource-indicator-alignment/issues/01-context-aware-resource-metadata-advertisement.md` | [#5](https://github.com/jjohnson83853/mcp-bridge/issues/5) | resolved | [chatgpt-resource-indicator-alignment] 01 — Context-Aware Resource Metadata Advertisement |
| `.scratch/chatgpt-resource-indicator-alignment/issues/02-integrated-chatgpt-wire-verification.md` | [#6](https://github.com/jjohnson83853/mcp-bridge/issues/6) | resolved | [chatgpt-resource-indicator-alignment] 02 — Integrated ChatGPT Wire Verification |
| `.scratch/chatgpt-session-admission-diagnostics/issues/01-session-admission-result-taxonomy.md` | [#7](https://github.com/jjohnson83853/mcp-bridge/issues/7) | closed | [chatgpt-session-admission-diagnostics] 01 — Typed Session Admission Seam & Outcome Classification |
| `.scratch/chatgpt-session-admission-diagnostics/issues/02-middleware-differentiated-rejection-diagnostics.md` | [#8](https://github.com/jjohnson83853/mcp-bridge/issues/8) | closed | [chatgpt-session-admission-diagnostics] 02 — Differentiated Session Rejections, Pseudonyms & Pre-Auth Method Peeking |
| `.scratch/chatgpt-session-admission-diagnostics/issues/03-initialize-handshake-lifecycle-diagnostics.md` | [#9](https://github.com/jjohnson83853/mcp-bridge/issues/9) | closed | [chatgpt-session-admission-diagnostics] 03 — Initialize Handshake Lifecycle Evidence & Stateless Representation |
| `.scratch/chatgpt-session-admission-diagnostics/issues/04-end-to-end-discovery-wire-verification-and-coverage.md` | [#10](https://github.com/jjohnson83853/mcp-bridge/issues/10) | closed | [chatgpt-session-admission-diagnostics] 04 — Composed Wire Verification, Zero Leakage Across 401/403/404 & Triage Runbook |
| `.scratch/config-admission-resilience/issues/02-diagnose-and-fix-network-startup-loop.md` | [#11](https://github.com/jjohnson83853/mcp-bridge/issues/11) | ready-for-agent | [config-admission-resilience] 02 — Diagnose and fix the network instance startup crash loop |
| `.scratch/config-admission-resilience/issues/03-last-known-good-config-fallback.md` | [#12](https://github.com/jjohnson83853/mcp-bridge/issues/12) | ready-for-agent | [config-admission-resilience] 03 — Serve a validated last-known-good config instead of crash-looping |
| `.scratch/config-admission-resilience/issues/04-strict-mode-config-loaded-event.md` | [#13](https://github.com/jjohnson83853/mcp-bridge/issues/13) | ready-for-agent | [config-admission-resilience] 04 — Strict mode, config-loaded startup event, and documented storage base |
| `.scratch/discovery-enrichment/issues/01-indexed-text-v2-schema-surface.md` | [#14](https://github.com/jjohnson83853/mcp-bridge/issues/14) | ready-for-agent | [discovery-enrichment] 01 — Indexed text v2: schema surface in the search document |
| `.scratch/discovery-enrichment/issues/02-server-tags-and-aliases.md` | [#15](https://github.com/jjohnson83853/mcp-bridge/issues/15) | ready-for-agent | [discovery-enrichment] 02 — Server tags and aliases in configuration and metadata patches |
| `.scratch/discovery-enrichment/issues/03-search-filters-and-kind.md` | [#16](https://github.com/jjohnson83853/mcp-bridge/issues/16) | ready-for-agent | [discovery-enrichment] 03 — `search_tools` filters and result kind |
| `.scratch/discovery-enrichment/issues/04-tool-cooccurrence-boost.md` | [#17](https://github.com/jjohnson83853/mcp-bridge/issues/17) | ready-for-agent | [discovery-enrichment] 04 — Tool co-occurrence ranking boost |
| `.scratch/discovery-enrichment/issues/05-already-exposed-demotion.md` | [#18](https://github.com/jjohnson83853/mcp-bridge/issues/18) | ready-for-agent | [discovery-enrichment] 05 — Already-exposed demotion in search results |
| `.scratch/discovery-enrichment/issues/06-prompt-cross-indexing.md` | [#19](https://github.com/jjohnson83853/mcp-bridge/issues/19) | ready-for-agent | [discovery-enrichment] 06 — Cross-index upstream prompts into the same hybrid space |
| `.scratch/discovery-enrichment/issues/07-schema-normalization.md` | [#20](https://github.com/jjohnson83853/mcp-bridge/issues/20) | ready-for-agent | [discovery-enrichment] 07 — Schema normalization for model comprehension |
| `.scratch/dynamic-pin-guidance/issues/01-dynamic-pinned-inventory-and-built-in-guidance.md` | [#21](https://github.com/jjohnson83853/mcp-bridge/issues/21) | ready-for-agent | [dynamic-pin-guidance] 01 - Dynamic Pinned Inventory in search_tools and Guidance on Gateway Built-ins |
| `.scratch/dynamic-pin-guidance/issues/02-reload-consistency-regression-and-coverage.md` | [#22](https://github.com/jjohnson83853/mcp-bridge/issues/22) | ready-for-agent | [dynamic-pin-guidance] 02 - Configuration Reload Consistency, Regression, and Coverage Gate |
| `.scratch/endpoint-reconfiguration-reconciliation/issues/01-storage-reconciliation-endpoint-rebind.md` | [#23](https://github.com/jjohnson83853/mcp-bridge/issues/23) | resolved | [endpoint-reconfiguration-reconciliation] 01: Storage reconciliation endpoint rebind and cache eviction |
| `.scratch/endpoint-reconfiguration-reconciliation/issues/02-service-startup-and-reload-admission.md` | [#24](https://github.com/jjohnson83853/mcp-bridge/issues/24) | resolved | [endpoint-reconfiguration-reconciliation] 02: Service startup admission and dynamic reload for reconfigured endpoints |
| `.scratch/endpoint-reconfiguration-reconciliation/issues/03-end-to-end-process-and-state-persistence.md` | [#25](https://github.com/jjohnson83853/mcp-bridge/issues/25) | resolved | [endpoint-reconfiguration-reconciliation] 03: End-to-end CLI process and state persistence acceptance tests |
| `.scratch/gateway-tool-annotations/issues/01-explicit-annotations-built-in-discovery-tools.md` | [#26](https://github.com/jjohnson83853/mcp-bridge/issues/26) | ready-for-agent | [gateway-tool-annotations] 01 — Explicit Annotations for Built-in Discovery Tools |
| `.scratch/gateway-tool-annotations/issues/02-regression-and-invariant-verification.md` | [#27](https://github.com/jjohnson83853/mcp-bridge/issues/27) | ready-for-agent | [gateway-tool-annotations] 02 — Regression and Invariant Verification Across Upstream Metadata Layers |
| `.scratch/header-parameter-transit/issues/01-header-parameter-wire-reproduction.md` | [#28](https://github.com/jjohnson83853/mcp-bridge/issues/28) | ready-for-agent | [header-parameter-transit] 01 — Reproduce MB-001 against a header-annotated peer and capture the wire |
| `.scratch/header-parameter-transit/issues/02-header-parameters-traverse-stateless-calls.md` | [#29](https://github.com/jjohnson83853/mcp-bridge/issues/29) | ready-for-agent | [header-parameter-transit] 02 — Header parameters reach a header-bound upstream in default stateless mode |
| `.scratch/header-parameter-transit/issues/03-stateful-and-legacy-protocol-boundary.md` | [#30](https://github.com/jjohnson83853/mcp-bridge/issues/30) | ready-for-agent | [header-parameter-transit] 03 — Explicit stateful mode and legacy-protocol boundary are deterministic |
| `.scratch/header-parameter-transit/issues/04-deployed-github-verification.md` | [#31](https://github.com/jjohnson83853/mcp-bridge/issues/31) | resolved | [header-parameter-transit] 04 — Deployed github upstream verified end-to-end |
| `.scratch/homelab-llm-ergonomics/issues/01-optional-catalog-revision.md` | [#32](https://github.com/jjohnson83853/mcp-bridge/issues/32) | ready-for-agent | [homelab-llm-ergonomics] 01 — Optional catalog_revision Admission and Describe Correlation |
| `.scratch/homelab-llm-ergonomics/issues/02-semantic-wire-name-publication.md` | [#33](https://github.com/jjohnson83853/mcp-bridge/issues/33) | ready-for-agent | [homelab-llm-ergonomics] 02 — Semantic Wire Name Publication |
| `.scratch/homelab-llm-ergonomics/issues/03-prune-search-describe-call-payloads.md` | [#34](https://github.com/jjohnson83853/mcp-bridge/issues/34) | ready-for-agent | [homelab-llm-ergonomics] 03 — Prune Search, Describe, and Call Payloads |
| `.scratch/homelab-llm-ergonomics/issues/04-integrated-wire-verification-and-coverage.md` | [#35](https://github.com/jjohnson83853/mcp-bridge/issues/35) | ready-for-agent | [homelab-llm-ergonomics] 04 — Integrated Wire Verification and Coverage Gate |
| `.scratch/hybrid-tool-discovery/issues/01-preloaded-tool-configuration-and-naming.md` | [#36](https://github.com/jjohnson83853/mcp-bridge/issues/36) | complete | [hybrid-tool-discovery] 01: Preloaded tool configuration bounds and wire naming |
| `.scratch/hybrid-tool-discovery/issues/02-assembly-catalog-and-tools-list-exposure.md` | [#37](https://github.com/jjohnson83853/mcp-bridge/issues/37) | complete | [hybrid-tool-discovery] 02: Assembly catalog and tools/list preloaded exposure |
| `.scratch/hybrid-tool-discovery/issues/03-direct-preloaded-tool-execution-dispatch.md` | [#38](https://github.com/jjohnson83853/mcp-bridge/issues/38) | complete | [hybrid-tool-discovery] 03: Direct preloaded tool execution dispatch |
| `.scratch/hybrid-tool-discovery/issues/04-search-coexistence-and-metadata-alignment.md` | [#39](https://github.com/jjohnson83853/mcp-bridge/issues/39) | complete | [hybrid-tool-discovery] 04: Search coexistence and metadata alignment |
| `.scratch/hybrid-tool-discovery/issues/05-dynamic-reload-and-list-changed-notifications.md` | [#40](https://github.com/jjohnson83853/mcp-bridge/issues/40) | complete | [hybrid-tool-discovery] 05: Dynamic SIGHUP reload and tool list change notifications |
| `.scratch/logging-observability/issues/01-http-request-diagnostics.md` | [#41](https://github.com/jjohnson83853/mcp-bridge/issues/41) | complete | [logging-observability] 01: HTTP request diagnostics and precise admission failures |
| `.scratch/logging-observability/issues/02-correlated-mcp-backend-execution.md` | [#42](https://github.com/jjohnson83853/mcp-bridge/issues/42) | complete | [logging-observability] 02: Trace successful MCP operations through backend execution |
| `.scratch/logging-observability/issues/03-protocol-tool-backend-failures.md` | [#43](https://github.com/jjohnson83853/mcp-bridge/issues/43) | complete | [logging-observability] 03: Distinguish protocol, tool-admission, and backend failures |
| `.scratch/logging-observability/issues/04-upstream-client-response-shapes.md` | [#44](https://github.com/jjohnson83853/mcp-bridge/issues/44) | complete | [logging-observability] 04: Diagnose upstream and client-facing result shapes |
| `.scratch/logging-observability/issues/05-stream-timeout-cancellation-diagnostics.md` | [#45](https://github.com/jjohnson83853/mcp-bridge/issues/45) | complete | [logging-observability] 05: Diagnose streaming, timeouts, cancellations, and disconnects |
| `.scratch/logging-observability/issues/06-concurrent-reload-correlation-isolation.md` | [#46](https://github.com/jjohnson83853/mcp-bridge/issues/46) | complete | [logging-observability] 06: Keep concurrent and reloaded request chains isolated |
| `.scratch/mcp-inbound-observability/issues/01-inbound-method-operation-classification-and-metrics.md` | [#47](https://github.com/jjohnson83853/mcp-bridge/issues/47) | ready-for-agent | [mcp-inbound-observability] 01 — Inbound Method Operation Classification and Unified Prometheus Metrics |
| `.scratch/mcp-inbound-observability/issues/02-client-family-classification-and-exact-bytes.md` | [#48](https://github.com/jjohnson83853/mcp-bridge/issues/48) | ready-for-agent | [mcp-inbound-observability] 02 — Client Family Classification, Operation Slot and Exact Response Bytes |
| `.scratch/mcp-inbound-observability/issues/03-exposed-tool-count-and-wire-verification.md` | [#49](https://github.com/jjohnson83853/mcp-bridge/issues/49) | ready-for-agent | [mcp-inbound-observability] 03 — Exposed Tool Count and Wire Observability Verification |
| `.scratch/oauth-mcp-authorization/issues/01-jwx-dependency-and-auth-package.md` | [#50](https://github.com/jjohnson83853/mcp-bridge/issues/50) | resolved | [oauth-mcp-authorization] 01: Add JWX dependency and internal auth package |
| `.scratch/oauth-mcp-authorization/issues/02-protected-resource-metadata-handler.md` | [#51](https://github.com/jjohnson83853/mcp-bridge/issues/51) | resolved | [oauth-mcp-authorization] 02: Protected resource metadata handler (RFC 9728) |
| `.scratch/oauth-mcp-authorization/issues/03-http-auth-middleware-and-claim-validation.md` | [#52](https://github.com/jjohnson83853/mcp-bridge/issues/52) | resolved | [oauth-mcp-authorization] 03: HTTP authentication middleware and claim validation |
| `.scratch/oauth-mcp-authorization/issues/04-inbound-transport-context-integration.md` | [#53](https://github.com/jjohnson83853/mcp-bridge/issues/53) | resolved | [oauth-mcp-authorization] 04: Inbound transport context integration and session binding |
| `.scratch/oauth-mcp-authorization/issues/05-server-composition-wiring-and-wire-tests.md` | [#54](https://github.com/jjohnson83853/mcp-bridge/issues/54) | resolved | [oauth-mcp-authorization] 05: Server composition wiring and HTTP wire test suite |
| `.scratch/oauth-mcp-authorization/issues/06-expired-session-recovery.md` | [#55](https://github.com/jjohnson83853/mcp-bridge/issues/55) | needs-triage | [oauth-mcp-authorization] 06: Recover expired or unknown MCP sessions without an authentication loop |
| `.scratch/oauth-mcp-authorization/issues/07-client-neutral-token-renewal.md` | [#56](https://github.com/jjohnson83853/mcp-bridge/issues/56) | needs-triage | [oauth-mcp-authorization] 07: Support secure token renewal without trapping MCP clients in session-authentication failures |
| `.scratch/oauth-mcp-authorization/issues/08-path-bearing-resource-metadata.md` | [#57](https://github.com/jjohnson83853/mcp-bridge/issues/57) | needs-triage | [oauth-mcp-authorization] 08: Keep authentication metadata challenges reachable for path-bearing resource URLs |
| `.scratch/oauth-session-diagnostics/issues/01-identify-diagnostics-delta.md` | [#58](https://github.com/jjohnson83853/mcp-bridge/issues/58) | resolved | [oauth-session-diagnostics] 01: Identify the exact diagnostics delta |
| `.scratch/oauth-session-diagnostics/issues/02-outer-oauth-rejection-diagnostics.md` | [#59](https://github.com/jjohnson83853/mcp-bridge/issues/59) | complete | [oauth-session-diagnostics] 02: Make outer OAuth rejections diagnosable |
| `.scratch/oauth-session-diagnostics/issues/03-session-outcome-diagnostics.md` | [#60](https://github.com/jjohnson83853/mcp-bridge/issues/60) | complete | [oauth-session-diagnostics] 03: Distinguish missing sessions from token-binding mismatches |
| `.scratch/oauth-session-diagnostics/issues/04-runtime-diagnostic-provenance.md` | [#61](https://github.com/jjohnson83853/mcp-bridge/issues/61) | complete | [oauth-session-diagnostics] 04: Add runtime provenance to the existing diagnostic chain |
| `.scratch/oauth-session-diagnostics/issues/05-deployed-loki-verification.md` | [#62](https://github.com/jjohnson83853/mcp-bridge/issues/62) | needs-triage | [oauth-session-diagnostics] 05: Verify diagnostics survive into deployed Loki |
| `.scratch/observability-completeness/issues/01-request-duration-histograms.md` | [#63](https://github.com/jjohnson83853/mcp-bridge/issues/63) | ready-for-agent | [observability-completeness] 01 — Per-server request duration histograms |
| `.scratch/observability-completeness/issues/02-search-duration-histogram.md` | [#64](https://github.com/jjohnson83853/mcp-bridge/issues/64) | ready-for-agent | [observability-completeness] 02 — Search duration histogram with degradation mode |
| `.scratch/observability-completeness/issues/03-runtime-and-process-collectors.md` | [#65](https://github.com/jjohnson83853/mcp-bridge/issues/65) | ready-for-agent | [observability-completeness] 03 — Go runtime and process collectors |
| `.scratch/observability-completeness/issues/04-failure-taxonomy-counter.md` | [#66](https://github.com/jjohnson83853/mcp-bridge/issues/66) | ready-for-agent | [observability-completeness] 04 — Failure taxonomy counter (`bridge_errors_total`) |
| `.scratch/observability-completeness/issues/05-auth-and-session-counters.md` | [#67](https://github.com/jjohnson83853/mcp-bridge/issues/67) | ready-for-agent | [observability-completeness] 05 — Auth rejection and session lifecycle counters |
| `.scratch/observability-completeness/issues/06-inflight-session-and-reconnect-gauges.md` | [#68](https://github.com/jjohnson83853/mcp-bridge/issues/68) | ready-for-agent | [observability-completeness] 06 — Inflight, active-session, and reconnect gauges |
| `.scratch/observability-completeness/issues/07-catalog-resources-gauge.md` | [#69](https://github.com/jjohnson83853/mcp-bridge/issues/69) | ready-for-agent | [observability-completeness] 07 — Catalog resource gauge |
| `.scratch/observability-completeness/issues/08-embedding-usage-and-coverage.md` | [#70](https://github.com/jjohnson83853/mcp-bridge/issues/70) | ready-for-agent | [observability-completeness] 08 — Embedding usage and coverage metrics |
| `.scratch/observability-completeness/issues/09-snapshot-write-metrics.md` | [#71](https://github.com/jjohnson83853/mcp-bridge/issues/71) | ready-for-agent | [observability-completeness] 09 — Durable snapshot write counters |
| `.scratch/openai-tool-schema-compatibility/issues/01-tool-input-schema-alignment-and-bounds-normalization.md` | [#72](https://github.com/jjohnson83853/mcp-bridge/issues/72) | closed | [openai-tool-schema-compatibility] 01 — Tool Input Schema Alignment and Bounds Normalization |
| `.scratch/openai-tool-schema-compatibility/issues/02-end-to-end-action-discovery-wire-verification.md` | [#73](https://github.com/jjohnson83853/mcp-bridge/issues/73) | closed | [openai-tool-schema-compatibility] 02 — End-to-End Action Discovery Wire Verification |
| `.scratch/resource-passthrough/issues/01-namespaced-resource-discovery-and-list.md` | [#74](https://github.com/jjohnson83853/mcp-bridge/issues/74) | ready-for-agent | [resource-passthrough] 01 — Namespaced resource discovery and `resources/list` |
| `.scratch/resource-passthrough/issues/02-resource-template-listing.md` | [#75](https://github.com/jjohnson83853/mcp-bridge/issues/75) | ready-for-agent | [resource-passthrough] 02 — Resource template listing (`resources/templates/list`) |
| `.scratch/resource-passthrough/issues/03-resource-read-forwarding.md` | [#76](https://github.com/jjohnson83853/mcp-bridge/issues/76) | ready-for-agent | [resource-passthrough] 03 — `resources/read` forwarding and content fidelity |
| `.scratch/resource-passthrough/issues/04-resource-completion.md` | [#77](https://github.com/jjohnson83853/mcp-bridge/issues/77) | ready-for-agent | [resource-passthrough] 04 — Argument completion for resource references (`completion/complete`) |
| `.scratch/resource-passthrough/issues/05-resource-subscriptions-and-notifications.md` | [#78](https://github.com/jjohnson83853/mcp-bridge/issues/78) | ready-for-agent | [resource-passthrough] 05 — Resource subscriptions and change notifications |
| `.scratch/resource-passthrough/issues/06-resource-link-rewriting.md` | [#79](https://github.com/jjohnson83853/mcp-bridge/issues/79) | ready-for-agent | [resource-passthrough] 06 — `resource_link` and embedded resource rewriting in tool results |
| `.scratch/resource-passthrough/issues/07-model-driven-resource-access.md` | [#80](https://github.com/jjohnson83853/mcp-bridge/issues/80) | ready-for-agent | [resource-passthrough] 07 — Model-driven resource access: `search_resources` and `read_resource` |
| `.scratch/search-filter-and-lock/issues/01-search-server-filter-index-mapping.md` | [#81](https://github.com/jjohnson83853/mcp-bridge/issues/81) | resolved | [search-filter-and-lock] 01: Search server filter index mapping |
| `.scratch/search-filter-and-lock/issues/02-storage-flock-crash-recovery.md` | [#82](https://github.com/jjohnson83853/mcp-bridge/issues/82) | resolved | [search-filter-and-lock] 02: Storage flock crash recovery |
| `.scratch/session-bound-oauth-admission/issues/01-canonical-resource-metadata-alignment.md` | [#83](https://github.com/jjohnson83853/mcp-bridge/issues/83) | ready-for-agent | [session-bound-oauth-admission] 01 — Canonical Endpoint Resource Metadata Alignment |
| `.scratch/session-bound-oauth-admission/issues/02-session-admission-seam-and-discovery-burst.md` | [#84](https://github.com/jjohnson83853/mcp-bridge/issues/84) | ready-for-agent | [session-bound-oauth-admission] 02 — Session Admission Seam & Tokenless Post-Init Discovery Burst |
| `.scratch/session-bound-oauth-admission/issues/03-explicit-credential-precedence-and-token-renewal.md` | [#85](https://github.com/jjohnson83853/mcp-bridge/issues/85) | ready-for-agent | [session-bound-oauth-admission] 03 — Explicit Credential Precedence & Dynamic Token Renewal |
| `.scratch/session-bound-oauth-admission/issues/04-authorization-ceiling-and-scope-enforcement.md` | [#86](https://github.com/jjohnson83853/mcp-bridge/issues/86) | ready-for-agent | [session-bound-oauth-admission] 04 — Authorization Ceiling & Scope Policy Enforcement |
| `.scratch/session-bound-oauth-admission/issues/05-integrated-action-discovery-wire-verification.md` | [#87](https://github.com/jjohnson83853/mcp-bridge/issues/87) | ready-for-agent | [session-bound-oauth-admission] 05 — Integrated Action Discovery Wire Verification |
| `.scratch/snapshot-recovery/issues/01-untrusted-snapshot-self-recovery.md` | [#88](https://github.com/jjohnson83853/mcp-bridge/issues/88) | ready-for-agent | [snapshot-recovery] 01 — Unreadable snapshot must not wedge startup: quarantine and continue |
| `.scratch/startup-diagnostics/issues/01-surface-unclassified-startup-failure-cause.md` | [#89](https://github.com/jjohnson83853/mcp-bridge/issues/89) | ready-for-agent | [startup-diagnostics] 01 — Surface unclassified startup failures with a typed, sanitized cause |
| `.scratch/stateless-and-backend-modes/issues/01-upstream-server-config-stateless-stubs.md` | [#90](https://github.com/jjohnson83853/mcp-bridge/issues/90) | complete | [stateless-and-backend-modes] 01 — Upstream Server Configuration Schema & Fingerprinting (Default Stateless Stubs) |
| `.scratch/stateless-and-backend-modes/issues/02-upstream-runtime-stateless-execution.md` | [#91](https://github.com/jjohnson83853/mcp-bridge/issues/91) | complete | [stateless-and-backend-modes] 02 — Upstream Dedicated Runtime Stateless Execution & Pooled Keep-Alive |
| `.scratch/stateless-and-backend-modes/issues/03-upstream-adaptive-ephemeral-handshake.md` | [#92](https://github.com/jjohnson83853/mcp-bridge/issues/92) | complete | [stateless-and-backend-modes] 03 — Upstream Adaptive Ephemeral Handshake for State-Enforcing Stubs |
| `.scratch/stateless-and-backend-modes/issues/04-downstream-stateless-streamable-transport.md` | [#93](https://github.com/jjohnson83853/mcp-bridge/issues/93) | complete | [stateless-and-backend-modes] 04 — Downstream Stateless Streamable Transport & Multi-Version Protocol Negotiation |
| `.scratch/stateless-and-backend-modes/issues/05-e2e-chatgpt-wire-verification-and-coverage.md` | [#94](https://github.com/jjohnson83853/mcp-bridge/issues/94) | complete | [stateless-and-backend-modes] 05 — End-to-End ChatGPT Wire Verification Across Mixed Stubs & Coverage Gate |
| `.scratch/stateless-tools-call-bugfix/issues/01-stateless-tools-call-admission.md` | [#95](https://github.com/jjohnson83853/mcp-bridge/issues/95) | none | [stateless-tools-call-bugfix] 01 - Stateless Tool Call and Preloaded Tool Execution Admission |
| `.scratch/stateless-tools-exposure-bugfix/issues/01-stateless-tools-exposure-admission.md` | [#96](https://github.com/jjohnson83853/mcp-bridge/issues/96) | none | [stateless-tools-exposure-bugfix] 01 — Stateless Tool Exposure Admission and Wire Verification |
| `.scratch/structured-logging/issues/01-json-error-envelope-and-pin-ceiling-attribution.md` | [#97](https://github.com/jjohnson83853/mcp-bridge/issues/97) | ready-for-agent | [structured-logging] 01 - JSON Error Envelope and Pin-Ceiling Attribution |
| `.scratch/structured-logging/issues/02-configuration-node-attribution.md` | [#98](https://github.com/jjohnson83853/mcp-bridge/issues/98) | ready-for-agent | [structured-logging] 02 - Configuration-Node Attribution for Every Rejection Class |
| `.scratch/structured-logging/issues/03-typed-startup-and-cli-failures.md` | [#99](https://github.com/jjohnson83853/mcp-bridge/issues/99) | ready-for-agent | [structured-logging] 03 - Typed Startup and CLI Failures |
| `.scratch/structured-logging/issues/04-unified-log-envelope-migration.md` | [#100](https://github.com/jjohnson83853/mcp-bridge/issues/100) | ready-for-agent | [structured-logging] 04 - Unified Log Envelope Migration |
| `.scratch/structured-logging/issues/05-reload-rejection-consistency.md` | [#101](https://github.com/jjohnson83853/mcp-bridge/issues/101) | ready-for-agent | [structured-logging] 05 - Reload Rejection Consistency |
| `.scratch/structured-logging/issues/06-validation-success-record.md` | [#102](https://github.com/jjohnson83853/mcp-bridge/issues/102) | ready-for-agent | [structured-logging] 06 - Validation Success Record |
| `.scratch/structured-logging/issues/07-contract-docs-and-verification.md` | [#103](https://github.com/jjohnson83853/mcp-bridge/issues/103) | ready-for-agent | [structured-logging] 07 - Contract Docs and Verification Closeout |
| `.scratch/structured-logging/issues/08-refresh-pass-lifecycle-records.md` | [#104](https://github.com/jjohnson83853/mcp-bridge/issues/104) | ready-for-agent | [structured-logging] 08 - Refresh Pass Lifecycle Records |
| `.scratch/structured-logging/issues/09-pin-blocker-detail.md` | [#105](https://github.com/jjohnson83853/mcp-bridge/issues/105) | ready-for-agent | [structured-logging] 09 - Pin Blocker Detail |
| `.scratch/structured-logging/issues/10-discovery-attribution-and-readiness-reason.md` | [#106](https://github.com/jjohnson83853/mcp-bridge/issues/106) | ready-for-agent | [structured-logging] 10 - Discovery Attribution and Readiness Reason |
| `.scratch/structured-logging/issues/11-path-parity-and-logger-fallback.md` | [#107](https://github.com/jjohnson83853/mcp-bridge/issues/107) | ready-for-agent | [structured-logging] 11 - Path Parity and Logger Fallback |
| `.scratch/structured-logging/issues/12-contract-docs-and-verification-closeout.md` | [#108](https://github.com/jjohnson83853/mcp-bridge/issues/108) | ready-for-agent | [structured-logging] 12 - Contract Docs and Verification Closeout |
| `.scratch/structured-logging/issues/13-startup-pin-evaluation-leg.md` | [#109](https://github.com/jjohnson83853/mcp-bridge/issues/109) | needs-info | [structured-logging] 13 - Startup Pin-Evaluation Leg |
| `.scratch/structured-logging/issues/14-refresh-counts-and-age-assertions.md` | [#110](https://github.com/jjohnson83853/mcp-bridge/issues/110) | needs-info | [structured-logging] 14 - Refresh Counts and Age Assertions |
| `.scratch/structured-logging/issues/15-reload-acceptance-record.md` | [#111](https://github.com/jjohnson83853/mcp-bridge/issues/111) | needs-info | [structured-logging] 15 - Reload Acceptance Record |
| `.scratch/structured-logging/issues/16-pin-position-retention-and-origin-accessor.md` | [#112](https://github.com/jjohnson83853/mcp-bridge/issues/112) | needs-info | [structured-logging] 16 - Pin Position Retention and Origin Accessor |
| `.scratch/structured-logging/issues/17-r8-artifact-consistency-drift.md` | [#113](https://github.com/jjohnson83853/mcp-bridge/issues/113) | needs-info | [structured-logging] 17 - r8 Artifact Consistency Drift |
| `.scratch/structured-logging/issues/18-owner-ruling-misleading-counter-gap.md` | [#114](https://github.com/jjohnson83853/mcp-bridge/issues/114) | needs-info | [structured-logging] 18 - Owner Ruling: Misleading-Counter Gap |
| `.scratch/token-efficient-tool-responses/issues/01-bounded-results-with-paging.md` | [#115](https://github.com/jjohnson83853/mcp-bridge/issues/115) | ready-for-agent | [token-efficient-tool-responses] 01 — Bounded tool results with recoverable paging |
| `.scratch/token-efficient-tool-responses/issues/02-compact-search-index.md` | [#116](https://github.com/jjohnson83853/mcp-bridge/issues/116) | ready-for-agent | [token-efficient-tool-responses] 02 — Compact `search_tools` index |
| `.scratch/token-efficient-tool-responses/issues/03-stop-search-schema-fanout.md` | [#117](https://github.com/jjohnson83853/mcp-bridge/issues/117) | ready-for-agent | [token-efficient-tool-responses] 03 — Search must not fan out matched schemas into later prompts |
| `.scratch/token-efficient-tool-responses/issues/04-recursive-discovery-routing.md` | [#118](https://github.com/jjohnson83853/mcp-bridge/issues/118) | ready-for-agent | [token-efficient-tool-responses] 04 — Eliminate bridge-within-bridge discovery recursion |
| `.scratch/token-efficient-tool-responses/issues/05-default-list-projections.md` | [#119](https://github.com/jjohnson83853/mcp-bridge/issues/119) | ready-for-agent | [token-efficient-tool-responses] 05 — Default field projections for known list operations |
| `.scratch/token-efficient-tool-responses/issues/06-safe-duplicate-delivery.md` | [#120](https://github.com/jjohnson83853/mcp-bridge/issues/120) | ready-for-agent | [token-efficient-tool-responses] 06 — Duplicate result reuse without suppressing execution |
| `.scratch/tool-content-compatibility/issues/01-service-tool-content-population.md` | [#121](https://github.com/jjohnson83853/mcp-bridge/issues/121) | resolved | [tool-content-compatibility] 01: Service-layer tool content fallback population |
| `.scratch/tool-content-compatibility/issues/02-http-wire-contract-verification.md` | [#122](https://github.com/jjohnson83853/mcp-bridge/issues/122) | resolved | [tool-content-compatibility] 02: HTTP wire contract verification and full-suite coverage |
| `.scratch/tool-metadata/issues/01-resolve-compatibility-prerequisites.md` | [#123](https://github.com/jjohnson83853/mcp-bridge/issues/123) | ready-for-agent | [tool-metadata] 01: Resolve storage, session, and reporting prerequisites |
| `.scratch/tool-metadata/issues/02-classify-bridge-owned-tools.md` | [#124](https://github.com/jjohnson83853/mcp-bridge/issues/124) | ready-for-agent | [tool-metadata] 02: Classify bridge-owned tools accurately |
| `.scratch/tool-metadata/issues/03-preserve-upstream-metadata.md` | [#125](https://github.com/jjohnson83853/mcp-bridge/issues/125) | ready-for-agent | [tool-metadata] 03: Preserve upstream metadata end to end |
| `.scratch/tool-metadata/issues/04-apply-server-metadata-defaults.md` | [#126](https://github.com/jjohnson83853/mcp-bridge/issues/126) | ready-for-agent | [tool-metadata] 04: Apply server-level defaults to pinned tools |
| `.scratch/tool-metadata/issues/05-support-exact-tool-exceptions.md` | [#127](https://github.com/jjohnson83853/mcp-bridge/issues/127) | ready-for-agent | [tool-metadata] 05: Support exact-tool exceptions over server defaults |
| `.scratch/tool-metadata/issues/06-integrated-acceptance-and-release-evidence.md` | [#128](https://github.com/jjohnson83853/mcp-bridge/issues/128) | ready-for-agent | [tool-metadata] 06: Complete integrated acceptance and release evidence |
| `.scratch/upstream-breaker-recovery/issues/01-breaker-discovery-probe-recovery.md` | [#129](https://github.com/jjohnson83853/mcp-bridge/issues/129) | ready-for-agent | [upstream-breaker-recovery] 01 — Breaker Regains Reachable Close Path via Discovery Probe |
| `.scratch/upstream-stateless-wiring-bugfix/issues/01-upstream-stateless-wiring.md` | [#130](https://github.com/jjohnson83853/mcp-bridge/issues/130) | closed | [upstream-stateless-wiring-bugfix] 01 - Upstream Server Stateless Wiring and Restart Resilience |

## Late additions (2026-11-26)

Three feature directories under `.scratch/` carried a `spec.md` but had no issue and no row in the
tables above. They were published after the main migration:

- `progressive-metadata-reveal` → [#162](https://github.com/jjohnson83853/mcp-bridge/issues/162)
  (closed as completed; implemented by `1c6ef7f`).
- `result-spill-ignore` → [#163](https://github.com/jjohnson83853/mcp-bridge/issues/163)
  (closed as completed; `.gitignore:47`).
- `session-expiry-recovery` → [#164](https://github.com/jjohnson83853/mcp-bridge/issues/164)
  (published open with `status:needs-triage`; no implementing commit found, disposition unverified).
- `token-efficient-tool-responses` → [#157](https://github.com/jjohnson83853/mcp-bridge/issues/157)
  refreshed to specification r40: the body carries the header, status block, tickets and section
  index; the byte-exact specification is mirrored in three dated comments
  (`sha256 37dd21cee4cebdd8c5e4771741e05e594feb01292e43269ace89ee11d35ad686`) and the decision log in
  a fourth. The digest-pinned file at `.scratch/token-efficient-tool-responses/spec.md` remains the
  reviewed artifact bound by `.omp/plans/review-sequence.json`.
