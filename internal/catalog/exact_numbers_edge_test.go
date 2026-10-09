package catalog_test

import (
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
)

func TestValidateSchemaPreservesExactJSONNumberSemantics(t *testing.T) {
	const bounded = `{"type":"integer","minimum":9007199254740992,"maximum":18446744073709551615}`
	for _, value := range []string{"9007199254740993", "18446744073709551615"} {
		t.Run(value, func(t *testing.T) {
			if err := catalog.ValidateSchema(bounded, value); err != nil {
				t.Fatalf("exact in-range integer rejected: %v", err)
			}
		})
	}
	for _, value := range []string{"9007199254740991", "1e10000"} {
		t.Run(value+"_outside", func(t *testing.T) {
			if err := catalog.ValidateSchema(bounded, value); err == nil {
				t.Fatalf("out-of-range integer accepted: %s", value)
			}
		})
	}
	if err := catalog.ValidateSchema(`{"type":"number","minimum":12,"maximum":13}`, "12.5"); err != nil {
		t.Fatalf("fractional number rejected: %v", err)
	}
}
