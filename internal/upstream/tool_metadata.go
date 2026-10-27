package upstream

import (
	"encoding/json"
	"fmt"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/core"
)

// metadataFromSDKTool converts one SDK tool's supported upstream metadata
// (title, icons, annotation object, supported `_meta` keys) into the
// SDK-normalized catalog representation. It is the single discovery-time
// conversion: pointer hints keep their nil/false/true distinction, and the
// scalar hints are the SDK's own bare booleans. Unsupported shapes fail
// discovery rather than silently dropping the metadata.
func metadataFromSDKTool(tool *mcp.Tool) (core.ToolMetadata, error) {
	metadata := core.ToolMetadata{Title: tool.Title}
	if len(tool.Icons) > 0 {
		encoded, err := json.Marshal(tool.Icons)
		if err != nil {
			return core.ToolMetadata{}, fmt.Errorf("upstream: tool %q icons: %w", tool.Name, err)
		}
		metadata.Icons = encoded
	}
	if tool.Annotations != nil {
		encoded, err := json.Marshal(tool.Annotations)
		if err != nil {
			return core.ToolMetadata{}, fmt.Errorf("upstream: tool %q annotations: %w", tool.Name, err)
		}
		metadata.Annotations = encoded
	}
	if len(tool.Meta) > 0 {
		encoded, err := json.Marshal(map[string]any(tool.Meta))
		if err != nil {
			return core.ToolMetadata{}, fmt.Errorf("upstream: tool %q _meta: %w", tool.Name, err)
		}
		metadata.Meta = encoded
	}
	if err := metadata.Validate(); err != nil {
		return core.ToolMetadata{}, fmt.Errorf("upstream: tool %q: %w", tool.Name, err)
	}
	return metadata, nil
}
