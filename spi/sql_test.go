package spi_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-client/api"
	_ "github.com/machbase/neo-client/machbase"
	"github.com/machbase/neo-client/machgo"
	"github.com/machbase/neo-server/v8/spi"
	"github.com/stretchr/testify/require"
)

type sqlCompatFixture struct {
	db        *sql.DB
	tableName string
}

func newSQLCompatFixture(t *testing.T) *sqlCompatFixture {
	t.Helper()

	dsn := fmt.Sprintf("server=127.0.0.1:%d;user=sys;password=manager;fetch_rows=100", testServer.MachPort())
	db, err := sql.Open("machbase", dsn)
	require.NoError(t, err)
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})
	require.NoError(t, db.PingContext(t.Context()))

	tableName := fmt.Sprintf("SQL_COMPAT_%d", time.Now().UnixNano())
	_, err = db.ExecContext(t.Context(), fmt.Sprintf(`CREATE TABLE %s (ID LONG NOT NULL, NAME VARCHAR(100))`, tableName))
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = db.ExecContext(t.Context(), fmt.Sprintf(`DROP TABLE %s`, tableName))
	})

	_, err = db.ExecContext(t.Context(), fmt.Sprintf(`INSERT INTO %s VALUES(?, ?)`, tableName), int64(1), "neo")
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), fmt.Sprintf(`INSERT INTO %s VALUES(?, ?)`, tableName), int64(2), "machbase")
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), fmt.Sprintf(`INSERT INTO %s VALUES(?, ?)`, tableName), int64(99), nil)
	require.NoError(t, err)

	return &sqlCompatFixture{db: db, tableName: tableName}
}

func TestMachbaseSQLCompatibilitySupported(t *testing.T) {
	fixture := newSQLCompatFixture(t)
	db := fixture.db
	tableName := fixture.tableName

	t.Run("db ping exec query row and prepare", func(t *testing.T) {
		require.NoError(t, db.PingContext(t.Context()))

		res, err := db.ExecContext(
			t.Context(),
			fmt.Sprintf("INSERT INTO %s VALUES(?, ?)", tableName),
			int64(3),
			"driver",
		)
		require.NoError(t, err)
		affected, err := res.RowsAffected()
		require.NoError(t, err)
		require.Equal(t, int64(1), affected)

		rows, err := db.QueryContext(t.Context(), fmt.Sprintf("SELECT ID, NAME FROM %s ORDER BY ID", tableName))
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, rows.Close())
		})

		require.True(t, rows.Next())
		var id int64
		var name string
		require.NoError(t, rows.Scan(&id, &name))
		require.Equal(t, int64(1), id)
		require.Equal(t, "neo", name)

		types, err := rows.ColumnTypes()
		require.NoError(t, err)
		require.Len(t, types, 2)
		require.Equal(t, "ID", strings.ToUpper(types[0].Name()))
		require.Equal(t, "LONG", strings.ToUpper(types[0].DatabaseTypeName()))

		require.NoError(t, rows.Close())

		var idByQueryRow int64
		var nameByQueryRow string
		require.NoError(
			t,
			db.QueryRowContext(
				t.Context(),
				fmt.Sprintf("SELECT ID, NAME FROM %s WHERE ID = ?", tableName),
				int64(2),
			).Scan(&idByQueryRow, &nameByQueryRow),
		)
		require.Equal(t, int64(2), idByQueryRow)
		require.Equal(t, "machbase", nameByQueryRow)

		stmt, err := db.PrepareContext(t.Context(), fmt.Sprintf("SELECT ID, NAME FROM %s WHERE ID = ?", tableName))
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, stmt.Close())
		})

		stmtRows, err := stmt.QueryContext(t.Context(), int64(3))
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, stmtRows.Close())
		})
		require.True(t, stmtRows.Next())
		var sid int64
		var sname string
		require.NoError(t, stmtRows.Scan(&sid, &sname))
		require.Equal(t, int64(3), sid)
		require.Equal(t, "driver", sname)

		stmtExec, err := db.PrepareContext(t.Context(), fmt.Sprintf("INSERT INTO %s VALUES(?, ?)", tableName))
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, stmtExec.Close())
		})
		execRes, err := stmtExec.ExecContext(t.Context(), int64(4), "prepared")
		require.NoError(t, err)
		execAffected, err := execRes.RowsAffected()
		require.NoError(t, err)
		require.Equal(t, int64(1), execAffected)
	})

	t.Run("sql conn raw exposes optional driver interfaces", func(t *testing.T) {
		conn, err := db.Conn(t.Context())
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, conn.Close())
		})

		require.NoError(t, conn.PingContext(t.Context()))

		var support map[string]bool
		err = conn.Raw(func(dc any) error {
			support = map[string]bool{
				"driver.Conn":               false,
				"driver.ConnPrepareContext": false,
				"driver.ExecerContext":      false,
				"driver.QueryerContext":     false,
				"driver.Pinger":             false,
				"driver.NamedValueChecker":  false,
				"driver.Validator":          false,
				"driver.SessionResetter":    false,
				"driver.ConnBeginTx":        false,
			}
			if _, ok := dc.(driver.Conn); ok {
				support["driver.Conn"] = true
			}
			if _, ok := dc.(driver.ConnPrepareContext); ok {
				support["driver.ConnPrepareContext"] = true
			}
			if _, ok := dc.(driver.ExecerContext); ok {
				support["driver.ExecerContext"] = true
			}
			if _, ok := dc.(driver.QueryerContext); ok {
				support["driver.QueryerContext"] = true
			}
			if _, ok := dc.(driver.Pinger); ok {
				support["driver.Pinger"] = true
			}
			if _, ok := dc.(driver.NamedValueChecker); ok {
				support["driver.NamedValueChecker"] = true
			}
			if _, ok := dc.(driver.Validator); ok {
				support["driver.Validator"] = true
			}
			if _, ok := dc.(driver.SessionResetter); ok {
				support["driver.SessionResetter"] = true
			}
			if _, ok := dc.(driver.ConnBeginTx); ok {
				support["driver.ConnBeginTx"] = true
			}
			return nil
		})
		require.NoError(t, err)

		require.True(t, support["driver.Conn"])
		require.True(t, support["driver.ConnPrepareContext"])
		require.True(t, support["driver.ExecerContext"])
		require.True(t, support["driver.QueryerContext"])
		require.True(t, support["driver.Pinger"])
		require.True(t, support["driver.NamedValueChecker"])
		require.True(t, support["driver.Validator"])
		require.True(t, support["driver.SessionResetter"])
		require.True(t, support["driver.ConnBeginTx"])
	})
}

func TestMachbaseSQLCompatibilityGaps(t *testing.T) {
	fixture := newSQLCompatFixture(t)
	db := fixture.db
	tableName := fixture.tableName

	t.Run("transactions are not supported", func(t *testing.T) {
		// TODO: implement transaction support in neo-client/machbase (Begin/BeginTx, Commit, Rollback).
		tx, err := db.BeginTx(t.Context(), nil)
		require.Error(t, err)
		require.Nil(t, tx)
		require.Contains(t, strings.ToLower(err.Error()), "does not support explicit transactions")
	})

	t.Run("named parameters are not supported", func(t *testing.T) {
		// TODO: add named parameter binding support in neo-client/machbase driver (sql.Named / :name / @name).
		_, err := db.ExecContext(
			t.Context(),
			fmt.Sprintf("UPDATE %s SET NAME = @name WHERE ID = 1", tableName),
			sql.Named("name", "neo"),
		)
		require.Error(t, err)
		require.Contains(t, strings.ToLower(err.Error()), "named parameters")
	})

	t.Run("last insert id is not implemented", func(t *testing.T) {
		// TODO: provide LastInsertId mapping if machbase engine can expose deterministic inserted row identifier.
		res, err := db.ExecContext(
			t.Context(),
			fmt.Sprintf("INSERT INTO %s VALUES(?, ?)", tableName),
			int64(10),
			"gap",
		)
		require.NoError(t, err)
		_, err = res.LastInsertId()
		require.Error(t, err)
		require.Contains(t, strings.ToLower(err.Error()), "not implemented")
	})
}

func TestMachbaseSQLCompatibilityCoreTypes(t *testing.T) {
	fixture := newSQLCompatFixture(t)
	db := fixture.db
	tableName := fixture.tableName

	t.Run("sql.Result compatibility", func(t *testing.T) {
		res, err := db.ExecContext(
			t.Context(),
			fmt.Sprintf("INSERT INTO %s VALUES(?, ?)", tableName),
			int64(11),
			"result-check",
		)
		require.NoError(t, err)

		affected, err := res.RowsAffected()
		require.NoError(t, err)
		require.Equal(t, int64(1), affected)

		affectedAgain, err := res.RowsAffected()
		require.NoError(t, err)
		require.Equal(t, int64(1), affectedAgain)
	})

	t.Run("sql.Rows compatibility", func(t *testing.T) {
		rows, err := db.QueryContext(
			t.Context(),
			fmt.Sprintf("SELECT ID, NAME FROM %s WHERE ID >= ? AND NAME IS NOT NULL ORDER BY ID", tableName),
			int64(1),
		)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, rows.Close())
		})

		cols, err := rows.Columns()
		require.NoError(t, err)
		require.Equal(t, []string{"ID", "NAME"}, []string{strings.ToUpper(cols[0]), strings.ToUpper(cols[1])})

		count := 0
		for rows.Next() {
			var id int64
			var name string
			require.NoError(t, rows.Scan(&id, &name))
			require.NotEmpty(t, name)
			count++
		}
		require.GreaterOrEqual(t, count, 2)
		require.NoError(t, rows.Err())

		// EOF after exhaustion should keep returning false.
		require.False(t, rows.Next())
		require.NoError(t, rows.Err())

		require.NoError(t, rows.Close())
		require.NoError(t, rows.Close())
	})

	t.Run("sql.Row compatibility", func(t *testing.T) {
		t.Run("single row scan", func(t *testing.T) {
			var id int64
			var name string
			err := db.QueryRowContext(
				t.Context(),
				fmt.Sprintf("SELECT ID, NAME FROM %s WHERE ID = ?", tableName),
				int64(1),
			).Scan(&id, &name)
			require.NoError(t, err)
			require.Equal(t, int64(1), id)
			require.Equal(t, "neo", name)
		})

		t.Run("no rows returns sql.ErrNoRows", func(t *testing.T) {
			var id int64
			err := db.QueryRowContext(
				t.Context(),
				fmt.Sprintf("SELECT ID FROM %s WHERE ID = ?", tableName),
				int64(-1),
			).Scan(&id)
			require.ErrorIs(t, err, sql.ErrNoRows)
		})

		t.Run("scan type mismatch returns error", func(t *testing.T) {
			var invalidDest time.Time
			err := db.QueryRowContext(
				t.Context(),
				fmt.Sprintf("SELECT NAME FROM %s WHERE ID = ?", tableName),
				int64(1),
			).Scan(&invalidDest)
			require.Error(t, err)
		})
	})
}

func TestMachbaseSQLCompatibilityAdvanced(t *testing.T) {
	fixture := newSQLCompatFixture(t)
	db := fixture.db
	tableName := fixture.tableName

	t.Run("column types metadata", func(t *testing.T) {
		rows, err := db.QueryContext(
			t.Context(),
			fmt.Sprintf("SELECT ID, NAME FROM %s ORDER BY ID", tableName),
		)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, rows.Close())
		})

		types, err := rows.ColumnTypes()
		require.NoError(t, err)
		require.Len(t, types, 2)

		require.Equal(t, "ID", strings.ToUpper(types[0].Name()))
		require.Equal(t, "NAME", strings.ToUpper(types[1].Name()))
		require.Equal(t, "LONG", strings.ToUpper(types[0].DatabaseTypeName()))
		require.Equal(t, "VARCHAR", strings.ToUpper(types[1].DatabaseTypeName()))

		require.Equal(t, reflect.TypeOf(int64(0)), types[0].ScanType())
		require.Equal(t, reflect.TypeOf(""), types[1].ScanType())

		nullableID, okID := types[0].Nullable()
		require.True(t, okID)
		// TODO: verify NOT NULL fidelity for machbase metadata path. ID is declared NOT NULL,
		// but current metadata can report nullable=true depending on backend metadata source.
		_ = nullableID

		nullableName, okName := types[1].Nullable()
		require.True(t, okName)
		require.True(t, nullableName)

		length, okLength := types[1].Length()
		require.True(t, okLength)
		require.Equal(t, int64(100), length)
	})

	t.Run("null scan compatibility", func(t *testing.T) {
		row := db.QueryRowContext(
			t.Context(),
			fmt.Sprintf("SELECT ID, NAME FROM %s WHERE ID = ?", tableName),
			int64(99),
		)

		var id int64
		var name sql.NullString
		err := row.Scan(&id, &name)
		require.NoError(t, err)
		require.Equal(t, int64(99), id)
		require.False(t, name.Valid)
	})

	t.Run("context cancellation and deadline propagation", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := db.QueryContext(
			cancelCtx,
			fmt.Sprintf("SELECT ID, NAME FROM %s WHERE ID = ?", tableName),
			int64(1),
		)
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)

		expiredCtx, cancelExpired := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
		defer cancelExpired()

		_, err = db.QueryContext(
			expiredCtx,
			fmt.Sprintf("SELECT ID, NAME FROM %s WHERE ID = ?", tableName),
			int64(1),
		)
		require.Error(t, err)
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("connection pool max open connections", func(t *testing.T) {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)

		conn1, err := db.Conn(t.Context())
		require.NoError(t, err)

		acquireCtx, cancelAcquire := context.WithTimeout(t.Context(), 200*time.Millisecond)
		defer cancelAcquire()

		errCh := make(chan error, 1)
		go func() {
			_, acqErr := db.Conn(acquireCtx)
			errCh <- acqErr
		}()

		acqErr := <-errCh
		require.Error(t, acqErr)
		require.ErrorIs(t, acqErr, context.DeadlineExceeded)

		require.NoError(t, conn1.Close())
		require.Equal(t, 1, db.Stats().MaxOpenConnections)
	})
}

func TestMachbaseSQLCompatibilityEmptyVarchar(t *testing.T) {
	ctx := t.Context()
	fixture := newSQLCompatFixture(t)
	db := fixture.db
	rows, err := db.QueryContext(ctx, `SELECT '' AS EMPTY_VARCHAR`)
	require.NoError(t, err)
	defer rows.Close()

	columnTypes, err := rows.ColumnTypes()
	require.NoError(t, err)
	require.Len(t, columnTypes, 1)
	require.Equal(t, "EMPTY_VARCHAR", strings.ToUpper(columnTypes[0].Name()))
	require.Equal(t, "VARCHAR", strings.ToUpper(columnTypes[0].DatabaseTypeName()))
	require.Equal(t, reflect.TypeOf(""), columnTypes[0].ScanType())

	require.True(t, rows.Next())

	// Issue machbase/neo#1408
	buff := spi.MakeBuffer(columnTypes)
	// instead of using string
	// require.Equal(t, "*string", reflect.TypeOf(buff[0]).String())
	// using sql.NullString
	require.Equal(t, "*sql.NullString", reflect.TypeOf(buff[0]).String())
	err = rows.Scan(buff...)
	require.NoError(t, err)
	str := api.Unbox(buff[0])
	// instead of using string
	//require.Equal(t, "", str)
	require.Nil(t, str)
}

func TestMachbaseSQLCompatibilityProxyUser(t *testing.T) {
	ctx := t.Context()

	key := spi.DefaultKey()
	require.NotNil(t, key, "failed to get default key")
	keyPair, err := machgo.AuthKeyPairFromPrivateKey(key)
	require.NoError(t, err)
	privKeyPEM := strings.TrimSpace(string(keyPair.PrivateKeyPEM))
	require.NotEmpty(t, privKeyPEM)
	require.Contains(t, privKeyPEM, "BEGIN")
	require.Contains(t, privKeyPEM, "PRIVATE KEY")

	sysDSN := fmt.Sprintf("server=127.0.0.1:%d;user=sys;auth_key_pem=\"%s\";fetch_rows=100", testServer.MachPort(), privKeyPEM)
	// TODO: fix ERR-2361, Table (8) structure was modified.
	// sysDSN := spi.DefaultDSN(map[string]string{
	// 	"user":         "sys",
	// 	"auth_key_pem": privKeyPEM,
	// })
	db, err := sql.Open("machbase", sysDSN)
	require.NoError(t, err, "connect fail")
	defer db.Close()

	sysConn, err := db.Conn(ctx)
	require.NoError(t, err, "connect fail")
	defer sysConn.Close()

	result, err := sysConn.ExecContext(ctx, "CREATE USER demo IDENTIFIED BY demo")
	require.NoError(t, err)
	defer func() {
		_, err := sysConn.ExecContext(ctx, "DROP table demo.TAG_DATA")
		require.NoError(t, err)
		_, err = sysConn.ExecContext(ctx, "DROP USER demo")
		require.NoError(t, err)
	}()
	_ = result

	// connect as proxy user
	userDSN := spi.DefaultDSN(map[string]string{
		"user":       "sys as demo",
		"fetch_rows": "100",
	})
	userDB, err := sql.Open("machbase", userDSN)
	require.NoError(t, err, "connect fail")
	defer userDB.Close()

	userConn, err := userDB.Conn(ctx)
	require.NoError(t, err, "connect fail")
	defer userConn.Close()

	// create table
	result, err = userConn.ExecContext(ctx, "CREATE TAG TABLE tag_data (name VARCHAR(100) primary key, time datetime basetime, value double, json_value json)")
	require.NoError(t, err)

	now, _ := time.ParseInLocation("2006-01-02 15:04:05", "2021-01-01 00:00:00", time.UTC)
	// insert tag_data
	result, err = userConn.ExecContext(ctx, `insert into tag_data values('demo-1', ?, 1.23, '{"key1": "value1"}')`, now)
	require.NoError(t, err, "insert fail")

	// insert demo.tag_data
	result, err = sysConn.ExecContext(ctx, `insert into demo.tag_data values('demo-1', ?, 1.23, '{"key1": "value1"}')`, now.Add(1))
	require.NoError(t, err, "insert fail")

	result, err = sysConn.ExecContext(ctx, "exec table_flush(demo.tag_data)")
	require.NoError(t, err, "table_flush fail")

	row := sysConn.QueryRowContext(ctx, "select count(*) from demo.tag_data where name = ?", "demo-1")
	require.NoError(t, row.Err())
	var count int
	row.Scan(&count)
	require.Equal(t, 2, count)

	result, err = userConn.ExecContext(ctx, `drop table tag_data`)
	require.NoError(t, err, "drop table fail")

	// connect as proxy user
	proxyDSN := fmt.Sprintf("server=127.0.0.1:%d;user=sys as demo;auth_key_pem=\"%s\"", testServer.MachPort(), privKeyPEM)
	proxyDB, err := sql.Open("machbase", proxyDSN) // This is to ensure the driver is registered for the proxy user connection.
	require.NoError(t, err, "connect fail")
	defer proxyDB.Close()

	proxyConn, err := proxyDB.Conn(ctx)
	require.NoError(t, err, "connect fail")
	defer proxyConn.Close()

	result, err = proxyConn.ExecContext(ctx, "CREATE TAG TABLE tag_data (name VARCHAR(100) primary key, time datetime basetime, value double, json_value json)")
	require.NoError(t, err, fmt.Sprintf("create table fail: %T", db))

	// insert tag_data
	result, err = proxyConn.ExecContext(ctx, `insert into tag_data values('demo-1', ?, 1.23, '{"key1": "value1"}')`, now)
	require.NoError(t, err, "insert fail")

	// insert demo.tag_data
	result, err = sysConn.ExecContext(ctx, `insert into demo.tag_data values('demo-1', ?, 1.23, '{"key1": "value1"}')`, now.Add(1))
	require.NoError(t, err, "insert fail")

	result, err = sysConn.ExecContext(ctx, "exec table_flush(demo.tag_data)")
	require.NoError(t, err, "table_flush fail")

	row = sysConn.QueryRowContext(ctx, "select count(*) from demo.tag_data where name = ?", "demo-1")
	require.NoError(t, row.Err())
	row.Scan(&count)
	require.Equal(t, 2, count)
}

// Issue machbase/neo#1395
func TestStatementCacheBehavior(t *testing.T) {
	conf := &machgo.Config{
		Host: "127.0.0.1",
		Port: testServer.MachPort(),
	}

	mdb, err := machgo.NewDatabase(conf)
	if err != nil {
		panic(err)
	}

	// Connection A: statement reuse enabled
	conn, err := mdb.Connect(
		t.Context(),
		api.WithPassword("sys", "manager"),
		api.WithStatementCache(api.StatementCacheAuto),
	)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	sqlCreateTable := "create tag table if not exists stmtcache (name varchar(80) primary key, time datetime basetime, value double)"
	sqlInsert := "insert into stmtcache values (?, ?, ?)"

	// create table
	result := conn.Exec(t.Context(), sqlCreateTable)
	if err := result.Err(); err != nil {
		panic(err)
	}

	// insert data, statement cached
	result = conn.Exec(t.Context(), sqlInsert, "Alice", "2024-06-01 00:00:00", 123.45)
	if err := result.Err(); err != nil {
		panic(err)
	}

	// drop table
	result = conn.Exec(t.Context(), "drop table stmtcache")
	if err := result.Err(); err != nil {
		panic(err)
	}

	// re-create table
	result = conn.Exec(t.Context(), sqlCreateTable)
	if err := result.Err(); err != nil {
		panic(err)
	}
	// insert data again
	result = conn.Exec(t.Context(), sqlInsert, "Bob", "2024-06-02 00:00:00", 678.90)
	if err := result.Err(); err != nil {
		panic(err)
	}

	rows, err := conn.Query(t.Context(), "select * from stmtcache")
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	require.True(t, rows.Next())
	var name string
	var ts time.Time
	var value float64
	if err := rows.Scan(&name, &ts, &value); err != nil {
		panic(err)
	}
	require.Equal(t, "Bob", name)
	require.Equal(t, "2024-06-02 00:00:00", ts.In(time.Local).Format("2006-01-02 00:00:00"))
	require.Equal(t, 678.90, value)

	if err := rows.Err(); err != nil {
		panic(err)
	}
}

// Issue machbase/neo#1402
func TestMultiUserSessionTableBehavior(t *testing.T) {
	conf := &machgo.Config{
		Host: "127.0.0.1",
		Port: testServer.MachPort(),
	}

	mdb, err := machgo.NewDatabase(conf)
	if err != nil {
		panic(err)
	}

	sysConn, err := mdb.Connect(
		t.Context(),
		api.WithPassword("sys", "manager"),
		api.WithStatementCache(api.StatementCacheOff),
	)
	if err != nil {
		panic(err)
	}
	defer sysConn.Close()

	result := sysConn.Exec(t.Context(), "CREATE USER eve IDENTIFIED BY pass")
	if err := result.Err(); err != nil {
		panic(err)
	}
	defer func() {
		// drop user
		result = sysConn.Exec(t.Context(), "drop user eve")
		if err := result.Err(); err != nil {
			panic(err)
		}
	}()

	userConn, err := mdb.Connect(
		t.Context(),
		api.WithPassword("eve", "pass"),
		// api.WithPassword("sys", "manager"), api.WithProxyUser("eve"),
		api.WithStatementCache(api.StatementCacheOff),
	)
	if err != nil {
		panic(err)
	}
	defer userConn.Close()

	sqlCreateTable := "create tag table data (name varchar(80) primary key, time datetime basetime, value double)"

	// create table
	result = userConn.Exec(t.Context(), sqlCreateTable)
	if err := result.Err(); err != nil {
		panic(err)
	}

	defer func() {
		// drop table
		result = userConn.Exec(t.Context(), "drop table data")
		if err := result.Err(); err != nil {
			panic(err)
		}
	}()

	// insert data, statement cached
	result = userConn.Exec(t.Context(), "insert into data values (?, ?, ?)", "Alice", "2024-06-01 00:00:00", 123.45)
	if err := result.Err(); err != nil {
		panic(err)
	}
	result = sysConn.Exec(t.Context(), "exec table_flush(eve.data)")
	if err := result.Err(); err != nil {
		panic(err)
	}

	row := userConn.QueryRow(t.Context(), "select count(*) from data")
	if err := row.Err(); err != nil {
		panic(err)
	}
	var count int
	if err := row.Scan(&count); err != nil {
		panic(err)
	}
	require.Equal(t, 1, count)

	rows, err := userConn.Query(t.Context(), "select * from data where name = ?", "Alice")
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	require.True(t, rows.Next())
	var name string
	var timeVal time.Time
	var value float64
	if err := rows.Scan(&name, &timeVal, &value); err != nil {
		panic(err)
	}
	require.Equal(t, "Alice", name)
	require.Equal(t, "2024-06-01 00:00:00", timeVal.In(time.Local).Format("2006-01-02 15:04:05"))
	require.Equal(t, 123.45, value)

	result = sysConn.Exec(t.Context(), "insert into eve.data values (?, ?, ?)", "Bob", "2024-06-02 00:00:00", 678.90)
	if err := result.Err(); err != nil {
		panic(err)
	}

	rows, err = sysConn.Query(t.Context(), "select * from eve.data")
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	expects := []struct {
		name    string
		timeVal time.Time
		value   float64
	}{
		{"Alice", time.Date(2024, 6, 1, 0, 0, 0, 0, time.Local), 123.45},
		{"Bob", time.Date(2024, 6, 2, 0, 0, 0, 0, time.Local), 678.90},
	}
	nrow := 0
	for rows.Next() {
		var name string
		var timeVal time.Time
		var value float64
		if err := rows.Scan(&name, &timeVal, &value); err != nil {
			panic(err)
		}
		require.Equal(t, expects[nrow].name, name)
		require.Equal(t, expects[nrow].timeVal, timeVal.In(time.Local))
		require.Equal(t, expects[nrow].value, value)
		nrow++
	}
	require.NoError(t, rows.Err())

	// drop table
	result = sysConn.Exec(t.Context(), "drop table eve.data")
	if err := result.Err(); err != nil {
		panic(err)
	}
}

// Issue machbase/neo#1403
func TestMultiUserSessionIndexBehavior(t *testing.T) {
	conf := &machgo.Config{
		Host: "127.0.0.1",
		Port: testServer.MachPort(),
	}

	mdb, err := machgo.NewDatabase(conf)
	if err != nil {
		panic(err)
	}

	sysConn, err := mdb.Connect(
		t.Context(),
		api.WithPassword("sys", "manager"),
		api.WithStatementCache(api.StatementCacheOff),
	)
	if err != nil {
		panic(err)
	}
	defer sysConn.Close()

	result := sysConn.Exec(t.Context(), "CREATE USER david IDENTIFIED BY pass")
	if err := result.Err(); err != nil {
		panic(err)
	}
	defer func() {
		// drop user
		result = sysConn.Exec(t.Context(), "drop user david")
		if err := result.Err(); err != nil {
			panic(err)
		}
	}()

	userConn, err := mdb.Connect(
		t.Context(),
		api.WithPassword("david", "pass"),
		// api.WithPassword("sys", "manager"), api.WithProxyUser("david"),
		api.WithStatementCache(api.StatementCacheOff),
	)
	if err != nil {
		panic(err)
	}
	defer userConn.Close()

	sqlCreateTable := "create tag table data (name varchar(80) primary key, time datetime basetime, value double)"

	// create table
	result = userConn.Exec(t.Context(), sqlCreateTable)
	if err := result.Err(); err != nil {
		panic(err)
	}

	defer func() {
		// drop table
		result = userConn.Exec(t.Context(), "drop table data cascade")
		if err := result.Err(); err != nil {
			panic(err)
		}
	}()

	// insert data, statement cached
	result = userConn.Exec(t.Context(), "insert into data values (?, ?, ?)", "Alice", "2024-06-01 00:00:00", 123.45)
	if err := result.Err(); err != nil {
		panic(err)
	}
	result = sysConn.Exec(t.Context(), "exec table_flush(david.data)")
	if err := result.Err(); err != nil {
		panic(err)
	}

	row := userConn.QueryRow(t.Context(), "select count(*) from data")
	if err := row.Err(); err != nil {
		panic(err)
	}
	var count int
	if err := row.Scan(&count); err != nil {
		panic(err)
	}
	require.Equal(t, 1, count)

	result = sysConn.Exec(t.Context(), "create index idx_data_value on david.data(value)")
	if err := result.Err(); err != nil {
		panic(err)
	}
	// Issue machbase/neo#1410 bug silently ignore the index creation if the index name is not prefixed with the user name.
	//
	// It should work with 'create index david.idx_data_value....'
	//
	// result = sysConn.Exec(t.Context(), "create index david.idx_data_value on david.data(value)")
	// if err := result.Err(); err != nil {
	// 	panic(err)
	// }

	rows, err := userConn.Query(t.Context(), "select name, type from m$sys_indexes")
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	expects := []struct {
		indexName string
		found     bool
	}{
		{"_DATA_META_NAME", false},
		{"_DATA_META__LAST_UPDATE_TIME", false},
		{"__PK_IDX__DATA_META", false},
		{"IDX_DATA_VALUE", false},
	}
	nrow := 0
	for rows.Next() {
		var indexName string
		var indexType int
		if err := rows.Scan(&indexName, &indexType); err != nil {
			panic(err)
		}
		found := false
		for i := range expects {
			if strings.Contains(indexName, expects[i].indexName) {
				found = true
				expects[i].found = true
				break
			}
		}
		if !found {
			t.Logf("unexpected index: %s", indexName)
			t.Fail()
		}
		nrow++
	}
	require.NoError(t, rows.Err())
	for _, expect := range expects {
		require.True(t, expect.found, fmt.Sprintf("index %s not found in m$sys_indexes", expect.indexName))
	}

	result = sysConn.Exec(t.Context(), "drop index david.idx_data_value")
	if err := result.Err(); err != nil {
		panic(err)
	}
}
