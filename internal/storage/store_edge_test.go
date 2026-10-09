package storage_test

import (
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
