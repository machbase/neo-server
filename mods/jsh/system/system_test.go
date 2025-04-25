package system_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/machbase/neo-server/v8/mods/jsh"
)

func TestStatz(t *testing.T) {
	tests := []struct {
		Name      string
		Script    string
		Expect    []string
		ExpectLog []string
	}{
		{
			Name: "statz",
			Script: `
				const {print} = require("@jsh/process");
				const {statz} = require("@jsh/system");
				try {
					print(statz("1m", "go:goroutine_max").toString());
				} catch (e) {
				 	print(e.toString());
				}
			`,
			Expect: []string{
				"no metrics found",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.TODO()
			w := &bytes.Buffer{}
			j := jsh.NewJsh(ctx,
				jsh.WithNativeModules("@jsh/process", "@jsh/system"),
				jsh.WithJshWriter(w),
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
		})
	}
}
