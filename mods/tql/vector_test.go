package tql_test

import (
	"context"
	"testing"

	"github.com/machbase/neo-client/v2/api"
	"github.com/machbase/neo-server/v8/spi"
	"github.com/stretchr/testify/require"
)

func TestTqlVectorAppend(t *testing.T) {
	conn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err)
	defer conn.Close()
	const table = "NEO_TQL_VECTOR_4211"
	_, err = conn.ExecContext(t.Context(), "CREATE TRANSACTION TABLE "+table+"(ID INTEGER PRIMARY KEY,V VECTOR(3))")
	require.NoError(t, err)
	defer conn.ExecContext(context.Background(), "DROP TABLE "+table)
	TqlTestCase{
		Name: "vector-append",
		Script: `SCRIPT({ $.yield(1, [1,0,0]); })
APPEND(table('NEO_TQL_VECTOR_4211'))`,
		ExpectFunc: func(t *testing.T, result string) {
			require.Contains(t, result, "append 1 row")
		},
	}.run(t)
	spi.FlushAppendWorkers("", "", table)
	var vector api.Vector
	require.NoError(t, conn.QueryRowContext(t.Context(), "SELECT V FROM "+table+" WHERE ID=1").Scan(&vector))
	require.Equal(t, api.Vector{1, 0, 0}, vector)
}
