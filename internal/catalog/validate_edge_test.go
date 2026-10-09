package catalog_test

import (
	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
	"testing"
)

func TestSchemaValidationRejectsDocumentAndInstanceFailures(t *testing.T) {
	for _, schema := range []string{"{", `{"type":{}}`, `{"$ref":"#/$defs/missing"}`} {
		if err := catalog.ValidateSchemaDocument(schema); err == nil {
			t.Fatalf("schema accepted: %s", schema)
		}
	}
	cases := []struct{ name, instance string }{{"malformed instance", "{"}, {"trailing instance", "{} {}"}, {"numeric overflow", "1e10000"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := catalog.ValidateSchema(`{"type":"object"}`, tc.instance); err == nil {
				t.Fatalf("instance accepted: %s", tc.instance)
			}
		})
	}
}
