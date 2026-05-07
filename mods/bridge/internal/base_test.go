package internal

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/machbase/neo-client/api"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func openTestConn(t *testing.T) *sql.Conn {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+filepath.Join(t.TempDir(), "internal.db")+"?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	conn, err := db.Conn(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestConnAndRows(t *testing.T) {
	ctx := context.Background()
	conn := NewConn(openTestConn(t))

	require.Panics(t, func() {
		conn.Prepare(ctx, "select 1")
	})

	result := conn.Exec(ctx, `CREATE TABLE example(id INTEGER PRIMARY KEY, name TEXT, created_at DATETIME)`)
	require.NoError(t, result.Err())
	require.EqualValues(t, 0, result.RowsAffected())
	require.Empty(t, result.Message())

	insertedAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	result = conn.Exec(ctx, `INSERT INTO example(id, name, created_at) VALUES(?, ?, ?)`, 1, "alpha", insertedAt)
	require.NoError(t, result.Err())
	require.EqualValues(t, 1, result.RowsAffected())

	rows, err := conn.Query(ctx, `SELECT id, name FROM example ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()
	require.True(t, rows.IsFetchable())
	require.Equal(t, int64(0), rows.RowsAffected())
	require.Equal(t, "success", rows.Message())

	cols, err := rows.Columns()
	require.NoError(t, err)
	require.Len(t, cols.Names(), 2)
	require.Equal(t, api.DataTypeInt64, cols[0].DataType)
	require.Equal(t, api.DataTypeString, cols[1].DataType)

	require.True(t, rows.Next())
	var id int64
	var name string
	require.NoError(t, rows.Scan(&id, &name))
	require.EqualValues(t, 1, id)
	require.Equal(t, "alpha", name)
	require.False(t, rows.Next())
	require.NoError(t, rows.Err())

	row := conn.QueryRow(ctx, `SELECT id, name, created_at FROM example WHERE id = ?`, 1)
	require.NoError(t, row.Err())
	rowCols, err := row.Columns()
	require.NoError(t, err)
	require.Len(t, rowCols.Names(), 3)

	var rowID int64
	var rowName string
	var rowCreatedAt time.Time
	require.NoError(t, row.Scan(&rowID, &rowName, &rowCreatedAt))
	require.EqualValues(t, 1, rowID)
	require.Equal(t, "alpha", rowName)
	require.True(t, insertedAt.Equal(rowCreatedAt))
	require.Equal(t, int64(0), row.RowsAffected())
	require.Empty(t, row.Message())

	missing := conn.QueryRow(ctx, `SELECT id FROM example WHERE id = ?`, 999)
	require.EqualError(t, missing.Err(), sql.ErrNoRows.Error())

	row = conn.QueryRow(ctx, `SELECT id FROM example WHERE id = ?`, 1)
	require.Error(t, row.Scan(&rowID, &rowName))

	_, err = conn.Appender(ctx, "example")
	require.EqualError(t, err, api.ErrNotImplemented("Appender").Error())

	_, err = conn.Explain(ctx, `SELECT * FROM example`, false)
	require.EqualError(t, err, api.ErrNotImplemented("Explain").Error())

	require.NoError(t, conn.Close())
}

func TestResultAndScanTypeHelpers(t *testing.T) {
	errResult := &Result{err: sql.ErrConnDone}
	require.EqualError(t, errResult.Err(), sql.ErrConnDone.Error())
	require.Equal(t, sql.ErrConnDone.Error(), errResult.Message())

	cases := map[string]api.DataType{
		"bool":            api.DataTypeBoolean,
		"sql.NullBool":    api.DataTypeBoolean,
		"int8":            api.DataTypeInt16,
		"sql.NullByte":    api.DataTypeInt16,
		"int16":           api.DataTypeInt16,
		"sql.NullInt16":   api.DataTypeInt16,
		"int32":           api.DataTypeInt32,
		"sql.NullInt32":   api.DataTypeInt32,
		"int64":           api.DataTypeInt64,
		"sql.NullInt64":   api.DataTypeInt64,
		"float32":         api.DataTypeFloat32,
		"float64":         api.DataTypeFloat64,
		"sql.NullFloat64": api.DataTypeFloat64,
		"string":          api.DataTypeString,
		"sql.NullString":  api.DataTypeString,
		"time.Time":       api.DataTypeDatetime,
		"sql.NullTime":    api.DataTypeDatetime,
		"[]byte":          api.DataTypeBinary,
		"sql.RawBytes":    api.DataTypeBinary,
		"*interface {}":   api.DataTypeString,
		"unknown":         api.DataTypeAny,
	}
	for input, want := range cases {
		require.Equal(t, want, scanTypeToDataType(input))
	}

	base := &SqlBridgeBase{}
	require.IsType(t, new(bool), base.NewScanType("sql.NullBool", ""))
	require.IsType(t, new(uint8), base.NewScanType("sql.NullByte", ""))
	require.IsType(t, new(float64), base.NewScanType("sql.NullFloat64", ""))
	require.IsType(t, new(int16), base.NewScanType("sql.NullInt16", ""))
	require.IsType(t, new(int32), base.NewScanType("sql.NullInt32", ""))
	require.IsType(t, new(int64), base.NewScanType("sql.NullInt64", ""))
	require.IsType(t, new(string), base.NewScanType("sql.NullString", ""))
	require.IsType(t, new(sql.NullTime), base.NewScanType("sql.NullTime", ""))
	require.IsType(t, new([]byte), base.NewScanType("sql.RawBytes", ""))
	require.IsType(t, new([]byte), base.NewScanType("[]uint8", ""))
	require.IsType(t, new(bool), base.NewScanType("bool", ""))
	require.IsType(t, new(int32), base.NewScanType("int32", ""))
	require.IsType(t, new(int64), base.NewScanType("int64", ""))
	require.IsType(t, new(string), base.NewScanType("string", ""))
	require.IsType(t, new(time.Time), base.NewScanType("time.Time", ""))
	require.Nil(t, base.NewScanType("unknown", ""))

	nullBool := &sql.NullBool{Bool: true, Valid: true}
	nullByte := &sql.NullByte{Byte: 2, Valid: true}
	nullFloat := &sql.NullFloat64{Float64: 1.25, Valid: true}
	nullInt16 := &sql.NullInt16{Int16: 16, Valid: true}
	nullInt32 := &sql.NullInt32{Int32: 32, Valid: true}
	nullInt64 := &sql.NullInt64{Int64: 64, Valid: true}
	nullString := &sql.NullString{String: "text", Valid: true}
	nullTime := &sql.NullTime{Time: time.Unix(0, 10), Valid: true}
	raw := sql.RawBytes("bytes")

	normalized := base.NormalizeType([]any{
		raw,
		nullBool,
		nullByte,
		nullFloat,
		nullInt16,
		nullInt32,
		nullInt64,
		nullString,
		nullTime,
		&sql.NullString{},
	})
	require.Equal(t, []byte("bytes"), normalized[0])
	require.Equal(t, true, normalized[1])
	require.EqualValues(t, 2, normalized[2])
	require.Equal(t, 1.25, normalized[3])
	require.EqualValues(t, 16, normalized[4])
	require.EqualValues(t, 32, normalized[5])
	require.EqualValues(t, 64, normalized[6])
	require.Equal(t, "text", normalized[7])
	require.Equal(t, nullTime.Time, normalized[8])
	require.Nil(t, normalized[9])

	wrapped := base.Conn(openTestConn(t))
	require.IsType(t, &Conn{}, wrapped)
}
