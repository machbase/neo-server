package chat

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	InitWithConfig("../../../tmp/chat_llm")
	SetTesting(true)
	m.Run()
	os.RemoveAll("../../../tmp/chat_llm")
}

func TestRpcLLMProviders(t *testing.T) {
	types := RpcLLMGetProviders()
	require.EqualValues(t, []string{"claude", "ollama"}, types)

	providers, err := RpcLLMGetModels()
	require.NoError(t, err)
	require.EqualValues(t, map[string][]LLMProvider{
		"claude": {
			{Name: "Claude Sonnet 4", Provider: "claude", Model: "claude-sonnet-4-20250514"},
		},
		"ollama": {
			{Name: "Ollama qwen3:0.6b", Provider: "ollama", Model: "qwen3:0.6b"},
		},
	}, providers)

	err = RpcLLMAddModels0([]LLMProvider{
		{Name: "Ollama deepseek-r1:1.5b", Provider: "ollama", Model: "deepseek-r1:1.5b"},
	})
	require.NoError(t, err)

	providers, err = RpcLLMGetModels()
	require.NoError(t, err)
	require.EqualValues(t, map[string][]LLMProvider{
		"claude": {
			{Name: "Claude Sonnet 4", Provider: "claude", Model: "claude-sonnet-4-20250514"},
		},
		"ollama": {
			{Name: "Ollama qwen3:0.6b", Provider: "ollama", Model: "qwen3:0.6b"},
			{Name: "Ollama deepseek-r1:1.5b", Provider: "ollama", Model: "deepseek-r1:1.5b"},
		},
	}, providers)

	err = RpcLLMRemoveModels0([]LLMProvider{
		{Name: "Ollama deepseek-r1:1.5b", Provider: "ollama", Model: "deepseek-r1:1.5b"},
	})
	require.NoError(t, err)

	providers, err = RpcLLMGetModels()
	require.NoError(t, err)
	require.EqualValues(t, map[string][]LLMProvider{
		"claude": {
			{Name: "Claude Sonnet 4", Provider: "claude", Model: "claude-sonnet-4-20250514"},
		},
		"ollama": {
			{Name: "Ollama qwen3:0.6b", Provider: "ollama", Model: "qwen3:0.6b"},
		},
	}, providers)
}

func TestRpcLLMProviderConfig(t *testing.T) {
	RpcLLMSetProviderConfig("claude", map[string]any{
		"key":        "some-realically-long-string",
		"max_tokens": 1000,
	})
	RpcLLMSetProviderConfig("ollama", map[string]any{
		"url": "http://127.0.0.1:12345",
	})

	cfg, err := RpcLLMGetProviderConfig("claude")
	require.NoError(t, err)
	require.EqualValues(t, ClaudeConfig{
		Key:       "some-rea*******************",
		MaxTokens: 1000,
	}, cfg)

	cfg, err = RpcLLMGetProviderConfig("ollama")
	require.NoError(t, err)
	require.EqualValues(t, OllamaConfig{
		Url: "http://127.0.0.1:12345",
	}, cfg)

	cfg, err = RpcLLMGetProviderConfig("unknown")
	require.Error(t, err)
	require.Nil(t, cfg)
	err = RpcLLMSetProviderConfig("unknown", nil)
	require.Error(t, err)

	RpcLLMSetProviderConfig("claude", map[string]any{
		"key":        "your-key",
		"max_tokens": 1024,
	})
	RpcLLMSetProviderConfig("ollama", map[string]any{
		"url": "http://127.0.0.1:11434",
	})

	cfg, err = RpcLLMGetProviderConfig("claude")
	require.NoError(t, err)
	require.EqualValues(t, ClaudeConfig{
		Key:       "your-key",
		MaxTokens: 1024,
	}, cfg)

	cfg, err = RpcLLMGetProviderConfig("ollama")
	require.NoError(t, err)
	require.EqualValues(t, OllamaConfig{
		Url: "http://127.0.0.1:11434",
	}, cfg)
}

func TestNewTestingDialog(t *testing.T) {
	isTesting = true
	defer func() { isTesting = false }()
	cfg := DialogConfig{
		Topic:    "test_topic",
		Provider: "testing_provider",
		Model:    "testing_model",
		MsgID:    12345,
	}
	dialog := cfg.NewDialog()
	if dialog == nil {
		t.Fatal("Expected non-nil dialog")
	}
	dig, ok := dialog.(*TestingDialog)
	if !ok {
		t.Fatalf("Expected TestingDialog type, got %T", dialog)
	}
	require.Equal(t, "test_topic", dig.topic)
	require.Equal(t, int64(12345), dig.msgID)
	require.Equal(t, "testing_provider", dig.provider)
	require.Equal(t, "testing_model", dig.model)
}

func TestNewUnknownDialog(t *testing.T) {
	cfg := DialogConfig{
		Topic:    "test_topic",
		Provider: "unknown_provider",
		Model:    "unknown_model",
		MsgID:    12345,
	}
	dialog := cfg.NewDialog()
	if dialog == nil {
		t.Fatal("Expected non-nil dialog")
	}
	dig, ok := dialog.(*UnknownDialog)
	if !ok {
		t.Fatalf("Expected UnknownDialog type, got %T", dialog)
	}
	require.Equal(t, "test_topic", dig.topic)
	require.Equal(t, int64(12345), dig.msgID)
	require.Equal(t, "unknown_provider", dig.provider)
	require.Equal(t, "unknown_model", dig.model)
	require.Equal(t, "Unknown LLM provider: unknown_provider, model: unknown_model", dig.error)
}

func TestOllamaDialog(t *testing.T) {
	cfg := DialogConfig{
		Topic:    "test_topic",
		Provider: "ollama",
		Model:    "qwen3:0.6b",
		MsgID:    12345,
	}
	dialog := cfg.NewDialog()
	if dialog == nil {
		t.Fatal("Expected non-nil dialog")
	}
	diag, ok := dialog.(*OllamaDialog)
	if !ok {
		t.Fatalf("Expected OllamaDialog type, got %T", dialog)
	}
	require.Equal(t, "test_topic", diag.topic)
	require.Equal(t, int64(12345), diag.msgID)
	require.Equal(t, "qwen3:0.6b", diag.model)
	require.Equal(t, "http://127.0.0.1:11434", diag.Url) // default URL
	require.NotNil(t, diag.systemMessages)
	require.Greater(t, len(diag.systemMessages), 0)
}

func TestClaudeDialog(t *testing.T) {
	cfg := DialogConfig{
		Topic:    "test_topic",
		Provider: "claude",
		Model:    "claude-sonnet-4-20250514",
		MsgID:    12345,
		Session:  "test_session",
	}
	dialog := cfg.NewDialog()
	if dialog == nil {
		t.Fatal("Expected non-nil dialog")
	}
	diag, ok := dialog.(*ClaudeDialog)
	if !ok {
		t.Fatalf("Expected ClaudeDialog type, got %T", dialog)
	}
	require.Equal(t, "test_topic", diag.topic)
	require.Equal(t, int64(12345), diag.msgID)
	require.Equal(t, "claude-sonnet-4-20250514", diag.model)
	require.Equal(t, int64(1024), diag.MaxTokens) // default max tokens
	require.Equal(t, "your-key", diag.Key)        // default key
	require.NotNil(t, diag.systemMessages)
	require.Greater(t, len(diag.systemMessages), 0)
}
