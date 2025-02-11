package tql_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/testsuite"
	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/server"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

var testServer *testsuite.Server
var testHttpAddress string

func TestMain(m *testing.M) {
	testServer = testsuite.NewServer("./test/tmp")
	testServer.StartServer(m)
	testServer.CreateTestTables()

	db := testServer.DatabaseSVR()

	f, _ := ssfs.NewServerSideFileSystem([]string{"/=test"})
	ssfs.SetDefault(f)

	http, err := server.NewHttp(db,
		server.WithHttpListenAddress("tcp://127.0.0.1:0"),
	)
	if err != nil {
		panic(err)
	}
	if err := http.Start(); err != nil {
		panic(err)
	}
	testHttpAddress = http.AdvertiseAddress()
	if testHttpAddress == "" {
		panic("http server address is empty")
	}

	code := m.Run()

	http.Stop()
	testServer.DropTestTables()
	testServer.StopServer(m)
	os.Exit(code)
}

func flushTable(table string) error {
	conn, err := testServer.DatabaseSVR().Connect(context.TODO(), api.WithPassword("sys", "manager"))
	if err != nil {
		return err
	}
	result := conn.Exec(context.TODO(), fmt.Sprintf("EXEC TABLE_FLUSH('%s')", table))
	if result.Err() != nil {
		return result.Err()
	}
	conn.Close()
	return nil
}

type VolatileFileWriterMock struct {
	name     string
	deadline time.Time
	buff     bytes.Buffer
}

func (v *VolatileFileWriterMock) VolatileFilePrefix() string { return "/web/api/tql-assets/" }

func (v *VolatileFileWriterMock) VolatileFileWrite(name string, data []byte, deadline time.Time) fs.File {
	v.buff.Write(data)
	v.name = name
	v.deadline = deadline
	return nil
}

type TqlTestCase struct {
	Name               string
	Script             string
	Payload            string
	Params             map[string][]string
	ExpectErr          string
	ExpectCSV          []string
	ExpectText         []string
	ExpectFunc         func(t *testing.T, result string)
	ExpectVolatileFile func(t *testing.T, mock *VolatileFileWriterMock)
	RunCondition       func() bool
}

func runTestCase(t *testing.T, tc TqlTestCase) {
	t.Helper()
	if tc.RunCondition != nil && !tc.RunCondition() {
		t.Skip("Skip by tc.RunCondition")
		return
	}

	memMock := &VolatileFileWriterMock{}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	output := &bytes.Buffer{}
	task := tql.NewTaskContext(ctx)
	task.SetDatabase(testServer.DatabaseSVR())
	task.SetLogWriter(os.Stdout)
	task.SetOutputWriterJson(output, true)
	task.SetVolatileAssetsProvider(memMock)
	if tc.Payload != "" {
		task.SetInputReader(bytes.NewBufferString(tc.Payload))
	}
	if len(tc.Params) > 0 {
		task.SetParams(tc.Params)
	}
	if err := task.CompileString(tc.Script); err != nil {
		t.Log("ERROR:", tc.Name, err.Error())
		t.Fail()
		return
	}
	result := task.Execute()
	if tc.ExpectErr != "" {
		require.Error(t, result.Err)
		require.Equal(t, tc.ExpectErr, result.Err.Error())
		return
	}
	if result.Err != nil {
		t.Log("ERROR:", tc.Name, result.Err.Error())
		t.Fail()
		return
	}

	switch task.OutputContentType() {
	case "text/plain",
		"text/csv; charset=utf-8",
		"text/markdown",
		"application/xhtml+xml",
		"application/json",
		"application/x-ndjson":
		outputText := output.String()
		if outputText == "" && result.IsDbSink {
			if v, err := json.Marshal(result); err == nil {
				outputText = string(v)
			} else {
				outputText = "ERROR: failed to marshal result"
			}
		}
		if tc.ExpectFunc != nil {
			tc.ExpectFunc(t, outputText)
		} else if len(tc.ExpectCSV) > 0 {
			require.Equal(t, strings.Join(tc.ExpectCSV, "\n"), outputText)
		} else if len(tc.ExpectText) > 0 {
			require.Equal(t, strings.Join(tc.ExpectText, "\n"), outputText)
		} else {
			t.Fatalf("unhandled output %q: %s", task.OutputContentType(), outputText)
		}
		if tc.ExpectVolatileFile != nil {
			tc.ExpectVolatileFile(t, memMock)
		}
	default:
		t.Fatal("ERROR:", tc.Name, "unexpected content type:", task.OutputContentType())
	}
}

func TestDatabaseTql(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "SQL_show-tables",
			Script: `
				SQL("show tables;")
				CSV(header(true))
				`,
			ExpectCSV: []string{
				"DATABASE_NAME,USER_NAME,TABLE_NAME,TABLE_ID,TABLE_TYPE,TABLE_FLAG",
				"MACHBASEDB,SYS,LOG_DATA,13,Log,NULL",
				"MACHBASEDB,SYS,TAG_DATA,6,Tag,NULL",
				"MACHBASEDB,SYS,TAG_SIMPLE,12,Tag,NULL",
				"\n",
			},
		},
		{
			Name: "SQL_show-indexes",
			Script: `
				SQL("show indexes ")
				CSV(header(true))
				`,
			ExpectCSV: []string{
				"DATABASE_NAME,USER_NAME,TABLE_NAME,COLUMN_NAME,INDEX_NAME,INDEX_TYPE,INDEX_ID",
				"MACHBASEDB,SYS,_TAG_DATA_META,_ID,__PK_IDX__TAG_DATA_META_1,REDBLACK,3",
				"MACHBASEDB,SYS,_TAG_DATA_META,NAME,_TAG_DATA_META_NAME,REDBLACK,4",
				"MACHBASEDB,SYS,_TAG_SIMPLE_META,_ID,__PK_IDX__TAG_SIMPLE_META_1,REDBLACK,9",
				"MACHBASEDB,SYS,_TAG_SIMPLE_META,NAME,_TAG_SIMPLE_META_NAME,REDBLACK,10",
				"\n",
			},
		},
		{
			Name: "SQL_desc-table",
			Script: `
				SQL("desc tag_data;")
				CSV(header(true))
				`,
			ExpectCSV: []string{
				"COLUMN,TYPE,LENGTH,FLAG,INDEX",
				"NAME,varchar,100,tag name,",
				"TIME,datetime,31,basetime,",
				"VALUE,double,17,summarized,",
				"SHORT_VALUE,short,6,,",
				"USHORT_VALUE,ushort,5,,",
				"INT_VALUE,integer,11,,",
				"UINT_VALUE,uinteger,10,,",
				"LONG_VALUE,long,20,,",
				"ULONG_VALUE,ulong,20,,",
				"STR_VALUE,varchar,400,,",
				"JSON_VALUE,json,32767,,",
				"IPV4_VALUE,ipv4,15,,",
				"IPV6_VALUE,ipv6,45,,",
				"\n",
			},
		},
		{
			Name: "SQL_insert-tag1",
			Script: `
			CSV("tag1,1692686707380411000,0.100\ntag1,1692686708380411000,0.200\n",
				header(false),
				field(0, stringType(), "name"),
				field(1, datetimeType("ns"), "time"),
				field(2, doubleType(), "value")
			)
			INSERT('name', 'time', 'value', table('tag_simple'))
			`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool())
				require.Equal(t, "success", gjson.Get(result, "reason").String())
				require.Equal(t, `{"message":"2 rows inserted."}`, gjson.Get(result, "data").Raw)
				require.NoError(t, flushTable("tag_simple"))
			},
		},
		{
			Name: "SQL_show-tags",
			Script: `
				SQL("show tags tag_simple")
				CSV(header(true))
				`,
			ExpectCSV: []string{
				"_ID,NAME,ROW_COUNT,MIN_TIME,MAX_TIME,RECENT_ROW_TIME,MIN_VALUE,MIN_VALUE_TIME,MAX_VALUE,MAX_VALUE_TIME",
				"1,tag1,2,1692686707380411000,1692686708380411000,1692686708380411000,NULL,NULL,NULL,NULL",
				"\n",
			},
		},
		{
			Name: "SQL_explain-select",
			Script: `
				SQL("explain select * from tag_simple where name = 'tag1'")
				CSV(header(true))
				`,
			ExpectCSV: []string{
				"",
				`" PROJECT"`,
				`"  TAG READ (RAW)"`,
				`"   KEYVALUE INDEX SCAN (_TAG_SIMPLE_DATA_0)"`,
				`"    [KEY RANGE]"`,
				`"     * IN ()"`,
				`"   VOLATILE INDEX SCAN (_TAG_SIMPLE_META)"`,
				`"    [KEY RANGE]"`,
				`"     * name = 'tag1'"`,
				"", "", "",
			},
			RunCondition: func() bool {
				// FIXME: This test is not working on macOS
				//        because of EXPLAIN does not include the name compare part on macOS.
				// `"    [KEY RANGE]"`,
				// `"     * name = 'tag1'"`,
				return runtime.GOOS != "darwin"
			},
		},
		{
			Name: "SQL_select-from-table",
			Script: `
				SQL("select time, value from tag_simple where name = 'tag1'")
				CSV( precision(3), header(true) )
				`,
			ExpectCSV: []string{
				"TIME,VALUE",
				"1692686707380411000,0.100",
				"1692686708380411000,0.200",
				"\n",
			},
		},
		{
			Name: "SQL_select-from-table-rownum",
			Script: `
				SQL("select time, value from tag_simple where name = 'tag1'")
				PUSHKEY('test')
				CSV( precision(3), header(true) )
				`,
			ExpectCSV: []string{
				"ROWNUM,TIME,VALUE",
				"1,1692686707380411000,0.100",
				"2,1692686708380411000,0.200",
				"\n",
			},
		},
		{
			Name: "SQL_create-tag-table",
			Script: `
				SQL({create tag table if not exists tag_simple(
					name varchar(40) primary key, time datetime basetime, value double summarized )})
				MARKDOWN(html(true), rownum(true), heading(true), brief(true))
				`,
			ExpectText: loadLines("./test/sql_ddl_executed.txt"),
		},
		{
			Name: "QUERY_CSV",
			Script: `
				QUERY('value', from('tag_simple', 'tag1', "time"), between(1692686707000000000, 1692686709000000000))
				CSV( precision(3), header(true) )
				`,
			ExpectCSV: []string{
				"TIME,VALUE",
				"1692686707380411000,0.100",
				"1692686708380411000,0.200",
				"\n",
			},
		},
		{
			Name: "QUERY_JSON-rows-flatten",
			Script: `
				QUERY('value', from('tag_simple', 'tag1', "time"), between(1692686707000000000, 1692686709000000000))
				JSON( precision(3), rowsFlatten(true) )
				`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool())
				require.Equal(t, `["TIME","VALUE"]`, gjson.Get(result, "data.columns").Raw)
				require.Equal(t, `["datetime","double"]`, gjson.Get(result, "data.types").Raw)
				require.Equal(t, `[1692686707380411000,0.1,1692686708380411000,0.2]`, gjson.Get(result, "data.rows").Raw)
			},
		},
		{
			Name: "QUERY_JSON-rows-flatten-rownum",
			Script: `
				QUERY('value', from('tag_simple', 'tag1', "time"), between(1692686707000000000, 1692686709000000000))
				JSON( precision(3), rowsFlatten(true), rownum(true) )
				`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool())
				require.Equal(t, `["ROWNUM","TIME","VALUE"]`, gjson.Get(result, "data.columns").Raw)
				require.Equal(t, `["int64","datetime","double"]`, gjson.Get(result, "data.types").Raw)
				require.Equal(t, `[1,1692686707380411000,0.1,2,1692686708380411000,0.2]`, gjson.Get(result, "data.rows").Raw)
			},
		},
		{
			Name: "SQL_NDJSON",
			Script: `
				SQL("select time, value from tag_simple where name = 'tag1'")
				NDJSON( timeformat('default'), tz('UTC') )
				`,
			ExpectText: []string{
				`{"TIME":"2023-08-22 06:45:07.38","VALUE":0.1}`,
				`{"TIME":"2023-08-22 06:45:08.38","VALUE":0.2}`,
				"\n",
			},
		},
		{
			Name: "FAKE_INSERT",
			Script: `
				FAKE( linspace(0, 1, 3) )
				PUSHVALUE(0, timeAdd('now', value(0)*2000000000))
				INSERT('time', 'value', table('tag_simple'), tag('signal.3'))
				`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool(), "result: %q", result)
				require.Equal(t, "success", gjson.Get(result, "reason").String(), result)
				require.Equal(t, `{"message":"3 rows inserted."}`, gjson.Get(result, "data").Raw, result)
				require.NoError(t, flushTable("tag_simple"))
			},
		},
		{
			Name: "FAKE_INSERT-cleanup",
			Script: `
				SQL("delete from tag_simple where name = 'signal.3'")
				MARKDOWN()
				`,
			ExpectText: []string{
				`|MESSAGE|`,
				`|:-----|`,
				`|3 rows deleted.|`,
				``,
			},
		},
		{
			Name: "FAKE_APPEND",
			Script: `
				FAKE( linspace(0, 1, 3) )
				PUSHVALUE(0, timeAdd('now', value(0)*2000000000))
				PUSHVALUE(0, 'signal.append')
				APPEND( table('tag_simple') )
				`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool(), "result: %q", result)
				require.Equal(t, "success", gjson.Get(result, "reason").String(), result)
				require.Equal(t, `{"message":"append 3 rows (success 3, fail 0)"}`, gjson.Get(result, "data").Raw, result)
				require.NoError(t, flushTable("tag_simple"))
			},
		},
		{
			Name: "FAKE_APPEND-cleanup",
			Script: `
				SQL("delete from tag_simple where name = 'signal.append'")
				MARKDOWN()
				`,
			ExpectText: []string{
				`|MESSAGE|`,
				`|:-----|`,
				`|3 rows deleted.|`,
				``,
			},
		},
		{
			Name: "js-request-json",
			Script: fmt.Sprintf(`
				SCRIPT("js", {
					$.result = {
						columns: ["NAME", "TIME", "VALUE"],
						types : ["string", "datetime", "double"]
					};
				},{
					$.request("%s/db/query?q="+
						encodeURIComponent("select name, time, value from tag_simple limit 2"), {method: 'GET'})
						.do(function(rsp) {
							rsp.text(function(body){
								obj = JSON.parse(body);
								obj.data.rows.forEach(function(r){
									$.yield(r[0], r[1], r[2]);
								})
							})
						})
					})
				JSON(timeformat("s"))
			`, testHttpAddress),
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool(), "result: %q", result)
				require.Equal(t, `["NAME","TIME","VALUE"]`, gjson.Get(result, "data.columns").Raw, result)
				require.Equal(t, `["string","datetime","double"]`, gjson.Get(result, "data.types").Raw, result)
				require.Equal(t, `[["tag1",1692686707380411000,0.1],["tag1",1692686708380411000,0.2]]`, gjson.Get(result, "data.rows").Raw, result)
			},
		},
		{
			Name: "js-request-csv",
			Script: fmt.Sprintf(`
				SCRIPT("js", {
					$.result = {
						columns: ["NAME", "TIME", "VALUE"],
						types : ["string", "datetime", "double"]
					};
				},{
					$.request("%s/db/query?q="+
							encodeURIComponent("select name, time, value from tag_simple limit 2")+"&format=csv&header=skip", 
							{method: 'GET'}
						).do(function(rsp) {
							rsp.csv(function(r){
								$.yield(r[0], parseInt(r[1]), parseFloat(r[2]));
							})
						})
					})
				JSON(timeformat("s"))
			`, testHttpAddress),
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool(), "result: %q", result)
				require.Equal(t, `["NAME","TIME","VALUE"]`, gjson.Get(result, "data.columns").Raw, result)
				require.Equal(t, `["string","datetime","double"]`, gjson.Get(result, "data.types").Raw, result)
				require.Equal(t, `[["tag1",1692686707380411000,0.1],["tag1",1692686708380411000,0.2]]`, gjson.Get(result, "data.rows").Raw, result)
			},
		},
		{
			Name: "create-table",
			Script: `
				SCRIPT("js", {
					var ret = $.db().exec("create tag table js_tag (name varchar(40) primary key, time datetime basetime, value double)");
					if (ret instanceof Error) {
						$.yield(ret.message);
					} else {
						$.yield("create-table done");
					}
				})
				CSV()`,
			RunCondition: func() bool {
				// FIXME: This test is failing randomly on Windows
				return runtime.GOOS != "windows"
			},
			ExpectFunc: func(t *testing.T, result string) {
				require.Equal(t, "create-table done\n\n", result)
			},
		},
		{
			Name: "select-value",
			Script: `
				SCRIPT("js", {
					var tick = 1731900710328594958;
					for (i = 0; i < 10; i++) {
						tick += 1000000000; // add 1 second
						var ret = $.db().exec("insert into js_tag values('test-script', ?, ?)", tick, 1.23 * i);
						if (ret instanceof Error) {
							console.error(ret.message);
						}
					}
					$.yield("done");
				})
				SCRIPT("js", {
					$.result = {
						columns: ["name", "time", "value"],
						types: ["varchar", "datetime", "double"],
					}
				},{
					$.db().query("select * from js_tag").forEach(function(row) {
						$.yield(row[0], row[1], row[2]);
					});
				})
				CSV(header(true))
				`,
			ExpectCSV: []string{
				"name,time,value",
				"test-script,1731900711328594944,0",
				"test-script,1731900712328594944,1.23",
				"test-script,1731900713328594944,2.46",
				"test-script,1731900714328594944,3.69",
				"test-script,1731900715328594944,4.92",
				"test-script,1731900716328594944,6.15",
				"test-script,1731900717328594944,7.38",
				"test-script,1731900718328594944,8.61",
				"test-script,1731900719328594944,9.84",
				"test-script,1731900720328594944,11.07",
				"",
				"",
			},
			RunCondition: func() bool {
				// FIXME: 'create-table' test is failing randomly on Windows
				return runtime.GOOS != "windows"
			},
		},
		{
			Name: "select-value",
			Script: `
				SCRIPT("js", {
					$.db().query("select * from js_tag").yield();
				})
				CSV(header(true))
				`,
			ExpectCSV: []string{
				"NAME,TIME,VALUE",
				"test-script,1731900711328594944,0",
				"test-script,1731900712328594944,1.23",
				"test-script,1731900713328594944,2.46",
				"test-script,1731900714328594944,3.69",
				"test-script,1731900715328594944,4.92",
				"test-script,1731900716328594944,6.15",
				"test-script,1731900717328594944,7.38",
				"test-script,1731900718328594944,8.61",
				"test-script,1731900719328594944,9.84",
				"test-script,1731900720328594944,11.07",
				"",
				"",
			},
			RunCondition: func() bool {
				// FIXME: 'create-table' test is failing randomly on Windows
				return runtime.GOOS != "windows"
			},
		},
		{
			Name: "drop-table",
			Script: `
				SCRIPT("js", {
					var ret = $.db().exec("drop table js_tag");
					if (ret instanceof Error) {
						console.error(ret.message);
					}
				})
				DISCARD()`,
			RunCondition: func() bool {
				// FIXME: 'create-table' test is failing randomly on Windows
				return runtime.GOOS != "windows"
			},
			ExpectFunc: func(t *testing.T, result string) {
				require.Empty(t, result)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestTql(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "SHELL_shell-command",
			Script: `
				FAKE( once(1) )
				SHELL("echo 'Hello, World!'; echo 123;")
				CSV()
				`,
			ExpectCSV: []string{`"Hello, World!"`, "123", "", "", ""},
			RunCondition: func() bool {
				// FIXME: This test is not working on Windows
				return runtime.GOOS != "windows"
			},
		},
		{
			Name: "CSV_CSV",
			Script: `
				CSV("1,line1\n2,line2\n3,\n4,line4")
				CSV( heading(true) )
				`,
			ExpectCSV: []string{
				"column0,column1",
				"1,line1",
				"2,line2",
				"3,",
				"4,line4",
				"\n",
			},
		},
		{
			Name: "CSV_CSV_single_column",
			Script: `
				CSV("line1\nline2\n\nline4")
				CSV( heading(true) )
				`,
			ExpectCSV: []string{
				"column0",
				"line1",
				"line2",
				"line4",
				"\n",
			},
		},
		{
			Name: "CSV_payload_CSV",
			Script: `
				CSV(payload(),
					field(0, stringType(), "name"),
					field(1, datetimeType("s"), "time"),
					field(2, doubleType(), "value"),
					field(3, stringType(), "active")
				)
				CSV(timeformat("s"), heading(true))
				`,
			Payload: `temp.name,1691662156,123.456789,true` + "\n",
			ExpectCSV: []string{
				`name,time,value,active`,
				`temp.name,1691662156,123.456789,true`,
				"\n",
			},
		},
		{
			Name: "CSV_payload_CSV_timeformat",
			Script: `
				CSV(payload(),
					field(0, stringType(), "name"),
					field(1, datetimeType("2006/01/02 15:04:05", "KST"), "time"),
					field(2, doubleType(), "value"),
					field(3, stringType(), "active")
				)
				CSV(timeformat("s"), heading(true))
				`,
			Payload: `temp.name,2023/08/10 19:09:16,123.456789,true` + "\n",
			ExpectCSV: []string{
				`name,time,value,active`,
				`temp.name,1691662156,123.456789,true`,
				"\n",
			},
		},
		{
			Name: "CSV_payload_CSV_timeformat_precision",
			Script: `
				CSV(payload(), field(0, timeType("s"), "time"), field(2, floatType(), "value"), field(3, boolType(),"flag") )
				CSV(timeformat("s"), heading(true), precision(2))
			`,
			Payload: strings.Join([]string{
				"1700256261,dry,1,true",
				"1700256262,dry,2,false",
				"1700256262,wet,2,TRUE",
				"1700256263,dry,3,False",
				"1700256264,dry,4,1",
				"1700256264,wet,5,0",
				"",
			}, "\n"),
			ExpectCSV: []string{
				"time,column1,value,flag",
				"1700256261,dry,1.00,true",
				"1700256262,dry,2.00,false",
				"1700256262,wet,2.00,true",
				"1700256263,dry,3.00,false",
				"1700256264,dry,4.00,true",
				"1700256264,wet,5.00,false",
				"\n",
			},
		},
		{
			Name: "CSV_payload_MAPVALUE_MARKDOWN",
			Script: `
				CSV(payload(), header(false))
				MAPVALUE(2, value(2) != "VALUE" ? parseFloat(value(2))*10 : value(2))
				MARKDOWN()
				`,
			Payload: strings.Join([]string{
				`NAME,TIME,VALUE,BOOL`,
				`wave.sin,1676432361,0.000000,true`,
				`wave.cos,1676432361,1.0000000,false`,
				`wave.sin,1676432362,0.406736,true`,
				`wave.cos,1676432362,0.913546,false`,
				`wave.sin,1676432363,0.743144,true`,
			}, "\n") + "\n",
			ExpectText: []string{
				`|column0|column1|column2|column3|`,
				`|:-----|:-----|:-----|:-----|`,
				`|NAME|TIME|VALUE|BOOL|`,
				`|wave.sin|1676432361|0.000000|true|`,
				`|wave.cos|1676432361|10.000000|false|`,
				`|wave.sin|1676432362|4.067360|true|`,
				`|wave.cos|1676432362|9.135460|false|`,
				`|wave.sin|1676432363|7.431440|true|`,
				"",
			},
		},
		{
			Name: "CSV_MARKDOWN",
			Script: `
				CSV(payload(), header(true))
				MARKDOWN()
				`,
			Payload: strings.Join([]string{
				`NAME,TIME,VALUE`,
				`wave.sin,1676432361,0.000000`,
				`wave.cos,1676432361,1.000000`,
				`wave.sin,1676432362,0.406736`,
				`wave.cos,1676432362,0.913546`,
				`wave.sin,1676432363,0.743144`,
			}, "\n"),
			ExpectText: []string{
				`|NAME|TIME|VALUE|`,
				`|:-----|:-----|:-----|`,
				`|wave.sin|1676432361|0.000000|`,
				`|wave.cos|1676432361|1.000000|`,
				`|wave.sin|1676432362|0.406736|`,
				`|wave.cos|1676432362|0.913546|`,
				`|wave.sin|1676432363|0.743144|`,
				"",
			},
		},
		{
			Name: "CSV_payload_MARKDOWN",
			Script: `
				CSV(payload(), header(true))
				MARKDOWN()
				`,
			Payload: strings.Join([]string{
				`NAME,TIME,VALUE`,
				`wave.sin,1676432361,0.000000`,
				`wave.cos,1676432361,1.000000`,
				`wave.sin,1676432362,0.406736`,
				`wave.cos,1676432362,0.913546`,
				`wave.sin,1676432363,0.743144`,
				"\n"}, "\n"),
			ExpectText: []string{
				`|NAME|TIME|VALUE|`,
				`|:-----|:-----|:-----|`,
				`|wave.sin|1676432361|0.000000|`,
				`|wave.cos|1676432361|1.000000|`,
				`|wave.sin|1676432362|0.406736|`,
				`|wave.cos|1676432362|0.913546|`,
				`|wave.sin|1676432363|0.743144|`,
				"",
			},
		},
		{
			Name: "CSV_header(true)_MARKDOWN",
			Script: `
				CSV(payload(),
				field(0, stringType(), 'name'),
				field(1, datetimeType('s'), 'time'),
				field(2, doubleType(), 'value'),
				header(true))
				MARKDOWN()
				`,
			Payload: strings.Join([]string{
				`NAME,TIME,VALUE`,
				`wave.sin,1676432361,0.000000`,
				`wave.cos,1676432361,1.000000`,
				`wave.sin,1676432362,0.406736`,
				`wave.cos,1676432362,0.913546`,
				`wave.sin,1676432363,0.743144`,
			}, "\n"),
			ExpectText: []string{
				`|name|time|value|`,
				`|:-----|:-----|:-----|`,
				`|wave.sin|1676432361000000000|0.000000|`,
				`|wave.cos|1676432361000000000|1.000000|`,
				`|wave.sin|1676432362000000000|0.406736|`,
				`|wave.cos|1676432362000000000|0.913546|`,
				`|wave.sin|1676432363000000000|0.743144|`,
				"",
			},
		},
		{
			Name: "CSV_header(false)_MARKDOWN",
			Script: `
				CSV(payload(),
				field(0, stringType(), 'NAME'),
				field(1, datetimeType('s'), 'TIME'),
				field(2, doubleType(), 'VALUE'),
				header(false))
				MARKDOWN()
				`,
			Payload: strings.Join([]string{
				`wave.sin,1676432361,0.000000`,
				`wave.cos,1676432361,1.000000`,
				`wave.sin,1676432362,0.406736`,
				`wave.cos,1676432362,0.913546`,
				`wave.sin,1676432363,0.743144`,
			}, "\n"),
			ExpectText: []string{
				`|NAME|TIME|VALUE|`,
				`|:-----|:-----|:-----|`,
				`|wave.sin|1676432361000000000|0.000000|`,
				`|wave.cos|1676432361000000000|1.000000|`,
				`|wave.sin|1676432362000000000|0.406736|`,
				`|wave.cos|1676432362000000000|0.913546|`,
				`|wave.sin|1676432363000000000|0.743144|`,
				"",
			},
		},
		{
			Name: "CSV_no_header_MARKDOWN",
			Script: `
				CSV(payload())
				MARKDOWN()
				`,
			Payload: strings.Join([]string{
				`wave.sin,1676432361,0.000000`,
				`wave.cos,1676432361,1.000000`,
				`wave.sin,1676432362,0.406736`,
				`wave.cos,1676432362,0.913546`,
				`wave.sin,1676432363,0.743144`,
			}, "\n"),
			ExpectText: []string{
				`|column0|column1|column2|`,
				`|:-----|:-----|:-----|`,
				`|wave.sin|1676432361|0.000000|`,
				`|wave.cos|1676432361|1.000000|`,
				`|wave.sin|1676432362|0.406736|`,
				`|wave.cos|1676432362|0.913546|`,
				`|wave.sin|1676432363|0.743144|`,
				"",
			},
		},
		{
			Name: "CSV_NDJSON",
			Script: `
				CSV("1,line1\n2,line2\n3,\n4,line4")
				NDJSON( rownum(true) )
			`,
			ExpectText: []string{
				`{"ROWNUM":1,"column0":"1","column1":"line1"}`,
				`{"ROWNUM":2,"column0":"2","column1":"line2"}`,
				`{"ROWNUM":3,"column0":"3","column1":""}`,
				`{"ROWNUM":4,"column0":"4","column1":"line4"}`,
				"\n",
			},
		},
		{
			Name: "CSV_file",
			Script: `
				CSV(file('/iris.data'))
				DROP(10)
				TAKE(2)
				CSV()
				`,
			ExpectCSV: []string{
				`5.4,3.7,1.5,0.2,Iris-setosa`,
				`4.8,3.4,1.6,0.2,Iris-setosa`,
				"\n",
			},
		},
		{
			Name: "CSV_file_gz",
			Script: `
				CSV(file('/iris.data.gz'))
				DROP(10)
				TAKE(2)
				CSV()
				`,
			ExpectCSV: []string{
				`5.4,3.7,1.5,0.2,Iris-setosa`,
				`4.8,3.4,1.6,0.2,Iris-setosa`,
				"\n",
			},
		},
		{
			Name: "CSV_file_JSON_timeformat",
			Script: `
				CSV(file('/iris.data'))
				DROP(10)
				TAKE(2)
				JSON(timeformat('2006-01-02 15:04:05'), tz('LOCAL'))
				`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool())
				require.Equal(t, `["column0","column1","column2","column3","column4"]`, gjson.Get(result, "data.columns").Raw)
				require.Equal(t, `["string","string","string","string","string"]`, gjson.Get(result, "data.types").Raw)
				require.Equal(t, `["5.4","3.7","1.5","0.2","Iris-setosa"]`, gjson.Get(result, "data.rows.0").Raw)
				require.Equal(t, `["4.8","3.4","1.6","0.2","Iris-setosa"]`, gjson.Get(result, "data.rows.1").Raw)
			},
		},
		{
			Name: "CSV_charset_jp",
			Script: `
				CSV(file("/euc-jp.csv"), charset("EUC-JP"))
				CSV()
				`,
			ExpectCSV: []string{
				`利用されてきた文字コー,1701913182,3.141592`,
				"\n",
			},
		},
		{
			Name: "strSprintf",
			Script: `
				FAKE(json(strSprintf('[%.f, %q]', 123, "hello")))
				CSV( heading(false) )
				`,
			ExpectCSV: []string{
				`123,hello`,
				"\n",
			},
		},
		{
			Name: "FAKE_arrange_zero_step",
			Script: `FAKE( arrange(10, 30, 0) )
					CSV()`,
			ExpectErr: `FUNCTION "arrange" step can not be 0`,
		},
		{
			Name: "FAKE_arrange_start_stop_equal",
			Script: `FAKE( arrange(10, 10, 10) )
					CSV()`,
			ExpectErr: `FUNCTION "arrange" start, stop can not be equal`,
		},
		{
			Name: "FAKE_arrange_start_stop_invalid1",
			Script: `FAKE( arrange(10, 30, -10) )
					CSV()`,
			ExpectErr: `FUNCTION "arrange" step can not be less than 0`,
		},
		{
			Name: "FAKE_arrange_start_stop_invalid2",
			Script: `FAKE( arrange(30, 10, 10) )
					CSV()`,
			ExpectErr: `FUNCTION "arrange" step can not be greater than 0`,
		},
		{
			Name: "MAP_AVG",
			Script: `
				FAKE( arrange(10, 30, 10) )
				MAP_AVG(1, value(0))
				CSV( precision(0) )
				`,
			ExpectCSV: []string{
				"10,10",
				"20,15",
				"30,20",
				"\n",
			},
		},
		{
			Name: "MAP_MOVAVG",
			Script: `
				FAKE( linspace(0, 100, 100) )
				MAP_MOVAVG(1, value(0), 10)
				CSV( precision(4) )
				`,
			ExpectCSV: loadLines("./test/movavg_result.csv"),
		},
		{
			Name: "MAP_MOVAVG_nowait",
			Script: `
				FAKE( linspace(0, 100, 100) )
				MAP_MOVAVG(1, value(0), 10, noWait(true))
				CSV( precision(4) )
				`,
			ExpectCSV: loadLines("./test/movavg_result_nowait.csv"),
		},
		{
			Name: "MAP_LOWPASS",
			Script: `
				FAKE(arrange(1, 10, 1))
				MAPVALUE(1, value(0) + simplex(1, value(0))*3)
				MAP_LOWPASS(2, value(1), 0.3)
				CSV(precision(2))
				`,
			ExpectCSV: []string{
				`1.00,1.48,1.48`,
				`2.00,0.40,1.15`,
				`3.00,3.84,1.96`,
				`4.00,2.89,2.24`,
				`5.00,5.47,3.21`,
				`6.00,5.29,3.83`,
				`7.00,7.22,4.85`,
				`8.00,10.31,6.49`,
				`9.00,8.36,7.05`,
				`10.00,8.56,7.50`,
				"\n",
			},
		},
		{
			Name: "MAP_KALMAN",
			Script: `
				FAKE(json({[1.3], [10.2], [5.0], [3.4]}))
				MAP_KALMAN(1, value(0), model(1.0, 1.0, 2.0))
				CSV(precision(1))
				`,
			ExpectCSV: []string{
				`1.3,1.3`,
				`10.2,5.7`,
				`5.0,5.4`,
				`3.4,4.4`,
				"\n",
			},
		},
		{
			Name: "MAP_DIFF",
			Script: `
				FAKE( csv("1\n3\n2\n7") )
				MAP_DIFF(0, value(0))
				CSV()
				`,
			ExpectCSV: []string{"NULL", "2", "-1", "5", "\n"},
		},
		{
			Name: "MAP_NONEGDIFF",
			Script: `
				FAKE( csv("1\n3\n2\n7") )
				MAP_NONEGDIFF(0, value(0))
				CSV()
				`,
			ExpectCSV: []string{"NULL", "2", "0", "5", "\n"},
		},
		{
			Name: "MAP_ABSDIFF",
			Script: `
				FAKE( csv("1\n3\n2\n7") )
				MAP_ABSDIFF(0, value(0))
				CSV()
				`,
			ExpectCSV: []string{"NULL", "2", "1", "5", "\n"},
		},
		{
			Name: "FILTER_CHANGED_string",
			Script: `
				FAKE(json({
					["A", 1.0],
					["A", 2.0],
					["B", 3.0],
					["B", 4.0]
				}))
				FILTER_CHANGED(value(0))
				CSV()
				`,
			ExpectCSV: []string{"A,1", "B,3", "\n"},
		},
		{
			Name: "FILTER_CHANGED_bool",
			Script: `
				FAKE(json({
					["A", true, 1.0],
					["A", false, 2.0],
					["B", false, 3.0],
					["B", true, 4.0]
				}))
				FILTER_CHANGED(value(1))
				CSV()
				`,
			ExpectCSV: []string{"A,true,1", "A,false,2", "B,true,4", "\n"},
		},
		{
			Name: "FILTER_CHANGED_time",
			Script: `
				FAKE(json({
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
				}))
				MAPVALUE(1, parseTime(value(1), "s", tz("UTC")))
				FILTER_CHANGED(value(0), retain(value(1), "2s"))
				CSV(timeformat("s"))
				`,
			ExpectCSV: []string{
				"A,1692329338,1",
				"B,1692329342,5",
				"D,1692329349,9.3",
				"\n",
			},
		},
		{
			Name: "FILTER_CHANGED_useFirstWithLast(true)",
			Script: `
				FAKE(json({
					["A", 1.0], ["A", 2.0],
					["B", 3.0], ["B", 4.0], ["B", 5.0],
					["C", 6.0], ["C", 7.0],
					["D", 8.0], ["D", 9.0]
				}))
				FILTER_CHANGED(value(0), useFirstWithLast(true))
				CSV()
				`,
			ExpectCSV: []string{"A,1", "A,2", "B,3", "B,5", "C,6", "C,7", "D,8", "D,9", "\n"},
		},
		{
			Name: "FILTER_CHANGED_useFirstWithLast(false)",
			Script: `
				FAKE(json({
					["A", 1.0], ["A", 2.0],
					["B", 3.0], ["B", 4.0], ["B", 5.0],
					["C", 6.0], ["C", 7.0],
					["D", 8.0], ["D", 9.0]
				}))
				FILTER_CHANGED(value(0), useFirstWithLast(false))
				CSV()
				`,
			ExpectCSV: []string{"A,1", "B,3", "C,6", "D,8", "\n"},
		},
		{
			Name: "FILTER_CHANGED_useFirstWithLast(false)_implicit",
			Script: `
				FAKE(json({
					["A", 1.0], ["A", 2.0],
					["B", 3.0], ["B", 4.0], ["B", 5.0],
					["C", 6.0], ["C", 7.0],
					["D", 8.0], ["D", 9.0]
				}))
				FILTER_CHANGED(value(0))
				CSV()
				`,
			// This result should be same as using "useFirstWithLast(false)"
			ExpectCSV: []string{"A,1", "B,3", "C,6", "D,8", "\n"},
		},
		{
			Name: "FAKE_sphere_4_4",
			Script: `
				FAKE( sphere(4, 4) )
				PUSHKEY('test')
				CSV( header(true), precision(6) )
				`,
			ExpectCSV: loadLines("./test/sphere_4_4.csv"),
		},
		{
			Name: "FAKE_sphere_0_0",
			Script: `
				FAKE( sphere(0, 0) )
				PUSHKEY('test')
				CSV( header(false), precision(6) )
				`,
			ExpectCSV: loadLines("./test/sphere_0_0.csv"),
		},
		{
			Name: "FFT",
			Script: `
				FAKE( oscillator( range(timeAdd(1685714509*1000000000,'1s'), '1s', '100us'), freq(10, 1.0), freq(50, 2.0)))
				MAPKEY('samples')
				GROUPBYKEY(lazy(false))
				FFT(minHz(0), maxHz(60))
				CSV(precision(6))
				`,
			ExpectCSV: loadLines("./test/fft2d.csv"),
		},
		{
			Name: "FFT_not_enough_samples_0",
			Script: `
				FAKE( linspace(0, 10, 100) )
				FFT()
				CSV()
				`,
			ExpectCSV: []string{"\n"},
		},
		{
			Name: "FFT_not_enough_samples_16",
			Script: `
				FAKE( meshgrid(linspace(0, 10, 100), linspace(0, 10, 1000)) )
				PUSHKEY('sample')
				GROUPBYKEY()
				FFT()
				CSV()
				`,
			ExpectErr: "f(FFT) invalid 0th sample time, but int",
		},
		{
			Name: "FFT_3d",
			Script: `
				FAKE( oscillator( range(timeAdd(1685714509*1000000000,'1s'), '1s', '100us'), freq(10, 1.0), freq(50, 2.0)))
				MAPKEY( roundTime(value(0), '500ms') )
				GROUPBYKEY()
				FFT(maxHz(60))
				FLATTEN()
				PUSHKEY('fft3d')
				CSV(precision(6))
				`,
			ExpectCSV: loadLines("./test/fft3d.csv"),
		},
	}

	tql.ShellExecutable = func(addr, path string) ([]string, error) {
		return []string{"/bin/bash", path}, nil
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestFake_oscillator(t *testing.T) {
	back := util.StandardTimeNow
	util.StandardTimeNow = func() time.Time {
		return time.Unix(0, 1692329338315327000)
	}
	defer func() {
		util.StandardTimeNow = back
	}()

	tests := []TqlTestCase{
		{
			Name: "FAKE_oscillator_no_args",
			Script: `
				FAKE( oscillator() )
				JSON()`,
			ExpectErr: "f(oscillator) no time range is defined",
		},
		{
			Name: "FAKE_oscillator_invalid_args",
			Script: `
				FAKE( oscillator(123) )
				JSON()`,
			ExpectErr: "f(oscillator) invalid arg type 'float64'",
		},
		{
			Name: "FAKE_oscillator_no_time_range",
			Script: `
				FAKE( oscillator(freq(1.0, 1.0)) )
				JSON()
			`,
			ExpectErr: "f(oscillator) no time range is defined",
		},
		{
			Name: "FAKE_oscillator_dup_time_range",
			Script: `
				FAKE( oscillator(freq(1.0, 1.0), range(time('now-1s'), '1s', '200ms'), range(time('now-1s'), '1s', '200ms')) )
				JSON()
			`,
			ExpectErr: "f(oscillator) duplicated time range",
		},
		{
			Name: "FAKE_oscillator_minus_time_range",
			Script: `
				FAKE( oscillator(freq(1.0, 1.0), range(time('now-1s'), '1s', '-200ms')) )
				JSON()
			`,
			ExpectErr: "f(oscillator) period should be positive",
		},
		{
			Name: "FAKE_oscillator_1",
			Script: `
				FAKE( oscillator(freq(1.0, 1.0), range(time('now-1s'), '1s', '200ms')) )
				JSON()
			`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool(), "result: %q", result)
				require.Equal(t, `["time","value"]`, gjson.Get(result, "data.columns").Raw, result)
				require.Equal(t, `["datetime","double"]`, gjson.Get(result, "data.types").Raw, result)
				require.Equal(t, `[1692329337315327000,0.9169371548618853]`, gjson.Get(result, "data.rows.0").Raw, result)
				require.Equal(t, `[[1692329337315327000,0.9169371548618853],[1692329337515327000,-0.09615299237813928],[1692329337715327000,-0.9763628786653529],[1692329337915327000,-0.5072715014883364],[1692329338115327000,0.662850914928241]]`, gjson.Get(result, "data.rows").Raw, result)
			},
		},
		{
			Name: "FAKE_oscillator_2",
			Script: `
				FAKE( oscillator(freq(1.0, 1.0), range(time('now'), '-1s', '200ms')) )
				JSON()
			`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool(), "result: %q", result)
				require.Equal(t, `["time","value"]`, gjson.Get(result, "data.columns").Raw, result)
				require.Equal(t, `["datetime","double"]`, gjson.Get(result, "data.types").Raw, result)
				require.Equal(t, `[1692329337315327000,0.9169371548618853]`, gjson.Get(result, "data.rows.0").Raw, result)
				require.Equal(t, `[[1692329337315327000,0.9169371548618853],[1692329337515327000,-0.09615299237813928],[1692329337715327000,-0.9763628786653529],[1692329337915327000,-0.5072715014883364],[1692329338115327000,0.662850914928241]]`, gjson.Get(result, "data.rows").Raw, result)
			},
		},
		{
			Name: "FAKE_oscillator_1Hz_2Hz_3Hz",
			Script: `
				FAKE( 
					oscillator(
						range(timeAdd(1685714509*1000000000,'1s'), '1s', '1ms'), 
						freq(1, 1.0), freq(2, 2.0), freq(3, 3.0)))
				PUSHKEY('test')
				CSV( header(true), precision(6) )
				`,
			ExpectCSV: loadLines("./test/oscillator_1Hz_2Hz_3Hz.csv"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestScript(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "tengo_src",
			Script: `
				SCRIPT({
					ctx := import("context")
					for i := 0; i < 10; i++ {
						ctx.yieldKey("test", i, i*10)
					}
				})
				CSV()
			`,
			ExpectCSV: []string{
				"0,0", "1,10", "2,20", "3,30", "4,40", "5,50", "6,60", "7,70", "8,80", "9,90", "\n",
			},
		},
		{
			Name: "tengo_src_map",
			Script: `
				SCRIPT({
					ctx := import("context")
					a := 10*2+1
					// comment

					ctx.yield(a)
				})
				SCRIPT({
					ctx := import("context")
					a := ctx.value(0)
					ctx.yield(a+1, 2, 3, 4)
				})
				CSV()
				`,
			ExpectCSV: []string{"22,2,3,4", "\n"},
		},
		{
			Name: "tengo_2",
			Script: `
				FAKE( linspace(1,2,2))
				MAPKEY("hello")
				SCRIPT("tengo", {
					ctx := import("context")
					ctx.yield(ctx.key(), ctx.value(0), ctx.param("temp", 0))
				})
				MAPVALUE(0, value(0), "key")
				MAPVALUE(1, value(1), "value")
				MAPVALUE(2, value(2), "parameter")
				CSV(header(true))
			`,
			ExpectCSV: []string{
				`key,value,parameter`, `hello,1,0`, `hello,2,0`, "\n",
			},
		},
		{
			Name: "js-console-log",
			Script: `
				SCRIPT("js", "console.log('Hello, World!')")
				DISCARD()`,
			ExpectFunc: func(t *testing.T, result string) {
				require.Empty(t, result)
			},
		},
		{
			Name: "js-finalize",
			Script: `
				FAKE( linspace(1,3,3))
				SCRIPT("js", {
					function finalize(){ $.yieldKey("last", 1.234); }
					function square(x) { return x * x };
						$.yield(square($.values[0]));
					})
				CSV(header(false))
			`,
			ExpectCSV: []string{
				"1", "4", "9", "1.234", "\n",
			},
		},
		{
			Name: "js-timeformat",
			Script: `
				STRING(param("format_time") ?? "808210800", separator('\n'))
				SCRIPT("js", {
					epoch = parseInt($.values[0])
					time = new Date(epoch * 1000)
					$.yield(epoch, time.toISOString())
				})
				CSV()`,
			ExpectCSV: []string{"808210800,1995-08-12T07:00:00.000Z", "", ""},
		},
		{
			Name: "js-timeformat-parse",
			Script: `
				STRING(param("timestamp") ?? "1995-08-12T00:00:00.000Z", separator('\n'))
				SCRIPT("js", {
					ts = new Date( Date.parse($.values[0]) );
					epoch = ts / 1000;
					$.yield(epoch, ts.toISOString());
				})
				CSV()`,
			ExpectCSV: []string{"808185600,1995-08-12T00:00:00.000Z", "", ""},
		},
		{
			Name: "js-yieldArray-string",
			Script: `
				STRING('1,2,3,4,5', separator('\n'))
				SCRIPT("js", {
					$.yieldArray($.values[0].split(','))
				})
				JSON()
			`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool())
				require.Equal(t, `["STRING"]`, gjson.Get(result, "data.columns").Raw)
				require.Equal(t, `["string"]`, gjson.Get(result, "data.types").Raw)
				require.Equal(t, `[["1","2","3","4","5"]]`, gjson.Get(result, "data.rows").Raw)
			},
		},
		{
			Name: "js-yieldArray-bool",
			Script: `
				STRING('true,true,false,true,false', separator('\n'))
				SCRIPT("js", {
					$.yieldArray($.values[0].split(',').map(function(v){ return v === 'true'}))
				})
				JSON()
			`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool())
				require.Equal(t, `["STRING"]`, gjson.Get(result, "data.columns").Raw)
				require.Equal(t, `["string"]`, gjson.Get(result, "data.types").Raw)
				require.Equal(t, `[[true,true,false,true,false]]`, gjson.Get(result, "data.rows").Raw)
			},
		},
		{
			Name: "js-yieldArray-number",
			Script: `
				STRING('1.2,2.3,3.4,5.6', separator('\n'))
				SCRIPT("js", {
					$.yieldArray($.values[0].split(',').map(function(v){ return parseFloat(v) }))
				})
				JSON()
			`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool())
				require.Equal(t, `["STRING"]`, gjson.Get(result, "data.columns").Raw)
				require.Equal(t, `["string"]`, gjson.Get(result, "data.types").Raw)
				require.Equal(t, `[[1.2,2.3,3.4,5.6]]`, gjson.Get(result, "data.rows").Raw)
			},
		},
		{
			Name: "js-yieldArray-number-int64",
			Script: `
				STRING('1,2,3,4,5', separator('\n'))
				SCRIPT("js", {
					$.yieldArray($.values[0].split(',').map(function(v){ return parseInt(v) }))
				})
				JSON()
			`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool())
				require.Equal(t, `["STRING"]`, gjson.Get(result, "data.columns").Raw)
				require.Equal(t, `["string"]`, gjson.Get(result, "data.types").Raw)
				require.Equal(t, `[[1,2,3,4,5]]`, gjson.Get(result, "data.rows").Raw)
			},
		},
		{
			Name: "js-yieldArray-number-mixed",
			Script: `
				SCRIPT("js", {
					$.result = {
						columns: ["a", "b", "c", "d"],
						types: ["int64", "double", "string", "bool"]
					};
					var arr = [1, 2.3, '3.4', true];
					$.yieldArray(arr);
				})
				JSON()
			`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool())
				require.Equal(t, `["a","b","c","d"]`, gjson.Get(result, "data.columns").Raw)
				require.Equal(t, `["int64","double","string","bool"]`, gjson.Get(result, "data.types").Raw)
				require.Equal(t, `[[1,2.3,"3.4",true]]`, gjson.Get(result, "data.rows").Raw)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestBridgeSqlite(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "sqlite-table-not-exist",
			Script: `
				SQL(bridge('sqlite'), "select * from example_sql")
				CSV(heading(true))
			`,
			ExpectErr: "no such table: example_sql",
		},
		{
			Name: "sqlite-create-table",
			Script: `
				SQL(bridge('sqlite'), "create table example_sql (` +
				`	id INTEGER NOT NULL PRIMARY KEY,` +
				`	name TEXT,` +
				`	age INTEGER,` +
				`	address TEXT,` +
				`	weight REAL,` +
				`	memo BLOB,` +
				`	UNIQUE(name)` +
				`)")
				MARKDOWN()
			`,
			ExpectText: []string{
				"|MESSAGE|",
				"|:-----|",
				"|Created successfully.|",
				"",
			},
		},
		{
			Name: "sqlite-insert",
			Script: `
				CSV("100,alpha,10,street-100\n200,bravo,20,street-200\n")
				INSERT(bridge('sqlite'), "id", "name", "age", "address", table("example_sql"))
			`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool())
				require.Equal(t, "success", gjson.Get(result, "reason").String())
				require.Equal(t, `1 row inserted.`, gjson.Get(result, "data.message").String())
			},
		},
		// TODO: insert blob value
		// conn.ExecContext(ctx, `insert into example_sql values(?, ?, ?, ?, ?, ?)`,
		//                        200, "bravo", 20, "street-200", 56.789, []byte{0, 1, 0xFF})
		// TODO: select blob value
		// `200,bravo,20,street-200,56.789,\x00\x01\xFF`,
		{
			Name: "sqlite",
			Script: `
				SQL(bridge('sqlite'), "select id, name, age, address from example_sql")
				CSV(heading(true))
			`,
			ExpectCSV: []string{
				"id,name,age,address",
				"100,alpha,10,street-100",
				"200,bravo,20,street-200",
				"\n",
			},
		},
		{
			Name: "sqlite-update-100",
			Script: `
				SQL(bridge('sqlite'), 'update example_sql set weight=? where id = ?', 45.67, 100)
				CSV(heading(false))
			`,
			ExpectCSV: []string{"a row updated.", "\n"},
		},
		{
			Name: "sqlite-update-200",
			Script: `
				SQL(bridge('sqlite'), 'update example_sql set weight=? where id = ?', 56.789, 200)
				CSV(heading(false))
			`,
			ExpectCSV: []string{"a row updated.", "\n"},
		},
		{
			Name: "sqlite-select-updated",
			Script: `
				SQL(bridge('sqlite'), "select * from example_sql")
				CSV(heading(true),nullValue('NULL'))
			`,
			ExpectCSV: []string{
				"id,name,age,address,weight,memo",
				"100,alpha,10,street-100,45.67,NULL",
				`200,bravo,20,street-200,56.789,NULL`,
				"\n",
			},
			RunCondition: func() bool {
				// FIXME: sqlite3-CSV does not work with nullValue('NULL')
				return false
			},
		},
		{
			Name: "sqlite-delete-syntax-error",
			Script: `
				SQL(bridge('sqlite'), 'delete example_sql where id = ?', 100)
				CSV(heading(false))
				`,
			ExpectErr: "near \"example_sql\": syntax error",
		},
		{
			Name: "sqlite-delete-before-count",
			Script: `
				SQL(bridge('sqlite'), 'select count(*) from example_sql where id = ?', param('id'))
				JSON()
				`,
			Params: map[string][]string{"id": {"100"}},
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool())
				// FIXME: count(*) should be integer instead of string
				require.Equal(t, `{"columns":["count(*)"],"types":["string"],"rows":[["1"]]}`, gjson.Get(result, "data").Raw)
			},
		},
		{
			Name: "sqlite-delete",
			Script: `
				SQL(bridge('sqlite'), 'delete from example_sql where id = ?', param('id'))
				CSV(heading(false))
				`,
			Params:    map[string][]string{"id": {"100"}},
			ExpectCSV: []string{"a row deleted.", "\n"},
		},
		{
			Name: "sqlite-delete-after-count",
			Script: `
				SQL(bridge('sqlite'), 'select count(*) from example_sql where id = ?', param('id'))
				JSON()
				`,
			Params: map[string][]string{"id": {"100"}},
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool())
				// FIXME: count(*) should be integer instead of string
				require.Equal(t, `{"columns":["count(*)"],"types":["string"],"rows":[["0"]]}`, gjson.Get(result, "data").Raw)
			},
		},
		{
			Name: "sqlite-select-no-rows",
			Script: `
				SQL(bridge('sqlite'), "select * from example_sql where id = ?", param('id'))
				CSV(heading(true))
				`,
			Params:    map[string][]string{"id": {"-1"}},
			ExpectCSV: []string{"id,name,age,address,weight,memo", "\n"},
		},
		{
			Name: "sqlite-select-no-rows-no-header",
			Script: `
				SQL(bridge('sqlite'), "select * from example_sql where id = ?", param('id'))
				CSV(heading(false))
				`,
			Params:    map[string][]string{"id": {"-1"}},
			ExpectCSV: []string{"\n"},
		},
		{
			Name: "sqlite-js-insert",
			Script: `
				SCRIPT("js", {
					err = $.db({bridge: 'sqlite'})
						.exec("insert into example_sql values(?, ?, ?, ?, ?, ?)", 300, "charlie", 30, "street-300", 67.89, null)
					if (err) {
						$.yield(err.message);
					}
				})
				DISCARD()
				`,
			ExpectFunc: func(t *testing.T, result string) {
			},
		},
		{
			Name: "sqlite-js-query",
			Script: `
				SCRIPT("js", {
					err = $.db({bridge: 'sqlite'}).query("select * from example_sql where id = ?", $.params.id)
					    .forEach(function(row) {
							id = row[0];
							name = row[1];
							age = row[2];
							address = row[3];
							$.yield(id, name, age, address);
						})
					if (err) {
						$.yield(err.message);
					}
				})
				JSON()
				`,
			Params: map[string][]string{"id": {"300"}},
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool())
				require.Equal(t, "success", gjson.Get(result, "reason").String())
				require.Equal(t, `["column0","column1","column2","column3"]`, gjson.Get(result, "data.columns").Raw, result)
				require.Equal(t, `["any","any","any","any"]`, gjson.Get(result, "data.types").Raw, result)
				require.Equal(t, `[300,"charlie",30,"street-300"]`, gjson.Get(result, "data.rows.0").Raw, result)
			},
		},
	}

	if err := bridge.Register(&model.BridgeDefinition{
		Type: model.BRIDGE_SQLITE,
		Name: "sqlite",
		Path: "file::memory:?cache=shared",
	}); err == bridge.ErrBridgeDisabled {
		t.Fatal(err)
	} else {
		defer bridge.Unregister("sqlite")
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestGeoJSON(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-geojson-point",
			Script: `
				SCRIPT("js", {
					var lat = 37.497850;
					var lon =  127.027756;
					var name = "Gangnam-cross";
					var obj = $.geojson({
						type: "Feature",
						geometry: {
							type: "Point",
							coordinates: [lon, lat]
						}
					});
					if( obj instanceof Error ) {
						$.yield(obj.message);
					} else {
						$.yield(obj);
					}
				})
				GEOMAP(geomapID("MTY3NzQ2MDY4NzQyNTc4MTc2"))`,
			ExpectFunc: func(t *testing.T, result string) {
				require.Equal(t, "600px", gjson.Get(result, "style.width").String(), result)
				require.Equal(t, "600px", gjson.Get(result, "style.height").String(), result)
				require.Equal(t, int64(0), gjson.Get(result, "style.grayscale").Int(), result)
				require.Equal(t, `["/web/geomap/leaflet.js"]`, gjson.Get(result, "jsAssets").String(), result)
				require.Equal(t, `["/web/geomap/leaflet.css"]`, gjson.Get(result, "cssAssets").String(), result)
				id := gjson.Get(result, "geomapID").String()
				jsCodeAssets := gjson.Get(result, "jsCodeAssets.0").String()
				require.Equal(t, "/web/api/tql-assets/"+id+"_opt.js", jsCodeAssets, result)
				jsCodeAssets = gjson.Get(result, "jsCodeAssets.1").String()
				require.Equal(t, "/web/api/tql-assets/"+id+".js", jsCodeAssets, result)
			},
			ExpectVolatileFile: func(t *testing.T, mock *VolatileFileWriterMock) {
				b, _ := os.ReadFile("./test/js-geojson-point.js")
				expect := strings.ReplaceAll(string(b), "\r\n", "\n")
				require.Equal(t, expect, mock.buff.String())
			},
		},
		// FIXME: javascript 4 depth array
		// {
		// 	Name: "js-geojson-polygon",
		// 	Script: `
		// 		SCRIPT("js", {
		// 			$.yield({
		// 				type: "polygon",
		// 				value: [
		// 					[
		// 						[[37, -109.05],[41, -109.03],[41, -102.05],[37, -102.04]],
		// 						[[37.29, -108.58],[40.71, -108.58],[40.71, -102.50],[37.29, -102.50]]
		// 					],
		// 					[
		// 						[[41, -111.03],[45, -111.04],[45, -104.05],[41, -104.05]]
		// 					]
		// 				]
		// 			})
		// 		})
		// 		GEOMAP()`,
		// 	ExpectVolatileFile: func(t *testing.T, mock *VolatileFileWriterMock) {
		// 		b, _ := os.ReadFile("./test/js-geojson-polygon.js")
		// 		expect := strings.ReplaceAll(string(b), "\r\n", "\n")
		// 		require.Equal(t, expect, mock.buff.String())
		// 	},
		// },
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestThrottle(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "throttle-10tps",
			Script: `
				FAKE( linspace(1, 10, 10))
				THROTTLE( 10 )
				SCRIPT("js", {
					// Use javascript to add current time for validation
					$.yield((new Date).getTime() * 1000000, $.values[0])
				})
				MAPVALUE(0, time(value(0)))
				JSON()
				`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool())
				require.Equal(t, "success", gjson.Get(result, "reason").String())
				require.Equal(t, `10`, gjson.Get(result, "data.rows.#").String())
				var lastTime time.Time
				for i := 0; i < 10; i++ {
					ts := time.Unix(0, gjson.Get(result, fmt.Sprintf("data.rows.%d", i)).Get("0").Int())
					if i == 0 {
						lastTime = ts
						continue
					}
					delta := ts.Sub(lastTime)
					lastTime = ts
					// theoretically, 10tps should be 100ms
					// but it may take little bit less than 100ms
					require.True(t, delta > 90*time.Millisecond, "delta[%d]: %v", i, delta)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}
