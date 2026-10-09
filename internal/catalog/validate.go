package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"

	"github.com/google/jsonschema-go/jsonschema"
)

func ValidateSchema(schemaJSON, instanceJSON string) error {
	resolved, err := resolveSchema(schemaJSON)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewBufferString(instanceJSON))
	decoder.UseNumber()
	var instance any
	if err := decoder.Decode(&instance); err != nil {
		return fmt.Errorf("decode instance: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return fmt.Errorf("decode trailing instance data: %w", err)
	}
	instance, err = exactNumbers(instance)
	if err != nil {
		return err
	}
	if err := resolved.Validate(instance); err != nil {
		return fmt.Errorf("invalid instance: %w", err)
	}
	return nil
}

// ValidateSchemaDocument reports whether a JSON Schema document is a resolvable
// schema. It is the admission check applied wherever a stored tool schema is
// about to be used - the describe validation status and the call admission -
// because only a resolvable schema can decide whether arguments are sound.
func ValidateSchemaDocument(schemaJSON string) error {
	_, err := resolveSchema(schemaJSON)
	return err
}

// ValidateSchemaStructure reports whether a JSON Schema document is structurally
// valid in the pinned dialect, without requiring its references to resolve. It
// is the durable catalog's admission check: a schema the bridge cannot resolve
// is still searchable and describable, and the catalog must serve it verbatim
// before and after a restart (plan §4.4, §4.5, §4.9, AC-10), which requires
// storage to retain it. Resolvability is demanded only where a schema is
// actually applied to arguments.
func ValidateSchemaStructure(schemaJSON string) error {
	_, err := decodeSchema(schemaJSON)
	return err
}

func decodeSchema(schemaJSON string) (*jsonschema.Schema, error) {
	var schema jsonschema.Schema
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		return nil, fmt.Errorf("decode JSON Schema: %w", err)
	}
	return &schema, nil
}

func resolveSchema(schemaJSON string) (*jsonschema.Resolved, error) {
	schema, err := decodeSchema(schemaJSON)
	if err != nil {
		return nil, err
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("resolve JSON Schema: %w", err)
	}
	return resolved, nil
}

func exactNumbers(value any) (any, error) {
	switch typed := value.(type) {
	case json.Number:
		text := typed.String()
		exact, ok := new(big.Rat).SetString(text)
		if !ok {
			return nil, fmt.Errorf("invalid JSON number")
		}
		if exact.IsInt() {
			integer := new(big.Int).Set(exact.Num())
			if integer.IsInt64() {
				return integer.Int64(), nil
			}
			if integer.Sign() > 0 && integer.BitLen() <= 64 {
				return integer.Uint64(), nil
			}
		}
		// jsonschema-go recognizes numbers only through Go's numeric kinds, so
		// a bare json.Number is validated as a string. Fractions and
		// out-of-range integers must become float64 for sound type checks.
		// Conversion is only used where it cannot lose information a check
		// depends on: below 2^53 a double still resolves every integer, and
		// larger non-integral values keep their exact decimal form so numeric
		// bounds remain exact. Literals outside float64 range stay raw and are
		// then rejected by type rather than silently rounding.
		fraction, err := typed.Float64()
		if err != nil {
			return typed, nil
		}
		if exact.IsInt() || math.Abs(fraction) < 1<<53 {
			return fraction, nil
		}
		return typed, nil
	case []any:
		result := make([]any, len(typed))
		for i, item := range typed {
			converted, err := exactNumbers(item)
			if err != nil {
				return nil, err
			}
			result[i] = converted
		}
		return result, nil
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			converted, err := exactNumbers(item)
			if err != nil {
				return nil, err
			}
			result[key] = converted
		}
		return result, nil
	default:
		return value, nil
	}
}
