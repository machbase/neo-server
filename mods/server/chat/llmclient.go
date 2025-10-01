package chat

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/ollama/ollama/api"
)

type LLMMessage struct {
	IsError     bool   `json:"isError,omitempty"`
	IsPartial   bool   `json:"isPartial,omitempty"`
	Type        string `json:"type,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	Content     string `json:"content"`
}

type LLMDialog struct {
	conf  *LLMConfig
	ch    chan LLMMessage
	model string

	log logging.Log `json:"-"`
}

func ExecLLM(ctx context.Context, c *LLMConfig, model string, userMessage string) <-chan LLMMessage {
	d := NewDialog(c)
	go func() {
		defer d.Close()
		if strings.HasPrefix(model, "claude:") {
			if c.Claude.Key == "" {
				d.SendError("Claude model selected but no API key configured.")
				return
			}
			d.model = strings.TrimPrefix(model, "claude:")
			d.execClaude(ctx, userMessage)
		} else if strings.HasPrefix(model, "ollama:") {
			if c.Ollama.Url == "" {
				d.SendError("Ollama model selected but no Ollama URL configured.")
				return
			}
			d.model = strings.TrimPrefix(model, "ollama:")
			d.execOllama(ctx, userMessage)
		} else {
			d.SendError("Unknown model prefix. Please use 'claude:' or 'ollama:'.")
			return
		}
	}()
	return d.ch
}

func NewDialog(conf *LLMConfig) *LLMDialog {
	return &LLMDialog{
		conf: conf,
		ch:   make(chan LLMMessage),
		log:  logging.GetLog("chat"),
	}
}

func (d *LLMDialog) Close() {
	if d.ch != nil {
		close(d.ch)
		d.ch = nil
	}
}

func (d *LLMDialog) SendMessage(format string, args ...any) {
	m := LLMMessage{
		IsError: false,
	}
	if len(args) > 0 {
		m.Content = fmt.Sprintf(format, args...)
	} else {
		m.Content = format
	}
	d.Send(m)
}

func (d *LLMDialog) SendError(msg string, args ...any) {
	m := LLMMessage{
		IsError: true,
	}
	if len(args) > 0 {
		m.Content = fmt.Sprintf(msg, args...)
	} else {
		m.Content = msg
	}
	d.Send(m)
}

func (d *LLMDialog) Send(m LLMMessage) {
	if d.ch == nil {
		log.Println("Dialog channel is closed, cannot send message")
		return
	}
	d.ch <- m
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
