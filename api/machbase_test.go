package api_test

import (
	"context"
	"database/sql/driver"
	_ "embed"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/api/machsvr"
	"github.com/machbase/neo-server/api/types"
	"github.com/stretchr/testify/require"
)

type ConnectFunc func(t *testing.T, ctx context.Context, opts ...api.ConnectOption) api.Conn

func connect_machsvr(t *testing.T, ctx context.Context, opts ...api.ConnectOption) api.Conn {
	t.Helper()
	var db api.Database
	if machsvr_db, err := machsvr.NewDatabase(); err != nil {
		t.Log("Error", err.Error())
		t.Fail()
	} else {
		db = api.NewDatabase(machsvr_db)
	}

	if len(opts) == 0 {
		conn, err := db.Connect(ctx, api.WithTrustUser("sys"))
		require.NoError(t, err, "connect fail")
		return conn
	} else {
		conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
		require.NoError(t, err, "connect fail")
		return conn
	}
}

func TestInsert(t *testing.T) {
	ctx := context.TODO()
	conn := connect_machsvr(t, ctx)

	now, _ := time.ParseInLocation("2006-01-02 15:04:05", "2021-01-01 00:00:00", time.Local)
	// insert
	result := conn.Exec(ctx, `insert into tag_data values('insert-once', '2021-01-01 00:00:00', 1.23, 1, 2, 3, 'str1', '{"key1": "value1"}')`)
	require.NoError(t, result.Err(), "insert fail")
	conn.Close()

	time.Sleep(1 * time.Second)
	conn = connect_machsvr(t, ctx, api.WithPassword("sys", "manager"))
	defer conn.Close()

	// select
	rows, err := conn.Query(ctx, `select * from tag_data where name = ?`, "insert-once")
	require.NoError(t, err, "select fail")
	defer rows.Close()

	numRows := 0
	for rows.Next() {
		numRows++
		var name string
		var time time.Time
		var value float64
		var short_value int16
		var int_value int32
		var long_value int64
		var str_value string
		var json_value string
		err := rows.Scan(&name, &time, &value, &short_value, &int_value, &long_value, &str_value, &json_value)
		require.NoError(t, err, "scan fail")
		require.Equal(t, "insert-once", name)
		require.Equal(t, now.UnixNano(), time.UnixNano())
		require.Equal(t, 1.23, value)
		require.Equal(t, int16(1), short_value)
		require.Equal(t, int32(2), int_value)
		require.Equal(t, int64(3), long_value)
		require.Equal(t, "str1", str_value)
		require.Equal(t, `{"key1": "value1"}`, json_value)
	}
	require.Equal(t, 1, numRows)

	// query - select
	queryCtx := &api.Query{
		Begin: func(q *api.Query) {
			cols := q.Columns()
			require.Equal(t, []string{"NAME", "TIME", "VALUE", "SHORT_VALUE", "INT_VALUE", "LONG_VALUE", "STR_VALUE", "JSON_VALUE"}, cols.Names())
			require.Equal(t, []types.DataType{
				types.DataTypeString,
				types.DataTypeDatetime,
				types.DataTypeFloat64,
				types.DataTypeInt16,
				types.DataTypeInt32,
				types.DataTypeInt64,
				types.DataTypeString,
				types.DataTypeString,
			}, cols.DataTypes())
		},
		Next: func(q *api.Query, rownum int64, values []interface{}) bool {
			require.Equal(t, "insert-once", unbox(values[0]))
			require.Equal(t, now, unbox(values[1]))
			require.Equal(t, 1.23, unbox(values[2]))
			require.Equal(t, int16(1), unbox(values[3]))
			require.Equal(t, int32(2), unbox(values[4]))
			require.Equal(t, int64(3), unbox(values[5]))
			require.Equal(t, "str1", unbox(values[6]))
			require.Equal(t, `{"key1": "value1"}`, unbox(values[7]))
			return true
		},
		End: func(q *api.Query, userMessage string, rowsFetched int64) {
			require.True(t, q.IsFetch())
			require.Equal(t, "a row fetched.", userMessage)
		},
	}
	err = queryCtx.Execute(ctx, conn, `select * from tag_data where name = ?`, "insert-once")
	require.NoError(t, err, "query fail")

	// query - insert
	queryCtx = &api.Query{
		End: func(q *api.Query, userMessage string, rowsFetched int64) {
			require.False(t, q.IsFetch())
			require.Equal(t, "a row inserted.", userMessage)
		},
	}
	err = queryCtx.Execute(ctx, conn, `insert into tag_data values('insert-twice', '2021-01-01 00:00:00', 1.23, 1, 2, 3, 'str1', '{"key1": "value1"}')`)
	require.NoError(t, err, "query-insert fail")

	// tags
	tags := map[string]string{}
	api.Tags(ctx, conn, "TAG_DATA", func(name string, err error) bool {
		require.NoError(t, err, "tags fail")
		tags[name] = name
		return true
	})
	require.Equal(t, "insert-once", tags["insert-once"])

	// tag stat
	tagStat, err := api.TagStat(ctx, conn, "TAG_DATA", "insert-once")
	require.NoError(t, err, "tag stat fail")
	require.Equal(t, "insert-once", tagStat.Name)
	require.Equal(t, int64(1), tagStat.RowCount)
}

func TestTables(t *testing.T) {
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

	result := map[string]*api.TableInfo{}
	api.Tables(ctx, conn, func(ti *api.TableInfo, err error) bool {
		require.NoError(t, err, "tables fail")
		result[fmt.Sprintf("%s.%s.%s", ti.Database, ti.User, ti.Name)] = ti
		return true
	})
	ti := result["MACHBASEDB.SYS.TAG_DATA"]
	require.NotNil(t, ti, "table not found")
	require.Equal(t, types.TableTypeTag, ti.Type)
	require.Equal(t, types.TableFlagNone, ti.Flag)
	require.Equal(t, "Tag Table", ti.Kind())

	ti = result["MACHBASEDB.SYS._TAG_DATA_META"]
	require.NotNil(t, ti, "table not found")
	require.Equal(t, types.TableTypeLookup, ti.Type)
	require.Equal(t, types.TableFlagMeta, ti.Flag)
	require.Equal(t, "Lookup Table (meta)", ti.Kind())

	ti = result["MACHBASEDB.SYS._TAG_DATA_DATA_0"]
	require.NotNil(t, ti, "table not found")
	require.Equal(t, types.TableTypeKeyValue, ti.Type)
	require.Equal(t, types.TableFlagData, ti.Flag)
	require.Equal(t, "KeyValue Table (data)", ti.Kind())

	ti = result["MACHBASEDB.SYS.TAG_SIMPLE"]
	require.NotNil(t, ti, "table not found")
	require.Equal(t, types.TableTypeTag, ti.Type)
	require.Equal(t, types.TableFlagNone, ti.Flag)
	require.Equal(t, "Tag Table", ti.Kind())

	ti = result["MACHBASEDB.SYS._TAG_SIMPLE_META"]
	require.NotNil(t, ti, "table not found")
	require.Equal(t, types.TableTypeLookup, ti.Type)
	require.Equal(t, types.TableFlagMeta, ti.Flag)
	require.Equal(t, "Lookup Table (meta)", ti.Kind())
}

func TestExistsTable(t *testing.T) {
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

func TestIndexes(t *testing.T) {
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

	ret, err := api.Indexes(ctx, conn)
	require.NoError(t, err, "indexes fail")
	require.NotEmpty(t, ret, "indexes empty")
	for _, r := range ret {
		switch r.Name {
		case "_TAG_DATA_META_NAME":
			require.Equal(t, "MACHBASEDB", r.Database)
			require.Equal(t, "_TAG_DATA_META", r.Table)
			require.Equal(t, "NAME", r.Column)
			require.Equal(t, "REDBLACK", r.Type)
		case "__PK_IDX__TAG_DATA_META_1":
			require.Equal(t, "MACHBASEDB", r.Database)
			require.Equal(t, "_TAG_DATA_META", r.Table)
			require.Equal(t, "_ID", r.Column)
			require.Equal(t, "REDBLACK", r.Type)
		case "_TAG_SIMPLE_META_NAME":
			require.Equal(t, "MACHBASEDB", r.Database)
			require.Equal(t, "_TAG_SIMPLE_META", r.Table)
			require.Equal(t, "NAME", r.Column)
			require.Equal(t, "REDBLACK", r.Type)
		case "__PK_IDX__TAG_SIMPLE_META_1":
			require.Equal(t, "MACHBASEDB", r.Database)
			require.Equal(t, "_TAG_SIMPLE_META", r.Table)
			require.Equal(t, "_ID", r.Column)
			require.Equal(t, "REDBLACK", r.Type)
		default:
			t.Logf("Unknown index: %+v", r)
			t.Fail()
		}
	}
}

func unbox(val any) any {
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
