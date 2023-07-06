package connector_test

import (
	"context"
	"testing"

	"github.com/machbase/neo-server/mods/connector"
	"github.com/stretchr/testify/require"
)

func TestSqlite3(t *testing.T) {
	CONN_NAME := "sqlite"

	define := connector.Define{
		Type: connector.SQLITE3,
		Name: CONN_NAME,
		Path: "../../tmp/connector_sqlite3.db",
	}

	connector.Register(&define)

	cr, err := connector.NewConnector(CONN_NAME)
	require.Nil(t, err)
	require.NotNil(t, cr)
	require.Equal(t, connector.SQLITE3, cr.Type())
	require.Equal(t, CONN_NAME, cr.Name())
	defer cr.Close()

	c := cr.(connector.SqlConnector)
	conn, err := c.Connect(context.TODO())
	require.Nil(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	_, err = conn.ExecContext(context.TODO(), `CREATE TABLE IF NOT EXISTS example(id INTEGER NOT NULL PRIMARY KEY, name TEXT, age TEXT, address TEXT, UNIQUE(name))`)
	require.Nil(t, err)
}
