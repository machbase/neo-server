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

const (
	ModelClaudeSonnet4_5 = "claude-sonnet-4-5-20250929"
	ModelClaudeHaiku4_5  = "claude-haiku-4-5-20251001"
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
	ctxCancel      context.CancelFunc
}

func (d *ClaudeDialog) Input(line string) {
}

func (d *ClaudeDialog) Control(ctrl string) {
	d.log.Trace("User control:", ctrl)
	switch ctrl {
	case "^C":
		d.ctxCancel()
	}
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

func (d *ClaudeDialog) publishTextBlock(text string) {
	d.publish(eventbus.BodyTypeStreamBlockStart, nil)
	d.publish(eventbus.BodyTypeStreamBlockDelta,
		&eventbus.BodyUnion{
			OfStreamBlockDelta: &eventbus.StreamBlockDelta{
				ContentType: "text",
				Text:        text,
			},
		})
	d.publish(eventbus.BodyTypeStreamBlockStop, nil)
}

func (d *ClaudeDialog) publishTextBlockStart(text string) {
	d.publish(eventbus.BodyTypeStreamBlockStart, nil)
	if text == "" {
		return
	}
	d.publish(eventbus.BodyTypeStreamBlockDelta,
		&eventbus.BodyUnion{
			OfStreamBlockDelta: &eventbus.StreamBlockDelta{
				ContentType: "text",
				Text:        text,
			},
		})
}

func (d *ClaudeDialog) publishTextBlockStop() {
	d.publish(eventbus.BodyTypeStreamBlockStop, nil)
}

func (d *ClaudeDialog) publishTextBlockDelta(text string) {
	d.publish(eventbus.BodyTypeStreamBlockDelta,
		&eventbus.BodyUnion{
			OfStreamBlockDelta: &eventbus.StreamBlockDelta{
				ContentType: "text",
				Text:        text,
			},
		})
}

func (d *ClaudeDialog) publishThinkingBlockStart(thinking string) {
	d.publish(eventbus.BodyTypeStreamBlockStart, &eventbus.BodyUnion{
		OfStreamBlockDelta: &eventbus.StreamBlockDelta{
			ContentType: "thinking",
			Thinking:    thinking,
		},
	})
}

func (d *ClaudeDialog) publishThinkingBlockDelta(thinking string) {
	d.publish(eventbus.BodyTypeStreamBlockDelta, &eventbus.BodyUnion{
		OfStreamBlockDelta: &eventbus.StreamBlockDelta{
			ContentType: "thinking",
			Thinking:    thinking,
		},
	})
}

func (d *ClaudeDialog) publishThinkingBlockStop() {
	d.publish(eventbus.BodyTypeStreamBlockStop, &eventbus.BodyUnion{
		OfStreamBlockDelta: &eventbus.StreamBlockDelta{
			ContentType: "thinking",
		},
	})
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

func (d *ClaudeDialog) Process(ctxParent context.Context, userMessage string) {
	d.publish(eventbus.BodyTypeAnswerStart, nil)
	defer d.publish(eventbus.BodyTypeAnswerStop, nil)

	claudeClient := anthropic.NewClient(
		option.WithAPIKey(d.Key),
	)

	ctx, cancel := context.WithCancel(ctxParent)
	d.ctxCancel = cancel
	defer cancel()

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
					d.publishTextBlockStart(block.Text)
				case "thinking":
					block := event.ContentBlock.AsThinking()
					d.publishThinkingBlockStart(block.Thinking)
				}
			case anthropic.ContentBlockDeltaEvent:
				// Partial content block
				switch currentBlockType {
				case "text":
					d.publishTextBlockDelta(event.Delta.Text)
				case "thinking":
					d.publishThinkingBlockDelta(event.Delta.Thinking)
				}
			case anthropic.ContentBlockStopEvent:
				// End of a content block
				switch currentBlockType {
				case "text":
					d.publishTextBlockStop()
				case "thinking":
					d.publishThinkingBlockStop()
				}
				currentBlockType = ""
			}
		}

		if d.log.TraceEnabled() {
			d.log.Trace("message:", message.RawJSON())
		}
		if err := stream.Err(); err != nil {
			d.SendError(fmt.Sprintf("😡 Stream error: %v", err))
			return
		}
		switch message.StopReason {
		case anthropic.StopReasonEndTurn:
			// The most common stop reason. Indicates Claude finished its response naturally.
			return
		case anthropic.StopReasonMaxTokens:
			// Claude stopped because it reached the max_tokens limit specified in your request.
			d.publishTextBlock("\n⚠️ Response was cut off because the maximum token limit was reached.\n")
			return
		case anthropic.StopReasonStopSequence:
			// Claude encountered one of your custom stop sequences.
			d.publishTextBlock("\n⚠️ Response was cut off because a custom stop sequence was encountered.\n")
			return
		case anthropic.StopReasonToolUse:
			// Indicates the response was cut off because a stop sequence was generated.
		case anthropic.StopReasonPauseTurn:
			// Used with server tools like web search when Claude needs to pause a long-running operation.
			d.publishTextBlock("\n⏸️ Response paused to perform a long-running operation.\n")
			return
		case anthropic.StopReasonRefusal:
			// Claude refused to generate a response due to safety concerns.
			d.publishTextBlock("\n⚠️ Response was cut off due to safety concerns.\n")
			return
		default:
			d.publishTextBlock(fmt.Sprintf("\n⚠️ Response was cut off for unknown reason: %s\n", message.StopReason))
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
						d.publishTextBlock(fmt.Sprintf("\n🛠️ Executing TQL script:\n```\n%s\n```\n", script))
					}
				case "execute_sql_query":
					args := map[string]any{}
					json.Unmarshal(block.Input, &args)
					if script, ok := args["query"]; ok {
						d.publishTextBlock(fmt.Sprintf("\n🛠️ Executing SQL script:\n```sql\n%s\n```\n", script))
					}
				default:
					d.publishTextBlock(fmt.Sprintf("\n🛠️ Calling tool: %s\n", block.Name))
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
						d.publishTextBlock(c.Text)
					default:
						d.SendError(fmt.Sprintf("😡 Unhandled content type from tool: %#v", c))
					}
				}
				toolResults = append(toolResults, anthropic.NewToolResultBlock(block.ID, callResult, result.IsError))
			}
		}
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}
}
