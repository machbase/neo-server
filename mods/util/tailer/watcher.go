package tailer

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
	"time"
)

type Handler struct {
	CutPrefix string
	Terminal  Terminal

	fsServer http.Handler
	closeCh  chan struct{}
}

var _ http.Handler = Handler{}

func (to Terminal) Handler(cutPrefix string) Handler {
	return Handler{
		CutPrefix: cutPrefix,
		Terminal:  to,
		fsServer:  http.FileServerFS(staticFS),
		closeCh:   to.closeCh,
	}
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "watch.stream") {
		h.serveWatcher(w, r)
	} else {
		h.serveStatic(w, r)
	}
}

func (h Handler) serveWatcher(w http.ResponseWriter, r *http.Request) {
	// TODO: use array instead of map to keep order
	selectedTails := map[string][]Option{}
	if len(h.Terminal.tails) == 1 {
		to := h.Terminal.tails[0]
		selectedTails[to.Filename] = to.Options
	} else if h.Terminal.controlBar.Hide {
		// select all tails if control bar is not visible
		for _, to := range h.Terminal.tails {
			selectedTails[to.Filename] = to.Options
		}
	} else {
		fileParams := r.URL.Query()["file"]
		for _, to := range h.Terminal.tails {
			if slices.Contains(fileParams, to.Alias) {
				selectedTails[to.Filename] = to.Options
			}
		}
	}

	if len(selectedTails) == 0 {
		http.Error(w, "no logs selected", http.StatusBadRequest)
		return
	}

	opts := []Option{
		WithPollInterval(500 * time.Millisecond),
		WithBufferSize(1000),
	}

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

	var tail ITail
	if len(selectedTails) == 1 {
		for filename, opts := range selectedTails {
			tail = New(filename, opts...)
		}
	} else {
		var tails []ITail
		for filename, tailOpts := range selectedTails {
			t := New(filename, append(tailOpts, opts...)...)
			tails = append(tails, t)
		}
		tail = NewMultiTail(tails...)
	}
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
		case <-h.closeCh:
			return
		}
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
		err := tmplIndex.Execute(w, h.dataMap())
		if err != nil {
			http.Error(w, "Failed to render index.html", http.StatusInternalServerError)
		}
		return
	}
	h.fsServer.ServeHTTP(w, r)
}

func (h Handler) dataMap() TemplateData {
	files := []string{}
	for _, to := range h.Terminal.tails {
		files = append(files, to.Alias)
	}
	ctrlBar := h.Terminal.controlBar
	if ctrlBar.FontSize == 0 {
		ctrlBar.FontSize = h.Terminal.FontSize
	}
	if ctrlBar.FontFamily == "" {
		ctrlBar.FontFamily = h.Terminal.FontFamily
	}
	return TemplateData{
		Terminal:   h.Terminal,
		ControlBar: ctrlBar,
		Files:      files,
	}
}

type TemplateData struct {
	Terminal   Terminal
	ControlBar ControlBar
	Files      []string
}

func (td TemplateData) Localize(s string) string {
	if l, ok := td.Terminal.Localization[s]; ok {
		return l
	}
	return s
}

type Terminal struct {
	CursorBlink         bool          `json:"cursorBlink"`
	CursorInactiveStyle string        `json:"cursorInactiveStyle,omitempty"`
	CursorStyle         string        `json:"cursorStyle,omitempty"`
	FontSize            int           `json:"fontSize,omitempty"`
	FontFamily          string        `json:"fontFamily,omitempty"`
	Theme               TerminalTheme `json:"theme"`
	Scrollback          int           `json:"scrollback,omitempty"`
	DisableStdin        bool          `json:"disableStdin"`
	ConvertEol          bool          `json:"convertEol,omitempty"`

	tails        []TailOption      `json:"-"`
	controlBar   ControlBar        `json:"-"`
	closeCh      chan struct{}     `json:"-"`
	Localization map[string]string `json:"-"`
}

type TailOption struct {
	Filename string   `json:"filename"`
	Options  []Option `json:"options"`
	Alias    string   `json:"alias"`
	Label    string   `json:"label"`
}

type ControlBar struct {
	Hide       bool   `json:"hide"`
	FontSize   int    `json:"fontSize,omitempty"`
	FontFamily string `json:"fontFamily,omitempty"`
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

func (tt Terminal) String() string {
	opts, _ := json.MarshalIndent(tt, "", "  ")
	return string(opts)
}

type TerminalOption func(*Terminal)

func WithFontSize(size int) TerminalOption {
	return func(to *Terminal) {
		to.FontSize = size
	}
}

func WithFontFamily(family string) TerminalOption {
	return func(to *Terminal) {
		to.FontFamily = family
	}
}

func WithScrollback(lines int) TerminalOption {
	return func(to *Terminal) {
		to.Scrollback = lines
	}
}

func WithTheme(theme TerminalTheme) TerminalOption {
	return func(to *Terminal) {
		to.Theme = theme
	}
}

func WithControlBar(cb ControlBar) TerminalOption {
	return func(to *Terminal) {
		to.controlBar = cb
	}
}

func WithLocalization(localization map[string]string) TerminalOption {
	return func(to *Terminal) {
		to.Localization = localization
	}
}

func WithTail(filename string, opts ...Option) TerminalOption {
	return WithTailLabel(filepath.Base(filename), filename, opts...)
}

func WithTailLabel(label string, filename string, opts ...Option) TerminalOption {
	return func(to *Terminal) {
		alias := StripAnsiCodes(label)
		to.tails = append(to.tails, TailOption{
			Filename: filename,
			Options:  append([]Option{WithLabel(label)}, opts...),
			Label:    label,
			Alias:    alias,
		})
	}
}

func NewTerminal(opts ...TerminalOption) Terminal {
	to := DefaultTerminal()
	for _, opt := range opts {
		opt(&to)
	}
	return to
}

func DefaultTerminal() Terminal {
	return Terminal{
		CursorBlink:  false,
		FontSize:     12,
		FontFamily:   `"Monaspace Neon",ui-monospace,SFMono-Regular,"SF Mono",Menlo,Consolas,monospace`,
		Theme:        ThemeDefault,
		Scrollback:   5000,
		DisableStdin: true, // Terminal is read-only
		closeCh:      make(chan struct{}),
		Localization: map[string]string{},
	}
}

// Close stops any active watchers associated with the terminal
// and explicitly stop sse sessions.
// the http server might be blocked on Shutdown()
// if there are active watchers.
func (t Terminal) Close() {
	close(t.closeCh)
}
