package chat

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/ollama/ollama/api"
)

type LLMMessage struct {
	IsError   bool   `json:"isError"`
	IsPartial bool   `json:"isPartial,omitempty"`
	Content   string `json:"content"`
}

type LLMDialog struct {
	conf        *LLMConfig
	ch          chan LLMMessage
	userMessage string
	model       string

	client *client.Client `json:"-"`
	log    logging.Log    `json:"-"`
}

func ExecLLM(ctx context.Context, c *LLMConfig, model string, userMessage string) <-chan LLMMessage {
	d := NewDialog(userMessage, c)
	go func() {
		defer d.Close()
		if strings.HasPrefix(model, "claude:") {
			if c.Claude.Key == "" {
				d.SendError("Claude model selected but no API key configured.")
				return
			}
			d.model = strings.TrimPrefix(model, "claude:")
			d.execClaude(ctx)
		} else if strings.HasPrefix(model, "ollama:") {
			if c.Ollama.Url == "" {
				d.SendError("Ollama model selected but no Ollama URL configured.")
				return
			}
			d.model = strings.TrimPrefix(model, "ollama:")
			d.execOllama(ctx)
		} else {
			d.SendError("Unknown model prefix. Please use 'claude:' or 'ollama:'.")
			return
		}
	}()
	return d.ch
}

func NewDialog(userMessage string, conf *LLMConfig) *LLMDialog {
	return &LLMDialog{
		conf:        conf,
		ch:          make(chan LLMMessage),
		userMessage: userMessage,
		log:         logging.GetLog("chat"),
	}
}

func (d *LLMDialog) Close() {
	if d.client != nil {
		d.client.Close()
		d.client = nil
	}
	if d.ch != nil {
		close(d.ch)
		d.ch = nil
	}
}

func (d *LLMDialog) SendMessage(format string, args ...any) {
	m := LLMMessage{
		IsError: false,
	}
	if len(args) > 0 {
		m.Content = fmt.Sprintf(format, args...)
	} else {
		m.Content = format
	}
	d.Send(m)
}

func (d *LLMDialog) SendError(msg string, args ...any) {
	m := LLMMessage{
		IsError: true,
	}
	if len(args) > 0 {
		m.Content = fmt.Sprintf(msg, args...)
	} else {
		m.Content = msg
	}
	d.Send(m)
}

func (d *LLMDialog) Send(m LLMMessage) {
	if d.ch == nil {
		log.Println("Dialog channel is closed, cannot send message")
		return
	}
	d.ch <- m
}

func (d *LLMDialog) mcpClient(ctx context.Context) (*client.Client, error) {
	if d.client != nil {
		return d.client, nil
	}
	mcpClient, err := client.NewSSEMCPClient(d.conf.MCP.Endpoint)
	if err == nil {
		d.client = mcpClient
		if err = d.client.Start(ctx); err != nil {
			d.SendError("Failed to start mcp client: %v", err)
			return nil, err
		}
	}
	return mcpClient, err
}

func (d *LLMDialog) CallTool(ctx context.Context, toolCall mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	mcpClient, err := d.mcpClient(ctx)
	if err != nil {
		return nil, err
	}
	result, err := mcpClient.CallTool(ctx, toolCall)
	return result, err
}

func (d *LLMDialog) ListTools(ctx context.Context) (*mcp.ListToolsResult, error) {
	mcpClient, err := d.mcpClient(ctx)
	if err != nil {
		return nil, err
	}

	// Initialize the request
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "neo-mcp client",
		Version: "0.0.1",
	}

	// Initialize the client
	initResult, err := mcpClient.Initialize(ctx, initRequest)
	if err != nil {
		d.SendError("Failed to initialize mcp client: %v", err)
		return nil, err
	}
	_ = initResult
	d.SendMessage("🌍 %s\n", d.model)

	// Get the list of tools
	toolsRequest := mcp.ListToolsRequest{}
	tools, err := mcpClient.ListTools(ctx, toolsRequest)
	if err != nil {
		d.SendError("Failed to list tools: %v", err)
		return nil, err
	}
	return tools, nil
}

// Helper function to safely get string values from map
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// Helper function to safely get string values from map
func getType(m map[string]interface{}, key string) api.PropertyType {
	if v, ok := m[key].(string); ok {
		return api.PropertyType([]string{v})
	}
	return api.PropertyType([]string{})
}
