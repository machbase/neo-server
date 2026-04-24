package shell

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

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
			name:  "cat to cat preserves content",
			line:  "cat sample-lines.txt | cat",
			want:  "alpha\nbeta",
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

func TestPipelineInterruptForwardingIntegration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGINT integration is only covered on unix-like platforms")
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestPipelineInterruptForwardingHelper$", "--")
	cmd.Env = append(os.Environ(), "GO_WANT_PIPELINE_INTERRUPT_HELPER=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("interrupt helper failed: %v\noutput:\n%s", err, string(out))
	}

	text := normalizeTestNewlines(string(out))
	if !strings.Contains(text, "child-ready") {
		t.Fatalf("helper output does not contain child-ready marker:\n%s", text)
	}
	if !strings.Contains(text, "child-caught") {
		t.Fatalf("helper output does not contain child-caught marker:\n%s", text)
	}
	if !strings.Contains(text, "alive:true") {
		t.Fatalf("helper output does not contain alive:true marker:\n%s", text)
	}
}

func TestPipelineInterruptForwardingHelper(t *testing.T) {
	if os.Getenv("GO_WANT_PIPELINE_INTERRUPT_HELPER") != "1" {
		return
	}

	if runtime.GOOS == "windows" {
		t.Skip("helper runs only on unix-like platforms")
	}

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer devNull.Close()

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
		engine.WithReader(devNull),
	)
	env.Set("PATH", "/work:/sbin")
	env.Set("PWD", "/work")
	env.Set("HOME", "/work")

	sh := &Shell{env: env}

	go func() {
		// Trigger SIGINT while shell is blocked in external command wait path.
		time.Sleep(400 * time.Millisecond)
		if proc, err := os.FindProcess(os.Getpid()); err == nil {
			_ = proc.Signal(os.Interrupt)
		}
	}()

	exitCode, alive := sh.process(`@sh -c "trap 'echo child-caught; exit 0' INT; echo child-ready; while :; do sleep 1; done"`)

	text := normalizeTestNewlines(output.String())
	if !strings.Contains(text, "child-ready") {
		t.Fatalf("missing child-ready marker in helper output:\n%s", text)
	}
	if !strings.Contains(text, "child-caught") {
		t.Fatalf("missing child-caught marker in helper output:\n%s", text)
	}
	if !alive {
		t.Fatalf("shell should stay alive after SIGINT forwarding, exitCode=%d output:\n%s", exitCode, text)
	}

	writer := bufio.NewWriter(os.Stdout)
	_, _ = writer.WriteString(text)
	_, _ = writer.WriteString(fmt.Sprintf("\nexitCode:%d alive:%v\n", exitCode, alive))
	_ = writer.Flush()
}

func TestShouldForwardInterrupts(t *testing.T) {
	t.Run("non file reader", func(t *testing.T) {
		if shouldForwardInterrupts(bytes.NewBufferString("x")) {
			t.Fatal("shouldForwardInterrupts should be false for non-file readers")
		}
	})

	t.Run("stat error", func(t *testing.T) {
		f, err := os.CreateTemp("", "pipeline-test-*")
		if err != nil {
			t.Fatalf("create temp file: %v", err)
		}
		name := f.Name()
		_ = f.Close()
		defer os.Remove(name)

		if shouldForwardInterrupts(f) {
			t.Fatal("shouldForwardInterrupts should be false when file stat fails")
		}
	})

	t.Run("character device", func(t *testing.T) {
		f, err := os.Open(os.DevNull)
		if err != nil {
			t.Fatalf("open dev null: %v", err)
		}
		defer f.Close()

		st, err := f.Stat()
		if err != nil {
			t.Fatalf("stat dev null: %v", err)
		}
		want := (st.Mode() & os.ModeCharDevice) != 0
		if got := shouldForwardInterrupts(f); got != want {
			t.Fatalf("shouldForwardInterrupts(devnull)=%v, want %v", got, want)
		}
	})
}

func TestStartInterruptForwarder(t *testing.T) {
	t.Run("disabled no-op", func(t *testing.T) {
		stop := startInterruptForwarder(false, func() []*exec.Cmd {
			return nil
		})
		stop()
	})

	t.Run("enabled forwards interrupt in helper", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("SIGINT forwarding helper is only covered on unix-like platforms")
		}

		cmd := exec.Command(os.Args[0], "-test.run=^TestStartInterruptForwarderHelper$", "--")
		cmd.Env = append(os.Environ(), "GO_WANT_START_INTERRUPT_FORWARDER_HELPER=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("startInterruptForwarder helper failed: %v\noutput:\n%s", err, string(out))
		}
		if !strings.Contains(normalizeTestNewlines(string(out)), "child-caught") {
			t.Fatalf("helper output does not contain child-caught marker:\n%s", string(out))
		}
	})

	t.Run("enabled forwards interrupt in process", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("SIGINT forwarding is only covered on unix-like platforms")
		}

		child := exec.Command("sh", "-c", "trap 'echo child-caught; exit 0' INT; echo child-ready; while :; do :; done")
		var childOutput bytes.Buffer
		child.Stdout = &childOutput
		child.Stderr = &childOutput
		if err := child.Start(); err != nil {
			t.Fatalf("start child: %v", err)
		}

		stop := startInterruptForwarder(true, func() []*exec.Cmd {
			return []*exec.Cmd{nil, &exec.Cmd{}, child}
		})
		defer stop()

		ready := false
		for i := 0; i < 40; i++ {
			if strings.Contains(childOutput.String(), "child-ready") {
				ready = true
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if !ready {
			_ = child.Process.Kill()
			_, _ = waitCommand(child)
			t.Fatalf("timed out waiting for child-ready, output:\n%s", childOutput.String())
		}

		proc, err := os.FindProcess(os.Getpid())
		if err != nil {
			t.Fatalf("find self process: %v", err)
		}
		if err := proc.Signal(os.Interrupt); err != nil {
			t.Fatalf("send interrupt: %v", err)
		}

		doneWait := make(chan error, 1)
		go func() {
			_, err := waitCommand(child)
			doneWait <- err
		}()

		select {
		case err := <-doneWait:
			if err != nil {
				t.Fatalf("child wait: %v\noutput:\n%s", err, childOutput.String())
			}
		case <-time.After(3 * time.Second):
			_ = child.Process.Kill()
			<-doneWait
			t.Fatalf("timed out waiting for child exit, output:\n%s", childOutput.String())
		}

		out := normalizeTestNewlines(childOutput.String())
		if !strings.Contains(out, "child-caught") {
			t.Fatalf("did not observe child-caught after forwarded interrupt, output:\n%s", out)
		}
	})
}

func TestStartInterruptForwarderHelper(t *testing.T) {
	if os.Getenv("GO_WANT_START_INTERRUPT_FORWARDER_HELPER") != "1" {
		return
	}
	if runtime.GOOS == "windows" {
		t.Skip("helper runs only on unix-like platforms")
	}

	child := exec.Command("sh", "-c", "trap 'echo child-caught; exit 0' INT; echo child-ready; while :; do sleep 1; done")
	var childOutput bytes.Buffer
	child.Stdout = &childOutput
	child.Stderr = &childOutput
	if err := child.Start(); err != nil {
		t.Fatalf("start helper child: %v", err)
	}

	stop := startInterruptForwarder(true, func() []*exec.Cmd {
		return []*exec.Cmd{nil, {}, child}
	})
	defer stop()

	ready := false
	for i := 0; i < 40; i++ {
		if strings.Contains(childOutput.String(), "child-ready") {
			ready = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		_ = child.Process.Kill()
		_, _ = waitCommand(child)
		t.Fatalf("child-ready marker not observed, output:\n%s", childOutput.String())
	}

	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("find self process: %v", err)
	}
	if err := proc.Signal(os.Interrupt); err != nil {
		t.Fatalf("send interrupt to self: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := waitCommand(child)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("child wait error: %v\noutput:\n%s", err, childOutput.String())
		}
	case <-time.After(3 * time.Second):
		_ = child.Process.Kill()
		<-done
		t.Fatalf("child did not exit after forwarded interrupt, output:\n%s", childOutput.String())
	}

	_, _ = io.WriteString(os.Stdout, normalizeTestNewlines(childOutput.String()))
}

func TestProcessWordExpansion(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		want  string
		exit  int
		alive bool
	}{
		{
			name:  "unquoted env expands",
			line:  "echo $HOME",
			want:  "/work",
			exit:  0,
			alive: true,
		},
		{
			name:  "single quoted env stays literal",
			line:  "echo '$HOME'",
			want:  "$HOME",
			exit:  0,
			alive: true,
		},
		{
			name:  "double quoted env expands",
			line:  "echo \"$HOME\"",
			want:  "/work",
			exit:  0,
			alive: true,
		},
		{
			name:  "unquoted glob expands",
			line:  "echo *.txt",
			want:  "sample-lines.txt",
			exit:  0,
			alive: true,
		},
		{
			name:  "single quoted glob stays literal",
			line:  "echo '*.txt'",
			want:  "*.txt",
			exit:  0,
			alive: true,
		},
		{
			name:  "double quoted glob stays literal",
			line:  "echo \"*.txt\"",
			want:  "*.txt",
			exit:  0,
			alive: true,
		},
		{
			name:  "env expansion can feed glob",
			line:  "setenv PATTERN *.txt && echo $PATTERN",
			want:  "sample-lines.txt",
			exit:  0,
			alive: true,
		},
		{
			name:  "mixed fragments expand and join",
			line:  "echo ab\"$HOME\"cd",
			want:  "ab/workcd",
			exit:  0,
			alive: true,
		},
		{
			name:  "quoted wildcard remains protected inside mixed word",
			line:  "echo \"*.txt\"suffix",
			want:  "*.txtsuffix",
			exit:  0,
			alive: true,
		},
		{
			name:  "unquoted wildcard stays glob eligible across fragments",
			line:  "echo sample\"-lines\"*.txt",
			want:  "sample-lines.txt",
			exit:  0,
			alive: true,
		},
		{
			name:  "empty quoted argument is preserved",
			line:  "echo \"\"",
			want:  "",
			exit:  0,
			alive: true,
		},
		{
			name:  "unmatched glob stays literal",
			line:  "echo missing*.txt",
			want:  "missing*.txt",
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
			got := strings.TrimSpace(output.String())
			if got != tc.want {
				t.Fatalf("process(%q) output = %q, want %q", tc.line, got, tc.want)
			}
		})
	}
}

func TestProcessAliasExpansion(t *testing.T) {
	tests := []struct {
		name       string
		aliases    map[string][]string
		line       string
		wantOutput string
		exit       int
		alive      bool
	}{
		{
			name: "alias to internal command",
			aliases: map[string][]string{
				"where": {"which"},
			},
			line:       "where cat",
			wantOutput: "/sbin/cat.js",
			exit:       0,
			alive:      true,
		},
		{
			name: "alias prepends default args",
			aliases: map[string][]string{
				"say": {"echo", "hello"},
			},
			line:       "say world",
			wantOutput: "hello world",
			exit:       0,
			alive:      true,
		},
		{
			name: "alias works inside pipeline stage",
			aliases: map[string][]string{
				"show": {"cat"},
			},
			line:       "echo hello | show | wc -l",
			wantOutput: "1",
			exit:       0,
			alive:      true,
		},
		{
			name: "alias expands command with flags",
			aliases: map[string][]string{
				"lc": {"wc", "-l"},
			},
			line:       "echo hello | lc",
			wantOutput: "1",
			exit:       0,
			alive:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sh, output := newTestShell(t)
			for name, alias := range tc.aliases {
				sh.env.SetAlias(name, alias)
			}

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
		})
	}
}

func TestProcessAliasCommand(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantOutput string
		exit       int
		alive      bool
	}{
		{
			name:       "alias command defines alias in current shell",
			line:       "alias say echo hello && say world",
			wantOutput: "hello world",
			exit:       0,
			alive:      true,
		},
		{
			name:       "alias command lists aliases",
			line:       "alias say echo hello && alias",
			wantOutput: "alias say echo hello",
			exit:       0,
			alive:      true,
		},
		{
			name:       "alias command shows one alias",
			line:       "alias say echo hello && alias say",
			wantOutput: "alias say echo hello",
			exit:       0,
			alive:      true,
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
			if got := strings.TrimSpace(output.String()); got != tc.wantOutput {
				t.Fatalf("process(%q) output = %q, want %q", tc.line, got, tc.wantOutput)
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
		if _, err := sh.buildExternalExecCmd("cat", nil, nil); err == nil || !strings.Contains(err.Error(), "shell environment is not initialized") {
			t.Fatalf("buildExternalExecCmd() err = %v, want shell environment error", err)
		}
	})

	t.Run("resolve external command uses index fallback", func(t *testing.T) {
		sh, _ := newTestShell(t)
		writeTestFile(t, "tool/index.js", "module.exports = {}\n")
		defer cleanupTestFile(t, "tool/index.js")

		path, args, err := sh.resolveExternalCommand("tool", []string{"value"})
		if err != nil {
			t.Fatalf("resolveExternalCommand(tool): %v", err)
		}
		if path != "/work/tool/index.js" {
			t.Fatalf("resolveExternalCommand path = %q, want %q", path, "/work/tool/index.js")
		}
		if got, want := strings.Join(args, " "), "value"; got != want {
			t.Fatalf("resolveExternalCommand args = %q, want %q", got, want)
		}
	})

	t.Run("expand pipeline applies alias before execution", func(t *testing.T) {
		sh, _ := newTestShell(t)
		sh.env.SetAlias("where", []string{"which", "cat"})

		expanded, err := sh.expandPipeline(&Pipeline{
			CommandWord: &Word{Fragments: []WordFragment{{Text: "where", QuoteKind: QuoteNone}}},
			ArgWords: []Word{
				{Fragments: []WordFragment{{Text: "wc", QuoteKind: QuoteNone}}},
			},
		})
		if err != nil {
			t.Fatalf("expandPipeline(alias): %v", err)
		}
		if expanded.Command != "which" {
			t.Fatalf("expanded command = %q, want %q", expanded.Command, "which")
		}
		if got, want := strings.Join(expanded.Args, " "), "cat wc"; got != want {
			t.Fatalf("expanded args = %q, want %q", got, want)
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

// TestProcessAssignmentPrefix tests assignment-prefix syntax: NAME=VALUE cmd arg
func TestProcessAssignmentPrefix(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		want        string
		exit        int
		alive       bool
		wantContain string // partial match for multi-line output
	}{
		{
			name:  "single assignment passed to external command",
			line:  "FOO=bar env FOO",
			want:  "FOO=bar",
			exit:  0,
			alive: true,
		},
		{
			name:  "two assignments passed to external command",
			line:  "FOO=bar BAR=baz env FOO BAR",
			want:  "FOO=bar\nBAR=baz",
			exit:  0,
			alive: true,
		},
		{
			name:  "assignment with spaces in single-quoted value",
			line:  "NAME='hello world' env NAME",
			want:  "NAME=hello world",
			exit:  0,
			alive: true,
		},
		{
			name:  "assignment with single-quoted dollar stays literal",
			line:  "NAME='$HOME' env NAME",
			want:  "NAME=$HOME",
			exit:  0,
			alive: true,
		},
		{
			name:  "assignment with double-quoted dollar expands",
			line:  `NAME="$HOME" env NAME`,
			want:  "NAME=/work",
			exit:  0,
			alive: true,
		},
		{
			name:  "last assignment wins for duplicate name",
			line:  "FOO=1 FOO=2 env FOO",
			want:  "FOO=2",
			exit:  0,
			alive: true,
		},
		{
			// This tests that assignment prefixes are parsed and applied after word expansion,
			// so that the assignment value cannot affect the command or argument expansion.
			// This is identical to the behavior of bash and zsh.
			name:  "assignment does not affect same command argument expansion",
			line:  "VAR1=123 echo pre-$VAR1-post",
			want:  "pre--post",
			exit:  0,
			alive: true,
		},
		{
			name:  "assignment does not persist after command",
			line:  "FOO=bar env FOO && env FOO",
			want:  "FOO=bar",
			exit:  0,
			alive: true,
		},
		{
			name:  "assignment in first pipeline stage",
			line:  "FOO=bar env FOO | cat",
			want:  "FOO=bar",
			exit:  0,
			alive: true,
		},
		{
			name:  "assignment in later pipeline stage",
			line:  "echo hi | FOO=bar env FOO",
			want:  "FOO=bar",
			exit:  0,
			alive: true,
		},
		{
			name:  "assignment-only statement is an error",
			line:  "FOO=bar",
			want:  "assignment without command is not supported",
			exit:  1,
			alive: true,
		},
		{
			name:  "assignment with internal command rejected",
			line:  "FOO=bar setenv X y",
			want:  "temporary environment for internal commands is not supported",
			exit:  1,
			alive: true,
		},
		{
			name:  "invalid assignment name starting with digit",
			line:  "1BAD=x env FOO",
			want:  "invalid variable name: 1BAD",
			exit:  1,
			alive: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sh, output := newTestShell(t)
			exitCode, alive := sh.process(tc.line)
			if exitCode != tc.exit {
				t.Fatalf("process(%q) exitCode = %d, want %d\noutput: %s", tc.line, exitCode, tc.exit, output.String())
			}
			if alive != tc.alive {
				t.Fatalf("process(%q) alive = %v, want %v", tc.line, alive, tc.alive)
			}
			got := normalizeTestNewlines(strings.TrimSpace(output.String()))
			want := normalizeTestNewlines(tc.want)
			if !strings.Contains(got, want) {
				t.Fatalf("process(%q) output = %q, want to contain %q", tc.line, got, want)
			}
		})
	}
}
