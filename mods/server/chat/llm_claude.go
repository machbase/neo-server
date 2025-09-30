package chat

import (
	"context"
	"encoding/json"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
	"github.com/machbase/neo-server/v8/mods/server/mcpsvr"
	"github.com/mark3labs/mcp-go/mcp"
)

type LLMClaudeConfig struct {
	Key            string   `json:"key"`
	MaxTokens      int64    `json:"max_tokens"`
	SystemMessages []string `json:"system_messages,omitempty"`
}

func (d *LLMDialog) execClaude(ctx context.Context, userMessage string) {
	claudeClient := anthropic.NewClient(
		option.WithAPIKey(d.conf.Claude.Key),
	)

	var toolParams []anthropic.ToolUnionParam
	if tools, err := mcpsvr.ListTools(ctx); err != nil {
		d.SendError("Failed to list tools: %v", err)
		return
	} else {
		toolParams = ConvertToClaudeTools(tools.Tools)
	}

	claudeModel := anthropic.ModelClaudeSonnet4_20250514
	if d.model != "" {
		claudeModel = anthropic.Model(d.model)
	}

	// System messages
	systems := []anthropic.TextBlockParam{}
	for _, msg := range d.conf.Claude.SystemMessages {
		systemMessage := anthropic.TextBlockParam{
			Text: msg,
		}
		systems = append(systems, systemMessage)
	}

	// User message
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(userMessage)),
	}

	for {
		stream := claudeClient.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
			Model:     claudeModel,
			MaxTokens: d.conf.Claude.MaxTokens,
			System:    systems,
			Messages:  messages,
			Tools:     toolParams,
		})

		message := anthropic.Message{}
		var currentBlockType string
		for stream.Next() {
			event := stream.Current()
			if err := message.Accumulate(event); err != nil {
				d.SendError("😡 Failed to accumulate message: %v\n", err)
				return
			}
			switch event := event.AsAny().(type) {
			case anthropic.MessageStartEvent:
				// Start of a new message
				d.Send(LLMMessage{Type: "message-start", IsPartial: true})
			case anthropic.MessageDeltaEvent:
				// Partial message content
				d.Send(LLMMessage{Type: "message-delta", Content: string(event.Delta.StopReason), IsPartial: true})
			case anthropic.MessageStopEvent:
				// End of the message
				d.Send(LLMMessage{Type: "message-stop"})
			case anthropic.ContentBlockStartEvent:
				// Start of a new content block
				// Any of "text", "thinking", "redacted_thinking",
				// "tool_use", "server_tool_use", "web_search_tool_result".
				currentBlockType = event.ContentBlock.Type
				switch currentBlockType {
				case "text":
					block := event.ContentBlock.AsText()
					d.Send(LLMMessage{
						Type:        "content-block-start",
						ContentType: "text",
						Content:     block.Text,
						IsPartial:   true,
					})
				case "thinking":
					block := event.ContentBlock.AsThinking()
					d.Send(LLMMessage{
						Type:        "content-block-start",
						ContentType: "thinking",
						Content:     block.Thinking,
						IsPartial:   true,
					})
				}
			case anthropic.ContentBlockDeltaEvent:
				// Partial content block
				switch currentBlockType {
				case "text":
					d.Send(LLMMessage{
						Type:        "content-block-delta",
						ContentType: "text",
						Content:     event.Delta.Text,
						IsPartial:   true,
					})
				case "thinking":
					d.Send(LLMMessage{
						Type:        "content-block-delta",
						ContentType: "thinking",
						Content:     event.Delta.Thinking,
						IsPartial:   true,
					})
				}
			case anthropic.ContentBlockStopEvent:
				// End of a content block
				switch currentBlockType {
				case "text":
					d.Send(LLMMessage{
						Type:        "content-block-stop",
						ContentType: "text",
					})
				case "thinking":
					d.Send(LLMMessage{
						Type:        "content-block-stop",
						ContentType: "thinking",
					})
				}
				currentBlockType = ""
			}
		}

		if d.log.DebugEnabled() {
			b, _ := json.Marshal(message)
			d.log.Debug("Claude stream ended:", string(b))
		}

		if stream.Err() != nil {
			d.SendError("😡 Stream error: %v\n", stream.Err())
			return
		}

		messages = append(messages, message.ToParam())
		toolResults := []anthropic.ContentBlockParamUnion{}

		for _, block := range message.Content {
			switch variant := block.AsAny().(type) {
			case anthropic.ToolUseBlock:
				d.log.Debugf("%s Tool using: %s %v", block.ID, block.Name, variant.JSON.Input.Raw())
				// w := &strings.Builder{}
				// conv := mdconv.New(mdconv.WithDarkMode(false))
				// code := fmt.Sprintf("🛠️ **%s**\n```json\n%s\n```", block.Name, variant.JSON.Input.Raw())
				// conv.ConvertString(code, w)
				// d.Send(LLMMessage{Content: w.String(), IsPartial: true})

				fetchRequest := mcp.CallToolRequest{}
				fetchRequest.Request.Method = "tools/call"
				fetchRequest.Params.Name = block.Name
				fetchRequest.Params.Arguments = block.Input

				result, err := mcpsvr.CallTool(ctx, fetchRequest)
				if err != nil {
					d.SendError("😡 Failed to call tool: %v", err)
					continue
				}

				var callResult string
				for _, content := range result.Content {
					switch c := content.(type) {
					case mcp.TextContent:
						d.log.Debugf("%s Tool result: %s", block.ID, c.Text)
						callResult = c.Text
						// conv := mdconv.New(mdconv.WithDarkMode(false))
						// code := fmt.Sprintf("\n📎 **Result**\n```\n%s\n```\n", c.Text)
						// w := &strings.Builder{}
						// conv.ConvertString(code, w)
						// d.Send(LLMMessage{Content: w.String(), IsPartial: true})
					default:
						d.SendError("😡 Unhandled content type from tool: %#v\n", c)
					}
				}
				d.Send(LLMMessage{Content: "\n", IsPartial: true})
				toolResults = append(toolResults, anthropic.NewToolResultBlock(block.ID, callResult, result.IsError))
			}
		}
		if len(toolResults) == 0 {
			break
		}
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}
}

type ClaudeRequest struct {
	Model      string           `json:"model"`
	MaxTokens  int              `json:"max_tokens,omitempty"`
	System     string           `json:"system,omitempty"`
	Messages   []ClaudeMessage  `json:"messages"`
	Tools      []mcp.Tool       `json:"tools,omitempty"`
	ToolChoice ClaudeToolChoice `json:"tool_choice,omitempty"`
	Stream     *bool            `json:"stream,omitempty"`
}

type ClaudeToolChoice struct {
	Type string `json:"type"`
}

type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ClaudeTool struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	InputSchema ClaudeToolInputSchema `json:"input_schema"`
}

type ClaudeToolInputSchema struct {
	Type       string                        `json:"type"`
	Properties map[string]ClaudeToolProperty `json:"properties"`
	Required   []string                      `json:"required"`
}

type ClaudeToolProperty struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Default     any    `json:"default,omitempty"`
}

func ConvertToClaudeTools(tools []mcp.Tool) []anthropic.ToolUnionParam {
	// Convert tools to Claude format
	claudeTools := make([]anthropic.ToolUnionParam, len(tools))
	for i, tool := range tools {
		claudeTools[i] = anthropic.ToolUnionParam{OfTool: &anthropic.ToolParam{
			Name:        tool.Name,
			Description: anthropic.String(tool.Description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Type:       constant.Object("object"),
				Properties: convertClaudeProperties(tool.InputSchema.Properties),
				Required:   tool.InputSchema.Required,
			},
		}}
	}
	return claudeTools
}

func convertClaudeProperties(props map[string]interface{}) map[string]ClaudeToolProperty {
	result := make(map[string]ClaudeToolProperty)
	for name, prop := range props {
		if propMap, ok := prop.(map[string]interface{}); ok {
			prop := ClaudeToolProperty{
				Type:        getString(propMap, "type"),
				Description: getString(propMap, "description"),
				Default:     getString(propMap, "default"),
			}
			result[name] = prop
		}
	}
	return result
}
