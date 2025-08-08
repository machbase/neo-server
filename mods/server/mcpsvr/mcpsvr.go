package mcpsvr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

type MCPServer struct {
	conf      MCPServerConfig
	confTime  time.Time
	mcpServer *server.MCPServer
	sseServer *server.SSEServer
}

const MCPServerConfigVersion = 1

type MCPServerConfig struct {
	Version int `json:"version"`
	Tools   map[string]struct {
		Description string `json:"description"`
	} `json:"tools"`
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

func (ms *MCPServer) ReloadTools() error {
	// tool description
	var config MCPServerConfig
	confDir := "."
	if dir, err := os.UserHomeDir(); err == nil {
		confDir = filepath.Join(dir, ".config", "machbase")
	} else {
		fmt.Printf("Warning: Unable to get user home directory, using current directory for config: %v\n", err)
	}
	confFile := filepath.Join(confDir, "mcp_server.json")

regen:
	if stat, err := os.Stat(confFile); os.IsNotExist(err) {
		fmt.Printf("Warning: MCP server config file not found at %s, using default configuration\n", confFile)
		config = MCPServerConfig{
			Version: MCPServerConfigVersion,
			Tools: map[string]struct {
				Description string `json:"description"`
			}{
				"now": {
					Description: "Get current time in Unix Epoch Nanosecond",
				},
				"gen_sql": {
					Description: "Generate SQL query to retrieve data from a specified table within a given time range.",
				},
				"exec_query_sql": {
					Description: "Execute a specified SQL query and return the results.",
				},
			},
		}
		file, err := os.OpenFile(confFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			fmt.Printf("Error creating config file: %v\n", err)
			return nil
		}
		defer file.Close()
		// Write default config to file
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(&config); err != nil {
			fmt.Printf("Error encoding config file: %v\n", err)
			return nil
		}
		ms.confTime = time.Now()
	} else {
		if stat.ModTime().Equal(ms.confTime) {
			return nil // No changes
		}
		ms.confTime = stat.ModTime()
		file, err := os.Open(confFile)
		if err != nil {
			fmt.Printf("Error opening config file: %v\n", err)
			return nil
		}
		defer file.Close()
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&config); err != nil {
			fmt.Printf("Error decoding config file: %v\n", err)
			return nil
		}
		if config.Version != MCPServerConfigVersion {
			// Create backup filename with timestamp
			dir := filepath.Dir(confFile)
			base := filepath.Base(confFile)
			ext := filepath.Ext(base)
			name := base[:len(base)-len(ext)]
			timestamp := time.Now().Format("20060102_150405")
			bakFile := filepath.Join(dir, fmt.Sprintf("%s_%s%s", name, timestamp, ext))
			os.Rename(confFile, bakFile)
			goto regen
		}
	}
	ms.conf = config
	for _, tool := range tools {
		if desc, ok := config.Tools[tool.Tool.Name]; ok {
			tool.Tool.Description = desc.Description
		}
	}
	// Set a tool
	ms.mcpServer.SetTools(tools...)
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
