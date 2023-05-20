package util

import (
	"fmt"
	"strings"
)

func HelpShortcuts() string {
	normalMode := [][2]string{
		{"Ctrl + A", "Beginning of line"},
		{"Ctrl + B / ←", "Backward one character"},
		{"Meta + B", "Backward one word"},
		{"Ctrl + C", "Send io.EOF"},
		{"Ctrl + D", "Delete one character"},
		{"Meta + D", "Delete one word"},
		{"Ctrl + E", "End of line"},
		{"Ctrl + F / →", "Forward one character"},
		{"Meta + F", "Forward one word"},
		{"Ctrl + G", "Cancel"},
		{"Ctrl + H", "Delete previous character"},
		{"Ctrl + I / Tab", "Command line completion"},
		{"Ctrl + J", "Line feed"},
		{"Ctrl + K", "Cut text to the end of line"},
		{"Ctrl + L", "Clear screen"},
		{"Ctrl + M", "Same as Enter key"},
		{"Ctrl + N / ↓", "Next line (in history)"},
		{"Ctrl + P / ↑", "Prev line (in history)"},
		{"Ctrl + R", "Search backwards in history"},
		{"Ctrl + S", "Search forwards in history"},
		{"Ctrl + T", "Transpose characters"},
		{"Ctrl + U", "Cut text to the beginning of line"},
		{"Ctrl + W", "Cut previous word"},
		{"Backspace", "Delete previous character"},
		{"Meta+Backspace", "Cut previous word"},
	}
	keys := []string{}
	for _, v := range normalMode {
		keys = append(keys, fmt.Sprintf("      %-16s %s", v[0], v[1]))
	}
	return `    "Meta + B" means press 'ESC' and 'B' separately.
    Users can change that in terminal simulator (i.e. iTerm2) to 'Alt'+'B'
    Notice: "Meta + B" is equals with 'Alt'+'B' in Windows.

` + strings.Join(keys, "\n")
}
