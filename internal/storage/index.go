package storage

import (
	"fmt"
	"sort"
)

// IndexTools replaces one server's indexed tool rows with the exact definitions
// of the current cycle and keeps only the embedding records whose content
// identity is still current (plan §4.8/§4.9). A vector retained for a replaced
// row must match that row's embedding content hash, so a definition whose
// embedded text changed - and a tool the server no longer exposes - loses its
// vector here and is re-embedded before it is ever served. Rows and vectors of
// other servers are untouched, as are bindings and server rows, and the
// returned snapshot is canonical and validated. The input snapshot is never
// mutated.
func IndexTools(s *Snapshot, serverID string, rows []Tool) (*Snapshot, error) {
	if s == nil {
		return nil, fmt.Errorf("storage: no snapshot to index for server %q", serverID)
	}
	for i, row := range rows {
		if row.ServerID != serverID {
			return nil, fmt.Errorf("storage: indexed row %d belongs to server %q, not %q", i, row.ServerID, serverID)
		}
	}

	tools := make([]Tool, 0, len(s.Tools)+len(rows))
	for _, row := range s.Tools {
		if row.ServerID != serverID {
			tools = append(tools, row)
		}
	}
	tools = append(tools, rows...)

	content := make(map[string]string, len(rows))
	replaced := make(map[string]bool, len(rows))
	for _, row := range rows {
		identity := ToolIdentity(row.ServerID, row.RawName)
		content[identity] = row.EmbeddingContentHash
		replaced[identity] = true
	}
	for _, row := range s.Tools {
		// A row this server no longer exposes is replaced too, so its vector
		// must not survive as a dangling record.
		if row.ServerID == serverID {
			replaced[ToolIdentity(row.ServerID, row.RawName)] = true
		}
	}
	embeddings := make([]Embedding, 0, len(s.Embeddings))
	for _, row := range s.Embeddings {
		if replaced[row.ToolIdentity] && row.ContentHash != content[row.ToolIdentity] {
			// The text of this row changed: its vector is no longer current and
			// is recomputed before it can be served again.
			continue
		}
		embeddings = append(embeddings, row)
	}
	// Canonical order, as canonicalize produces it; the slices of the input
	// snapshot are never sorted in place.
	sort.Slice(tools, func(i, j int) bool { return tools[i].PublicName < tools[j].PublicName })
	sort.Slice(embeddings, func(i, j int) bool { return embeddings[i].ToolIdentity < embeddings[j].ToolIdentity })

	next := *s
	next.Tools = tools
	next.Embeddings = embeddings
	if err := next.Validate(); err != nil {
		return nil, fmt.Errorf("storage: indexed snapshot for server %q is invalid: %w", serverID, err)
	}
	return &next, nil
}
