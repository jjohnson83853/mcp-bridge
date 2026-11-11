package config

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// MetadataPatch is one metadata override patch: a mapping of supported
// metadata field names (description, title, icons, annotations, _meta) to
// their JSON-encoded values. Values are owned JSON bytes; explicit false,
// empty members and nested null values are preserved exactly as configured —
// a null is a value, never a deletion instruction.
type MetadataPatch map[string]json.RawMessage

// ToolMetadataConfig is the optional server-owned tool_metadata mapping:
// defaults patch eligible targets of the owning server, with exact
// raw-tool-name exception patches layered above those defaults.
type ToolMetadataConfig struct {
	Defaults MetadataPatch            `yaml:"defaults" json:"defaults,omitempty"`
	Tools    map[string]MetadataPatch `yaml:"tools" json:"tools,omitempty"`
}

// supportedMetadataPatchMembers is the finite set of defaults patch members
// this contract admits; anything else rejects the whole candidate.
var supportedMetadataPatchMembers = set("description title icons annotations _meta")

// supportedAnnotationKeys mirrors the build layer's annotation inventory in
// core.ToolMetadata.Validate: the SDK ToolAnnotation fields only, so a claim
// the SDK cannot represent rejects at the config layer instead of surfacing
// as a swallowed publication error.
var supportedAnnotationKeys = set("title readOnlyHint destructiveHint idempotentHint openWorldHint")

// UnmarshalYAML converts one patch mapping into owned JSON bytes without
// imposing shape here: structural validation is the typed node walk, so the
// decoder only performs lossless JSON conversion. A null member converts to
// the JSON literal null and stays a value.
func (p *MetadataPatch) UnmarshalYAML(node *yaml.Node) error {
	if node.Tag != "!!map" {
		return fmt.Errorf("metadata patch must be a mapping")
	}
	patch := make(MetadataPatch, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		var decoded any
		if err := node.Content[i+1].Decode(&decoded); err != nil {
			return fmt.Errorf("metadata patch member conversion: %w", err)
		}
		raw, err := json.Marshal(decoded)
		if err != nil {
			return fmt.Errorf("metadata patch member conversion: %w", err)
		}
		patch[key] = raw
	}
	*p = patch
	return nil
}

// ValidatePins keeps config parsing independent of the live tool catalog.
// Exact selector membership is validated against an accepted candidate
// catalog during service reload, after discovery has identified raw names.
func (m *ToolMetadataConfig) ValidatePins(_ string, _ []map[string]string) error {
	return nil
}

// validateDefaultsValue is the parse-time shape gate of one defaults member.
// It rejects unsupported members and unsupported value shapes with the typed,
// metadata-only diagnostics vocabulary — no YAML or value echo — so an
// invalid patch rejects the entire candidate atomically.
func (v *docValidator) validateDefaultsValue(path, key string, keyNode, node *yaml.Node, server string) *Diagnostics {
	if !supportedMetadataPatchMembers[key] {
		return v.fail(Diagnostic{
			Key: path, Server: server, Line: keyNode.Line, Column: keyNode.Column,
			Code: "tool_metadata_unknown_member", Property: "tool_metadata", Shape: "unknown_key",
			Reason: "metadata patches may only override description, title, icons, annotations and _meta",
		})
	}
	switch key {
	case "title", "description":
		if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
			return v.fail(Diagnostic{
				Key: path, Server: server, Line: node.Line, Column: node.Column,
				Code: "tool_metadata_value", Property: "tool_metadata", Shape: shapeOf(node),
				Reason: "must be a string",
			})
		}
	case "icons":
		if node.Kind != yaml.SequenceNode {
			return v.fail(Diagnostic{
				Key: path, Server: server, Line: node.Line, Column: node.Column,
				Code: "tool_metadata_value", Property: "tool_metadata", Shape: shapeOf(node),
				Reason: "must be a sequence of icon objects",
			})
		}
		for index, element := range node.Content {
			if element.Kind != yaml.MappingNode {
				return v.fail(Diagnostic{
					Key: fmt.Sprintf("%s[%d]", path, index), Server: server, Line: element.Line, Column: element.Column,
					Code: "tool_metadata_value", Property: "tool_metadata", Shape: shapeOf(element),
					Reason: "icons elements must be objects",
				})
			}
		}
	case "annotations":
		if node.Kind != yaml.MappingNode {
			return v.fail(Diagnostic{
				Key: path, Server: server, Line: node.Line, Column: node.Column,
				Code: "tool_metadata_value", Property: "tool_metadata", Shape: shapeOf(node),
				Reason: "must be a mapping of annotation hints",
			})
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			member := node.Content[i+1]
			if !supportedAnnotationKeys[node.Content[i].Value] {
				return v.fail(Diagnostic{
					Key: joinKey(path, node.Content[i].Value), Server: server,
					Line: node.Content[i].Line, Column: node.Content[i].Column,
					Code: "tool_metadata_value", Property: "tool_metadata", Shape: "unknown_key",
					Reason: "unsupported annotation key",
				})
			}
			if node.Content[i].Value == "title" {
				if member.Kind != yaml.ScalarNode || member.Tag != "!!str" {
					return v.fail(Diagnostic{
						Key: joinKey(path, "title"), Server: server, Line: member.Line, Column: member.Column,
						Code: "tool_metadata_value", Property: "tool_metadata", Shape: shapeOf(member),
						Reason: "annotation title must be a string",
					})
				}
				continue
			}
			if member.Kind != yaml.ScalarNode || member.Tag != "!!bool" {
				return v.fail(Diagnostic{
					Key: joinKey(path, node.Content[i].Value), Server: server, Line: member.Line, Column: member.Column,
					Code: "tool_metadata_value", Property: "tool_metadata", Shape: shapeOf(member),
					Reason: "annotation hints must be booleans",
				})
			}
		}
	case "_meta":
		if node.Kind != yaml.MappingNode {
			return v.fail(Diagnostic{
				Key: path, Server: server, Line: node.Line, Column: node.Column,
				Code: "tool_metadata_value", Property: "tool_metadata", Shape: shapeOf(node),
				Reason: "must be a mapping",
			})
		}
	}
	return nil
}

// validateMetadataMember is the programmatic shape gate for one decoded
// patch member, mirroring the parse-time node walk's member inventory and
// value types.
func validateMetadataMember(key string, value any) error {
	switch key {
	case "title", "description":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("must be a string")
		}
	case "icons":
		elements, ok := value.([]any)
		if !ok {
			return fmt.Errorf("must be a sequence of icon objects")
		}
		for _, element := range elements {
			if _, ok := element.(map[string]any); !ok {
				return fmt.Errorf("icons elements must be objects")
			}
		}
	case "annotations":
		members, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("must be a mapping of annotation hints")
		}
		for key, member := range members {
			if !supportedAnnotationKeys[key] {
				return fmt.Errorf("unsupported annotation key")
			}
			if key == "title" {
				if _, ok := member.(string); !ok {
					return fmt.Errorf("annotation title must be a string")
				}
				continue
			}
			if _, ok := member.(bool); !ok {
				return fmt.Errorf("annotation hints must be booleans")
			}
		}
	case "_meta":
		if _, ok := value.(map[string]any); !ok {
			return fmt.Errorf("must be a mapping")
		}
	}
	return nil
}
