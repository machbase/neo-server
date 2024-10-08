package mqttd

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/service/msg"
)

func TestQuery(t *testing.T) {
	expectRows := 1
	connMock := &ConnMock{
		CloseFunc: func() error { return nil },
		QueryFunc: func(ctx context.Context, sqlText string, params ...any) (api.Rows, error) {
			rows := &RowsMock{}
			switch sqlText {
			case "select * from example":
				rows.ScanFunc = func(cols ...any) error {
					cols[0] = new(string)
					*(cols[0].(*string)) = "temp"
					*(cols[1].(*time.Time)) = testTimeTick
					*(cols[2].(*float64)) = 3.14
					return nil
				}
				rows.ColumnsFunc = func() ([]string, []string, error) {
					return []string{
							"name", "time", "value",
						}, []string{
							api.ColumnTypeString(api.VarcharColumnType),
							api.ColumnTypeString(api.DatetimeColumnType),
							api.ColumnTypeString(api.Float64ColumnType),
						}, nil
				}
				rows.IsFetchableFunc = func() bool { return true }
				rows.NextFunc = func() bool {
					expectRows--
					return expectRows >= 0
				}
				rows.CloseFunc = func() error { return nil }
				rows.MessageFunc = func() string {
					return "a row selected"
				}
			default:
				t.Log("=========> unknown mock db SQL:", sqlText)
				t.Fail()
			}
			return rows, nil
		},
	}

	tests := []TestCase{
		{
			Name:      "db/query simple",
			ConnMock:  connMock,
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example" }`),
			Subscribe: "db/reply",
			Expect: &msg.QueryResponse{
				Success: true,
				Reason:  "success",
				Data: &msg.QueryData{
					Columns: []string{"name", "time", "value"},
					Types:   []string{"varchar", "datetime", "double"},
					Rows: [][]any{
						{"temp", testTimeTick.UnixNano(), 3.14},
					},
				},
			},
		},
		{
			Name:      "db/query simple timeformat",
			ConnMock:  connMock,
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example", "format":"json", "tz":"UTC", "timeformat": "DEFAULT" }`),
			Subscribe: "db/reply",
			Expect: &msg.QueryResponse{
				Success: true,
				Reason:  "success",
				Data: &msg.QueryData{
					Columns: []string{"name", "time", "value"},
					Types:   []string{"varchar", "datetime", "double"},
					Rows: [][]any{
						{"temp", "2024-01-15 04:10:59", 3.14},
					},
				},
			},
		},
		{
			Name:      "db/query json timeformat rowsFlatten",
			ConnMock:  connMock,
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example", "format":"json", "tz":"UTC", "timeformat": "DEFAULT", "rowsFlatten": true }`),
			Subscribe: "db/reply",
			Expect:    `/r/{"data":{"columns":\["name","time","value"\],"types":\["varchar","datetime","double"\],"rows":\["temp","2024-01-15 04:10:59",3.14\]},"success":true,"reason":"success","elapse":".*"}`,
		},
		{
			Name:      "db/query json transpose",
			ConnMock:  connMock,
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example", "format":"json", "transpose": true }`),
			Subscribe: "db/reply",
			Expect:    `/r/{"data":{"columns":\["name","time","value"\],"types":\["varchar","datetime","double"\],"cols":\[\["temp"\],\[1705291859000000000\],\[3.14\]\]},"success":true,"reason":"success","elapse":".+"}`,
		},
		{
			Name:      "db/query json timeformat rowsArray",
			ConnMock:  connMock,
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example", "format":"json", "tz":"UTC", "timeformat": "DEFAULT", "rowsArray": true }`),
			Subscribe: "db/reply",
			Expect:    `/r/{"data":{"columns":\["name","time","value"\],"types":\["varchar","datetime","double"\],"rows":\[{"name":"temp","time":"2024-01-15 04:10:59","value":3.14}\]},"success":true,"reason":"success","elapse":".+"}`,
		},
		{
			Name:      "db/query simple, format=csv, reply",
			ConnMock:  connMock,
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example", "format": "csv", "reply":"db/reply/123" }`),
			Subscribe: "db/reply/123",
			Expect:    "name,time,value\ntemp,1705291859000000000,3.14\n\n",
		},
		{
			Name:      "db/query simple, format=csv",
			ConnMock:  connMock,
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example", "format": "csv" }`),
			Subscribe: "db/reply",
			Expect:    "name,time,value\ntemp,1705291859000000000,3.14\n\n",
		},
		{
			Name:      "db/query simple, format=csv, compress",
			ConnMock:  connMock,
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example", "format": "csv", "compress":"gzip" }`),
			Subscribe: "db/reply",
			Expect:    compress([]byte("name,time,value\ntemp,1705291859000000000,3.14\n\n")),
		},
		{
			Name:      "db/query simple, format=csv, timeformat",
			ConnMock:  connMock,
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example", "format": "csv", "tz": "UTC", "timeformat": "DEFAULT" }`),
			Subscribe: "db/reply",
			Expect:    "name,time,value\ntemp,2024-01-15 04:10:59,3.14\n\n",
		},
	}

	for _, tt := range tests {
		expectRows = 1
		runTest(t, &tt)
	}
}

func TestWrite(t *testing.T) {
	var count int
	var values = []float64{1.2345, 2.3456}

	connMock := &ConnMock{
		CloseFunc: func() error { return nil },
		ExecFunc: func(ctx context.Context, sqlText string, params ...any) api.Result {
			rt := &ResultMock{}
			switch sqlText {
			case "INSERT INTO EXAMPLE(NAME,TIME,VALUE) VALUES(?,?,?)":
				if len(params) == 3 && strings.HasPrefix(params[0].(string), "mycar") && params[2] == values[count] {
					rt.ErrFunc = func() error { return nil }
					rt.RowsAffectedFunc = func() int64 { return 1 }
					rt.MessageFunc = func() string { return "a row inserted" }
					count++
				} else {
					t.Log("ExecFunc => unexpected insert params:", params)
					t.Fatal(sqlText)
				}
			default:
				t.Log("ExecFunc => unknown mock db SQL:", sqlText)
				t.Fail()
			}
			return rt
		},
		QueryRowFunc: func(ctx context.Context, sqlText string, params ...any) api.Row {
			if sqlText == "select count(*) from M$SYS_TABLES T, M$SYS_USERS U where U.NAME = ? and U.USER_ID = T.USER_ID AND T.NAME = ?" && params[1] == "EXAMPLE" {
				return &RowMock{
					ErrFunc: func() error { return nil },
					ScanFunc: func(cols ...any) error {
						*(cols[0].(*int)) = 1
						return nil
					},
				}
			} else if len(params) == 3 && params[0] == "SYS" && params[1] == -1 && params[2] == "EXAMPLE" {
				return &RowMock{
					ErrFunc: func() error { return nil },
					ScanFunc: func(cols ...any) error {
						*(cols[0].(*int)) = 0                     // TABLE_ID
						*(cols[1].(*int)) = int(api.TagTableType) // TABLE_TYPE
						*(cols[3].(*int)) = 3                     // TABLE_COLCOUNT
						return nil
					},
				}
			} else {
				fmt.Println("QueryRowFunc ->", sqlText, params)
				t.Fail()
			}
			return nil
		},
		QueryFunc: func(ctx context.Context, sqlText string, params ...any) (api.Rows, error) {
			if sqlText == "select name, type, length, id, flag from M$SYS_COLUMNS where table_id = ? AND database_id = ? order by id" {
				return NewRowsWrap([]*api.Column{
					{Name: "NAME", Type: "string"},
					{Name: "TYPE", Type: "int"},
					{Name: "LENGTH", Type: "int"},
					{Name: "ID", Type: "int"},
					{Name: "FLAG", Type: "int"},
				},
					[][]any{
						{"NAME", api.VarcharColumnType, 0, 0, 0},
						{"TIME", api.DatetimeColumnType, 0, 1, 0},
						{"VALUE", api.Float64ColumnType, 0, 2, 0},
					}), nil
			} else if sqlText == "select name, type, id from M$SYS_INDEXES where table_id = ? AND database_id = ?" {
				return NewRowsWrap(
					[]*api.Column{
						{Name: "NAME", Type: "string"},
						{Name: "TYPE", Type: "int"},
						{Name: "ID", Type: "int"},
					},
					[][]any{
						{"NAME", 8, 0},
						{"TYPE", 1, 1},
						{"ID", 1, 2},
					}), nil
			} else if sqlText == "select name from M$SYS_INDEX_COLUMNS where index_id = ? AND database_id = ? order by col_id" {
				return NewRowsWrap(
					[]*api.Column{{Name: "NAME", Type: "string"}},
					[][]any{},
				), nil
			} else {
				fmt.Println("QueryFunc ->", sqlText)
				t.Fail()
			}
			return nil, nil
		},
	}

	jsonData := []byte(`[["mycar", 1705291859000000000, 1.2345], ["mycar", 1705291860000000000, 2.3456]]`)
	csvData := []byte("mycar,1705291859000000000,1.2345\nmycar,1705291860000000000,2.3456")
	ilpData := []byte("mycar speed=1.2345 167038034500000\nmycar speed=2.3456 167038034500000\n")
	jsonGzipData := compress(jsonData)
	csvGzipData := compress(csvData)

	tests := []TestCase{
		{
			Name:     "db/write/example json",
			ConnMock: connMock,
			Topic:    "db/write/example",
			Payload:  jsonData,
		},
		{
			Name:     "db/write/example csv",
			ConnMock: connMock,
			Topic:    "db/write/example:csv",
			Payload:  csvData,
		},
		{
			Name:     "db/write/example json gzip",
			ConnMock: connMock,
			Topic:    "db/write/example:json:gzip",
			Payload:  jsonGzipData,
		},
		{
			Name:     "db/write/example csv gzip",
			ConnMock: connMock,
			Topic:    "db/write/example:csv:gzip",
			Payload:  csvGzipData,
		},
		{
			Name:     "metrics/example ILP",
			ConnMock: connMock,
			Topic:    "metrics/example",
			Payload:  ilpData,
		},
	}

	for _, tt := range tests {
		count = 0
		runTest(t, &tt)
		if count != 2 {
			t.Logf("Test %q count should be 2, got %d", tt.Name, count)
			t.Fail()
		}
	}
}

func NewRowsWrap(columns api.Columns, values [][]any) *RowsMockWrap {
	ret := &RowsMockWrap{columns: columns, values: values}
	rows := &RowsMock{}
	rows.NextFunc = ret.Next
	rows.CloseFunc = ret.Close
	rows.ColumnsFunc = ret.Columns
	rows.ScanFunc = ret.Scan
	ret.RowsMock = rows
	ret.cursor = -1
	return ret
}

type RowsMockWrap struct {
	*RowsMock
	columns api.Columns
	values  [][]any
	cursor  int
}

func (rw *RowsMockWrap) Close() error {
	return nil
}

func (rw *RowsMockWrap) Columns() ([]string, []string, error) {
	names := make([]string, len(rw.columns))
	types := make([]string, len(rw.columns))
	for i, c := range rw.columns {
		names[i] = c.Name
		types[i] = c.Type
	}
	return names, types, nil
}

func (rw *RowsMockWrap) Next() bool {
	rw.cursor++
	return rw.cursor < len(rw.values)
}

func (rw *RowsMockWrap) Scan(cols ...any) error {
	for i := range cols {
		switch v := cols[i].(type) {
		case *string:
			*v = rw.values[rw.cursor][i].(string)
		case *int:
			*v = rw.values[rw.cursor][i].(int)
		case *uint64:
			*v = uint64(rw.values[rw.cursor][i].(int))
		default:
			fmt.Printf("ERR RowsMockWrap.Scan() %T\n", v)
		}
	}
	return nil
}

func TestAppend(t *testing.T) {
	count := 0
	connMock := &ConnMock{
		CloseFunc: func() error { return nil },
		AppenderFunc: func(ctx context.Context, tableName string, opts ...api.AppenderOption) (api.Appender, error) {
			app := &AppenderMock{}
			app.CloseFunc = func() (int64, int64, error) { return int64(count), 0, nil }
			app.AppendFunc = func(values ...any) error {
				if len(values) == 3 && values[0] == "mycar" {
					count++
				} else {
					t.Log("=========> invalid append:", values)
					t.Fail()
				}
				return nil
			}
			app.ColumnsFunc = func() ([]string, []string, error) {
				return []string{
						"name", "time", "value",
					}, []string{
						api.ColumnTypeString(api.VarcharColumnType),
						api.ColumnTypeString(api.DatetimeColumnType),
						api.ColumnTypeString(api.Float64ColumnType),
					}, nil
			}
			return app, nil
		},
	}

	jsonData := []byte(`[["mycar", 1705291859000000000, 1.2345], ["mycar", 1705291860000000000, 2.3456]]`)
	csvData := []byte("mycar,1705291859000000000,1.2345\nmycar,1705291860000000000,2.3456")
	jsonGzipData := compress(jsonData)
	csvGzipData := compress(csvData)
	tests := []TestCase{
		{
			Name:     "db/append/example",
			ConnMock: connMock,
			Topic:    "db/append/example",
			Payload:  jsonData,
		},
		{
			Name:     "db/append/example json",
			ConnMock: connMock,
			Topic:    "db/append/example:json",
			Payload:  jsonData,
		},
		{
			Name:     "db/append/example json gzip",
			ConnMock: connMock,
			Topic:    "db/append/example:json:gzip",
			Payload:  jsonGzipData,
		},
		{
			Name:     "db/append/example csv",
			ConnMock: connMock,
			Topic:    "db/append/example:csv",
			Payload:  csvData,
		},
		{
			Name:     "db/append/example csv gzip",
			ConnMock: connMock,
			Topic:    "db/append/example:csv: gzip",
			Payload:  csvGzipData,
		},
	}

	for _, tt := range tests {
		count = 0
		runTest(t, &tt)
		if count != 2 {
			t.Logf("Test %q expect 2 rows, got %d", tt.Name, count)
			t.Fail()
		}
	}
}

func compress(data []byte) []byte {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)

	_, err := zw.Write(data)
	if err != nil {
		panic(err)
	}

	zw.Close()

	return buf.Bytes()
}

func TestTql(t *testing.T) {
	var count = 0
	connMock := &ConnMock{
		CloseFunc: func() error { return nil },
		AppenderFunc: func(ctx context.Context, tableName string, opts ...api.AppenderOption) (api.Appender, error) {
			app := &AppenderMock{}
			app.CloseFunc = func() (int64, int64, error) { return int64(count), 0, nil }
			app.AppendFunc = func(values ...any) error {
				if len(values) == 3 && values[0] == "mycar" {
					count++
				} else {
					t.Log("=========> invalid append:", values)
					t.Fail()
				}
				return nil
			}
			app.ColumnsFunc = func() ([]string, []string, error) {
				return []string{
						"name", "time", "value",
					}, []string{
						api.ColumnTypeString(api.VarcharColumnType),
						api.ColumnTypeString(api.DatetimeColumnType),
						api.ColumnTypeString(api.Float64ColumnType),
					}, nil
			}
			return app, nil
		},
	}

	csvData := []byte("mycar,1705291859000000000,1.2345\nmycar,1705291860000000000,2.3456")

	tests := []TestCase{
		{
			Name:     "db/tql/csv_append.tql",
			ConnMock: connMock,
			Topic:    "db/tql/csv_append.tql",
			Payload:  csvData,
		},
	}
	for _, tt := range tests {
		count = 0
		runTest(t, &tt)
		if count != 2 {
			t.Logf("Test %q expect 2 rows, got %d", tt.Name, count)
			t.Fail()
		}
	}
}
