package spi_test

import (
	"database/sql"
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
	t.Run("TableExists", testTableExists)
	t.Run("TableType", testTableTypes)
	t.Run("InsertAndQuery", testInsertAndQuery)
	t.Run("Watcher", testWatcher)
}

func testTableTypes(t *testing.T) {
	ctx := t.Context()
	db, err := spi.DefaultPool()
	require.NoError(t, err)
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	defer conn.Close()

	typeTag, err := spi.QueryTableType(ctx, conn, "tag_data")
	require.NoError(t, err)
	require.Equal(t, api.TableTypeTag, typeTag)

	_, err = spi.QueryTableType(ctx, conn, "table_not_exists")
	require.Error(t, err)

	exists, truncated, err := spi.TruncateTableIfExists(ctx, conn, "table_not_exists", true)
	require.NoError(t, err)
	require.False(t, exists)
	require.False(t, truncated)

	exists, truncated, err = spi.TruncateTableIfExists(ctx, conn, "log_data", false)
	require.NoError(t, err)
	require.True(t, exists)
	require.False(t, truncated)
}

func testTableExists(t *testing.T) {
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
		for i, t := range types {
			typeNames[i] = t.DatabaseTypeName()
		}
		require.NoError(t, err, "column types fail")
		require.Equal(t, []string{"NAME", "TIME", "VALUE",
			"SHORT_VALUE", "USHORT_VALUE", "INT_VALUE", "UINT_VALUE", "LONG_VALUE", "ULONG_VALUE",
			"STR_VALUE", "JSON_VALUE", "IPV4_VALUE", "IPV6_VALUE", "BIN_VALUE"}, cols)
		require.EqualValues(t, []string{
			"VARCHAR", "DATETIME", "DOUBLE",
			"SHORT", "USHORT", "INTEGER", "UINTEGER", "LONG", "ULONG",
			"VARCHAR", "JSON", "IPV4", "IPV6", "BINARY"}, typeNames)

		var nextCalled int
		for rows.Next() {
			nextCalled++
			values := spi.MakeBuffer(types)
			require.NoError(t, err)
			err = rows.Scan(values...)
			require.NoError(t, err)
			require.Equal(t, "insert-once", api.Unbox(values[0]))
			require.Equal(t, now.In(time.Local), api.Unbox(values[1]))
			require.Equal(t, 1.23, api.Unbox(values[2]))
			require.Equal(t, int16(1), api.Unbox(values[3]))
			require.Equal(t, nil, api.Unbox(values[4]))
			require.Equal(t, int32(2), api.Unbox(values[5]))
			require.Equal(t, nil, api.Unbox(values[6]))
			require.Equal(t, int64(3), api.Unbox(values[7]))
			require.Equal(t, nil, api.Unbox(values[8]))
			require.Equal(t, "str1", api.Unbox(values[9]))
			require.Equal(t, api.JSONString(`{"key1": "value1"}`), api.Unbox(values[10]))
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
