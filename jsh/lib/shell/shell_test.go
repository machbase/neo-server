package shell

import (
	"bytes"
	"context"
	"testing"

	"github.com/dop251/goja"
	"github.com/hymkor/go-multiline-ny"
	"github.com/machbase/neo-server/v8/jsh/engine"
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

func TestModuleAndConstructor(t *testing.T) {
	rt := goja.New()
	module := rt.NewObject()
	exports := rt.NewObject()
	if err := module.Set("exports", exports); err != nil {
		t.Fatalf("set exports: %v", err)
	}

	Module(context.Background(), rt, module)
	if goja.IsUndefined(exports.Get("Shell")) {
		t.Fatal("exports.Shell is undefined")
	}
	if goja.IsUndefined(exports.Get("Repl")) {
		t.Fatal("exports.Repl is undefined")
	}

	obj := shell(rt)(goja.ConstructorCall{})
	if goja.IsUndefined(obj.Get("run")) {
		t.Fatal("constructed shell.run is undefined")
	}
}

func TestPromptAndSubmitBehavior(t *testing.T) {
	env := engine.NewEnv()
	env.Set("PWD", "/work/demo")
	sh := &Shell{}
	prompt := sh.prompt(env)

	var first bytes.Buffer
	if _, err := prompt(&first, 0); err != nil {
		t.Fatalf("prompt first line: %v", err)
	}
	if got, want := first.String(), "\x1b[34m/work/demo\x1B[31m >\x1B[0m "; got != want {
		t.Fatalf("prompt first line = %q, want %q", got, want)
	}

	var next bytes.Buffer
	if _, err := prompt(&next, 1); err != nil {
		t.Fatalf("prompt next line: %v", err)
	}
	if got, want := next.String(), "             "; got != want {
		t.Fatalf("prompt next line = %q, want %q", got, want)
	}

	if got := sh.submitOnEnterWhen([]string{"echo hello"}, 0); !got {
		t.Fatal("submitOnEnterWhen(single line) = false, want true")
	}
	if got := sh.submitOnEnterWhen([]string{"echo \\"}, 0); got {
		t.Fatal("submitOnEnterWhen(continuation) = true, want false")
	}
}

func TestPredictHistoryAndCompletionCandidates(t *testing.T) {
	sh := &Shell{}
	buf := &readline.Buffer{
		Editor: &readline.Editor{
			History: stubHistory{"select * from dual"},
			Cursor:  4,
		},
	}
	buf.InsertString(0, "sele")
	if got := sh.predictHistory(buf); got != "select * from dual" {
		t.Fatalf("predictHistory = %q, want %q", got, "select * from dual")
	}

	ed := &multiline.Editor{}
	sh.bindPredictionKeys(ed)

	forCompletion, forListing := sh.getCompletionCandidates([]string{"echo"})
	if forCompletion != nil || forListing != nil {
		t.Fatalf("getCompletionCandidates() = (%v, %v), want (nil, nil)", forCompletion, forListing)
	}
}

func TestExecReturnsExceptionValue(t *testing.T) {
	sh := &Shell{rt: goja.New()}
	value := sh.exec("echo", []string{"hello"})
	if value == nil {
		t.Fatal("exec returned nil, want exception value")
	}
	if got := value.String(); got == "undefined" || got == "null" {
		t.Fatalf("exec returned %q, want error detail", got)
	}
}
