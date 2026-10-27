# Outer HTTP diagnostics runtime smoke — PASS (2026-10-26)

Command: `go run ./.scratch/oauth-session-diagnostics/runtime-smoke` → exit 0.

Final output: `runtime-smoke ok: post_mcp=401 livez=200 unknown_path=404 ids=3 unique=3 http_complete=3 diag_schema=oauth_diag_v1 reason=credentials_missing failure_stage=outer_oauth session=not_evaluated upstream=not_attempted protected_inner=0 root_fallback=1 keyset_calls=0 max_row_bytes=1524 canaries_absent=true`

Real loopback HTTP against the landed 02 production composition (`observe.HTTPDiagnostics.Wrap` → mux `/livez`, `/mcp` = `auth.NewAuthMiddleware` → `service.NewInboundHandler`, `/` stub fallback):

- Credential-free POST `/mcp` → 401 + Bearer resource challenge; row: `failure_reason=credentials_missing`, `failure_stage=outer_oauth`, `session_state=not_evaluated`, `upstream_state=not_attempted`; 3× `http_complete`, `diag_schema=oauth_diag_v1`, max row 1524 B (≤4096).
- `/livez` 200; unknown path 404; every response carried a unique 32-lowerhex `Mcp-Diagnostic-Id`; exactly one matching completion per returned ID; spoofed caller canaries absent from headers/bodies/log rows.
- Isolation: protected-inner 0 calls; KeySetProvider 0 invocations.

Throwaway-smoke fixes during run (script only, no production/test source): `jwk.Set` composite literal → `nil`; root stub now emits configured 404; row keys corrected to approved schema (`failure_reason`, `session_state`, `upstream_state`) per spec.md:126/144/146/153.

Not covered: real issuer/JWKS flows (out of smoke scope); facade capability matrix (still failing in suite — separate item).

Throwaway smoke main.go removed after proof (2026-10-26); results retained.
