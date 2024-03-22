package httpd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	spi "github.com/machbase/neo-spi"
)

func TestRead(t *testing.T) {
	dbMock := &TestClientMock{}
	dbMock.ConnectFunc = func(ctx context.Context, options ...spi.ConnectOption) (spi.Conn, error) {
		conn := &ConnMock{}
		conn.CloseFunc = func() error { return nil }
		conn.QueryRowFunc = func(ctx context.Context, sqlText string, params ...any) spi.Row {
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

	t.Log("b : ", b.String())

	webService, err := New(dbMock,
		OptionDebugMode(true),
		OptionHandler("/lakes", HandlerLake),
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

	t.Log(w.Body.String())
	rsp := ResSet{}
	err = json.Unmarshal(w.Body.Bytes(), &rsp)
	t.Logf("rsp : %+v", rsp)
	if err != nil {
		t.Log(w.Body.String())
		t.Fatal(err)
	}

	t.Logf("rsp : %+v", rsp)

}
