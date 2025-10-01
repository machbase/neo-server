package mcpsvr

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func NewClient(ctx context.Context, endpoint string) (*client.Client, error) {
	var ret *client.Client
	if endpoint == "" {
		return nil, nil
	}
	mcpClient, err := client.NewSSEMCPClient(endpoint)
	if err == nil {
		if err = mcpClient.Start(ctx); err != nil {
			return nil, err
		}
		ret = mcpClient
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
		return nil, err
	}
	_ = initResult
	return ret, nil
}

func CallTool(ctx context.Context, toolCall mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if toolCall.Request.Method != "tools/call" {
		return mcp.NewToolResultError("invalid method: " + toolCall.Request.Method), nil
	}
	// direct mode, find the tool from registered tools
	for _, tool := range registeredTools {
		if tool.Tool.Name == toolCall.Params.Name {
			switch raw := toolCall.Params.Arguments.(type) {
			case map[string]interface{}: // ok
			case json.RawMessage:
				var args map[string]interface{}
				if err := json.Unmarshal(raw, &args); err == nil {
					toolCall.Params.Arguments = args
				}
			default:
				return mcp.NewToolResultError(fmt.Sprintf("tool invalid arguments: %s, %T", toolCall.Params.Name, raw)), nil
			}
			return tool.Handler(ctx, toolCall)
		}
	}
	return mcp.NewToolResultError("tool not found: " + toolCall.Params.Name), nil
}

func ListTools(ctx context.Context) (*mcp.ListToolsResult, error) {
	// direct mode, return registered tools
	ret := &mcp.ListToolsResult{}
	for _, tool := range registeredTools {
		ret.Tools = append(ret.Tools, tool.Tool)
	}
	return ret, nil
}
