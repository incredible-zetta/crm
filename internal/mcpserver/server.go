package mcpserver

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewMCPServer creates the mcp.Server (tools registered separately in Task 10 via a RegisterTools func that Task 10 will add).
func NewMCPServer(name, version string) *mcp.Server {
	return mcp.NewServer(&mcp.Implementation{
		Name:    name,
		Version: version,
	}, nil)
}

// Handler builds the full HTTP handler for the /mcp endpoint: wraps the streamable handler with AuthHandler.
func Handler(apiKey string, srv *mcp.Server) http.Handler {
	stream := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
	return AuthHandler(apiKey, stream)
}
