package spi_test

import (
	"database/sql/driver"
	_ "embed"
	"net"
	"os"
	"testing"
	"time"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/spi"
	"github.com/machbase/neo-server/v8/spi/testsuite"
	"github.com/stretchr/testify/require"
)

var testServer *testsuite.Server

func TestMain(m *testing.M) {
	testServer = testsuite.NewServer("./testsuite_tmp")
	testServer.StartServer()
	code := m.Run()
	testServer.StopServer()
	os.Exit(code)
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
			a, b, c := api.TableName(test.input).Split()
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
		ExpectExec  func(t *testing.T, result api.Result)
		ExpectQuery func(t *testing.T, rows api.Rows, err error)
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
			ExpectExec: func(t *testing.T, result api.Result) {
				require.NoError(t, result.Err())
				require.Equal(t, "Created successfully.", result.Message())
			},
		},
		{
			Name:    "sqlite-insert",
			Bridge:  "sqlite",
			SqlExec: `INSERT INTO example (id, name, age, address) VALUES (100, 'alpha', 10, 'street-100')`,
			ExpectExec: func(t *testing.T, result api.Result) {
				require.NoError(t, result.Err())
				require.Equal(t, "a row inserted.", result.Message())
			},
		},
		{
			Name:     "sqlite-select-all",
			Bridge:   "sqlite",
			SqlQuery: `SELECT * FROM example`,
			ExpectQuery: func(t *testing.T, rows api.Rows, err error) {
				require.NoError(t, err)
				columns, err := rows.Columns()
				require.NoError(t, err)
				require.NotNil(t, columns)
				require.Equal(t, []string{"id", "name", "age", "address", "weight", "memo"}, columns.Names())
				require.Equal(t, []api.DataType{
					api.DataTypeInt64, api.DataTypeString, api.DataTypeInt64,
					api.DataTypeString, api.DataTypeFloat64, api.DataTypeBinary}, columns.DataTypes())
				require.True(t, rows.Next())
				buff, err := columns.MakeBuffer()
				require.NoError(t, err)
				err = rows.Scan(buff...)
				require.NoError(t, err)
				require.Equal(t, int64(100), buff[0])
				require.Equal(t, "alpha", buff[1])
				require.Equal(t, int64(10), buff[2])
				require.Equal(t, "street-100", buff[3])
				nilRaw := new([]byte)
				require.EqualValues(t, nilRaw, buff[5])
			},
		},
		{
			Name:     "sqlite-select-count",
			Bridge:   "sqlite",
			SqlQuery: `SELECT count(*) FROM example`,
			ExpectQuery: func(t *testing.T, rows api.Rows, err error) {
				require.NoError(t, err)
				columns, err := rows.Columns()
				require.NoError(t, err)
				require.NotNil(t, columns)
				require.Equal(t, []string{"count(*)"}, columns.Names())
				// TODO: improve datatype detection, currently it's always string; see api/columns.go scanTypeToDataType()
				require.Equal(t, []api.DataType{api.DataTypeString}, columns.DataTypes())
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

			sqlConn, err := db.Connect(ctx)
			require.NoError(t, err)

			conn := spi.WrapSqlConn(sqlConn)
			t.Cleanup(func() {
				conn.Close()
			})

			if tc.SqlExec != "" {
				result := conn.Exec(ctx, tc.SqlExec)
				tc.ExpectExec(t, result)
			} else if tc.SqlQuery != "" {
				rows, err := conn.Query(ctx, tc.SqlQuery)
				t.Cleanup(func() {
					if rows != nil {
						rows.Close()
					}
				})
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
		if err := api.Scan(tt.src, tt.dst, time.UTC); err != nil {
			t.Errorf("%s: Scan(%v, %v) got error: %v", tt.name, tt.src, tt.dst, err)
		}
		result := api.Unbox(tt.dst)
		require.EqualValues(t, tt.expect, result, "%s: Scan(%T, %T) got %v, want %v", tt.name, tt.src, tt.dst, result, tt.expect)

		if err := api.Scan(box(tt.src), tt.dst, time.UTC); err != nil {
			t.Errorf("%s: Scan(*%v, %v) got error: %v", tt.name, tt.src, tt.dst, err)
		}
		result = api.Unbox(tt.dst)
		require.EqualValues(t, tt.expect, result, "%s: Scan(*%T, %T) got %v, want %v", tt.name, tt.src, tt.dst, result, tt.expect)
	}
}

func TestDatabaseBasedCases(t *testing.T) {
	if err := testServer.CreateTestTables(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := testServer.DropTestTables(); err != nil {
			t.Fatal(err)
		}
	}()
	t.Run("Helpers", testDatabaseHelpers)
}

func testDatabaseHelpers(t *testing.T) {
	ctx := t.Context()
	conn, err := spi.Default().Connect(ctx, api.WithAuthKey("sys", spi.DefaultKey()))
	require.NoError(t, err)
	defer conn.Close()

	tablesSet := spi.QueryTables(ctx, conn, false)
	require.NoError(t, tablesSet.Err())
	tables := []string{}
	tablesSet.Iter(func(values []any) bool {
		tables = append(tables, values[2].(string))
		return true
	})
	require.GreaterOrEqual(t, len(tables), 3)

	tablesAllSet := spi.QueryTables(ctx, conn, true)
	require.NoError(t, tablesAllSet.Err())
	tablesAll := []string{}
	tablesAllSet.Iter(func(values []any) bool {
		tablesAll = append(tablesAll, values[2].(string))
		return true
	})
	require.GreaterOrEqual(t, len(tablesAll), len(tables))

	typeTag, err := spi.QueryTableType(ctx, conn, "tag_data")
	require.NoError(t, err)
	require.Equal(t, api.TableTypeTag, typeTag)

	_, err = spi.QueryTableType(ctx, conn, "table_not_exists")
	require.Error(t, err)

	indexes, err := spi.ListIndexes(ctx, conn)
	require.NoError(t, err)
	require.NotEmpty(t, indexes)

	// tags, err := spi.ListTags(ctx, conn, "tag_data", "NAME")
	// require.NoError(t, err)
	//require.Len(t, tags, 1)
	//require.Equal(t, "tag1", tags[0].Name)

	exists, truncated, err := spi.TruncateTableIfExists(ctx, conn, "table_not_exists", true)
	require.NoError(t, err)
	require.False(t, exists)
	require.False(t, truncated)

	exists, truncated, err = spi.TruncateTableIfExists(ctx, conn, "log_data", false)
	require.NoError(t, err)
	require.True(t, exists)
	require.False(t, truncated)
}
