package machsvr

import (
	"context"
	"reflect"
	"testing"

	"github.com/machbase/neo-client/v2/api"
	"github.com/stretchr/testify/require"
)

func TestVectorNativeRoundTrip(t *testing.T) {
	ctx := context.Background()
	conn, err := testServer.MachSvr().Connect(ctx, WithPassword("sys", "manager"))
	require.NoError(t, err)
	defer conn.Close()

	const table = "NEO_VECTOR_4211"
	require.NoError(t, conn.Exec(ctx, "CREATE TRANSACTION TABLE "+table+
		"(ID INTEGER PRIMARY KEY,V VECTOR(3))").Err())
	defer conn.Exec(ctx, "DROP TABLE "+table)

	require.NoError(t, conn.Exec(ctx, "INSERT INTO "+table+" VALUES(?,?)", int32(1), api.Vector{1, 0, 0}).Err())
	var got api.Vector
	row := conn.QueryRow(ctx, "SELECT V FROM "+table+" WHERE ID=1")
	require.NoError(t, row.Scan(&got))
	require.True(t, reflect.DeepEqual(got, api.Vector{1, 0, 0}), "got %v", got)
	require.NoError(t, conn.Exec(ctx, "INSERT INTO "+table+" VALUES(?,?)", int32(3), api.Vector(nil)).Err())
	got = api.Vector{9}
	require.NoError(t, conn.QueryRow(ctx, "SELECT V FROM "+table+" WHERE ID=3").Scan(&got))
	require.Nil(t, got)
	require.NoError(t, conn.QueryRow(ctx, "SELECT TO_VECTOR('[0,1,0]',3)").Scan(&got))
	require.Equal(t, api.Vector{0, 1, 0}, got)

	appender, err := conn.Appender(ctx, table)
	require.NoError(t, err)
	require.NoError(t, appender.Append(int32(2), api.Vector{0, 1, 0}))
	_, _, err = appender.Close()
	require.NoError(t, err)
	require.NoError(t, conn.QueryRow(ctx, "SELECT V FROM "+table+" WHERE ID=2").Scan(&got))
	require.True(t, reflect.DeepEqual(got, api.Vector{0, 1, 0}), "got %v", got)

	const search = "SELECT R.ID,R.__SEARCH_VECTOR_DISTANCE FROM " +
		"VECTOR_SEARCH(TABLE NEO_VECTOR_4211,VECTOR V,QUERY_VECTOR ?," +
		"METRIC COSINE,MODE EXACT,TOP_K 2) R ORDER BY R.__SEARCH_VECTOR_DISTANCE,R.ID"
	rows, err := conn.Query(ctx, search, api.Vector{1, 0, 0})
	require.NoError(t, err)
	defer rows.Close()
	require.True(t, rows.Next())
	var id int32
	var distance float64
	require.NoError(t, rows.Scan(&id, &distance))
	require.Equal(t, int32(1), id)
	require.Zero(t, distance)
}

func TestVectorNativeLargeValue(t *testing.T) {
	ctx := context.Background()
	conn, err := testServer.MachSvr().Connect(ctx, WithPassword("sys", "manager"))
	require.NoError(t, err)
	defer conn.Close()
	const table = "NEO_VECTOR_LARGE_4211"
	require.NoError(t, conn.Exec(ctx, "CREATE TRANSACTION TABLE "+table+"(ID INTEGER PRIMARY KEY,V VECTOR(17000))").Err())
	defer conn.Exec(ctx, "DROP TABLE "+table)
	input := make(api.Vector, 17000)
	input[0], input[len(input)-1] = 1.5, -2.25
	require.NoError(t, conn.Exec(ctx, "INSERT INTO "+table+" VALUES(?,?)", int32(1), input).Err())
	var got api.Vector
	require.NoError(t, conn.QueryRow(ctx, "SELECT V FROM "+table+" WHERE ID=1").Scan(&got))
	require.Equal(t, len(input), len(got))
	require.Equal(t, input[0], got[0])
	require.Equal(t, input[len(input)-1], got[len(got)-1])
}
