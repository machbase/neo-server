package internal

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/machbase/neo-server/v8/jsh/engine"
)

var envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func IsCommand(cmd string) bool {
	switch cmd {
	case "cd", "setenv", "unsetenv", "which":
		return true
	default:
		return false
	}
}

// Run executes a built-in command directly against the current shell environment.
// It returns the exit code and a boolean indicating whether the command was found.
func Run(env *engine.Env, writer io.Writer, cmd string, args ...string) (int, bool) {
	switch cmd {
	case "cd":
		return runCD(env, writer, args...), true
	case "setenv":
		return runSetenv(env, writer, args...), true
	case "unsetenv":
		return runUnsetenv(env, writer, args...), true
	case "which":
		return runWhich(env, writer, args...), true
	default:
		return 0, false
	}
}

func runCD(env *engine.Env, writer io.Writer, args ...string) int {
	if env == nil {
		fprintf(writer, "cd: shell environment is not initialized\n")
		return 1
	}

	target := ""
	if len(args) > 0 {
		target = args[0]
	}
	if target == "" {
		target = "$HOME"
	}

	path := env.ResolvePath(target)
	if !strings.HasPrefix(path, "/") {
		if pwd, ok := env.Get("PWD").(string); ok && pwd != "" {
			path = pwd + "/" + path
		}
	}

	fs := env.Filesystem()
	if fs == nil {
		fprintf(writer, "cd: no such file or directory: %s\n", displayPathArg(args))
		return 1
	}

	fd, err := fs.Open(path)
	if err != nil {
		fprintf(writer, "cd: no such file or directory: %s\n", displayPathArg(args))
		return 1
	}
	defer fd.Close()

	info, err := fd.Stat()
	if err != nil {
		fprintf(writer, "cd: no such file or directory: %s\n", displayPathArg(args))
		return 1
	}
	if !info.IsDir() {
		fprintf(writer, "cd: not a directory: %s\n", displayPathArg(args))
		return 1
	}

	env.Set("PWD", filepath.ToSlash(filepath.Clean(path)))
	return 0
}

func runSetenv(env *engine.Env, writer io.Writer, args ...string) int {
	if env == nil {
		fprintf(writer, "setenv: shell environment is not initialized\n")
		return 1
	}
	if len(args) == 0 || len(args) > 2 {
		fprintf(writer, "usage: setenv NAME VALUE\n")
		fprintf(writer, "   or: setenv NAME=VALUE\n")
		return 1
	}

	var name string
	var value string
	if len(args) == 1 {
		idx := strings.IndexByte(args[0], '=')
		if idx <= 0 {
			fprintf(writer, "usage: setenv NAME VALUE\n")
			fprintf(writer, "   or: setenv NAME=VALUE\n")
			return 1
		}
		name = args[0][:idx]
		value = args[0][idx+1:]
	} else {
		name = args[0]
		value = args[1]
	}

	if !envNamePattern.MatchString(name) {
		fprintf(writer, "setenv: invalid variable name: %s\n", name)
		return 1
	}

	env.Set(name, value)
	return 0
}

func runUnsetenv(env *engine.Env, writer io.Writer, args ...string) int {
	if env == nil {
		fprintf(writer, "unsetenv: shell environment is not initialized\n")
		return 1
	}
	if len(args) != 1 {
		fprintf(writer, "usage: unsetenv NAME\n")
		return 1
	}
	if !envNamePattern.MatchString(args[0]) {
		fprintf(writer, "unsetenv: invalid variable name: %s\n", args[0])
		return 1
	}
	env.Set(args[0], nil)
	return 0
}

func runWhich(env *engine.Env, writer io.Writer, args ...string) int {
	if env == nil {
		fprintf(writer, "which: shell environment is not initialized\n")
		return 1
	}
	if len(args) == 0 {
		fprintf(writer, "which: missing operand\n")
		return 1
	}
	where := env.Which(args[0])
	if where == "" {
		fprintf(writer, "which: command not found: %s\n", args[0])
		return 1
	}
	fprintf(writer, "%s\n", where)
	return 0
}

func displayPathArg(args []string) string {
	if len(args) == 0 || args[0] == "" {
		return "~"
	}
	return args[0]
}

func fprintf(writer io.Writer, format string, args ...any) {
	if writer == nil {
		return
	}
	_, _ = fmt.Fprintf(writer, format, args...)
}
