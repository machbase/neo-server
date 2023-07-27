package bridge_test

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"testing"

	"github.com/machbase/neo-server/mods/bridge"
	"github.com/stretchr/testify/require"
)

func TestSqlite3(t *testing.T) {
	BRIDGE_NAME := "sqlite"

	define := bridge.Define{
		Type: bridge.SQLITE,
		Name: BRIDGE_NAME,
		Path: "file:../../tmp/connector_sqlite3.db?cache=shared",
	}

	bridge.Register(&define)
	defer bridge.Unregister(BRIDGE_NAME)

	br, err := bridge.GetBridge(BRIDGE_NAME)
	require.Nil(t, err)
	require.NotNil(t, br)
	_, ok := br.(bridge.SqlBridge)
	require.True(t, ok)
	require.Equal(t, BRIDGE_NAME, br.Name())

	ctx := context.TODO()

	sqlBr := br.(bridge.SqlBridge)
	conn, err := sqlBr.Connect(ctx)
	require.Nil(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	_, err = conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS example(id INTEGER NOT NULL PRIMARY KEY, name TEXT, age TEXT, address TEXT, UNIQUE(name))`)
	require.Nil(t, err)

	result := conn.QueryRowContext(ctx, `SELECT count(*) from example`)
	require.NotNil(t, result)
	var count int
	result.Scan(&count)

	_, err = conn.ExecContext(ctx, fmt.Sprintf(`INSERT INTO example VALUES(%d, 'hong-%d', '12', 'address for %d')`, count+1, count+1, count+1))
	require.Nil(t, err)

	rows, err := conn.QueryContext(context.TODO(), `SELECT * FROM example`)
	require.Nil(t, err)
	defer rows.Close()
	colTypes, err := rows.ColumnTypes()
	require.Nil(t, err)
	require.Equal(t, "id", colTypes[0].Name())
	require.Equal(t, "name", colTypes[1].Name())
	require.Equal(t, "age", colTypes[2].Name())
	require.Equal(t, "address", colTypes[3].Name())
	require.Equal(t, "INTEGER", colTypes[0].DatabaseTypeName())
	require.Equal(t, "TEXT", colTypes[1].DatabaseTypeName())
	require.Equal(t, "TEXT", colTypes[2].DatabaseTypeName())
	require.Equal(t, "TEXT", colTypes[3].DatabaseTypeName())
	require.Equal(t, reflect.TypeOf(sql.NullInt64{}), colTypes[0].ScanType())
	require.Equal(t, reflect.TypeOf(sql.NullString{}), colTypes[1].ScanType())
	require.Equal(t, reflect.TypeOf(sql.NullString{}), colTypes[2].ScanType())
	require.Equal(t, reflect.TypeOf(sql.NullString{}), colTypes[3].ScanType())
	for rows.Next() {
		var id int
		var name, age, address string
		rows.Scan(&id, &name, &age, &address)
		fmt.Println(id, name, age, address)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	require.Nil(t, err)
}
