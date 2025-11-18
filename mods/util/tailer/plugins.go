package tailer

import (
	"regexp"
	"strings"
)

type Plugin interface {
	// Apply processes a line and returns the modified line
	// and a boolean indicating processing ahead
	// if further plugins should continue processing
	// or drop the line.
	Apply(line string) (string, bool)
}

// ANSI color codes for xterm.js
const (
	colorReset     = "\033[0m"
	colorLightGray = "\033[37m" // Light gray
	colorDarkGray  = "\033[90m" // Dark gray
	colorGreen     = "\033[32m" // Green
	colorYellow    = "\033[33m" // Yellow
	colorRed       = "\033[31m" // Red
	colorCyan      = "\033[36m" // Cyan for keys
	colorBlue      = "\033[34m" // Blue for values
)

func NewSyntaxColoring(syntax ...string) Plugin {
	return syntaxColoring(syntax)
}

type syntaxColoring []string

var slogKeyValuePattern = regexp.MustCompile(`(\w+)=("(?:[^"\\]|\\.)*"|[^\s]+)`)

func (c syntaxColoring) Apply(line string) (string, bool) {
	// Default Keywords
	for _, syntax := range c {
		switch strings.ToLower(syntax) {
		case "loglevel", "loglevels", "level", "levels":
			line = strings.ReplaceAll(line, "TRACE", colorDarkGray+"TRACE"+colorReset)
			line = strings.ReplaceAll(line, "DEBUG", colorLightGray+"DEBUG"+colorReset)
			line = strings.ReplaceAll(line, "INFO", colorGreen+"INFO"+colorReset)
			line = strings.ReplaceAll(line, "WARN", colorYellow+"WARN"+colorReset)
			line = strings.ReplaceAll(line, "ERROR", colorRed+"ERROR"+colorReset)
		case "slog":
			// Color name=value patterns in slog format
			line = slogKeyValuePattern.ReplaceAllStringFunc(line, func(match string) string {
				parts := strings.SplitN(match, "=", 2)
				if len(parts) == 2 {
					key := parts[0]
					value := parts[1]
					return colorCyan + key + colorReset + "=" + colorBlue + value + colorReset
				}
				return match
			})
		}
	}
	return line, true
}
