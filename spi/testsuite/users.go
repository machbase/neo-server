package testsuite

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-client/machgo"
	"github.com/machbase/neo-server/v8/spi"
	"github.com/stretchr/testify/require"
)

func DemoUser(t *testing.T, db api.Database, ctx context.Context) {
	if _, ok := db.(*machgo.Database); !ok {
		t.Skipf("skip DemoUser test for %T", db)
	}
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

	result = conn.Exec(ctx, "CREATE TAG TABLE tag_data (name VARCHAR(100) primary key, time datetime basetime, value double, json_value json)")
	require.NoError(t, result.Err())

	now, _ := time.ParseInLocation("2006-01-02 15:04:05", "2021-01-01 00:00:00", time.UTC)
	// insert tag_data
	result = conn.Exec(ctx, `insert into tag_data values('demo-1', ?, 1.23, '{"key1": "value1"}')`, now)
	require.NoError(t, result.Err(), "insert fail")

	// insert demo.tag_data
	result = sysConn.Exec(ctx, `insert into demo.tag_data values('demo-1', ?, 1.23, '{"key1": "value1"}')`, now.Add(1))
	require.NoError(t, result.Err(), "insert fail")

	result = sysConn.Exec(ctx, "exec table_flush(demo.tag_data)")
	require.NoError(t, result.Err(), "table_flush fail")

	row := sysConn.QueryRow(ctx, "select count(*) from demo.tag_data where name = ?", "demo-1")
	require.NoError(t, row.Err())
	var count int
	row.Scan(&count)
	require.Equal(t, 2, count)

	result = conn.Exec(ctx, `drop table tag_data`)
	require.NoError(t, result.Err(), "drop table fail")
	conn.Close()

	// connect as proxy user
	proxyConn, err := db.Connect(ctx, api.WithAuthKey("sys", spi.DefaultKey()), api.WithProxyUser("demo"))
	require.NoError(t, err, "connect fail")
	defer proxyConn.Close()

	result = proxyConn.Exec(ctx, "CREATE TAG TABLE tag_data (name VARCHAR(100) primary key, time datetime basetime, value double, json_value json)")
	require.NoError(t, result.Err(), fmt.Sprintf("create table fail: %T", db))

	// insert tag_data
	result = proxyConn.Exec(ctx, `insert into tag_data values('demo-1', ?, 1.23, '{"key1": "value1"}')`, now)
	require.NoError(t, result.Err(), "insert fail")

	// insert demo.tag_data
	result = sysConn.Exec(ctx, `insert into demo.tag_data values('demo-1', ?, 1.23, '{"key1": "value1"}')`, now.Add(1))
	require.NoError(t, result.Err(), "insert fail")

	result = sysConn.Exec(ctx, "exec table_flush(demo.tag_data)")
	require.NoError(t, result.Err(), "table_flush fail")

	row = sysConn.QueryRow(ctx, "select count(*) from demo.tag_data where name = ?", "demo-1")
	require.NoError(t, row.Err())
	row.Scan(&count)
	require.Equal(t, 2, count)
}
