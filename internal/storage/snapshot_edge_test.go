package storage_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

func TestSnapshotRejectsSchemaVersionAndMalformedDocuments(t *testing.T) {
	s := openStore(t, filepath.Join(t.TempDir(), "state"))
	if err := s.Save(validSnapshot()); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, content string }{{"version", strings.Replace(string(original), `"schema_version":1`, `"schema_version":99`, 1)}, {"unknown field", strings.TrimSuffix(string(original), "}") + `,"mystery":true}`}, {"truncated", `{"schema_version":1`}, {"oversized", strings.Repeat("x", 2<<20)}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(s.Path(), []byte(tc.content), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Load(); !errors.Is(err, storage.ErrCorrupt) {
				t.Fatalf("Load error=%v", err)
			}
		})
	}
}
