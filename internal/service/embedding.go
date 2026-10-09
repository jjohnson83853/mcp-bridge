package service

import (
	"sort"

	"github.com/jjohnson83853/mcp-bridge/internal/search"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

// fillVectors completes the indexed snapshot with the vectors of the tools that
// have no current one, reusing every stored vector whose tool identity, content
// hash, and model digest still match this run (plan §4.8/§4.9; AC-13).
//
// A failed or unusable embedding pass degrades to that reuse-only set: the
// generation still publishes its exact catalog, and the search layer reports
// the absent vectors instead of serving a vector computed for other text or
// another model.
func (in buildInput) fillVectors(snapshot *storage.Snapshot) *storage.Snapshot {
	documents := missingVectors(snapshot, in.deps.ModelIdentityDigest)
	if in.deps.Embed == nil || len(documents) == 0 {
		return snapshot
	}
	embedded, err := in.deps.Embed(documents)
	if err != nil || len(embedded) == 0 {
		return snapshot
	}
	return mergeVectors(snapshot, in.deps.ModelIdentityDigest, embedded)
}

// missingVectors lists the indexed tools whose stored vector is not current for
// the model identity of this run, in stable public-name order.
func missingVectors(snapshot *storage.Snapshot, digest string) []search.Document {
	current := make(map[string]storage.Embedding, len(snapshot.Embeddings))
	for _, row := range snapshot.Embeddings {
		current[row.ToolIdentity] = row
	}
	documents := make([]search.Document, 0, len(snapshot.Tools))
	for _, tool := range snapshot.Tools {
		if row, ok := current[storage.ToolIdentity(tool.ServerID, tool.RawName)]; ok &&
			row.ContentHash == tool.EmbeddingContentHash &&
			row.ModelIdentityDigest == digest &&
			row.Dimensions > 0 && len(row.Vector) == row.Dimensions {
			continue
		}
		documents = append(documents, search.Document{
			ServerID:    tool.ServerID,
			RawName:     tool.RawName,
			PublicName:  tool.PublicName,
			Description: tool.Description,
		})
	}
	return documents
}

// mergeVectors attaches one embedding pass to the indexed rows. Every returned
// row must describe an indexed tool of this snapshot, for the same content hash
// and model digest, with a self-consistent vector in one shared dimension;
// otherwise the pass is refused whole and the reuse-only set stands. The merged
// snapshot must still validate, so a bad pass can never be persisted or served.
func mergeVectors(snapshot *storage.Snapshot, digest string, embedded []storage.Embedding) *storage.Snapshot {
	indexed := make(map[string]string, len(snapshot.Tools))
	for _, tool := range snapshot.Tools {
		indexed[storage.ToolIdentity(tool.ServerID, tool.RawName)] = tool.EmbeddingContentHash
	}
	merged := make(map[string]storage.Embedding, len(snapshot.Embeddings)+len(embedded))
	for _, row := range snapshot.Embeddings {
		merged[row.ToolIdentity] = row
	}
	dimensions := 0
	for _, row := range embedded {
		content, known := indexed[row.ToolIdentity]
		if !known || row.ContentHash != content || row.ModelIdentityDigest != digest {
			return snapshot
		}
		if row.Dimensions <= 0 || len(row.Vector) != row.Dimensions {
			return snapshot
		}
		if dimensions == 0 {
			dimensions = row.Dimensions
		} else if row.Dimensions != dimensions {
			return snapshot
		}
		merged[row.ToolIdentity] = row
	}
	rows := make([]storage.Embedding, 0, len(merged))
	for _, row := range merged {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ToolIdentity < rows[j].ToolIdentity })

	next := *snapshot
	next.Embeddings = rows
	if err := next.Validate(); err != nil {
		return snapshot
	}
	return &next
}
