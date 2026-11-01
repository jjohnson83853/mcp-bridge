package config

import (
	"errors"
	"strings"
	"testing"
)

const metadataDefaultsBase = `schema_version: 1
servers:
  - id: alpha
    endpoint: https://alpha.invalid/mcp
client_auth:
  bearer_tokens:
    - {env: TEST_CLIENT_SECRET}
pins:
  - {server: alpha, tool: read}
`

func metadataDefaultsConfig(defaults string) string {
	return strings.Replace(metadataDefaultsBase, "    endpoint: https://alpha.invalid/mcp\n",
		"    endpoint: https://alpha.invalid/mcp\n"+defaults, 1)
}

func requireClientSecret(t *testing.T) {
	t.Helper()
	t.Setenv("TEST_CLIENT_SECRET", "private-client-value")
}

// TestToolMetadataDefaultsParse verifies the optional server-owned
// tool_metadata.defaults patch parses with full scalar/pointer semantics:
// absent members stay absent, explicit false and empty members are retained,
// nested _meta objects admit null as a value, and a server without pins may
// still declare defaults.
func TestToolMetadataDefaultsParse(t *testing.T) {
	requireClientSecret(t)
	cfg, err := Parse([]byte(metadataDefaultsConfig(`    tool_metadata:
      defaults:
        title: Corrected read
        description: Advisory read description
        annotations:
          readOnlyHint: true
          destructiveHint: false
          idempotentHint: true
        icons:
          - {source: "https://example.test/read.svg", mime_type: "image/svg+xml"}
        _meta:
          policy: strict
          audience: null
`)), ".")
	if err != nil {
		t.Fatalf("Parse rejected tool_metadata defaults: %v", err)
	}
	if cfg.Servers[0].ToolMetadata == nil {
		t.Fatal("Parse dropped the tool_metadata defaults")
	}
	patch := cfg.Servers[0].ToolMetadata.Defaults
	if len(patch) == 0 {
		t.Fatal("Parse dropped the configured defaults patch")
	}
	if raw := string(patch["title"]); raw != `"Corrected read"` {
		t.Fatalf("defaults title = %s; want quoted JSON string", raw)
	}
	if raw := string(patch["annotations"]); !strings.Contains(raw, `"readOnlyHint":true`) || !strings.Contains(raw, `"destructiveHint":false`) {
		t.Fatalf("defaults annotations lost scalar/pointer semantics: %s", raw)
	}
	if raw := string(patch["_meta"]); !strings.Contains(raw, `"audience":null`) {
		t.Fatalf("defaults _meta lost null value: %s", raw)
	}
	if raw := string(patch["icons"]); !strings.Contains(raw, `"source":"https://example.test/read.svg"`) {
		t.Fatalf("defaults icons lost members: %s", raw)
	}

	// A server with no pins may retain defaults without affecting unpinned
	// tools, and empty/false members are retained.
	pinned := false
	cfg, err = Parse([]byte(strings.Replace(metadataDefaultsConfig(`    tool_metadata:
      defaults:
        annotations: {}
`), "pins:\n  - {server: alpha, tool: read}\n", "pins: []", 1)), ".")
	if err != nil {
		t.Fatalf("Parse rejected pinless server defaults: %v", err)
	}
	for _, pin := range cfg.Pins {
		if pin["server"] == "alpha" {
			pinned = true
		}
	}
	if pinned {
		t.Fatal("alpha unexpectedly retained its pin")
	}
	if patch := cfg.Servers[0].ToolMetadata.Defaults; patch == nil || !strings.Contains(string(patch["annotations"]), "{}") {
		t.Fatalf("empty defaults members not retained: %v", patch)
	}
	// An explicit false annotation value is retained, never dropped.
	cfg, err = Parse([]byte(metadataDefaultsConfig(`    tool_metadata:
      defaults:
        annotations:
          openWorldHint: false
`)), ".")
	if err != nil {
		t.Fatalf("Parse rejected explicit false hint: %v", err)
	}
	if patch := cfg.Servers[0].ToolMetadata.Defaults; patch == nil || !strings.Contains(string(patch["annotations"]), `"openWorldHint":false`) {
		t.Fatalf("explicit false annotation not retained: %v", patch)
	}
}

// TestToolMetadataDefaultsValidation verifies invalid defaults reject the
// whole candidate atomically through the existing typed, metadata-only
// diagnostics contract: no raw YAML, no value echo.
func TestToolMetadataDefaultsValidation(t *testing.T) {
	requireClientSecret(t)
	cases := []struct{ name, defaults, wantCode string }{
		{"unknown member", `    tool_metadata:
      defaults:
        not_a_metadata_field: true
`, "tool_metadata_unknown_member"},
		{"title type", `    tool_metadata:
      defaults:
        title: [not, a, scalar]
`, "tool_metadata_value"},
		{"annotations array", `    tool_metadata:
      defaults:
        annotations: [true]
`, "tool_metadata_value"},
		{"annotations non-scalar member", `    tool_metadata:
      defaults:
        annotations:
          readOnlyHint: {nested: object}
`, "tool_metadata_value"},
		{"icons mapping", `    tool_metadata:
      defaults:
        icons: {source: "https://example.test/x.svg"}
`, "tool_metadata_value"},
		{"meta scalar", `    tool_metadata:
      defaults:
        _meta: scalar
`, "tool_metadata_value"},
		{"unknown annotation key", `    tool_metadata:
      defaults:
        annotations:
          audience: user
`, "tool_metadata_value"},
		{"annotation hint not boolean", `    tool_metadata:
      defaults:
        annotations:
          readOnlyHint: "yes"
`, "tool_metadata_value"},
		{"annotation title not string", `    tool_metadata:
      defaults:
        annotations:
          title: true
`, "tool_metadata_value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(metadataDefaultsConfig(tc.defaults)), ".")
			if err == nil {
				t.Fatal("Parse accepted invalid defaults; want rejection")
			}
			var diags *Diagnostics
			if !errors.As(err, &diags) || len(diags.Records) == 0 {
				t.Fatalf("rejection is not a typed Diagnostics collection: %v", err)
			}
			if diags.Records[0].Code != tc.wantCode {
				t.Fatalf("diagnostic code = %q; want %q (%s)", diags.Records[0].Code, tc.wantCode, err)
			}
			if diags.Records[0].Reason == "" {
				t.Fatal("diagnostic carries no static reason")
			}
		})
	}
	// A duplicate tool_metadata mapping key is the existing recursive
	// duplicate policy, not a new vocabulary.
	_, err := Parse([]byte(strings.Replace(metadataDefaultsBase, "    endpoint: https://alpha.invalid/mcp\n",
		"    endpoint: https://alpha.invalid/mcp\n    tool_metadata:\n      defaults:\n        title: One\n    tool_metadata:\n      defaults:\n        title: Two\n", 1)), ".")
	if err == nil {
		t.Fatal("Parse accepted duplicate tool_metadata keys; want rejection")
	}
	var diags *Diagnostics
	if !errors.As(err, &diags) {
		t.Fatalf("duplicate rejection is not typed: %v", err)
	}
	for _, record := range diags.Records {
		if record.Code == "tool_metadata_unknown_member" {
			t.Fatalf("duplicate key surfaced as an unknown member instead of the duplicate policy: %v", diags)
		}
	}
}

// TestToolMetadataAnnotationsShape verifies the config layer enforces the
// build layer's annotation contract at parse time: only the SDK annotation
// inventory (title plus the four hints) is admitted, the title is a string
// and the hints are booleans, and a valid title-plus-hint patch is accepted.
func TestToolMetadataAnnotationsShape(t *testing.T) {
	requireClientSecret(t)
	cfg, err := Parse([]byte(metadataDefaultsConfig(`    tool_metadata:
      defaults:
        annotations:
          title: Read
          readOnlyHint: true
`)), ".")
	if err != nil {
		t.Fatalf("Parse rejected a valid annotation patch: %v", err)
	}
	if cfg.Servers[0].ToolMetadata == nil || len(cfg.Servers[0].ToolMetadata.Defaults) == 0 {
		t.Fatal("Parse dropped the defaults patch")
	}
	raw := string(cfg.Servers[0].ToolMetadata.Defaults["annotations"])
	if !strings.Contains(raw, `"title":"Read"`) || !strings.Contains(raw, `"readOnlyHint":true`) {
		t.Fatalf("valid annotation patch lost members: %s", raw)
	}
}

// TestValidateMetadataMemberAnnotations verifies the programmatic shape gate
// mirrors the parse-time annotation contract: unknown keys, non-string
// titles and non-boolean hints are rejected.
func TestValidateMetadataMemberAnnotations(t *testing.T) {
	cases := []struct {
		name   string
		member map[string]any
	}{
		{"unknown key", map[string]any{"audience": "user"}},
		{"non-string title", map[string]any{"title": true}},
		{"non-boolean hint", map[string]any{"readOnlyHint": "yes"}},
	}
	for _, tc := range cases {
		if err := validateMetadataMember("annotations", tc.member); err == nil {
			t.Errorf("%s: programmatic gate accepted an unsupported annotation member", tc.name)
		}
	}
	if err := validateMetadataMember("annotations", map[string]any{"title": "Read", "readOnlyHint": true}); err != nil {
		t.Fatalf("programmatic gate rejected a valid annotation mapping: %v", err)
	}
}

// TestToolMetadataDefaultsPublicFingerprint verifies the defaults join the
// public identity projection (config_public_fingerprint changes when only the
// defaults change) while credential identity is untouched.
func TestToolMetadataDefaultsPublicFingerprint(t *testing.T) {
	requireClientSecret(t)
	withDefaults, err := Parse([]byte(metadataDefaultsConfig(`    tool_metadata:
      defaults:
        title: Corrected read
`)), ".")
	if err != nil {
		t.Fatalf("Parse rejected defaults: %v", err)
	}
	withOtherDefaults, err := Parse([]byte(metadataDefaultsConfig(`    tool_metadata:
      defaults:
        title: Other read
`)), ".")
	if err != nil {
		t.Fatalf("Parse rejected changed defaults: %v", err)
	}
	withoutDefaults, err := Parse([]byte(metadataDefaultsBase), ".")
	if err != nil {
		t.Fatalf("Parse rejected baseline: %v", err)
	}
	if withDefaults.Fingerprint() == withoutDefaults.Fingerprint() {
		t.Fatal("adding tool_metadata defaults did not change the public fingerprint")
	}
	if withDefaults.Fingerprint() == withOtherDefaults.Fingerprint() {
		t.Fatal("changing a defaults member did not change the public fingerprint")
	}
	before, ok := withDefaults.ServerCredentialFingerprint("alpha")
	if !ok {
		t.Fatal("ServerCredentialFingerprint(alpha) unresolved")
	}
	if changed, _ := withOtherDefaults.ServerCredentialFingerprint("alpha"); changed != before {
		t.Fatal("credential identity changed with a metadata-only config edit")
	}
}

// exceptionsBase pins both alpha tools the exception scenarios exercise.
const exceptionsBase = `schema_version: 1
servers:
  - id: alpha
    endpoint: https://alpha.invalid/mcp
client_auth:
  bearer_tokens:
    - {env: TEST_CLIENT_SECRET}
pins:
  - {server: alpha, tool: read}
  - {server: alpha, tool: write}
`

func exceptionsConfig(metadata string) string {
	return strings.Replace(exceptionsBase, "    endpoint: https://alpha.invalid/mcp\n",
		"    endpoint: https://alpha.invalid/mcp\n"+metadata, 1)
}

// exceptionsDefaults is the broad alpha default the exact write exception
// overrides in every scenario below.
const exceptionsDefaults = `    tool_metadata:
      defaults:
        annotations:
          readOnlyHint: true
          destructiveHint: false
          idempotentHint: true
`

// exceptionsWriteException is the exact raw-tool-name exception for write.
const exceptionsWriteException = `      tools:
        write:
          annotations:
            readOnlyHint: false
            destructiveHint: true
            idempotentHint: false
`

// TestToolMetadataExceptionsParse verifies the exact raw-tool-name mapping
// parses as a per-tool patch layered above defaults: a valid tool-only patch
// (no defaults), nested merge members and explicit false values all parse,
// and an empty tools mapping is accepted for a server that declares none.
func TestToolMetadataExceptionsParse(t *testing.T) {
	requireClientSecret(t)
	cfg, err := Parse([]byte(exceptionsConfig(exceptionsDefaults+exceptionsWriteException)), ".")
	if err != nil {
		t.Fatalf("Parse rejected tool_metadata tools exceptions: %v (RED)", err)
	}
	// The configured exception parses onto the owning server's exact-tools
	// mapping, retrievable per raw tool name.
	if cfg.Servers[0].ToolMetadata == nil {
		t.Fatal("Parse dropped the tool_metadata exceptions")
	}
	patch := cfg.Servers[0].ToolMetadata.Tools["write"]
	if patch == nil {
		t.Fatal("Parse dropped the configured write exception")
	}
	if raw := string(patch["annotations"]); !strings.Contains(raw, `"readOnlyHint":false`) {
		t.Fatalf("exception annotations = %s; want the explicit false values", raw)
	}
	if _, ok := cfg.Servers[0].ToolMetadata.Tools["read"]; ok {
		t.Fatal("Parse retained an exception for a tool without one")
	}
	if len(cfg.Servers[0].ToolMetadata.Tools) != 1 {
		t.Fatalf("tools mapping = %d entries; want the single configured write exception", len(cfg.Servers[0].ToolMetadata.Tools))
	}
	// A tool-only patch without any defaults block is valid.
	toolOnly := strings.Replace(exceptionsConfig(`    tool_metadata:
      tools:
        write:
          title: Corrected write
`), exceptionsDefaults, "", 1)
	if _, err := Parse([]byte(toolOnly), "."); err != nil {
		t.Fatalf("Parse rejected a tool-only patch without defaults: %v (RED)", err)
	}
	// An empty tools mapping stays configured and empty.
	empty, err := Parse([]byte(exceptionsConfig(`    tool_metadata:
      tools: {}
`)), ".")
	if err != nil {
		t.Fatalf("Parse rejected an empty tools mapping: %v (RED)", err)
	}
	if empty.Servers[0].ToolMetadata == nil || len(empty.Servers[0].ToolMetadata.Tools) != 0 {
		t.Fatal("Parse dropped the empty tools mapping")
	}
}

// TestToolMetadataExceptionsValidation verifies invalid tool-name shapes and
// patch values reject the whole config with typed metadata-only diagnostics.
// Exact selector membership is checked later against the accepted candidate
// catalog, because config parsing has no live tool inventory.
func TestToolMetadataExceptionsValidation(t *testing.T) {
	requireClientSecret(t)
	cases := []struct{ name, metadata, wantCode string }{
		{"invalid tool name", exceptionsDefaults + `      tools:
        "bad name!":
          title: Nope
`, "tool_metadata_tool_name"},
		{"unsupported patch member", exceptionsDefaults + `      tools:
        write:
          inputSchema: {}
`, "tool_metadata_unknown_member"},
		{"annotation non-scalar", exceptionsDefaults + `      tools:
        write:
          annotations:
            readOnlyHint: [true]
`, "tool_metadata_value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(exceptionsConfig(tc.metadata)), ".")
			if err == nil {
				t.Fatalf("Parse accepted invalid tools entry (%s); want rejection (RED)", tc.name)
			}
			var diags *Diagnostics
			if !errors.As(err, &diags) || len(diags.Records) == 0 {
				t.Fatalf("rejection is not a typed Diagnostics collection: %v (RED)", err)
			}
			if diags.Records[0].Code != tc.wantCode {
				t.Fatalf("diagnostic code = %q; want %q (%s) (RED)", diags.Records[0].Code, tc.wantCode, err)
			}
			if diags.Records[0].Reason == "" {
				t.Fatal("diagnostic carries no static reason (RED)")
			}
		})
	}
	// A duplicate entry inside one tools mapping follows the existing
	// duplicate-key policy, not a new vocabulary.
	_, err := Parse([]byte(exceptionsConfig(exceptionsDefaults+`      tools:
        write:
          title: One
        write:
          title: Two
`)), ".")
	if err == nil {
		t.Fatal("Parse accepted duplicate tools entries; want rejection (RED)")
	}
	var diags *Diagnostics
	if !errors.As(err, &diags) {
		t.Fatalf("duplicate rejection is not typed: %v (RED)", err)
	}
	for _, record := range diags.Records {
		if record.Code == "tool_metadata_unknown_member" {
			t.Fatalf("duplicate tools entry surfaced as an unknown member: %v (RED)", diags)
		}
	}
}

// TestToolMetadataExceptionsPublicFingerprint verifies the exact-tools
// mapping joins the public identity projection: adding or changing an
// exception changes config_public_fingerprint while credential identity is
// untouched.
func TestToolMetadataExceptionsPublicFingerprint(t *testing.T) {
	requireClientSecret(t)
	withException, err := Parse([]byte(exceptionsConfig(exceptionsDefaults+exceptionsWriteException)), ".")
	if err != nil {
		t.Fatalf("Parse rejected the exception config: %v (RED)", err)
	}
	withOtherException, err := Parse([]byte(exceptionsConfig(exceptionsDefaults+`      tools:
        write:
          annotations:
            readOnlyHint: false
            destructiveHint: true
            idempotentHint: true
`)), ".")
	if err != nil {
		t.Fatalf("Parse rejected the changed exception config: %v (RED)", err)
	}
	withoutException, err := Parse([]byte(exceptionsConfig(exceptionsDefaults)), ".")
	if err != nil {
		t.Fatalf("Parse rejected the defaults-only config: %v (RED)", err)
	}
	if withException.Fingerprint() == withoutException.Fingerprint() {
		t.Fatal("adding a tools exception did not change the public fingerprint (RED)")
	}
	if withException.Fingerprint() == withOtherException.Fingerprint() {
		t.Fatal("changing a tools exception did not change the public fingerprint (RED)")
	}
	before, ok := withException.ServerCredentialFingerprint("alpha")
	if !ok {
		t.Fatal("ServerCredentialFingerprint(alpha) unresolved")
	}
	if changed, _ := withOtherException.ServerCredentialFingerprint("alpha"); changed != before {
		t.Fatal("credential identity changed with a metadata-only config edit (RED)")
	}
}
// TestProgressiveMetadataEligibility accepts an exact same-server selector
// even when the config does not pin the target; the accepted candidate
// catalog validates that selector later.
func TestProgressiveMetadataEligibility(t *testing.T) {
	requireClientSecret(t)
	document := metadataDefaultsConfig(`    tool_metadata:
      tools:
        read:
          title: Corrected read
`)
	document = strings.Replace(document, "pins:\n  - {server: alpha, tool: read}\n", "pins: []\n", 1)

	cfg, err := Parse([]byte(document), ".")
	if err != nil {
		t.Fatalf("Parse rejected an exact unpinned metadata selector: %v", err)
	}
	if len(cfg.Pins) != 0 {
		t.Fatalf("metadata selector revealed or pinned a tool: %v", cfg.Pins)
	}
	if patch := cfg.Servers[0].ToolMetadata.Tools["read"]; string(patch["title"]) != `"Corrected read"` {
		t.Fatalf("exact unpinned selector was not retained: %v", patch)
	}
}
