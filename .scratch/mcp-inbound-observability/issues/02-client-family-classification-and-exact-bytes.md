# 02 — Client Family Classification and Exact Response Bytes

**What to build:**
Classify client User-Agent headers into a closed, low-cardinality `client_family` enum (`openai`, `claude`, `cursor`, `omp`, `browser`, `other`, `absent`) using specific-first priority substring matching in the HTTP diagnostics owner. Emit `client_family` and exact integer `http_bytes` in the terminal `http_complete` diagnostic JSON line alongside the existing `http_bytes_bucket`.

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
- [ ] Diagnostic record includes `client_family: "<value>"` in JSON completion lines.
- [ ] Diagnostic record includes `http_bytes: <int64>` (exact integer bytes written) in JSON completion lines alongside `http_bytes_bucket`.
- [ ] Unit tests in `internal/observe/httpdiagnostics_test.go` assert correct classification across varied User-Agent headers and exact byte formatting.
