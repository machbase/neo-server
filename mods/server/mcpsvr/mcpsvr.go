package mcpsvr

import (
	"fmt"
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

func NewMCPServer() *MCPServer {
	ret := &MCPServer{}
	ret.mcpServer = server.NewMCPServer(
		"neo-mcp",
		"0.0.1",
		server.WithLogging(),
	)
	if err := ret.ReloadTools(); err != nil {
		fmt.Printf("Error loading tools: %v\n", err)
		return nil
	}
	return ret
}

var registeredTools = []server.ServerTool{}

func RegisterTools(ts ...server.ServerTool) {
	registeredTools = append(registeredTools, ts...)
}

func (ms *MCPServer) ReloadTools() error {
	// Set a tool
	ms.mcpServer.SetTools(registeredTools...)
	return nil
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
		ms.ReloadTools()
		ms.sseServer.ServeHTTP(w, r)
	}
}

func (ms *MCPServer) SSEEndpoint() (string, error) {
	return ms.sseServer.CompleteSseEndpoint()
}

func (ms *MCPServer) MessageEndpoint() (string, error) {
	return ms.sseServer.CompleteMessageEndpoint()
}
