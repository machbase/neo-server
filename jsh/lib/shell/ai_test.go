package shell

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dop251/goja"
)

// ─── Config Tests ─────────────────────────────────────────────────────────────

func TestDefaultLLMConfig(t *testing.T) {
	cfg := defaultLLMConfig()

	if cfg.DefaultProvider != "claude" {
		t.Errorf("DefaultProvider = %q, want %q", cfg.DefaultProvider, "claude")
	}
	claudeCfg, ok := cfg.Providers["claude"]
	if !ok {
		t.Fatal("providers map missing 'claude' entry")
	}
	if claudeCfg.Model != "claude-opus-4-5" {
		t.Errorf("claude model = %q, want %q", claudeCfg.Model, "claude-opus-4-5")
	}
	if claudeCfg.MaxTokens != 8192 {
		t.Errorf("claude maxTokens = %d, want 8192", claudeCfg.MaxTokens)
	}
	ollamaCfg, ok := cfg.Providers["ollama"]
	if !ok {
		t.Fatal("providers map missing 'ollama' entry")
	}
	if ollamaCfg.BaseURL != "http://127.0.0.1:11434" {
		t.Errorf("ollama baseUrl = %q, want %q", ollamaCfg.BaseURL, "http://127.0.0.1:11434")
	}
	if ollamaCfg.Model != "llama3.1" {
		t.Errorf("ollama model = %q, want %q", ollamaCfg.Model, "llama3.1")
	}
	if ollamaCfg.MaxTokens != 8192 {
		t.Errorf("ollama maxTokens = %d, want 8192", ollamaCfg.MaxTokens)
	}
	if cfg.Exec.MaxRows != 1000 {
		t.Errorf("exec.maxRows = %d, want 1000", cfg.Exec.MaxRows)
	}
	if cfg.Exec.TimeoutMs != 30000 {
		t.Errorf("exec.timeoutMs = %d, want 30000", cfg.Exec.TimeoutMs)
	}
	if !cfg.Exec.ReadOnly {
		t.Error("exec.readOnly should be true by default")
	}
	if len(cfg.Prompt.Segments) == 0 {
		t.Error("prompt.segments should not be empty")
	}
}

func TestLLMConfigPath(t *testing.T) {
	path, err := llmConfigPath()
	if err != nil {
		t.Fatalf("llmConfigPath() error: %v", err)
	}
	if !strings.Contains(path, ".config") {
		t.Errorf("config path %q does not contain .config", path)
	}
	if !strings.HasSuffix(path, "config.json") {
		t.Errorf("config path %q does not end with config.json", path)
	}
}

func TestLLMCustomPromptDir(t *testing.T) {
	dir, err := llmCustomPromptDir()
	if err != nil {
		t.Fatalf("llmCustomPromptDir() error: %v", err)
	}
	if !strings.Contains(dir, ".config") {
		t.Errorf("custom prompt dir %q does not contain .config", dir)
	}
	if !strings.HasSuffix(dir, "prompts") {
		t.Errorf("custom prompt dir %q does not end with prompts", dir)
	}
}

func TestLoadLLMConfig_NonExistent(t *testing.T) {
	// Redirect home to a temp dir with no config file
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp) // Windows uses USERPROFILE, not HOME

	cfg, err := loadLLMConfig()
	if err != nil {
		t.Fatalf("loadLLMConfig() error for non-existent config: %v", err)
	}
	// Should return defaults
	if cfg.DefaultProvider != "claude" {
		t.Errorf("DefaultProvider = %q, want default %q", cfg.DefaultProvider, "claude")
	}
}

func TestLoadAndSaveLLMConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp) // Windows uses USERPROFILE, not HOME

	original := defaultLLMConfig()
	original.DefaultProvider = "openai"
	original.Providers["openai"] = &llmProviderConf{
		APIKey:    "test-key",
		Model:     "gpt-4o",
		MaxTokens: 4096,
	}

	if err := saveLLMConfig(original); err != nil {
		t.Fatalf("saveLLMConfig() error: %v", err)
	}

	loaded, err := loadLLMConfig()
	if err != nil {
		t.Fatalf("loadLLMConfig() error after save: %v", err)
	}
	if loaded.DefaultProvider != "openai" {
		t.Errorf("loaded DefaultProvider = %q, want %q", loaded.DefaultProvider, "openai")
	}
	openaiCfg, ok := loaded.Providers["openai"]
	if !ok {
		t.Fatal("loaded config missing 'openai' provider")
	}
	if openaiCfg.Model != "gpt-4o" {
		t.Errorf("openai model = %q, want %q", openaiCfg.Model, "gpt-4o")
	}
}

func TestLoadLLMConfig_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp) // Windows uses USERPROFILE, not HOME

	path, _ := llmConfigPath()
	if err := os.MkdirAll(strings.TrimSuffix(path, "config.json"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not-valid-json{{{"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := loadLLMConfig()
	if err == nil {
		t.Error("loadLLMConfig() should return error for invalid JSON")
	}
}

// ─── setDotKey Tests ─────────────────────────────────────────────────────────

func TestSetDotKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		verify  func(t *testing.T, cfg *llmConfig)
		wantErr bool
	}{
		{
			name:  "set defaultProvider",
			key:   "defaultProvider",
			value: "openai",
			verify: func(t *testing.T, cfg *llmConfig) {
				if cfg.DefaultProvider != "openai" {
					t.Errorf("DefaultProvider = %q, want %q", cfg.DefaultProvider, "openai")
				}
			},
		},
		{
			name:  "set nested provider model",
			key:   "providers.claude.model",
			value: "claude-haiku-3-5",
			verify: func(t *testing.T, cfg *llmConfig) {
				if cfg.Providers["claude"].Model != "claude-haiku-3-5" {
					t.Errorf("claude model = %q, want %q", cfg.Providers["claude"].Model, "claude-haiku-3-5")
				}
			},
		},
		{
			name:  "set numeric value",
			key:   "providers.claude.maxTokens",
			value: "4096",
			verify: func(t *testing.T, cfg *llmConfig) {
				if cfg.Providers["claude"].MaxTokens != 4096 {
					t.Errorf("claude maxTokens = %d, want 4096", cfg.Providers["claude"].MaxTokens)
				}
			},
		},
		{
			name:  "set bool value",
			key:   "exec.readOnly",
			value: "false",
			verify: func(t *testing.T, cfg *llmConfig) {
				if cfg.Exec.ReadOnly {
					t.Error("exec.readOnly should be false after set")
				}
			},
		},
		{
			name:    "non-object intermediate key",
			key:     "defaultProvider.foo.bar",
			value:   "baz",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := defaultLLMConfig()
			err := setDotKey(cfg, tc.key, tc.value)
			if tc.wantErr {
				if err == nil {
					t.Error("setDotKey() should return error")
				}
				return
			}
			if err != nil {
				t.Fatalf("setDotKey() error: %v", err)
			}
			if tc.verify != nil {
				tc.verify(t, cfg)
			}
		})
	}
}

// ─── Provider Tests ──────────────────────────────────────────────────────────

func TestNewClaudeProvider_Defaults(t *testing.T) {
	p := newClaudeProvider(&llmProviderConf{})
	if p.name() != "claude" {
		t.Errorf("name() = %q, want %q", p.name(), "claude")
	}
	if p.model() != "claude-opus-4-5" {
		t.Errorf("model() = %q, want %q", p.model(), "claude-opus-4-5")
	}
	if p.maxTokens != 8192 {
		t.Errorf("maxTokens = %d, want 8192", p.maxTokens)
	}
}

func TestNewClaudeProvider_FromConf(t *testing.T) {
	p := newClaudeProvider(&llmProviderConf{
		APIKey:    "my-key",
		Model:     "claude-haiku-3-5",
		MaxTokens: 1024,
	})
	if p.apiKey != "my-key" {
		t.Errorf("apiKey = %q, want %q", p.apiKey, "my-key")
	}
	if p.model() != "claude-haiku-3-5" {
		t.Errorf("model() = %q, want %q", p.model(), "claude-haiku-3-5")
	}
	if p.maxTokens != 1024 {
		t.Errorf("maxTokens = %d, want 1024", p.maxTokens)
	}
}

func TestNewClaudeProvider_EnvAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "env-api-key")
	p := newClaudeProvider(&llmProviderConf{})
	if p.apiKey != "env-api-key" {
		t.Errorf("apiKey from env = %q, want %q", p.apiKey, "env-api-key")
	}
}

func TestNewOpenAIProvider_Defaults(t *testing.T) {
	p := newOpenAIProvider(&llmProviderConf{})
	if p.name() != "openai" {
		t.Errorf("name() = %q, want %q", p.name(), "openai")
	}
	if p.model() != "gpt-4o" {
		t.Errorf("model() = %q, want %q", p.model(), "gpt-4o")
	}
	if p.baseURL != "https://api.openai.com/v1" {
		t.Errorf("baseURL = %q, want %q", p.baseURL, "https://api.openai.com/v1")
	}
	if p.maxTokens != 8192 {
		t.Errorf("maxTokens = %d, want 8192", p.maxTokens)
	}
}

func TestNewOpenAIProvider_EnvAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env-openai-key")
	p := newOpenAIProvider(&llmProviderConf{})
	if p.apiKey != "env-openai-key" {
		t.Errorf("apiKey from env = %q, want %q", p.apiKey, "env-openai-key")
	}
}

func TestNewOllamaProvider_Defaults(t *testing.T) {
	p := newOllamaProvider(&llmProviderConf{})
	if p.name() != "ollama" {
		t.Errorf("name() = %q, want %q", p.name(), "ollama")
	}
	if p.model() != "llama3.1" {
		t.Errorf("model() = %q, want %q", p.model(), "llama3.1")
	}
	if p.baseURL != "http://127.0.0.1:11434" {
		t.Errorf("baseURL = %q, want %q", p.baseURL, "http://127.0.0.1:11434")
	}
	if p.maxTokens != 8192 {
		t.Errorf("maxTokens = %d, want 8192", p.maxTokens)
	}
}

func TestNewOllamaProvider_FromConf(t *testing.T) {
	p := newOllamaProvider(&llmProviderConf{
		BaseURL:   "ollama.internal:11434/",
		Model:     "qwen3:8b",
		MaxTokens: 2048,
	})
	if p.baseURL != "http://ollama.internal:11434" {
		t.Errorf("baseURL = %q, want %q", p.baseURL, "http://ollama.internal:11434")
	}
	if p.model() != "qwen3:8b" {
		t.Errorf("model() = %q, want %q", p.model(), "qwen3:8b")
	}
	if p.maxTokens != 2048 {
		t.Errorf("maxTokens = %d, want 2048", p.maxTokens)
	}
}

func TestNewOllamaProvider_EnvBaseURL(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "ollama.env:11434")
	p := newOllamaProvider(&llmProviderConf{})
	if p.baseURL != "http://ollama.env:11434" {
		t.Errorf("baseURL from env = %q, want %q", p.baseURL, "http://ollama.env:11434")
	}
}

func TestNewOpenAIProvider_EnvBaseURL(t *testing.T) {
	t.Setenv("OPENAI_BASE_URL", "https://openai.internal/v1/")
	p := newOpenAIProvider(&llmProviderConf{})
	if p.baseURL != "https://openai.internal/v1" {
		t.Errorf("baseURL from env = %q, want %q", p.baseURL, "https://openai.internal/v1")
	}
}

// ─── claudeProvider.buildBody Tests ─────────────────────────────────────────

func TestClaudeProvider_BuildBody(t *testing.T) {
	p := newClaudeProvider(&llmProviderConf{Model: "claude-opus-4-5", MaxTokens: 8192})
	req := llmRequest{
		Messages: []llmMessage{
			{Role: "user", Content: "hello"},
		},
		SystemPrompt: "You are helpful.",
	}

	body, err := p.buildBody(req, false)
	if err != nil {
		t.Fatalf("buildBody() error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if m["model"] != "claude-opus-4-5" {
		t.Errorf("model = %v, want %q", m["model"], "claude-opus-4-5")
	}
	if m["system"] != "You are helpful." {
		t.Errorf("system = %v, want %q", m["system"], "You are helpful.")
	}
	if m["stream"] != false {
		t.Errorf("stream = %v, want false", m["stream"])
	}

	// Test streaming flag
	bodyStream, err := p.buildBody(req, true)
	if err != nil {
		t.Fatalf("buildBody(stream=true) error: %v", err)
	}
	var ms map[string]any
	json.Unmarshal(bodyStream, &ms)
	if ms["stream"] != true {
		t.Errorf("stream = %v, want true", ms["stream"])
	}
}

func TestClaudeProvider_BuildBody_ModelOverride(t *testing.T) {
	p := newClaudeProvider(&llmProviderConf{Model: "claude-opus-4-5"})
	req := llmRequest{
		Model:     "claude-haiku-3-5",
		MaxTokens: 512,
		Messages:  []llmMessage{{Role: "user", Content: "hi"}},
	}
	body, err := p.buildBody(req, false)
	if err != nil {
		t.Fatalf("buildBody() error: %v", err)
	}
	var m map[string]any
	json.Unmarshal(body, &m)
	if m["model"] != "claude-haiku-3-5" {
		t.Errorf("model = %v, want %q (request override)", m["model"], "claude-haiku-3-5")
	}
	if m["max_tokens"] != float64(512) {
		t.Errorf("max_tokens = %v, want 512", m["max_tokens"])
	}
}

func TestOllamaProvider_BuildBody(t *testing.T) {
	p := newOllamaProvider(&llmProviderConf{
		BaseURL:   "http://127.0.0.1:11434",
		Model:     "llama3.1",
		MaxTokens: 4096,
	})
	req := llmRequest{
		Messages: []llmMessage{
			{Role: "user", Content: "hello"},
		},
		SystemPrompt: "You are helpful.",
	}

	body, err := p.buildBody(req, true)
	if err != nil {
		t.Fatalf("buildBody() error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if m["model"] != "llama3.1" {
		t.Errorf("model = %v, want %q", m["model"], "llama3.1")
	}
	if m["stream"] != true {
		t.Errorf("stream = %v, want true", m["stream"])
	}
	options, ok := m["options"].(map[string]any)
	if !ok {
		t.Fatalf("options missing or wrong type: %T", m["options"])
	}
	if options["num_predict"] != float64(4096) {
		t.Errorf("options.num_predict = %v, want 4096", options["num_predict"])
	}
	messages, ok := m["messages"].([]any)
	if !ok {
		t.Fatalf("messages missing or wrong type: %T", m["messages"])
	}
	if len(messages) != 2 {
		t.Fatalf("messages length = %d, want 2", len(messages))
	}
	first, _ := messages[0].(map[string]any)
	second, _ := messages[1].(map[string]any)
	if first["role"] != "system" || first["content"] != "You are helpful." {
		t.Errorf("first message = %#v, want system prompt", first)
	}
	if second["role"] != "user" || second["content"] != "hello" {
		t.Errorf("second message = %#v, want user content", second)
	}
}

func TestOpenAIProvider_BuildBody(t *testing.T) {
	p := newOpenAIProvider(&llmProviderConf{
		BaseURL:   "https://api.openai.com/v1",
		Model:     "gpt-4o",
		MaxTokens: 4096,
	})
	req := llmRequest{
		Messages: []llmMessage{
			{Role: "user", Content: "hello"},
		},
		SystemPrompt: "You are helpful.",
	}

	body, err := p.buildBody(req, true)
	if err != nil {
		t.Fatalf("buildBody() error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if m["model"] != "gpt-4o" {
		t.Errorf("model = %v, want %q", m["model"], "gpt-4o")
	}
	if m["stream"] != true {
		t.Errorf("stream = %v, want true", m["stream"])
	}
	if m["max_tokens"] != float64(4096) {
		t.Errorf("max_tokens = %v, want 4096", m["max_tokens"])
	}
	streamOptions, ok := m["stream_options"].(map[string]any)
	if !ok {
		t.Fatalf("stream_options missing or wrong type: %T", m["stream_options"])
	}
	if streamOptions["include_usage"] != true {
		t.Errorf("stream_options.include_usage = %v, want true", streamOptions["include_usage"])
	}
	messages, ok := m["messages"].([]any)
	if !ok {
		t.Fatalf("messages missing or wrong type: %T", m["messages"])
	}
	if len(messages) != 2 {
		t.Fatalf("messages length = %d, want 2", len(messages))
	}
	first, _ := messages[0].(map[string]any)
	second, _ := messages[1].(map[string]any)
	if first["role"] != "system" || first["content"] != "You are helpful." {
		t.Errorf("first message = %#v, want system prompt", first)
	}
	if second["role"] != "user" || second["content"] != "hello" {
		t.Errorf("second message = %#v, want user content", second)
	}
}

// ─── claudeProvider HTTP Tests (mock server) ─────────────────────────────────

func newMockClaudeServer(handler http.HandlerFunc) (*httptest.Server, *claudeProvider) {
	srv := httptest.NewServer(handler)
	p := newClaudeProvider(&llmProviderConf{APIKey: "test-key", Model: "claude-test", MaxTokens: 100})
	// Override the HTTP client to point at the mock server by temporarily
	// replacing the URL inside doRequest via a thin wrapper test provider.
	return srv, p
}

func newMockOllamaServer(handler http.HandlerFunc) (*httptest.Server, *ollamaProvider) {
	srv := httptest.NewServer(handler)
	p := newOllamaProvider(&llmProviderConf{
		BaseURL:   srv.URL,
		Model:     "llama3.1",
		MaxTokens: 100,
	})
	return srv, p
}

func newMockOpenAIServer(handler http.HandlerFunc) (*httptest.Server, *openaiProvider) {
	srv := httptest.NewServer(handler)
	p := newOpenAIProvider(&llmProviderConf{
		BaseURL:   srv.URL,
		APIKey:    "test-key",
		Model:     "gpt-4o",
		MaxTokens: 100,
	})
	return srv, p
}

// stubClaudeProvider patches the endpoint so tests don't call real APIs.
type stubClaudeProvider struct {
	*claudeProvider
	url string
}

func (p *stubClaudeProvider) doRequest(ctx context.Context, bodyBytes []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(req)
}

func (p *stubClaudeProvider) send(ctx context.Context, req llmRequest) (*llmResponse, error) {
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

func TestClaudeProvider_Send_Success(t *testing.T) {
	responseBody := `{
		"content": [{"type": "text", "text": "Hello, world!"}],
		"usage": {"input_tokens": 10, "output_tokens": 5},
		"model": "claude-test"
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, responseBody)
	}))
	defer srv.Close()

	stub := &stubClaudeProvider{
		claudeProvider: newClaudeProvider(&llmProviderConf{APIKey: "test", Model: "claude-test", MaxTokens: 100}),
		url:            srv.URL,
	}

	resp, err := stub.send(context.Background(), llmRequest{
		Messages: []llmMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("send() error: %v", err)
	}
	if resp.Content != "Hello, world!" {
		t.Errorf("Content = %q, want %q", resp.Content, "Hello, world!")
	}
	if resp.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10", resp.InputTokens)
	}
	if resp.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d, want 5", resp.OutputTokens)
	}
	if resp.Provider != "claude" {
		t.Errorf("Provider = %q, want %q", resp.Provider, "claude")
	}
}

func TestClaudeProvider_Send_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error": "unauthorized"}`)
	}))
	defer srv.Close()

	stub := &stubClaudeProvider{
		claudeProvider: newClaudeProvider(&llmProviderConf{APIKey: "bad-key", Model: "claude-test"}),
		url:            srv.URL,
	}

	_, err := stub.send(context.Background(), llmRequest{
		Messages: []llmMessage{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Error("send() should return error for non-200 status")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention status 401, got: %v", err)
	}
}

func TestClaudeProvider_Stream_Success(t *testing.T) {
	sseBody := strings.Join([]string{
		`data: {"type":"message_start","message":{"model":"claude-test","usage":{"input_tokens":10,"output_tokens":0}}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":", world!"}}`,
		`data: {"type":"message_delta","usage":{"output_tokens":5}}`,
	}, "\n") + "\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseBody)
	}))
	defer srv.Close()

	p := newClaudeProvider(&llmProviderConf{APIKey: "test", Model: "claude-test", MaxTokens: 100})

	var tokens []string
	resp, err := p.stream(context.Background(), llmRequest{
		Messages: []llmMessage{{Role: "user", Content: "hi"}},
	}, func(token string) {
		tokens = append(tokens, token)
	})

	// Note: the real provider hits api.anthropic.com which doesn't exist in tests.
	// If the server is reachable (our mock), we verify behavior.
	// Since doRequest is not overridable without embedding, we test via
	// stubClaudeProvider for streaming behavior.
	_ = resp
	_ = err
	_ = tokens
	// The streaming test needs stubClaudeProvider — see TestClaudeProvider_Stream_WithStub
}

// stubStreamProvider replicates the stream() logic but with a custom URL.
type stubStreamProvider struct {
	*claudeProvider
	url string
}

func (p *stubStreamProvider) doRequest(ctx context.Context, bodyBytes []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(req)
}

func (p *stubStreamProvider) stream(ctx context.Context, req llmRequest, onToken func(string)) (*llmResponse, error) {
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
		return nil, fmt.Errorf("claude API error %d", resp.StatusCode)
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
	return &llmResponse{
		Content:      sb.String(),
		InputTokens:  totalIn,
		OutputTokens: totalOut,
		Provider:     "claude",
		Model:        finalModel,
	}, nil
}

func TestClaudeProvider_Stream_WithStub(t *testing.T) {
	sseBody := strings.Join([]string{
		`data: {"type":"message_start","message":{"model":"claude-test","usage":{"input_tokens":10,"output_tokens":0}}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":", world!"}}`,
		`data: {"type":"message_delta","usage":{"output_tokens":5}}`,
	}, "\n") + "\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseBody)
	}))
	defer srv.Close()

	stub := &stubStreamProvider{
		claudeProvider: newClaudeProvider(&llmProviderConf{APIKey: "test", Model: "claude-test", MaxTokens: 100}),
		url:            srv.URL,
	}

	var tokens []string
	resp, err := stub.stream(context.Background(), llmRequest{
		Messages: []llmMessage{{Role: "user", Content: "hi"}},
	}, func(token string) {
		tokens = append(tokens, token)
	})
	if err != nil {
		t.Fatalf("stream() error: %v", err)
	}
	if resp.Content != "Hello, world!" {
		t.Errorf("Content = %q, want %q", resp.Content, "Hello, world!")
	}
	if resp.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10", resp.InputTokens)
	}
	if resp.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d, want 5", resp.OutputTokens)
	}
	if resp.Model != "claude-test" {
		t.Errorf("Model = %q, want %q", resp.Model, "claude-test")
	}
	if len(tokens) != 2 {
		t.Errorf("token count = %d, want 2", len(tokens))
	}
	if strings.Join(tokens, "") != "Hello, world!" {
		t.Errorf("joined tokens = %q, want %q", strings.Join(tokens, ""), "Hello, world!")
	}
}

func TestClaudeProvider_Stream_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error":"forbidden"}`)
	}))
	defer srv.Close()

	stub := &stubStreamProvider{
		claudeProvider: newClaudeProvider(&llmProviderConf{APIKey: "bad", Model: "claude-test"}),
		url:            srv.URL,
	}

	_, err := stub.stream(context.Background(), llmRequest{
		Messages: []llmMessage{{Role: "user", Content: "hi"}},
	}, func(string) {})
	if err == nil {
		t.Error("stream() should return error for non-200 status")
	}
}

func TestOpenAIProvider_Send_Success(t *testing.T) {
	srv, p := newMockOpenAIServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("URL path = %q, want %q", r.URL.Path, "/chat/completions")
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("Authorization = %q, want %q", r.Header.Get("Authorization"), "Bearer test-key")
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll(body) error: %v", err)
		}
		defer r.Body.Close()

		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("json.Unmarshal(request) error: %v", err)
		}
		if req["model"] != "gpt-4o" {
			t.Errorf("request model = %v, want %q", req["model"], "gpt-4o")
		}
		if req["stream"] != false {
			t.Errorf("request stream = %v, want false", req["stream"])
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"gpt-4o","choices":[{"message":{"role":"assistant","content":"hello from openai"}}],"usage":{"prompt_tokens":11,"completion_tokens":6}}`)
	})
	defer srv.Close()

	resp, err := p.send(context.Background(), llmRequest{
		SystemPrompt: "You are helpful.",
		Messages:     []llmMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("send() error: %v", err)
	}
	if resp.Content != "hello from openai" {
		t.Errorf("Content = %q, want %q", resp.Content, "hello from openai")
	}
	if resp.InputTokens != 11 {
		t.Errorf("InputTokens = %d, want 11", resp.InputTokens)
	}
	if resp.OutputTokens != 6 {
		t.Errorf("OutputTokens = %d, want 6", resp.OutputTokens)
	}
	if resp.Provider != "openai" {
		t.Errorf("Provider = %q, want %q", resp.Provider, "openai")
	}
	if resp.Model != "gpt-4o" {
		t.Errorf("Model = %q, want %q", resp.Model, "gpt-4o")
	}
}

func TestOpenAIProvider_Send_APIError(t *testing.T) {
	srv, p := newMockOpenAIServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"invalid api key"}`)
	})
	defer srv.Close()

	_, err := p.send(context.Background(), llmRequest{
		Messages: []llmMessage{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Error("send() should return error for non-200 status")
	}
	if !strings.Contains(err.Error(), "openai API error 401") {
		t.Errorf("error should mention status 401, got: %v", err)
	}
}

func TestOpenAIProvider_Stream_Success(t *testing.T) {
	streamBody := strings.Join([]string{
		`data: {"model":"gpt-4o","choices":[{"delta":{"content":"hello "}}]}`,
		`data: {"model":"gpt-4o","choices":[{"delta":{"content":"world"}}]}`,
		`data: {"model":"gpt-4o","choices":[],"usage":{"prompt_tokens":13,"completion_tokens":8}}`,
		`data: [DONE]`,
	}, "\n") + "\n"

	srv, p := newMockOpenAIServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, streamBody)
	})
	defer srv.Close()

	var tokens []string
	resp, err := p.stream(context.Background(), llmRequest{
		Messages: []llmMessage{{Role: "user", Content: "hi"}},
	}, func(token string) {
		tokens = append(tokens, token)
	})
	if err != nil {
		t.Fatalf("stream() error: %v", err)
	}
	if strings.Join(tokens, "") != "hello world" {
		t.Errorf("joined tokens = %q, want %q", strings.Join(tokens, ""), "hello world")
	}
	if resp.Content != "hello world" {
		t.Errorf("Content = %q, want %q", resp.Content, "hello world")
	}
	if resp.InputTokens != 13 {
		t.Errorf("InputTokens = %d, want 13", resp.InputTokens)
	}
	if resp.OutputTokens != 8 {
		t.Errorf("OutputTokens = %d, want 8", resp.OutputTokens)
	}
	if resp.Provider != "openai" {
		t.Errorf("Provider = %q, want %q", resp.Provider, "openai")
	}
	if resp.Model != "gpt-4o" {
		t.Errorf("Model = %q, want %q", resp.Model, "gpt-4o")
	}
}

func TestOpenAIProvider_Stream_APIError(t *testing.T) {
	srv, p := newMockOpenAIServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"error":"temporarily unavailable"}`)
	})
	defer srv.Close()

	_, err := p.stream(context.Background(), llmRequest{
		Messages: []llmMessage{{Role: "user", Content: "hi"}},
	}, func(string) {})
	if err == nil {
		t.Error("stream() should return error for non-200 status")
	}
	if !strings.Contains(err.Error(), "openai API error 503") {
		t.Errorf("error should mention status 503, got: %v", err)
	}
}

func TestOllamaProvider_Send_Success(t *testing.T) {
	srv, p := newMockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("URL path = %q, want %q", r.URL.Path, "/api/chat")
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll(body) error: %v", err)
		}
		defer r.Body.Close()

		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("json.Unmarshal(request) error: %v", err)
		}
		if req["model"] != "llama3.1" {
			t.Errorf("request model = %v, want %q", req["model"], "llama3.1")
		}
		if req["stream"] != false {
			t.Errorf("request stream = %v, want false", req["stream"])
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"llama3.1","message":{"role":"assistant","content":"hello from ollama"},"prompt_eval_count":12,"eval_count":7,"done":true}`)
	})
	defer srv.Close()

	resp, err := p.send(context.Background(), llmRequest{
		SystemPrompt: "You are helpful.",
		Messages:     []llmMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("send() error: %v", err)
	}
	if resp.Content != "hello from ollama" {
		t.Errorf("Content = %q, want %q", resp.Content, "hello from ollama")
	}
	if resp.InputTokens != 12 {
		t.Errorf("InputTokens = %d, want 12", resp.InputTokens)
	}
	if resp.OutputTokens != 7 {
		t.Errorf("OutputTokens = %d, want 7", resp.OutputTokens)
	}
	if resp.Provider != "ollama" {
		t.Errorf("Provider = %q, want %q", resp.Provider, "ollama")
	}
	if resp.Model != "llama3.1" {
		t.Errorf("Model = %q, want %q", resp.Model, "llama3.1")
	}
}

func TestOllamaProvider_Send_APIError(t *testing.T) {
	srv, p := newMockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprint(w, `{"error":"upstream unavailable"}`)
	})
	defer srv.Close()

	_, err := p.send(context.Background(), llmRequest{
		Messages: []llmMessage{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Error("send() should return error for non-200 status")
	}
	if !strings.Contains(err.Error(), "ollama API error 502") {
		t.Errorf("error should mention status 502, got: %v", err)
	}
}

func TestOllamaProvider_Stream_Success(t *testing.T) {
	streamBody := strings.Join([]string{
		`{"model":"llama3.1","message":{"role":"assistant","content":"hello "},"done":false}`,
		`{"model":"llama3.1","message":{"role":"assistant","content":"world"},"done":false}`,
		`{"model":"llama3.1","message":{"role":"assistant","content":""},"prompt_eval_count":14,"eval_count":9,"done":true}`,
	}, "\n") + "\n"

	srv, p := newMockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprint(w, streamBody)
	})
	defer srv.Close()

	var tokens []string
	resp, err := p.stream(context.Background(), llmRequest{
		Messages: []llmMessage{{Role: "user", Content: "hi"}},
	}, func(token string) {
		tokens = append(tokens, token)
	})
	if err != nil {
		t.Fatalf("stream() error: %v", err)
	}
	if strings.Join(tokens, "") != "hello world" {
		t.Errorf("joined tokens = %q, want %q", strings.Join(tokens, ""), "hello world")
	}
	if resp.Content != "hello world" {
		t.Errorf("Content = %q, want %q", resp.Content, "hello world")
	}
	if resp.InputTokens != 14 {
		t.Errorf("InputTokens = %d, want 14", resp.InputTokens)
	}
	if resp.OutputTokens != 9 {
		t.Errorf("OutputTokens = %d, want 9", resp.OutputTokens)
	}
	if resp.Provider != "ollama" {
		t.Errorf("Provider = %q, want %q", resp.Provider, "ollama")
	}
	if resp.Model != "llama3.1" {
		t.Errorf("Model = %q, want %q", resp.Model, "llama3.1")
	}
}

func TestOllamaProvider_Stream_APIError(t *testing.T) {
	srv, p := newMockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"error":"ollama unavailable"}`)
	})
	defer srv.Close()

	_, err := p.stream(context.Background(), llmRequest{
		Messages: []llmMessage{{Role: "user", Content: "hi"}},
	}, func(string) {})
	if err == nil {
		t.Error("stream() should return error for non-200 status")
	}
	if !strings.Contains(err.Error(), "ollama API error 503") {
		t.Errorf("error should mention status 503, got: %v", err)
	}
}

// ─── parseMessagesArgs Tests ─────────────────────────────────────────────────

func TestParseMessagesArgs(t *testing.T) {
	rt := goja.New()

	t.Run("valid messages array", func(t *testing.T) {
		prog, err := goja.Compile("", `[{role:"user",content:"hello"},{role:"assistant",content:"hi"}]`, false)
		if err != nil {
			t.Fatal(err)
		}
		val, err := rt.RunProgram(prog)
		if err != nil {
			t.Fatal(err)
		}

		call := goja.FunctionCall{
			Arguments: []goja.Value{val, rt.ToValue("system prompt")},
		}
		msgs, sys, err := parseMessagesArgs(rt, call)
		if err != nil {
			t.Fatalf("parseMessagesArgs() error: %v", err)
		}
		if sys != "system prompt" {
			t.Errorf("systemPrompt = %q, want %q", sys, "system prompt")
		}
		if len(msgs) != 2 {
			t.Errorf("messages count = %d, want 2", len(msgs))
		}
		if msgs[0].Role != "user" || msgs[0].Content != "hello" {
			t.Errorf("msgs[0] = %+v, want {user, hello}", msgs[0])
		}
		if msgs[1].Role != "assistant" || msgs[1].Content != "hi" {
			t.Errorf("msgs[1] = %+v, want {assistant, hi}", msgs[1])
		}
	})

	t.Run("non-array argument returns error", func(t *testing.T) {
		call := goja.FunctionCall{
			Arguments: []goja.Value{rt.ToValue("not-an-array"), rt.ToValue("")},
		}
		_, _, err := parseMessagesArgs(rt, call)
		if err == nil {
			t.Error("parseMessagesArgs() should return error for non-array")
		}
	})

	t.Run("empty array is valid", func(t *testing.T) {
		prog, _ := goja.Compile("", `[]`, false)
		val, _ := rt.RunProgram(prog)
		call := goja.FunctionCall{
			Arguments: []goja.Value{val, rt.ToValue("")},
		}
		msgs, _, err := parseMessagesArgs(rt, call)
		if err != nil {
			t.Fatalf("parseMessagesArgs() error for empty array: %v", err)
		}
		if len(msgs) != 0 {
			t.Errorf("messages count = %d, want 0", len(msgs))
		}
	})
}

// ─── aiModule Tests ──────────────────────────────────────────────────────────

func makeTestPromptFS() fs.ReadFileFS {
	return fstest.MapFS{
		"jsh-runtime.md":    &fstest.MapFile{Data: []byte("# JSH Runtime\nThis is the runtime docs.")},
		"machbase-sql.md":   &fstest.MapFile{Data: []byte("# SQL\nSELECT * FROM ...")},
		"not-a-segment.txt": &fstest.MapFile{Data: []byte("ignored")},
	}
}

func TestAIModule_ListSegments(t *testing.T) {
	rt := goja.New()
	promptFS := makeTestPromptFS()
	m := &aiModule{rt: rt, cfg: defaultLLMConfig(), promptFS: promptFS}
	m.provider = m.buildProvider("claude")

	segs, err := m.listSegments()
	if err != nil {
		t.Fatalf("listSegments() error: %v", err)
	}

	found := map[string]bool{}
	for _, s := range segs {
		found[s] = true
	}

	if !found["jsh-runtime"] {
		t.Error("listSegments() should include 'jsh-runtime'")
	}
	if !found["machbase-sql"] {
		t.Error("listSegments() should include 'machbase-sql'")
	}
	// .txt files should not appear
	if found["not-a-segment"] {
		t.Error("listSegments() should not include non-.md files")
	}
}

func TestAIModule_LoadSegment(t *testing.T) {
	rt := goja.New()
	promptFS := makeTestPromptFS()
	m := &aiModule{rt: rt, cfg: defaultLLMConfig(), promptFS: promptFS}
	m.provider = m.buildProvider("claude")

	t.Run("existing segment", func(t *testing.T) {
		content, err := m.loadSegment("jsh-runtime")
		if err != nil {
			t.Fatalf("loadSegment() error: %v", err)
		}
		if !strings.Contains(content, "JSH Runtime") {
			t.Errorf("content does not contain expected text, got: %q", content)
		}
	})

	t.Run("missing segment returns error", func(t *testing.T) {
		_, err := m.loadSegment("non-existent-segment")
		if err == nil {
			t.Error("loadSegment() should return error for missing segment")
		}
	})
}

func TestAIModule_LoadSegment_CustomOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp) // Windows uses USERPROFILE, not HOME

	// Create custom prompt dir and segment
	customDir, _ := llmCustomPromptDir()
	if err := os.MkdirAll(customDir, 0700); err != nil {
		t.Fatal(err)
	}
	customContent := "# Custom Override\nCustom content."
	if err := os.WriteFile(customDir+"/jsh-runtime.md", []byte(customContent), 0600); err != nil {
		t.Fatal(err)
	}

	rt := goja.New()
	promptFS := makeTestPromptFS()
	m := &aiModule{rt: rt, cfg: defaultLLMConfig(), promptFS: promptFS}

	content, err := m.loadSegment("jsh-runtime")
	if err != nil {
		t.Fatalf("loadSegment() error: %v", err)
	}
	if !strings.Contains(content, "Custom Override") {
		t.Errorf("custom override not applied, got: %q", content)
	}
}

func TestAIModule_BuildProvider(t *testing.T) {
	rt := goja.New()
	m := &aiModule{rt: rt, cfg: defaultLLMConfig()}
	m.provider = m.buildProvider("claude")

	t.Run("claude provider", func(t *testing.T) {
		p := m.buildProvider("claude")
		if p.name() != "claude" {
			t.Errorf("provider name = %q, want %q", p.name(), "claude")
		}
	})

	t.Run("openai provider", func(t *testing.T) {
		m.cfg.Providers["openai"] = &llmProviderConf{Model: "gpt-4o"}
		p := m.buildProvider("openai")
		if p.name() != "openai" {
			t.Errorf("provider name = %q, want %q", p.name(), "openai")
		}
	})

	t.Run("ollama provider", func(t *testing.T) {
		p := m.buildProvider("ollama")
		if p.name() != "ollama" {
			t.Errorf("provider name = %q, want %q", p.name(), "ollama")
		}
	})

	t.Run("unknown defaults to claude", func(t *testing.T) {
		p := m.buildProvider("unknown-provider")
		if p.name() != "claude" {
			t.Errorf("provider name = %q, want %q (fallback)", p.name(), "claude")
		}
	})
}

// ─── registerAIModule (JS API) Tests ─────────────────────────────────────────

func TestRegisterAIModule_JSAPIShape(t *testing.T) {
	rt := goja.New()
	obj := rt.NewObject()
	m := &aiModule{rt: rt, cfg: defaultLLMConfig(), promptFS: makeTestPromptFS()}
	m.provider = m.buildProvider("claude")

	// Register the ai object manually (like registerAIModule does internally)
	aiObj := rt.NewObject()
	aiObj.Set("send", m.jsSend)
	aiObj.Set("stream", m.jsStream)
	aiObj.Set("setProvider", m.jsSetProvider)
	aiObj.Set("providerInfo", m.jsProviderInfo)
	aiObj.Set("listSegments", m.jsListSegments)
	aiObj.Set("loadSegment", m.jsLoadSegment)
	aiObj.Set("editConfig", m.jsEditConfig)
	aiObj.Set("config", m.makeConfigObject())
	obj.Set("ai", aiObj)

	rt.Set("shell", obj)

	// Verify all expected functions are callable
	for _, method := range []string{"send", "stream", "setProvider", "providerInfo", "listSegments", "loadSegment", "editConfig"} {
		prog, err := goja.Compile("", fmt.Sprintf("typeof shell.ai.%s", method), false)
		if err != nil {
			t.Fatalf("compile error: %v", err)
		}
		val, err := rt.RunProgram(prog)
		if err != nil {
			t.Fatalf("runtime error: %v", err)
		}
		if val.String() != "function" {
			t.Errorf("shell.ai.%s type = %q, want %q", method, val.String(), "function")
		}
	}
}

func TestAIModule_JSListSegments(t *testing.T) {
	rt := goja.New()
	m := &aiModule{rt: rt, cfg: defaultLLMConfig(), promptFS: makeTestPromptFS()}
	m.provider = m.buildProvider("claude")

	call := goja.FunctionCall{}
	result := m.jsListSegments(call)

	arr, ok := result.Export().([]any)
	if !ok {
		t.Fatalf("jsListSegments() did not return array, got %T", result.Export())
	}
	names := make([]string, len(arr))
	for i, v := range arr {
		names[i] = fmt.Sprint(v)
	}

	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	if !found["jsh-runtime"] {
		t.Error("jsListSegments() should include 'jsh-runtime'")
	}
}

func TestAIModule_JSLoadSegment(t *testing.T) {
	rt := goja.New()
	m := &aiModule{rt: rt, cfg: defaultLLMConfig(), promptFS: makeTestPromptFS()}
	m.provider = m.buildProvider("claude")

	call := goja.FunctionCall{Arguments: []goja.Value{rt.ToValue("jsh-runtime")}}
	result := m.jsLoadSegment(call)

	content := result.String()
	if !strings.Contains(content, "JSH Runtime") {
		t.Errorf("jsLoadSegment() returned unexpected content: %q", content)
	}
}

func TestAIModule_JSLoadSegment_Missing(t *testing.T) {
	rt := goja.New()
	m := &aiModule{rt: rt, cfg: defaultLLMConfig(), promptFS: makeTestPromptFS()}
	m.provider = m.buildProvider("claude")

	call := goja.FunctionCall{Arguments: []goja.Value{rt.ToValue("does-not-exist")}}

	defer func() {
		if r := recover(); r == nil {
			t.Error("jsLoadSegment() should panic (throw JS error) for missing segment")
		}
	}()
	m.jsLoadSegment(call)
}

func TestAIModule_JSSetProvider(t *testing.T) {
	rt := goja.New()
	cfg := defaultLLMConfig()
	cfg.Providers["openai"] = &llmProviderConf{Model: "gpt-4o"}
	m := &aiModule{rt: rt, cfg: cfg}
	m.provider = m.buildProvider("claude")

	if m.provider.name() != "claude" {
		t.Errorf("initial provider = %q, want %q", m.provider.name(), "claude")
	}

	call := goja.FunctionCall{Arguments: []goja.Value{rt.ToValue("openai")}}
	m.jsSetProvider(call)

	if m.provider.name() != "openai" {
		t.Errorf("provider after setProvider = %q, want %q", m.provider.name(), "openai")
	}
}

func TestAIModule_JSProviderInfo(t *testing.T) {
	rt := goja.New()
	cfg := defaultLLMConfig()
	m := &aiModule{rt: rt, cfg: cfg}
	m.provider = m.buildProvider("claude")

	call := goja.FunctionCall{}
	result := m.jsProviderInfo(call)

	info, ok := result.Export().(map[string]any)
	if !ok {
		t.Fatalf("jsProviderInfo() did not return object, got %T", result.Export())
	}
	if info["name"] != "claude" {
		t.Errorf("info.name = %v, want %q", info["name"], "claude")
	}
	if info["model"] != "claude-opus-4-5" {
		t.Errorf("info.model = %v, want %q", info["model"], "claude-opus-4-5")
	}
	if info["hasApiKey"] != false {
		t.Errorf("info.hasApiKey = %v, want false", info["hasApiKey"])
	}
}

func TestAIModule_JSProviderInfo_ClaudeEnvAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "env-api-key")

	rt := goja.New()
	cfg := defaultLLMConfig()
	m := &aiModule{rt: rt, cfg: cfg}
	m.provider = m.buildProvider("claude")

	info, ok := m.jsProviderInfo(goja.FunctionCall{}).Export().(map[string]any)
	if !ok {
		t.Fatal("jsProviderInfo() did not return object")
	}
	if info["hasApiKey"] != true {
		t.Errorf("info.hasApiKey = %v, want true", info["hasApiKey"])
	}
}

func TestAIModule_JSProviderInfo_Ollama(t *testing.T) {
	rt := goja.New()
	cfg := defaultLLMConfig()
	m := &aiModule{rt: rt, cfg: cfg}
	m.provider = m.buildProvider("ollama")

	info, ok := m.jsProviderInfo(goja.FunctionCall{}).Export().(map[string]any)
	if !ok {
		t.Fatal("jsProviderInfo() did not return object")
	}
	if info["name"] != "ollama" {
		t.Errorf("info.name = %v, want %q", info["name"], "ollama")
	}
	if info["baseUrl"] != "http://127.0.0.1:11434" {
		t.Errorf("info.baseUrl = %v, want %q", info["baseUrl"], "http://127.0.0.1:11434")
	}
	if info["hasBaseUrl"] != true {
		t.Errorf("info.hasBaseUrl = %v, want true", info["hasBaseUrl"])
	}
}

func TestAIModule_JSProviderInfo_OpenAI(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env-openai-key")

	rt := goja.New()
	cfg := defaultLLMConfig()
	m := &aiModule{rt: rt, cfg: cfg}
	m.provider = m.buildProvider("openai")

	info, ok := m.jsProviderInfo(goja.FunctionCall{}).Export().(map[string]any)
	if !ok {
		t.Fatal("jsProviderInfo() did not return object")
	}
	if info["name"] != "openai" {
		t.Errorf("info.name = %v, want %q", info["name"], "openai")
	}
	if info["model"] != "gpt-4o" {
		t.Errorf("info.model = %v, want %q", info["model"], "gpt-4o")
	}
	if info["hasApiKey"] != true {
		t.Errorf("info.hasApiKey = %v, want true", info["hasApiKey"])
	}
	if info["baseUrl"] != "https://api.openai.com/v1" {
		t.Errorf("info.baseUrl = %v, want %q", info["baseUrl"], "https://api.openai.com/v1")
	}
}

func TestAIModule_ConfigObject(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp) // Windows uses USERPROFILE, not HOME

	rt := goja.New()
	m := newAIModule(rt, makeTestPromptFS())

	configObj := m.makeConfigObject()

	// Test config.path()
	pathFn, _ := goja.AssertFunction(configObj.Get("path"))
	pathVal, err := pathFn(goja.Undefined())
	if err != nil {
		t.Fatalf("config.path() error: %v", err)
	}
	if !strings.Contains(pathVal.String(), ".config") {
		t.Errorf("config.path() = %q, should contain .config", pathVal.String())
	}

	// Test config.load()
	loadFn, _ := goja.AssertFunction(configObj.Get("load"))
	loadVal, err := loadFn(goja.Undefined())
	if err != nil {
		t.Fatalf("config.load() error: %v", err)
	}
	cfgExported := loadVal.Export()
	if cfgExported == nil {
		t.Error("config.load() returned nil")
	}

	// Test config.set() round-trip
	setFn, _ := goja.AssertFunction(configObj.Get("set"))
	if _, err := setFn(goja.Undefined(), rt.ToValue("defaultProvider"), rt.ToValue("openai")); err != nil {
		t.Fatalf("config.set() error: %v", err)
	}
	if m.cfg.DefaultProvider != "openai" {
		t.Errorf("config.set() did not update defaultProvider, got %q", m.cfg.DefaultProvider)
	}
}

// ─── claudeProvider direct send/stream via transport override ────────────────

// mockRoundTripper redirects all requests to the test server, allowing
// claudeProvider.send() and .stream() to be tested without code changes.
type mockRoundTripper struct {
	target  string
	handler http.Handler
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	m.handler.ServeHTTP(w, req)
	return w.Result(), nil
}

// withMockTransport replaces http.DefaultTransport for the duration of f.
func withMockTransport(handler http.HandlerFunc, f func()) {
	orig := http.DefaultTransport
	http.DefaultTransport = &mockRoundTripper{handler: handler}
	defer func() { http.DefaultTransport = orig }()
	f()
}

func TestClaudeProvider_Send_Direct(t *testing.T) {
	responseBody := `{
		"content": [{"type": "text", "text": "direct send works"}],
		"usage": {"input_tokens": 7, "output_tokens": 3},
		"model": "claude-direct"
	}`
	p := newClaudeProvider(&llmProviderConf{APIKey: "test", Model: "claude-direct", MaxTokens: 100})

	withMockTransport(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if r.Header.Get("x-api-key") != "test" {
			t.Errorf("x-api-key = %q, want %q", r.Header.Get("x-api-key"), "test")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("anthropic-version header missing")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, responseBody)
	}, func() {
		resp, err := p.send(context.Background(), llmRequest{
			Messages: []llmMessage{{Role: "user", Content: "hi"}},
		})
		if err != nil {
			t.Fatalf("send() error: %v", err)
		}
		if resp.Content != "direct send works" {
			t.Errorf("Content = %q, want %q", resp.Content, "direct send works")
		}
		if resp.InputTokens != 7 {
			t.Errorf("InputTokens = %d, want 7", resp.InputTokens)
		}
		if resp.OutputTokens != 3 {
			t.Errorf("OutputTokens = %d, want 3", resp.OutputTokens)
		}
	})
}

func TestClaudeProvider_Send_Direct_Error(t *testing.T) {
	p := newClaudeProvider(&llmProviderConf{APIKey: "bad", Model: "claude-test"})

	withMockTransport(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"unauthorized"}`)
	}, func() {
		_, err := p.send(context.Background(), llmRequest{
			Messages: []llmMessage{{Role: "user", Content: "hi"}},
		})
		if err == nil {
			t.Error("send() should return error for non-200")
		}
		if !strings.Contains(err.Error(), "401") {
			t.Errorf("error should mention 401, got: %v", err)
		}
	})
}

func TestClaudeProvider_Stream_Direct(t *testing.T) {
	sseBody := strings.Join([]string{
		`data: {"type":"message_start","message":{"model":"claude-direct","usage":{"input_tokens":8,"output_tokens":0}}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"stream"}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" works"}}`,
		`data: {"type":"message_delta","usage":{"output_tokens":4}}`,
	}, "\n") + "\n"

	p := newClaudeProvider(&llmProviderConf{APIKey: "test", Model: "claude-direct", MaxTokens: 100})

	var tokens []string
	var resp *llmResponse
	var streamErr error

	withMockTransport(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseBody)
	}, func() {
		resp, streamErr = p.stream(context.Background(), llmRequest{
			Messages: []llmMessage{{Role: "user", Content: "hi"}},
		}, func(token string) {
			tokens = append(tokens, token)
		})
	})

	if streamErr != nil {
		t.Fatalf("stream() error: %v", streamErr)
	}
	if resp.Content != "stream works" {
		t.Errorf("Content = %q, want %q", resp.Content, "stream works")
	}
	if resp.InputTokens != 8 {
		t.Errorf("InputTokens = %d, want 8", resp.InputTokens)
	}
	if resp.OutputTokens != 4 {
		t.Errorf("OutputTokens = %d, want 4", resp.OutputTokens)
	}
	if resp.Model != "claude-direct" {
		t.Errorf("Model = %q, want %q", resp.Model, "claude-direct")
	}
	if strings.Join(tokens, "") != "stream works" {
		t.Errorf("tokens = %q, want %q", strings.Join(tokens, ""), "stream works")
	}
}

func TestClaudeProvider_Stream_Direct_Error(t *testing.T) {
	p := newClaudeProvider(&llmProviderConf{APIKey: "bad", Model: "claude-test"})

	withMockTransport(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error":"forbidden"}`)
	}, func() {
		_, err := p.stream(context.Background(), llmRequest{
			Messages: []llmMessage{{Role: "user", Content: "hi"}},
		}, func(string) {})
		if err == nil {
			t.Error("stream() should return error for non-200")
		}
	})
}

// ─── jsSend / jsStream via mock provider ─────────────────────────────────────

// mockProvider implements llmProvider for testing jsSend/jsStream.
type mockProvider struct {
	sendResp  *llmResponse
	sendErr   error
	streamErr error
	tokens    []string
}

func (p *mockProvider) name() string  { return "mock" }
func (p *mockProvider) model() string { return "mock-model" }

func (p *mockProvider) send(_ context.Context, _ llmRequest) (*llmResponse, error) {
	return p.sendResp, p.sendErr
}

func (p *mockProvider) stream(_ context.Context, _ llmRequest, onToken func(string)) (*llmResponse, error) {
	for _, tok := range p.tokens {
		onToken(tok)
	}
	return p.sendResp, p.streamErr
}

func TestAIModule_JSSend_Success(t *testing.T) {
	rt := goja.New()
	m := &aiModule{
		rt:  rt,
		cfg: defaultLLMConfig(),
		provider: &mockProvider{
			sendResp: &llmResponse{
				Content:      "mock response",
				InputTokens:  5,
				OutputTokens: 3,
				Provider:     "mock",
				Model:        "mock-model",
			},
		},
	}

	prog, _ := goja.Compile("", `[{role:"user",content:"hello"}]`, false)
	messages, _ := rt.RunProgram(prog)

	call := goja.FunctionCall{
		Arguments: []goja.Value{messages, rt.ToValue("system")},
	}
	result := m.jsSend(call)

	obj := result.ToObject(rt)
	if obj.Get("content").String() != "mock response" {
		t.Errorf("content = %q, want %q", obj.Get("content").String(), "mock response")
	}
	if obj.Get("provider").String() != "mock" {
		t.Errorf("provider = %q, want %q", obj.Get("provider").String(), "mock")
	}
}

func TestAIModule_JSSend_Error(t *testing.T) {
	rt := goja.New()
	m := &aiModule{
		rt:  rt,
		cfg: defaultLLMConfig(),
		provider: &mockProvider{
			sendErr: fmt.Errorf("mock send error"),
		},
	}

	prog, _ := goja.Compile("", `[{role:"user",content:"hello"}]`, false)
	messages, _ := rt.RunProgram(prog)

	call := goja.FunctionCall{
		Arguments: []goja.Value{messages, rt.ToValue("")},
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("jsSend() should panic on provider error")
		}
	}()
	m.jsSend(call)
}

func TestAIModule_JSSend_InvalidMessages(t *testing.T) {
	rt := goja.New()
	m := &aiModule{rt: rt, cfg: defaultLLMConfig(), provider: &mockProvider{}}

	call := goja.FunctionCall{
		Arguments: []goja.Value{rt.ToValue("not-an-array"), rt.ToValue("")},
	}
	defer func() {
		if r := recover(); r == nil {
			t.Error("jsSend() should panic on invalid messages")
		}
	}()
	m.jsSend(call)
}

func TestAIModule_JSStream_CallsHandlers(t *testing.T) {
	rt := goja.New()
	var collectedTokens []string
	var endCalled bool

	m := &aiModule{
		rt:  rt,
		cfg: defaultLLMConfig(),
		provider: &mockProvider{
			tokens: []string{"tok1", "tok2"},
			sendResp: &llmResponse{
				Content:      "tok1tok2",
				InputTokens:  2,
				OutputTokens: 2,
				Provider:     "mock",
				Model:        "mock-model",
			},
		},
	}

	prog, _ := goja.Compile("", `[{role:"user",content:"hi"}]`, false)
	messages, _ := rt.RunProgram(prog)

	// Build handler object { data, end, error } in the runtime.
	// Use rt.RunString to create native JS arrays so push() works correctly.
	rt.RunString(`var __tokens = []; var __endCalled = false;`)
	handlersCode := `({
		data: function(tok) { __tokens.push(tok); },
		end:  function(resp) { __endCalled = true; },
		error: function(err) {}
	})`
	handlers, _ := rt.RunString(handlersCode)

	call := goja.FunctionCall{
		Arguments: []goja.Value{messages, rt.ToValue(""), handlers},
	}

	result := m.jsStream(call)
	if result != goja.Undefined() {
		t.Error("jsStream() should return undefined (callback-based, not emitter)")
	}

	// Inspect collected state via runtime.
	tokVal := rt.Get("__tokens")
	if tokArr, ok := tokVal.Export().([]any); ok {
		for _, v := range tokArr {
			collectedTokens = append(collectedTokens, v.(string))
		}
	}
	endVal := rt.Get("__endCalled")
	endCalled = endVal.ToBoolean()

	if len(collectedTokens) == 0 {
		t.Error("expected data handler to be called with tokens")
	}
	if !endCalled {
		t.Error("expected end handler to be called")
	}
}

// ─── findHostEditor Tests ─────────────────────────────────────────────────────

func TestFindHostEditor_FromEnv(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	// If vi is in PATH, it should be returned
	ed := findHostEditor()
	// We just verify the function doesn't panic; vi may or may not be present.
	_ = ed
}

func TestFindHostEditor_EnvNotInPath(t *testing.T) {
	t.Setenv("EDITOR", "/nonexistent/editor-that-does-not-exist")
	// Should fall through to candidates
	ed := findHostEditor()
	// Result depends on whether vi/nano exist in the test environment.
	_ = ed
}

func TestAIExecutorExtractCodeBlocks_OnlyJshRun(t *testing.T) {
	rt := goja.New()
	module := rt.NewObject()
	exports := rt.NewObject()
	if err := module.Set("exports", exports); err != nil {
		t.Fatalf("module.exports setup failed: %v", err)
	}
	if err := rt.Set("module", module); err != nil {
		t.Fatalf("module setup failed: %v", err)
	}
	if err := rt.Set("exports", exports); err != nil {
		t.Fatalf("exports setup failed: %v", err)
	}
	if err := rt.Set("require", func(call goja.FunctionCall) goja.Value {
		name := call.Argument(0).String()
		if name != "@jsh/shell" {
			panic(rt.NewTypeError("unexpected module: %s", name))
		}
		obj := rt.NewObject()
		aiObj := rt.NewObject()
		aiObj.Set("exec", func(goja.FunctionCall) goja.Value { return goja.Undefined() })
		obj.Set("ai", aiObj)
		return obj
	}); err != nil {
		t.Fatalf("require setup failed: %v", err)
	}

	if _, err := rt.RunString(string(aiExecutorJS)); err != nil {
		t.Fatalf("loading ai_executor.js failed: %v", err)
	}

	exportsObj := module.Get("exports").ToObject(rt)
	extractVal := exportsObj.Get("extractCodeBlocks")
	extractFn, ok := goja.AssertFunction(extractVal)
	if !ok {
		t.Fatal("extractCodeBlocks export is not a function")
	}

	text := strings.Join([]string{
		"Example only:",
		"```jsh",
		"console.log('example');",
		"```",
		"",
		"Runnable:",
		"```jsh-run",
		"console.log('run');",
		"```",
		"",
		"Legacy JS fence:",
		"```javascript",
		"console.log('legacy');",
		"```",
	}, "\n")

	result, err := extractFn(goja.Undefined(), rt.ToValue(text))
	if err != nil {
		t.Fatalf("extractCodeBlocks() failed: %v", err)
	}

	blocks, ok := result.Export().([]any)
	if !ok {
		t.Fatalf("unexpected extractCodeBlocks() result type: %T", result.Export())
	}
	if len(blocks) != 1 {
		t.Fatalf("extractCodeBlocks() returned %d blocks, want 1", len(blocks))
	}

	block, ok := blocks[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected block type: %T", blocks[0])
	}
	if got := block["lang"]; got != "jsh-run" {
		t.Fatalf("block lang = %v, want jsh-run", got)
	}
	code, ok := block["code"].(string)
	if !ok {
		t.Fatalf("block code type = %T, want string", block["code"])
	}
	if !strings.Contains(code, "console.log('run');") {
		t.Fatalf("block code = %q, want runnable jsh-run content", code)
	}
}

func TestAIExecutorExtractCodeBlocks_MultipleRunnableBlocks(t *testing.T) {
	rt := goja.New()
	module := rt.NewObject()
	exports := rt.NewObject()
	if err := module.Set("exports", exports); err != nil {
		t.Fatalf("module.exports setup failed: %v", err)
	}
	if err := rt.Set("module", module); err != nil {
		t.Fatalf("module setup failed: %v", err)
	}
	if err := rt.Set("exports", exports); err != nil {
		t.Fatalf("exports setup failed: %v", err)
	}
	if err := rt.Set("require", func(call goja.FunctionCall) goja.Value {
		name := call.Argument(0).String()
		if name != "@jsh/shell" {
			panic(rt.NewTypeError("unexpected module: %s", name))
		}
		obj := rt.NewObject()
		aiObj := rt.NewObject()
		aiObj.Set("exec", func(goja.FunctionCall) goja.Value { return goja.Undefined() })
		obj.Set("ai", aiObj)
		return obj
	}); err != nil {
		t.Fatalf("require setup failed: %v", err)
	}

	if _, err := rt.RunString(string(aiExecutorJS)); err != nil {
		t.Fatalf("loading ai_executor.js failed: %v", err)
	}

	exportsObj := module.Get("exports").ToObject(rt)
	extractVal := exportsObj.Get("extractCodeBlocks")
	extractFn, ok := goja.AssertFunction(extractVal)
	if !ok {
		t.Fatal("extractCodeBlocks export is not a function")
	}

	text := strings.Join([]string{
		"```jsh-run",
		"console.log('first');",
		"```",
		"",
		"```jsh",
		"console.log('example');",
		"```",
		"",
		"```jsh-run",
		"console.log('second');",
		"```",
	}, "\n")

	result, err := extractFn(goja.Undefined(), rt.ToValue(text))
	if err != nil {
		t.Fatalf("extractCodeBlocks() failed: %v", err)
	}

	blocks, ok := result.Export().([]any)
	if !ok {
		t.Fatalf("unexpected extractCodeBlocks() result type: %T", result.Export())
	}
	if len(blocks) != 2 {
		t.Fatalf("extractCodeBlocks() returned %d blocks, want 2", len(blocks))
	}

	first, ok := blocks[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected first block type: %T", blocks[0])
	}
	second, ok := blocks[1].(map[string]any)
	if !ok {
		t.Fatalf("unexpected second block type: %T", blocks[1])
	}
	if !strings.Contains(first["code"].(string), "first") {
		t.Fatalf("first block code = %q, want first runnable block", first["code"])
	}
	if !strings.Contains(second["code"].(string), "second") {
		t.Fatalf("second block code = %q, want second runnable block", second["code"])
	}
}
