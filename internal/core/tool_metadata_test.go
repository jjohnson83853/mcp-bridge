package core

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

// legacyReadDigest is the DefinitionDigest of digestFixture captured at HEAD
// eb879ef, before ticket-03 introduced ToolMetadata. It freezes the r9
// compatibility baseline: metadata-absent definitions must keep hashing to it.
const legacyReadDigest = "a8298cfd4ae81b1fcfd3453b4167defefba3c3e81de784a88d97192a0fd07e53"

func digestFixture() ToolDefinition {
	return ToolDefinition{
		RawName:     "read",
		Description: "Read a file",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
	}
}

func metadataFixture() ToolMetadata {
	return ToolMetadata{
		Title:       "Read a file",
		Icons:       json.RawMessage(`[{"src":"https://example.test/read.svg","mime_type":"image/svg+xml"}]`),
		Annotations: json.RawMessage(`{"title":"Read","readOnlyHint":true,"destructiveHint":false,"idempotentHint":true,"openWorldHint":false}`),
		Meta:        json.RawMessage(`{"audience":["user"]}`),
	}
}

// The r9 compatibility baseline: a definition without metadata must hash to
// the pre-ticket-03 digest, so legacy revisions and stored definition hashes
// stay stable.
func TestDefinitionDigestIgnoresAbsentMetadata(t *testing.T) {
	if got := DefinitionDigest("alpha", "read", digestFixture()); got != legacyReadDigest {
		t.Fatalf("digest of metadata-absent definition = %s; want legacy baseline %s", got, legacyReadDigest)
	}
}

// A metadata-only source change must move the definition digest and the
// catalog revision even though routing, schemas and description are
// unchanged, so the describe revision gate and republish see it.
func TestDefinitionDigestAndRevisionChangeOnMetadataOnly(t *testing.T) {
	base := digestFixture()
	enriched := digestFixture()
	enriched.Metadata = metadataFixture()

	baseDigest := DefinitionDigest("alpha", "read", base)
	enrichedDigest := DefinitionDigest("alpha", "read", enriched)
	if baseDigest == enrichedDigest {
		t.Fatalf("metadata-only change kept the definition digest %s", baseDigest)
	}

	baseBridge := NewBridge()
	if err := baseBridge.Register("alpha", "http://upstream.test", []ToolDefinition{base}); err != nil {
		t.Fatalf("register base: %v", err)
	}
	enrichedBridge := NewBridge()
	if err := enrichedBridge.Register("alpha", "http://upstream.test", []ToolDefinition{enriched}); err != nil {
		t.Fatalf("register enriched: %v", err)
	}
	if baseBridge.Revision() == enrichedBridge.Revision() {
		t.Fatalf("metadata-only change kept the catalog revision %s", baseBridge.Revision())
	}
}

// Clone must deep-copy the raw metadata bytes: the clone shares no backing
// array with its source, so callers can never mutate catalog state.
func TestToolMetadataCloneDeepCopiesRawBytes(t *testing.T) {
	metadata := metadataFixture()
	clone := metadata.Clone()
	if !reflect.DeepEqual(metadata, clone) {
		t.Fatalf("clone = %+v; want an equal copy of %+v", clone, metadata)
	}
	if len(metadata.Icons) > 0 && &metadata.Icons[0] == &clone.Icons[0] {
		t.Fatal("clone shares the icons backing array with its source")
	}
	if len(metadata.Annotations) > 0 && &metadata.Annotations[0] == &clone.Annotations[0] {
		t.Fatal("clone shares the annotations backing array with its source")
	}
	if len(metadata.Meta) > 0 && &metadata.Meta[0] == &clone.Meta[0] {
		t.Fatal("clone shares the _meta backing array with its source")
	}
	zero := ToolMetadata{}
	if !reflect.DeepEqual(zero, zero.Clone()) {
		t.Fatal("cloning zero-value metadata changed it")
	}
}

func TestToolMetadataValidateAcceptsSupportedShape(t *testing.T) {
	if err := metadataFixture().Validate(); err != nil {
		t.Fatalf("supported metadata rejected: %v", err)
	}
	if err := (ToolMetadata{}).Validate(); err != nil {
		t.Fatalf("absent metadata rejected: %v", err)
	}
	titleOnly := ToolMetadata{Title: "Read"}
	if err := titleOnly.Validate(); err != nil {
		t.Fatalf("title-only metadata rejected: %v", err)
	}
}

func TestToolMetadataValidateRejectsUnsupportedShape(t *testing.T) {
	cases := map[string]ToolMetadata{
		"icons object":         {Icons: json.RawMessage(`{"src":"x"}`)},
		"icons invalid json":   {Icons: json.RawMessage(`[`)},
		"annotations array":    {Annotations: json.RawMessage(`[]`)},
		"annotations invalid":  {Annotations: json.RawMessage(`{"title":`)},
		"annotations unknown":  {Annotations: json.RawMessage(`{"readOnlyHint":true,"audience":["user"]}`)},
		"hint not boolean":     {Annotations: json.RawMessage(`{"readOnlyHint":"true"}`)},
		"meta scalar":          {Meta: json.RawMessage(`"audience"`)},
		"meta invalid json":    {Meta: json.RawMessage(`{"a":`)},
		"annotations non-json": {Annotations: json.RawMessage(`not json`)},
		"annotations trailing": {Annotations: json.RawMessage(`{"readOnlyHint":true} trailing`)},
		"meta trailing":        {Meta: json.RawMessage(`{"a":1} garbage`)},
	}
	for label, metadata := range cases {
		if err := metadata.Validate(); err == nil {
			t.Errorf("%s: metadata accepted; want rejection", label)
		}
	}
}

// canonical metadata of a zero value must serialize to the absent form (nil),
// which is what keeps the frozen legacy digest stable.
func TestToolMetadataZeroValueCanonicalizesToAbsent(t *testing.T) {
	var zero ToolMetadata
	canonical := zero.canonical()
	if len(canonical) != 0 {
		t.Fatalf("zero-value canonical metadata = %s; want absent (empty)", canonical)
	}
	enriched := metadataFixture()
	if len(enriched.canonical()) == 0 {
		t.Fatal("non-zero metadata canonicalized to the absent form")
	}
	if !bytes.Contains(enriched.canonical(), []byte(`"title"`)) {
		t.Fatalf("canonical metadata %s lost the title", enriched.canonical())
	}
}
