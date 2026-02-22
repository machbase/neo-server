package testsuite

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machgo"
	"github.com/stretchr/testify/require"
)

func FetchRowsChunk(t *testing.T, db api.Database, ctx context.Context) {
	goDB, ok := db.(*machgo.Database)
	if !ok {
		t.Skip("fetch chunk test is only for machgo database")
		return
	}

	host, port := machgoEndpoint(t, goDB)
	const fetchRows = 100
	const totalRows = 10000

	fetchDB, err := machgo.NewDatabase(&machgo.Config{
		Host:         host,
		Port:         port,
		TrustUsers:   map[string]string{"sys": "manager"},
		MaxOpenConn:  -1,
		MaxOpenQuery: -1,
		FetchRows:    fetchRows,
	})
	require.NoError(t, err)
	defer fetchDB.Close()

	conn, err := fetchDB.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err)
	defer conn.Close()

	tableName := fmt.Sprintf("fetch_chunk_%d", time.Now().UnixNano())
	result := conn.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s (
			time datetime,
			value integer
		)`, tableName))
	require.NoError(t, result.Err())

	defer func() {
		_ = conn.Exec(context.Background(), fmt.Sprintf("DROP TABLE %s", tableName)).Err()
	}()

	appender, err := conn.Appender(ctx, tableName)
	require.NoError(t, err)

	baseTS := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < totalRows; i++ {
		err = appender.Append(baseTS.Add(time.Millisecond*time.Duration(i)), i)
		require.NoError(t, err)
	}
	success, fail, err := appender.Close()
	require.NoError(t, err)
	require.Equal(t, int64(totalRows), success)
	require.Equal(t, int64(0), fail)

	result = conn.Exec(ctx, fmt.Sprintf("EXEC table_flush(%s)", tableName))
	require.NoError(t, result.Err())

	rows, err := conn.Query(ctx, fmt.Sprintf("SELECT * FROM %s ORDER BY time", tableName))
	require.NoError(t, err)
	defer rows.Close()

	machRows, ok := rows.(*machgo.Rows)
	require.True(t, ok)

	fetched := 0
	batchEndCount := 0
	for rows.Next() {
		var ts time.Time
		var v int32
		require.NoError(t, rows.Scan(&ts, &v))
		require.Equal(t, int32(fetched), v)
		fetched++
		if isMachgoBatchEnd(machRows) {
			batchEndCount++
		}
	}
	require.NoError(t, rows.Err())
	require.Equal(t, totalRows, fetched)

	expectedBatchCount := totalRows / fetchRows
	require.GreaterOrEqual(t, batchEndCount, expectedBatchCount-1)
	require.LessOrEqual(t, batchEndCount, expectedBatchCount+1)
}

func machgoEndpoint(t *testing.T, db *machgo.Database) (string, int) {
	t.Helper()
	v := reflect.ValueOf(db)
	require.Equal(t, reflect.Pointer, v.Kind())
	e := v.Elem()

	hostField := e.FieldByName("host")
	require.True(t, hostField.IsValid())
	require.Equal(t, reflect.String, hostField.Kind())

	portField := e.FieldByName("port")
	require.True(t, portField.IsValid())
	require.Equal(t, reflect.Int, portField.Kind())

	return hostField.String(), int(portField.Int())
}

func isMachgoBatchEnd(rows *machgo.Rows) bool {
	v := reflect.ValueOf(rows)
	if !v.IsValid() || v.Kind() != reflect.Pointer || v.IsNil() {
		return false
	}
	rowsElem := v.Elem()
	stmtField := rowsElem.FieldByName("stmt")
	if !stmtField.IsValid() || stmtField.IsNil() {
		return false
	}
	stmtElem := stmtField.Elem()
	handleField := stmtElem.FieldByName("handle")
	if !handleField.IsValid() || handleField.IsNil() {
		return false
	}
	handleElem := handleField.Elem()

	rowPosField := handleElem.FieldByName("rowPos")
	rowsField := handleElem.FieldByName("rows")
	if !rowPosField.IsValid() || !rowsField.IsValid() {
		return false
	}
	return rowPosField.Int() == 0 && rowsField.Len() == 0
}
