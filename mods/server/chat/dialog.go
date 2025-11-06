package chat

import (
	"context"
	"fmt"
	"os"
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
	LoadConfig(ret, "ollama.json", false)
	LoadConfig(&ret.systemMessages, "system.json", true)
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
	LoadConfig(ret, "claude.json", false)
	LoadConfig(&ret.systemMessages, "system.json", true)
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
	Process(context.Context, string)
	Input(string)
	Control(string)
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

type ListModels struct {
	ConfigExist bool          `json:"config_exist"`
	Models      []LLMProvider `json:"models"`
}

func RpcLLMListModels() map[string]ListModels {
	llmProvidersMutex.Lock()
	defer llmProvidersMutex.Unlock()
	ret := map[string]ListModels{}
	for _, provider := range llmSupportedProviders {
		configExist := false
		models := []LLMProvider{}
		switch provider {
		case "claude":
			var cfg ClaudeConfig
			err := LoadConfig(&cfg, provider+".json", false)
			if err == nil {
				configExist = true
				models = llmProviders[provider]
			} else if err != os.ErrNotExist {
				logging.GetLog("chat").Error("Failed to load config for %s: %v", provider, err)
			}
		case "ollama":
			var cfg OllamaConfig
			err := LoadConfig(&cfg, provider+".json", false)
			if err == nil {
				configExist = true
				models = llmProviders[provider]
			} else if err != os.ErrNotExist {
				logging.GetLog("chat").Error("Failed to load config for %s: %v", provider, err)
			}
		}
		ret[provider] = ListModels{
			ConfigExist: configExist,
			Models:      models,
		}
	}
	return ret
}

func RpcLLMGetModels() (map[string][]LLMProvider, error) {
	llmProvidersMutex.Lock()
	defer llmProvidersMutex.Unlock()
	return llmProviders, nil
}

func RpcLLMAddModels(providers ...map[string]any) error {
	vals := []LLMProvider{}
	for _, p := range providers {
		nameVal, ok := p["name"]
		if !ok {
			return fmt.Errorf("missing name field")
		}
		name, ok := nameVal.(string)
		if !ok {
			return fmt.Errorf("invalid name field")
		}
		providerVal, ok := p["provider"]
		if !ok {
			return fmt.Errorf("missing provider field")
		}
		provider, ok := providerVal.(string)
		if !ok {
			return fmt.Errorf("invalid provider field")
		}
		modelVal, ok := p["model"]
		if !ok {
			return fmt.Errorf("missing model field")
		}
		model, ok := modelVal.(string)
		if !ok {
			return fmt.Errorf("invalid model field")
		}
		vals = append(vals, LLMProvider{
			Name:     name,
			Provider: provider,
			Model:    model,
		})
	}
	return RpcLLMAddModels0(vals)
}

func RpcLLMAddModels0(providers []LLMProvider) error {
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

func RpcLLMRemoveModels(providers ...map[string]any) error {
	vals := []LLMProvider{}
	for _, p := range providers {
		providerVal, ok := p["provider"]
		if !ok {
			return fmt.Errorf("missing provider field")
		}
		provider, ok := providerVal.(string)
		if !ok {
			return fmt.Errorf("invalid provider field")
		}
		modelVal, ok := p["model"]
		if !ok {
			return fmt.Errorf("missing model field")
		}
		model, ok := modelVal.(string)
		if !ok {
			return fmt.Errorf("invalid model field")
		}
		vals = append(vals, LLMProvider{
			Provider: provider,
			Model:    model,
		})
	}
	return RpcLLMRemoveModels0(vals)
}

func RpcLLMRemoveModels0(providers []LLMProvider) error {
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

type RpcLLMGetProviderConfigResponse struct {
	Provider string      `json:"provider"`
	Exist    bool        `json:"exist"`
	Config   interface{} `json:"config"`
}

func RpcLLMGetProviderConfig(provider string) (RpcLLMGetProviderConfigResponse, error) {
	var ret RpcLLMGetProviderConfigResponse
	var err error
	switch provider {
	case "claude":
		ret.Provider = provider
		cfg := NewClaudeConfig()
		err = LoadConfig(&cfg, provider+".json", false)
		if err == nil {
			cfg.MaskSensitive()
			ret.Exist = true
		}
		ret.Config = cfg
	case "ollama":
		ret.Provider = provider
		cfg := NewOllamaConfig()
		err = LoadConfig(&cfg, provider+".json", false)
		if err == nil {
			ret.Exist = true
		}
		ret.Config = cfg
	default:
		err = fmt.Errorf("unknown provider: %s", provider)
	}
	return ret, err
}

func RpcLLMSetProviderConfig(provider string, config any) error {
	if !isSupportedLLMProvider(provider) {
		return fmt.Errorf("unknown provider: %s", provider)
	}
	return SaveConfig(config, provider+".json")
}
