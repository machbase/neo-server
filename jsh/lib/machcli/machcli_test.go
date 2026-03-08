package machcli_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api/testsuite"
	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestDatabase(t *testing.T) {
	var testServer *testsuite.Server
	testServer = testsuite.NewServer(t.TempDir())
	testServer.StartServer()
	defer func() {
		testServer.StopServer()
	}()

	tick, _ := time.ParseInLocation(time.DateTime, "2025-12-17 16:49:28", time.Local)

	tests := []test_engine.TestCase{
		{
			Name: "mach_exec",
			Script: `
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
			Output: []string{
				"Created Table Message: ",
				"Inserted rows: 1 Message: ",
			},
		},
		{
			Name: "mach_append",
			Script: `
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
			Output: []string{
				"Appended rows: 99 0",
			},
		},
		{
			Name: "mach_query_row",
			Script: `
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
			Output: []string{
				"ROWNUM: 1 Count: 100",
			},
		},
		{
			Name: "mach_query",
			Script: `
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
			Output: []string{
				fmt.Sprintf("NAME: jsh TIME: %s VALUE: 123", tick.Local().Format(time.DateTime)),
				"a row selected.",
			},
		},
		{
			Name: "mach_explain",
			Script: `
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
			Output: []string{
				" PROJECT",
				"  LIMIT SORT",
				"   TAG READ (RAW)",
				"    KEYVALUE FULL SCAN (_TAG_DATA_0)",
				"    VOLATILE FULL SCAN (_TAG_META)",
			},
		},
	}

	for _, tc := range tests {
		tc.Vars = map[string]any{
			"conf": map[string]any{
				"host":     "127.0.0.1",
				"port":     testServer.MachPort(),
				"user":     "sys",
				"password": "manager",
			},
			"tick": tick,
		}
		test_engine.RunTest(t, tc)
	}
}
