package httpd

import (
	"context"
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
	dbMock.ConnectFunc = func(ctx context.Context, options ...spi.ConnectOption) (spi.Conn, error) {
		conn := &ConnMock{}
		conn.CloseFunc = func() error { return nil }
		conn.QueryFunc = func(ctx context.Context, sqlText string, params ...any) (spi.Rows, error) {
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
			case "SELECT to_timestamp((mTime))/1000000 AS TIME, SUM(SUMMVAL) / SUM(CNTMVAL) AS VALUE from (select TIME / (1 * 1 * 1000000000) * (1 * 1 * 1000000000) as mtime, sum(VALUE) as SUMMVAL, count(VALUE) as CNTMVAL from EXAMPLE where NAME in ('wave%3B') and TIME between 1693552595418000000 and 1693552598408000000 group by mTime) Group by TIME order by TIME LIMIT 441":
				rows.ScanFunc = func(cols ...any) error {
					if len(params) == 2 {
						*(cols[0].(*time.Time)) = time.Time{}
						*(cols[1].(*float64)) = 1.2345
					}
					return nil
				}
				rows.ColumnsFunc = func() (spi.Columns, error) {
					return []*spi.Column{
						{Name: "TIME", Type: spi.ColumnTypeString(spi.DatetimeColumnType)},
						{Name: "VALUE", Type: spi.ColumnTypeString(spi.Float64ColumnType)},
					}, nil
				}
				rows.IsFetchableFunc = func() bool { return true }
				rows.NextFunc = func() bool {
					expectRows--
					return expectRows >= 0
				}
				rows.CloseFunc = func() error { return nil }
			default:
				fmt.Println("======>SQL:", sqlText)
			}
			return rows, nil
		}
		return conn, nil
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

	expectRows = 1
	w = httptest.NewRecorder()
	u := fmt.Sprintf("/db/query?q=%s", url.QueryEscape(`select (min(min_time)),(max(max_time)) from v$EXAMPLE_stat where name = 'my-car;'`))
	req, _ = http.NewRequest("GET", u, nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Log("StatusCode:", w.Result().Status, "Body:", w.Body.String())
		t.Fail()
	}

	expectRows = 1
	w = httptest.NewRecorder()
	u = `/db/query?q=SELECT%20to_timestamp((mTime))/1000000%20AS%20TIME,%20SUM(SUMMVAL)%20/%20SUM(CNTMVAL)%20AS%20VALUE%20from%20(select%20TIME%20/%20(1%20*%201%20*%201000000000)%20*%20(1%20*%201%20*%201000000000)%20as%20mtime,%20sum(VALUE)%20as%20SUMMVAL,%20count(VALUE)%20as%20CNTMVAL%20from%20EXAMPLE%20where%20NAME%20in%20(%27wave%253B%27)%20and%20TIME%20between%201693552595418000000%20and%201693552598408000000%20group%20by%20mTime)%20Group%20by%20TIME%20order%20by%20TIME%20LIMIT%20441`
	req, _ = http.NewRequest("GET", u, nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Log("StatusCode:", w.Result().Status, "Body:", w.Body.String())
		t.Fail()
	}
}
