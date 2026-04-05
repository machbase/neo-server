package shell

import (
	"os"

	"github.com/hymkor/go-multiline-ny"
)

// EditorSession is the result of NewEditorSession. It holds both the
// multiline.Editor and the SessionHistory so callers do not need to
// re-extract the history via a type assertion on ed.LineEditor.History.
type EditorSession struct {
	Editor  *multiline.Editor
	History SessionHistory
}

// NewEditorSession builds the shared editor/session wiring used by Shell and
// Repl. Product-specific customization should remain in Hooks.ConfigureEditor.
func NewEditorSession(cfg SessionConfig) EditorSession {
	ed := &multiline.Editor{}
	ed.SetTty(NewTty()) // See TtyWrap comment.

	writer := cfg.Writer
	if writer == nil {
		writer = os.Stdout
	}
	ed.SetWriter(writer)

	hist := NewHistory(cfg.History)
	ed.SetHistory(hist)
	ed.SetHistoryCycling(cfg.EnableHistoryCycling)

	if cfg.Hooks.Prompt != nil {
		ed.SetPrompt(cfg.Hooks.Prompt)
	}
	ed.SubmitOnEnterWhen(cfg.Hooks.SubmitOnEnterWhen)

	if cfg.Hooks.ConfigureEditor != nil {
		cfg.Hooks.ConfigureEditor(ed)
	}

	return EditorSession{Editor: ed, History: hist}
}
