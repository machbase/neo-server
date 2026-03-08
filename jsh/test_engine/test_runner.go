package test_engine

import (
	"bytes"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib"
	"github.com/machbase/neo-server/v8/jsh/root"
)

var testDir string

func init() {
	//
	// find directory neo-server/jsh/test
	//
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("Failed to get current file path")
	}
	testDir = filepath.Join(filepath.Dir(filename), "..", "test")
	fmt.Printf("TestRunner initialized with testDir: %s\n", testDir)
}

// TestCase defines a single test case for the JS runtime.
// Output lines can contain "$VAR" which will be expanded using the runtime's environment variables.
// Output lines can end with "..." to indicate that only the prefix needs to match.
type TestCase struct {
	Name        string
	Script      string
	Input       []string
	Output      []string
	ExpectFunc  func(t *testing.T, result string)
	Err         string
	Vars        map[string]any
	ExecBuilder engine.ExecBuilderFunc
}

func RunTest(t *testing.T, tc TestCase) {
	t.Helper()
	t.Run(tc.Name, func(t *testing.T) {
		t.Helper()
		tmpDir := t.TempDir()
		env := map[string]any{
			"PATH":         "/work:/sbin",
			"PWD":          "/work",
			"HOME":         "/work",
			"LIBRARY_PATH": "./node_modules:/lib",
		}
		for k, v := range tc.Vars {
			env[k] = v
		}
		conf := engine.Config{
			Name: tc.Name,
			Code: tc.Script,
			FSTabs: []engine.FSTab{
				root.RootFSTab(),
				{MountPoint: "/work", Source: testDir},
				{MountPoint: "/tmp", Source: tmpDir},
				{MountPoint: "/lib", FS: lib.LibFS()},
			},
			Env:         env,
			ExecBuilder: tc.ExecBuilder,
			Reader:      &bytes.Buffer{},
			Writer:      &bytes.Buffer{},
		}
		jr, err := engine.New(conf)
		if err != nil {
			t.Fatalf("Failed to create JSRuntime: %v", err)
		}
		lib.Enable(jr)

		if len(tc.Input) > 0 {
			conf.Reader.(*bytes.Buffer).WriteString(strings.Join(tc.Input, "\n") + "\n")
		}
		if err := jr.Run(); err != nil {
			if tc.Err == "" || !strings.Contains(err.Error(), tc.Err) {
				t.Fatalf("Unexpected error: %v", err)
			}
			return
		}

		gotOutput := conf.Writer.(*bytes.Buffer).String()
		if tc.ExpectFunc != nil {
			tc.ExpectFunc(t, gotOutput)
			return
		}
		lines := strings.Split(gotOutput, "\n")
		for i, expectedLine := range tc.Output {
			if len(lines) <= i {
				t.Fatalf("Expected at least %d lines of output, got %d\nmissing:\n%s", i+1, len(lines)-1, strings.Join(tc.Output[i:], "\n"))
			}
			// Expand env variables in expected line
			if strings.Contains(expectedLine, "$") {
				str := strings.ReplaceAll(expectedLine, "$$", "_")
				if strings.Contains(str, "$") {
					expectedLine = jr.Env.Expand(expectedLine)
				} else {
					expectedLine = strings.ReplaceAll(expectedLine, "$$", "$")
				}
			}
			// Support prefix matching with "..." suffix
			if strings.HasSuffix(expectedLine, "...") {
				prefix := strings.TrimSuffix(expectedLine, "...")
				if !strings.HasPrefix(lines[i], prefix) {
					t.Errorf("Output line %d: expected to start with %q, got %q", i, prefix, lines[i])
				}
			} else if trimLine(lines[i]) != expectedLine {
				t.Errorf("Output line %d: expected %q, got %q", i, expectedLine, lines[i])
			}
		}
		if len(lines) > len(tc.Output)+1 { // +1 for trailing newline
			t.Fatalf("Expected %d output lines, got %d\nextra:\n%s", len(tc.Output), len(lines)-1, strings.Join(lines[len(tc.Output):], "\n"))
		}
	})
}

func trimLine(s string) string {
	if runtime.GOOS == "windows" {
		return strings.TrimSuffix(s, "\r")
	} else {
		return s
	}
}
