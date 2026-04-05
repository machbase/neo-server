package shell

import (
	"io"

	"github.com/hymkor/go-multiline-ny"
)

// Phase S1 boundary lock:
//
//   - Shell is the command interpreter product.
//   - Repl is the JavaScript runtime console product.
//   - Shared foundation is limited to editor/history/profile/render/bootstrap
//     hooks that are neutral to both products.
//   - Command parsing and JavaScript evaluation are intentionally not shared.
//
// Shell-only responsibilities (never extracted):
//   - command line parsing
//   - statement and operator handling
//   - alias expansion
//   - process execution
//   - shell command completion candidates
//
// Repl-only responsibilities (never extracted):
//   - JavaScript source accumulation
//   - JavaScript completeness checks
//   - evaluation via goja runtime
//   - expression result rendering
//   - slash command semantics for runtime inspection
//
// Shared foundation candidates (targeted for extraction in S2–S4):
//   - multiline editor setup
//   - tty and writer wiring
//   - history configuration
//   - prompt wiring
//   - submit policy hooks
//   - startup banner and help hooks
//   - generic render and error hooks
//   - runtime profile metadata
//
// The types below are draft contracts for S2–S4. They document the allowed
// extraction surface without changing the current public behavior of Shell or Repl.

// SessionConfig describes the shared editor/session configuration surface that
// S2 will extract from both Shell.Run and Repl.Loop.
//
// Prompt and SubmitOnEnterWhen live in Hooks rather than as direct fields so
// that callers can pass a zero-value SessionConfig with only the fields they
// care about, without forcing every constructor to provide every callback.
type SessionConfig struct {
	Writer               io.Writer
	EnableHistoryCycling bool
	History              HistoryConfig
	Profile              RuntimeProfile
	Renderer             Renderer
	Hooks                SessionHooks
}

// HistoryConfig describes history storage policy that can be shared by Shell
// and Repl while keeping different logical history names.
type HistoryConfig struct {
	Name    string
	Size    int
	Enabled bool
}

// RuntimeProfile describes prompt/banner/runtime identity metadata for a
// session without mixing Shell semantics with Repl semantics.
//
// TODO(S3): Banner will become func() string to support dynamic content.
// It is kept as string here to stay minimal during S1.
// Startup func(rt *goja.Runtime) error will be added in S3.
// Metadata map[string]any will be added in S3.
type RuntimeProfile struct {
	Name        string
	Description string
	Banner      string
}

// Renderer is the shared render hook candidate.
//
// RenderValue intentionally uses 'any' rather than 'goja.Value' to keep the
// foundation layer free of a direct goja import. Foundation code does not
// evaluate JavaScript; only Shell and Repl leaf code does. Callers that hold
// a goja.Value pass it as any, and the concrete renderer implementation can
// type-assert back to goja.Value where needed.
//
// Concrete rendering behavior remains product-specific: Shell renders
// command-oriented output; Repl renders evaluation-oriented output.
type Renderer interface {
	RenderBanner(w io.Writer, profile RuntimeProfile) error
	RenderValue(w io.Writer, value any) error
	RenderError(w io.Writer, err error) error
}

// SessionHooks captures editor/session lifecycle hooks that can be shared
// without merging Shell execution semantics with Repl evaluation semantics.
type SessionHooks struct {
	Prompt            func(w io.Writer, lineNo int) (int, error)
	SubmitOnEnterWhen func(lines []string, lineNo int) bool
	ConfigureEditor   func(ed *multiline.Editor)
	OnStart           func() error
	OnStop            func(err error) error
}
