package config

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// requireDiagnostics pins that a rejection is the typed collection, not an
// opaque error (spec §4.1/§5.2 C-1).
func requireDiagnostics(t *testing.T, err error) *Diagnostics {
	t.Helper()
	if err == nil {
		t.Fatal("expected a rejection, got nil")
	}
	var diags *Diagnostics
	if !errors.As(err, &diags) {
		t.Fatalf("expected errors.As(*Diagnostics), got %T: %v", err, err)
	}
	if len(diags.Records) == 0 {
		t.Fatalf("expected at least one record, got %+v", diags)
	}
	return diags
}

// requireRendered asserts every fragment appears in the single rendered line.
func requireRendered(t *testing.T, err error, fragments ...string) {
	t.Helper()
	line := err.Error()
	if strings.Contains(line, "\n") {
		t.Fatalf("expected one physical line, got %q", line)
	}
	for _, fragment := range fragments {
		if !strings.Contains(line, fragment) {
			t.Fatalf("rendered line lacks %q: %s", fragment, line)
		}
	}
}

// forbidRendered asserts none of the fragments appear (no raw echo fallbacks).
func forbidRendered(t *testing.T, err error, fragments ...string) {
	t.Helper()
	line := err.Error()
	for _, fragment := range fragments {
		if strings.Contains(line, fragment) {
			t.Fatalf("rendered line must not contain %q: %s", fragment, line)
		}
	}
}

// TestParseRejectsMalformedDocuments migrates the C-1 document table to
// full-metadata oracles: each rejection carries the supplied path, the known
// location, a finite code/property/shape class and a static reason, with the
// stage barrier naming the skipped later stages (spec §5.2).
func TestParseRejectsMalformedDocuments(t *testing.T) {
	cases := []struct {
		name      string
		data      []byte
		fragments []string
		forbidden []string
		wantLine  bool
	}{
		{
			name: "oversized", data: []byte(strings.Repeat("x", maxConfigBytes+1)),
			fragments: []string{`config="config.yaml"`, `key="<document>"`, "code=config_too_large", "property=document", "shape=over_limit", "1 MiB"},
			forbidden: []string{"1073741825"},
		},
		{
			name: "malformed yaml", data: []byte("schema_version: [\n"),
			fragments: []string{`config="config.yaml"`, `key="<document>"`, "code=yaml_syntax", "property=document", "shape=invalid_yaml", "cannot decode YAML configuration"},
			forbidden: []string{"did not find", "yaml.v3"}, wantLine: true,
		},
		{
			name: "scalar root", data: []byte("hello\n"),
			fragments: []string{`config="config.yaml"`, `key="<document>"`, "code=yaml_root_type", "property=root", "shape=scalar-string", "must be a mapping"},
			forbidden: []string{"hello"},
		},
		{
			name: "sequence root", data: []byte("- hello\n"),
			fragments: []string{`config="config.yaml"`, `key="<document>"`, "code=yaml_root_type", "property=root", "shape=sequence"},
		},
		{
			name: "second document", data: []byte(boundsEdgeBase + "---\nschema_version: 1\n"),
			fragments: []string{`config="config.yaml"`, `key="<document>"`, "code=yaml_documents", "property=document_count", "shape=not_one", "configuration must contain one YAML document"},
		},
		{
			name: "nested unknown key", data: []byte(strings.Replace(boundsEdgeBase, "pins: []", "search:\n  not_a_setting: true\npins: []", 1)),
			fragments: []string{`config="config.yaml"`, `key="search"`, "code=yaml_key_unknown", "property=unknown_key", "shape=unknown_key", "unknown key"},
			forbidden: []string{"not_a_setting"}, wantLine: true,
		},
		{
			name: "wrong section type", data: []byte(strings.Replace(boundsEdgeBase, "pins: []", "http: []\npins: []", 1)),
			fragments: []string{`config="config.yaml"`, `key="http"`, "code=yaml_type", "property=type", "shape=sequence", "must be a mapping"},
			forbidden: []string{"decode configuration"},
		},
		{
			name: "non-string allowlist element", data: []byte(strings.Replace(boundsEdgeBase, "pins: []", "http:\n  allowed_hosts: [42]\npins: []", 1)),
			fragments: []string{`config="config.yaml"`, `key="http.allowed_hosts[0]"`, "code=yaml_type", "property=type", "shape=integer", "entries must be strings"},
			forbidden: []string{"42"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.data, "config.yaml")
			diags := requireDiagnostics(t, err)
			requireRendered(t, err, tc.fragments...)
			forbidRendered(t, err, tc.forbidden...)
			if diags.RemainingStages != stageNotChecked {
				t.Fatalf("expected the stage barrier to name skipped stages, got %q", diags.RemainingStages)
			}
			if tc.wantLine && diags.Records[0].Line <= 0 {
				t.Fatalf("expected a safe parser position, got line=%d column=%d", diags.Records[0].Line, diags.Records[0].Column)
			}
		})
	}
}

// TestReadSettingReportsMissingAndWrongTypedValues migrates the C-1 direct
// setting guards: every record reports config=<memory> for a directly
// constructed Config, the exact full key, and a finite code/property/shape
// class with a static reason (spec §5.2).
func TestReadSettingReportsMissingAndWrongTypedValues(t *testing.T) {
	cases := []struct {
		name                          string
		cfg                           Config
		section, key                  string
		target                        any
		code, property, shape, reason string
		forbidden                     []string
	}{
		{"missing nested parent", Config{}, "upstream_defaults", "breaker.consecutive_failures", new(int),
			"setting_missing", "value", "missing", "configuration is missing", nil},
		{"missing nested leaf", Config{UpstreamDefaults: map[string]any{"breaker": map[string]any{}}}, "upstream_defaults", "breaker.consecutive_failures", new(int),
			"setting_missing", "value", "missing", "configuration is missing", nil},
		{"wrong string", Config{HTTP: map[string]any{"listen": 5}}, "http", "listen", new(string),
			"setting_type", "type", "integer", "must be a string", []string{"5"}},
		{"wrong int", Config{HTTP: map[string]any{"burst": 1.5}}, "http", "burst", new(int),
			"setting_type", "type", "number", "must be an integer", []string{"1.5"}},
		{"wrong int64", Config{HTTP: map[string]any{"max_request_bytes": "large"}}, "http", "max_request_bytes", new(int64),
			"setting_type", "type", "string", "must be an integer", []string{"large"}},
		{"wrong float", Config{HTTP: map[string]any{"requests_per_second": "fast"}}, "http", "requests_per_second", new(float64),
			"setting_type", "type", "string", "must be a number", []string{"fast"}},
		{"wrong duration type", Config{Search: map[string]any{"timeout": 1}}, "search", "timeout", new(time.Duration),
			"setting_duration", "type", "wrong_type", "must be a duration string", nil},
		{"invalid duration", Config{Search: map[string]any{"timeout": "tomorrow"}}, "search", "timeout", new(time.Duration),
			"setting_duration", "type", "invalid_duration", "must be a duration string", []string{"tomorrow"}},
		{"unsupported destination", Config{HTTP: map[string]any{"listen": "127.0.0.1"}}, "http", "listen", new(bool),
			"setting_internal", "destination", "unsupported_destination", "configuration reader has no destination", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := readSetting(tc.cfg, tc.section, tc.key, tc.target)
			requireDiagnostics(t, err)
			requireRendered(t, err,
				`config="<memory>"`,
				`key="`+tc.section+"."+tc.key+`"`,
				"code="+tc.code, "property="+tc.property, "shape="+tc.shape, tc.reason,
			)
			forbidRendered(t, err, tc.forbidden...)
		})
	}
}

func TestIntegerAndNumberValueBoundaries(t *testing.T) {
	for _, tc := range []struct {
		value  any
		wantOK bool
	}{{1, true}, {int64(2), true}, {uint64(3), true}, {float64(4), true}, {float64(1.5), false}, {"1", false}} {
		_, ok := integerValue(tc.value)
		if ok != tc.wantOK {
			t.Fatalf("integerValue(%#v) ok=%t, want %t", tc.value, ok, tc.wantOK)
		}
	}
	for _, tc := range []struct {
		value  any
		wantOK bool
	}{{1, true}, {int64(2), true}, {float64(3), true}, {"3", false}} {
		_, ok := numberValue(tc.value)
		if ok != tc.wantOK {
			t.Fatalf("numberValue(%#v) ok=%t, want %t", tc.value, ok, tc.wantOK)
		}
	}
}
