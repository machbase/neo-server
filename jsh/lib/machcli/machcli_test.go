package machcli_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib"
	"github.com/machbase/neo-server/v8/jsh/lib/machcli"
	"github.com/machbase/neo-server/v8/jsh/root"
	"github.com/machbase/neo-server/v8/jsh/test_engine"
	"github.com/machbase/neo-server/v8/spi/machsvr"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

var machcliTestServer *machsvr.TestServer

func TestMain(m *testing.M) {
	machcliTestServer = &machsvr.TestServer{}
	machcliTestServer.StartServer("./testsuite_tmp")
	code := m.Run()
	machcliTestServer.StopServer()
	os.Exit(code)
}

func TestDatabase(t *testing.T) {
	tick, _ := time.ParseInLocation(time.DateTime, "2025-12-17 16:49:28", time.Local)
	vars := map[string]any{
		"conf": map[string]any{
			"host":     "127.0.0.1",
			"port":     machcliTestServer.MachPort(),
			"user":     "sys",
			"password": "manager",
		},
		"tick": tick,
	}

	test_engine.TestCase{
		Name: "mach_exec",
		Vars: vars,
		Script: `
			const {Client} = require('machcli');
			const conf = require("process").env.get("conf");
			const tick = require("process").env.get("tick");
			try {
				db = new Client(conf);
				conn = db.connect();
				result = conn.exec("CREATE TAG TABLE IF NOT EXISTS TAG (NAME VARCHAR(100) primary key, TIME DATETIME basetime, VALUE DOUBLE, JSON_VALUE JSON)");
				console.println("Created Table Message:", result.message);

				// null record max test, sql.Null[uint64]
				const rows = conn.query("select max(_id) as max_id from _tag_meta")
				const rec = rows.next()
				console.println("done:", rec.done);
				console.println("rows.max_id:", rec.max_id); // expect null
				rows.close();

				result = conn.exec("CREATE VIEW TAGVIEW as select * from TAG where name='jsh'");
				console.println("Created View Message:", result.message);

				result = conn.exec("INSERT INTO TAG values(?, ?, ?, ?)", 'jsh', tick, 123, '{"key": "value"}');
				console.println("Inserted rows:", result.rowsAffected, "Message:", result.message);
			} catch(err) {
				console.println("Error: ", err.message);
			} finally {
				conn && conn.close();
				db && db.close();
			}
		`,
		Output: []string{
			"Created Table Message: table created.",
			"done: false",
			"rows.max_id: null",
			"Created View Message: view created.",
			"Inserted rows: 1 Message: a row inserted.",
		},
	}.RunTest(t)
	test_engine.TestCase{
		Name: "mach_table_types",
		Vars: vars,
		Script: `
			const {Client, stringTableType} = require('machcli');
			const conf = require("process").env.get("conf");
			db = new Client(conf);
			conn = db.connect();
			rows = conn.query("SELECT NAME, TYPE from M$SYS_TABLES ORDER BY NAME");
			for (const row of rows) {
				if (row.NAME.startsWith("_")) continue;
				console.println("TABLE:", row.NAME, "TYPE:", stringTableType(row.TYPE));
			}
			rows.close();
			conn.close();
			db.close();
		`,
		Output: []string{
			"TABLE: TAG TYPE: Tag",
			"TABLE: TAGVIEW TYPE: View",
		},
	}.RunTest(t)
	test_engine.TestCase{
		Name: "mach_append",
		Vars: vars,
		Script: `
			const {Client} = require('machcli');
			const {now} = require("process");
			const conf = require("process").env.get("conf");
			try {
				db = new Client(conf);
				conn = db.connect();
				appender = conn.append("TAG");
				for (let i = 0; i < 99; i++) {
					appender.append('jsh', now(), 123 + i, '{"append":${i}}');
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
	}.RunTest(t)
	test_engine.TestCase{
		Name: "mach_exec_flush",
		Vars: vars,
		Script: `
			const {Client} = require('machcli');
			const conf = require("process").env.get("conf");
			try {
				db = new Client(conf);
				conn = db.connect();
				result = conn.exec('exec table_flush(TAG)');
				console.println("result:", result.message);
			} catch(err) {
				console.println("Error: ", err.message);
			} finally {
				conn && conn.close();
				db && db.close();
			}
		`,
		Output: []string{
			"result: table flushed.",
		},
	}.RunTest(t)
	test_engine.TestCase{
		Name: "mach_query_row",
		Vars: vars,
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
	}.RunTest(t)
	test_engine.TestCase{
		Name: "mach_query",
		Vars: vars,
		Script: `
			const {Client} = require('machcli');
			const conf = require("process").env.get("conf");
			try {
				db = new Client(conf);
				conn = db.connect();
				rows = conn.query("SELECT * from TAG order by time limit ?", 1);
				for (const row of rows) {
					console.println("NAME:", row.NAME, "TIME:", row.TIME, "VALUE:", row.VALUE, "JSON_VALUE:", typeof row.JSON_VALUE);
				}
				console.println(rows.message());
			} catch(err) {
				console.println("Error: ", err.message);
			} finally {
				rows && rows.close();
				conn && conn.close();
				db && db.close();
			}
		`,
		Output: []string{
			fmt.Sprintf("NAME: jsh TIME: %s VALUE: 123 JSON_VALUE: string", tick.Local().Format(time.DateTime)),
			"a row selected.",
		},
	}.RunTest(t)
	test_engine.TestCase{
		Name: "mach_query_named_params",
		Vars: vars,
		Script: `
			const {Client} = require('machcli');
			const conf = require("process").env.get("conf");
			var db, conn, rows;
			try {
				db = new Client(conf);
				conn = db.connect();
				rows = conn.query("SELECT * from TAG where name = :name order by time limit :one", {one: 1, name: "jsh"});
				for (const row of rows) {
					console.println("NAME:", row.NAME, "TIME:", row.TIME, "VALUE:", row.VALUE, "JSON_VALUE:", row.JSON_VALUE);
				}
				console.println(rows.message());
			} catch(err) {
				console.println("Error: ", err.message);
			} finally {
				rows && rows.close();
				conn && conn.close();
				db && db.close();
			}
		`,
		Output: []string{
			fmt.Sprintf("NAME: jsh TIME: %s VALUE: 123 JSON_VALUE: {\"key\": \"value\"}", tick.Local().Format(time.DateTime)),
			"a row selected.",
		},
	}.RunTest(t)
	test_engine.TestCase{
		Name: "mach_query_stat",
		Vars: vars,
		Script: `
			const {Client} = require('machcli');
			const conf = require("process").env.get("conf");
			var db, conn, rows;
			try {
				db = new Client(conf);
				conn = db.connect();
				rows = conn.query("SELECT * from v$TAG_STAT order by name");
				console.println("{\"rows\": [");
				for (const row of rows) {
					console.println(JSON.stringify(row), ",");
				}
				console.println("], \"reason\":", "\"" + rows.message() + "\"");
				console.println("}");
			} catch(err) {
				console.println("Error: ", err.message);
			} finally {
				rows && rows.close();
				conn && conn.close();
				db && db.close();
			}
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.Equal(t, "jsh", gjson.Get(result, "rows.0.NAME").String(), result)
			require.Equal(t, int64(100), gjson.Get(result, "rows.0.ROW_COUNT").Int(), result)
			require.Regexp(t, `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}.*`, gjson.Get(result, "rows.0.MIN_TIME").String(), result)
			require.Regexp(t, `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}.*`, gjson.Get(result, "rows.0.MAX_TIME").String(), result)
			require.Contains(t, result, "\"MIN_VALUE\":null", result)
			require.Contains(t, result, "\"MIN_VALUE_TIME\":null", result)
			require.Contains(t, result, "\"MAX_VALUE\":null", result)
			require.Contains(t, result, "\"MAX_VALUE_TIME\":null", result)
			require.Regexp(t, `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}.*`, gjson.Get(result, "rows.0.RECENT_ROW_TIME").String(), result)
			require.Equal(t, "a row selected.", gjson.Get(result, "reason").String(), result)
		},
	}.RunTest(t)
	test_engine.TestCase{
		Name: "mach_explain",
		Vars: vars,
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
	}.RunTest(t)
}

func TestNewDatabaseCoverage(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		db, err := machcli.NewDatabase("{invalid")
		require.Error(t, err)
		require.Nil(t, db)
	})

	t.Run("defaults and alternative config", func(t *testing.T) {
		cfg := fmt.Sprintf(`{"host":"127.0.0.1","port":%d,"user":"sys","password":"manager","alternativeHost":"127.0.0.2","alternativePort":5657}`,
			machcliTestServer.MachPort(),
		)
		db, err := machcli.NewDatabase(cfg)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, db.Close())
		})

		require.Equal(t, "SYS", db.User())

		conn, err := db.Connect()
		require.NoError(t, err)
		require.NotNil(t, conn)
		require.NoError(t, conn.Close())
	})

	t.Run("connect with wrong password", func(t *testing.T) {
		cfg := fmt.Sprintf(`{"host":"127.0.0.1","port":%d,"user":"sys","password":"wrong"}`,
			machcliTestServer.MachPort(),
		)
		db, err := machcli.NewDatabase(cfg)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, db.Close())
		})

		conn, err := db.Connect()
		require.Error(t, err)
		require.Nil(t, conn)
	})
}

func TestNewDatabaseAppliesDatabaseDSNOption(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "database key", key: "database"},
		{name: "db alias", key: "db"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := fmt.Sprintf(`{"host":"127.0.0.1","port":%d,"user":"sys","password":"manager","%s":"MACHBASEDB"}`,
				machcliTestServer.MachPort(), tc.key,
			)
			db, err := machcli.NewDatabase(cfg)
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, db.Close())
			})

			conn, err := db.Connect()
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, conn.Close())
			})

			var currentDatabase string
			row := conn.QueryRowContext(context.Background(), "select current_database()")
			require.NoError(t, row.Scan(&currentDatabase))
			require.Equal(t, "MACHBASEDB", currentDatabase)
		})
	}
}

func TestClientLoadsSharedConfigAndMergesCallerConfig(t *testing.T) {
	sharedDir := t.TempDir()
	sharedConfig := fmt.Sprintf(`{"host":"127.0.0.1","port":%d,"user":"demo","password":"demo"}`,
		machcliTestServer.MachPort(),
	)
	require.NoError(t, os.WriteFile(filepath.Join(sharedDir, "db.json"), []byte(sharedConfig), 0o644))

	writer := &bytes.Buffer{}
	jr, err := engine.New(engine.Config{
		Name: "machcli_shared_config",
		Code: `
			const { Client } = require('machcli');

			const dbFromShared = new Client();
			console.println("shared-user:", dbFromShared.user());
			dbFromShared.close();

			const dbFromMerged = new Client({ user: "sys", password: "manager" });
			console.println("merged-user:", dbFromMerged.user());
			const conn = dbFromMerged.connect();
			console.println("connected:", typeof conn.close === "function");
			conn.close();
			dbFromMerged.close();
		`,
		FSTabs: []engine.FSTab{
			root.RootFSTab(),
			{MountPoint: "/proc/share", Source: sharedDir},
			{MountPoint: "/lib", FS: lib.LibFS()},
		},
		Env: map[string]any{
			"LIBRARY_PATH": "/lib",
		},
		Writer: writer,
	})
	require.NoError(t, err)
	lib.Enable(jr)
	require.NoError(t, jr.Run())

	require.Equal(t, "shared-user: DEMO\nmerged-user: SYS\nconnected: true\n", writer.String())
}

func TestNormalizeTableNameCoverage(t *testing.T) {
	cfg := fmt.Sprintf(`{"host":"127.0.0.1","port":%d,"user":"demo","password":"demo"}`,
		machcliTestServer.MachPort(),
	)
	db, err := machcli.NewDatabase(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	require.Equal(t, [3]string{"MACHBASEDB", "DEMO", "TAG_DATA"}, db.NormalizeTableName("tag_data"))
	require.Equal(t, [3]string{"MACHBASEDB", "SYS", "TAG_DATA"}, db.NormalizeTableName("sys.tag_data"))
	require.Equal(t, [3]string{"OTHERDB", "SYS", "TAG_DATA"}, db.NormalizeTableName("otherdb.sys.tag_data"))
	require.Equal(t, [3]string{"", "", "A.B.C.D"}, db.NormalizeTableName("a.b.c.d"))
}

func TestRowsScanCoverage(t *testing.T) {
	ctx := context.Background()
	tableName := "JSH_MACHCLI_COVER"
	tick := time.Date(2026, time.March, 30, 12, 0, 0, 0, time.UTC)

	cfg := fmt.Sprintf(`{"host":"127.0.0.1","port":%d,"user":"sys","password":"manager"}`,
		machcliTestServer.MachPort(),
	)
	db, err := machcli.NewDatabase(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	conn, err := db.Connect()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	_, err = conn.ExecContext(ctx, fmt.Sprintf("CREATE TAG TABLE IF NOT EXISTS %s (NAME VARCHAR(100) PRIMARY KEY, TIME DATETIME BASETIME, VALUE DOUBLE)", tableName))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = conn.ExecContext(ctx, "DROP TABLE "+tableName)
	})

	_, err = conn.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s VALUES(?, ?, ?)", tableName), "row-1", tick, 123.45)
	require.NoError(t, err)

	rows, err := conn.QueryContext(ctx, fmt.Sprintf("SELECT NAME, VALUE FROM %s ORDER BY TIME LIMIT 1", tableName))
	require.NoError(t, err)
	require.NoError(t, rows.Close())

	rows, err = conn.QueryContext(ctx, fmt.Sprintf("SELECT NAME, VALUE FROM %s ORDER BY TIME LIMIT 1", tableName))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = rows.Close()
	})

	require.True(t, rows.Next())
	buffer, err := machcli.RowsScan(rows)
	require.NoError(t, err)
	require.Len(t, buffer, 2)
	require.Equal(t, "row-1", machcli.Unbox(buffer[0]))
	require.Equal(t, 123.45, machcli.Unbox(buffer[1]))
}
