package shell

import (
	"encoding/json"
	"io"
	"time"

	"github.com/dop251/goja"
)

// AgentRenderer implements Renderer for agent/machine-readable output.
// Every output line is a self-contained JSON object, enabling simple
// line-by-line parsing by agent orchestrators without any screen-scraping.
//
// Output schema:
//
//	banner:  {"event":"banner","profile":"<name>","text":"..."}
//	value:   {"ok":true,  "type":"<js-type>", "value":<json>, "elapsedMs":<N>}
//	value:   {"ok":true,  "type":"<js-type>", "value":<json>, "elapsedMs":<N>, "truncated":true}
//	error:   {"ok":false, "error":"<message>", "elapsedMs":<N>}
//
// When MaxOutputBytes is non-zero and the serialized value exceeds that limit,
// the value field is replaced with a truncation notice and truncated=true is set.
type AgentRenderer struct {
	// MaxOutputBytes limits the JSON-serialized value size in bytes.
	// When exceeded, value is replaced with "[truncated: N bytes]" and
	// truncated=true is set in the output object. 0 = no limit.
	MaxOutputBytes int
}

// evalResult is the wire format for a single agent evaluation result.
type evalResult struct {
	OK        bool   `json:"ok"`
	Type      string `json:"type,omitempty"`
	Value     any    `json:"value,omitempty"`
	Error     string `json:"error,omitempty"`
	ElapsedMs int64  `json:"elapsedMs"`
	Truncated bool   `json:"truncated,omitempty"`
}

// RenderBanner emits a JSON banner event. In agent mode banners are structural
// events rather than human-readable text. If the profile has no banner the
// method is a no-op (agent profiles set Banner=nil for clean output).
func (r *AgentRenderer) RenderBanner(w io.Writer, profile RuntimeProfile) error {
	text := profile.ResolveBanner()
	if text == "" {
		return nil
	}
	return r.writeJSON(w, map[string]any{
		"event":   "banner",
		"profile": profile.Name,
		"text":    text,
	})
}

// RenderValue emits a structured JSON result for a value produced outside the
// evaluation loop (e.g. from a preload module). ElapsedMs is 0 because no
// timing context is available.
func (r *AgentRenderer) RenderValue(w io.Writer, value any) error {
	return r.emitResult(w, value, nil, 0)
}

// RenderError emits a structured JSON error line.
func (r *AgentRenderer) RenderError(w io.Writer, err error) error {
	if err == nil {
		return nil
	}
	return r.emitResult(w, nil, err, 0)
}

// RenderEvalResult is called by renderEvalResult() in repl.go with full timing
// context from the evaluation loop. It is NOT part of the Renderer interface;
// it is dispatched via type assertion in renderEvalResult().
func (r *AgentRenderer) RenderEvalResult(w io.Writer, rt *goja.Runtime, val goja.Value, evalErr error, elapsed time.Duration) error {
	ms := elapsed.Milliseconds()
	if evalErr != nil {
		return r.emitResult(w, nil, evalErr, ms)
	}
	return r.emitResult(w, val, nil, ms)
}

// emitResult serializes a value or error into one JSON line.
func (r *AgentRenderer) emitResult(w io.Writer, value any, evalErr error, elapsedMs int64) error {
	res := evalResult{ElapsedMs: elapsedMs}

	if evalErr != nil {
		res.OK = false
		res.Error = evalErr.Error()
		return r.writeJSON(w, res)
	}

	res.OK = true

	if value == nil {
		res.Type = "undefined"
		return r.writeJSON(w, res)
	}

	if val, ok := value.(goja.Value); ok {
		res.Type, res.Value, res.Truncated = r.marshalGojaValue(val)
	} else {
		// Non-goja value (e.g. from RenderValue called with a plain Go type).
		res.Type = "value"
		res.Value = value
	}

	return r.writeJSON(w, res)
}

// marshalGojaValue extracts the JS type name and a serializable Go value from
// a goja.Value. When MaxOutputBytes is set and the serialized form exceeds it,
// the value is replaced with a truncation notice.
func (r *AgentRenderer) marshalGojaValue(val goja.Value) (typeName string, exported any, truncated bool) {
	if val == nil || val == goja.Null() {
		return "null", nil, false
	}
	if val == goja.Undefined() {
		return "undefined", nil, false
	}
	if _, isFunc := goja.AssertFunction(val); isFunc {
		name := ""
		if obj, ok := val.(*goja.Object); ok {
			if n := obj.Get("name"); n != nil && n != goja.Undefined() {
				name = n.String()
			}
		}
		if name != "" && name != "anonymous" {
			return "function", "[Function: " + name + "]", false
		}
		return "function", "[Function (anonymous)]", false
	}

	raw := val.Export()
	switch raw.(type) {
	case string:
		typeName = "string"
	case int64, float64:
		typeName = "number"
	case bool:
		typeName = "boolean"
	default:
		typeName = "object"
	}

	if r.MaxOutputBytes > 0 {
		b, err := json.Marshal(raw)
		if err == nil && len(b) > r.MaxOutputBytes {
			return typeName, "[truncated: " + itoa(len(b)) + " bytes]", true
		}
	}

	return typeName, raw, false
}

// writeJSON marshals v as a single JSON line (no indent) followed by a newline.
func (r *AgentRenderer) writeJSON(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}

// itoa converts an int to its decimal string representation without fmt.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
