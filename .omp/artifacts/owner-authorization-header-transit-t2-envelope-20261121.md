# Owner directive - T2 required restart oracle path

Recorded: 2026-11-21 (installer sets exact timestamp).

Owner directive, verbatim:

> authorize ticket 02 and dispatch it

Necessary scope detail: AC02-3 requires `TestParameterHeaderRestartSurvivesInStatelessMode` in `cmd/mcp-bridge`, but T2's recorded effects omit `cmd/mcp-bridge/serve_header_params_test.go` (T1 owned it and is complete). The installed owner amendment adds exactly that file to T2's effects so the authorized ticket can satisfy its own acceptance criterion. No other path is added.
