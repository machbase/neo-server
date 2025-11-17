package tailer

import "strings"

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
)

// Solarized Dark theme colors
const (
	solarizedBase01 = "\033[38;5;240m" // base01 - emphasized content
	solarizedBase0  = "\033[38;5;244m" // base0 - body text
	solarizedCyan   = "\033[38;5;37m"  // cyan - emphasis
	solarizedYellow = "\033[38;5;136m" // yellow - warning
	solarizedRed    = "\033[38;5;160m" // red - error
)

// Molokai theme colors
const (
	molokaiGray   = "\033[38;5;244m" // gray - trace
	molokaiPurple = "\033[38;5;141m" // purple - debug
	molokaiGreen  = "\033[38;5;148m" // green - info
	molokaiOrange = "\033[38;5;208m" // orange - warning
	molokaiRed    = "\033[38;5;197m" // red - error
)

// Ubuntu theme colors
const (
	ubuntuGray   = "\033[38;5;246m" // gray - trace
	ubuntuWhite  = "\033[38;5;231m" // white - debug
	ubuntuGreen  = "\033[38;5;34m"  // green - info
	ubuntuYellow = "\033[38;5;220m" // yellow - warning
	ubuntuRed    = "\033[38;5;196m" // red - error
)

func NewColoring(style string) Plugin {
	return coloring(style)
}

type coloring string

func (c coloring) Apply(line string) (string, bool) {
	switch string(c) {
	case "solarized", "solarized-dark":
		// Solarized Dark theme
		line = strings.ReplaceAll(line, "TRACE", solarizedBase01+"TRACE"+colorReset)
		line = strings.ReplaceAll(line, "DEBUG", solarizedBase0+"DEBUG"+colorReset)
		line = strings.ReplaceAll(line, "INFO", solarizedCyan+"INFO"+colorReset)
		line = strings.ReplaceAll(line, "WARN", solarizedYellow+"WARN"+colorReset)
		line = strings.ReplaceAll(line, "ERROR", solarizedRed+"ERROR"+colorReset)
	case "molokai":
		// Molokai theme
		line = strings.ReplaceAll(line, "TRACE", molokaiGray+"TRACE"+colorReset)
		line = strings.ReplaceAll(line, "DEBUG", molokaiPurple+"DEBUG"+colorReset)
		line = strings.ReplaceAll(line, "INFO", molokaiGreen+"INFO"+colorReset)
		line = strings.ReplaceAll(line, "WARN", molokaiOrange+"WARN"+colorReset)
		line = strings.ReplaceAll(line, "ERROR", molokaiRed+"ERROR"+colorReset)
	case "ubuntu":
		// Ubuntu theme
		line = strings.ReplaceAll(line, "TRACE", ubuntuGray+"TRACE"+colorReset)
		line = strings.ReplaceAll(line, "DEBUG", ubuntuWhite+"DEBUG"+colorReset)
		line = strings.ReplaceAll(line, "INFO", ubuntuGreen+"INFO"+colorReset)
		line = strings.ReplaceAll(line, "WARN", ubuntuYellow+"WARN"+colorReset)
		line = strings.ReplaceAll(line, "ERROR", ubuntuRed+"ERROR"+colorReset)
	default:
		// Default theme
		line = strings.ReplaceAll(line, "TRACE", colorDarkGray+"TRACE"+colorReset)
		line = strings.ReplaceAll(line, "DEBUG", colorLightGray+"DEBUG"+colorReset)
		line = strings.ReplaceAll(line, "INFO", colorGreen+"INFO"+colorReset)
		line = strings.ReplaceAll(line, "WARN", colorYellow+"WARN"+colorReset)
		line = strings.ReplaceAll(line, "ERROR", colorRed+"ERROR"+colorReset)
	}
	return line, true
}
