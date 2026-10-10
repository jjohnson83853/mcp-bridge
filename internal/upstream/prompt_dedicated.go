package upstream

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GetPrompt rejects requests routed through a Dedicated for another server.
func (d *Dedicated) GetPrompt(ctx context.Context, serverID, rawName string, arguments map[string]string) (*mcp.GetPromptResult, error) {
	if serverID != d.ID() {
		return nil, errors.New("upstream: server identity mismatch")
	}

	session, err := d.sessionFor(ctx)
	if err != nil {
		return nil, err
	}
	return d.client.GetPrompt(ctx, session, &mcp.GetPromptParams{
		Name:      rawName,
		Arguments: arguments,
	})
}
