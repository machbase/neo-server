package engine_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
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
	args = append(args, "..")
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
