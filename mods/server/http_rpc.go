package server

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func init() {
	llmProviders = loadLLMProviders()

	RegisterWebSocketRPCHandler("llmGetProviders", handleLLMGetProviders)
}

type LLMProvider struct {
	Name     string `json:"name"`     // display name
	Provider string `json:"provider"` // "claude" or "ollama"
	Model    string `json:"model"`    // model identifier
}

var llmProviders = []LLMProvider{}
var llmTesting bool

func handleLLMGetProviders() ([]LLMProvider, error) {
	if llmTesting {
		return []LLMProvider{
			{Name: "Claude Sonnet 4", Provider: "claude", Model: "claude-sonnet-4-20250514"},
			{Name: "Ollama qwen3:0.6b", Provider: "ollama", Model: "qwen3:0.6b"},
		}, nil
	}
	return llmProviders, nil
}

// useTestingLLMProviders sets the llmProviders to default values for testing purposes
func useTestingLLMProviders() {
	llmTesting = true
}

func loadLLMProviders() []LLMProvider {
	fallbackModels := []LLMProvider{
		{Name: "Claude Sonnet 4", Provider: "claude", Model: "claude-sonnet-4-20250514"},
		{Name: "Ollama qwen3:0.6b", Provider: "ollama", Model: "qwen3:0.6b"},
	}

	confDir := "."
	if dir, err := os.UserHomeDir(); err == nil {
		confDir = filepath.Join(dir, ".config", "machbase", "llm")
		if err := os.MkdirAll(confDir, 0755); err != nil {
			return fallbackModels
		}
	} else {
		return fallbackModels
	}

	confFile := filepath.Join(confDir, "models.json")
	if _, err := os.Stat(confFile); os.IsNotExist(err) {
		file, err := os.OpenFile(confFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return fallbackModels
		}
		defer file.Close()
		// Write default config to file
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		encoder.Encode(fallbackModels)
		return fallbackModels
	} else {
		file, err := os.Open(confFile)
		if err != nil {
			return fallbackModels
		}
		defer file.Close()
		var models []LLMProvider
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&models); err != nil {
			return fallbackModels
		}
		if len(models) == 0 {
			return fallbackModels
		}
		return models
	}
}
