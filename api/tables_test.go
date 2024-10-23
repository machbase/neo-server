package api_test

import (
	"context"
	"testing"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/api/machsvr"
	"github.com/stretchr/testify/require"
)

func TestExists(t *testing.T) {
	var db api.Database

	if machsvr_db, err := machsvr.NewDatabase(); err != nil {
		t.Log("Error", err.Error())
		t.Fail()
	} else {
		db = api.NewDatabase(machsvr_db)
	}

	ctx := context.TODO()
	conn, err := db.Connect(ctx, api.WithTrustUser("sys"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	for _, table_name := range []string{"tag_data", "sys.tag_data", "machbasedb.sys.tag_data"} {
		// table exists
		exists, err := api.ExistsTable(ctx, conn, table_name)
		require.NoError(t, err, "exists table %q fail", table_name)
		require.True(t, exists, "table %q not exists", table_name)

		// table not exists
		exists, err = api.ExistsTable(ctx, conn, table_name+"_not_exists")
		require.NoError(t, err, "exists table %q_not_exists fail", table_name)
		require.False(t, exists, "table %q_not_exists exists", table_name)

		// table exists and truncate
		exists, truncated, err := api.ExistsTableTruncate(ctx, conn, table_name, true)
		require.NoError(t, err, "exists table %q fail", table_name)
		require.True(t, exists, "table %q not exists", table_name)
		require.True(t, truncated, "table %q not truncated", table_name)
	}
}
