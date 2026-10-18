package config

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestParseRejectsMalformedSecretReferencesAndHeaders migrates the F-01 matrix
// to full-metadata oracles: every negative case retains its fixture and gains
// the exact key/owner/class plus a static reason, with the temp-joined config
// filename as the supplied path (spec §5.2 F-01).
func TestParseRejectsMalformedSecretReferencesAndHeaders(t *testing.T) {
	cases := []struct {
		name, insertion string
		fragments       []string
		forbidden       []string
	}{
		{"literal server bearer", "credentials: {bearer: plain}\n",
			[]string{`key="servers[0].credentials.bearer"`, `server="alpha"`, "code=reference_required", "property=reference", "shape=string", "secret must use an env or file reference"},
			[]string{"plain"}},
		{"invalid secret mapping", "credentials: {bearer: {other: TOKEN}}\n",
			[]string{`key="servers[0].credentials.bearer"`, `server="alpha"`, "code=reference_shape", "property=reference", "shape=mapping", "secret must be exactly {env: NAME} or {file: PATH}"},
			nil},
		{"invalid env name", "credentials: {bearer: {env: BAD-NAME}}\n",
			[]string{`key="servers[0].credentials.bearer"`, `server="alpha"`, "code=reference_env_name", "property=reference", "shape=invalid_identifier", "invalid environment variable name"},
			[]string{"BAD-NAME"}},
		{"interpolated file reference", "credentials: {bearer: {file: '$HOME/token'}}\n",
			[]string{`key="servers[0].credentials.bearer"`, `server="alpha"`, "code=reference_literal_required", "property=reference", "shape=literal", "secret references cannot contain interpolation or commands"},
			[]string{"$HOME", "token"}},
		{"invalid header name", "credentials:\n  headers:\n    'bad header': {env: TOKEN}\n",
			[]string{`key="servers[0].credentials.headers"`, `server="alpha"`, "code=header_name_invalid", "property=name", "shape=invalid_identifier", "invalid credential header name"},
			[]string{"bad header"}},
		{"forbidden authorization header", "credentials:\n  headers:\n    Authorization: {env: TOKEN}\n",
			[]string{`key="servers[0].credentials.headers[\"Authorization\"]"`, `server="alpha"`, "code=header_forbidden", "property=name", "shape=forbidden", "credential header is forbidden"},
			nil},
		{"duplicate case-insensitive headers", "credentials:\n  headers:\n    X-Test: {env: TOKEN}\n    x-test: {env: TOKEN2}\n",
			[]string{`key="servers[0].credentials.headers[\"x-test\"]"`, `related_key="servers[0].credentials.headers[\"X-Test\"]"`, `server="alpha"`, "code=header_duplicate", "property=name", "shape=casefold", "duplicate credential header"},
			nil},
		{"header literal value", "credentials:\n  headers:\n    X-Test: literal\n",
			[]string{`key="servers[0].credentials.headers[\"X-Test\"]"`, `server="alpha"`, "code=reference_required", "property=reference", "shape=string", "credential header value must use a secret reference"},
			[]string{"literal"}},
		{"embedding invalid base url", "embedding:\n  base_url: 'file:///tmp/api'\n",
			[]string{`key="embedding.base_url"`, "code=url_invalid", "property=absolute_http_url", "shape=invalid_url", "invalid absolute HTTP(S) URL"},
			[]string{"file:///tmp/api"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := strings.Replace(boundsEdgeBase, "endpoint: https://alpha.invalid/mcp", "endpoint: https://alpha.invalid/mcp\n    "+strings.ReplaceAll(tc.insertion, "\n", "\n    "), 1)
			if strings.HasPrefix(tc.insertion, "embedding:") {
				input = strings.Replace(boundsEdgeBase, "pins: []", tc.insertion+"pins: []", 1)
			}
			path := filepath.Join(t.TempDir(), "config.yaml")
			_, err := Parse([]byte(input), path)
			diags := requireDiagnostics(t, err)
			requireRendered(t, err, append([]string{`config="` + path + `"`}, tc.fragments...)...)
			forbidRendered(t, err, tc.forbidden...)
			if diags.RemainingStages != stageNotChecked {
				t.Fatalf("expected the stage barrier to name skipped stages, got %q", diags.RemainingStages)
			}
		})
	}
}
