package chat

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed ui
var chatFS embed.FS

var chatDir = http.FS(chatFS)

func ChatDirHandler() http.Handler {
	return http.StripPrefix("/db/chat", http.FileServer(chatDir))
}

type Message struct {
	Method string `json:"method"`
	Params struct {
		Name      string `json:"name"`
		Arguments struct {
			Message   string              `json:"message"`
			History   []map[string]string `json:"history"`
			SessionId string              `json:"sessionId"`
		} `json:"arguments"`
	} `json:"params"`
}

func ChatMessageHandler(w http.ResponseWriter, r *http.Request) {
	msg := Message{}
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, fmt.Sprintf("Error decoding message: %v", err), http.StatusBadRequest)
		return
	}
	botsMutex.Lock()
	defer botsMutex.Unlock()
	bot, exists := bots[msg.Params.Arguments.SessionId]
	if !exists {
		http.Error(w, "Bot not found", http.StatusNotFound)
		return
	}
	bot.input <- msg
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, `{"status":"success","message":"Message received"}`)
}

func ChatSSEHandler(w http.ResponseWriter, r *http.Request) {
	sessionId := r.URL.Query().Get("sessionId")

	// SSE Headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// LLMConfig
	var config LLMConfig

	confDir := "."
	if dir, err := os.UserHomeDir(); err == nil {
		confDir = filepath.Join(dir, ".config", "machbase")
	} else {
		fmt.Printf("Warning: Unable to get user home directory, using current directory for config: %v\n", err)
	}
	confFile := filepath.Join(confDir, "llm_config.json")
	if _, err := os.Stat(confFile); os.IsNotExist(err) {
		fmt.Printf("Warning: LLM config file not found at %s, using default configuration\n", confFile)
		config = LLMConfig{
			MCPSSEEndpoint: "http://127.0.0.1:5654/db/mcp/sse",
			OllamaUrl:      "http://127.0.0.1:11434",
			ChatModel:      "deepseek-r1:8b",
			ToolModel:      "qwen3:0.6b",
			ToolMessages: []LLMToolMessage{
				{
					Role:    "system",
					Content: "You are a database that executes SQL statement and return the results.",
				},
			},
		}
		file, err := os.OpenFile(confFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error creating config file: %v", err), http.StatusInternalServerError)
			return
		}
		defer file.Close()
		// Write default config to file
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(&config); err != nil {
			http.Error(w, fmt.Sprintf("Error encoding config file: %v", err), http.StatusInternalServerError)
			return
		}
	} else {
		file, err := os.Open(confFile)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error opening config file: %v", err), http.StatusInternalServerError)
			return
		}
		defer file.Close()
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&config); err != nil {
			http.Error(w, fmt.Sprintf("Error decoding config file: %v", err), http.StatusInternalServerError)
			return
		}
	}

	for i, m := range config.ToolMessages {
		if strings.HasPrefix(m.Content, "@") {
			// Load tool message from file
			filePath := strings.TrimPrefix(m.Content, "@")
			filePath = filepath.Join(confDir, filePath)
			content, err := os.ReadFile(filePath)
			if err != nil {
				http.Error(w, fmt.Sprintf("Error reading tool message file: %v", err), http.StatusInternalServerError)
				return
			}
			config.ToolMessages[i].Content = string(content)
		}
	}

	bot := newBot(r.Context(), sessionId, w, config)
	if bot == nil {
		return
	}
	botsMutex.Lock()
	bots[sessionId] = bot
	botsMutex.Unlock()

	defer func() {
		botsMutex.Lock()
		delete(bots, sessionId)
		botsMutex.Unlock()
	}()

	bot.exec()
}

var botsMutex sync.Mutex
var bots = make(map[string]*Bot)

type Bot struct {
	ctx       context.Context
	sessionId string
	w         http.ResponseWriter
	flusher   http.Flusher
	input     chan Message
	config    LLMConfig
}

func newBot(ctx context.Context, sessionId string, w http.ResponseWriter, conf LLMConfig) *Bot {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return nil
	}
	return &Bot{
		ctx:       ctx,
		sessionId: sessionId,
		flusher:   flusher,
		w:         w,
		input:     make(chan Message, 10),
		config:    conf,
	}
}

func (b *Bot) exec() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// initial response
	fmt.Fprintf(b.w, `data: {"type":"connected","message":"Chat connected"}`+"\n\n")
	b.flusher.Flush()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			fmt.Fprintf(b.w, `data: {"type":"ping","timestamp":%d}`+"\n\n", time.Now().Unix())
			b.flusher.Flush()
		case msg := <-b.input:
			// simple error handling
			if msg.Params.Arguments.Message == "error" {
				b.SendError("Test error message")
				continue
			}

			question := msg.Params.Arguments.Message

			replyCh := ExecLLM(b.ctx, b.config, question)
			seq := 0
			for reply := range replyCh {
				if reply.IsError {
					b.SendError(reply.Content)
					continue
				}
				b.SendStream(reply.Content, seq, false)
				seq++
			}
			b.SendStream("", seq, true) // End of stream
		}
	}
}

func (b *Bot) SendStream(content string, seq int, end bool) {
	streamResponse := fmt.Sprintf(`data: {"type":"stream","delta":%q,"seq":%d,"end":%t}`+"\n\n", content, seq, end)
	fmt.Fprint(b.w, streamResponse)
	b.flusher.Flush()
}

// non-streaming response
func (b *Bot) SendResponse(content string) {
	finalResponse := fmt.Sprintf(`data: {"type":"response","content":%q}`+"\n\n", content)
	fmt.Fprint(b.w, finalResponse)
	b.flusher.Flush()
}

func (b *Bot) SendError(message string) {
	errorResponse := fmt.Sprintf(`data: {"type":"error","message":%q}`+`\n\n`, message)
	fmt.Fprint(b.w, errorResponse)
	b.flusher.Flush()
}
