package storage_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

func TestPromptStoreIndependentRecoveryAndRollback(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	toolSnapshot := validSnapshot()
	if err := store.Save(toolSnapshot); err != nil {
		t.Fatal(err)
	}
	toolBytes, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	loadedTool, err := store.Load()
	if err != nil {
		t.Fatalf("old tool-only snapshot load error = %v", err)
	}
	if loadedTool.CatalogRevision != toolSnapshot.CatalogRevision {
		t.Fatalf("catalog revision = %q, want %q", loadedTool.CatalogRevision, toolSnapshot.CatalogRevision)
	}
	promptSet := &storage.PromptSnapshot{SchemaVersion: storage.PromptSchemaVersion, Prompts: []storage.Prompt{{ServerID: "alpha", RawName: "raw", PublicName: "p_opaque", Title: "Prompt"}}}
	if err := store.SavePrompts(promptSet); err != nil {
		t.Fatalf("SavePrompts error = %v", err)
	}
	loadedPrompts, err := store.LoadPrompts()
	if err != nil {
		t.Fatalf("LoadPrompts error = %v", err)
	}
	if len(loadedPrompts.Prompts) != 1 || loadedPrompts.Prompts[0].RawName != "raw" {
		t.Fatalf("prompts = %#v, want saved prompt", loadedPrompts.Prompts)
	}
	cases := []struct {
		name string
		data []byte
	}{
		{name: "corrupt", data: []byte("not-json")},
		{name: "checksum-invalid", data: []byte("{\"schema_version\":1,\"prompts\":[],\"checksum\":\"bad\"}")},
		{name: "future-version", data: []byte("{\"schema_version\":999,\"prompts\":[],\"checksum\":\"bad\"}")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(store.PromptPath(), tc.data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.LoadPrompts(); err == nil {
				t.Fatalf("LoadPrompts error = %v, want corrupt prompt data rejected", err)
			}
			got, err := store.Load()
			if err != nil {
				t.Fatalf("tool Load error = %v, want prompt corruption isolated", err)
			}
			gotBytes, err := os.ReadFile(store.Path())
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(gotBytes, toolBytes) {
				t.Fatalf("tool snapshot bytes changed after %s prompt object", tc.name)
			}
			if got.CatalogRevision != toolSnapshot.CatalogRevision {
				t.Fatalf("tool catalog revision = %q, want active %q", got.CatalogRevision, toolSnapshot.CatalogRevision)
			}
		})
	}
}
