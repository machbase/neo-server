package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	_ "embed"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/testsuite"
	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/stretchr/testify/require"
)

var testServer *testsuite.Server

func TestMain(m *testing.M) {
	testServer = testsuite.NewServer("./testsuite_tmp")
	testServer.StartServer(m)
	code := m.Run()
	testServer.StopServer(m)
	os.Exit(code)
}

func TestAll(t *testing.T) {
	if err := testServer.CreateTestTables(); err != nil {
		t.Fatal(err)
	}
	db := testsuite.Database_machsvr(t)
	testsuite.TestAll(t, db,
		tcParseCommandLine,
		tcCommands,
		tcBridge,
		tcScan,
	)
	if err := testServer.DropTestTables(); err != nil {
		t.Fatal(err)
	}
}

func TestTableName(t *testing.T) {
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

func tcBridge(t *testing.T) {
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
				require.Equal(t, &sql.NullInt64{Valid: true, Int64: 100}, buff[0])
				require.Equal(t, &sql.NullString{Valid: true, String: "alpha"}, buff[1])
				require.Equal(t, &sql.NullInt64{Valid: true, Int64: 10}, buff[2])
				require.Equal(t, &sql.NullString{Valid: true, String: "street-100"}, buff[3])
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
			ctx := context.Background()

			db, err := bridge.GetSqlBridge(tc.Bridge)
			require.NoError(t, err)

			sqlConn, err := db.Connect(ctx)
			require.NoError(t, err)

			conn := api.WrapSqlConn(sqlConn)
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

func tcScan(t *testing.T) {
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
		if err := api.Scan(tt.src, tt.dst); err != nil {
			t.Errorf("%s: Scan(%v, %v) got error: %v", tt.name, tt.src, tt.dst, err)
		}
		result := api.Unbox(tt.dst)
		require.EqualValues(t, tt.expect, result, "%s: Scan(%T, %T) got %v, want %v", tt.name, tt.src, tt.dst, result, tt.expect)

		if err := api.Scan(box(tt.src), tt.dst); err != nil {
			t.Errorf("%s: Scan(*%v, %v) got error: %v", tt.name, tt.src, tt.dst, err)
		}
		result = api.Unbox(tt.dst)
		require.EqualValues(t, tt.expect, result, "%s: Scan(*%T, %T) got %v, want %v", tt.name, tt.src, tt.dst, result, tt.expect)
	}
}

func tcParseCommandLine(t *testing.T) {
	tests := []struct {
		input  string
		expect []string
	}{
		{
			input:  "show tables -a",
			expect: []string{"show", "tables", "-a"},
		},
		{
			input:  `sql 'select * from tt where A=\'a\''`,
			expect: []string{"sql", "select * from tt where A='a'"},
		},
		{
			input:  `sql select * from tt where A='a'`,
			expect: []string{"sql", "select * from tt where A='a'"},
		},
		{
			input:  `sql --format xyz --heading -- select * from example`,
			expect: []string{"sql", "--format", "xyz", "--heading", "select * from example"},
		},
		{
			input:  `explain --full select * from example`,
			expect: []string{"explain", "--full", "select * from example"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := api.ParseCommandLine(tc.input)
			require.Equal(t, tc.expect, result)
		})
	}
}

func tcCommands(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		expect     []string
		expectErr  string
		expectFunc func(t *testing.T, actual string)
	}{
		{
			name:      "wrong-command",
			input:     "cmd_not_exist",
			expectErr: `unknown command "cmd_not_exist" for "machbase-neo"`,
		},
		{
			name:  "show_tables",
			input: "show tables",
			expect: []string{
				"ROWNUM,DATABASE,USER,NAME,ID,TYPE,FLAG",
				"1,MACHBASEDB,SYS,LOG_DATA,13,Log,",
				"2,MACHBASEDB,SYS,TAG_DATA,6,Tag,",
				"3,MACHBASEDB,SYS,TAG_SIMPLE,12,Tag,",
			},
		},
		{
			name:  "show_tables_all",
			input: "show tables --all",
			expect: []string{
				"ROWNUM,DATABASE,USER,NAME,ID,TYPE,FLAG",
				"1,MACHBASEDB,SYS,LOG_DATA,13,Log,",
				"2,MACHBASEDB,SYS,TAG_DATA,6,Tag,",
				"3,MACHBASEDB,SYS,TAG_SIMPLE,12,Tag,",
				"4,MACHBASEDB,SYS,_TAG_DATA_DATA_0,1,KeyValue,Data",
				"5,MACHBASEDB,SYS,_TAG_DATA_META,2,Lookup,Meta",
				"6,MACHBASEDB,SYS,_TAG_SIMPLE_DATA_0,7,KeyValue,Data",
				"7,MACHBASEDB,SYS,_TAG_SIMPLE_META,8,Lookup,Meta",
			},
		},
		{
			name:  "show_table_log_data",
			input: "show table log_data",
			expect: []string{
				"TIME datetime 31  ",
				"SHORT_VALUE short 6  ",
				"USHORT_VALUE ushort 5  ",
				"INT_VALUE integer 11  ",
				"UINT_VALUE uinteger 10  ",
				"LONG_VALUE long 20  ",
				"ULONG_VALUE ulong 20  ",
				"DOUBLE_VALUE double 17  ",
				"FLOAT_VALUE float 17  ",
				"STR_VALUE varchar 400  ",
				"JSON_VALUE json 32767  ",
				"IPV4_VALUE ipv4 15  ",
				"IPV6_VALUE ipv6 45  ",
				"TEXT_VALUE text 67108864  ",
				"BIN_VALUE binary 67108864  "},
		},
		{
			name:  "show_table_log_data_all",
			input: "show table -a log_data",
			expect: []string{
				"_ARRIVAL_TIME datetime 31  ",
				"TIME datetime 31  ",
				"SHORT_VALUE short 6  ",
				"USHORT_VALUE ushort 5  ",
				"INT_VALUE integer 11  ",
				"UINT_VALUE uinteger 10  ",
				"LONG_VALUE long 20  ",
				"ULONG_VALUE ulong 20  ",
				"DOUBLE_VALUE double 17  ",
				"FLOAT_VALUE float 17  ",
				"STR_VALUE varchar 400  ",
				"JSON_VALUE json 32767  ",
				"IPV4_VALUE ipv4 15  ",
				"IPV6_VALUE ipv6 45  ",
				"TEXT_VALUE text 67108864  ",
				"BIN_VALUE binary 67108864  ",
				"_RID long 20  "},
		},
		{
			name:  "desc_table_tag_data_all",
			input: "desc -a tag_data",
			expect: []string{
				"NAME varchar 100 tag name ",
				"TIME datetime 31 basetime ",
				"VALUE double 17 summarized ",
				"SHORT_VALUE short 6  ",
				"USHORT_VALUE ushort 5  ",
				"INT_VALUE integer 11  ",
				"UINT_VALUE uinteger 10  ",
				"LONG_VALUE long 20  ",
				"ULONG_VALUE ulong 20  ",
				"STR_VALUE varchar 400  ",
				"JSON_VALUE json 32767  ",
				"IPV4_VALUE ipv4 15  ",
				"IPV6_VALUE ipv6 45  ",
				"_RID long 20  "},
		},
		{
			name:  "show_indexes",
			input: `show indexes`,
			expect: []string{
				"ROWNUM,ID,DATABASE,USER,TABLE_NAME,COLUMN_NAME,INDEX_NAME,INDEX_TYPE,KEY_COMPRESS,MAX_LEVEL,PART_VALUE_COUNT,BITMAP_ENCODE",
				"1,3,MACHBASEDB,SYS,_TAG_DATA_META,_ID,__PK_IDX__TAG_DATA_META_1,REDBLACK,UNCOMPRESS,0,100000,EQUAL",
				"2,4,MACHBASEDB,SYS,_TAG_DATA_META,NAME,_TAG_DATA_META_NAME,REDBLACK,UNCOMPRESS,0,100000,EQUAL",
				"3,9,MACHBASEDB,SYS,_TAG_SIMPLE_META,_ID,__PK_IDX__TAG_SIMPLE_META_1,REDBLACK,UNCOMPRESS,0,100000,EQUAL",
				"4,10,MACHBASEDB,SYS,_TAG_SIMPLE_META,NAME,_TAG_SIMPLE_META_NAME,REDBLACK,UNCOMPRESS,0,100000,EQUAL",
			},
		},
		{
			name:   "show_lsm",
			input:  "show lsm",
			expect: []string{},
		},
		{
			name:  "show_tags_tag_data",
			input: "show tags tag_data",
			expectFunc: func(t *testing.T, actual string) {
				lines := strings.Split(actual, "\n")
				require.Greater(t, len(lines), 100)
			},
		},
		{
			name:  "show_indexgap",
			input: "show indexgap",
			expectFunc: func(t *testing.T, actual string) {
				lines := strings.Split(actual, "\n")
				require.Equal(t, lines[0], "")
				require.GreaterOrEqual(t, len(lines), 1)
			},
		},
		{
			name:  "show_tagindexgap",
			input: "show tagindexgap",
			expectFunc: func(t *testing.T, actual string) {
				lines := strings.Split(actual, "\n")
				require.Equal(t, lines[0], "ROWNUM,ID,STATUS,DISK_GAP,MEMORY_GAP")
				require.GreaterOrEqual(t, len(lines), 1)
			},
		},
		{
			name:  "show_rollupgap",
			input: "show rollupgap",
			expectFunc: func(t *testing.T, actual string) {
				lines := strings.Split(actual, "\n")
				require.Equal(t, lines[0], "")
				require.GreaterOrEqual(t, len(lines), 1)
			},
		},
		{
			name:  "show_sessions",
			input: "show sessions",
			expectFunc: func(t *testing.T, actual string) {
				lines := strings.Split(actual, "\n")
				require.Greater(t, len(lines), 1)
			},
		},
		{
			name:  "show_statements",
			input: "show statements",
			expectFunc: func(t *testing.T, actual string) {
				lines := strings.Split(actual, "\n")
				require.Greater(t, len(lines), 1)
			},
		},
		{
			name:  "show_storage",
			input: "show storage",
			expectFunc: func(t *testing.T, actual string) {
				lines := strings.Split(actual, "\n")
				require.Greater(t, len(lines), 1)
			},
		},
		{
			name:  "show_table_usage",
			input: "show table-usage",
			expectFunc: func(t *testing.T, actual string) {
				lines := strings.Split(actual, "\n")
				require.Greater(t, len(lines), 1)
			},
		},
		{
			name:  "show_license",
			input: "show license",
			expectFunc: func(t *testing.T, actual string) {
				require.Contains(t, actual, "LICENSE")
			},
		},
		{
			name:   "explain-select-all",
			input:  `explain -- select * from log_data`,
			expect: []string{" PROJECT", "  FULL SCAN (LOG_DATA)", ""},
		},
		{
			name:  "explain-full-select-all",
			input: `explain -- full select * from tag_data`,
			expectFunc: func(t *testing.T, actual string) {
				require.Greater(t, len(actual), 5000, actual)
				require.Contains(t, actual, "EXECUTE")
			},
		},
		{
			name:  "sql-select",
			input: `sql -- select * from tag_data limit 0`,
			expect: []string{
				"NAME,TIME,VALUE,SHORT_VALUE,USHORT_VALUE,INT_VALUE,UINT_VALUE,LONG_VALUE,ULONG_VALUE,STR_VALUE,JSON_VALUE,IPV4_VALUE,IPV6_VALUE",
				"no rows fetched.",
			},
		},
	}

	h := &api.CommandHandler{
		Database: func(ctx context.Context) (api.Conn, error) {
			return testServer.DatabaseSVR().Connect(ctx, api.WithPassword("sys", "manager"))
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	output := &bytes.Buffer{}
	h.ShowTables = printShowResult[*api.TableInfo](t, output)
	h.ShowIndexes = printShowResult[*api.IndexInfo](t, output)
	h.ShowIndex = showIndex(t, output)
	h.ShowLsmIndexes = printShowResult[*api.LsmIndexInfo](t, output)
	h.DescribeTable = descTable(t, output)
	h.ShowTags = showTags(t, output)
	h.ShowIndexGap = printShowResult[*api.IndexGapInfo](t, output)
	h.ShowTagIndexGap = printShowResult[*api.IndexGapInfo](t, output)
	h.ShowRollupGap = printShowResult[*api.RollupGapInfo](t, output)
	h.ShowSessions = printShowResult[*api.SessionInfo](t, output)
	h.ShowStatements = printShowResult[*api.StatementInfo](t, output)
	h.ShowStorage = printShowResult[*api.StorageInfo](t, output)
	h.ShowTableUsage = printShowResult[*api.TableUsageInfo](t, output)
	h.ShowLicense = showLicense(t, output)
	h.Explain = explain(t, output)
	h.SqlQuery = sqlQuery(t, output)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer output.Reset()
			err := h.Exec(context.TODO(), api.ParseCommandLine(tt.input))
			if err != nil {
				if tt.expectErr != "" {
					require.Contains(t, err.Error(), tt.expectErr)
					return
				} else {
					t.Errorf("cmd.Execute() error = %v", err)
					return
				}
			}
			if tt.expectFunc != nil {
				tt.expectFunc(t, output.String())
			} else {
				actual := strings.Split(output.String(), "\n")
				if actual[len(actual)-1] == "" {
					// remove last empty line that comes from split by '\n'
					actual = actual[:len(actual)-1]
				}
				require.Equal(t, tt.expect, actual)
			}
		})
	}
}

func printShowResult[T api.InfoType](t *testing.T, output io.Writer) func(nfo T, nrow int64) bool {
	return func(nfo T, nrow int64) bool {
		if nfo.Err() != nil {
			if nfo.Err() != io.EOF {
				t.Fatal(nfo.Err())
			}
			return false
		}
		if nrow == 1 {
			columns := append([]string{"ROWNUM"}, nfo.Columns().Names()...)
			fmt.Fprintln(output, strings.Join(columns, ","))
		}
		rec := []string{fmt.Sprint(nrow)}
		for _, v := range nfo.Values() {
			rec = append(rec, fmt.Sprint(v))
		}
		fmt.Fprintln(output, strings.Join(rec, ","))
		return true
	}
}

func showIndex(t *testing.T, output io.Writer) func(nfo *api.IndexInfo) bool {
	return func(nfo *api.IndexInfo) bool {
		if nfo.Err() != nil {
			if nfo.Err() != io.EOF {
				t.Fatal(nfo.Err())
			}
			return false
		}
		fmt.Fprintln(output, nfo.TableName, nfo.ColumnName, nfo.IndexName, nfo.IndexType)
		return true
	}
}

func showLicense(_ *testing.T, output io.Writer) func(ti *api.LicenseInfo) bool {
	return func(ti *api.LicenseInfo) bool {
		fmt.Fprintln(output, ti.Values()...)
		return true
	}
}

func descTable(_ *testing.T, output io.Writer) func(desc *api.TableDescription) {
	return func(desc *api.TableDescription) {
		for _, col := range desc.Columns {
			indexes := []string{}
			for _, idxDesc := range desc.Indexes {
				for _, colName := range idxDesc.Cols {
					if colName == col.Name {
						indexes = append(indexes, idxDesc.Name)
						break
					}
				}
			}
			fmt.Fprintln(output, col.Name, col.Type, col.Width(), col.Flag, strings.Join(indexes, ","))
		}
	}
}

func showTags(t *testing.T, output io.Writer) func(ti *api.TagInfo, nrow int64) bool {
	return func(ti *api.TagInfo, nrow int64) bool {
		if ti.Err != nil {
			if ti.Err != io.EOF {
				t.Fatal(ti.Err)
			}
			return false
		}
		if ti.Stat != nil {
			fmt.Fprintln(output, nrow, ti.Id, ti.Name, ti.Database, ti.User, ti.Table, ti.Stat.RowCount)
		} else {
			fmt.Fprintln(output, nrow, ti.Id, ti.Name, ti.Database, ti.User, ti.Table, "NULL")
		}
		return true
	}
}

func explain(t *testing.T, output io.Writer) func(plan string, err error) {
	return func(plan string, err error) {
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintln(output, plan)
	}
}

func sqlQuery(t *testing.T, output io.Writer) func(q *api.Query, nrow int64) bool {
	return func(q *api.Query, nrow int64) bool {
		if nrow == 0 {
			columns := q.Columns()
			line := []string{}
			for _, c := range columns {
				line = append(line, c.Name)
			}
			fmt.Fprintln(output, strings.Join(line, ","))
		} else if nrow > 0 {
			columns := q.Columns()
			buffer, err := columns.MakeBuffer()
			if err != nil {
				t.Fatal(err)
			}
			q.Scan(buffer...)
			line := []string{}
			for _, c := range buffer {
				line = append(line, fmt.Sprintf("%v", c))
			}
			fmt.Fprintln(output, nrow, strings.Join(line, ","))
		} else {
			fmt.Fprintln(output, q.UserMessage())
		}
		return true
	}
}
