package httpd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/util"
	"github.com/stretchr/testify/require"
)

func TestQuery(t *testing.T) {
	var expectRows = 0

	dbMock := &DatabaseMock{}
	dbMock.ConnectFunc = func(ctx context.Context, options ...api.ConnectOption) (api.Conn, error) {
		conn := &ConnMock{}
		conn.CloseFunc = func() error { return nil }
		conn.QueryFunc = func(ctx context.Context, sqlText string, params ...any) (api.Rows, error) {
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
				rows.ColumnsFunc = func() (api.Columns, error) {
					return api.Columns{
						{Name: "min(min_time)", DataType: api.ColumnTypeDatetime.DataType()},
						{Name: "max(max_time)", DataType: api.ColumnTypeDatetime.DataType()},
					}, nil
				}
				rows.IsFetchableFunc = func() bool { return true }
				rows.MessageFunc = func() string { return "success" }
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
				rows.ColumnsFunc = func() (api.Columns, error) {
					return api.Columns{
						{Name: "TIME", DataType: api.ColumnTypeDatetime.DataType()},
						{Name: "VALUE", DataType: api.ColumnTypeDouble.DataType()},
					}, nil
				}
				rows.IsFetchableFunc = func() bool { return true }
				rows.MessageFunc = func() string { return "success" }
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

type SplitSqlResult struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
	Data    struct {
		Statements []*util.SqlStatement `json:"statements"`
	} `json:"data,omitempty"`
}

func TestSplitSQL(t *testing.T) {
	dbMock := &DatabaseMock{}
	svr, err := New(dbMock,
		OptionDebugMode(true),
	)
	if err != nil {
		t.Fatal(err)
	}

	httpSvr := svr.(*httpd)
	r := httpSvr.Router()

	runTestSplitSQL := func(sqlText string, expect []*util.SqlStatement) {
		var w *httptest.ResponseRecorder
		var req *http.Request

		w = httptest.NewRecorder()
		req, _ = http.NewRequest("POST", "/web/api/splitter/sql", strings.NewReader(sqlText))
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Log("StatusCode:", w.Result().Status, "Body:", w.Body.String())
			t.Fail()
		}
		result := SplitSqlResult{}
		response := w.Body.String()
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Log("Error:", err, response)
			t.Fail()
		}
		if !result.Success {
			t.Log("Error:", result.Reason, response)
			t.Fail()
		}
		require.EqualValues(t, expect, result.Data.Statements, response)
	}
	runTestSplitSQL(`select * from first;`,
		[]*util.SqlStatement{
			{BeginLine: 1, EndLine: 1, IsComment: false, Text: "select * from first;", Env: &util.SqlStatementEnv{}},
		})

	runTestSplitSQL("\nselect * from second;  ",
		[]*util.SqlStatement{
			{BeginLine: 2, EndLine: 2, IsComment: false, Text: "select * from second;", Env: &util.SqlStatementEnv{}},
		})
}
