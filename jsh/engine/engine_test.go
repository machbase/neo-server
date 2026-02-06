package engine

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/dop251/goja"
)

type TestCase struct {
	name     string
	script   string
	input    []string
	output   []string
	preTest  func(*JSRuntime)
	postTest func(*JSRuntime)
}

func RunTest(t *testing.T, tc TestCase) {
	t.Helper()
	t.Run(tc.name, func(t *testing.T) {
		t.Helper()
		tmpDir := t.TempDir()
		conf := Config{
			Name: tc.name,
			Code: tc.script,
			FSTabs: []FSTab{
				{MountPoint: "/", Source: "../root/embed/"},
				{MountPoint: "/work", Source: "../test/"},
				{MountPoint: "/tmp", Source: tmpDir},
			},
			Env: map[string]any{
				"PATH":         "/work:/sbin",
				"PWD":          "/work",
				"HOME":         "/work",
				"LIBRARY_PATH": "./node_modules:/lib",
			},
			Reader:      &bytes.Buffer{},
			Writer:      &bytes.Buffer{},
			ExecBuilder: testExecBuilder,
		}
		jr, err := New(conf)
		if err != nil {
			t.Fatalf("Failed to create JSRuntime: %v", err)
		}
		jr.RegisterNativeModule("@jsh/process", jr.Process)
		jr.RegisterNativeModule("@jsh/fs", jr.Filesystem)
		conf.Reader.(*bytes.Buffer).WriteString(strings.Join(tc.input, "\n") + "\n")

		if tc.preTest != nil {
			tc.preTest(jr)
		}
		if err := jr.Run(); err != nil {
			if jsErr, ok := err.(*goja.Exception); ok {
				t.Fatalf("Unexpected error: %v", jsErr.String())
			} else {
				t.Fatalf("Unexpected error: %v", err)
			}
		}
		if tc.postTest != nil {
			tc.postTest(jr)
		}

		gotOutput := conf.Writer.(*bytes.Buffer).String()
		lines := strings.Split(gotOutput, "\n")
		if len(lines) != len(tc.output)+1 { // +1 for trailing newline
			t.Fatalf("Expected %d output lines, got %d\n%s", len(tc.output), len(lines)-1, gotOutput)
		}
		for i, expectedLine := range tc.output {
			if lines[i] != expectedLine {
				t.Errorf("Output line %d: expected %q, got %q", i, expectedLine, lines[i])
			}
		}
	})
}

var testExecBuilder ExecBuilderFunc

func TestMain(m *testing.M) {
	args := []string{"build", "-o"}
	if runtime.GOOS == "windows" {
		args = append(args, "../tmp/jsh.exe")
	} else {
		args = append(args, "../tmp/jsh")
	}
	args = append(args, "..")
	cmd := exec.Command("go", args...)
	if err := cmd.Run(); err != nil {
		fmt.Println("Failed to build jsh binary for tests:", err)
		os.Exit(2)
	}
	testExecBuilder = func(source string, args []string, env map[string]any) (*exec.Cmd, error) {
		bin := "../tmp/jsh"
		if runtime.GOOS == "windows" {
			bin = "../tmp/jsh.exe"
		}
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
		return exec.Command(bin, args...), nil
	}
	os.Exit(m.Run())
}

func TestEngine(t *testing.T) {
	ts := []TestCase{
		{
			name:   "console_log",
			script: `console.log("Hello, World!");`,
			output: []string{"INFO  Hello, World!"},
		},
		{
			name: "module_demo",
			script: `
				const { sayHello } = require("demo");
				sayHello("");
			`,
			output: []string{
				"Hello  from demo.js!",
			},
		},
		{
			name: "module_package_json",
			script: `
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
			output: []string{
				"Package help",
			},
		},
	}

	for _, tc := range ts {
		RunTest(t, tc)
	}
}

func TestSetTimeout(t *testing.T) {
	tests := []TestCase{
		{
			name: "setTimeout_basic",
			script: `
				const {now} = require("/lib/process");
				let t = now();
				setTimeout(() => {
					console.log("Timeout executed");
					testDone();
				}, 100);
			`,
			output: []string{
				"INFO  Timeout executed",
			},
		},
		{
			name: "setTimeout_args",
			script: `
				var arg1, arg2;
				setTimeout((a, b) => {
					console.println("Timeout with args:", a, b);
					arg1 = a;
					arg2 = b;
					testDone();
				}, 50,  "test", 42);
			`,
			output: []string{
				"Timeout with args: test 42",
			},
		},
		{
			name: "clearTimeout_basic",
			script: `
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
			output: []string{
				"count: 1 , sum: 1",
				"count: 2 , sum: 3",
				"count: 3 , sum: 6",
			},
		},
		{
			name: "clearTimeout_twice",
			script: `
				var executed = false;
				var tm = setTimeout(()=>{ executed = true; testDone(); }, 50);
				clearTimeout(tm);
				clearTimeout(tm);
				setTimeout(()=>{ testDone(); }, 50); // Ensure test completes
				`,
			output: []string{
				// No output expected regarding execution
			},
		},
	}
	for _, tc := range tests {
		RunTest(t, tc)
	}
}

// TestShutdownHook tests have been moved to process_test.go

func TestEventLoop(t *testing.T) {
	testCases := []TestCase{
		{
			name: "eventloop",
			script: `
				console.log("Add event loop");
				setImmediate(() => {
					console.debug("event loop called");
				});
			`,
			output: []string{
				"INFO  Add event loop",
				"DEBUG event loop called",
			},
		},
		{
			// the problem is the nested runOnLoop can not append to the loop
			// while loop is running with mutex lock of the job queue.
			name: "eventloop_loop",
			script: `
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
			output: []string{
				"Add event loop",
				"Starting doIt",
				"Timeout before doIt",
				"event loop called from #1",
				"event loop called from #2",
			},
		},
		{
			name: "eventloop_promise",
			script: `
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
			output: []string{
				"Add event loop",
				"event loop called from promise",
				"Promise resolved",
				"Event loop after promise resolved",
			},
		},
	}
	for _, tc := range testCases {
		RunTest(t, tc)
	}
}

// TestExec tests have been moved to process_test.go

func TestUtilSplitFields(t *testing.T) {
	testCases := []TestCase{
		{
			name: "util_splitFields_basic",
			script: `
				const {splitFields} = require("/lib/util");
				const result = splitFields("  foo   bar baz  ");
				console.println("Fields:", JSON.stringify(result));
			`,
			output: []string{
				"Fields: [\"foo\",\"bar\",\"baz\"]",
			},
		},
		{
			name: "util_splitFields_double_quotes",
			script: `
				const {splitFields} = require("/lib/util");
				const result = splitFields('hello "world foo" bar');
				console.println("Fields:", JSON.stringify(result));
			`,
			output: []string{
				"Fields: [\"hello\",\"world foo\",\"bar\"]",
			},
		},
		{
			name: "util_splitFields_single_quotes",
			script: `
				const {splitFields} = require("/lib/util");
				const result = splitFields("hello 'world foo' bar");
				console.println("Fields:", JSON.stringify(result));
			`,
			output: []string{
				"Fields: [\"hello\",\"world foo\",\"bar\"]",
			},
		},
		{
			name: "util_splitFields_mixed_quotes",
			script: `
				const {splitFields} = require("/lib/util");
				const result = splitFields("a \"b c\" d 'e f' g");
				console.println("Fields:", JSON.stringify(result));
			`,
			output: []string{
				"Fields: [\"a\",\"b c\",\"d\",\"e f\",\"g\"]",
			},
		},
		{
			name: "util_splitFields_empty_string",
			script: `
				const {splitFields} = require("/lib/util");
				const result = splitFields("");
				console.println("Fields:", JSON.stringify(result));
			`,
			output: []string{
				"Fields: []",
			},
		},
		{
			name: "util_splitFields_only_whitespace",
			script: `
				const {splitFields} = require("/lib/util");
				const result = splitFields("   \t  \n  ");
				console.println("Fields:", JSON.stringify(result));
			`,
			output: []string{
				"Fields: []",
			},
		},
		{
			name: "util_splitFields_tabs_and_newlines",
			script: `
				const {splitFields} = require("/lib/util");
				const result = splitFields("foo\tbar\nbaz");
				console.println("Fields:", JSON.stringify(result));
			`,
			output: []string{
				"Fields: [\"foo\",\"bar\",\"baz\"]",
			},
		},
		{
			name: "util_splitFields_quoted_with_tabs",
			script: `
				const {splitFields} = require("/lib/util");
				const result = splitFields('a "b\tc" d');
				console.println("Fields:", JSON.stringify(result));
			`,
			output: []string{
				"Fields: [\"a\",\"b\\tc\",\"d\"]",
			},
		},
		{
			name: "util_splitFields_multiple_quoted",
			script: `
				const {splitFields} = require("/lib/util");
				const result = splitFields('cmd "arg 1" "arg 2" "arg 3"');
				console.println("Fields:", JSON.stringify(result));
			`,
			output: []string{
				"Fields: [\"cmd\",\"arg 1\",\"arg 2\",\"arg 3\"]",
			},
		},
		{
			name: "util_splitFields_no_spaces",
			script: `
				const {splitFields} = require("/lib/util");
				const result = splitFields("hello");
				console.println("Fields:", JSON.stringify(result));
			`,
			output: []string{
				"Fields: [\"hello\"]",
			},
		},
	}
	for _, tc := range testCases {
		RunTest(t, tc)
	}
}

func TestUtilParseArgs(t *testing.T) {
	testCases := []TestCase{
		{
			name: "util_parseArgs_basic",
			script: `
				const {parseArgs} = require("/lib/util");
				const result = parseArgs(['-f', '--bar', 'value', 'positional'], {
					options: {
						foo: { type: 'boolean', short: 'f' },
						bar: { type: 'string' }
					},
					allowPositionals: true
				});
				console.println("Values:", JSON.stringify(result.values));
				console.println("Positionals:", JSON.stringify(result.positionals));
			`,
			output: []string{
				"Values: {\"foo\":true,\"bar\":\"value\"}",
				"Positionals: [\"positional\"]",
			},
		},
		{
			name: "util_parseArgs_long_options",
			script: `
				const {parseArgs} = require("/lib/util");
				const result = parseArgs(['--verbose', '--output', 'file.txt'], {
					options: {
						verbose: { type: 'boolean' },
						output: { type: 'string' }
					}
				});
				console.println("Values:", JSON.stringify(result.values));
			`,
			output: []string{
				"Values: {\"verbose\":true,\"output\":\"file.txt\"}",
			},
		},
		{
			name: "util_parseArgs_short_options",
			script: `
				const {parseArgs} = require("/lib/util");
				const result = parseArgs(['-v', '-o', 'out.txt'], {
					options: {
						verbose: { type: 'boolean', short: 'v' },
						output: { type: 'string', short: 'o' }
					}
				});
				console.println("Values:", JSON.stringify(result.values));
			`,
			output: []string{
				"Values: {\"verbose\":true,\"output\":\"out.txt\"}",
			},
		},
		{
			name: "util_parseArgs_inline_value",
			script: `
				const {parseArgs} = require("/lib/util");
				const result = parseArgs(['--output=file.txt', '-o=out.txt'], {
					options: {
						output: { type: 'string', short: 'o' }
					}
				});
				console.println("Values:", JSON.stringify(result.values));
			`,
			output: []string{
				"Values: {\"output\":\"out.txt\"}",
			},
		},
		{
			name: "util_parseArgs_multiple",
			script: `
				const {parseArgs} = require("/lib/util");
				const result = parseArgs(['--include', 'a.js', '--include', 'b.js', '-I', 'c.js'], {
					options: {
						include: { type: 'string', short: 'I', multiple: true }
					}
				});
				console.println("Values:", JSON.stringify(result.values));
			`,
			output: []string{
				"Values: {\"include\":[\"a.js\",\"b.js\",\"c.js\"]}",
			},
		},
		{
			name: "util_parseArgs_default_values",
			script: `
				const {parseArgs} = require("/lib/util");
				const result = parseArgs(['--foo'], {
					options: {
						foo: { type: 'boolean' },
						bar: { type: 'string', default: 'default_value' },
						count: { type: 'string', default: '0' }
					}
				});
				console.println("Values:", JSON.stringify(result.values));
			`,
			output: []string{
				"Values: {\"bar\":\"default_value\",\"count\":\"0\",\"foo\":true}",
			},
		},
		{
			name: "util_parseArgs_short_group",
			script: `
				const {parseArgs} = require("/lib/util");
				const result = parseArgs(['-abc'], {
					options: {
						a: { type: 'boolean', short: 'a' },
						b: { type: 'boolean', short: 'b' },
						c: { type: 'boolean', short: 'c' }
					}
				});
				console.println("Values:", JSON.stringify(result.values));
			`,
			output: []string{
				"Values: {\"a\":true,\"b\":true,\"c\":true}",
			},
		},
		{
			name: "util_parseArgs_terminator",
			script: `
				const {parseArgs} = require("/lib/util");
				const result = parseArgs(['--foo', '--', '--bar', 'baz'], {
					options: {
						foo: { type: 'boolean' },
						bar: { type: 'boolean' }
					},
					allowPositionals: true
				});
				console.println("Values:", JSON.stringify(result.values));
				console.println("Positionals:", JSON.stringify(result.positionals));
			`,
			output: []string{
				"Values: {\"foo\":true}",
				"Positionals: [\"--bar\",\"baz\"]",
			},
		},
		{
			name: "util_parseArgs_allow_negative",
			script: `
				const {parseArgs} = require("/lib/util");
				const result = parseArgs(['--no-color', '--verbose'], {
					options: {
						color: { type: 'boolean' },
						verbose: { type: 'boolean' }
					},
					allowNegative: true
				});
				console.println("Values:", JSON.stringify(result.values));
			`,
			output: []string{
				"Values: {\"color\":false,\"verbose\":true}",
			},
		},
		{
			name: "util_parseArgs_tokens",
			script: `
				const {parseArgs} = require("/lib/util");
				const result = parseArgs(['-f', '--bar', 'value'], {
					options: {
						foo: { type: 'boolean', short: 'f' },
						bar: { type: 'string' }
					},
					tokens: true
				});
				console.println("Token count:", result.tokens.length);
				console.println("First token kind:", result.tokens[0].kind);
				console.println("First token name:", result.tokens[0].name);
			`,
			output: []string{
				"Token count: 2",
				"First token kind: option",
				"First token name: foo",
			},
		},
		{
			name: "util_parseArgs_old_signature",
			script: `
				const {parseArgs} = require("/lib/util");
				const result = parseArgs(['-v', '--output', 'file.txt'], {
					options: {
						verbose: { type: 'boolean', short: 'v' },
						output: { type: 'string' }
					}
				});
				console.println("Values:", JSON.stringify(result.values));
			`,
			output: []string{
				"Values: {\"verbose\":true,\"output\":\"file.txt\"}",
			},
		},
		{
			name: "util_parseArgs_named_positionals",
			script: `
				const {parseArgs} = require("/lib/util");
				const result = parseArgs(['input.txt', 'output.txt'], {
					options: {},
					allowPositionals: true,
					positionals: ['inputFile', 'outputFile']
				});
				console.println("Positionals:", JSON.stringify(result.positionals));
				console.println("Named:", JSON.stringify(result.namedPositionals));
			`,
			output: []string{
				"Positionals: [\"input.txt\",\"output.txt\"]",
				"Named: {\"inputFile\":\"input.txt\",\"outputFile\":\"output.txt\"}",
			},
		},
		{
			name: "util_parseArgs_optional_positionals",
			script: `
				const {parseArgs} = require("/lib/util");
				const result = parseArgs(['input.txt'], {
					options: {},
					allowPositionals: true,
					positionals: [
						'inputFile',
						{ name: 'outputFile', optional: true, default: 'stdout' }
					]
				});
				console.println("Named:", JSON.stringify(result.namedPositionals));
			`,
			output: []string{
				"Named: {\"inputFile\":\"input.txt\",\"outputFile\":\"stdout\"}",
			},
		},
		{
			name: "util_parseArgs_variadic_positionals",
			script: `
				const {parseArgs} = require("/lib/util");
				const result = parseArgs(['input.txt', 'out.txt', 'a.js', 'b.js'], {
					options: {},
					allowPositionals: true,
					positionals: [
						'inputFile',
						'outputFile',
						{ name: 'files', variadic: true }
					]
				});
				console.println("Named:", JSON.stringify(result.namedPositionals));
			`,
			output: []string{
				"Named: {\"inputFile\":\"input.txt\",\"outputFile\":\"out.txt\",\"files\":[\"a.js\",\"b.js\"]}",
			},
		},
	}
	for _, tc := range testCases {
		RunTest(t, tc)
	}
}
