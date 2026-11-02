package service

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
)

func NewMCPServer() *mcp.Server {
	return mcp.NewServer(&mcp.Implementation{Name: "mcp-bridge", Version: "1"}, &mcp.ServerOptions{HasPrompts: true})
}
func NewStreamableHandler(server *mcp.Server, opts ...*mcp.StreamableHTTPOptions) http.Handler {
	var streamableOpts *mcp.StreamableHTTPOptions
	if len(opts) > 0 && opts[0] != nil {
		streamableOpts = opts[0]
	}
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, streamableOpts)
}
