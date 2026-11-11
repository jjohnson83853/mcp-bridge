# 02 - Configuration-Node Attribution for Every Rejection Class

Status: ready-for-agent
Blocked-by: 01

## What to build
Every configuration rejection identifies the exact document node that caused it, not just the section name. Malformed servers, invalid or credential-bearing URLs, secret references, out-of-range bounds, duplicate pins, duplicate or invalid keys, unsupported metadata patches, and document-level failures all report a property path, one-based line and column, and the safe identifiers already validated for that node.

Conflicting entries report both sides: the rejected location and the earlier location it conflicts with, under `config.related`.

Values stay out of the record. Secret-like fields, URLs, query strings, and raw YAML fragments are never serialized; only validated identifiers, finite classes, and static reasons appear.

## Acceptance Criteria
- [ ] AC-1: Each rejection class emits `config.property_path`, `config.line`, and `config.column` pointing at the offending node.
- [ ] AC-2: Duplicate pin and duplicate key rejections include `config.related.property_path`, `line`, and `column` for the earlier entry.
- [ ] AC-3: Server-scoped rejections include the validated server id; pin-scoped rejections include the validated server and tool.
- [ ] AC-4: No record contains a secret value, a URL, a query string, a YAML fragment, or arbitrary decoder prose.
- [ ] AC-5: Document-level failures use `property_path: "<document>"` with the parser position when one exists.
- [ ] AC-6: Tests cover every rejection class and assert both the position fields and the absence of forbidden values.
