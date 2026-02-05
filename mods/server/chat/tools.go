package chat

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/ollama/ollama/api"
)

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

func ConvertToOllamaTools(tools []mcp.Tool) []api.Tool {
	// Convert tools to Ollama format
	ollamaTools := make([]api.Tool, len(tools))
	for i, tool := range tools {
		ollamaTools[i] = api.Tool{
			Type: "function",
			Function: api.ToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters: api.ToolFunctionParameters{
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
func convertProperties(props map[string]interface{}) *api.ToolPropertiesMap {
	ret := api.NewToolPropertiesMap()

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

			ret.Set(name, prop)
		}
	}

	return ret
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
