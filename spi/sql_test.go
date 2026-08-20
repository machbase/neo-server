package spi_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/machbase/neo-client/v2"
	client "github.com/machbase/neo-client/v2"
	"github.com/machbase/neo-client/v2/api"
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

	t.Run("transactions are supported", func(t *testing.T) {
		// TODO: implement transaction support in neo-client/driver (Begin/BeginTx, Commit, Rollback).
		tx, err := db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		require.NotNil(t, tx)
	})

	t.Run("named parameters are supported", func(t *testing.T) {
		_, err := db.ExecContext(
			t.Context(),
			fmt.Sprintf("UPDATE %s SET NAME = :name WHERE ID = 1", tableName),
			sql.Named("name", "neo"),
		)
		require.NoError(t, err)
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
	nullable, supportNullable := columnTypes[0].Nullable()
	require.False(t, nullable)
	require.True(t, supportNullable)

	require.True(t, rows.Next())

	// Issue machbase/neo#1408, fixed in v8.7.0
	// Column nullability is supported since v8.7.0, empty string can be scanned into string
	// but the previous implementation considered all columns are nullable, so the scan type should be *sql.NullString
	buff := spi.MakeBuffer(columnTypes)
	require.Equal(t, "*string", reflect.TypeOf(buff[0]).String())

	// TODO: remove this code after fix dbms-nfx#4071
	buff[0] = &sql.NullString{}
	//

	err = rows.Scan(buff...)
	require.NoError(t, err)

	str := client.Unbox(buff[0])
	require.Nil(t, str)
}

func TestMachbaseSQLCompatibilityProxyUser(t *testing.T) {
	ctx := t.Context()

	key := spi.DefaultKey()
	require.NotNil(t, key, "failed to get default key")
	keyPair, err := api.AuthKeyPairFromPrivateKey(key)
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
	// userDSN := fmt.Sprintf("host=127.0.0.1;port=%d;user=sys as demo;password=manager", testServer.MachPort())
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

	// TODO: issue machbase/neo#1445 meta table accessibility for proxy user
	// for _, metaTable := range []string{"_tag_data_meta", "demo._tag_data_meta", "machbasedb.demo._tag_data_meta"} {
	for _, metaTable := range []string{"_tag_data_meta", "demo._tag_data_meta"} {
		rows, err := proxyConn.QueryContext(ctx, fmt.Sprintf("select * from %s", metaTable))
		require.NoError(t, err)
		nrow := 0
		for rows.Next() {
			var id int64
			var name string
			err := rows.Scan(&id, &name)
			require.NoError(t, err)
			nrow++
		}
		rows.Close()
		require.Equal(t, 1, nrow)
	}

	for _, table := range []string{"demo._tag_data_meta", "machbasedb.demo._tag_data_meta"} {
		rows, err := sysConn.QueryContext(ctx, fmt.Sprintf("select * from %s", table))
		require.NoError(t, err)
		nrow := 0
		for rows.Next() {
			var id int64
			var name string
			err := rows.Scan(&id, &name)
			require.NoError(t, err)
			nrow++
		}
		rows.Close()
		require.Equal(t, 1, nrow)
	}
	row = sysConn.QueryRowContext(ctx, "select count(*) from demo.tag_data where name = ?", "demo-1")
	require.NoError(t, row.Err())
	row.Scan(&count)
	require.Equal(t, 2, count)
}

// Issue machbase/neo#1395
func TestStatementCacheBehavior(t *testing.T) {
	conn, err := spi.Connect(t.Context(), "sys")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	sqlCreateTable := "create tag table if not exists stmtcache (name varchar(80) primary key, time datetime basetime, value double)"
	sqlInsert := "insert into stmtcache values (?, ?, ?)"

	// create table
	result, err := conn.ExecContext(t.Context(), sqlCreateTable)
	_ = result
	if err != nil {
		panic(err)
	}

	// insert data, statement cached
	result, err = conn.ExecContext(t.Context(), sqlInsert, "Alice", "2024-06-01 00:00:00", 123.45)
	if err != nil {
		panic(err)
	}

	// drop table
	result, err = conn.ExecContext(t.Context(), "drop table stmtcache")
	if err != nil {
		panic(err)
	}

	// re-create table
	result, err = conn.ExecContext(t.Context(), sqlCreateTable)
	if err != nil {
		panic(err)
	}
	// insert data again
	result, err = conn.ExecContext(t.Context(), sqlInsert, "Bob", "2024-06-02 00:00:00", 678.90)
	if err != nil {
		panic(err)
	}

	rows, err := conn.QueryContext(t.Context(), "select * from stmtcache")
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
	sysConn, err := spi.Connect(t.Context(), "sys")
	if err != nil {
		panic(err)
	}
	defer sysConn.Close()

	result, err := sysConn.ExecContext(t.Context(), "CREATE USER eve IDENTIFIED BY pass")
	_ = result
	if err != nil {
		panic(err)
	}
	defer func() {
		// drop user
		result, err = sysConn.ExecContext(t.Context(), "drop user eve")
		if err != nil {
			panic(err)
		}
	}()

	userConn, err := spi.Connect(t.Context(), "eve")
	if err != nil {
		panic(err)
	}
	defer userConn.Close()

	sqlCreateTable := "create tag table data (name varchar(80) primary key, time datetime basetime, value double)"

	// create table
	result, err = userConn.ExecContext(t.Context(), sqlCreateTable)
	if err != nil {
		panic(err)
	}

	// insert data, statement cached
	result, err = userConn.ExecContext(t.Context(), "insert into data values (?, ?, ?)", "Alice", "2024-06-01 00:00:00", 123.45)
	if err != nil {
		panic(err)
	}
	result, err = sysConn.ExecContext(t.Context(), "exec table_flush(eve.data)")
	if err != nil {
		panic(err)
	}

	row := userConn.QueryRowContext(t.Context(), "select count(*) from data")
	if err := row.Err(); err != nil {
		panic(err)
	}
	var count int
	if err := row.Scan(&count); err != nil {
		panic(err)
	}
	require.Equal(t, 1, count)

	rows, err := userConn.QueryContext(t.Context(), "select * from data where name = ?", "Alice")
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

	result, err = sysConn.ExecContext(t.Context(), "insert into eve.data values (?, ?, ?)", "Bob", "2024-06-02 00:00:00", 678.90)
	_ = result
	if err != nil {
		panic(err)
	}

	rows, err = sysConn.QueryContext(t.Context(), "select * from eve.data")
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
	result, err = sysConn.ExecContext(t.Context(), "drop table eve.data")
	if err != nil {
		panic(err)
	}
}

// Issue machbase/neo#1403
// Issue machbase/neo#1410
// Issue machbase/neo#1418
func TestMultiUserSessionIndexBehavior(t *testing.T) {
	sysConn, err := spi.Connect(t.Context(), "sys")
	if err != nil {
		panic(err)
	}
	defer sysConn.Close()

	result, err := sysConn.ExecContext(t.Context(), "CREATE USER david IDENTIFIED BY pass")
	_ = result
	if err != nil {
		panic(err)
	}
	defer func() {
		// drop user
		result, err = sysConn.ExecContext(t.Context(), "drop user david")
		if err != nil {
			panic(err)
		}
	}()

	userConn, err := spi.Connect(t.Context(), "david")
	if err != nil {
		panic(err)
	}
	defer userConn.Close()

	sqlCreateTable := "create tag table data (name varchar(80) primary key, time datetime basetime, value double)"

	// create table
	result, err = userConn.ExecContext(t.Context(), sqlCreateTable)
	if err != nil {
		panic(err)
	}

	defer func() {
		// drop table
		result, err = userConn.ExecContext(t.Context(), "drop table data cascade")
		if err != nil {
			panic(err)
		}
	}()

	// insert data, statement cached
	result, err = userConn.ExecContext(t.Context(), "insert into data values (?, ?, ?)", "Alice", "2024-06-01 00:00:00", 123.45)
	if err != nil {
		panic(err)
	}
	result, err = sysConn.ExecContext(t.Context(), "exec table_flush(david.data)")
	if err != nil {
		panic(err)
	}

	row := userConn.QueryRowContext(t.Context(), "select count(*) from data")
	if err := row.Err(); err != nil {
		panic(err)
	}
	var count int
	if err := row.Scan(&count); err != nil {
		panic(err)
	}
	require.Equal(t, 1, count)

	for _, indexName := range []string{"idx_data_value", "david.idx_data_value"} {
		result, err = sysConn.ExecContext(t.Context(), fmt.Sprintf("create index %s on david.data(value)", indexName))
		if err != nil {
			panic(err)
		}
		time.Sleep(3 * time.Second)

		rows, err := userConn.QueryContext(t.Context(), "select name, type from m$sys_indexes")
		if err != nil {
			panic(err)
		}

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
			var foundIndexName string
			var indexType int
			if err := rows.Scan(&foundIndexName, &indexType); err != nil {
				panic(err)
			}
			found := false
			for i := range expects {
				if strings.Contains(foundIndexName, expects[i].indexName) {
					found = true
					expects[i].found = true
					break
				}
			}
			if !found {
				t.Logf("unexpected index: %s", foundIndexName)
				t.Fail()
			}
			nrow++
		}
		require.NoError(t, rows.Err())
		rows.Close()

		for _, expect := range expects {
			require.True(t, expect.found, fmt.Sprintf("%s not found in m$sys_indexes (created %s)", expect.indexName, indexName))
		}

		// Issue machbase/neo#1418
		result, err = sysConn.ExecContext(t.Context(), "drop index david.idx_data_value")
		if err != nil {
			panic(err)
		}
	}
}

func TestMachbaseSQLCompatibilityAffectedRows(t *testing.T) {
	dsn := fmt.Sprintf("server=127.0.0.1:%d;user=sys;password=manager;fetch_rows=100", testServer.MachPort())
	db, err := sql.Open("machbase", dsn)
	require.NoError(t, err)
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})
	require.NoError(t, db.PingContext(t.Context()))

	conn, err := db.Conn(t.Context())
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	result, err := conn.ExecContext(t.Context(), "CREATE TAG TABLE IF NOT EXISTS affected_rows_test (name VARCHAR(100) primary key, time DATETIME base time, value DOUBLE)")
	if err != nil {
		panic(err)
	}
	defer func() {
		_, err := conn.ExecContext(t.Context(), "DROP TABLE affected_rows_test")
		if err != nil {
			panic(err)
		}
	}()

	result, err = conn.ExecContext(t.Context(), "INSERT INTO affected_rows_test VALUES (?, ?, ?)", "Alice", "2024-06-01 00:00:00", 123.45)
	if err != nil {
		panic(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		panic(err)
	}
	require.Equal(t, int64(1), affected)

	// result, err = conn.ExecContext(t.Context(), "UPDATE affected_rows_test SET value = ? WHERE name = ? AND time = ?", 456.78, "Alice", "2024-06-01 00:00:00")
	// if err != nil {
	// 	panic(err)
	// }
	// affected, err = result.RowsAffected()
	// if err != nil {
	// 	panic(err)
	// }
	// require.Equal(t, int64(1), affected)

	result, err = conn.ExecContext(t.Context(), "DELETE FROM affected_rows_test WHERE name = ?", "Alice")
	if err != nil {
		panic(err)
	}
	affected, err = result.RowsAffected()
	if err != nil {
		panic(err)
	}
	require.Equal(t, int64(1), affected)
}

func TestMachbaseSQLCompatibilityDatabase(t *testing.T) {
	dsn := fmt.Sprintf("server=127.0.0.1:%d;user=sys;password=manager;fetch_rows=100", testServer.MachPort())
	db, err := sql.Open("machbase", dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})
	require.NoError(t, db.PingContext(t.Context()))

	_, err = db.ExecContext(t.Context(), "CREATE DATABASE IF NOT EXISTS testdb")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err = db.ExecContext(context.Background(), "USE MACHBASEDB")
		require.NoError(t, err)
		_, err = db.ExecContext(context.Background(), "DROP DATABASE testdb CASCADE")
		require.NoError(t, err)
	})

	dbConn, err := db.Conn(t.Context())
	require.NoError(t, err)
	defer dbConn.Close()

	_, err = dbConn.ExecContext(t.Context(), "USE testdb")
	require.NoError(t, err)
	_, err = dbConn.ExecContext(t.Context(), "CREATE TABLE test_table (id LONG PRIMARY KEY AUTO_INCREMENT, name VARCHAR(100))")
	require.NoError(t, err)

	dsn = fmt.Sprintf("%s;database=testdb", dsn)
	testdb, err := sql.Open("machbase", dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, testdb.Close())
	})

	testConn, err := testdb.Conn(t.Context())
	require.NoError(t, err)
	defer testConn.Close()

	result, err := testdb.ExecContext(t.Context(), "INSERT INTO test_table (name) VALUES ('test1')")
	require.NoError(t, err)
	affected, err := result.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)
	// lastInsertID, err := result.LastInsertId()
	// require.NoError(t, err)
	// require.Equal(t, int64(1), lastInsertID)

	result, err = dbConn.ExecContext(t.Context(), "INSERT INTO test_table (name) VALUES ('test2')")
	require.NoError(t, err)
	affected, err = result.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)
	// lastInsertID, err = result.LastInsertId()
	// require.NoError(t, err)
	// require.Equal(t, int64(2), lastInsertID)

	rows, err := testConn.QueryContext(t.Context(), "SELECT * FROM test_table")
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var id int64
		var name string
		err := rows.Scan(&id, &name)
		require.NoError(t, err)
		switch name {
		case "test1":
			require.Equal(t, int64(1), id)
		case "test2":
			require.Equal(t, int64(2), id)
		default:
			t.Fatalf("unexpected name: %s", name)
		}
	}
	require.NoError(t, rows.Err())
}

// TestPartialFetchStatementReuse is a regression test for MACHCLI-ERR-3008
// ("Protocol = Prepare, State = Fetch in progress"): a result set abandoned
// before the server sent its last chunk used to be returned to the statement
// cache with the server-side cursor still open, so every other reuse failed.
// Issue: machbase/neo#1459
func TestPartialFetchStatementReuse(t *testing.T) {
	dsn := fmt.Sprintf("server=127.0.0.1:%d;user=sys;password=manager;fetch_rows=10", testServer.MachPort())
	db, err := sql.Open("machbase", dsn)
	require.NoError(t, err)
	// a single connection forces the cached statement to be reused every time
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	tableName := fmt.Sprintf("PARTIAL_FETCH_%d", time.Now().UnixNano())
	_, err = db.ExecContext(t.Context(), fmt.Sprintf(`CREATE TAG TABLE %s ( NAME VARCHAR(100) PRIMARY KEY, TIME DATETIME BASE TIME, VALUE DOUBLE)`, tableName))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(t.Context(), fmt.Sprintf(`DROP TABLE %s`, tableName))
	})

	// more rows than fetch_rows, so reading a few rows leaves the cursor open
	for i := 0; i < 50; i++ {
		_, err = db.ExecContext(t.Context(),
			fmt.Sprintf(`INSERT INTO %s VALUES(?, ?, ?)`, tableName), fmt.Sprintf("name-%d", i), time.Now(), float64(i))
		require.NoError(t, err)
	}

	selectSql := fmt.Sprintf("SELECT NAME, TIME, VALUE FROM %s ORDER BY TIME", tableName)

	t.Run("query closed after partial fetch", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			rows, err := db.QueryContext(t.Context(), selectSql)
			require.NoError(t, err, "iteration %d", i)
			require.True(t, rows.Next())
			var name string
			var timeVal time.Time
			var value float64
			require.NoError(t, rows.Scan(&name, &timeVal, &value))
			require.NoError(t, rows.Close(), "iteration %d", i)
		}
	})

	t.Run("query row on multi row result", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			var name string
			var timeVal time.Time
			var value float64
			require.NoError(t, db.QueryRowContext(t.Context(), selectSql).Scan(&name, &timeVal, &value), "iteration %d", i)
		}
	})

	t.Run("prepared statement reused after partial fetch", func(t *testing.T) {
		stmt, err := db.PrepareContext(t.Context(), selectSql)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, stmt.Close())
		})
		for i := 0; i < 5; i++ {
			rows, err := stmt.QueryContext(t.Context())
			require.NoError(t, err, "iteration %d", i)
			require.True(t, rows.Next())
			var name string
			var timeVal time.Time
			var value float64
			require.NoError(t, rows.Scan(&name, &timeVal, &value))
			require.NoError(t, rows.Close(), "iteration %d", i)
		}
		// the statement must still be usable for a full scan
		rows, err := stmt.QueryContext(t.Context())
		require.NoError(t, err)
		defer rows.Close()
		count := 0
		for rows.Next() {
			count++
		}
		require.NoError(t, rows.Err())
		require.Equal(t, 50, count)
	})
}

type tagScanFixture struct {
	db        *sql.DB
	tableName string
}

func newTagScanFixture(t *testing.T) *tagScanFixture {
	t.Helper()

	dsn := fmt.Sprintf("server=127.0.0.1:%d;user=sys;password=manager", testServer.MachPort())
	db, err := sql.Open("machbase", dsn)
	require.NoError(t, err)
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	tableName := fmt.Sprintf("TAG_SCAN_%d", time.Now().UnixNano())
	_, err = db.ExecContext(t.Context(),
		fmt.Sprintf(`CREATE TABLE %s (ID LONG, NAME VARCHAR(100), VALUE DOUBLE, BINDATA BINARY)`, tableName))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), fmt.Sprintf(`DROP TABLE %s`, tableName))
	})

	insert := fmt.Sprintf(`INSERT INTO %s VALUES(?, ?, ?, ?)`, tableName)
	_, err = db.ExecContext(t.Context(), insert, int64(1), "neo", 1.5, []byte{0x01, 0x02, 0xfe})
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), insert, int64(2), "machbase", 2.5, []byte("mach"))
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), insert, int64(3), nil, 3.5, nil)
	require.NoError(t, err)

	return &tagScanFixture{db: db, tableName: tableName}
}

func (f *tagScanFixture) query(t *testing.T, sqlText string, args ...any) *sql.Rows {
	t.Helper()
	rows, err := f.db.QueryContext(t.Context(), fmt.Sprintf(sqlText, f.tableName), args...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rows.Close() })
	return rows
}

type tagScanRow struct {
	ID    int64   `db:"ID"`
	Name  *string `db:"NAME"`
	Value float64 `db:"VALUE"`
}

type pointerReceiverValuer struct {
	Name string `db:"NAME"`
}

func (*pointerReceiverValuer) Value() (driver.Value, error) {
	return nil, nil
}

func TestScanByTag(t *testing.T) {
	fixture := newTagScanFixture(t)

	t.Run("scan all", func(t *testing.T) {
		rows := fixture.query(t, `SELECT ID, NAME, VALUE FROM %s ORDER BY ID`)
		defer rows.Close()

		items, err := client.ScanAll[tagScanRow](rows)
		require.NoError(t, err)
		require.Len(t, items, 3)
		require.Equal(t, int64(1), items[0].ID)
		require.Equal(t, "neo", *items[0].Name)
		require.Equal(t, 1.5, items[0].Value)
		// a NULL VARCHAR arrives as a nil pointer
		require.Nil(t, items[2].Name)
		require.Equal(t, 3.5, items[2].Value)
	})

	t.Run("scan all into pointer slice", func(t *testing.T) {
		rows := fixture.query(t, `SELECT ID, NAME, VALUE FROM %s ORDER BY ID`)
		defer rows.Close()

		items, err := client.ScanAll[*tagScanRow](rows)
		require.NoError(t, err)
		require.Len(t, items, 3)
		require.Equal(t, "machbase", *items[1].Name)
	})

	t.Run("scan one", func(t *testing.T) {
		rows := fixture.query(t, `SELECT ID, NAME, VALUE FROM %s WHERE ID = ?`, int64(2))
		defer rows.Close()

		item, err := client.ScanOne[tagScanRow](rows)
		require.NoError(t, err)
		require.Equal(t, int64(2), item.ID)
		require.Equal(t, "machbase", *item.Name)
	})

	t.Run("scan one returns ErrNoRows", func(t *testing.T) {
		rows := fixture.query(t, `SELECT ID, NAME, VALUE FROM %s WHERE ID = ?`, int64(9999))
		defer rows.Close()

		_, err := client.ScanOne[tagScanRow](rows)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("scan struct on current row", func(t *testing.T) {
		rows := fixture.query(t, `SELECT ID, NAME, VALUE FROM %s ORDER BY ID`)
		defer rows.Close()

		require.True(t, rows.Next())
		var item tagScanRow
		require.NoError(t, client.ScanStruct(rows, &item))
		require.Equal(t, int64(1), item.ID)
	})

	t.Run("scan row", func(t *testing.T) {
		rows := fixture.query(t, `SELECT ID, NAME, VALUE FROM %s ORDER BY ID`)
		defer rows.Close()

		var item tagScanRow
		require.NoError(t, client.ScanRow(rows, &item))
		require.Equal(t, int64(1), item.ID)
	})

	t.Run("scan rows into slice", func(t *testing.T) {
		rows := fixture.query(t, `SELECT ID, NAME, VALUE FROM %s ORDER BY ID`)
		defer rows.Close()

		var items []tagScanRow
		require.NoError(t, client.ScanRows(rows, &items))
		require.Len(t, items, 3)
		require.Equal(t, int64(3), items[2].ID)
	})

	t.Run("column name match is case insensitive", func(t *testing.T) {
		type lowerCased struct {
			ID   int64  `db:"id"`
			Name string `db:"name"`
		}
		rows := fixture.query(t, `SELECT ID, NAME FROM %s WHERE ID = ?`, int64(1))
		defer rows.Close()

		item, err := client.ScanOne[lowerCased](rows)
		require.NoError(t, err)
		require.Equal(t, "neo", item.Name)
	})

	t.Run("json tag is used as fallback", func(t *testing.T) {
		type jsonTagged struct {
			ID   int64  `json:"id"`
			Name string `json:"name,omitempty"`
		}
		rows := fixture.query(t, `SELECT ID, NAME FROM %s WHERE ID = ?`, int64(1))
		defer rows.Close()

		item, err := client.ScanOne[jsonTagged](rows)
		require.NoError(t, err)
		require.Equal(t, int64(1), item.ID)
		require.Equal(t, "neo", item.Name)
	})

	t.Run("empty db and json tag names use field names", func(t *testing.T) {
		type tagOptions struct {
			ID   int64  `json:",omitempty"`
			Name string `db:",omitempty"`
		}
		rows := fixture.query(t, `SELECT ID, NAME FROM %s WHERE ID = ?`, int64(1))
		defer rows.Close()

		item, err := client.ScanOne[tagOptions](rows)
		require.NoError(t, err)
		require.Equal(t, int64(1), item.ID)
		require.Equal(t, "neo", item.Name)
	})

	t.Run("scalar destination", func(t *testing.T) {
		rows := fixture.query(t, `SELECT NAME FROM %s WHERE ID = ?`, int64(1))
		defer rows.Close()

		name, err := client.ScanOne[string](rows)
		require.NoError(t, err)
		require.Equal(t, "neo", name)
	})

	t.Run("map destination", func(t *testing.T) {
		rows := fixture.query(t, `SELECT ID, NAME FROM %s WHERE ID = ?`, int64(1))
		defer rows.Close()

		item, err := client.ScanOne[map[string]any](rows)
		require.NoError(t, err)
		require.Equal(t, int64(1), item["ID"])
		require.Equal(t, "neo", item["NAME"])
	})

	t.Run("map destination rejects duplicate columns", func(t *testing.T) {
		rows := fixture.query(t, `SELECT ID, ID FROM %s WHERE ID = ?`, int64(1))
		defer rows.Close()

		_, err := client.ScanOne[map[string]any](rows)
		require.ErrorIs(t, err, client.ErrScanDuplicateColumn)
	})

	t.Run("pointer receiver Valuer is currently flattened", func(t *testing.T) {
		rows := fixture.query(t, `SELECT NAME FROM %s WHERE ID = ?`, int64(1))
		defer rows.Close()

		item, err := client.ScanOne[pointerReceiverValuer](rows)
		require.NoError(t, err)
		require.Equal(t, "neo", item.Name)
	})

	t.Run("sql.Null field", func(t *testing.T) {
		type nullable struct {
			ID   int64             `db:"ID"`
			Name sql.Null[string]  `db:"NAME"`
			Val  sql.Null[float64] `db:"VALUE"`
		}
		rows := fixture.query(t, `SELECT ID, NAME, VALUE FROM %s WHERE ID = ?`, int64(3))
		defer rows.Close()

		item, err := client.ScanOne[nullable](rows)
		require.NoError(t, err)
		require.False(t, item.Name.Valid)
		require.True(t, item.Val.Valid)
		require.Equal(t, 3.5, item.Val.V)
	})

	t.Run("binary column scans into byte string null and map destinations", func(t *testing.T) {
		t.Run("bytes", func(t *testing.T) {
			type binaryRow struct {
				ID   int64  `db:"ID"`
				Data []byte `db:"BINDATA"`
			}
			rows := fixture.query(t, `SELECT ID, BINDATA FROM %s WHERE ID = ?`, int64(1))
			defer rows.Close()

			item, err := client.ScanOne[binaryRow](rows)
			require.NoError(t, err)
			require.Equal(t, int64(1), item.ID)
			require.Equal(t, []byte{0x01, 0x02, 0xfe}, item.Data)
		})

		t.Run("string", func(t *testing.T) {
			type binaryRow struct {
				Data string `db:"BINDATA"`
			}
			rows := fixture.query(t, `SELECT BINDATA FROM %s WHERE ID = ?`, int64(2))
			defer rows.Close()

			item, err := client.ScanOne[binaryRow](rows)
			require.NoError(t, err)
			require.Equal(t, "mach", item.Data)
		})

		t.Run("nullable wrappers", func(t *testing.T) {
			type binaryRow struct {
				Bytes sql.Null[[]byte] `db:"BINDATA"`
				Text  sql.NullString   `db:"BINDATA_TEXT"`
				Text2 sql.Null[string] `db:"BINDATA_TEXT2"`
			}
			rows := fixture.query(t, `SELECT BINDATA, BINDATA AS BINDATA_TEXT, BINDATA AS BINDATA_TEXT2 FROM %s WHERE ID = ?`, int64(2))
			defer rows.Close()

			item, err := client.ScanOne[binaryRow](rows)
			require.NoError(t, err)
			require.True(t, item.Bytes.Valid)
			require.Equal(t, []byte("mach"), item.Bytes.V)
			require.True(t, item.Text.Valid)
			require.Equal(t, "mach", item.Text.String)
			require.True(t, item.Text2.Valid)
			require.Equal(t, "mach", item.Text2.V)

			rows = fixture.query(t, `SELECT BINDATA, BINDATA AS BINDATA_TEXT, BINDATA AS BINDATA_TEXT2 FROM %s WHERE ID = ?`, int64(3))
			defer rows.Close()

			item, err = client.ScanOne[binaryRow](rows)
			require.NoError(t, err)
			require.False(t, item.Bytes.Valid)
			require.False(t, item.Text.Valid)
			require.False(t, item.Text2.Valid)
		})

		t.Run("nullable pointer", func(t *testing.T) {
			type binaryRow struct {
				Data *[]byte `db:"BINDATA"`
			}
			rows := fixture.query(t, `SELECT BINDATA FROM %s WHERE ID = ?`, int64(3))
			defer rows.Close()

			item, err := client.ScanOne[binaryRow](rows)
			require.NoError(t, err)
			require.Nil(t, item.Data)
		})

		t.Run("map", func(t *testing.T) {
			rows := fixture.query(t, `SELECT BINDATA FROM %s WHERE ID = ?`, int64(1))
			defer rows.Close()

			item, err := client.ScanOne[map[string]any](rows)
			require.NoError(t, err)
			require.Equal(t, []byte{0x01, 0x02, 0xfe}, item["BINDATA"])
		})
	})

	t.Run("embedded struct is flattened", func(t *testing.T) {
		type identity struct {
			ID int64 `db:"ID"`
		}
		type embedded struct {
			identity
			Name *string `db:"NAME"`
		}
		rows := fixture.query(t, `SELECT ID, NAME FROM %s WHERE ID = ?`, int64(2))
		defer rows.Close()

		item, err := client.ScanOne[embedded](rows)
		require.NoError(t, err)
		require.Equal(t, int64(2), item.ID)
		require.Equal(t, "machbase", *item.Name)
	})
}

func TestScanByTagStrictness(t *testing.T) {
	fixture := newTagScanFixture(t)

	t.Run("untagged field is excluded", func(t *testing.T) {
		type untagged struct {
			ID   int64 `db:"ID"`
			Name string
		}
		rows := fixture.query(t, `SELECT ID, NAME FROM %s WHERE ID = ?`, int64(1))
		defer rows.Close()

		_, err := client.ScanOne[untagged](rows)
		require.ErrorIs(t, err, client.ErrScanNoMatchedField)
	})

	t.Run("name mapper enables implicit mapping", func(t *testing.T) {
		type untagged struct {
			ID   int64
			Name string
		}
		rows := fixture.query(t, `SELECT ID, NAME FROM %s WHERE ID = ?`, int64(1))
		defer rows.Close()

		item, err := client.ScanOne[untagged](rows, client.WithNameMapper(client.NameMapperIdentity()))
		require.NoError(t, err)
		require.Equal(t, "neo", item.Name)
	})

	t.Run("name mapper closures do not share mappings", func(t *testing.T) {
		type untagged struct {
			ID int64
		}
		mapper := func(column string) func(string) string {
			return func(string) string { return column }
		}

		firstRows := fixture.query(t, `SELECT ID AS FIRST FROM %s WHERE ID = ?`, int64(1))
		first, err := client.ScanOne[untagged](firstRows, client.WithNameMapper(mapper("FIRST")))
		require.NoError(t, err)
		require.Equal(t, int64(1), first.ID)
		require.NoError(t, firstRows.Close())

		secondRows := fixture.query(t, `SELECT ID AS SECOND FROM %s WHERE ID = ?`, int64(2))
		second, err := client.ScanOne[untagged](secondRows, client.WithNameMapper(mapper("SECOND")))
		require.NoError(t, err)
		require.Equal(t, int64(2), second.ID)
	})

	t.Run("struct without any tag", func(t *testing.T) {
		type noTag struct {
			ID int64
		}
		rows := fixture.query(t, `SELECT ID FROM %s WHERE ID = ?`, int64(1))
		defer rows.Close()

		_, err := client.ScanOne[noTag](rows)
		require.ErrorIs(t, err, client.ErrScanNoMappedField)
		require.Contains(t, err.Error(), "WithNameMapper")
	})

	t.Run("unmatched column is rejected", func(t *testing.T) {
		type idOnly struct {
			ID int64 `db:"ID"`
		}
		rows := fixture.query(t, `SELECT ID, NAME FROM %s WHERE ID = ?`, int64(1))
		defer rows.Close()

		_, err := client.ScanOne[idOnly](rows)
		require.ErrorIs(t, err, client.ErrScanNoMatchedField)
		require.Contains(t, err.Error(), "NAME")
	})

	t.Run("unmatched column is allowed with WithLaxColumns", func(t *testing.T) {
		type idOnly struct {
			ID int64 `db:"ID"`
		}
		rows := fixture.query(t, `SELECT ID, NAME FROM %s WHERE ID = ?`, int64(1))
		defer rows.Close()

		item, err := client.ScanOne[idOnly](rows, client.WithLaxColumns())
		require.NoError(t, err)
		require.Equal(t, int64(1), item.ID)
	})

	t.Run("unmatched field is rejected", func(t *testing.T) {
		type extra struct {
			ID    int64  `db:"ID"`
			Extra string `db:"NOT_A_COLUMN"`
		}
		rows := fixture.query(t, `SELECT ID FROM %s WHERE ID = ?`, int64(1))
		defer rows.Close()

		_, err := client.ScanOne[extra](rows)
		require.ErrorIs(t, err, client.ErrScanNoMatchedColumn)
	})

	t.Run("unmatched field is allowed with WithLaxFields", func(t *testing.T) {
		type extra struct {
			ID    int64  `db:"ID"`
			Extra string `db:"NOT_A_COLUMN"`
		}
		rows := fixture.query(t, `SELECT ID FROM %s WHERE ID = ?`, int64(1))
		defer rows.Close()

		item, err := client.ScanOne[extra](rows, client.WithLaxFields())
		require.NoError(t, err)
		require.Equal(t, int64(1), item.ID)
		require.Empty(t, item.Extra)
	})

	t.Run("excluded field", func(t *testing.T) {
		type excluded struct {
			ID    int64  `db:"ID"`
			Cache string `db:"-"`
		}
		rows := fixture.query(t, `SELECT ID FROM %s WHERE ID = ?`, int64(1))
		defer rows.Close()

		item, err := client.ScanOne[excluded](rows)
		require.NoError(t, err)
		require.Equal(t, int64(1), item.ID)
	})

	t.Run("destination must be a pointer", func(t *testing.T) {
		rows := fixture.query(t, `SELECT ID FROM %s WHERE ID = ?`, int64(1))
		defer rows.Close()

		require.True(t, rows.Next())
		err := client.ScanStruct(rows, tagScanRow{})
		require.ErrorIs(t, err, client.ErrScanDestNotPointer)
	})
}

func TestScanByTagStreaming(t *testing.T) {
	fixture := newTagScanFixture(t)

	t.Run("scan each", func(t *testing.T) {
		rows := fixture.query(t, `SELECT ID, NAME, VALUE FROM %s ORDER BY ID`)
		defer rows.Close()

		var ids []int64
		require.NoError(t, client.ScanEach(rows, func(r tagScanRow) error {
			ids = append(ids, r.ID)
			return nil
		}))
		require.Equal(t, []int64{1, 2, 3}, ids)
	})

	t.Run("scan each stops on error", func(t *testing.T) {
		rows := fixture.query(t, `SELECT ID, NAME, VALUE FROM %s ORDER BY ID`)
		defer rows.Close()

		stop := errors.New("stop")
		count := 0
		err := client.ScanEach(rows, func(r tagScanRow) error {
			count++
			return stop
		})
		require.ErrorIs(t, err, stop)
		require.Equal(t, 1, count)
	})

	t.Run("cursor", func(t *testing.T) {
		rows := fixture.query(t, `SELECT ID, NAME, VALUE FROM %s ORDER BY ID`)
		defer rows.Close()

		cursor, err := client.NewCursor[tagScanRow](rows)
		require.NoError(t, err)
		var ids []int64
		for cursor.Next() {
			ids = append(ids, cursor.Value().ID)
		}
		require.NoError(t, cursor.Err())
		require.Equal(t, []int64{1, 2, 3}, ids)
	})

	t.Run("max rows", func(t *testing.T) {
		rows := fixture.query(t, `SELECT ID, NAME, VALUE FROM %s ORDER BY ID`)
		defer rows.Close()

		_, err := client.ScanAll[tagScanRow](rows, client.WithMaxRows(2))
		require.ErrorIs(t, err, client.ErrScanTooManyRows)
		require.Contains(t, err.Error(), "ScanEach")
	})

	t.Run("max rows disabled", func(t *testing.T) {
		rows := fixture.query(t, `SELECT ID, NAME, VALUE FROM %s ORDER BY ID`)
		defer rows.Close()

		items, err := client.ScanAll[tagScanRow](rows, client.WithMaxRows(0))
		require.NoError(t, err)
		require.Len(t, items, 3)
	})

	t.Run("max rows on ScanRows", func(t *testing.T) {
		rows := fixture.query(t, `SELECT ID, NAME, VALUE FROM %s ORDER BY ID`)
		defer rows.Close()

		var items []tagScanRow
		err := client.ScanRows(rows, &items, client.WithMaxRows(1))
		require.ErrorIs(t, err, client.ErrScanTooManyRows)
	})

	t.Run("rows stay open for the caller", func(t *testing.T) {
		rows := fixture.query(t, `SELECT ID, NAME, VALUE FROM %s ORDER BY ID`)

		_, err := client.ScanAll[tagScanRow](rows)
		require.NoError(t, err)
		// the helper must not have closed rows; Close is the caller's responsibility
		require.NoError(t, rows.Close())
	})
}

func TestScanByTagQueryHelpers(t *testing.T) {
	fixture := newTagScanFixture(t)

	t.Run("select", func(t *testing.T) {
		items, err := client.Select[tagScanRow](t.Context(), fixture.db,
			fmt.Sprintf(`SELECT ID, NAME, VALUE FROM %s ORDER BY ID`, fixture.tableName))
		require.NoError(t, err)
		require.Len(t, items, 3)
	})

	t.Run("get", func(t *testing.T) {
		item, err := client.Get[tagScanRow](t.Context(), fixture.db,
			fmt.Sprintf(`SELECT ID, NAME, VALUE FROM %s WHERE ID = ?`, fixture.tableName), int64(2))
		require.NoError(t, err)
		require.Equal(t, int64(2), item.ID)
	})

	t.Run("get returns ErrNoRows", func(t *testing.T) {
		_, err := client.Get[tagScanRow](t.Context(), fixture.db,
			fmt.Sprintf(`SELECT ID, NAME, VALUE FROM %s WHERE ID = ?`, fixture.tableName), int64(9999))
		require.ErrorIs(t, err, sql.ErrNoRows)
	})
}

func TestNamedArgs(t *testing.T) {
	fixture := newTagScanFixture(t)

	t.Run("from map", func(t *testing.T) {
		args, err := client.NamedArgs(map[string]any{"id": int64(1), "name": "neo"})
		require.NoError(t, err)
		require.Equal(t, []any{
			sql.Named("id", int64(1)),
			sql.Named("name", "neo"),
		}, args)
	})

	t.Run("from struct", func(t *testing.T) {
		type cond struct {
			ID    int64  `db:"id"`
			Name  string `db:"name"`
			Cache string `db:"-"`
		}
		args, err := client.NamedArgs(cond{ID: 1, Name: "neo"})
		require.NoError(t, err)
		require.Equal(t, []any{
			sql.Named("id", int64(1)),
			sql.Named("name", "neo"),
		}, args)
	})

	t.Run("from struct pointer", func(t *testing.T) {
		type cond struct {
			ID int64 `db:"id"`
		}
		args, err := client.NamedArgs(&cond{ID: 7})
		require.NoError(t, err)
		require.Equal(t, []any{sql.Named("id", int64(7))}, args)
	})

	t.Run("rejects unsupported source", func(t *testing.T) {
		_, err := client.NamedArgs(42)
		require.Error(t, err)
		_, err = client.NamedArgs(nil)
		require.Error(t, err)
	})

	t.Run("named query", func(t *testing.T) {
		supported, err := client.SupportsNamedParameters(t.Context(), fixture.db)
		require.NoError(t, err)
		t.Logf("named parameters supported: %v", supported)

		args, err := client.NamedArgs(map[string]any{"id": int64(2)})
		require.NoError(t, err)

		query := fmt.Sprintf(`SELECT ID, NAME, VALUE FROM %s WHERE ID = :id`, fixture.tableName)
		rows, err := fixture.db.QueryContext(t.Context(), query, args...)
		if !supported {
			if err == nil {
				defer rows.Close()
			}
			require.Error(t, err)
			require.ErrorIs(t, err, client.ErrNamedParamsUnsupported)
			return
		}
		require.NoError(t, err)
		defer rows.Close()

		item, err := client.ScanOne[tagScanRow](rows)
		require.NoError(t, err)
		require.Equal(t, int64(2), item.ID)
		require.Equal(t, "machbase", *item.Name)
	})

	t.Run("named query matches parameter names case insensitively", func(t *testing.T) {
		supported, err := client.SupportsNamedParameters(t.Context(), fixture.db)
		require.NoError(t, err)
		if !supported {
			t.Skip("server does not expose parameter name metadata")
		}
		type cond struct {
			ID int64 `db:"ID"`
		}
		args, err := client.NamedArgs(cond{ID: 1})
		require.NoError(t, err)

		query := fmt.Sprintf(`SELECT ID, NAME, VALUE FROM %s WHERE ID = :id`, fixture.tableName)
		rows, err := fixture.db.QueryContext(t.Context(), query, args...)
		require.NoError(t, err)
		defer rows.Close()

		item, err := client.ScanOne[tagScanRow](rows)
		require.NoError(t, err)
		require.Equal(t, int64(1), item.ID)
	})
}

type tagScanDateTimeFixture struct {
	db        *sql.DB
	tableName string
}

func newTagScanDateTimeFixture(t *testing.T) *tagScanDateTimeFixture {
	t.Helper()

	dsn := fmt.Sprintf("server=127.0.0.1:%d;user=sys;password=manager", testServer.MachPort())
	db, err := sql.Open("machbase", dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	tableName := fmt.Sprintf("TAG_SCAN_DT_%d", time.Now().UnixNano())
	_, err = db.ExecContext(t.Context(),
		fmt.Sprintf(`CREATE TABLE %s (ID LONG, TIME DATETIME, NAME VARCHAR(100))`, tableName))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), fmt.Sprintf(`DROP TABLE %s`, tableName))
	})

	insert := fmt.Sprintf(`INSERT INTO %s VALUES(?, ?, ?)`, tableName)
	_, err = db.ExecContext(t.Context(), insert, int64(1), time.Date(2026, 8, 20, 12, 34, 56, 0, time.UTC), "neo")
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), insert, int64(2), nil, "machbase")
	require.NoError(t, err)

	return &tagScanDateTimeFixture{db: db, tableName: tableName}
}

func (f *tagScanDateTimeFixture) query(t *testing.T, sqlText string, args ...any) *sql.Rows {
	t.Helper()
	rows, err := f.db.QueryContext(t.Context(), fmt.Sprintf(sqlText, f.tableName), args...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rows.Close() })
	return rows
}

// TestScanByTagDateTimeOptions covers the DATETIME field conversion options
// (timeformat=/tz=, WithDateTime) and the ColumnTypes()-based bind-time
// DATETIME confirmation, against a real server so *sql.Rows.ColumnTypes()
// reports genuine scan types.
func TestScanByTagDateTimeOptions(t *testing.T) {
	fixture := newTagScanDateTimeFixture(t)

	// canonical is the TIME value of row ID=1 as the server actually returns
	// it, used as the ground truth for every other subtest's expectations
	// instead of re-deriving it from the originally inserted time.Time.
	var canonical time.Time
	{
		type row struct {
			Time time.Time `db:"TIME"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		item, err := client.ScanOne[row](rows)
		require.NoError(t, err)
		canonical = item.Time
	}

	// rawCanonical is the same value read through plain database/sql, with no
	// timeformat/tz customization applied. It's the value a tagged
	// sql.NullTime/sql.Null[time.Time] field's tz gets reapplied onto, since
	// their raw-pointer scan (native sql.Scanner) populates them with it first.
	var rawCanonical time.Time
	{
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		require.True(t, rows.Next())
		require.NoError(t, rows.Scan(&rawCanonical))
	}

	t.Run("timeformat and tz on string field", func(t *testing.T) {
		type row struct {
			Time string `db:"TIME,timeformat=2006-01-02,tz=UTC"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		item, err := client.ScanOne[row](rows)
		require.NoError(t, err)
		require.Equal(t, canonical.In(time.UTC).Format("2006-01-02"), item.Time)
	})

	t.Run("timeformat=ms on int64 field", func(t *testing.T) {
		type row struct {
			Time int64 `db:"TIME,timeformat=ms"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		item, err := client.ScanOne[row](rows)
		require.NoError(t, err)
		require.Equal(t, canonical.UnixMilli(), item.Time)
	})

	t.Run("default unit ns on int64 field without tag", func(t *testing.T) {
		type row struct {
			Time int64 `db:"TIME"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		item, err := client.ScanOne[row](rows)
		require.NoError(t, err)
		require.Equal(t, canonical.UnixNano(), item.Time)
	})

	t.Run("default format and Local tz without tag or WithDateTime", func(t *testing.T) {
		type row struct {
			Time string `db:"TIME"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		item, err := client.ScanOne[row](rows)
		require.NoError(t, err)
		require.Equal(t, canonical.In(time.Local).Format("2006-01-02 15:04:05.999"), item.Time)
	})

	t.Run("WithDateTime supplies the default for untagged fields", func(t *testing.T) {
		type row struct {
			Time string `db:"TIME"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		item, err := client.ScanOne[row](rows, client.WithDateTime(time.RFC3339, "UTC"))
		require.NoError(t, err)
		require.Equal(t, canonical.In(time.UTC).Format(time.RFC3339), item.Time)
	})

	t.Run("per-field tag overrides WithDateTime default", func(t *testing.T) {
		type row struct {
			Time string `db:"TIME,timeformat=2006-01-02,tz=UTC"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		item, err := client.ScanOne[row](rows, client.WithDateTime(time.RFC3339, "America/New_York"))
		require.NoError(t, err)
		require.Equal(t, canonical.In(time.UTC).Format("2006-01-02"), item.Time)
	})

	t.Run("pointer field resets to nil on NULL", func(t *testing.T) {
		type row struct {
			Time *int64 `db:"TIME,timeformat=us"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(2))
		item, err := client.ScanOne[row](rows)
		require.NoError(t, err)
		require.Nil(t, item.Time)

		rows = fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		item, err = client.ScanOne[row](rows)
		require.NoError(t, err)
		require.NotNil(t, item.Time)
		require.Equal(t, canonical.UnixMicro(), *item.Time)
	})

	t.Run("pointer string field resets to nil on NULL", func(t *testing.T) {
		type row struct {
			Time *string `db:"TIME,timeformat=2006-01-02,tz=UTC"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(2))
		item, err := client.ScanOne[row](rows)
		require.NoError(t, err)
		require.Nil(t, item.Time)

		rows = fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		item, err = client.ScanOne[row](rows)
		require.NoError(t, err)
		require.NotNil(t, item.Time)
		require.Equal(t, canonical.In(time.UTC).Format("2006-01-02"), *item.Time)
	})

	t.Run("pointer time.Time field resets to nil on NULL", func(t *testing.T) {
		type row struct {
			Time *time.Time `db:"TIME,tz=UTC"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(2))
		item, err := client.ScanOne[row](rows)
		require.NoError(t, err)
		require.Nil(t, item.Time)

		rows = fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		item, err = client.ScanOne[row](rows)
		require.NoError(t, err)
		require.NotNil(t, item.Time)
		require.True(t, item.Time.Equal(canonical))
		require.Equal(t, time.UTC, item.Time.Location())
	})

	t.Run("non-pointer field errors on NULL", func(t *testing.T) {
		type row struct {
			Time int64 `db:"TIME,timeformat=ms"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(2))
		_, err := client.ScanOne[row](rows)
		require.ErrorIs(t, err, client.ErrScanNullNotSupported)
	})

	t.Run("invalid option combination rejected", func(t *testing.T) {
		type row struct {
			Time int64 `db:"TIME,timeformat=2006-01-02"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		_, err := client.ScanOne[row](rows)
		require.ErrorIs(t, err, client.ErrScanInvalidTagOption)
	})

	t.Run("timeformat=ms on string field renders numeric epoch", func(t *testing.T) {
		type row struct {
			Time string `db:"TIME,timeformat=ms"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		item, err := client.ScanOne[row](rows)
		require.NoError(t, err)
		require.Equal(t, strconv.FormatInt(canonical.UnixMilli(), 10), item.Time)
	})

	t.Run("time.Time field applies default tz when untagged", func(t *testing.T) {
		type row struct {
			At time.Time `db:"TIME"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		item, err := client.ScanOne[row](rows, client.WithDateTime("", "UTC"))
		require.NoError(t, err)
		require.True(t, item.At.Equal(canonical))
		require.Equal(t, time.UTC, item.At.Location())
	})

	t.Run("non-datetime columns still use the raw-pointer fast path", func(t *testing.T) {
		type row struct {
			ID   int64  `db:"ID"`
			Name string `db:"NAME"`
		}
		rows := fixture.query(t, `SELECT ID, NAME FROM %s WHERE ID = ?`, int64(1))
		item, err := client.ScanOne[row](rows)
		require.NoError(t, err)
		require.Equal(t, "neo", item.Name)
	})

	t.Run("explicit timeformat tag on a confirmed non-DATETIME column errors at bind time", func(t *testing.T) {
		type row struct {
			Name string `db:"NAME,timeformat=2006-01-02"`
		}
		rows := fixture.query(t, `SELECT NAME FROM %s WHERE ID = ?`, int64(1))
		_, err := client.ScanOne[row](rows)
		require.ErrorIs(t, err, client.ErrScanInvalidTagOption)
	})

	t.Run("invalid WithDateTime tz rejected", func(t *testing.T) {
		type row struct {
			Time string `db:"TIME"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		_, err := client.ScanOne[row](rows, client.WithDateTime("", "Not/AZone"))
		require.ErrorIs(t, err, client.ErrScanInvalidTagOption)
	})

	// sql.Null[T]/sql.NullTime are structs, not string/int64/time.Time, but
	// ambiguity with a plain VARCHAR/INTEGER column is resolved the same way
	// as for plain fields: at scan time by checking whether the value is
	// actually a time.Time, with ColumnTypes() only as a bind-time fast path.
	// So sql.NullTime/sql.Null[time.Time]/sql.Null[int64]/sql.NullString/
	// sql.Null[string] all honor timeformat=/tz=, and a bare `db:"TIME"` (no
	// options) never fails eligibility for them: NULL support always works.

	t.Run("sql.Null[time.Time] applies tz on the scanned value", func(t *testing.T) {
		type row struct {
			Time sql.Null[time.Time] `db:"TIME,tz=UTC"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(2))
		item, err := client.ScanOne[row](rows)
		require.NoError(t, err)
		require.False(t, item.Time.Valid)

		rows = fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		item, err = client.ScanOne[row](rows)
		require.NoError(t, err)
		require.True(t, item.Time.Valid)
		require.True(t, item.Time.V.Equal(rawCanonical))
		require.Equal(t, time.UTC, item.Time.V.Location())
	})

	t.Run("sql.Null[time.Time] applies the default tz when untagged", func(t *testing.T) {
		type row struct {
			Time sql.Null[time.Time] `db:"TIME"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		item, err := client.ScanOne[row](rows, client.WithDateTime("", "UTC"))
		require.NoError(t, err)
		require.True(t, item.Time.Valid)
		require.True(t, item.Time.V.Equal(rawCanonical))
		require.Equal(t, time.UTC, item.Time.V.Location())
	})

	t.Run("sql.NullTime applies tz on the scanned value", func(t *testing.T) {
		type row struct {
			Time sql.NullTime `db:"TIME,tz=UTC"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(2))
		item, err := client.ScanOne[row](rows)
		require.NoError(t, err)
		require.False(t, item.Time.Valid)

		rows = fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		item, err = client.ScanOne[row](rows)
		require.NoError(t, err)
		require.True(t, item.Time.Valid)
		require.True(t, item.Time.Time.Equal(rawCanonical))
		require.Equal(t, time.UTC, item.Time.Time.Location())
	})

	t.Run("sql.Null[string] applies timeformat and tz like a plain string field", func(t *testing.T) {
		type row struct {
			Time sql.Null[string] `db:"TIME,timeformat=2006-01-02,tz=UTC"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(2))
		item, err := client.ScanOne[row](rows)
		require.NoError(t, err)
		require.False(t, item.Time.Valid)

		rows = fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		item, err = client.ScanOne[row](rows)
		require.NoError(t, err)
		require.True(t, item.Time.Valid)
		require.Equal(t, canonical.In(time.UTC).Format("2006-01-02"), item.Time.V)
	})

	t.Run("sql.NullString applies the default format and tz when untagged", func(t *testing.T) {
		type row struct {
			Time sql.NullString `db:"TIME"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		item, err := client.ScanOne[row](rows, client.WithDateTime(time.RFC3339, "UTC"))
		require.NoError(t, err)
		require.True(t, item.Time.Valid)
		require.Equal(t, canonical.In(time.UTC).Format(time.RFC3339), item.Time.String)
	})

	t.Run("sql.Null[int64] applies timeformat like a plain int64 field", func(t *testing.T) {
		type row struct {
			Time sql.Null[int64] `db:"TIME,timeformat=ms"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(2))
		item, err := client.ScanOne[row](rows)
		require.NoError(t, err)
		require.False(t, item.Time.Valid)

		rows = fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		item, err = client.ScanOne[row](rows)
		require.NoError(t, err)
		require.True(t, item.Time.Valid)
		require.Equal(t, canonical.UnixMilli(), item.Time.V)
	})

	t.Run("sql.Null[int64] defaults to ns unit when untagged", func(t *testing.T) {
		type row struct {
			Time sql.Null[int64] `db:"TIME"`
		}
		rows := fixture.query(t, `SELECT TIME FROM %s WHERE ID = ?`, int64(1))
		item, err := client.ScanOne[row](rows)
		require.NoError(t, err)
		require.True(t, item.Time.Valid)
		require.Equal(t, canonical.UnixNano(), item.Time.V)
	})
}

func exampleTagTable() (*sql.DB, string, func()) {
	dsn := fmt.Sprintf("server=127.0.0.1:%d;user=sys;password=manager", testServer.MachPort())
	db, err := sql.Open("machbase", dsn)
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	table := fmt.Sprintf("TAG_EXAMPLE_%d", time.Now().UnixNano())
	if _, err := db.ExecContext(ctx,
		fmt.Sprintf(`CREATE TABLE %s (ID LONG, NAME VARCHAR(100), VALUE DOUBLE)`, table)); err != nil {
		panic(err)
	}
	insert := fmt.Sprintf(`INSERT INTO %s VALUES(?, ?, ?)`, table)
	for _, row := range []struct {
		id    int64
		name  any
		value float64
	}{
		{1, "neo", 1.5},
		{2, "machbase", 2.5},
		{3, nil, 3.5},
	} {
		if _, err := db.ExecContext(ctx, insert, row.id, row.name, row.value); err != nil {
			panic(err)
		}
	}
	return db, table, func() {
		_, _ = db.ExecContext(ctx, fmt.Sprintf(`DROP TABLE %s`, table))
		_ = db.Close()
	}
}

type exampleRow struct {
	ID    int64   `db:"ID"`
	Name  *string `db:"NAME"`
	Value float64 `db:"VALUE"`
}

func (r exampleRow) name() string {
	if r.Name == nil {
		return "<null>"
	}
	return *r.Name
}

func Example_scanAll() {
	db, table, cleanup := exampleTagTable()
	defer cleanup()

	rows, err := db.QueryContext(context.Background(),
		fmt.Sprintf(`SELECT ID, NAME, VALUE FROM %s ORDER BY ID`, table))
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	items, err := client.ScanAll[exampleRow](rows)
	if err != nil {
		panic(err)
	}
	for _, item := range items {
		fmt.Printf("%d %s %.1f\n", item.ID, item.name(), item.Value)
	}

	// Output:
	// 1 neo 1.5
	// 2 machbase 2.5
	// 3 <null> 3.5
}

func Example_scanEach() {
	db, table, cleanup := exampleTagTable()
	defer cleanup()

	rows, err := db.QueryContext(context.Background(),
		fmt.Sprintf(`SELECT ID, NAME, VALUE FROM %s ORDER BY ID`, table))
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	// ScanEach keeps only one row alive at a time, so it is safe for large results.
	var total float64
	err = client.ScanEach(rows, func(item exampleRow) error {
		total += item.Value
		return nil
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("total %.1f\n", total)

	// Output:
	// total 7.5
}

func Example_cursor() {
	db, table, cleanup := exampleTagTable()
	defer cleanup()

	rows, err := db.QueryContext(context.Background(),
		fmt.Sprintf(`SELECT ID, NAME, VALUE FROM %s ORDER BY ID`, table))
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	cursor, err := client.NewCursor[exampleRow](rows)
	if err != nil {
		panic(err)
	}
	for cursor.Next() {
		fmt.Println(cursor.Value().ID)
	}
	if err := cursor.Err(); err != nil {
		panic(err)
	}

	// Output:
	// 1
	// 2
	// 3
}

func Example_select() {
	db, table, cleanup := exampleTagTable()
	defer cleanup()

	ctx := context.Background()
	items, err := client.Select[exampleRow](ctx,
		db, fmt.Sprintf(`SELECT ID, NAME, VALUE FROM %s ORDER BY ID`, table))
	if err != nil {
		panic(err)
	}
	fmt.Println(len(items))

	item, err := client.Get[exampleRow](ctx,
		db, fmt.Sprintf(`SELECT ID, NAME, VALUE FROM %s WHERE ID = ?`, table), int64(2))
	if err != nil {
		panic(err)
	}
	fmt.Println(item.name())

	// Output:
	// 3
	// machbase
}

func Example_select_WithDateTime() {
	db, _, cleanup := exampleTagTable()
	defer cleanup()

	ctx := context.Background()
	items, err := client.Select[time.Time](ctx, db, `SELECT TO_DATE(?) TS`,
		"2026-02-03 16:05:06",
		client.WithDateTime("2006/01/02 15:04:05", "Asia/Seoul"))
	if err != nil {
		panic(err)
	}
	fmt.Println("time.Time:", items[0])

	items2, err := client.Select[string](ctx, db, `SELECT TO_DATE(?) TS`,
		"2026-02-03 16:05:06",
		client.WithDateTime("2006/01/02 15:04:05", "Asia/Seoul"))
	if err != nil {
		panic(err)
	}
	fmt.Println("string:", items2[0])

	item3, err := client.Select[sql.NullString](ctx, db, `SELECT TO_DATE(?) TS`,
		"2026-02-03 16:05:06",
		client.WithDateTime("2006/01/02 15:04:05", "Asia/Seoul"))
	if err != nil {
		panic(err)
	}
	fmt.Println("sql.NullString:", item3[0].String)

	// Output:
	// time.Time: 2026-02-03 16:05:06 +0900 KST
	// string: 2026/02/03 16:05:06
	// sql.NullString: 2026/02/03 16:05:06
}

func Example_namedArgs() {
	db, table, cleanup := exampleTagTable()
	defer cleanup()

	type condition struct {
		MinValue float64 `db:"min_value"`
		MaxValue float64 `db:"max_value"`
	}
	args, err := client.NamedArgs(condition{MinValue: 2.0, MaxValue: 3.0})
	if err != nil {
		panic(err)
	}

	// The server resolves the :name placeholders; the SQL text is never rewritten.
	query := fmt.Sprintf(`SELECT ID, NAME, VALUE FROM %s WHERE VALUE BETWEEN :min_value AND :max_value`, table)
	items, err := client.Select[exampleRow](context.Background(), db, query, args...)
	if err != nil {
		panic(err)
	}
	for _, item := range items {
		fmt.Printf("%s %.1f\n", item.name(), item.Value)
	}

	// Output:
	// machbase 2.5
}
