package tql_test

//go:generate moq -out ./task_mock_test.go -pkg tql_test ../../../neo-spi Database Conn Rows Result Appender

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
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
		}
	}

	code := strings.Join(codeLines, "\n")
	w := &bytes.Buffer{}

	timeCtx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	doneCh := make(chan any)

	logBuf := &bytes.Buffer{}

	task := tql.NewTaskContext(timeCtx)
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
	err := task.CompileString(code)
	if compileErr != "" {
		require.NotNil(t, err)
		require.Equal(t, compileErr, err.Error())
		cancel()
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
	case <-timeCtx.Done():
		t.Log(code)
		t.Fatal("ERROR time out!!!")
		cancel()
	case <-doneCh:
		cancel()
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
			require.Equal(t, len(expect), len(resultLines))

			for n, expectLine := range expect {
				if strings.HasPrefix(expectLine, "/r/") {
					reg := regexp.MustCompile("^" + strings.TrimPrefix(expectLine, "/r/"))
					if !reg.MatchString(resultLines[n]) {
						t.Logf("Expected: %s", expectLine)
						t.Logf("Actual  : %s", resultLines[n])
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
}

func TestTimeWindow(t *testing.T) {
	var codeLines, payload, resultLines []string

	for _, agg := range []string{"avg", "sum", "first", "last", "min", "max", "rss", "rms"} {
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
		`|column0|column1|column2|`,
		`|:-----|:-----|:-----|`,
		`|NAME|TIME|VALUE|`,
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
		"BRIDGE_QUERY('sqlite', `select * from example`)",
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
