package testsuite

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-client/machgo"
	"github.com/stretchr/testify/require"
)

func InsertMeta(t *testing.T, db api.Database, ctx context.Context) {
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err)
	defer conn.Close()

	// create tag table
	result := conn.Exec(ctx, api.SqlTidy(`
		CREATE TAG TABLE MYTAG (
			name varchar(32) primary key,
			time datetime basetime,
			value double summarized
		) METADATA(
			factory varchar(32),
			equipment varchar(64) 
		)`))
	require.NoError(t, result.Err())

	result = conn.Exec(ctx, "INSERT INTO MYTAG METADATA(name, factory, equipment) values('FA1_CNC', 'FA1', 'CNC')")
	require.NoError(t, result.Err())
	result = conn.Exec(ctx, "INSERT INTO MYTAG METADATA(name, factory, equipment) values('FA4_MILLING', 'FA4', 'MILLING')")
	require.NoError(t, result.Err())

	// flush
	result = conn.Exec(ctx, "EXEC table_flush(MYTAG)")
	require.NoError(t, result.Err(), "table_flush fail")

	// select tag metadata
	rows, err := conn.Query(ctx, "SELECT _id, name, factory, equipment FROM _MYTAG_META")
	require.NoError(t, err)
	var id, name, factory, equipment string
	for rows.Next() {
		require.NoError(t, rows.Scan(&id, &name, &factory, &equipment))
		switch id {
		case "1":
			require.Equal(t, "FA1_CNC", name)
			require.Equal(t, "FA1", factory)
			require.Equal(t, "CNC", equipment)
		case "2":
			require.Equal(t, "FA4_MILLING", name)
			require.Equal(t, "FA4", factory)
			require.Equal(t, "MILLING", equipment)
		default:
			t.Fatalf("Unknown tag metadata: %s", id)
		}
	}
	rows.Close()

	// drop tag table
	result = conn.Exec(ctx, "DROP TABLE MYTAG")
	require.NoError(t, result.Err())
}

func InsertNewTags(t *testing.T, db api.Database, ctx context.Context) {
	expectCount := 1000
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
		require.NoError(t, err, "connect fail")
		defer func() {
			conn.Close()
			wg.Done()
		}()
		ts := time.Now()
		for i := 0; i < expectCount; i++ {
			result := conn.Exec(ctx, `INSERT INTO TAG_SIMPLE (name, time, value) VALUES(?, ?, ?)`,
				fmt.Sprintf("tag-%d", i),
				ts.Add(1),
				1.23*float64(i),
			)
			require.NoError(t, result.Err(), "insert fail, count=%d", i)
		}
	}()

	wg.Add(1)
	go func() {
		conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
		require.NoError(t, err, "connect fail")
		defer func() {
			conn.Close()
			wg.Done()
		}()
		for i := 0; i < expectCount; i++ {
			rows, err := conn.Query(ctx, `SELECT _ID, NAME FROM _TAG_SIMPLE_META`)
			require.NoError(t, err, "list tags fail")
			count := 0
			for rows.Next() {
				count++
			}
			rows.Close()
		}
	}()

	wg.Wait()

	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	rows, err := conn.Query(ctx, `SELECT _ID, NAME FROM _TAG_SIMPLE_META`)
	require.NoError(t, err, "list tags fail")
	count := 0
	for rows.Next() {
		count++
	}
	rows.Close()
	require.Equal(t, expectCount, count)
}

func BitTable(t *testing.T, db api.Database, ctx context.Context) {
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	result := conn.Exec(ctx,
		"CREATE TABLE bit_table (i1 INTEGER, i2 UINTEGER, i3 FLOAT, i4 DOUBLE, i5 SHORT, i6 VARCHAR(10))",
	)
	require.NoError(t, result.Err(), "create bit table fail")

	result = conn.Exec(ctx, "INSERT INTO bit_table VALUES (-1, 1, 1, 1, 2, 'aaa')")
	require.NoError(t, result.Err(), "insert bit table fail")
	require.NoError(t, err)

	rows, err := conn.Query(ctx, "SELECT * FROM bit_table WHERE BITAND(i2, 1) = 1")
	require.NoError(t, err, "select bit table BITAND(i2, 1) should not fail")
	for rows.Next() {
		var i1 int
		var i2 uint
		var i3 float32
		var i4 float64
		var i5 int16
		var i6 string
		err := rows.Scan(&i1, &i2, &i3, &i4, &i5, &i6)
		require.NoError(t, err, "scan bit table fail")
		require.Equal(t, -1, i1)
		require.Equal(t, uint(1), i2)
		require.Equal(t, float32(1), i3)
		require.Equal(t, float64(1), i4)
		require.Equal(t, int16(2), i5)
		require.Equal(t, "aaa", i6)
	}
	rows.Close()

	rows, err = conn.Query(ctx, "SELECT * FROM bit_table WHERE BITAND(i4, 1) = 1")
	if _, ok := conn.(*machgo.Conn); ok {
		require.Error(t, err, "select bit table BITAND(i1, i3) should fail within Query()")
		require.Equal(t, "MACHCLI-ERR-2037, Function [BITAND] argument data type is mismatched.", err.Error())
	} else {
		require.NoError(t, err, "select bit table BITAND(i1, i3) should not fail within Query()")
		require.False(t, rows.Next(), "select bit table BITAND(i4, 1) should fail")
		require.Error(t, rows.Err(), "select bit table BITAND(i4, 1) should fail")
		// https://github.com/machbase/neo/issues/956
		require.Equal(t, "MACH-ERR 2037 Function [BITAND] argument data type is mismatched.", rows.Err().Error())
	}

	if rows != nil {
		rows.Close()
	}

	rows, err = conn.Query(ctx, "SELECT BITAND(i1, i3) FROM bit_table")
	if _, ok := conn.(*machgo.Conn); ok {
		require.Error(t, err, "select bit table BITAND(i1, i3) should fail within Query()")
		require.Equal(t, "MACHCLI-ERR-2037, Function [BITAND] argument data type is mismatched.", err.Error())
	} else {
		require.NoError(t, err, "select bit table BITAND(i1, i3) should not fail within Query()")
		require.False(t, rows.Next(), "select bit table BITAND(i1, i3) should fail")
		require.Error(t, rows.Err(), "select bit table BITAND(i4, 1) should fail")
		// https://github.com/machbase/neo/issues/956
		require.Equal(t, "MACH-ERR 2037 Function [BITAND] argument data type is mismatched.", rows.Err().Error())
	}
	if rows != nil {
		rows.Close()
	}

	result = conn.Exec(ctx, "DROP TABLE bit_table")
	require.NoError(t, result.Err(), "drop bit table fail")
}
