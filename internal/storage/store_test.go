package storage_test

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	// T-07 durable storage RED gate (r8 plan §4.9): exclusive single writer,
	// private 0700/0600 layout, snapshot.json schema 1, canonical checksum,
	// atomic temp+fsync+rename publication, corrupt snapshots ignored without
	// destructive overwrite, and bounded, identity-validated load.
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

const (
	revisionValue    = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	fingerprintValue = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	configValue      = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
)

func validSnapshot() *storage.Snapshot {
	tool := storage.Tool{
		ServerID:             "portainer",
		RawName:              "list_containers",
		PublicName:           "t_" + strings.Repeat("a", 52),
		Description:          "list running containers",
		DefinitionHash:       strings.Repeat("d", 64),
		EmbeddingContentHash: strings.Repeat("e", 64),
		InputSchema:          json.RawMessage(`{"type":"object"}`),
		OutputSchema:         json.RawMessage(`{"type":"object"}`),
	}
	return &storage.Snapshot{
		Envelope: storage.Envelope{
			SchemaVersion:           storage.SchemaVersion,
			GenerationSequence:      7,
			CreatedAt:               time.Date(2026, 10, 9, 5, 4, 3, 123456789, time.UTC),
			ConfigPublicFingerprint: configValue,
			CatalogRevision:         revisionValue,
		},
		Bindings: []storage.Binding{
			{ServerID: "portainer", EndpointFingerprint: fingerprintValue},
		},
		Servers: []storage.Server{
			{
				ServerID:            "portainer",
				EndpointFingerprint: fingerprintValue,
				NegotiatedVersion:   "2025-06-18",
				LastSuccessAt:       time.Date(2026, 10, 9, 5, 0, 0, 0, time.UTC),
				LastAttemptAt:       time.Date(2026, 10, 9, 5, 0, 0, 0, time.UTC),
				CatalogStatus:       "fresh",
			},
		},
		Tools: []storage.Tool{tool},
		Embeddings: []storage.Embedding{
			{
				ToolIdentity:        storage.ToolIdentity("portainer", "list_containers"),
				ContentHash:         strings.Repeat("e", 64),
				ModelIdentityDigest: strings.Repeat("f", 64),
				Dimensions:          1024,
				FormatVersion:       "v1",
				Vector:              finiteVector(1024, 1),
			},
		},
	}
}

func finiteVector(dimensions int, seed float64) []float64 {
	vector := make([]float64, dimensions)
	for i := range vector {
		vector[i] = seed + float64(i)/1000
	}
	return vector
}

func openStore(t *testing.T, directory string) *storage.Store {
	t.Helper()
	store, err := storage.Open(directory)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// §4.9: private directory 0700, exclusive single writer.
func TestOpenCreatesPrivateLayoutAndExclusiveWriter(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "state")
	store := openStore(t, directory)
	info, err := os.Stat(directory)
	if err != nil {
		t.Fatalf("stat store directory: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("directory mode = %o; want 700", perm)
	}
	if _, err := storage.Open(directory); !errors.Is(err, storage.ErrWriterLocked) {
		t.Fatalf("second writer error = %v; want ErrWriterLocked", err)
	}
	if _, err := store.Load(); !errors.Is(err, storage.ErrNoSnapshot) {
		t.Fatalf("fresh store Load = %v; want ErrNoSnapshot", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reopened, err := storage.Open(directory)
	if err != nil {
		t.Fatalf("reopen after Close: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("Close reopened: %v", err)
	}
}

// §4.9: complete immutable snapshot, files 0600, canonical checksum, UTC
// RFC3339Nano timestamps, and a re-save that reproduces identical bytes.
func TestSaveLoadRoundTripIsAtomicAndPrivate(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "state")
	store := openStore(t, directory)
	snapshot := validSnapshot()
	if err := store.Save(snapshot); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path := filepath.Join(directory, "snapshot.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat snapshot: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("snapshot mode = %o; want 600", perm)
	}
	emptied, err := filepath.Glob(filepath.Join(directory, "*.tmp"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(emptied) != 0 {
		t.Fatalf("temporary files left behind: %v", emptied)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Envelope.SchemaVersion != storage.SchemaVersion {
		t.Fatalf("schema_version = %d", loaded.Envelope.SchemaVersion)
	}
	if loaded.Envelope.GenerationSequence != snapshot.Envelope.GenerationSequence {
		t.Fatalf("generation = %d; want %d", loaded.Envelope.GenerationSequence, snapshot.Envelope.GenerationSequence)
	}
	if !loaded.Envelope.CreatedAt.Equal(snapshot.Envelope.CreatedAt) || loaded.Envelope.CreatedAt.Location() != time.UTC {
		t.Fatalf("created_at = %v; want UTC RFC3339Nano %v", loaded.Envelope.CreatedAt, snapshot.Envelope.CreatedAt)
	}
	if strings.TrimSpace(loaded.Envelope.SnapshotChecksum) == "" || loaded.Envelope.SnapshotChecksum != snapshot.Envelope.SnapshotChecksum {
		t.Fatalf("snapshot_checksum = %q; want the canonical checksum", loaded.Envelope.SnapshotChecksum)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	response, err := store.Load()
	if err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if response.Envelope.SnapshotChecksum != loaded.Envelope.SnapshotChecksum {
		t.Fatal("load is not deterministic")
	}
	if err := store.Save(loaded); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read resaved snapshot: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("re-save is not canonical:\n%s\n%s", first, second)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		if name != "snapshot.json" && name != ".writer.lock" {
			t.Fatalf("unexpected store entry %q in %v", name, names)
		}
	}
}

// §4.9: a corrupt snapshot and a leftover partial file are ignored, never
// adopted, and never destructively overwritten.
func TestLoadIgnoresCorruptAndTempFiles(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "state")
	store := openStore(t, directory)
	path := filepath.Join(directory, "snapshot.json")
	corrupt := []byte("{ this is not a snapshot")
	if err := os.WriteFile(path, corrupt, 0o600); err != nil {
		t.Fatalf("write corrupt snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, storage.ErrCorrupt) {
		t.Fatalf("Load corrupt = %v; want ErrCorrupt", err)
	}
	temp := filepath.Join(directory, ".snapshot.partial.tmp")
	if err := os.WriteFile(temp, []byte(`{"schema_version":1}`), 0o600); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, storage.ErrCorrupt) {
		t.Fatalf("Load with temp present = %v; want ErrCorrupt", err)
	}
	preserved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if string(preserved) != string(corrupt) {
		t.Fatal("a rejected snapshot was overwritten")
	}
	if _, err := os.Stat(temp); err != nil {
		t.Fatalf("temporary file was removed: %v", err)
	}
}

// §4.9: a corrupt published snapshot is ignored for reuse but is never
// destructively overwritten, so publication is refused until an operator
// resolves the file.
func TestSaveRefusesToOverwriteCorruptSnapshot(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "state")
	store := openStore(t, directory)
	path := filepath.Join(directory, "snapshot.json")
	corrupt := []byte("{ this is not a snapshot")
	if err := os.WriteFile(path, corrupt, 0o600); err != nil {
		t.Fatalf("write corrupt snapshot: %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, storage.ErrCorrupt) {
		t.Fatalf("Load corrupt = %v; want ErrCorrupt", err)
	}
	if err := store.Save(validSnapshot()); !errors.Is(err, storage.ErrCorrupt) {
		t.Fatalf("Save over a corrupt snapshot = %v; want ErrCorrupt", err)
	}
	preserved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if string(preserved) != string(corrupt) {
		t.Fatal("Save overwrote the corrupt snapshot")
	}

	// A store whose published snapshot is valid keeps publishing, and a first
	// save into an empty state directory still works.
	healthy := openStore(t, filepath.Join(t.TempDir(), "healthy"))
	if err := healthy.Save(validSnapshot()); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	successor := validSnapshot()
	successor.Envelope.GenerationSequence = 8
	if err := healthy.Save(successor); err != nil {
		t.Fatalf("Save over a valid snapshot: %v", err)
	}
	loaded, err := healthy.Load()
	if err != nil {
		t.Fatalf("Load after replacing a valid snapshot: %v", err)
	}
	if loaded.Envelope.GenerationSequence != 8 {
		t.Fatalf("generation = %d; want 8", loaded.Envelope.GenerationSequence)
	}
}

// §4.9: a failed publication preserves the previous trusted snapshot.
func TestFailedSavePreservesPreviousSnapshot(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "state")
	store := openStore(t, directory)
	if err := store.Save(validSnapshot()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path := filepath.Join(directory, "snapshot.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}

	invalid := validSnapshot()
	invalid.Envelope.GenerationSequence = 8
	invalid.Tools = append(invalid.Tools, invalid.Tools[0]) // duplicate public name
	if err := store.Save(invalid); err == nil {
		t.Fatal("Save accepted a duplicate public name")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot after failed save: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("failed save modified the published snapshot")
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load after failed save: %v", err)
	}
	if loaded.Envelope.GenerationSequence != 7 {
		t.Fatalf("generation = %d; want the previous 7", loaded.Envelope.GenerationSequence)
	}

	fresh := filepath.Join(t.TempDir(), "fresh")
	freshStore := openStore(t, fresh)
	if err := freshStore.Save(invalid); err == nil {
		t.Fatal("Save accepted an invalid snapshot")
	}
	if _, err := os.Stat(filepath.Join(fresh, "snapshot.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid snapshot was published: %v", err)
	}
}

// §4.9: load validates version, checksum and unknown fields, and preserves the
// rejected file.
func TestLoadRejectsTamperingAndUnknownFields(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "state")
	store := openStore(t, directory)
	if err := store.Save(validSnapshot()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path := filepath.Join(directory, "snapshot.json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}

	rewrite := func(mutate func(document map[string]any)) []byte {
		var document map[string]any
		if err := json.Unmarshal(original, &document); err != nil {
			t.Fatalf("decode snapshot: %v", err)
		}
		mutate(document)
		encoded, err := json.Marshal(document)
		if err != nil {
			t.Fatalf("encode snapshot: %v", err)
		}
		if err := os.WriteFile(path, encoded, 0o600); err != nil {
			t.Fatalf("write snapshot: %v", err)
		}
		return encoded
	}

	// Content tampering that leaves the stored checksum stale.
	tampered := rewrite(func(document map[string]any) {
		servers, _ := document["servers"].([]any)
		row, _ := servers[0].(map[string]any)
		row["reason"] = "invented"
	})
	if _, err := store.Load(); !errors.Is(err, storage.ErrCorrupt) {
		t.Fatalf("Load tampered = %v; want ErrCorrupt", err)
	}
	if current, _ := os.ReadFile(path); string(current) != string(tampered) {
		t.Fatal("rejected tampered snapshot was modified")
	}

	// Version tampering.
	versioned := rewrite(func(document map[string]any) {
		document["schema_version"] = float64(storage.SchemaVersion + 1)
	})
	if _, err := store.Load(); !errors.Is(err, storage.ErrCorrupt) {
		t.Fatalf("Load future schema = %v; want ErrCorrupt", err)
	}
	if current, _ := os.ReadFile(path); string(current) != string(versioned) {
		t.Fatal("rejected future-schema snapshot was modified")
	}

	// An unknown key (a legacy namespace column) is never accepted.
	namespaced := rewrite(func(document map[string]any) {
		document["schema_version"] = float64(storage.SchemaVersion)
		document["namespace"] = "portainer"
	})
	if _, err := store.Load(); !errors.Is(err, storage.ErrCorrupt) {
		t.Fatalf("Load with unknown field = %v; want ErrCorrupt", err)
	}
	if current, _ := os.ReadFile(path); string(current) != string(namespaced) {
		t.Fatal("rejected unknown-field snapshot was modified")
	}
}

// §4.9: bounds, version, identity and vector validation.
func TestValidateRejectsBoundsAndIdentityConflicts(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*storage.Snapshot)
	}{
		{name: "schema version", mutate: func(s *storage.Snapshot) { s.Envelope.SchemaVersion = storage.SchemaVersion + 1 }},
		{name: "empty revision", mutate: func(s *storage.Snapshot) { s.Envelope.CatalogRevision = "" }},
		{name: "short revision", mutate: func(s *storage.Snapshot) { s.Envelope.CatalogRevision = "abc" }},
		{name: "empty fingerprint", mutate: func(s *storage.Snapshot) { s.Envelope.ConfigPublicFingerprint = "" }},
		{name: "zero created_at", mutate: func(s *storage.Snapshot) { s.Envelope.CreatedAt = time.Time{} }},
		{name: "public name pattern", mutate: func(s *storage.Snapshot) { s.Tools[0].PublicName = "t_" + strings.Repeat("A", 52) }},
		{name: "duplicate public name", mutate: func(s *storage.Snapshot) {
			duplicate := s.Tools[0]
			duplicate.RawName = "other"
			s.Tools = append(s.Tools, duplicate)
		}},
		{name: "duplicate server tool", mutate: func(s *storage.Snapshot) { s.Tools = append(s.Tools, s.Tools[0]) }},
		{name: "empty definition hash", mutate: func(s *storage.Snapshot) { s.Tools[0].DefinitionHash = "" }},
		{name: "unresolvable schema", mutate: func(s *storage.Snapshot) { s.Tools[0].InputSchema = json.RawMessage(`{"type":5}`) }},
		{name: "server id pattern", mutate: func(s *storage.Snapshot) {
			s.Servers[0].ServerID = "Portainer"
			s.Tools[0].ServerID = "Portainer"
			s.Bindings[0].ServerID = "Portainer"
			s.Embeddings[0].ToolIdentity = storage.ToolIdentity("Portainer", "list_containers")
		}},
		{name: "zero dimensions", mutate: func(s *storage.Snapshot) { s.Embeddings[0].Dimensions = 0 }},
		{name: "vector length", mutate: func(s *storage.Snapshot) { s.Embeddings[0].Vector = finiteVector(8, 1) }},
		{name: "non finite vector", mutate: func(s *storage.Snapshot) { s.Embeddings[0].Vector[3] = math.NaN() }},
		{name: "zero vector", mutate: func(s *storage.Snapshot) { s.Embeddings[0].Vector = make([]float64, 1024) }},
		{name: "orphan embedding", mutate: func(s *storage.Snapshot) {
			s.Embeddings[0].ToolIdentity = storage.ToolIdentity("portainer", "unknown")
		}},
		{name: "retired binding with live server", mutate: func(s *storage.Snapshot) { s.Bindings[0].Retired = true }},
		{name: "orphan server row", mutate: func(s *storage.Snapshot) { s.Bindings = nil }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			snapshot := validSnapshot()
			testCase.mutate(snapshot)
			if err := snapshot.Validate(); err == nil {
				t.Fatalf("Validate accepted %s", testCase.name)
			}
		})
	}
	if err := validSnapshot().Validate(); err != nil {
		t.Fatalf("Validate rejected the valid snapshot: %v", err)
	}
}

// §4.9: tombstones reject a rebind, removed servers stay excluded.
func TestTombstoneRejectsRebind(t *testing.T) {
	snapshot := validSnapshot()
	snapshot.Bindings = append(snapshot.Bindings, storage.Binding{
		ServerID:            "retired-server",
		EndpointFingerprint: strings.Repeat("9", 64),
		Retired:             true,
	})
	if !snapshot.BindingAllowed("portainer", fingerprintValue) {
		t.Fatal("active binding was refused")
	}
	if snapshot.BindingAllowed("retired-server", strings.Repeat("9", 64)) {
		t.Fatal("retired endpoint was allowed to rebind")
	}
	if snapshot.BindingAllowed("retired-server", strings.Repeat("8", 64)) {
		t.Fatal("a retired server was allowed to rebind on a new endpoint")
	}
	if !snapshot.BindingAllowed("greenfield", strings.Repeat("7", 64)) {
		t.Fatal("a server with no binding was refused its first bind")
	}
}
