package storage_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

func TestStoreDirectoryAndOpenFailures(t *testing.T) {
	root := t.TempDir()
	asFile := filepath.Join(root, "not-dir")
	if err := os.WriteFile(asFile, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Open(asFile); err == nil {
		t.Fatal("Open accepted regular file as directory")
	}
	s, err := storage.Open(filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Dir() != filepath.Join(root, "state") {
		t.Fatalf("Dir=%q", s.Dir())
	}
	if _, err := storage.Open(s.Dir()); !errors.Is(err, storage.ErrWriterLocked) {
		t.Fatalf("second writer error=%v", err)
	}
}

func TestStoreDistinguishesMissingAndCorruptAndChecksChecksum(t *testing.T) {
	s := openStore(t, filepath.Join(t.TempDir(), "state"))
	if _, err := s.Load(); !errors.Is(err, storage.ErrNoSnapshot) {
		t.Fatalf("missing error=%v", err)
	}
	if err := s.Save(validSnapshot()); err != nil {
		t.Fatal(err)
	}
	path := s.Path()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(raw), "list running containers", "different description", 1)
	if tampered == string(raw) {
		t.Fatal("fixture description not present")
	}
	if err := os.WriteFile(path, []byte(tampered), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); !errors.Is(err, storage.ErrCorrupt) {
		t.Fatalf("tampered error=%v", err)
	}
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); !errors.Is(err, storage.ErrCorrupt) {
		t.Fatalf("malformed error=%v", err)
	}
}

func TestStoreSavePublishBoundaries(t *testing.T) {
	s := openStore(t, filepath.Join(t.TempDir(), "state"))
	if err := s.Save(nil); err == nil {
		t.Fatal("nil save succeeded")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(validSnapshot()); err == nil {
		t.Fatal("closed store save succeeded")
	}
	s2 := openStore(t, filepath.Join(t.TempDir(), "state"))
	if err := os.Mkdir(s2.Path(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := s2.Save(validSnapshot()); err == nil {
		t.Fatal("directory snapshot replacement succeeded")
	}
}
func TestRecoverNegativeOracles(t *testing.T) {
	store := openStore(t, filepath.Join(t.TempDir(), "state"))
	defer store.Close()

	if err := os.WriteFile(store.Path(), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); !errors.Is(err, storage.ErrCorrupt) {
		t.Fatalf("Load error=%v; want ErrCorrupt", err)
	}
	if err := store.Save(validSnapshot()); err == nil ||
		!strings.Contains(err.Error(), "storage: refusing to overwrite untrusted snapshot") {
		t.Fatalf("Save error=%v; want untrusted-snapshot refusal", err)
	}

	if err := os.WriteFile(store.PromptPath(), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadPrompts(); !errors.Is(err, storage.ErrCorrupt) {
		t.Fatalf("LoadPrompts error=%v; want ErrCorrupt", err)
	}
	prompts := &storage.PromptSnapshot{
		SchemaVersion: storage.PromptSchemaVersion,
		Prompts: []storage.Prompt{{
			ServerID:   "portainer",
			RawName:    "hello",
			PublicName: "p_hello",
		}},
	}
	if checksum, err := prompts.ChecksumValue(); err != nil {
		t.Fatal(err)
	} else {
		prompts.Checksum = checksum
	}
	if err := store.SavePrompts(prompts); err == nil ||
		!strings.Contains(err.Error(), "storage: refusing to overwrite untrusted prompt snapshot") {
		t.Fatalf("SavePrompts error=%v; want untrusted-prompt refusal", err)
	}

	for _, pattern := range []string{
		filepath.Join(store.Dir(), "snapshot.corrupt-*.json"),
		filepath.Join(store.Dir(), "prompts.corrupt-*.json"),
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 0 {
			t.Errorf("non-recovery operation created quarantine files %v", matches)
		}
	}
}

func TestRecoverRejectedMovesSymlinkEntryAndPreservesTarget(t *testing.T) {
	store := openStore(t, filepath.Join(t.TempDir(), "state"))
	defer store.Close()

	target := filepath.Join(store.Dir(), "rejected-target.json")
	original := []byte("{")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(target), store.Path()); err != nil {
		t.Fatal(err)
	}

	result, err := store.RecoverRejected()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Recoveries) != 1 {
		t.Fatalf("recoveries=%+v; want exactly one recovery", result.Recoveries)
	}
	recovery := result.Recoveries[0]
	if recovery.Family != storage.FamilySnapshot || recovery.Reason != storage.ReasonCorruptJSON {
		t.Fatalf("recovery=%+v; want snapshot corrupt_json", recovery)
	}
	if got, err := os.Readlink(recovery.QuarantinePath); err != nil || got != filepath.Base(target) {
		t.Fatalf("quarantined entry target=%q err=%v", got, err)
	}
	if _, err := os.Lstat(store.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("configured symlink entry remains: %v", err)
	}
	if got, err := os.ReadFile(target); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("target bytes=%q err=%v", got, err)
	}
	if got, err := os.ReadFile(recovery.QuarantinePath); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("quarantine read-through=%q err=%v", got, err)
	}
}
