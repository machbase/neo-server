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
		WithWriter(w),
		WithNativeModules("@jsh/process", "@jsh/opcua"),
		WithWorkingDir("/"),
	)

	go func() {
		time.Sleep(1 * time.Second)
		jsh.Kill("interrupted")
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
			expect: []string{"Hello, World!", "Current directory: /etc_services/", ""},
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
				WithWriter(w),
				WithWorkingDir("/"))

			err := jsh.Exec(ts.args)
			require.NoError(t, err)
			require.Equal(t, strings.Join(ts.expect, "\n"), w.String())
		})
	}
}

func TestParseCommandLine(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		expect []CommandPart
	}{
		{
			name:   "empty",
			line:   "",
			expect: []CommandPart{},
		},
		{
			name: "args_with_quotes",
			line: `cmd arg1 "arg2 with space" 'arg3 "double quote" within single quote'`,
			expect: []CommandPart{
				{Args: []string{"cmd", "arg1", "arg2 with space", `arg3 "double quote" within single quote`}},
			},
		},
		{
			name: "pipe_commands",
			line: `cmd1 arg1 | cmd2 arg2 | cmd3 arg3`,
			expect: []CommandPart{
				{Args: []string{"cmd1", "arg1"}, Pipe: true},
				{Args: []string{"cmd2", "arg2"}, Pipe: true},
				{Args: []string{"cmd3", "arg3"}},
			},
		},
		{
			name: "pipe_and_redirect",
			line: `echo "hello world" | grep "world" > output.txt`,
			expect: []CommandPart{
				{Args: []string{"echo", "hello world"}, Pipe: true, Redirect: "", Target: ""},
				{Args: []string{"grep", "world"}, Pipe: false, Redirect: ">", Target: "output.txt"},
			},
		},
		{
			name: "pipe_and_redirect_contains_tabs",
			line: `echo "hello		world" | grep "world" >> output.txt`,
			expect: []CommandPart{
				{Args: []string{"echo", "hello		world"}, Pipe: true, Redirect: "", Target: ""},
				{Args: []string{"grep", "world"}, Pipe: false, Redirect: ">>", Target: "output.txt"},
			},
		},
	}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			result := ParseCommandLine(ts.line)
			for i := 0; i < len(result); i++ {
				require.Equal(t, ts.expect[i], result[i])
			}
			if len(ts.expect) > len(result) {
				t.Fatalf("Expected %d parts, got %d", len(ts.expect), len(result))
				for i := len(result); i < len(ts.expect); i++ {
					t.Fatalf("Expected [%d] %v", i, ts.expect[i])
				}
			}
		})
	}
}
