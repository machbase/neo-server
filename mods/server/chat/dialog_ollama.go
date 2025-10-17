package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/server/mcpsvr"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/ollama/ollama/api"
)

type OllamaConfig struct {
	Url string `json:"url"`
}

func NewOllamaConfig() OllamaConfig {
	return OllamaConfig{
		Url: "http://127.0.0.1:11434",
	}
}

type OllamaDialog struct {
	OllamaConfig
	systemMessages []string
	topic          string
	session        string
	msgID          int64
	model          string
	log            logging.Log
}

func (d *OllamaDialog) Input(line string) {
}

func (d *OllamaDialog) Control(ctrl string) {
}

func (d *OllamaDialog) publish(typ eventbus.BodyType, body *eventbus.BodyUnion) {
	eventbus.PublishMessage(d.topic, d.session,
		&eventbus.Message{
			Ver:  "1.0",
			ID:   d.msgID,
			Type: typ,
			Body: body,
		})
}

func (d *OllamaDialog) SendError(errMsg string) {
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

func (d *OllamaDialog) publishTextBlock(text string) {
	d.publish(eventbus.BodyTypeStreamBlockDelta,
		&eventbus.BodyUnion{
			OfStreamBlockDelta: &eventbus.StreamBlockDelta{
				ContentType: "text",
				Text:        text,
			},
		})
}

func (d *OllamaDialog) Process(ctx context.Context, message string) {
	d.publish(eventbus.BodyTypeAnswerStart, nil)
	defer d.publish(eventbus.BodyTypeAnswerStop, nil)

	url, _ := url.Parse(d.Url)
	ollamaClient := api.NewClient(url, &http.Client{
		Timeout: 0,
	})

	var ollamaTools []api.Tool
	if tools, err := mcpsvr.ListTools(ctx); err != nil {
		d.SendError(fmt.Sprintf("Failed to list tools: %v", err))
		return
	} else {
		ollamaTools = ConvertToOllamaTools(tools.Tools)
	}

	// Have a "tool chat" with Ollama 🦙
	// Prompt construction
	messages := []api.Message{}

	for _, toolMessage := range d.systemMessages {
		messages = append(messages, api.Message{
			Role:    "system",
			Content: toolMessage,
		})
	}
	messages = append(messages, api.Message{
		Role:    "user",
		Content: message,
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

	for {
		d.publish(eventbus.BodyTypeStreamMessageStart, nil)
		d.publish(eventbus.BodyTypeStreamBlockStart, nil)

		messages = []api.Message{}
		err := ollamaClient.Chat(ctx, req, func(resp api.ChatResponse) error {
			messages = append(messages, resp.Message)
			if len(resp.Message.ToolCalls) == 0 {
				d.publishTextBlock(resp.Message.Content)
			}
			return nil
		})
		if err != nil {
			d.publishTextBlock(fmt.Sprintf("Failed to chat with Ollama: %v\n", err))
			d.publish(eventbus.BodyTypeStreamBlockStop, nil)
			d.publish(eventbus.BodyTypeStreamMessageStop, nil)
			return
		}
		d.publish(eventbus.BodyTypeStreamBlockStop, nil)

		toolCalls := 0
		for _, msg := range messages {
			if len(msg.ToolCalls) == 0 {
				req.Messages = append(req.Messages, msg)
				continue
			}
			// Ollama tools to call
			for _, toolCall := range msg.ToolCalls {
				toolCalls++
				d.publish(eventbus.BodyTypeStreamBlockStart, nil)
				d.publishTextBlock(fmt.Sprintf("🛠️ %s %v\n", toolCall.Function.Name, toolCall.Function.Arguments))

				fetchRequest := mcp.CallToolRequest{}
				fetchRequest.Request.Method = "tools/call"
				fetchRequest.Params.Name = toolCall.Function.Name
				fetchRequest.Params.Arguments = toolCall.Function.Arguments

				result, err := mcpsvr.CallTool(ctx, fetchRequest)
				if err != nil {
					d.publishTextBlock(fmt.Sprintf("Failed to call tool: %v", err))
					d.publish(eventbus.BodyTypeStreamBlockStop, nil)
					d.publish(eventbus.BodyTypeStreamMessageStop, nil)
					return
				}
				if result.IsError {
					buf, _ := json.Marshal(result)
					d.publishTextBlock(fmt.Sprintf("Error to call tool: %v", string(buf)))
					d.publish(eventbus.BodyTypeStreamBlockStop, nil)
					d.publish(eventbus.BodyTypeStreamMessageStop, nil)
					return
				}
				// display the text content of result
				d.publishTextBlock("🌍 call result:\n")
				for _, content := range result.Content {
					switch c := content.(type) {
					case mcp.TextContent:
						req.Messages = append(req.Messages, api.Message{Role: "tool", ToolName: toolCall.Function.Name, Content: c.Text})
						d.publishTextBlock(">>" + c.Text + "<<\n")
					default:
						d.publishTextBlock(fmt.Sprintf("😡 Unhandled content type from tool: %#v\n", c))
					}
				}
			}
		}
		d.publish(eventbus.BodyTypeStreamMessageStop, nil)
		if toolCalls == 0 {
			break
		}
	}
}
