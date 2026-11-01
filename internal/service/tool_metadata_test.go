package service

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

// legacyRowDigest is the definition digest of the metadata-absent descriptor
// below, captured at HEAD eb879ef before ticket-03. It freezes the baseline
// that metadata-absent rows keep their stored definition hash.
const legacyRowDigest = "a8298cfd4ae81b1fcfd3453b4167defefba3c3e81de784a88d97192a0fd07e53"

func metadataDefinition() core.ToolDefinition {
	return core.ToolDefinition{
		RawName:     "read",
		Description: "Read a file",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
		Metadata: core.ToolMetadata{
			Title:       "Read a file",
			Annotations: json.RawMessage(`{"title":"Read","readOnlyHint":true,"idempotentHint":true}`),
			Icons:       json.RawMessage(`[{"src":"https://example.test/read.svg"}]`),
			Meta:        json.RawMessage(`{"audience":["user"]}`),
		},
	}
}

// The published SDK descriptor of a pinned tool carries the full effective
// definition: schemas, output schema, title, annotations, icons and _meta.
func TestMetadataDescriptorCarriesSource(t *testing.T) {
	descriptor, err := sdkToolDescriptor("alpha__read", metadataDefinition())
	if err != nil {
		t.Fatalf("sdkToolDescriptor: %v", err)
	}
	if descriptor.Name != "alpha__read" || descriptor.Description != "Read a file" {
		t.Fatalf("descriptor identity = %q/%q", descriptor.Name, descriptor.Description)
	}
	if string(descriptor.InputSchema.(json.RawMessage)) != `{"type":"object","properties":{"path":{"type":"string"}}}` {
		t.Fatalf("descriptor input schema = %s", descriptor.InputSchema)
	}
	if descriptor.Title != "Read a file" {
		t.Fatalf("descriptor title = %q; want %q", descriptor.Title, "Read a file")
	}
	if descriptor.Annotations == nil || !descriptor.Annotations.ReadOnlyHint || !descriptor.Annotations.IdempotentHint {
		t.Fatalf("descriptor annotations lost the source hints: %+v", descriptor.Annotations)
	}
	if descriptor.Annotations.Title != "Read" {
		t.Fatalf("descriptor annotation title = %q; want %q", descriptor.Annotations.Title, "Read")
	}
	if len(descriptor.Icons) != 1 || descriptor.Icons[0].Source != "https://example.test/read.svg" {
		t.Fatalf("descriptor icons = %+v", descriptor.Icons)
	}
	if descriptor.Meta["audience"] == nil {
		t.Fatalf("descriptor _meta lost the supported keys: %v", descriptor.Meta)
	}

	absent := metadataDefinition()
	absent.Metadata = core.ToolMetadata{}
	descriptor, err = sdkToolDescriptor("alpha__read", absent)
	if err != nil {
		t.Fatalf("sdkToolDescriptor absent metadata: %v", err)
	}
	if descriptor.Title != "" || descriptor.Annotations != nil || len(descriptor.Icons) != 0 || len(descriptor.Meta) != 0 {
		t.Fatalf("absent metadata produced descriptor metadata: %+v", descriptor)
	}

	invalid := metadataDefinition()
	invalid.Metadata = core.ToolMetadata{Icons: json.RawMessage(`{"src":"x"}`)}
	if _, err := sdkToolDescriptor("alpha__read", invalid); err == nil {
		t.Fatal("sdkToolDescriptor accepted an unsupported metadata shape; want rejection")
	}
}

// The durable round trip preserves source metadata: snapshot rows with
// metadata restore into definitions with metadata, and the indexed rows hash
// metadata into the definition digest while metadata-absent rows keep the
// frozen legacy digest.
// A pinned tool whose stored metadata passes Validate but fails SDK descriptor
// construction must reject the whole publication rather than silently dropping
// one tool while publishing the rest (fail-closed, like the derivation gate).
func TestPublishedRejectsCandidateWhenDescriptorFails(t *testing.T) {
	server := NewMCPServer()
	surface := &ToolSurface{runtime: NewRuntime()}
	if err := surface.Register(server); err != nil {
		t.Fatal(err)
	}
	goodDef := metadataDefinition()
	goodDef.RawName = "good"
	brokenDef := metadataDefinition()
	brokenDef.RawName = "broken"
	brokenDef.Metadata.Icons = json.RawMessage(`[123]`) // passes Validate (JSON array), fails SDK icon unmarshal
	assembly := &Assembly{pins: []core.ResolvedPin{
		{PinnedTool: core.PinnedTool{ServerID: "alpha", RawName: "good"}, Binding: &core.Binding{Def: goodDef}},
		{PinnedTool: core.PinnedTool{ServerID: "alpha", RawName: "broken"}, Binding: &core.Binding{Def: brokenDef}},
	}}
	surface.Published(assembly)
	client := connectPromptClient(t, server, nil)
	page, err := client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"search_tools": true, "describe_tool": true, "call_tool": true}
	got := make([]string, 0, len(page.Tools))
	for _, tool := range page.Tools {
		got = append(got, tool.Name)
		delete(want, tool.Name)
	}
	if len(want) != 0 || len(got) != 3 {
		t.Fatalf("published tools after rejected candidate = %v; missing %v", got, want)
	}
}

func TestMetadataSourceHashAndRestoreRoundtrip(t *testing.T) {
	row := storage.Tool{
		ServerID:             "alpha",
		RawName:              "read",
		PublicName:           "alpha__read",
		Description:          "Read a file",
		DefinitionHash:       legacyRowDigest,
		EmbeddingContentHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		InputSchema:          json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
		Metadata: &core.ToolMetadata{
			Title:       "Read a file",
			Annotations: json.RawMessage(`{"title":"Read","readOnlyHint":true}`),
		},
	}
	definitions := definitionsFromSnapshot(&storage.Snapshot{
		Bindings: []storage.Binding{{ServerID: "alpha", EndpointFingerprint: endpointFingerprint("http://upstream.test")}},
		Tools:    []storage.Tool{row},
	}, "alpha", "http://upstream.test")
	if len(definitions) != 1 {
		t.Fatalf("restore returned %d definitions; want 1", len(definitions))
	}
	restored := definitions[0]
	if restored.Metadata.Title != "Read a file" || len(restored.Metadata.Annotations) == 0 {
		t.Fatalf("restore lost source metadata: %+v", restored.Metadata)
	}

	rows := indexedToolRows("alpha", []core.ToolDefinition{restored})
	if len(rows) != 1 {
		t.Fatalf("index returned %d rows; want 1", len(rows))
	}
	expected := core.DefinitionDigest("alpha", "read", restored)
	if rows[0].DefinitionHash != expected {
		t.Fatalf("row definition hash %s; want %s", rows[0].DefinitionHash, expected)
	}
	if rows[0].Metadata == nil || rows[0].Metadata.Title != "Read a file" {
		t.Fatalf("indexed row lost metadata: %+v", rows[0].Metadata)
	}

	absent := restored
	absent.Metadata = core.ToolMetadata{}
	legacyRows := indexedToolRows("alpha", []core.ToolDefinition{absent})
	if legacyRows[0].DefinitionHash != legacyRowDigest {
		t.Fatalf("metadata-absent row hash %s; want frozen legacy %s", legacyRows[0].DefinitionHash, legacyRowDigest)
	}
	if legacyRows[0].Metadata != nil {
		t.Fatalf("metadata-absent row gained metadata: %+v", legacyRows[0].Metadata)
	}
}

// An output schema rides the published descriptor next to the metadata, so
// descriptor-aware publication never drops it.
func TestMetadataDescriptorCarriesOutputSchema(t *testing.T) {
	def := metadataDefinition()
	def.OutputSchema = json.RawMessage(`{"type":"object","properties":{"content":{"type":"string"}}}`)
	descriptor, err := sdkToolDescriptor("alpha__read", def)
	if err != nil {
		t.Fatalf("sdkToolDescriptor: %v", err)
	}
	if string(descriptor.OutputSchema.(json.RawMessage)) != `{"type":"object","properties":{"content":{"type":"string"}}}` {
		t.Fatalf("descriptor output schema = %s", descriptor.OutputSchema)
	}
}

// Derivation diagnostics must surface constant-text errors only: a malformed
// configured metadata value must never echo a fragment of the withheld value.
func TestDerivationErrorDoesNotEchoValue(t *testing.T) {
	source := core.ToolDefinition{RawName: "read", InputSchema: json.RawMessage(`{"type":"object"}`)}
	_, err := deriveToolMetadata(source, config.MetadataPatch{"title": json.RawMessage(`"secret" trailing`)}, nil)
	if err == nil {
		t.Fatal("malformed title patch accepted")
	}
	if got := err.Error(); got != "service: defaults title: invalid JSON" {
		t.Fatalf("title error prose = %q; want constant text", got)
	}
	_, err = deriveToolMetadata(source, config.MetadataPatch{"description": json.RawMessage(`[not-json`)}, nil)
	if err == nil {
		t.Fatal("malformed description patch accepted")
	}
	if got := err.Error(); got != "service: defaults description: invalid JSON" {
		t.Fatalf("description error prose = %q; want constant text", got)
	}
}

// TestMetadataServerDefaults verifies the pure server-defaults derivation:
// nested object members merge, arrays and scalars replace, explicit false,
// empty and null stay values, provenance is sorted with the server_default
// layer, the source definition is never aliased or mutated, and an invalid
// effective shape rejects the whole derivation.
func TestMetadataServerDefaults(t *testing.T) {
	source := core.ToolDefinition{
		RawName:     "read",
		Description: "Read a file",
		InputSchema: json.RawMessage(`{"type":"object"}`),
		Metadata: core.ToolMetadata{
			Title:       "Read a file",
			Annotations: json.RawMessage(`{"title":"Read","readOnlyHint":true}`),
			Meta:        json.RawMessage(`{"owner":"alpha"}`),
		},
	}
	defaults := config.MetadataPatch{
		"title": json.RawMessage(`"Corrected read"`),
		"annotations": json.RawMessage(
			`{"readOnlyHint":false,"destructiveHint":false,"title":"Corrected"}`),
		"_meta": json.RawMessage(`{"owner":"beta","policy":null}`),
	}
	view, err := deriveToolMetadata(source, defaults, nil)
	if err != nil {
		t.Fatalf("deriveToolMetadata: %v", err)
	}
	effective := view.Definition.Metadata
	if effective.Title != "Corrected read" {
		t.Fatalf("effective title = %q", effective.Title)
	}
	var annotations map[string]any
	if err := json.Unmarshal(effective.Annotations, &annotations); err != nil {
		t.Fatalf("decode effective annotations: %v", err)
	}
	if annotations["readOnlyHint"] != false || annotations["destructiveHint"] != false {
		t.Fatalf("explicit false not retained: %s", effective.Annotations)
	}
	if annotations["title"] != "Corrected" {
		t.Fatalf("nested annotation member not replaced: %s", effective.Annotations)
	}
	var meta map[string]any
	if err := json.Unmarshal(effective.Meta, &meta); err != nil {
		t.Fatalf("decode effective _meta: %v", err)
	}
	if meta["owner"] != "beta" {
		t.Fatalf("nested _meta member not replaced: %s", effective.Meta)
	}
	value, present := meta["policy"]
	if !present || value != nil {
		t.Fatalf("null _meta value not retained as a value: %s", effective.Meta)
	}
	// Provenance: every winning path, server_default layer, sorted order.
	wantPaths := []string{"_meta.owner", "_meta.policy", "annotations.destructiveHint", "annotations.readOnlyHint", "annotations.title", "title"}
	if len(view.OverrideFields) != len(wantPaths) {
		t.Fatalf("provenance = %v; want %v", view.OverrideFields, wantPaths)
	}
	for index, want := range wantPaths {
		if view.OverrideFields[index].Path != want || view.OverrideFields[index].Layer != "server_default" {
			t.Fatalf("provenance[%d] = %+v; want %s/server_default", index, view.OverrideFields[index], want)
		}
	}
	// The source definition and its metadata are untouched and unaliased.
	if source.Metadata.Title != "Read a file" || string(source.Metadata.Annotations) != `{"title":"Read","readOnlyHint":true}` {
		t.Fatalf("derivation mutated the source metadata: %+v", source.Metadata)
	}
	if view.Definition.Metadata.Annotations == nil || &view.Definition.Metadata.Annotations[0] == &source.Metadata.Annotations[0] {
		t.Fatal("effective annotations aliased the source bytes")
	}
	if view.Definition.Description != "Read a file" || view.Definition.InputSchema == nil {
		t.Fatal("derivation disturbed the definition outside the patched fields")
	}

	// No patch: identical definition, empty nonnil provenance.
	untouched, err := deriveToolMetadata(source, nil, nil)
	if err != nil {
		t.Fatalf("deriveToolMetadata(nil): %v", err)
	}
	if len(untouched.OverrideFields) != 0 || untouched.OverrideFields == nil {
		t.Fatalf("no-patch provenance = %v; want empty nonnil", untouched.OverrideFields)
	}

	// An unsupported effective shape rejects the derivation instead of
	// publishing it.
	invalid := config.MetadataPatch{"annotations": json.RawMessage(`{"unknownHint":true}`)}
	if _, err := deriveToolMetadata(source, nil, invalid); err == nil {
		t.Fatal("deriveToolMetadata accepted an unsupported annotation key")
	}
	broken := config.MetadataPatch{"title": json.RawMessage(`[1]`)}
	if _, err := deriveToolMetadata(source, nil, broken); err == nil {
		t.Fatal("deriveToolMetadata accepted a non-string title")
	}
}

// exceptionsConfigYAML builds a parsed config whose alpha server declares
// defaults plus exact tool exceptions and pins both tools the scenarios use.
func exceptionsConfigYAML(t *testing.T, metadata string) *config.Config {
	t.Helper()
	t.Setenv("TEST_CLIENT_SECRET", "private-client-value")
	document := `schema_version: 1
servers:
  - id: alpha
    endpoint: https://alpha.invalid/mcp
` + metadata + `client_auth:
  bearer_tokens:
    - {env: TEST_CLIENT_SECRET}
pins:
  - {server: alpha, tool: read}
  - {server: alpha, tool: write}
`
	candidate, err := config.Parse([]byte(document), ".")
	if err != nil {
		t.Fatalf("config.Parse rejected the exception fixture: %v", err)
	}
	return candidate
}

func exceptionPins(read, write core.ToolDefinition) []core.ResolvedPin {
	return []core.ResolvedPin{
		{PinnedTool: core.PinnedTool{ServerID: "alpha", RawName: "read"}, Binding: &core.Binding{ServerID: "alpha", RawName: "read", Def: read}},
		{PinnedTool: core.PinnedTool{ServerID: "alpha", RawName: "write"}, Binding: &core.Binding{ServerID: "alpha", RawName: "write", Def: write}},
	}
}

func annotationsFor(t *testing.T, metadata core.ToolMetadata) map[string]json.RawMessage {
	t.Helper()
	members := map[string]json.RawMessage{}
	if err := json.Unmarshal(metadata.Annotations, &members); err != nil {
		t.Fatalf("decode annotations %s: %v", metadata.Annotations, err)
	}
	return members
}

func hintValue(t *testing.T, members map[string]json.RawMessage, key string) bool {
	t.Helper()
	var value bool
	if err := json.Unmarshal(members[key], &value); err != nil {
		t.Fatalf("decode hint %s: %v", members[key], err)
	}
	return value
}

// TestMetadataExactToolPrecedence verifies the pure three-layer derivation on
// the resolved pins: source → server defaults → exact-tool exception, shared
// merge semantics, winning provenance per path, and that a pin without an
// exception keeps only the defaults.
func TestMetadataExactToolPrecedence(t *testing.T) {
	candidate := exceptionsConfigYAML(t, `    tool_metadata:
      defaults:
        annotations:
          readOnlyHint: true
          destructiveHint: false
          idempotentHint: true
      tools:
        write:
          annotations:
            readOnlyHint: false
            destructiveHint: true
            idempotentHint: false
            title: Write
`)
	write := core.ToolDefinition{RawName: "write", Description: "Write a file", InputSchema: json.RawMessage(`{"type":"object"}`)}
	read := core.ToolDefinition{RawName: "read", Description: "Read a file", InputSchema: json.RawMessage(`{"type":"object"}`)}
	definitions := map[string][]core.ToolDefinition{"alpha": {read, write}}
	_, views, err := applyServerMetadataDefaults(candidate, exceptionPins(read, write), definitions, buildReload)
	if err != nil {
		t.Fatalf("applyServerMetadataDefaults: %v (RED)", err)
	}
	writeEntry := views["alpha\x00write"]
	members := annotationsFor(t, writeEntry.Effective)
	if hintValue(t, members, "readOnlyHint") || !hintValue(t, members, "destructiveHint") || hintValue(t, members, "idempotentHint") {
		t.Fatalf("write exception did not win over the broad default: %s (RED)", writeEntry.Effective.Annotations)
	}
	readEntry := views["alpha\x00read"]
	readMembers := annotationsFor(t, readEntry.Effective)
	if !hintValue(t, readMembers, "readOnlyHint") {
		t.Fatalf("read lost its default readOnlyHint: %s (RED)", readEntry.Effective.Annotations)
	}
	layers := map[string]string{}
	for _, field := range writeEntry.OverrideFields {
		layers[field.Path] = field.Layer
	}
	for path, layer := range map[string]string{
		"annotations.readOnlyHint":    "tool",
		"annotations.destructiveHint": "tool",
		"annotations.idempotentHint":  "tool",
		"annotations.title":           "tool",
	} {
		if layers[path] != layer {
			t.Fatalf("provenance %q layer = %q; want %q (RED)", path, layers[path], layer)
		}
	}
	readLayers := map[string]string{}
	for _, field := range readEntry.OverrideFields {
		readLayers[field.Path] = field.Layer
	}
	for _, path := range []string{"annotations.readOnlyHint", "annotations.destructiveHint", "annotations.idempotentHint"} {
		if readLayers[path] != "server_default" {
			t.Fatalf("provenance %q layer = %q; want server_default (RED)", path, readLayers[path])
		}
	}

	// A tool-only patch without defaults applies directly to the source, and
	// a pin without an exception entry keeps the source view untouched.
	toolOnly := exceptionsConfigYAML(t, `    tool_metadata:
      tools:
        write:
          annotations:
            readOnlyHint: false
`)
	source := core.ToolDefinition{RawName: "write", Description: "Write a file", InputSchema: json.RawMessage(`{"type":"object"}`), Metadata: core.ToolMetadata{Annotations: json.RawMessage(`{"readOnlyHint":true}`)}}
	toolDefinitions := map[string][]core.ToolDefinition{"alpha": {source}}
	_, views, err = applyServerMetadataDefaults(toolOnly, []core.ResolvedPin{{PinnedTool: core.PinnedTool{ServerID: "alpha", RawName: "write"}, Binding: &core.Binding{ServerID: "alpha", RawName: "write", Def: source}}}, toolDefinitions, buildReload)
	if err != nil {
		t.Fatalf("applyServerMetadataDefaults tool-only: %v (RED)", err)
	}
	if hintValue(t, annotationsFor(t, views["alpha\x00write"].Effective), "readOnlyHint") {
		t.Fatalf("tool-only exception not applied to the source: %s (RED)", views["alpha\x00write"].Effective.Annotations)
	}
	readSource := core.ToolDefinition{RawName: "read", Description: "Read a file", InputSchema: json.RawMessage(`{"type":"object"}`)}
	readDefinitions := map[string][]core.ToolDefinition{"alpha": {source, readSource}}
	_, views, err = applyServerMetadataDefaults(toolOnly, []core.ResolvedPin{{PinnedTool: core.PinnedTool{ServerID: "alpha", RawName: "read"}, Binding: &core.Binding{ServerID: "alpha", RawName: "read", Def: readSource}}}, readDefinitions, buildReload)
	if err != nil {
		t.Fatalf("applyServerMetadataDefaults unpatched pin: %v (RED)", err)
	}
	if entry, ok := views["alpha\x00read"]; ok && len(entry.OverrideFields) != 0 {
		t.Fatalf("unpatched pin gained provenance %v (RED)", entry.OverrideFields)
	}
}

// TestUpstreamMetadataValueOmitsEmptyTitle verifies the describe view emits
// the "title" member only when the metadata carries a non-empty title:
// icons-only metadata must not publish an empty title member.
func TestUpstreamMetadataValueOmitsEmptyTitle(t *testing.T) {
	value := upstreamMetadataValue(core.ToolMetadata{
		Title: "Read",
		Icons: json.RawMessage(`[{"src":"https://example.test/read.svg"}]`),
	})
	if got, _ := value["title"].(string); got != "Read" {
		t.Fatalf("titled metadata lost its title: %v", value)
	}
	iconsOnly := upstreamMetadataValue(core.ToolMetadata{
		Icons: json.RawMessage(`[{"src":"https://example.test/read.svg"}]`),
	})
	if iconsOnly == nil {
		t.Fatal("icons-only metadata rendered as nil")
	}
	if _, present := iconsOnly["title"]; present {
		t.Fatalf("icons-only metadata emitted an empty title member: %v", iconsOnly)
	}
	if iconsOnly["icons"] == nil {
		t.Fatalf("icons-only metadata lost its icons: %v", iconsOnly)
	}
}
// TestProgressiveMetadataParity verifies an unpinned exact target uses the
// same source → defaults → exact-tool derivation and winning provenance as a
// pinned target, without changing the source definition or its schemas.
func TestProgressiveMetadataParity(t *testing.T) {
	t.Setenv("TEST_CLIENT_SECRET", "private-client-value")
	metadata := `    tool_metadata:
      defaults:
        title: Default title
        description: Default description
        annotations:
          readOnlyHint: true
          destructiveHint: true
          idempotentHint: false
      tools:
        read:
          title: Exact title
          annotations:
            readOnlyHint: false
            destructiveHint: false
`
	pinnedCandidate := exceptionsConfigYAML(t, metadata)
	document := `schema_version: 1
servers:
  - id: alpha
    endpoint: https://alpha.invalid/mcp
` + metadata + `client_auth:
  bearer_tokens:
    - {env: TEST_CLIENT_SECRET}
pins: []
`
	revealedCandidate, err := config.Parse([]byte(document), ".")
	if err != nil {
		t.Fatalf("Parse rejected the unpinned candidate config: %v", err)
	}
	if len(revealedCandidate.Pins) != 0 {
		t.Fatalf("exact metadata selector revealed or pinned a tool: %v", revealedCandidate.Pins)
	}

	source := metadataDefinition()
	source.OutputSchema = json.RawMessage(`{"type":"object","properties":{"content":{"type":"string"}}}`)
	sourceBefore := source
	sourceBefore.InputSchema = append(json.RawMessage(nil), source.InputSchema...)
	sourceBefore.OutputSchema = append(json.RawMessage(nil), source.OutputSchema...)
	sourceBefore.Metadata = source.Metadata.Clone()
	definitions := map[string][]core.ToolDefinition{"alpha": {source}}
	unpinned, unpinnedViews, err := applyServerMetadataDefaults(revealedCandidate, nil, definitions, buildReload)
	if err != nil {
		t.Fatalf("catalog rejected an exact same-server unpinned selector: %v", err)
	}
	if len(unpinned) != 0 || len(unpinnedViews) != 0 {
		t.Fatalf("metadata configuration revealed an unpinned tool: pins=%v views=%v", unpinned, unpinnedViews)
	}
	if _, _, err := applyServerMetadataDefaults(revealedCandidate, nil, map[string][]core.ToolDefinition{"beta": {source}}, buildReload); err == nil {
		t.Fatal("catalog accepted a same raw name belonging only to another server")
	}
	unmatchedDocument := strings.Replace(document, "            destructiveHint: false\n", "            destructiveHint: false\n        absent:\n          title: Unknown\n", 1)
	unmatchedCandidate, err := config.Parse([]byte(unmatchedDocument), ".")
	if err != nil {
		t.Fatalf("Parse rejected a well-shaped selector before catalog validation: %v", err)
	}
	unmatchedPins, unmatchedViews, err := applyServerMetadataDefaults(unmatchedCandidate, nil, definitions, buildReload)
	if err == nil {
		t.Fatal("candidate with a selector absent from its same-server catalog was accepted")
	}
	if unmatchedPins != nil || unmatchedViews != nil {
		t.Fatalf("unmatched selector partially applied metadata: pins=%v views=%v", unmatchedPins, unmatchedViews)
	}
	resolved := []core.ResolvedPin{{
		PinnedTool: core.PinnedTool{ServerID: "alpha", RawName: "read"},
		Binding:    &core.Binding{ServerID: "alpha", RawName: "read", Def: source},
	}}
	pinned, views, err := applyServerMetadataDefaults(pinnedCandidate, resolved, definitions, buildReload)
	if err != nil {
		t.Fatalf("applyServerMetadataDefaults: %v", err)
	}
	toolMetadata := revealedCandidate.Servers[0].ToolMetadata
	revealed, err := deriveToolMetadata(source, toolMetadata.Defaults, toolMetadata.Tools["read"])
	if err != nil {
		t.Fatalf("deriveToolMetadata for revealed target: %v", err)
	}
	if !reflect.DeepEqual(pinned[0].Binding.Def, revealed.Definition) {
		t.Fatalf("pinned and revealed effective definitions differ:\npinned:  %+v\nrevealed: %+v", pinned[0].Binding.Def, revealed.Definition)
	}
	pinnedFields := views["alpha\x00read"].OverrideFields
	if !reflect.DeepEqual(pinnedFields, revealed.OverrideFields) {
		t.Fatalf("pinned and revealed provenance differs: %v vs %v", pinnedFields, revealed.OverrideFields)
	}
	wantLayers := map[string]string{
		"title":                         "tool",
		"description":                  "server_default",
		"annotations.readOnlyHint":     "tool",
		"annotations.destructiveHint":  "tool",
		"annotations.idempotentHint":   "server_default",
	}
	gotLayers := make(map[string]string, len(revealed.OverrideFields))
	for _, field := range revealed.OverrideFields {
		gotLayers[field.Path] = field.Layer
	}
	if !reflect.DeepEqual(gotLayers, wantLayers) {
		t.Fatalf("winning provenance = %v; want %v", gotLayers, wantLayers)
	}
	if revealed.Definition.Description != "Default description" || revealed.Definition.Metadata.Title != "Exact title" {
		t.Fatalf("source/default/exact precedence was not preserved: %+v", revealed.Definition)
	}
	if hintValue(t, annotationsFor(t, revealed.Definition.Metadata), "readOnlyHint") ||
		hintValue(t, annotationsFor(t, revealed.Definition.Metadata), "destructiveHint") ||
		hintValue(t, annotationsFor(t, revealed.Definition.Metadata), "idempotentHint") {
		t.Fatalf("explicit false or default hint precedence was lost: %s", revealed.Definition.Metadata.Annotations)
	}
	if !reflect.DeepEqual(source, sourceBefore) ||
		string(revealed.Definition.InputSchema) != string(source.InputSchema) ||
		string(revealed.Definition.OutputSchema) != string(source.OutputSchema) {
		t.Fatal("progressive metadata derivation mutated source data or schemas")
	}
}
