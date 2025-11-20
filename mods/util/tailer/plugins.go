package tailer

import (
	"fmt"
	"regexp"
	"strings"
)

// Remove any ANSI color codes from label, with regexp
var stripAnsiCodesRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func StripAnsiCodes(s string) string {
	return stripAnsiCodesRegexp.ReplaceAllString(s, "")
}

func Colorize(s string, color string) string {
	return fmt.Sprintf("%s%s%s", color, s, ColorReset)
}

type Plugin interface {
	// Apply processes a line and returns the modified line
	// and a boolean indicating processing ahead
	// if further plugins should continue processing
	// or drop the line.
	Apply(line string) (string, bool)
}

// ANSI color codes
const (
	ColorReset         = "\033[0m"
	ColorBlack         = "\033[30m"       // Black
	ColorRed           = "\033[31m"       // Red
	ColorGreen         = "\033[32m"       // Green
	ColorYellow        = "\033[33m"       // Yellow
	ColorBlue          = "\033[34m"       // Blue
	ColorMagenta       = "\033[35m"       // Magenta
	ColorCyan          = "\033[36m"       // Cyan for keys
	ColorLightGray     = "\033[37m"       // Light gray
	ColorNavy          = "\033[38;5;17m"  // Navy
	ColorTeal          = "\033[38;5;51m"  // Teal
	ColorMaroon        = "\033[38;5;52m"  // Maroon
	ColorIndigo        = "\033[38;5;57m"  // Indigo
	ColorLightBlue     = "\033[38;5;81m"  // Light Blue
	ColorBrown         = "\033[38;5;94m"  // Brown
	ColorOlive         = "\033[38;5;100m" // Olive
	ColorLightGreen    = "\033[38;5;120m" // Light Green
	ColorPurple        = "\033[38;5;135m" // Purple
	ColorLime          = "\033[38;5;154m" // Lime
	ColorPink          = "\033[38;5;205m" // Pink
	ColorOrange        = "\033[38;5;208m" // Orange
	ColorGray          = "\033[38;5;245m" // Gray
	ColorDarkGray      = "\033[90m"       // Dark gray
	ColorBrightRed     = "\033[91m"       // Bright Red
	ColorBrightGreen   = "\033[92m"       // Bright Green
	ColorBrightYellow  = "\033[93m"       // Bright Yellow
	ColorBrightBlue    = "\033[94m"       // Bright Blue
	ColorBrightMagenta = "\033[95m"       // Bright Magenta
	ColorBrightCyan    = "\033[96m"       // Bright Cyan
	ColorWhite         = "\033[97m"       // White
)

func NewWithSyntaxHighlighting(syntax ...string) Plugin {
	return syntaxColoring(syntax)
}

type syntaxColoring []string

var slogKeyValuePattern = regexp.MustCompile(`(\w+)=("(?:[^"\\]|\\.)*"|[^\s]+)`)

func (c syntaxColoring) Apply(line string) (string, bool) {
	// Default Keywords
	for _, syntax := range c {
		switch strings.ToLower(syntax) {
		case "level", "levels":
			line = strings.ReplaceAll(line, "TRACE", ColorDarkGray+"TRACE"+ColorReset)
			line = strings.ReplaceAll(line, "DEBUG", ColorLightGray+"DEBUG"+ColorReset)
			line = strings.ReplaceAll(line, "INFO", ColorGreen+"INFO"+ColorReset)
			line = strings.ReplaceAll(line, "WARN", ColorYellow+"WARN"+ColorReset)
			line = strings.ReplaceAll(line, "ERROR", ColorRed+"ERROR"+ColorReset)
		case "slog-text":
			// Color name=value patterns in slog format
			line = slogKeyValuePattern.ReplaceAllStringFunc(line, func(match string) string {
				parts := strings.SplitN(match, "=", 2)
				if len(parts) == 2 {
					key := parts[0]
					value := parts[1]
					return ColorCyan + key + ColorReset + "=" + ColorBlue + value + ColorReset
				}
				return match
			})
		case "slog-json":
			// Color JSON key:value patterns in slog format
			line = regexp.MustCompile(`"(\w+)":\s*("(?:[^"\\]|\\.)*"|[^\s,}]+)`).ReplaceAllStringFunc(line, func(match string) string {
				parts := strings.SplitN(match, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					return ColorCyan + key + ColorReset + ":" + ColorBlue + value + ColorReset
				}
				return match
			})
		case "syslog":
			// /var/log/syslog specific coloring
			// Pattern: timestamp hostname process[pid]: message
			syslogPattern := regexp.MustCompile(`^(\S+)\s+(\S+)\s+([^\s:]+(?:\[\d+\])?):(.*)$`)
			line = syslogPattern.ReplaceAllStringFunc(line, func(match string) string {
				matches := syslogPattern.FindStringSubmatch(match)
				if len(matches) == 5 {
					timestamp := ColorBlue + matches[1] + ColorReset
					hostname := ColorCyan + matches[2] + ColorReset
					process := ColorYellow + matches[3] + ColorReset
					message := matches[4]
					return timestamp + " " + hostname + " " + process + ":" + message
				}
				return match
			})
			// syslog file encodes ESC as #033[
			line = strings.ReplaceAll(line, "#033[", "\033[")
		}
	}
	return line, true
}
