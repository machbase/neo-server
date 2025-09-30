package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/machbase/neo-server/v8/mods/server/chat"
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

func (cons *WebConsole) handleMessage(ctx context.Context, msg *eventbus.Message) {
	if msg.Ver != "1.0" {
		eventbus.PublishLog(cons.topic, "ERROR",
			fmt.Sprintf("unsupported msg.ver: %q", msg.Ver))
		return
	}
	if msg.Type != "question" {
		eventbus.PublishLog(cons.topic, "ERROR",
			fmt.Sprintf("invalid message type %s", msg.Type))
		return
	}
	if msg.Body == nil || msg.Body.OfQuestion == nil {
		eventbus.PublishLog(cons.topic, "ERROR",
			"missing question body")
		return
	}
	question := msg.Body.OfQuestion
	dc := DialogConfig{
		topic:    cons.topic,
		provider: question.Provider,
		model:    question.Model,
		msgID:    msg.ID,
	}
	d := dc.NewDialog()
	d.Talk(ctx, question.Text)
}

type DialogConfig struct {
	topic    string
	provider string
	model    string
	msgID    int64
}

type Dialog interface {
	Talk(context.Context, string)
}

var (
	_ Dialog = (*DialogUnknown)(nil)
	_ Dialog = (*DialogTesting)(nil)
	_ Dialog = (*chat.DialogCalude)(nil)
	_ Dialog = (*chat.DialogOllama)(nil)
)

func (c DialogConfig) NewDialog() Dialog {
	if llmTesting {
		return &DialogTesting{topic: c.topic, msgID: c.msgID, provider: c.provider, model: c.model}
	}
	var errorMsg string
	for _, p := range llmProviders {
		if p.Provider == c.provider && p.Model == c.model {
			switch c.provider {
			case "claude":
				ret := chat.NewClaudeDialog(c.topic, c.msgID, c.model)
				c.loadConfig(ret, "claude.json")
				ret.SystemMessages, _ = c.loadSystemMessages(ret.SystemMessages)
				return ret
			case "ollama":
				ret := chat.NewOllamaDialog(c.topic, c.msgID, c.model)
				c.loadConfig(ret, "ollama.json")
				ret.SystemMessages, _ = c.loadSystemMessages(ret.SystemMessages)
				return ret
			default:
				errorMsg = fmt.Sprintf("Unknown LLM provider: %s, model: %s", c.provider, c.model)
			}
		}
	}
	return &DialogUnknown{
		topic:    c.topic,
		provider: c.provider,
		model:    c.model,
		msgID:    c.msgID,
		error:    errorMsg,
	}
}

func (c DialogConfig) loadConfig(d interface{}, filename string) error {
	confDir := "."
	if dir, err := os.UserHomeDir(); err == nil {
		confDir = filepath.Join(dir, ".config", "machbase", "llm")
		if err := os.MkdirAll(confDir, 0755); err != nil {
			return err
		}
	} else {
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

func (c *DialogConfig) loadSystemMessages(messages []string) ([]string, error) {
	var confDir string
	if dir, err := os.UserHomeDir(); err == nil {
		confDir = filepath.Join(dir, ".config", "machbase", "llm")
	} else {
		return messages, nil
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

type DialogUnknown struct {
	topic    string
	msgID    int64
	provider string
	model    string
	error    string
}

func (d *DialogUnknown) Talk(ctx context.Context, _ string) {
	eventbus.PublishMessage(d.topic, &eventbus.Message{
		Ver:  "1.0",
		ID:   d.msgID,
		Type: eventbus.BodyTypeStreamBlockStart,
	})
	eventbus.PublishMessage(d.topic, &eventbus.Message{
		Ver:  "1.0",
		ID:   d.msgID,
		Type: eventbus.BodyTypeStreamBlockDelta,
		Body: &eventbus.BodyUnion{
			OfStreamBlockDelta: &eventbus.StreamBlockDelta{
				ContentType: "error",
				Text:        d.error,
			},
		},
	})
	eventbus.PublishMessage(d.topic, &eventbus.Message{
		Ver:  "1.0",
		ID:   d.msgID,
		Type: eventbus.BodyTypeStreamBlockStop,
	})
}

type DialogTesting struct {
	topic    string
	msgID    int64
	provider string
	model    string
}

func (d *DialogTesting) Talk(ctx context.Context, message string) {
	// Simulate a response
	eventbus.PublishMessage(d.topic, &eventbus.Message{
		Ver:  "1.0",
		ID:   d.msgID,
		Type: eventbus.BodyTypeStreamMessageStart,
	})
	eventbus.PublishMessage(d.topic, &eventbus.Message{
		Ver:  "1.0",
		ID:   d.msgID,
		Type: eventbus.BodyTypeStreamBlockStart,
	})
	eventbus.PublishMessage(d.topic, &eventbus.Message{
		Ver:  "1.0",
		ID:   d.msgID,
		Type: eventbus.BodyTypeStreamBlockDelta,
		Body: &eventbus.BodyUnion{
			OfStreamBlockDelta: &eventbus.StreamBlockDelta{
				ContentType: "text",
				Text: fmt.Sprintf("This is a simulated response from %s model %s to your message: %s\n",
					d.provider, d.model, message),
			},
		},
	})
	eventbus.PublishMessage(d.topic, &eventbus.Message{
		Ver:  "1.0",
		ID:   d.msgID,
		Type: eventbus.BodyTypeStreamBlockStop,
	})
	eventbus.PublishMessage(d.topic, &eventbus.Message{
		Ver:  "1.0",
		ID:   d.msgID,
		Type: eventbus.BodyTypeStreamMessageStop,
	})

}
