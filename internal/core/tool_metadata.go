package core

import (
	"encoding/json"
	"fmt"
)

// annotationKeys is the complete supported annotation inventory: the SDK
// ToolAnnotation fields. Any other key is unsupported and rejected, because a
// claim the SDK cannot represent must never be stored or published.
var annotationKeys = map[string]bool{
	"title":           true,
	"readOnlyHint":    true,
	"destructiveHint": true,
	"idempotentHint":  true,
	"openWorldHint":   true,
}

// ToolMetadata is one tool's SDK-normalized upstream metadata, separate from
// the description (which remains the definition's own authoritative field).
// The supported inventory is the title, the icons, the annotation object
// (title plus the four standard hints) and the supported `_meta` keys, each
// held as canonical raw JSON. The zero value is the metadata-absent form and
// canonicalizes to nil, so legacy digests stay byte-stable.
type ToolMetadata struct {
	Title       string          `json:"title,omitempty"`
	Icons       json.RawMessage `json:"icons,omitempty"`
	Annotations json.RawMessage `json:"annotations,omitempty"`
	Meta        json.RawMessage `json:"_meta,omitempty"`
}

// IsZero reports whether the metadata is the absent form.
func (m ToolMetadata) IsZero() bool {
	return m.Title == "" && len(m.Icons) == 0 && len(m.Annotations) == 0 && len(m.Meta) == 0
}

// Clone deep-copies the raw metadata bytes, so a clone shares no backing array
// with its source and can never mutate catalog state. The result is immutable
// by ownership: callers must treat it as read-only.
func (m ToolMetadata) Clone() ToolMetadata {
	clone := ToolMetadata{Title: m.Title}
	clone.Icons = cloneRaw(m.Icons)
	clone.Annotations = cloneRaw(m.Annotations)
	clone.Meta = cloneRaw(m.Meta)
	return clone
}

func cloneRaw(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	clone := make(json.RawMessage, len(raw))
	copy(clone, raw)
	return clone
}

// canonical returns the compact canonical serialization of the metadata, or
// nil for the absent form, which is the exact form folded into the definition
// digest and the catalog revision.
func (m ToolMetadata) canonical() json.RawMessage {
	if m.IsZero() {
		return nil
	}
	encoded, err := json.Marshal(struct {
		Title       string          `json:"title,omitempty"`
		Icons       json.RawMessage `json:"icons,omitempty"`
		Annotations json.RawMessage `json:"annotations,omitempty"`
		Meta        json.RawMessage `json:"_meta,omitempty"`
	}{m.Title, m.Icons, m.Annotations, m.Meta})
	if err != nil {
		// Every member is already valid JSON, so this is unreachable.
		return nil
	}
	return encoded
}

// Validate admits only the supported metadata shape. Icons must be a JSON
// array; annotations and `_meta` must be JSON objects; annotation members are
// the SDK inventory with a string title and boolean hints. Validation checks
// shape, not semantics: hints are advisory claims, never authorization.
func (m ToolMetadata) Validate() error {
	if len(m.Icons) > 0 {
		var icons []json.RawMessage
		if err := json.Unmarshal(m.Icons, &icons); err != nil {
			return fmt.Errorf("core: tool metadata icons must be a JSON array: %w", err)
		}
	}
	if len(m.Annotations) > 0 {
		var annotations map[string]json.RawMessage
		if err := json.Unmarshal(m.Annotations, &annotations); err != nil {
			return fmt.Errorf("core: tool metadata annotations must be a JSON object: %w", err)
		}
		for key, value := range annotations {
			if !annotationKeys[key] {
				return fmt.Errorf("core: tool metadata annotation key %q is unsupported", key)
			}
			if key == "title" {
				var title string
				if err := json.Unmarshal(value, &title); err != nil {
					return fmt.Errorf("core: tool metadata annotation title must be a string")
				}
				continue
			}
			var hint bool
			if err := json.Unmarshal(value, &hint); err != nil {
				return fmt.Errorf("core: tool metadata annotation hint %q must be a boolean", key)
			}
		}
	}
	if len(m.Meta) > 0 {
		var meta map[string]json.RawMessage
		if err := json.Unmarshal(m.Meta, &meta); err != nil {
			return fmt.Errorf("core: tool metadata _meta must be a JSON object: %w", err)
		}
	}
	return nil
}
