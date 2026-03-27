package engine_test

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib"
	"github.com/machbase/neo-server/v8/jsh/root"
	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestProcess(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "process_env",
			Script: `
				const process = require("process");
				console.println("PATH:", process.env.get("PATH"));
				console.println("PWD:", process.env.get("PWD"));
				console.println("LIBRARY_PATH:", process.env.get("LIBRARY_PATH"));
			`,
			Output: []string{
				"PATH: /work:/sbin",
				"PWD: /work",
				"LIBRARY_PATH: ./node_modules:/lib",
			},
		},
		{
			Name: "process_expand",
			Script: `
				const process = require("process");
				const expanded1 = process.expand("$HOME/file.txt");
				const expanded2 = process.expand("$HOME/../lib/file.txt");
				console.println("expanded1:", expanded1);
				console.println("expanded2:", expanded2);
			`,
			Output: []string{
				"expanded1: /work/file.txt",
				"expanded2: /work/../lib/file.txt",
			},
		},
		{
			Name: "process_argv",
			Script: `
				const process = require("process");
				console.println("argc:", process.argv.length);
				console.println("argv[1]:", process.argv[1]);
			`,
			Output: []string{
				"argc: 2",
				"argv[1]: process_argv",
			},
		},
		{
			Name: "process_cwd",
			Script: `
				const process = require("process");
				console.println("cwd:", process.cwd());
			`,
			Output: []string{
				"cwd: /work",
			},
		},
		{
			Name: "process_chdir",
			Script: `
				const process = require("process");
				console.println("before:", process.cwd());
				process.chdir("/lib");
				console.println("after:", process.cwd());
			`,
			Output: []string{
				"before: /work",
				"after: /lib",
			},
		},
		{
			Name: "process_chdir_relative",
			Script: `
				const process = require("process");
				console.println("before:", process.cwd());
				process.chdir("../lib");
				console.println("after:", process.cwd());
			`,
			Output: []string{
				"before: /work",
				"after: /lib",
			},
		},

		{
			Name: "process_now",
			Script: `
				const process = require("process");
				const now = process.now();
				console.println("type:", typeof now);
			`,
			// preTest:  func(jr *JSRuntime) { jr.nowFunc = func() time.Time { return time.Unix(1764728536, 0) } },
			// postTest: func(jr *JSRuntime) { jr.nowFunc = time.Now },
			Output: []string{
				"type: object",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessStdin(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "stdin_readLines",
			Script: `
				const process = require("process");
				const lines = process.stdin.readLines();
				console.println("lines:", lines.length);
				lines.forEach((line, i) => {
					console.println("line", i + ":", line);
				});
			`,
			Input: []string{"first line", "second line", "third line"},
			Output: []string{
				"lines: 3",
				"line 0: first line",
				"line 1: second line",
				"line 2: third line",
			},
		},
		{
			Name: "stdin_readLine",
			Script: `
				const process = require("process");
				const line = process.stdin.readLine();
				console.println("got:", line);
			`,
			Input: []string{"hello world"},
			Output: []string{
				"got: hello world",
			},
		},
		{
			Name: "stdin_read",
			Script: `
				const process = require("process");
				const data = process.stdin.read();
				console.println("length:", data.length);
				const lines = data.split("\n").filter(l => l.length > 0);
				console.println("lines:", lines.length);
			`,
			Input: []string{"line1", "line2"},
			Output: []string{
				"length: 12",
				"lines: 2",
			},
		},
		{
			Name: "stdin_readBytes",
			Script: `
				const process = require("process");
				const data = process.stdin.readBytes(5);
				console.println("read:", data);
				console.println("length:", data.length);
			`,
			Input: []string{"hello world"},
			Output: []string{
				"read: hello",
				"length: 5",
			},
		},
		{
			Name: "stdin_readBuffer",
			Script: `
				const process = require("process");
				const data = process.stdin.readBuffer(4);
				const bytes = Array.from(new Uint8Array(data));
				console.println("byteLength:", data.byteLength);
				console.println("bytes:", bytes.join(","));
			`,
			InputBytes: []byte{0x1f, 0x8b, 0x08, 0x00},
			Output: []string{
				"byteLength: 4",
				"bytes: 31,139,8,0",
			},
		},
		{
			Name: "stdin_isTTY",
			Script: `
				const process = require("process");
				const isTTY = process.stdin.isTTY();
				console.println("isTTY:", isTTY);
			`,
			Input: []string{},
			Output: []string{
				"isTTY: false",
			},
		},
		{
			Name: "stdin_empty",
			Script: `
				const process = require("process");
				const lines = process.stdin.readLines();
				const nonEmpty = lines.filter(l => l.length > 0);
				console.println("non-empty lines:", nonEmpty.length);
			`,
			Input: []string{},
			Output: []string{
				"non-empty lines: 0",
			},
		},
		{
			Name: "stdin_process_lines",
			Script: `
				const process = require("process");
				const lines = process.stdin.readLines();
				let total = 0;
				lines.forEach(line => {
					const num = parseInt(line);
					if (!isNaN(num)) {
						total += num;
					}
				});
				console.println("sum:", total);
			`,
			Input: []string{"10", "20", "30"},
			Output: []string{
				"sum: 60",
			},
		},
		{
			Name: "stdin_filter_lines",
			Script: `
				const process = require("process");
				const lines = process.stdin.readLines();
				const filtered = lines.filter(line => line.includes("test"));
				console.println("found:", filtered.length);
				filtered.forEach(line => console.println(line));
			`,
			Input: []string{"test1", "something", "test2", "other", "testing"},
			Output: []string{
				"found: 3",
				"test1",
				"test2",
				"testing",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessExec(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "exec_basic",
			Script: `
				const process = require("process");
				const path = process.which('echo');
				const exitCode = process.exec(path, "hello from exec");
				console.println("exit code:", exitCode);
			`,
			ExecBuilder: testExecBuilder,
			Output: []string{
				"hello from exec",
				"exit code: 0",
			},
		},
		{
			Name: "execString_basic",
			Script: `
				const process = require("process");
				const exitCode = process.execString("console.println('hello from execString')");
				console.println("exit code:", exitCode);
			`,
			ExecBuilder: testExecBuilder,
			Output: []string{
				"hello from execString",
				"exit code: 0",
			},
		},
		{
			Name: "exec_with_args",
			Script: `
				const process = require("process");
				const path = process.which('echo');
				const exitCode = process.exec(path, "arg1", "arg2", "arg3");
				console.println("done");
			`,
			ExecBuilder: testExecBuilder,
			Output: []string{
				"arg1 arg2 arg3",
				"done",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessShutdownHook(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "shutdown_hook_single",
			Script: `
				const process = require("process");
				process.addShutdownHook(() => {
					console.println("cleanup");
				});
				console.println("main");
			`,
			Output: []string{
				"main",
				"cleanup",
			},
		},
		{
			Name: "shutdown_hook_multiple",
			Script: `
				const process = require("process");
				process.addShutdownHook(() => {
					console.println("first hook");
				});
				process.addShutdownHook(() => {
					console.println("second hook");
				});
				console.println("main");
			`,
			Output: []string{
				"main",
				"second hook",
				"first hook",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessInfo(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "process_pid",
			Script: `
				const process = require("process");
				console.println("pid type:", typeof process.pid);
				console.println("pid > 0:", process.pid > 0);
			`,
			Output: []string{
				"pid type: number",
				"pid > 0: true",
			},
		},
		{
			Name: "process_platform_arch",
			Script: `
				const process = require("process");
				console.println("platform:", process.platform);
				console.println("arch:", process.arch);
			`,
			Output: []string{
				fmt.Sprintf("platform: %s", runtime.GOOS),
				fmt.Sprintf("arch: %s", runtime.GOARCH),
			},
		},
		{
			Name: "process_version",
			Script: `
				const process = require("process");
				console.println("version:", process.version);
				console.println("has versions:", typeof process.versions);
			`,
			Output: []string{
				"version: jsh-1.0.0",
				"has versions: object",
			},
		},
		{
			Name: "process_stdout",
			Script: `
				const process = require("process");
				process.stdout.write("Hello from stdout\n");
				console.println("stdout written");
			`,
			Output: []string{
				"Hello from stdout",
				"stdout written",
			},
		},
		{
			Name: "process_nextTick",
			Script: `
				const process = require("process");
				console.println("before nextTick");
				process.nextTick(() => {
					console.println("in nextTick");
				});
				console.println("after nextTick");
			`,
			Output: []string{
				"before nextTick",
				"after nextTick",
				"in nextTick",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessResources(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "process_memoryUsage",
			Script: `
				const process = require("process");
				const mem = process.memoryUsage();
				console.println("has rss:", typeof mem.rss);
				console.println("has heapTotal:", typeof mem.heapTotal);
				console.println("has heapUsed:", typeof mem.heapUsed);
			`,
			Output: []string{
				"has rss: number",
				"has heapTotal: number",
				"has heapUsed: number",
			},
		},
		{
			Name: "process_cpuUsage",
			Script: `
				const process = require("process");
				const cpu = process.cpuUsage();
				console.println("has user:", typeof cpu.user);
				console.println("has system:", typeof cpu.system);
			`,
			Output: []string{
				"has user: number",
				"has system: number",
			},
		},
		{
			Name: "process_uptime",
			Script: `
				const process = require("process");
				const uptime = process.uptime();
				console.println("uptime type:", typeof uptime);
				console.println("uptime >= 0:", uptime >= 0);
			`,
			Output: []string{
				"uptime type: number",
				"uptime >= 0: true",
			},
		},
		{
			Name: "process_hrtime",
			Script: `
				const process = require("process");
				const time = process.hrtime();
				console.println("is array:", Array.isArray(time));
				console.println("length:", time.length);
			`,
			Output: []string{
				"is array: true",
				"length: 2",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessEvents(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "process_event_emitter",
			Script: `
				const process = require("process");
				console.println("has on:", typeof process.on);
				console.println("has emit:", typeof process.emit);
				console.println("has removeListener:", typeof process.removeListener);
			`,
			Output: []string{
				"has on: function",
				"has emit: function",
				"has removeListener: function",
			},
		},
		{
			Name: "process_custom_event",
			Script: `
				const process = require("process");
				process.on('test', (msg) => {
					console.println("received:", msg);
				});
				process.emit('test', 'hello');
			`,
			Output: []string{
				"received: hello",
			},
		},
		{
			Name: "process_multiple_listeners",
			Script: `
				const process = require("process");
				process.on('test', () => console.println("listener 1"));
				process.on('test', () => console.println("listener 2"));
				process.emit('test');
			`,
			Output: []string{
				"listener 1",
				"listener 2",
			},
		},
		{
			Name: "process_signal_registration",
			Script: `
				const process = require("process");
				console.println("SIGINT:", process.on('sigint', () => {}) === process);
				console.println("SIGTERM:", process.once('SIGTERM', () => {}) === process);
				console.println("SIGQUIT:", process.once('SIGQUIT', () => {}) === process);
				console.println("watchSignal:", typeof process.watchSignal);
			`,
			Output: []string{
				"SIGINT: true",
				"SIGTERM: true",
				"SIGQUIT: true",
				"watchSignal: undefined",
			},
		},
		{
			Name: "process_signal_event_normalization",
			Script: `
				const process = require("process");
				let count = 0;
				process.on('sigterm', () => {
					count += 1;
					console.println('lowercase');
				});
				process.once('SIGTERM', () => {
					count += 1;
					console.println('canonical');
				});
				process.emit('SIGTERM');
				console.println('count:', count);
			`,
			Output: []string{
				"lowercase",
				"canonical",
				"count: 2",
			},
		},
		{
			Name: "process_custom_term_event_preserved",
			Script: `
				const process = require("process");
				process.once('term', () => console.println('custom term'));
				process.emit('term');
			`,
			Output: []string{
				"custom term",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessSignalForwarding(t *testing.T) {
	if runtime.GOOS == "windows" {
		requireWindowsSignalIntegration(t)
	}

	signals := []string{"SIGINT"}
	if runtime.GOOS != "windows" {
		signals = append(signals, "SIGTERM", "SIGQUIT")
	}

	for _, signalName := range signals {
		t.Run(signalName, func(t *testing.T) {
			lines, waitErr, stderrOutput := runProcessSignalHelper(t, signalName, true)
			if waitErr != nil {
				t.Fatalf("helper failed for %s: %v\nstdout:\n%s\nstderr:\n%s", signalName, waitErr, strings.Join(lines, "\n"), stderrOutput)
			}
			assertLinePresent(t, lines, "ready: "+signalName)
			assertLinePresent(t, lines, "caught: "+signalName)
		})
	}
}

func TestProcessSignalDefaultBehavior(t *testing.T) {
	if runtime.GOOS == "windows" {
		requireWindowsSignalIntegration(t)
	}

	lines, waitErr, stderrOutput := runProcessSignalHelper(t, "SIGINT", false)
	if waitErr == nil {
		t.Fatalf("expected helper without listener to terminate by signal\nstdout:\n%s\nstderr:\n%s", strings.Join(lines, "\n"), stderrOutput)
	}
	assertLinePresent(t, lines, "ready: SIGINT")
	assertLineAbsent(t, lines, "caught: SIGINT")

	lines, waitErr, stderrOutput = runProcessSignalHelper(t, "SIGINT", true)
	if waitErr != nil {
		t.Fatalf("expected helper with listener to exit cleanly: %v\nstdout:\n%s\nstderr:\n%s", waitErr, strings.Join(lines, "\n"), stderrOutput)
	}
	assertLinePresent(t, lines, "ready: SIGINT")
	assertLinePresent(t, lines, "caught: SIGINT")
}

func TestProcessSignalHelper(t *testing.T) {
	if os.Getenv("GO_WANT_PROCESS_SIGNAL_HELPER") != "1" {
		return
	}

	signalName := os.Getenv("JSH_TEST_SIGNAL")
	listenForSignal := os.Getenv("JSH_TEST_LISTEN_SIGNAL") == "1"
	script := `
		const process = require("process");
		const signalName = process.env.get("TEST_SIGNAL");
		const timer = setInterval(() => {}, 1000);
		setTimeout(() => {
			console.println("timeout:", signalName);
			clearInterval(timer);
		}, 5000);
		console.println("ready:", signalName);
	`
	if listenForSignal {
		script = `
			const process = require("process");
			const signalName = process.env.get("TEST_SIGNAL");
			const timer = setInterval(() => {}, 1000);
			const timeout = setTimeout(() => {
				console.println("timeout:", signalName);
				clearInterval(timer);
			}, 5000);
			process.once(signalName, () => {
				console.println("caught:", signalName);
				clearInterval(timer);
				clearTimeout(timeout);
			});
			console.println("ready:", signalName);
		`
	}
	conf := engine.Config{
		Name: "process_signal_helper",
		Code: script,
		Env: map[string]any{
			"PATH":         "/work:/sbin",
			"PWD":          "/work",
			"HOME":         "/work",
			"LIBRARY_PATH": "./node_modules:/lib",
			"TEST_SIGNAL":  signalName,
		},
		FSTabs: []engine.FSTab{
			root.RootFSTab(),
			lib.LibFSTab(),
		},
		Reader: bytes.NewBuffer(nil),
		Writer: os.Stdout,
	}

	jr, err := engine.New(conf)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	lib.Enable(jr)
	if err := jr.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	os.Exit(jr.ExitCode())
}

func runProcessSignalHelper(t *testing.T, signalName string, listenForSignal bool) ([]string, error, string) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=^TestProcessSignalHelper$", "--")
	prepareSignalHelperCommand(cmd)
	cmd.Env = append(os.Environ(),
		"GO_WANT_PROCESS_SIGNAL_HELPER=1",
		"JSH_TEST_SIGNAL="+signalName,
		"JSH_TEST_LISTEN_SIGNAL="+boolToEnv(listenForSignal),
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}

	linesCh := make(chan string, 16)
	scanErrCh := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			linesCh <- scanner.Text()
		}
		scanErrCh <- scanner.Err()
		close(linesCh)
	}()

	var lines []string
	readyLine := "ready: " + signalName
	readyTimer := time.NewTimer(5 * time.Second)
	defer readyTimer.Stop()

	ready := false
	for !ready {
		select {
		case line, ok := <-linesCh:
			if !ok {
				t.Fatalf("helper exited before readiness for %s\nstdout:\n%s\nstderr:\n%s", signalName, strings.Join(lines, "\n"), stderr.String())
			}
			lines = append(lines, line)
			if line == readyLine {
				ready = true
			}
		case <-readyTimer.C:
			_ = cmd.Process.Kill()
			t.Fatalf("timeout waiting for readiness for %s\nstdout:\n%s\nstderr:\n%s", signalName, strings.Join(lines, "\n"), stderr.String())
		}
	}

	if err := sendTestSignal(cmd, signalName); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("send %s: %v", signalName, err)
	}

	drainTimer := time.NewTimer(5 * time.Second)
	defer drainTimer.Stop()
	for linesCh != nil {
		select {
		case line, ok := <-linesCh:
			if !ok {
				linesCh = nil
				continue
			}
			lines = append(lines, line)
		case <-drainTimer.C:
			_ = cmd.Process.Kill()
			t.Fatalf("timeout draining helper output for %s\nstdout:\n%s\nstderr:\n%s", signalName, strings.Join(lines, "\n"), stderr.String())
		}
	}
	if err := <-scanErrCh; err != nil {
		t.Fatalf("scan helper output for %s: %v", signalName, err)
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	var waitErr error
	select {
	case err := <-waitCh:
		waitErr = err
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("timeout waiting for helper exit for %s\nstdout:\n%s\nstderr:\n%s", signalName, strings.Join(lines, "\n"), stderr.String())
	}

	return lines, waitErr, stderr.String()
}

func assertLinePresent(t *testing.T, lines []string, want string) {
	t.Helper()
	for _, line := range lines {
		if line == want {
			return
		}
	}
	t.Fatalf("missing line %q in output:\n%s", want, strings.Join(lines, "\n"))
}

func assertLineAbsent(t *testing.T, lines []string, want string) {
	t.Helper()
	for _, line := range lines {
		if line == want {
			t.Fatalf("unexpected line %q in output:\n%s", want, strings.Join(lines, "\n"))
		}
	}
}

func boolToEnv(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func testSignalByName(signalName string) os.Signal {
	switch signalName {
	case "SIGINT":
		return os.Interrupt
	case "SIGTERM":
		return syscall.Signal(15)
	case "SIGQUIT":
		return syscall.Signal(3)
	default:
		return os.Interrupt
	}
}

func TestProcessStderr(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "stderr_write",
			Script: `
				const process = require("process");
				const result = process.stderr.write("error message\n");
				console.println("write success:", result);
			`,
			Output: []string{
				"write success: true",
			},
		},
		{
			Name: "stderr_write_empty",
			Script: `
				const process = require("process");
				const result = process.stderr.write("");
				console.println("write empty:", result);
			`,
			Output: []string{
				"write empty: true",
			},
		},
		{
			Name: "stderr_isTTY",
			Script: `
				const process = require("process");
				const isTTY = process.stderr.isTTY();
				console.println("isTTY type:", typeof isTTY);
			`,
			Output: []string{
				"isTTY type: boolean",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessStdout(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "stdout_write_empty",
			Script: `
				const process = require("process");
				const result = process.stdout.write("");
				console.println("empty write:", result);
			`,
			Output: []string{
				"empty write: true",
			},
		},
		{
			Name: "stdout_isTTY",
			Script: `
				const process = require("process");
				const isTTY = process.stdout.isTTY();
				console.println("isTTY type:", typeof isTTY);
			`,
			Output: []string{
				"isTTY type: boolean",
			},
		},
		{
			Name: "stdout_write_multiple",
			Script: `
				const process = require("process");
				process.stdout.write("first\n");
				process.stdout.write("second\n");
				console.println("done");
			`,
			Output: []string{
				"first",
				"second",
				"done",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessStdinErrors(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "stdin_readBytes_no_args",
			Script: `
				const process = require("process");
				const result = process.stdin.readBytes();
				if (result instanceof Error) {
					console.println("error:", result.message.includes("requires a number"));
				} else {
					console.println("no error, got:", typeof result);
				}
			`,
			Input: []string{"test"},
			Output: []string{
				"error: true",
			},
		},
		{
			Name: "stdin_readBytes_negative",
			Script: `
				const process = require("process");
				const result = process.stdin.readBytes(-1);
				if (result instanceof Error) {
					console.println("error:", result.message.includes("positive number"));
				} else {
					console.println("no error, got:", typeof result);
				}
			`,
			Input: []string{"test"},
			Output: []string{
				"error: true",
			},
		},
		{
			Name: "stdin_readBytes_zero",
			Script: `
				const process = require("process");
				const result = process.stdin.readBytes(0);
				if (result instanceof Error) {
					console.println("error:", result.message.includes("positive number"));
				} else {
					console.println("no error, got:", typeof result);
				}
			`,
			Input: []string{"test"},
			Output: []string{
				"error: true",
			},
		},
		{
			Name: "stdin_readBytes_more_than_available",
			Script: `
				const process = require("process");
				const data = process.stdin.readBytes(100);
				console.println("read length:", data.length);
			`,
			Input: []string{"short"},
			Output: []string{
				"read length: 6",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessHrtime(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "hrtime_basic",
			Script: `
				const process = require("process");
				const time1 = process.hrtime();
				console.println("is array:", Array.isArray(time1));
				console.println("length:", time1.length);
				console.println("has seconds:", typeof time1[0]);
				console.println("has nanos:", typeof time1[1]);
			`,
			Output: []string{
				"is array: true",
				"length: 2",
				"has seconds: number",
				"has nanos: number",
			},
		},
		{
			Name: "hrtime_diff",
			Script: `
				const process = require("process");
				const start = process.hrtime();
				console.println("start type:", Array.isArray(start));
				// Small delay
				let sum = 0;
				for (let i = 0; i < 1000; i++) {
					sum += i;
				}
				const diff = process.hrtime([start[0], start[1]]);
				console.println("diff is array:", Array.isArray(diff));
				console.println("diff length:", diff.length);
				console.println("has elapsed:", diff[0] >= 0 && diff[1] >= 0);
			`,
			Output: []string{
				"start type: true",
				"diff is array: true",
				"diff length: 2",
				"has elapsed: true",
			},
		},
		{
			Name: "hrtime_with_invalid_arg",
			Script: `
				const process = require("process");
				const time = process.hrtime("invalid");
				console.println("is array:", Array.isArray(time));
				console.println("length:", time.length);
			`,
			Output: []string{
				"is array: true",
				"length: 2",
			},
		},
		{
			Name: "hrtime_with_empty_array",
			Script: `
				const process = require("process");
				const time = process.hrtime([]);
				console.println("is array:", Array.isArray(time));
				console.println("length:", time.length);
			`,
			Output: []string{
				"is array: true",
				"length: 2",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessKill(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "kill_no_args",
			Script: `
				const process = require("process");
				const result = process.kill();
				if (result instanceof Error) {
					console.println("error:", result.message.includes("requires a pid"));
				} else {
					console.println("result:", result);
				}
			`,
			Output: []string{
				"error: true",
			},
		},
		{
			Name: "kill_invalid_pid",
			Script: `
				const process = require("process");
				const result = process.kill(-1);
				if (result instanceof Error) {
					console.println("error:", result.message.includes("positive pid"));
				} else {
					console.println("result:", result);
				}
			`,
			Output: []string{
				"error: true",
			},
		},
		{
			Name: "kill_zero_current_process",
			Script: `
				const process = require("process");
				const result = process.kill(process.pid, 0);
				console.println("result:", result);
			`,
			Output: []string{
				"result: true",
			},
		},
		{
			Name: "kill_unsupported_signal",
			Script: `
				const process = require("process");
				const result = process.kill(12345, "SIGWHATEVER");
				if (result instanceof Error) {
					console.println("error:", result.message.includes("unsupported signal"));
				} else {
					console.println("result:", result);
				}
			`,
			Output: []string{
				"error: true",
			},
		},
		{
			Name: "kill_unsupported_numeric_signal",
			Script: `
				const process = require("process");
				const result = process.kill(12345, 999);
				if (result instanceof Error) {
					console.println("error:", result.message.includes("unsupported signal: 999"));
				} else {
					console.println("result:", result);
				}
			`,
			Output: []string{
				"error: true",
			},
		},
		{
			Name: "kill_missing_process_alias_signal",
			Script: `
				const process = require("process");
				const result = process.kill(99999, "term");
				if (result instanceof Error) {
					console.println("error:", result.message.includes("kill 99999 with SIGTERM"));
				} else {
					console.println("result:", result);
				}
			`,
			Output: []string{
				"error: true",
			},
		},
		{
			Name: "kill_missing_process",
			Script: `
				const process = require("process");
				const result = process.kill(99999, "SIGTERM");
				if (result instanceof Error) {
					console.println("error:", result.message.includes("kill 99999 with SIGTERM"));
				} else {
					console.println("result:", result);
				}
			`,
			Output: []string{
				"error: true",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessKillIntegration(t *testing.T) {
	signalName := "SIGTERM"
	if runtime.GOOS == "windows" {
		requireWindowsSignalIntegration(t)
		signalName = "SIGINT"
	}
	lines, cmd, stderr := startProcessSignalHelper(t, signalName, true)

	runProcessKillScript(t, cmd.Process.Pid, fmt.Sprintf(`%q`, signalName))

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case err := <-waitCh:
		if err != nil {
			_ = cmd.Process.Kill()
			t.Fatalf("helper failed after process.kill: %v\nstderr:\n%s", err, stderr.String())
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("timeout waiting for helper after process.kill")
	}

	finalLines := collectRemainingLines(lines)
	assertLinePresent(t, finalLines, "ready: "+signalName)
	assertLinePresent(t, finalLines, "caught: "+signalName)
}

func TestProcessKillNumericIntegration(t *testing.T) {
	signalName := "SIGTERM"
	signalExpr := `15`
	if runtime.GOOS == "windows" {
		requireWindowsSignalIntegration(t)
		signalName = "SIGINT"
		signalExpr = `2`
	}
	lines, cmd, stderr := startProcessSignalHelper(t, signalName, true)

	runProcessKillScript(t, cmd.Process.Pid, signalExpr)

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case err := <-waitCh:
		if err != nil {
			_ = cmd.Process.Kill()
			t.Fatalf("helper failed after numeric process.kill: %v\nstderr:\n%s", err, stderr.String())
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("timeout waiting for helper after numeric process.kill")
	}

	finalLines := collectRemainingLines(lines)
	assertLinePresent(t, finalLines, "ready: "+signalName)
	assertLinePresent(t, finalLines, "caught: "+signalName)
}

func TestProcessKillAliasIntegration(t *testing.T) {
	signalName := "SIGTERM"
	signalExpr := `"term"`
	if runtime.GOOS == "windows" {
		requireWindowsSignalIntegration(t)
		signalName = "SIGINT"
		signalExpr = `"int"`
	}

	lines, cmd, stderr := startProcessSignalHelper(t, signalName, true)

	runProcessKillScript(t, cmd.Process.Pid, signalExpr)

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case err := <-waitCh:
		if err != nil {
			_ = cmd.Process.Kill()
			t.Fatalf("helper failed after alias process.kill: %v\nstderr:\n%s", err, stderr.String())
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("timeout waiting for helper after alias process.kill")
	}

	finalLines := collectRemainingLines(lines)
	assertLinePresent(t, finalLines, "ready: "+signalName)
	assertLinePresent(t, finalLines, "caught: "+signalName)
}

func runProcessKillScript(t *testing.T, pid int, signalExpr string) {
	t.Helper()
	writer := &bytes.Buffer{}
	conf := engine.Config{
		Name: "process_kill_integration",
		Code: fmt.Sprintf(`
			const process = require("process");
			const result = process.kill(%d, %s);
			if (result instanceof Error) {
				throw result;
			}
			console.println("kill:", result);
		`, pid, signalExpr),
		Env: map[string]any{
			"PATH":         "/work:/sbin",
			"PWD":          "/work",
			"HOME":         "/work",
			"LIBRARY_PATH": "./node_modules:/lib",
		},
		FSTabs: []engine.FSTab{
			root.RootFSTab(),
			lib.LibFSTab(),
		},
		Reader: bytes.NewBuffer(nil),
		Writer: writer,
	}

	jr, err := engine.New(conf)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	lib.Enable(jr)
	if err := jr.Run(); err != nil {
		t.Fatalf("run process.kill script: %v", err)
	}
	if !strings.Contains(writer.String(), "kill: true") {
		t.Fatalf("unexpected process.kill output: %s", writer.String())
	}
}

func startProcessSignalHelper(t *testing.T, signalName string, listenForSignal bool) (<-chan string, *exec.Cmd, *bytes.Buffer) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=^TestProcessSignalHelper$", "--")
	prepareSignalHelperCommand(cmd)
	cmd.Env = append(os.Environ(),
		"GO_WANT_PROCESS_SIGNAL_HELPER=1",
		"JSH_TEST_SIGNAL="+signalName,
		"JSH_TEST_LISTEN_SIGNAL="+boolToEnv(listenForSignal),
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}

	linesCh := make(chan string, 16)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			linesCh <- scanner.Text()
		}
		close(linesCh)
	}()

	var lines []string
	readyLine := "ready: " + signalName
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	for {
		select {
		case line, ok := <-linesCh:
			if !ok {
				t.Fatalf("helper exited before readiness for %s\nstdout:\n%s\nstderr:\n%s", signalName, strings.Join(lines, "\n"), stderr.String())
			}
			lines = append(lines, line)
			if line == readyLine {
				buffered := make(chan string, 16)
				for _, existing := range lines {
					buffered <- existing
				}
				go func() {
					for line := range linesCh {
						buffered <- line
					}
					close(buffered)
				}()
				return buffered, cmd, stderr
			}
		case <-timer.C:
			_ = cmd.Process.Kill()
			t.Fatalf("timeout waiting for readiness for %s\nstdout:\n%s\nstderr:\n%s", signalName, strings.Join(lines, "\n"), stderr.String())
		}
	}
}

func collectRemainingLines(lines <-chan string) []string {
	var collected []string
	for line := range lines {
		collected = append(collected, line)
	}
	return collected
}

func TestProcessNextTick(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "nextTick_with_args",
			Script: `
				const process = require("process");
				process.nextTick((a, b, c) => {
					console.println("args:", a, b, c);
				}, "first", "second", "third");
				console.println("main");
			`,
			Output: []string{
				"main",
				"args: first second third",
			},
		},
		{
			Name: "nextTick_no_callback",
			Script: `
				const process = require("process");
				const result = process.nextTick();
				console.println("result:", result === undefined ? "undefined" : result);
			`,
			Output: []string{
				"result: undefined",
			},
		},
		{
			Name: "nextTick_non_function",
			Script: `
				const process = require("process");
				const result = process.nextTick("not a function");
				console.println("result:", result === undefined ? "undefined" : result);
			`,
			Output: []string{
				"result: undefined",
			},
		},
		{
			Name: "nextTick_multiple",
			Script: `
				const process = require("process");
				process.nextTick(() => console.println("tick 1"));
				process.nextTick(() => console.println("tick 2"));
				process.nextTick(() => console.println("tick 3"));
				console.println("main");
			`,
			Output: []string{
				"main",
				"tick 1",
				"tick 2",
				"tick 3",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessChdir(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "chdir_to_home",
			Script: `
				const process = require("process");
				process.chdir("~");
				console.println("cwd after ~:", process.cwd());
			`,
			Output: []string{
				"cwd after ~: /work",
			},
		},
		{
			Name: "chdir_empty_string",
			Script: `
				const process = require("process");
				process.chdir("");
				console.println("cwd after empty:", process.cwd());
			`,
			Output: []string{
				"cwd after empty: /work",
			},
		},
		{
			Name: "chdir_nonexistent",
			Script: `
				const process = require("process");
				try {
					process.chdir("/nonexistent/path");
					console.println("should not reach here");
				} catch (e) {
					console.println("error caught:", e.message.includes("no such file"));
				}
			`,
			Output: []string{
				"error caught: true",
			},
		},
		{
			Name: "chdir_to_file",
			Script: `
				const process = require("process");
				try {
					process.chdir("/sbin/echo.js");
					console.println("should not reach here");
				} catch (e) {
					console.println("error caught:", e.message.includes("not a directory"));
				}
			`,
			Output: []string{
				"error caught: true",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessExecErrors(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "exec_no_args",
			Script: `
				const process = require("process");
				try {
					const result = process.exec();
				} catch (e) {
					console.println("error caught:", e.message);
				}
			`,
			Output: []string{
				"error caught: no command provided",
			},
		},
		{
			Name: "execString_no_args",
			Script: `
				const process = require("process");
				try {
					const result = process.execString();
				} catch (e) {
					console.println("error caught:", e.message);
				}
			`,
			Output: []string{
				"error caught: no source provided",
			},
		},
		{
			Name: "execString_with_args",
			Script: `
				const process = require("process");
				const exitCode = process.execString(
					"console.println('sum:', 10 + 20)",
					"10", "20"
				);
				console.println("exit code:", exitCode);
			`,
			ExecBuilder: testExecBuilder,
			Output: []string{
				"sum: 30",
				"exit code: 0",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessProperties(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "process_ppid",
			Script: `
				const process = require("process");
				console.println("ppid type:", typeof process.ppid);
				console.println("ppid > 0:", process.ppid > 0);
			`,
			Output: []string{
				"ppid type: number",
				"ppid > 0: true",
			},
		},
		{
			Name: "process_execPath",
			Script: `
				const process = require("process");
				console.println("execPath type:", typeof process.execPath);
				console.println("has execPath:", process.execPath.length > 0);
			`,
			Output: []string{
				"execPath type: string",
				"has execPath: true",
			},
		},
		{
			Name: "process_title",
			Script: `
				const process = require("process");
				console.println("title:", process.title);
			`,
			Output: []string{
				"title: process_title",
			},
		},
		{
			Name: "process_versions_details",
			Script: `
				const process = require("process");
				console.println("jsh version:", process.versions.jsh);
				console.println("go version type:", typeof process.versions.go);
			`,
			Output: []string{
				"jsh version: 1.0.0",
				"go version type: string",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessDumpStack(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "dumpStack",
			Script: `
				const process = require("process");
				function testFunc() {
					process.dumpStack(5);
					console.println("stack dumped");
				}
				testFunc();
			`,
			Output: []string{
				"stack dumped",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessHrtimeEdgeCases(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "hrtime_with_string",
			Script: `
				const process = require("process");
				const time = process.hrtime("invalid");
				console.println("is array:", Array.isArray(time));
				console.println("length:", time.length);
			`,
			Output: []string{
				"is array: true",
				"length: 2",
			},
		},
		{
			Name: "hrtime_with_empty_array",
			Script: `
				const process = require("process");
				const time = process.hrtime([]);
				console.println("is array:", Array.isArray(time));
				console.println("length:", time.length);
			`,
			Output: []string{
				"is array: true",
				"length: 2",
			},
		},
		{
			Name: "hrtime_with_single_element_array",
			Script: `
				const process = require("process");
				const time = process.hrtime([123]);
				console.println("is array:", Array.isArray(time));
				console.println("length:", time.length);
			`,
			Output: []string{
				"is array: true",
				"length: 2",
			},
		},
		{
			Name: "hrtime_with_invalid_types_in_array",
			Script: `
				const process = require("process");
				const time = process.hrtime(["string", {}]);
				console.println("is array:", Array.isArray(time));
				console.println("length:", time.length);
			`,
			Output: []string{
				"is array: true",
				"length: 2",
			},
		},
		{
			Name: "hrtime_with_mixed_valid_types",
			Script: `
				const process = require("process");
				const start = process.hrtime();
				// Use integers instead of floats
				const time = process.hrtime([Math.floor(start[0]), Math.floor(start[1])]);
				console.println("is array:", Array.isArray(time));
				console.println("has non-negative values:", time[0] >= 0 && time[1] >= 0);
			`,
			Output: []string{
				"is array: true",
				"has non-negative values: true",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}
