package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/server/mcpsvr"
	"github.com/mark3labs/mcp-go/mcp"
)

type ClaudeConfig struct {
	Key       string `json:"key"`
	MaxTokens int64  `json:"max_tokens"`
}

func (c *ClaudeConfig) MaskSensitive() {
	if len(c.Key) > 8 {
		c.Key = c.Key[:8] + strings.Repeat("*", len(c.Key)-8)
	}
}

func NewClaudeConfig() ClaudeConfig {
	return ClaudeConfig{
		Key:       "your-key",
		MaxTokens: 1024,
	}
}

type ClaudeDialog struct {
	ClaudeConfig
	systemMessages []string
	topic          string
	msgID          int64
	session        string
	model          string
	log            logging.Log
}

func (d *ClaudeDialog) publish(typ eventbus.BodyType, body *eventbus.BodyUnion) {
	eventbus.PublishMessage(d.topic, d.session,
		&eventbus.Message{
			Ver:  "1.0",
			ID:   d.msgID,
			Type: typ,
			Body: body,
		})
}

func (d *ClaudeDialog) publishCodeFence(lang, code string) {
	d.publish(eventbus.BodyTypeStreamBlockStart, nil)
	d.publish(eventbus.BodyTypeStreamBlockDelta,
		&eventbus.BodyUnion{
			OfStreamBlockDelta: &eventbus.StreamBlockDelta{
				ContentType: "text",
				Text:        fmt.Sprintf("\n🛠️ Executing TQL script:\n```%s\n%s\n```\n\n", lang, code),
			},
		})
	d.publish(eventbus.BodyTypeStreamBlockStop, nil)
}

func (d *ClaudeDialog) SendError(errMsg string) {
	d.publish(eventbus.BodyTypeStreamBlockStart, nil)
	d.publish(eventbus.BodyTypeStreamBlockDelta,
		&eventbus.BodyUnion{
			OfStreamBlockDelta: &eventbus.StreamBlockDelta{
				ContentType: "text",
				Text:        errMsg,
			},
		})
	d.publish(eventbus.BodyTypeStreamBlockStop, nil)
}

func (d *ClaudeDialog) Talk(ctx context.Context, userMessage string) {
	d.publish(eventbus.BodyTypeAnswerStart, nil)
	defer d.publish(eventbus.BodyTypeAnswerStop, nil)

	claudeClient := anthropic.NewClient(
		option.WithAPIKey(d.Key),
	)

	var toolParams []anthropic.ToolUnionParam
	if tools, err := mcpsvr.ListTools(ctx); err != nil {
		d.SendError(fmt.Sprintf("Failed to list tools: %v", err))
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
	for _, msg := range d.systemMessages {
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
			MaxTokens: d.MaxTokens,
			System:    systems,
			Messages:  messages,
			Tools:     toolParams,
		})

		message := anthropic.Message{}
		event := stream.Current()
		if err := message.Accumulate(event); err != nil {
			d.SendError(fmt.Sprintf("😡 Failed to accumulate message: %v", err))
			return
		}
		var currentBlockType string
		for stream.Next() {
			event := stream.Current()
			if err := message.Accumulate(event); err != nil {
				d.SendError(fmt.Sprintf("😡 Failed to accumulate message: %v", err))
				return
			}
			if d.log.TraceEnabled() {
				d.log.Trace("stream:", event.RawJSON())
			}
			switch event := event.AsAny().(type) {
			case anthropic.MessageStartEvent:
				// Start of a new message
				d.publish(eventbus.BodyTypeStreamMessageStart, nil)
			case anthropic.MessageDeltaEvent:
				// Partial message content
				d.publish(eventbus.BodyTypeStreamMessageDelta,
					&eventbus.BodyUnion{
						OfStreamBlockDelta: &eventbus.StreamBlockDelta{
							ContentType: "text",
							Text:        string(event.Delta.StopReason),
						},
					})
			case anthropic.MessageStopEvent:
				// End of the message
				d.publish(eventbus.BodyTypeStreamMessageStop, nil)
			case anthropic.ContentBlockStartEvent:
				// Start of a new content block
				// Any of "text", "thinking", "redacted_thinking",
				// "tool_use", "server_tool_use", "web_search_tool_result".
				currentBlockType = event.ContentBlock.Type
				switch currentBlockType {
				case "text":
					block := event.ContentBlock.AsText()
					d.publish(eventbus.BodyTypeStreamBlockStart, &eventbus.BodyUnion{
						OfStreamBlockDelta: &eventbus.StreamBlockDelta{
							ContentType: "text",
							Text:        block.Text,
						},
					})
				case "thinking":
					block := event.ContentBlock.AsThinking()
					d.publish(eventbus.BodyTypeStreamBlockStart, &eventbus.BodyUnion{
						OfStreamBlockDelta: &eventbus.StreamBlockDelta{
							ContentType: "thinking",
							Thinking:    block.Thinking,
						},
					})
				}
			case anthropic.ContentBlockDeltaEvent:
				// Partial content block
				switch currentBlockType {
				case "text":
					d.publish(eventbus.BodyTypeStreamBlockDelta, &eventbus.BodyUnion{
						OfStreamBlockDelta: &eventbus.StreamBlockDelta{
							ContentType: "text",
							Text:        event.Delta.Text,
						},
					})
				case "thinking":
					d.publish(eventbus.BodyTypeStreamBlockDelta, &eventbus.BodyUnion{
						OfStreamBlockDelta: &eventbus.StreamBlockDelta{
							ContentType: "thinking",
							Thinking:    event.Delta.Thinking,
						},
					})
				}
			case anthropic.ContentBlockStopEvent:
				// End of a content block
				switch currentBlockType {
				case "text":
					d.publish(eventbus.BodyTypeStreamBlockStop, &eventbus.BodyUnion{
						OfStreamBlockDelta: &eventbus.StreamBlockDelta{
							ContentType: "text",
						},
					})
				case "thinking":
					d.publish(eventbus.BodyTypeStreamBlockStop, &eventbus.BodyUnion{
						OfStreamBlockDelta: &eventbus.StreamBlockDelta{
							ContentType: "thinking",
						},
					})
				}
				currentBlockType = ""
			}
		}

		if d.log.TraceEnabled() {
			d.log.Trace("stream:", message.RawJSON())
		}
		if err := stream.Err(); err != nil {
			d.SendError(fmt.Sprintf("😡 Stream error: %v", err))
			return
		}

		messages = append(messages, message.ToParam())
		toolResults := []anthropic.ContentBlockParamUnion{}

		for _, block := range message.Content {
			switch variant := block.AsAny().(type) {
			case anthropic.ToolUseBlock:
				if d.log.TraceEnabled() {
					d.log.Tracef("%s Tool using: %s %v", block.ID, block.Name, variant.JSON.Input.Raw())
				}
				switch block.Name {
				case "execute_tql_script":
					args := map[string]any{}
					json.Unmarshal(block.Input, &args)
					if script, ok := args["script"]; ok {
						d.publishCodeFence("", script.(string))
					}
				case "execute_sql_query":
					args := map[string]any{}
					json.Unmarshal(block.Input, &args)
					if script, ok := args["query"]; ok {
						d.publishCodeFence("sql", script.(string))
					}
				}

				fetchRequest := mcp.CallToolRequest{}
				fetchRequest.Request.Method = "tools/call"
				fetchRequest.Params.Name = block.Name
				fetchRequest.Params.Arguments = block.Input

				result, err := mcpsvr.CallTool(ctx, fetchRequest)
				if err != nil {
					d.SendError(fmt.Sprintf("😡 Failed to call tool: %v", err))
					continue
				}

				var callResult string
				for _, content := range result.Content {
					switch c := content.(type) {
					case mcp.TextContent:
						if d.log.TraceEnabled() {
							peek := c.Text
							if len(peek) > 128 {
								peek = peek[:128] + "..."
							}
							peek = strings.ReplaceAll(peek, "\n", "\\n")
							d.log.Tracef("%s Tool result:\n%s", block.ID, peek)
						}
						callResult = c.Text
					default:
						d.SendError(fmt.Sprintf("😡 Unhandled content type from tool: %#v", c))
					}
				}
				toolResults = append(toolResults, anthropic.NewToolResultBlock(block.ID, callResult, result.IsError))
			}
		}
		if len(toolResults) == 0 {
			break
		}
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}
}
