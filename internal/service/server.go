package service

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
)

func NewMCPServer() *mcp.Server {
	return mcp.NewServer(&mcp.Implementation{Name: "mcp-bridge", Version: "1"}, &mcp.ServerOptions{HasPrompts: true})
}
func NewStreamableHandler(server *mcp.Server) http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
}
