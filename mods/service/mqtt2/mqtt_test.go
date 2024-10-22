package mqtt2

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/url"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/api/types"
	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/stretchr/testify/require"
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
				rows.ColumnsFunc = func() ([]string, []types.DataType, error) {
					return []string{
							"name", "time", "value",
						}, []types.DataType{
							types.ColumnTypeVarchar.DataType(),
							types.ColumnTypeDatetime.DataType(),
							types.ColumnTypeDouble.DataType(),
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
					Types:   []types.DataType{types.DataTypeString, types.DataTypeDatetime, types.DataTypeFloat64},
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
					Types:   []types.DataType{types.DataTypeString, types.DataTypeDatetime, types.DataTypeFloat64},
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
			Expect:    `/r/{"data":{"columns":\["name","time","value"\],"types":\["string","datetime","double"\],"rows":\["temp","2024-01-15 04:10:59",3.14\]},"success":true,"reason":"success","elapse":".*"}`,
		},
		{
			Name:      "db/query json transpose",
			ConnMock:  connMock,
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example", "format":"json", "transpose": true }`),
			Subscribe: "db/reply",
			Expect:    `/r/{"data":{"columns":\["name","time","value"\],"types":\["string","datetime","double"\],"cols":\[\["temp"\],\[1705291859000000000\],\[3.14\]\]},"success":true,"reason":"success","elapse":".+"}`,
		},
		{
			Name:      "db/query json timeformat rowsArray",
			ConnMock:  connMock,
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example", "format":"json", "tz":"UTC", "timeformat": "DEFAULT", "rowsArray": true }`),
			Subscribe: "db/reply",
			Expect:    `/r/{"data":{"columns":\["name","time","value"\],"types":\["string","datetime","double"\],"rows":\[{"name":"temp","time":"2024-01-15 04:10:59","value":3.14}\]},"success":true,"reason":"success","elapse":".+"}`,
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

	for _, ver := range []uint{4, 5} {
		for _, tt := range tests {
			expectRows = 1
			tt.Ver = ver
			runTest(t, &tt)
		}
	}
}

func TestWriteResponse(t *testing.T) {
	tm := &TestMock{t: t}
	mqttServer.db = tm.NewDB()

	brokerUrl, err := url.Parse("tcp://" + brokerAddr)
	require.NoError(t, err)

	ctx := context.Background()

	cfg := autopaho.ClientConfig{
		ServerUrls:                    []*url.URL{brokerUrl},
		KeepAlive:                     20,
		CleanStartOnInitialConnection: true,
	}

	readyWg := sync.WaitGroup{}

	cfg.OnConnectionUp = func(cm *autopaho.ConnectionManager, connAck *paho.Connack) {
		defer readyWg.Done()

		t.Log("CONN", connAck.ReasonCode)
		if connAck.ReasonCode != 0 {
			t.Fail()
			return
		}
		subAck, err := cm.Subscribe(ctx, &paho.Subscribe{
			Subscriptions: []paho.SubscribeOptions{
				{Topic: "db/reply/#", QoS: 1},
			},
		})
		if err != nil {
			t.Log("ERROR", "SUB", err.Error())
			t.Fail()
		}
		t.Log("SUB:", subAck.Reasons)
	}
	cfg.OnConnectError = func(err error) {
		t.Log("ERROR", "OnConnect", err.Error())
	}
	cfg.ClientConfig.ClientID = "mqtt2-test"
	cfg.ClientConfig.OnPublishReceived = append(cfg.ClientConfig.OnPublishReceived,
		func(r paho.PublishReceived) (bool, error) {
			t.Log("PUB:", r.Packet.Topic, string(r.Packet.Payload))
			readyWg.Done()
			return true, nil
		})
	cfg.ClientConfig.OnClientError = func(err error) {
		t.Log("ERROR", "OnClient", err.Error())
	}
	cfg.ClientConfig.OnServerDisconnect = func(d *paho.Disconnect) {
		t.Log("ServerDisconnect", d.ReasonCode)
	}

	readyWg.Add(1)
	c, err := autopaho.NewConnection(ctx, cfg)
	require.NoError(t, err)
	defer c.Disconnect(ctx)
	readyWg.Wait()

	readyWg.Add(1)
	props := &paho.PublishProperties{}
	// props.ResponseTopic = "db/reply/123"
	props.User.Add("method", "insert")
	props.User.Add("format", "csv")
	props.User.Add("reply", "db/reply/123")
	c.Publish(ctx, &paho.Publish{
		Topic:      "db/write/example",
		Payload:    []byte(`mycar,1705291859000000000,1.2345`),
		QoS:        2,
		Properties: props,
	})
	readyWg.Wait()
}

func TestWrite(t *testing.T) {
	tm := &TestMock{t: t, values: []float64{1.2345, 2.3456}}
	mqttServer.db = tm.NewDB()

	tests := []struct {
		Vers        []uint
		TC          TestCase
		ExpectCount int
	}{
		{
			TC: TestCase{
				Name:    "db/write/example json",
				Topic:   "db/write/example",
				Payload: []byte(`[["mycar", 1705291859000000000, 1.2345], ["mycar", 1705291860000000000, 2.3456]]`),
			},
			ExpectCount: 2,
		},
		{
			TC: TestCase{
				Name:    "db/write/example json columns",
				Topic:   "db/write/example",
				Payload: []byte(`{"data":{"columns":["NAME","TIME","VALUE"],"rows":[["mycar", 1705291859000000000, 1.2345], ["mycar", 1705291860000000000, 2.3456]]}}}`),
			},
			ExpectCount: 2,
		},
		{
			TC: TestCase{
				Name:  "db/write/example ndjson",
				Topic: "db/write/example",
				Payload: []byte(`{"NAME":"mycar", "TIME":1705291859, "VALUE":1.2345}` + "\n" +
					`{"NAME":"mycar", "TIME":1705291860, "VALUE":2.3456}` + "\n"),
				Properties: map[string]string{"format": "ndjson", "timeformat": "s"},
			},
			ExpectCount: 2,
		},
		{
			TC: TestCase{
				Name:    "db/write/example csv",
				Topic:   "db/write/example:csv",
				Payload: []byte("mycar,1705291859000000000,1.2345\nmycar,1705291860000000000,2.3456"),
			},
			ExpectCount: 2,
		},
		{
			TC: TestCase{
				Name:       "db/write/example csv v5",
				Topic:      "db/write/example",
				Properties: map[string]string{"format": "csv", "timeformat": "s"},
				Payload:    []byte("mycar,1705291859,1.2345\nmycar,170529186,2.3456"),
			},
			ExpectCount: 2,
			Vers:        []uint{5},
		},
		{
			TC: TestCase{
				Name:       "db/write/example csv v5-time-value",
				Topic:      "db/write/example",
				Properties: map[string]string{"format": "csv", "timeformat": "s", "header": "columns"},
				Payload:    []byte("TIME,VALUE\n1705291859,1.2345\n170529186,2.3456"),
			},
			ExpectCount: 2,
			Vers:        []uint{5},
		},
		{
			TC: TestCase{
				Name:    "db/write/example json gzip",
				Topic:   "db/write/example:json:gzip",
				Payload: compress([]byte(`[["mycar", 1705291859000000000, 1.2345], ["mycar", 1705291860000000000, 2.3456]]`)),
			},
			ExpectCount: 2,
		},
		{
			TC: TestCase{
				Name:    "db/write/example csv gzip",
				Topic:   "db/write/example:csv:gzip",
				Payload: compress([]byte("mycar,1705291859000000000,1.2345\nmycar,1705291860000000000,2.3456")),
			},
			ExpectCount: 2,
		},
		{
			TC: TestCase{
				Name:    "db/metrics/example ILP",
				Topic:   "db/metrics/example",
				Payload: []byte("mycar speed=1.2345 167038034500000\nmycar speed=2.3456 167038034500000\n"),
			},
			ExpectCount: 2,
		},
	}

	for _, tt := range tests {
		vers := tt.Vers
		if len(vers) == 0 {
			vers = []uint{4, 5}
		}
		for _, ver := range vers {
			tm.count = 0
			tt.TC.Ver = ver
			runTest(t, &tt.TC)
			if tm.count != tt.ExpectCount {
				t.Logf("Test %q count should be %d, got %d", tt.TC.Name, tt.ExpectCount, tm.count)
				t.Fail()
			}
		}
	}
}

type TestMock struct {
	values []float64
	count  int
	t      *testing.T
}

func (tm *TestMock) NewDB() *dbMock {
	tm.t.Helper()
	return &dbMock{conn: &ConnMock{
		CloseFunc: func() error { return nil },
		ExecFunc: func(ctx context.Context, sqlText string, params ...any) api.Result {
			defer func() {
				if r := recover(); r != nil {
					tm.t.Log("panic", "onPublished", r)
					debug.PrintStack()
				}
			}()
			rt := &ResultMock{
				ErrFunc: func() error { return nil },
			}
			switch sqlText {
			case "INSERT INTO EXAMPLE(NAME,TIME,VALUE) VALUES(?,?,?)":
				if len(tm.values) == 0 {
					return rt
				}
				if len(params) == 3 && strings.HasPrefix(params[0].(string), "mycar") && params[2] == tm.values[tm.count] {
					rt.ErrFunc = func() error { return nil }
					rt.RowsAffectedFunc = func() int64 { return 1 }
					rt.MessageFunc = func() string { return "a row inserted" }
					tm.count++
				} else {
					tm.t.Log("ExecFunc => unexpected insert params:", params)
					tm.t.Fatal(sqlText)
				}
			case "INSERT INTO EXAMPLE(TIME,VALUE) VALUES(?,?)":
				if len(tm.values) == 0 {
					return rt
				}
				if len(params) == 2 && params[1] == tm.values[tm.count] {
					rt.ErrFunc = func() error { return nil }
					rt.RowsAffectedFunc = func() int64 { return 1 }
					rt.MessageFunc = func() string { return "a row inserted" }
					tm.count++
				} else {
					tm.t.Log("ExecFunc => unexpected insert params:", params)
					tm.t.Fatal(sqlText)
				}
			default:
				tm.t.Log("ExecFunc => unknown mock db SQL:", sqlText)
				tm.t.Fail()
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
						*(cols[0].(*int)) = 0                       // TABLE_ID
						*(cols[1].(*int)) = int(types.TableTypeTag) // TABLE_TYPE
						*(cols[3].(*int)) = 3                       // TABLE_COLCOUNT
						return nil
					},
				}
			} else {
				fmt.Println("QueryRowFunc ->", sqlText, params)
				tm.t.Fail()
			}
			return nil
		},
		QueryFunc: func(ctx context.Context, sqlText string, params ...any) (api.Rows, error) {
			if sqlText == "select name, type, length, id, flag from M$SYS_COLUMNS where table_id = ? AND database_id = ? order by id" {
				return NewRowsWrap([]*types.Column{
					{Name: "NAME", DataType: "string"},
					{Name: "TYPE", DataType: "int"},
					{Name: "LENGTH", DataType: "int"},
					{Name: "ID", DataType: "int"},
					{Name: "FLAG", DataType: "int"},
				},
					[][]any{
						{"NAME", int(types.ColumnTypeVarchar), 0, 0, 0},
						{"TIME", int(types.ColumnTypeDatetime), 0, 1, 0},
						{"VALUE", int(types.ColumnTypeDouble), 0, 2, 0},
					}), nil
			} else if sqlText == "select name, type, id from M$SYS_INDEXES where table_id = ? AND database_id = ?" {
				return NewRowsWrap(
					[]*types.Column{
						{Name: "NAME", DataType: "string"},
						{Name: "TYPE", DataType: "int"},
						{Name: "ID", DataType: "int"},
					},
					[][]any{
						{"NAME", 8, 0},
						{"TYPE", 1, 1},
						{"ID", 1, 2},
					}), nil
			} else if sqlText == "select name from M$SYS_INDEX_COLUMNS where index_id = ? AND database_id = ? order by col_id" {
				return NewRowsWrap(
					[]*types.Column{{Name: "NAME", DataType: "string"}},
					[][]any{},
				), nil
			} else {
				fmt.Println("QueryFunc ->", sqlText)
				tm.t.Fail()
			}
			return nil, nil
		},
	},
	}
}

func NewRowsWrap(columns types.Columns, values [][]any) *RowsMockWrap {
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
	columns types.Columns
	values  [][]any
	cursor  int
}

func (rw *RowsMockWrap) Close() error {
	return nil
}

func (rw *RowsMockWrap) Columns() ([]string, []types.DataType, error) {
	columnNames := make([]string, len(rw.columns))
	columnTypes := make([]types.DataType, len(rw.columns))
	for i, c := range rw.columns {
		columnNames[i] = c.Name
		columnTypes[i] = c.DataType
	}
	return columnNames, columnTypes, nil
}

func (rw *RowsMockWrap) Next() bool {
	rw.cursor++
	return rw.cursor < len(rw.values)
}

func (rw *RowsMockWrap) Scan(cols ...any) error {
	for i := range cols {
		val := rw.values[rw.cursor][i]
		if err := types.Scan(val, cols[i]); err != nil {
			return fmt.Errorf("ERR RowsMockWrap.Scan() %T %s", cols[i], err.Error())
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
			app.ColumnsFunc = func() ([]string, []types.DataType, error) {
				return []string{
						"NAME", "TIME", "VALUE",
					}, []types.DataType{
						types.ColumnTypeVarchar.DataType(),
						types.ColumnTypeDatetime.DataType(),
						types.ColumnTypeDouble.DataType(),
					}, nil
			}
			app.TableNameFunc = func() string {
				return "example"
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
			Ver:      uint(4),
		},
		{
			Name:     "db/append/example",
			ConnMock: connMock,
			Topic:    "db/append/example",
			Payload:  jsonData,
			Ver:      uint(5),
		},
		{
			Name:       "db/write/example?method=append",
			ConnMock:   connMock,
			Topic:      "db/write/example",
			Payload:    jsonData,
			Ver:        uint(5),
			Properties: map[string]string{"method": "append"},
		},
		{
			Name:     "db/append/example json",
			ConnMock: connMock,
			Topic:    "db/append/example:json",
			Payload:  jsonData,
			Ver:      uint(4),
		},
		{
			Name:     "db/append/example json",
			ConnMock: connMock,
			Topic:    "db/append/example:json",
			Payload:  jsonData,
			Ver:      uint(5),
		},
		{
			Name:     "db/append/example json gzip",
			ConnMock: connMock,
			Topic:    "db/append/example:json:gzip",
			Payload:  jsonGzipData,
			Ver:      uint(4),
		},
		{
			Name:     "db/append/example json gzip",
			ConnMock: connMock,
			Topic:    "db/append/example:json:gzip",
			Payload:  jsonGzipData,
			Ver:      uint(5),
		},
		{
			Name:       "db/write/example?method=append&format=json&compress=gzip",
			ConnMock:   connMock,
			Topic:      "db/write/example",
			Payload:    jsonGzipData,
			Ver:        uint(5),
			Properties: map[string]string{"method": "append", "format": "json", "compress": "gzip"},
		},
		{
			Name:     "db/append/example csv",
			ConnMock: connMock,
			Topic:    "db/append/example:csv",
			Payload:  csvData,
			Ver:      uint(4),
		},
		{
			Name:     "db/append/example csv",
			ConnMock: connMock,
			Topic:    "db/append/example:csv",
			Payload:  csvData,
			Ver:      uint(5),
		},
		{
			Name:     "db/append/example csv gzip",
			ConnMock: connMock,
			Topic:    "db/append/example:csv: gzip",
			Payload:  csvGzipData,
			Ver:      uint(4),
		},
		{
			Name:       "db/write/example?format=csv&method=append",
			ConnMock:   connMock,
			Topic:      "db/write/example",
			Payload:    csvData,
			Ver:        uint(5),
			Properties: map[string]string{"method": "append", "format": "csv"},
		},
		{
			Name:     "db/append/example csv gzip",
			ConnMock: connMock,
			Topic:    "db/append/example:csv: gzip",
			Payload:  csvGzipData,
			Ver:      uint(5),
		},
		{
			Name:     "db/write/example?format=ndjson&method=append",
			ConnMock: connMock,
			Topic:    "db/write/example",
			Payload: []byte(
				`{"NAME":"mycar", "TIME":1705291859, "VALUE":1.2345}` + "\n" +
					`{"NAME":"mycar", "TIME":1705291860, "VALUE":2.3456}` + "\n"),
			Ver:        uint(5),
			Properties: map[string]string{"method": "append", "format": "ndjson", "timeformat": "s"},
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
			app.ColumnsFunc = func() ([]string, []types.DataType, error) {
				return []string{
						"name", "time", "value",
					}, []types.DataType{
						types.ColumnTypeVarchar.DataType(),
						types.ColumnTypeDatetime.DataType(),
						types.ColumnTypeDouble.DataType(),
					}, nil
			}
			app.TableNameFunc = func() string {
				return "example"
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
	for _, ver := range []uint{4, 5} {
		for _, tt := range tests {
			count = 0
			tt.Ver = ver
			runTest(t, &tt)
			if count != 2 {
				t.Logf("Test %q expect 2 rows, got %d", tt.Name, count)
				t.Fail()
			}
		}
	}
}
