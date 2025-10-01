package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// isTesting for testing purposes
var isTesting bool

func SetTesting(testing bool) {
	isTesting = testing
}

func configDir() (string, error) {
	if isTesting {
		confDir := "../../tmp/llm"
		if err := os.MkdirAll(confDir, 0755); err != nil {
			return "", err
		}
		return confDir, nil
	}
	confDir := "."
	if dir, err := os.UserHomeDir(); err == nil {
		confDir = filepath.Join(dir, ".config", "machbase", "llm")
		if err := os.MkdirAll(confDir, 0755); err != nil {
			return "", err
		}
	} else {
		return "", err
	}
	return confDir, nil
}

func SaveConfig(d interface{}, filename string) error {
	confDir, err := configDir()
	if err != nil {
		return err
	}

	switch c := d.(type) {
	case ClaudeDialog:
		m := map[string]any{}
		if err := LoadConfig(m, "claude.json"); err != nil {
			return err
		}
		m["key"] = c.Key
		m["max_tokens"] = c.MaxTokens
		d = m
	case OllamaDialog:
		m := map[string]any{}
		if err := LoadConfig(m, "ollama.json"); err != nil {
			return err
		}
		m["url"] = c.Url
		d = m
	}
	confFile := filepath.Join(confDir, filename)
	// make backup .bak
	if _, err := os.Stat(confFile); err == nil {
		os.Rename(confFile, confFile+time.Now().Format(".bak.20060102_150405"))
	}
	file, err := os.OpenFile(confFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	// Write config to file
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(d); err != nil {
		return err
	}
	return nil
}

func LoadConfig(d interface{}, filename string) error {
	confDir, err := configDir()
	if err != nil {
		return err
	}

	confFile := filepath.Join(confDir, filename)
	if _, err := os.Stat(confFile); os.IsNotExist(err) {
		file, err := os.OpenFile(confFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		defer file.Close()
		// Write default config to file
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		encoder.Encode(d)
	} else {
		file, err := os.Open(confFile)
		if err != nil {
			return err
		}
		defer file.Close()
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(d); err != nil {
			return err
		}
	}
	switch c := d.(type) {
	case *ClaudeDialog:
		c.SystemMessages, _ = loadSystemMessages(c.SystemMessages)
	case *OllamaDialog:
		c.SystemMessages, _ = loadSystemMessages(c.SystemMessages)
	}
	return nil
}

func loadSystemMessages(messages []string) ([]string, error) {
	confDir, err := configDir()
	if err != nil {
		return messages, err
	}

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

func loadLLMProviders() []LLMProvider {
	fallbackModels := llmFallbackProviders

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
