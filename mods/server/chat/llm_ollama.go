package chat

import (
	"context"
	"html"
	"net/http"
	"net/url"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/ollama/ollama/api"
)

type LLMOllamaConfig struct {
	Url            string   `json:"url"`
	SystemMessages []string `json:"system_messages,omitempty"`
}

func (d *LLMDialog) execOllama(ctx context.Context) {
	url, _ := url.Parse(d.conf.Ollama.Url)
	ollamaClient := api.NewClient(url, &http.Client{
		Timeout: 0,
	})

	tools, err := d.ListTools(ctx)
	if err != nil {
		d.SendError("Failed to list tools: %v", err)
		return
	}

	// Define/Convert tool with Ollama format
	ollamaTools := ConvertToOllamaTools(tools.Tools)

	// Have a "tool chat" with Ollama 🦙
	// Prompt construction
	messages := []api.Message{}

	for _, toolMessage := range d.conf.Ollama.SystemMessages {
		messages = append(messages, api.Message{
			Role:    "system",
			Content: toolMessage,
		})
	}
	messages = append(messages, api.Message{
		Role:    "user",
		Content: d.userMessage,
	})

	var stream = true
	var think = api.ThinkValue{Value: false}
	req := &api.ChatRequest{
		Model:    d.model, // d.conf.Ollama.ToolModel,
		Messages: messages,
		Options: map[string]interface{}{
			"temperature":   0,
			"repeat_last_n": 1,
		},
		Tools:  ollamaTools,
		Stream: &stream,
		Think:  &think,
	}

	d.SendMessage("🦙 Ollama response: \n")
	err = ollamaClient.Chat(ctx, req, func(resp api.ChatResponse) error {
		d.SendMessage("%s", html.EscapeString(resp.Message.Content))
		// Ollma tools to call
		for _, toolCall := range resp.Message.ToolCalls {
			// 🖐️ Call the mcp server
			d.SendMessage("🦙🛠️ %s %v\n", toolCall.Function.Name, toolCall.Function.Arguments)
			fetchRequest := mcp.CallToolRequest{}
			fetchRequest.Request.Method = "tools/call"
			fetchRequest.Params.Name = toolCall.Function.Name
			fetchRequest.Params.Arguments = toolCall.Function.Arguments

			result, err := d.CallTool(ctx, fetchRequest)
			if err != nil {
				d.SendError("Failed to call tool: %v", err)
			}
			// display the text content of result
			d.SendMessage("🌍 call result:")
			for _, content := range result.Content {
				switch c := content.(type) {
				case mcp.TextContent:
					d.SendMessage("%s", html.EscapeString(c.Text))
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
