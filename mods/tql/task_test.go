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
	"github.com/machbase/neo-server/mods/util/ssfs"
)

type ExpectErr string
type ExpectLog string
type Payload string

type Param = struct {
	name  string
	value string
}

func runTest(t *testing.T, codeLines []string, expect []string, options ...any) {
	var expectErr string
	var expectLog string
	var payload []byte
	var params map[string][]string
	for _, o := range options {
		switch v := o.(type) {
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
		}
	}

	code := strings.Join(codeLines, "\n")
	w := &bytes.Buffer{}

	timeCtx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	doneCh := make(chan any)

	logBuf := &bytes.Buffer{}

	task := tql.NewTaskContext(timeCtx)
	task.SetOutputWriter(w)
	task.SetLogWriter(logBuf)
	if len(payload) > 0 {
		task.SetInputReader(bytes.NewBuffer(payload))
	}
	if len(params) > 0 {
		task.SetParams(params)
	}
	err := task.CompileString(code)
	require.Nil(t, err)

	var executeErr error
	go func() {
		executeErr = task.Execute()
		doneCh <- true
	}()

	select {
	case <-timeCtx.Done():
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
		require.Equal(t, expectLog, logString)
	} else {
		if len(logString) > 0 {
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
		require.Equal(t, strings.Join(expect, "\n"), strings.TrimSpace(result))
		require.Equal(t, "", logString)
	}
}

func TestString(t *testing.T) {
	codeLines := []string{
		`STRING("line1\nline2\n\nline4", separator("\n"))`,
		"CSV( heading(true) )",
	}
	resultLines := []string{
		"id,string",
		"1,line1",
		"2,line2",
		"3,",
		"4,line4",
	}
	runTest(t, codeLines, resultLines)

	f, _ := ssfs.NewServerSideFileSystem([]string{"test"})
	ssfs.SetDefault(f)

	codeLines = []string{
		`STRING(file("/lines.txt"), separator("\n"))`,
		"CSV( header(true) )",
	}
	runTest(t, codeLines, resultLines)
}

func TestBytes(t *testing.T) {
	codeLines := []string{
		`BYTES("line1\nline2\n\nline4", separator("\n"))`,
		"CSV( heading(true) )",
	}
	resultLines := []string{
		"id,bytes",
		`1,\x6C\x69\x6E\x65\x31`,
		`2,\x6C\x69\x6E\x65\x32`,
		`3,`,
		`4,\x6C\x69\x6E\x65\x34`,
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

func TestCsvCsv(t *testing.T) {
	codeLines := []string{
		`CSV("1,line1\n2,line2\n3,\n4,line4")`,
		"CSV( heading(true) )",
	}
	resultLines := []string{
		"C00,C01",
		"1,line1",
		"2,line2",
		"3,",
		"4,line4",
	}
	runTest(t, codeLines, resultLines)
}

func TestLinspace(t *testing.T) {
	codeLines := []string{
		"FAKE( linspace(0, 2, 3))",
		"CSV( heading(true) )",
	}
	resultLines := []string{
		"id,x",
		"1,0.000000",
		"2,1.000000",
		"3,2.000000",
	}
	runTest(t, codeLines, resultLines)
}

func TestMeshgrid(t *testing.T) {
	codeLines := []string{
		"FAKE( meshgrid(linspace(0, 2, 3), linspace(0, 2, 3)) )",
		"CSV( heading(true) )",
	}
	resultLines := []string{
		"id,x,y",
		"1,0.000000,0.000000",
		"2,0.000000,1.000000",
		"3,0.000000,2.000000",
		"4,1.000000,0.000000",
		"5,1.000000,1.000000",
		"6,1.000000,2.000000",
		"7,2.000000,0.000000",
		"8,2.000000,1.000000",
		"9,2.000000,2.000000",
	}
	runTest(t, codeLines, resultLines)
}

func TestSphere(t *testing.T) {
	codeLines := []string{
		"FAKE( sphere(4, 4) )",
		"CSV( heading(true) )",
	}
	resultLines := []string{
		"id,x,y,z",
		"1,0.000000,0.000000,1.000000",
		"2,0.707107,0.000000,0.707107",
		"3,1.000000,0.000000,0.000000",
		"4,0.707107,0.000000,-0.707107",
		"5,0.000000,0.000000,1.000000",
		"6,0.000000,0.707107,0.707107",
		"7,0.000000,1.000000,0.000000",
		"8,0.000000,0.707107,-0.707107",
		"9,-0.000000,0.000000,1.000000",
		"10,-0.707107,0.000000,0.707107",
		"11,-1.000000,0.000000,0.000000",
		"12,-0.707107,0.000000,-0.707107",
		"13,-0.000000,-0.000000,1.000000",
		"14,-0.000000,-0.707107,0.707107",
		"15,-0.000000,-1.000000,0.000000",
		"16,-0.000000,-0.707107,-0.707107",
	}
	runTest(t, codeLines, resultLines)
}

func TestScriptSource(t *testing.T) {
	codeLines := []string{
		"SCRIPT(`",
		`ctx := import("context")`,
		`for i := 0; i < 10; i++ {`,
		`  ctx.yieldKey(i, i*10)`,
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

func TestDropTake(t *testing.T) {
	codeLines := []string{
		"FAKE( linspace(0, 2, 100))",
		"DROP(50)",
		"TAKE(3)",
		"CSV()",
	}
	resultLines := []string{
		"51,1.010101",
		"52,1.030303",
		"53,1.050505",
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
		"PUSHKEY( roundTime(key(), '500ms') )",
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

func TestSourceCSV1(t *testing.T) {
	codeLines := []string{
		`CSV(payload(),
			field(0, stringType(), "name"),
			field(1, datetimeType("s"), "time"),
			field(2, doubleType(), "value"),
			field(3, stringType(), "active")
		)`,
		`CSV(timeformat("s"), heading(true))`,
	}
	payload := []string{
		`temp.name,1691662156,123.456789,true`,
	}
	resultLines := []string{
		`name,time,value,active`,
		`temp.name,1691662156,123.456789,true`,
	}
	runTest(t, codeLines, resultLines, Payload(strings.Join(payload, "\n")))
}

func TestSourceCSV2(t *testing.T) {
	codeLines := []string{
		`CSV(payload(),
			field(0, stringType(), "name"),
			field(1, datetimeType("2006/01/02 15:04:05", "KST"), "time"),
			field(2, doubleType(), "value"),
			field(3, stringType(), "active")
		)`,
		`CSV(timeformat("s"), heading(true))`,
	}
	payload := []string{
		`temp.name,2023/08/10 19:09:16,123.456789,true`,
	}
	resultLines := []string{
		`name,time,value,active`,
		`temp.name,1691662156,123.456789,true`,
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
	expectLog := ExpectLog(`[ERROR] execute error no such table: example`)
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
	expectLog := ExpectLog("[ERROR] execute error no such table: example_sql")
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
		`200,bravo,20,street-200,56.789000,\x00\x01\xFF`,
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
		"100,alpha,10,street-100,45.670000,",
		`200,bravo,20,street-200,56.789000,\x00\x01\xFF`,
	}
	runTest(t, codeLines, resultLines)

	// delete - syntax error
	codeLines = []string{
		`SQL(bridge('sqlite'), 'delete example_sql where id = ?', 100)`,
		"CSV(heading(false))",
	}
	resultLines = []string{}
	expectErr = ExpectErr("near \"example_sql\": syntax error")
	expectLog = ExpectLog(`[ERROR] execute error near "example_sql": syntax error`)
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
		`200,bravo,20,street-200,56.789000,\x00\x01\xFF`,
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
