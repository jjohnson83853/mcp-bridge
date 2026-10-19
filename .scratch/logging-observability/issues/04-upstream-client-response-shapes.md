# 04: Diagnose upstream and client-facing result shapes

**What to build:** Operators can compare the shape of an upstream tool result with the actual final client-facing result without logging content. Text-only, structured-only, mixed, empty and error results remain distinguishable even when the bridge adds its structured wrapper.

**Blocked by:** 02 — Trace successful MCP operations through backend execution.

Status: complete

Type: task

Source: Logging observability specification, revision LOG-OBS-r4 (supersedes the original r1 source reference; approved ticket scope and dependencies unchanged). Coverage: AC11, AC12 and shape-related AC22; relevant AC5, AC8, AC14–AC16, AC18–AC21 and AC24. Ticket publication approval does not authorize implementation; applicable project gates still apply.

- [ ] Each observed tool result has separately attributable upstream and final shape summaries joined to the correct operation/backend chain. Summarize the actual returned representations, including error results; never infer upstream structured content from the final bridge wrapper. No observed result remains distinguishable from an observed empty result.
- [ ] Summaries follow the spec's closed shape contract: content count saturated at 1024 with cap flag; at most 64 inspected blocks; explicit completeness; deduplicated finite block-type set; text and structured-content presence; and bounded encoded-byte buckets. Unknown block-type strings never enter logs. An empty text block still establishes text presence; uninspected blocks cannot justify text absence.
- [ ] Size buckets use the defined byte boundaries 0, 1024, 16384, 262144 and 1048576, plus greater-than-1-MiB and unavailable states. Use existing attributable upstream raw lengths; final encoded size remains unavailable unless already exposed without extra encoding/copying. Do not scan text, parse structured bodies solely for logging, marshal again, buffer streams, or attribute whole-stream bytes to one operation.
- [ ] Content, document text, structured values, resource identifiers, annotations and arbitrary result metadata are never logged. Only allowlisted finite/type-checked summary fields survive, and their keys do not collide with the existing redactor. Preserve the current diagnostic exception; it must not become a route for arbitrary new shape fields.
- [ ] The real HTTP/MCP harness proves text-only, structured-only, mixed, empty and error-result cases. In particular, an upstream without structured content reports upstream absent while the final successful wrapper reports final present. Original content, structured wrapper and isError remain unchanged; shape is diagnostic evidence, never proof of successful client rendering.
- [ ] Boundary cases cover more than 64 blocks, more than 1024 blocks, empty text, unknown types, large text/structured results, nil/unavailable results and every byte threshold. Use the high-level harness where transport-representable and focused safety tests only for genuinely unrepresentable type/limit edges. Inject unique secrets into text, structured data, resource metadata and nested wrapper values and prove complete absence from emitted logs.
- [ ] Update operator documentation/changelog with upstream-versus-final interpretation, caps, completeness and unavailable measurements. Record deterministic RED/GREEN evidence, smoke actual local shape variants, retain unchanged metrics/auth/session/network/public-contract regressions, run focused/full-suite checks with at least 90% full-suite source-line coverage, and complete applicable code/Go/security review. No LibreChat compatibility or tool-response change is included.
