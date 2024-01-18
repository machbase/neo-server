package httpd

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
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

	runTestQuery := func(sqlText string, expect string, params map[string]string) {
		var w *httptest.ResponseRecorder
		var req *http.Request

		expectRows = 1
		w = httptest.NewRecorder()
		//u := fmt.Sprintf("/db/query?q=%s", url.QueryEscape(sqlText))
		args := []string{fmt.Sprintf("/db/query?q=%s", url.QueryEscape(sqlText))}
		for k, v := range params {
			args = append(args, fmt.Sprintf("%s=%s", k, url.QueryEscape(v)))
		}
		req, _ = http.NewRequest("GET", strings.Join(args, "&"), nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Log("StatusCode:", w.Result().Status, "Body:", w.Body.String())
			t.Fail()
		}
		if strings.HasPrefix(expect, "/r/") {
			reg := regexp.MustCompile("^" + strings.TrimPrefix(expect, "/r/"))
			if actual := w.Body.String(); !reg.MatchString(actual) {
				t.Log("Expect:", expect)
				t.Log("Actual:", actual)
				t.Fail()
			}
		} else {
			if actual := w.Body.String(); actual != expect {
				t.Log("Expect:", expect)
				t.Log("Actual:", actual)
				t.Fail()
			}
		}
	}
	runTestQuery(`select (min(min_time)),(max(max_time)) from v$EXAMPLE_stat where name = 'my-car;'`,
		`/r/{"data":{"columns":\["min\(min_time\)","max\(max_time\)"\],"types":\["datetime","datetime"\],"rows":\[\[-6795364578871345152,-6795364578871345152\]\]},"success":true,"reason":"success","elapse":".+"}`,
		map[string]string{})

	runTestQuery(`select (min(min_time)),(max(max_time)) from v$EXAMPLE_stat where name = 'my-car;'`,
		`/r/{"data":{"columns":\["min\(min_time\)","max\(max_time\)"],"types":\["datetime","datetime"\],"rows":\[-6795364578871345152,-6795364578871345152\]},"success":true,"reason":"success","elapse":".+"}`,
		map[string]string{"format": "json", "rowsFlatten": "true"})

	runTestQuery(`select (min(min_time)),(max(max_time)) from v$EXAMPLE_stat where name = 'my-car;'`,
		`/r/{"data":{"columns":\["min\(min_time\)","max\(max_time\)"\],"types":\["datetime","datetime"\],"rows":\[{"max\(max_time\)":-6795364578871345152,"min\(min_time\)":-6795364578871345152}\]},"success":true,"reason":"success","elapse":".+"}`,
		map[string]string{"format": "json", "rowsArray": "true"})

	runTestQuery(`SELECT to_timestamp((mTime))/1000000 AS TIME, SUM(SUMMVAL) / SUM(CNTMVAL) AS VALUE from (select TIME / (1 * 1 * 1000000000) * (1 * 1 * 1000000000) as mtime, sum(VALUE) as SUMMVAL, count(VALUE) as CNTMVAL from EXAMPLE where NAME in ('wave%3B') and TIME between 1693552595418000000 and 1693552598408000000 group by mTime) Group by TIME order by TIME LIMIT 441`,
		`/r/{"data":{"columns":\["TIME","VALUE"\],"types":\["datetime","double"\],"rows":\[\[-6795364578871345152,0\]\]},"success":true,"reason":"success","elapse":".+"}`,
		map[string]string{})
}
