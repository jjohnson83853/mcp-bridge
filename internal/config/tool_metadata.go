package config

import (
	"encoding/json"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// MetadataPatch is one metadata override patch: a mapping of supported
// metadata field names (description, title, icons, annotations, _meta) to
// their JSON-encoded values. Values are owned JSON bytes; explicit false,
// empty members and nested null values are preserved exactly as configured —
// a null is a value, never a deletion instruction.
type MetadataPatch map[string]json.RawMessage

// ToolMetadataConfig is the optional server-owned tool_metadata mapping: the
// defaults patch every configured pin of the owning server inherits, plus the
// exact raw-tool-name exception patches layered above those defaults.
type ToolMetadataConfig struct {
	Defaults MetadataPatch            `yaml:"defaults" json:"defaults,omitempty"`
	Tools    map[string]MetadataPatch `yaml:"tools" json:"tools,omitempty"`
}

// supportedMetadataPatchMembers is the finite set of defaults patch members
// this contract admits; anything else rejects the whole candidate.
var supportedMetadataPatchMembers = set("description title icons annotations _meta")

// sortedKeys lists a patch's member names in sorted order so validation and
// hashing are deterministic regardless of document order.
func sortedKeys(patch MetadataPatch) []string {
	keys := make([]string, 0, len(patch))
	for key := range patch {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

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

// Validate admits only the supported patch shape on the JSON values. It is
// the programmatic counterpart of the parse-time node walk for patches that
// reach the package outside a document parse.
func (p MetadataPatch) Validate() error {
	for _, key := range sortedKeys(p) {
		if !supportedMetadataPatchMembers[key] {
			return fmt.Errorf("unsupported metadata patch member")
		}
		var value any
		if err := json.Unmarshal(p[key], &value); err != nil {
			return fmt.Errorf("metadata patch member is not valid JSON")
		}
		if err := validateMetadataMember(key, value); err != nil {
			return err
		}
	}
	return nil
}

// Clone returns an owned deep copy of the patch: no returned byte slice
// aliases the receiver or its callers.
func (p MetadataPatch) Clone() MetadataPatch {
	clone := make(MetadataPatch, len(p))
	for key, value := range p {
		clone[key] = append(json.RawMessage(nil), value...)
	}
	return clone
}

// ValidatePins admits one server's exact-tools mapping only when every entry
// names a tool actually pinned on that same server. Wildcard or cross-server
// selectors do not exist: an entry that matches no configured pin of the
// owning server rejects the whole candidate. Duplicate mapping entries are
// already the existing duplicate-key policy; the tool-name shape gate runs at
// parse time.
func (m *ToolMetadataConfig) ValidatePins(serverID string, pins []map[string]string) error {
	if m == nil || len(m.Tools) == 0 {
		return nil
	}
	configured := make(map[string]bool, len(pins))
	for _, pin := range pins {
		if pin["server"] == serverID {
			configured[pin["tool"]] = true
		}
	}
	for _, name := range sortedToolNames(m.Tools) {
		if !configured[name] {
			return fmt.Errorf("tool_metadata tools entry must match a configured pin of this server")
		}
	}
	return nil
}

// ToolMetadataTools returns an owned copy of one server's exact-tools
// exception mapping, or nil when the server declares none. Neither the
// returned map nor its patches alias the stored configuration.
func (c Config) ToolMetadataTools(serverID string) map[string]MetadataPatch {
	for _, server := range c.Servers {
		if server.ID == serverID && server.ToolMetadata != nil && len(server.ToolMetadata.Tools) > 0 {
			tools := make(map[string]MetadataPatch, len(server.ToolMetadata.Tools))
			for name, patch := range server.ToolMetadata.Tools {
				tools[name] = patch.Clone()
			}
			return tools
		}
	}
	return nil
}

// ToolMetadataToolPatch returns an owned copy of one exact-tool exception
// patch, or nil when the server declares none for that raw tool name.
func (c Config) ToolMetadataToolPatch(serverID, rawName string) MetadataPatch {
	for _, server := range c.Servers {
		if server.ID == serverID && server.ToolMetadata != nil {
			if patch, ok := server.ToolMetadata.Tools[rawName]; ok && len(patch) > 0 {
				return patch.Clone()
			}
			return nil
		}
	}
	return nil
}

// sortedToolNames lists the tools mapping entries in sorted order so
// validation is deterministic regardless of document order.
func sortedToolNames(tools map[string]MetadataPatch) []string {
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ToolMetadataDefaults returns an owned copy of one server's configured
// metadata defaults patch, or nil when the server declares none. The returned
// patch never aliases the stored configuration.
func (c Config) ToolMetadataDefaults(serverID string) MetadataPatch {
	for _, server := range c.Servers {
		if server.ID == serverID {
			if server.ToolMetadata == nil || len(server.ToolMetadata.Defaults) == 0 {
				return nil
			}
			return server.ToolMetadata.Defaults.Clone()
		}
	}
	return nil
}

// validateDefaultsValue is the parse-time shape gate of one defaults member.
// It rejects unsupported members and unsupported value shapes with the typed,
// metadata-only diagnostics vocabulary — no YAML or value echo — so an
// invalid patch rejects the entire candidate atomically.
func (v *docValidator) validateDefaultsValue(path, key string, node *yaml.Node, server string) *Diagnostics {
	if !supportedMetadataPatchMembers[key] {
		return v.fail(Diagnostic{
			Key: path, Server: server, Line: node.Line, Column: node.Column,
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
			if member.Kind != yaml.ScalarNode || (member.Tag != "!!str" && member.Tag != "!!bool") {
				return v.fail(Diagnostic{
					Key: joinKey(path, node.Content[i].Value), Server: server, Line: member.Line, Column: member.Column,
					Code: "tool_metadata_value", Property: "tool_metadata", Shape: shapeOf(member),
					Reason: "annotation hints must be strings or booleans",
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

// validateMetadataMember is the programmatic shape gate used by Validate.
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
		for _, member := range members {
			switch member.(type) {
			case string, bool:
			default:
				return fmt.Errorf("annotation hints must be strings or booleans")
			}
		}
	case "_meta":
		if _, ok := value.(map[string]any); !ok {
			return fmt.Errorf("must be a mapping")
		}
	}
	return nil
}
