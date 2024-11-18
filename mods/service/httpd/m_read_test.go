package httpd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/machbase/neo-server/v8/api"
)

func TestRead(t *testing.T) {
	dbMock := &TestClientMock{}
	dbMock.ConnectFunc = func(ctx context.Context, options ...api.ConnectOption) (api.Conn, error) {
		conn := &ConnMock{}
		conn.CloseFunc = func() error { return nil }
		conn.QueryRowFunc = func(ctx context.Context, sqlText string, params ...any) api.Row {
			rm := &RowMock{}

			switch sqlText {
			case "SELECT NAME, TO_TIMESTAMP(DATE_TRUNC('SEC', TIME, 60)/1000000) AS TIME, AVG(VALUE) AS VALUE FROM (SELECT NAME, TIME ROLLUP 1 SEC TIME, AVVALUE) VALUE FROM TAG WHERE NAME IN('LAKE_TEST_RASPBERY001') AND TIME BETWEEN FROM_TIMESTAMP(1704670971000000000) AND FROM_TIMESTAMP(17046871000000000) GROUP BY TIME, NAME) GROUP BY TIME, NAME ORDER BY TIME ASC LIMIT 1000":
				rm.ScanFunc = func(cols ...any) error {
					t.Log("case ok")
					return nil
				}
			default:
				t.Logf("QueryRow sqlText: %s, params:%v", sqlText, params)
			}
			return rm
		}
		conn.QueryFunc = func(ctx context.Context, sqlText string, params ...any) (api.Rows, error) {
			rm := &RowsMock{}
			nextCount := 0

			switch sqlText {
			case "SELECT NAME, TO_CHAR(DATE_TRUNC('SEC', TIME, 1), 'YYYY-MM-DD HH24:MI:SS') AS TIME, AVG(VALUE) AS VALUE FROM (SELECT NAME, TIME ROLLUP 1 SEC TIME, AVG(VALUE) VALUE FROM TAG WHERE NAME IN('LAKE_TEST_RASPBERY001') AND TIME BETWEEN TO_DATE('2024-01-08 09:12:00 000', 'YYYY-MM-DD HH24:MI:SS mmm') AND TO_DATE('2024-01-08 10:12:00 000', 'YYYY-MM-DD HH24:MI:SS mmm') GROUP BY TIME, NAME) GROUP BY TIME, NAME ORDER BY TIME ASC LIMIT 1000":
				rm.ColumnsFunc = func() (api.Columns, error) {
					return api.Columns{
						{Name: "name", DataType: api.ColumnTypeVarchar.DataType()},
						{Name: "time", DataType: api.ColumnTypeDatetime.DataType()},
						{Name: "value", DataType: api.ColumnTypeDouble.DataType()},
					}, nil
				}
				rm.ScanFunc = func(cols ...any) error {
					if len(cols) != 3 {
						return fmt.Errorf("invalid lake read-api, scan length is 3 ['name', 'time', 'value'] (length: %d)", len(cols))
					}
					api.Scan("LAKE_TEST_RASPBERY001", cols[0])
					api.Scan("2024-01-08 09:36:00 000", cols[1])
					api.Scan(64.125, cols[1])
					return nil
				}
				rm.NextFunc = func() bool {
					if nextCount == 1 {
						return false
					}
					nextCount++
					return true
				}
				rm.CloseFunc = func() error { return nil }
				return rm, nil
			default:
				t.Logf("QueryRow sqlText: %s, params:%v", sqlText, params)
			}
			return rm, nil
		}
		return conn, nil
	}

	b := &bytes.Buffer{}
	selectCalc := SelectCalc{
		TagName:   "DEM_CEMS0_RASPBERYY0.taglet-sim-tag",
		TableName: "TAG",
		TagList:   []string{},
	}

	if err := json.NewEncoder(b).Encode(selectCalc); err != nil {
		t.Fatal(err)
	}

	webService, err := New(dbMock,
		OptionDebugMode(true),
	)
	if err != nil {
		t.Fatal(err)
	}

	wsvr := webService.(*httpd)
	router := wsvr.Router()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/lakes/values/calculated", nil)
	q := req.URL.Query()
	q.Add("tag_name", "LAKE_TEST_RASPBERY001")
	q.Add("table_name", "TAG")
	q.Add("start_time", "2024-01-08 09:12:00 000")
	q.Add("end_time", "2024-01-08 10:12:00 000")
	req.URL.RawQuery = q.Encode()
	router.ServeHTTP(w, req)

	rsp := ResSet{}
	err = json.Unmarshal(w.Body.Bytes(), &rsp)
	if err != nil {
		t.Log(w.Body.String())
		t.Fatal(err)
	}
}
