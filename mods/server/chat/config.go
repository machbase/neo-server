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
var dirConfig string

func Init() {
	llmProvidersMutex.Lock()
	defer llmProvidersMutex.Unlock()
	llmProviders = loadLLMProviders()
}

func InitWithConfig(dir string) {
	llmProvidersMutex.Lock()
	defer llmProvidersMutex.Unlock()
	if dir != "" {
		dirConfig = dir
	}
	llmProviders = loadLLMProviders()
}

func SetTesting(testing bool) {
	isTesting = testing
}

func configDir() (string, error) {
	if dirConfig != "" {
		confDir := dirConfig
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
	old := map[string]interface{}{}
	confFile := filepath.Join(confDir, filename)
	// remove old backup files
	files, err := os.ReadDir(confDir)
	if err == nil {
		for _, f := range files {
			if strings.HasPrefix(f.Name(), filename+".bak.") {
				os.Remove(filepath.Join(confDir, f.Name()))
			}
		}
	}
	// make backup .bak
	if _, err := os.Stat(confFile); err == nil {
		bakFile := confFile + time.Now().Format(".bak.20060102_150405")
		if err := os.Rename(confFile, bakFile); err == nil {
			data, err := os.ReadFile(bakFile)
			if err == nil {
				json.Unmarshal(data, &old)
			}
		}
	}
	newBytes, _ := json.Marshal(d)
	new := map[string]interface{}{}
	json.Unmarshal(newBytes, &new)
	// restore old values for missing keys
	for k, v := range old {
		if _, ok := new[k]; !ok {
			new[k] = v
		}
	}
	d = new

	// Open config file for writing
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

func ExistsConfig(filename string) bool {
	confDir, err := configDir()
	if err != nil {
		return false
	}
	confFile := filepath.Join(confDir, filename)
	if _, err := os.Stat(confFile); os.IsNotExist(err) {
		return false
	}
	return true
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

func loadLLMProviders() map[string][]LLMProvider {
	fallbackModels := llmFallbackProviders

	confDir, err := configDir()
	if err != nil {
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
		var ret = map[string][]LLMProvider{}
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&ret); err != nil {
			return fallbackModels
		}
		if len(ret) == 0 {
			return fallbackModels
		}
		return ret
	}
}
