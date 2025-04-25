package jsh

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/util/ssfs"
	"github.com/stretchr/testify/require"
)

func TestJshInterrupt(t *testing.T) {
	serverFs, _ := ssfs.NewServerSideFileSystem([]string{"/=./test"})
	ssfs.SetDefault(serverFs)

	ctx := context.TODO()
	w := &bytes.Buffer{}
	jsh := NewJsh(
		ctx,
		WithJshWriter(w),
		WithNativeModules("@jsh/process", "@jsh/opcua"),
		WithJshWorkingDir("/"),
	)

	go func() {
		time.Sleep(1 * time.Second)
		jsh.Interrupt()
	}()

	err := jsh.Exec([]string{"jsh-interrupt.js"})
	if !strings.HasPrefix(err.Error(), "interrupted at") {
		t.Fatalf("Expected interrupted error, got: %v", err)
	}
}

func TestJsh(t *testing.T) {
	serverFs, _ := ssfs.NewServerSideFileSystem([]string{"/=./test"})
	ssfs.SetDefault(serverFs)

	tests := []struct {
		name   string
		args   []string
		expect []string
	}{
		{
			name: "jsh-exception",
			args: []string{"jsh-exception.js"},
			expect: []string{
				"Error: TypeError: Object has no member 'undefinedFunction'",
				"Error: Object has no member 'undefinedFunction'",
				"",
			},
		},
		{
			name:   "jsh-hello-world",
			args:   []string{"jsh-hello-world.js"},
			expect: []string{"Hello, World!\n"},
		},
		{
			name: "jsh-cleanup",
			args: []string{"jsh-cleanup.js"},
			expect: []string{
				"Running cleanup code3...",
				"Running cleanup code1...",
				"",
			},
		},
	}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			ctx := context.TODO()
			w := &bytes.Buffer{}
			jsh := NewJsh(
				ctx,
				WithNativeModules("@jsh/process", "@jsh/opcua"),
				WithJshWriter(w),
				WithJshWorkingDir("/"))

			err := jsh.Exec(ts.args)
			require.NoError(t, err)
			require.Equal(t, strings.Join(ts.expect, "\n"), w.String())
		})
	}
}
