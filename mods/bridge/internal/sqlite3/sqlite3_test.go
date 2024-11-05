package sqlite3_test

import (
	"context"
	"testing"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/bridge/internal"
	"github.com/machbase/neo-server/mods/bridge/internal/sqlite3"
	"github.com/stretchr/testify/require"
)

func TestSqlite(t *testing.T) {
	ctx := context.TODO()

	br := sqlite3.New("test", ":memory:")

	err := br.BeforeRegister()
	require.NoError(t, err)
	defer br.AfterUnregister()

	sqlConn, err := br.Connect(ctx)
	require.NoError(t, err)

	conn := internal.NewConn(sqlConn)
	defer conn.Close()

	result := conn.Exec(ctx, `CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)`)
	require.NoError(t, result.Err())

	conn.Exec(ctx, `INSERT INTO test VALUES (?, ?)`, 1, "foo")
	conn.Exec(ctx, `INSERT INTO test VALUES (?, ?)`, 2, "bar")

	beginCalled := false
	endCalled := false
	nextCalled := 0
	expectNames := []string{"foo", "bar"}
	q := api.Query{
		Begin: func(q *api.Query) {
			beginCalled = true
		},
		Next: func(q *api.Query, rownum int64) bool {
			nextCalled++
			values, _ := q.Columns().MakeBuffer()
			q.Scan(values...)
			require.Equal(t, 2, len(values))
			require.Equal(t, int64(rownum), *(values[0].(*int64)))
			require.Equal(t, expectNames[rownum-1], *(values[1].(*string)))
			return true
		},
		End: func(q *api.Query) {
			endCalled = true
		},
	}
	err = q.Execute(ctx, conn, `SELECT * FROM test order by id`)
	require.NoError(t, err)
	require.True(t, beginCalled)
	require.True(t, endCalled)
	require.Equal(t, 2, nextCalled)

	row := conn.QueryRow(ctx, `select count(*) from test`)
	require.NoError(t, row.Err())
	var count int64
	err = row.Scan(&count)
	require.NoError(t, err)
	require.Equal(t, int64(2), count)

	rows, err := conn.Query(ctx, `select * from test where id = ?`, 1)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var id int64
		var name string
		err = rows.Scan(&id, &name)
		require.NoError(t, err)
		require.Equal(t, int64(1), id)
		require.Equal(t, "foo", name)
	}
}
