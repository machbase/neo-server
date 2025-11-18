package tailer

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"text/template"
	"time"
)

type Handler struct {
	Filename  string
	CutPrefix string
	fsServer  http.Handler
	tailOpts  []Option

	TerminalOption TerminalOption
}

var shutdownCh = make(chan struct{})

// Shutdown signals all SSE handlers to shut down
// This will cause all active watchers to terminate gracefully.
func Shutdown() {
	close(shutdownCh)
}

var _ http.Handler = Handler{}

func (to TerminalOption) Handler(cutPrefix string, filepath string, opts ...Option) Handler {
	return Handler{
		Filename:  filepath,
		CutPrefix: cutPrefix,
		fsServer:  http.FileServerFS(staticFS),
		tailOpts: append([]Option{
			WithPollInterval(500 * time.Millisecond),
			WithBufferSize(1000),
		}, opts...),
		TerminalOption: to,
	}
}

func NewHandler(cutPrefix string, filepath string, opts ...Option) Handler {
	return DefaultTerminalOption().Handler(cutPrefix, filepath, opts...)
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "watch.stream") {
		h.serveWatcher(w, r)
	} else {
		h.serveStatic(w, r)
	}
}

func (h Handler) serveWatcher(w http.ResponseWriter, r *http.Request) {
	if h.Filename == "" {
		http.Error(w, "Filename not configured", http.StatusNotImplemented)
		return
	}

	opts := append([]Option{}, h.tailOpts...)

	filterParam := r.URL.Query().Get("filter")
	filters := strings.Split(filterParam, "||")
	for _, filter := range filters {
		splits := strings.Split(filter, "&&")
		toks := make([]string, 0, len(splits))
		for _, tok := range splits {
			tok = strings.TrimSpace(tok)
			if tok != "" {
				toks = append(toks, tok)
			}
		}
		if len(toks) > 0 {
			opts = append(opts, WithPattern(toks...))
		}
	}

	tail := New(h.Filename, opts...)
	if err := tail.Start(); err != nil {
		http.Error(w, "Failed to start watcher", http.StatusInternalServerError)
		return
	}
	defer tail.Stop()

	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	rc.Flush()

	flushTicker := time.NewTicker(1 * time.Second)
	defer flushTicker.Stop()
	for {
		select {
		case <-flushTicker.C:
			rc.Flush()
		case line := <-tail.Lines():
			fmt.Fprintf(w, "data: %s\n\n", line)
		case <-r.Context().Done():
			return
		case <-shutdownCh:
			return
		}
	}
}

type TerminalOption struct {
	CursorBlink         bool          `json:"cursorBlink"`
	CursorInactiveStyle string        `json:"cursorInactiveStyle,omitempty"`
	CursorStyle         string        `json:"cursorStyle,omitempty"`
	FontSize            int           `json:"fontSize,omitempty"`
	FontFamily          string        `json:"fontFamily,omitempty"`
	Theme               TerminalTheme `json:"theme"`
	Scrollback          int           `json:"scrollback,omitempty"`
	DisableStdin        bool          `json:"disableStdin"`
	ConvertEol          bool          `json:"convertEol,omitempty"`
}

type TerminalTheme struct {
	Background                  string `json:"background,omitempty"`
	Foreground                  string `json:"foreground.omitempty"`
	SelectionBackground         string `json:"selectionBackground,omitempty"`
	SelectionForeground         string `json:"selectionForeground,omitempty"`
	SelectionInactiveBackground string `json:"selectionInactiveBackground,omitempty"`
	Cursor                      string `json:"cursor,omitempty"`
	CursorAccent                string `json:"cursorAccent,omitempty"`
	ExtendedAnsi                string `json:"extendedAnsi,omitempty"`
	Black                       string `json:"black,omitempty"`
	Blue                        string `json:"blue,omitempty"`
	BrightBlack                 string `json:"brightBlack,omitempty"`
	BrightBlue                  string `json:"brightBlue,omitempty"`
	BrightCyan                  string `json:"brightCyan,omitempty"`
	BrightGreen                 string `json:"brightGreen,omitempty"`
	BrightMagenta               string `json:"brightMagenta,omitempty"`
	BrightRed                   string `json:"brightRed,omitempty"`
	BrightWhite                 string `json:"brightWhite,omitempty"`
	BrightYellow                string `json:"brightYellow,omitempty"`
	Cyan                        string `json:"cyan,omitempty"`
	Green                       string `json:"green,omitempty"`
	Magenta                     string `json:"magenta,omitempty"`
	Red                         string `json:"red,omitempty"`
	White                       string `json:"white,omitempty"`
	Yellow                      string `json:"yellow,omitempty"`
}

func (tt TerminalOption) String() string {
	opts, _ := json.MarshalIndent(tt, "", "  ")
	return string(opts)
}

func DefaultTerminalOption() TerminalOption {
	return TerminalOption{
		CursorBlink:  false,
		FontSize:     12,
		FontFamily:   `"Monaspace Neon",ui-monospace,SFMono-Regular,"SF Mono",Menlo,Consolas,monospace`,
		Theme:        ThemeDefault,
		Scrollback:   5000,
		DisableStdin: true, // Terminal is read-only
	}
}

//go:embed static/*
var staticFS embed.FS

var tmplIndex *template.Template

func (h Handler) serveStatic(w http.ResponseWriter, r *http.Request) {
	if tmplIndex == nil {
		if b, err := staticFS.ReadFile("static/index.html"); err != nil {
			http.Error(w, "Failed to read index.html", http.StatusInternalServerError)
			return
		} else {
			tmplIndex = template.Must(template.New("index").Parse(string(b)))
		}
	}
	r.URL.Path = "static/" + strings.TrimPrefix(r.URL.Path, h.CutPrefix)
	if r.URL.Path == "static/" {
		err := tmplIndex.Execute(w, map[string]any{
			"TerminalOptions": h.TerminalOption,
		})
		if err != nil {
			http.Error(w, "Failed to render index.html", http.StatusInternalServerError)
		}
		return
	}
	h.fsServer.ServeHTTP(w, r)
}
