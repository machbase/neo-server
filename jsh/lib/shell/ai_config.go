package shell

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LLMConfig is the on-disk structure stored in
// $HOME/.config/machbase/llm/config.json.
type LLMConfig struct {
	DefaultProvider string                      `json:"defaultProvider"`
	Providers       map[string]*LLMProviderConf `json:"providers"`
	Exec            LLMExecConf                 `json:"exec"`
	Prompt          LLMPromptConf               `json:"prompt"`
}

type LLMProviderConf struct {
	APIKey    string `json:"apiKey"`
	BaseURL   string `json:"baseUrl"`
	Model     string `json:"model"`
	MaxTokens int    `json:"maxTokens"`
}

type LLMLastConfig struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type LLMExecConf struct {
	MaxRows   int  `json:"maxRows"`
	TimeoutMs int  `json:"timeoutMs"`
	ReadOnly  bool `json:"readOnly"`
}

type LLMPromptConf struct {
	Segments  []string `json:"segments"`
	CustomDir string   `json:"customDir"`
}

type LLMRequest struct {
	Messages     []LLMMessage
	SystemPrompt string
	Model        string
	MaxTokens    int
}

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LLMResponse struct {
	Content      string `json:"content"`
	InputTokens  int    `json:"inputTokens"`
	OutputTokens int    `json:"outputTokens"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
}

type LLMProvider interface {
	send(ctx context.Context, req LLMRequest) (*LLMResponse, error)
	stream(ctx context.Context, req LLMRequest, onToken func(token string)) (*LLMResponse, error)
	name() string
	model() string
}

func DefaultLLMConfig() *LLMConfig {
	return &LLMConfig{
		DefaultProvider: "claude",
		Providers: map[string]*LLMProviderConf{
			"claude": {
				MaxTokens: 8192,
			},
			"openai": {
				BaseURL:   "https://api.openai.com/v1",
				MaxTokens: 8192,
			},
			"ollama": {
				BaseURL:   "http://127.0.0.1:11434",
				MaxTokens: 8192,
			},
		},
		Exec: LLMExecConf{
			MaxRows:   1000,
			TimeoutMs: 30000,
			ReadOnly:  true,
		},
		Prompt: LLMPromptConf{
			Segments: []string{"jsh-runtime", "jsh-modules", "agent-api", "machbase-sql"},
		},
	}
}

func DefaultLastConfig() *LLMLastConfig {
	return &LLMLastConfig{}
}

func LLMConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "machbase", "llm", "config.json"), nil
}

func LLMLastConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "machbase", "llm", "last.config"), nil
}

func LLMCustomPromptDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "machbase", "llm", "prompts"), nil
}

func LoadLLMConfig() (*LLMConfig, error) {
	path, err := LLMConfigPath()
	if err != nil {
		return DefaultLLMConfig(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultLLMConfig(), nil
		}
		return nil, err
	}
	cfg := DefaultLLMConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config parse error: %w", err)
	}
	SanitizeLLMConfig(cfg)
	return cfg, nil
}

func SaveLLMConfig(cfg *LLMConfig) error {
	SanitizeLLMConfig(cfg)
	path, err := LLMConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func LoadLastConfig() (*LLMLastConfig, error) {
	path, err := LLMLastConfigPath()
	if err != nil {
		return DefaultLastConfig(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultLastConfig(), nil
		}
		return nil, err
	}
	cfg := DefaultLastConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("last config parse error: %w", err)
	}
	return cfg, nil
}

func SaveLastConfig(cfg *LLMLastConfig) error {
	path, err := LLMLastConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func SetDotKey(cfg *LLMConfig, key string, value string) error {
	if isProviderModelKey(key) {
		return fmt.Errorf("provider model is stored in last.config; use \\model instead")
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	parts := strings.Split(key, ".")
	cur := m
	for i, part := range parts {
		if i == len(parts)-1 {
			var v any
			if err := json.Unmarshal([]byte(value), &v); err == nil {
				cur[part] = v
			} else {
				cur[part] = value
			}
		} else {
			sub, ok := cur[part]
			if !ok {
				sub = map[string]any{}
				cur[part] = sub
			}
			next, ok := sub.(map[string]any)
			if !ok {
				return fmt.Errorf("key %q is not an object", strings.Join(parts[:i+1], "."))
			}
			cur = next
		}
	}
	merged, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(merged, cfg); err != nil {
		return err
	}
	SanitizeLLMConfig(cfg)
	return nil
}

func SanitizeLLMConfig(cfg *LLMConfig) {
	if cfg == nil {
		return
	}
	if cfg.Providers == nil {
		cfg.Providers = map[string]*LLMProviderConf{}
	}
	for _, name := range supportedProviderNames() {
		if cfg.Providers[name] == nil {
			cfg.Providers[name] = &LLMProviderConf{}
		}
		cfg.Providers[name].Model = ""
		if cfg.Providers[name].MaxTokens == 0 {
			cfg.Providers[name].MaxTokens = 8192
		}
	}
	if cfg.Providers["openai"].BaseURL == "" {
		cfg.Providers["openai"].BaseURL = "https://api.openai.com/v1"
	}
	if cfg.Providers["ollama"].BaseURL == "" {
		cfg.Providers["ollama"].BaseURL = "http://127.0.0.1:11434"
	}
	if _, err := normalizeProviderName(cfg.DefaultProvider); err != nil {
		cfg.DefaultProvider = "claude"
	}
	if cfg.Exec.MaxRows == 0 {
		cfg.Exec.MaxRows = 1000
	}
	if cfg.Exec.TimeoutMs == 0 {
		cfg.Exec.TimeoutMs = 30000
	}
	if len(cfg.Prompt.Segments) == 0 {
		cfg.Prompt.Segments = []string{"jsh-runtime", "jsh-modules", "agent-api", "machbase-sql"}
	}
}

func isProviderModelKey(key string) bool {
	parts := strings.Split(key, ".")
	return len(parts) == 3 && parts[0] == "providers" && parts[2] == "model"
}

func supportedProviderNames() []string {
	return []string{"claude", "openai", "ollama"}
}

func defaultModelForProvider(name string) string {
	switch name {
	case "openai":
		return "gpt-4o"
	case "ollama":
		return "llama3.1"
	default:
		return "claude-opus-4-5"
	}
}

func normalizeProviderName(name string) (string, error) {
	switch name {
	case "claude", "openai", "ollama":
		return name, nil
	default:
		return "", fmt.Errorf("unsupported provider: %s", name)
	}
}

// Backward-compatible internal aliases used by existing ai module and tests.
type llmConfig = LLMConfig
type llmProviderConf = LLMProviderConf
type llmLastConfig = LLMLastConfig
type llmExecConf = LLMExecConf
type llmPromptConf = LLMPromptConf
type llmRequest = LLMRequest
type llmMessage = LLMMessage
type llmResponse = LLMResponse
type llmProvider = LLMProvider

func defaultLLMConfig() *llmConfig                      { return DefaultLLMConfig() }
func defaultLastConfig() *llmLastConfig                 { return DefaultLastConfig() }
func loadLLMConfig() (*llmConfig, error)                { return LoadLLMConfig() }
func saveLLMConfig(cfg *llmConfig) error                { return SaveLLMConfig(cfg) }
func loadLastConfig() (*llmLastConfig, error)           { return LoadLastConfig() }
func saveLastConfig(cfg *llmLastConfig) error           { return SaveLastConfig(cfg) }
func llmConfigPath() (string, error)                    { return LLMConfigPath() }
func llmLastConfigPath() (string, error)                { return LLMLastConfigPath() }
func llmCustomPromptDir() (string, error)               { return LLMCustomPromptDir() }
func setDotKey(cfg *llmConfig, key, value string) error { return SetDotKey(cfg, key, value) }
func sanitizeLLMConfig(cfg *llmConfig)                  { SanitizeLLMConfig(cfg) }
