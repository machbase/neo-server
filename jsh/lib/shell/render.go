package shell

import (
	"fmt"
	"io"
)

// OutputMode controls the color preference for rendered output.
// It allows callers to select plain text mode (for piped output or agent
// consumption) or forced color mode independently of TTY detection.
type OutputMode int

const (
	// OutputModeAuto uses color when the output destination is a TTY.
	OutputModeAuto OutputMode = iota
	// OutputModePlain disables ANSI color sequences.
	OutputModePlain
	// OutputModeColor forces ANSI color even when not writing to a TTY.
	OutputModeColor
)

// WriterRenderer is the foundation default Renderer. It writes output to the
// provided io.Writer using fmt formatting without ANSI color.
//
// Repl uses its own console-backed renderer (consoleRenderer in repl.go) which
// delegates to the JSH console module for value formatting. Shell uses
// WriterRenderer for generic banner/error output. Both products may replace it
// with a product-specific implementation via SessionConfig.Renderer.
type WriterRenderer struct {
	Mode OutputMode
}

// RenderBanner writes the profile banner to w. It is a no-op when the profile
// banner function is nil or returns an empty string.
func (r WriterRenderer) RenderBanner(w io.Writer, profile RuntimeProfile) error {
	msg := profile.ResolveBanner()
	if msg == "" {
		return nil
	}
	_, err := fmt.Fprint(w, msg)
	return err
}

// RenderValue writes a formatted value to w. Nil values produce no output.
func (r WriterRenderer) RenderValue(w io.Writer, value any) error {
	if value == nil {
		return nil
	}
	_, err := fmt.Fprintf(w, "%v\n", value)
	return err
}

// RenderError writes a formatted error message to w. Nil errors produce no output.
func (r WriterRenderer) RenderError(w io.Writer, err error) error {
	if err == nil {
		return nil
	}
	_, writeErr := fmt.Fprintf(w, "Error: %v\n", err)
	return writeErr
}
