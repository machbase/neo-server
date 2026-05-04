package engine_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib"
	"github.com/machbase/neo-server/v8/jsh/root"
	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

var testExecBuilder engine.ExecBuilderFunc
var jshBinPath string

func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_PROCESS_SIGNAL_HELPER") == "1" || os.Getenv("GO_WANT_PROCESS_EXEC_SIGNAL_HELPER") == "1" {
		os.Exit(m.Run())
	}

	tmpDir := os.TempDir()
	jshBinPath = filepath.Join(tmpDir, "jsh")
	args := []string{"build", "-o"}
	if runtime.GOOS == "windows" {
		jshBinPath = jshBinPath + ".exe"
	}
	args = append(args, jshBinPath)
	args = append(args, "../../cmd/jsh")
	cmd := exec.Command("go", args...)
	if err := cmd.Run(); err != nil {
		fmt.Println("Failed to build jsh binary for tests:", err)
		os.Exit(2)
	}
	testExecBuilder = func(source string, args []string, env map[string]any) (*exec.Cmd, error) {
		if source != "" {
			args = append([]string{
				"-v", "/work=../test/",
				"-C", source,
			}, args...)
		} else {
			args = append([]string{
				"-v", "/work=../test/",
			}, args...)
		}
		return exec.Command(jshBinPath, args...), nil
	}
	os.Exit(m.Run())
}

func TestEngine(t *testing.T) {
	ts := []test_engine.TestCase{
		{
			Name:   "console_log",
			Script: `console.log("Hello, World!");`,
			Output: []string{"INFO  Hello, World!"},
		},
		{
			Name:   "console_println",
			Script: `console.println("Hello, World!");`,
			Output: []string{"Hello, World!"},
		},
		{
			Name: "module_demo",
			Script: `
				const { sayHello } = require("demo");
				sayHello("");
			`,
			Output: []string{
				"Hello  from demo.js!",
			},
		},
		{
			Name: "module_package_json",
			Script: `
				const optparse = require("optparse");
				var SWITCHES = [
					['-h', '--help', 'Show this help message'],
				];
				var parser = new optparse.OptionParser(SWITCHES);
				parser.on('help', function() {
					console.println("Package help");
				});
				parser.parse(['-h']);
			`,
			Output: []string{
				"Package help",
			},
		},
	}

	for _, tc := range ts {
		test_engine.RunTest(t, tc)
	}
}

func TestExecWithOptions(t *testing.T) {
	jr, err := engine.New(engine.Config{ExecBuilder: testExecBuilder})
	if err != nil {
		t.Fatalf("engine.New() error = %v", err)
	}

	var stdout bytes.Buffer
	exitCode, err := jr.ExecWithOptions("/sbin/echo.js", engine.ExecOptions{
		Stdout: &stdout,
		Stderr: &stdout,
	}, "hello from options")
	if err != nil {
		t.Fatalf("ExecWithOptions() error = %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("ExecWithOptions() exitCode = %d, want 0", exitCode)
	}
	if got := strings.TrimSpace(stdout.String()); got != "hello from options" {
		t.Fatalf("ExecWithOptions() stdout = %q, want %q", got, "hello from options")
	}
}

func TestExecStringWithOptions(t *testing.T) {
	jr, err := engine.New(engine.Config{ExecBuilder: testExecBuilder})
	if err != nil {
		t.Fatalf("engine.New() error = %v", err)
	}

	var stdout bytes.Buffer
	exitCode, err := jr.ExecStringWithOptions(`
		const process = require("process");
		const text = process.stdin.read();
		console.println(text.trim());
	`, engine.ExecOptions{
		Stdin:  strings.NewReader("hello from stdin\n"),
		Stdout: &stdout,
		Stderr: &stdout,
	})
	if err != nil {
		t.Fatalf("ExecStringWithOptions() error = %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("ExecStringWithOptions() exitCode = %d, want 0", exitCode)
	}
	if got := strings.TrimSpace(stdout.String()); got != "hello from stdin" {
		t.Fatalf("ExecStringWithOptions() stdout = %q, want %q", got, "hello from stdin")
	}
}

func TestProcessStderrUsesErrorWriter(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	jr, err := engine.New(engine.Config{
		Code:        `const process = require("process"); process.stderr.write("stderr only"); console.println("stdout only");`,
		Writer:      &stdout,
		ErrorWriter: &stderr,
		FSTabs: []engine.FSTab{
			root.RootFSTab(),
			lib.LibFSTab(),
		},
	})
	if err != nil {
		t.Fatalf("engine.New() error = %v", err)
	}
	lib.Enable(jr)
	if err := jr.Run(); err != nil {
		t.Fatalf("jr.Run() error = %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "stdout only" {
		t.Fatalf("stdout = %q, want %q", got, "stdout only")
	}
	if got := stderr.String(); got != "stderr only" {
		t.Fatalf("stderr = %q, want %q", got, "stderr only")
	}
}

func TestExecStringSeparatesStdoutAndStderrByDefault(t *testing.T) {
	var stderr bytes.Buffer
	jr, err := engine.New(engine.Config{ExecBuilder: testExecBuilder, ErrorWriter: &stderr})
	if err != nil {
		t.Fatalf("engine.New() error = %v", err)
	}

	var stdout bytes.Buffer
	exitCode, err := jr.ExecStringWithOptions(`
		const process = require("process");
		process.stderr.write("stderr from child");
		console.println("stdout from child");
	`, engine.ExecOptions{
		Stdout: &stdout,
	})
	if err != nil {
		t.Fatalf("ExecStringWithOptions() error = %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("ExecStringWithOptions() exitCode = %d, want 0", exitCode)
	}
	if got := strings.TrimSpace(stdout.String()); got != "stdout from child" {
		t.Fatalf("stdout = %q, want %q", got, "stdout from child")
	}
	if got := stderr.String(); got != "stderr from child" {
		t.Fatalf("stderr = %q, want %q", got, "stderr from child")
	}
}

// TestScriptExceptionGoesToErrorWriter verifies that a JS exception thrown by a
// script does NOT contaminate the stdout writer (e.g. CgiBinWriter). This is the
// root cause of "missing header separator" when a cgi-bin script throws before
// emitting the CGI header separator.
func TestScriptExceptionGoesToErrorWriter(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	jr, err := engine.New(engine.Config{
		Code:        `console.println("before"); throw new Error("boom");`,
		Writer:      &stdout,
		ErrorWriter: &stderr,
		FSTabs: []engine.FSTab{
			root.RootFSTab(),
			lib.LibFSTab(),
		},
	})
	if err != nil {
		t.Fatalf("engine.New() error = %v", err)
	}
	lib.Enable(jr)
	// Run returns an error (the exception) - that is expected
	_ = jr.Run()

	if got := strings.TrimSpace(stdout.String()); got != "before" {
		t.Fatalf("stdout should only contain 'before', got %q", got)
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Fatalf("stderr should contain 'boom', got %q", stderr.String())
	}
}

func TestSetTimeout(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "setTimeout_basic",
			Script: `
				const {now} = require("/lib/process");
				let t = now();
				setTimeout(() => {
					console.log("Timeout executed");
					testDone();
				}, 100);
			`,
			Output: []string{
				"INFO  Timeout executed",
			},
		},
		{
			Name: "setTimeout_args",
			Script: `
				var arg1, arg2;
				setTimeout((a, b) => {
					console.println("Timeout with args:", a, b);
					arg1 = a;
					arg2 = b;
					testDone();
				}, 50,  "test", 42);
			`,
			Output: []string{
				"Timeout with args: test 42",
			},
		},
		{
			Name: "clearTimeout_basic",
			Script: `
				var counter = 0;
				var sum = 0;

				function add(a) {
					counter++;
					sum += a;
					tm = setTimeout(add, 50, a+1);
					if(counter >= 3) {
						clearTimeout(tm);
						setTimeout(()=>{testDone();}, 100);
					}
					console.println("count:", counter,", sum:", sum);					
				}
				var tm = setTimeout(add, 50, 1);
			`,
			Output: []string{
				"count: 1 , sum: 1",
				"count: 2 , sum: 3",
				"count: 3 , sum: 6",
			},
		},
		{
			Name: "clearTimeout_twice",
			Script: `
				var executed = false;
				var tm = setTimeout(()=>{ executed = true; testDone(); }, 50);
				clearTimeout(tm);
				clearTimeout(tm);
				setTimeout(()=>{ testDone(); }, 50); // Ensure test completes
				`,
			Output: []string{
				// No output expected regarding execution
			},
		},
	}
	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

// TestShutdownHook tests have been moved to process_test.go

func TestEventLoop(t *testing.T) {
	testCases := []test_engine.TestCase{
		{
			Name: "eventloop",
			Script: `
				console.log("Add event loop");
				setImmediate(() => {
					console.debug("event loop called");
				});
			`,
			Output: []string{
				"INFO  Add event loop",
				"DEBUG event loop called",
			},
		},
		{
			// the problem is the nested runOnLoop can not append to the loop
			// while loop is running with mutex lock of the job queue.
			Name: "eventloop_loop",
			Script: `
				function doIt() {
					console.println("Timeout before doIt");
					setImmediate(() => {
						console.println("event loop called from #1");
						setImmediate(() => {
							console.println("event loop called from #2");
						});
					});
				}
				function doLater() {
					console.println("Event loop after promise resolved");
				}
				console.println("Add event loop");
				setImmediate(() => {
					console.println("Starting doIt");
					setImmediate(() => {
						doIt();
					});
				});
			`,
			Output: []string{
				"Add event loop",
				"Starting doIt",
				"Timeout before doIt",
				"event loop called from #1",
				"event loop called from #2",
			},
		},
		{
			Name: "eventloop_promise",
			Script: `
				const {eventLoop} = require('/lib/process');
				function doIt() {
					return new Promise((resolve) => {
						setImmediate(() => {
							console.println("event loop called from promise");
							resolve();
						});
					});
				}
				function doLater() {
					console.println("Event loop after promise resolved");
				}
				console.println("Add event loop");
				doIt().then(() => {
					console.println("Promise resolved");
					setImmediate(doLater);
				});
			`,
			Output: []string{
				"Add event loop",
				"event loop called from promise",
				"Promise resolved",
				"Event loop after promise resolved",
			},
		},
	}
	for _, tc := range testCases {
		test_engine.RunTest(t, tc)
	}
}

// Returns nil context intentionally for testing RunContext(nil) behavior.
func nilContextForTest() context.Context {
	return nil
}

func TestRunContext(t *testing.T) {
	t.Run("nil ctx falls through to Run", func(t *testing.T) {
		var buf bytes.Buffer
		jr, err := engine.New(engine.Config{
			Code:   `console.println("ok")`,
			Writer: &buf,
		})
		if err != nil {
			t.Fatalf("engine.New: %v", err)
		}
		if err := jr.RunContext(nilContextForTest()); err != nil {
			t.Fatalf("RunContext(nil): %v", err)
		}
		if got := strings.TrimSpace(buf.String()); got != "ok" {
			t.Fatalf("output=%q, want %q", got, "ok")
		}
	})

	t.Run("normal completion returns nil", func(t *testing.T) {
		ctx := context.Background()
		var buf bytes.Buffer
		jr, err := engine.New(engine.Config{
			Code:   `console.println("done")`,
			Writer: &buf,
		})
		if err != nil {
			t.Fatalf("engine.New: %v", err)
		}
		if err := jr.RunContext(ctx); err != nil {
			t.Fatalf("RunContext: %v", err)
		}
		if got := strings.TrimSpace(buf.String()); got != "done" {
			t.Fatalf("output=%q, want %q", got, "done")
		}
	})

	t.Run("cancelled ctx returns ctx error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		jr, err := engine.New(engine.Config{
			// Script loops indefinitely; cancellation interrupts it.
			Code: `while(true) {}`,
		})
		if err != nil {
			t.Fatalf("engine.New: %v", err)
		}

		go func() {
			// Give the event loop a moment to start before cancelling.
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()

		runErr := jr.RunContext(ctx)
		if runErr == nil {
			t.Fatal("RunContext should return an error when ctx is cancelled")
		}
		if runErr != context.Canceled {
			t.Fatalf("RunContext error = %v, want context.Canceled", runErr)
		}
	})

	t.Run("script error propagates when ctx is still active", func(t *testing.T) {
		ctx := context.Background()
		jr, err := engine.New(engine.Config{
			Code: `throw new Error("boom")`,
		})
		if err != nil {
			t.Fatalf("engine.New: %v", err)
		}
		runErr := jr.RunContext(ctx)
		if runErr == nil {
			t.Fatal("RunContext should return an error for a thrown exception")
		}
		if strings.Contains(runErr.Error(), "boom") == false {
			t.Fatalf("RunContext error = %v, want it to contain 'boom'", runErr)
		}
	})

	// Regression history:
	//
	// During development of CGI SSE support and SIGINT handling, we hit a severe
	// shutdown issue: request/context cancellation interrupted the JS runtime, but
	// a child process started by process.exec() could remain alive and keep waiting
	// in the background. In practice this surfaced as:
	//   1) foreground server Ctrl+C appearing to hang,
	//   2) lingering CGI child processes visible in ps,
	//   3) shutdown only completing after manually killing those child processes.
	//
	// The fix path bound RunContext()'s context into JSRuntime and made exec0()
	// terminate child process groups when context cancellation is observed.
	//
	// This test protects that exact failure mode by running a long-lived shell
	// command that intentionally ignores SIGTERM, then forcing context timeout.
	// We assert that RunContext() returns promptly with a cancellation error
	// instead of hanging due to a leaked child process.
	t.Run("cancelled ctx stops process.exec child", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("process.exec child cancellation regression test is unix-only")
		}
		if _, err := os.Stat("/bin/sh"); err != nil {
			t.Skipf("/bin/sh is unavailable: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		jr, err := engine.New(engine.Config{
			Code: `
				const process = require("process");
				process.exec("@/bin/sh", "-c", "trap '' TERM; while :; do sleep 1; done");
			`,
			FSTabs: []engine.FSTab{
				root.RootFSTab(),
				lib.LibFSTab(),
			},
		})
		if err != nil {
			t.Fatalf("engine.New: %v", err)
		}
		lib.Enable(jr)

		start := time.Now()
		runErr := jr.RunContext(ctx)
		elapsed := time.Since(start)

		if runErr == nil {
			t.Fatal("RunContext should return context cancellation error")
		}
		if runErr != context.DeadlineExceeded && runErr != context.Canceled {
			t.Fatalf("RunContext error = %v, want context cancellation", runErr)
		}
		if elapsed > 5*time.Second {
			t.Fatalf("RunContext cancellation took too long: %v", elapsed)
		}
	})
}
