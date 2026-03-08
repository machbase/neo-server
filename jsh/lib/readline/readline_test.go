package readline_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
	"github.com/nyaosorg/go-readline-ny/keys"
)

func TestReadLineModule(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "module",
			Script: `
				const rl = require('readline');
				console.println("MODULE:", typeof rl.ReadLine);
			`,
			Output: []string{
				"MODULE: function",
			},
		},
		{
			Name: "constructor-no-args",
			Script: `
				const {ReadLine} = require('readline');
				const r = new ReadLine();
				console.printf("RL: %X\n", ReadLine.CtrlJ);
			`,
			Output: []string{
				"RL: 0A",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestReadLine(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "readline-simple",
			Script: `
			try{
				const {env} = require('process');
				const {ReadLine} = require('readline');
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
			Vars: map[string]any{
				"auto_input": []string{
					"Hello World", keys.Enter,
				},
			},
			Output: []string{
				"PS: prompt> ",
				"OK: Hello World",
			},
		},
	}
	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestReadLineSubmitOnEnterWhen(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "readline-submit-on-enter-when",
			Script: `
			try{
				const process = require('process');
				const {ReadLine} = require('readline');
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
			Vars: map[string]any{
				"auto_input": []string{
					"Submit by", keys.Enter,
					"semi-colon;", keys.Enter,
				},
			},
			Output: []string{
				"OK: Submit by",
				"semi-colon;",
			},
		},
	}
	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestReadLineCancel(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "readline-cancel",
			Script: `
			try{
				const process = require('process');
				const {ReadLine} = require('readline');
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
			Vars: map[string]any{
				"auto_input": []string{
					"Hello World", // <-- wait timeout with no Enter
				},
			},
			Output: []string{
				"ERR: EOF",
			},
		},
	}
	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}
