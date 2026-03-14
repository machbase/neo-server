package db_test

import (
	"database/sql"
	"fmt"
	"net"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/testsuite"
	"github.com/machbase/neo-server/v8/jsh/test_engine"
	dockertest "github.com/ory/dockertest/v4"
)

func TestMain(m *testing.M) {
	testServer := testsuite.NewServer("./test/tmp")
	testServer.StartServer()
	testServer.CreateTestTables()

	db := testServer.DatabaseSVR()
	api.SetDefault(db)

	m.Run()

	testServer.DropTestTables()
	testServer.StopServer()
}

func TestDBMS(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "dbms-select-no-rows",
			Script: `
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
			Output: []string{
				`cols.names: ["NAME","TIME","VALUE","SHORT_VALUE","USHORT_VALUE","INT_VALUE","UINT_VALUE","LONG_VALUE","ULONG_VALUE","STR_VALUE","JSON_VALUE","IPV4_VALUE","IPV6_VALUE"]`,
				`cols.types: ["string","datetime","double","int16","int16","int32","int32","int64","int64","string","string","ipv4","ipv6"]`,
				"rows: 0",
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
			Output: []string{
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
			Name: "dbms-append",
			Script: `
				const db = require("@jsh/db");
				const { now, parseTime } = require("@jsh/system");
				client = new db.Client({lowerCaseColumns:true});
				console.println("client.supportAppend:", client.supportAppend);
				var conn = null;
				var appender = null;
				try{
					conn = client.connect();
					appender = conn.appender("tag_data", "name", "time", "value");
					let tsFrom = new Date();
					for (let i = 0; i < 100; i++) {
						let ts = tsFrom.getTime() + 1000;
						appender.append("test-append", new Date(ts), i);
					}
				} catch(e) {
					console.println("Error:", e.message);
				} finally {
				 	if (appender) appender.close();
					if (conn) conn.close();
				}
				console.println("appender:", appender.result().success, appender.result().fail);
			`,
			Output: []string{
				"client.supportAppend: true",
				"appender: 100 0",
			},
		},
	}
	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestPostgreSql(t *testing.T) {
	pool := dockertest.NewPoolT(t, "")
	postgres := pool.RunT(t, "postgres",
		dockertest.WithTag("16"),
		dockertest.WithEnv([]string{
			"POSTGRES_USER=dbuser",
			"POSTGRES_PASSWORD=dbpass",
			"POSTGRES_DB=db",
		}),
	)
	hostPort := postgres.GetHostPort("5432/tcp")
	host, port, _ := net.SplitHostPort(hostPort)
	dsn := fmt.Sprintf("host=%s port=%s dbname=db user=dbuser password=dbpass sslmode=disable", host, port)
	// wait for postgres to be ready
	err := pool.Retry(t.Context(), 30*time.Second, func() error {
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			return err
		}
		return db.Ping()
	})
	if err != nil {
		t.Fatalf("could not connect to postgres: %v", err)
	}

	tests := []test_engine.TestCase{
		{
			Name: "dbms-postgresql",
			Script: `
				const db = require("@jsh/db");
				const { now, parseTime } = require("@jsh/system");
				
				client = new db.Client({
					driver: "postgres",
					dataSource: "` + dsn + `",
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
			Output: []string{
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
		test_engine.RunTest(t, tc)
	}
}
