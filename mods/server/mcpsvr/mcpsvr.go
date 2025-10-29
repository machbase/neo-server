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

const MCPServerConfigVersion = 1

type MCPServerConfig struct {
	Version int                            `json:"version"`
	Tools   map[string]MCPServerToolConfig `json:"tools"`
}

type MCPServerToolConfig struct {
	Description string            `json:"description"`
	Inputs      map[string]string `json:"inputs"`
}

func NewMCPServer(opts ...MCPServerOption) *MCPServer {
	ret := &MCPServer{}
	ret.mcpServer = server.NewMCPServer(
		"neo-mcp",
		"0.0.1",
		server.WithLogging(),
	)
	// Set the default tools
	ret.mcpServer.SetTools(registeredTools...)
	// Apply options
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

type MCPServerOption func(*MCPServer)

func WithTools(tools ...server.ServerTool) MCPServerOption {
	return func(ms *MCPServer) {
		ms.mcpServer.AddTools(tools...)
	}
}

func WithVersion(version string) MCPServerOption {
	return func(ms *MCPServer) {
		neoVersionString = version
	}
}

var neoVersionString = "unknown"

var registeredTools = []server.ServerTool{}

func RegisterTools(ts ...server.ServerTool) {
	registeredTools = append(registeredTools, ts...)
}

// Handler creates a new HTTP handler for the MCP SSE handler
func (ms *MCPServer) HandlerFunc() http.HandlerFunc {
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
	return func(w http.ResponseWriter, r *http.Request) {
		ms.sseServer.ServeHTTP(w, r)
	}
}

func (ms *MCPServer) SSEEndpoint() (string, error) {
	return ms.sseServer.CompleteSseEndpoint()
}

func (ms *MCPServer) MessageEndpoint() (string, error) {
	return ms.sseServer.CompleteMessageEndpoint()
}
