package shell

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/root"
)

var shellTestExecBuilder engine.ExecBuilderFunc
var shellTestJshBinPath string
var shellTestDir string

func TestMain(m *testing.M) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to resolve test file path")
	}
	shellDir := filepath.Dir(filename)
	shellTestDir = filepath.Join(shellDir, "..", "..", "test")

	tmpDir := os.TempDir()
	shellTestJshBinPath = filepath.Join(tmpDir, "jsh-shell-test")
	args := []string{"build", "-o"}
	if runtime.GOOS == "windows" {
		shellTestJshBinPath += ".exe"
	}
	args = append(args, shellTestJshBinPath, filepath.Join(shellDir, "..", ".."))
	cmd := exec.Command("go", args...)
	if err := cmd.Run(); err != nil {
		os.Exit(2)
	}

	shellTestExecBuilder = func(source string, args []string, env map[string]any) (*exec.Cmd, error) {
		argv := []string{"-v", "/work=" + shellTestDir}
		keys := make([]string, 0, len(env))
		for key := range env {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			argv = append(argv, "-e", fmt.Sprintf("%s=%v", key, env[key]))
		}
		if source != "" {
			argv = append(argv, "-C", source)
		}
		argv = append(argv, args...)
		return exec.Command(shellTestJshBinPath, argv...), nil
	}

	os.Exit(m.Run())
}

func TestProcessStreamingPipeline(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		want  string
		exit  int
		alive bool
	}{
		{
			name:  "echo to cat",
			line:  "echo hello | cat",
			want:  "hello",
			exit:  0,
			alive: true,
		},
		{
			name:  "cat to wc line count",
			line:  "echo hello | cat | wc -l",
			want:  "1",
			exit:  0,
			alive: true,
		},
		{
			name:  "cat file through pipe",
			line:  "cat sample-lines.txt | cat",
			want:  "alpha\nbeta",
			exit:  0,
			alive: true,
		},
		{
			name:  "wc file line count through pipe",
			line:  "wc -l sample-lines.txt | cat",
			want:  "1 sample-lines.txt",
			exit:  0,
			alive: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sh, output := newTestShell(t)
			exitCode, alive := sh.process(tc.line)
			if exitCode != tc.exit {
				t.Fatalf("process(%q) exitCode = %d, want %d", tc.line, exitCode, tc.exit)
			}
			if alive != tc.alive {
				t.Fatalf("process(%q) alive = %v, want %v", tc.line, alive, tc.alive)
			}
			if got := normalizeTestNewlines(strings.TrimSpace(output.String())); got != normalizeTestNewlines(tc.want) {
				t.Fatalf("process(%q) output = %q, want %q", tc.line, got, tc.want)
			}
		})
	}
}

func TestProcessRedirection(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantOutput string
		readFile   string
		wantFile   string
		exit       int
		alive      bool
		prepare    func(t *testing.T)
	}{
		{
			name:       "single command stdin redirection",
			line:       "cat < sample-lines.txt",
			wantOutput: "alpha\nbeta",
			exit:       0,
			alive:      true,
		},
		{
			name:       "single command stdout redirection",
			line:       "echo hello > redir-out.txt",
			wantOutput: "",
			readFile:   "redir-out.txt",
			wantFile:   "hello\n",
			exit:       0,
			alive:      true,
		},
		{
			name:       "single command stdout append redirection",
			line:       "echo world >> redir-append.txt",
			wantOutput: "",
			readFile:   "redir-append.txt",
			wantFile:   "seed\nworld\n",
			exit:       0,
			alive:      true,
			prepare: func(t *testing.T) {
				writeTestFile(t, "redir-append.txt", "seed\n")
			},
		},
		{
			name:       "pipeline first stage stdin redirection",
			line:       "cat < sample-lines.txt | wc -l",
			wantOutput: "1",
			exit:       0,
			alive:      true,
		},
		{
			name:       "pipeline final stage stdout redirection",
			line:       "cat < sample-lines.txt | cat > redir-pipe.txt",
			wantOutput: "",
			readFile:   "redir-pipe.txt",
			wantFile:   "alpha\nbeta",
			exit:       0,
			alive:      true,
		},
		{
			name:       "mid pipeline stdout redirection rejected",
			line:       "echo hello > redir-mid.txt | cat",
			wantOutput: "",
			exit:       1,
			alive:      true,
		},
		{
			name:       "single command stderr redirection",
			line:       "write-stderr boom 1 2> redir-err.txt",
			wantOutput: "",
			readFile:   "redir-err.txt",
			wantFile:   "boom\n",
			exit:       1,
			alive:      true,
		},
		{
			name:       "single command stderr append redirection",
			line:       "write-stderr boom 1 2>> redir-err-append.txt",
			wantOutput: "",
			readFile:   "redir-err-append.txt",
			wantFile:   "seed\nboom\n",
			exit:       1,
			alive:      true,
			prepare: func(t *testing.T) {
				writeTestFile(t, "redir-err-append.txt", "seed\n")
			},
		},
		{
			name:       "pipeline stderr redirection",
			line:       "write-stderr boom 1 2> redir-pipe-err.txt | wc -l",
			wantOutput: "0",
			readFile:   "redir-pipe-err.txt",
			wantFile:   "boom\n",
			exit:       0,
			alive:      true,
		},
		{
			name:       "single command stderr merge redirection",
			line:       "write-stderr boom 1 2>&1",
			wantOutput: "boom",
			exit:       1,
			alive:      true,
		},
		{
			name:       "stderr merge follows stdout file redirection",
			line:       "write-stderr boom 1 2>&1 > redir-merged.txt",
			wantOutput: "",
			readFile:   "redir-merged.txt",
			wantFile:   "boom\n",
			exit:       1,
			alive:      true,
		},
		{
			name:       "pipeline stderr merge into stdout pipe",
			line:       "write-stderr boom 0 2>&1 | wc -l",
			wantOutput: "1",
			exit:       0,
			alive:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, name := range []string{"redir-out.txt", "redir-append.txt", "redir-pipe.txt", "redir-mid.txt", "redir-err.txt", "redir-err-append.txt", "redir-pipe-err.txt", "redir-merged.txt"} {
				fileName := name
				cleanupTestFile(t, fileName)
				t.Cleanup(func() {
					cleanupTestFile(t, fileName)
				})
			}
			if tc.prepare != nil {
				tc.prepare(t)
			}

			sh, output := newTestShell(t)
			exitCode, alive := sh.process(tc.line)
			if exitCode != tc.exit {
				t.Fatalf("process(%q) exitCode = %d, want %d", tc.line, exitCode, tc.exit)
			}
			if alive != tc.alive {
				t.Fatalf("process(%q) alive = %v, want %v", tc.line, alive, tc.alive)
			}
			if got := normalizeTestNewlines(strings.TrimSpace(output.String())); got != normalizeTestNewlines(tc.wantOutput) {
				t.Fatalf("process(%q) output = %q, want %q", tc.line, got, tc.wantOutput)
			}
			if tc.readFile != "" {
				if got := normalizeTestNewlines(readTestFile(t, tc.readFile)); got != normalizeTestNewlines(tc.wantFile) {
					t.Fatalf("file %q = %q, want %q", tc.readFile, got, tc.wantFile)
				}
			}
		})
	}
}

func TestProcessSetenvAndEnv(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantLines []string
		exit      int
		alive     bool
		checkEnv  func(t *testing.T, sh *Shell)
	}{
		{
			name: "setenv with separate name and value",
			line: "setenv GREETING hello && env",
			wantLines: []string{
				"GREETING=hello",
			},
			exit:  0,
			alive: true,
			checkEnv: func(t *testing.T, sh *Shell) {
				t.Helper()
				if got := sh.env.Get("GREETING"); got != "hello" {
					t.Fatalf("GREETING = %v, want hello", got)
				}
			},
		},
		{
			name: "setenv with equals form and quoted spaces",
			line: "setenv MESSAGE='hello world' && env",
			wantLines: []string{
				"MESSAGE=hello world",
			},
			exit:  0,
			alive: true,
			checkEnv: func(t *testing.T, sh *Shell) {
				t.Helper()
				if got := sh.env.Get("MESSAGE"); got != "hello world" {
					t.Fatalf("MESSAGE = %v, want hello world", got)
				}
			},
		},
		{
			name: "env prints only requested variables",
			line: "setenv GREETING hello && setenv TARGET world && env TARGET GREETING",
			wantLines: []string{
				"TARGET=world",
				"GREETING=hello",
			},
			exit:  0,
			alive: true,
		},
		{
			name: "env skips missing requested variables",
			line: "setenv GREETING hello && env MISSING GREETING",
			wantLines: []string{
				"GREETING=hello",
			},
			exit:  0,
			alive: true,
		},
		{
			name: "setenv rejects invalid variable name",
			line: "setenv 1BAD value",
			wantLines: []string{
				"setenv: invalid variable name: 1BAD",
			},
			exit:  1,
			alive: true,
			checkEnv: func(t *testing.T, sh *Shell) {
				t.Helper()
				if got := sh.env.Get("1BAD"); got != nil {
					t.Fatalf("1BAD = %v, want nil", got)
				}
			},
		},
		{
			name: "unsetenv removes existing variable",
			line: "setenv GREETING hello && unsetenv GREETING && env",
			wantLines: []string{
				"HOME=/work",
			},
			exit:  0,
			alive: true,
			checkEnv: func(t *testing.T, sh *Shell) {
				t.Helper()
				if got := sh.env.Get("GREETING"); got != nil {
					t.Fatalf("GREETING = %v, want nil", got)
				}
				if got := strings.TrimSpace(sh.env.Expand("$GREETING")); got != "" {
					t.Fatalf("expanded GREETING = %q, want empty", got)
				}
			},
		},
		{
			name: "unsetenv rejects invalid variable name",
			line: "unsetenv 1BAD",
			wantLines: []string{
				"unsetenv: invalid variable name: 1BAD",
			},
			exit:  1,
			alive: true,
		},
		{
			name: "unsetenv requires exactly one argument",
			line: "unsetenv",
			wantLines: []string{
				"usage: unsetenv NAME",
			},
			exit:  1,
			alive: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sh, output := newTestShell(t)
			exitCode, alive := sh.process(tc.line)
			if exitCode != tc.exit {
				t.Fatalf("process(%q) exitCode = %d, want %d", tc.line, exitCode, tc.exit)
			}
			if alive != tc.alive {
				t.Fatalf("process(%q) alive = %v, want %v", tc.line, alive, tc.alive)
			}
			gotOutput := strings.TrimSpace(output.String())
			for _, wantLine := range tc.wantLines {
				if !strings.Contains(gotOutput, wantLine) {
					t.Fatalf("process(%q) output = %q, want line %q", tc.line, gotOutput, wantLine)
				}
			}
			if strings.Contains(tc.line, "unsetenv GREETING") && strings.Contains(gotOutput, "GREETING=hello") {
				t.Fatalf("process(%q) output = %q, did not expect removed variable", tc.line, gotOutput)
			}
			if strings.Contains(tc.line, "env TARGET GREETING") {
				if strings.Contains(gotOutput, "HOME=/work") || strings.Contains(gotOutput, "PATH=/work:/sbin") {
					t.Fatalf("process(%q) output = %q, expected only selected variables", tc.line, gotOutput)
				}
			}
			if strings.Contains(tc.line, "env MISSING GREETING") && strings.Contains(gotOutput, "MISSING=") {
				t.Fatalf("process(%q) output = %q, did not expect missing variable output", tc.line, gotOutput)
			}
			if tc.checkEnv != nil {
				tc.checkEnv(t, sh)
			}
		})
	}
}

func newTestShell(t *testing.T) (*Shell, *bytes.Buffer) {
	t.Helper()
	fileSystem := engine.NewFS()
	rootTab := root.RootFSTab()
	if err := fileSystem.Mount(rootTab.MountPoint, rootTab.FS); err != nil {
		t.Fatalf("mount root fs: %v", err)
	}
	workFS, err := engine.DirFS(shellTestDir)
	if err != nil {
		t.Fatalf("work dir fs: %v", err)
	}
	if err := fileSystem.Mount("/work", workFS); err != nil {
		t.Fatalf("mount work fs: %v", err)
	}

	var output bytes.Buffer
	env := engine.NewEnv(
		engine.WithFilesystem(fileSystem),
		engine.WithExecBuilder(shellTestExecBuilder),
		engine.WithWriter(&output),
		engine.WithReader(strings.NewReader("")),
	)
	env.Set("PATH", "/work:/sbin")
	env.Set("PWD", "/work")
	env.Set("HOME", "/work")

	return &Shell{env: env}, &output
}

func testFilePath(name string) string {
	return filepath.Join(shellTestDir, name)
}

func cleanupTestFile(t *testing.T, name string) {
	t.Helper()
	if err := os.Remove(testFilePath(name)); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove %s: %v", name, err)
	}
}

func writeTestFile(t *testing.T, name string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(testFilePath(name)), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(name), err)
	}
	if err := os.WriteFile(testFilePath(name), []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func readTestFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(testFilePath(name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

func normalizeTestNewlines(value string) string {
	return strings.ReplaceAll(value, "\r\n", "\n")
}

func TestRunSinglePipelineGuards(t *testing.T) {
	t.Run("exit stops shell", func(t *testing.T) {
		sh, _ := newTestShell(t)
		exitCode, alive := sh.runSinglePipeline(&Pipeline{Command: "exit"})
		if exitCode != 0 || alive {
			t.Fatalf("runSinglePipeline(exit) = (%d, %v), want (0, false)", exitCode, alive)
		}
	})

	t.Run("internal redirection rejected", func(t *testing.T) {
		sh, _ := newTestShell(t)
		exitCode, alive := sh.runSinglePipeline(&Pipeline{
			Command: "setenv",
			Args:    []string{"GREETING", "hello"},
			Stdout:  &Redirect{Type: ">", Target: "ignored.txt"},
		})
		if exitCode != 1 || !alive {
			t.Fatalf("runSinglePipeline(internal redirect) = (%d, %v), want (1, true)", exitCode, alive)
		}
	})
}

func TestRunStreamingPipelineGuards(t *testing.T) {
	t.Run("nil env", func(t *testing.T) {
		sh := &Shell{}
		if got := sh.runStreamingPipeline([]*Pipeline{{Command: "cat"}}); got != 1 {
			t.Fatalf("runStreamingPipeline(nil env) = %d, want 1", got)
		}
	})

	for _, tc := range []struct {
		name      string
		pipelines []*Pipeline
	}{
		{
			name:      "exit in pipeline",
			pipelines: []*Pipeline{{Command: "exit"}, {Command: "cat"}},
		},
		{
			name:      "internal command in pipeline",
			pipelines: []*Pipeline{{Command: "setenv", Args: []string{"A", "B"}}, {Command: "cat"}},
		},
		{
			name: "stdin redirect only first stage",
			pipelines: []*Pipeline{{Command: "cat"}, {
				Command: "cat",
				Stdin:   &Redirect{Type: "<", Target: "sample-lines.txt"},
			}},
		},
		{
			name: "stdout redirect only last stage",
			pipelines: []*Pipeline{{
				Command: "cat",
				Stdout:  &Redirect{Type: ">", Target: "redir-mid.txt"},
			}, {Command: "cat"}},
		},
		{
			name: "unsupported stderr redirect",
			pipelines: []*Pipeline{{
				Command: "cat",
				Stderr:  &Redirect{Type: "2<", Target: "redir-mid.txt"},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sh, _ := newTestShell(t)
			if got := sh.runStreamingPipeline(tc.pipelines); got != 1 {
				t.Fatalf("runStreamingPipeline(%s) = %d, want 1", tc.name, got)
			}
		})
	}
}

func TestPipelineHelpers(t *testing.T) {
	t.Run("build external command requires env", func(t *testing.T) {
		sh := &Shell{}
		if _, err := sh.buildExternalExecCmd("cat", nil); err == nil || !strings.Contains(err.Error(), "shell environment is not initialized") {
			t.Fatalf("buildExternalExecCmd() err = %v, want shell environment error", err)
		}
	})

	t.Run("resolve external command uses alias and index fallback", func(t *testing.T) {
		sh, _ := newTestShell(t)
		sh.env.SetAlias("alias-tool", []string{"tool", "--flag"})
		writeTestFile(t, "tool/index.js", "module.exports = {}\n")
		defer cleanupTestFile(t, "tool/index.js")

		path, args, err := sh.resolveExternalCommand("alias-tool", []string{"value"})
		if err != nil {
			t.Fatalf("resolveExternalCommand(alias-tool): %v", err)
		}
		if path != "/work/tool/index.js" {
			t.Fatalf("resolveExternalCommand path = %q, want %q", path, "/work/tool/index.js")
		}
		if got, want := strings.Join(args, " "), "--flag value"; got != want {
			t.Fatalf("resolveExternalCommand args = %q, want %q", got, want)
		}
	})

	t.Run("redirect helpers reject unsupported configuration", func(t *testing.T) {
		sh, _ := newTestShell(t)
		if _, _, err := openInputRedirect(sh.env, &Redirect{Type: ">", Target: "x"}); err == nil {
			t.Fatal("openInputRedirect unsupported type: err = nil, want error")
		}
		if _, _, err := openOutputRedirect(sh.env, &Redirect{Type: "<", Target: "x"}); err == nil {
			t.Fatal("openOutputRedirect unsupported type: err = nil, want error")
		}
		if _, _, err := openErrorRedirect(sh.env, &Redirect{Type: "2>&1"}, nil); err == nil {
			t.Fatal("openErrorRedirect missing stdout: err = nil, want error")
		}
		badEnv := engine.NewEnv(engine.WithFilesystem(os.DirFS(shellTestDir)))
		if _, err := shellFilesystem(badEnv); err == nil {
			t.Fatal("shellFilesystem(non-engine fs) err = nil, want error")
		}
	})

	t.Run("wait and process helpers", func(t *testing.T) {
		cmd := exec.Command("sleep", "5")
		if err := cmd.Start(); err != nil {
			t.Fatalf("start sleep: %v", err)
		}
		killStarted([]*exec.Cmd{cmd})
		waitStarted([]*exec.Cmd{cmd})

		fail := exec.Command("sh", "-c", "exit 7")
		if err := fail.Start(); err != nil {
			t.Fatalf("start failing shell: %v", err)
		}
		exitCode, err := waitCommand(fail)
		if err != nil {
			t.Fatalf("waitCommand(fail): %v", err)
		}
		if exitCode != 7 {
			t.Fatalf("waitCommand(fail) exitCode = %d, want 7", exitCode)
		}
	})
}
