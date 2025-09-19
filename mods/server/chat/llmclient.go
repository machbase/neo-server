package chat

import (
	"context"
	"fmt"
	"log"

	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/ollama/ollama/api"
)

type LLMConfig struct {
	Ollama         LLMOllamaConfig  `json:"ollama"`
	Claude         LLMClaudeConfig  `json:"claude"`
	ChatModel      string           `json:"-"`
	ToolModel      string           `json:"toolModel"`
	ToolMessages   []LLMToolMessage `json:"toolMessages,omitempty"`
	MCPSSEEndpoint string           `json:"mcpSSEEndpoint"`
}

type LLMToolMessage struct {
	Role     string `json:"role"`
	Content  string `json:"content"`
	Thinking string `json:"thinking,omitempty"`
}

type LLMMessage struct {
	IsError bool   `json:"isError"`
	Content string `json:"content"`
}

type LLMDialog struct {
	conf        LLMConfig
	ch          chan LLMMessage
	userMessage string

	log logging.Log `json:"-"`
}

func ExecLLM(ctx context.Context, c LLMConfig, userMessage string) <-chan LLMMessage {
	d := NewDialog(userMessage, c)
	go func() {
		defer d.Close()
		if c.Claude.Key == "" && c.Ollama.Url == "" {
			d.SendError("No LLM configured. Please set either Ollama URL or Claude API key.")
			return
		}
		if c.Claude.Key != "" {
			d.execClaude(ctx)
		} else {
			d.execOllama(ctx)
		}
	}()
	return d.ch
}

func NewDialog(userMessage string, conf LLMConfig) *LLMDialog {
	return &LLMDialog{
		conf:        conf,
		ch:          make(chan LLMMessage),
		userMessage: userMessage,
		log:         logging.GetLog("chat"),
	}
}

func (d *LLMDialog) Close() {
	if d.ch != nil {
		close(d.ch)
		d.ch = nil
	}
}

func (d *LLMDialog) SendMessage(msg string, args ...any) {
	d.Send(false, msg, args...)
}

func (d *LLMDialog) SendError(msg string, args ...any) {
	d.Send(true, msg, args...)
}

func (d *LLMDialog) Send(isError bool, msg string, args ...any) {
	if d.ch == nil {
		log.Println("Dialog channel is closed, cannot send message")
		return
	}
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	d.ch <- LLMMessage{
		IsError: isError,
		Content: msg,
	}
}

// Helper function to safely get string values from map
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// Helper function to safely get string values from map
func getType(m map[string]interface{}, key string) api.PropertyType {
	if v, ok := m[key].(string); ok {
		return api.PropertyType([]string{v})
	}
	return api.PropertyType([]string{})
}
