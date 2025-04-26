package generator_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/mods/jsh"
	"github.com/stretchr/testify/require"
)

type TestCase struct {
	Name       string
	Script     string
	Expect     []string
	ExpectFunc func(t *testing.T, result string)
}

func runTestCase(t *testing.T, tc TestCase) {
	t.Helper()
	ctx := context.TODO()
	w := &bytes.Buffer{}
	j := jsh.NewJsh(ctx,
		jsh.WithNativeModules("@jsh/process", "@jsh/generator"),
		jsh.WithWriter(w),
	)
	err := j.Run(tc.Name, tc.Script, nil)
	if err != nil {
		t.Fatalf("Error running script: %s", err)
	}

	if tc.ExpectFunc != nil {
		tc.ExpectFunc(t, w.String())
		return
	} else {
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
}

func TestSimplex(t *testing.T) {
	tests := []TestCase{
		{
			Name: "js-simplex",
			Script: `
				const {Simplex} = require("@jsh/generator")
				simplex = new Simplex(123);
				for(i=0; i < 5; i++) {
					console.log(i, simplex.eval(i, i * 0.6).toFixed(3));
				}
			`,
			Expect: []string{
				"0 0.000",
				"1 0.349",
				"2 0.319",
				"3 0.038",
				"4 -0.364",
				""},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestUUID(t *testing.T) {
	tests := []TestCase{
		{
			Name: "js-uuid",
			Script: `
				const {UUID} = require("@jsh/generator")
				gen = new UUID(1);
				for(i=0; i < 5; i++) {
					console.log(gen.eval());
				}
			`,
			ExpectFunc: func(t *testing.T, result string) {
				rows := strings.Split(strings.TrimSpace(result), "\n")
				require.Equal(t, 5, len(rows), result)
				for _, l := range rows {
					require.Equal(t, 36, len(l))
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestMeshgrid(t *testing.T) {
	tests := []TestCase{
		{
			Name: "js-meshgrid",
			Script: `
				gen = require("@jsh/generator").meshgrid([1, 2, 3], [4, 5]);
				for(i=0; i < gen.length; i++) {
					console.log(JSON.stringify(gen[i]));
				}
			`,
			Expect: []string{
				"[1,4]",
				"[1,5]",
				"[2,4]",
				"[2,5]",
				"[3,4]",
				"[3,5]",
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
