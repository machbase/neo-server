package chat

import (
	"context"
	"fmt"
)

func init() {
	llmProviders = loadLLMProviders()
}

type DialogConfig struct {
	Topic    string
	Provider string
	Model    string
	MsgID    int64
}

func (c DialogConfig) NewDialog() Dialog {
	if isTesting {
		return &TestingDialog{topic: c.Topic, msgID: c.MsgID, provider: c.Provider, model: c.Model}
	}
	var errorMsg string
	for _, p := range llmProviders {
		if p.Provider == c.Provider && p.Model == c.Model {
			switch p.Provider {
			case "claude":
				ret := NewClaudeDialog(c.Topic, c.MsgID, c.Model)
				LoadConfig(ret, "claude.json")
				return ret
			case "ollama":
				ret := NewOllamaDialog(c.Topic, c.MsgID, c.Model)
				LoadConfig(ret, "ollama.json")
				return ret
			default:
				errorMsg = fmt.Sprintf("Unknown LLM provider: %s, model: %s", c.Provider, c.Model)
			}
		}
	}
	return &UnknownDialog{
		topic:    c.Topic,
		provider: c.Provider,
		model:    c.Model,
		msgID:    c.MsgID,
		error:    errorMsg,
	}
}

type Dialog interface {
	Talk(context.Context, string)
}

var (
	_ Dialog = (*UnknownDialog)(nil)
	_ Dialog = (*TestingDialog)(nil)
	_ Dialog = (*ClaudeDialog)(nil)
	_ Dialog = (*OllamaDialog)(nil)
)

type LLMProvider struct {
	Name     string `json:"name"`     // display name
	Provider string `json:"provider"` // "claude" or "ollama"
	Model    string `json:"model"`    // model identifier
}

var llmProviders = []LLMProvider{}
var llmFallbackProviders = []LLMProvider{
	{Name: "Claude Sonnet 4", Provider: "claude", Model: "claude-sonnet-4-20250514"},
	{Name: "Ollama qwen3:0.6b", Provider: "ollama", Model: "qwen3:0.6b"},
}

func RpcLLMGetProviders() ([]LLMProvider, error) {
	if isTesting {
		return llmFallbackProviders, nil
	}
	return llmProviders, nil
}
func RpcLLMGetClaudeConfig() (ClaudeConfig, error) {
	ret := NewClaudeConfig()
	if err := LoadConfig(&ret, "claude.json"); err != nil {
		return ret, err
	}
	return ret, nil
}

func RpcLLMGetOllamaConfig() (OllamaConfig, error) {
	ret := NewOllamaConfig()
	if err := LoadConfig(&ret, "ollama.json"); err != nil {
		return ret, err
	}
	return ret, nil
}

func RpcLLMSetClaudeConfig(config ClaudeConfig) error {
	return SaveConfig(config, "claude.json")
}

func RpcLLMSetOllamaConfig(config OllamaConfig) error {
	return SaveConfig(config, "ollama.json")
}
