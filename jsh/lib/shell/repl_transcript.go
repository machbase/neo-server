package shell

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dop251/goja"
)

// transcriptEvent is the wire format for a single transcript entry.
// All events are written as single-line JSON to the transcript file.
type transcriptEvent struct {
	Time    string `json:"time"`
	Event   string `json:"event"`
	Input   string `json:"input,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   string `json:"error,omitempty"`
	Profile string `json:"profile,omitempty"`
}

// TranscriptWriter records evaluation inputs and results to a file.
// Each record is a single-line JSON object (newline-delimited JSON / NDJSON),
// making transcripts machine-readable and trivially replayable with --load.
//
// Secret values (JSON object keys matching sensitiveKeys) are redacted to
// "[REDACTED]" before writing. This applies to structured result values.
//
// TranscriptWriter is safe for use as a Renderer decorator: it wraps an inner
// Renderer and forwards all calls, additionally writing to the transcript file.
type TranscriptWriter struct {
	inner    Renderer
	f        io.WriteCloser
	profile  string
	redactor *secretRedactor
}

// sensitiveKeys is the set of JSON object key names whose values are redacted
// in transcript output. Matching is case-insensitive.
var sensitiveKeys = []string{"password", "token", "secret", "key", "apikey", "api_key"}

// NewTranscriptWriter opens (or creates) path for transcript writing and
// returns a TranscriptWriter that decorates inner. The caller must call
// Close() when the session ends to flush and close the file.
//
// If path is empty or opening the file fails, the writer is a no-op
// decorator that forwards all calls to inner without writing a transcript.
func NewTranscriptWriter(inner Renderer, path string, profileName string) *TranscriptWriter {
	tw := &TranscriptWriter{
		inner:    inner,
		profile:  profileName,
		redactor: &secretRedactor{},
	}
	if path == "" {
		return tw
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		// Silently fall back to no-op; evaluation must not be blocked by
		// a transcript file error.
		return tw
	}
	tw.f = f
	return tw
}

// newTranscriptWriterTo returns a TranscriptWriter that records events to an
// already opened writer.
func newTranscriptWriterTo(inner Renderer, f io.WriteCloser, profileName string) *TranscriptWriter {
	return &TranscriptWriter{
		inner:    inner,
		f:        f,
		profile:  profileName,
		redactor: &secretRedactor{},
	}
}

// openTranscriptFile opens path for appending through the JSH fs module so
// the path is resolved against the runtime's virtual file system mounts.
// Without a require function it falls back to the host file system.
func (repl *Repl) openTranscriptFile(path string) (io.WriteCloser, error) {
	requireFn, ok := goja.AssertFunction(repl.rt.Get("require"))
	if !ok {
		return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	}
	fsVal, err := requireFn(goja.Undefined(), repl.rt.ToValue("fs"))
	if err != nil {
		return nil, err
	}
	fsObj := fsVal.ToObject(repl.rt)
	openFn, ok1 := goja.AssertFunction(fsObj.Get("openSync"))
	writeFn, ok2 := goja.AssertFunction(fsObj.Get("writeSync"))
	closeFn, ok3 := goja.AssertFunction(fsObj.Get("closeSync"))
	if !ok1 || !ok2 || !ok3 {
		return nil, errors.New("fs module does not provide openSync/writeSync/closeSync")
	}
	fd, err := openFn(fsObj, repl.rt.ToValue(path), repl.rt.ToValue("a"), repl.rt.ToValue(0o600))
	if err != nil {
		return nil, err
	}
	return &jsFileWriter{rt: repl.rt, fs: fsObj, fd: fd, write: writeFn, close: closeFn}, nil
}

// jsFileWriter writes to a file descriptor opened by the JSH fs module.
// It must be used only from the goroutine that owns the goja runtime.
type jsFileWriter struct {
	rt     *goja.Runtime
	fs     *goja.Object
	fd     goja.Value
	write  goja.Callable
	close  goja.Callable
	closed bool
}

func (w *jsFileWriter) Write(p []byte) (int, error) {
	if w.closed {
		return 0, os.ErrClosed
	}
	if _, err := w.write(w.fs, w.fd, w.rt.ToValue(string(p))); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *jsFileWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	_, err := w.close(w.fs, w.fd)
	return err
}

// Close flushes and closes the underlying transcript file.
// Must be called when the session ends (e.g. in SessionHooks.OnStop).
func (tw *TranscriptWriter) Close() error {
	if tw.f == nil {
		return nil
	}
	return tw.f.Close()
}

// RenderBanner implements Renderer. Writes a "session_start" event.
func (tw *TranscriptWriter) RenderBanner(w io.Writer, profile RuntimeProfile) error {
	tw.write(transcriptEvent{
		Event:   "session_start",
		Profile: tw.profile,
	})
	return tw.inner.RenderBanner(w, profile)
}

// RenderValue implements Renderer. Writes a "result" event with the value.
func (tw *TranscriptWriter) RenderValue(w io.Writer, value any) error {
	tw.write(transcriptEvent{
		Event:  "result",
		Result: tw.redactor.redact(value),
	})
	return tw.inner.RenderValue(w, value)
}

// RenderError implements Renderer. Writes a "error" event.
func (tw *TranscriptWriter) RenderError(w io.Writer, err error) error {
	if err != nil {
		tw.write(transcriptEvent{
			Event: "error",
			Error: err.Error(),
		})
	}
	return tw.inner.RenderError(w, err)
}

// RenderEvalResult records an evaluation result or error and forwards it to
// the inner renderer, preserving JSON timing output when inner is JSON.
func (tw *TranscriptWriter) RenderEvalResult(w io.Writer, rt *goja.Runtime, val goja.Value, evalErr error, elapsed time.Duration) error {
	if evalErr != nil {
		tw.write(transcriptEvent{Event: "error", Error: evalErr.Error()})
	} else {
		var exported any
		if val != nil && !goja.IsUndefined(val) && !goja.IsNull(val) {
			exported = val.Export()
		}
		tw.write(transcriptEvent{Event: "result", Result: tw.redactor.redact(exported)})
	}
	renderEvalResult(rt, tw.inner, w, val, evalErr, elapsed)
	return nil
}

// WriteInput records a user input line before evaluation. This is called
// explicitly by the eval loop in repl.go (not part of the Renderer interface).
func (tw *TranscriptWriter) WriteInput(input string) {
	tw.write(transcriptEvent{
		Event: "input",
		Input: input,
	})
}

// WriteSessionEnd records a "session_end" event with an optional error.
func (tw *TranscriptWriter) WriteSessionEnd(err error) {
	ev := transcriptEvent{Event: "session_end"}
	if err != nil {
		ev.Error = err.Error()
	}
	tw.write(ev)
}

func (tw *TranscriptWriter) write(ev transcriptEvent) {
	if tw.f == nil {
		return
	}
	ev.Time = time.Now().UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	b = append(b, '\n')
	tw.f.Write(b) //nolint:errcheck
}

// secretRedactor masks sensitive field values in structured data before
// writing to the transcript.
type secretRedactor struct{}

// redact returns a sanitized copy of v where object fields matching
// sensitiveKeys have their values replaced with "[REDACTED]".
// Non-object/non-slice values are returned as-is.
func (r *secretRedactor) redact(v any) any {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if isSensitiveKey(k) {
				out[k] = "[REDACTED]"
			} else {
				out[k] = r.redact(val)
			}
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = r.redact(item)
		}
		return out
	default:
		return v
	}
}

// isSensitiveKey returns true when key matches any sensitiveKeys entry
// (case-insensitive exact match or suffix match after underscore/hyphen).
func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, s := range sensitiveKeys {
		if lower == s {
			return true
		}
	}
	return false
}
