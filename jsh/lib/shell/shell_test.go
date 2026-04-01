package shell

import (
	"testing"

	"github.com/nyaosorg/go-readline-ny"
)

type stubHistory []string

func (h stubHistory) Len() int {
	return len(h)
}

func (h stubHistory) At(i int) string {
	return h[i]
}

func TestPredictShellHistory(t *testing.T) {
	tests := []struct {
		name    string
		current string
		history readline.IHistory
		want    string
	}{
		{
			name:    "single line history",
			current: "sele",
			history: stubHistory{"help", "select * from example"},
			want:    "select * from example",
		},
		{
			name:    "prefer latest match",
			current: "sel",
			history: stubHistory{"select * from old", "select * from latest"},
			want:    "select * from latest",
		},
		{
			name:    "strip continuation marker from prediction",
			current: "echo hel",
			history: stubHistory{"echo hello \\\nworld", "noop"},
			want:    "echo hello ",
		},
		{
			name:    "do not predict on continuation line",
			current: "echo hello \\",
			history: stubHistory{"echo hello \\\nworld"},
			want:    "",
		},
		{
			name:    "ignore whitespace only current",
			current: "   ",
			history: stubHistory{"select * from example"},
			want:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := predictShellHistory(tc.current, tc.history); got != tc.want {
				t.Fatalf("predictShellHistory(%q) = %q, want %q", tc.current, got, tc.want)
			}
		})
	}
}

func TestShouldAcceptPrediction(t *testing.T) {
	tests := []struct {
		name       string
		cursor     int
		bufferLen  int
		cursorLine int
		lineCount  int
		want       bool
	}{
		{
			name:       "not at end of line",
			cursor:     2,
			bufferLen:  5,
			cursorLine: 0,
			lineCount:  1,
			want:       false,
		},
		{
			name:       "accept at end of last line",
			cursor:     5,
			bufferLen:  5,
			cursorLine: 0,
			lineCount:  1,
			want:       true,
		},
		{
			name:       "do not accept in middle line",
			cursor:     4,
			bufferLen:  4,
			cursorLine: 0,
			lineCount:  2,
			want:       false,
		},
		{
			name:       "empty line state treated as last line",
			cursor:     0,
			bufferLen:  0,
			cursorLine: 0,
			lineCount:  0,
			want:       true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldAcceptPrediction(tc.cursor, tc.bufferLen, tc.cursorLine, tc.lineCount)
			if got != tc.want {
				t.Fatalf("shouldAcceptPrediction(%d, %d, %d, %d) = %v, want %v", tc.cursor, tc.bufferLen, tc.cursorLine, tc.lineCount, got, tc.want)
			}
		})
	}
}
