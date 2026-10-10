package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptConfigOptOutDefaultAndValidation(t *testing.T) {
	base := "schema_version: 1\nservers:\n  - id: alpha\n    endpoint: https://alpha.invalid/mcp\npins: []\n"
	cases := []struct {
		name         string
		input        string
		wantDisabled bool
		wantErr      bool
	}{
		{name: "default enabled", input: base},
		{name: "explicit false", input: strings.Replace(base, "endpoint: https://alpha.invalid/mcp", "endpoint: https://alpha.invalid/mcp\n    prompts_disabled: false", 1)},
		{name: "opted out", input: strings.Replace(base, "endpoint: https://alpha.invalid/mcp", "endpoint: https://alpha.invalid/mcp\n    prompts_disabled: true", 1), wantDisabled: true},
		{name: "unknown key", input: strings.Replace(base, "endpoint: https://alpha.invalid/mcp", "endpoint: https://alpha.invalid/mcp\n    promptz_disabled: true", 1), wantErr: true},
		{name: "wrong type", input: strings.Replace(base, "endpoint: https://alpha.invalid/mcp", "endpoint: https://alpha.invalid/mcp\n    prompts_disabled: yes", 1), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Parse([]byte(tc.input), filepath.Join(t.TempDir(), "config.yaml"))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Parse error = %v, want rejection", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse error = %v, want success", err)
			}
			got := cfg.Servers[0].PromptsDisabled
			if got != tc.wantDisabled {
				t.Fatalf("PromptsDisabled = %v, want %v", got, tc.wantDisabled)
			}
		})
	}
}
