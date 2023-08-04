package tql_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/d5/tengo/v2/require"
	"github.com/machbase/neo-server/mods/bridge"
	"github.com/machbase/neo-server/mods/model"
	"github.com/machbase/neo-server/mods/tql"
)

func TestLinspace(t *testing.T) {
	codeLines := []string{
		"FAKE( linspace(0, 2, 3))",
		"CSV()",
	}
	resultLines := []string{
		"1,0.000000",
		"2,1.000000",
		"3,2.000000",
	}
	runTest(t, codeLines, resultLines)
}

func TestPushKey(t *testing.T) {
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
		"FAKE( linspace(0, 1, 3))",
		"PUSHKEY('sample')",
		"POPKEY()",
		"CSV()",
	}
	resultLines := []string{
		"1,0.000000",
		"2,0.500000",
		"3,1.000000",
	}
	runTest(t, codeLines, resultLines)
}

func TestGroupByKey(t *testing.T) {
	codeLines := []string{
		"FAKE( linspace(0, 2, 3))",
		"PUSHKEY('sample')",
		"GROUPBYKEY()",
		"FLATTEN()",
		"CSV()",
	}
	resultLines := []string{
		"sample,1,0.000000",
		"sample,2,1.000000",
		"sample,3,2.000000",
	}
	runTest(t, codeLines, resultLines)
}

func TestFFT2D(t *testing.T) {
	codeLines := []string{
		"FAKE( oscillator( range(timeAdd(1685714509*1000000000,'1s'), '1s', '100us'), freq(10, 1.0), freq(50, 2.0)))",
		"PUSHKEY('samples')",
		"GROUPBYKEY(lazy(false))",
		"FFT(minHz(0), maxHz(60))",
		"POPKEY()",
		"CSV()",
	}
	resultLines := []string{
		"1.000100,0.000000", "2.000200,0.000000", "3.000300,0.000000", "4.000400,0.000000", "5.000500,0.000000", "6.000600,0.000000", "7.000700,0.000000", "8.000800,0.000000", "9.000900,0.000000", "10.001000,1.000000",
		"11.001100,0.000000", "12.001200,0.000000", "13.001300,0.000000", "14.001400,0.000000", "15.001500,0.000001", "16.001600,0.000000", "17.001700,0.000000", "18.001800,0.000000", "19.001900,0.000000", "20.002000,0.000000",
		"21.002100,0.000001", "22.002200,0.000000", "23.002300,0.000000", "24.002400,0.000000", "25.002500,0.000000", "26.002600,0.000000", "27.002700,0.000000", "28.002800,0.000000", "29.002900,0.000000", "30.003000,0.000000",
		"31.003100,0.000000", "32.003200,0.000000", "33.003300,0.000000", "34.003400,0.000000", "35.003500,0.000000", "36.003600,0.000000", "37.003700,0.000000", "38.003800,0.000000", "39.003900,0.000000", "40.004000,0.000000",
		"41.004100,0.000000", "42.004200,0.000000", "43.004300,0.000000", "44.004400,0.000000", "45.004500,0.000000", "46.004600,0.000000", "47.004700,0.000000", "48.004800,0.000000", "49.004900,0.000000", "50.005001,2.000000",
		"51.005101,0.000000", "52.005201,0.000000", "53.005301,0.000004", "54.005401,0.000000", "55.005501,0.000000", "56.005601,0.000000", "57.005701,0.000000", "58.005801,0.000000", "59.005901,0.000000",
	}
	runTest(t, codeLines, resultLines)
}

func TestFFT3D(t *testing.T) {
	codeLines := []string{
		"FAKE( oscillator( range(timeAdd(1685714509*1000000000,'1s'), '1s', '100us'), freq(10, 1.0), freq(50, 2.0)))",
		"PUSHKEY( roundTime(K, '500ms') )",
		"GROUPBYKEY()",
		"FFT(maxHz(60))",
		"CSV()",
	}
	resultLines := []string{
		"1685714510000000000,2.000400,0.000000", "1685714510000000000,4.000800,0.000000", "1685714510000000000,6.001200,0.000000",
		"1685714510000000000,8.001600,0.000000", "1685714510000000000,10.002000,1.000000", "1685714510000000000,12.002400,0.000001",
		"1685714510000000000,14.002801,0.000001", "1685714510000000000,16.003201,0.000000", "1685714510000000000,18.003601,0.000000",
		"1685714510000000000,20.004001,0.000000", "1685714510000000000,22.004401,0.000000", "1685714510000000000,24.004801,0.000000",
		"1685714510000000000,26.005201,0.000000", "1685714510000000000,28.005601,0.000000", "1685714510000000000,30.006001,0.000000",
		"1685714510000000000,32.006401,0.000000", "1685714510000000000,34.006801,0.000000", "1685714510000000000,36.007201,0.000001",
		"1685714510000000000,38.007602,0.000000", "1685714510000000000,40.008002,0.000000", "1685714510000000000,42.008402,0.000000",
		"1685714510000000000,44.008802,0.000000", "1685714510000000000,46.009202,0.000000", "1685714510000000000,48.009602,0.000000",
		"1685714510000000000,50.010002,2.000000", "1685714510000000000,52.010402,0.000002", "1685714510000000000,54.010802,0.000002",
		"1685714510000000000,56.011202,0.000001", "1685714510000000000,58.011602,0.000000", "1685714510500000000,2.000400,0.000000",
		"1685714510500000000,4.000800,0.000000", "1685714510500000000,6.001200,0.000000", "1685714510500000000,8.001600,0.000000",
		"1685714510500000000,10.002000,1.000000", "1685714510500000000,12.002400,0.000000", "1685714510500000000,14.002801,0.000000",
		"1685714510500000000,16.003201,0.000001", "1685714510500000000,18.003601,0.000000", "1685714510500000000,20.004001,0.000001",
		"1685714510500000000,22.004401,0.000000", "1685714510500000000,24.004801,0.000000", "1685714510500000000,26.005201,0.000000",
		"1685714510500000000,28.005601,0.000000", "1685714510500000000,30.006001,0.000000", "1685714510500000000,32.006401,0.000000",
		"1685714510500000000,34.006801,0.000001", "1685714510500000000,36.007201,0.000000", "1685714510500000000,38.007602,0.000000",
		"1685714510500000000,40.008002,0.000000", "1685714510500000000,42.008402,0.000000", "1685714510500000000,44.008802,0.000000",
		"1685714510500000000,46.009202,0.000000", "1685714510500000000,48.009602,0.000000", "1685714510500000000,50.010002,2.000000",
		"1685714510500000000,52.010402,0.000002", "1685714510500000000,54.010802,0.000002", "1685714510500000000,56.011202,0.000001",
		"1685714510500000000,58.011602,0.000000",
	}
	runTest(t, codeLines, resultLines)
}

func TestBridgeQuerySqlite(t *testing.T) {
	err := bridge.Register(&model.BridgeDefinition{Type: model.BRIDGE_SQLITE, Name: "sqlite", Path: "file::memory:?cache=shared"})
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
	runTest(t, codeLines, resultLines, "no such table: example")

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

func runTest(t *testing.T, codeLines []string, expect []string, expectErr ...string) {
	code := strings.Join(codeLines, "\n")
	w := &bytes.Buffer{}

	timeCtx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	doneCh := make(chan any)

	task := tql.NewTaskContext(timeCtx)
	task.SetOutputWriter(w)
	err := task.CompileString(code)
	require.Nil(t, err)

	var executeErr error
	go func() {
		executeErr = task.Execute(nil)
		doneCh <- true
	}()

	select {
	case <-timeCtx.Done():
		t.Fatal("ERROR time out!!!")
		cancel()
	case <-doneCh:
		cancel()
	}
	if len(expectErr) == 1 {
		// case error
		require.NotNil(t, executeErr)
		require.Equal(t, expectErr[0], executeErr.Error())
	} else {
		// case success
		require.Nil(t, err)
		result := w.String()
		require.Equal(t, strings.Join(expect, "\n"), strings.TrimSpace(result))
	}
}
