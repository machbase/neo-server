package shell

import (
	jshrl "github.com/machbase/neo-server/v8/jsh/lib/readline"
	"github.com/nyaosorg/go-readline-ny"
)

// SessionHistory is the shared history contract used by Shell and Repl.
// It remains compatible with multiline.Editor while allowing a no-op history
// implementation when history is disabled.
type SessionHistory interface {
	readline.IHistory
	Add(line string)
}

type disabledHistory struct{}

func (disabledHistory) Len() int      { return 0 }
func (disabledHistory) At(int) string { return "" }
func (disabledHistory) Add(string)    {}

// NewHistory creates the shared history implementation for editor sessions.
// It never returns nil so callers can safely pass the result into
// multiline.Editor.SetHistory without additional checks.
func NewHistory(cfg HistoryConfig) SessionHistory {
	if !cfg.Enabled {
		return disabledHistory{}
	}
	size := cfg.Size
	if size <= 0 {
		size = 100
	}
	name := cfg.Name
	if name == "" {
		name = "history"
	}
	return jshrl.NewHistory(name, size)
}
