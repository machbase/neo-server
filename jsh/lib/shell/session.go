package shell

import (
	"errors"
	"io"
	"os"

	"github.com/dop251/goja"
	"github.com/hymkor/go-multiline-ny"
)

// EditorSession is the result of NewEditorSession. It holds both the
// multiline.Editor and the SessionHistory so callers do not need to
// re-extract the history via a type assertion on ed.LineEditor.History.
//
// Renderer is the active render hook for the session. Shell and Repl may
// supply a product-specific Renderer via SessionConfig; otherwise the default
// WriterRenderer is used. Callers can invoke Renderer methods directly using
// the Writer field as the output destination.
type EditorSession struct {
	Editor   *multiline.Editor
	History  SessionHistory
	Profile  RuntimeProfile
	Renderer Renderer
	Writer   io.Writer
	hooks    SessionHooks
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

	renderer := cfg.Renderer
	if renderer == nil {
		renderer = WriterRenderer{}
	}

	return EditorSession{
		Editor:   ed,
		History:  hist,
		Profile:  cfg.Profile,
		Renderer: renderer,
		Writer:   writer,
		hooks:    cfg.Hooks,
	}
}

func (s EditorSession) Banner() string {
	return s.Profile.ResolveBanner()
}

func (s EditorSession) Metadata(runtimeName string) RuntimeMetadata {
	return s.Profile.RuntimeMetadata(runtimeName)
}

func (s EditorSession) Start(rt *goja.Runtime) error {
	if err := s.Profile.RunStartup(rt); err != nil {
		return err
	}
	if s.hooks.OnStart != nil {
		return s.hooks.OnStart()
	}
	return nil
}

func (s EditorSession) Stop(loopErr error) error {
	if s.hooks.OnStop == nil {
		return nil
	}
	stopErr := s.hooks.OnStop(loopErr)
	if loopErr == nil {
		return stopErr
	}
	if stopErr == nil {
		return loopErr
	}
	return errors.Join(loopErr, stopErr)
}
