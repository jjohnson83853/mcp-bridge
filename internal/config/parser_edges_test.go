package config

import (
	"strings"
	"testing"
	"time"
)

func TestParseRejectsMalformedDocuments(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"oversized", []byte(strings.Repeat("x", maxConfigBytes+1)), "1 MiB"},
		{"malformed yaml", []byte("schema_version: [\n"), "decode configuration"},
		{"scalar root", []byte("hello\n"), "mapping"},
		{"sequence root", []byte("- hello\n"), "mapping"},
		{"second document", []byte(boundsEdgeBase + "---\nschema_version: 1\n"), "one YAML document"},
		{"nested unknown key", []byte(strings.Replace(boundsEdgeBase, "pins: []", "search:\n  not_a_setting: true\npins: []", 1)), "unknown key"},
		{"wrong section type", []byte(strings.Replace(boundsEdgeBase, "pins: []", "http: []\npins: []", 1)), "decode configuration"},
		{"non-string allowlist element", []byte(strings.Replace(boundsEdgeBase, "pins: []", "http:\n  allowed_hosts: [42]\npins: []", 1)), "entries must be strings"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.data, "config.yaml")
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.want)) {
				t.Fatalf("Parse error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestReadSettingReportsMissingAndWrongTypedValues(t *testing.T) {
	cases := []struct {
		name         string
		cfg          Config
		section, key string
		target       any
		want         string
	}{
		{"missing nested parent", Config{}, "upstream_defaults", "breaker.consecutive_failures", new(int), "upstream_defaults.breaker.consecutive_failures"},
		{"missing nested leaf", Config{UpstreamDefaults: map[string]any{"breaker": map[string]any{}}}, "upstream_defaults", "breaker.consecutive_failures", new(int), "upstream_defaults.breaker.consecutive_failures"},
		{"wrong string", Config{HTTP: map[string]any{"listen": 5}}, "http", "listen", new(string), "http.listen"},
		{"wrong int", Config{HTTP: map[string]any{"burst": 1.5}}, "http", "burst", new(int), "http.burst"},
		{"wrong int64", Config{HTTP: map[string]any{"max_request_bytes": "large"}}, "http", "max_request_bytes", new(int64), "http.max_request_bytes"},
		{"wrong float", Config{HTTP: map[string]any{"requests_per_second": "fast"}}, "http", "requests_per_second", new(float64), "http.requests_per_second"},
		{"wrong duration type", Config{Search: map[string]any{"timeout": 1}}, "search", "timeout", new(time.Duration), "search.timeout"},
		{"invalid duration", Config{Search: map[string]any{"timeout": "tomorrow"}}, "search", "timeout", new(time.Duration), "search.timeout"},
		{"unsupported destination", Config{HTTP: map[string]any{"listen": "127.0.0.1"}}, "http", "listen", new(bool), "configuration reader has no destination"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := readSetting(tc.cfg, tc.section, tc.key, tc.target)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("readSetting error = %v, want %q", err, tc.want)
			}
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
