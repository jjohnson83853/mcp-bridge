package catalog_test

import (
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
)

func TestJSONSchemaValidationMatrix(t *testing.T) {
	tests := []struct {
		name      string
		schema    string
		instance  string
		wantError bool
	}{
		{"large integer exact", `{"type":"integer","maximum":9007199254740992}`, `9007199254740993`, true},
		{"large decimal exact", `{"maximum":9007199254740992}`, `9007199254740992.5`, true},
		{"required", `{"type":"object","properties":{"a":{"type":"string"}},"required":["a"]}`, `{}`, true},
		{"additional property", `{"type":"object","properties":{"a":{"type":"string"}},"additionalProperties":false}`, `{"a":"ok","b":1}`, true},
		{"enum", `{"enum":["yes","no"]}`, `"maybe"`, true},
		{"valid", `{"type":"object","properties":{"a":{"type":"string"}},"required":["a"],"additionalProperties":false}`, `{"a":"ok"}`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := catalog.ValidateSchema(tc.schema, tc.instance)
			if (err != nil) != tc.wantError {
				t.Fatalf("ValidateSchema error=%v, wantError=%t", err, tc.wantError)
			}
		})
	}
}

func TestJSONSchemaRejectsUnresolvableExternalRef(t *testing.T) {
	err := catalog.ValidateSchema(`{"$ref":"https://schema.invalid/external.json"}`, `1`)
	if err == nil {
		t.Fatal("external reference was not rejected")
	}
}

func TestSchemaAdmissionDistinguishesStructureFromResolvability(t *testing.T) {
	unresolvable := `{"type":"object","properties":{"x":{"$ref":"#/$defs/missing"}}}`
	tests := []struct {
		name      string
		schema    string
		wantError bool
	}{
		{"unresolvable internal reference", unresolvable, false},
		{"resolvable internal reference", `{"$defs":{"x":{"type":"string"}},"type":"object","properties":{"x":{"$ref":"#/$defs/x"}}}`, false},
		{"type outside the dialect", `{"type":5}`, true},
		{"truncated document", `{"type":"object"`, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := catalog.ValidateSchemaStructure(tc.schema); (err != nil) != tc.wantError {
				t.Fatalf("ValidateSchemaStructure error=%v, wantError=%t", err, tc.wantError)
			}
		})
	}

	// The durable catalog retains a well-formed schema it cannot resolve,
	// because such a tool stays searchable and describable; only applying a
	// schema to arguments demands resolvability.
	if err := catalog.ValidateSchemaStructure(unresolvable); err != nil {
		t.Fatalf("retainable schema rejected: %v", err)
	}
	if err := catalog.ValidateSchemaDocument(unresolvable); err == nil {
		t.Fatal("unresolvable schema accepted as applicable")
	}
}
