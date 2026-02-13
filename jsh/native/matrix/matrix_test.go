package matrix

import (
	"bytes"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/root"
)

type TestCase struct {
	name       string
	script     string
	input      []string
	output     []string
	expectFunc func(t *testing.T, result string)
	err        string
	vars       map[string]any
}

func RunTest(t *testing.T, tc TestCase) {
	t.Helper()
	t.Run(tc.name, func(t *testing.T) {
		t.Helper()
		conf := engine.Config{
			Name:   tc.name,
			Code:   tc.script,
			FSTabs: []engine.FSTab{root.RootFSTab()},
			Env:    tc.vars,
			Reader: &bytes.Buffer{},
			Writer: &bytes.Buffer{},
		}
		jr, err := engine.New(conf)
		if err != nil {
			t.Fatalf("Failed to create JSRuntime: %v", err)
		}
		jr.RegisterNativeModule("@jsh/matrix", Module)

		if len(tc.input) > 0 {
			conf.Reader.(*bytes.Buffer).WriteString(strings.Join(tc.input, ""))
		}
		if err := jr.Run(); err != nil {
			if tc.err == "" || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("Unexpected error: %v", err)
			}
			return
		}

		gotOutput := conf.Writer.(*bytes.Buffer).String()
		if tc.expectFunc != nil {
			tc.expectFunc(t, gotOutput)
			return
		}
		lines := strings.Split(gotOutput, "\n")
		if len(lines) != len(tc.output)+1 { // +1 for trailing newline
			t.Fatalf("Expected %d output lines, got %d\n%s", len(tc.output), len(lines)-1, gotOutput)
		}
		for i, expectedLine := range tc.output {
			if lines[i] != expectedLine {
				t.Errorf("Output line %d: expected %q, got %q", i, expectedLine, lines[i])
			}
		}
	})
}

func TestFormat(t *testing.T) {
	tests := []TestCase{
		{
			name: "format",
			script: `const m = require("@jsh/matrix")
				A = new m.Dense(100, 100)
				for (let i = 0; i < 100; i++) {
					for (let j = 0; j < 100; j++) {
						A.set(i, j, i + j)
					}
				}
				console.println(m.format(A, {
					format: "A = %v",
					prefix: "    ",
					squeeze: true,
					excerpt: 3,
				}))
			`,
			output: []string{
				"A = Dims(100, 100)",
				"    ⎡ 0    1    2  ...  ...   97   98   99⎤",
				"    ⎢ 1    2    3             98   99  100⎥",
				"    ⎢ 2    3    4             99  100  101⎥",
				"     .",
				"     .",
				"     .",
				"    ⎢97   98   99            194  195  196⎥",
				"    ⎢98   99  100            195  196  197⎥",
				"    ⎣99  100  101  ...  ...  196  197  198⎦",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			RunTest(t, tc)
		})
	}
}
