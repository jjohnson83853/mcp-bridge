package service

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func NewMCPServer() *mcp.Server {
	return mcp.NewServer(&mcp.Implementation{Name: "mcp-bridge", Version: "1"}, nil)
}

func NewStreamableHandler(server *mcp.Server) http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
}
