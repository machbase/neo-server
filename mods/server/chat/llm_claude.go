package chat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
	"github.com/mark3labs/mcp-go/mcp"
)

type LLMClaudeConfig struct {
	Key            string   `json:"key"`
	MaxTokens      int64    `json:"max_tokens"`
	SystemMessages []string `json:"system_messages,omitempty"`
}

func (d *LLMDialog) execClaude(ctx context.Context) {
	claudeClient := anthropic.NewClient(
		option.WithAPIKey(d.conf.Claude.Key),
	)

	var toolParams []anthropic.ToolUnionParam
	if tools, err := d.ListTools(ctx); err != nil {
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
		anthropic.NewUserMessage(anthropic.NewTextBlock(d.userMessage)),
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
		for stream.Next() {
			event := stream.Current()
			if err := message.Accumulate(event); err != nil {
				d.SendError("😡 Failed to accumulate message: %v\n", err)
				return
			}
			switch event := event.AsAny().(type) {
			case anthropic.ContentBlockStartEvent:
				// Start of a new content block
				if event.ContentBlock.Name != "" {
					d.Send(LLMMessage{Content: fmt.Sprintf("%s: ", event.ContentBlock.Name), IsPartial: true})
				}
			case anthropic.ContentBlockDeltaEvent:
				// Partial content block
				d.Send(LLMMessage{Content: event.Delta.Text, IsPartial: true})
				d.Send(LLMMessage{Content: event.Delta.PartialJSON, IsPartial: true})
			case anthropic.ContentBlockStopEvent:
				// End of a content block
				d.Send(LLMMessage{Content: "\n"})
			case anthropic.MessageStopEvent:
				// End of the message
				d.Send(LLMMessage{Content: "\n"})
			}
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
				inputJSON, _ := json.Marshal(variant.JSON.Input.Raw())
				d.Send(LLMMessage{Content: fmt.Sprintf(" 🛠️ Tool %q <code>%s</code>", block.Name, inputJSON), IsPartial: true})

				fetchRequest := mcp.CallToolRequest{}
				fetchRequest.Request.Method = "tools/call"
				fetchRequest.Params.Name = block.Name
				fetchRequest.Params.Arguments = block.Input

				result, err := d.CallTool(ctx, fetchRequest)
				if err != nil {
					d.SendError("😡 Failed to call tool: %v", err)
					continue
				}

				var callResult string
				for _, content := range result.Content {
					switch c := content.(type) {
					case mcp.TextContent:
						callResult = c.Text
						d.Send(LLMMessage{Content: fmt.Sprintf("\nCallResult:\n<code>%s</code>", callResult), IsPartial: true})
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
