package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

type LLMClaudeConfig struct {
	Key string `json:"key"`
}

func (d *LLMDialog) execClaude(ctx context.Context) {
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
	d.SendMessage("🌍 MCP Server Info: %s %s\n", initResult.ServerInfo.Name, initResult.ServerInfo.Version)

	d.SendMessage("Use claude model\n")

	// Get the list of tools
	toolsRequest := mcp.ListToolsRequest{}
	tools, err := mcpClient.ListTools(ctx, toolsRequest)
	if err != nil {
		d.SendError("Failed to list tools: %v", err)
		return
	}

	// Have a "tool chat"
	// Prompt construction
	client := anthropic.NewClient(
		option.WithAPIKey(d.conf.Claude.Key),
	)
	request := anthropic.MessageNewParams{
		Model:      anthropic.ModelClaudeSonnet4_20250514,
		MaxTokens:  1024,
		Tools:      ConvertToClaudeTools(tools.Tools),
		ToolChoice: anthropic.ToolChoiceUnionParam{OfAuto: &anthropic.ToolChoiceAutoParam{}},
	}
	systemMessages := []anthropic.TextBlockParam{
		{Text: `당신은 한국어로 대화하는 친근한 Machbase Neo DB의 AI 어시스턴트입니다.
답변에 대한 규칙은 아래와 같습니다.
1. 응답 전체를 무조건 순수한 JSON 형식으로만 답변.
2. 마크다운 코드블록 사용 금지.`},
	}
	requestMessages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(d.userMessage)),
	}

repeat:
	request.System = systemMessages
	request.Messages = requestMessages

	message, err := client.Messages.New(ctx, request)
	if err != nil {
		fmt.Println("Error creating message:", err)
		d.SendError("😡 Failed to accumulate message: %v\n", err)
		return
	}
	if d.log.DebugEnabled() {
		buf := &bytes.Buffer{}
		enc := json.NewEncoder(buf)
		enc.SetIndent("", "  ")
		enc.Encode(message)
		d.log.Debug("User message:", d.userMessage)
		d.log.Debug("Claude response:", buf.String())
	}
	d.SendMessage("Claude response: \n")
	switch message.StopReason {
	default:
	// case anthropic.StopReasonEndTurn:
	// case anthropic.StopReasonMaxTokens:
	// case anthropic.StopReasonStopSequence:
	// case anthropic.StopReasonPauseTurn:
	// case anthropic.StopReasonRefusal:
	case anthropic.StopReasonToolUse:
		content, callId, callResult := d.handleToolUse(ctx, message, mcpClient)
		requestMessages = append(requestMessages, anthropic.MessageParam{
			Role: anthropic.MessageParamRoleAssistant,
			Content: []anthropic.ContentBlockParamUnion{
				{
					OfText: &anthropic.TextBlockParam{Text: content},
				},
			},
		})
		requestMessages = append(requestMessages, anthropic.MessageParam{
			Role: anthropic.MessageParamRoleUser,
			Content: []anthropic.ContentBlockParamUnion{
				{
					OfToolResult: &anthropic.ToolResultBlockParam{
						ToolUseID: callId,
						Content: []anthropic.ToolResultBlockParamContentUnion{
							{
								OfText: &anthropic.TextBlockParam{Text: callResult},
							},
						},
					},
				},
			},
		})
		// go back to repeat
		goto repeat
	}
}

func (d *LLMDialog) handleToolUse(ctx context.Context, message *anthropic.Message, mcpClient *client.Client) (content string, toolUseId string, toolResult string) {
	for _, contentBlock := range message.Content {
		switch block := contentBlock.AsAny().(type) {
		default:
			fmt.Printf("😡 Unhandled block type from Claude: %#v\n", block)
		case anthropic.TextBlock:
			content = block.Text
			d.SendMessage(fmt.Sprintf("📝 Claude message:\n<pre>%s</pre>\n", block.Text))
		case anthropic.ToolUseBlock:
			// 🖐️ Call the mcp server
			inputJSON, _ := json.Marshal(block.Input)
			d.SendMessage("🛠️ Claude Tool - %q\n<pre>%s</pre>\n", block.Name, inputJSON)

			fetchRequest := mcp.CallToolRequest{}
			fetchRequest.Request.Method = "tools/call"
			fetchRequest.Params.Name = block.Name
			fetchRequest.Params.Arguments = block.Input

			result, err := mcpClient.CallTool(ctx, fetchRequest)
			if err != nil {
				d.SendError("😡 Failed to call tool: %v", err)
				continue
			}
			// display the text content of result
			d.SendMessage("🧾 call result:\n")
			for _, content := range result.Content {
				switch c := content.(type) {
				case mcp.TextContent:
					d.SendMessage("<pre>" + c.Text + "</pre>\n")
					toolUseId = contentBlock.ToolUseID
					toolResult = c.Text
				default:
					d.SendError("😡 Unhandled content type from tool: %#v\n", c)
				}
			}
		}
	}
	return
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
