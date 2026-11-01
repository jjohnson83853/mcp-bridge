package storage_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// frozenSchema1Checksum is the checksum of testdata/tool-metadata/schema1.json,
// captured from the exact pre-ticket-03 validSnapshot fixture at HEAD eb879ef
// before any schema edit. The fixture is immutable.
const frozenSchema1Checksum = "d4413a8209ab6c34bbfdcdea2f95aba42cbe47313a88fa812fc0cb1176448f90"

// The frozen schema-1 fixture must load verbatim with its recorded checksum
// and stay free of metadata: legacy durable state keeps its exact identity.
func TestSnapshotV1FixtureLoadsVerbatim(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "state")
	store := openStore(t, directory)
	raw, err := os.ReadFile(filepath.Join("testdata", "tool-metadata", "schema1.json"))
	if err != nil {
		t.Fatalf("read frozen schema-1 fixture: %v", err)
	}
	if err := os.WriteFile(store.Path(), raw, 0o600); err != nil {
		t.Fatalf("stage fixture: %v", err)
	}

	snapshot, err := store.Load()
	if err != nil {
		t.Fatalf("Load frozen schema-1 fixture: %v", err)
	}
	if snapshot.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d; want 1", snapshot.SchemaVersion)
	}
	if got := snapshot.Envelope.SnapshotChecksum; got != frozenSchema1Checksum {
		t.Fatalf("fixture checksum = %s; want frozen %s", got, frozenSchema1Checksum)
	}
	if len(snapshot.Tools) != 1 || snapshot.Tools[0].RawName != "list_containers" {
		t.Fatalf("fixture tools changed: %+v", snapshot.Tools)
	}
	// The re-serialized schema-1 bytes must stay byte-stable: a checksum
	// recomputation over the canonical form matches the frozen digest, so no
	// legacy identity eroded during the round trip.
	checksum, err := snapshot.Checksum()
	if err != nil {
		t.Fatalf("recompute checksum: %v", err)
	}
	if checksum != frozenSchema1Checksum {
		t.Fatalf("roundtrip checksum = %s; want frozen %s", checksum, frozenSchema1Checksum)
	}
}

// The reader must refuse a schema-1 document that carries a metadata member:
// legacy bytes are verified only through the frozen schema-1 representation.
func TestSnapshotV1FixtureRejectsMetadataMember(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "state")
	store := openStore(t, directory)
	raw, err := os.ReadFile(filepath.Join("testdata", "tool-metadata", "schema1.json"))
	if err != nil {
		t.Fatalf("read frozen schema-1 fixture: %v", err)
	}
	legacy := string(raw)
	// Any metadata member appended to a schema-1 tool row is an unknown field
	// to the strict schema-1 representation.
	tampered := strings.Replace(legacy, `"definition_hash"`, `"metadata":{"title":"x"},"definition_hash"`, 1)
	if tampered == legacy {
		t.Fatal("failed to tamper the schema-1 fixture")
	}
	if err := os.WriteFile(store.Path(), []byte(tampered), 0o600); err != nil {
		t.Fatalf("stage tampered fixture: %v", err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("schema-1 document with a metadata member loaded; want rejection")
	}
}

// The strict schema-1 reader must reject a document carrying appended
// non-JSON garbage: only a clean EOF after the single JSON value is accepted,
// never a mid-parse syntax error that could silently fail open.
func TestSnapshotV1RejectsTrailingGarbage(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "state")
	store := openStore(t, directory)
	raw, err := os.ReadFile(filepath.Join("testdata", "tool-metadata", "schema1.json"))
	if err != nil {
		t.Fatalf("read frozen schema-1 fixture: %v", err)
	}
	garbage := append(append([]byte{}, raw...), []byte("\nnot-json")...)
	if err := os.WriteFile(store.Path(), garbage, 0o600); err != nil {
		t.Fatalf("stage garbage-appended fixture: %v", err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("schema-1 document with trailing garbage loaded; want rejection")
	}
}
