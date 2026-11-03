# 01 — Inbound Method Operation Classification and Unified Prometheus Metrics

**What to build:**
Classify inbound JSON-RPC messages and SSE stream requests into a closed finite enum across Prometheus metrics and structured legacy HTTP evidence: `initialize`, `initialized`, `tools_list`, `tools_call`, `resources_list`, `resources_read`, `ping`, `prompts_list`, `prompts_get`, `sse_stream`, and `request`. This enables operators to monitor MCP method volume and inspect protocol failures in Loki and Prometheus without packet inspection.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] Inbound handler method classifier (`inboundOperation`) extracts the JSON-RPC method from captured request bodies and maps standard MCP methods:
  - `"initialize"` -> `"initialize"`
  - `"notifications/initialized"` -> `"initialized"`
  - `"tools/list"` -> `"tools_list"`
  - `"tools/call"` -> `"tools_call"`
  - `"resources/list"` -> `"resources_list"`
  - `"resources/read"` -> `"resources_read"`
  - `"ping"` -> `"ping"`
  - `"prompts/list"` -> `"prompts_list"`
  - `"prompts/get"` -> `"prompts_get"`
  - Unparseable or non-standard bodies -> `"request"`
- [ ] Inbound handler classifies incoming `GET` requests with SSE headers as `operation: "sse_stream"`.
- [ ] Prometheus metric `RecordRequest` records the mapped `operation` value.
- [ ] Legacy diagnostic evidence (`observe.HTTPLegacyEvidence{Operation: ...}`) records the mapped `operation` value.
- [ ] Unit tests in `internal/service/inbound_test.go` assert operation classification for each supported method and fallback cases.
