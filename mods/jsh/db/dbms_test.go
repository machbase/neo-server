package db_test

import (
	"bytes"
	"context"
	"testing"

	embedded_postgres "github.com/fergusstrange/embedded-postgres"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/testsuite"
	"github.com/machbase/neo-server/v8/mods/jsh"
)

func TestMain(m *testing.M) {
	pgConf := embedded_postgres.DefaultConfig().
		Username("dbuser").
		Password("dbpass").
		Database("db").
		CachePath("./test/postgres").
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
	testServer.StartServer(m)
	testServer.CreateTestTables()

	db := testServer.DatabaseSVR()
	api.SetDefault(db)

	m.Run()

	testServer.DropTestTables()
	testServer.StopServer(m)
}

type TestCase struct {
	Name   string
	Script string
	Expect []string
}

func runTestCase(t *testing.T, tc TestCase) {
	t.Helper()
	ctx := context.TODO()
	w := &bytes.Buffer{}
	j := jsh.NewJsh(ctx,
		jsh.WithNativeModules("@jsh/process", "@jsh/db", "@jsh/system"),
		jsh.WithWriter(w),
	)
	err := j.Run(tc.Name, tc.Script, nil)
	if err != nil {
		t.Fatalf("Error running script: %s", err)
	}
	lines := bytes.Split(w.Bytes(), []byte{'\n'})
	for i, line := range lines {
		if i >= len(tc.Expect) {
			break
		}
		if !bytes.Equal(line, []byte(tc.Expect[i])) {
			t.Errorf("Expected %q, got %q", tc.Expect[i], line)
		}
	}
	if len(lines) > len(tc.Expect) {
		t.Errorf("Expected %d lines, got %d", len(tc.Expect), len(lines))
	}
}

func TestDBMS(t *testing.T) {
	tests := []TestCase{
		{
			Name: "dbms-select-no-rows",
			Script: `
				db = require("@jsh/db");
				client = new db.Client();
				try {
					conn = client.connect();
					rows = conn.query("select * from tag_data")
					cols = rows.columns()
					console.log("cols.names:", JSON.stringify(cols.columns));
					console.log("cols.types:", JSON.stringify(cols.types));
					count = 0;
					for (let rec = rows.next(); rec != null; rec = rows.next()) {
						console.log(rec);
						count++;
					}
					console.log("rows:", count);
				} catch(e) {
				 	console.log("Error:", e);
				} finally {
				 	//!! intentionally not close, to see if it properly warns
				 	// if (rows) rows.close();
				 	// if (conn) conn.close();
				}
			`,
			Expect: []string{
				`cols.names: ["NAME","TIME","VALUE","SHORT_VALUE","USHORT_VALUE","INT_VALUE","UINT_VALUE","LONG_VALUE","ULONG_VALUE","STR_VALUE","JSON_VALUE","IPV4_VALUE","IPV6_VALUE"]`,
				`cols.types: ["string","datetime","double","int16","int16","int32","int32","int64","int64","string","string","ipv4","ipv6"]`,
				"rows: 0",
				"forced db rows to close by cleanup",
				"forced db connection to close by cleanup",
				"",
			},
		},
		{
			Name: "dbms-insert",
			Script: `
				const db = require("@jsh/db");
				const { now } = require("@jsh/system");
				client = new db.Client({lowerCaseColumns:true});
				try{
					conn = client.connect();
					result = conn.exec("insert into tag_data (name, time, value) values (?, ?, ?)",
						"test-js", 1745324796000000000, 1.234);
					console.log("rowsAffected:", result.rowsAffected, "message:", result.message);
					
					conn.exec("EXEC table_flush(tag_data)")

					rows = conn.query("select name, time, value from tag_data where name = ?", "test-js")
					for (const rec of rows) {
						console.log(...rec);
					}

					rows = conn.query("select name, time, value from tag_data where name = ?", "test-js")
					console.log("cols.names:", JSON.stringify(rows.columnNames()));
					console.log("cols.types:", JSON.stringify(rows.columnTypes()));
					for (let rec = rows.next(); rec != null; rec = rows.next()) {
						console.log(rec.name+", "+rec.time.Unix()+", "+rec.value);
						for( const n in rec) {
							console.log("for_in", n, ":", rec[n]);
						}
					}

					row = conn.queryRow("select count(*) from tag_data where name = ?", "test-js")
					console.log("queryRow:", row.values["count(*)"]);
				} catch(e) {
					console.log("Error:", e);
				} finally {
					if (rows) rows.close();
				 	if (conn) conn.close();
				}
			`,
			Expect: []string{
				"rowsAffected: 1 message: a row inserted.",
				"test-js 2025-04-22 12:26:36 +0000 UTC 1.234",
				`cols.names: ["name","time","value"]`,
				`cols.types: ["string","datetime","double"]`,
				"test-js, 1745324796, 1.234",
				"for_in name : test-js",
				"for_in time : 2025-04-22 12:26:36 +0000 UTC",
				"for_in value : 1.234",
				"queryRow: 1",
				"",
			},
		},
		{
			Name: "dbms-append",
			Script: `
				const db = require("@jsh/db");
				const { now, parseTime } = require("@jsh/system");
				client = new db.Client({lowerCaseColumns:true});
				console.log("client.supportAppend:", client.supportAppend);
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
					console.log("Error:", e);
				} finally {
				 	if (appender) appender.close();
					if (conn) conn.close();
				}
				console.log("appender:", appender.result().success, appender.result().fail);
			`,
			Expect: []string{
				"client.supportAppend: true",
				"appender: 100 0",
				"",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestPostgreSql(t *testing.T) {
	tests := []TestCase{
		{
			Name: "dbms-postgresql",
			Script: `
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
					console.log("create table:", r.message);
					r = conn.exec("INSERT INTO test (name) VALUES ($1)", "foo")
					console.log("insert foo:", r.message, r.rowsAffected);
					r = conn.exec("INSERT INTO test (name) VALUES ($1)", "bar")
					console.log("insert bar:", r.message, r.rowsAffected);

					rows = conn.query("SELECT * FROM test ORDER BY id");
					console.log("cols.names:", JSON.stringify(rows.columnNames()));
					for (const rec of rows) {
						console.log(...rec);
					}
				} catch(e) {
					console.log("Error:", e.message);
				} finally {
				 	if(rows) rows.close();
					if(conn) conn.close();
				}
			`,
			Expect: []string{
				"create table: Created successfully.",
				"insert foo: a row inserted. 1",
				"insert bar: a row inserted. 1",
				`cols.names: ["id","name"]`,
				"1 foo",
				"2 bar",
				"",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}
