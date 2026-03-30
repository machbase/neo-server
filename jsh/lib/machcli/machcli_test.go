package machcli_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-client/machgo"
	"github.com/machbase/neo-server/v8/api/testsuite"
	machclilib "github.com/machbase/neo-server/v8/jsh/lib/machcli"
	"github.com/machbase/neo-server/v8/jsh/test_engine"
	"github.com/stretchr/testify/require"
)

var machcliTestServer *testsuite.Server

func TestMain(m *testing.M) {
	machcliTestServer = testsuite.NewServer("./testsuite_tmp")
	machcliTestServer.StartServer()
	code := m.Run()
	machcliTestServer.StopServer()
	os.Exit(code)
}

func TestDatabase(t *testing.T) {
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
				"port":     machcliTestServer.MachPort(),
				"user":     "sys",
				"password": "manager",
			},
			"tick": tick,
		}
		test_engine.RunTest(t, tc)
	}
}

func TestNewDatabaseCoverage(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		db, err := machclilib.NewDatabase("{invalid")
		require.Error(t, err)
		require.Nil(t, db)
	})

	t.Run("defaults and alternative config", func(t *testing.T) {
		cfg := fmt.Sprintf(`{"host":"127.0.0.1","port":%d,"alternativeHost":"127.0.0.2","alternativePort":5657}`,
			machcliTestServer.MachPort(),
		)
		db, err := machclilib.NewDatabase(cfg)
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
		db, err := machclilib.NewDatabase(cfg)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, db.Close())
		})

		conn, err := db.Connect()
		require.Error(t, err)
		require.Nil(t, conn)
	})
}

func TestNormalizeTableNameCoverage(t *testing.T) {
	cfg := fmt.Sprintf(`{"host":"127.0.0.1","port":%d,"user":"demo","password":"demo"}`,
		machcliTestServer.MachPort(),
	)
	db, err := machclilib.NewDatabase(cfg)
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
	db, err := machclilib.NewDatabase(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	conn, err := db.Connect()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	result := conn.Exec(ctx, fmt.Sprintf("CREATE TAG TABLE IF NOT EXISTS %s (NAME VARCHAR(100) PRIMARY KEY, TIME DATETIME BASETIME, VALUE DOUBLE)", tableName))
	require.NoError(t, result.Err())
	t.Cleanup(func() {
		_ = conn.Exec(ctx, "DROP TABLE "+tableName).Err()
	})

	result = conn.Exec(ctx, fmt.Sprintf("INSERT INTO %s VALUES(?, ?, ?)", tableName), "row-1", tick, 123.45)
	require.NoError(t, result.Err())

	rowsAny, err := conn.Query(ctx, fmt.Sprintf("SELECT NAME, VALUE FROM %s ORDER BY TIME LIMIT 1", tableName))
	require.NoError(t, err)
	rows, ok := rowsAny.(*machgo.Rows)
	require.True(t, ok)
	require.NoError(t, rows.Close())

	rowsAny, err = conn.Query(ctx, fmt.Sprintf("SELECT NAME, VALUE FROM %s ORDER BY TIME LIMIT 1", tableName))
	require.NoError(t, err)
	rows, ok = rowsAny.(*machgo.Rows)
	require.True(t, ok)
	t.Cleanup(func() {
		_ = rows.Close()
	})

	require.True(t, rows.Next())
	buffer, err := machclilib.RowsScan(rows)
	require.NoError(t, err)
	require.Len(t, buffer, 2)
	require.Equal(t, "row-1", api.Unbox(buffer[0]))
	require.Equal(t, 123.45, api.Unbox(buffer[1]))
}
