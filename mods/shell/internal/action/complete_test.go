package action_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/mods/shell/internal/action"
	_ "github.com/machbase/neo-server/v8/mods/shell/internal/cmd"
)

func TestPrefixComplete(t *testing.T) {
	pc := action.BuildPrefixCompleter()
	tests := []struct {
		line         string
		expectLength int
		expectLines  []string
	}{
		{
			line:         "",
			expectLength: 0,
			expectLines: []string{
				"bridge", "connect", "desc", "explain", "export", "fake", "help", "http",
				"import", "key", "ping", "run", "session", "set", "shell", "show", "shutdown",
				"sql", "ssh-key", "subscriber", "timer",
			},
		},
		{
			line:         "x",
			expectLength: 0,
			expectLines:  []string{},
		},
		{
			line:         "s",
			expectLength: 1,
			expectLines:  []string{"ession", "et", "hell", "how", "hutdown", "ql", "sh-key", "ubscriber"},
		},
		{
			line:         "sh",
			expectLength: 2,
			expectLines:  []string{"ell", "ow", "utdown"},
		},
		{
			line:         "show in",
			expectLength: 2,
			expectLines:  []string{"fo", "dexes", "dex", "dexgap"},
		},
		{
			line:         "show inf",
			expectLength: 3,
			expectLines:  []string{"o"},
		},
	}

	for _, tt := range tests {
		newLine, length := pc.Do([]rune(tt.line), len(tt.line))
		if length != tt.expectLength {
			t.Logf("%q returns wrong length expecting %d, got %d", tt.line, tt.expectLength, length)
			t.Fail()
			continue
		}
		if len(newLine) != len(tt.expectLines) {
			str := []string{}
			for _, line := range newLine {
				str = append(str, string(line))
			}
			t.Logf("%q returns wrong expecting %v, got %v", tt.line, tt.expectLines, str)
			t.Fail()
			continue
		}
		for i, line := range newLine {
			if string(line) != tt.expectLines[i]+" " {
				t.Logf("%q returns wrong expecting lines[%d] %q, got %q", tt.line, i, tt.expectLines[i], string(line))
				t.Fail()
				continue
			}
		}
	}
}
