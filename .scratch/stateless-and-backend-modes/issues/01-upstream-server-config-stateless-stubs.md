# 01 — Upstream Server Configuration Schema & Fingerprinting (Default Stateless Stubs)

**What to build:** Operators can define upstream MCP server stubs in `bridge.yaml` without specifying a session mode, and they automatically default to `stateless: true`. For backends requiring session continuity, operators can explicitly configure `stateless: false`. The bridge's configuration loader parses this setting, and the configuration fingerprinter includes it in server projections so that changing an upstream stub's mode triggers a proper configuration reload.

**Blocked by:** None — can start immediately.

**Status:** completed

- [X] Configuration schema in `internal/config/config.go` supports an optional `stateless: true|false` boolean on `Server`. When omitted or nil, `ServerStateless()` reports `true`.
- [X] Explicitly setting `stateless: false` in YAML causes `ServerStateless()` to report `false`.
- [X] Configuration fingerprinting in `internal/config/fingerprint.go` includes each server's effective stateless mode in its JSON projection so that toggling the flag changes the fingerprint.
- [X] Unit tests in `internal/config/config_test.go` and `internal/config/fingerprint_test.go` verify default stateless parsing, explicit stateful parsing, and fingerprint differentiation.
