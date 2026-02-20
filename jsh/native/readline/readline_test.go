package readline

import (
	"bytes"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/root"
	"github.com/nyaosorg/go-readline-ny/keys"
)

type TestCase struct {
	name   string
	script string
	input  []string
	output []string
	err    string
	vars   map[string]any
}

func RunTest(t *testing.T, tc TestCase) {
	t.Helper()
	t.Run(tc.name, func(t *testing.T) {
		t.Helper()
		conf := engine.Config{
			Name:   tc.name,
			Code:   tc.script,
			FSTabs: []engine.FSTab{root.RootFSTab(), {MountPoint: "/work", Source: "../../test/"}},
			Env:    tc.vars,
			Reader: &bytes.Buffer{},
			Writer: &bytes.Buffer{},
		}
		jr, err := engine.New(conf)
		if err != nil {
			t.Fatalf("Failed to create JSRuntime: %v", err)
		}
		jr.RegisterNativeModule("@jsh/process", jr.Process)
		jr.RegisterNativeModule("@jsh/readline", Module)

		if len(tc.input) > 0 {
			conf.Reader.(*bytes.Buffer).WriteString(strings.Join(tc.input, ""))
		}
		if err := jr.Run(); err != nil {
			if tc.err == "" || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("Unexpected error: %v", err)
			}
			return
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

func TestReadLineModule(t *testing.T) {
	tests := []TestCase{
		{
			name: "module",
			script: `
				const rl = require('/lib/readline');
				console.println("MODULE:", typeof rl.ReadLine);
			`,
			output: []string{
				"MODULE: function",
			},
		},
		{
			name: "constructor-no-args",
			script: `
				const {ReadLine} = require('/lib/readline');
				const r = new ReadLine();
				console.printf("RL: %X\n", ReadLine.CtrlJ);
			`,
			output: []string{
				"RL: 0A",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestReadLine(t *testing.T) {
	tests := []TestCase{
		{
			name: "readline-simple",
			script: `
			try{
				const {env} = require('/lib/process');
				const {ReadLine} = require('/lib/readline');
				const r = new ReadLine({
					prompt: (lineno) => { return "prompt> "},
					autoInput: env.get("auto_input"),
				});
				console.println("PS:", r.options.prompt(0));
				const line1 = r.readLine();
				if (line1 instanceof Error) {
					throw line1;
				}
				console.println("OK:", line1);
				r.addHistory(line1);
			} catch(e) {
				console.println("ERR:", e.message);
			}
			`,
			vars: map[string]any{
				"auto_input": []string{
					"Hello World", keys.Enter,
				},
			},
			output: []string{
				"PS: prompt> ",
				"OK: Hello World",
			},
		},
	}
	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestReadLineSubmitOnEnterWhen(t *testing.T) {
	tests := []TestCase{
		{
			name: "readline-submit-on-enter-when",
			script: `
			try{
				const process = require('/lib/process');
				const {ReadLine} = require('/lib/readline');
				const r = new ReadLine({
					autoInput: process.env.get("auto_input"),
					submitOnEnterWhen: (lines, idx) => {
						return lines[idx].endsWith(";");
					},
				});
				const line = r.readLine();
				if (line instanceof Error) {
					throw line;
				}
				console.println("OK:", line);
			} catch(e) {
				console.println("ERR:", e.message);
			}
			`,
			vars: map[string]any{
				"auto_input": []string{
					"Submit by", keys.Enter,
					"semi-colon;", keys.Enter,
				},
			},
			output: []string{
				"OK: Submit by",
				"semi-colon;",
			},
		},
	}
	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestReadLineCancel(t *testing.T) {
	tests := []TestCase{
		{
			name: "readline-cancel",
			script: `
			try{
				const process = require('/lib/process');
				const {ReadLine} = require('/lib/readline');
				const r = new ReadLine({
					autoInput: process.env.get("auto_input"),
				});
				const to = setTimeout(()=>{ r.close() }, 200);
				const line = r.readLine();
				console.println("OK:", line);
				clearTimeout(to);
		    } catch(e) {
				console.println("ERR:", e.message);
			}
			`,
			vars: map[string]any{
				"auto_input": []string{
					"Hello World", // <-- wait timeout with no Enter
				},
			},
			output: []string{
				"ERR: EOF",
			},
		},
	}
	for _, tc := range tests {
		RunTest(t, tc)
	}
}
