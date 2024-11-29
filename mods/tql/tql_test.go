package tql_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api/testsuite"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/stretchr/testify/require"
)

var testServer *testsuite.Server

func TestMain(m *testing.M) {
	testServer = testsuite.NewServer("./test/tmp")
	testServer.StartServer(m)
	code := m.Run()
	testServer.StopServer(m)
	os.Exit(code)
}

type TqlTestCase struct {
	Name      string
	Script    string
	ExpectCSV []string
}

func runTestCase(t *testing.T, tc TqlTestCase) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	output := &bytes.Buffer{}
	logOutput := &bytes.Buffer{}
	task := tql.NewTaskContext(ctx)
	task.SetDatabase(testServer.DatabaseSVR())
	task.SetOutputWriter(output)
	task.SetLogWriter(logOutput)
	if err := task.CompileString(tc.Script); err != nil {
		t.Log("ERROR:", tc.Name, err.Error())
		t.Fail()
		return
	}
	result := task.Execute()
	if result.Err != nil {
		t.Log("ERROR:", tc.Name, result.Err.Error())
		t.Fail()
		return
	}
	fmt.Println("LOG:", tc.Name, logOutput.String())
	switch task.OutputContentType() {
	case "text/plain", "text/csv; charset=utf-8":
		if len(tc.ExpectCSV) > 0 {
			require.Equal(t, tc.ExpectCSV, strings.Split(output.String(), "\n"))
		} else {
			fmt.Println(output.String())
		}
	default:
		t.Log("ERROR:", tc.Name, "unexpected content type:", task.OutputContentType())
	}
}

func TestSql(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "show-tables",
			Script: `
				SQL("show tables;")
				CSV(header(true))
				`,
			ExpectCSV: []string{
				"DATABASE_NAME,USER_NAME,TABLE_NAME,TABLE_ID,TABLE_TYPE,TABLE_FLAG",
				"MACHBASEDB,SYS,LOG_DATA,13,Log,NULL",
				"MACHBASEDB,SYS,TAG_DATA,6,Tag,NULL",
				"MACHBASEDB,SYS,TAG_SIMPLE,12,Tag,NULL",
				"",
				"",
			},
		},
		{
			Name: "desc-table",
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
				"",
				"",
			},
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
			Name: "hello-world",
			Script: `
				SCRIPT("js", "console.log('Hello, World!')")
				DISCARD()`,
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
			Name: "create-table",
			Script: `
				SCRIPT("js", {
					var ret = $.db().exec("create tag table js_tag (name varchar(40) primary key, time datetime basetime, value double)");
					if (ret instanceof Error) {
						console.error(ret.message);
					}
				})
				DISCARD()`,
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
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}
