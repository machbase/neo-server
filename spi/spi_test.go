package spi_test

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	_ "embed"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	client "github.com/machbase/neo-client/v2"
	"github.com/machbase/neo-client/v2/api"
	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/spi"
	"github.com/machbase/neo-server/v8/spi/machsvr"
	"github.com/stretchr/testify/require"
)

var testServer *machsvr.TestServer

func TestMain(m *testing.M) {
	testServer = &machsvr.TestServer{}
	testServer.StartServer("./testsuite_tmp")
	code := m.Run()
	testServer.StopServer()
	os.Exit(code)
}

func SqlTidy(sqlTextLines ...string) string {
	sqlText := strings.Join(sqlTextLines, "\n")
	lines := strings.Split(sqlText, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimSpace(ln)
	}
	return strings.Join(lines, " ")
}

func TestPing(t *testing.T) {
	db, err := spi.DefaultPool()
	require.NoError(t, err)
	require.NoError(t, db.PingContext(t.Context()))
}

func TestTableNames(t *testing.T) {
	tests := []struct {
		input  string
		expect [3]string
	}{
		{"a.b.c", [3]string{"A", "B", "C"}},
		{"user.table", [3]string{"MACHBASEDB", "USER", "TABLE"}},
		{"table", [3]string{"MACHBASEDB", "SYS", "TABLE"}},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			a, b, c := spi.TableName(test.input).Split()
			require.Equal(t, test.expect[0], a)
			require.Equal(t, test.expect[1], b)
			require.Equal(t, test.expect[2], c)
		})
	}
}

func TestBridge(t *testing.T) {
	tests := []struct {
		Name        string
		Bridge      string
		SqlExec     string
		SqlQuery    string
		Params      []any
		ExpectExec  func(t *testing.T, result sql.Result, err error)
		ExpectQuery func(t *testing.T, rows *sql.Rows, err error)
	}{
		{
			Name:   "sqlite-create-table",
			Bridge: "sqlite",
			SqlExec: `create table example (` +
				`	id INTEGER NOT NULL PRIMARY KEY,` +
				`	name TEXT,` +
				`	age INTEGER,` +
				`	address TEXT,` +
				`	weight REAL,` +
				`	memo BLOB,` +
				`	UNIQUE(name)` +
				`)`,
			ExpectExec: func(t *testing.T, result sql.Result, err error) {
				require.NoError(t, err)
				require.Equal(t, "Created successfully.", spi.MakeUserMessage(spi.SQLStatementTypeCreate, 0))
			},
		},
		{
			Name:    "sqlite-insert",
			Bridge:  "sqlite",
			SqlExec: `INSERT INTO example (id, name, age, address) VALUES (100, 'alpha', 10, 'street-100')`,
			ExpectExec: func(t *testing.T, result sql.Result, err error) {
				require.NoError(t, err)
				n, err := result.RowsAffected()
				require.NoError(t, err)
				require.Equal(t, "a row inserted.", spi.MakeUserMessage(spi.SQLStatementTypeInsert, n))
			},
		},
		{
			Name:     "sqlite-select-all",
			Bridge:   "sqlite",
			SqlQuery: `SELECT * FROM example`,
			ExpectQuery: func(t *testing.T, rows *sql.Rows, err error) {
				require.NoError(t, err)
				columns, err := rows.Columns()
				require.NoError(t, err)
				require.NotNil(t, columns)
				names, err := rows.Columns()
				require.NoError(t, err)
				require.Equal(t, []string{"id", "name", "age", "address", "weight", "memo"}, names)
				types, err := rows.ColumnTypes()
				typeNames, err := spi.ColumnTypes(rows)
				require.NoError(t, err)
				require.Equal(t, []string{"INTEGER", "TEXT", "INTEGER", "TEXT", "REAL", "BLOB"}, typeNames)
				require.True(t, rows.Next())
				buff := spi.MakeBuffer(types)
				err = rows.Scan(buff...)
				require.NoError(t, err)
				require.Equal(t, &sql.NullInt64{Int64: 100, Valid: true}, buff[0])
				require.Equal(t, &sql.NullString{String: "alpha", Valid: true}, buff[1])
				require.Equal(t, &sql.NullInt64{Int64: 10, Valid: true}, buff[2])
				require.Equal(t, &sql.NullString{String: "street-100", Valid: true}, buff[3])
				require.Equal(t, &sql.NullFloat64{Float64: 0, Valid: false}, buff[4])
				require.IsType(t, &sql.RawBytes{}, buff[5])
				require.Nil(t, *(buff[5].(*sql.RawBytes)))
			},
		},
		{
			Name:     "sqlite-select-count",
			Bridge:   "sqlite",
			SqlQuery: `SELECT count(*) FROM example`,
			ExpectQuery: func(t *testing.T, rows *sql.Rows, err error) {
				require.NoError(t, err)
				columns, err := rows.Columns()
				require.NoError(t, err)
				require.NotNil(t, columns)
				require.Equal(t, []string{"count(*)"}, columns)
				typeNames, err := spi.ColumnTypes(rows)
				require.NoError(t, err)
				require.Equal(t, []string{"string"}, typeNames)
				require.True(t, rows.Next())
				var cnt int64
				err = rows.Scan(&cnt)
				require.NoError(t, err)
				require.Equal(t, int64(1), cnt)
			},
		},
	}

	if err := bridge.Register(&model.BridgeDefinition{
		Type: model.BRIDGE_SQLITE,
		Name: "sqlite",
		Path: "file::memory:?cache=shared",
	}); err == bridge.ErrBridgeDisabled {
		t.Fatal(err)
	} else {
		defer bridge.Unregister("sqlite")
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := t.Context()

			db, err := bridge.GetSqlBridge(tc.Bridge)
			require.NoError(t, err)

			conn, err := db.Connect(ctx)
			require.NoError(t, err)
			defer conn.Close()

			if tc.SqlExec != "" {
				result, err := conn.ExecContext(ctx, tc.SqlExec)
				require.NoError(t, err)
				tc.ExpectExec(t, result, err)
			} else if tc.SqlQuery != "" {
				rows, err := conn.QueryContext(ctx, tc.SqlQuery)
				defer func() {
					if rows != nil {
						rows.Close()
					}
				}()
				tc.ExpectQuery(t, rows, err)
			}
		})
	}
}

func TestScan(t *testing.T) {
	t.Parallel()

	now := time.Unix(0, 1729578712564320000).In(time.UTC)

	tests := []struct {
		name   string
		src    any
		dst    any
		expect any
	}{
		///////////////////////////////////
		// src: int
		{name: "int to int   ", src: int(321), dst: new(int), expect: int(321)},
		{name: "int to uint  ", src: int(123), dst: new(uint), expect: uint(123)},
		{name: "int to int16 ", src: int(123), dst: new(int16), expect: uint(123)},
		{name: "int to uint16", src: int(123), dst: new(uint16), expect: uint(123)},
		{name: "int to int32 ", src: int(123), dst: new(int32), expect: int32(123)},
		{name: "int to uint32", src: int(123), dst: new(uint32), expect: uint32(123)},
		{name: "int to int64 ", src: int(123), dst: new(int64), expect: int64(123)},
		{name: "int to uint64", src: int(123), dst: new(uint64), expect: uint64(123)},
		{name: "int to string", src: int(123), dst: new(string), expect: "123"},
		///////////////////////////////////
		// src: int16
		{name: "int16 to int   ", src: int16(321), dst: new(int), expect: int(321)},
		{name: "int16 to uint  ", src: int16(123), dst: new(uint), expect: uint(123)},
		{name: "int16 to int16 ", src: int16(123), dst: new(int16), expect: uint(123)},
		{name: "int16 to uint16", src: int16(123), dst: new(uint16), expect: uint(123)},
		{name: "int16 to int32 ", src: int16(123), dst: new(int32), expect: int32(123)},
		{name: "int16 to uint32", src: int16(123), dst: new(uint32), expect: uint32(123)},
		{name: "int16 to int64 ", src: int16(123), dst: new(int64), expect: int64(123)},
		{name: "int16 to uint64", src: int16(123), dst: new(uint64), expect: uint64(123)},
		{name: "int16 to string", src: int16(123), dst: new(string), expect: "123"},
		///////////////////////////////////
		// src: int32
		{name: "int32 to int   ", src: int32(321), dst: new(int), expect: int(321)},
		{name: "int32 to uint  ", src: int32(123), dst: new(uint), expect: uint(123)},
		{name: "int32 to int16 ", src: int32(123), dst: new(int16), expect: uint(123)},
		{name: "int32 to uint16", src: int32(123), dst: new(uint16), expect: uint(123)},
		{name: "int32 to int32 ", src: int32(123), dst: new(int32), expect: int32(123)},
		{name: "int32 to uint32", src: int32(123), dst: new(uint32), expect: uint32(123)},
		{name: "int32 to int64 ", src: int32(123), dst: new(int64), expect: int64(123)},
		{name: "int32 to uint64", src: int32(123), dst: new(uint64), expect: uint64(123)},
		{name: "int32 to string", src: int32(123), dst: new(string), expect: "123"},
		///////////////////////////////////
		// src: int64
		{name: "int64 to int   ", src: int64(987654321), dst: new(int), expect: int(987654321)},
		{name: "int64 to uint  ", src: int64(987654321), dst: new(uint), expect: uint(987654321)},
		{name: "int64 to int16 ", src: int64(987654321), dst: new(int16), expect: int16(26801)},
		{name: "int64 to uint16", src: int64(987654321), dst: new(uint16), expect: uint16(26801)},
		{name: "int64 to int32 ", src: int64(987654321), dst: new(int32), expect: int32(987654321)},
		{name: "int64 to uint32", src: int64(987654321), dst: new(uint32), expect: uint32(987654321)},
		{name: "int64 to int64 ", src: int64(987654321), dst: new(int64), expect: int64(987654321)},
		{name: "int64 to uint64", src: int64(987654321), dst: new(uint64), expect: uint64(987654321)},
		{name: "int64 to string", src: int64(987654321), dst: new(string), expect: "987654321"},
		///////////////////////////////////
		// src: int64
		{name: "time to int64   ", src: now, dst: new(int64), expect: int64(1729578712564320000)},
		{name: "time to time    ", src: now, dst: new(time.Time), expect: now},
		{name: "time to string  ", src: now, dst: new(string), expect: "2024-10-22T06:31:52Z"},
		///////////////////////////////////
		// src: float32
		{name: "float32 to float32", src: float32(3.141592), dst: new(float32), expect: float32(3.141592)},
		{name: "float32 to float64", src: float32(3.141592), dst: new(float64), expect: float64(float32(3.141592))},
		{name: "float32 to string ", src: float32(3.141592), dst: new(string), expect: "3.141592"},
		///////////////////////////////////
		// src: float64
		{name: "float64 to float32", src: float64(3.141592), dst: new(float32), expect: float32(3.141592)},
		{name: "float64 to float64", src: float64(3.141592), dst: new(float64), expect: float64(3.141592)},
		{name: "float64 to string ", src: float64(3.141592), dst: new(string), expect: "3.141592"},
		///////////////////////////////////
		// src: string
		{name: "string to string", src: "1.2.3.4.5", dst: new(string), expect: "1.2.3.4.5"},
		{name: "string to []byte", src: "1.2.3.4.5", dst: new([]byte), expect: []byte("1.2.3.4.5")},
		{name: "string to net.IP", src: "192.168.1.10", dst: new(net.IP), expect: net.ParseIP("192.168.1.10")},
		///////////////////////////////////
		// src: []byte
		{name: "[]byte to []byte", src: []byte("1.2.3.4.5"), dst: new([]byte), expect: []byte("1.2.3.4.5")},
		{name: "[]byte to string", src: []byte("1.2.3.4.5"), dst: new(string), expect: "1.2.3.4.5"},
		///////////////////////////////////
		// src: net.IP
		{name: "net.IP to []byte", src: net.ParseIP("192.168.1.10"), dst: new(net.IP), expect: net.ParseIP("192.168.1.10")},
		{name: "net.IP to string", src: net.ParseIP("192.168.1.10"), dst: new(string), expect: "192.168.1.10"},
	}

	var box = func(val any) any {
		switch v := val.(type) {
		case int:
			return &v
		case uint:
			return &v
		case int16:
			return &v
		case uint16:
			return &v
		case int32:
			return &v
		case uint32:
			return &v
		case int64:
			return &v
		case uint64:
			return &v
		case float64:
			return &v
		case float32:
			return &v
		case string:
			return &v
		case time.Time:
			return &v
		case []byte:
			return &v
		case net.IP:
			return &v
		case driver.Value:
			return &v
		default:
			return val
		}
	}

	for _, tt := range tests {
		if err := client.Scan(tt.src, tt.dst, time.UTC); err != nil {
			t.Errorf("%s: Scan(%v, %v) got error: %v", tt.name, tt.src, tt.dst, err)
		}
		result := client.Unbox(tt.dst)
		require.EqualValues(t, tt.expect, result, "%s: Scan(%T, %T) got %v, want %v", tt.name, tt.src, tt.dst, result, tt.expect)

		if err := client.Scan(box(tt.src), tt.dst, time.UTC); err != nil {
			t.Errorf("%s: Scan(*%v, %v) got error: %v", tt.name, tt.src, tt.dst, err)
		}
		result = client.Unbox(tt.dst)
		require.EqualValues(t, tt.expect, result, "%s: Scan(*%T, %T) got %v, want %v", tt.name, tt.src, tt.dst, result, tt.expect)
	}
}

func TestTagTable(t *testing.T) {
	t.Run("CreateTagTables", testCreateTagTables)
	t.Run("TableExists", testTagTableExists)
	t.Run("TableType", testTagTableType)
	t.Run("DescribeTable", testDescribeTable)
	t.Run("Explain", testExplain)
	t.Run("ExplainFull", testExplainFull)
	t.Run("InsertNewTags", testInsertNewTags)
	t.Run("InsertAndQuery", testInsertAndQuery)
	t.Run("AppendTags", testAppendTags)
	t.Run("QueryRow", testQueryRow)
	t.Run("Watcher", testWatcher)
	t.Run("DropTagTables", testDropTagTables)
}

func testCreateTagTables(t *testing.T) {
	conn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.ExecContext(t.Context(), SqlTidy(`
		create tag table if not exists tag_data(
			name            varchar(100) primary key, 
			time            datetime basetime, 
			value           double summarized,
			short_value     short,
			ushort_value    ushort,
			int_value       integer,
			uint_value 	    uinteger,
			long_value      long,
			ulong_value 	ulong,
			str_value       varchar(400),
			json_value      json,
			ipv4_value      ipv4,
			ipv6_value      ipv6,
			bin_value		binary
		) TAG_DUPLICATE_CHECK_DURATION=1;
	`))
	require.NoError(t, err)

	_, err = conn.ExecContext(t.Context(), SqlTidy(`
		create tag table if not exists tag_simple(
			name            varchar(100) primary key, 
			time            datetime basetime, 
			value           double
		) TAG_DUPLICATE_CHECK_DURATION=1;
	`))
	require.NoError(t, err)
}

func testDropTagTables(t *testing.T) {
	conn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.ExecContext(t.Context(), "DROP TABLE tag_data")
	require.NoError(t, err)
	_, err = conn.ExecContext(t.Context(), "DROP TABLE tag_simple")
	require.NoError(t, err)
}

func testTagTableType(t *testing.T) {
	ctx := t.Context()
	db, err := spi.DefaultPool()
	require.NoError(t, err)
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	defer conn.Close()

	typeTag, err := spi.QueryTableType(ctx, conn, "tag_data")
	require.NoError(t, err)
	require.Equal(t, client.TableTypeTag, typeTag)

	_, err = spi.QueryTableType(ctx, conn, "table_not_exists")
	require.Error(t, err)

	exists, truncated, err := spi.TruncateTableIfExists(ctx, conn, "table_not_exists", true)
	require.NoError(t, err)
	require.False(t, exists)
	require.False(t, truncated)
}

func testDescribeTable(t *testing.T) {
	conn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	expect := client.Columns{
		{Name: "NAME", Type: api.ColumnTypeVarchar, DataType: api.DataTypeString},
		{Name: "TIME", Type: api.ColumnTypeDatetime, DataType: api.DataTypeDatetime},
		{Name: "VALUE", Type: api.ColumnTypeDouble, DataType: api.DataTypeFloat64},
		{Name: "SHORT_VALUE", Type: api.ColumnTypeShort, DataType: api.DataTypeInt16},
		{Name: "USHORT_VALUE", Type: api.ColumnTypeUShort, DataType: api.DataTypeUInt16},
		{Name: "INT_VALUE", Type: api.ColumnTypeInteger, DataType: api.DataTypeInt32},
		{Name: "UINT_VALUE", Type: api.ColumnTypeUInteger, DataType: api.DataTypeUInt32},
		{Name: "LONG_VALUE", Type: api.ColumnTypeLong, DataType: api.DataTypeInt64},
		{Name: "ULONG_VALUE", Type: api.ColumnTypeULong, DataType: api.DataTypeUInt64},
		{Name: "STR_VALUE", Type: api.ColumnTypeVarchar, DataType: api.DataTypeString},
		{Name: "JSON_VALUE", Type: api.ColumnTypeJSON, DataType: api.DataTypeJSON},
		{Name: "IPV4_VALUE", Type: api.ColumnTypeIPv4, DataType: api.DataTypeIPv4},
		{Name: "IPV6_VALUE", Type: api.ColumnTypeIPv6, DataType: api.DataTypeIPv6},
		{Name: "BIN_VALUE", Type: api.ColumnTypeBinary, DataType: api.DataTypeBinary},
		{Name: "_RID", Type: api.ColumnTypeLong, DataType: api.DataTypeInt64},
	}

	expectColumns := []map[string]interface{}{
		{"name": "NAME", "type": "varchar", "data_type": "string", "length": 100, "flag": api.ColumnFlagTagName},
		{"name": "TIME", "type": "datetime", "data_type": "datetime", "length": 8, "flag": api.ColumnFlagBasetime},
		{"name": "VALUE", "type": "double", "data_type": "double", "length": 8, "flag": api.ColumnFlagSummarized},
		{"name": "SHORT_VALUE", "type": "short", "data_type": "int16", "length": 2},
		{"name": "USHORT_VALUE", "type": "ushort", "data_type": "uint16", "length": 2},
		{"name": "INT_VALUE", "type": "integer", "data_type": "int32", "length": 4},
		{"name": "UINT_VALUE", "type": "uinteger", "data_type": "uint32", "length": 4},
		{"name": "LONG_VALUE", "type": "long", "data_type": "int64", "length": 8},
		{"name": "ULONG_VALUE", "type": "ulong", "data_type": "uint64", "length": 8},
		{"name": "STR_VALUE", "type": "varchar", "data_type": "string", "length": 400},
		{"name": "JSON_VALUE", "type": "json", "data_type": "json", "length": 32767},
		{"name": "IPV4_VALUE", "type": "ipv4", "data_type": "ipv4", "length": 5},
		{"name": "IPV6_VALUE", "type": "ipv6", "data_type": "ipv6", "length": 17},
		{"name": "BIN_VALUE", "type": "binary", "data_type": "binary", "length": 32767},
		{"name": "_RID", "type": "long", "data_type": "int64", "length": 8},
	}
	for _, table_name := range []string{"tag_data", "sys.tag_data", "machbasedb.sys.tag_data"} {
		// describe table
		result := spi.ShowTable(t.Context(), conn, "", "", table_name, true)
		require.NoError(t, result.Err(), "describe table %q fail", table_name)
		desc := result.Description
		require.Equal(t, "TAG_DATA", desc.Name)
		require.Equal(t, "SYS", desc.User)
		require.Equal(t, "MACHBASEDB", desc.Database)
		require.Equal(t, "Tag Table", desc.String())
		require.Equal(t, client.TableTypeTag, desc.Type)

		require.Equal(t, len(expect), len(desc.Columns))

		for i, e := range expect {
			require.Equal(t, e.Name, desc.Columns[i].Name, "column %d: name=%s", i, e.Name)
			require.Equal(t, e.Type, desc.Columns[i].Type, "column %d: name=%s", i, e.Name)
			require.Equal(t, e.DataType, desc.Columns[i].DataType, "column %d: name=%s", i, e.Name)
		}

		if table_name != "tag_data" {
			continue
		}

		buf := &bytes.Buffer{}
		json.NewEncoder(buf).Encode(desc)

		m := make(map[string]interface{})
		json.Unmarshal(buf.Bytes(), &m)

		require.Equal(t, "TAG_DATA", m["name"])
		require.Equal(t, "SYS", m["user"])
		require.Equal(t, "MACHBASEDB", m["database"])
		require.Equal(t, "TagTable", m["type"])
		require.Equal(t, 15, len(m["columns"].([]interface{})))

		columns := m["columns"].([]interface{})

		for i, e := range expectColumns {
			col := columns[i].(map[string]interface{})
			col["length"] = int(col["length"].(float64))
			if flag, ok := col["flag"]; ok {
				col["flag"] = int(flag.(float64))
			}
			// copy actual id to expected id, just for comparison
			if floatId, ok := col["id"]; ok {
				e["id"] = int(floatId.(float64))
				col["id"] = int(floatId.(float64))
			}
			require.Equal(t, e, col)
		}
	}

	result := spi.ShowTable(t.Context(), conn, "MACHBASEDB", "SYS", "m$sys_tables", false)
	require.NoError(t, result.Err(), "describe m$sys_tables fail")
	desc := result.Description
	require.Equal(t, "M$SYS_TABLES", desc.Name)
}

func testTagTableExists(t *testing.T) {
	ctx := t.Context()
	db, err := spi.DefaultPool()
	require.NoError(t, err)
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	defer conn.Close()
	for _, table_name := range []string{"tag_data", "sys.tag_data", "machbasedb.sys.tag_data"} {
		// table exists
		exists, err := spi.ExistsTable(t.Context(), conn, table_name)
		require.NoError(t, err, "exists table %q fail", table_name)
		require.True(t, exists, "table %q not exists", table_name)

		// table not exists
		exists, err = spi.ExistsTable(t.Context(), conn, table_name+"_not_exists")
		require.NoError(t, err, "exists table %q_not_exists fail", table_name)
		require.False(t, exists, "table %q_not_exists exists", table_name)

		// table exists and truncate
		exists, truncated, err := spi.TruncateTableIfExists(t.Context(), conn, table_name, true)
		require.NoError(t, err, "exists table %q fail", table_name)
		require.True(t, exists, "table %q not exists", table_name)
		require.True(t, truncated, "table %q not truncated", table_name)
	}
}

func testExplain(t *testing.T) {
	conn, err := spi.Connect(t.Context(), "sys")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	sqlText := `select * from TAG_DATA order by time desc`
	plan := ""
	conn.Raw(func(driverConn any) error {
		if c, ok := driverConn.(spi.Explainer); ok {
			// Use the Explainer interface if available
			if text, err := c.Explain(t.Context(), sqlText, false); err != nil {
				t.Fatal(err)
			} else {
				plan = text
			}
		} else {
			t.Fatal("database driver does not support Explain interface")
		}
		return nil
	})

	require.True(t, len(plan) > 0)
	require.True(t, strings.HasPrefix(plan, " PROJECT"))
	require.True(t, strings.Contains(plan, "KEYVALUE FULL SCAN"))
	require.True(t, strings.Contains(plan, "VOLATILE FULL SCAN"))
}

func testExplainFull(t *testing.T) {
	conn, err := spi.Connect(t.Context(), "sys")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	sqlText := `select * from TAG_DATA order by time desc`
	plan := ""
	conn.Raw(func(driverConn any) error {
		if c, ok := driverConn.(spi.Explainer); ok {
			// Use the Explainer interface if available
			if text, err := c.Explain(t.Context(), sqlText, true); err != nil {
				t.Fatal(err)
			} else {
				plan = text
			}
		} else {
			t.Fatal("database driver does not support Explain interface")
		}
		return nil
	})

	require.True(t, len(plan) > 0)
	require.True(t, strings.Contains(plan, "********"))
	require.True(t, strings.Contains(plan, " NAME           COUNT   ACCUM(ms)  AVG(ms)"))
}

func testQueryRow(t *testing.T) {
	conn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	// QueryRowContext does not return an error immediately when no rows are found,
	// Instead, the error is deferred until the Scan method is called.
	row := conn.QueryRowContext(t.Context(), "SELECT * from tag_data WHERE name='_not_exist_'")
	require.NoError(t, row.Err())
	var result int
	err = row.Scan(&result)
	require.EqualError(t, err, "sql: no rows in result set")

	row = conn.QueryRowContext(t.Context(), "SELECT count(*) from tag_data")
	require.NoError(t, row.Err())
	userMessage := spi.MakeUserMessage(spi.DetectSQLStatementType("SELECT count(*) from tag_data"), 1)
	require.Equal(t, "a row selected.", userMessage)

	var count int64
	err = row.Scan(&count)
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(0))
}

func testWatcher(t *testing.T) {
	db, err := spi.DefaultPool()
	require.NoError(t, err)

	conf := spi.WatcherConfig{
		ConnProvider: func() (*sql.Conn, error) {
			return db.Conn(t.Context())
		},
		Timeformat: "2006-01-02 15:04:05.999999",
		Timezone:   time.UTC,
		TableName:  "tag_data",
		TagNames:   []string{"tag1", "tag2"},
	}
	w, err := spi.NewWatcher(t.Context(), conf)
	require.NoError(t, err, "new watcher fail")
	defer w.Close()

	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	tickCount := 0

	for {
		select {
		case data := <-w.C:
			if err, ok := data.(error); ok {
				t.Log("Error", err.Error())
				t.Fail()
				return
			} else if rec, ok := data.(spi.WatchData); !ok {
				t.Log("Data", data)
				t.Fail()
				return
			} else {
				if tickCount > 5 {
					return
				}
				require.Equal(t, 4, len(rec["NAME"].(string)), "NAME")
				require.IsType(t, "", rec["TIME"], "TIME")
				require.LessOrEqual(t, 1.23, rec["VALUE"], "VALUE")
				require.Equal(t, int16(1), rec["SHORT_VALUE"], "SHORT_VALUE")
				require.Equal(t, nil, rec["USHORT_VALUE"], "USHORT_VALUE")
				require.Less(t, int32(0), rec["INT_VALUE"], "INT_VALUE")
				require.Equal(t, int64(2), rec["LONG_VALUE"], "LONG_VALUE")
				require.Equal(t, "str1", rec["STR_VALUE"], "STR_VALUE")
				require.Equal(t, api.JSONString(`{"key1":"value1"}`), rec["JSON_VALUE"], "JSON_VALUE")
			}
		case <-tick.C:
			tickCount++
			conn, err := conf.ConnProvider()
			require.NoError(t, err, "connect fail")
			name := "tag1"
			if tickCount%2 == 0 {
				name = "tag2"
			}
			values := []any{name, time.Now(), 1.23 * float64(tickCount), 1, tickCount, 2, "str1", `{"key1":"value1"}`}
			_, err = conn.ExecContext(t.Context(), `insert into tag_data (name, time, value, short_value, int_value, long_value, str_value, json_value) values(?, ?, ?, ?, ?, ?, ?, ?)`, values...)
			conn.Close()
			require.NoError(t, err, "insert fail")
			time.Sleep(100 * time.Millisecond)
			w.Execute()
		}
	}
}

func testInsertNewTags(t *testing.T) {
	expectCount := 1000
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		conn, err := spi.Connect(t.Context(), "sys")
		require.NoError(t, err, "connect fail")
		defer func() {
			conn.Close()
			wg.Done()
		}()
		ts := time.Now()
		for i := 0; i < expectCount; i++ {
			result, err := conn.ExecContext(t.Context(), `INSERT INTO TAG_SIMPLE (name, time, value) VALUES(?, ?, ?)`,
				fmt.Sprintf("tag-%d", i),
				ts.Add(time.Duration(i)),
				1.23*float64(i),
			)
			require.NoError(t, err, "insert fail, count=%d", i)
			affected, err := result.RowsAffected()
			require.NoError(t, err, "rows affected fail, count=%d", i)
			require.Equal(t, int64(1), affected, "expect 1 row affected, count=%d", i)
		}
	}()

	wg.Add(1)
	go func() {
		conn, err := spi.Connect(t.Context(), "sys")
		require.NoError(t, err, "connect fail")
		defer func() {
			conn.Close()
			wg.Done()
		}()
		for i := 0; i < expectCount; i++ {
			rows, err := conn.QueryContext(t.Context(), `SELECT _ID, NAME FROM _TAG_SIMPLE_META`)
			require.NoError(t, err, "list tags fail")
			count := 0
			for rows.Next() {
				count++
			}
			rows.Close()
		}
	}()

	wg.Wait()

	conn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	rows, err := conn.QueryContext(t.Context(), `SELECT _ID, NAME FROM _TAG_SIMPLE_META`)
	require.NoError(t, err, "list tags fail")
	count := 0
	for rows.Next() {
		count++
	}
	rows.Close()
	require.Equal(t, expectCount, count)
}

func testInsertAndQuery(t *testing.T) {
	now, _ := time.ParseInLocation("2006-01-02 15:04:05", "2021-01-01 00:00:00", time.UTC)

	// Because INSERT statement uses '2021-01-01 00:00:00' as time value which was parsed in Local timezone,
	// the time value should be converted to UTC timezone to compare
	// TODO: improve this behavior
	nowStrInLocal := now.In(time.Local).Format("2006-01-02 15:04:05")

	conn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	// insert
	func() {
		result, err := conn.ExecContext(t.Context(), `insert into tag_data (name, time, value, short_value, int_value, long_value, str_value, json_value) `+
			`values('insert-once', '`+nowStrInLocal+`', 1.23, 1, 2, 3, 'str1', '{"key1": "value1"}')`)
		require.NoError(t, err, "insert fail")
		rowsAffected, err := result.RowsAffected()
		require.NoError(t, err, "rows affected fail")
		require.Equal(t, int64(1), rowsAffected, "expect 1 row affected")
	}()

	func() {
		sysConn, err := spi.Connect(t.Context(), "sys")
		require.NoError(t, err, "connect fail")
		defer sysConn.Close()
		result, err := sysConn.ExecContext(t.Context(), `EXEC table_flush(tag_data)`)
		require.NoError(t, err, "table_flush fail")
		rowsAffected, err := result.RowsAffected()
		require.NoError(t, err, "rows affected fail")
		require.Equal(t, int64(0), rowsAffected)
	}()

	// prepare and query
	func() {
		sqlText := `select name, time, value, short_value, int_value, long_value, str_value, json_value from tag_data where name = ?`
		for nth := range 10 {
			rows, err := conn.QueryContext(t.Context(), sqlText, "insert-once")
			require.NoError(t, err, "query fail")
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
			rows.Close()
			require.Equal(t, 1, numRows, "expect 1 row in nth=%d", nth+1)
		}
	}()

	// select
	func() {
		sqlText := `select name, time, value, short_value, int_value, long_value, str_value, json_value from tag_data where name = ?`
		rows, err := conn.QueryContext(t.Context(), sqlText, "insert-once")
		require.NoError(t, err, "select fail")
		defer rows.Close()
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
	}()

	// query - select
	func() {
		sqlText := `select * from tag_data where name = ?`
		rows, err := conn.QueryContext(t.Context(), sqlText, "insert-once")
		require.NoError(t, err, "select fail")
		defer rows.Close()

		cols, err := rows.Columns()
		require.NoError(t, err, "columns fail")
		types, err := rows.ColumnTypes()
		typeNames := make([]string, len(types))
		typeScan := make([]string, len(types))
		for i, t := range types {
			typeNames[i] = t.DatabaseTypeName()
			typeScan[i] = t.ScanType().String()
		}
		require.NoError(t, err, "column types fail")
		require.Equal(t, []string{"NAME", "TIME", "VALUE",
			"SHORT_VALUE", "USHORT_VALUE", "INT_VALUE", "UINT_VALUE", "LONG_VALUE", "ULONG_VALUE",
			"STR_VALUE", "JSON_VALUE", "IPV4_VALUE", "IPV6_VALUE", "BIN_VALUE"}, cols)
		require.EqualValues(t, []string{
			"VARCHAR", "DATETIME", "DOUBLE",
			"SHORT", "USHORT", "INTEGER", "UINTEGER", "LONG", "ULONG",
			"VARCHAR", "JSON", "IPV4", "IPV6", "BINARY"}, typeNames)
		require.EqualValues(t, []string{
			"string", "time.Time", "float64",
			"int16", "uint16", "int32", "uint32", "int64", "uint64",
			"string", "api.JSONString", "net.IP", "net.IP", "[]uint8"}, typeScan)

		var nextCalled int
		for rows.Next() {
			nextCalled++
			values := spi.MakeBuffer(types)
			require.NoError(t, err)
			err = rows.Scan(values...)
			require.NoError(t, err)
			require.Equal(t, "insert-once", client.Unbox(values[0]))
			require.Equal(t, now.In(time.Local), client.Unbox(values[1]))
			require.Equal(t, 1.23, client.Unbox(values[2]))
			require.Equal(t, int16(1), client.Unbox(values[3]))
			require.Equal(t, nil, client.Unbox(values[4]))
			require.Equal(t, int32(2), client.Unbox(values[5]))
			require.Equal(t, nil, client.Unbox(values[6]))
			require.Equal(t, int64(3), client.Unbox(values[7]))
			require.Equal(t, nil, client.Unbox(values[8]))
			require.Equal(t, "str1", client.Unbox(values[9]))
			require.Equal(t, api.JSONString(`{"key1": "value1"}`), client.Unbox(values[10]))
		}
		require.NoError(t, rows.Err())
		stmtType := spi.DetectSQLStatementType(sqlText)
		require.Equal(t, "a row selected.", spi.MakeUserMessage(stmtType, int64(nextCalled)))
		require.Equal(t, 1, nextCalled)
	}()

	// query - insert
	func() {
		_, err := conn.ExecContext(t.Context(), `insert into tag_data values('insert-twice', '2021-01-01 00:00:00', ?,`+ // name, time, value
			`1, ?, ?, ?,`+ // short_value, ushort_value, int_value, uint_value
			`?, ?, `+ // long_value, ulong_value
			`?, ?, ?, ?, ? )`, // str_value, json_value, ipv4_value, ipv6_value, bin_value
			1.23,                     // value
			10,                       // ushort_value
			2,                        // int_value
			20,                       // uint_value
			3,                        // long_value
			40,                       // ulong_value
			"str1",                   // str_value
			`{"key1": "value1"}`,     // json_value
			nil,                      // ipv4_value
			nil,                      // ipv6_value
			[]byte{0x01, 0x02, 0x03}, // bin_value
		)
		require.NoError(t, err)
		userMsg := spi.MakeUserMessage(spi.SQLStatementTypeInsert, 1)
		require.Equal(t, "a row inserted.", userMsg)
	}()

	func() {
		result, err := conn.ExecContext(t.Context(), "EXEC table_flush(tag_data)")
		require.NoError(t, err, "table_flush fail")

		// tags
		spi.ListTagsWalk(t.Context(), conn, "TAG_DATA", "NAME", func(tag *spi.TagInfo, err error) bool {
			require.NoError(t, err, "tags fail")
			require.Greater(t, tag.Id, int64(0))
			require.Contains(t, []string{"insert-once", "insert-twice"}, tag.Name)
			return true
		})
		require.NoError(t, err, "tags fail")

		// tag stat
		tagStat, err := spi.QueryTagStat(t.Context(), conn, "TAG_DATA", "insert-once")
		require.NoError(t, err, "tag stat fail")
		require.Equal(t, "insert-once", tagStat.Name)
		require.Equal(t, int64(1), tagStat.RowCount)
		require.Equal(t, 1.23, tagStat.MinValue)
		require.Equal(t, 1.23, tagStat.MaxValue)

		// tag stat
		tagStat, err = spi.QueryTagStat(t.Context(), conn, "TAG_DATA", "insert-twice")
		require.NoError(t, err, "tag stat fail")
		require.Equal(t, "insert-twice", tagStat.Name)
		require.Equal(t, int64(1), tagStat.RowCount)

		// delete test data
		result, err = conn.ExecContext(t.Context(), `delete from tag_data where name = ?`, "insert-once")
		require.NoError(t, err, "delete fail")
		rowsAffected, err := result.RowsAffected()
		require.NoError(t, err, "rows affected fail")
		require.Equal(t, int64(1), rowsAffected)

		result, err = conn.ExecContext(t.Context(), `delete from tag_data where name = ?`, "insert-twice")
		require.NoError(t, err, "delete fail")
		rowsAffected, err = result.RowsAffected()
		require.NoError(t, err, "rows affected fail")
		require.Equal(t, int64(1), rowsAffected)
	}()

}

func testAppendTags(t *testing.T) {
	dsn := spi.DefaultDSN(map[string]string{"user": "sys"})
	appender := &client.Appender{}
	if err := appender.Connect(t.Context(), dsn, "tag_data"); err != nil {
		t.Fatal(err)
	}
	defer appender.Close()
	require.Equal(t, "TAG_DATA", appender.TableName())
	require.Equal(t, client.TableTypeTag, appender.TableType())
	appender = appender.WithInputFormats()

	// On systems with slow network configurations (e.g., GitHub Actions runners),
	// the appender may flush data too frequently (default: 5ms), causing rapid,
	// fragmented exchanges that can fail tests. Disable delay based flushing by setting it to 0.
	appender = appender.
		WithBatchMaxDelay(0).
		WithBatchMaxBytes(1024). // reduce tcp packet size
		WithBatchMaxRows(2000)

	expectCols := []*client.Column{
		{Name: "NAME", Type: api.ColumnTypeVarchar, Length: 100, DataType: api.DataTypeString},
		{Name: "TIME", Type: api.ColumnTypeDatetime, Length: 8, DataType: api.DataTypeDatetime},
		{Name: "VALUE", Type: api.ColumnTypeDouble, Length: 8, DataType: api.DataTypeFloat64},
		{Name: "SHORT_VALUE", Type: api.ColumnTypeShort, Length: 2, DataType: api.DataTypeInt16},
		{Name: "USHORT_VALUE", Type: api.ColumnTypeUShort, Length: 2, DataType: api.DataTypeUInt16},
		{Name: "INT_VALUE", Type: api.ColumnTypeInteger, Length: 4, DataType: api.DataTypeInt32},
		{Name: "UINT_VALUE", Type: api.ColumnTypeUInteger, Length: 4, DataType: api.DataTypeUInt32},
		{Name: "LONG_VALUE", Type: api.ColumnTypeLong, Length: 8, DataType: api.DataTypeInt64},
		{Name: "ULONG_VALUE", Type: api.ColumnTypeULong, Length: 8, DataType: api.DataTypeUInt64},
		{Name: "STR_VALUE", Type: api.ColumnTypeVarchar, Length: 400, DataType: api.DataTypeString},
		{Name: "JSON_VALUE", Type: api.ColumnTypeJSON, Length: 32767, DataType: api.DataTypeJSON},
		{Name: "IPV4_VALUE", Type: api.ColumnTypeIPv4, Length: 5, DataType: api.DataTypeIPv4},
		{Name: "IPV6_VALUE", Type: api.ColumnTypeIPv6, Length: 17, DataType: api.DataTypeIPv6},
		{Name: "BIN_VALUE", Type: api.ColumnTypeBinary, Length: 32767, DataType: api.DataTypeBinary},
	}
	cols := appender.Columns()
	require.Equal(t, len(expectCols), len(cols))
	for i, c := range cols {
		require.Equal(t, expectCols[i].Name, c.Name)
		require.Equal(t, expectCols[i].Type, c.Type, "diff column: "+c.Name)
		require.Equal(t, expectCols[i].DataType, c.DataType, "diff column: "+c.Name)
		require.Equal(t, expectCols[i].Length, c.Length, "diff column: "+c.Name)
	}

	// FIXME: windows github actions runner failed to append 10000 rows, need to investigate further, for now reduce the count to 5000
	// It might be related with host's network configurations.
	//
	// For the refrence, here are some settings that can be applied to Windows to improve the performance of appending large number of rows:
	//
	// - name: Windows Network Tuning
	//    if: matrix.os == 'windows'
	//    shell: powershell
	//    run: |
	//      Write-Host "===== BEFORE SETTINGS ====="
	//      netsh int tcp show global
	//      netsh int ipv4 show dynamicport tcp

	//      Write-Host "===== EXPAND DYNAMIC PORT ====="
	//      netsh int ipv4 set dynamicport tcp start=10000 num=55000

	//      Write-Host "===== REDUCE TIME_WAIT ====="
	//      reg add HKLM\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters `
	//        /v TcpTimedWaitDelay /t REG_DWORD /d 30 /f

	//      Write-Host "===== DISABLE TCP AUTOTUNING ====="
	//      netsh int tcp set global autotuninglevel=disabled

	//      Write-Host "===== AFTER SETTINGS ====="
	//      netsh int ipv4 show dynamicport tcp
	//
	// expectCount := 10000
	expectCount := 5000
	var err error
	for i := 0; i < expectCount; i++ {
		ip4 := net.ParseIP(fmt.Sprintf("192.168.0.%d", i%255))
		ip6 := net.ParseIP(fmt.Sprintf("12:FF:FF:FF:CC:EE:FF:%02X", i%255))
		varchar := fmt.Sprintf("varchar_append-%d", i)
		err = appender.Append(
			fmt.Sprintf("name-%d", i%100),   // name
			time.Now(),                      // time
			float64(i)*1.1,                  // value
			int16(i),                        // short_value
			uint16(i*10),                    // ushort_value
			int(i*100),                      // int_value
			uint(i*1000),                    // uint_value
			int64(i*10000),                  // long_value
			uint64(i*100000),                // ulong_value
			varchar,                         // str_value
			fmt.Sprintf("{\"json\":%d}", i), // json_value
			ip4,                             // IPv4_value
			ip6,                             // IPv6_value
			[]byte{0x01, 0x02, 0x03},        // bin_value
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(10 * time.Millisecond) // wait for appender to flush
	err = appender.Flush()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond) // wait for appender to flush
	sc, fc, err := appender.Close()
	require.NoError(t, err)
	require.Equal(t, int64(expectCount), sc)
	require.Equal(t, int64(0), fc)
}

func TestInsertMeta(t *testing.T) {
	conn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err)
	defer conn.Close()

	// create tag table
	result, err := conn.ExecContext(t.Context(), SqlTidy(`
		CREATE TAG TABLE MYTAG (
			name varchar(32) primary key,
			time datetime basetime,
			value double summarized
		) METADATA(
			factory varchar(32),
			equipment varchar(64) 
		)`))
	_ = result
	require.NoError(t, err)

	result, err = conn.ExecContext(t.Context(), "INSERT INTO MYTAG METADATA(name, factory, equipment) values('FA1_CNC', 'FA1', 'CNC')")
	require.NoError(t, err)
	result, err = conn.ExecContext(t.Context(), "INSERT INTO MYTAG METADATA(name, factory, equipment) values('FA4_MILLING', 'FA4', 'MILLING')")
	require.NoError(t, err)

	// flush
	result, err = conn.ExecContext(t.Context(), "EXEC table_flush(MYTAG)")
	require.NoError(t, err, "table_flush fail")

	// select tag metadata
	rows, err := conn.QueryContext(t.Context(), "SELECT _id, name, factory, equipment FROM _MYTAG_META")
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
	result, err = conn.ExecContext(t.Context(), "DROP TABLE MYTAG")
	require.NoError(t, err)
}

func TestBitTable(t *testing.T) {
	conn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	result, err := conn.ExecContext(t.Context(),
		"CREATE TABLE bit_table (i1 INTEGER, i2 UINTEGER, i3 FLOAT, i4 DOUBLE, i5 SHORT, i6 VARCHAR(10))",
	)
	_ = result
	require.NoError(t, err, "create bit table fail")

	result, err = conn.ExecContext(t.Context(), "INSERT INTO bit_table VALUES (-1, 1, 1, 1, 2, 'aaa')")
	require.NoError(t, err, "insert bit table fail")

	rows, err := conn.QueryContext(t.Context(), "SELECT * FROM bit_table WHERE BITAND(i2, 1) = 1")
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

	rows, err = conn.QueryContext(t.Context(), "SELECT * FROM bit_table WHERE BITAND(i4, 1) = 1")
	require.Error(t, err, "select bit table BITAND(i4, 1) should fail")
	require.Nil(t, rows, "select bit table BITAND(i4, 1) should fail")
	// https://github.com/machbase/neo/issues/956
	require.Equal(t, "MACHCLI-ERR-2037, Function [BITAND] argument data type is mismatched.", err.Error())
	if rows != nil {
		rows.Close()
	}

	rows, err = conn.QueryContext(t.Context(), "SELECT BITAND(i1, i3) FROM bit_table")
	require.Error(t, err, "select bit table BITAND(i1, i3) should fail")
	require.Nil(t, rows, "select bit table BITAND(i1, i3) should fail")
	// https://github.com/machbase/neo/issues/956
	require.Equal(t, "MACHCLI-ERR-2037, Function [BITAND] argument data type is mismatched.", err.Error())
	if rows != nil {
		rows.Close()
	}

	result, err = conn.ExecContext(t.Context(), "DROP TABLE bit_table")
	require.NoError(t, err, "drop bit table fail")
}

func TestAppendTagAndQuery(t *testing.T) {
	tableName := "append_tag"

	conn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err, "connect fail")
	result, err := conn.ExecContext(t.Context(), fmt.Sprintf(`CREATE TAG TABLE %s (
		name     varchar(200) primary key,
		time     datetime basetime,
		value    double summarized,
		id       varchar(80),
		jsondata json,
		bindata  binary)`, tableName))
	_ = result
	require.NoError(t, err, "create table fail")
	conn.Close()

	defer func() {
		conn, err := spi.Connect(t.Context(), "sys")
		require.NoError(t, err, "connect fail")
		conn.ExecContext(t.Context(), fmt.Sprintf(`DROP TABLE %s`, tableName))
		conn.Close()
	}()

	testCount := 100
	ts := time.Now()

	appender := &client.Appender{}
	err = appender.Connect(t.Context(), spi.DefaultDSN(map[string]string{"user": "sys"}), tableName)
	require.NoError(t, err, "appender connect fail")

	require.Equal(t, strings.ToUpper(tableName), appender.TableName())
	require.Equal(t, client.TableTypeTag, appender.TableType())

	for i := 0; i < testCount; i++ {
		err = appender.Append(
			fmt.Sprintf("name-%d", i%5),
			ts.Add(time.Duration(i)),
			1.001*float64(i+1),
			"some-id-string",
			`{"name":"json"}`,
			[]byte{0x01, 0x02, 0x03},
		)
		if err != nil {
			panic(err)
		}
	}
	appender.Close()

	conn, err = spi.Connect(t.Context(), "sys")
	require.NoError(t, err)
	row := conn.QueryRowContext(t.Context(), "select count(*) from "+tableName+" where time >= ?", ts)
	require.NoError(t, row.Err())
	defer conn.Close()

	var count int
	err = row.Scan(&count)
	require.NoError(t, err)
	require.Equal(t, testCount, count)

	rows, err := conn.QueryContext(t.Context(), "select * from "+tableName+" where time >= ?", ts)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var time time.Time
		var value float64
		var id string
		var jsondata string
		var bindata []byte
		err = rows.Scan(&name, &time, &value, &id, &jsondata, &bindata)
		if err != nil {
			panic(err)
		}
		require.NotEmpty(t, name)
		require.NotZero(t, time)
		require.NotZero(t, value)
		require.NotEmpty(t, id)
		require.Equal(t, `{"name":"json"}`, jsondata)
		require.Equal(t, []byte{0x01, 0x02, 0x03}, bindata)
	}
}

func TestAppendTagNotExist(t *testing.T) {
	appender := &client.Appender{}
	err := appender.Connect(t.Context(), spi.DefaultDSN(map[string]string{"user": "sys"}), "notexist")
	require.True(t, strings.Contains(err.Error(), "does not exist"), err.Error())
	appender.Close()
}

func TestAppendTagPartial(t *testing.T) {
	tableName := "append_tag2"

	conn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err, "connect fail")
	result, err := conn.ExecContext(t.Context(), fmt.Sprintf(`
	CREATE TAG TABLE %s (
		name     varchar(200) primary key,
		time     datetime basetime,
		value    double summarized,
		id       varchar(80),
		jsondata json)
	METADATA( factory varchar(32), equipment varchar(64) )`, tableName))
	_ = result
	conn.Close()
	require.NoError(t, err, "create table fail")

	defer func() {
		conn, err := spi.Connect(t.Context(), "sys")
		require.NoError(t, err, "connect fail")
		conn.ExecContext(t.Context(), fmt.Sprintf(`DROP TABLE %s`, tableName))
		conn.Close()
	}()

	conn, err = spi.Connect(t.Context(), "sys")
	require.NoError(t, err)

	testCount := 100
	ts := time.Now()

	appender := &client.Appender{}
	err = appender.Connect(t.Context(), spi.DefaultDSN(map[string]string{"user": "sys"}), tableName)

	require.Equal(t, strings.ToUpper(tableName), appender.TableName())
	require.Equal(t, client.TableTypeTag, appender.TableType())

	// arbitrary column order
	appender = appender.WithInputColumns("time", "name", "jsondata", "value")

	for i := 0; i < testCount; i++ {
		err = appender.Append(
			ts.Add(time.Duration(i)),
			fmt.Sprintf("name-%d", i%5),
			`{"name":"json"}`,
			1.001*float64(i+1))
		if err != nil {
			panic(err)
		}
	}
	appender.Close()

	conn.Close()

	conn, err = spi.Connect(t.Context(), "sys")
	require.NoError(t, err)
	row := conn.QueryRowContext(t.Context(), "select count(*) from "+tableName+" where time >= ?", ts)
	require.NoError(t, row.Err())

	var count int
	err = row.Scan(&count)
	require.NoError(t, err)

	require.Equal(t, testCount, count)
	conn.Close()
}

func TestDemoUser(t *testing.T) {
	sysConn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err, "connect fail")
	defer sysConn.Close()

	result, err := sysConn.ExecContext(t.Context(), "CREATE USER demo IDENTIFIED BY demo")
	_ = result
	require.NoError(t, err)
	defer func() {
		result, err := sysConn.ExecContext(t.Context(), "DROP TABLE demo.TAG_DATA")
		_ = result
		require.NoError(t, err)
		result, err = sysConn.ExecContext(t.Context(), "DROP USER demo")
		require.NoError(t, err)
	}()

	// create table
	conn, err := spi.Connect(t.Context(), "demo")
	require.NoError(t, err, "connect fail")

	result, err = conn.ExecContext(t.Context(), "CREATE TAG TABLE tag_data (name VARCHAR(100) primary key, time datetime basetime, value double, json_value json)")
	require.NoError(t, err)

	now, _ := time.ParseInLocation("2006-01-02 15:04:05", "2021-01-01 00:00:00", time.UTC)
	// insert tag_data
	result, err = conn.ExecContext(t.Context(), `insert into tag_data values('demo-1', ?, 1.23, '{"key1": "value1"}')`, now)
	require.NoError(t, err, "insert fail")

	// insert demo.tag_data
	result, err = sysConn.ExecContext(t.Context(), `insert into demo.tag_data values('demo-1', ?, 1.23, '{"key1": "value1"}')`, now.Add(1))
	require.NoError(t, err, "insert fail")

	result, err = sysConn.ExecContext(t.Context(), "exec table_flush(demo.tag_data)")
	require.NoError(t, err, "table_flush fail")

	row := sysConn.QueryRowContext(t.Context(), "select count(*) from demo.tag_data where name = ?", "demo-1")
	require.NoError(t, row.Err())
	var count int
	row.Scan(&count)
	require.Equal(t, 2, count)

	result, err = conn.ExecContext(t.Context(), `drop table tag_data`)
	require.NoError(t, err, "drop table fail")
	conn.Close()

	// connect as proxy user
	proxyConn, err := spi.Connect(t.Context(), "demo")
	require.NoError(t, err, "connect fail")
	defer proxyConn.Close()

	result, err = proxyConn.ExecContext(t.Context(), "CREATE TAG TABLE tag_data (name VARCHAR(100) primary key, time datetime basetime, value double, json_value json)")
	require.NoError(t, err, "create table fail")

	// insert tag_data
	result, err = proxyConn.ExecContext(t.Context(), `insert into tag_data values('demo-1', ?, 1.23, '{"key1": "value1"}')`, now)
	require.NoError(t, err, "insert fail")

	// insert demo.tag_data
	result, err = sysConn.ExecContext(t.Context(), `insert into demo.tag_data values('demo-1', ?, 1.23, '{"key1": "value1"}')`, now.Add(1))
	require.NoError(t, err, "insert fail")

	result, err = sysConn.ExecContext(t.Context(), "exec table_flush(demo.tag_data)")
	require.NoError(t, err, "table_flush fail")

	row = sysConn.QueryRowContext(t.Context(), "select count(*) from demo.tag_data where name = ?", "demo-1")
	require.NoError(t, row.Err())
	row.Scan(&count)
	require.Equal(t, 2, count)
}

func TestLogTable(t *testing.T) {
	t.Run("CreateLogTable", testCreateLogTable)
	t.Run("LogTableType", testLogTableType)
	t.Run("LogInsertAndQuery", testLogInsertAndQuery)
	t.Run("ColumnCases", testColumnCases)
	t.Run("LogTableAppend", testLogTableAppend)
	t.Run("DropLogTable", testDropLogTable)
}

func testCreateLogTable(t *testing.T) {
	conn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err, "connect fail")
	defer conn.Close()
	_, err = conn.ExecContext(t.Context(), SqlTidy(`
		create table if not exists log_data(
		    time datetime,
			short_value short,
			ushort_value ushort,
			int_value integer,
			uint_value uinteger,
			long_value long,
			ulong_value ulong,
			double_value double,
			float_value float,
			str_value varchar(400),
			json_value json,
			ipv4_value ipv4,
			ipv6_value ipv6,
			text_value text,
			bin_value binary)
	`))
	require.NoError(t, err)

	exists, truncated, err := spi.TruncateTableIfExists(t.Context(), conn, "log_data", false)
	require.NoError(t, err)
	require.True(t, exists)
	require.False(t, truncated)

}

func testDropLogTable(t *testing.T) {
	conn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	_, err = conn.ExecContext(t.Context(), "DROP TABLE log_data")
	require.NoError(t, err)
}

func testLogTableType(t *testing.T) {
	conn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	tableType, err := spi.QueryTableType(t.Context(), conn, "log_data")
	require.NoError(t, err)
	require.Equal(t, client.TableTypeLog, tableType)
}

func testLogInsertAndQuery(t *testing.T) {
	conn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	var one int = 1
	var two int = 2
	var three int16 = 3
	var four int16 = 4
	var five int32 = 5
	var f32 float32 = 6.6
	var f64 float64 = 7.77
	var tick time.Time = time.Now()

	result, err := conn.ExecContext(t.Context(), "insert into log_data values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		tick,   // time
		0, one, // short, ushort
		&two, three, // int, uint
		&four, five, // long, ulong
		f64, f32, // double, float
		"hello world",                                                    // str_value
		`{"data":"some_data", "id":1}`,                                   // json
		net.ParseIP("127.0.0.1"), net.ParseIP("AB:CC:CC:CC:CC:CC:CC:FF"), // ipv4, ipv6
		fmt.Sprintf("varchar_1_%s.", randomVarchar()), // text_value
		[]byte("binary_00"),                           // bin_value
	)
	require.NoError(t, err)
	affected, err := result.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)

	result, err = conn.ExecContext(t.Context(), "insert into log_data values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		tick.Add(1), // time
		0, one,      // short, ushort
		&two, three, // int, uint
		&four, five, // long, ulong
		f64, f32, // double, float
		"hello world",                                                    // str_value
		`{"data":"some_data", "id":2}`,                                   // json
		net.ParseIP("127.0.0.1"), net.ParseIP("AB:CC:CC:CC:CC:CC:CC:FF"), // ipv4, ipv6
		fmt.Sprintf("varchar_2_%s.", randomVarchar()), // text_value
		[]byte("binary_01"),                           // bin_value
	)
	require.NoError(t, err)
	affected, err = result.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)
}

func testColumnCases(t *testing.T) {
	conn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	rows, err := conn.QueryContext(t.Context(), "select TiMe,Short_Value from log_data limit 10")
	if err != nil {
		t.Fatal(err)
	}
	require.NotNil(t, rows, "no rows selected")
	defer rows.Close()
	colums, err := rows.ColumnTypes()
	if err != nil {
		t.Fatal(err)
	}
	colNames := make([]string, len(colums))
	colTypes := make([]string, len(colums))
	colLengths := make([]int64, len(colums))
	for i, typ := range colums {
		colNames[i] = typ.Name()
		colTypes[i] = typ.DatabaseTypeName()
		if length, ok := typ.Length(); ok {
			colLengths[i] = length
		}
	}

	data := []struct {
		name   string
		typ    string
		length int
	}{
		{"TiMe", "DATETIME", 8},
		{"Short_Value", "SHORT", 2},
	}
	require.Equal(t, len(data), len(colNames), "column count was %d, want %d", len(colNames), len(data))
	for i, cd := range data {
		require.Equal(t, cd.name, colNames[i], "column[%d] name was %q, want %q", i, colNames[i], cd.name)
		require.Equal(t, cd.typ, colTypes[i], "column[%d] %q's type was %q, want %q", i, colNames[i], colTypes[i], cd.typ)
		require.Equal(t, int64(cd.length), colLengths[i], "column[%d] %q's length was %d, want %d", i, colNames[i], colLengths[i], cd.length)
	}
}

func testLogTableAppend(t *testing.T) {
	conn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	appender := &client.Appender{}
	err = appender.Connect(t.Context(), spi.DefaultDSN(map[string]string{"user": "sys"}), "log_data")
	require.NoError(t, err, "appender connect fail")
	defer appender.Close()
	require.Equal(t, "LOG_DATA", appender.TableName())
	require.Equal(t, client.TableTypeLog, appender.TableType())
	appender = appender.WithInputFormats()

	expectCols := []*client.Column{
		{Name: "_ARRIVAL_TIME", Type: api.ColumnTypeDatetime, Length: 8, DataType: api.DataTypeDatetime},
		{Name: "TIME", Type: api.ColumnTypeDatetime, Length: 8, DataType: api.DataTypeDatetime},
		{Name: "SHORT_VALUE", Type: api.ColumnTypeShort, Length: 2, DataType: api.DataTypeInt16},
		{Name: "USHORT_VALUE", Type: api.ColumnTypeUShort, Length: 2, DataType: api.DataTypeUInt16},
		{Name: "INT_VALUE", Type: api.ColumnTypeInteger, Length: 4, DataType: api.DataTypeInt32},
		{Name: "UINT_VALUE", Type: api.ColumnTypeUInteger, Length: 4, DataType: api.DataTypeUInt32},
		{Name: "LONG_VALUE", Type: api.ColumnTypeLong, Length: 8, DataType: api.DataTypeInt64},
		{Name: "ULONG_VALUE", Type: api.ColumnTypeULong, Length: 8, DataType: api.DataTypeUInt64},
		{Name: "DOUBLE_VALUE", Type: api.ColumnTypeDouble, Length: 8, DataType: api.DataTypeFloat64},
		{Name: "FLOAT_VALUE", Type: api.ColumnTypeFloat, Length: 4, DataType: api.DataTypeFloat32},
		{Name: "STR_VALUE", Type: api.ColumnTypeVarchar, Length: 400, DataType: api.DataTypeString},
		{Name: "JSON_VALUE", Type: api.ColumnTypeJSON, Length: 32767, DataType: api.DataTypeJSON},
		{Name: "IPV4_VALUE", Type: api.ColumnTypeIPv4, Length: 5, DataType: api.DataTypeIPv4},
		{Name: "IPV6_VALUE", Type: api.ColumnTypeIPv6, Length: 17, DataType: api.DataTypeIPv6},
		{Name: "TEXT_VALUE", Type: api.ColumnTypeText, Length: 67108864, DataType: api.DataTypeString},
		{Name: "BIN_VALUE", Type: api.ColumnTypeBinary, Length: 67108864, DataType: api.DataTypeBinary},
	}
	cols := appender.Columns()
	require.Equal(t, len(expectCols), len(cols), strings.Join(cols.Names(), ", "))
	for i, col := range cols {
		require.Equal(t, expectCols[i].Name, col.Name)
		require.Equal(t, expectCols[i].Type, col.Type, "diff column: "+col.Name)
		require.Equal(t, expectCols[i].DataType, col.DataType, "diff column: "+col.Name)
		require.Equal(t, expectCols[i].Length, col.Length, "diff column: "+col.Name)
	}

	expectCount := 10000
	for i := 0; i < expectCount; i++ {
		ip4 := net.ParseIP(fmt.Sprintf("192.168.0.%d", i%255))
		ip6 := net.ParseIP(fmt.Sprintf("12:FF:FF:FF:CC:EE:FF:%02X", i%255))
		varchar := fmt.Sprintf("varchar_append-%d", i)
		err = appender.AppendLogTime(
			time.Now(),                      // _arrival_time
			time.Now(),                      // time
			int16(i),                        // short
			uint16(i*10),                    // ushort
			int(i*100),                      // int
			uint(i*1000),                    // uint
			int64(i*10000),                  // long
			uint64(i*100000),                // ulong
			float64(i),                      // double
			float32(i),                      // float
			varchar,                         // varchar
			fmt.Sprintf("{\"json\":%d}", i), // json
			ip4,                             // IPv4
			ip6,                             // IPv6
			fmt.Sprintf("text_append-%d-%s.", i, randomVarchar()),
			[]byte(fmt.Sprintf("binary_append_%02d", i)),
		)
		require.NoError(t, err)
	}
	sc, fc, err := appender.Close()
	require.NoError(t, err)
	require.Equal(t, int64(expectCount), sc)
	require.Equal(t, int64(0), fc)
	sc, fc, err = appender.Close()
	require.NoError(t, err)
	require.Equal(t, int64(expectCount), sc)
	require.Equal(t, int64(0), fc)
}

func randomVarchar() string {
	rangeStart := 0
	rangeEnd := 10
	offset := rangeEnd - rangeStart
	randLength := rand.IntN(offset) + rangeStart

	charset := "aAbBcCdDeEfFgGhHiIjJkKlLmMnNoOpPqQrRsStTuUvVwWxXyYzZ"

	b := make([]byte, randLength)
	for i := range b {
		b[i] = charset[rand.IntN(len(charset)-1)]
	}
	return string(b)
}
