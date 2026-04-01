package readline

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeHistoryFilename(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "default", input: "readline", want: "readline"},
		{name: "slashes", input: "dir/sub", want: "dir_sub"},
		{name: "backslashes", input: `dir\sub`, want: "dir_sub"},
		{name: "parent segments", input: "../history", want: "_history"},
		{name: "mixed traversal", input: `..\..//etc/passwd`, want: "___etc_passwd"},
		{name: "only dots", input: "....", want: "readline"},
		{name: "empty", input: "", want: "readline"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeHistoryFilename(tc.input); got != tc.want {
				t.Fatalf("sanitizeHistoryFilename(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNewHistoryStaysUnderPrefDir(t *testing.T) {
	h := NewHistory(`..\..//etc/passwd`, 10)
	prefDir := PrefDir()
	rel, err := filepath.Rel(prefDir, h.filepath)
	if err != nil {
		t.Fatalf("filepath.Rel() error: %v", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("history path escaped pref dir: pref=%q path=%q rel=%q", prefDir, h.filepath, rel)
	}
	if got, want := filepath.Base(h.filepath), "___etc_passwd"; got != want {
		t.Fatalf("filepath.Base() = %q, want %q", got, want)
	}
}
