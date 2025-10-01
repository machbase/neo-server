package chat

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewTestingDialog(t *testing.T) {
	SetTesting(true, "../../../tmp/llm")

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
	SetTesting(false, "../../../tmp/llm")
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
	SetTesting(false, "../../../tmp/llm")
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
	SetTesting(false, "../../../tmp/llm")
	cfg := DialogConfig{
		Topic:    "test_topic",
		Provider: "claude",
		Model:    "claude-sonnet-4-20250514",
		MsgID:    12345,
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
