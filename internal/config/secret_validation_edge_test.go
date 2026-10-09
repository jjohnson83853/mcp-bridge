package config

import (
	"strings"
	"testing"
)

func TestParseRejectsMalformedSecretReferencesAndHeaders(t *testing.T) {
	cases := []struct{ name, insertion, want string }{
		{"literal server bearer", "credentials: {bearer: plain}\n", "secret must use an env or file reference"},
		{"invalid secret mapping", "credentials: {bearer: {other: TOKEN}}\n", "exactly {env: NAME} or {file: PATH}"},
		{"invalid env name", "credentials: {bearer: {env: BAD-NAME}}\n", "environment variable name"},
		{"interpolated file reference", "credentials: {bearer: {file: '$HOME/token'}}\n", "interpolation or commands"},
		{"invalid header name", "credentials:\n  headers:\n    'bad header': {env: TOKEN}\n", "invalid credential header name"},
		{"forbidden authorization header", "credentials:\n  headers:\n    Authorization: {env: TOKEN}\n", "is forbidden"},
		{"duplicate case-insensitive headers", "credentials:\n  headers:\n    X-Test: {env: TOKEN}\n    x-test: {env: TOKEN2}\n", "duplicate credential header"},
		{"header literal value", "credentials:\n  headers:\n    X-Test: literal\n", "must use a secret reference"},
		{"embedding invalid base url", "embedding:\n  base_url: 'file:///tmp/api'\n", "embedding base_url"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := strings.Replace(boundsEdgeBase, "endpoint: https://alpha.invalid/mcp", "endpoint: https://alpha.invalid/mcp\n    "+strings.ReplaceAll(tc.insertion, "\n", "\n    "), 1)
			if strings.HasPrefix(tc.insertion, "embedding:") {
				input = strings.Replace(boundsEdgeBase, "pins: []", tc.insertion+"pins: []", 1)
			}
			_, err := Parse([]byte(input), t.TempDir())
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.want)) {
				t.Fatalf("Parse error = %v, want class/field %q", err, tc.want)
			}
		})
	}
}
