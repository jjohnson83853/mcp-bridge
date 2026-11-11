package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const boundsEdgeBase = `schema_version: 1
servers:
  - id: alpha
    endpoint: https://alpha.invalid/mcp
pins: []
`

// TestParseRejectsOutOfRangeFields migrates the §3.1D bounds/weights rows to
// full-metadata oracles: every rejection carries the supplied path, the dotted
// key (with breaker nesting), a finite code/property/shape class and a static
// reason. The record struct itself carries no value field, so the exact
// metadata assertions below are the no-observed-number guarantee (spec
// §4.1/§4.3).
func TestParseRejectsOutOfRangeFields(t *testing.T) {
	cases := []struct {
		name                  string
		yaml                  string
		key                   string
		code, property, shape string
		reason                string
		relatedKey            string
	}{
		{"http max request bytes", "http:\n  max_request_bytes: 0\n", "http.max_request_bytes", "bound_range", "range", "below_minimum", "outside permitted bounds", ""},
		{"http max request bytes ceiling", "http:\n  max_request_bytes: 1048577\n", "http.max_request_bytes", "bound_range", "range", "above_maximum", "outside permitted bounds", ""},
		{"http max sessions", "http:\n  max_sessions: 0\n", "http.max_sessions", "bound_range", "range", "below_minimum", "outside permitted bounds", ""},
		{"storage snapshot bytes", "storage:\n  max_snapshot_bytes: 0\n", "storage.max_snapshot_bytes", "bound_range", "range", "below_minimum", "outside permitted bounds", ""},
		{"catalog tools per server", "catalog:\n  max_tools_per_server: 0\n", "catalog.max_tools_per_server", "bound_range", "range", "below_minimum", "outside permitted bounds", ""},
		{"catalog tools total", "catalog:\n  max_tools_total: 0\n", "catalog.max_tools_total", "bound_range", "range", "below_minimum", "outside permitted bounds", ""},
		{"catalog pages", "catalog:\n  max_pages_per_server: 0\n", "catalog.max_pages_per_server", "bound_range", "range", "below_minimum", "outside permitted bounds", ""},
		{"catalog schema bytes", "catalog:\n  max_schema_bytes: 0\n", "catalog.max_schema_bytes", "bound_range", "range", "below_minimum", "outside permitted bounds", ""},
		{"search top-k", "search:\n  default_top_k: 0\n", "search.default_top_k", "bound_range", "range", "below_minimum", "outside permitted bounds", ""},
		{"search max top-k", "search:\n  max_top_k: 51\n", "search.max_top_k", "bound_range", "range", "above_maximum", "outside permitted bounds", ""},
		{"embedding dimensions", "embedding:\n  dimensions: 0\n", "embedding.dimensions", "bound_range", "range", "below_minimum", "outside permitted bounds", ""},
		{"embedding batch size", "embedding:\n  batch_size: 129\n", "embedding.batch_size", "bound_range", "range", "above_maximum", "outside permitted bounds", ""},
		{"embedding concurrency", "embedding:\n  max_concurrency: 0\n", "embedding.max_concurrency", "bound_range", "range", "below_minimum", "outside permitted bounds", ""},
		{"embedding response bytes", "embedding:\n  max_response_bytes: 0\n", "embedding.max_response_bytes", "bound_range", "range", "below_minimum", "outside permitted bounds", ""},
		{"upstream breaker open duration", "upstream_defaults:\n  breaker:\n    open_duration: 0s\n", "upstream_defaults.breaker.open_duration", "duration_positive", "duration", "nonpositive", "must be a positive duration", ""},
		{"search invalid weight sum", "search:\n  keyword_weight: 0.4\n  embedding_weight: 0.5\n", "search.keyword_weight", "weights_invalid", "weights", "sum_not_one", "search weights must be nonnegative, sum to one, and keyword_weight positive", "search.embedding_weight"},
		{"search nonpositive keyword weight", "search:\n  keyword_weight: 0\n  embedding_weight: 1\n", "search.keyword_weight", "weights_invalid", "weights", "keyword_nonpositive", "search weights must be nonnegative, sum to one, and keyword_weight positive", "search.embedding_weight"},
		{"search negative embedding weight", "search:\n  keyword_weight: 1.2\n  embedding_weight: -0.2\n", "search.embedding_weight", "weights_invalid", "weights", "embedding_negative", "search weights must be nonnegative, sum to one, and keyword_weight positive", "search.keyword_weight"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := strings.Replace(boundsEdgeBase, "pins: []", tc.yaml+"pins: []", 1)
			path := filepath.Join(t.TempDir(), "config.yaml")
			_, err := Parse([]byte(input), path)
			diags := requireDiagnostics(t, err)
			if diags.Incomplete {
				t.Fatal("a single bounds rejection is never incomplete")
			}
			record := diags.Records[0]
			if record.Path != path || record.Key != tc.key || record.Code != tc.code ||
				record.Property != tc.property || record.Shape != tc.shape ||
				record.Reason != tc.reason || record.RelatedKey != tc.relatedKey {
				t.Fatalf("record = %+v, want path=%q key=%q code=%s property=%s shape=%s reason=%q related=%q",
					record, path, tc.key, tc.code, tc.property, tc.shape, tc.reason, tc.relatedKey)
			}
			assertDiagnosticAtYAMLPath(t, []byte(input), record, tc.key)
			if tc.relatedKey != "" {
				assertRelatedDiagnosticAtYAMLPath(t, []byte(input), record, tc.relatedKey)
			}
			requireRendered(t, err,
				`config="`+path+`"`,
				`key="`+tc.key+`"`,
				"code="+tc.code, "property="+tc.property, "shape="+tc.shape,
				"; remaining_stages="+stageNotChecked,
			)
		})
	}
}

// TestParseCredentialFileBoundaries migrates the §5.2 bounds rows to
// full-metadata oracles: every rejection names the supplied config path, the
// complete location, the validated owner and file reference, a finite
// code/property/shape class and a static reason, with the not_checked stage
// footer. Every original case name is retained verbatim; the added rows cover
// the §5.2 additions (UTF-8, exactly 8192, one LF, only LF, CRLF, env parity).
func TestParseCredentialFileBoundaries(t *testing.T) {
	cases := []struct {
		name      string
		content   []byte
		special   string
		fragments []string
		forbidden []string
	}{
		{"missing", nil, "missing",
			[]string{"code=secret_file_missing", "property=existence", "shape=not_found", "secret file does not exist"},
			[]string{"secret file must be regular", "secret file must be nonempty"}},
		{"directory", nil, "directory",
			[]string{"code=secret_file_policy", "property=policy", "shape=non_regular", "secret file must be regular, non-symlink, operator-owned, and private"},
			nil},
		{"empty", []byte{}, "file",
			[]string{"code=secret_empty", "property=value", "shape=empty", "secret must be nonempty; provide a nonempty value for the named reference"},
			nil},
		{"oversized", []byte(strings.Repeat("x", 8193)), "file",
			[]string{"code=secret_too_large", "property=value", "shape=over_limit", "secret exceeds permitted 8KiB limit; provide a value within the configured limit"},
			[]string{"8193"}},
		{"carriage return", []byte("a\rb"), "file",
			[]string{"code=secret_cr", "property=value", "shape=contains_cr", "secret contains CR (carriage return); CRLF is not normalized"},
			nil},
		{"line feed", []byte("a\nb"), "file",
			[]string{"code=secret_lf", "property=value", "shape=contains_lf", "secret contains LF (newline) after stripping one final LF; files permit only one removable final LF and no remaining CR/LF/NUL"},
			nil},
		{"nul", []byte{'a', 0, 'b'}, "file",
			[]string{"code=secret_nul", "property=value", "shape=contains_nul", "secret contains NUL; remove the NUL from the credential source"},
			nil},
		{"two final newlines", []byte("token\n\n"), "file",
			[]string{"code=secret_lf", "property=value", "shape=trailing_lf", "after stripping one final LF"},
			nil},
		{"only newline", []byte("\n"), "file",
			[]string{"code=secret_empty", "property=value", "shape=empty", "secret must be nonempty; provide a nonempty value for the named reference"},
			nil},
		{"carriage return line feed", []byte("a\r\n"), "file",
			[]string{"code=secret_cr", "property=value", "shape=contains_cr", "secret contains CR (carriage return); CRLF is not normalized"},
			nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			ref := "secret"
			switch tc.special {
			case "missing":
			case "directory":
				if err := os.Mkdir(filepath.Join(dir, ref), 0700); err != nil {
					t.Fatal(err)
				}
			default:
				if err := os.WriteFile(filepath.Join(dir, ref), tc.content, 0600); err != nil {
					t.Fatal(err)
				}
			}
			input := strings.Replace(boundsEdgeBase, "endpoint: https://alpha.invalid/mcp", fmt.Sprintf("endpoint: https://alpha.invalid/mcp\n    credentials: {bearer: {file: %s}}", ref), 1)
			path := filepath.Join(dir, "config.yaml")
			_, err := Parse([]byte(input), path)
			diags := requireDiagnostics(t, err)
			requireRendered(t, err, append([]string{
				`config="` + path + `"`,
				`key="servers[0].credentials.bearer"`,
				`server="alpha"`,
				`ref="file:secret"`,
				"; remaining_stages=" + stageNotChecked,
			}, tc.fragments...)...)
			forbidRendered(t, err, tc.forbidden...)
			if diags.RemainingStages != stageNotChecked {
				t.Fatalf("expected the stage barrier to name skipped stages, got %q", diags.RemainingStages)
			}
		})
	}
	// §5.2 additions: the accepted boundaries of the unchanged one-terminal-LF
	// rule — a multi-byte UTF-8 token, exactly 8192 bytes, and exactly one
	// final LF — must parse and resolve to the exact file contents.
	for _, tc := range []struct {
		name    string
		content []byte
	}{
		{"valid utf-8", []byte("tökén-✓")},
		{"exactly 8192", []byte(strings.Repeat("x", 8192))},
		{"one final newline", []byte("token\n")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "secret"), tc.content, 0600); err != nil {
				t.Fatal(err)
			}
			input := strings.Replace(boundsEdgeBase, "endpoint: https://alpha.invalid/mcp", "endpoint: https://alpha.invalid/mcp\n    credentials: {bearer: {file: secret}}", 1)
			cfg, err := Parse([]byte(input), filepath.Join(dir, "config.yaml"))
			if err != nil {
				t.Fatalf("Parse rejected %s: %v", tc.name, err)
			}
			credentials, err := cfg.ServerCredentials("alpha")
			if err != nil {
				t.Fatalf("ServerCredentials rejected %s: %v", tc.name, err)
			}
			want := strings.TrimSuffix(string(tc.content), "\n")
			if credentials.Bearer != want {
				t.Fatalf("bearer token = %d bytes, want the %d file bytes with only one final LF removed", len(credentials.Bearer), len(want))
			}
			if strings.Contains(cfg.String(), string(want)) {
				t.Fatal("Config.String leaked the resolved credential")
			}
		})
	}
	t.Run("symlink rejected", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		if err := os.WriteFile(target, []byte("token"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(dir, "secret")); err != nil {
			t.Fatal(err)
		}
		input := strings.Replace(boundsEdgeBase, "endpoint: https://alpha.invalid/mcp", "endpoint: https://alpha.invalid/mcp\n    credentials: {bearer: {file: secret}}", 1)
		_, err := Parse([]byte(input), filepath.Join(dir, "config.yaml"))
		requireDiagnostics(t, err)
		requireRendered(t, err, "code=secret_file_policy", "property=policy", "shape=symlink", "secret file must be regular, non-symlink, operator-owned, and private")
	})
	t.Run("permissions rejected", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "secret")
		if err := os.WriteFile(path, []byte("token"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0644); err != nil {
			t.Fatal(err)
		}
		input := strings.Replace(boundsEdgeBase, "endpoint: https://alpha.invalid/mcp", "endpoint: https://alpha.invalid/mcp\n    credentials: {bearer: {file: secret}}", 1)
		_, err := Parse([]byte(input), filepath.Join(dir, "config.yaml"))
		requireDiagnostics(t, err)
		requireRendered(t, err, "code=secret_file_policy", "property=policy", "shape=not_private", "secret file must be regular, non-symlink, operator-owned, and private")
	})
	t.Run("environment parity", func(t *testing.T) {
		// The same finite cause classes apply to env references, with the
		// environment-specific LF reason: env values are never newline-stripped.
		for _, tc := range []struct {
			name      string
			value     string
			set       bool
			fragments []string
		}{
			{"env unset", "", false,
				[]string{"code=secret_unset", "property=value", "shape=unset", "secret environment variable is unset; set the named reference without printing its value"}},
			{"env empty", "", true,
				[]string{"code=secret_empty", "property=value", "shape=empty", "secret must be nonempty; provide a nonempty value for the named reference"}},
			{"env trailing newline", "canary\n", true,
				[]string{"code=secret_lf", "property=value", "shape=trailing_lf", "secret contains LF (newline); environment values are not newline-stripped"}},
			{"env embedded newline", "a\nb", true,
				[]string{"code=secret_lf", "property=value", "shape=contains_lf", "secret contains LF (newline); environment values are not newline-stripped"}},
			{"env carriage return", "a\rb", true,
				[]string{"code=secret_cr", "property=value", "shape=contains_cr", "secret contains CR (carriage return); CRLF is not normalized"}},
			{"env oversized", strings.Repeat("x", 8193), true,
				[]string{"code=secret_too_large", "property=value", "shape=over_limit", "secret exceeds permitted 8KiB limit; provide a value within the configured limit"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				const name = "BOUNDS_EDGE_PARITY_TOKEN"
				if tc.set {
					t.Setenv(name, tc.value)
				} else {
					t.Setenv(name, "present temporarily")
					if err := os.Unsetenv(name); err != nil {
						t.Fatal(err)
					}
				}
				input := strings.Replace(boundsEdgeBase, "endpoint: https://alpha.invalid/mcp", "endpoint: https://alpha.invalid/mcp\n    credentials: {bearer: {env: "+name+"}}", 1)
				_, err := Parse([]byte(input), filepath.Join(t.TempDir(), "config.yaml"))
				diags := requireDiagnostics(t, err)
				requireRendered(t, err, append([]string{
					`ref="env:` + name + `"`,
					"; remaining_stages=" + stageNotChecked,
				}, tc.fragments...)...)
				if diags.RemainingStages != stageNotChecked {
					t.Fatalf("expected the stage barrier, got %q", diags.RemainingStages)
				}
			})
		}
		t.Run("env accepted", func(t *testing.T) {
			t.Setenv("BOUNDS_EDGE_PARITY_TOKEN", "token")
			input := strings.Replace(boundsEdgeBase, "endpoint: https://alpha.invalid/mcp", "endpoint: https://alpha.invalid/mcp\n    credentials: {bearer: {env: BOUNDS_EDGE_PARITY_TOKEN}}", 1)
			cfg, err := Parse([]byte(input), filepath.Join(t.TempDir(), "config.yaml"))
			if err != nil {
				t.Fatalf("Parse rejected a valid env reference: %v", err)
			}
			credentials, err := cfg.ServerCredentials("alpha")
			if err != nil || credentials.Bearer != "token" {
				t.Fatalf("bearer = %q, err = %v; want the exact env value", credentials.Bearer, err)
			}
		})
	})
}

func TestEndpointNormalizationContributesCanonicalFingerprint(t *testing.T) {
	parse := func(endpoint string) *Config {
		t.Helper()
		input := strings.Replace(boundsEdgeBase, "https://alpha.invalid/mcp", endpoint, 1)
		cfg, err := Parse([]byte(input), t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		return cfg
	}
	first := parse("HTTPS://ALPHA.INVALID/mcp")
	second := parse("https://alpha.invalid/mcp")
	if first.Servers[0].Endpoint != "https://alpha.invalid/mcp" || first.Fingerprint() != second.Fingerprint() {
		t.Fatalf("normalized endpoint/fingerprint mismatch: %q", first.Servers[0].Endpoint)
	}
	if parse("https://alpha.invalid/mcp/").Fingerprint() == second.Fingerprint() {
		t.Fatal("trailing slash unexpectedly ignored by fingerprint")
	}
}

func TestMissingWeightKeyOmitsUnavailableSourcePosition(t *testing.T) {
	input := strings.Replace(boundsEdgeBase, "pins: []", "search:\n  keyword_weight: 0.5\npins: []", 1)
	_, err := Parse([]byte(input), filepath.Join(t.TempDir(), "config.yaml"))
	diags := requireDiagnostics(t, err)
	record := diags.Records[0]
	if record.Code != "weight_invalid" || record.Key != "search.embedding_weight" || record.Shape != "missing" {
		t.Fatalf("diagnostic = %+v, want missing embedding weight", record)
	}
	assertNoDiagnosticNodePosition(t, record)
}
