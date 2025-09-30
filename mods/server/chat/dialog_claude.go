package chat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/server/mcpsvr"
	"github.com/mark3labs/mcp-go/mcp"
)

func NewClaudeDialog(topic string, msgID int64, model string) *DialogCalude {
	const systemMessage = "You are a friendly AI assistant for Machbase Neo DB."
	ret := &DialogCalude{
		topic:          topic,
		msgID:          msgID,
		model:          model,
		Key:            "your-key",
		MaxTokens:      1024,
		SystemMessages: []string{systemMessage},
		log:            logging.GetLog("chat/claude"),
	}
	return ret
}

type DialogCalude struct {
	Key            string   `json:"key"`
	MaxTokens      int64    `json:"max_tokens"`
	SystemMessages []string `json:"system_messages,omitempty"`

	topic string      `json:"-"`
	msgID int64       `json:"-"`
	model string      `json:"-"`
	log   logging.Log `json:"-"`
}

func (d *DialogCalude) publish(typ eventbus.BodyType, body *eventbus.BodyUnion) {
	eventbus.PublishMessage(d.topic, &eventbus.Message{
		Ver:  "1.0",
		ID:   d.msgID,
		Type: typ,
		Body: body,
	})
}

func (d *DialogCalude) SendError(errMsg string) {
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

func (d *DialogCalude) Talk(ctx context.Context, userMessage string) {
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
	for _, msg := range d.SystemMessages {
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
			if d.log.InfoEnabled() {
				bs, _ := json.Marshal(event)
				d.log.Info(string(bs))
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

		if d.log.DebugEnabled() {
			d.log.Debugf("Claude stream ended: %#v", message)
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
					d.SendError(fmt.Sprintf("😡 Failed to call tool: %v", err))
					continue
				}

				var callResult string
				for _, content := range result.Content {
					switch c := content.(type) {
					case mcp.TextContent:
						d.log.Debugf("%s Tool result:\n%s", block.ID, c.Text)
						callResult = c.Text
						// conv := mdconv.New(mdconv.WithDarkMode(false))
						// code := fmt.Sprintf("\n📎 **Result**\n```\n%s\n```\n", c.Text)
						// w := &strings.Builder{}
						// conv.ConvertString(code, w)
						// d.Send(LLMMessage{Content: w.String(), IsPartial: true})
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

	d.publish(eventbus.BodyTypeStreamBlockStart, nil)
	d.publish(eventbus.BodyTypeStreamBlockDelta,
		&eventbus.BodyUnion{
			OfStreamBlockDelta: &eventbus.StreamBlockDelta{
				ContentType: "text",
				Text:        fmt.Sprintf("message from %s", d.model),
			},
		})
	d.publish(eventbus.BodyTypeStreamBlockStop, nil)
}
