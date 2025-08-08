package chat

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/ollama/ollama/api"
)

type LLMConfig struct {
	OllamaUrl      string           `json:"ollamaUrl"`
	ChatModel      string           `json:"-"`
	ToolModel      string           `json:"toolModel"`
	ToolMessages   []LLMToolMessage `json:"toolMessages,omitempty"`
	MCPSSEEndpoint string           `json:"mcpSSEEndpoint"`
}

type LLMToolMessage struct {
	Role     string `json:"role"`
	Content  string `json:"content"`
	Thinking string `json:"thinking,omitempty"`
}

type LLMMessage struct {
	IsError bool   `json:"isError"`
	Content string `json:"content"`
}

type LLMDialog struct {
	conf        LLMConfig
	ch          chan LLMMessage
	userMessage string
}

func ExecLLM(ctx context.Context, c LLMConfig, userMessage string) <-chan LLMMessage {
	d := NewDialog(userMessage, c)
	go func() {
		defer d.Close()
		d.execLLM(ctx)
	}()
	return d.ch
}

func NewDialog(userMessage string, conf LLMConfig) *LLMDialog {
	return &LLMDialog{
		conf:        conf,
		ch:          make(chan LLMMessage),
		userMessage: userMessage,
	}
}

func (d *LLMDialog) Close() {
	if d.ch != nil {
		close(d.ch)
		d.ch = nil
	}
}

func (d *LLMDialog) SendMessage(msg string, args ...any) {
	d.Send(false, msg+"\n", args...)
}

func (d *LLMDialog) SendError(msg string, args ...any) {
	d.Send(true, msg+"\n", args...)
}

func (d *LLMDialog) Send(isError bool, msg string, args ...any) {
	if d.ch == nil {
		log.Println("Dialog channel is closed, cannot send message")
		return
	}
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	d.ch <- LLMMessage{
		IsError: isError,
		Content: msg,
	}
}

func (d *LLMDialog) execLLM(ctx context.Context) {
	url, _ := url.Parse(d.conf.OllamaUrl)
	ollamaClient := api.NewClient(url, &http.Client{
		Timeout: 0,
	})

	mcpClient, err := client.NewSSEMCPClient(d.conf.MCPSSEEndpoint)
	if err != nil {
		d.SendError("Failed to create mcp client: %v", err)
		return
	}
	if err = mcpClient.Start(ctx); err != nil {
		d.SendError("Failed to start mcp client: %v", err)
		return
	}
	defer mcpClient.Close()

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
		return
	}
	d.SendMessage("🌍 MCP Server Info: %s %s", initResult.ServerInfo.Name, initResult.ServerInfo.Version)

	// Get the list of tools
	toolsRequest := mcp.ListToolsRequest{}
	tools, err := mcpClient.ListTools(ctx, toolsRequest)
	if err != nil {
		d.SendError("Failed to list tools: %v", err)
		return
	}

	// Define/Convert tool with Ollama format
	ollamaTools := ConvertToOllamaTools(tools.Tools)

	// Have a "tool chat" with Ollama 🦙
	// Prompt construction
	messages := []api.Message{}

	for _, toolMessage := range d.conf.ToolMessages {
		messages = append(messages, api.Message{
			Role:     toolMessage.Role,
			Content:  toolMessage.Content,
			Thinking: toolMessage.Thinking,
		})
	}
	messages = append(messages, api.Message{
		Role:    "user",
		Content: d.userMessage,
	})

	var FALSE = false
	req := &api.ChatRequest{
		Model:    d.conf.ToolModel,
		Messages: messages,
		Options: map[string]interface{}{
			"temperature":   0,
			"repeat_last_n": 1,
		},
		Tools:  ollamaTools,
		Stream: &FALSE,
	}

	err = ollamaClient.Chat(ctx, req, func(resp api.ChatResponse) error {
		d.SendMessage("🦙 Ollama response: %s", resp.Message.Content)
		// Ollma tools to call
		for _, toolCall := range resp.Message.ToolCalls {
			// 🖐️ Call the mcp server
			d.SendMessage("🦙🛠️ %s %s", toolCall.Function.Name, toolCall.Function.Arguments)
			fetchRequest := mcp.CallToolRequest{}
			fetchRequest.Request.Method = "tools/call"
			fetchRequest.Params.Name = toolCall.Function.Name
			fetchRequest.Params.Arguments = toolCall.Function.Arguments

			result, err := mcpClient.CallTool(ctx, fetchRequest)
			if err != nil {
				d.SendError("Failed to call tool: %v", err)
			}
			// display the text content of result
			d.SendMessage("🌍 call result:")
			for _, content := range result.Content {
				switch c := content.(type) {
				case mcp.TextContent:
					d.SendMessage(c.Text)
				default:
					d.SendError("😡 Unhandled content type from tool: %#v", c)
				}
			}
		}
		return nil
	})

	if err != nil {
		d.SendError("Failed to chat with Ollama: %v", err)
	}
}

func ConvertToOllamaTools(tools []mcp.Tool) []api.Tool {
	// Convert tools to Ollama format
	ollamaTools := make([]api.Tool, len(tools))
	for i, tool := range tools {
		ollamaTools[i] = api.Tool{
			Type: "function",
			Function: api.ToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters: struct {
					Type       string                      `json:"type"`
					Defs       any                         `json:"$defs,omitempty"`
					Items      any                         `json:"items,omitempty"`
					Required   []string                    `json:"required"`
					Properties map[string]api.ToolProperty `json:"properties"`
				}{
					Type:       tool.InputSchema.Type,
					Required:   tool.InputSchema.Required,
					Properties: convertProperties(tool.InputSchema.Properties),
				},
			},
		}
	}
	return ollamaTools
}

// Helper function to convert properties to Ollama's format
func convertProperties(props map[string]interface{}) map[string]api.ToolProperty {
	result := make(map[string]api.ToolProperty)

	for name, prop := range props {
		if propMap, ok := prop.(map[string]interface{}); ok {
			prop := api.ToolProperty{
				Type:        getType(propMap, "type"),
				Description: getString(propMap, "description"),
			}

			// Handle enum if present
			if enumRaw, ok := propMap["enum"].([]interface{}); ok {
				for _, e := range enumRaw {
					if str, ok := e.(string); ok {
						prop.Enum = append(prop.Enum, str)
					}
				}
			}

			result[name] = prop
		}
	}

	return result
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
