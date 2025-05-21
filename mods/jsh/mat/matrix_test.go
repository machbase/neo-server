package mat_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/machbase/neo-server/v8/mods/jsh"
)

type TestCase struct {
	Name   string
	Script string
	Expect []string
}

func runTestCase(t *testing.T, tc TestCase) {
	t.Helper()
	ctx := context.TODO()
	w := &bytes.Buffer{}
	j := jsh.NewJsh(ctx,
		jsh.WithNativeModules("@jsh/process", "@jsh/mat", "@jsh/generator"),
		jsh.WithWriter(w),
	)
	err := j.Run(tc.Name, tc.Script, nil)
	if err != nil {
		t.Fatalf("Error running script: %s", err)
	}
	lines := bytes.Split(w.Bytes(), []byte{'\n'})
	for i, line := range lines {
		if i >= len(tc.Expect) {
			break
		}
		if !bytes.Equal(line, []byte(tc.Expect[i])) {
			t.Errorf("Expected %q, got %q", tc.Expect[i], line)
		}
	}
	if len(lines) > len(tc.Expect) {
		t.Errorf("Expected %d lines, got %d", len(tc.Expect), len(lines))
	}
}

func TestFormat(t *testing.T) {
	tests := []TestCase{
		{
			Name: "format",
			Script: `const m = require("@jsh/mat")
				A = new m.Dense(100, 100)
				for (let i = 0; i < 100; i++) {
					for (let j = 0; j < 100; j++) {
						A.set(i, j, i + j)
					}
				}
				console.log(m.format(A, {
					format: "A = %v",
					prefix: "    ",
					squeeze: true,
					excerpt: 3,
				}))
			`,
			Expect: []string{
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
				"",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}
