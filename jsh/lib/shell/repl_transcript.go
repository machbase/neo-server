package shell

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"time"
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
