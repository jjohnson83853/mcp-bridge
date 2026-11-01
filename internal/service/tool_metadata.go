package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
)

// sdkToolDescriptor builds the complete effective SDK descriptor of one
// published tool: the identity, description and schemas, plus the title,
// annotations, icons and `_meta` of the definition's metadata. It is the one
// shared conversion for pinned publication, so no published tool ever loses
// supported metadata, and unsupported metadata shapes reject before
// publication instead of surfacing as a swallowed AddTool error.
func sdkToolDescriptor(name string, def core.ToolDefinition) (*mcp.Tool, error) {
	if err := def.Metadata.Validate(); err != nil {
		return nil, fmt.Errorf("service: tool %q metadata: %w", name, err)
	}
	descriptor := &mcp.Tool{
		Name:        name,
		Description: def.Description,
		InputSchema: json.RawMessage(def.InputSchema),
		Title:       def.Metadata.Title,
	}
	if len(def.OutputSchema) > 0 {
		descriptor.OutputSchema = json.RawMessage(def.OutputSchema)
	}
	if len(def.Metadata.Icons) > 0 {
		var icons []mcp.Icon
		if err := json.Unmarshal(def.Metadata.Icons, &icons); err != nil {
			return nil, fmt.Errorf("service: tool %q icons: %w", name, err)
		}
		descriptor.Icons = icons
	}
	if len(def.Metadata.Annotations) > 0 {
		annotations := &mcp.ToolAnnotations{}
		if err := json.Unmarshal(def.Metadata.Annotations, annotations); err != nil {
			return nil, fmt.Errorf("service: tool %q annotations: %w", name, err)
		}
		descriptor.Annotations = annotations
	}
	if len(def.Metadata.Meta) > 0 {
		meta := mcp.Meta{}
		if err := json.Unmarshal(def.Metadata.Meta, &meta); err != nil {
			return nil, fmt.Errorf("service: tool %q _meta: %w", name, err)
		}
		descriptor.Meta = meta
	}
	return descriptor, nil
}

// upstreamMetadataValue renders a definition's metadata for the describe
// payload, or nil when the definition carries none, so the describe view
// exposes exactly the SDK-normalized source metadata.
func upstreamMetadataValue(metadata core.ToolMetadata) map[string]any {
	if metadata.IsZero() {
		return nil
	}
	value := map[string]any{}
	if metadata.Title != "" {
		value["title"] = metadata.Title
	}
	if len(metadata.Icons) > 0 {
		value["icons"] = rawValue(metadata.Icons)
	}
	if len(metadata.Annotations) > 0 {
		value["annotations"] = rawValue(metadata.Annotations)
	}
	if len(metadata.Meta) > 0 {
		value["_meta"] = rawValue(metadata.Meta)
	}
	return value
}

// MetadataOverrideField names one metadata field path a server default (or,
// in a later slice, an exact-tool exception) won for one pinned tool.
type MetadataOverrideField struct {
	Path  string `json:"path"`
	Layer string `json:"layer"`
}

// effectiveMetadataView is the derived, owned effective view of one pinned
// tool: the definition (including description and effective metadata) and the
// sorted provenance of every winning override path.
type effectiveMetadataView struct {
	Definition     core.ToolDefinition
	OverrideFields []MetadataOverrideField
}

// deriveToolMetadata applies the approved layering source → server defaults →
// exact-tool exception to one source definition. Object members merge
// recursively; arrays and scalars replace outright; absent, false, empty and
// null values stay distinct values, so the merge semantics are shared across
// both patch layers. The result is owned: nothing aliases the definition or
// either patch. No patches yields the untouched definition with an empty,
// nonnil provenance list, and a path both layers touch records the tool
// layer as the winner.
func deriveToolMetadata(def core.ToolDefinition, defaults, tool config.MetadataPatch) (effectiveMetadataView, error) {
	view := effectiveMetadataView{Definition: def, OverrideFields: []MetadataOverrideField{}}
	if len(defaults) == 0 && len(tool) == 0 {
		return view, nil
	}
	metadata := def.Metadata.Clone()
	description := def.Description
	layers := map[string]string{}
	for _, layer := range []struct {
		scope string
		patch config.MetadataPatch
	}{{"defaults", defaults}, {"tool", tool}} {
		if len(layer.patch) == 0 {
			continue
		}
		won, err := applyMetadataPatch(&metadata, &description, layer.patch, layer.scope)
		if err != nil {
			return view, err
		}
		for path := range won {
			// Defaults run before the tool layer, so the assignment here is
			// unconditional; a path the tool layer also touches is recorded
			// as the tool layer's win when that layer runs later.
			if layer.scope == "tool" {
				layers[path] = "tool"
			} else {
				layers[path] = "server_default"
			}
		}
	}
	view.Definition.Description = description
	view.Definition.Metadata = metadata
	if err := metadata.Validate(); err != nil {
		return view, fmt.Errorf("service: effective metadata: %w", err)
	}
	for _, path := range sortedOverridePaths(wonPaths(layers)) {
		view.OverrideFields = append(view.OverrideFields, MetadataOverrideField{Path: path, Layer: layers[path]})
	}
	return view, nil
}

// wonPaths lists the winning field paths from the provenance map.
func wonPaths(layers map[string]string) map[string]bool {
	paths := make(map[string]bool, len(layers))
	for path := range layers {
		paths[path] = true
	}
	return paths
}

// withheldValueError wraps a JSON parse cause behind constant-text prose so
// derivation diagnostics never echo a fragment of the configured (withheld)
// value, matching the config-diagnostic convention. Unwrap exposes the
// internal cause for tests and internal callers; surfaced text is constant.
type withheldValueError struct {
	text  string
	cause error
}

func (e *withheldValueError) Error() string { return e.text }
func (e *withheldValueError) Unwrap() error { return e.cause }

// withheldValue builds the constant-text error for one rejected JSON member.
func withheldValue(text string, cause error) error {
	return &withheldValueError{text: text, cause: cause}
}

// applyMetadataPatch overlays one patch layer onto the cloned metadata and
// description: the supported members are applied with the shared merge
// semantics and the winning field paths are returned. The scope names the
// layer in owned error text only.
func applyMetadataPatch(metadata *core.ToolMetadata, description *string, patch config.MetadataPatch, scope string) (map[string]bool, error) {
	paths := map[string]bool{}
	for _, key := range sortedPatchKeys(patch) {
		raw := patch[key]
		switch key {
		case "title":
			var title string
			if err := json.Unmarshal(raw, &title); err != nil {
				return nil, withheldValue(fmt.Sprintf("service: %s title: invalid JSON", scope), err)
			}
			metadata.Title = title
			paths["title"] = true
		case "description":
			var member string
			if err := json.Unmarshal(raw, &member); err != nil {
				return nil, withheldValue(fmt.Sprintf("service: %s description: invalid JSON", scope), err)
			}
			*description = member
			paths["description"] = true
		case "icons":
			if !json.Valid(raw) {
				return nil, fmt.Errorf("service: %s icons is not valid JSON", scope)
			}
			metadata.Icons = append(json.RawMessage(nil), raw...)
			paths["icons"] = true
		case "annotations":
			merged, won, err := mergeMetadataObject(metadata.Annotations, raw, "annotations")
			if err != nil {
				return nil, fmt.Errorf("service: %s annotations: %w", scope, err)
			}
			metadata.Annotations = merged
			for path := range won {
				paths[path] = true
			}
		case "_meta":
			merged, won, err := mergeMetadataObject(metadata.Meta, raw, "_meta")
			if err != nil {
				return nil, fmt.Errorf("service: %s _meta: %w", scope, err)
			}
			metadata.Meta = merged
			for path := range won {
				paths[path] = true
			}
		}
	}
	return paths, nil
}

// sortedPatchKeys lists one patch's member names in sorted order so the
// effective view is deterministic regardless of document order.
func sortedPatchKeys(patch config.MetadataPatch) []string {
	keys := make([]string, 0, len(patch))
	for key := range patch {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// sortedOverridePaths lists the winning field paths in sorted order.
func sortedOverridePaths(paths map[string]bool) []string {
	list := make([]string, 0, len(paths))
	for path := range paths {
		list = append(list, path)
	}
	sort.Strings(list)
	return list
}

// mergeMetadataObject recursively overlays one JSON object patch member onto
// the base object: object members merge member-by-member, arrays and scalars
// replace outright, and a null is a value, never a deletion. It returns the
// owned merged JSON and the set of winning leaf paths.
func mergeMetadataObject(base, patch json.RawMessage, rootPath string) (json.RawMessage, map[string]bool, error) {
	baseMembers := map[string]json.RawMessage{}
	if len(base) > 0 {
		if err := json.Unmarshal(base, &baseMembers); err != nil {
			return nil, nil, fmt.Errorf("source %s: %w", rootPath, err)
		}
	}
	patchMembers := map[string]json.RawMessage{}
	if err := json.Unmarshal(patch, &patchMembers); err != nil {
		return nil, nil, fmt.Errorf("patch member: %w", err)
	}
	won := map[string]bool{}
	for _, key := range sortedPatchMemberKeys(patchMembers) {
		patchValue := patchMembers[key]
		baseValue := baseMembers[key]
		if isJSONObject(patchValue) && isJSONObject(baseValue) {
			merged, nested, err := mergeMetadataObject(baseValue, patchValue, rootPath+"."+key)
			if err != nil {
				return nil, nil, err
			}
			baseMembers[key] = merged
			for path := range nested {
				won[path] = true
			}
			continue
		}
		if !json.Valid(patchValue) {
			return nil, nil, fmt.Errorf("member %q is not valid JSON", key)
		}
		baseMembers[key] = append(json.RawMessage(nil), patchValue...)
		won[rootPath+"."+key] = true
	}
	merged, err := json.Marshal(baseMembers)
	if err != nil {
		return nil, nil, err
	}
	return json.RawMessage(merged), won, nil
}

// sortedPatchMemberKeys lists one object's member keys in sorted order.
func sortedPatchMemberKeys(members map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(members))
	for key := range members {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// isJSONObject reports whether one raw JSON value is an object.
func isJSONObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimLeft(raw, " \t\r\n")
	return len(trimmed) > 0 && trimmed[0] == '{'
}

// applyServerMetadataDefaults derives effective metadata for resolved pins
// using the shared source → defaults → exact-tool function. Exact selectors
// are validated against the accepted same-server catalog/pins, but unpinned
// tools remain unrevealed until normal reveal eligibility admits them.
func applyServerMetadataDefaults(
	candidate *config.Config,
	resolved []core.ResolvedPin,
	definitions map[string][]core.ToolDefinition,
	mode buildMode,
) ([]core.ResolvedPin, map[string]pinMetadataEntry, error) {
	type serverPatches struct {
		defaults config.MetadataPatch
		tools    map[string]config.MetadataPatch
	}
	patches := make(map[string]serverPatches, len(candidate.Servers))
	for _, server := range candidate.Servers {
		if server.ToolMetadata == nil {
			continue
		}
		patch := serverPatches{tools: server.ToolMetadata.Tools}
		if len(server.ToolMetadata.Defaults) > 0 {
			patch.defaults = server.ToolMetadata.Defaults
		}
		if len(patch.defaults) > 0 || len(patch.tools) > 0 {
			patches[server.ID] = patch
		}
	}
	if len(patches) == 0 {
		return resolved, nil, nil
	}
	if err := validateMetadataToolTargets(candidate, resolved, definitions, mode); err != nil {
		return nil, nil, err
	}
	views := make(map[string]pinMetadataEntry, len(resolved))
	updated := append([]core.ResolvedPin(nil), resolved...)
	for index := range updated {
		serverID := updated[index].PinnedTool.ServerID
		server, ok := patches[serverID]
		if !ok {
			continue
		}
		rawName := updated[index].PinnedTool.RawName
		view, err := deriveToolMetadata(updated[index].Binding.Def, server.defaults, server.tools[rawName])
		if err != nil {
			return nil, nil, fmt.Errorf("service: %s metadata overrides for [server=%q,tool=%q]: %w",
				mode, serverID, rawName, err)
		}
		binding := *updated[index].Binding
		binding.Def = view.Definition
		updated[index].Binding = &binding
		views[serverID+"\x00"+rawName] = pinMetadataEntry{
			Effective:      view.Definition.Metadata,
			OverrideFields: view.OverrideFields,
		}
	}
	return updated, views, nil
}

type serverToolMetadata struct {
	defaults config.MetadataPatch
	tools    map[string]config.MetadataPatch
}

func cloneServerToolMetadata(candidate *config.Config) map[string]serverToolMetadata {
	if candidate == nil {
		return nil
	}
	var metadata map[string]serverToolMetadata
	for _, server := range candidate.Servers {
		if server.ToolMetadata == nil ||
			(len(server.ToolMetadata.Defaults) == 0 && len(server.ToolMetadata.Tools) == 0) {
			continue
		}
		if metadata == nil {
			metadata = make(map[string]serverToolMetadata, len(candidate.Servers))
		}
		var tools map[string]config.MetadataPatch
		if len(server.ToolMetadata.Tools) > 0 {
			tools = make(map[string]config.MetadataPatch, len(server.ToolMetadata.Tools))
			for rawName, patch := range server.ToolMetadata.Tools {
				tools[rawName] = cloneMetadataPatch(patch)
			}
		}
		metadata[server.ID] = serverToolMetadata{
			defaults: cloneMetadataPatch(server.ToolMetadata.Defaults),
			tools:    tools,
		}
	}
	return metadata
}

func cloneMetadataPatch(source config.MetadataPatch) config.MetadataPatch {
	if len(source) == 0 {
		return nil
	}
	clone := make(config.MetadataPatch, len(source))
	for key, value := range source {
		clone[key] = append(json.RawMessage(nil), value...)
	}
	return clone
}

// validateMetadataToolTargets checks every exact selector before applying any
// patch, so a stale or cross-server name rejects the complete candidate.
func validateMetadataToolTargets(
	candidate *config.Config,
	resolved []core.ResolvedPin,
	definitions map[string][]core.ToolDefinition,
	mode buildMode,
) error {
	for _, server := range candidate.Servers {
		if server.ToolMetadata == nil {
			continue
		}
		for _, rawName := range sortedMetadataToolNames(server.ToolMetadata.Tools) {
			if !hasAcceptedMetadataTarget(definitions, resolved, server.ID, rawName) {
				return fmt.Errorf("service: %s metadata overrides for [server=%q,tool=%q]: selector does not match an accepted candidate tool or pin",
					mode, server.ID, rawName)
			}
		}
	}
	return nil
}

func hasAcceptedMetadataTarget(
	definitions map[string][]core.ToolDefinition,
	resolved []core.ResolvedPin,
	serverID, rawName string,
) bool {
	for _, definition := range definitions[serverID] {
		if definition.RawName == rawName {
			return true
		}
	}
	for _, pin := range resolved {
		if pin.PinnedTool.ServerID == serverID && pin.PinnedTool.RawName == rawName {
			return true
		}
	}
	return false
}

func sortedMetadataToolNames(tools map[string]config.MetadataPatch) []string {
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
