package chat

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
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
			Model     string              `json:"model"`
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
	config, err := loadLLMConfig()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error loading LLM config: %v", err), http.StatusInternalServerError)
		return
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
	config    *LLMConfig
}

func newBot(ctx context.Context, sessionId string, w http.ResponseWriter, conf *LLMConfig) *Bot {
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
			model := msg.Params.Arguments.Model

			replyCh := ExecLLM(b.ctx, b.config, model, question)
			seq := 0
			for reply := range replyCh {
				if reply.IsError {
					b.SendError(reply.Content)
					continue
				}
				b.SendStream(reply, seq, false)
				seq++
			}
			b.SendStream(LLMMessage{}, seq, true) // End of stream
		}
	}
}

func (b *Bot) SendStream(msg LLMMessage, seq int, end bool) {
	if end {
		streamResponse := fmt.Sprintf(`data: {"type":"stream","delta":"","seq":%d,"end":true}`+"\n\n", seq)
		fmt.Fprint(b.w, streamResponse)
		b.flusher.Flush()
		return
	}
	if msg.IsError {
		b.SendError(msg.Content)
		return
	}
	// TODO marshal msg to JSON
	switch msg.Type {
	case "thinking":
		// skip thinking messages
		return
	case "message-delta":
		// including "tool_use" and "end_turn"
		return
	}
	streamResponse := fmt.Sprintf(`data: {"type":"stream","delta":%q,"seq":%d,"end":%t}`+"\n\n",
		msg.Content, seq, end)
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
