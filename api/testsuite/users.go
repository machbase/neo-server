package testsuite

import (
	"context"
	"testing"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/stretchr/testify/require"
)

func DemoUser(t *testing.T, db api.Database, ctx context.Context) {
	sysConn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer sysConn.Close()

	result := sysConn.Exec(ctx, "CREATE USER demo IDENTIFIED BY demo")
	require.NoError(t, result.Err())
	defer func() {
		result := sysConn.Exec(ctx, "DROP table demo.TAG_DATA")
		require.NoError(t, result.Err())
		result = sysConn.Exec(ctx, "DROP USER demo")
		require.NoError(t, result.Err())
	}()

	// create table
	conn, err := db.Connect(ctx, api.WithPassword("demo", "demo"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	result = conn.Exec(ctx, "CREATE TAG TABLE tag_data (name VARCHAR(100) primary key, time datetime basetime, value double, json_value json)")
	require.NoError(t, result.Err())

	now, _ := time.ParseInLocation("2006-01-02 15:04:05", "2021-01-01 00:00:00", time.UTC)
	// insert
	result = conn.Exec(ctx, `insert into tag_data values('demo-1', ?, 1.23, '{"key1": "value1"}')`, now)
	require.NoError(t, result.Err(), "insert fail")
	conn.Exec(ctx, "exec table_flush(tag_data)")

	row := sysConn.QueryRow(ctx, "select count(*) from demo.tag_data where name = ?", "demo-1")
	require.NoError(t, row.Err())
	var count int
	row.Scan(&count)
	require.Equal(t, 1, count)
}
