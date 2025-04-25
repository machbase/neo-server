package testsuite

import (
	"context"
	"database/sql/driver"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machcli"
	"github.com/stretchr/testify/require"
)

func ShowTables(t *testing.T, db api.Database, ctx context.Context) {
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	result := map[string]*api.TableInfo{}
	api.ListTablesWalk(ctx, conn, true, func(ti *api.TableInfo) bool {
		require.NoError(t, err, "tables fail")
		result[fmt.Sprintf("%s.%s.%s", ti.Database, ti.User, ti.Name)] = ti
		return true
	})
	ti := result["MACHBASEDB.SYS.TAG_DATA"]
	require.NotNil(t, ti, "table not found")
	require.Equal(t, api.TableTypeTag, ti.Type)
	require.Equal(t, api.TableFlagNone, ti.Flag)
	require.Equal(t, "Tag Table", ti.Kind())

	ti = result["MACHBASEDB.SYS._TAG_DATA_META"]
	require.NotNil(t, ti, "table not found")
	require.Equal(t, api.TableTypeLookup, ti.Type)
	require.Equal(t, api.TableFlagMeta, ti.Flag)
	require.Equal(t, "Lookup Table (meta)", ti.Kind())

	ti = result["MACHBASEDB.SYS._TAG_DATA_DATA_0"]
	require.NotNil(t, ti, "table not found")
	require.Equal(t, api.TableTypeKeyValue, ti.Type)
	require.Equal(t, api.TableFlagData, ti.Flag)
	require.Equal(t, "KeyValue Table (data)", ti.Kind())

	ti = result["MACHBASEDB.SYS.TAG_SIMPLE"]
	require.NotNil(t, ti, "table not found")
	require.Equal(t, api.TableTypeTag, ti.Type)
	require.Equal(t, api.TableFlagNone, ti.Flag)
	require.Equal(t, "Tag Table", ti.Kind())

	ti = result["MACHBASEDB.SYS._TAG_SIMPLE_META"]
	require.NotNil(t, ti, "table not found")
	require.Equal(t, api.TableTypeLookup, ti.Type)
	require.Equal(t, api.TableFlagMeta, ti.Flag)
	require.Equal(t, "Lookup Table (meta)", ti.Kind())

	tables, err := api.ListTables(ctx, conn, true)
	require.NoError(t, err, "show tables fail")
	require.Equal(t, len(result), len(tables))

	resultList, err := api.ListTables(ctx, conn, false)
	require.NoError(t, err, "show tables fail")
	require.NotEmpty(t, resultList, "tables empty")

	result2 := map[string]*api.TableInfo{}
	for _, v := range tables {
		result2[fmt.Sprintf("%s.%s.%s", v.Database, v.User, v.Name)] = v
	}
	require.Equal(t, result, result2)
}

func ExistsTable(t *testing.T, db api.Database, ctx context.Context) {
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
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

func Indexes(t *testing.T, db api.Database, ctx context.Context) {
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	ret, err := api.ListIndexes(ctx, conn)
	require.NoError(t, err, "indexes fail")
	require.NotEmpty(t, ret, "indexes empty")
	for _, r := range ret {
		switch r.IndexName {
		case "_TAG_DATA_META_NAME":
			require.Equal(t, "MACHBASEDB", r.Database)
			require.Equal(t, "_TAG_DATA_META", r.TableName)
			require.Equal(t, "NAME", r.ColumnName)
			require.Equal(t, "REDBLACK", r.IndexType)
		case "__PK_IDX__TAG_DATA_META_1":
			require.Equal(t, "MACHBASEDB", r.Database)
			require.Equal(t, "_TAG_DATA_META", r.TableName)
			require.Equal(t, "_ID", r.ColumnName)
			require.Equal(t, "REDBLACK", r.IndexType)
		case "_TAG_SIMPLE_META_NAME":
			require.Equal(t, "MACHBASEDB", r.Database)
			require.Equal(t, "_TAG_SIMPLE_META", r.TableName)
			require.Equal(t, "NAME", r.ColumnName)
			require.Equal(t, "REDBLACK", r.IndexType)
		case "__PK_IDX__TAG_SIMPLE_META_1":
			require.Equal(t, "MACHBASEDB", r.Database)
			require.Equal(t, "_TAG_SIMPLE_META", r.TableName)
			require.Equal(t, "_ID", r.ColumnName)
			require.Equal(t, "REDBLACK", r.IndexType)
		default:
			t.Logf("Unknown index: %+v", r)
			t.Fail()
		}
	}
}

func InsertAndQuery(t *testing.T, db api.Database, ctx context.Context) {
	now, _ := time.ParseInLocation("2006-01-02 15:04:05", "2021-01-01 00:00:00", time.UTC)

	// Because INSERT statement uses '2021-01-01 00:00:00' as time value which was parsed in Local timezone,
	// the time value should be converted to UTC timezone to compare
	// TODO: improve this behavior
	nowStrInLocal := now.In(time.Local).Format("2006-01-02 15:04:05")

	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	// insert
	result := conn.Exec(ctx, `insert into tag_data (name, time, value, short_value, int_value, long_value, str_value, json_value) `+
		`values('insert-once', '`+nowStrInLocal+`', 1.23, 1, 2, 3, 'str1', '{"key1": "value1"}')`)
	require.NoError(t, result.Err(), "insert fail")

	sysConn, _ := db.Connect(ctx, api.WithPassword("sys", "manager"))
	result = sysConn.Exec(ctx, `EXEC table_flush(tag_data)`)
	require.NoError(t, result.Err(), "table_flush fail")
	sysConn.Close()

	// select
	rows, err := conn.Query(ctx, `select name, time, value, short_value, int_value, long_value, str_value, json_value from tag_data where name = ?`,
		"insert-once")
	require.NoError(t, err, "select fail")

	numRows := 0
	for rows.Next() {
		numRows++
		var name string
		var timeVal time.Time
		var value float64
		var short_value int16
		var int_value int32
		var long_value int64
		var str_value string
		var json_value string
		err := rows.Scan(&name, &timeVal, &value, &short_value, &int_value, &long_value, &str_value, &json_value)
		require.NoError(t, err, "scan fail")
		require.Equal(t, "insert-once", name)
		require.Equal(t, now.Unix(), timeVal.Unix())
		require.Equal(t, 1.23, value)
		require.Equal(t, int16(1), short_value)
		require.Equal(t, int32(2), int_value)
		require.Equal(t, int64(3), long_value)
		require.Equal(t, "str1", str_value)
		require.Equal(t, `{"key1": "value1"}`, json_value)
	}
	require.Equal(t, 1, numRows)
	err = rows.Close()
	require.NoError(t, err, "close fail")

	var unbox = func(val any) any {
		switch v := val.(type) {
		case *int:
			return *v
		case *uint:
			return *v
		case *int16:
			return *v
		case *uint16:
			return *v
		case *int32:
			return *v
		case *uint32:
			return *v
		case *int64:
			return *v
		case *uint64:
			return *v
		case *float64:
			return *v
		case *float32:
			return *v
		case *string:
			return *v
		case *time.Time:
			return *v
		case *[]byte:
			return *v
		case *net.IP:
			return *v
		case *driver.Value:
			return *v
		default:
			return val
		}
	}

	var beginCalled, endCalled bool
	var nextCalled int
	// query - select
	queryCtx := &api.Query{
		Begin: func(q *api.Query) {
			beginCalled = true
			cols := q.Columns()
			require.Equal(t, []string{"NAME", "TIME", "VALUE",
				"SHORT_VALUE", "USHORT_VALUE", "INT_VALUE", "UINT_VALUE", "LONG_VALUE", "ULONG_VALUE",
				"STR_VALUE", "JSON_VALUE", "IPV4_VALUE", "IPV6_VALUE"}, cols.Names())
			require.Equal(t, []api.DataType{
				api.DataTypeString,
				api.DataTypeDatetime,
				api.DataTypeFloat64,
				api.DataTypeInt16,
				api.DataTypeInt16,
				api.DataTypeInt32,
				api.DataTypeInt32,
				api.DataTypeInt64,
				api.DataTypeInt64,
				api.DataTypeString,
				api.DataTypeString,
				api.DataTypeIPv4,
				api.DataTypeIPv6,
			}, cols.DataTypes())
		},
		Next: func(q *api.Query, rownum int64) bool {
			nextCalled++
			values, err := q.Columns().MakeBuffer()
			require.NoError(t, err)
			err = q.Scan(values...)
			require.NoError(t, err)
			require.Equal(t, "insert-once", unbox(values[0]))
			require.Equal(t, now, unbox(values[1]))
			require.Equal(t, 1.23, unbox(values[2]))
			require.Equal(t, int16(1), unbox(values[3]))
			require.Equal(t, nil, unbox(values[4]))
			require.Equal(t, int32(2), unbox(values[5]))
			require.Equal(t, nil, unbox(values[6]))
			require.Equal(t, int64(3), unbox(values[7]))
			require.Equal(t, nil, unbox(values[8]))
			require.Equal(t, "str1", unbox(values[9]))
			require.Equal(t, `{"key1": "value1"}`, unbox(values[10]))
			return true
		},
		End: func(q *api.Query) {
			endCalled = true
			require.NoError(t, q.Err())
			require.True(t, q.IsFetch())
			require.Equal(t, "a row fetched.", q.UserMessage())
			require.Equal(t, int64(1), q.RowNum())
		},
	}
	err = queryCtx.Execute(ctx, conn, `select * from tag_data where name = ?`, "insert-once")
	require.NoError(t, err, "query fail")
	require.True(t, beginCalled)
	require.True(t, endCalled)
	require.Equal(t, 1, nextCalled)

	// query - insert
	endCalled = false
	queryCtx = &api.Query{
		End: func(q *api.Query) {
			endCalled = true
			require.False(t, q.IsFetch())
			require.NoError(t, q.Err())
			require.Equal(t, "a row inserted.", q.UserMessage())
		},
	}
	err = queryCtx.Execute(ctx, conn, `insert into tag_data values('insert-twice', '2021-01-01 00:00:00', ?,`+ // name, time, value
		`1, ?, ?, ?,`+ // short_value, ushort_value, int_value, uint_value
		`?, ?, `+ // long_value, ulong_value
		`?, ?, ?, ? )`, // str_value, json_value, ipv4_value, ipv6_value
		1.23,                 // value
		10,                   // ushort_value
		2,                    // int_value
		20,                   // uint_value
		3,                    // long_value
		40,                   // ulong_value
		"str1",               // str_value
		`{"key1": "value1"}`, // json_value
		nil,                  // ipv4_value
		nil,                  // ipv6_value
	)
	require.NoError(t, err, "query-insert fail")
	require.True(t, endCalled)

	result = conn.Exec(ctx, "EXEC table_flush(tag_data)")
	require.NoError(t, result.Err(), "table_flush fail")

	if _, ok := db.(*machcli.Database); ok {
		// TODO: MACHCLI-ERR-3, Communication link failure
		<-time.After(time.Second * 1)
	}

	// tags
	tags := []*api.TagInfo{}
	api.ListTagsWalk(ctx, conn, "TAG_DATA", func(tag *api.TagInfo) bool {
		// TODO: MACHCLI-ERR-3, Communication link failure
		require.NoError(t, tag.Err, "tags fail")
		require.Greater(t, tag.Id, int64(0))
		require.Contains(t, []string{"insert-once", "insert-twice"}, tag.Name)
		tags = append(tags, tag)
		return true
	})
	tags2, err := api.ListTags(ctx, conn, "TAG_DATA")
	require.NoError(t, err, "tags fail")
	require.EqualValues(t, tags, tags2)

	// tag stat
	tagStat, err := api.TagStat(ctx, conn, "TAG_DATA", "insert-once")
	require.NoError(t, err, "tag stat fail")
	require.Equal(t, "insert-once", tagStat.Name)
	require.Equal(t, int64(1), tagStat.RowCount)
	require.Equal(t, 1.23, tagStat.MinValue)
	require.Equal(t, 1.23, tagStat.MaxValue)

	// tag stat
	tagStat, err = api.TagStat(ctx, conn, "TAG_DATA", "insert-twice")
	require.NoError(t, err, "tag stat fail")
	require.Equal(t, "insert-twice", tagStat.Name)
	require.Equal(t, int64(1), tagStat.RowCount)

	// delete test data
	// TODO: delete test data with BIND variable tag name
	result = conn.Exec(ctx, `delete from tag_data where name = 'insert-once'`)
	require.NoError(t, result.Err(), "delete fail")
	require.Equal(t, int64(1), result.RowsAffected())

	// TODO: delete test data with BIND variable tag name
	result = conn.Exec(ctx, `delete from tag_data where name = 'insert-twice'`)
	require.NoError(t, result.Err(), "delete fail")
	require.Equal(t, int64(1), result.RowsAffected())
}

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
		if id == "1" {
			require.Equal(t, "FA1_CNC", name)
			require.Equal(t, "FA1", factory)
			require.Equal(t, "CNC", equipment)
		} else if id == "2" {
			require.Equal(t, "FA4_MILLING", name)
			require.Equal(t, "FA4", factory)
			require.Equal(t, "MILLING", equipment)
		} else {
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

	if _, ok := db.(*machcli.Database); ok {
		rows, err = conn.Query(ctx, "SELECT * FROM bit_table WHERE BITAND(i4, 1) = 1")
		require.Error(t, err, "select bit table BITAND(i4, 1) should fail")
		if rows != nil {
			rows.Close()
		}
	} else {
		// TODO : BITAND(i4, 1) SHOULD fail
		// BUG: https://github.com/machbase/neo/issues/956
		rows, err = conn.Query(ctx, "SELECT * FROM bit_table WHERE BITAND(i4, 1) = 1")
		// require.Error(t, err, "select bit table BITAND(i4, 1) should fail")
		if rows != nil {
			rows.Close()
		}
	}

	// TODO : BITAND(i1, i3) SHOULD fail
	// BUG: https://github.com/machbase/neo/issues/956
	// rows, err = conn.Query(ctx, "SELECT BITAND(i1, i3) FROM bit_table")
	//require.Error(t, err, "select bit table BITAND(i4, 1) should fail")
	// if rows != nil {
	// 	rows.Close()
	// }

	result = conn.Exec(ctx, "DROP TABLE bit_table")
	require.NoError(t, result.Err(), "drop bit table fail")
}
