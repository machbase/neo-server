package tql_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/d5/tengo/v2/require"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/tql"
)

func TestFFTChain(t *testing.T) {
	strExprs := []string{
		"FAKE( oscillator( range(timeAdd(1685714509*1000000000,'1s'), '1s', '100us'), freq(10, 1.0), freq(50, 2.0)))",
		"PUSHKEY('samples')",
		"GROUPBYKEY(lazy(false))",
		"FFT(minHz(0), maxHz(60))",
		"POPKEY()",
		"CSV()",
	}
	reader := strings.NewReader(strings.Join(strExprs, "\n"))
	output, _ := stream.NewOutputStream("-")

	timeCtx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	doneCh := make(chan bool)

	go func() {
		task := tql.NewTaskContext(timeCtx)
		task.SetOutputWriter(output)
		err := task.Compile(reader)
		require.Nil(t, err)
		err = task.Execute(nil)
		require.Nil(t, err)
		doneCh <- true
	}()

	select {
	case <-timeCtx.Done():
		t.Fatal("time out!!!")
		cancel()
	case <-doneCh:
		fmt.Println("done")
		cancel()
	}
}

func TestLinspace2(t *testing.T) {
	codeLines := []string{
		"FAKE( linspace(0, 1, 2))",
		"CSV()",
	}
	resultLines := []string{
		"1,0.000000",
		"2,1.000000",
	}
	runTest(t, codeLines, resultLines)
}

func TestLinspaceMonad(t *testing.T) {
	codeLines := []string{
		"FAKE( linspace(0, 1, 2))",
		"PUSHKEY('sample')",
		"CSV()",
	}
	resultLines := []string{
		"sample,1,0.000000",
		"sample,2,1.000000",
	}
	runTest(t, codeLines, resultLines)
}

func TestPushAndPopMonad(t *testing.T) {
	codeLines := []string{
		"FAKE( linspace(0, 1, 2))",
		"PUSHKEY('sample')",
		"POPKEY()",
		"CSV()",
	}
	resultLines := []string{
		"1,0.000000",
		"2,1.000000",
	}
	runTest(t, codeLines, resultLines)
}

/* FIXME:!!
func TestGroupByKey(t *testing.T) {
	codeLines := []string{
		"FAKE( linspace(0, 2, 3))",
		"PUSHKEY('sample')",
		"GROUPBYKEY()",
		"FLATTEN()",
		"CSV()",
	}
	resultLines := []string{
		"1,0.000000",
		"2,1.000000",
	}
	runTest(t, codeLines, resultLines)
}
*/

func runTest(t *testing.T, codeLines []string, expect []string) {
	code := strings.Join(codeLines, "\n")

	timeCtx, cancel := context.WithTimeout(context.TODO(), 15*time.Second)
	defer cancel()
	doneCh := make(chan any)

	out := &bytes.Buffer{}

	task := tql.NewTaskContext(timeCtx)
	task.SetOutputWriter(out)
	err := task.CompileString(code)
	require.Nil(t, err)

	go func() {
		err = task.Execute(nil)
		require.Nil(t, err)
		doneCh <- true
	}()

	select {
	case <-timeCtx.Done():
		t.Fatal("ERROR time out!!!")
	case <-doneCh:
	}
	result := out.String()
	require.Equal(t, strings.Join(expect, "\n"), strings.TrimSpace(result))
}
