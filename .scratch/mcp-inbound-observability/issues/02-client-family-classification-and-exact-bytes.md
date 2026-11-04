# 02 — Client Family Classification, Operation Slot and Exact Response Bytes

**What to build:**
Classify client User-Agent headers into a closed, low-cardinality `client_family` enum (`openai`, `claude`, `cursor`, `omp`, `browser`, `other`, `absent`) using specific-first priority substring matching in the HTTP diagnostics owner. Emit `operation`, `client_family`, and exact integer `http_bytes` in the terminal `http_complete` diagnostic JSON line alongside the existing `http_bytes_bucket`.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] HTTP diagnostics package defines closed `client_family` values: `openai`, `claude`, `cursor`, `omp`, `browser`, `other`, `absent`.
- [ ] User-Agent classifier evaluates incoming `User-Agent` headers in strict priority order:
  1. Contains `"openai"` (case-insensitive) -> `"openai"`
  2. Contains `"claude"` or `"anthropic"` (case-insensitive) -> `"claude"`
  3. Contains `"cursor"` (case-insensitive) -> `"cursor"`
  4. Contains `"omp/"` -> `"omp"`
  5. Contains `"mozilla"`, `"chrome"`, or `"safari"` -> `"browser"`
  6. Non-empty string -> `"other"`
  7. Empty / absent -> `"absent"`
- [ ] In `internal/observe/httpdiagnostics.go`, expand `operationVocabulary` to include `"initialized"`, `"resources_list"`, `"resources_read"`, `"ping"`, `"sse_stream"`.
- [ ] In `internal/observe/httpdiagnostic_types.go`, add `Operation`, `ClientFamily`, `HTTPBytes`, and `ExposedTools` to `HTTPDiagnostic`.
- [ ] In `internal/observe/httpwriter.go`, add `clientFamily` and `exposedTools` fields to `httpOwner` and initialize in `init()`.
- [ ] In `internal/observe/httpdiagnostic_encode.go`, add slots for `operation`, `client_family`, `http_bytes`, and `exposed_tools` to `slotKeys` and fill them in `httpEncoder.build()`.
- [ ] Diagnostic record includes `operation: "<op>"`, `client_family: "<value>"`, and `http_bytes: <int64>` in JSON completion lines.
- [ ] Unit tests in `internal/observe/httpdiagnostics_test.go` assert correct classification across varied User-Agent headers, exact byte formatting, and `TestChargedBytesEqualEncodedRecord`.
