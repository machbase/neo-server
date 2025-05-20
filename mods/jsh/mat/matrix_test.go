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
