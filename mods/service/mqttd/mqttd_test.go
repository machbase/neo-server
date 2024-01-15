package mqttd

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/service/msg"
	spi "github.com/machbase/neo-spi"
)

func TestQuery(t *testing.T) {
	expectRows := 1
	connMock := &ConnMock{
		CloseFunc: func() error { return nil },
		QueryFunc: func(ctx context.Context, sqlText string, params ...any) (spi.Rows, error) {
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
				rows.ColumnsFunc = func() (spi.Columns, error) {
					return []*spi.Column{
						{Name: "name", Type: spi.ColumnTypeString(spi.VarcharColumnType)},
						{Name: "time", Type: spi.ColumnTypeString(spi.DatetimeColumnType)},
						{Name: "value", Type: spi.ColumnTypeString(spi.Float64ColumnType)},
					}, nil
				}
				rows.IsFetchableFunc = func() bool { return true }
				rows.NextFunc = func() bool {
					expectRows--
					return expectRows >= 0
				}
				rows.CloseFunc = func() error { return nil }
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
				Reason:  "1 rows selected",
				Data: &msg.QueryData{
					Columns: []string{"name", "time(UTC)", "value"},
					Types:   []string{"varchar", "datetime", "double"},
					Rows: [][]any{
						{"temp", testTimeTick, 3.14},
					},
				},
			},
		},
		{
			Name:      "db/query simple, format=csv",
			ConnMock:  connMock,
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example", "format": "csv" }`),
			Subscribe: "db/reply",
			Expect:    "name,time(UTC),value\ntemp,1705291859000000000,3.14\n",
		},
	}

	for _, tt := range tests {
		expectRows = 1
		runTest(t, &tt)
	}
}

func TestWrite(t *testing.T) {
	connMock := &ConnMock{
		CloseFunc: func() error { return nil },
		ExecFunc: func(ctx context.Context, sqlText string, params ...any) spi.Result {
			rt := &ResultMock{}
			switch sqlText {
			case "insert into example (name,time,value) values(?,?,?)":
				if len(params) == 3 && params[0] == "mycar" && params[2] == 1.2345 {
					rt.ErrFunc = func() error { return nil }
					rt.RowsAffectedFunc = func() int64 { return 1 }
					rt.MessageFunc = func() string { return "a row inserted" }
				} else {
					t.Log("=========> unknown mock db SQL:", params)
				}
			default:
				t.Log("=========> unknown mock db SQL:", sqlText)
				t.Fail()
			}
			return rt
		},
	}

	tests := []TestCase{
		{
			Name:     "db/write/example json single",
			ConnMock: connMock,
			Topic:    "db/write/example",
			Payload:  []byte(`{ "data": { "columns":["name", "time", "value"], "rows":[["mycar", 1705291859, 1.2345]]}}`),
		},
		{
			Name:     "db/write/example csv single",
			ConnMock: connMock,
			Topic:    "db/write/example:csv",
			Payload:  []byte(`mycar,1705291859,1.2345`),
		},
	}

	for _, tt := range tests {
		runTest(t, &tt)
	}
}

func TestAppend(t *testing.T) {
	count := 0
	connMock := &ConnMock{
		CloseFunc: func() error { return nil },
		AppenderFunc: func(ctx context.Context, tableName string, opts ...spi.AppenderOption) (spi.Appender, error) {
			app := &AppenderMock{}
			app.CloseFunc = func() (int64, int64, error) { return int64(count), 0, nil }
			app.AppendFunc = func(values ...any) error {
				fmt.Println("=========> append", tableName, values)
				if len(values) == 3 && values[0] == "mycar" {
					count++
				} else {
					t.Log("=========> invalid append:", values)
					t.Fail()
				}
				return nil
			}
			app.ColumnsFunc = func() (spi.Columns, error) {
				return []*spi.Column{
					{Name: "name", Type: spi.ColumnTypeString(spi.VarcharColumnType)},
					{Name: "time", Type: spi.ColumnTypeString(spi.DatetimeColumnType)},
					{Name: "value", Type: spi.ColumnTypeString(spi.Float64ColumnType)},
				}, nil
			}
			return app, nil
		},
	}

	tests := []TestCase{
		{
			Name:     "db/append/example json",
			ConnMock: connMock,
			Topic:    "db/append/example:json",
			Payload:  []byte(`[["mycar", 1705291859000000000, 1.2345], ["mycar", 1705291860000000000, 2.3456]]`),
		},
		{
			Name:     "db/append/example csv",
			ConnMock: connMock,
			Topic:    "db/append/example:csv",
			Payload:  []byte(`mycar,1705291859000000000,1.2345\nmycar,1705291860000000000,2.3456`),
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
