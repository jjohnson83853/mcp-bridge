# Headless MCP Bridge — Configuration and Secret Diagnostics (SpecDD)

## 1. STATUS

**Spec ID:** HMB-CD-v1 · **Revision:** 5 · **Status:** EXECUTION-AUTHORIZED — BridgeCritic7 CONFIRMED (r5) + user execution checkpoint recorded (§9) · **Date:** 2026-10-16.

**Objective:** Attribute every configuration rejection to its document, YAML location and affected server/pin/reference, with an actionable, value-free cause rather than an opaque secret-validation message.

**Artifact:** `.omp/plans/headless-mcp-bridge-config-diagnostics.md`. Spec-only: no production source, tests, configuration, checkpoint ledger or gates changed. Existing headless-mcp-bridge.md and headless-mcp-bridge-checkpoints.md were read first as format/workflow precedents, not current runtime proof. R-28–R-34, AC-30–AC-39 and T-15–T-20 continue after the prompt increment's R-27/AC-29/T-14. No parent functional requirement removed. Prior implementation approvals and the two policy resolutions below do not authorize this increment.

**Revision record:** r2 added G1–G6 probes/schema barrier. r3 addressed F-01 legacy secret tests/literal decisions/env-sequence acceptance/classified private-file helper. r4 recorded coordinator-confirmed user acceptance of U-01/U-02 defaults, no design change. **BridgeCritic6 verdict on r4: APPROVE-WITH-AMENDMENTS, C-1 HIGH and C-2/C-3 MINOR. r5 is the coordinator-routed amendment addressing all three, with no design change and stable R/AC/T IDs:** §5.2 explicitly enumerates parser_edges_test.go/validate_edge_test.go/api_edges_test.go cases and literal choices, T-16 runs their actual names; §4.5/AC-38/T-19 pin exact logger fields; §3.2/§5.2 distinguish non-nil file-policy guards from substring oracles. **BridgeCritic7 delta re-review of r5: CONFIRMED** — C-1/C-2/C-3 all closed against the live tree, no new findings, no design change (verdict appended at §8.5). The next gate is the **user execution checkpoint** on this spec; until it is recorded the task ledger stays unstarted and gated. No implementation executed or approval inferred.

## 2. REQUIREMENTS

| ID | Required behavior / constraint | Acceptance | Tasks |
|---|---|---|---|
| R-28 | Attribute every §3 failure to config path/full YAML key/index where known, with relevant server/pin/header/reference identity. Syntax uses document/root/available parser position, never guessed key. Identifier attribution normative under resolved U-02. | AC-30,AC-33–AC-35 | T-15,T-16,T-18 |
| R-29 | Separate unset/empty, over-8KiB, non-UTF8, CR/LF/NUL; explicitly name newline causes. Distinguish missing/uninspectable/unsafe/unreadable files. Report independent reference failures rather than silently first only; bound output with explicit incompleteness. | AC-31,AC-32,AC-36 | T-16,T-17 |
| R-30 | Never disclose secret contents/previews/hashes/fingerprints/exact lengths/offsets or raw YAML/URL/OS text. Necessary validated identifiers/coarse classes only. | AC-31,AC-33,AC-37,AC-38 | T-15–T-19 |
| R-31 | Preserve strict YAML/schema/bounds/URL/header/pin restrictions, relative-file resolution/private-file policy/exactly-one-terminal-LF file normalization. No env trim or policy relaxation for startup. | AC-32,AC-34,AC-35,AC-39 | T-16,T-17,T-20 |
| R-32 | Preflight/startup one attributable escaped physical stderr line/exit1/existing outer prefixes; replace opaque bodies, no shim/duplicate generic append. Rejected reload retains working state, safe diagnostics in existing logger. | AC-30,AC-36,AC-38 | T-18,T-19 |
| R-33 | Deterministic unit/CLI/composition/reload/negative-leakage tests and concrete offline commands; retain >=90% project source-line gate. | AC-30–AC-39 | T-15–T-20 |
| R-34 | Document Compose-visible output/safe repairs; record user policies separately from technical design. | AC-36,AC-39 | T-18,T-20 |

**Non-goals:** Repair/inject credentials/prove actual culprit; change Compose interpolation/restart, schema/defaults/auth/discovery/network or preflight validation scope; accept CRLF files/arbitrary whitespace trim; new observability stack/dashboard/alerts/listeners/dependencies/remote diagnostics. Aggregation never permits partial startup. No old-body shim.

## 3. EVIDENCE AND UNCERTAINTY

### 3.1 VERIFIED live-source failure inventory

Repository-relative anchors. Quoted text is **exact current literal/format template**, not fabricated expanded stderr; `%s/%q/%w` remain substitutions. External error returns have no one literal, so operation/return site is given. Partially attributable errors included: key without file/server/index is insufficient.

Every row adopts **C0 (§4.1): path, complete known location, finite code/property/shape, no value reflection**. Context abbreviations: K indexed YAML key/type; S server index+validated ID; P pin index+safe server/tool; H header+S; R secret consumer+kind/name+applicable S; B property+static range+coarse class. Duplicates identify first occurrence too. §5.2 fixes legacy reason choices explicitly; retained phrases are static explanations, not compatibility shims.

#### A. Parse/document/semantic — internal/config/config.go

| Site | Exact current text / return | Improved contract beyond C0 |
|---|---|---|
| :67 | `configuration exceeds 1 MiB` | Document/config_too_large/over_limit/fixed1MiB, no actual bytes. |
| :72,:88 | `decode configuration: %w` | Document/K, yaml_syntax or yaml_type, safe available parser position, never wrapped prose. |
| :76 | `configuration must contain one YAML document` | Document/yaml_documents/document_count/not_one; static reason retained within record. :75 also routes second-document decode error here. |
| :79 | `configuration must be a mapping` | Document/yaml_root_type/actual kind/expected mapping. |
| :91 | `schema_version must equal 1` | schema_version/code schema_version/missing or unsupported_integer/expected1, no scalar echo. |
| :94 | `servers must contain 1..1024 entries` | servers/server_count/empty or over_limit/static range. |
| :99 | `invalid server id %q` | servers[i].id/server_id_invalid/invalid_identifier; invalid scalar withheld. |
| :102 | `duplicate server id %q` | S+first servers[j].id/server_id_duplicate. |
| :107 | `server %s endpoint: %w` | servers[i].endpoint+S; extend existing server-ID precedent with file/index/property/URL cause. |
| :114 | `invalid pin` | P; pin_fields, pin_server_missing, pin_tool_missing or pin_server_unknown; precise property/static reasons below, not vague old body. |
| :118 | `duplicate pin` | P+first pin/pin_duplicate; reason duplicate pin within metadata record; same pair equality :116–120. |
| :130 | `embedding base_url: %w` | embedding.base_url/URL cause, old wrapper replaced by key metadata. |
| :155 | `client tokens and backend credentials must be disjoint` | Both consumer keys/R identities/credential_overlap, no values/hash/equality mechanism; old standalone body gone. |
| :166 | returns os.ReadFile(path) error unchanged | Requested path/config_read/not_found,permission_denied,io_error, no OS text. |
| :183 | `invalid absolute HTTP(S) URL` | Owning URL K/S/url_invalid/property absolute_http_url/shape invalid_url, no URL component. |
| :189 | `URL query contains a secret parameter` | Owning URL K/S/url_secret_parameter/query/credential_parameter, neither query name nor value. |

#### B. YAML/reference syntax/types — same file

| Site | Exact current text | Improved contract beyond C0 |
|---|---|---|
| :232 | `secret must be exactly {env: NAME} or {file: PATH}` | R consumer/reference_shape/expected single env-file mapping/actual shape, malformed identity omitted. Static reason retained. |
| :236 | `invalid secret reference` | R/reference_invalid/reference_name/missing,empty,wrong_type. |
| :239 | `invalid environment variable name` | R/reference_env_name/invalid_identifier; unsafe name omitted; static reason retained. |
| :242 | `secret references cannot contain interpolation or commands` | R/reference_literal_required/kind if known, unsafe name omitted; reason retained. |
| :247 | `expected mapping` | K/yaml_type/actual kind/expected mapping. |
| :253 | `aliases, merge keys, and non-string keys are not allowed` | Parent K/key position/yaml_key_invalid/alias,merge,non_string, no raw key. |
| :256 | `duplicate key %q` | Recognized K or parent+position/yaml_key_duplicate/first position, no arbitrary unknown text. |
| :261 | `unknown key %q` | Parent K/key position/yaml_key_unknown/property unknown_key, text withheld. |
| :264,:287 | `YAML aliases are not allowed` | Indexed K/yaml_alias. |
| :269 | `non-finite numbers are not allowed` | K/number_nonfinite/nonfinite_or_invalid, no scalar. |
| :274 | `secret must use an env or file reference` | R/reference_required/actual shape, inline value absent, static reason retained. |
| :290 | `secret must be a reference` | Indexed token R/reference_required/property reference, reason retained. |
| :333 | `%s must be a boolean` | Full K/yaml_type/expected bool-actual shape. |
| :344 | `%s must use secret references` | R/reference_required/actual shape. |
| :350 | `%s must be a string` | Full K/yaml_type/expected string-actual shape. |
| :353 | `%s must be an integer` | Full K/yaml_type/expected integer-actual shape. |
| :356 | `%s must be numeric` | Full K/yaml_type/expected number-actual shape. |
| :359 | `%s must be a sequence` | Full K/yaml_type/expected sequence-actual shape. |
| :364 | `%s entries must be strings` | Full K/entry index/yaml_type/expected string-actual shape. |
| :369 | `%s must be a structured value` | Full K/yaml_type/expected mapping_or_sequence-actual shape. |

#### C. Headers/secrets — same file

| Site | Exact current text / return | Improved contract beyond C0 |
|---|---|---|
| :376 | `credentials.headers must be a mapping` | S/headers K/header_mapping/actual shape. |
| :382 | `invalid credential header name` | S/headers parent/key position/header_name_invalid, invalid name omitted, reason retained. |
| :386 | `duplicate credential header %q` | H/first occurrence/header_duplicate; reason duplicate credential header, identity in metadata; same case-folding :384–388. |
| :391 | `credential header %q is forbidden` | H/header_forbidden; reason credential header is forbidden, no value. |
| :396 | `credential header value must use a secret reference` | H/R/reference_required/shape only, reason retained. |
| :440 | `secret file must be regular, non-symlink, operator-owned, and private` | R; Lstat not_found/permission_denied/io_error separate from policy classes §4.2, same predicates :418–423,438–440; opaque-only body gone. |
| :444 | returns os.ReadFile(path) error unchanged | R/secret_file_read/permission_denied or io_error, no OS prose. |
| :449 | `secret must be nonempty UTF-8, <=8KiB, without CR/LF/NUL` | R/exact §4.2 code-class-reason, no content/size, combined body retired. |

resolveSecrets uses os.Getenv (:431–432), so unset/empty collapse; file removes exactly one final LF (:446); recursive walk first-error-return (:454–463). base is config directory only (:137,426), not location; result kind:name identity (:451) not in current errors. Lstat errors/private-policy failures collapse :439–440; resolution before disjointness :136–155. Source facts, not production state proof.

#### D. Weights/bounds — same file

| Site | Exact current text | Improved contract beyond C0 |
|---|---|---|
| :511 | `invalid keyword_weight` | search.keyword_weight/weight_invalid/missing or invalid_numeric. |
| :514 | `invalid embedding_weight` | search.embedding_weight/weight_invalid. |
| :517 | `search weights must be nonnegative, sum to one, and keyword_weight positive` | Both keys/weights_invalid/static constraints, no observed numbers. |
| :548 | `%s outside permitted bounds` | B/bound_range/add indices-S to dotted path. |
| :561,:569 | `%s must be a duration` | B/duration_type/actual kind; defensive branch may be preempted by type validation. |
| :565,:580 | `%s outside permitted duration bounds` | B/duration_range/invalid_duration,nonpositive,below_minimum,above_maximum. |
| :586 | `%s must be a positive duration` | B/duration_positive/invalid_duration or nonpositive. |

Mapping paths :529–548, no sequence index :595–597. Numeric ranges :530–537. Session idle1s..1h :31–32,559–565; timeout30s/other_timeout120s existing heuristic :567–580. Positive scheduling/TTL/breaker :583–586. Preserve policy. slots range :536 not allowed schema :203–215, no new setting.

#### E. Runtime settings/accessor/restart guards

Some are defensive direct-Config guards, not normally reachable after Parse.

| Site | Exact current text | Improved contract beyond C0 |
|---|---|---|
| internal/config/command.go:151,157 | `configuration is missing %s` | Full K/setting_missing/missing. |
| command.go:163 | `%s must be a string` | K/setting_type/expected string-actual kind. |
| command.go:169,175 | `%s must be an integer` | K/setting_type/expected integer-actual kind. |
| command.go:181 | `%s must be a number` | K/setting_type/expected number-actual kind. |
| command.go:187,191 | `%s must be a duration string` | K/setting_duration/wrong_type or invalid_duration. |
| command.go:195 | `configuration reader has no destination for %s` | K/setting_internal/unsupported_destination, internal guard. |
| command.go:215,225 | `%s must be a sequence of strings` | K/bad index/setting_type. |
| internal/config/credentials.go:33 | `credentials reference unknown server %q` | Safe requested ID/servers/server_unknown/property credentials, reason credentials reference unknown server. |
| credentials.go:46 | `server %q credentials.headers must be a mapping` | S/K/header_mapping, reason credentials.headers must be a mapping. |
| credentials.go:51 | `server %q credential header name is invalid` | S/headers parent/header_name_invalid, canonical reason invalid credential header name. |
| credentials.go:67 | `limits reference unknown server %q` | Safe requested ID/servers/server_unknown/property limits, reason limits reference unknown server. |
| credentials.go:73 | `server %q limits.max_concurrent_calls must be an integer` | S/K/setting_type. |
| credentials.go:81,85 | `server %q limits.call_timeout must be a duration string` | S/K/setting_duration. |
| credentials.go:104 | `embedding.api_key must reference exactly one secret` | R key/reference_shape, static reason must reference exactly one secret. |
| credentials.go:108 | `embedding.api_key does not resolve` | R key/valid known identity/reference_unresolved, static reason secret reference does not resolve. |
| credentials.go:118 | `server %q credentials.%s must reference exactly one secret` | S/R key/reference_shape, reason must reference exactly one secret, no inline scalar. |
| credentials.go:122 | `server %q credentials.%s does not resolve` | S/R key/known identity/reference_unresolved, reason secret reference does not resolve. |
| cmd/mcp-bridge/serve.go:50 | `%s can only change across a restart` | Candidate path/full K/restart_required, no setting values. |

ParseFile delegates Parse (:163–168); startup/reload then Command (serve.go:40–47). Preflight only ParseFile (main.go:49–55). “without resolving” comment main.go:46–48 contradicted by config.go:137,168, fix comment not behavior. Runtime remote-provider/upstream/storage/listener failures outside config inventory. Bounded error-constructor/return-error search fingerprint.go returned no matches; no diagnostic changes there.

### 3.2 VERIFIED channels/tests/container wiring

- main.go:15,39–43 stderr mcp-bridge: <error>/exit1; serve.go:58 configuration: %w. main.go:50–55 preflight invalid configuration:/exit1/reference-only success.
- serve.go:37 observe.NewLogger(stdout). observe/log.go:13–24,46–80 Entry/JSON; :53,:59 finite event/reason, :64–75 narrow extra fields, :106–130 sensitive-name redaction. secret_ref/config_key would be dropped/redacted; exact safe manifest §4.5 must be pinned by tests.
- wiring.go:411–419 discards Logf args/format; loop.go:95–103,189–191,204–208 wraps and passes error, :91–113 rejected-load candidate/reload count unchanged. Parse formatting alone insufficient for reload.
- configs/config.example.yaml:41 embedding env; :48 client; :64 developer bearer; :67 relative secret file; :74 portainer bearer. example_config_test.go:29–50 stages private one-LF file.
- example_config_test.go:80–90 empty client/prefix-only :88; serve_edge_test.go:37–44 malformed/prefix :43, :54–59 schema9/prefix :58. bounds_edge_test.go:48–83/:57–62 generic substrings/twoLF/no invalidUTF8 row.
- **F-01:** secret_validation_edge_test.go:8–19,26–28 nine negative substring cases; secret_sequence_edge_test.go:31–52 six bad token elements/reason :50 and :61–79 two legal env refs/two hashes; private_file_test.go:10–20,22–39 synthetic policy/bool assertions. LSP helper refs config.go:418/439 and private_file_test.go:35 only. All explicit §5.2.
- **C-1 live follow-up:** parser_edges_test.go:9–31 malformed-doc table, :34–59 direct readSetting guards; validate_edge_test.go:17–35 semantic table, :38–69 missing-file/unset-env/oneLF-file and :72–91 direct limits; api_edges_test.go:8–31 credential guards, :34–50 embedding guards and :53–68 fingerprint regression. Exact retired substrings/cases in §5.2. Scoped search across config tests confirmed nine matches for decode configuration/one YAML document/invalid pin/duplicate pin/does not resolve/exactly one secret/secret must be nonempty, including **validate_edge_test.go:51's unset-env assertion**, beyond critic's original list.
- **C-3 citation precision:** config_test.go:201–202,222–223,245–246 assert only non-nil errors for world-readable/symlink/foreign-owner files. They do NOT pin non-symlink/secret-file/value-message text. Retain these behavioral guards, not substring oracles. New classified-policy+full-context tests and full regression suite establish the new reason contract.
- Coordinator's scoped negative four-string search found no targeted opaque literals in embedding_credential_test.go/credentials_test.go; both stay regression inputs, no inference of zero impact.
- docker-compose.yaml:32,35–44 restart policy/path/seven ${NAME:?...} env bindings; :45–49 configs mount. configs/config.mcp-grafana-stack.yaml:45 embedding,:52 client,:73–76 portainer, other envs, no example file ref. Compose interpolation can reject before process; log doesn't prove host .env/deployed document/environment.
- README:180–190 full/race/vet/coverage; covcheck/main.go:22–25,59–62 default90% source-line.

### 3.3 Runtime evidence — coordinator execution, not planner execution

Planner exec unavailable: read xd://bash “No such tool”, read/search/LSP/write available. Coordinator supplied these live no-real-secret probes verbatim; user crash ground truth, not rerun for confirmation. No prior session transcript/recap used.

```text
All runs from repo root /home/opencode/projects/default/mcp-bridge, `go run ./cmd/mcp-bridge --check-config --config <file>`; stderr shown; exit status noted.

[E1] configs/config.mcp-grafana-stack.yaml, all 7 env refs unset (BRIDGE_CLIENT_TOKEN LITELLM_API_KEY PORTAINER_MCP_TOKEN GRAPHIFY_MCP_KEY PROXMOX_MCP_TOKEN MYSQL_MCP_TOKEN SSH_MCP_UPSTREAM_TOKEN):
  invalid configuration: secret must be nonempty UTF-8, <=8KiB, without CR/LF/NUL
  exit status 1
  -> Names neither the config path, the key path, the server ID, nor WHICH env var was empty. Matches the user's container log exactly (same string, `configuration:` wrap in cmd/mcp-bridge/serve.go:58).

[E2] configs/config.example.yaml, all four env refs unset (BRIDGE_EMBEDDING_KEY BRIDGE_CLIENT_TOKEN DEVELOPER_UPSTREAM_TOKEN PORTAINER_UPSTREAM_TOKEN):
  invalid configuration: secret must be nonempty UTF-8, <=8KiB, without CR/LF/NUL
  exit status 1
  -> Identical string to [E1]: two different documents, two different upstreams, one message.

[E3] configs/config.example.yaml, all four env refs set to `k`:
  invalid configuration: secret file must be regular, non-symlink, operator-owned, and private
  exit status 1
  -> NEW finding: this is a MISSING file, not a permission problem. The ref is configs/config.example.yaml:67 `X-API-Key: {file: "secrets/developer-api-key"}`; configs/secrets/ does not exist in the tree. The message does not name the path, does not say "missing", and misattributes absence as an ownership/permission violation.

[E4] PORTAINER_UPSTREAM_TOKEN="" with the other three set to `k`:
  invalid configuration: secret file must be regular, non-symlink, operator-owned, and private
  -> The file-ref failure MASKS the env-ref failure; the operator cannot tell that an env var is also empty. Traversal stops at the first failure, and the reported failure is the least informative one.

[E5] PORTAINER_UPSTREAM_TOKEN=$'k\n' (trailing newline) with others set:
  invalid configuration: secret file must be regular, non-symlink, operator-owned, and private
  -> trailing-newline cause is masked here too; when unmasked it collapses into the same [E1] string.

[E6] PORTAINER_UPSTREAM_TOKEN=$(8193 x's) with others set:
  invalid configuration: secret file must be regular, non-symlink, operator-owned, and private
  -> oversized value likewise masked; when unmasked, same [E1] string.
```

```text
Second batch: UNMASKED probes on the env-only document (configs/config.mcp-grafana-stack.yaml has no {file:} refs, so these reach the secret-value check at config.go:449 directly). Command form: `env <VARS> go run ./cmd/mcp-bridge --check-config --config configs/config.mcp-grafana-stack.yaml`. Copy verbatim.

[G1] all 7 refs set to the single value "k":
  invalid configuration: client tokens and backend credentials must be disjoint
  exit status 1
  -> THIRD unattributable class: internal/config/config.go:155, and it names neither the offending ref names nor the collision pair.

[G2] all set except MYSQL_MCP_TOKEN="" (empty):
  invalid configuration: secret must be nonempty UTF-8, <=8KiB, without CR/LF/NUL
  exit status 1

[G3] all set except PORTAINER_MCP_TOKEN=$'k\n' (trailing newline):
  invalid configuration: secret must be nonempty UTF-8, <=8KiB, without CR/LF/NUL
  exit status 1
  -> Identical to G2. A trailing-newline value is indistinguishable from an empty one, and the message does not mention the newline.

[G4] all set except LITELLM_API_KEY=$(8193 x's) (oversized):
  invalid configuration: secret must be nonempty UTF-8, <=8KiB, without CR/LF/NUL
  exit status 1
  -> Identical to G2/G3: over-8KiB reads the same as empty. Operator cannot tell "the .env value has a trailing newline" from "the var is unset" from "the value is too long".

[G5] non-UTF8 byte in a value: NOT reproducible through this command channel — the tool rejected the byte string before exec (`error: i/o error: stream did not contain valid UTF-8`). The spec must therefore specify a fixture-based test (Go test writing raw bytes into a {file:} secret, as bounds_edge_test.go already does at :57-62) rather than a shell probe for this case, and must state that this is a harness limitation, not a product behaviour.

[G6] all set except BOTH MYSQL_MCP_TOKEN="" and SSH_MCP_UPSTREAM_TOKEN="" (two simultaneous failures):
  invalid configuration: secret must be nonempty UTF-8, <=8KiB, without CR/LF/NUL
  exit status 1
  -> Same single line as G2. Run twice (G6 and "G6 repeat"): byte-identical output both times. Two broken refs are reported as one unattributed failure; the operator fixes one, restarts, and hits the next.
```

Limits: E4–E6 only masking; G2–G4 unmasked evidence. G5 harness pre-exec, not product; raw bytes via Go fixtures. G6 multi-failure generic. E2 upstream wording doesn't identify first secret (embedding :41/:45). exit status1 go-run extra stderr, app exit1 main.go:52–53. Numeric status explicitly supplied E1–E3/G1–G4/G6, not separately E4–E6. $(8193 x's) size notation, not runnable shell syntax. No production culprit proven.

### 3.4 Uncertainty/access

- UNKNOWN actual deployed invalid ref/class/document/env; private identity-matched preflight/diagnostics, no value dumps; not task prerequisite.
- [INFERENCE] LF/CRLF plausible operational causes, G3 synthetic proof only.
- UNKNOWN external full-body consumers, but policy resolved U-01 replacement; no shim blocker.
- Bounded MCP/find/LSP ParseFile nine refs/private helper three; C-1 bounded reads of named files and scoped literal search, not broad restart. Direct docs/config reads only, no credentials. Graphify unnecessary/skipped/no clean/fresh claim. Planner no builds/tests/formatters/network/Docker/Git; coordinator probes distinguished.

## 4. DESIGN

### 4.1 C0 metadata-only error/renderer

Existing Parse/ParseFile/accessor signatures. config.Diagnostic/config.Diagnostics error collection, errors.As after %w. Only path/location/position, validated owner/pin/header/ref, finite code/property/shape, static reason/hint/expectation, related location/ref. Copied records/no mutable slices. No credential/arbitrary text; retained causal errors errors.Is/classification only, never prose formatting.

Supplied config path unchanged/nonserialized origin for later methods. Direct Config no origin -> <memory>. String/fingerprints/snapshots/catalog identity unchanged; missing file requested path. Dotted schema+zero-based indices; validated header bracket quote; malformed key parent+position; invalid/duplicate/missing IDs index remains; syntax <document>/available safe position, no invented key.

```text
config="<path>" key="<location>" [line=N column=N] [server="<id>"] [pin_server="<id>" pin_tool="<name>"] [ref="<kind>:<name>"] [related_key="<location>" related_ref="<kind>:<name>"] code=<code> property=<property> shape=<shape>: <static reason>[; <static hint>]
```

Brackets optional only by relevance/availability, not pending user policy. Order fixed/Go quoted escaping/no physical controls. Syntax/invalid-key position required when available; ordinary optional. Omit only genuinely missing/unsafe IDs; emit all valid relevant identifiers. No invented server. Static reasons replace echoes. Parser syntax `cannot decode YAML configuration`; wrong decoded mapping `must be a mapping`; yaml_documents `configuration must contain one YAML document` retained static. All §5.2 choices normative, not compatibility shims.

Pin reasons fixed: pin_fields `pin must contain exactly server and tool`; pin_server_missing `pin server must be nonempty`; pin_tool_missing `pin tool must be nonempty`; pin_server_unknown `pin references unknown server`; pin_duplicate `duplicate pin`. Metadata carries safe tuple/related first index. Accessor reference_shape `must reference exactly one secret`; reference_unresolved `secret reference does not resolve`. These are static policy reasons, never raw Config/decoder text.

Resolved U-01: prefixes invalid configuration: (main.go:52)/configuration: (serve.go:58)/exit1 retained. Whole opaque bodies replaced; no retention shim/duplicate append. T-15/T-18 must test this.

### 4.2 Secret value/file causes and unchanged LF rule

LookupEnv unset vs empty, both invalid. File exactly one TrimSuffix LF (:446), not whitespace/repeated/CRLF normalization; env no trim; size post-file normalization <=8192 if otherwise valid. Primary **unset, empty, over-limit, invalidUTF8, NUL, CR, LF**, stable order/avoid scans of over-limit. Other refs still checked.

| Code | Shape | Exact reason/hint (property=value) |
|---|---|---|
| secret_unset | unset | `secret environment variable is unset; set the named reference without printing its value` |
| secret_empty | empty | `secret must be nonempty; provide a nonempty value for the named reference` |
| secret_too_large | over_limit | `secret exceeds permitted 8KiB limit; provide a value within the configured limit` |
| secret_invalid_utf8 | invalid_utf8 | `secret is not valid UTF-8; replace it with a valid UTF-8 credential` |
| secret_nul | contains_nul | `secret contains NUL; remove the NUL from the credential source` |
| secret_cr | contains_cr | `secret contains CR (carriage return); CRLF is not normalized` |
| secret_lf env | trailing_lf/contains_lf | `secret contains LF (newline); environment values are not newline-stripped` |
| secret_lf file | trailing_lf/contains_lf | `secret contains LF (newline) after stripping one final LF; files permit only one removable final LF and no remaining CR/LF/NUL` |

trailing_lf means validated value ends LF, no offset/count. Explicit LF/newline for env final/extra file LF, CR for CRLF after stripping LF. One final file LF valid silently; onlyLF -> empty. No weakened private policy/blind env trimming advice.

| File code | Property / shape | Static reason |
|---|---|---|
| secret_file_missing | existence/not_found | `secret file does not exist` |
| secret_file_stat | metadata/permission_denied,io_error | `cannot inspect secret file` |
| secret_file_policy | precise property/symlink,non_regular,owner_unverified,wrong_owner,not_private | `secret file must be regular, non-symlink, operator-owned, and private` |
| secret_file_read | readability/permission_denied,io_error | `cannot read secret file` |

File metadata/read before value checks. Replace private bool helper with private classifier proposed secretFilePolicy returning success/finite cause, resolver+test migrated/no wrapper. Same predicates :418–423: symlink first, non_regular (incl nil), owner_unverified if no usable Stat_t, wrong_owner, not_private. Same fail-closed, no policy expansion. Lstat failure handled before classifier. No UID/mode/size. Configured relative path internally config-dir-resolved, no second expanded output. Existing Lstat/read semantics, not unrelated hardening.

Overlap reason `client tokens and backend credentials must be disjoint` after record with both refs. **Whole standalone bodies retired:** combined secret-value string, generic private-file string and disjointness string from :449/:440/:155. Useful policy text survives only as reason in attributed record, not whole body or duplicate generic append. Tests reject opaque-only/duplicate output; new code+context is regression contract.

### 4.3 Identifier visibility/redaction

U-02 resolved: configured file paths/ref names env:NAME,file:relative/path/pin tools may appear escaped locally; relevant valid identifiers must appear per R-28/AC-35. Also known schema/index/position/validated server/header/static limit/type/finite class. Unsafe ref/unknown-invalid key withheld for parent/position, never inferred from inline secret. Escape supplied config path and control/nonUTF8 metadata; no expanded resolved path.

NEVER resolved/inline values, prefixes/suffixes/previews, encodings/hashes/fingerprints, exact observed length/count/offset, YAML snippets/Config/maps, URL components/query names-values, raw decoder/OS/HTTP/stack/env. No value/actual/length/bytes/fingerprint fields. over_limit+static8KiB only size disclosure; unset/empty necessary classes not telemetry. Other scalar errors coarse kind/tag/missing/empty/unsupported_integer/invalid_identifier/invalid_url/credential_parameter/invalid_duration/nonpositive/below_minimum/above_maximum/finite policy-I/O. No observed numeric scalar echo.

Construct from validated metadata/trusted reasons, not scrub-after-format. Identifier visibility NEVER value visibility. No remote surface. Distinct canary values/valid metadata, separate malicious scalar-key-URL fixtures; cannot infer operator intent to keep an identifier confidential.

### 4.4 Aggregation/stages

Decode/root -> structural/type -> schema -> semantic servers/pins/bounds/weights/URLs -> secrets -> overlap -> Command only startup/reload. Unsupported schema own barrier; safely collect within stages/YAML order then no next stage on failure. Malformed stage no file reads; not_checked footer explicitly skipped later stages. Syntax one document error. No false completeness.

Secret stage all valid occurrences despite failure; cache kind/name/no repeated shared reads, each affected owner reported. No secret copies in records/failed refs in successful map/unresolved overlap/partial Config/runtime resources. Failed secrets skip later checks. First overlap pair document order, no mechanism.

Max32 records; further failure stop with `diagnostics_incomplete=true; further locations may be invalid; fix reported errors and rerun`; no guessed total. Single direct record; multi `diagnostics=N: <record> | <record>` one physical line, footer once. Count diagnostics not secret bytes. No changed exit/restart/publication.

### 4.5 Channels and exact logger schema

1. Preflight stderr invalid configuration: <collection> newline/exit1/stdout empty; success unchanged/ParseFile-only/comment correction.
2. Startup mcp-bridge: configuration: <collection> newline/exit1/no duplicate JSON.
3. Reload stdout existing JSON/Logf; typed errors.As only/no arbitrary args, finite existing non-config fallback. One record/diagnostic <=32/incomplete marker if cap/event reload/level warn/outcome failure/reason invalid_config/existing generation/correlation.
4. **Exact C-2 field manifest (r4 spellings retained, no rename):**
   ```text
   config_path location line column code property shape ref_kind ref_name
   pin_server pin_tool related_location related_ref_kind related_ref_name
   diagnostic_index diagnostics_incomplete remaining_stages
   ```
   Existing top-level server=valid owner or _bridge. Human key -> location, ref -> ref_kind/ref_name. These exact snake_case keys, not configpath/refname/refkind aliases. Typed config-diagnostic reload entries only; numeric indices/line/column. Narrow allowlist extension, no config_key/secret_ref/header/url/endpoint/message/error freeform or broad redaction bypass. No metric/public-health metadata.
5. **C-2 mandatory observe-level schema test:** new TestConfigDiagnosticFieldManifest supplies safe typed records covering every manifest field (use multiple records if mutually exclusive). Decode actual Logger.Log JSON, assert exact diagnostic key set/spelling/types and original safe values, not merely presence or nonempty. Assert no [REDACTED]/unknown substitution/omission for approved fields. Assert aliases configpath/refname/refkind absent. Negative cases for disallowed config_key/secret_ref/header/url/endpoint/freeform error confirm they are dropped/redacted according to existing policy, never reflected; unrelated credential redaction tests remain. Do not freeze randomized standard time/request ID values. This pins both allowlist and recursive redactor against future silent field rename/drop.

Illustrative proposed Compose output, NOT actual culprit attribution (valid document except empty Portainer):

```text
mcp-bridge  | mcp-bridge: configuration: config="/app/configs/config.mcp-grafana-stack.yaml" key="servers[3].credentials.bearer" server="portainer" ref="env:PORTAINER_MCP_TOKEN" code=secret_empty property=value shape=empty: secret must be nonempty; provide a nonempty value for the named reference; remaining_stages=not_checked
```

Specimen index3 (:65–76); Compose prefix external, compiled app no go-run status line; restart may repeat/no app retry. Developer file failure servers[0]/developer-api/X-API-Key/file ref/specific cause; multi one separated line/no generic append. Existing stderr/reload JSON enough, bounded metrics/readiness unchanged; new dashboards/alerts/tracing/health/series N/A.

## 5. ACCEPTANCE CRITERIA

Planned except observed probes. Behavioral RED then unchanged GREEN; compile failure not behavioral RED.

| ID | Observable pass/fail / method | Requirements |
|---|---|---|
| AC-30 | Full file/index/owner/ref/code/shape/reason; exact1/stdout empty/one escaped stderr/prefix retained/no opaque-only or duplicate shim. §5.1. | R-28,R-32,R-33 |
| AC-31 | Unset-empty/raw invalidUTF8 file+Go env/8192-8193/NUL-CR-LF/priority, correct acceptance/classes/no content-length-offset. G5 not product. | R-29,R-30,R-33 |
| AC-32 | One fileLF accepted, env/extra LF named newline, CRLF named CR; same private predicates incl unverifiable owner; two legal env refs/two client hashes preserved. | R-29,R-31,R-33 |
| AC-33 | Fixture/site/fullmetadata not substring-only incl malformed/second-doc/typed decode and direct Config guards. Every §5.2 legacy case retained with explicit phrase choice; no raw decoder. | R-28,R-30,R-33 |
| AC-34 | Same-property servers distinct index/ID, malformed IDs locatable; duplicate header owner/spelling/first/case-fold, no value; bounds full context/static range/predicate unchanged. | R-28,R-31,R-33 |
| AC-35 | URL no component; pin full index/property/safe tuple/related first; overlap both refs/no mechanism/hash/value. Known valid path/ref/pin metadata mandatory U-02; direct methods <memory> when no origin. | R-28,R-31,R-33 |
| AC-36 | File+later empty-LF-size both ordered; shared allowners/no reread; >32 cap/incomplete; early badstage no reads/partial and not_checked. | R-29,R-32,R-33,R-34 |
| AC-37 | All sinks no canary/previews/encodings/scalars/URL/query/unsafe key/exact size; control escaping, invalid names withheld, unrelated redaction unchanged. | R-30,R-33 |
| AC-38 | Invalid reload preserves state/routing; fields survive filters/wrappers/no arbitrary Logf. **Observe TestConfigDiagnosticFieldManifest pins every exact §4.5 key/type/value, no omission/redaction/alias; negative forbidden fields and unrelated redaction preserved.** | R-30,R-32,R-33 |
| AC-39 | Full/race/vet/build/>=90%; docs LF/file repair/completeness/prefix-exit/Compose/resolved policies. No weaker/deleted fixture for GREEN. | R-31,R-33,R-34 |

### 5.1 Concrete offline commands/output

No real credential/fixture write, /dev/stdin on Linux, invalid secret before Command/network:

```sh
env -u DIAG_TOKEN go run ./cmd/mcp-bridge --check-config --config /dev/stdin <<'YAML'
schema_version: 1
servers:
  - id: alpha
    endpoint: https://alpha.invalid/mcp
    credentials: {bearer: {env: DIAG_TOKEN}}
pins: []
YAML
```

Program empty stdout/exit1/exact stderr:

```text
invalid configuration: config="/dev/stdin" key="servers[0].credentials.bearer" server="alpha" ref="env:DIAG_TOKEN" code=secret_unset property=value shape=unset: secret environment variable is unset; set the named reference without printing its value; remaining_stages=not_checked
```

go-run extra exit-status line distinguished. Repeat heredoc env DIAG_TOKEN='' -> secret_empty/empty/reason; Bash env DIAG_TOKEN=$'canary\n' -> secret_lf/value/trailing_lf/env newline reason. Remove --check-config -> startup prefix mcp-bridge: configuration:; no runtime. Raw invalidUTF8/size/NUL Go fixtures, POSIX env cannot contain NUL.

Shipped specimen:

```sh
env -u BRIDGE_EMBEDDING_KEY -u BRIDGE_CLIENT_TOKEN \
  -u DEVELOPER_UPSTREAM_TOKEN -u PORTAINER_UPSTREAM_TOKEN \
  go run ./cmd/mcp-bridge --check-config --config configs/config.example.yaml
```

With E3 missing file unchanged one invalid configuration: diagnostics=5: line, each config=configs/config.example.yaml, ordered §4.1 records/§4.2 reasons/footer once:

| key | owner | ref | code/property/shape |
|---|---|---|---|
| embedding.api_key | none | env:BRIDGE_EMBEDDING_KEY | secret_unset/value/unset |
| client_auth.bearer_tokens[0] | none | env:BRIDGE_CLIENT_TOKEN | secret_unset/value/unset |
| servers[0].credentials.bearer | developer-api | env:DEVELOPER_UPSTREAM_TOKEN | secret_unset/value/unset |
| servers[0].credentials.headers["X-API-Key"] | developer-api | file:secrets/developer-api-key | secret_file_missing/existence/not_found |
| servers[1].credentials.bearer | portainer-api | env:PORTAINER_UPSTREAM_TOKEN | secret_unset/value/unset |

Legitimate file not called missing. Automated temp sample/absent file, no repo secrets/state assumptions. E4-E6 temp file missing+later Portainer empty/LF/8193 => both; G6 env doc/twoempty both identities, otherwise distinct canaries avoid incidental overlap, G1 deliberate overlap separate.

Existing serve_edge_test malformed :37–44 -> exact temp path/document/yaml_syntax property=document shape=invalid_yaml/static cannot decode YAML configuration/safe available position/footer, no yaml.v3 prose. schema9 :54–59 one schema key/code/schema_version property/unsupported_integer/static must equal1/path/footer/wrappers, no cascaded missing servers.

### 5.2 Existing-test migration and explicit static-reason matrix

Every complete body becomes code/context, all retained phrases chosen static explanation not body shim. No negative fixture deleted/weakened to obtain GREEN. Every row below also asserts supplied path or **<memory> for direct originless Config**, full known location/property/shape, escaping and applicable not_checked footer. Invalid inline values/unsafe names absent. For tuple/ref absent or malformed omit identity rather than invent one.

#### Original requested CLI/value tests

- example_config_test.go:88 fullprefix/exact1/empty stdout/oneline/tempfile/client_auth.bearer_tokens[0]/BRIDGE_CLIENT_TOKEN/secret_empty+reason/no canaries. Unset separate, success report :58–74 and oneLF :45 preserved.
- serve_edge_test.go:43 syntax full contract; :58 schema/wrapper/path/code plus runContext; reset buffers. Missing config :46–47 requested path/config_read/not_found.
- bounds_edge_test.go:57–62 empty->secret_empty,size->secret_too_large,CR->secret_cr,LF->secret_lf,NUL->secret_nul,twoLF->secret_lf/trailing_lf/afterstrip; fullcontext not generic CR/LF/NUL. :55–56 missing vs nonregular. Add UTF8/8192/oneLF/onlyLF/CRLF/env parity.
- **C-3:** config_test.go:201–202/222–223/245–246 are **non-nil-only rejection guards**, not literal oracles. Preserve world-mode/symlink/foreign-owner behavior. New classified cause/full-context tests explicitly pin retained secret_file_policy reason **within** records; these tests plus full suite are new-message regression basis. Never infer old substrings from those non-nil assertions.

#### F-01 malformed-secret/header matrix — secret_validation_edge_test.go

Use temp-joined config filename instead of t.TempDir as filename (:26); old :27 substring supplements only, never sole oracle. All nine negative cases retained.

| File line/case/current substring | Producer | Static reason decision | Required context/cause assertions |
|---|---|---|---|
| :10 literal bearer / secret must use an env or file reference | config.go:274 | Retain phrase as static reason. | servers[0].credentials.bearer/alpha/reference_required/string/no plain echo. |
| :11 invalid mapping / exactly {env: NAME} or {file: PATH} | :232 | Retain full static secret must be exactly {env: NAME} or {file: PATH}. | bearer/reference_shape/mapping/no valid-ref invented. |
| :12 env name / environment variable name | :239 | Retain invalid environment variable name. | bearer/reference_env_name/invalid_identifier/BAD-NAME withheld. |
| :13 interpolated file / interpolation or commands | :242 | Retain secret references cannot contain interpolation or commands. | bearer/reference_literal_required/file kind only/$HOME/token withheld. |
| :14 bad header / invalid credential header name | :382 | Retain static. | alpha/headers parent/key position/header_name_invalid/no bad text. |
| :15 forbidden / is forbidden | :391 | Canonical credential header is forbidden, identifier metadata only. | headers["Authorization"]/header_forbidden/owner/no value. |
| :16 case duplicate / duplicate credential header | :386 | Canonical duplicate credential header, identifier metadata only. | second x-test/related first X-Test/alpha/header_duplicate/casefold. |
| :17 literal header / must use a secret reference | :396 | Retain credential header value must use a secret reference. | header/alpha/reference_required/string/literal withheld. |
| :18 base_url / embedding base_url | :130/:183 | Replace old wrapper, no retained space-prefix. | embedding.base_url/url_invalid/absolute_http_url/invalid_url/no URL echo/server/ref invented. |

Sequence secret_sequence_edge_test.go:50 retains static secret must be a reference (:290), not standalone body. Six :37–42 cases get path/client_auth.bearer_tokens[0]/reference_required/property reference/actual string-int-bool-null-sequence shape/footer/no plain-secret or scalar echo; no invented ref. :61–79 two valid env refs accepted/two hashes/no silent drops/no diagnostics/no output hashes unchanged (AC-32). :27–30 comment only richer context, not weaker rule.

Private_file_test.go:22–39 migrate boolean to classifier while preserving :29 success/:30 wrong_owner/:31 not_private/:32 symlink. Add directories/nonregular/world mode/nil/owner_unverified/combined cause order. Synthetic Stat_t+os.Geteuid+uid1, no privileged chown/no mode-UID output. Missing/stat/read not faked as policy FileInfo; bounds resolver tests full secret_file_policy record; existing config_test nonnil integrations retained. All three LSP callsites migrate/no bool wrapper.

#### C-1 parser document cases — parser_edges_test.go:9–31

Every existing row retained. Producer columns are current error source, not newly invented runtime output. Type shape is finite YAML kind/tag, no supplied scalar.

| File line / case / asserted substring | Current producer | Retain-static-or-replace decision | Required full-metadata assertion |
|---|---|---|---|
| :15 oversized / 1 MiB | config.go:67 | Retain constant limit in static reason. | config.yaml/<document>/config_too_large/property document/over_limit, no observed size. |
| :16 malformed yaml / decode configuration | :72 | **Replace** substring with cannot decode YAML configuration. | config.yaml/<document>/yaml_syntax/document/invalid_yaml/available safe position/footer, no source echo. |
| :17 scalar root / mapping | :79 | Retain expected mapping static wording. | <document>/yaml_root_type/root/scalar-string, no hello echo. |
| :18 sequence root / mapping | :79 | Retain expected mapping wording. | <document>/yaml_root_type/root/sequence. |
| :19 second document / one YAML document | :76 | **Retain** full configuration must contain one YAML document as static reason within record. | <document>/yaml_documents/document_count/not_one, no second-doc dump. |
| :20 nested unknown / unknown key | :261 | Retain static unknown key without interpolated unknown text. | search parent+offending node position/yaml_key_unknown/unknown_key, not_a_setting withheld. |
| :21 wrong section type http:[] / decode configuration | :88 | **Replace** decoder substring with must be a mapping. | key http/yaml_type/property type/shape sequence/expected mapping, no raw wrapped decoder text. |
| :22 nonstring allowed host / entries must be strings | :364 | Retain static entries must be strings. | http.allowed_hosts[0]/yaml_type/type/integer, no 42 echo. |

The type error at :21 currently reaches strict decode; implementation may diagnose expected schema type earlier without changing acceptance. It must attribute http and sequence shape from parsed node, never print decoder prose or invent syntax error. YAML syntax and schema-type codes stay distinct.

#### C-1 direct setting guards — parser_edges_test.go:34–59

Each direct Config uses config=<memory>; new code does not fabricate a YAML source line. Existing key substrings remain present through location metadata, not compatibility prose. Each row separately retained:

| Line / existing case-substring | Producer | Decision | Required code/location/shape |
|---|---|---|---|
| :42 missing parent / upstream_defaults.breaker.consecutive_failures | command.go:151 | Retain as location, static configuration is missing. | setting_missing/exact full key/missing. |
| :43 missing leaf / same key | :157 | Retain location, same static reason. | setting_missing/exact key/missing. |
| :44 wrong string / http.listen | :163 | Retain location, static must be a string. | setting_type/http.listen/integer, no5 echo. |
| :45 wrong int / http.burst | :169 | Retain location, static must be an integer. | setting_type/http.burst/number, no1.5 echo. |
| :46 wrong int64 / http.max_request_bytes | :175 | Retain location/static integer. | setting_type/exact key/string, no large echo. |
| :47 wrong float / http.requests_per_second | :181 | Retain location/static number. | setting_type/exact key/string, no fast echo. |
| :48 duration type / search.timeout | :187 | Retain location/static duration string. | setting_duration/exact key/wrong_type. |
| :49 bad duration / search.timeout | :191 | Retain location/static duration string. | setting_duration/exact key/invalid_duration, no tomorrow echo. |
| :50 unsupported target / configuration reader has no destination | :195 | Retain static phrase, never destination dump. | setting_internal/http.listen/unsupported_destination. |

TestIntegerAndNumberValueBoundaries :62–81 remains unchanged acceptance-helper regression, no diagnostic output needed.

#### C-1 semantic cases — validate_edge_test.go:17–35

All retain non-nil rejection; full temp path already supplied :30. Add per-row context rather than broad want substring.

| Line / case / asserted substring | Producer | Decision | Required metadata/cause |
|---|---|---|---|
| :19 schema2 / schema_version | config.go:91 | Retain key+static reason must equal1. | schema_version/schema_version/property schema_version/unsupported_integer, no2 echo. |
| :20 unknown root / unknown key | :261 | Retain static unknown key, not raw key. | <document> root parent+node position/yaml_key_unknown/unknown_key, no unexpected echo. |
| :21 no schema / schema_version | :91 | Retain key/static reason. | schema_version/schema_version/missing. |
| :22 no servers / servers | :94 | Retain key/static range. | servers/server_count/empty. |
| :23 duplicate server / duplicate server id | :102 | Retain static duplicate server id, valid identity metadata. | servers[1].id/alpha/server_id_duplicate/related servers[0].id. |
| :24 invalid endpoint / endpoint | :107/:183 | Retain as full location, static URL reason. | servers[0].endpoint/alpha/url_invalid/absolute_http_url/invalid_url/no URL. |
| :25 unknown pin server / invalid pin | :114 | **Replace** vague phrase with pin references unknown server. | pins[0].server/pin_server_unknown/property server/shape unknown_server, safe pin_server absent and pin_tool x, no resolved owner invented. |
| :26 duplicate pin / duplicate pin | :118 | **Retain** static duplicate pin only within metadata record. | pins[1]/pin_duplicate/property identity/shape duplicate, pin_server alpha/pin_tool x/related pins[0]. |

Here `absent` in :25 is a syntactically valid configured pin-server identifier, not a credential: emit pin_server="absent" as requested identity metadata; “no resolved owner” means omit server owner, not omit pin_server. This makes unknown membership attributable without pretending the server exists.

#### C-1 file/env/limits — validate_edge_test.go:38–91

| Line / case / current assertion | Producer | Decision | Required verification |
|---|---|---|---|
| :39–43 missing file / non-nil only | config.go:166 | No literal promise to retain. | requested absent path/config_read/document/not_found; no raw OS. |
| :45–52 env explicitly unset / secret must be nonempty at :51 | :432/:449 | **Replace** old conflated phrase with §4.2 unset reason. Rename case description to unset, not empty; keep actual os.Unsetenv setup. | full temp config filename/servers[0].credentials.bearer/alpha/env:CONFIG_EDGE_MISSING/secret_unset/value/unset/footer. |
| :55–68 one final file newline / successful bearer token | :446,credentials.go:30–60 | Preserve success behavior, no diagnostic. | one LF removed and resolved bearer token unchanged; no output value. |
| :78 noninteger limit / max_concurrent_calls | credentials.go:73 | Retain as full location metadata, static integer reason. | <memory>/servers[0].limits.max_concurrent_calls/alpha/setting_type/number, no1.5. |
| :79 duration type / call_timeout | :81 | Retain location/static duration string. | <memory>/servers[0].limits.call_timeout/alpha/setting_duration/wrong_type. |
| :80 duration invalid / call_timeout | :85 | Retain location/static duration string. | same key/owner/setting_duration/invalid_duration, no soon. |

#### C-1 direct credential/embedding guards — api_edges_test.go

Direct Config stays direct, config=<memory>, no parser bypass weakening. References read only from valid single-reference shapes; unresolved map remains unresolved rather than silently injecting values.

| File line / case / substring | Producer | Decision | Required full-metadata assertions |
|---|---|---|---|
| :15 unknown server / unknown server | credentials.go:33 | Retain credentials reference unknown server static. | servers/server_unknown/property credentials/unknown_server, safe requested unknown ID/no fabricated index. |
| :16 headers wrong type / headers must be a mapping | :46 | Retain credentials.headers must be a mapping static. | servers[0].credentials.headers/alpha/header_mapping/type/string/no bad echo. |
| :17 bad header / header name is invalid | :51 | **Replace** word order with canonical invalid credential header name. | servers[0].credentials.headers/alpha/header_name_invalid/invalid_identifier; no YAML position invented/no bad header echo. |
| :18 unresolved bearer / does not resolve | :122 | **Retain** phrase via canonical secret reference does not resolve. | servers[0].credentials.bearer/alpha/env:TOKEN/reference_unresolved/reference/unresolved. |
| :19 literal bearer / exactly one secret | :118 | **Retain** must reference exactly one secret static. | servers[0].credentials.bearer/alpha/reference_shape/reference/string; no plain echo/no invented ref. |
| :29 limits unknown server / unknown server | :67 | Retain limits reference unknown server static. | servers/server_unknown/property limits/unknown_server/safe requested missing ID/no fake index. |
| :41 embedding two refs / exactly one | :104 | **Retain** must reference exactly one secret static (also contains exactly one). | embedding.api_key/reference_shape/reference/mapping, no single ref identity invented. |
| :42 embedding unresolved / does not resolve | :108 | **Retain** canonical secret reference does not resolve. | embedding.api_key/env:TOKEN/reference_unresolved/reference/unresolved/no owner invented. |

api_edges_test.go:53–68 fingerprint failures/success remain regression behavior, no fingerprint value in new diagnostics. Existing fingerprint test's internal assertions do not authorize logging hashes. These rows plus per-site matrix satisfy C-1; no fixture removed or rewritten to an easier malformed input.

Other embedding_credential_test.go/credentials_test.go stay full-suite regression; scoped negative search not permission to omit. Accessor provenance assertions updated when affected.

## 6. TASK LEDGER

**EXECUTION-AUTHORIZED (2026-10-16).** r5 independent critic re-review CONFIRMED (BridgeCritic7, §8.5); user execution checkpoint recorded at revision r5 covering requirements (R-28–R-34), design (§4.1–§4.5) and this task ledger (T-15–T-20) — see §9. Authorized implementer: the `engineer` role, TDD RED→GREEN per task in ledger order, no acceptance weakening, no scope extension. Main consolidated checks run once after the slices converge. U-01/U-02 policies resolved and normative.

### T-15 — Records/renderer/origin

- Deps/inputs: approved r5 after re-review/checkpoints, §3/§4.1/4.3/§8.2/LSP.
- Components: proposed diagnostics.go/config.go/tests; safe metadata copies/errors.As/origin only, no schema/default/dependency/fingerprint/snapshot change.
- RED fullmetadata/safe-line/opaque-only-duplicate rejection fails old output; not compile-only.
- GREEN one renderer/provenance/no causal prose/static reasons no shim; `go test ./internal/config -run 'Test.*Diagnostic' -count=1` unchanged tests.
- AC-30,AC-33,AC-37; stop value reflection/identity effect/old-body fallback.

### T-16 — Complete site and legacy-case migration

- Deps T-15/§3+§5.2 all matrices/current ranges/resolved policies. Explicit parser_edges_test.go, validate_edge_test.go, api_edges_test.go, secret_validation_edge_test.go, secret_sequence_edge_test.go; direct fixtures stay direct.
- Components config.go/command.go/credentials.go/tests, path/cause/static reason/schema barrier only. No new predicates/preflight Command checks/guessed syntax keys.
- RED every named case gets full metadata/code-class oracle even if phrase retained; preserve input/negative fixture, no weakening Parse to reach guard.
- GREEN all site metadata/literal choices/related positions/type distinction. **C-1 exact test-name gate, not broad regex that misses cases:**
  ```sh
  go test ./internal/config -run '^(TestParseRejectsMalformedDocuments|TestReadSettingReportsMissingAndWrongTypedValues|TestParseValidationEdges|TestParseFileAndCredentialResolutionEdges|TestServerLimitsRejectInvalidValues|TestCredentialAccessorsRejectUnknownOrMalformedReferences|TestEmbeddingCredentialResolutionFailures|TestParseRejectsMalformedSecretReferencesAndHeaders|TestParseRejectsNonMappingBearerTokenElements|TestParseAcceptsEnvBearerTokenReferences)$' -count=1
  go test ./internal/config -count=1
  ```
  First command explicitly includes the current names from all three C-1 files. Do not substitute Test.*(Credential|Secret|Diagnostic|Parse): it misses TestReadSettingReportsMissingAndWrongTypedValues/TestServerLimitsRejectInvalidValues. Full package remains necessary.
- AC-32–AC-35,AC-37. Stop omitted negative case/unsupported reachability/literal-policy drift/two-hash acceptance change; record fixture baseline, fix implementation not acceptance.

### T-17 — Specific secret/file causes and aggregation

- Deps T-15,T-16/§4.2/4.4/E-G/private_file matrix/three LSP helper refs.
- Components secret walk/classifier/collection/bounds/private_file/sequence-positive. Migrate boolean resolver+test/no wrapper. LookupEnv/cache allowed; env trim/CRLF/relaxed policy/unresolved overlap/secret copies forbidden.
- RED cause/simultaneous/shared/cap/footer assertions fail old code; classified synthetic tests preserve acceptance; rawUTF8 Go file not G5.
- GREEN all finite classes/stage-safe order/no partial:
  ```sh
  go test ./internal/config -run 'TestPrivateSecretFileRequiresCurrentOwnerAndPrivateRegularFile|TestParseCredentialFileBoundaries|TestParseAcceptsEnvBearerTokenReferences' -count=1
  go test ./internal/config -count=1
  ```
  Renamed policy test record exact name/preserve cases, never disappear as GREEN. Fullpackage includes C-1 cases beyond any targeted filter.
- AC-31,AC-32,AC-36,AC-37; stop changed policy/panic/redundant read-copy/partial admit/privileged chown.

### T-18 — Prefix-preserving preflight/startup

- Deps T-15–T-17/main/serve/original CLI tests/resolvedU-01.
- Components CLI/error route/comment/tests only, no scope/retry/logstack/container change.
- RED fullstatus/stdout/line/metadata/multi/noopaque-duplicate assertions; GREEN existing prefixes+newrecords, static phrases no shim.
- `go test ./cmd/mcp-bridge -run 'Test(ShippedSampleFailsWithoutOneReference|ExampleConfigPassesCheck|RunCheckConfigAndMissingFile|ServeStartupFailures|.*Diagnostic)' -count=1`, main §5.1 smokes.
- AC-30,AC-36,AC-37; no real endpoints/secrets/old-body fallback.

### T-19 — Typed reload and exact logger manifest

- Deps T-15–T-18/observe filters/operatorLog/wrapped errors/§4.5 exact snake_case manifest.
- Components observe/log.go/tests/wiring/reload tests. Logf signature unchanged/errors.As/narrow fields/finite fallback; no arbitrary formatting/broadredaction exception/metriclabels/publichealth.
- **C-2 RED:** new observe TestConfigDiagnosticFieldManifest decodes emitted JSON; exact names/type/safe values for **all 18 manifest fields**, no dropped/redacted/mutated values or aliases; negative forbidden-field cases/standard redaction intact. Current extras allowlist drops them, a behavioral assertion failure. Separate real reload test preserves generation/routing and typed context; arbitrary args never echoed.
- GREEN unchanged manifest tests and composition regression:
  ```sh
  go test ./internal/observe -run '^TestConfigDiagnosticFieldManifest$' -count=1
  go test ./internal/observe ./internal/service ./cmd/mcp-bridge -count=1
  ```
  Exact key spellings pinned in test expectations, not only by reading production list; changes require deliberate contract revision, not silently updating expected names.
- AC-37,AC-38. Any omission/[REDACTED]/alias/value leak/state change/broad exception blocks.

### T-20 — Docs and final gate

- Deps T-15–T-19 GREEN/independent Go-code-security review/approved r5 after re-review/checkpoints/§5 smokes.
- README troubleshooting/testing, existing changelog only if discovered; no new convention/infra/secrets/approval edits.
- Docs safe repair/Compose/resolved prefix-body-ID policies/LF/incomplete; preserve expectations.
- Main commands planned, not results:
  ```sh
  go test ./...
  go test -race ./...
  go vet ./...
  go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...
  go run ./cmd/covcheck
  go build ./cmd/mcp-bridge
  ```
  Then §5.1. >=90%, no weaker threshold; all old secret/API/parser/limits/fingerprint tests/full suite.
- AC-39/all consolidation. Escalate fail/leak, no narrow matrix; main retains evidence/cleans artifacts under implementation scope.

Review immutable metadata/order/no avoidable copies/reads/clean helper cutover/unchanged admission. Security every sink/identifier/redactor/canaries/exact manifest. C-3 nonnil file guards preserved but not treated as message oracles. Actual secrets not prerequisite. U policies settled, no alias/shim pending decision; future changes require explicit revised contract/checkpoints.

## 7. RELEASE AND RECOVERY

No config/database/snapshot/volume migration/irreversible conversion. Release verified implementation/docs by established image/executable procedure only after critic/checkpoints; spec task deploys/pushes no image/edits no Compose. Private identity-matched real preflight/no values in evidence.

Rollback previous verified executable/image; unchanged YAML/secret/snapshot formats/predicates, no conversion/feature-required rotation. New field consumers tolerate old version during rollback. Prefix/status retained, old full bodies deliberately retired. On value leak stop rollout/incident/rotation policy; cannot retract logs.

## 8. CRITIC HANDOFF

### 8.1 Selected design

Typed safe records/fullindexed metadata/specific causes/LookupEnv/exact-oneLF/privateclassifier/all callers migrated/bounded staged collection. Prefix/status retained, opaque bodies replaced/no shim. Valid relevant local IDs normative, value-preview-length forbidden. Existing channels/no stack/>=90%. Static reasons deliberate explanations only. C-1/2/3 add specificity/test coverage, **no design change**.

### 8.2 User decisions — RESOLVED

Coordinator reports user accepted both defaults. Policy decisions only, not ledger execution approval.

| ID | Status | Confirmed decision |
|---|---|---|
| U-01 | RESOLVED/defaults accepted | Keep invalid configuration: (main.go:52), configuration: (serve.go:58), exit1. Replace opaque bodies/code-context. No fulloldbody shim/duplicate generic append. Useful static reasons only within new records. |
| U-02 | RESOLVED/defaults accepted | Escaped configured file paths/env:NAME/file:relative/path/pin tools MAY appear locally, relevant metadata required R-28/AC-35. NEVER values/previews/prefixes-suffixes/hash-fingerprint/exactlength-offset/expandedpath/URL components; identifier permission not value permission. |

No open user policy decisions. Production culprit unknown fact, not plan blocker. r5 critic re-review next, before execution checkpoints.

### 8.3 Finding dispositions and review focus

- **F-01 AMENDED r3/retained:** malformed-ref/header/sequence/private policy existing tests fully mapped, other secret regressions retained.
- **C-1 HIGH — AMENDED r5:** each parser_edges/validate_edge/api_edges case has source/current-substring/preserve-or-replace/fullmetadata row. Added actual-unset :51 retirement and direct readSetting/limits guards. No negative fixture dropped/weakened. T-16 exact real names + fullpackage, not incomplete regex.
- **C-2 MINOR — AMENDED r5:** exact existing snake_case names (config_path/ref_kind/ref_name, not aliases) and observe JSON manifest/all18 types-values/negative cases pinned in §4.5/AC-38/T-19. No field rename/design change.
- **C-3 MINOR — AMENDED r5:** config_test anchors explicitly nonnil-only/policy acceptance, not substring oracles. New policy reason+fullcontext tests/full suite regression basis.

BridgeCritic6 r4 APPROVE-WITH-AMENDMENTS recorded, **not** represented as unconditional approval. R/AC/T stable, no implementation begun. Independent critic re-review of r5 before any execution checkpoint; NOT EXECUTION-AUTHORIZED.

Review questions: every site/case incl direct guards and decoded wrongtype; barriers/cache/no partial/no false total; exact logger field survival without unrelated reflection; resolved identifier boundary/G5 harness distinction; LF/limits/preflight/fingerprints/snapshots unchanged; no opaque-only/duplicate-shim outputs. Verify retained reasons are intentional and baseline negative fixtures survive unchanged in meaning.

Verification: planner no implementation/tests/builds. Coordinator probes E/G; bounded MCP C-1 reads/targeted literal search, LSP helper calls; only this spec written. Policy approvals recorded as supplied, no broader permission invented.

Self-evaluation not approval: Accuracy4/5 anchored full inventory/probes/cases, future decoded guard behavior needs fixtures. Completeness4/5 requested paths/channels/allcritic amendments, production culprit intentionally unknown. Clarity4/5 explicit matrices/manifest but dense. Actionability4/5 exact named gates, no new behavior run. Conciseness4/5 required exhaustive evidence dominates. Overall4.0/5. Next independent re-review, then authorized RED/GREEN; spec not shipped fix.

### 8.4 Independent critic verdict — BridgeCritic6 on r4 (coordinator-applied)

The critic role had no filesystem-write tool, so the coordinator appended this section verbatim-equivalent on its behalf. The critic modified no file, test, gate or source; it reviewed r4 (503 lines) read-only, relied on the supplied coordinator probes E1–E6/G1–G6, and read `internal/config/{config.go,command.go,credentials.go}`, `internal/observe/log.go`, `cmd/mcp-bridge/{main.go,serve.go}` plus test-literal searches. The coordinator independently re-verified every C-1 citation against the live tree and confirms they are real.

| Revision | Critic | Verdict |
|---|---|---|
| HMB-CD-v1 r4 | BridgeCritic6 | **APPROVE-WITH-AMENDMENTS** — C-1 (HIGH, §5.2 incomplete existing-test inventory) must be amended before/at implementation; C-2/C-3 MINOR. Design, redaction policy and fail-closed aggregation independently confirmed; no unresolved blocking correctness/safety defect. Technical only — user checkpoints still govern execution. |

**C-1 — HIGH — §5.2 (mandatory existing-test changes) — incomplete inventory; existing substring oracles will fail and are not listed.** §5.2 as reviewed enumerated `example_config_test.go`, `serve_edge_test.go`, `bounds_edge_test.go`, `secret_validation_edge_test.go`, `secret_sequence_edge_test.go`, `private_file_test.go`, but omitted: `parser_edges_test.go:16,:21` asserting `"decode configuration"` (retired by the "never render wrapped decoder prose" decision at §3.1 :72/:88; the retained phrase "cannot decode YAML configuration" does not contain it); `parser_edges_test.go:19` asserting `"one YAML document"` (replaced by `yaml_documents`/`document_count`/`not_one` with retention unstated); `validate_edge_test.go:25,:26` asserting `"invalid pin"`/`"duplicate pin"` (mapped to pin codes with reason text unfixed); `api_edges_test.go:18,:19,:42` asserting `"does not resolve"`/`"exactly one secret"` against `credentials.go:104,108,118,122` via legitimate direct `Config` guards. Consequence: implementers reach GREEN with unlisted failing oracles and ad-hoc assertion edits would silently degrade the AC-33 "not substring-only" contract. Also noted: T-16's `-run 'Test.*(Credential|Secret|Diagnostic|Parse)'` does not match the `validate_edge_test.go`/`api_edges_test.go` test names, so the first named command would not surface them. **Disposition: AMENDED r5** — §5.2 rows added, T-16 names the real tests; the planner's own scoped search additionally found `validate_edge_test.go:51` (`"secret must be nonempty"` for an actually-unset env var), now explicitly retired to `secret_unset`.

**C-2 — MINOR — §4.5 item 4 / T-19 — logger allowlist dependency is real and only implicitly covered.** `internal/observe/log.go` `Log:59–75` emits only `count|slots|catalog_tools|mode` from `Entry.Fields`, so every proposed diagnostic field is currently silently dropped; `sensitive()` (`log.go:118–130`) would redact any field named `config_key`/`secret_ref`. The design's chosen names match no redactor marker (correct), but T-19 did not require pinning the names in an observe test, so a later rename could silently redact or drop diagnostics. **Disposition: AMENDED r5** — §4.5/AC-38/T-19 pin the exact snake_case names, types/values and negative alias/redaction/drop cases; no rename.

**C-3 — MINOR — §3.2 evidence claim not reproducible as stated.** `config_test.go:201–202,222–223,245–246` are `if err == nil { t.Fatal("…accepted") }` rejection assertions; they assert non-nil only and contain none of `non-symlink` / `secret file must be` / the value literal. **Disposition: AMENDED r5** — the anchors are labelled non-nil-only policy guards, not substring oracles; the retained `secret_file_policy` reason plus full-suite regression is the stated regression basis.

**NON-BLOCKING notes.** §4.1 single-record format, zero-based index convention and `<memory>` origin are consistent with §4.4; §7 rollback (prefixes/status retained, old bodies deliberately not retained) is coherent with resolved U-01; AC-36's `>32` cap and `remaining_stages=not_checked` footer are self-consistent.

**What the spec gets right (critic-confirmed, not assumed).** (1) The live inventory is real: every in-scope config-rejection constructor exists at the cited line with the cited text, including `errors.New` at `credentials.go:104,108`. (2) The logger claim is verified and the renaming is genuinely necessary: `log.go:59–75` drops unknown fields and `sensitive()` markers (`key`, `secret`, `header`, `url`, `endpoint`) would redact the natural names — so narrow renamed fields are the only route without a blanket exemption; `reason` already allowlists `invalid_config`, matching §4.5 item 3. (3) Fail-closed is preserved: every `Parse` failure returns `nil, err` (`config.go:67–168`) and `serve.go:58` returns before store creation (`:65`) and both listeners (`:164,:169`), so no partial `Config` and no runtime resource can start on a failed load. (4) G5 handling is correct — non-UTF-8 is labelled harness-before-exec, not product behaviour, with Go raw-byte fixtures required.

**R1 inventory completeness:** no missed site found. Every `fmt.Errorf`/`errors.New` in `internal/config` (`command.go`, `config.go`, `credentials.go`) appears in §3.1 A–E; the nearest unlisted candidates (`serve.go:65,75,83,87,161,166,171`, `wiring.go:146,192,207,226,231,275`) are runtime/non-config constructs correctly out of scope. **R2 logger survival:** they survive only because the spec mandates extending the allowlist, and it does — no blanket exemption is proposed or needed; the redactor scans keys, not values. **R3 partial admission:** cannot occur — stage barriers keep Parse's nil-on-failure contract; the "collect every valid occurrence within a stage" rule and the "do not advance a failed stage" barrier are the same stage's collect-vs-advance distinction. **R4 redaction sufficiency:** sufficient for the resolved U-02 boundary; the only leak that would otherwise follow from emitting file refs (the `base`-joined expanded path) is explicitly forbidden, and the irreducible residue (an operator-chosen identifier that is itself confidential) is the acknowledged, user-confirmed boundary — settled policy, not a blocker. **R5 §5.2 per-literal decisions:** the rows present are sound; the gap was C-1, now amended.

**Critic limits (stated by the critic).** No build/test execution in that role; did not fully read `config.go:300–330/460–500`; did not exhaustively enumerate runtime-only `wiring.go` constructors; accepted E1–E6/G1–G6 as coordinator evidence without re-derivation. No GREEN evidence exists yet and none is claimed. **No tasks or approvals are invalidated; T-15–T-20 remain PLANNED and execution-unauthorized pending the r5 re-review and user execution checkpoints.**

### 8.5 Independent critic delta re-review — BridgeCritic7 on r5 (coordinator-applied)

Delta re-review of r5 against C-1/C-2/C-3 only, per the repo's delta convention. The critic role has no write tool; the coordinator appended this section on its behalf. The critic re-read the three test files, `internal/observe/log.go` and the cited spec sections; no file, test, gate or source was modified.

| Revision | Critic | Verdict |
|---|---|---|
| HMB-CD-v1 r5 | BridgeCritic7 | **CONFIRMED** — r5 amendment resolves C-1 (HIGH) and C-2/C-3 (MINOR) against the live tree; R/AC/T stable, no design change, no new defect. Technical only — user execution checkpoints still govern. |

**C-1 CLOSED.** §5.2 enumerates every literal the three files assert, at the live lines: `parser_edges_test.go` :15 `"1 MiB"`, :16/:21 `"decode configuration"` (both replace), :17/:18 `"mapping"`, :19 `"one YAML document"` (retain), :20 `"unknown key"`, :22 `"entries must be strings"`, direct guards :42–:50 mapped 1:1; `validate_edge_test.go` :25 `"invalid pin"`, :26 `"duplicate pin"`, :39–:43 missing-file non-nil-only, :45–:52 with :51 `"secret must be nonempty"` for an **actually-unset** var retired to `secret_unset`, :55–:68 one-LF success, :78–:80 limits; `api_edges_test.go` :15–:19, :29, :41/:42 confirmed, :53–:68 fingerprint noted as regression. T-16's `-run` filter names all ten functions that exist (`TestParseRejectsMalformedDocuments`:9, `…MissingAndWrongTypedValues`:34, `TestParseValidationEdges`:17, `TestParseFileAndCredentialResolutionEdges`:38, `TestServerLimitsRejectInvalidValues`:72, `TestCredentialAccessorsRejectUnknownOrMalformedReferences`:8, `TestEmbeddingCredentialResolutionFailures`:34, `TestParseRejectsMalformedSecretReferencesAndHeaders`:8, `TestParseRejectsNonMappingBearerTokenElements`:31, `TestParseAcceptsEnvBearerTokenReferences`:61), so the gate cannot silently match zero tests; the full-package run covers the two declared-unchanged helpers.

**C-2 CLOSED.** §4.5 item 4's 17 names plus the existing `server` = 18, matching T-19. `Log` allowlists only `count|slots|catalog_tools|mode`; `sensitive()` normalizes away `-`/`_` and redacts markers `token,key,secret,password,authorization,cookie,session,argument,result,query,schema,description,payload,url,endpoint,header,stack`. None of the 17 manifest names normalizes to contain a marker, so once allowlisted none is dropped or redacted; the rejected `config_key`/`secret_ref` would hit `key`/`secret` and are correctly excluded. T-19/AC-38 mandate `TestConfigDiagnosticFieldManifest` decoding real JSON with exact key set/spelling/types/values plus alias and forbidden-field negatives. The names are real allowlist extensions, not invented collisions.

**C-3 CLOSED.** `config_test.go:201–202` (world-readable), `:222–223` (symlink), `:245–246` (foreign-owned) are `if _, err := config.ParseFile(path); err == nil { t.Fatal("… accepted") }` — non-nil-only, no substring. r5 §3.2/§5.2/§8.3 correctly label them non-nil policy guards and base regression on the `secret_file_policy` full-context tests plus the full suite.

**New findings: none.** The only nuances (T-16's filter omitting `TestIntegerAndNumberValueBoundaries` and `TestCredentialFingerprintFailureAndSuccess`) are explicitly declared unchanged and covered by the full-package command — not a defect.

**Residual risk for the implementer.** The decoding JSON manifest test and the exact-name gate are the sole mechanical guards against a silent field rename or test drop; they must be written to fail behaviourally (assertion failure from the current allowlist drop), never compile-only, and must not be weakened to reach GREEN.

**Critic limits.** No build/test execution in that role; reused coordinator-verified anchors and BridgeCritic6's C-3 verification; `[INFERENCE]` that future allowlist extensions behave as the switch requires, since no code was executed.

## 9. EXECUTION CHECKPOINT — USER, 2026-10-16

**Authorizing instruction (main session, verbatim):** "implement with engineering agent".

Recorded at revision **r5** (BridgeCritic7 CONFIRMED), separately for each required view:

1. **Requirements** — R-28–R-34 as written in §2.
2. **Design** — §4.1 (typed metadata-only records + single renderer), §4.2 (secret value/file cause classes, unchanged one-terminal-LF rule), §4.3 (identifier visibility and redaction list), §4.4 (stage barriers, bounded aggregation ≤32, coverage footers), §4.5 (channels and the exact 18-field logger schema).
3. **Task ledger** — T-15–T-20 in ledger order, executed by the `engineer` role under TDD RED→GREEN per task.

**Policy inputs:** U-01 and U-02 resolved as the accepted defaults (§8.2) and normative.

**Gate marker:** `.superpowers/sdd/GATE_OPEN` line 3 authorizes HMB-CD-v1 r5 code work only. It is not shippable alone and MUST NOT be committed or pushed before the implementation meets every acceptance criterion (AC-30–AC-39), the required tests, and the project-wide ≥90% source-line coverage gate.

**Constraints retained.** No acceptance weakening or negative-fixture deletion; no scope extension beyond the ledger; no Docker image build and no registry write (pipeline-owned); direct-to-main only, no task branches; no SSH; commit/push only after independent verification passes, and then the implementation and these planning artifacts ship together.
