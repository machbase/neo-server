package db

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	embedded_postgres "github.com/fergusstrange/embedded-postgres"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/testsuite"
	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/native/system"
	"github.com/machbase/neo-server/v8/jsh/root"
)

func TestMain(m *testing.M) {
	pgConf := embedded_postgres.DefaultConfig().
		Username("dbuser").
		Password("dbpass").
		Database("db").
		CachePath("./test/postgres").
		Version(embedded_postgres.V16).
		Port(15455)
	pgdb := embedded_postgres.NewDatabase(pgConf)
	if err := pgdb.Start(); err != nil {
		panic(err)
	}
	defer func() {
		if err := pgdb.Stop(); err != nil {
			panic(err)
		}
	}()

	testServer := testsuite.NewServer("./test/tmp")
	testServer.StartServer()
	testServer.CreateTestTables()

	db := testServer.DatabaseSVR()
	api.SetDefault(db)

	m.Run()

	testServer.DropTestTables()
	testServer.StopServer()
}

type TestCase struct {
	name   string
	script string
	input  []string
	output []string
	err    string
	vars   map[string]any
}

func runTestCase(t *testing.T, tc TestCase) {
	t.Helper()
	conf := engine.Config{
		Name:   tc.name,
		Code:   tc.script,
		FSTabs: []engine.FSTab{root.RootFSTab()},
		Env:    tc.vars,
		Reader: &bytes.Buffer{},
		Writer: &bytes.Buffer{},
	}
	jr, err := engine.New(conf)
	if err != nil {
		t.Fatalf("Failed to create JSRuntime: %v", err)
	}
	jr.RegisterNativeModule("@jsh/db", Module)
	jr.RegisterNativeModule("@jsh/system", system.Module)
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
}

func TestDBMS(t *testing.T) {
	tests := []TestCase{
		{
			name: "dbms-select-no-rows",
			script: `
				db = require("@jsh/db");
				client = new db.Client();
				try {
					conn = client.connect();
					rows = conn.query("select * from tag_data")
					cols = rows.columns()
					console.println("cols.names:", JSON.stringify(cols.columns));
					console.println("cols.types:", JSON.stringify(cols.types));
					count = 0;
					for (let rec = rows.next(); rec != null; rec = rows.next()) {
						console.println(rec);
						count++;
					}
					console.println("rows:", count);
				} catch(e) {
				 	console.println("Error:", e);
				} finally {
				 	//!! intentionally not close, to see if it properly warns
				 	// if (rows) rows.close();
				 	// if (conn) conn.close();
				}
			`,
			output: []string{
				`cols.names: ["NAME","TIME","VALUE","SHORT_VALUE","USHORT_VALUE","INT_VALUE","UINT_VALUE","LONG_VALUE","ULONG_VALUE","STR_VALUE","JSON_VALUE","IPV4_VALUE","IPV6_VALUE"]`,
				`cols.types: ["string","datetime","double","int16","int16","int32","int32","int64","int64","string","string","ipv4","ipv6"]`,
				"rows: 0",
			},
		},
		{
			name: "dbms-insert",
			script: `
				const db = require("@jsh/db");
				const { now } = require("@jsh/system");
				client = new db.Client({lowerCaseColumns:true});
				try{
					conn = client.connect();
					result = conn.exec("insert into tag_data (name, time, value) values (?, ?, ?)",
						"test-js", 1745324796000000000, 1.234);
					console.println("rowsAffected:", result.rowsAffected, "message:", result.message);
					
					conn.exec("EXEC table_flush(tag_data)")

					rows = conn.query("select name, time, value from tag_data where name = ?", "test-js")
					for (const rec of rows) {
						console.println(...rec);
					}

					rows = conn.query("select name, time, value from tag_data where name = ?", "test-js")
					console.println("cols.names:", JSON.stringify(rows.columnNames()));
					console.println("cols.types:", JSON.stringify(rows.columnTypes()));
					for (let rec = rows.next(); rec != null; rec = rows.next()) {
						console.println(rec.name+", "+rec.time.unix()+", "+rec.value);
						for( const n in rec) {
							console.println("for_in", n, ":", rec[n]);
						}
					}

					row = conn.queryRow("select count(*) from tag_data where name = ?", "test-js")
					console.println("queryRow:", row.values["count(*)"]);
				} catch(e) {
					console.println("Error:", e.message);
				} finally {
					if (rows) rows.close();
				 	if (conn) conn.close();
				}
			`,
			output: []string{
				"rowsAffected: 1 message: a row inserted.",
				fmt.Sprintf("test-js %s 1.234", time.Unix(1745324796, 0).Format("2006-01-02 15:04:05")),
				`cols.names: ["name","time","value"]`,
				`cols.types: ["string","datetime","double"]`,
				"test-js, 1745324796, 1.234",
				"for_in name : test-js",
				fmt.Sprintf("for_in time : %s", time.Unix(1745324796, 0).Format("2006-01-02 15:04:05")),
				"for_in value : 1.234",
				"queryRow: 1",
			},
		},
		{
			name: "dbms-append",
			script: `
				const db = require("@jsh/db");
				const { now, parseTime } = require("@jsh/system");
				client = new db.Client({lowerCaseColumns:true});
				console.println("client.supportAppend:", client.supportAppend);
				var conn = null;
				var appender = null;
				try{
					conn = client.connect();
					appender = conn.appender("tag_data", "name", "time", "value");
					let ts = (new Date()).getTime();
					for (let i = 0; i < 100; i++) {
						ts = ts + 1000;
						appender.append("test-append", parseTime(ts, "ms"), i);
					}
				} catch(e) {
					console.println("Error:", e);
				} finally {
				 	if (appender) appender.close();
					if (conn) conn.close();
				}
				console.println("appender:", appender.result().success, appender.result().fail);
			`,
			output: []string{
				"client.supportAppend: true",
				"appender: 100 0",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestPostgreSql(t *testing.T) {
	tests := []TestCase{
		{
			name: "dbms-postgresql",
			script: `
				const db = require("@jsh/db");
				const { now, parseTime } = require("@jsh/system");
				
				client = new db.Client({
					driver: "postgres",
					dataSource: "host=127.0.0.1 port=15455 dbname=db user=dbuser password=dbpass sslmode=disable",
					lowerCaseColumns:true,
				});
				var conn = null;
				var rows = null;
				try{
					conn = client.connect();
					r = conn.exec("CREATE TABLE test (id SERIAL PRIMARY KEY, name TEXT)");
					console.println("create table:", r.message);
					r = conn.exec("INSERT INTO test (name) VALUES ($1)", "foo")
					console.println("insert foo:", r.message, r.rowsAffected);
					r = conn.exec("INSERT INTO test (name) VALUES ($1)", "bar")
					console.println("insert bar:", r.message, r.rowsAffected);

					rows = conn.query("SELECT * FROM test ORDER BY id");
					console.println("cols.names:", JSON.stringify(rows.columnNames()));
					for (const rec of rows) {
						console.println(...rec);
					}
				} catch(e) {
					console.println("Error:", e.message);
				} finally {
				 	if(rows) rows.close();
					if(conn) conn.close();
				}
			`,
			output: []string{
				"create table: Created successfully.",
				"insert foo: a row inserted. 1",
				"insert bar: a row inserted. 1",
				`cols.names: ["id","name"]`,
				"1 foo",
				"2 bar",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}
