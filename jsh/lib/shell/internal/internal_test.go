package internal

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
)

func TestIsCommand(t *testing.T) {
	for _, tc := range []struct {
		name string
		cmd  string
		want bool
	}{
		{name: "alias", cmd: "alias", want: true},
		{name: "cd", cmd: "cd", want: true},
		{name: "setenv", cmd: "setenv", want: true},
		{name: "unsetenv", cmd: "unsetenv", want: true},
		{name: "which", cmd: "which", want: true},
		{name: "external", cmd: "echo", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsCommand(tc.cmd); got != tc.want {
				t.Fatalf("IsCommand(%q) = %v, want %v", tc.cmd, got, tc.want)
			}
		})
	}
}

func TestRunDispatch(t *testing.T) {
	env := newInternalTestEnv(t)
	var out bytes.Buffer

	exitCode, ok := Run(env, &out, "setenv", "GREETING", "hello")
	if !ok {
		t.Fatal("Run(setenv) ok = false, want true")
	}
	if exitCode != 0 {
		t.Fatalf("Run(setenv) exitCode = %d, want 0", exitCode)
	}
	if got := env.Get("GREETING"); got != "hello" {
		t.Fatalf("GREETING = %v, want hello", got)
	}

	exitCode, ok = Run(env, &out, "alias", "ll", "tool", "--long")
	if !ok {
		t.Fatal("Run(alias) ok = false, want true")
	}
	if exitCode != 0 {
		t.Fatalf("Run(alias) exitCode = %d, want 0", exitCode)
	}
	if got, ok := env.LookupAlias("ll"); !ok || strings.Join(got, " ") != "tool --long" {
		t.Fatalf("LookupAlias(ll) = (%v, %v), want (tool --long, true)", got, ok)
	}

	if exitCode, ok = Run(env, &out, "missing"); ok || exitCode != 0 {
		t.Fatalf("Run(missing) = (%d, %v), want (0, false)", exitCode, ok)
	}
}

func TestRunAlias(t *testing.T) {
	for _, tc := range []struct {
		name       string
		env        *engine.Env
		args       []string
		prepare    func(*engine.Env)
		wantExit   int
		wantOutput string
		check      func(t *testing.T, env *engine.Env)
	}{
		{
			name:       "nil env",
			env:        nil,
			wantExit:   1,
			wantOutput: "alias: shell environment is not initialized",
		},
		{
			name:     "set alias",
			env:      newInternalTestEnv(t),
			args:     []string{"ll", "tool", "--long"},
			wantExit: 0,
			check: func(t *testing.T, env *engine.Env) {
				t.Helper()
				if got, ok := env.LookupAlias("ll"); !ok || strings.Join(got, " ") != "tool --long" {
					t.Fatalf("LookupAlias(ll) = (%v, %v), want (tool --long, true)", got, ok)
				}
			},
		},
		{
			name:     "show one alias",
			env:      newInternalTestEnv(t),
			args:     []string{"ll"},
			wantExit: 0,
			prepare: func(env *engine.Env) {
				env.SetAlias("ll", []string{"tool", "--long"})
			},
			wantOutput: "alias ll tool --long",
		},
		{
			name:     "list aliases sorted",
			env:      newInternalTestEnv(t),
			wantExit: 0,
			prepare: func(env *engine.Env) {
				env.SetAlias("zz", []string{"tool"})
				env.SetAlias("aa", []string{"tool", "hello world"})
			},
			wantOutput: "alias aa tool 'hello world'\nalias zz tool",
		},
		{
			name:       "missing alias",
			env:        newInternalTestEnv(t),
			args:       []string{"missing"},
			wantExit:   1,
			wantOutput: "alias: not found: missing",
		},
		{
			name:       "invalid empty name",
			env:        newInternalTestEnv(t),
			args:       []string{"", "tool"},
			wantExit:   1,
			wantOutput: "alias: invalid name",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.prepare != nil {
				tc.prepare(tc.env)
			}
			var out bytes.Buffer
			exitCode := runAlias(tc.env, &out, tc.args...)
			if exitCode != tc.wantExit {
				t.Fatalf("runAlias(%v) exitCode = %d, want %d", tc.args, exitCode, tc.wantExit)
			}
			if got := strings.TrimSpace(out.String()); got != tc.wantOutput {
				t.Fatalf("runAlias(%v) output = %q, want %q", tc.args, got, tc.wantOutput)
			}
			if tc.check != nil {
				tc.check(t, tc.env)
			}
		})
	}
}

func TestRunCD(t *testing.T) {
	for _, tc := range []struct {
		name       string
		env        *engine.Env
		args       []string
		wantPWD    string
		wantExit   int
		wantOutput string
	}{
		{
			name:       "nil env",
			env:        nil,
			wantExit:   1,
			wantOutput: "cd: shell environment is not initialized",
		},
		{
			name:     "defaults to home",
			env:      newInternalTestEnv(t),
			wantExit: 0,
			wantPWD:  "/work/home",
		},
		{
			name:     "relative path from pwd",
			env:      newInternalTestEnv(t),
			args:     []string{"child"},
			wantExit: 0,
			wantPWD:  "/work/current/child",
		},
		{
			name:       "missing path",
			env:        newInternalTestEnv(t),
			args:       []string{"missing"},
			wantExit:   1,
			wantOutput: "cd: no such file or directory: missing",
		},
		{
			name:       "target is file",
			env:        newInternalTestEnv(t),
			args:       []string{"/work/current/file.txt"},
			wantExit:   1,
			wantOutput: "cd: not a directory: /work/current/file.txt",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if exitCode := runCD(tc.env, &out, tc.args...); exitCode != tc.wantExit {
				t.Fatalf("runCD(%v) exitCode = %d, want %d", tc.args, exitCode, tc.wantExit)
			}
			if tc.wantPWD != "" {
				if got := tc.env.Get("PWD"); got != tc.wantPWD {
					t.Fatalf("PWD = %v, want %s", got, tc.wantPWD)
				}
			}
			if got := strings.TrimSpace(out.String()); got != tc.wantOutput {
				t.Fatalf("runCD(%v) output = %q, want %q", tc.args, got, tc.wantOutput)
			}
		})
	}
}

func TestRunSetenv(t *testing.T) {
	for _, tc := range []struct {
		name       string
		env        *engine.Env
		args       []string
		wantExit   int
		wantValue  any
		wantOutput string
	}{
		{
			name:       "nil env",
			env:        nil,
			args:       []string{"NAME", "value"},
			wantExit:   1,
			wantOutput: "setenv: shell environment is not initialized",
		},
		{
			name:       "missing args",
			env:        newInternalTestEnv(t),
			wantExit:   1,
			wantOutput: "usage: setenv NAME VALUE\n   or: setenv NAME=VALUE",
		},
		{
			name:       "invalid single arg format",
			env:        newInternalTestEnv(t),
			args:       []string{"NAME"},
			wantExit:   1,
			wantOutput: "usage: setenv NAME VALUE\n   or: setenv NAME=VALUE",
		},
		{
			name:       "invalid name",
			env:        newInternalTestEnv(t),
			args:       []string{"1BAD", "value"},
			wantExit:   1,
			wantOutput: "setenv: invalid variable name: 1BAD",
		},
		{
			name:      "separate args",
			env:       newInternalTestEnv(t),
			args:      []string{"GREETING", "hello"},
			wantExit:  0,
			wantValue: "hello",
		},
		{
			name:      "equals form",
			env:       newInternalTestEnv(t),
			args:      []string{"GREETING=hello world"},
			wantExit:  0,
			wantValue: "hello world",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			exitCode := runSetenv(tc.env, &out, tc.args...)
			if exitCode != tc.wantExit {
				t.Fatalf("runSetenv(%v) exitCode = %d, want %d", tc.args, exitCode, tc.wantExit)
			}
			if tc.env != nil && tc.wantValue != nil {
				if got := tc.env.Get("GREETING"); got != tc.wantValue {
					t.Fatalf("GREETING = %v, want %v", got, tc.wantValue)
				}
			}
			if got := strings.TrimSpace(out.String()); got != tc.wantOutput {
				t.Fatalf("runSetenv(%v) output = %q, want %q", tc.args, got, tc.wantOutput)
			}
		})
	}
}

func TestRunUnsetenv(t *testing.T) {
	for _, tc := range []struct {
		name       string
		env        *engine.Env
		args       []string
		prepare    func(*engine.Env)
		wantExit   int
		wantValue  any
		wantOutput string
	}{
		{
			name:       "nil env",
			env:        nil,
			args:       []string{"GREETING"},
			wantExit:   1,
			wantOutput: "unsetenv: shell environment is not initialized",
		},
		{
			name:       "requires one arg",
			env:        newInternalTestEnv(t),
			wantExit:   1,
			wantOutput: "usage: unsetenv NAME",
		},
		{
			name:       "invalid name",
			env:        newInternalTestEnv(t),
			args:       []string{"1BAD"},
			wantExit:   1,
			wantOutput: "unsetenv: invalid variable name: 1BAD",
		},
		{
			name:     "removes variable",
			env:      newInternalTestEnv(t),
			args:     []string{"GREETING"},
			wantExit: 0,
			prepare: func(env *engine.Env) {
				env.Set("GREETING", "hello")
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.prepare != nil {
				tc.prepare(tc.env)
			}
			var out bytes.Buffer
			exitCode := runUnsetenv(tc.env, &out, tc.args...)
			if exitCode != tc.wantExit {
				t.Fatalf("runUnsetenv(%v) exitCode = %d, want %d", tc.args, exitCode, tc.wantExit)
			}
			if tc.env != nil && tc.wantExit == 0 {
				if got := tc.env.Get("GREETING"); got != nil {
					t.Fatalf("GREETING = %v, want nil", got)
				}
			}
			if got := strings.TrimSpace(out.String()); got != tc.wantOutput {
				t.Fatalf("runUnsetenv(%v) output = %q, want %q", tc.args, got, tc.wantOutput)
			}
		})
	}
}

func TestRunWhich(t *testing.T) {
	for _, tc := range []struct {
		name       string
		env        *engine.Env
		args       []string
		wantExit   int
		wantOutput string
	}{
		{
			name:       "nil env",
			env:        nil,
			args:       []string{"tool"},
			wantExit:   1,
			wantOutput: "which: shell environment is not initialized",
		},
		{
			name:       "missing operand",
			env:        newInternalTestEnv(t),
			wantExit:   1,
			wantOutput: "which: missing operand",
		},
		{
			name:       "command not found",
			env:        newInternalTestEnv(t),
			args:       []string{"missing"},
			wantExit:   1,
			wantOutput: "which: command not found: missing",
		},
		{
			name:       "command found",
			env:        newInternalTestEnv(t),
			args:       []string{"tool"},
			wantExit:   0,
			wantOutput: "/work/bin/tool.js",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			exitCode := runWhich(tc.env, &out, tc.args...)
			if exitCode != tc.wantExit {
				t.Fatalf("runWhich(%v) exitCode = %d, want %d", tc.args, exitCode, tc.wantExit)
			}
			if got := strings.TrimSpace(out.String()); got != tc.wantOutput {
				t.Fatalf("runWhich(%v) output = %q, want %q", tc.args, got, tc.wantOutput)
			}
		})
	}
}

func TestDisplayPathArgAndFprintf(t *testing.T) {
	if got := displayPathArg(nil); got != "~" {
		t.Fatalf("displayPathArg(nil) = %q, want ~", got)
	}
	if got := displayPathArg([]string{""}); got != "~" {
		t.Fatalf("displayPathArg(empty) = %q, want ~", got)
	}
	if got := displayPathArg([]string{"/tmp"}); got != "/tmp" {
		t.Fatalf("displayPathArg(/tmp) = %q, want /tmp", got)
	}

	var out bytes.Buffer
	fprintf(&out, "%s=%d", "VALUE", 7)
	if got := out.String(); got != "VALUE=7" {
		t.Fatalf("fprintf wrote %q, want VALUE=7", got)
	}
	fprintf(nil, "ignored")
}

func newInternalTestEnv(t *testing.T) *engine.Env {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"home", "current", filepath.Join("current", "child"), "bin"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "current", "file.txt"), []byte("content\n"), 0644); err != nil {
		t.Fatalf("write file.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "tool.js"), []byte("module.exports = {}\n"), 0644); err != nil {
		t.Fatalf("write tool.js: %v", err)
	}

	fileSystem := engine.NewFS()
	workFS, err := engine.DirFS(root)
	if err != nil {
		t.Fatalf("open test fs: %v", err)
	}
	if err := fileSystem.Mount("/work", workFS); err != nil {
		t.Fatalf("mount test fs: %v", err)
	}
	env := engine.NewEnv(engine.WithFilesystem(fileSystem))
	env.Set("PATH", "/work/bin")
	env.Set("HOME", "/work/home")
	env.Set("PWD", "/work/current")
	return env
}
