package testsuite

import (
	"context"
	"testing"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-server/v8/api/machrpc"
	"github.com/stretchr/testify/require"
)

func QueryRow(t *testing.T, db api.Database, ctx context.Context) {
	if _, ok := db.(*machrpc.Client); ok {
		t.Skip("skip QueryRow test for RPC database, because it does not support QueryRow")
	}
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	row := conn.QueryRow(ctx, "SELECT * from tag_data WHERE name='_not_exist_'")
	require.EqualError(t, row.Err(), "sql: no rows in result set")
	var result int
	err = row.Scan(&result)
	require.EqualError(t, err, "sql: no rows in result set")
	columns, err := row.Columns()
	require.NoError(t, err)

	expectedColumns := []string{
		"NAME", "TIME", "VALUE", "SHORT_VALUE", "USHORT_VALUE",
		"INT_VALUE", "UINT_VALUE", "LONG_VALUE", "ULONG_VALUE",
		"STR_VALUE", "JSON_VALUE", "IPV4_VALUE", "IPV6_VALUE",
	}
	require.Equal(t, len(expectedColumns), len(columns))
	for i, col := range columns {
		require.Equal(t, expectedColumns[i], col.Name)
	}
}
