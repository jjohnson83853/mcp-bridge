package service

import (
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/search"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

// indexedToolRows renders one server's exact definitions of this cycle as the
// durable rows of its catalog (plan §4.9). Both identities the storage model
// distinguishes are derived here: the exact definition digest, which changes
// with any real definition change, and the formatter content identity, which
// covers only the embedded text - so a schema-only change keeps the row's
// content hash and therefore its cached vector (plan §4.8, AC-13).
func indexedToolRows(serverID string, defs []core.ToolDefinition) []storage.Tool {
	rows := make([]storage.Tool, 0, len(defs))
	for _, def := range defs {
		row := storage.Tool{
			ServerID:             serverID,
			RawName:              def.RawName,
			PublicName:           core.PublicName(serverID, def.RawName),
			Description:          def.Description,
			DefinitionHash:       core.DefinitionDigest(serverID, def.RawName, def),
			EmbeddingContentHash: search.ContentHash(def.RawName, def.Description),
			InputSchema:          def.InputSchema,
			OutputSchema:         def.OutputSchema,
		}
		if !def.Metadata.IsZero() {
			metadata := def.Metadata.Clone()
			row.Metadata = &metadata
		}
		rows = append(rows, row)
	}
	return rows
}
