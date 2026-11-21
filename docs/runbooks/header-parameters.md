# Header parameter transit

## Protocol versions

Protocol version `2026-07-28` and later carries `Mcp-Param-<name>` headers for parameters annotated with `x-mcp-header`; older negotiated versions cannot. A missing `InitializeResult` is treated as below `2026-07-28`.

## Mechanism

Stateless mode opens a fresh session per call, checks the header-annotation index and protocol gate, then runs one paged `tools/list` definition pass until the target tool page is found, within the single `limits.call_timeout`. The SDK then sends one `tools/call` with the unchanged body arguments and parameter headers. Stateful mode reuses the discovery session and applies the same gate and pass; a positive `tools/list` TTL gives zero pass cost, while a zero-TTL miss permits one refetch pass. Body-only tools pay no pass. A successful pass emits no extra log record.

## Failure code and operator action

For a header-bound call below the version threshold, the bridge returns payload-free `parameter_header_unsupported` with `execution_state=not_started`. The gate precedes the definition pass, so this failure has zero tool executions and zero post-gate `tools/list` requests. Upgrade the upstream or pin/repoint it to negotiate `2026-07-28` or later, or call a body-only tool. Repeated records classify as `configuration` and non-retryable.

## Cost

A definition pass can require one upstream `tools/list` request for every page up to and including the target tool's page, all within the single `limits.call_timeout`. Every stateless header call pays this pass; stateful zero-TTL misses may refetch, while a positive TTL removes the stateful pass cost. Raise `limits.call_timeout` for large catalogs.
