package generator

import (
	"bytes"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/root"
	"github.com/stretchr/testify/require"
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
		jr.RegisterNativeModule("@jsh/generator", Module)

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

func TestSimplex(t *testing.T) {
	tests := []TestCase{
		{
			name: "js-simplex",
			script: `
				const {Simplex} = require("generator")
				simplex = new Simplex(123);
				for(i=0; i < 5; i++) {
					console.println(i, simplex.eval(i, i * 0.6).toFixed(3));
				}
			`,
			output: []string{
				"0 0.000",
				"1 0.349",
				"2 0.319",
				"3 0.038",
				"4 -0.364",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			RunTest(t, tc)
		})
	}
}

func TestUUID(t *testing.T) {
	tests := []TestCase{
		{
			name: "js-uuid",
			script: `
				const {UUID} = require("generator")
				gen = new UUID(1);
				for(i=0; i < 5; i++) {
					console.println(gen.eval());
				}
			`,
			expectFunc: func(t *testing.T, result string) {
				rows := strings.Split(strings.TrimSpace(result), "\n")
				require.Equal(t, 5, len(rows), result)
				for _, l := range rows {
					require.Equal(t, 36, len(l))
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			RunTest(t, tc)
		})
	}
}

func TestMeshgrid(t *testing.T) {
	tests := []TestCase{
		{
			name: "js-meshgrid",
			script: `
				gen = require("generator").meshgrid([1, 2, 3], [4, 5]);
				for(i=0; i < gen.length; i++) {
					console.println(JSON.stringify(gen[i]));
				}
			`,
			output: []string{
				"[1,4]",
				"[1,5]",
				"[2,4]",
				"[2,5]",
				"[3,4]",
				"[3,5]",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			RunTest(t, tc)
		})
	}
}
