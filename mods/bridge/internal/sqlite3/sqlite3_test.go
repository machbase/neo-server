package sqlite3_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api"
	bridgepkg "github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/bridge/internal"
	"github.com/machbase/neo-server/v8/mods/bridge/internal/sqlite3"
	"github.com/stretchr/testify/require"
)

func TestSqlite(t *testing.T) {
	ctx := t.Context()

	br := sqlite3.New("test", ":memory:")

	err := br.BeforeRegister()
	require.NoError(t, err)
	defer br.AfterUnregister()

	sqlConn, err := br.Connect(ctx)
	require.NoError(t, err)

	conn := internal.NewConn(sqlConn)
	defer conn.Close()

	result := conn.Exec(ctx, `CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)`)
	require.NoError(t, result.Err())

	conn.Exec(ctx, `INSERT INTO test VALUES (?, ?)`, 1, "foo")
	conn.Exec(ctx, `INSERT INTO test VALUES (?, ?)`, 2, "bar")

	beginCalled := false
	endCalled := false
	nextCalled := 0
	expectNames := []string{"foo", "bar"}
	q := api.Query{
		Begin: func(q *api.Query) {
			beginCalled = true
		},
		Next: func(q *api.Query, rownum int64) bool {
			nextCalled++
			values, _ := q.Columns().MakeBuffer()
			q.Scan(values...)
			require.Equal(t, 2, len(values))
			require.Equal(t, int64(rownum), *(values[0].(*int64)))
			require.Equal(t, expectNames[rownum-1], *(values[1].(*string)))
			return true
		},
		End: func(q *api.Query) {
			endCalled = true
		},
	}
	err = q.Execute(ctx, conn, `SELECT * FROM test order by id`)
	require.NoError(t, err)
	require.True(t, beginCalled)
	require.True(t, endCalled)
	require.Equal(t, 2, nextCalled)

	row := conn.QueryRow(ctx, `select count(*) from test`)
	require.NoError(t, row.Err())
	var count int64
	err = row.Scan(&count)
	require.NoError(t, err)
	require.Equal(t, int64(2), count)

	rows, err := conn.Query(ctx, `select * from test where id = ?`, 1)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var id int64
		var name string
		err = rows.Scan(&id, &name)
		require.NoError(t, err)
		require.Equal(t, int64(1), id)
		require.Equal(t, "foo", name)
	}
}

func TestSqliteSupportedTypes(t *testing.T) {
	ctx := t.Context()

	br := sqlite3.New("test", ":memory:")
	require.NoError(t, br.BeforeRegister())
	defer br.AfterUnregister()

	sqlConn, err := br.Connect(ctx)
	require.NoError(t, err)
	defer sqlConn.Close()

	conn := internal.NewConn(sqlConn)
	defer conn.Close()

	createdAt := time.Date(2026, 3, 14, 5, 29, 1, 0, time.UTC)

	result := conn.Exec(ctx, `CREATE TABLE test_supported (
		id INTEGER PRIMARY KEY,
		event_bool BOOLEAN,
		event_integer INTEGER,
		event_real REAL,
		event_text TEXT,
		event_blob BLOB,
		event_datetime DATETIME
	)`)
	require.NoError(t, result.Err())

	result = conn.Exec(ctx, `INSERT INTO test_supported(id, event_bool, event_integer, event_real, event_text, event_blob, event_datetime) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		1,
		true,
		42,
		3.25,
		"sqlite-text",
		[]byte{0x0a, 0x0b, 0x0c},
		createdAt,
	)
	require.NoError(t, result.Err())

	result = conn.Exec(ctx, `INSERT INTO test_supported(id, event_bool, event_integer, event_real, event_text, event_blob, event_datetime) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		2,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	require.NoError(t, result.Err())

	rows, err := sqlConn.QueryContext(ctx, `SELECT event_bool, event_integer, event_real, event_text, event_blob, event_datetime FROM test_supported WHERE id = 1`)
	require.NoError(t, err)
	defer rows.Close()

	columns, err := rows.ColumnTypes()
	require.NoError(t, err)
	require.Len(t, columns, 6)

	fields := make([]any, len(columns))
	for i, column := range columns {
		fields[i] = br.NewScanType(column.ScanType().String(), column.DatabaseTypeName())
	}

	require.True(t, rows.Next())
	require.NoError(t, rows.Scan(fields...))
	require.False(t, rows.Next())
	require.NoError(t, rows.Err())

	boolValue, ok := fields[0].(*sql.NullBool)
	require.True(t, ok)
	require.True(t, boolValue.Valid)
	require.True(t, boolValue.Bool)

	intValue, ok := fields[1].(*sql.NullInt64)
	require.True(t, ok)
	require.True(t, intValue.Valid)
	require.Equal(t, int64(42), intValue.Int64)

	realValue, ok := fields[2].(*sql.NullFloat64)
	require.True(t, ok)
	require.True(t, realValue.Valid)
	require.InDelta(t, 3.25, realValue.Float64, 0.0001)

	textValue, ok := fields[3].(*sql.NullString)
	require.True(t, ok)
	require.True(t, textValue.Valid)
	require.Equal(t, "sqlite-text", textValue.String)

	blobValue, ok := fields[4].(*[]byte)
	require.True(t, ok)
	require.Equal(t, []byte{0x0a, 0x0b, 0x0c}, *blobValue)

	timeValue, ok := fields[5].(*sql.NullTime)
	require.True(t, ok)
	require.True(t, timeValue.Valid)
	require.Equal(t, createdAt.UTC(), timeValue.Time.UTC())

	values := make([]any, len(fields))
	for i, v := range fields {
		values[i] = bridgepkg.UnboxValueToNative(v)
	}
	require.Equal(t, true, values[0])
	require.Equal(t, int64(42), values[1])
	convertedReal, ok := values[2].(float64)
	require.True(t, ok)
	require.InDelta(t, 3.25, convertedReal, 0.0001)
	require.Equal(t, "sqlite-text", values[3])
	require.Equal(t, []byte{0x0a, 0x0b, 0x0c}, values[4])
	convertedTime, ok := values[5].(time.Time)
	require.True(t, ok)
	require.Equal(t, createdAt.UTC(), convertedTime.UTC())

	nullRows, err := sqlConn.QueryContext(ctx, `SELECT event_bool, event_integer, event_real, event_text, event_blob, event_datetime FROM test_supported WHERE id = 2`)
	require.NoError(t, err)
	defer nullRows.Close()

	nullColumns, err := nullRows.ColumnTypes()
	require.NoError(t, err)
	require.Len(t, nullColumns, 6)

	nullFields := make([]any, len(nullColumns))
	for i, column := range nullColumns {
		nullFields[i] = br.NewScanType(column.ScanType().String(), column.DatabaseTypeName())
	}

	require.True(t, nullRows.Next())
	require.NoError(t, nullRows.Scan(nullFields...))
	require.False(t, nullRows.Next())
	require.NoError(t, nullRows.Err())

	nullBoolValue, ok := nullFields[0].(*sql.NullBool)
	require.True(t, ok)
	require.False(t, nullBoolValue.Valid)

	nullIntValue, ok := nullFields[1].(*sql.NullInt64)
	require.True(t, ok)
	require.False(t, nullIntValue.Valid)

	nullRealValue, ok := nullFields[2].(*sql.NullFloat64)
	require.True(t, ok)
	require.False(t, nullRealValue.Valid)

	nullTextValue, ok := nullFields[3].(*sql.NullString)
	require.True(t, ok)
	require.False(t, nullTextValue.Valid)

	nullBlobValue, ok := nullFields[4].(*[]byte)
	require.True(t, ok)
	require.Nil(t, *nullBlobValue)

	nullTimeValue, ok := nullFields[5].(*sql.NullTime)
	require.True(t, ok)
	require.False(t, nullTimeValue.Valid)

	nullValues := make([]any, len(nullFields))
	for i, v := range nullFields {
		nullValues[i] = bridgepkg.UnboxValueToNative(v)
	}
	require.Nil(t, nullValues[0])
	require.Nil(t, nullValues[1])
	require.Nil(t, nullValues[2])
	require.Nil(t, nullValues[3])
	require.Equal(t, []byte{}, nullValues[4])
	require.Nil(t, nullValues[5])
}
