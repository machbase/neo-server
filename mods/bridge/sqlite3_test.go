package bridge_test

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	"github.com/machbase/neo-server/mods/bridge"
	"github.com/stretchr/testify/require"
)

func TestSqlite3(t *testing.T) {
	CONN_NAME := "sqlite"

	define := bridge.Define{
		Type: bridge.SQLITE,
		Name: CONN_NAME,
		Path: "../../tmp/connector_sqlite3.db",
	}

	bridge.Register(&define)
	defer bridge.Unregister(CONN_NAME)

	cr, err := bridge.GetConnector(CONN_NAME)
	require.Nil(t, err)
	require.NotNil(t, cr)
	require.Equal(t, bridge.SQLITE, cr.Type())
	require.Equal(t, CONN_NAME, cr.Name())

	c := cr.(bridge.SqlConnector)
	conn, err := c.Connect(context.TODO())
	require.Nil(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	_, err = conn.ExecContext(context.TODO(), `CREATE TABLE IF NOT EXISTS example(id INTEGER NOT NULL PRIMARY KEY, name TEXT, age TEXT, address TEXT, UNIQUE(name))`)
	require.Nil(t, err)

	rows, err := conn.QueryContext(context.TODO(), `SELECT * FROM example`)
	require.Nil(t, err)
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
	for rows.NextResultSet() {

	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	require.Nil(t, err)
}
