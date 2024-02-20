package tql_test

//go:generate moq -out ./task_mock_test.go -pkg tql_test ../../../neo-spi Database Conn Rows Result Appender

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/bridge"
	"github.com/machbase/neo-server/mods/model"
	"github.com/machbase/neo-server/mods/tql"
	"github.com/machbase/neo-server/mods/util"
	"github.com/machbase/neo-server/mods/util/ssfs"
	spi "github.com/machbase/neo-spi"
	"github.com/stretchr/testify/require"
)

type CompileErr string
type ExpectErr string
type ExpectLog string
type Payload string
type MatchPrefix bool

type Param = struct {
	name  string
	value string
}

func runTest(t *testing.T, codeLines []string, expect []string, options ...any) {
	t.Helper()
	var compileErr string
	var expectErr string
	var expectLog string
	var payload []byte
	var params map[string][]string
	var matchPrefix bool
	var httpClient *http.Client

	var ctx context.Context
	var ctxCancel context.CancelFunc
	var ctxCancelIgnore bool

	for _, o := range options {
		switch v := o.(type) {
		case CompileErr:
			compileErr = string(v)
		case ExpectErr:
			expectErr = string(v)
		case ExpectLog:
			expectLog = string(v)
		case Payload:
			payload = []byte(v)
		case Param:
			if params == nil {
				params = map[string][]string{}
			}
			arr := params[v.name]
			arr = append(arr, v.value)
			params[v.name] = arr
		case MatchPrefix:
			matchPrefix = bool(v)
		case *http.Client:
			httpClient = v
		case context.Context:
			ctx = v
		}
	}

	code := strings.Join(codeLines, "\n")
	w := &bytes.Buffer{}

	if ctx == nil {
		ctx, ctxCancel = context.WithTimeout(context.TODO(), 10*time.Second)
	} else {
		ctx, ctxCancel = context.WithCancel(ctx)
		ctxCancelIgnore = true
		defer ctxCancel()
	}
	doneCh := make(chan any)

	logBuf := &bytes.Buffer{}

	task := tql.NewTaskContext(ctx)
	task.SetOutputWriter(w)
	task.SetLogWriter(logBuf)
	task.SetLogLevel(tql.INFO)
	task.SetConsoleLogLevel(tql.FATAL)
	task.SetDatabase(&mockDb)
	if len(payload) > 0 {
		task.SetInputReader(bytes.NewBuffer(payload))
	}
	if len(params) > 0 {
		task.SetParams(params)
	}
	if httpClient != nil {
		task.SetHttpClientFactory(func() *http.Client {
			return httpClient
		})
	}
	err := task.CompileString(code)
	if compileErr != "" {
		require.NotNil(t, err)
		require.Equal(t, compileErr, err.Error())
		ctxCancel()
		return
	} else {
		require.Nil(t, err)
	}

	var executeErr error
	go func() {
		result := task.Execute()
		executeErr = result.Err
		if result.IsDbSink {
			b, _ := result.MarshalJSON()
			w.Write(b)
		}
		doneCh <- true
	}()

	select {
	case <-ctx.Done():
		if !ctxCancelIgnore {
			t.Logf("CODE:\n%s", code)
			t.Logf("LOG:\n%s", strings.TrimSpace(logBuf.String()))
			t.Fatal("ERROR time out!!!")
			ctxCancel()
		}
	case <-doneCh:
		ctxCancel()
	}
	logString := strings.TrimSpace(logBuf.String())
	if expectErr != "" {
		// case error
		require.NotNil(t, executeErr)
		require.Equal(t, expectErr, executeErr.Error())
	}
	if expectLog != "" {
		// log message
		if !strings.Contains(logString, expectLog) {
			t.Log("LOG OUTPUT:", logString)
			t.Log("LOG EXPECT:", expectLog)
			t.Fail()
		}
	} else {
		if len(logString) > 0 && expectErr == "" {
			t.Log("LOG OUTPUT:", logString)
		}
	}

	if expectErr == "" {
		require.Nil(t, executeErr)
	}

	if expectErr == "" && expectLog == "" {
		// case success
		require.Nil(t, err)
		result := w.String()
		if matchPrefix {
			strexpect := strings.Join(expect, "\n")
			trimResult := strings.TrimSpace(result)
			strresult := "<N/A>"
			if len(trimResult) >= len(strexpect) {
				strresult = trimResult[0:len(strexpect)]
			} else {
				strresult = trimResult
			}
			require.Equal(t, strexpect, strresult)
		} else {
			resultLines := strings.Split(result, "\n")
			if len(resultLines) > 0 && resultLines[len(resultLines)-1] == "" {
				// remove trailing empty line
				resultLines = resultLines[0 : len(resultLines)-1]
			}
			if len(expect) != len(resultLines) {
				t.Logf("Expect result %d lines, got %d", len(expect), len(resultLines))
				t.Logf("\n%s", strings.Join(resultLines, "\n"))
				t.Fail()
				return
			}

			for n, expectLine := range expect {
				if strings.HasPrefix(expectLine, "/r/") {
					reg := regexp.MustCompile("^" + strings.TrimPrefix(expectLine, "/r/"))
					if !reg.MatchString(resultLines[n]) {
						t.Logf("Expected: %s", expectLine)
						t.Logf("Actual  :    %s", resultLines[n])
						t.Fail()
					}
				} else {
					require.Equal(t, expectLine, resultLines[n], fmt.Sprintf("Expected(line#%d): %s", n, expectLine))
				}
			}
		}
		if strings.Contains(logString, "ERROR") || strings.Contains(logString, "WARN") {
			t.Log("LOG OUTPUT:", logString)
			t.Fail()
		}
	}
}

var mockDbResult [][]any
var mockDbCursor = 0
var mockDb = DatabaseMock{
	ConnectFunc: func(ctx context.Context, options ...spi.ConnectOption) (spi.Conn, error) {
		conn := &ConnMock{
			CloseFunc: func() error { return nil },
			QueryFunc: func(ctx context.Context, sqlText string, params ...any) (spi.Rows, error) {
				switch sqlText {
				case `SELECT time, value FROM EXAMPLE WHERE name = 'tag1' AND time BETWEEN 1 AND 2 LIMIT 0, 1000000`:
					fallthrough
				case `select time, value from example where name = 'tag1'`:
					return &RowsMock{
						IsFetchableFunc: func() bool { return true },
						NextFunc:        func() bool { mockDbCursor++; return len(mockDbResult) >= mockDbCursor },
						CloseFunc:       func() error { return nil },
						ColumnsFunc: func() (spi.Columns, error) {
							return []*spi.Column{
								{Name: "time", Type: "datetime"},
								{Name: "value", Type: "double"},
							}, nil
						},
						MessageFunc: func() string { return "no rows selected." },
						ScanFunc: func(cols ...any) error {
							cols[0] = mockDbResult[mockDbCursor-1][0]
							cols[1] = mockDbResult[mockDbCursor-1][1]
							return nil
						},
					}, nil
				case `create tag table example(...)`:
					return &RowsMock{
						IsFetchableFunc:  func() bool { return false },
						NextFunc:         func() bool { return false },
						CloseFunc:        func() error { return nil },
						MessageFunc:      func() string { return "executed." },
						RowsAffectedFunc: func() int64 { return 0 },
					}, nil
				default:
					fmt.Println("===>", sqlText)
					return &RowsMock{
						IsFetchableFunc: func() bool { return true },
						NextFunc:        func() bool { return false },
						CloseFunc:       func() error { return nil },
					}, nil
				}
			},
			ExecFunc: func(ctx context.Context, sqlText string, params ...any) spi.Result {
				switch sqlText {
				case `INSERT INTO example (name,a) VALUES(?,?)`:
					fmt.Println("task_test, mockdb: ", sqlText, params)
					return &ResultMock{
						ErrFunc:          func() error { return nil },
						MessageFunc:      func() string { return "a row inserted." },
						RowsAffectedFunc: func() int64 { return 1 },
					}
				default:
					fmt.Println("task_test, mockdb: ", sqlText)
				}
				return nil
			},
			AppenderFunc: func(ctx context.Context, tableName string, opts ...spi.AppenderOption) (spi.Appender, error) {
				return &AppenderMock{
					AppendFunc: func(values ...any) error { return nil },
					CloseFunc:  func() (int64, int64, error) { return 0, 0, nil },
				}, nil
			},
		}
		return conn, nil
	},
}

func TestDBSql(t *testing.T) {
	mockDbCursor = 0
	mockDbResult = [][]any{
		{1692686707380411000, 0.1},
		{1692686708380411000, 0.2},
	}
	codeLines := []string{
		`SQL("select time, value from example where name = 'tag1'")`,
		`CSV( precision(3), header(true) )`,
	}
	resultLines := []string{
		"time,value",
		"1692686707380411000,0.100",
		"1692686708380411000,0.200",
	}
	runTest(t, codeLines, resultLines)
}

func TestDBSqlRownum(t *testing.T) {
	mockDbCursor = 0
	mockDbResult = [][]any{
		{1692686707380411000, 0.1},
		{1692686708380411000, 0.2},
	}
	codeLines := []string{
		`SQL("select time, value from example where name = 'tag1'")`,
		`PUSHKEY('test')`,
		`CSV( precision(3), header(true) )`,
	}
	resultLines := []string{
		"ROWNUM,time,value",
		"1,1692686707380411000,0.100",
		"2,1692686708380411000,0.200",
	}
	runTest(t, codeLines, resultLines)
}

func TestDBQuery(t *testing.T) {
	mockDbCursor = 0
	mockDbResult = [][]any{
		{1692686707380411000, 0.1},
		{1692686708380411000, 0.2},
	}
	codeLines := []string{
		`QUERY('value', from('example', 'tag1', "time"), between(1, 2))`,
		`CSV( precision(3), header(true) )`,
	}
	resultLines := []string{
		"time,value",
		"1692686707380411000,0.100",
		"1692686708380411000,0.200",
	}
	runTest(t, codeLines, resultLines)
}

func TestDBQueryRowsFlatten(t *testing.T) {
	mockDbCursor = 0
	mockDbResult = [][]any{
		{1692686707380411000, 0.1},
		{1692686708380411000, 0.2},
	}
	codeLines := []string{
		`QUERY('value', from('example', 'tag1', "time"), between(1, 2))`,
		`JSON( precision(3), rowsFlatten(true) )`,
	}
	resultLines := []string{
		`/r/{"data":{"columns":\["time","value"\],"types":\["datetime","double"\],"rows":\[1692686707380411000,0.1,1692686708380411000,0.2\]},"success":true,"reason":"success","elapse":".+"}`,
	}
	runTest(t, codeLines, resultLines)

	mockDbCursor = 0
	codeLines = []string{
		`QUERY('value', from('example', 'tag1', "time"), between(1, 2))`,
		`JSON( precision(3), rowsFlatten(true), rownum(true) )`,
	}
	resultLines = []string{
		`/r/{"data":{"columns":\["ROWNUM","time","value"\],"types":\["int64","datetime","double"\],"rows":\[1,1692686707380411000,0.1,2,1692686708380411000,0.2\]},"success":true,"reason":"success","elapse":".+"}`,
	}
	runTest(t, codeLines, resultLines)
}

func TestDBInsert(t *testing.T) {
	codeLines := []string{
		`FAKE( linspace(0, 1, 3) )`,
		`INSERT('a', table('example'), tag('signal'))`,
	}
	resultLines := []string{
		`/r/{"success":true,"reason":"success","elapse":".+","data":{"message":"3 rows inserted."}}`,
	}
	runTest(t, codeLines, resultLines)
}

func TestDBAppend(t *testing.T) {
	codeLines := []string{
		`FAKE( linspace(0, 1, 3) )`,
		`MAPVALUE(-1, 'singal')`,
		`APPEND( table('example') )`,
	}
	resultLines := []string{
		`/r/{"success":true,"reason":"success","elapse":".+","data":{"message":"append 3 rows \(success 0, fail 0\)"}}`,
	}
	runTest(t, codeLines, resultLines)
}

func TestDBddl(t *testing.T) {
	var codeLines, resultLines []string

	codeLines = []string{
		`SQL("create tag table example(...)")`,
		`MARKDOWN(html(true), rownum(true), heading(true), brief(true))`,
	}
	resultLines = loadLines("./test/sql_ddl_executed.txt")
	runTest(t, codeLines, resultLines)
}

func TestStrLib(t *testing.T) {
	codeLines := []string{
		`FAKE(json(strSprintf('[%.f, %q]', 123, "hello")))`,
		"CSV( heading(false) )",
	}
	resultLines := []string{
		"123,hello",
	}
	runTest(t, codeLines, resultLines)
}

func TestMovingAvg(t *testing.T) {
	var codeLines, resultLines []string
	codeLines = []string{
		`FAKE( linspace(0, 100, 100) )`,
		`MAP_MOVAVG(1, value(0), 10)`,
		`CSV( precision(4) )`,
	}
	resultLines = loadLines("./test/movavg_result.txt")
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`FAKE( csv("1\n3\n2\n7") )`,
		`MAP_DIFF(0, value(0))`,
		`CSV()`,
	}
	resultLines = []string{"NULL", "2", "-1", "5"}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`FAKE( csv("1\n3\n2\n7") )`,
		`MAP_NONEGDIFF(0, value(0))`,
		`CSV()`,
	}
	resultLines = []string{"NULL", "2", "0", "5"}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`FAKE( csv("1\n3\n2\n7") )`,
		`MAP_ABSDIFF(0, value(0))`,
		`CSV()`,
	}
	resultLines = []string{"NULL", "2", "1", "5"}
	runTest(t, codeLines, resultLines)
}

func TestString(t *testing.T) {
	codeLines := []string{
		`STRING("line1\nline2\n\nline4", separator("\n"))`,
		`PUSHKEY('test')`,
		"CSV( heading(true) )",
	}
	resultLines := []string{
		"ROWNUM,STRING",
		"1,line1",
		"2,line2",
		"3,",
		"4,line4",
	}
	runTest(t, codeLines, resultLines)

	f, _ := ssfs.NewServerSideFileSystem([]string{"test"})
	ssfs.SetDefault(f)

	codeLines = []string{
		`STRING(file("/lines.txt"), separator("\n"), trimspace(true))`,
		`PUSHKEY('test')`,
		"CSV( header(true) )",
	}
	runTest(t, codeLines, resultLines)
}

func TestBytes(t *testing.T) {
	var codeLines, resultLines []string

	codeLines = []string{
		`BYTES("line1\nline2\n\nline4", separator("\n"))`,
		`PUSHKEY('test')`,
		"CSV( heading(true) )",
	}
	resultLines = []string{
		"ROWNUM,BYTES",
		`1,\x6C\x69\x6E\x65\x31`,
		`2,\x6C\x69\x6E\x65\x32`,
		`3,`,
		`4,\x6C\x69\x6E\x65\x34`,
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`BYTES("line1\nline2\n\nline4", separator("\n"))`,
		"CSV( heading(true) )",
	}
	resultLines = []string{
		"BYTES",
		`\x6C\x69\x6E\x65\x31`,
		`\x6C\x69\x6E\x65\x32`,
		``,
		`\x6C\x69\x6E\x65\x34`,
	}
	runTest(t, codeLines, resultLines)

	f, _ := ssfs.NewServerSideFileSystem([]string{"test"})
	ssfs.SetDefault(f)

	codeLines = []string{
		`BYTES(file("/lines.txt"), separator("\n"))`,
		"CSV( header(true) )",
	}
	runTest(t, codeLines, resultLines)
}

func TestHttpFile(t *testing.T) {
	var codeLines, resultLines []string

	httpClient := &http.Client{Transport: TestRoundTripFunc(func(req *http.Request) *http.Response {
		if req.Method != "GET" {
			t.Error("expected request method to be GET, got", req.Method)
			t.Fail()
		}
		var body io.ReadCloser
		switch req.URL.Path {
		case "/string":
			body = io.NopCloser(strings.NewReader("ok."))
		case "/bytes":
			body = io.NopCloser(strings.NewReader("ok."))
		case "/csv":
			body = io.NopCloser(strings.NewReader("1,3.141592,true,\"escaped, string\",123456"))
		default:
			t.Error("Unexpected request path, got", req.URL.Path)
			t.Fail()
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       body,
		}
	})}

	codeLines = []string{
		`STRING(file("http://example.com/string"))`,
		`CSV()`,
	}
	resultLines = []string{
		`ok.`,
	}
	runTest(t, codeLines, resultLines, httpClient)

	codeLines = []string{
		`BYTES(file("http://example.com/bytes"))`,
		`CSV()`,
	}
	resultLines = []string{
		`\x6F\x6B\x2E`,
	}
	runTest(t, codeLines, resultLines, httpClient)

	codeLines = []string{
		`CSV(file("http://example.com/csv"))`,
		`CSV()`,
	}
	resultLines = []string{
		`1,3.141592,true,"escaped, string",123456`,
	}
	runTest(t, codeLines, resultLines, httpClient)
}

func TestDiscardSink(t *testing.T) {
	var codeLines, resultLines []string
	var resultLog ExpectLog

	codeLines = []string{
		`CSV("1,line-1\n2,line-2\n3,line-3")`,
		`MAPVALUE(0, parseFloat(value(0)))`,
		`WHEN(`,
		`  value(0) == 2 && `,
		`  strHasPrefix( strToUpper(value(1)), "LINE-") &&`,
		`  strHasSuffix(value(1), "-2"),`,
		`  do(value(0), strToUpper(value(1)), {`,
		`    ARGS()`,
		`    WHEN(true, doLog("OUTPUT:", value(0), strToLower(value(1)) ))`,
		`    CSV()`,
		`  })`,
		`)`,
		`DISCARD()`,
	}
	resultLines = []string{}
	resultLog = ExpectLog("[WARN] do: CSV() sink does not work in a sub-routine")
	runTest(t, codeLines, resultLines, resultLog)
	resultLog = ExpectLog("[INFO] OUTPUT: 2 line-2")
	runTest(t, codeLines, resultLines, resultLog)

	codeLines = []string{
		`FAKE( json({         `,
		`	[ 1, "hello" ],   `,
		`	[ 2, "你好"],      `,
		`	[ 3, "world" ],   `,
		`	[ 4, "世界"]       `,
		`}))                  `,
		`WHEN(                `,
		`	mod(value(0), 2) == 0,                      `,
		`	do( value(0), strToUpper(value(1)), {       `,
		`		ARGS()                                  `,
		`		WHEN( true, doLog("OUTPUT:", value(0), value(1)))`,
		`		DISCARD()                               `,
		`	})`,
		`)`,
		`CSV()`,
	}
	resultLines = []string{}
	resultLog = ExpectLog("[INFO] OUTPUT: 2 你好")
	runTest(t, codeLines, resultLines, resultLog)
	resultLog = ExpectLog("[INFO] OUTPUT: 4 世界")
	runTest(t, codeLines, resultLines, resultLog)
}

func TestCsvToCsv(t *testing.T) {
	var codeLines, resultLines []string

	codeLines = []string{
		`CSV("1,line1\n2,line2\n3,\n4,line4")`,
		"CSV( heading(true) )",
	}
	resultLines = []string{
		"column0,column1",
		"1,line1",
		"2,line2",
		"3,",
		"4,line4",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`CSV("line1\nline2\n\nline4")`,
		"CSV( heading(true) )",
	}
	resultLines = []string{
		"column0",
		"line1",
		"line2",
		"line4",
	}
	runTest(t, codeLines, resultLines)
}

func TestMath(t *testing.T) {
	codeLines := []string{
		"FAKE( linspace(0, 3.141592/2, 3))",
		"PUSHKEY(sin(value(0)))",
		"PUSHKEY(0)",
		"POPKEY(1)",
		"POPKEY(1)",
		"PUSHKEY('test')",
		"CSV(precision(6))",
	}
	resultLines := []string{
		"0.000000,0.000000",
		"0.785398,0.707107",
		"1.570796,1.000000",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( linspace(0, 3.141592/2, 3))",
		"PUSHKEY(cos(value(0)))",
		"PUSHKEY(0)",
		"POPKEY(1)",
		"POPKEY(1)",
		"PUSHKEY('test')",
		"CSV(precision(6))",
	}
	resultLines = []string{
		"0.000000,1.000000",
		"0.785398,0.707107",
		"1.570796,0.000000",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( linspace(0, 3.141592/2, 3))",
		"PUSHKEY(tan(value(0)))",
		"PUSHKEY(0)",
		"POPKEY(1)",
		"POPKEY(1)",
		"PUSHKEY('test')",
		"CSV(precision(6))",
	}
	resultLines = []string{
		"0.000000,0.000000",
		"0.785398,1.000000",
		"1.570796,3060023.306953",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( linspace(-2, 2, 5))",
		"PUSHKEY(exp(value(0)))",
		"PUSHKEY(0)",
		"POPKEY(1)",
		"POPKEY(1)",
		"PUSHKEY('test')",
		"CSV(precision(6))",
	}
	resultLines = []string{
		"-2.000000,0.135335",
		"-1.000000,0.367879",
		"0.000000,1.000000",
		"1.000000,2.718282",
		"2.000000,7.389056",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( linspace(-2, 2, 5))",
		"PUSHKEY(exp2(value(0)))",
		"PUSHKEY(0)",
		"POPKEY(1)",
		"POPKEY(1)",
		"PUSHKEY('test')",
		"CSV(precision(6))",
	}
	resultLines = []string{
		"-2.000000,0.250000",
		"-1.000000,0.500000",
		"0.000000,1.000000",
		"1.000000,2.000000",
		"2.000000,4.000000",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( linspace(-2, 2, 5))",
		"PUSHKEY(log(value(0)))",
		"PUSHKEY(0)",
		"POPKEY(1)",
		"POPKEY(1)",
		"PUSHKEY('test')",
		"CSV(precision(6))",
	}
	resultLines = []string{
		"-2.000000,NaN",
		"-1.000000,NaN",
		"0.000000,-Inf",
		"1.000000,0.000000",
		"2.000000,0.693147",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( linspace(-2, 2, 5))",
		"PUSHKEY(log10(value(0)))",
		"PUSHKEY(0)",
		"POPKEY(1)",
		"POPKEY(1)",
		"PUSHKEY('test')",
		"CSV(precision(6))",
	}
	resultLines = []string{
		"-2.000000,NaN",
		"-1.000000,NaN",
		"0.000000,-Inf",
		"1.000000,0.000000",
		"2.000000,0.301030",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( linspace(1000, 100, -1) )",
		"CSV(precision(5), header(true))",
	}
	resultLines = []string{"x"}
	runTest(t, codeLines, resultLines)
}

func TestSetVariables(t *testing.T) {
	var codeLines, resultLines []string
	codeLines = []string{
		`FAKE( linspace(0, 1, 3))`,
		`SET(x10, value(0) * 10)`,
		`SET(x10, $x10 + 1)`,
		`MAPVALUE(1, $x10)`,
		`CSV(header(true))`,
	}
	resultLines = []string{
		`x,column`,
		`0,1`,
		`0.5,6`,
		`1,11`,
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`FAKE( arrange(0, 3, 1))`,
		`SET(flag, value(0) != 0 && mod(value(0), 2) == 0 )`,
		`MAPVALUE(1, !$flag)`,
		`CSV(header(true))`,
	}
	resultLines = []string{
		`x,column`,
		`0,true`,
		`1,true`,
		`2,false`,
		`3,true`,
	}
	runTest(t, codeLines, resultLines)
}

func TestMathMarkdown(t *testing.T) {
	var codeLines, resultLines []string
	codeLines = []string{
		`FAKE( linspace(0, 1, 2))`,
		`PUSHKEY('signal')`,
		`MARKDOWN()`,
	}
	resultLines = []string{
		`|ROWNUM|x|`,
		`|:-----|:-----|`,
		`|1|0.000000|`,
		`|2|1.000000|`,
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`FAKE( linspace(0, 1, 2))`,
		`MARKDOWN()`,
	}
	resultLines = []string{
		`|x|`,
		`|:-----|`,
		`|0.000000|`,
		`|1.000000|`,
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`FAKE( linspace(0, 1, -1))`,
		`MARKDOWN()`,
	}
	resultLines = []string{
		`|x|`,
		`|:-----|`,
		"",
		"> *No record*",
	}
	runTest(t, codeLines, resultLines)
}

func TestArrange(t *testing.T) {
	codeLines := []string{
		"FAKE( arrange(0, 2, 1) )",
		"CSV( heading(true), precision(1) )",
	}
	resultLines := []string{
		"x",
		"0.0",
		"1.0",
		"2.0",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( arrange(2, 0, -1) )",
		"CSV( heading(true), precision(1) )",
	}
	resultLines = []string{
		"x",
		"2.0",
		"1.0",
		"0.0",
	}
	runTest(t, codeLines, resultLines)
}

func TestLinspace(t *testing.T) {
	codeLines := []string{
		"FAKE( linspace(0, 2, 3))",
		"CSV( heading(true), precision(1) )",
	}
	resultLines := []string{
		"x",
		"0.0",
		"1.0",
		"2.0",
	}
	runTest(t, codeLines, resultLines)
}

func TestMeshgrid(t *testing.T) {
	codeLines := []string{
		"FAKE( meshgrid(linspace(0, 2, 3), linspace(0, 2, 3)) )",
		"CSV( heading(true), precision(6) )",
	}
	resultLines := []string{
		"x,y",
		"0.000000,0.000000",
		"0.000000,1.000000",
		"0.000000,2.000000",
		"1.000000,0.000000",
		"1.000000,1.000000",
		"1.000000,2.000000",
		"2.000000,0.000000",
		"2.000000,1.000000",
		"2.000000,2.000000",
	}
	runTest(t, codeLines, resultLines)
}

func TestSphere(t *testing.T) {
	codeLines := []string{
		"FAKE( sphere(4, 4) )",
		"PUSHKEY('test')",
		"CSV( header(true), precision(6) )",
	}
	resultLines := loadLines("./test/sphere_4_4.csv")
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( sphere(0, 0) )",
		"PUSHKEY('test')",
		"CSV(header(false), precision(6))",
	}
	resultLines = loadLines("./test/sphere_0_0.csv")
	runTest(t, codeLines, resultLines)
}

func TestScriptSource(t *testing.T) {
	codeLines := []string{
		"SCRIPT(`",
		`ctx := import("context")`,
		`for i := 0; i < 10; i++ {`,
		`  ctx.yieldKey("test", i, i*10)`,
		`}`,
		"`)",
		"CSV()",
	}
	resultLines := []string{
		"0,0", "1,10", "2,20", "3,30", "4,40", "5,50", "6,60", "7,70", "8,80", "9,90",
	}
	runTest(t, codeLines, resultLines)
}

func TestPushKey(t *testing.T) {
	codeLines := []string{
		"FAKE( linspace(0, 1, 2))",
		"PUSHKEY('sample')",
		"PUSHKEY('test')",
		"CSV(header(true))",
	}
	resultLines := []string{
		"key,ROWNUM,x",
		"sample,1,0",
		"sample,2,1",
	}
	runTest(t, codeLines, resultLines)
}

func TestPushAndPopMonad(t *testing.T) {
	codeLines := []string{
		"FAKE( linspace(0, 1, 3))",
		"PUSHKEY('sample')",
		"POPKEY()",
		"CSV(precision(1))",
	}
	resultLines := []string{
		"0.0",
		"0.5",
		"1.0",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`FAKE( linspace(0, 3.141592/2, 5) )`,
		`PUSHKEY(sin(value(0)))`,
		`PUSHKEY(value(0))`,
		`POPKEY(1)`,
		`POPKEY(1)`,
		`PUSHKEY('test')`,
		`CSV(precision(3))`,
	}
	resultLines = []string{
		"0.000,0.000",
		"0.393,0.383",
		"0.785,0.707",
		"1.178,0.924",
		"1.571,1.000",
	}
	runTest(t, codeLines, resultLines)

}

func TestGroupByKey(t *testing.T) {
	codeLines := []string{
		"FAKE( linspace(0, 2, 3))",
		"PUSHKEY('sample')",
		"GROUPBYKEY()",
		"FLATTEN()",
		"PUSHKEY('test')",
		"CSV(precision(6))",
	}
	resultLines := []string{
		"sample,1,0.000000",
		"sample,2,1.000000",
		"sample,3,2.000000",
	}
	runTest(t, codeLines, resultLines)
}

func TestMapKey(t *testing.T) {
	var codeLines, resultLines []string

	codeLines = []string{
		"FAKE( linspace(0, 2, 3))",
		"MAPKEY(value(0)*2)",
		"PUSHKEY('test')",
		"CSV(precision(0))",
	}
	resultLines = []string{
		"0,0",
		"2,1",
		"4,2",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( linspace(0, 2, 3))",
		"MAPKEY(key())",
		"PUSHKEY('test')",
		"CSV(precision(0))",
	}
	resultLines = []string{
		"1,0",
		"2,1",
		"3,2",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( linspace(0, 2, 3))",
		"MAPKEY( key() + 100 )",
		"PUSHKEY('test')",
		"CSV(precision(1))",
	}
	resultLines = []string{
		"101.0,0.0",
		"102.0,1.0",
		"103.0,2.0",
	}
	runTest(t, codeLines, resultLines)
}

func TestPushValue(t *testing.T) {
	var codeLines, resultLines []string

	for i := -2; i <= 0; i++ {
		codeLines = []string{
			"FAKE( linspace(0, 2, 3))",
			fmt.Sprintf("PUSHVALUE(%d, value(0)*1.5)", i),
			"CSV(precision(1), heading(true), rownum(true))",
		}
		resultLines = []string{
			"ROWNUM,column,x",
			"1,0.0,0.0",
			"2,1.5,1.0",
			"3,3.0,2.0",
		}
		runTest(t, codeLines, resultLines)
	}

	for i := 1; i < 2; i++ {
		codeLines = []string{
			"FAKE( linspace(0, 2, 3))",
			fmt.Sprintf("PUSHVALUE(%d, value(0)*1.5, 'x1.5')", i),
			"CSV(precision(1), heading(true), rownum(false))",
		}
		resultLines = []string{
			"x,x1.5",
			"0.0,0.0",
			"1.0,1.5",
			"2.0,3.0",
		}
		runTest(t, codeLines, resultLines)
	}

	for i := 1; i < 2; i++ {
		codeLines = []string{
			`FAKE( json({["a", 0],["b", 1],["c", 2]}))`,
			"POPKEY(0)",
			fmt.Sprintf("PUSHVALUE(%d, value(0)*1.5, 'x1.5')", i),
			"CSV(precision(1), heading(false), rownum(false))",
		}
		resultLines = []string{
			"0.0,0.0",
			"1.0,1.5",
			"2.0,3.0",
		}
		runTest(t, codeLines, resultLines)
	}

	codeLines = []string{
		"FAKE( linspace(0, 2, 3))",
		"PUSHVALUE(1, value(0)*1.5, 'x1.5')",
		"PUSHVALUE(2, value(1)+10, 'add')",
		"CSV(precision(1), heading(true), rownum(false))",
	}
	resultLines = []string{
		"x,x1.5,add",
		"0.0,0.0,10.0",
		"1.0,1.5,11.5",
		"2.0,3.0,13.0",
	}
	runTest(t, codeLines, resultLines)
}

func TestPushPopValue(t *testing.T) {
	var codeLines, resultLines []string
	codeLines = []string{
		"FAKE( linspace(0, 2, 3))",
		"PUSHVALUE(1, value(0)*1.5, 'x1.5')",
		"PUSHVALUE(2, value(1)+10, 'add')",
		"PUSHVALUE(3, value(2)+0.5, 'add2')",
		"POPVALUE(0,1,2)",
		"CSV(precision(1), heading(true), rownum(true))",
	}
	resultLines = []string{
		"ROWNUM,add2",
		"1,10.5",
		"2,12.0",
		"3,13.5",
	}
	runTest(t, codeLines, resultLines)
}

func TestMapValue(t *testing.T) {
	var codeLines, resultLines []string

	codeLines = []string{
		"FAKE( linspace(0, 2, 3))",
		"MAPVALUE(-1, value(0)*1.5)",
		"CSV(precision(1))",
	}
	resultLines = []string{
		"0.0,0.0",
		"1.5,1.0",
		"3.0,2.0",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( linspace(0, 2, 3))",
		"MAPVALUE(99, value(0)*1.5)",
		"CSV(precision(1), header(true))",
	}
	resultLines = []string{
		"x,column",
		"0.0,0.0",
		"1.0,1.5",
		"2.0,3.0",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( linspace(0, 2, 3))",
		"MAPVALUE(0, value(0)*1.5, 'new_column')",
		"CSV(precision(1), header(true))",
	}
	resultLines = []string{
		"new_column",
		"0.0",
		"1.5",
		"3.0",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( csv(`world,3.141592`) )",
		"MAPVALUE(1, parseFloat(value(1)))",
		"MAPVALUE(2, strSprintf(`hello %s, %1.2f`, value(0), value(1)))",
		"CSV()",
	}
	resultLines = []string{
		"world,3.141592,\"hello world, 3.14\"",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( csv(`1,,3`) )",
		"MAPVALUE(0, parseFloat(value(0)))",
		`MAPVALUE(1, value(1) == "" ? 100 : parseFloat(value(1)) )`,
		"MAPVALUE(2, parseFloat(value(2)))",
		"CSV()",
	}
	resultLines = []string{
		"1,100,3",
	}
	runTest(t, codeLines, resultLines)
}

func TestThrottle(t *testing.T) {
	var codeLines, resultLines []string
	codeLines = []string{
		"FAKE( linspace(1, 10, 10))",
		"THROTTLE( 10 )",
		"CSV()",
	}
	resultLines = []string{
		"1", "2", "3", "4", "5", "6", "7", "8", "9", "10",
	}
	t1 := time.Now()
	runTest(t, codeLines, resultLines)
	t2 := time.Now()
	require.GreaterOrEqual(t, t2.Sub(t1), 1*time.Second, "it should take 1 second or longer")

	// TODO: better way to test timeout?
	//
	// codeLines = []string{
	// 	"FAKE( linspace(1, 10, 10))",
	// 	"THROTTLE( 1 )",
	// 	`WHEN(true, doLog("throttled", value(0)))`,
	// 	"CSV()",
	// }
	// resultLines = []string{
	// 	"1", "2",
	// }
	// ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	// defer cancel()
	// t1 = time.Now()
	// runTest(t, codeLines, resultLines, ctx)
	// t2 = time.Now()
	// require.Less(t, t2.Sub(t1), 3*time.Second, "it should be cancelled in time, but %v", t2.Sub(t1))
}

type TestRoundTripFunc func(req *http.Request) *http.Response

func (f TestRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func TestWhen(t *testing.T) {
	err := bridge.Register(&model.BridgeDefinition{
		Type: model.BRIDGE_SQLITE,
		Name: "sqlite",
		Path: "file::memory:?cache=shared",
	})
	if err == bridge.ErrBridgeDisabled {
		return
	}
	require.Nil(t, err)

	br, _ := bridge.GetSqlBridge("sqlite")
	ctx := context.TODO()
	conn, _ := br.Connect(ctx)
	conn.ExecContext(ctx, `create table if not exists test_when (
		id INTEGER NOT NULL PRIMARY KEY,
		name TEXT,
		value INTEGER
	)`)
	conn.Close()

	var codeLines, resultLines []string

	codeLines = []string{
		`FAKE( linspace(0, 2, 2) )`,
		`PUSHVALUE(0, "msg123")`,
		`WHEN( glob("msg*", value(0)), doLog("hello", value(0), value(1)) )`,
		`INSERT(bridge("sqlite"), table("test_when"), "name", "value")`,
	}
	resultLines = []string{
		`/r/{"success":true,"reason":"success","elapse":".+","data":{"message":"1 row inserted."}}`,
	}
	resultLog := ExpectLog("[INFO] hello msg123 0\n[INFO] hello msg123 2")
	runTest(t, codeLines, resultLines, resultLog)

	var notifiedValues = []string{}
	var httpClient *http.Client
	httpClient = &http.Client{Transport: TestRoundTripFunc(func(req *http.Request) *http.Response {
		if req.URL.Path != "/notify" {
			t.Error("expected request to /notify, got", req.URL.Path)
			t.Fail()
		}
		if req.Method != "GET" {
			t.Error("expected request method to be GET, got", req.Method)
			t.Fail()
		}
		notifiedValues = append(notifiedValues, req.URL.Query()["v"]...)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok.")),
		}
	})}
	codeLines = []string{
		`FAKE( linspace(0, 2, 2) )`,
		`PUSHVALUE(0, "msg123")`,
		`WHEN( glob("msg*", value(0)), doHttp("GET", strSprintf("http://example.com/notify?v=%f", value(1)), nil) )`,
		`INSERT(bridge("sqlite"), table("test_when"), "name", "value")`,
	}
	resultLines = []string{
		`/r/{"success":true,"reason":"success","elapse":".+","data":{"message":"1 row inserted."}}`,
	}
	runTest(t, codeLines, resultLines, httpClient)
	require.Equal(t, 2, len(notifiedValues), "notified should call 2 time, but %d", len(notifiedValues))
	require.Equal(t, "0.000000", notifiedValues[0])
	require.Equal(t, "2.000000", notifiedValues[1])

	notifiedValues = notifiedValues[0:0]
	httpClient = &http.Client{Transport: TestRoundTripFunc(func(req *http.Request) *http.Response {
		if req.URL.String() != "http://example.com/notify" {
			t.Error("expected request to http://example.com/notify, got", req.URL.String())
			t.Fail()
		}
		if req.Method != "POST" {
			t.Error("expected request method to be POST, got", req.Method)
			t.Fail()
		}
		if req.Header.Get("Content-Type") != "text/csv" {
			t.Error("expected request Content-Type header to be text/csv, got", req.Header.Get("Content-Type"))
			t.Fail()
		}
		scan := bufio.NewScanner(req.Body)
		for scan.Scan() {
			notifiedValues = append(notifiedValues, scan.Text())
		}
		fmt.Println(notifiedValues)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok.")),
		}
	})}
	codeLines = []string{
		`FAKE( linspace(0, 2, 2) )`,
		`PUSHVALUE(0, "msg123")`,
		`WHEN( glob("msg*", value(0)), doHttp("POST", "http://example.com/notify", value()) )`,
		`INSERT(bridge("sqlite"), table("test_when"), "name", "value")`,
	}
	resultLines = []string{
		`/r/{"success":true,"reason":"success","elapse":".+","data":{"message":"1 row inserted."}}`,
	}
	runTest(t, codeLines, resultLines, httpClient)
	require.Equal(t, 2, len(notifiedValues), "notified should call 2 time, but %d", len(notifiedValues))
	require.Equal(t, "msg123,0", notifiedValues[0])
	require.Equal(t, "msg123,2", notifiedValues[1])

	codeLines = []string{
		`FAKE( linspace(0, 1, 2) )`,
		`WHEN( mod(value(0),2) == 1, do("test", value(0), {`,
		`  ARGS() // some comment`,
		`  WHEN(true, doLog("MSG", args(0), args(1), "안녕") ) // some comment`,
		`  DISCARD() // some comment`,
		`} )) // some comment`,
		`DISCARD() // some comment`,
	}
	resultLines = []string{}
	resultLog = ExpectLog("[INFO] MSG test 1 안녕")
	runTest(t, codeLines, resultLines, httpClient)

	codeLines = []string{
		`FAKE( linspace(0, 1, 2) )`,
		`WHEN( mod(value(0),2) == 1, do("test", value(0), {`,
		`  FAKE( args() )`,
		`  WHEN(true, doLog("MSG", args(0), args(1), "안녕") )`,
		`  DISCARD()`,
		`} ))`,
		`DISCARD()`,
	}
	resultLines = []string{}
	resultLog = ExpectLog("[INFO] MSG test 1 안녕")
	runTest(t, codeLines, resultLines, httpClient)
}

func TestGroup(t *testing.T) {
	var codeLines, payload, resultLines []string

	payload = []string{
		"A,1",
		"B,3",
		"C,6",
	}

	codeLines = []string{
		`CSV(payload(), field(0, stringType(), "name"), field(1, doubleType(), "value"))`,
		`GROUP( )`,
		"CSV()",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")), ExpectErr("GROUP() has no aggregator"))

	codeLines = []string{
		`CSV(payload(), field(0, stringType(), "name"), field(1, doubleType(), "value"))`,
		`SET(ErrKey, NULL)`,
		`GROUP( by($ErrKey, "NAME"), avg(value(1)))`,
		"CSV()",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")), ExpectErr("GROUP() has by() with NULL"))

	codeLines = []string{
		`CSV(payload(), field(0, stringType(), "name"), field(1, doubleType(), "value"))`,
		`GROUP( by(value(0), "NAME"), avg(value(1)), true)`,
		"CSV()",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")), ExpectErr("GROUP() unknown type 'bool' in arguments"))

	payload = []string{
		"A,1",
		"A,2",
		"B,3",
		"B,4",
		"B,5",
		"C,6",
		"C,7",
		"C,8",
		"C,9",
	}
	// first, last, avg, sum
	codeLines = []string{
		`CSV(payload(), field(0, stringType(), "name"), field(1, doubleType(), "value"))`,
		`GROUP(by(value(0)), first(value(1)), last(value(1)), avg(value(1)), sum(value(1)), count(value(1)) )`,
		`CSV(heading(true), precision(2))`,
	}
	resultLines = []string{
		"GROUP,FIRST,LAST,AVG,SUM,COUNT",
		"A,1.00,2.00,1.50,3.00,2.00",
		"B,3.00,5.00,4.00,12.00,3.00",
		"C,6.00,9.00,7.50,30.00,4.00",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	// min, max, rss, rms
	codeLines = []string{
		`CSV(payload(), field(0, stringType(), "name"), field(1, doubleType(), "value"))`,
		`GROUP(by(value(0)), min(value(1)), max(value(1)), rss(value(1)), rms(value(1)) )`,
		`CSV(heading(true), precision(2))`,
	}
	resultLines = []string{
		"GROUP,MIN,MAX,RSS,RMS",
		"A,1.00,2.00,2.24,1.58",
		"B,3.00,5.00,7.07,4.08",
		"C,6.00,9.00,15.17,7.58",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	// mean, median, stddev, stderr, entropy
	codeLines = []string{
		`CSV(payload(), field(0, stringType(), "name"), field(1, doubleType(), "value"))`,
		`GROUP(by(value(0)), mean(value(1)), median(value(1)), stddev(value(1)), stderr(value(1)), entropy(value(1)) )`,
		`CSV(heading(true), precision(2))`,
	}
	resultLines = []string{
		"GROUP,MEAN,QUANTILE,STDDEV,STDERR,ENTROPY",
		"A,1.50,1.00,0.71,0.50,-1.39",
		"B,4.00,4.00,1.00,0.58,-16.89",
		"C,7.50,7.00,1.29,0.65,-60.78",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	// mean
	codeLines = []string{
		`CSV(payload(), field(0, stringType(), "name"), field(1, doubleType(), "value"))`,
		`GROUP(by(value(0)), mean(value(1)), mean(value(1), weight(value(1))), variance(value(1)) )`,
		`CSV(heading(true), precision(2))`,
	}
	resultLines = []string{
		"GROUP,MEAN,MEAN,VARIANCE",
		"A,1.50,1.67,0.50",
		"B,4.00,4.17,1.00",
		"C,7.50,7.67,1.67",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	// stddev
	codeLines = []string{
		`CSV(payload(), field(0, stringType(), "name"), field(1, doubleType(), "value"))`,
		`GROUP(by(value(0)), stddev(value(1)), stddev(value(1), weight(value(1))) )`,
		`CSV(heading(true), precision(2))`,
	}
	resultLines = []string{
		"GROUP,STDDEV,STDDEV",
		"A,0.71,0.58",
		"B,1.00,0.83",
		"C,1.29,1.12",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	// stderr
	codeLines = []string{
		`CSV(payload(), field(0, stringType(), "name"), field(1, doubleType(), "value"))`,
		`GROUP(by(value(0)), stderr(value(1)), stderr(value(1), weight(value(1))) )`,
		`CSV(heading(true), precision(2))`,
	}
	resultLines = []string{
		"GROUP,STDERR,STDERR",
		"A,0.50,0.41",
		"B,0.58,0.48",
		"C,0.65,0.56",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	// quantile
	codeLines = []string{
		`CSV(payload(), field(0, stringType(), "name"), field(1, doubleType(), "value"))`,
		`GROUP(by(value(0)), quantile(value(1), 0.99, "P99"), quantile(value(1), 0.5, "P50"), median(value(1), "MEDIAN") )`,
		`CSV(heading(true), precision(2))`,
	}
	resultLines = []string{
		"GROUP,P99,P50,MEDIAN",
		"A,2.00,1.00,1.00",
		"B,5.00,4.00,4.00",
		"C,9.00,7.00,7.00",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	// weighted quantile
	codeLines = []string{
		`CSV(payload(), field(0, stringType(), "name"), field(1, doubleType(), "value"))`,
		`GROUP(by(value(0)), quantile(value(1), 0.99, weight(value(1)), "P99"), quantile(value(1), 0.5, "P50"), median(value(1), "MEDIAN") )`,
		`CSV(heading(true), precision(2))`,
	}
	resultLines = []string{
		"GROUP,P99,P50,MEDIAN",
		"A,2.00,1.00,1.00",
		"B,5.00,4.00,4.00",
		"C,9.00,7.00,7.00",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	// payload
	payload = []string{
		"A,1.1",
		"A,1.1",
		"B,2.1",
		"B,2.2",
		"B,2.1",
		"C,3.1",
		"C,3.2",
		"C,3.3",
		"C,3.3",
	}
	// mode
	codeLines = []string{
		`CSV(payload(), field(0, stringType(), "name"), field(1, doubleType(), "value"))`,
		`GROUP(by(value(0)), mode(value(1)), mode(value(1), weight(value(1))) )`,
		`CSV(heading(true), precision(2))`,
	}
	resultLines = []string{
		"GROUP,MODE,MODE",
		"A,1.10,1.10",
		"B,2.10,2.10",
		"C,3.30,3.30",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	// quantile
	codeLines = []string{
		`CSV(payload(), field(0, stringType(), "name"), field(1, doubleType(), "value"))`,
		`GROUP(by(value(0)), quantile(value(1), 0.99, "P99"), quantile(value(1), 0.5, "P50") )`,
		`CSV(heading(true), precision(2))`,
	}
	resultLines = []string{
		"GROUP,P99,P50",
		"A,1.10,1.10",
		"B,2.20,2.10",
		"C,3.30,3.20",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	// cdf
	codeLines = []string{
		`CSV(payload(), field(0, stringType(), "name"), field(1, doubleType(), "value"))`,
		`GROUP(by(value(0)), cdf(value(1), 3.1, "Q99") )`,
		`CSV(heading(true), precision(2))`,
	}
	resultLines = []string{
		"GROUP,Q99",
		"A,1.00",
		"B,1.00",
		"C,0.25",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	// payload
	payload = []string{
		"A,0,0",
		"A,1,1",
		"A,2,2",
		"B,1,1",
		"B,1,1",
		"B,1,1",
		"C,1,10",
		"C,2,100",
		"C,3,200",
		"C,4,300",
	}

	// lrs - linear regression slope
	codeLines = []string{
		`CSV(payload(), field(0, stringType(), "name"), field(1, doubleType(), "x"), field(2, doubleType(), "y"))`,
		`GROUP(by(value(0)), lrs(value(1), value(2), "SLOPE") )`,
		`CSV(heading(true), precision(2))`,
	}
	resultLines = []string{
		"GROUP,SLOPE",
		"A,1.00",
		"B,NULL",
		"C,97.00",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	// payload
	payload = []string{
		"8,10,2",
		"-3,5,1.5",
		"7,6,3",
		"8,3,3",
		"-4,-1,2",
	}
	// correlation
	codeLines = []string{
		`CSV(payload(), field(0, doubleType(), "x"), field(1, doubleType(), "y"), field(2, doubleType(), "w"))`,
		`GROUP(correlation(value(0), value(1), weight(value(2)), "CORR") )`,
		`CSV(heading(true), precision(5))`,
	}
	resultLines = []string{
		"CORR",
		"0.59915",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	// payload
	payload = []string{
		"8,10,1",
		"-3,2,2",
		"7,2,3",
		"8,4,4",
		"-4,1,5",
	}
	// moment
	codeLines = []string{
		`CSV(payload(), field(0, doubleType(), "x"), field(1, doubleType(), "y1"), field(2, doubleType(), "y2"))`,
		`GROUP(
			moment(value(0), 2, weight(2.0), "N1"),
			moment(value(2), 2, weight(1.0), "N2"),
			moment(value(2), 1, "N3")
		)`,
		`CSV(heading(true), precision(2))`,
	}
	resultLines = []string{
		"N1,N2,N3",
		"30.16,2.00,0.00",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	// payload
	payload = []string{
		"8,2",
		"2,2",
		"-9,6",
		"15,7",
		"4,1",
	}
	// variance
	codeLines = []string{
		`CSV(payload(), field(0, doubleType(), "x"), field(1, doubleType(), "w") )`,
		`GROUP(
			variance(value(0), "VARIANCE"),
			variance(value(0), weight(value(1)), "VARIANCE-WEIGHTED")
		)`,
		`CSV(heading(true), precision(4))`,
	}
	resultLines = []string{
		"VARIANCE,VARIANCE-WEIGHTED",
		"77.5000,111.7941",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))
}

func TestGroupWhere(t *testing.T) {
	var codeLines, payload, resultLines []string

	// time
	payload = []string{
		// 50
		// 55
		// 60
		"1700256261,dry,1",
		"1700256262,dry,2",
		"1700256262,wet,2",
		"1700256263,dry,3",
		"1700256264,dry,4",
		"1700256264,wet,4",
		// 65
		"1700256265,wet,5",
		"1700256265,dry,5",
		"1700256266,dry,6",
		"1700256267,dry,7",
		"1700256268,dry,8",
		"1700256269,dry,9",
		// 70
		// 75
		"1700256276,dry,10",
		// 80
	}
	codeLines = []string{
		`CSV(payload(), field(0, datetimeType("s"), "time"), field(2, doubleType(), "value"))`,
		`GROUP(`,
		`  by( roundTime(value(0), "2s")),`,
		`  avg(value(2), where(value(1) == "dry"), "DRY"),`,
		`  last(value(2), where(value(1) == "wet"), "WET") )`,
		`CSV(timeformat("s"), heading(true), precision(2))`,
	}
	resultLines = []string{
		"GROUP,DRY,WET",
		"1700256260,1.00,NULL",
		"1700256262,2.50,2.00",
		"1700256264,4.50,5.00",
		"1700256266,6.50,NULL",
		"1700256268,8.50,NULL",
		"1700256276,10.00,NULL",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	codeLines = []string{
		`CSV(payload(), field(0, datetimeType("s"), "time"), field(2, doubleType(), "value"))`,
		`GROUP(`,
		`  by( roundTime(value(0), "2s")),`,
		`  avg(value(2), where(value(1) == "dry"), "DRY"),`,
		`  last(value(2), where(value(1) == "wet"), nullValue("1"), "WET") )`,
		`CSV(timeformat("s"), heading(true), precision(2))`,
	}
	resultLines = []string{
		"GROUP,DRY,WET",
		"1700256260,1.00,1",
		"1700256262,2.50,2.00",
		"1700256264,4.50,5.00",
		"1700256266,6.50,1",
		"1700256268,8.50,1",
		"1700256276,10.00,1",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))
}

func TestGroupByTimeWindow(t *testing.T) {
	var codeLines, payload, resultLines []string

	// time
	payload = []string{
		// 55
		// 60
		"1700256261,1",
		"1700256262,2",
		"1700256263,3",
		"1700256264,4",
		// 65
		"1700256266,5",
		"1700256267,6",
		"1700256268,7",
		"1700256269,8",
		// 70
		// 75
		"1700256276,9",
		// 80
	}
	codeLines = []string{
		`CSV(payload(), field(0, datetimeType("s"), "time"), field(1, doubleType(), "value"))`,
		`GROUP( by( value(0), timewindow(`,
		`           time(1700256255 * 1000000000),`,
		`           time(1700256282 * 1000000000),`,
		`           period("2s"))),`,
		`      avg(value(1)),`,
		`      last(value(1), nullValue(0)),`,
		`      last(value(1), predict("linearregression"), "PREDICT"),`,
		`      last(value(1), predict("akimaspline"), nullValue(100), "PREDICT")`,
		` )`,
		`CSV(timeformat("s"), heading(true), precision(2))`,
	}
	resultLines = []string{
		"GROUP,AVG,LAST,PREDICT,PREDICT",
		"1700256256,NULL,0.00,NULL,100.00",
		"1700256258,NULL,0.00,NULL,100.00",
		"1700256260,1.00,1.00,1.00,1.00",
		"1700256262,2.50,3.00,3.00,3.00",
		"1700256264,4.00,4.00,4.00,4.00",
		"1700256266,5.50,6.00,6.00,6.00",
		"1700256268,7.50,8.00,8.00,8.00",
		"1700256270,NULL,0.00,9.50,8.00",
		"1700256272,NULL,0.00,11.20,8.00",
		"1700256274,NULL,0.00,12.90,8.00",
		"1700256276,9.00,9.00,9.00,9.00",
		"1700256278,NULL,0.00,11.17,9.00",
		"1700256280,NULL,0.00,12.17,9.00",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	codeLines = []string{
		`CSV(payload(), field(0, datetimeType("s"), "time"), field(1, doubleType(), "value"))`,
		`GROUP( by( value(0), timewindow(`,
		`             time(1700256255 * 1000000000),`,
		`             time(1700256282 * 1000000000),`,
		`             period("4s"))),`,
		`      avg(value(1)),`,
		`      sum(value(1)),`,
		`      last(value(1))`,
		`)`,
		`CSV(timeformat("s"), heading(true), precision(2))`,
	}
	resultLines = []string{
		"GROUP,AVG,SUM,LAST",
		"1700256256,NULL,NULL,NULL",
		"1700256260,2.00,6.00,3.00",  // 1,2,3
		"1700256264,5.00,15.00,6.00", // 4,5,6
		"1700256268,7.50,15.00,8.00", // 7,8
		"1700256272,NULL,NULL,NULL",  //
		"1700256276,9.00,9.00,9.00",  // 9
		"1700256280,NULL,NULL,NULL",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

}

func TestTimeWindow(t *testing.T) {
	var codeLines, payload, resultLines []string

	for _, agg := range []string{
		"avg", "mean", "median", "median-interpolated",
		"stddev", "stderr", "entropy",
		"sum", "first", "last", "min", "max",
		"rss", "rms",
		"rss:LinearRegression", "rss:PiecewiseConstant", "rss:PiecewiseLinear",
	} {
		codeLines = []string{
			`CSV(payload(),
				field(0, datetimeType("s"), "time"),
				field(1, doubleType(), "value"))`,
			fmt.Sprintf(`TIMEWINDOW(
				time(1700256250 * 1000000000),
				time(1700256285 * 1000000000),
				period('5s'),
				nullValue(0),
				'time', '%s')`, agg),
			`CSV(timeformat("s"), heading(true), precision(2))`,
		}
		payload = []string{
			// 50
			// 55
			// 60
			"1700256261,1",
			"1700256262,2",
			"1700256263,3",
			"1700256264,4",
			// 65
			"1700256265,5",
			"1700256266,6",
			"1700256267,7",
			"1700256268,8",
			"1700256269,9",
			// 70
			// 75
			"1700256276,10",
			// 80
		}

		switch agg {
		case "stddev":
			resultLines = []string{
				`time,value`,
				"1700256250,0.00",
				"1700256255,0.00",
				"1700256260,1.29",
				"1700256265,1.58",
				"1700256270,0.00",
				"1700256275,0.00",
				"1700256280,0.00",
			}
		case "stderr":
			resultLines = []string{
				`time,value`,
				"1700256250,0.00",
				"1700256255,0.00",
				"1700256260,0.65",
				"1700256265,0.71",
				"1700256270,0.00",
				"1700256275,0.00",
				"1700256280,0.00",
			}
		case "entropy":
			resultLines = []string{
				`time,value`,
				"1700256250,0.00",
				"1700256255,0.00",
				"1700256260,-10.23",
				"1700256265,-68.83",
				"1700256270,0.00",
				"1700256275,-23.03",
				"1700256280,0.00",
			}
		case "avg":
			resultLines = []string{
				`time,value`,
				"1700256250,0.00",
				"1700256255,0.00",
				"1700256260,2.50",
				"1700256265,7.00",
				"1700256270,0.00",
				"1700256275,10.00",
				"1700256280,0.00",
			}
		case "mean":
			resultLines = []string{
				`time,value`,
				"1700256250,0.00",
				"1700256255,0.00",
				"1700256260,2.50",
				"1700256265,7.00",
				"1700256270,0.00",
				"1700256275,10.00",
				"1700256280,0.00",
			}
		case "median":
			resultLines = []string{
				`time,value`,
				"1700256250,0.00",
				"1700256255,0.00",
				"1700256260,2.00",
				"1700256265,7.00",
				"1700256270,0.00",
				"1700256275,10.00",
				"1700256280,0.00",
			}
		case "median-interpolated":
			resultLines = []string{
				`time,value`,
				"1700256250,0.00",
				"1700256255,0.00",
				"1700256260,2.00",
				"1700256265,6.50",
				"1700256270,0.00",
				"1700256275,10.00",
				"1700256280,0.00",
			}
		case "sum":
			resultLines = []string{
				`time,value`,
				"1700256250,0.00",
				"1700256255,0.00",
				"1700256260,10.00",
				"1700256265,35.00",
				"1700256270,0.00",
				"1700256275,10.00",
				"1700256280,0.00",
			}
		case "first", "min":
			resultLines = []string{
				`time,value`,
				"1700256250,0.00",
				"1700256255,0.00",
				"1700256260,1.00",
				"1700256265,5.00",
				"1700256270,0.00",
				"1700256275,10.00",
				"1700256280,0.00",
			}
		case "last", "max":
			resultLines = []string{
				`time,value`,
				"1700256250,0.00",
				"1700256255,0.00",
				"1700256260,4.00",
				"1700256265,9.00",
				"1700256270,0.00",
				"1700256275,10.00",
				"1700256280,0.00",
			}
		case "rss":
			resultLines = []string{
				`time,value`,
				"1700256250,0.00",
				"1700256255,0.00",
				"1700256260,5.48",
				"1700256265,15.97",
				"1700256270,0.00",
				"1700256275,10.00",
				"1700256280,0.00",
			}
		case "rss:LinearRegression":
			resultLines = []string{
				`time,value`,
				"1700256250,7.60",
				"1700256255,8.46",
				"1700256260,5.48",
				"1700256265,15.97",
				"1700256270,11.06",
				"1700256275,10.00",
				"1700256280,12.79",
			}
		case "rss:PiecewiseConstant":
			resultLines = []string{
				`time,value`,
				"1700256250,5.48",
				"1700256255,5.48",
				"1700256260,5.48",
				"1700256265,15.97",
				"1700256270,10.00",
				"1700256275,10.00",
				"1700256280,10.00",
			}
		case "rss:PiecewiseLinear":
			resultLines = []string{
				`time,value`,
				"1700256250,5.48",
				"1700256255,5.48",
				"1700256260,5.48",
				"1700256265,15.97",
				"1700256270,12.98",
				"1700256275,10.00",
				"1700256280,10.00",
			}
		case "rms":
			resultLines = []string{
				`time,value`,
				"1700256250,0.00",
				"1700256255,0.00",
				"1700256260,2.74",
				"1700256265,7.14",
				"1700256270,0.00",
				"1700256275,10.00",
				"1700256280,0.00",
			}
		}
		runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))
	}
}

func TestTimeWindowMs(t *testing.T) {
	var codeLines, payload, resultLines []string

	codeLines = []string{
		`CSV(payload(),
			field(0, datetimeType("ms"), "time"),
			field(1, doubleType(), "value"))`,
		`TIMEWINDOW(
			time(1700256250 * 1000000000),
			time(1700256285 * 1000000000),
			period('5s'),
			'time', 'avg')`,
		`CSV(timeformat("ms"), heading(true))`,
	}
	payload = []string{
		// 50
		// 55
		// 60
		"1700256261001,1",
		"1700256262010,2",
		"1700256263100,3",
		"1700256264010,4",
		// 65
		"1700256265002,5",
		"1700256266020,6",
		"1700256267200,7",
		"1700256268020,8",
		"1700256269002,9",
		// 70
		// 75
		"1700256276300,10",
		// 80
	}
	resultLines = []string{
		`time,value`,
		"1700256250000,NULL",
		"1700256255000,NULL",
		"1700256260000,2.5",
		"1700256265000,7",
		"1700256270000,NULL",
		"1700256275000,10",
		"1700256280000,NULL",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))
}

func TestTimeWindowHighDef(t *testing.T) {
	var codeLines, resultLines []string

	tick := time.Unix(0, 1692329338315327000)
	util.StandardTimeNow = func() time.Time { return tick }

	codeLines = []string{
		`FAKE( 
			oscillator(
			  freq(15, 1.0), freq(24, 1.5),
			  range('now', '10s', '1ms')) 
		  )`,
		`TIMEWINDOW(
			time('now'),
			time('now+10s'),
			period('1s'),
			'time', 'first')`,
		`CSV(timeformat("ns"), heading(true), precision(7))`,
	}
	resultLines = []string{
		`time,value`,
		"1692329339000000000,0.1046705",
		"1692329340000000000,0.1046637",
		"1692329341000000000,0.1046874",
		"1692329342000000000,0.1046806",
		"1692329343000000000,0.1046738",
		"1692329344000000000,0.1046670",
		"1692329345000000000,0.1046906",
		"1692329346000000000,0.1046838",
		"1692329347000000000,0.1046770",
		"1692329348000000000,0.1046702",
	}
	runTest(t, codeLines, resultLines)
}

func TestDropTake(t *testing.T) {
	codeLines := []string{
		"FAKE( linspace(0, 2, 100))",
		"DROP(50)",
		"TAKE(3)",
		"PUSHKEY('test')",
		"CSV(precision(6))",
	}
	resultLines := []string{
		"51,1.010101",
		"52,1.030303",
		"53,1.050505",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( linspace(0, 2, 100))",
		"DROP(0)",
		"TAKE(2)",
		"PUSHKEY('test')",
		"CSV(precision(6))",
	}
	resultLines = []string{
		"1,0.000000",
		"2,0.020202",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( linspace(0, 2, 100))",
		"DROP(0)",
		"TAKE(0)",
		"PUSHKEY('test')",
		"CSV(precision(6))",
	}
	resultLines = []string{}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( linspace(0, 2, 100))",
		"DROP(5, 45)",
		"TAKE(5, 3)",
		"PUSHKEY('test')",
		"CSV(precision(6))",
	}
	resultLines = []string{
		"51,1.010101",
		"52,1.030303",
		"53,1.050505",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( linspace(0, 2, 100) )",
		"TAKE(5, -1)",
		"CSV(precision(6))",
	}
	runTest(t, codeLines, []string{}, ExpectErr("f(TAKE) arg(1) limit should be larger than 0"))

	codeLines = []string{
		"FAKE( linspace(0, 2, 100) )",
		"DROP(5, -1)",
		"CSV(precision(6))",
	}
	runTest(t, codeLines, []string{}, ExpectErr("f(DROP) arg(1) limit should be larger than 0"))
}

func TestDict(t *testing.T) {
	var codeLines, resultLines []string

	codeLines = []string{
		"FAKE( arrange(0, 1, 1) )",
		`MAPVALUE(0, dict("key", value(0)) )`,
		"JSON(precision(0))",
	}
	resultLines = []string{`/r/{"data":{"columns":\["x"\],"types":\["double"\],"rows":\[\[{"key":0}\],\[{"key":1}\]\]},"success":true,"reason":"success","elapse":".+"}`}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( arrange(0, 1, 1) )",
		`MAPVALUE(0, dict("key", value(0), "value") )`,
		"JSON(precision(0))",
	}
	resultLines = []string{}
	runTest(t, codeLines, resultLines, ExpectErr("dict() name \"value\" doen't match with any value"))

	codeLines = []string{
		"FAKE( arrange(0, 1, 1) )",
		`MAPVALUE(0, dict(123, value(0)) )`,
		"JSON(precision(0))",
	}
	resultLines = []string{}
	runTest(t, codeLines, resultLines, ExpectErr("dict() name should be string, got args[0] float64"))

}

func TestSrcError(t *testing.T) {
	codeLines := []string{
		"SQL('select * from example')",
		"SQL('select * from example')",
		"JSON()",
	}
	resultLines := []string{}
	runTest(t, codeLines, resultLines, CompileErr("\"SQL()\" is not applicable for MAP, line 2"))

	codeLines = []string{
		"MAPVALUE(0, 1)",
		"SQL('select * from example')",
		"JSON()",
	}
	runTest(t, codeLines, resultLines, CompileErr("\"MAPVALUE()\" is not applicable for SRC, line 1"))
}

func TestOcillator(t *testing.T) {
	tick := time.Unix(0, 1692329338315327000)
	util.StandardTimeNow = func() time.Time { return tick }

	codeLines := []string{
		"FAKE( oscillator() )",
		"JSON()",
	}
	resultLines := []string{}
	runTest(t, codeLines, resultLines, ExpectErr("f(oscillator) no time range is defined"))

	codeLines = []string{
		"FAKE( oscillator(123) )",
		"JSON()",
	}
	runTest(t, codeLines, resultLines, ExpectErr("f(oscillator) invalid arg type 'float64'"))

	codeLines = []string{
		"FAKE( oscillator(freq(1.0, 1.0)) )",
		"JSON()",
	}
	runTest(t, codeLines, resultLines, ExpectErr("f(oscillator) no time range is defined"))

	codeLines = []string{
		"FAKE( oscillator(freq(1.0, 1.0), range(time('now-1s'), '1s', '200ms'), range(time('now-1s'), '1s', '200ms')) )",
		"JSON()",
	}
	runTest(t, codeLines, resultLines, ExpectErr("f(oscillator) duplicated time range"))

	codeLines = []string{
		"FAKE( oscillator(freq(1.0, 1.0), range(time('now-1s'), '1s', '-200ms')) )",
		"JSON()",
	}
	runTest(t, codeLines, resultLines, ExpectErr("f(oscillator) period should be positive"))

	codeLines = []string{
		"FAKE( oscillator(freq(1.0, 1.0), range(time('now-1s'), '1s', '200ms')) )",
		"JSON()",
	}
	resultLines = loadLines("./test/oscillator_1.txt")
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( oscillator(freq(1.0, 1.0), range(time('now'), '-1s', '200ms')) )",
		"JSON()",
	}
	resultLines = loadLines("./test/oscillator_1.txt")
	runTest(t, codeLines, resultLines)
}

func TestFFT2D(t *testing.T) {
	codeLines := []string{
		"FAKE( oscillator( range(timeAdd(1685714509*1000000000,'1s'), '1s', '100us'), freq(10, 1.0), freq(50, 2.0)))",
		"MAPKEY('samples')",
		"GROUPBYKEY(lazy(false))",
		"FFT(minHz(0), maxHz(60))",
		"CSV(precision(6))",
	}
	resultLines := loadLines("./test/fft2d.txt")
	runTest(t, codeLines, resultLines)

	// less than 16 samples
	codeLines = []string{
		"FAKE( linspace(0, 10, 100) )",
		"FFT()",
		"CSV()",
	}
	runTest(t, codeLines, []string{})

	codeLines = []string{
		"FAKE( meshgrid(linspace(0, 10, 100), linspace(0, 10, 1000)) )",
		"PUSHKEY('sample')",
		"GROUPBYKEY()",
		"FFT()",
		"CSV()",
	}
	runTest(t, codeLines, []string{}, ExpectErr("f(FFT) invalid 0th sample time, but int"))
}

func TestFFT3D(t *testing.T) {
	codeLines := []string{
		"FAKE( oscillator( range(timeAdd(1685714509*1000000000,'1s'), '1s', '100us'), freq(10, 1.0), freq(50, 2.0)))",
		"MAPKEY( roundTime(value(0), '500ms') )",
		"GROUPBYKEY()",
		"FFT(maxHz(60))",
		"FLATTEN()",
		"PUSHKEY('fft3d')",
		"CSV(precision(6))",
	}
	resultLines := loadLines("./test/fft3d.txt")
	runTest(t, codeLines, resultLines)
}

func TestSourceCSV(t *testing.T) {
	var codeLines, payload, resultLines []string

	codeLines = []string{
		`CSV(payload(),
			field(0, stringType(), "name"),
			field(1, datetimeType("s"), "time"),
			field(2, doubleType(), "value"),
			field(3, stringType(), "active")
		)`,
		`CSV(timeformat("s"), heading(true))`,
	}
	payload = []string{
		`temp.name,1691662156,123.456789,true`,
	}
	resultLines = []string{
		`name,time,value,active`,
		`temp.name,1691662156,123.456789,true`,
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	codeLines = []string{
		`CSV(payload(),
			field(0, stringType(), "name"),
			field(1, datetimeType("2006/01/02 15:04:05", "KST"), "time"),
			field(2, doubleType(), "value"),
			field(3, stringType(), "active")
		)`,
		`CSV(timeformat("s"), heading(true))`,
	}
	payload = []string{
		`temp.name,2023/08/10 19:09:16,123.456789,true`,
	}
	resultLines = []string{
		`name,time,value,active`,
		`temp.name,1691662156,123.456789,true`,
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	codeLines = []string{
		"CSV(payload(), header(false))",
		`MAPVALUE(2, value(2) != "VALUE" ? parseFloat(value(2))*10 : value(2))`,
		"MARKDOWN()",
	}
	payload = []string{
		`NAME,TIME,VALUE,BOOL`,
		`wave.sin,1676432361,0.000000,true`,
		`wave.cos,1676432361,1.0000000,false`,
		`wave.sin,1676432362,0.406736,true`,
		`wave.cos,1676432362,0.913546,false`,
		`wave.sin,1676432363,0.743144,true`,
	}
	resultLines = []string{
		`|column0|column1|column2|column3|`,
		`|:-----|:-----|:-----|:-----|`,
		`|NAME|TIME|VALUE|BOOL|`,
		`|wave.sin|1676432361|0.000000|true|`,
		`|wave.cos|1676432361|10.000000|false|`,
		`|wave.sin|1676432362|4.067360|true|`,
		`|wave.cos|1676432362|9.135460|false|`,
		`|wave.sin|1676432363|7.431440|true|`,
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	codeLines = []string{
		"CSV(payload(), header(true))",
		"MARKDOWN()",
	}
	payload = []string{
		`NAME,TIME,VALUE`,
		`wave.sin,1676432361,0.000000`,
		`wave.cos,1676432361,1.000000`,
		`wave.sin,1676432362,0.406736`,
		`wave.cos,1676432362,0.913546`,
		`wave.sin,1676432363,0.743144`,
	}
	resultLines = []string{
		`|NAME|TIME|VALUE|`,
		`|:-----|:-----|:-----|`,
		`|wave.sin|1676432361|0.000000|`,
		`|wave.cos|1676432361|1.000000|`,
		`|wave.sin|1676432362|0.406736|`,
		`|wave.cos|1676432362|0.913546|`,
		`|wave.sin|1676432363|0.743144|`,
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	codeLines = []string{
		"CSV(payload(), header(true))",
		"MARKDOWN()",
	}
	payload = []string{
		`NAME,TIME,VALUE`,
		`wave.sin,1676432361,0.000000`,
		`wave.cos,1676432361,1.000000`,
		`wave.sin,1676432362,0.406736`,
		`wave.cos,1676432362,0.913546`,
		`wave.sin,1676432363,0.743144`,
	}
	resultLines = []string{
		`|NAME|TIME|VALUE|`,
		`|:-----|:-----|:-----|`,
		`|wave.sin|1676432361|0.000000|`,
		`|wave.cos|1676432361|1.000000|`,
		`|wave.sin|1676432362|0.406736|`,
		`|wave.cos|1676432362|0.913546|`,
		`|wave.sin|1676432363|0.743144|`,
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	codeLines = []string{
		"CSV(payload(), ",
		"    field(0, stringType(), 'name'),",
		"    field(1, datetimeType('s'), 'time'),",
		"    field(2, doubleType(), 'value'),",
		"    header(true))",
		"MARKDOWN()",
	}
	payload = []string{
		`NAME,TIME,VALUE`,
		`wave.sin,1676432361,0.000000`,
		`wave.cos,1676432361,1.000000`,
		`wave.sin,1676432362,0.406736`,
		`wave.cos,1676432362,0.913546`,
		`wave.sin,1676432363,0.743144`,
	}
	resultLines = []string{
		`|name|time|value|`,
		`|:-----|:-----|:-----|`,
		`|wave.sin|1676432361000000000|0.000000|`,
		`|wave.cos|1676432361000000000|1.000000|`,
		`|wave.sin|1676432362000000000|0.406736|`,
		`|wave.cos|1676432362000000000|0.913546|`,
		`|wave.sin|1676432363000000000|0.743144|`,
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	codeLines = []string{
		"CSV(payload(), ",
		"    field(0, stringType(), 'name'),",
		"    field(1, datetimeType('s'), 'time'),",
		"    field(2, doubleType(), 'value'),",
		"    header(false))",
		"MARKDOWN()",
	}
	payload = []string{
		`wave.sin,1676432361,0.000000`,
		`wave.cos,1676432361,1.000000`,
		`wave.sin,1676432362,0.406736`,
		`wave.cos,1676432362,0.913546`,
		`wave.sin,1676432363,0.743144`,
	}
	resultLines = []string{
		`|name|time|value|`,
		`|:-----|:-----|:-----|`,
		`|wave.sin|1676432361000000000|0.000000|`,
		`|wave.cos|1676432361000000000|1.000000|`,
		`|wave.sin|1676432362000000000|0.406736|`,
		`|wave.cos|1676432362000000000|0.913546|`,
		`|wave.sin|1676432363000000000|0.743144|`,
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	codeLines = []string{
		"CSV(payload())",
		"MARKDOWN()",
	}
	payload = []string{
		`wave.sin,1676432361,0.000000`,
		`wave.cos,1676432361,1.000000`,
		`wave.sin,1676432362,0.406736`,
		`wave.cos,1676432362,0.913546`,
		`wave.sin,1676432363,0.743144`,
	}
	resultLines = []string{
		`|column0|column1|column2|`,
		`|:-----|:-----|:-----|`,
		`|wave.sin|1676432361|0.000000|`,
		`|wave.cos|1676432361|1.000000|`,
		`|wave.sin|1676432362|0.406736|`,
		`|wave.cos|1676432362|0.913546|`,
		`|wave.sin|1676432363|0.743144|`,
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))
}

func TestSourceCSVFile(t *testing.T) {
	f, _ := ssfs.NewServerSideFileSystem([]string{"test"})
	ssfs.SetDefault(f)
	codeLines := []string{
		`CSV(file('/iris.data'))`,
		`DROP(10)`,
		`TAKE(2)`,
		`CSV()`,
	}
	resultLines := []string{
		`5.4,3.7,1.5,0.2,Iris-setosa`,
		`4.8,3.4,1.6,0.2,Iris-setosa`,
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`CSV(file('/iris.data'))`,
		`DROP(10)`,
		`TAKE(2)`,
		`JSON(timeformat('2006-01-02 15:04:05'), tz('LOCAL'))`,
	}
	resultLines = []string{
		`/r/{"data":{"columns":\["column0","column1","column2","column3","column4"\],"types":\["string","string","string","string","string"\],"rows":\[\["5.4","3.7","1.5","0.2","Iris-setosa"\],\["4.8","3.4","1.6","0.2","Iris-setosa"\]\]},"success":true,"reason":"success","elapse":".+"}`,
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`CSV(file("/euc-jp.csv"), charset("EUC-JP"))`,
		`CSV()`,
	}
	resultLines = []string{
		`利用されてきた文字コー,1701913182,3.141592`,
	}
	runTest(t, codeLines, resultLines)
}

func TestSinkMarkdown(t *testing.T) {
	f, _ := ssfs.NewServerSideFileSystem([]string{"test"})
	ssfs.SetDefault(f)
	codeLines := []string{
		"STRING(file('/lines.txt'), separator('\\n'))",
		"MARKDOWN(true)",
	}
	resultLines := []string{}
	runTest(t, codeLines, resultLines, CompileErr("line 2: encoder 'markdown' invalid option true (bool)"))

	codeLines = []string{
		"STRING(file('/lines.txt'), separator('\\n'))",
		"PUSHKEY('test')",
		"MARKDOWN(html(true))",
	}
	resultLines = loadLines("./test/markdown_xhtml.txt")
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"STRING(file('/lines.txt'), separator('\\n'))",
		"MARKDOWN(html(false))",
	}
	resultLines = []string{
		"|STRING|",
		"|:-----|",
		"|line1|",
		"|line2|",
		"||",
		"|line4|",
	}
	runTest(t, codeLines, resultLines)
}

func TestBridgeQuerySqlite(t *testing.T) {
	err := bridge.Register(&model.BridgeDefinition{Type: model.BRIDGE_SQLITE, Name: "sqlite", Path: "file::memory:?cache=shared"})
	if err == bridge.ErrBridgeDisabled {
		return
	}
	require.Nil(t, err)

	codeLines := []string{
		"SQL(bridge('sqlite'), `select * from example`)",
		"CSV(heading(true))",
	}
	resultLines := []string{
		"id,name,age,address",
		"100,alpha,10,street-100",
		"200,bravo,20,street-200",
	}
	expectErr := ExpectErr("no such table: example")
	expectLog := ExpectLog(`no such table: example`)
	runTest(t, codeLines, resultLines, expectErr, expectLog)

	br, err := bridge.GetSqlBridge("sqlite")
	require.Nil(t, err)
	require.NotNil(t, br)
	ctx := context.TODO()
	conn, err := br.Connect(ctx)
	require.Nil(t, err)
	require.NotNil(t, conn)
	defer conn.Close()
	_, err = conn.ExecContext(ctx, `create table if not exists example (
		id INTEGER NOT NULL PRIMARY KEY, name TEXT, age TEXT, address TEXT, UNIQUE(name)
	)`)
	require.Nil(t, err)
	_, err = conn.ExecContext(ctx, `insert into example values(?, ?, ?, ?)`, 100, "alpha", "10", "street-100")
	require.Nil(t, err)
	_, err = conn.ExecContext(ctx, `insert into example values(?, ?, ?, ?)`, 200, "bravo", "20", "street-200")
	require.Nil(t, err)

	runTest(t, codeLines, resultLines)
}

func TestBridgeSqlite(t *testing.T) {
	err := bridge.Register(&model.BridgeDefinition{
		Type: model.BRIDGE_SQLITE,
		Name: "sqlite",
		Path: "file::memory:?cache=shared",
	})
	if err == bridge.ErrBridgeDisabled {
		return
	}
	require.Nil(t, err)

	codeLines := []string{
		"SQL(bridge('sqlite'), `select * from example_sql`)",
		"CSV(heading(true))",
	}
	resultLines := []string{
		"id,name,age,address",
		"100,alpha,10,street-100",
		"200,bravo,20,street-200",
	}
	expectErr := ExpectErr("no such table: example_sql")
	expectLog := ExpectLog("no such table: example_sql")
	runTest(t, codeLines, resultLines, expectErr, expectLog)

	br, err := bridge.GetSqlBridge("sqlite")
	require.Nil(t, err)
	require.NotNil(t, br)
	ctx := context.TODO()
	conn, err := br.Connect(ctx)
	require.Nil(t, err)
	require.NotNil(t, conn)
	defer conn.Close()
	_, err = conn.ExecContext(ctx, `create table if not exists example_sql (
		id INTEGER NOT NULL PRIMARY KEY,
		name TEXT,
		age INTEGER,
		address TEXT,
		weight REAL,
		memo BLOB,
		UNIQUE(name)
	)`)
	require.Nil(t, err)
	_, err = conn.ExecContext(ctx, `insert into example_sql (id, name, age, address) values(?, ?, ?, ?)`,
		100, "alpha", "10", "street-100")
	require.Nil(t, err)
	_, err = conn.ExecContext(ctx, `insert into example_sql values(?, ?, ?, ?, ?, ?)`,
		200, "bravo", 20, "street-200", 56.789, []byte{0, 1, 0xFF})
	require.Nil(t, err)

	// select all
	codeLines = []string{
		"SQL(bridge('sqlite'), `select * from example_sql`)",
		"CSV(heading(true), substituteNull('<NULL>'))",
	}
	resultLines = []string{
		"id,name,age,address,weight,memo",
		"100,alpha,10,street-100,<NULL>,",
		`200,bravo,20,street-200,56.789,\x00\x01\xFF`,
	}
	runTest(t, codeLines, resultLines)

	// update
	codeLines = []string{
		`SQL(bridge('sqlite'), 'update example_sql set weight=? where id = ?', 45.67, 100)`,
		"CSV(heading(false))",
	}
	resultLines = []string{}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"SQL(bridge('sqlite'), `select * from example_sql`)",
		"CSV(heading(true))",
	}
	resultLines = []string{
		"id,name,age,address,weight,memo",
		"100,alpha,10,street-100,45.67,",
		`200,bravo,20,street-200,56.789,\x00\x01\xFF`,
	}
	runTest(t, codeLines, resultLines)

	// delete - syntax error
	codeLines = []string{
		`SQL(bridge('sqlite'), 'delete example_sql where id = ?', 100)`,
		"CSV(heading(false))",
	}
	resultLines = []string{}
	expectErr = ExpectErr("near \"example_sql\": syntax error")
	expectLog = ExpectLog(`near "example_sql": syntax error`)
	runTest(t, codeLines, resultLines, expectErr, expectLog)

	// before delete
	codeLines = []string{
		`SQL(bridge('sqlite'), 'select count(*) from example_sql where id = ?', param('id'))`,
		"CSV(heading(false))",
	}
	resultLines = []string{"1"}
	runTest(t, codeLines, resultLines, Param{name: "id", value: "100"})

	// delete
	codeLines = []string{
		`SQL(bridge('sqlite'), 'delete from example_sql where id = ?', param('id'))`,
		"CSV(heading(false))",
	}
	resultLines = []string{}
	runTest(t, codeLines, resultLines, Param{name: "id", value: "100"})

	// after delete
	codeLines = []string{
		`SQL(bridge('sqlite'), 'select count(*) from example_sql where id = ?', param('id'))`,
		"CSV(heading(false))",
	}
	resultLines = []string{"0"}
	runTest(t, codeLines, resultLines, Param{name: "id", value: "100"})

	codeLines = []string{
		"SQL(bridge('sqlite'), `select * from example_sql`)",
		"CSV(heading(true))",
	}
	resultLines = []string{
		"id,name,age,address,weight,memo",
		`200,bravo,20,street-200,56.789,\x00\x01\xFF`,
	}
	runTest(t, codeLines, resultLines)
}

func TestQuerySql(t *testing.T) {
	var codeLines, resultLines []string
	codeLines = []string{
		`QUERY('value', between('last-10s', 'last'), from("table", "tag", "time"), dump(true))`,
		`CSV()`,
	}
	resultLines = []string{normalize(`
		SELECT time, value 
		FROM TABLE WHERE name = 'tag' 
		AND time BETWEEN 
				(SELECT MAX_TIME-10000000000 FROM V$TABLE_STAT WHERE name = 'tag') 
			AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag')
		LIMIT 0, 1000000`)}
	runTest(t, codeLines, resultLines)

	// basic
	codeLines = []string{
		`QUERY('value', from('table', 'tag'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000`)}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('val', from('table', 'tag'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, val FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000`)}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('value', from('table', 'tag'), between('last -1.0s', 'last'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000`)}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('value', from('table', 'tag'), between('last-12.0s', 'last'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-12000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000`)}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('val1', 'val2' , from('table', 'tag'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, val1, val2 FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000`)}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('(val * 0.01) altVal', 'val2', from('table', 'tag'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, (val * 0.01) altVal, val2 FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000`)}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('(val + val2/2)', from('table', 'tag'), between('last-2.34s', 'last'), limit(10, 2000), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, (val + val2/2) FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-2340000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 10, 2000`)}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('val', from('table', 'tag'), between('now -2.34s', 'now'), limit(5, 100), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, val FROM TABLE WHERE name = 'tag' AND time BETWEEN (now-2340000000) AND now LIMIT 5, 100`)}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('value', from('table', 'tag'), between(123456789000-2.34*1000000000, 123456789000), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN 121116789000 AND 123456789000 LIMIT 0, 1000000`)}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('AVG(val1+val2)', from('table', 'tag'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, AVG(val1+val2) FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000`)}
	runTest(t, codeLines, resultLines)

	// between()
	codeLines = []string{
		`QUERY( 'value', from('example', 'barn'), between('last -1h', 'last'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, value FROM EXAMPLE WHERE name = 'barn' AND time BETWEEN (SELECT MAX_TIME-3600000000000 FROM V$EXAMPLE_STAT WHERE name = 'barn') AND (SELECT MAX_TIME FROM V$EXAMPLE_STAT WHERE name = 'barn') LIMIT 0, 1000000`)}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY( 'value', from('example', 'barn'), between('last -1h23m45s', 'last'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, value FROM EXAMPLE WHERE name = 'barn' AND time BETWEEN (SELECT MAX_TIME-5025000000000 FROM V$EXAMPLE_STAT WHERE name = 'barn') AND (SELECT MAX_TIME FROM V$EXAMPLE_STAT WHERE name = 'barn') LIMIT 0, 1000000`)}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY( 'STDDEV(value)', from('example', 'barn'), between('last -1h23m45s', 'last', '10m'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT from_timestamp(round(to_timestamp(time)/600000000000)*600000000000) time, STDDEV(value) FROM EXAMPLE WHERE name = 'barn' AND time BETWEEN (SELECT MAX_TIME-5025000000000 FROM V$EXAMPLE_STAT WHERE name = 'barn') AND (SELECT MAX_TIME FROM V$EXAMPLE_STAT WHERE name = 'barn') GROUP BY time ORDER BY time LIMIT 0, 1000000`)}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY( 'STDDEV(value)', from('example', 'barn'), between(1677646906*1000000000, 'last', '1s'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT from_timestamp(round(to_timestamp(time)/1000000000)*1000000000) time, STDDEV(value) FROM EXAMPLE WHERE name = 'barn' AND time BETWEEN 1677646906000000000 AND (SELECT MAX_TIME FROM V$EXAMPLE_STAT WHERE name = 'barn') GROUP BY time ORDER BY time LIMIT 0, 1000000`)}
	runTest(t, codeLines, resultLines)

	// GroupBy time
	codeLines = []string{
		`QUERY('STDDEV(val)', from('table', 'tag'), between(123456789000 - 3.45*1000000000, 123456789000, '1ms'), limit(1, 100), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT from_timestamp(round(to_timestamp(time)/1000000)*1000000) time, STDDEV(val) FROM TABLE WHERE name = 'tag' AND time BETWEEN 120006789000 AND 123456789000 GROUP BY time ORDER BY time LIMIT 1, 100`)}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('STDDEV(val)', 'zval', from('table', 'tag'), between('last-2.34s', 'last', '0.5ms'), limit(2, 100), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT from_timestamp(round(to_timestamp(time)/500000)*500000) time, STDDEV(val), zval FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-2340000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') GROUP BY time ORDER BY time LIMIT 2, 100`)}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('STDDEV(val)', from('table', 'tag'), between('now-2.34s', 'now', '0.5ms'), limit(3, 100), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT from_timestamp(round(to_timestamp(time)/500000)*500000) time, STDDEV(val) FROM TABLE WHERE name = 'tag' AND time BETWEEN (now-2340000000) AND now GROUP BY time ORDER BY time LIMIT 3, 100`)}
	runTest(t, codeLines, resultLines)
}

func TestTengoScript(t *testing.T) {
	var codeLines, resultLines []string

	codeLines = []string{
		`SCRIPT({`,
		`	ctx := import("context")`,
		`   a := 10*2+1`,
		`   // comment`,
		`   `,
		`	ctx.yield(a)`,
		`})`,
		`SCRIPT({`,
		`	ctx := import("context")`,
		`   a := ctx.value(0)`,
		`   ctx.yield(a+1, 2, 3, 4)`,
		`})`,
		`CSV()`,
	}
	resultLines = []string{"22,2,3,4"}
	runTest(t, codeLines, resultLines)
}

func normalize(ret string) string {
	csvQuote := true
	lines := []string{}
	for _, str := range strings.Split(ret, "\n") {
		l := strings.TrimSpace(str)
		if l == "" {
			continue
		}
		lines = append(lines, l)
	}
	text := strings.Join(lines, " ")
	if csvQuote {
		return `"` + text + `"`
	} else {
		return text
	}
}

func loadLines(file string) []string {
	data, _ := os.ReadFile(file)
	r := bufio.NewReader(bytes.NewBuffer(data))
	lines := []string{}
	for {
		line, _, err := r.ReadLine()
		if err != nil {
			break
		}
		lines = append(lines, string(line))
	}
	return lines
}

func TestRecordFields(t *testing.T) {
	require.Equal(t, "EOF", tql.EofRecord.String())
	require.Equal(t, "CIRCUITBREAK", tql.BreakRecord.String())
	require.Equal(t, "BYTES", tql.NewBytesRecord([]byte{0x1, 0x2}).String())

	r := tql.NewRecord("key", nil)
	fields := r.Fields()
	require.Equal(t, "key", fields[0])

	r = tql.NewRecord("key", "value")
	require.Equal(t, []any{"key", "value"}, r.Fields())
	require.Equal(t, "K:string(key) V:string", r.String())

	r = tql.NewRecord("key", []any{"v1", "v2"})
	require.Equal(t, []any{"key", "v1", "v2"}, r.Fields())
	require.Equal(t, "K:string(key) V:string, string", r.String())

	r = tql.NewRecord("key", [][]any{{"v1", "v2"}, {"w1", "w2"}})
	require.Equal(t, []any{"key", "v1", "v2", "w1", "w2"}, r.Fields())
	require.Equal(t, "K:string(key) V:(len=2) [][]any{[0]{string, string},[1]{string, string}}", r.String())
}

func TestLoader(t *testing.T) {
	loader := tql.NewLoader([]string{"./test"})
	var task *tql.Task
	var sc *tql.Script
	var expect string
	var err error

	_, err = loader.Load(".")
	require.NotNil(t, err)
	require.Equal(t, "not found '.'", err.Error())

	_, err = loader.Load("../task_test.go")
	require.NotNil(t, err)
	require.Equal(t, "not found '../task_test.go'", err.Error())

	tick := time.Unix(0, 1692329338315327000) // 2023-08-18 03:28:58.315
	util.StandardTimeNow = func() time.Time { return tick }

	tests := []struct {
		name string
	}{
		{"TestLoader"},
		{"TestLoader_Pi"},
		{"TestLoader_qq"},
		{"TestLoader_groupbykey"},
		{"TestLoader_iris"},
		{"TestLoader_iris_setosa"},
		{"TestLoader_group"},
		{"TestLoader_simplex"},
		{"transpose_all"},
		{"transpose_all_hdr"},
		{"transpose_hdr"},
		{"transpose_nohdr"},
	}

	f, _ := ssfs.NewServerSideFileSystem([]string{"test"})
	ssfs.SetDefault(f)

	for _, tt := range tests {
		sc, err = loader.Load(fmt.Sprintf("%s.tql", tt.name))
		require.Nil(t, err)
		require.NotNil(t, sc)
		resultFile := filepath.Join(".", "test", fmt.Sprintf("%s.csv", tt.name))
		if b, err := os.ReadFile(resultFile); err != nil {
			t.Log("ERROR", err.Error())
			t.Fail()
		} else {
			expect = string(b)
			// for windows
			expect = strings.ReplaceAll(expect, "\r\n", "\n")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		w := &bytes.Buffer{}

		task = tql.NewTaskContext(ctx)
		task.SetOutputWriter(w)
		if err := task.CompileScript(sc); err != nil {
			t.Log("ERROR", err.Error())
			t.Fail()
		}
		result := task.Execute()
		require.NotNil(t, result)

		if w.String() != expect {
			t.Log("Test Case:", tt.name)
			t.Logf("EXPECT:\n%s", expect)
			t.Logf("ACTUAL:\n%s", w.String())
			t.Fail()
		}
	}
}
