package service

import (
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func measureResult(result *mcp.CallToolResult) (int, error) {
	encoded, err := json.Marshal(result)
	if err != nil {
		return 0, err
	}
	return len(encoded), nil
}
