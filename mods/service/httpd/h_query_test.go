package httpd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	spi "github.com/machbase/neo-spi"
)

func TestQuery(t *testing.T) {
	var expectRows = 0

	dbMock := &DatabaseMock{}
	dbMock.QueryFunc = func(sqlText string, params ...any) (spi.Rows, error) {
		rows := &RowsMock{}
		switch sqlText {
		case `select (min(min_time)),(max(max_time)) from v$EXAMPLE_stat where name = 'my-car;'`:
			rows.ScanFunc = func(cols ...any) error {
				if len(params) == 2 {
					*(cols[0].(*time.Time)) = time.Time{}
					*(cols[1].(*time.Time)) = time.Time{}
				}
				return nil
			}
			rows.ColumnsFunc = func() (spi.Columns, error) {
				return []*spi.Column{
					{Name: "min(min_time)", Type: spi.ColumnTypeString(spi.DatetimeColumnType)},
					{Name: "max(max_time)", Type: spi.ColumnTypeString(spi.DatetimeColumnType)},
				}, nil
			}
			rows.IsFetchableFunc = func() bool { return true }
			rows.NextFunc = func() bool {
				expectRows--
				return expectRows >= 0
			}
			rows.CloseFunc = func() error { return nil }
		}
		return rows, nil
	}

	svr, err := New(dbMock,
		OptionDebugMode(true),
		OptionHandler("/db", HandlerMachbase),
	)
	if err != nil {
		t.Fatal(err)
	}

	wsvr := svr.(*httpd)
	r := wsvr.Router()

	var w *httptest.ResponseRecorder
	var req *http.Request

	w = httptest.NewRecorder()
	u := fmt.Sprintf("/db/query?q=%s", url.QueryEscape(`select (min(min_time)),(max(max_time)) from v$EXAMPLE_stat where name = 'my-car;'`))
	req, _ = http.NewRequest("GET", u, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Log("StatusCode:", w.Result().Status, "Body:", w.Body.String())
		t.Fail()
	}
}
