package shell

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	jshlog "github.com/machbase/neo-server/v8/jsh/log"
)

// ─── Config ──────────────────────────────────────────────────────────────────

// llmConfig is the on-disk structure stored in
// $HOME/.config/machbase/llm/config.json
type llmConfig struct {
	DefaultProvider string                      `json:"defaultProvider"`
	Providers       map[string]*llmProviderConf `json:"providers"`
	Exec            llmExecConf                 `json:"exec"`
	Prompt          llmPromptConf               `json:"prompt"`
}

type llmProviderConf struct {
	APIKey    string `json:"apiKey"`
	BaseURL   string `json:"baseUrl"`
	Model     string `json:"model"`
	MaxTokens int    `json:"maxTokens"`
}

type llmLastConfig struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type llmExecConf struct {
	MaxRows   int  `json:"maxRows"`
	TimeoutMs int  `json:"timeoutMs"`
	ReadOnly  bool `json:"readOnly"`
}

type llmPromptConf struct {
	Segments  []string `json:"segments"`
	CustomDir string   `json:"customDir"`
}

func defaultLLMConfig() *llmConfig {
	return &llmConfig{
		DefaultProvider: "claude",
		Providers: map[string]*llmProviderConf{
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
		Exec: llmExecConf{
			MaxRows:   1000,
			TimeoutMs: 30000,
			ReadOnly:  true,
		},
		Prompt: llmPromptConf{
			Segments: []string{"jsh-runtime", "jsh-modules", "agent-api", "machbase-sql"},
		},
	}
}

func defaultLastConfig() *llmLastConfig {
	return &llmLastConfig{}
}

func llmConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "machbase", "llm", "config.json"), nil
}

func llmLastConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "machbase", "llm", "last.config"), nil
}

func llmCustomPromptDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "machbase", "llm", "prompts"), nil
}

func loadLLMConfig() (*llmConfig, error) {
	path, err := llmConfigPath()
	if err != nil {
		return defaultLLMConfig(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultLLMConfig(), nil
		}
		return nil, err
	}
	cfg := defaultLLMConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config parse error: %w", err)
	}
	sanitizeLLMConfig(cfg)
	return cfg, nil
}

func saveLLMConfig(cfg *llmConfig) error {
	sanitizeLLMConfig(cfg)
	path, err := llmConfigPath()
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

func loadLastConfig() (*llmLastConfig, error) {
	path, err := llmLastConfigPath()
	if err != nil {
		return defaultLastConfig(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultLastConfig(), nil
		}
		return nil, err
	}
	cfg := defaultLastConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("last config parse error: %w", err)
	}
	return cfg, nil
}

func saveLastConfig(cfg *llmLastConfig) error {
	path, err := llmLastConfigPath()
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

func sanitizeLLMConfig(cfg *llmConfig) {
	if cfg == nil {
		return
	}
	if cfg.Providers == nil {
		cfg.Providers = map[string]*llmProviderConf{}
	}
	for _, name := range supportedProviderNames() {
		if cfg.Providers[name] == nil {
			cfg.Providers[name] = &llmProviderConf{}
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

// setDotKey sets a value in cfg using dot-notation key (e.g. "providers.claude.model").
func setDotKey(cfg *llmConfig, key string, value string) error {
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
			// Try to parse as number or bool, fall back to string
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
	sanitizeLLMConfig(cfg)
	return nil
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

// ─── LLM Provider interface ──────────────────────────────────────────────────

type llmRequest struct {
	Messages     []llmMessage
	SystemPrompt string
	Model        string
	MaxTokens    int
}

type llmMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type llmResponse struct {
	Content      string `json:"content"`
	InputTokens  int    `json:"inputTokens"`
	OutputTokens int    `json:"outputTokens"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
}

type llmProvider interface {
	send(ctx context.Context, req llmRequest) (*llmResponse, error)
	stream(ctx context.Context, req llmRequest, onToken func(token string)) (*llmResponse, error)
	name() string
	model() string
}

// ─── Claude provider ─────────────────────────────────────────────────────────

type claudeProvider struct {
	apiKey    string
	modelName string
	maxTokens int
}

func newClaudeProvider(conf *llmProviderConf) *claudeProvider {
	apiKey := conf.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	model := conf.Model
	if model == "" {
		model = "claude-opus-4-5"
	}
	maxTokens := conf.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8192
	}
	return &claudeProvider{apiKey: apiKey, modelName: model, maxTokens: maxTokens}
}

func (p *claudeProvider) name() string  { return "claude" }
func (p *claudeProvider) model() string { return p.modelName }

func (p *claudeProvider) buildBody(req llmRequest, stream bool) ([]byte, error) {
	messages := make([]map[string]string, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = map[string]string{"role": m.Role, "content": m.Content}
	}
	model := req.Model
	if model == "" {
		model = p.modelName
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = p.maxTokens
	}
	body := map[string]any{
		"model":      model,
		"max_tokens": maxTokens,
		"system":     req.SystemPrompt,
		"messages":   messages,
		"stream":     stream,
	}
	return json.Marshal(body)
}

func (p *claudeProvider) doRequest(ctx context.Context, bodyBytes []byte) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	return http.DefaultClient.Do(httpReq)
}

func (p *claudeProvider) send(ctx context.Context, req llmRequest) (*llmResponse, error) {
	body, err := p.buildBody(req, false)
	if err != nil {
		return nil, err
	}
	resp, err := p.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("claude API error %d: %s", resp.StatusCode, string(data))
	}
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		Model string `json:"model"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	content := ""
	if len(result.Content) > 0 {
		content = result.Content[0].Text
	}
	return &llmResponse{
		Content:      content,
		InputTokens:  result.Usage.InputTokens,
		OutputTokens: result.Usage.OutputTokens,
		Provider:     "claude",
		Model:        result.Model,
	}, nil
}

func (p *claudeProvider) stream(ctx context.Context, req llmRequest, onToken func(string)) (*llmResponse, error) {
	body, err := p.buildBody(req, true)
	if err != nil {
		return nil, err
	}
	resp, err := p.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("claude API error %d: %s", resp.StatusCode, string(data))
	}

	var totalIn, totalOut int
	var finalModel string
	var sb strings.Builder

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}
		var event struct {
			Type  string `json:"type"`
			Index int    `json:"index"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
			Message struct {
				Model string `json:"model"`
				Usage struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}
		switch event.Type {
		case "message_start":
			finalModel = event.Message.Model
			totalIn = event.Message.Usage.InputTokens
		case "content_block_delta":
			if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
				onToken(event.Delta.Text)
				sb.WriteString(event.Delta.Text)
			}
		case "message_delta":
			totalOut = event.Usage.OutputTokens
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &llmResponse{
		Content:      sb.String(),
		InputTokens:  totalIn,
		OutputTokens: totalOut,
		Provider:     "claude",
		Model:        finalModel,
	}, nil
}

// ─── OpenAI provider (stub) ──────────────────────────────────────────────────

type openaiProvider struct {
	baseURL   string
	apiKey    string
	modelName string
	maxTokens int
}

func newOpenAIProvider(conf *llmProviderConf) *openaiProvider {
	baseURL := conf.BaseURL
	if baseURL == "" {
		baseURL = os.Getenv("OPENAI_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	apiKey := conf.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	model := conf.Model
	if model == "" {
		model = "gpt-4o"
	}
	maxTokens := conf.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8192
	}
	return &openaiProvider{baseURL: baseURL, apiKey: apiKey, modelName: model, maxTokens: maxTokens}
}

func (p *openaiProvider) name() string  { return "openai" }
func (p *openaiProvider) model() string { return p.modelName }

func (p *openaiProvider) buildBody(req llmRequest, stream bool) ([]byte, error) {
	model := req.Model
	if model == "" {
		model = p.modelName
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = p.maxTokens
	}

	messages := make([]map[string]string, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": req.SystemPrompt,
		})
	}
	for _, m := range req.Messages {
		messages = append(messages, map[string]string{
			"role":    m.Role,
			"content": m.Content,
		})
	}

	body := map[string]any{
		"model":      model,
		"messages":   messages,
		"max_tokens": maxTokens,
		"stream":     stream,
	}
	if stream {
		body["stream_options"] = map[string]any{
			"include_usage": true,
		}
	}
	return json.Marshal(body)
}

func (p *openaiProvider) doRequest(ctx context.Context, bodyBytes []byte) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	return http.DefaultClient.Do(httpReq)
}

func (p *openaiProvider) send(ctx context.Context, req llmRequest) (*llmResponse, error) {
	body, err := p.buildBody(req, false)
	if err != nil {
		return nil, err
	}
	resp, err := p.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai API error %d: %s", resp.StatusCode, string(data))
	}

	var result struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	content := ""
	if len(result.Choices) > 0 {
		content = result.Choices[0].Message.Content
	}
	return &llmResponse{
		Content:      content,
		InputTokens:  result.Usage.PromptTokens,
		OutputTokens: result.Usage.CompletionTokens,
		Provider:     "openai",
		Model:        result.Model,
	}, nil
}

func (p *openaiProvider) stream(ctx context.Context, req llmRequest, onToken func(string)) (*llmResponse, error) {
	body, err := p.buildBody(req, true)
	if err != nil {
		return nil, err
	}
	resp, err := p.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai API error %d: %s", resp.StatusCode, string(data))
	}

	var totalIn, totalOut int
	var finalModel string
	var sb strings.Builder

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}

		var event struct {
			Model   string `json:"model"`
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}

		if event.Model != "" {
			finalModel = event.Model
		}
		if len(event.Choices) > 0 {
			token := event.Choices[0].Delta.Content
			if token != "" {
				onToken(token)
				sb.WriteString(token)
			}
		}
		if event.Usage != nil {
			totalIn = event.Usage.PromptTokens
			totalOut = event.Usage.CompletionTokens
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &llmResponse{
		Content:      sb.String(),
		InputTokens:  totalIn,
		OutputTokens: totalOut,
		Provider:     "openai",
		Model:        finalModel,
	}, nil
}

// ─── Ollama provider ─────────────────────────────────────────────────────────

type ollamaProvider struct {
	baseURL   string
	modelName string
	maxTokens int
}

func newOllamaProvider(conf *llmProviderConf) *ollamaProvider {
	baseURL := conf.BaseURL
	if baseURL == "" {
		baseURL = os.Getenv("OLLAMA_HOST")
	}
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	if !strings.Contains(baseURL, "://") {
		baseURL = "http://" + baseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	model := conf.Model
	if model == "" {
		model = "llama3.1"
	}
	maxTokens := conf.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8192
	}
	return &ollamaProvider{baseURL: baseURL, modelName: model, maxTokens: maxTokens}
}

func (p *ollamaProvider) name() string  { return "ollama" }
func (p *ollamaProvider) model() string { return p.modelName }

func (p *ollamaProvider) buildBody(req llmRequest, stream bool) ([]byte, error) {
	model := req.Model
	if model == "" {
		model = p.modelName
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = p.maxTokens
	}

	messages := make([]map[string]string, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": req.SystemPrompt,
		})
	}
	for _, m := range req.Messages {
		messages = append(messages, map[string]string{
			"role":    m.Role,
			"content": m.Content,
		})
	}

	body := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   stream,
		"options": map[string]any{
			"num_predict": maxTokens,
		},
	}
	return json.Marshal(body)
}

func (p *ollamaProvider) doRequest(ctx context.Context, bodyBytes []byte) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/api/chat", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(httpReq)
}

func (p *ollamaProvider) send(ctx context.Context, req llmRequest) (*llmResponse, error) {
	body, err := p.buildBody(req, false)
	if err != nil {
		return nil, err
	}
	resp, err := p.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama API error %d: %s", resp.StatusCode, string(data))
	}

	var result struct {
		Model   string `json:"model"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		PromptEvalCount int `json:"prompt_eval_count"`
		EvalCount       int `json:"eval_count"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &llmResponse{
		Content:      result.Message.Content,
		InputTokens:  result.PromptEvalCount,
		OutputTokens: result.EvalCount,
		Provider:     "ollama",
		Model:        result.Model,
	}, nil
}

func (p *ollamaProvider) stream(ctx context.Context, req llmRequest, onToken func(string)) (*llmResponse, error) {
	body, err := p.buildBody(req, true)
	if err != nil {
		return nil, err
	}
	resp, err := p.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API error %d: %s", resp.StatusCode, string(data))
	}

	var totalIn, totalOut int
	var finalModel string
	var sb strings.Builder

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event struct {
			Model   string `json:"model"`
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Done            bool `json:"done"`
			PromptEvalCount int  `json:"prompt_eval_count"`
			EvalCount       int  `json:"eval_count"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event.Model != "" {
			finalModel = event.Model
		}
		if event.Message.Content != "" {
			onToken(event.Message.Content)
			sb.WriteString(event.Message.Content)
		}
		if event.Done {
			totalIn = event.PromptEvalCount
			totalOut = event.EvalCount
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &llmResponse{
		Content:      sb.String(),
		InputTokens:  totalIn,
		OutputTokens: totalOut,
		Provider:     "ollama",
		Model:        finalModel,
	}, nil
}

// ─── aiModule — state held per JS runtime ───────────────────────────────────

type aiModule struct {
	rt           *goja.Runtime
	cfg          *llmConfig
	last         *llmLastConfig
	provider     llmProvider
	activeName   string
	activeModel  string
	activeCancel context.CancelFunc
	cancelMu     sync.Mutex
	promptFS     fs.ReadFileFS // builtin ai_prompts embedded FS
}

func newAIModule(rt *goja.Runtime, promptFS fs.ReadFileFS) *aiModule {
	cfg, err := loadLLMConfig()
	if err != nil || cfg == nil {
		cfg = defaultLLMConfig()
	}
	last, err := loadLastConfig()
	if err != nil || last == nil {
		last = defaultLastConfig()
	}
	m := &aiModule{rt: rt, cfg: cfg, last: last, promptFS: promptFS}
	provider, model := m.resolveSelection()
	m.applySelection(provider, model)
	return m
}

func (m *aiModule) buildProvider(name string) llmProvider {
	conf, ok := m.cfg.Providers[name]
	if !ok {
		conf = &llmProviderConf{}
	}
	switch name {
	case "openai":
		return newOpenAIProvider(conf)
	case "ollama":
		return newOllamaProvider(conf)
	default: // "claude" and fallback
		return newClaudeProvider(conf)
	}
}

func (m *aiModule) resolveSelection() (string, string) {
	provider := m.cfg.DefaultProvider
	if _, err := normalizeProviderName(provider); err != nil {
		provider = "claude"
	}
	if m.last != nil && m.last.Provider != "" {
		if name, err := normalizeProviderName(m.last.Provider); err == nil {
			provider = name
		}
	}
	model := defaultModelForProvider(provider)
	if m.last != nil && m.last.Provider == provider && m.last.Model != "" {
		model = m.last.Model
	}
	return provider, model
}

func (m *aiModule) applySelection(provider string, model string) {
	m.activeName = provider
	m.activeModel = model
	m.provider = m.buildProvider(provider)
}

func (m *aiModule) rebuildActiveProvider() {
	provider := m.activeName
	if _, err := normalizeProviderName(provider); err != nil {
		provider = m.cfg.DefaultProvider
	}
	model := m.activeModel
	if model == "" {
		model = defaultModelForProvider(provider)
	}
	m.applySelection(provider, model)
}

func (m *aiModule) ensureActiveSelection() {
	if m.activeName != "" && m.activeModel != "" && m.provider != nil {
		return
	}
	if m.provider != nil {
		name := m.provider.name()
		model := m.activeModel
		if model == "" {
			if providerModel := m.provider.model(); providerModel != "" {
				model = providerModel
			} else {
				model = defaultModelForProvider(name)
			}
		}
		if _, err := normalizeProviderName(name); err == nil {
			m.applySelection(name, model)
		} else {
			m.activeName = name
			m.activeModel = model
		}
		return
	}
	provider, model := m.resolveSelection()
	m.applySelection(provider, model)
}

func (m *aiModule) setActiveCancel(cancel context.CancelFunc) {
	m.cancelMu.Lock()
	defer m.cancelMu.Unlock()
	m.activeCancel = cancel
}

func (m *aiModule) clearActiveCancel(cancel context.CancelFunc) {
	m.cancelMu.Lock()
	defer m.cancelMu.Unlock()
	m.activeCancel = nil
}

// ─── JS API: ai.send / ai.stream ─────────────────────────────────────────────

func (m *aiModule) jsSend(call goja.FunctionCall) goja.Value {
	m.ensureActiveSelection()
	messages, systemPrompt, err := parseMessagesArgs(m.rt, call)
	if err != nil {
		panic(m.rt.NewGoError(err))
	}
	req := llmRequest{Messages: messages, SystemPrompt: systemPrompt, Model: m.activeModel}
	resp, err := m.provider.send(context.Background(), req)
	if err != nil {
		panic(m.rt.NewGoError(err))
	}
	return m.rt.ToValue(map[string]any{
		"content":      resp.Content,
		"inputTokens":  resp.InputTokens,
		"outputTokens": resp.OutputTokens,
		"provider":     resp.Provider,
		"model":        resp.Model,
	})
}

// jsStream streams LLM tokens synchronously.
// Accepts: stream(messages, systemPrompt, handlers)
// handlers = { data: function(token), end: function(resp), error: function(err) }
// Because goja is single-threaded, streaming runs on the calling goroutine;
// goroutines must never call back into the goja runtime.
func (m *aiModule) jsStream(call goja.FunctionCall) goja.Value {
	m.ensureActiveSelection()
	messages, systemPrompt, err := parseMessagesArgs(m.rt, call)
	if err != nil {
		panic(m.rt.NewGoError(err))
	}

	// Parse callback handlers from the third argument: { data, end, error }
	var onData, onEnd, onError goja.Callable
	if h := call.Argument(2); h != nil && !goja.IsUndefined(h) && !goja.IsNull(h) {
		if obj := h.ToObject(m.rt); obj != nil {
			onData, _ = goja.AssertFunction(obj.Get("data"))
			onEnd, _ = goja.AssertFunction(obj.Get("end"))
			onError, _ = goja.AssertFunction(obj.Get("error"))
		}
	}

	waitLabel := ""
	waitIntervalMs := int64(200)
	if opts := call.Argument(3); opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		if obj := opts.ToObject(m.rt); obj != nil {
			if label := obj.Get("waitLabel"); !goja.IsUndefined(label) && !goja.IsNull(label) {
				waitLabel = label.String()
			}
			if raw := obj.Get("waitIntervalMs"); !goja.IsUndefined(raw) && !goja.IsNull(raw) {
				if n := toInt64(raw.Export()); n > 0 {
					waitIntervalMs = n
				}
			}
		}
	}

	// Run synchronously on the current goroutine — safe for goja.
	ctx, cancel := context.WithCancel(context.Background())
	m.setActiveCancel(cancel)
	defer m.clearActiveCancel(cancel)

	stopWait := startWaitingIndicator(waitLabel, time.Duration(waitIntervalMs)*time.Millisecond)
	firstToken := true
	req := llmRequest{Messages: messages, SystemPrompt: systemPrompt, Model: m.activeModel}
	resp, err := m.provider.stream(ctx, req, func(token string) {
		if firstToken {
			firstToken = false
			stopWait()
		}
		if onData != nil {
			_, _ = onData(goja.Undefined(), m.rt.ToValue(token))
		}
	})
	stopWait()
	if err != nil {
		if onError != nil {
			msg := err.Error()
			if errors.Is(err, context.Canceled) {
				msg = "cancelled"
			}
			_, _ = onError(goja.Undefined(), m.rt.ToValue(msg))
		}
		return goja.Undefined()
	}
	if onEnd != nil {
		_, _ = onEnd(goja.Undefined(), m.rt.ToValue(map[string]any{
			"content":      resp.Content,
			"inputTokens":  resp.InputTokens,
			"outputTokens": resp.OutputTokens,
			"provider":     resp.Provider,
			"model":        resp.Model,
		}))
	}
	return goja.Undefined()
}

func startWaitingIndicator(label string, interval time.Duration) func() {
	if label == "" {
		return func() {}
	}
	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		frames := []string{"thinking", "thinking.", "thinking..", "thinking..."}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		writeWaitingFrame(label, frames[0])
		frameIdx := 0
		for {
			select {
			case <-stopCh:
				clearWaitingFrame(label)
				return
			case <-ticker.C:
				frameIdx = (frameIdx + 1) % len(frames)
				writeWaitingFrame(label, frames[frameIdx])
			}
		}
	}()
	return func() {
		select {
		case <-stopCh:
		default:
			close(stopCh)
		}
		<-doneCh
	}
}

func writeWaitingFrame(label string, frame string) {
	_, _ = fmt.Fprintf(os.Stdout, "\r%s%s", label, frame)
}

func clearWaitingFrame(label string) {
	clearWidth := len(label) + len("thinking...")
	_, _ = fmt.Fprintf(os.Stdout, "\r%s\r", strings.Repeat(" ", clearWidth))
}

func parseMessagesArgs(rt *goja.Runtime, call goja.FunctionCall) ([]llmMessage, string, error) {
	rawMessages := call.Argument(0).Export()
	systemPrompt := call.Argument(1).String()

	arr, ok := rawMessages.([]any)
	if !ok {
		return nil, "", fmt.Errorf("messages must be an array")
	}
	messages := make([]llmMessage, 0, len(arr))
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role, _ := obj["role"].(string)
		content, _ := obj["content"].(string)
		messages = append(messages, llmMessage{Role: role, Content: content})
	}
	return messages, systemPrompt, nil
}

// ─── JS API: ai.config.* ─────────────────────────────────────────────────────

func (m *aiModule) makeConfigObject() *goja.Object {
	obj := m.rt.NewObject()

	obj.Set("load", func(call goja.FunctionCall) goja.Value {
		cfg, err := loadLLMConfig()
		if err != nil {
			panic(m.rt.NewGoError(err))
		}
		m.cfg = cfg
		m.rebuildActiveProvider()
		data, _ := json.Marshal(cfg)
		var result any
		json.Unmarshal(data, &result)
		return m.rt.ToValue(result)
	})

	obj.Set("save", func(call goja.FunctionCall) goja.Value {
		exported := call.Argument(0).Export()
		data, err := json.Marshal(exported)
		if err != nil {
			panic(m.rt.NewGoError(err))
		}
		if err := json.Unmarshal(data, m.cfg); err != nil {
			panic(m.rt.NewGoError(err))
		}
		if err := saveLLMConfig(m.cfg); err != nil {
			panic(m.rt.NewGoError(err))
		}
		m.rebuildActiveProvider()
		return goja.Undefined()
	})

	obj.Set("set", func(call goja.FunctionCall) goja.Value {
		key := call.Argument(0).String()
		value := call.Argument(1).String()
		if err := setDotKey(m.cfg, key, value); err != nil {
			panic(m.rt.NewGoError(err))
		}
		if err := saveLLMConfig(m.cfg); err != nil {
			panic(m.rt.NewGoError(err))
		}
		m.rebuildActiveProvider()
		return goja.Undefined()
	})

	obj.Set("path", func(call goja.FunctionCall) goja.Value {
		path, err := llmConfigPath()
		if err != nil {
			panic(m.rt.NewGoError(err))
		}
		return m.rt.ToValue(path)
	})

	return obj
}

func (m *aiModule) makeLastConfigObject() *goja.Object {
	obj := m.rt.NewObject()

	obj.Set("load", func(call goja.FunctionCall) goja.Value {
		cfg, err := loadLastConfig()
		if err != nil {
			panic(m.rt.NewGoError(err))
		}
		m.last = cfg
		return m.rt.ToValue(map[string]any{
			"provider": cfg.Provider,
			"model":    cfg.Model,
		})
	})

	obj.Set("save", func(call goja.FunctionCall) goja.Value {
		exported, _ := call.Argument(0).Export().(map[string]any)
		next := defaultLastConfig()
		if provider, ok := exported["provider"].(string); ok {
			if provider != "" {
				name, err := normalizeProviderName(provider)
				if err != nil {
					panic(m.rt.NewGoError(err))
				}
				next.Provider = name
			}
		}
		if model, ok := exported["model"].(string); ok {
			next.Model = strings.TrimSpace(model)
		}
		if next.Provider != "" && next.Model == "" {
			next.Model = defaultModelForProvider(next.Provider)
		}
		if err := saveLastConfig(next); err != nil {
			panic(m.rt.NewGoError(err))
		}
		m.last = next
		return goja.Undefined()
	})

	obj.Set("path", func(call goja.FunctionCall) goja.Value {
		path, err := llmLastConfigPath()
		if err != nil {
			panic(m.rt.NewGoError(err))
		}
		return m.rt.ToValue(path)
	})

	return obj
}

// ─── JS API: ai.setProvider / ai.providerInfo ────────────────────────────────

func (m *aiModule) jsSetProvider(call goja.FunctionCall) goja.Value {
	name, err := normalizeProviderName(call.Argument(0).String())
	if err != nil {
		panic(m.rt.NewGoError(err))
	}
	model := defaultModelForProvider(name)
	if m.last != nil && m.last.Provider == name && m.last.Model != "" {
		model = m.last.Model
	}
	m.applySelection(name, model)
	return goja.Undefined()
}

func (m *aiModule) jsSetModel(call goja.FunctionCall) goja.Value {
	model := strings.TrimSpace(call.Argument(0).String())
	if model == "" {
		panic(m.rt.NewGoError(fmt.Errorf("model name cannot be empty")))
	}
	m.activeModel = model
	return goja.Undefined()
}

func (m *aiModule) jsCancel(call goja.FunctionCall) goja.Value {
	m.cancelMu.Lock()
	cancel := m.activeCancel
	m.cancelMu.Unlock()
	if cancel != nil {
		cancel()
	}
	return goja.Undefined()
}

func (m *aiModule) jsProviderInfo(call goja.FunctionCall) goja.Value {
	m.ensureActiveSelection()
	conf := m.cfg.Providers[m.activeName]
	maxTokens := 0
	if conf != nil {
		maxTokens = conf.MaxTokens
	}
	info := map[string]any{
		"name":      m.activeName,
		"model":     m.activeModel,
		"maxTokens": maxTokens,
	}

	switch m.activeName {
	case "claude":
		hasKey := conf != nil && conf.APIKey != ""
		if !hasKey {
			hasKey = os.Getenv("ANTHROPIC_API_KEY") != ""
		}
		info["hasApiKey"] = hasKey
	case "openai":
		hasKey := conf != nil && conf.APIKey != ""
		if !hasKey {
			hasKey = os.Getenv("OPENAI_API_KEY") != ""
		}
		info["hasApiKey"] = hasKey
		baseURL := ""
		if conf != nil {
			baseURL = conf.BaseURL
		}
		if baseURL == "" {
			baseURL = os.Getenv("OPENAI_BASE_URL")
		}
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		baseURL = strings.TrimRight(baseURL, "/")
		info["baseUrl"] = baseURL
	case "ollama":
		baseURL := ""
		if conf != nil {
			baseURL = conf.BaseURL
		}
		if baseURL == "" {
			baseURL = os.Getenv("OLLAMA_HOST")
		}
		if baseURL == "" {
			baseURL = "http://127.0.0.1:11434"
		}
		if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
			baseURL = "http://" + baseURL
		}
		baseURL = strings.TrimRight(baseURL, "/")
		info["baseUrl"] = baseURL
		info["hasBaseUrl"] = baseURL != ""
	}

	return m.rt.ToValue(info)
}

// ─── JS API: ai.listSegments / ai.loadSegment ────────────────────────────────

func (m *aiModule) jsListSegments(call goja.FunctionCall) goja.Value {
	segments, err := m.listSegments()
	if err != nil {
		panic(m.rt.NewGoError(err))
	}
	vals := make([]any, len(segments))
	for i, s := range segments {
		vals[i] = s
	}
	return m.rt.ToValue(vals)
}

func (m *aiModule) jsLoadSegment(call goja.FunctionCall) goja.Value {
	name := call.Argument(0).String()
	content, err := m.loadSegment(name)
	if err != nil {
		panic(m.rt.NewGoError(err))
	}
	return m.rt.ToValue(content)
}

func (m *aiModule) listSegments() ([]string, error) {
	seen := map[string]bool{}
	var result []string

	// 1. builtin from embedded FS
	if m.promptFS != nil {
		entries, err := fs.ReadDir(m.promptFS, ".")
		if err == nil {
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
					continue
				}
				base := strings.TrimSuffix(e.Name(), ".md")
				if !seen[base] {
					seen[base] = true
					result = append(result, base)
				}
			}
		}
	}

	// 2. custom from host OS
	customDir, err := llmCustomPromptDir()
	if err == nil {
		entries, err := os.ReadDir(customDir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
					continue
				}
				base := strings.TrimSuffix(e.Name(), ".md")
				if !seen[base] {
					seen[base] = true
					result = append(result, base)
				}
			}
		}
	}
	return result, nil
}

func (m *aiModule) loadSegment(name string) (string, error) {
	// 1. custom override — host OS
	customDir, err := llmCustomPromptDir()
	if err == nil {
		path := filepath.Join(customDir, name+".md")
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data), nil
		}
	}

	// 2. builtin — embedded FS
	if m.promptFS != nil {
		data, err := m.promptFS.ReadFile(name + ".md")
		if err == nil {
			return string(data), nil
		}
	}

	return "", fmt.Errorf("segment %q not found", name)
}

// ─── JS API: ai.editConfig ───────────────────────────────────────────────────

func (m *aiModule) jsEditConfig(call goja.FunctionCall) goja.Value {
	configPath, err := llmConfigPath()
	if err != nil {
		panic(m.rt.NewGoError(err))
	}

	editor := findHostEditor()
	if editor == "" {
		return m.rt.ToValue("no-editor")
	}

	// Ensure config file exists before opening
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := saveLLMConfig(m.cfg); err != nil {
			panic(m.rt.NewGoError(err))
		}
	}

	cmd := exec.Command(editor, configPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return m.rt.ToValue("cancelled")
	}

	// Validate JSON and reload
	data, err := os.ReadFile(configPath)
	if err != nil {
		panic(m.rt.NewGoError(err))
	}
	if !json.Valid(data) {
		return m.rt.ToValue("invalid-json")
	}
	newCfg := defaultLLMConfig()
	if err := json.Unmarshal(data, newCfg); err != nil {
		return m.rt.ToValue("invalid-json")
	}
	sanitizeLLMConfig(newCfg)
	m.cfg = newCfg
	m.rebuildActiveProvider()
	return m.rt.ToValue("saved")
}

func findHostEditor() string {
	// $EDITOR env variable first
	if ed := os.Getenv("EDITOR"); ed != "" {
		if path, err := exec.LookPath(ed); err == nil {
			return path
		}
	}
	// Platform preference
	candidates := []string{"vi", "nano"}
	if runtime.GOOS == "windows" {
		candidates = []string{"notepad"}
	}
	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

// ─── Module registration ─────────────────────────────────────────────────────

//go:embed ai_prompts
var aiPromptsEmbed embed.FS

// aiPromptSubFS is the sub-FS rooted at ai_prompts/ for segment lookup.
var aiPromptSubFS, _ = fs.Sub(aiPromptsEmbed, "ai_prompts")

// aiModule registration — called from Module() in shell.go
// jsExecJsh runs jsh code via the agent REPL profile and returns the
// structured JSON result objects emitted by AgentRenderer.
//
// Signature: ai.exec(code [, options]) → [{ok, type, value, elapsedMs, ...}]
// options: { readOnly, maxRows, timeoutMs, maxOutputBytes }
//
// Profile.Startup runs before eval so globalThis.agent (db, schema, runtime)
// is available. Because this uses the same goja runtime as the caller, the
// agent module is loaded via require() which is cached after the first call.
func (m *aiModule) jsExecJsh(call goja.FunctionCall) goja.Value {
	code := call.Argument(0).String()

	readOnly := true
	maxRows := 1000
	var timeoutMs int64 = 30000
	maxOutputBytes := 65536

	if opts, ok := call.Argument(1).Export().(map[string]any); ok {
		if v, ok := opts["readOnly"].(bool); ok {
			readOnly = v
		}
		if v, ok := opts["maxRows"]; ok {
			if n := toInt64(v); n > 0 {
				maxRows = int(n)
			}
		}
		if v, ok := opts["timeoutMs"]; ok {
			if n := toInt64(v); n > 0 {
				timeoutMs = n
			}
		}
		if v, ok := opts["maxOutputBytes"]; ok {
			if n := toInt64(v); n > 0 {
				maxOutputBytes = int(n)
			}
		}
	}

	// Build an agent-profile Repl config. Profile.Startup will inject
	// __agentConfig and load globalThis.agent before running eval.
	agentCfg := agentProfileConfig{
		ReadOnly:       readOnly,
		MaxRows:        maxRows,
		MaxOutputBytes: maxOutputBytes,
	}
	cfg := defaultReplConfig()
	cfg.Profile = agentReplProfileWith(agentCfg)
	cfg.Renderer = &AgentRenderer{MaxOutputBytes: maxOutputBytes}
	cfg.Eval = code
	cfg.PrintEval = true
	cfg.ReadOnly = readOnly
	cfg.MaxRows = maxRows
	cfg.MaxOutputBytes = maxOutputBytes
	cfg.TimeoutMs = timeoutMs
	cfg.History.Enabled = false

	// Capture AgentRenderer NDJSON output into a buffer.
	var buf bytes.Buffer

	// Redirect console.log/println to a separate buffer so the output is captured
	// for the LLM context. Without this redirect, console writes go to the engine's
	// default writer (stdout) and are visible to the user but invisible to the LLM.
	var consoleBuf bytes.Buffer
	oldWriter := jshlog.SetDefaultWriter(&consoleBuf)

	r := &Repl{rt: m.rt, cfg: cfg}
	r.registerBuiltinCommands()
	// Profile.Startup + runEval are invoked inside loopWithConfig.
	// We call the internal path directly to inject our writer.
	if err := cfg.Profile.RunStartup(r.rt); err != nil {
		jshlog.SetDefaultWriter(oldWriter)
		panic(m.rt.NewGoError(err))
	}
	r.runEval(code, true, &buf, cfg.Renderer, timeoutMs)

	// Restore console output writer.
	jshlog.SetDefaultWriter(oldWriter)

	// If console output was produced, prepend it as a print-type NDJSON entry
	// so it appears in the structured result before the expression value.
	var combined bytes.Buffer
	if consoleBuf.Len() > 0 {
		text := strings.TrimRight(consoleBuf.String(), "\n")
		// Strip log-level prefixes (e.g. "INFO  ", "WARN  ") added by log.makeConsoleLog
		// when the agent code calls console.log() instead of console.println().
		text = stripLogLevelPrefixes(text)
		printLine, _ := json.Marshal(map[string]any{
			"ok":        true,
			"type":      "print",
			"value":     text,
			"elapsedMs": 0,
		})
		combined.Write(printLine)
		combined.WriteByte('\n')
	}
	combined.Write(buf.Bytes())

	// Parse the NDJSON lines emitted by AgentRenderer into a JS array.
	var results []any
	scanner := bufio.NewScanner(&combined)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var obj any
		if err := json.Unmarshal([]byte(line), &obj); err == nil {
			results = append(results, obj)
		}
	}
	if results == nil {
		results = []any{}
	}
	return m.rt.ToValue(results)
}

// stripLogLevelPrefixes removes slog level prefixes ("INFO  ", "WARN  ", "DEBUG ",
// "ERROR ") that log.makeConsoleLog prepends to each line. Agent code often calls
// console.log() which routes through makeConsoleLog; when the output is captured
// for LLM context those prefixes are noise.
func stripLogLevelPrefixes(text string) string {
	prefixes := []string{"INFO  ", "WARN  ", "WARN ", "DEBUG ", "ERROR "}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		for _, pfx := range prefixes {
			if strings.HasPrefix(line, pfx) {
				lines[i] = line[len(pfx):]
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}

func registerAIModule(rt *goja.Runtime, o *goja.Object) {
	m := newAIModule(rt, aiPromptSubFS.(fs.ReadFileFS))

	obj := rt.NewObject()
	obj.Set("send", m.jsSend)
	obj.Set("stream", m.jsStream)
	obj.Set("exec", m.jsExecJsh)
	obj.Set("setProvider", m.jsSetProvider)
	obj.Set("setModel", m.jsSetModel)
	obj.Set("cancel", m.jsCancel)
	obj.Set("providerInfo", m.jsProviderInfo)
	obj.Set("listSegments", m.jsListSegments)
	obj.Set("loadSegment", m.jsLoadSegment)
	obj.Set("editConfig", m.jsEditConfig)
	obj.Set("config", m.makeConfigObject())
	obj.Set("lastConfig", m.makeLastConfigObject())
	o.Set("ai", obj)
}
