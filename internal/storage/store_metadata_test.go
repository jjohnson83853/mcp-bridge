package storage_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

func metadataSnapshot() *storage.Snapshot {
	snapshot := validSnapshot()
	snapshot.SchemaVersion = 2
	tool := snapshot.Tools[0]
	tool.Metadata = &core.ToolMetadata{
		Title:       "List containers",
		Annotations: json.RawMessage(`{"title":"List","readOnlyHint":true,"idempotentHint":true}`),
	}
	snapshot.Tools[0] = tool
	return snapshot
}

// The schema-2 writer persists source metadata and the strict reader returns
// it verbatim; a tampered metadata member fails validation.
func TestSnapshotV2MetadataIntegrity(t *testing.T) {
	directory := t.TempDir()
	store := openStore(t, directory)
	if err := store.Save(metadataSnapshot()); err != nil {
		t.Fatalf("Save schema-2 snapshot: %v", err)
	}

	raw, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("read published snapshot: %v", err)
	}
	var envelope struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.SchemaVersion != 2 {
		t.Fatalf("published schema_version = %d; want 2", envelope.SchemaVersion)
	}
	if !strings.Contains(string(raw), `"metadata"`) {
		t.Fatalf("published snapshot dropped the metadata member: %s", raw)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load schema-2 snapshot: %v", err)
	}
	if len(loaded.Tools) != 1 || loaded.Tools[0].Metadata == nil {
		t.Fatalf("loaded snapshot lost tool metadata: %+v", loaded.Tools)
	}
	want := metadataSnapshot().Tools[0].Metadata
	if got := loaded.Tools[0].Metadata; !jsonMetadataEqual(got, want) {
		t.Fatalf("loaded metadata = %+v; want %+v", got, want)
	}

	// Tamper the metadata member: the strict reader must reject it.
	tampered := strings.Replace(string(raw), `"List containers"`, `"List CONTAINERS"`, 1)
	if tampered == string(raw) {
		t.Fatal("failed to tamper the metadata member")
	}
	if err := os.WriteFile(store.Path(), []byte(tampered), 0o600); err != nil {
		t.Fatalf("stage tampered snapshot: %v", err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("tampered metadata loaded; want rejection")
	}
}

func jsonMetadataEqual(a, b *core.ToolMetadata) bool {
	if a == nil || b == nil {
		return a == b
	}
	encodedA, errA := json.Marshal(a)
	encodedB, errB := json.Marshal(b)
	return errA == nil && errB == nil && string(encodedA) == string(encodedB)
}

// A snapshot whose metadata violates the supported shape is rejected by Save,
// and the rejection leaves the previously published snapshot untouched.
func TestStoreMetadataFailClosed(t *testing.T) {
	directory := t.TempDir()
	store := openStore(t, directory)
	if err := store.Save(metadataSnapshot()); err != nil {
		t.Fatalf("Save valid schema-2 snapshot: %v", err)
	}
	raw, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("read published snapshot: %v", err)
	}

	invalid := metadataSnapshot()
	invalid.Tools[0].Metadata = &core.ToolMetadata{Icons: json.RawMessage(`{"src":"x"}`)}
	if err := store.Save(invalid); err == nil {
		t.Fatal("Save accepted an invalid metadata shape; want rejection")
	}
	after, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("read snapshot after rejected save: %v", err)
	}
	if string(after) != string(raw) {
		t.Fatal("a rejected save disturbed the previously published snapshot")
	}
}

// Upgrading a trusted legacy store: the schema-1 fixture loads, the next
// accepted save publishes schema 2 with metadata, the reader returns it, and
// the schema-2 bytes are opaque to a metadata-less legacy reader.
func TestStoreMetadataAtomicUpgrade(t *testing.T) {
	directory := t.TempDir()
	store := openStore(t, directory)
	legacy, err := os.ReadFile(filepath.Join("testdata", "tool-metadata", "schema1.json"))
	if err != nil {
		t.Fatalf("read frozen schema-1 fixture: %v", err)
	}
	if err := os.WriteFile(store.Path(), legacy, 0o600); err != nil {
		t.Fatalf("stage fixture: %v", err)
	}
	// Load alone must not rewrite the file: it stays schema 1 verbatim.
	if _, err := store.Load(); err != nil {
		t.Fatalf("Load legacy snapshot: %v", err)
	}
	if after, err := os.ReadFile(store.Path()); err != nil || string(after) != string(legacy) {
		t.Fatalf("Load rewrote the legacy snapshot (err=%v)", err)
	}

	if err := store.Save(metadataSnapshot()); err != nil {
		t.Fatalf("Save upgraded snapshot: %v", err)
	}
	upgraded, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("read upgraded snapshot: %v", err)
	}
	if !strings.Contains(string(upgraded), `"schema_version":2`) {
		t.Fatalf("upgraded snapshot did not publish schema 2: %s", upgraded)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load upgraded snapshot: %v", err)
	}
	if loaded.Tools[0].Metadata == nil || loaded.Tools[0].Metadata.Title != "List containers" {
		t.Fatalf("upgraded snapshot lost metadata: %+v", loaded.Tools[0])
	}

	// A legacy reader (this codebase's frozen schema-1 shape) cannot verify
	// schema-2 bytes: the metadata member is an unknown field there.
	var legacyShape struct {
		Tools []struct {
			Metadata json.RawMessage `json:"metadata"`
		} `json:"tools"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(upgraded)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&legacyShape); err == nil {
		t.Fatal("schema-2 bytes decoded under the frozen schema-1 shape; old readers must reject them")
	}
}

// Save must not mutate the caller's snapshot in place: the schema-version
// stamp and the published checksum belong to the owned copy, so the caller
// keeps its original values after the save returns.
func TestStoreSaveDoesNotMutateCallerSnapshot(t *testing.T) {
	directory := t.TempDir()
	store := openStore(t, directory)
	snapshot := metadataSnapshot()
	originalVersion := snapshot.SchemaVersion
	originalChecksum := snapshot.Envelope.SnapshotChecksum

	if err := store.Save(snapshot); err != nil {
		t.Fatalf("Save snapshot: %v", err)
	}
	if snapshot.SchemaVersion != originalVersion {
		t.Fatalf("caller schema_version mutated to %d; want %d", snapshot.SchemaVersion, originalVersion)
	}
	if got := snapshot.Envelope.SnapshotChecksum; got != originalChecksum {
		t.Fatalf("caller snapshot_checksum mutated to %q; want %q", got, originalChecksum)
	}

	// The published document is the current schema and its recorded checksum
	// is the canonical one, even though the caller's struct was untouched.
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load published snapshot: %v", err)
	}
	if loaded.SchemaVersion != 2 {
		t.Fatalf("published schema_version = %d; want 2", loaded.SchemaVersion)
	}
	checksum, err := loaded.Checksum()
	if err != nil {
		t.Fatalf("recompute published checksum: %v", err)
	}
	if loaded.Envelope.SnapshotChecksum != checksum {
		t.Fatalf("published checksum %q; want canonical %q", loaded.Envelope.SnapshotChecksum, checksum)
	}
}
