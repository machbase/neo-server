package mcpsvr

import (
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

type MCPServer struct {
	mcpServer *server.MCPServer
	sseServer *server.SSEServer
}

func NewMCPServer() *MCPServer {
	ret := &MCPServer{}
	ret.mcpServer = server.NewMCPServer(
		"neo-mcp",
		"0.0.1",
		server.WithLogging(),
	)
	// Add a tool
	ret.mcpServer.AddTools(tools...)
	return ret
}

// Handler creates a new HTTP handler for the MCP SSE handler
func (ms *MCPServer) Handler() http.Handler {
	if ms.sseServer == nil {
		ms.sseServer = server.NewSSEServer(
			ms.mcpServer,
			server.WithKeepAlive(true),
			server.WithKeepAliveInterval(30*time.Second),
			server.WithBaseURL("http://127.0.0.1:5654/db/mcp/"),
			server.WithSSEEndpoint("/sse"),
			server.WithMessageEndpoint("/message"),
		)
	}
	return ms.sseServer
}

func (ms *MCPServer) SSEEndpoint() (string, error) {
	return ms.sseServer.CompleteSseEndpoint()
}

func (ms *MCPServer) MessageEndpoint() (string, error) {
	return ms.sseServer.CompleteMessageEndpoint()
}
