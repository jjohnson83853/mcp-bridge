package storage_test

import (
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
	"strings"
	"testing"
)

func TestIndexToolsReplacesAndRetainsOnlyCurrentEmbeddings(t *testing.T) {
	a := validSnapshot()
	old := a.Tools[0]
	other := old
	other.RawName = "other"
	other.PublicName = "t_" + strings.Repeat("b", 52)
	a.Tools = append(a.Tools, other)
	a.Embeddings = append(a.Embeddings, storage.Embedding{ToolIdentity: storage.ToolIdentity("portainer", "other"), ContentHash: other.EmbeddingContentHash, ModelIdentityDigest: strings.Repeat("f", 64), Dimensions: 1024, FormatVersion: "v1", Vector: finiteVector(1024, 3)})
	next, err := storage.IndexTools(a, "portainer", []storage.Tool{old})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Tools) != 1 || len(next.Embeddings) != 1 {
		t.Fatalf("tools=%d embeddings=%d", len(next.Tools), len(next.Embeddings))
	}
	if len(a.Tools) != 2 {
		t.Fatal("input snapshot mutated")
	}
	if _, err := storage.IndexTools(nil, "x", nil); err == nil {
		t.Fatal("nil snapshot accepted")
	}
	if _, err := storage.IndexTools(a, "portainer", []storage.Tool{{ServerID: "other"}}); err == nil {
		t.Fatal("foreign row accepted")
	}
	changed := old
	changed.EmbeddingContentHash = strings.Repeat("9", 64)
	changed.DefinitionHash = strings.Repeat("8", 64)
	next, err = storage.IndexTools(a, "portainer", []storage.Tool{changed})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range next.Embeddings {
		if e.ToolIdentity == storage.ToolIdentity("portainer", old.RawName) {
			t.Fatal("stale embedding retained")
		}
	}
}
