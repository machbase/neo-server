package shell

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
			if got := strings.TrimSpace(output.String()); got != tc.want {
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
			if got := strings.TrimSpace(output.String()); got != tc.wantOutput {
				t.Fatalf("process(%q) output = %q, want %q", tc.line, got, tc.wantOutput)
			}
			if tc.readFile != "" {
				if got := readTestFile(t, tc.readFile); got != tc.wantFile {
					t.Fatalf("file %q = %q, want %q", tc.readFile, got, tc.wantFile)
				}
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
