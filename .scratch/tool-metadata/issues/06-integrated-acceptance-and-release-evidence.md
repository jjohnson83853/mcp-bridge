# 06: Complete integrated acceptance and release evidence

**What to build:** An evidence-backed acceptance package proving the complete metadata feature preserves safety boundaries, compatibility, and operator-visible behavior. This is an integrated verification gate, not a substitute for the preceding slices' own tests or a new product slice.

**Blocked by:** 02: Classify bridge-owned tools accurately; 05: Support exact-tool exceptions over server defaults. Ticket 05 transitively covers 01, 03, and 04.

**Status:** ready-for-agent

Type: task
Source: Tool Metadata specification, METADATA-r4. Coverage: AC1-AC13, especially AC12.

Execution prerequisite: all applicable requirements/design/tasks approvals, evidence prerequisites, and review gates must be satisfied. This ticket grants no deployment, image-build, registry-write, or client-policy authority.

- [ ] Consolidate evidence for every acceptance criterion AC1-AC13, retaining each implementation slice's intended behavioral RED and unchanged GREEN results. No incomplete feature may be hidden behind this final gate.
- [ ] Exercise the complete feature through the confirmed existing real-bridge HTTP MCP seam with controlled upstreams: built-in classification, upstream metadata preservation, source/effective provenance, defaults/exceptions, explicit false, isolation, removal, reload, rejected updates, restart, and legacy compatibility.
- [ ] Record commands and exit results for the repository-prescribed full Go suite, race checks, static vet checks, atomic all-package coverage-profile generation, and custom source-line coverage gate. The custom gate must prove at least 90% SOURCE-LINE coverage; aggregate statement percentage, excluded affected packages, weakened assertions, or narrowed acceptance criteria are not substitutes.
- [ ] Complete required general, Go, and security reviews; resolve blocking findings and record verification of any substantive correction. Verify no schema/auth/routing/result regression, unsafe trust bypass, new telemetry, or transport/session-policy change.
- [ ] Confirm evidence-backed durable reader/writer compatibility, supported recovery/rollback behavior, source-versus-effective persistence, and existing actionable redacted failure reporting. Do not claim additive-format or rollback safety without evidence.
- [ ] Verify operator documentation/changelog covers supported metadata, SDK normalization, concrete override ownership and precedence, mixed-service risks, lifecycle, source provenance, and client-policy limits. Do not modify the parent specification as part of ticket completion.
- [ ] If safe access to the actual Gemini client exists, record its product/version/policy and a non-mutating confirmation-behavior observation. Otherwise record NOT RUN with the exact access limitation; this does not waive mandatory bridge wire acceptance or imply prompts were reduced.
- [ ] Produce a concise final pass/fail and handoff with evidence references, unresolved risks, and actual verification state. This planning ticket alone is not shippable; do not independently commit/push planning artifacts, build images, deploy, or write to the Docker registry.
