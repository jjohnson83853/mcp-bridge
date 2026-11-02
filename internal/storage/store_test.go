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

// §4.9: a leftover .writer.lock from a dead process must not wedge the store —
// the advisory kernel lock (flock), not the file's existence, is the writer gate.
func TestOpenRecoversStaleLockfileWhenProcessNotAlive(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Simulate a crashed process: the lock file exists on disk, but no open
	// descriptor holds the advisory lock.
	if err := os.WriteFile(filepath.Join(directory, ".writer.lock"), nil, 0o600); err != nil {
		t.Fatalf("WriteFile stale lock: %v", err)
	}
	store, err := storage.Open(directory)
	if err != nil {
		t.Fatalf("Open with stale lockfile = %v; want success", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
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
	canonicalChecksum, err := snapshot.Checksum()
	if err != nil {
		t.Fatalf("canonical checksum: %v", err)
	}
	if strings.TrimSpace(loaded.Envelope.SnapshotChecksum) == "" || loaded.Envelope.SnapshotChecksum != canonicalChecksum {
		t.Fatalf("snapshot_checksum = %q; want the canonical checksum %q", loaded.Envelope.SnapshotChecksum, canonicalChecksum)
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
		{name: "mismatched endpoint fingerprint", mutate: func(s *storage.Snapshot) { s.Servers[0].EndpointFingerprint = "other" }},
		{name: "invalid catalog status", mutate: func(s *storage.Snapshot) { s.Servers[0].CatalogStatus = "bogus" }},
		{name: "stale status without reason", mutate: func(s *storage.Snapshot) { s.Servers[0].CatalogStatus = storage.StatusStale; s.Servers[0].Reason = "" }},
		{name: "empty raw name", mutate: func(s *storage.Snapshot) { s.Tools[0].RawName = "" }},
		{name: "invalid embedding content hash", mutate: func(s *storage.Snapshot) { s.Tools[0].EmbeddingContentHash = "not-hex" }},
		{name: "duplicate binding", mutate: func(s *storage.Snapshot) { s.Bindings = append(s.Bindings, s.Bindings[0]) }},
		{name: "invalid binding server id", mutate: func(s *storage.Snapshot) { s.Bindings[0].ServerID = "invalid id" }},
		{name: "empty binding endpoint fingerprint", mutate: func(s *storage.Snapshot) { s.Bindings[0].EndpointFingerprint = "" }},
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
func TestPromptSnapshotAndStoreEdgeCases(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	if store.Dir() != dir {
		t.Fatalf("Dir: got %s, want %s", store.Dir(), dir)
	}
	if store.Path() != filepath.Join(dir, "snapshot.json") {
		t.Fatalf("Path: got %s", store.Path())
	}
	if store.PromptPath() != filepath.Join(dir, "prompts.json") {
		t.Fatalf("PromptPath: got %s", store.PromptPath())
	}

	var nilPrompt *storage.PromptSnapshot
	if err := nilPrompt.Validate(); err == nil {
		t.Fatal("expected error validating nil prompt snapshot")
	}
	badVersion := &storage.PromptSnapshot{SchemaVersion: 999}
	if err := badVersion.Validate(); err == nil {
		t.Fatal("expected error validating bad version")
	}
	badServer := &storage.PromptSnapshot{
		SchemaVersion: storage.PromptSchemaVersion,
		Prompts: []storage.Prompt{{ServerID: "BadServer", RawName: "r", PublicName: "p"}},
	}
	if err := badServer.Validate(); err == nil {
		t.Fatal("expected error validating bad server ID")
	}
	badArgs := &storage.PromptSnapshot{
		SchemaVersion: storage.PromptSchemaVersion,
		Prompts: []storage.Prompt{{ServerID: "alpha", RawName: "r", PublicName: "p", Arguments: json.RawMessage("invalid-json")}},
	}
	if err := badArgs.Validate(); err == nil {
		t.Fatal("expected error validating bad arguments")
	}
	dupPublic := &storage.PromptSnapshot{
		SchemaVersion: storage.PromptSchemaVersion,
		Prompts: []storage.Prompt{
			{ServerID: "alpha", RawName: "r1", PublicName: "p"},
			{ServerID: "alpha", RawName: "r2", PublicName: "p"},
		},
	}
	if err := dupPublic.Validate(); err == nil {
		t.Fatal("expected error validating duplicate public name")
	}

	if err := store.SavePrompts(nil); err == nil {
		t.Fatal("expected error saving nil prompts")
	}
	if err := store.SavePrompts(badVersion); err == nil {
		t.Fatal("expected error saving invalid prompts")
	}

	valid := &storage.PromptSnapshot{
		SchemaVersion: storage.PromptSchemaVersion,
		Prompts: []storage.Prompt{
			{ServerID: "alpha", RawName: "r", PublicName: "p", Arguments: json.RawMessage("{\"type\":\"object\"}")},
		},
	}
	if err := store.SavePrompts(valid); err != nil {
		t.Fatalf("SavePrompts: %v", err)
	}
	loaded, err := store.LoadPrompts()
	if err != nil {
		t.Fatalf("LoadPrompts: %v", err)
	}
	if len(loaded.Prompts) != 1 {
		t.Fatalf("got %d prompts, want 1", len(loaded.Prompts))
	}

	dir2 := t.TempDir()
	store2, err := storage.Open(dir2)
	if err != nil {
		t.Fatalf("Open store2: %v", err)
	}
	store2.Close()
	if err := store2.SavePrompts(valid); err == nil {
		t.Fatal("expected error saving on closed store")
	}
}
func TestStoreErrorPathsAndEdgeCases(t *testing.T) {
	f, err := os.CreateTemp("", "store-test-file")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())
	if _, err := storage.Open(filepath.Join(f.Name(), "sub")); err == nil {
		t.Fatal("expected Open error on file path")
	}

	var nilStore *storage.Store
	if err := nilStore.Close(); err != nil {
		t.Fatalf("nil Close: %v", err)
	}
	if err := nilStore.Save(nil); err == nil {
		t.Fatal("expected Save error on nil store")
	}

	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := store.Save(nil); err == nil {
		t.Fatal("expected Save error on nil snapshot")
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if err := store.Save(&storage.Snapshot{}); err == nil {
		t.Fatal("expected Save error on closed store")
	}

	dir2 := t.TempDir()
	store2, err := storage.Open(dir2)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store2.Close()

	if err := os.WriteFile(store2.Path(), []byte("{\"schema_version\": 999}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store2.Load(); err == nil {
		t.Fatal("expected error loading unsupported schema_version")
	}

	valid := validSnapshot()
	data, _ := json.Marshal(valid)
	if err := os.WriteFile(store2.Path(), append(data, []byte(" trailing")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store2.Load(); err == nil {
		t.Fatal("expected error loading snapshot with trailing data")
	}

	store3, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store3.Close()
	if err := os.WriteFile(store3.Path(), []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store3.Save(valid); err == nil {
		t.Fatal("expected Save error on corrupt untrusted snapshot")
	}

	if err := os.WriteFile(store3.PromptPath(), []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	validPrompts := &storage.PromptSnapshot{
		SchemaVersion: storage.PromptSchemaVersion,
		Prompts:       []storage.Prompt{{ServerID: "alpha", RawName: "r", PublicName: "p"}},
	}
	if err := store3.SavePrompts(validPrompts); err == nil {
		t.Fatal("expected SavePrompts error on corrupt untrusted prompts")
	}

	if _, err := store3.LoadPrompts(); err == nil {
		t.Fatal("expected LoadPrompts error on corrupt json")
	}
	pData, _ := json.Marshal(validPrompts)
	if err := os.WriteFile(store3.PromptPath(), append(pData, []byte(" trailing")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store3.LoadPrompts(); err == nil {
		t.Fatal("expected LoadPrompts error on trailing data")
	}
}
func TestSnapshotBindingAllowed(t *testing.T) {
	snap := validSnapshot()
	if !snap.BindingAllowed("portainer", snap.Bindings[0].EndpointFingerprint) {
		t.Fatal("expected binding to be allowed")
	}
	if snap.BindingAllowed("portainer", "mismatched") {
		t.Fatal("expected mismatched fingerprint to be rejected")
	}
	snap.Bindings[0].Retired = true
	if snap.BindingAllowed("portainer", snap.Bindings[0].EndpointFingerprint) {
		t.Fatal("expected retired binding to be rejected")
	}
}
func TestSnapshotValidateEdges(t *testing.T) {
	snap := validSnapshot()
	snap.Servers = append(snap.Servers, storage.Server{ServerID: "orphan", EndpointFingerprint: fingerprintValue, CatalogStatus: storage.StatusNone})
	if err := snap.Validate(); err == nil {
		t.Fatal("expected error on server without binding")
	}

	snap = validSnapshot()
	snap.Embeddings[0].ContentHash = "not-a-hex-digest"
	if err := snap.Validate(); err == nil {
		t.Fatal("expected error on embedding invalid content hash")
	}

	snap = validSnapshot()
	snap.Embeddings[0].FormatVersion = ""
	if err := snap.Validate(); err == nil {
		t.Fatal("expected error on embedding missing format version")
	}
}
