package tql

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/tql/expression"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type CompileErr string
type ExpectErr string
type ExpectLog string
type Payload string
type MatchPrefix bool
type ExpectFunc func(t *testing.T, result string)

type Param = struct {
	name  string
	value string
}

func runTest(t *testing.T, codeLines []string, expect []string, options ...any) {
	t.Helper()
	var compileErr string
	var expectErr string
	var expectLog string
	var expectFunc ExpectFunc
	var payload []byte
	var params map[string][]string
	var matchFunc func(*testing.T, string)
	var matchPrefix bool
	var httpClient *http.Client

	for _, o := range options {
		switch v := o.(type) {
		case CompileErr:
			compileErr = string(v)
		case ExpectErr:
			expectErr = string(v)
		case ExpectLog:
			expectLog = string(v)
		case ExpectFunc:
			expectFunc = v
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
		case func(*testing.T, string):
			matchFunc = v
		case *http.Client:
			httpClient = v
		}
	}

	code := strings.Join(codeLines, "\n")
	w := &bytes.Buffer{}

	ctx, ctxCancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer ctxCancel()

	doneCh := make(chan any)

	logBuf := &bytes.Buffer{}

	task := NewTaskContext(ctx)
	task.SetOutputWriter(w)
	task.SetLogWriter(logBuf)
	task.SetConsoleLogLevel(ERROR)
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
		t.Logf("CODE:\n%s", code)
		t.Logf("LOG:\n%s", strings.TrimSpace(logBuf.String()))
		t.Fatal("ERROR time out!!!")
		ctxCancel()
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

	if expectFunc != nil {
		expectFunc(t, w.String())
		return
	}
	if expectErr == "" && expectLog == "" {
		// case success
		require.Nil(t, err)
		result := w.String()
		if matchPrefix {
			strExpect := strings.Join(expect, "\n") + "\n"
			trimResult := strings.TrimSpace(result)
			strResult := "<N/A>"
			if len(trimResult) >= len(strExpect) {
				strResult = trimResult[0:len(strExpect)]
			} else {
				strResult = trimResult
			}
			require.Equal(t, strExpect, strResult)
		} else if matchFunc != nil {
			matchFunc(t, result)
		} else {
			resultLines := strings.Split(result, "\n")
			if len(resultLines) > 0 && resultLines[len(resultLines)-1] == "" {
				// remove trailing empty line
				resultLines = resultLines[0 : len(resultLines)-1]
			}
			if len(expect) != len(resultLines) {
				t.Logf("Expect result %d lines, got %d", len(expect), len(resultLines))
				t.Logf("Expect:\n%s", strings.Join(expect, "\n"))
				t.Logf("Actual:\n%s", strings.Join(resultLines, "\n"))
				t.Fail()
				return
			}

			for n, expectLine := range expect {
				require.Equal(t, expectLine, resultLines[n], fmt.Sprintf("Expected(line#%d): %s", n, expectLine))
			}
		}
		if strings.Contains(logString, "ERROR") || (strings.Contains(logString, "WARN") && !strings.Contains(logString, "deprecated")) {
			t.Log("LOG OUTPUT:", logString)
			t.Fail()
		}
	}
}

func TestTaskCancelNotifiesLateShouldStopListener(t *testing.T) {
	task := NewTaskContext(context.Background())
	task.Cancel()

	called := make(chan struct{}, 1)
	task.AddShouldStopListener(func() {
		select {
		case called <- struct{}{}:
		default:
		}
	})

	select {
	case <-called:
		return
	case <-time.After(1 * time.Second):
		t.Fatal("late should-stop listener was not notified after task cancellation")
	}
}

func TestTaskContextCancelNotifiesLateShouldStopListener(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	task := NewTaskContext(ctx)
	cancel()

	called := make(chan struct{}, 1)
	task.AddShouldStopListener(func() {
		select {
		case called <- struct{}{}:
		default:
		}
	})

	select {
	case <-called:
		return
	case <-time.After(1 * time.Second):
		t.Fatal("late should-stop listener was not notified after context cancellation")
	}
}

func TestMapChanged(t *testing.T) {
	var codeLines, resultLines []string

	data := `FAKE(json({
		["A", 1.0],
		["A", 2.0],
		["B", 3.0],
		["B", 4.0],
		["B", 5.0],
		["C", 6.0],
		["C", 7.0],
		["D", 8.0],
		["D", 9.0]
	}))`

	data = `FAKE(json({
		["A", 1692329338, 1.0],
		["A", 1692329339, 2.0],
		["B", 1692329340, 3.0],
		["B", 1692329341, 4.0],
		["B", 1692329342, 5.0],
		["B", 1692329343, 6.0],
		["B", 1692329344, 7.0],
		["B", 1692329345, 8.0],
		["C", 1692329346, 9.0],
		["D", 1692329347, 9.1],
		["D", 1692329348, 9.2],
		["D", 1692329349, 9.3]
	}))`

	codeLines = []string{
		data,
		`MAPVALUE(1, parseTime(value(1), "s", tz("UTC")))`,
		`FILTER_CHANGED(value(0), retain(value(1), "2s"), useFirstWithLast(false))`,
		`CSV(timeformat("s"))`,
	}
	resultLines = []string{
		"A,1692329338,1",
		"B,1692329340,3",
		"D,1692329347,9.1",
		"",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		data,
		`MAPVALUE(1, parseTime(value(1), "s", tz("UTC")))`,
		`FILTER_CHANGED(value(0), retain(value(1), "2s"), useFirstWithLast(true))`,
		`CSV(timeformat("s"))`,
	}
	resultLines = []string{
		"A,1692329338,1",
		"A,1692329339,2",
		"B,1692329340,3",
		"B,1692329345,8",
		"D,1692329347,9.1",
		"D,1692329349,9.3",
		"",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		data,
		`MAPVALUE(1, parseTime(value(1), "s", tz("UTC")))`,
		`FILTER_CHANGED(value(0), useFirstWithLast(true))`,
		`CSV(timeformat("s"))`,
	}
	resultLines = []string{
		"A,1692329338,1",
		"A,1692329339,2",
		"B,1692329340,3",
		"B,1692329345,8",
		"C,1692329346,9",
		"C,1692329346,9",
		"D,1692329347,9.1",
		"D,1692329349,9.3",
		"",
	}
	runTest(t, codeLines, resultLines)

	data = `FAKE(json({
		["A", 1692329338, 1.0],
		["A", 1692329341, 2.0],
		["A", 1692329344, 2.0],
		["B", 1692329339, 1.0],
		["B", 1692329342, 2.0],
		["B", 1692329345, 1.0],
		["C", 1692329340, 1.0],
		["C", 1692329343, 1.0],
		["C", 1692329346, 1.0]
	}))`
	codeLines = []string{
		data,
		`MAPVALUE(1, parseTime(value(1), "s", tz("UTC")))`,
		`FILTER_CHANGED(strSprintf("%s.%.f", value(0),value(2)), useFirstWithLast(true))`,
		`CSV(timeformat("s"))`,
	}
	resultLines = []string{
		`A,1692329338,1`,
		`A,1692329338,1`,
		`A,1692329341,2`,
		`A,1692329344,2`,
		`B,1692329339,1`,
		`B,1692329339,1`,
		`B,1692329342,2`,
		`B,1692329342,2`,
		`B,1692329345,1`,
		`B,1692329345,1`,
		`C,1692329340,1`,
		`C,1692329346,1`,
		"",
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
		"",
	}
	runTest(t, codeLines, resultLines, httpClient)

	codeLines = []string{
		`BYTES(file("http://example.com/bytes"))`,
		`CSV(binaryformat("hex"))`,
	}
	resultLines = []string{
		`0x6f6b2e`,
		"",
	}
	runTest(t, codeLines, resultLines, httpClient)

	codeLines = []string{
		`CSV(file("http://example.com/csv"))`,
		`CSV()`,
	}
	resultLines = []string{
		`1,3.141592,true,"escaped, string",123456`,
		"",
	}
	runTest(t, codeLines, resultLines, httpClient)
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
		"",
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
		"",
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
		"",
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
		"",
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
		"",
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
		"",
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
		"",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( linspace(1000, 100, -1) )",
		"CSV(precision(5), header(true))",
	}
	resultLines = []string{"x", ""}
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
		"",
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
		"",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`STRING("temp")`,
		`SET(temp, 11)`,
		`MAPVALUE(0, 1.234)`,
		`MAPVALUE(1, $temp)`,
		`CSV()`,
	}
	resultLines = []string{
		`1.234,11`,
		"",
	}
	runTest(t, codeLines, resultLines)
}

func TestMathMarkdown(t *testing.T) {
	var codeLines, resultLines []string
	codeLines = []string{
		`FAKE( linspace(0, 1, 2))`,
		`PUSHKEY('signal.md')`,
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
		"",
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
		"",
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
		"",
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
		"",
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
		"",
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
		"",
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
		"",
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
		"",
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
		"",
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
			"",
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
			"",
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
			"",
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
		"",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( linspace(0, 2, 3))",
		"PUSHVALUE(1, value(0)*1.5, 'x1.5')",
		"PUSHVALUE(2, value(1)+10, 'add', where(value(0) != 1.0 ))",
		"CSV(precision(1), heading(true), rownum(false))",
	}
	resultLines = []string{
		"x,x1.5,add",
		"0.0,0.0,10.0",
		"1.0,1.5,NULL",
		"2.0,3.0,13.0",
		"",
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
		"",
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
		"",
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
		"",
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
		"",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( csv(`world,3.141592`) )",
		"MAPVALUE(1, parseFloat(value(1)))",
		"MAPVALUE(2, strSprintf(`hello %s, %1.2f`, value(0), value(1)))",
		"CSV()",
	}
	resultLines = []string{
		"world,3.141592,\"hello world, 3.14\"", "",
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
		"",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`FAKE( json({[1],[null],[3]}) )`,
		"MAPVALUE(0, value(0), nullValue(2))",
		"CSV()",
	}
	resultLines = []string{
		"1",
		"2",
		"3",
		"",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`FAKE( json({[1],[null],[3]}) )`,
		"MAPVALUE(0, value(0), nullValue(2))",
		"MAPVALUE(0, value(0) * 10, where( value(0) % 2 == 0) )",
		"CSV()",
	}
	resultLines = []string{
		"1",
		"20",
		"3",
		"",
	}
	runTest(t, codeLines, resultLines)
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
	conn, _ := br.Connect(t.Context())
	conn.ExecContext(t.Context(), `create table if not exists test_when (
		id INTEGER NOT NULL PRIMARY KEY,
		name TEXT,
		value INTEGER
	)`)
	conn.Close()

	var codeLines, resultLines []string

	codeLines = []string{
		`#pragma log-level=INFO`,
		`FAKE( linspace(0, 2, 2) )`,
		`PUSHVALUE(0, "msg123")`,
		`WHEN( glob("msg*", value(0)), doLog("hello", value(0), value(1)) )`,
		`INSERT(bridge("sqlite"), table("test_when"), "name", "value")`,
	}
	resultLog := ExpectLog("[INFO] hello msg123 0\n[INFO] hello msg123 2")
	runTest(t, codeLines, nil, resultLog, ExpectFunc(func(t *testing.T, result string) {
		require.True(t, gjson.Get(result, "success").Bool())
		require.Equal(t, "success", gjson.Get(result, "reason").String())
		require.Equal(t, "1 row inserted.", gjson.Get(result, "data.message").String())
	}))

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
	runTest(t, codeLines, nil, httpClient, ExpectFunc(func(t *testing.T, result string) {
		require.True(t, gjson.Get(result, "success").Bool())
		require.Equal(t, "success", gjson.Get(result, "reason").String())
		require.Equal(t, "1 row inserted.", gjson.Get(result, "data.message").String())
	}))
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
		if err := scan.Err(); err != nil {
			t.Error("failed to read request body:", err)
			t.Fail()
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
	runTest(t, codeLines, nil, httpClient, ExpectFunc(func(t *testing.T, result string) {
		require.True(t, gjson.Get(result, "success").Bool())
		require.Equal(t, "success", gjson.Get(result, "reason").String())
		require.Equal(t, "1 row inserted.", gjson.Get(result, "data.message").String())
	}))
	require.Equal(t, 2, len(notifiedValues), "notified should call 2 time, but %d", len(notifiedValues))
	require.Equal(t, "msg123,0", notifiedValues[0])
	require.Equal(t, "msg123,2", notifiedValues[1])

	codeLines = []string{
		`#pragma log-level=INFO`,
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
		`#pragma log-level=INFO`,
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

func TestArgs(t *testing.T) {
	var codeLines, resultLines []string

	codeLines = []string{
		`ARGS()`,
		`MAPVALUE(0, 'tag-1', 'name')`,
		`MAPVALUE(1, 123.4, 'value')`,
		`CSV(heading(true))`,
	}
	resultLines = []string{
		`name,value`,
		`tag-1,123.4`,
		"",
	}
	runTest(t, codeLines, resultLines)
}

func TestGroup(t *testing.T) {
	var codeLines, payload, resultLines []string

	payload = []string{
		"A,1",
		"B,3",
		"C,6",
		"",
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
		"",
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
		"",
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
		"",
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
		"",
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
		"",
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
		"",
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
		"",
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
		"",
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
		"",
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
		"",
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
		"",
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
		"",
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
		"",
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
		"",
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
		"",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	// list - list array
	codeLines = []string{
		`CSV(payload(), field(0, stringType(), "name"), field(1, doubleType(), "x"), field(2, doubleType(), "y"))`,
		`GROUP(by(value(0)), list(value(2)) )`,
		`POPKEY(0)`,
		`FLATTEN()`,
		`PUSHKEY('result')`,
		`CSV(heading(true), precision(2))`,
	}
	resultLines = []string{
		"GROUP,LIST",
		"A,0.00,1.00,2.00",
		"B,1.00,1.00,1.00",
		"C,10.00,100.00,200.00,300.00",
		"",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	// list - list array
	codeLines = []string{
		`CSV(payload(), field(0, stringType(), "name"), field(1, doubleType(), "x"), field(2, doubleType(), "y"))`,
		`GROUP(by(value(0)), list(value(2),"VALUES") )`,
		`POPKEY(0)`,
		`FLATTEN()`,
		`PUSHKEY('result')`,
		`CSV(heading(true), precision(2))`,
	}
	resultLines = []string{
		"GROUP,VALUES",
		"A,0.00,1.00,2.00",
		"B,1.00,1.00,1.00",
		"C,10.00,100.00,200.00,300.00",
		"",
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
		"",
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
		"",
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
		"",
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
		"",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	codeLines = []string{
		`CSV(payload(), field(0, timeType("s"), "time"), field(2, floatType(), "value"))`,
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
		"",
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
		"",
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
		"",
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
		"",
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))

	// src data is larger time range than timewindow.
	codeLines = []string{
		`CSV(payload(), field(0, datetimeType("s"), "time"), field(1, doubleType(), "value"))`,
		`GROUP( by( value(0), timewindow(`,
		`             time(1700256262 * 1000000000),`,
		`             time(1700256276 * 1000000000),`,
		`             period("4s"))),`,
		`      avg(value(1)),`,
		`      sum(value(1)),`,
		`      last(value(1))`,
		`)`,
		`CSV(timeformat("s"), heading(true), precision(2))`,
	}
	resultLines = []string{
		"GROUP,AVG,SUM,LAST",
		"1700256264,5.00,15.00,6.00",
		"1700256268,7.50,15.00,8.00",
		"1700256272,NULL,NULL,NULL",
		"",
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
				"",
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
				"",
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
				"",
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
				"",
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
				"",
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
				"",
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
				"",
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
				"",
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
				"",
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
				"",
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
				"",
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
				"",
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
				"",
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
				"",
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
				"",
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
		"",
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
		"",
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
		"",
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
		"",
	}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		"FAKE( linspace(0, 2, 100))",
		"DROP(0)",
		"TAKE(0)",
		"PUSHKEY('test')",
		"CSV(precision(6))",
	}
	resultLines = []string{""}
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
		"",
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
	runTest(t, codeLines, nil, ExpectFunc(func(t *testing.T, result string) {
		require.True(t, gjson.Get(result, "success").Bool())
		require.Equal(t, "success", gjson.Get(result, "reason").String())
		require.Equal(t, `["x"]`, gjson.Get(result, "data.columns").String())
		require.Equal(t, `["double"]`, gjson.Get(result, "data.types").String())
		require.Equal(t, `[[{"key":0}],[{"key":1}]]`, gjson.Get(result, "data.rows").String())
	}))

	codeLines = []string{
		"FAKE( arrange(0, 1, 1) )",
		`MAPVALUE(0, dict("key", value(0), "value") )`,
		"JSON(precision(0))",
	}
	resultLines = []string{}
	runTest(t, codeLines, resultLines, ExpectErr("dict() name \"value\" doesn't match with any value"))

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
		"FAKE( arrange(0, 1, 1) )",
		"INSERT(table('example'))",
		"JSON()",
	}
	resultLines := []string{}
	runTest(t, codeLines, resultLines, CompileErr("line 2, column 1: \"INSERT()\" is not applicable for MAP [statement: INSERT(table('example'))]"))

	codeLines = []string{
		"MAPVALUE(0, 1)",
		"SQL('select * from example')",
		"JSON()",
	}
	runTest(t, codeLines, resultLines, CompileErr("line 1, column 1: \"MAPVALUE()\" is not applicable for SRC [statement: MAPVALUE(0, 1)]"))

	codeLines = []string{
		"FAKE( arrange(0, 1, 1) )",
		"SQL('select * from example')",
	}
	runTest(t, codeLines, resultLines, CompileErr("line 2, column 1: f(SQL) sink does not allow fetch verb \"SELECT\" [statement: SQL('select * from example')]"))
}

func TestCompileErrorIsScriptErrorForSink(t *testing.T) {
	code := strings.Join([]string{
		"STRING(file('/lines.txt'), separator('\\n'))",
		"MARKDOWN(true)",
	}, "\n")

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	task := NewTaskContext(ctx)
	err := task.CompileString(code)
	require.Error(t, err)
	require.Equal(t, "line 2, column 1: encoder 'markdown' invalid option true (bool) [statement: MARKDOWN(true)]", err.Error())

	var scriptErr *ScriptError
	require.True(t, errors.As(err, &scriptErr))
	require.Equal(t, "sink_compile_error", scriptErr.Kind)
	require.Equal(t, 2, scriptErr.Span.Start.Line)
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
		LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	// basic
	codeLines = []string{
		`QUERY('value', from('table', 'tag'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('val', from('table', 'tag'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, val FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('value', from('table', 'tag'), between('last -1.0s', 'last'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('value', from('table', 'tag'), between('last-12.0s', 'last'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-12000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('val1', 'val2' , from('table', 'tag'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, val1, val2 FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('(val * 0.01) altVal', 'val2', from('table', 'tag'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, (val * 0.01) altVal, val2 FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('(val + val2/2)', from('table', 'tag'), between('last-2.34s', 'last'), limit(10, 2000), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, (val + val2/2) FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-2340000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 10, 2000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('val', from('table', 'tag'), between('now -2.34s', 'now'), limit(5, 100), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, val FROM TABLE WHERE name = 'tag' AND time BETWEEN (now-2340000000) AND now LIMIT 5, 100`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('value', from('table', 'tag'), between(123456789000-2.34*1000000000, 123456789000), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN 121116789000 AND 123456789000 LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('AVG(val1+val2)', from('table', 'tag'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, AVG(val1+val2) FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	// between()
	codeLines = []string{
		`QUERY( 'value', from('example', 'barn'), between('last -1h', 'last'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, value FROM EXAMPLE WHERE name = 'barn' AND time BETWEEN (SELECT MAX_TIME-3600000000000 FROM V$EXAMPLE_STAT WHERE name = 'barn') AND (SELECT MAX_TIME FROM V$EXAMPLE_STAT WHERE name = 'barn') LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY( 'value', from('example', 'barn'), between('last -1h23m45s', 'last'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, value FROM EXAMPLE WHERE name = 'barn' AND time BETWEEN (SELECT MAX_TIME-5025000000000 FROM V$EXAMPLE_STAT WHERE name = 'barn') AND (SELECT MAX_TIME FROM V$EXAMPLE_STAT WHERE name = 'barn') LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY( 'STDDEV(value)', from('example', 'barn'), between('last -1h23m45s', 'last', '10m'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT from_timestamp(round(to_timestamp(time)/600000000000)*600000000000) time, STDDEV(value) FROM EXAMPLE WHERE name = 'barn' AND time BETWEEN (SELECT MAX_TIME-5025000000000 FROM V$EXAMPLE_STAT WHERE name = 'barn') AND (SELECT MAX_TIME FROM V$EXAMPLE_STAT WHERE name = 'barn') GROUP BY time ORDER BY time LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY( 'STDDEV(value)', from('example', 'barn'), between(1677646906*1000000000, 'last', '1s'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT from_timestamp(round(to_timestamp(time)/1000000000)*1000000000) time, STDDEV(value) FROM EXAMPLE WHERE name = 'barn' AND time BETWEEN 1677646906000000000 AND (SELECT MAX_TIME FROM V$EXAMPLE_STAT WHERE name = 'barn') GROUP BY time ORDER BY time LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	// GroupBy time
	codeLines = []string{
		`QUERY('STDDEV(val)', from('table', 'tag'), between(123456789000 - 3.45*1000000000, 123456789000, '1ms'), limit(1, 100), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT from_timestamp(round(to_timestamp(time)/1000000)*1000000) time, STDDEV(val) FROM TABLE WHERE name = 'tag' AND time BETWEEN 120006789000 AND 123456789000 GROUP BY time ORDER BY time LIMIT 1, 100`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('STDDEV(val)', 'zval', from('table', 'tag'), between('last-2.34s', 'last', '0.5ms'), limit(2, 100), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT from_timestamp(round(to_timestamp(time)/500000)*500000) time, STDDEV(val), zval FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-2340000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') GROUP BY time ORDER BY time LIMIT 2, 100`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`QUERY('STDDEV(val)', from('table', 'tag'), between('now-2.34s', 'now', '0.5ms'), limit(3, 100), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT from_timestamp(round(to_timestamp(time)/500000)*500000) time, STDDEV(val) FROM TABLE WHERE name = 'tag' AND time BETWEEN (now-2340000000) AND now GROUP BY time ORDER BY time LIMIT 3, 100`), ""}
	runTest(t, codeLines, resultLines)
}

func TestSqlSelect(t *testing.T) {
	var codeLines, resultLines []string
	codeLines = []string{
		`SQL_SELECT('value', between('last-10s', 'last'), from("table", "tag", "time"), dump(true))`,
		`CSV()`,
	}
	resultLines = []string{normalize(`
		SELECT value 
		FROM TABLE WHERE name = 'tag' 
		AND time BETWEEN 
				(SELECT MAX_TIME-10000000000 FROM V$TABLE_STAT WHERE name = 'tag') 
			AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag')
		LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	// basic
	codeLines = []string{
		`SQL_SELECT('time', 'value', from('table', 'tag'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`SQL_SELECT('val', from('table', 'tag'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT val FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`SQL_SELECT('value', from('table', 'tag'), between('last -1.0s', 'last'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`SQL_SELECT('time', 'value', from('table', 'tag'), between('last-12.0s', 'last'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, value FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-12000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`SQL_SELECT('val1', 'val2' , from('table', 'tag'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT val1, val2 FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`SQL_SELECT('(val * 0.01) altVal', 'val2', from('table', 'tag'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT (val * 0.01) altVal, val2 FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`SQL_SELECT('(val + val2/2)', from('table', 'tag'), between('last-2.34s', 'last'), limit(10, 2000), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT (val + val2/2) FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-2340000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 10, 2000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`SQL_SELECT('time', 'val', from('table', 'tag'), between('now -2.34s', 'now'), limit(5, 100), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT time, val FROM TABLE WHERE name = 'tag' AND time BETWEEN (now-2340000000) AND now LIMIT 5, 100`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`SQL_SELECT('value', from('table', 'tag'), between(123456789000-2.34*1000000000, 123456789000), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT value FROM TABLE WHERE name = 'tag' AND time BETWEEN 121116789000 AND 123456789000 LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`SQL_SELECT('AVG(val1+val2)', from('table', 'tag'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT AVG(val1+val2) FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-1000000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	// between()
	codeLines = []string{
		`SQL_SELECT( 'value', from('example', 'barn'), between('last -1h', 'last'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT value FROM EXAMPLE WHERE name = 'barn' AND time BETWEEN (SELECT MAX_TIME-3600000000000 FROM V$EXAMPLE_STAT WHERE name = 'barn') AND (SELECT MAX_TIME FROM V$EXAMPLE_STAT WHERE name = 'barn') LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`SQL_SELECT( 'value', from('example', 'barn'), between('last -1h23m45s', 'last'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT value FROM EXAMPLE WHERE name = 'barn' AND time BETWEEN (SELECT MAX_TIME-5025000000000 FROM V$EXAMPLE_STAT WHERE name = 'barn') AND (SELECT MAX_TIME FROM V$EXAMPLE_STAT WHERE name = 'barn') LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`SQL_SELECT( 'time', 'STDDEV(value)', from('example', 'barn'), between('last -1h23m45s', 'last', '10m'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT from_timestamp(round(to_timestamp(time)/600000000000)*600000000000) time, STDDEV(value) FROM EXAMPLE WHERE name = 'barn' AND time BETWEEN (SELECT MAX_TIME-5025000000000 FROM V$EXAMPLE_STAT WHERE name = 'barn') AND (SELECT MAX_TIME FROM V$EXAMPLE_STAT WHERE name = 'barn') GROUP BY time ORDER BY time LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`SQL_SELECT( 'STDDEV(value)', from('example', 'barn'), between(1677646906*1000000000, 'last', '1s'), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT STDDEV(value) FROM EXAMPLE WHERE name = 'barn' AND time BETWEEN 1677646906000000000 AND (SELECT MAX_TIME FROM V$EXAMPLE_STAT WHERE name = 'barn') GROUP BY time ORDER BY time LIMIT 0, 1000000`), ""}
	runTest(t, codeLines, resultLines)

	// GroupBy time
	codeLines = []string{
		`SQL_SELECT('time', 'STDDEV(val)', from('table', 'tag'), between(123456789000 - 3.45*1000000000, 123456789000, '1ms'), limit(1, 100), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT from_timestamp(round(to_timestamp(time)/1000000)*1000000) time, STDDEV(val) FROM TABLE WHERE name = 'tag' AND time BETWEEN 120006789000 AND 123456789000 GROUP BY time ORDER BY time LIMIT 1, 100`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`SQL_SELECT('time', 'STDDEV(val)', 'zval', from('table', 'tag'), between('last-2.34s', 'last', '0.5ms'), limit(2, 100), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT from_timestamp(round(to_timestamp(time)/500000)*500000) time, STDDEV(val), zval FROM TABLE WHERE name = 'tag' AND time BETWEEN (SELECT MAX_TIME-2340000000 FROM V$TABLE_STAT WHERE name = 'tag') AND (SELECT MAX_TIME FROM V$TABLE_STAT WHERE name = 'tag') GROUP BY time ORDER BY time LIMIT 2, 100`), ""}
	runTest(t, codeLines, resultLines)

	codeLines = []string{
		`SQL_SELECT('STDDEV(val)', from('table', 'tag'), between('now-2.34s', 'now', '0.5ms'), limit(3, 100), dump(true))`,
		"CSV()",
	}
	resultLines = []string{normalize(`SELECT STDDEV(val) FROM TABLE WHERE name = 'tag' AND time BETWEEN (now-2340000000) AND now GROUP BY time ORDER BY time LIMIT 3, 100`), ""}
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

func TestRecordFields(t *testing.T) {
	require.Equal(t, "EOF", EofRecord.String())
	require.Equal(t, "CIRCUITBREAK", BreakRecord.String())
	require.Equal(t, "BYTES", NewBytesRecord([]byte{0x1, 0x2}).String())

	r := NewRecord("key", nil)
	fields := r.Fields()
	require.Equal(t, "key", fields[0])

	r = NewRecord("key", "value")
	require.Equal(t, []any{"key", "value"}, r.Fields())
	require.Equal(t, "K:string(key) V:string", r.String())

	r = NewRecord("key", []any{"v1", "v2"})
	require.Equal(t, []any{"key", "v1", "v2"}, r.Fields())
	require.Equal(t, "K:string(key) V:string, string", r.String())

	r = NewRecord("key", [][]any{{"v1", "v2"}, {"w1", "w2"}})
	require.Equal(t, []any{"key", "v1", "v2", "w1", "w2"}, r.Fields())
	require.Equal(t, "K:string(key) V:(len=2) [][]any{[0]{string, string},[1]{string, string}}", r.String())
}

type recordReceiver struct {
	name     string
	received *Record
}

func (r *recordReceiver) Name() string { return r.name }

func (r *recordReceiver) Receive(rec *Record) {
	r.received = rec
}

func TestRecordKindsAndArrayAccessors(t *testing.T) {
	t.Run("array record", func(t *testing.T) {
		items := []*Record{NewRecord("k", 1), NewRecord("k2", 2)}
		rec := ArrayRecord(items)

		require.True(t, rec.IsArray())
		require.False(t, rec.IsTuple())
		require.Equal(t, items, rec.Array())
		require.Equal(t, "ARRAY", rec.String())
	})

	t.Run("bytes and image records", func(t *testing.T) {
		bytesRec := NewBytesRecord([]byte("abc"))
		require.True(t, bytesRec.IsBytes())
		require.False(t, bytesRec.IsTuple())
		require.Equal(t, "BYTES", bytesRec.String())
		require.Nil(t, bytesRec.Array())

		imageRec := NewImageRecord([]byte("img"), "image/png")
		require.True(t, imageRec.IsImage())
		require.False(t, imageRec.IsTuple())
		require.Equal(t, "IMAGE", imageRec.String())
		require.Equal(t, "image/png", imageRec.contentType)
	})

	t.Run("special records", func(t *testing.T) {
		require.Equal(t, "EOF", EofRecord.String())
		require.Equal(t, "CIRCUITBREAK", BreakRecord.String())

		errRec := ErrorRecord(errors.New("boom"))
		require.True(t, errRec.IsError())
		require.EqualError(t, errRec.Error(), "boom")
		require.Equal(t, "ERROR boom", errRec.String())

		normal := NewRecord("key", 1)
		require.Nil(t, normal.Error())
		require.True(t, normal.IsTuple())
		require.Equal(t, "K:string(key) V:int", normal.String())
	})

	t.Run("nil record string", func(t *testing.T) {
		var rec *Record
		require.Equal(t, "<nil>", rec.String())
	})
}

func TestRecordVariableAndTell(t *testing.T) {
	rec := NewRecord("key", 1)
	rec.SetVariable("answer", 42)

	v, err := rec.GetVariable("$answer")
	require.NoError(t, err)
	require.Equal(t, 42, v)

	v, err = rec.GetVariable("$missing")
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = rec.GetVariable("answer")
	require.EqualError(t, err, "undefined variable 'answer'")
	require.Nil(t, v)

	v, err = NewRecord("other", 2).GetVariable("$answer")
	require.EqualError(t, err, "undefined variable '$answer'")
	require.Nil(t, v)

	rec.Tell(nil)
	rcv := &recordReceiver{name: "capture"}
	rec.Tell(rcv)
	require.Same(t, rec, rcv.received)
}

func TestRecordFlattenAndStringValueTypes(t *testing.T) {
	t.Run("flatten variants", func(t *testing.T) {
		require.Equal(t, []any{"k", 1, "two"}, NewRecord("k", []any{1, "two"}).Flatten())
		require.Equal(t, []any{"k", "value"}, NewRecord("k", "value").Flatten())
		require.Equal(t, []any{"k"}, NewRecord("k", nil).Flatten())
	})

	t.Run("string value types", func(t *testing.T) {
		rec := NewRecord("k", []any{1, "two", []any{true, 3.14, "x", 7}, 9})
		require.Equal(t, "int, string, []any{bool,float64,string,int,... (len=4)}, int, ... (len=4)", rec.StringValueTypes())

		matrix := NewRecord("k", [][]any{{1, "a"}, {true}, {3.14}, {"tail", 2}})
		require.Equal(t, "(len=4) [][]any{[0]{int, string},[1]{bool},[2]{float64},[3]{string, int}, ...}", matrix.StringValueTypes())
	})
}

func TestRecordEqualKeyAndValue(t *testing.T) {
	base := time.Date(2024, 1, 2, 3, 4, 5, 6, time.UTC)
	require.True(t, NewRecord(base, nil).EqualKey(NewRecord(base, nil)))
	require.False(t, NewRecord(base, nil).EqualKey(NewRecord(base.Add(time.Nanosecond), nil)))

	require.True(t, NewRecord([]int{1, 2}, nil).EqualKey(NewRecord([]int{1, 2}, nil)))
	require.False(t, NewRecord([]int{1, 2}, nil).EqualKey(NewRecord([]int{1, 3}, nil)))
	require.False(t, NewRecord("x", nil).EqualKey(nil))

	require.True(t, NewRecord("k", []any{1, "a"}).EqualValue(NewRecord("k", []any{1, "a"})))
	require.False(t, NewRecord("k", []any{1, "a"}).EqualValue(NewRecord("k", []any{2, "a"})))
	require.False(t, NewRecord("k", 1).EqualValue(nil))
}

func TestLoader(t *testing.T) {
	fileDirs := []string{"/=./test"}
	serverFs, _ := ssfs.NewServerSideFileSystem(fileDirs)
	ssfs.SetDefault(serverFs)

	loader := NewLoader()
	var task *Task
	var sc *Script
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

	f, _ := ssfs.NewServerSideFileSystem([]string{"/=./test"})
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
			expect = strings.ReplaceAll(expect, "\r", "") + "\n"
		}
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()

		w := &bytes.Buffer{}

		task = NewTaskContext(ctx)
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

type ReadTestCase struct {
	code   string
	expect []Line
}

func (tc ReadTestCase) run(t *testing.T) {
	t.Helper()

	buf := []string{}
	src := strings.Split(tc.code, "\n")
	for _, l := range src {
		l = strings.TrimPrefix(strings.TrimSpace(l), "|")
		buf = append(buf, l)
	}
	code := strings.Join(buf, "\n")
	reader := bytes.NewBufferString(code)
	lines, err := scanLines(reader, functions)
	if err != nil {
		t.Fail()
		return
	}
	if len(tc.expect) != len(lines) {
		t.Logf("Expect %d lines, got %d", len(tc.expect), len(lines))
		t.Log("Expect:")
		for i, l := range tc.expect {
			t.Logf("  expect[%d] %s", i, l.text)
		}
		t.Log("Actual:")
		for i, l := range lines {
			t.Logf("  actual[%d] %s", i, l.text)
		}
		t.Fail()
		return
	}
	for i := 0; i < len(tc.expect); i++ {
		e := tc.expect[i]
		l := lines[i]
		require.Equal(t, e.text, l.text)
		if e.text != l.text || e.isComment != l.isComment || e.isPragma != l.isPragma || e.line != l.line {
			t.Logf("Expect[%d]:%d %v", i, e.line, e)
			t.Logf("Actual[%d]:%d %v", i, l.line, *l)
			t.Fail()
		}
	}
	if len(lines) > len(tc.expect) {
		for i := len(tc.expect); i < len(lines); i++ {
			l := lines[i]
			t.Logf("Actual[%d]:%d %v", i, l.line, *l)
		}
		t.Fail()
	}
}

func TestReadLine(t *testing.T) {
	ReadTestCase{
		`FAKE('안녕') // comment
			|CSV()
			`,
		[]Line{
			{text: "FAKE('안녕')", line: 1},
			{text: "CSV()", line: 2},
		},
	}.run(t)
	ReadTestCase{
		`//comment1
			|FAKE('hello') // comment2
			| MAPVALUE(2,
			|  value(1) * 10, // inline comment
			|  true
			| ) // end of MAPVALUE
			|// comment3 // and
			|CSV()
			`,
		[]Line{
			{text: "comment1", isComment: true, line: 1},
			{text: "\nFAKE('hello')", line: 2},
			{text: " MAPVALUE(2,\n  value(1) * 10,\n  true\n )", line: 3},
			{text: " comment3 // and", isComment: true, line: 7},
			{text: "\nCSV()", line: 8},
		},
	}.run(t)
	ReadTestCase{
		`FAKE(meshgrid(linspace(-4,4,100), linspace(-4,4, 100)))
			|MAPVALUE(2,
			|	sin(pow(value(0), 2) + pow(value(1), 2))
			|	/
			|	(pow(value(0), 2) + pow(value(1), 2))
			|)
			|CHART_LINE3D()
			`,
		[]Line{
			{text: "FAKE(meshgrid(linspace(-4,4,100), linspace(-4,4, 100)))", line: 1},
			{text: "MAPVALUE(2,\n	sin(pow(value(0), 2) + pow(value(1), 2))\n	/\n	(pow(value(0), 2) + pow(value(1), 2))\n)", line: 2},
			{text: "CHART_LINE3D()", line: 7},
		},
	}.run(t)
	ReadTestCase{
		`FAKE(meshgrid(linspace(-4,4,100), linspace(-4,4, 100))) // comment
			|MAPVALUE()
			`,
		[]Line{
			{text: "FAKE(meshgrid(linspace(-4,4,100), linspace(-4,4, 100)))", line: 1},
			{text: "MAPVALUE()", line: 2},
		},
	}.run(t)
	ReadTestCase{
		`FAKE(meshgrid(linspace(-4,4,100 // comment
			|),
			|linspace(-4,4, 100)))
			|MAPVALUE()
			`,
		[]Line{
			{text: "FAKE(meshgrid(linspace(-4,4,100 \n),\nlinspace(-4,4, 100)))", line: 1},
			{text: "MAPVALUE()", line: 4},
		},
	}.run(t)
	ReadTestCase{
		`FAKE(meshgrid(linspace(-4,4,100),linspace(-4,4, 100)))
			|//+ stateful
			|WHEN( cond, doHttp())
			|MAPVALUE()
			`,
		[]Line{
			{text: "FAKE(meshgrid(linspace(-4,4,100),linspace(-4,4, 100)))", line: 1},
			{text: " stateful", isComment: true, isPragma: true, line: 2},
			{text: "WHEN( cond, doHttp())", line: 3},
			{text: "MAPVALUE()", line: 4},
		},
	}.run(t)
	ReadTestCase{
		`FAKE(meshgrid(linspace(-4,4,100),linspace(-4,4, 100)))
			|#pragma stateful
			|WHEN( cond, doHttp())
			|MAPVALUE()
			`,
		[]Line{
			{text: "FAKE(meshgrid(linspace(-4,4,100),linspace(-4,4, 100)))", line: 1},
			{text: " stateful", isComment: true, isPragma: true, line: 2},
			{text: "WHEN( cond, doHttp())", line: 3},
			{text: "MAPVALUE()", line: 4},
		},
	}.run(t)
	ReadTestCase{
		`SCRIPT({
			|
			|  // comment-first
			|  line2;
			|  // comment-second
			|  line4;
			|
			|}) // comment
			|CSV()
			`,
		[]Line{
			{text: "SCRIPT({\n\n  // comment-first\n  line2;\n  // comment-second\n  line4;\n\n})", line: 1},
			{text: "CSV()", line: 9},
		},
	}.run(t)
	ReadTestCase{
		`RESTCLIENT({
			|  POST http://127.0.0.1:5654/db/query
			|  Content-Type: application/json
			|
			|  {
			|    "q": "select all"
			|  }
			|}) // comment
			|TEXT()
			`,
		[]Line{
			{text: `RESTCLIENT({
  POST http://127.0.0.1:5654/db/query
  Content-Type: application/json

  {
    "q": "select all"
  }
})`, line: 1},
			{text: "TEXT()", line: 9},
		},
	}.run(t)
	ReadTestCase{
		`SCRIPT({<<JS
			|  // this is a function return '{'
			|  function a () { return '{' };
			|JS})
			|CSV()
			`,
		[]Line{
			{text: "SCRIPT({<<JS\n  // this is a function return '{'\n  function a () { return '{' };\nJS})", line: 1},
			{text: "CSV()", line: 5},
		},
	}.run(t)
	ReadTestCase{
		"MARKDOWN({<<MD\n```mermaid\nerDiagram\n    CUSTOMER ||--o{ ORDER :places\n```\nMD})\nCSV()\n",
		[]Line{
			{text: "MARKDOWN({<<MD\n```mermaid\nerDiagram\nCUSTOMER ||--o{ ORDER :places\n```\nMD})", line: 1},
			{text: "CSV()", line: 7},
		},
	}.run(t)
	ReadTestCase{
		"MARKDOWN(`<<MD\n```mermaid\nerDiagram\n    CUSTOMER ||--o{ ORDER :places\n    NOTE : `inline` text\n```\nMD`)\nCSV()\n",
		[]Line{
			{text: "MARKDOWN(`<<MD\n```mermaid\nerDiagram\nCUSTOMER ||--o{ ORDER :places\nNOTE : `inline` text\n```\nMD`)", line: 1},
			{text: "CSV()", line: 8},
		},
	}.run(t)
	ReadTestCase{
		"MARKDOWN({<<EOF\n{{ if .IsFirst }}\n```d2\n{{ end }}\nEOF}, html(true))\nCSV()\n",
		[]Line{
			{text: "MARKDOWN({<<EOF\n{{ if .IsFirst }}\n```d2\n{{ end }}\nEOF}, html(true))", line: 1},
			{text: "CSV()", line: 6},
		},
	}.run(t)
	ReadTestCase{
		"MARKDOWN({<<EOF\nsome text\n# this is not a comment but a title\n// this is not a comment either\nEOF})\nCSV()\n",
		[]Line{
			{text: "MARKDOWN({<<EOF\nsome text\n# this is not a comment but a title\n// this is not a comment either\nEOF})", line: 1},
			{text: "CSV()", line: 6},
		},
	}.run(t)
}

func TestPragma(t *testing.T) {
	tests := []struct {
		Name       string
		Script     string
		ExpectFunc func(t *testing.T, task *Task)
	}{
		{
			Name: "pragma-log-level-bang",
			Script: `
				#pragma log-level=warn
				FAKE( linspace(1, 5, 5))
				JSON()`,
			ExpectFunc: func(t *testing.T, task *Task) {
				require.Equal(t, ParseLogLevel("warn"), task.logLevel)
			},
		},
		{
			Name: "pragma-log-level",
			Script: `
				//+ log-level=trace sql-thread-lock
				SQL( 'select count(*) from example' )
				JSON()`,
			ExpectFunc: func(t *testing.T, task *Task) {
				require.Equal(t, ParseLogLevel("trace"), task.logLevel)
			},
		},
		{
			Name: "pragma-sql-thread-lock-bang",
			Script: `
				#pragma sql-thread-lock=0
				SQL( 'select count(*) from example' )
				JSON()`,
			ExpectFunc: func(t *testing.T, task *Task) {
				require.Equal(t, ParseLogLevel("error"), task.logLevel)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			task := NewTaskContext(t.Context())
			task.SetLogWriter(os.Stdout)
			if err := task.CompileString(tc.Script); err != nil {
				t.Log("ERROR:", tc.Name, err.Error())
				t.Fail()
				return
			}
			tc.ExpectFunc(t, task)
		})
	}
}

func TestParseScript(t *testing.T) {
	script, err := ParseScript("FAKE(json({\n  [1]\n}))\nMAPVALUE(0, value(0)*10)\nCSV()", nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(script.Statements) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(script.Statements))
	}
	if script.Statements[0].Name != "FAKE()" || script.Statements[0].Kind != StatementSource {
		t.Fatalf("unexpected first statement: name=%s kind=%v", script.Statements[0].Name, script.Statements[0].Kind)
	}
	if script.Statements[1].Name != "MAPVALUE()" || script.Statements[1].Kind != StatementMap {
		t.Fatalf("unexpected second statement: name=%s kind=%v", script.Statements[1].Name, script.Statements[1].Kind)
	}
	if script.Statements[2].Name != "CSV()" || script.Statements[2].Kind != StatementSourceOrSink {
		t.Fatalf("unexpected third statement: name=%s kind=%v", script.Statements[2].Name, script.Statements[2].Kind)
	}
}

func TestValidateScriptStructureValid(t *testing.T) {
	script, err := ParseScript("FAKE(json({[1]}))\nMAPVALUE(0, value(0))\nCSV()", nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if err := ValidateScriptStructure(script); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateScriptStructureInvalidSource(t *testing.T) {
	script, err := ParseScript("MAPVALUE(0, 1)\nCSV()", nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	err = ValidateScriptStructure(script)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var scriptErr *ScriptError
	if !errors.As(err, &scriptErr) {
		t.Fatalf("expected ScriptError, got %T", err)
	}
	if scriptErr.Kind != "invalid_source" {
		t.Fatalf("unexpected script error kind: %s", scriptErr.Kind)
	}
}

func TestValidateScriptStructureInvalidMap(t *testing.T) {
	script, err := ParseScript("FAKE(json({[1]}))\nINSERT(table('example'))\nCSV()", nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	err = ValidateScriptStructure(script)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var scriptErr *ScriptError
	if !errors.As(err, &scriptErr) {
		t.Fatalf("expected ScriptError, got %T", err)
	}
	if scriptErr.Kind != "invalid_map" {
		t.Fatalf("unexpected script error kind: %s", scriptErr.Kind)
	}
}

func TestValidateScriptStructureSqlAsMapAndSink(t *testing.T) {
	script, err := ParseScript("FAKE(json({[1]}))\nSQL('select 1')\nSQL('insert into example values(1)')", nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if err := ValidateScriptStructure(script); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestParseScriptKeepsCommentAndPragmaStatements(t *testing.T) {
	script, err := ParseScript("FAKE(json({[1]}))\n//+ stateful\n// comment\nCSV()", nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(script.Statements) != 4 {
		t.Fatalf("expected 4 statements, got %d", len(script.Statements))
	}
	if !script.Statements[1].IsPragma || script.Statements[1].Kind != StatementPragma {
		t.Fatalf("unexpected pragma statement: %+v", script.Statements[1])
	}
	if !script.Statements[2].IsComment || script.Statements[2].Kind != StatementComment {
		t.Fatalf("unexpected comment statement: %+v", script.Statements[2])
	}
}

func TestParseScriptMultilineStatement(t *testing.T) {
	script, err := ParseScript("FAKE(json({[1]}))\nMAPVALUE(2,\n value(1) * 10,\n true\n)\nCSV()", nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(script.Statements) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(script.Statements))
	}
	if script.Statements[1].Name != "MAPVALUE()" {
		t.Fatalf("unexpected multiline statement name: %s", script.Statements[1].Name)
	}
	if script.Statements[1].Line != 2 {
		t.Fatalf("unexpected multiline statement start line: %d", script.Statements[1].Line)
	}
}

func TestValidateScriptStructureNoSource(t *testing.T) {
	script := &TQLScript{}
	err := ValidateScriptStructure(script)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var scriptErr *ScriptError
	if !errors.As(err, &scriptErr) {
		t.Fatalf("expected ScriptError, got %T", err)
	}
	if scriptErr.Kind != "no_source" {
		t.Fatalf("unexpected script error kind: %s", scriptErr.Kind)
	}
}

func TestValidateScriptStructureNoSink(t *testing.T) {
	script, err := ParseScript("FAKE(json({[1]}))", nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	err = ValidateScriptStructure(script)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var scriptErr *ScriptError
	if !errors.As(err, &scriptErr) {
		t.Fatalf("expected ScriptError, got %T", err)
	}
	if scriptErr.Kind != "no_sink" {
		t.Fatalf("unexpected script error kind: %s", scriptErr.Kind)
	}
}

func TestValidateScriptStructureSourceOrSinkAllowedAsSource(t *testing.T) {
	script, err := ParseScript("CSV(file(\"/tmp/x.csv\"))\nTEXT()", nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if err := ValidateScriptStructure(script); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestParseScriptStatementSpanRawMatch(t *testing.T) {
	source := "FAKE(json({[1]})) // trailing\nMAPVALUE(2,\n value(1) * 10,\n true\n)\nCSV()"
	script, err := ParseScript(source, nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(script.Statements) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(script.Statements))
	}
	runes := []rune(source)

	first := script.Statements[0]
	firstRaw := first.Span.RawFrom(runes)
	if firstRaw != "FAKE(json({[1]}))" {
		t.Fatalf("unexpected first raw span: %q", firstRaw)
	}

	second := script.Statements[1]
	secondRaw := second.Span.RawFrom(runes)
	if secondRaw != second.Text {
		t.Fatalf("unexpected second raw span: %q", secondRaw)
	}
}

func TestParseScriptErrorUsesAbsoluteLineNumber(t *testing.T) {
	_, err := ParseScript("FAKE( linspace(0, 360, 50))\nMAPVALUE(1, sin((value(0)/180)*PI))\nMAPVALUE(2, cos((value(0)/180)*PI))3\nCHART()", nil)
	if err == nil {
		t.Fatal("expected parse error")
	}
	var parseErr *expression.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected ParseError, got %T", err)
	}
	if parseErr.Span.Start.Line != 3 {
		t.Fatalf("expected absolute line 3, got %d", parseErr.Span.Start.Line)
	}
	if parseErr.Near != "3" {
		t.Fatalf("expected near token 3, got %q", parseErr.Near)
	}
}

func TestParseErrorFormatsLocation(t *testing.T) {
	err := &expression.ParseError{
		Message: "unexpected token '3'",
		Near:    "3",
		Span: expression.SourceSpan{
			Start: expression.SourcePosition{Line: 3, Column: 36},
		},
	}

	formatted := err.Error()
	if formatted != "unexpected token '3' (line=3, column=36, near=\"3\")" {
		t.Fatalf("unexpected formatted error: %q", formatted)
	}
}

func TestCompileLogsAbsoluteParseErrorLocations(t *testing.T) {
	tests := []struct {
		name       string
		code       string
		expectLine int
		expectNear string
	}{
		{
			name:       "line 2 literal",
			code:       "FAKE( linspace(0, 360, 50))\nMAPVALUE(1, sin((value(0)/180)*PI))2\nCHART()",
			expectLine: 2,
			expectNear: "2",
		},
		{
			name:       "line 3 literal",
			code:       "FAKE( linspace(0, 360, 50))\nMAPVALUE(1, sin((value(0)/180)*PI))\nMAPVALUE(2, cos((value(0)/180)*PI))3\nCHART()",
			expectLine: 3,
			expectNear: "3",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			logBuf := &bytes.Buffer{}
			task := NewTaskContext(ctx)
			task.SetLogWriter(logBuf)
			task.SetConsoleLogLevel(ERROR)

			err := task.CompileString(tc.code)
			if err == nil {
				t.Fatal("expected compile error")
			}

			logOutput := logBuf.String()
			if !strings.Contains(logOutput, "Compile unexpected token '") {
				t.Fatalf("expected compile log, got %q", logOutput)
			}
			if !strings.Contains(logOutput, "line="+strconv.Itoa(tc.expectLine)) {
				t.Fatalf("expected line=%d in log, got %q", tc.expectLine, logOutput)
			}
			if !strings.Contains(logOutput, "near=\""+tc.expectNear+"\"") {
				t.Fatalf("expected near=%q in log, got %q", tc.expectNear, logOutput)
			}
		})
	}
}
