package machcli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api/testsuite"
	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/root"
)

type TestCase struct {
	name   string
	script string
	output []string
	err    string
	vars   map[string]any
}

func RunTest(t *testing.T, tc TestCase) {
	t.Helper()
	t.Run(tc.name, func(t *testing.T) {
		t.Helper()
		conf := engine.Config{
			Name: tc.name,
			Code: tc.script,
			FSTabs: []engine.FSTab{
				root.RootFSTab(),
			},
			Env: map[string]any{
				"PATH": "/sbin:/lib:/work",
				"PWD":  "/work",
			},
			Reader: &bytes.Buffer{},
			Writer: &bytes.Buffer{},
		}
		jr, err := engine.New(conf)
		if err != nil {
			t.Fatalf("Failed to create JSRuntime: %v", err)
		}
		jr.RegisterNativeModule("@jsh/process", jr.Process)
		jr.RegisterNativeModule("@jsh/machcli", Module)

		for k, v := range tc.vars {
			jr.Env.Set(k, v)
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

func TestDatabase(t *testing.T) {
	var testServer *testsuite.Server
	testServer = testsuite.NewServer(t.TempDir())
	testServer.StartServer()
	defer func() {
		testServer.StopServer()
	}()

	tick, _ := time.ParseInLocation(time.DateTime, "2025-12-17 16:49:28", time.Local)

	tests := []TestCase{
		{
			name: "mach_exec",
			script: `
				const {Client} = require('machcli');
				const conf = require("process").env.get("conf");
				const tick = require("process").env.get("tick");
				try {
					db = new Client(conf);
					conn = db.connect();
					result = conn.exec("CREATE TAG TABLE IF NOT EXISTS TAG (NAME VARCHAR(100) primary key, TIME DATETIME basetime, VALUE DOUBLE)");
					console.println("Created Table Message:", result.message);

					result = conn.exec("INSERT INTO TAG values(?, ?, ?)", 'jsh', tick, 123);
					console.println("Inserted rows:", result.rowsAffected, "Message:", result.message);
				} catch(err) {
					console.println("Error: ", err.message);
				} finally {
					conn && conn.close();
				 	db && db.close();
				}
			`,
			output: []string{
				"Created Table Message: ",
				"Inserted rows: 1 Message: ",
			},
		},
		{
			name: "mach_append",
			script: `
				const {Client} = require('machcli');
				const {now} = require("process");
				const conf = require("process").env.get("conf");
				try {
					db = new Client(conf);
					conn = db.connect();
					appender = conn.append("TAG");
					for (let i = 0; i < 99; i++) {
						appender.append('jsh', now(), 123 + i);
					}
					appender.flush();
					result = appender.close();
					console.println("Appended rows:", ...result);
				} catch(err) {
					console.println("Error: ", err.message);
				} finally {
					conn && conn.close();
				 	db && db.close();
				}
			`,
			output: []string{
				"Appended rows: 99 0",
			},
		},
		{
			name: "mach_query_row",
			script: `
				const {Client} = require('machcli');
				const conf = require("process").env.get("conf");
				try {
					db = new Client(conf);
					conn = db.connect();
					row = conn.queryRow("SELECT count(*) from TAG");
					console.println("ROWNUM:", row._ROWNUM, "Count:", row["count(*)"]);
				} catch(err) {
					console.println("Error: ", err.message);
				} finally {
					conn && conn.close();
				 	db && db.close();
				}
			`,
			output: []string{
				"ROWNUM: 1 Count: 100",
			},
		},
		{
			name: "mach_query",
			script: `
				const {Client} = require('machcli');
				const conf = require("process").env.get("conf");
				try {
					db = new Client(conf);
					conn = db.connect();
					rows = conn.query("SELECT * from TAG order by time limit ?", 1);
					for (const row of rows) {
						console.println("NAME:", row.NAME, "TIME:", row.TIME, "VALUE:", row.VALUE);
					}
					console.println(rows.message);
				} catch(err) {
					console.println("Error: ", err.message);
				} finally {
					rows && rows.close();
					conn && conn.close();
				 	db && db.close();
				}
			`,
			output: []string{
				fmt.Sprintf("NAME: jsh TIME: %s VALUE: 123", tick.Local().Format(time.DateTime)),
				"a row selected.",
			},
		},
		{
			name: "mach_explain",
			script: `
				const {Client} = require('machcli');
				const conf = require('process').env.get('conf');
				try {
					db = new Client(conf);
					conn = db.connect();
					result = conn.explain("SELECT * from TAG order by time limit 1");
					console.println(result);
				} catch(err) {
					console.println("Error: ", err.message);
				} finally {
					conn && conn.close();
				 	db && db.close();
				}
			`,
			output: []string{
				" PROJECT",
				"  LIMIT SORT",
				"   TAG READ (RAW)",
				"    KEYVALUE FULL SCAN (_TAG_DATA_0)",
				"    VOLATILE FULL SCAN (_TAG_META)",
			},
		},
	}

	for _, tc := range tests {
		tc.vars = map[string]any{
			"conf": map[string]any{
				"host":     "127.0.0.1",
				"port":     testServer.MachPort(),
				"user":     "sys",
				"password": "manager",
			},
			"tick": tick,
		}
		RunTest(t, tc)
	}
}
