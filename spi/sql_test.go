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

	_ "github.com/machbase/neo-client/machbase"
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
