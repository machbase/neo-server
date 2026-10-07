package db_test

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/machbase/neo-server/v8/jsh/test_engine"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/spi"
	"github.com/machbase/neo-server/v8/spi/machsvr"
	"github.com/machbase/neo-server/v8/test"
	dockertest "github.com/ory/dockertest/v4"
)

func TestMain(m *testing.M) {
	testServer := machsvr.TestServer{}
	testServer.StartServer("./test/tmp")
	createTagTables()

	m.Run()

	dropTagTables()
	testServer.StopServer()
}

func SqlTidy(sqlTextLines ...string) string {
	sqlText := strings.Join(sqlTextLines, "\n")
	lines := strings.Split(sqlText, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimSpace(ln)
	}
	return strings.Join(lines, " ")
}

func createTagTables() {
	ctx := context.Background()
	conn, err := spi.Connect(ctx, "sys")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	_, err = conn.ExecContext(ctx, SqlTidy(`
		create tag table if not exists tag_data(
			name            varchar(100) primary key, 
			time            datetime basetime, 
			value           double summarized,
			short_value     short,
			ushort_value    ushort,
			int_value       integer,
			uint_value 	    uinteger,
			long_value      long,
			ulong_value 	ulong,
			str_value       varchar(400),
			json_value      json,
			ipv4_value      ipv4,
			ipv6_value      ipv6,
			bin_value		binary
		) TAG_DUPLICATE_CHECK_DURATION=1;
	`))
	if err != nil {
		panic(err)
	}

	_, err = conn.ExecContext(ctx, SqlTidy(`
		create tag table if not exists tag_simple(
			name            varchar(100) primary key, 
			time            datetime basetime, 
			value           double
		) TAG_DUPLICATE_CHECK_DURATION=1;
	`))
	if err != nil {
		panic(err)
	}
}

func dropTagTables() {
	ctx := context.Background()
	conn, err := spi.Connect(ctx, "sys")
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	_, err = conn.ExecContext(ctx, "DROP TABLE tag_data")
	if err != nil {
		panic(err)
	}
	_, err = conn.ExecContext(ctx, "DROP TABLE tag_simple")
	if err != nil {
		panic(err)
	}
}

func TestDBMS(t *testing.T) {
	test_engine.TestCase{
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
			`cols.names: ["NAME","TIME","VALUE","SHORT_VALUE","USHORT_VALUE","INT_VALUE","UINT_VALUE","LONG_VALUE","ULONG_VALUE","STR_VALUE","JSON_VALUE","IPV4_VALUE","IPV6_VALUE","BIN_VALUE"]`,
			`cols.types: ["VARCHAR","DATETIME","DOUBLE","SHORT","USHORT","INTEGER","UINTEGER","LONG","ULONG","VARCHAR","JSON","IPV4","IPV6","BINARY"]`,
			"rows: 0",
		},
	}.RunTest(t)
	test_engine.TestCase{
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
			`cols.types: ["VARCHAR","DATETIME","DOUBLE"]`,
			"test-js, 1745324796, 1.234",
			"for_in name : test-js",
			fmt.Sprintf("for_in time : %s", time.Unix(1745324796, 0).Format("2006-01-02 15:04:05")),
			"for_in value : 1.234",
			"queryRow: 1",
		},
	}.RunTest(t)
	test_engine.TestCase{
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
				console.println("Error:", e);
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
	}.RunTest(t)
}

// TestDBMSUserScope guards new_client's context wiring (machbase/neo#1468): a
// non-sys UserScope threaded into the jsh engine's context via
// model.ContextWithUserScope must be applied to the Appender's DSN, instead of
// always defaulting to "sys".
func TestDBMSUserScope(t *testing.T) {
	ctx := context.Background()
	username := fmt.Sprintf("dbms_scope_user_%d", time.Now().UnixNano())
	table := "DBMS_SCOPE_TBL"

	sysConn, err := spi.Connect(ctx, "sys")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sysConn.ExecContext(ctx, fmt.Sprintf("CREATE USER %s IDENTIFIED BY 'password'", username)); err != nil {
		t.Fatal(err)
	}
	sysConn.Close()
	t.Cleanup(func() {
		cleanupConn, err := spi.Connect(context.Background(), "sys")
		if err != nil {
			return
		}
		defer cleanupConn.Close()
		_, _ = cleanupConn.ExecContext(context.Background(), fmt.Sprintf("DROP TABLE %s.%s", username, table))
		_, _ = cleanupConn.ExecContext(context.Background(), "DROP USER "+username)
	})

	ownConn, err := spi.Connect(ctx, username)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ownConn.ExecContext(ctx, fmt.Sprintf(
		"CREATE TAG TABLE %s (NAME VARCHAR(40) PRIMARY KEY, TIME DATETIME BASETIME, VALUE DOUBLE SUMMARIZED)", table)); err != nil {
		t.Fatal(err)
	}
	ownConn.Close()

	test_engine.TestCase{
		Name:    "dbms-append-user-scope",
		Context: model.ContextWithUserScope(ctx, model.UserScope{User: username}),
		Script: fmt.Sprintf(`
			const db = require("@jsh/db");
			const client = new db.Client();
			const conn = client.connect();
			const appender = conn.appender("%s.%s", "name", "time", "value");
			appender.append("scoped", new Date(), 1.0);
			appender.close();
			conn.close();
		`, username, table),
	}.RunTest(t)

	checkConn, err := spi.Connect(ctx, "sys")
	if err != nil {
		t.Fatal(err)
	}
	defer checkConn.Close()
	var count int
	if err := checkConn.QueryRowContext(ctx,
		fmt.Sprintf("select count(*) from %s.%s", username, table)).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row scoped under %s.%s, got %d", username, table, count)
	}
}

// TestDBMSDefaultPoolRespectsContextScope guards new_client's default-pool
// branch (machbase/neo#1468): a plain `new db.Client()` (no bridge/driver
// option) must query using the ctx-derived user scope, not always the shared
// "sys" pool, so SCRIPT({})/CLI shell scoping also applies to conn.query/exec,
// not just Appender.
func TestDBMSDefaultPoolRespectsContextScope(t *testing.T) {
	ctx := context.Background()
	username := fmt.Sprintf("dbms_scope_pool_%d", time.Now().UnixNano())

	sysConn, err := spi.Connect(ctx, "sys")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sysConn.ExecContext(ctx, fmt.Sprintf("CREATE USER %s IDENTIFIED BY 'password'", username)); err != nil {
		t.Fatal(err)
	}
	sysConn.Close()
	t.Cleanup(func() {
		cleanupConn, err := spi.Connect(context.Background(), "sys")
		if err != nil {
			return
		}
		defer cleanupConn.Close()
		_, _ = cleanupConn.ExecContext(context.Background(), "DROP USER "+username)
	})

	test_engine.TestCase{
		Name:    "dbms-default-pool-context-scope",
		Context: model.ContextWithUserScope(ctx, model.UserScope{User: username}),
		Script: `
			const db = require("@jsh/db");
			const conn = new db.Client().connect();
			const row = conn.queryRow("select current_user()");
			console.println("current_user:", row.values["current_user()"]);
			conn.close();
		`,
		Output: []string{"current_user: " + strings.ToUpper(username)},
	}.RunTest(t)
}

// TestDBMSClientExplicitUserOption guards ClientOptions.User (machbase/neo#1468):
// a script author must be able to declare the connection's user scope
// explicitly (e.g. from CLI jsh/cgi-bin, which have no wired context),
// overriding whatever (if any) scope ctx carries.
func TestDBMSClientExplicitUserOption(t *testing.T) {
	ctx := context.Background()
	username := fmt.Sprintf("dbms_scope_explicit_%d", time.Now().UnixNano())

	sysConn, err := spi.Connect(ctx, "sys")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sysConn.ExecContext(ctx, fmt.Sprintf("CREATE USER %s IDENTIFIED BY 'password'", username)); err != nil {
		t.Fatal(err)
	}
	sysConn.Close()
	t.Cleanup(func() {
		cleanupConn, err := spi.Connect(context.Background(), "sys")
		if err != nil {
			return
		}
		defer cleanupConn.Close()
		_, _ = cleanupConn.ExecContext(context.Background(), "DROP USER "+username)
	})

	test_engine.TestCase{
		// no Context set: mirrors CLI jsh/cgi-bin, which have no wired identity.
		Name: "dbms-client-explicit-user-option",
		Script: fmt.Sprintf(`
			const db = require("@jsh/db");
			const conn = new db.Client({user: "%s"}).connect();
			const row = conn.queryRow("select current_user()");
			console.println("current_user:", row.values["current_user()"]);
			conn.close();
		`, username),
		Output: []string{"current_user: " + strings.ToUpper(username)},
	}.RunTest(t)
}

func TestPostgreSql(t *testing.T) {
	if !test.SupportDockerTest() {
		t.Skip("dockertest does not work in this environment")
	}
	pool := dockertest.NewPoolT(t, "")
	postgresRepository, postgresTag := test.PostgresDockerImage.Resolve()
	postgres := pool.RunT(t, postgresRepository,
		dockertest.WithTag(postgresTag),
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

	test_engine.TestCase{
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
	}.RunTest(t)
}
