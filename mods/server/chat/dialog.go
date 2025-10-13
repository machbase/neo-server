package chat

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/machbase/neo-server/v8/mods/logging"
)

type DialogConfig struct {
	Topic    string
	Provider string
	Model    string
	MsgID    int64
	Session  string
}

func (c DialogConfig) NewDialog() Dialog {
	if isTesting {
		return c.NewTest()
	}
	for _, providers := range llmProviders {
		for _, p := range providers {
			if p.Provider == c.Provider && p.Model == c.Model {
				switch p.Provider {
				case "claude":
					return c.NewClaude()
				case "ollama":
					return c.NewOllama()
				}
			}
		}
	}
	return c.NewUnknown()
}

func (c DialogConfig) NewOllama() *OllamaDialog {
	const systemMessage = "You are a friendly AI assistant for Machbase Neo DB."
	ret := &OllamaDialog{
		OllamaConfig:   NewOllamaConfig(),
		systemMessages: []string{systemMessage},
		topic:          c.Topic,
		session:        c.Session,
		msgID:          c.MsgID,
		model:          c.Model,
		log:            logging.GetLog("chat.ollama"),
	}
	LoadConfig(ret, "ollama.json")
	LoadConfig(ret.systemMessages, "system.json")
	ret.systemMessages, _ = loadSystemMessages(ret.systemMessages)
	return ret
}

func (c DialogConfig) NewClaude() *ClaudeDialog {
	const systemMessage = "You are a friendly AI assistant for Machbase Neo DB."
	ret := &ClaudeDialog{
		ClaudeConfig:   NewClaudeConfig(),
		systemMessages: []string{systemMessage},
		topic:          c.Topic,
		session:        c.Session,
		msgID:          c.MsgID,
		model:          c.Model,
		log:            logging.GetLog("chat.claude"),
	}
	LoadConfig(ret, "claude.json")
	LoadConfig(ret.systemMessages, "system.json")
	ret.systemMessages, _ = loadSystemMessages(ret.systemMessages)
	return ret
}

func (c DialogConfig) NewTest() *TestingDialog {
	return &TestingDialog{
		topic:    c.Topic,
		session:  c.Session,
		msgID:    c.MsgID,
		provider: c.Provider,
		model:    c.Model,
	}
}

func (c DialogConfig) NewUnknown() *UnknownDialog {
	errorMsg := fmt.Sprintf("Unknown LLM provider: %s, model: %s", c.Provider, c.Model)
	return &UnknownDialog{
		topic:    c.Topic,
		provider: c.Provider,
		model:    c.Model,
		session:  c.Session,
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

var llmProviders = map[string][]LLMProvider{}
var llmProvidersMutex sync.Mutex

var llmFallbackProviders = map[string][]LLMProvider{
	"claude": {
		{Name: "Claude Sonnet 4", Provider: "claude", Model: "claude-sonnet-4-20250514"},
	},
	"ollama": {
		{Name: "Ollama qwen3:0.6b", Provider: "ollama", Model: "qwen3:0.6b"},
	},
}
var llmSupportedProviders = []string{"claude", "ollama"}

func isSupportedLLMProvider(provider string) bool {
	return slices.Contains(llmSupportedProviders, provider)
}

func RpcLLMGetModels() (map[string][]LLMProvider, error) {
	llmProvidersMutex.Lock()
	defer llmProvidersMutex.Unlock()
	return llmProviders, nil
}

func RpcLLMAddModels(providers []LLMProvider) error {
	llmProvidersMutex.Lock()
	defer llmProvidersMutex.Unlock()
	for _, p := range providers {
		if !isSupportedLLMProvider(p.Provider) {
			return fmt.Errorf("unknown provider: %s", p.Provider)
		}
		llmProviders[p.Provider] = append(llmProviders[p.Provider], p)
	}
	SaveConfig(llmProviders, "models.json")
	return nil
}

func RpcLLMRemoveModels(providers []LLMProvider) error {
	llmProvidersMutex.Lock()
	defer llmProvidersMutex.Unlock()
	for _, p := range providers {
		if !isSupportedLLMProvider(p.Provider) {
			return fmt.Errorf("unknown provider: %s", p.Provider)
		}
		if ps, ok := llmProviders[p.Provider]; ok {
			for i, exist := range ps {
				if exist.Model == p.Model {
					llmProviders[p.Provider] = append(ps[:i], ps[i+1:]...)
					break
				}
			}
		}
	}
	SaveConfig(llmProviders, "models.json")
	return nil
}

func RpcLLMGetProviders() []string {
	return llmSupportedProviders
}

func RpcLLMGetProviderConfigTemplate(provider string) (any, error) {
	if !isSupportedLLMProvider(provider) {
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
	switch provider {
	case "claude":
		return NewClaudeConfig(), nil
	case "ollama":
		return NewOllamaConfig(), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

func RpcLLMGetProviderConfig(provider string) (any, error) {
	var ret any
	var err error
	switch provider {
	case "claude":
		cfg := NewClaudeConfig()
		err = LoadConfig(&cfg, provider+".json")
		cfg.MaskSensitive()
		ret = cfg
	case "ollama":
		cfg := NewOllamaConfig()
		err = LoadConfig(&cfg, provider+".json")
		ret = cfg
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
	if err != nil {
		return ret, err
	}
	return ret, nil
}

func RpcLLMSetProviderConfig(provider string, config any) error {
	if !isSupportedLLMProvider(provider) {
		return fmt.Errorf("unknown provider: %s", provider)
	}
	return SaveConfig(config, provider+".json")
}
