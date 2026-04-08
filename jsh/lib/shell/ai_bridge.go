package shell

import (
	"context"
	"fmt"
	"strings"
)

// LLMChatMessage represents one chat message for provider requests.
type LLMChatMessage struct {
	Role    string
	Content string
}

// LLMStreamRequest describes one streaming request to the shared LLM core.
type LLMStreamRequest struct {
	Provider     string
	Model        string
	SystemPrompt string
	MaxTokens    int
	Messages     []LLMChatMessage
}

// LLMStreamResponse contains normalized completion metadata from providers.
type LLMStreamResponse struct {
	Content      string
	InputTokens  int
	OutputTokens int
	Provider     string
	Model        string
}

// StreamLLM streams tokens using the same provider core used by ai.js.
// It is intended for non-goja callers (e.g. service RPC handlers).
func StreamLLM(ctx context.Context, req LLMStreamRequest, onToken func(string)) (*LLMStreamResponse, error) {
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("messages are required")
	}

	cfg, err := LoadLLMConfig()
	if err != nil || cfg == nil {
		cfg = DefaultLLMConfig()
	}

	providerName := strings.TrimSpace(req.Provider)
	if providerName == "" {
		providerName = cfg.DefaultProvider
	}
	providerName, err = normalizeProviderName(providerName)
	if err != nil {
		return nil, err
	}

	conf := cfg.Providers[providerName]
	if conf == nil {
		conf = &LLMProviderConf{}
	}

	var provider LLMProvider
	switch providerName {
	case "openai":
		provider = newOpenAIProvider(conf)
	case "ollama":
		provider = newOllamaProvider(conf)
	default:
		provider = newClaudeProvider(conf)
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = defaultModelForProvider(providerName)
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = conf.MaxTokens
	}

	msgs := make([]LLMMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, LLMMessage{Role: m.Role, Content: m.Content})
	}

	if onToken == nil {
		onToken = func(string) {}
	}

	resp, err := provider.stream(ctx, LLMRequest{
		Messages:     msgs,
		SystemPrompt: req.SystemPrompt,
		Model:        model,
		MaxTokens:    maxTokens,
	}, onToken)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("empty llm response")
	}

	if resp.Provider == "" {
		resp.Provider = providerName
	}
	if resp.Model == "" {
		resp.Model = model
	}

	return &LLMStreamResponse{
		Content:      resp.Content,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
		Provider:     resp.Provider,
		Model:        resp.Model,
	}, nil
}
