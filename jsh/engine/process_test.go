package engine

import (
	"fmt"
	"runtime"
	"testing"
	"time"
)

func TestProcess(t *testing.T) {
	tests := []TestCase{
		{
			name: "process_env",
			script: `
				const process = require("/lib/process");
				console.println("PATH:", process.env.get("PATH"));
				console.println("PWD:", process.env.get("PWD"));
				console.println("LIBRARY_PATH:", process.env.get("LIBRARY_PATH"));
			`,
			output: []string{
				"PATH: /work:/sbin",
				"PWD: /work",
				"LIBRARY_PATH: ./node_modules:/lib",
			},
		},
		{
			name: "process_expand",
			script: `
				const process = require("/lib/process");
				const expanded1 = process.expand("$HOME/file.txt");
				const expanded2 = process.expand("$HOME/../lib/file.txt");
				console.println("expanded1:", expanded1);
				console.println("expanded2:", expanded2);
			`,
			output: []string{
				"expanded1: /work/file.txt",
				"expanded2: /work/../lib/file.txt",
			},
		},
		{
			name: "process_argv",
			script: `
				const process = require("/lib/process");
				console.println("argc:", process.argv.length);
				console.println("argv[1]:", process.argv[1]);
			`,
			output: []string{
				"argc: 2",
				"argv[1]: process_argv",
			},
		},
		{
			name: "process_cwd",
			script: `
				const process = require("/lib/process");
				console.println("cwd:", process.cwd());
			`,
			output: []string{
				"cwd: /work",
			},
		},
		{
			name: "process_chdir",
			script: `
				const process = require("/lib/process");
				console.println("before:", process.cwd());
				process.chdir("/lib");
				console.println("after:", process.cwd());
			`,
			output: []string{
				"before: /work",
				"after: /lib",
			},
		},
		{
			name: "process_chdir_relative",
			script: `
				const process = require("/lib/process");
				console.println("before:", process.cwd());
				process.chdir("../lib");
				console.println("after:", process.cwd());
			`,
			output: []string{
				"before: /work",
				"after: /lib",
			},
		},

		{
			name: "process_now",
			script: `
				const process = require("/lib/process");
				const now = process.now();
				console.println("type:", typeof now);
			`,
			preTest:  func(jr *JSRuntime) { jr.nowFunc = func() time.Time { return time.Unix(1764728536, 0) } },
			postTest: func(jr *JSRuntime) { jr.nowFunc = time.Now },
			output: []string{
				"type: object",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestProcessStdin(t *testing.T) {
	tests := []TestCase{
		{
			name: "stdin_readLines",
			script: `
				const process = require("/lib/process");
				const lines = process.stdin.readLines();
				console.println("lines:", lines.length);
				lines.forEach((line, i) => {
					console.println("line", i + ":", line);
				});
			`,
			input: []string{"first line", "second line", "third line"},
			output: []string{
				"lines: 3",
				"line 0: first line",
				"line 1: second line",
				"line 2: third line",
			},
		},
		{
			name: "stdin_readLine",
			script: `
				const process = require("/lib/process");
				const line = process.stdin.readLine();
				console.println("got:", line);
			`,
			input: []string{"hello world"},
			output: []string{
				"got: hello world",
			},
		},
		{
			name: "stdin_read",
			script: `
				const process = require("/lib/process");
				const data = process.stdin.read();
				console.println("length:", data.length);
				const lines = data.split("\n").filter(l => l.length > 0);
				console.println("lines:", lines.length);
			`,
			input: []string{"line1", "line2"},
			output: []string{
				"length: 12",
				"lines: 2",
			},
		},
		{
			name: "stdin_readBytes",
			script: `
				const process = require("/lib/process");
				const data = process.stdin.readBytes(5);
				console.println("read:", data);
				console.println("length:", data.length);
			`,
			input: []string{"hello world"},
			output: []string{
				"read: hello",
				"length: 5",
			},
		},
		{
			name: "stdin_isTTY",
			script: `
				const process = require("/lib/process");
				const isTTY = process.stdin.isTTY();
				console.println("isTTY:", isTTY);
			`,
			input: []string{},
			output: []string{
				"isTTY: false",
			},
		},
		{
			name: "stdin_empty",
			script: `
				const process = require("/lib/process");
				const lines = process.stdin.readLines();
				const nonEmpty = lines.filter(l => l.length > 0);
				console.println("non-empty lines:", nonEmpty.length);
			`,
			input: []string{},
			output: []string{
				"non-empty lines: 0",
			},
		},
		{
			name: "stdin_process_lines",
			script: `
				const process = require("/lib/process");
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
			input: []string{"10", "20", "30"},
			output: []string{
				"sum: 60",
			},
		},
		{
			name: "stdin_filter_lines",
			script: `
				const process = require("/lib/process");
				const lines = process.stdin.readLines();
				const filtered = lines.filter(line => line.includes("test"));
				console.println("found:", filtered.length);
				filtered.forEach(line => console.println(line));
			`,
			input: []string{"test1", "something", "test2", "other", "testing"},
			output: []string{
				"found: 3",
				"test1",
				"test2",
				"testing",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestProcessExec(t *testing.T) {
	tests := []TestCase{
		{
			name: "exec_basic",
			script: `
				const process = require("process");
				const path = process.which('echo');
				const exitCode = process.exec(path, "hello from exec");
				console.println("exit code:", exitCode);
			`,
			output: []string{
				"hello from exec",
				"exit code: 0",
			},
		},
		{
			name: "execString_basic",
			script: `
				const process = require("process");
				const exitCode = process.execString("console.println('hello from execString')");
				console.println("exit code:", exitCode);
			`,
			output: []string{
				"hello from execString",
				"exit code: 0",
			},
		},
		{
			name: "exec_with_args",
			script: `
				const process = require("process");
				const path = process.which('echo');
				const exitCode = process.exec(path, "arg1", "arg2", "arg3");
				console.println("done");
			`,
			output: []string{
				"arg1 arg2 arg3",
				"done",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestProcessShutdownHook(t *testing.T) {
	tests := []TestCase{
		{
			name: "shutdown_hook_single",
			script: `
				const process = require("/lib/process");
				process.addShutdownHook(() => {
					console.println("cleanup");
				});
				console.println("main");
			`,
			output: []string{
				"main",
				"cleanup",
			},
		},
		{
			name: "shutdown_hook_multiple",
			script: `
				const process = require("/lib/process");
				process.addShutdownHook(() => {
					console.println("first hook");
				});
				process.addShutdownHook(() => {
					console.println("second hook");
				});
				console.println("main");
			`,
			output: []string{
				"main",
				"second hook",
				"first hook",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestProcessInfo(t *testing.T) {
	tests := []TestCase{
		{
			name: "process_pid",
			script: `
				const process = require("/lib/process");
				console.println("pid type:", typeof process.pid);
				console.println("pid > 0:", process.pid > 0);
			`,
			output: []string{
				"pid type: number",
				"pid > 0: true",
			},
		},
		{
			name: "process_platform_arch",
			script: `
				const process = require("/lib/process");
				console.println("platform:", process.platform);
				console.println("arch:", process.arch);
			`,
			output: []string{
				fmt.Sprintf("platform: %s", runtime.GOOS),
				fmt.Sprintf("arch: %s", runtime.GOARCH),
			},
		},
		{
			name: "process_version",
			script: `
				const process = require("/lib/process");
				console.println("version:", process.version);
				console.println("has versions:", typeof process.versions);
			`,
			output: []string{
				"version: jsh-1.0.0",
				"has versions: object",
			},
		},
		{
			name: "process_stdout",
			script: `
				const process = require("/lib/process");
				process.stdout.write("Hello from stdout\n");
				console.println("stdout written");
			`,
			output: []string{
				"Hello from stdout",
				"stdout written",
			},
		},
		{
			name: "process_nextTick",
			script: `
				const process = require("/lib/process");
				console.println("before nextTick");
				process.nextTick(() => {
					console.println("in nextTick");
				});
				console.println("after nextTick");
			`,
			output: []string{
				"before nextTick",
				"after nextTick",
				"in nextTick",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestProcessResources(t *testing.T) {
	tests := []TestCase{
		{
			name: "process_memoryUsage",
			script: `
				const process = require("/lib/process");
				const mem = process.memoryUsage();
				console.println("has rss:", typeof mem.rss);
				console.println("has heapTotal:", typeof mem.heapTotal);
				console.println("has heapUsed:", typeof mem.heapUsed);
			`,
			output: []string{
				"has rss: number",
				"has heapTotal: number",
				"has heapUsed: number",
			},
		},
		{
			name: "process_cpuUsage",
			script: `
				const process = require("/lib/process");
				const cpu = process.cpuUsage();
				console.println("has user:", typeof cpu.user);
				console.println("has system:", typeof cpu.system);
			`,
			output: []string{
				"has user: number",
				"has system: number",
			},
		},
		{
			name: "process_uptime",
			script: `
				const process = require("/lib/process");
				const uptime = process.uptime();
				console.println("uptime type:", typeof uptime);
				console.println("uptime >= 0:", uptime >= 0);
			`,
			output: []string{
				"uptime type: number",
				"uptime >= 0: true",
			},
		},
		{
			name: "process_hrtime",
			script: `
				const process = require("/lib/process");
				const time = process.hrtime();
				console.println("is array:", Array.isArray(time));
				console.println("length:", time.length);
			`,
			output: []string{
				"is array: true",
				"length: 2",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestProcessEvents(t *testing.T) {
	tests := []TestCase{
		{
			name: "process_event_emitter",
			script: `
				const process = require("/lib/process");
				console.println("has on:", typeof process.on);
				console.println("has emit:", typeof process.emit);
				console.println("has removeListener:", typeof process.removeListener);
			`,
			output: []string{
				"has on: function",
				"has emit: function",
				"has removeListener: function",
			},
		},
		{
			name: "process_custom_event",
			script: `
				const process = require("/lib/process");
				process.on('test', (msg) => {
					console.println("received:", msg);
				});
				process.emit('test', 'hello');
			`,
			output: []string{
				"received: hello",
			},
		},
		{
			name: "process_multiple_listeners",
			script: `
				const process = require("/lib/process");
				process.on('test', () => console.println("listener 1"));
				process.on('test', () => console.println("listener 2"));
				process.emit('test');
			`,
			output: []string{
				"listener 1",
				"listener 2",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestProcessStderr(t *testing.T) {
	tests := []TestCase{
		{
			name: "stderr_write",
			script: `
				const process = require("/lib/process");
				const result = process.stderr.write("error message\n");
				console.println("write success:", result);
			`,
			output: []string{
				"write success: true",
			},
		},
		{
			name: "stderr_write_empty",
			script: `
				const process = require("/lib/process");
				const result = process.stderr.write("");
				console.println("write empty:", result);
			`,
			output: []string{
				"write empty: true",
			},
		},
		{
			name: "stderr_isTTY",
			script: `
				const process = require("/lib/process");
				const isTTY = process.stderr.isTTY();
				console.println("isTTY type:", typeof isTTY);
			`,
			output: []string{
				"isTTY type: boolean",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestProcessStdout(t *testing.T) {
	tests := []TestCase{
		{
			name: "stdout_write_empty",
			script: `
				const process = require("/lib/process");
				const result = process.stdout.write("");
				console.println("empty write:", result);
			`,
			output: []string{
				"empty write: true",
			},
		},
		{
			name: "stdout_isTTY",
			script: `
				const process = require("/lib/process");
				const isTTY = process.stdout.isTTY();
				console.println("isTTY type:", typeof isTTY);
			`,
			output: []string{
				"isTTY type: boolean",
			},
		},
		{
			name: "stdout_write_multiple",
			script: `
				const process = require("/lib/process");
				process.stdout.write("first\n");
				process.stdout.write("second\n");
				console.println("done");
			`,
			output: []string{
				"first",
				"second",
				"done",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestProcessStdinErrors(t *testing.T) {
	tests := []TestCase{
		{
			name: "stdin_readBytes_no_args",
			script: `
				const process = require("/lib/process");
				const result = process.stdin.readBytes();
				if (result instanceof Error) {
					console.println("error:", result.message.includes("requires a number"));
				} else {
					console.println("no error, got:", typeof result);
				}
			`,
			input: []string{"test"},
			output: []string{
				"error: true",
			},
		},
		{
			name: "stdin_readBytes_negative",
			script: `
				const process = require("/lib/process");
				const result = process.stdin.readBytes(-1);
				if (result instanceof Error) {
					console.println("error:", result.message.includes("positive number"));
				} else {
					console.println("no error, got:", typeof result);
				}
			`,
			input: []string{"test"},
			output: []string{
				"error: true",
			},
		},
		{
			name: "stdin_readBytes_zero",
			script: `
				const process = require("/lib/process");
				const result = process.stdin.readBytes(0);
				if (result instanceof Error) {
					console.println("error:", result.message.includes("positive number"));
				} else {
					console.println("no error, got:", typeof result);
				}
			`,
			input: []string{"test"},
			output: []string{
				"error: true",
			},
		},
		{
			name: "stdin_readBytes_more_than_available",
			script: `
				const process = require("/lib/process");
				const data = process.stdin.readBytes(100);
				console.println("read length:", data.length);
			`,
			input: []string{"short"},
			output: []string{
				"read length: 6",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestProcessHrtime(t *testing.T) {
	tests := []TestCase{
		{
			name: "hrtime_basic",
			script: `
				const process = require("/lib/process");
				const time1 = process.hrtime();
				console.println("is array:", Array.isArray(time1));
				console.println("length:", time1.length);
				console.println("has seconds:", typeof time1[0]);
				console.println("has nanos:", typeof time1[1]);
			`,
			output: []string{
				"is array: true",
				"length: 2",
				"has seconds: number",
				"has nanos: number",
			},
		},
		{
			name: "hrtime_diff",
			script: `
				const process = require("/lib/process");
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
			output: []string{
				"start type: true",
				"diff is array: true",
				"diff length: 2",
				"has elapsed: true",
			},
		},
		{
			name: "hrtime_with_invalid_arg",
			script: `
				const process = require("/lib/process");
				const time = process.hrtime("invalid");
				console.println("is array:", Array.isArray(time));
				console.println("length:", time.length);
			`,
			output: []string{
				"is array: true",
				"length: 2",
			},
		},
		{
			name: "hrtime_with_empty_array",
			script: `
				const process = require("/lib/process");
				const time = process.hrtime([]);
				console.println("is array:", Array.isArray(time));
				console.println("length:", time.length);
			`,
			output: []string{
				"is array: true",
				"length: 2",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestProcessKill(t *testing.T) {
	tests := []TestCase{
		{
			name: "kill_no_args",
			script: `
				const process = require("/lib/process");
				const result = process.kill();
				if (result instanceof Error) {
					console.println("error:", result.message.includes("requires a pid"));
				} else {
					console.println("result:", result);
				}
			`,
			output: []string{
				"error: true",
			},
		},
		{
			name: "kill_with_pid",
			script: `
				const process = require("/lib/process");
				const result = process.kill(12345);
				console.println("kill result:", result);
			`,
			output: []string{
				"kill result: true",
			},
		},
		{
			name: "kill_with_signal",
			script: `
				const process = require("/lib/process");
				const result = process.kill(12345, "SIGKILL");
				console.println("kill with signal:", result);
			`,
			output: []string{
				"kill with signal: true",
			},
		},
		{
			name: "kill_with_sigterm",
			script: `
				const process = require("/lib/process");
				const result = process.kill(99999, "SIGTERM");
				console.println("result:", result);
			`,
			output: []string{
				"result: true",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestProcessNextTick(t *testing.T) {
	tests := []TestCase{
		{
			name: "nextTick_with_args",
			script: `
				const process = require("/lib/process");
				process.nextTick((a, b, c) => {
					console.println("args:", a, b, c);
				}, "first", "second", "third");
				console.println("main");
			`,
			output: []string{
				"main",
				"args: first second third",
			},
		},
		{
			name: "nextTick_no_callback",
			script: `
				const process = require("/lib/process");
				const result = process.nextTick();
				console.println("result:", result === undefined ? "undefined" : result);
			`,
			output: []string{
				"result: undefined",
			},
		},
		{
			name: "nextTick_non_function",
			script: `
				const process = require("/lib/process");
				const result = process.nextTick("not a function");
				console.println("result:", result === undefined ? "undefined" : result);
			`,
			output: []string{
				"result: undefined",
			},
		},
		{
			name: "nextTick_multiple",
			script: `
				const process = require("/lib/process");
				process.nextTick(() => console.println("tick 1"));
				process.nextTick(() => console.println("tick 2"));
				process.nextTick(() => console.println("tick 3"));
				console.println("main");
			`,
			output: []string{
				"main",
				"tick 1",
				"tick 2",
				"tick 3",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestProcessChdir(t *testing.T) {
	tests := []TestCase{
		{
			name: "chdir_to_home",
			script: `
				const process = require("/lib/process");
				process.chdir("~");
				console.println("cwd after ~:", process.cwd());
			`,
			output: []string{
				"cwd after ~: /work",
			},
		},
		{
			name: "chdir_empty_string",
			script: `
				const process = require("/lib/process");
				process.chdir("");
				console.println("cwd after empty:", process.cwd());
			`,
			output: []string{
				"cwd after empty: /work",
			},
		},
		{
			name: "chdir_nonexistent",
			script: `
				const process = require("/lib/process");
				try {
					process.chdir("/nonexistent/path");
					console.println("should not reach here");
				} catch (e) {
					console.println("error caught:", e.message.includes("no such file"));
				}
			`,
			output: []string{
				"error caught: true",
			},
		},
		{
			name: "chdir_to_file",
			script: `
				const process = require("/lib/process");
				try {
					process.chdir("/sbin/echo.js");
					console.println("should not reach here");
				} catch (e) {
					console.println("error caught:", e.message.includes("not a directory"));
				}
			`,
			output: []string{
				"error caught: true",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestProcessExecErrors(t *testing.T) {
	tests := []TestCase{
		{
			name: "exec_no_args",
			script: `
				const process = require("/lib/process");
				const result = process.exec();
				if (result instanceof Error) {
					console.println("error:", result.message.includes("no command"));
				} else {
					console.println("result:", result);
				}
			`,
			output: []string{
				"error: true",
			},
		},
		{
			name: "execString_no_args",
			script: `
				const process = require("/lib/process");
				const result = process.execString();
				if (result instanceof Error) {
					console.println("error:", result.message.includes("no source"));
				} else {
					console.println("result:", result);
				}
			`,
			output: []string{
				"error: true",
			},
		},
		{
			name: "execString_with_args",
			script: `
				const process = require("/lib/process");
				const exitCode = process.execString(
					"console.println('sum:', 10 + 20)",
					"10", "20"
				);
				console.println("exit code:", exitCode);
			`,
			output: []string{
				"sum: 30",
				"exit code: 0",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestProcessProperties(t *testing.T) {
	tests := []TestCase{
		{
			name: "process_ppid",
			script: `
				const process = require("/lib/process");
				console.println("ppid type:", typeof process.ppid);
				console.println("ppid > 0:", process.ppid > 0);
			`,
			output: []string{
				"ppid type: number",
				"ppid > 0: true",
			},
		},
		{
			name: "process_execPath",
			script: `
				const process = require("/lib/process");
				console.println("execPath type:", typeof process.execPath);
				console.println("has execPath:", process.execPath.length > 0);
			`,
			output: []string{
				"execPath type: string",
				"has execPath: true",
			},
		},
		{
			name: "process_title",
			script: `
				const process = require("/lib/process");
				console.println("title:", process.title);
			`,
			output: []string{
				"title: process_title",
			},
		},
		{
			name: "process_versions_details",
			script: `
				const process = require("/lib/process");
				console.println("jsh version:", process.versions.jsh);
				console.println("go version type:", typeof process.versions.go);
			`,
			output: []string{
				"jsh version: 1.0.0",
				"go version type: string",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestProcessDumpStack(t *testing.T) {
	tests := []TestCase{
		{
			name: "dumpStack",
			script: `
				const process = require("/lib/process");
				function testFunc() {
					process.dumpStack(5);
					console.println("stack dumped");
				}
				testFunc();
			`,
			output: []string{
				"stack dumped",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestProcessHrtimeEdgeCases(t *testing.T) {
	tests := []TestCase{
		{
			name: "hrtime_with_string",
			script: `
				const process = require("/lib/process");
				const time = process.hrtime("invalid");
				console.println("is array:", Array.isArray(time));
				console.println("length:", time.length);
			`,
			output: []string{
				"is array: true",
				"length: 2",
			},
		},
		{
			name: "hrtime_with_empty_array",
			script: `
				const process = require("/lib/process");
				const time = process.hrtime([]);
				console.println("is array:", Array.isArray(time));
				console.println("length:", time.length);
			`,
			output: []string{
				"is array: true",
				"length: 2",
			},
		},
		{
			name: "hrtime_with_single_element_array",
			script: `
				const process = require("/lib/process");
				const time = process.hrtime([123]);
				console.println("is array:", Array.isArray(time));
				console.println("length:", time.length);
			`,
			output: []string{
				"is array: true",
				"length: 2",
			},
		},
		{
			name: "hrtime_with_invalid_types_in_array",
			script: `
				const process = require("/lib/process");
				const time = process.hrtime(["string", {}]);
				console.println("is array:", Array.isArray(time));
				console.println("length:", time.length);
			`,
			output: []string{
				"is array: true",
				"length: 2",
			},
		},
		{
			name: "hrtime_with_mixed_valid_types",
			script: `
				const process = require("/lib/process");
				const start = process.hrtime();
				// Use integers instead of floats
				const time = process.hrtime([Math.floor(start[0]), Math.floor(start[1])]);
				console.println("is array:", Array.isArray(time));
				console.println("has non-negative values:", time[0] >= 0 && time[1] >= 0);
			`,
			output: []string{
				"is array: true",
				"has non-negative values: true",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}
