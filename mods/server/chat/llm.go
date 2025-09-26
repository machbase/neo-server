package chat

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/mods/util/mdconv"
)

type LLMConfig struct {
	Ollama LLMOllamaConfig `json:"ollama"`
	Claude LLMClaudeConfig `json:"claude"`
	MCP    MCPConfig       `json:"mcp"`
}

type MCPConfig struct {
	Endpoint string `json:"endpoint"`
}

func loadLLMConfig() (*LLMConfig, error) {
	var config *LLMConfig

	confDir := "."
	if dir, err := os.UserHomeDir(); err == nil {
		confDir = filepath.Join(dir, ".config", "machbase")
	} else {
		return nil, fmt.Errorf("unable to get user home directory: %v", err)
	}
	confFile := filepath.Join(confDir, "llm_config.json")
	if _, err := os.Stat(confFile); os.IsNotExist(err) {
		fmt.Printf("Warning: LLM config file not found at %s, using default configuration\n", confFile)
		config = &LLMConfig{
			MCP: MCPConfig{
				Endpoint: "http://127.0.0.1:5654/db/mcp/sse",
			},
			Claude: LLMClaudeConfig{
				Key:       "",
				MaxTokens: 1000,
				SystemMessages: []string{
					`You are a friendly AI assistant for Machbase Neo DB.`,
				},
			},
			Ollama: LLMOllamaConfig{
				Url: "http://127.0.0.1:11434",
				SystemMessages: []string{
					"You are a database that executes SQL statement and return the results.",
				},
			},
		}
		file, err := os.OpenFile(confFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		// Write default config to file
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(&config); err != nil {
			return nil, err
		}
	} else {
		file, err := os.Open(confFile)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&config); err != nil {
			return nil, err
		}
	}
	if m, err := loadSystemMessages(confDir, config.Claude.SystemMessages); err != nil {
		return nil, fmt.Errorf("error reading system message file: %v", err)
	} else {
		config.Claude.SystemMessages = m
	}
	if m, err := loadSystemMessages(confDir, config.Ollama.SystemMessages); err != nil {
		return nil, fmt.Errorf("error reading system message file: %v", err)
	} else {
		config.Ollama.SystemMessages = m
	}
	return config, nil
}

func loadSystemMessages(confDir string, messages []string) ([]string, error) {
	for i, m := range messages {
		if strings.HasPrefix(m, "@") {
			// Load tool message from file
			filePath := strings.TrimPrefix(m, "@")
			filePath = filepath.Join(confDir, filePath)
			content, err := os.ReadFile(filePath)
			if err != nil {
				return nil, err
			}
			messages[i] = string(content)
		}
	}
	return messages, nil
}

type LLMProvider struct {
	Name     string `json:"name"`     // display name
	Provider string `json:"provider"` // "claude" or "ollama"
	Model    string `json:"model"`    // model identifier
}

func loadLLMProviders() []LLMProvider {
	fallbackModels := []LLMProvider{
		{Name: "Claude Sonnet 4", Provider: "claude", Model: "claude-sonnet-4-20250514"},
		{Name: "Ollama qwen3:0.6b", Provider: "ollama", Model: "qwen3:0.6b"},
	}

	confDir := "."
	if dir, err := os.UserHomeDir(); err == nil {
		confDir = filepath.Join(dir, ".config", "machbase")
	} else {
		return nil
	}
	confFile := filepath.Join(confDir, "llm_models.json")
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

var models []LLMProvider

func ChatModelsHandler(w http.ResponseWriter, r *http.Request) {
	tick := time.Now()
	if len(models) == 0 {
		models = loadLLMProviders()
	}
	w.Header().Set("Content-Type", "application/json")
	rsp := map[string]any{
		"success": true,
		"elapsed": time.Since(tick).String(),
		"data": map[string]any{
			"models": models,
		},
	}
	json.NewEncoder(w).Encode(rsp)
}

func ChatMarkdownHandler(w http.ResponseWriter, r *http.Request) {
	content, err := io.ReadAll(r.Body)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.Write(content)
		return
	}
	w.Header().Set("Content-Type", "text/xhtml")
	conv := mdconv.New(mdconv.WithDarkMode(false))
	conv.ConvertString(string(content), w)
}
