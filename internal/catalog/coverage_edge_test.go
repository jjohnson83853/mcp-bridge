package catalog_test

import (
	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
	"testing"
)

func TestValidationAdditionalObservableBoundaries(t *testing.T) {
	schema := `{"$ref":"#/$defs/missing"}`
	if err := catalog.ValidateSchemaStructure(schema); err != nil {
		t.Fatalf("unresolvable reference is structurally valid: %v", err)
	}
	if err := catalog.ValidateSchemaDocument(schema); err == nil {
		t.Fatal("unresolvable reference accepted for use")
	}
	for _, valid := range []string{`{"unknownKeyword":true}`, `true`, `false`} {
		if err := catalog.ValidateSchemaDocument(valid); err != nil {
			t.Errorf("resolvable schema rejected %s: %v", valid, err)
		}
	}
	for _, bad := range []string{"{", "{} {}"} {
		if err := catalog.ValidateSchemaDocument(bad); err == nil {
			t.Errorf("malformed schema accepted: %q", bad)
		}
	}
	validSchema := `{"type":"object","required":["name"],"properties":{"name":{"enum":["Ada"]}},"additionalProperties":false}`
	if err := catalog.ValidateSchema(validSchema, `{"name":"Ada"}`); err != nil {
		t.Fatalf("valid instance rejected: %v", err)
	}
	for _, bad := range []string{`[]`, `{}`, `{"name":"Grace"}`, `{"name":"Ada","extra":true}`} {
		if err := catalog.ValidateSchema(validSchema, bad); err == nil {
			t.Errorf("invalid instance accepted: %s", bad)
		}
	}
}
