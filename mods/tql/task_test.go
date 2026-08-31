package tql_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"os"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gopcua/opcua/id"
	opc_server "github.com/gopcua/opcua/server"
	"github.com/gopcua/opcua/server/attrs"
	"github.com/gopcua/opcua/ua"
	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/server"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/mods/util/metric"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
	"github.com/machbase/neo-server/v8/spi"
	"github.com/machbase/neo-server/v8/spi/machsvr"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func createTestTables() {
	ctx := context.Background()
	conn, err := spi.Connect(ctx, "sys")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	_, err = conn.ExecContext(ctx, `
		create tag table if not exists tag_data(
			name            varchar(100) primary key, 
			time            datetime basetime, 
			value           double summarized,
			short_value     short,
			ushort_value    ushort,
			int_value       integer,
			uint_value 	    uinteger,
			long_value      long,
			ulong_value 	ulong,
			str_value       varchar(400),
			json_value      json,
			ipv4_value      ipv4,
			ipv6_value      ipv6,
			bin_value		binary
		) TAG_DUPLICATE_CHECK_DURATION=1;
	`)
	if err != nil {
		panic(err)
	}
	_, err = conn.ExecContext(ctx, `
		create tag table if not exists tag_simple(
			name            varchar(100) primary key, 
			time            datetime basetime, 
			value           double
		) TAG_DUPLICATE_CHECK_DURATION=1;
	`)
	if err != nil {
		panic(err)
	}
	_, err = conn.ExecContext(ctx, `
		create log table if not exists log_data(
		    time datetime,
			short_value short,
			ushort_value ushort,
			int_value integer,
			uint_value uinteger,
			long_value long,
			ulong_value ulong,
			double_value double,
			float_value float,
			str_value varchar(400),
			json_value json,
			ipv4_value ipv4,
			ipv6_value ipv6,
			text_value text,
			bin_value binary)
	`)
	if err != nil {
		panic(err)
	}
}

func dropTestTables() {
	ctx := context.Background()
	conn, err := spi.Connect(ctx, "sys")
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	_, err = conn.ExecContext(ctx, "DROP TABLE tag_data")
	if err != nil {
		panic(err)
	}
	_, err = conn.ExecContext(ctx, "DROP TABLE tag_simple")
	if err != nil {
		panic(err)
	}
	_, err = conn.ExecContext(ctx, "DROP TABLE log_data")
	if err != nil {
		panic(err)
	}
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
	if strings.HasSuffix(file, ".csv") {
		lines = append(lines, "\n")
	}
	return lines
}

var testServer *machsvr.TestServer
var testHttpAddress string

func TestMain(m *testing.M) {
	testServer = &machsvr.TestServer{}
	testServer.StartServer("./test/tmp")
	createTestTables()

	spi.StartAppendWorkers()

	spi.StartMetrics()

	f, _ := ssfs.NewServerSideFileSystem([]string{"/=test"})
	ssfs.SetDefault(f)

	tql.Init()

	http, err := server.NewHttp(server.WithHttpListenAddress("tcp://127.0.0.1:0"))
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
	tql.Deinit()
	dropTestTables()
	testServer.StopServer()
	os.Exit(code)
}

type TqlTestCase struct {
	Name               string
	Script             string
	Payload            string
	Params             map[string][]string
	LogLevel           *tql.Level
	CtxTimeout         time.Duration
	ExpectErr          string
	ExpectCSV          []string
	ExpectText         []string
	ExpectFunc         func(t *testing.T, result string)
	ExpectVolatileFile func(t *testing.T, mock *VolatileFileWriterMock)
	ExpectLog          []string
	RunCondition       func() bool
}

func (tc TqlTestCase) run(t *testing.T) {
	t.Helper()
	if tc.RunCondition != nil && !tc.RunCondition() {
		t.Skip("Skip by tc.RunCondition")
		return
	}

	memMock := &VolatileFileWriterMock{}

	timeout := 5 * time.Second
	if tc.CtxTimeout > 0 {
		timeout = tc.CtxTimeout
	}
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()

	output := &bytes.Buffer{}
	log := &bytes.Buffer{}
	task := tql.NewTaskContext(ctx)
	task.SetLogWriter(log)
	if tc.LogLevel != nil {
		task.SetLogLevel(*tc.LogLevel)
	}
	task.SetOutputWriterJson(output, true)
	task.SetVolatileAssetsProvider(memMock)
	if tc.Payload != "" {
		task.SetInputReader(bytes.NewBufferString(tc.Payload))
	}
	if len(tc.Params) > 0 {
		task.SetParams(tc.Params)
	}
	if err := task.CompileString(tc.Script); err != nil {
		if tc.ExpectErr != "" {
			require.Equal(t, tc.ExpectErr, err.Error())
		} else {
			t.Log("ERROR:", tc.Name, err.Error())
			t.Fail()
		}
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

	logLines := strings.Split(log.String(), "\n")
	if len(logLines) > 0 && logLines[len(logLines)-1] == "" {
		logLines = logLines[:len(logLines)-1]
	}
	for i, expectLog := range tc.ExpectLog {
		if i >= len(logLines) {
			t.Errorf("Expected Log[%d] %q, but no log line", i, expectLog)
			return
		}
		line := logLines[i]
		if i >= len(tc.ExpectLog) {
			break
		}
		if line != expectLog {
			t.Errorf("Expected Log[%d] %q, got %q", i, expectLog, line)
			return
		}
	}
	if len(logLines) > len(tc.ExpectLog) {
		t.Errorf("Expected Log %d lines, got %d\n%s",
			len(tc.ExpectLog), len(logLines), strings.Join(logLines[len(tc.ExpectLog):], "\n"))
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

func TestTqlCache(t *testing.T) {
	expectText := ""
	tests := []struct {
		Name       string
		Script     string
		Params     map[string][]string
		Payload    string
		Filename   string
		ExpectFunc func(t *testing.T, result string)
	}{
		{
			Name: "cache-enlist",
			Script: `
				FAKE( linspace(
						parseFloat(param("begin")), 
						parseFloat(param("end")),
						parseFloat(param("count"))) )
				MAPVALUE(0, value(0)*random()*10)
				CSV(
					cache(param("begin") + "-" + param("end") + "-" +  param("count"), "5s")
				)`,
			Params:   map[string][]string{"begin": {"1"}, "end": {"10"}, "count": {"10"}},
			Filename: "/test/cache-enlist.tql",
			ExpectFunc: (func(t *testing.T, result string) {
				expectText = result
			}),
		},
		{
			Name: "cache-hit",
			Script: `
				FAKE( linspace(
						parseFloat(param("begin")), 
						parseFloat(param("end")),
						parseFloat(param("count"))) )
				MAPVALUE(0, value(0)*random()*10)
				CSV(
					cache(param("begin") + "-" + param("end") + "-" +  param("count"), "5s")
				)`,
			Params:   map[string][]string{"begin": {"1"}, "end": {"10"}, "count": {"10"}},
			Filename: "/test/cache-enlist.tql",
			ExpectFunc: (func(t *testing.T, result string) {
				require.Equal(t, expectText, result)
			}),
		},
	}

	tql.StartCache(tql.CacheOption{})
	defer tql.StopCache()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()

			output := &bytes.Buffer{}
			task := tql.NewTaskContext(ctx)
			//task.sourcePath = tc.Filename
			task.SetLogWriter(os.Stdout)
			task.SetOutputWriterJson(output, true)
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
				tc.ExpectFunc(t, outputText)
			default:
				t.Fatal("ERROR:", tc.Name, "unexpected content type:", task.OutputContentType())
			}
		})
	}
}

func TestSql_explain(t *testing.T) {
	TqlTestCase{
		Name: "SQL_explain",
		Script: `
			SQL('explain select * from tag_data')
			CSV(header(true))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.Greater(t, len(result), 50, result)
			require.Contains(t, result, "TAG READ (RAW)")
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_explain_full",
		Script: `
			SQL('explain full select * from tag_data')
			CSV(header(true))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.Greater(t, len(result), 5000, result)
			require.Contains(t, result, "EXECUTE")
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_show_storage_like",
		Script: `
			SQL("show storage like 'LOG_DATA'")
			CSV(header(true))
		`,
		ExpectCSV: []string{
			"DATABASE_NAME,TABLE_NAME,DATA_SIZE,INDEX_SIZE,TOTAL_SIZE",
			"MACHBASEDB,LOG_DATA,0,0,0",
			"", "",
		},
	}.run(t)
}

func TestSql_insert_flush_select(t *testing.T) {
	TqlTestCase{
		Name: "SQL_sink",
		Script: `
			SCRIPT({
				const dt = new Date('2026-07-10T17:10:20');
				$.yield(
					'sql_test', dt, 3.142, 			// name, time, value
					-123, 123,						// short, ushort
					-1234, 1234,					// int, uint
					-12345, 12345,					// long, ulong
					'STR', '{"json":true}',			// str, json
					'192.168.0.1', '2001:db8::1',	// ipv4, ipv6
					new Uint8Array([1,2,3]) 		// bin
			)})
			SQL('insert into tag_data (name,time,value, '+
				'short_value,ushort_value,int_value,uint_value, '+
				'long_value,ulong_value,str_value,json_value,ipv4_value,ipv6_value,bin_value) '+
				'values(?,?,?,?,?,?,?,?,?,?,?,?,?,?)',
					value(0), value(1), value(2),
					value(3), value(4), value(5), value(6),
					value(7), value(8), value(9), value(10), value(11), value(12), value(13)
			)
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool())
			require.Equal(t, gjson.Get(result, "data.message").String(), "a row inserted.")
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_FLUSH",
		Script: `
			FAKE(once(1))
			SQL('exec table_flush(tag_data)')
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool())
			require.Equal(t, gjson.Get(result, "data.message").String(), "executed.")
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_json",
		Script: `
			SQL('select * from tag_data where name = ?', 'sql_test')
			JSON(timeformat('default'), tz('Local'))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool())
			require.Equal(t, gjson.Get(result, "reason").String(), "success")
			columns := gjson.Get(result, "data.columns").String()
			require.Equal(t, `["NAME","TIME","VALUE","SHORT_VALUE","USHORT_VALUE","INT_VALUE","UINT_VALUE","LONG_VALUE","ULONG_VALUE","STR_VALUE","JSON_VALUE","IPV4_VALUE","IPV6_VALUE","BIN_VALUE"]`, columns)
			types := gjson.Get(result, "data.types").String()
			require.Equal(t, `["string","datetime","double","int16","uint16","int32","uint32","int64","uint64","string","json","ipv4","ipv6","binary"]`, types)
			values := gjson.Get(result, "data.rows").String()
			require.Equal(t, `[["sql_test","2026-07-10 17:10:20",3.142,-123,123,-1234,1234,-12345,12345,"STR","{\"json\":true}","192.168.0.1","2001:db8::1","0x010203"]]`, values)
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_ndjson",
		Script: `
			SQL('select * from tag_data where name = ?', 'sql_test')
			NDJSON(timeformat('default'), tz('Local'))
			`,
		ExpectFunc: func(t *testing.T, result string) {
			require.Equal(t, `{"NAME":"sql_test","TIME":"2026-07-10 17:10:20","VALUE":3.142,"SHORT_VALUE":-123,"USHORT_VALUE":123,"INT_VALUE":-1234,"UINT_VALUE":1234,"LONG_VALUE":-12345,"ULONG_VALUE":12345,"STR_VALUE":"STR","JSON_VALUE":"{\"json\":true}","IPV4_VALUE":"192.168.0.1","IPV6_VALUE":"2001:db8::1","BIN_VALUE":"0x010203"}`+"\n\n", result)
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_csv",
		Script: `
			SQL('select * from tag_data where name = ?', 'sql_test')
			CSV(header(true), timeformat('default'), tz('Local'))
		`,
		ExpectCSV: []string{
			"NAME,TIME,VALUE,SHORT_VALUE,USHORT_VALUE,INT_VALUE,UINT_VALUE,LONG_VALUE,ULONG_VALUE,STR_VALUE,JSON_VALUE,IPV4_VALUE,IPV6_VALUE,BIN_VALUE",
			`sql_test,2026-07-10 17:10:20,3.142,-123,123,-1234,1234,-12345,12345,STR,"{""json"":true}",192.168.0.1,2001:db8::1,0x010203`,
			"", "",
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_markdown",
		Script: `
			SQL('select * from tag_data where name = ?', 'sql_test')
			MARKDOWN(timeformat('default'), tz('Local'))
		`,
		ExpectText: []string{
			"|NAME|TIME|VALUE|SHORT_VALUE|USHORT_VALUE|INT_VALUE|UINT_VALUE|LONG_VALUE|ULONG_VALUE|STR_VALUE|JSON_VALUE|IPV4_VALUE|IPV6_VALUE|BIN_VALUE|",
			"|:-----|:-----|:-----|:-----|:-----|:-----|:-----|:-----|:-----|:-----|:-----|:-----|:-----|:-----|",
			`|sql_test|2026-07-10 17:10:20|3.142000|-123|123|-1234|1234|-12345|12345|STR|{"json":true}|192.168.0.1|2001:db8::1|0x010203|`,
			"",
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_show_table_usage_like",
		Script: `
			SQL("show table-usage like 'LOG_DATA'")
			CSV(header(true))
		`,
		ExpectCSV: []string{
			"DATABASE,USER,TABLE,STORAGE_USAGE",
			"MACHBASEDB,SYS,LOG_DATA,0",
			"", "",
		},
	}.run(t)
}

func TestSql_show_wrong(t *testing.T) {
	TqlTestCase{
		Name: "SQL_show_wrong",
		Script: `
			SQL('show wrong')
			CSV(header(true))
		`,
		ExpectErr: `f(SQL) unsupported show command "wrong"`,
	}.run(t)
}

func TestSql_show_info(t *testing.T) {
	spi.SetServerInfoProvider(func() map[string]any { return map[string]any{"purpose": "test"} })
	TqlTestCase{
		Name: "SQL_show_info",
		Script: `
			SQL('show info')
			CSV(header(true))
		`,
		ExpectCSV: []string{
			"NAME,VALUE",
			"purpose,test",
			"", "",
		},
	}.run(t)
}

func TestSql_show_license(t *testing.T) {
	TqlTestCase{
		Name: "SQL_show_license",
		Script: `
			SQL('show license')
			CSV(header(true))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
			require.Equal(t, 2, len(lines), result)
			require.Equal(t, "ID,TYPE,CUSTOMER,PROJECT,COUNTRY_CODE,INSTALL_DATE,ISSUE_DATE,STATUS", lines[0])
			// "00000000,COMMUNITY,NONE,NONE,KR,2026-07-08 10:15:59,20991231,Valid",
			require.Regexp(t, regexp.MustCompile(`^[0-9]+,[A-Z]+,[A-Z0-9]+,[A-Z0-9]+,[A-Z]{2},[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2},[0-9]{8},[A-Za-z]+$`), lines[1])
		},
	}.run(t)
}

func TestSql_show_ports(t *testing.T) {
	spi.SetServerPortsProvider(func(svc string) ([]*spi.ServicePort, error) {
		ret := []*spi.ServicePort{}
		if svc == "" || svc == "http" {
			ret = append(ret, &spi.ServicePort{Service: "http", Address: "tcp://127.0.0.1:5654"})
		}
		if svc == "" || svc == "mqtt" {
			ret = append(ret, &spi.ServicePort{Service: "mqtt", Address: "tcp://127.0.0.1:1883"})
		}
		return ret, nil
	})
	TqlTestCase{
		Name: "SQL_show_ports",
		Script: `
			SQL('show ports')
			CSV(header(true))
		`,
		ExpectCSV: []string{
			"PORT,ADDRESS",
			"http,tcp://127.0.0.1:5654",
			"mqtt,tcp://127.0.0.1:1883",
			"", "",
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_show_ports_mqtt",
		Script: `
			SQL('show ports mqtt')
			BOX()
			`,
		ExpectText: []string{
			"+------+----------------------+",
			"| PORT | ADDRESS              |",
			"+------+----------------------+",
			"| mqtt | tcp://127.0.0.1:1883 |",
			"+------+----------------------+",
			"",
		},
	}.run(t)
}

func TestSql_show_users(t *testing.T) {
	TqlTestCase{
		Name: "SQL_create_user",
		Script: `
			SQL('create user testuser identified by "testpass"')
			CSV(header(true))
		`,
		ExpectCSV: []string{
			"MESSAGE",
			"user created.",
			"", "",
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_show_users",
		Script: `
			SQL('show users')
			CSV(header(true))
		`,
		ExpectCSV: []string{
			"USER_ID,NAME",
			"1,SYS",
			"2,TESTUSER",
			"", "",
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_drop_user",
		Script: `
			SQL('drop user testuser')
			CSV(header(true))
		`,
		ExpectText: []string{
			"MESSAGE",
			"user dropped.",
			"", "",
		},
	}.run(t)
}

func TestSql_show_databases(t *testing.T) {
	TqlTestCase{
		Name: "SQL_create_database",
		Script: `
			SQL('create database testdb')
			CSV(header(true))
		`,
		ExpectCSV: []string{
			"MESSAGE",
			"database created.",
			"", "",
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_show_databases",
		Script: `
			SQL('show databases')
			BOX()
		`,
		ExpectText: []string{
			"+-------------+------------+--------+-------------+---------+--------+------------+",
			"| DATABASE_ID | NAME       | KIND   | ACCESS_MODE | CAN_USE | STATE  | IS_DEFAULT |",
			"+-------------+------------+--------+-------------+---------+--------+------------+",
			"| 1           | MACHBASEDB | ACTIVE | READ_WRITE  | 1       | NORMAL | 1          |",
			"| 2           | TESTDB     | ACTIVE | READ_WRITE  | 1       | NORMAL | 0          |",
			"+-------------+------------+--------+-------------+---------+--------+------------+",
			"",
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_drop_database",
		Script: `
			SQL('drop database testdb')
			CSV(header(true))
		`,
		ExpectCSV: []string{
			"MESSAGE",
			"database dropped.",
			"", "",
		},
	}.run(t)
}

func TestSql_show_tables(t *testing.T) {
	TqlTestCase{
		Name: "SQL_show_tables",
		Script: `
			SQL('show tables')
			CSV(header(true))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
			require.GreaterOrEqual(t, len(lines), 4)
			require.Equal(t, "DATABASE_NAME,USER_NAME,TABLE_NAME,TABLE_ID,TABLE_TYPE,TABLE_FLAG", lines[0])
			require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,LOG_DATA,[0-9]+,Log,$`), lines[1])
			require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,TAG_DATA,[0-9]+,Tag,$`), lines[2])
			require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,TAG_SIMPLE,[0-9]+,Tag,$`), lines[3])
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_show_tables_all",
		Script: `
			SQL('show tables --all')
			CSV(header(true))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
			require.GreaterOrEqual(t, len(lines), 4)
			require.Equal(t, "DATABASE_NAME,USER_NAME,TABLE_NAME,TABLE_ID,TABLE_TYPE,TABLE_FLAG", lines[0])
			require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,LOG_DATA,[0-9]+,Log,$`), lines[1])
			require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,TAG_DATA,[0-9]+,Tag,$`), lines[2])
			require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,TAG_SIMPLE,[0-9]+,Tag,$`), lines[3])
			require.GreaterOrEqual(t, len(lines), 8)
			require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,_TAG_DATA_DATA_0,[0-9]+,KeyValue,Data$`), lines[4])
			require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,_TAG_DATA_META,[0-9]+,Lookup,Meta$`), lines[5])
			require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,_TAG_SIMPLE_DATA_0,[0-9]+,KeyValue,Data$`), lines[6])
			require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,_TAG_SIMPLE_META,[0-9]+,Lookup,Meta$`), lines[7])
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_show_tables_like",
		Script: `
			SQL("show tables like 'TAG%'")
			CSV(header(true))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
			require.GreaterOrEqual(t, len(lines), 3)
			require.Equal(t, "DATABASE_NAME,USER_NAME,TABLE_NAME,TABLE_ID,TABLE_TYPE,TABLE_FLAG", lines[0])
			require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,TAG_DATA,[0-9]+,Tag,$`), lines[1])
			require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,TAG_SIMPLE,[0-9]+,Tag,$`), lines[2])
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_show_tables_with_all",
		Script: `
			SQL("show tables with all like '_TAG_DATA%'")
			CSV(header(true))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
			require.GreaterOrEqual(t, len(lines), 3)
			require.Equal(t, "DATABASE_NAME,USER_NAME,TABLE_NAME,TABLE_ID,TABLE_TYPE,TABLE_FLAG", lines[0])
			require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,_TAG_DATA_DATA_0,[0-9]+,KeyValue,Data$`), lines[1])
			require.Regexp(t, regexp.MustCompile(`^MACHBASEDB,SYS,_TAG_DATA_META,[0-9]+,Lookup,Meta$`), lines[2])
		},
	}.run(t)
}

func TestSql_show_table(t *testing.T) {
	TqlTestCase{
		Name: "SQL_show_table_log_data",
		Script: `
			SQL('show table log_data')
			CSV(header(true))
		`,
		ExpectCSV: []string{
			"COLUMN,TYPE,LENGTH,FLAG,INDEX",
			"TIME,datetime,31,,",
			"SHORT_VALUE,short,6,,",
			"USHORT_VALUE,ushort,5,,",
			"INT_VALUE,integer,11,,",
			"UINT_VALUE,uinteger,10,,",
			"LONG_VALUE,long,20,,",
			"ULONG_VALUE,ulong,20,,",
			"DOUBLE_VALUE,double,17,,",
			"FLOAT_VALUE,float,17,,",
			"STR_VALUE,varchar,400,,",
			"JSON_VALUE,json,32767,,",
			"IPV4_VALUE,ipv4,15,,",
			"IPV6_VALUE,ipv6,45,,",
			"TEXT_VALUE,text,67108864,,",
			"BIN_VALUE,binary,67108864,,",
			"", "",
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_show_table_log_data_all",
		Script: `
			SQL('show table log_data --all')
			CSV(header(true))
		`,
		ExpectCSV: []string{
			"COLUMN,TYPE,LENGTH,FLAG,INDEX",
			"_ARRIVAL_TIME,datetime,31,,",
			"TIME,datetime,31,,",
			"SHORT_VALUE,short,6,,",
			"USHORT_VALUE,ushort,5,,",
			"INT_VALUE,integer,11,,",
			"UINT_VALUE,uinteger,10,,",
			"LONG_VALUE,long,20,,",
			"ULONG_VALUE,ulong,20,,",
			"DOUBLE_VALUE,double,17,,",
			"FLOAT_VALUE,float,17,,",
			"STR_VALUE,varchar,400,,",
			"JSON_VALUE,json,32767,,",
			"IPV4_VALUE,ipv4,15,,",
			"IPV6_VALUE,ipv6,45,,",
			"TEXT_VALUE,text,67108864,,",
			"BIN_VALUE,binary,67108864,,",
			"_RID,long,20,,",
			"", "",
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_desc_tag_data",
		Script: `
			SQL('desc tag_data')
			CSV(header(true))
		`,
		ExpectCSV: []string{
			"COLUMN,TYPE,LENGTH,FLAG,INDEX",
			"NAME,varchar,100,tag name,",
			"TIME,datetime,31,base time,",
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
			"BIN_VALUE,binary,32767,,",
			"", "",
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_describe_tag_data_all",
		Script: `
			SQL('describe tag_data --all')
			CSV(header(true))
		`,
		ExpectCSV: []string{
			"COLUMN,TYPE,LENGTH,FLAG,INDEX",
			"NAME,varchar,100,tag name,",
			"TIME,datetime,31,base time,",
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
			"BIN_VALUE,binary,32767,,",
			"_RID,long,20,,",
			"", "",
		},
	}.run(t)
}

func TestSql_show_indexes(t *testing.T) {
	TqlTestCase{
		Name: "SQL_show_indexes",
		Script: `
			SQL('show indexes')
			CSV(header(true))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
			require.GreaterOrEqual(t, len(lines), 5)
			require.Equal(t, "ID,DATABASE,USER,TABLE,COLUMN,INDEX_NAME,INDEX_TYPE,KEY_COMPRESS,MAX_LEVEL,PART_VALUE_COUNT,BITMAP_ENCODE", lines[0])

			required := map[string]struct {
				table  string
				column string
			}{
				"__PK_IDX__TAG_DATA_META_1":   {table: "_TAG_DATA_META", column: "_ID"},
				"_TAG_DATA_META_NAME":         {table: "_TAG_DATA_META", column: "NAME"},
				"__PK_IDX__TAG_SIMPLE_META_1": {table: "_TAG_SIMPLE_META", column: "_ID"},
				"_TAG_SIMPLE_META_NAME":       {table: "_TAG_SIMPLE_META", column: "NAME"},
			}
			seen := map[string]bool{}
			for _, line := range lines[1:] {
				fields := strings.Split(line, ",")
				require.GreaterOrEqual(t, len(fields), 11)
				idxName := fields[5]
				req, ok := required[idxName]
				if !ok {
					continue
				}
				require.Equal(t, "MACHBASEDB", fields[1])
				require.Equal(t, "SYS", fields[2])
				require.Equal(t, req.table, fields[3])
				require.Equal(t, req.column, fields[4])
				require.Equal(t, "REDBLACK", fields[6])
				seen[idxName] = true
			}
			for name := range required {
				require.True(t, seen[name], "required index missing: %s", name)
			}
		},
	}.run(t)
}

func TestSql_show_index(t *testing.T) {
	TqlTestCase{
		Name: "SQL_show_index",
		Script: `
			SQL('show index _TAG_DATA_META_NAME')
			CSV(header(true))
		`,
		ExpectCSV: []string{
			"ID,DATABASE,USER,TABLE,COLUMN,INDEX_NAME,INDEX_TYPE,KEY_COMPRESS,MAX_LEVEL,PART_VALUE_COUNT,BITMAP_ENCODE",
			"4,MACHBASEDB,SYS,_TAG_DATA_META,NAME,_TAG_DATA_META_NAME,REDBLACK,UNCOMPRESSED,0,100000,EQUAL",
			"", "",
		},
	}.run(t)
}

func TestSql_show_indexgap(t *testing.T) {
	TqlTestCase{
		Name: "SQL_show-indexgap_JSON",
		Script: `
				SQL("show indexgap")
				JSON()
				`,
		ExpectFunc: func(t *testing.T, result string) {
			if strings.TrimSpace(result) == "" {
				return
			}
			require.True(t, gjson.Get(result, "success").Bool(), "result: %q", result)
			require.Equal(t, "INDEX_ID", gjson.Get(result, "data.columns.0").String(), result)
			require.Equal(t, "TABLE_NAME", gjson.Get(result, "data.columns.1").String(), result)
			require.Equal(t, "INDEX_NAME", gjson.Get(result, "data.columns.2").String(), result)
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_show-tagindexgap_JSON",
		Script: `
				SQL("show tagindexgap")
				JSON()
				`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool(), "result: %q", result)
			require.Equal(t, "TABLE_ID", gjson.Get(result, "data.columns.0").String(), result)
			require.Equal(t, "TABLE_NAME", gjson.Get(result, "data.columns.1").String(), result)
			require.Equal(t, "STATUS", gjson.Get(result, "data.columns.2").String(), result)
		},
	}.run(t)
}

func TestSql_show_lsm(t *testing.T) {
	TqlTestCase{
		Name: "SQL_show_lsm",
		Script: `
			SQL('show lsm')
			CSV(header(true))
		`,
		ExpectCSV: []string{
			"TABLE_NAME,INDEX_NAME,LEVEL,COUNT",
			"", "",
		},
	}.run(t)
}

func TestSql_show_tags(t *testing.T) {
	TqlTestCase{
		Name: "SQL_insert",
		Script: `
			SCRIPT({$.yield('show_test', 1.234)})
			SQL('insert into tag_data (name,time,value) values(?,now,?)', value(0), value(1))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool())
			require.Equal(t, gjson.Get(result, "data.message").String(), "a row inserted.")
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_exec_flush_table",
		Script: `
			FAKE(once(1))
			SQL('exec table_flush(tag_data)')
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool())
			require.Equal(t, gjson.Get(result, "data.message").String(), "executed.")
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_show_tags_no_args",
		Script: `
			SQL('show tags')
			CSV(header(true))
		`,
		ExpectErr: `f(SQL) show tags expects at least 1 argument, got 0`,
	}.run(t)
	TqlTestCase{
		Name: "SQL_show_tags",
		Script: `
			SQL('show tags tag_data')
			CSV(header(true))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
			require.GreaterOrEqual(t, len(lines), 2)
			require.Equal(t, "ID,NAME,ROW_COUNT,MIN_TIME,MAX_TIME,RECENT_ROW_TIME,MIN_VALUE,MIN_VALUE_TIME,MAX_VALUE,MAX_VALUE_TIME", lines[0])
			hasTag := false
			hasValue := false
			for _, line := range lines[1:] {
				if strings.Contains(line, "show_test") {
					hasTag = true
				}
				if strings.Contains(line, "1.234") {
					hasValue = true
				}
			}
			require.True(t, hasTag, "expected to find tag 'show_test' in output")
			require.True(t, hasValue, "expected to find value '1.234' in output")
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_show_tags_log_table",
		Script: `
			SQL('show tags log_data')
			CSV(header(true))
		`,
		ExpectErr: `table 'LOG_DATA' is not a tag table`,
	}.run(t)
	TqlTestCase{
		Name: "SQL_show_tagindexgap",
		Script: `
			SQL('show tagindexgap')
			CSV(header(true))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
			require.GreaterOrEqual(t, len(lines), 1)
			require.Equal(t, "TABLE_ID,TABLE_NAME,STATUS,DISK_GAP,MEMORY_GAP", lines[0])
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_show_rollupgap",
		Script: `
			SQL('show rollupgap')
			CSV(header(true))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
			require.GreaterOrEqual(t, len(lines), 1)
			require.Equal(t, "USER_NAME,ROLLUP_NAME,SRC_TABLE,ROLLUP_TABLE,SRC_END_RID,ROLLUP_END_RID,GAP,RUN_STATE,LAST_ELAPSED_MSEC,LAST_WAKEUP_TIME,NEXT_WAKEUP_TIME", lines[0])
		},
	}.run(t)
}

func TestSql_show_sessions(t *testing.T) {
	TqlTestCase{
		Name: "SQL_show_sessions",
		Script: `
			SQL('show sessions')
			CSV(header(true))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
			require.GreaterOrEqual(t, len(lines), 2)
			require.Equal(t, "ID,USER_NAME,USER_ID,LOGIN_TIME,TYPE,USER_IP,MAX_QPX_MEM", lines[0])
			require.Regexp(t, regexp.MustCompile(`^[0-9]+,[A-Z]+,[0-9]+,[0-9]+,CLI,127.0.0.1,[0-9]+([.][0-9]+)?[KMG]?B$`), lines[1])
		},
	}.run(t)
}

func TestSql_show_statements(t *testing.T) {
	TqlTestCase{
		Name: "SQL_show_statements",
		Script: `
			SQL('show statements')
			CSV(header(true))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
			require.GreaterOrEqual(t, len(lines), 2)
			require.Equal(t, "ID,SESSION_ID,STATE,RECORD_SIZE,QUERY", lines[0])
			require.Regexp(t, regexp.MustCompile(`^[0-9]+,[0-9]+,.+,[0-9]+,.+$`), lines[1])
		},
	}.run(t)
}

func TestSql_show_storage(t *testing.T) {
	TqlTestCase{
		Name: "SQL_show_storage",
		Script: `
			SQL('show storage')
			CSV(header(true))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
			require.GreaterOrEqual(t, len(lines), 2)
			require.Equal(t, "DATABASE_NAME,TABLE_NAME,DATA_SIZE,INDEX_SIZE,TOTAL_SIZE", lines[0])
			// LOG_DATA,0,0,0
			require.Regexp(t, regexp.MustCompile(`[A-Z_]+,[A-Z0-9_]+,[0-9]+,[0-9]+,[0-9]+$`), lines[1])
		},
	}.run(t)
}

func TestSql_show_table_usage(t *testing.T) {
	TqlTestCase{
		Name: "SQL_show_table_usage",
		Script: `
			SQL('show table-usage')
			CSV(header(true))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			lines := strings.Split(strings.TrimSuffix(result, "\n\n"), "\n")
			require.GreaterOrEqual(t, len(lines), 2)
			require.Equal(t, "DATABASE,USER,TABLE,STORAGE_USAGE", lines[0])
			// LOG_DATA,0,0,0
			require.Regexp(t, regexp.MustCompile(`^.+,.+,.+,[0-9]+$`), lines[1])
		},
	}.run(t)
}

func TestSql_show_others(t *testing.T) {
	TqlTestCase{
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
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_exec_flush_table",
		Script: `
			SQL("EXEC table_flush(tag_simple)")
			MARKDOWN()
			`,
		ExpectText: []string{
			`|MESSAGE|`,
			`|:-----|`,
			`|table flushed.|`,
			``,
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_show-tags",
		Script: `
			SQL("show tags tag_simple")
			CSV(header(true))
			`,
		ExpectCSV: []string{
			"ID,NAME,ROW_COUNT,MIN_TIME,MAX_TIME,RECENT_ROW_TIME,MIN_VALUE,MIN_VALUE_TIME,MAX_VALUE,MAX_VALUE_TIME",
			"1,tag1,2,1692686707380411000,1692686708380411000,1692686708380411000,NULL,NULL,NULL,NULL",
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_explain-json-select",
		Script: `
			SQL("explain select * from tag_simple where name = 'tag1'")
			DROP(0)
			TAKE(50)
			JSON(timeformat('2006-01-02 15:04:05'), tz('LOCAL'))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.Contains(t, result, `"success":true`, result)
			require.Contains(t, result, "PROJECT")
			require.Contains(t, result, "SCAN")
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_select-from-table",
		Script: `
			SQL("select TIME, VALUE from tag_simple where name = 'tag1'")
			CSV( precision(3), header(true) )
			`,
		ExpectCSV: []string{
			"TIME,VALUE",
			"1692686707380411000,0.100",
			"1692686708380411000,0.200",
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_select-from-table-bind-params",
		Script: `
			SQL("select TIME, VALUE from tag_simple where name = ?", 'tag1')
			CSV( precision(3), header(true) )
			`,
		ExpectCSV: []string{
			"TIME,VALUE",
			"1692686707380411000,0.100",
			"1692686708380411000,0.200",
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_select-from-table-named-bind-params",
		Script: `
			SQL("select TIME, VALUE from tag_simple where name = :name limit :one, :one",
				named("name", "tag1"),
				named("one", 1))
			CSV( precision(3), header(true) )
			`,
		ExpectCSV: []string{
			"TIME,VALUE",
			"1692686708380411000,0.200",
			"\n",
		},
	}.run(t)

	TqlTestCase{
		Name: "SQL_select-from-table-rownum",
		Script: `
			SQL("select TIME, VALUE from tag_simple where name = 'tag1'")
			PUSHKEY('test')
			CSV( precision(3), header(true) )
			`,
		ExpectCSV: []string{
			"ROWNUM,TIME,VALUE",
			"1,1692686707380411000,0.100",
			"2,1692686708380411000,0.200",
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_select-from-table-rownum_BOX",
		Script: `
			SQL("select TIME, VALUE from tag_simple where name = 'tag1'")
			PUSHKEY('test')
			BOX( precision(3), header(true) )
			`,
		ExpectText: []string{
			"+--------+---------------------+-------+",
			"| ROWNUM | TIME                | VALUE |",
			"+--------+---------------------+-------+",
			"| 1      | 1692686707380411000 | 0.100 |",
			"| 2      | 1692686708380411000 | 0.200 |",
			"+--------+---------------------+-------+",
			"",
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_map-select",
		Script: `
			FAKE(json({["tag1"]}))
			SQL("select TIME, VALUE from tag_simple where name = ?", value(0))
			CSV( precision(3), header(true) )
			`,
		ExpectCSV: []string{
			"TIME,VALUE",
			"1692686707380411000,0.100",
			"1692686708380411000,0.200",
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_SQL",
		Script: `
			SQL("select TIME, VALUE from tag_simple where name = ?", param("name"))
			SQL("insert into tag_simple (name, time, value) values (?, ?, ?)", "tag2", value(0), value(1))
		`,
		Params: map[string][]string{
			"name": {"tag1"},
		},
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool(), "result: %q", result)
			require.Equal(t, "success", gjson.Get(result, "reason").String(), result)
			require.Equal(t, "2 rows inserted.", gjson.Get(result, "data.message").String(), result)
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_exec_flush_table",
		Script: `
			SQL("EXEC table_flush(tag_simple)")
			BOX()
			`,
		ExpectText: []string{
			"+----------------+",
			"| MESSAGE        |",
			"+----------------+",
			"| table flushed. |",
			"+----------------+",
			"",
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_SQL-cleanup",
		Script: `
			SQL("delete from tag_simple where name = ?", param("name"))
			MARKDOWN()
		`,
		Params: map[string][]string{
			"name": {"tag2"},
		},
		ExpectText: []string{
			`|MESSAGE|`,
			`|:-----|`,
			`|2 rows deleted.|`,
			``,
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_create-tag-table",
		Script: `
			SQL({create tag table if not exists tag_simple(
				name varchar(40) primary key, time datetime basetime, value double summarized )})
			MARKDOWN(html(true), rownum(true), heading(true), brief(true))
			`,
		ExpectText: loadLines("./test/sql_ddl_executed.txt"),
	}.run(t)
	TqlTestCase{
		Name: "QUERY_CSV",
		Script: `
			QUERY('VALUE', from('tag_simple', 'tag1', "TIME"), between(1692686707000000000, 1692686709000000000))
			CSV( precision(3), header(true) )
			`,
		ExpectCSV: []string{
			"TIME,VALUE",
			"1692686707380411000,0.100",
			"1692686708380411000,0.200",
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "QUERY_JSON-rows-flatten",
		Script: `
			QUERY('VALUE', from('tag_simple', 'tag1', "TIME"), between(1692686707000000000, 1692686709000000000))
			JSON( precision(3), rowsFlatten(true) )
			`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool())
			require.Equal(t, `["TIME","VALUE"]`, gjson.Get(result, "data.columns").Raw)
			require.Equal(t, `["datetime","double"]`, gjson.Get(result, "data.types").Raw)
			require.Equal(t, `[1692686707380411000,0.1,1692686708380411000,0.2]`, gjson.Get(result, "data.rows").Raw)
		},
	}.run(t)
	TqlTestCase{
		Name: "QUERY_JSON-rows-flatten-rownum",
		Script: `
			QUERY('VALUE', from('tag_simple', 'tag1', "TIME"), between(1692686707000000000, 1692686709000000000))
			JSON( precision(3), rowsFlatten(true), rownum(true) )
			`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool())
			require.Equal(t, `["ROWNUM","TIME","VALUE"]`, gjson.Get(result, "data.columns").Raw)
			require.Equal(t, `["int64","datetime","double"]`, gjson.Get(result, "data.types").Raw)
			require.Equal(t, `[1,1692686707380411000,0.1,2,1692686708380411000,0.2]`, gjson.Get(result, "data.rows").Raw)
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_NDJSON",
		Script: `
			SQL("select TIME, VALUE from tag_simple where name = 'tag1'")
			NDJSON( timeformat('default'), tz('UTC') )
			`,
		ExpectText: []string{
			`{"TIME":"2023-08-22 06:45:07.38","VALUE":0.1}`,
			`{"TIME":"2023-08-22 06:45:08.38","VALUE":0.2}`,
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "FAKE_INSERT",
		Script: `
			FAKE( linspace(0, 1, 3) )
			PUSHVALUE(0, timeAdd('now', value(0)*2000000000))
			INSERT('TIME', 'VALUE', table('tag_simple'), tag('signal.3'))
			`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool(), "result: %q", result)
			require.Equal(t, "success", gjson.Get(result, "reason").String(), result)
			require.Equal(t, `{"message":"3 rows inserted."}`, gjson.Get(result, "data").Raw, result)
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_exec_flush_table",
		Script: `
			SQL("EXEC table_flush(tag_simple)")
			BOX()
			`,
		ExpectText: []string{
			"+----------------+",
			"| MESSAGE        |",
			"+----------------+",
			"| table flushed. |",
			"+----------------+",
			"",
		},
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
		Name: "FAKE_APPEND",
		Script: `
			FAKE( linspace(0, 1, 3) )
			PUSHVALUE(0, timeAdd('now', value(0)*2000000000))
			PUSHVALUE(0, 'signal.append')
			APPEND( table('tag_simple') )
			`,
		ExpectFunc: func(t *testing.T, result string) {
			spi.FlushAppendWorkers("tag_simple")
			require.True(t, gjson.Get(result, "success").Bool(), "result: %q", result)
			require.Equal(t, "success", gjson.Get(result, "reason").String(), result)
			// since we are using api.AppendWorker, the success and fail count is always same as the number of records
			require.Equal(t, `{"message":"append 3 rows (success 3, fail 0)"}`, gjson.Get(result, "data").Raw, result)
		},
	}.run(t)
	TqlTestCase{
		Name: "SQL_exec_flush_table",
		Script: `
			SQL("EXEC table_flush(tag_simple)")
			BOX()
			`,
		ExpectText: []string{
			"+----------------+",
			"| MESSAGE        |",
			"+----------------+",
			"| table flushed. |",
			"+----------------+",
			"",
		},
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
								$.yield(r[0], new Date(r[1]/1000000), r[2]);
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
			require.Equal(t, `[["tag1",1692686707,0.1],["tag1",1692686708,0.2]]`, gjson.Get(result, "data.rows").Raw, result)
		},
	}.run(t)
	TqlTestCase{
		Name: "js-request-csv",
		Script: fmt.Sprintf(`
			SCRIPT("js", {
				$.result = {
					columns: ["NAME", "TIME", "VALUE"],
					types : ["string", "datetime", "double"]
				};
			},{
				$.request("%s/db/query?q="+
						encodeURIComponent("select name, time, value from tag_simple where name = 'tag1' limit 2")+"&format=csv&header=skip", 
						{method: 'GET'}
					).do(function(rsp) {
						rsp.csv(function(r){
							$.yield(r[0], new Date(parseInt(r[1]/1000000)), parseFloat(r[2]));
						})
					})
				})
			JSON(timeformat("s"))
		`, testHttpAddress),
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool(), "result: %q", result)
			require.Equal(t, `["NAME","TIME","VALUE"]`, gjson.Get(result, "data.columns").Raw, result)
			require.Equal(t, `["string","datetime","double"]`, gjson.Get(result, "data.types").Raw, result)
			require.Equal(t, `[["tag1",1692686707,0.1],["tag1",1692686708,0.2]]`, gjson.Get(result, "data.rows").Raw, result)
		},
	}.run(t)
}

// taskTestBridgeDefProviderStub is a minimal BridgeDefProvider used by
// tests that register an ad-hoc bridge directly (bypassing model.Provider).
type taskTestBridgeDefProviderStub struct {
	defs map[string]*model.BridgeDefinition
}

func (p *taskTestBridgeDefProviderStub) LoadBridge(_ context.Context, _ model.UserScope, name string) (*model.BridgeDefinition, error) {
	def, ok := p.defs[name]
	if !ok {
		return nil, fmt.Errorf("undefined bridge name '%s'", name)
	}
	return def, nil
}

func (p *taskTestBridgeDefProviderStub) LoadAllBridgesForBootstrap(ctx context.Context) ([]*model.BridgeDefinition, error) {
	return []*model.BridgeDefinition{}, nil
}
func (p *taskTestBridgeDefProviderStub) LoadAllBridges(ctx context.Context, scope model.UserScope) ([]*model.BridgeDefinition, error) {
	return []*model.BridgeDefinition{}, nil
}
func (p *taskTestBridgeDefProviderStub) SaveBridge(ctx context.Context, scope model.UserScope, def *model.BridgeDefinition) error {
	return nil
}
func (p *taskTestBridgeDefProviderStub) RemoveBridge(ctx context.Context, scope model.UserScope, name string) error {
	return nil
}

func TestSql_bridge_sqlite(t *testing.T) {
	def := &model.BridgeDefinition{
		Id:   1,
		Type: model.BRIDGE_SQLITE,
		Name: "sqlite",
		Path: "file::memory:?cache=shared",
	}
	bridge.SetBridgeProvider(&taskTestBridgeDefProviderStub{defs: map[string]*model.BridgeDefinition{"sqlite": def}})
	if err := bridge.RegisterByID(def); err == bridge.ErrBridgeDisabled {
		t.Fatal(err)
	} else {
		defer bridge.UnregisterByID(def.Id)
	}

	TqlTestCase{
		Name: "sqlite-table-not-exist",
		Script: `
			SQL(bridge('sqlite'), "select * from example_sql")
			CSV(heading(true))
		`,
		ExpectErr: "no such table: example_sql",
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	// TODO: insert blob value
	// conn.ExecContext(ctx, `insert into example_sql values(?, ?, ?, ?, ?, ?)`,
	//                        200, "bravo", 20, "street-200", 56.789, []byte{0, 1, 0xFF})
	// TODO: select blob value
	// `200,bravo,20,street-200,56.789,\x00\x01\xFF`,
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
		Name: "sqlite-params-format",
		Script: `
			SQL(bridge('sqlite'), "select id, name, age, address from example_sql")
			HTML({
				{{- .V.name }}: {{ .V.age | format (param "f") }}, {{ .V.address }}{{ "\n" -}}
			})
		`,
		Params: map[string][]string{"f": {"age=%d"}},
		ExpectCSV: []string{
			"alpha: age=10, street-100",
			"bravo: age=20, street-200",
			"",
		},
	}.run(t)
	TqlTestCase{
		Name: "sqlite-to-html",
		Script: `
			SQL(bridge('sqlite'), "select id, name, age, address from example_sql")
			HTML({
{{- if .IsFirst }}<ul>{{ end }}
<li>{{ .V.id }}: {{ .V.name }}, {{ .V.age }}, {{ .V.address }}
{{ if .IsLast }}</ul>{{ end -}}
 				})
			`,
		ExpectText: []string{
			"<ul>",
			"<li>100: alpha, 10, street-100",
			"",
			"<li>200: bravo, 20, street-200",
			"</ul>",
		},
	}.run(t)
	TqlTestCase{
		Name: "sqlite-to-html-files",
		RunCondition: func() bool {
			return runtime.GOOS != "windows" // TODO: fix windows line endings
		},
		Script: `
			SQL(bridge('sqlite'), "select id, name, age, address from example_sql")
			HTML(file("/html_template_item.html"), file("/html_template_list.html"))
		`,
		ExpectText: []string{
			"<ul>",
			"<li>100: alpha, 10, street-100",
			"",
			"<li>200: bravo, 20, street-200",
			"</ul>",
		},
	}.run(t)
	TqlTestCase{
		Name: "sqlite-to-text",
		Script: `
			SQL(bridge('sqlite'), "select id, name, age, address from example_sql")
			TEXT({
			{{- if .IsFirst }}--begin--{{ end }}
- {{ .V.id }}: {{ .V.name }}, {{ .V.age }}, {{ .V.address }}
{{ if .IsLast }}--end--{{ end -}}
				})
			`,

		ExpectText: []string{
			"--begin--",
			"- 100: alpha, 10, street-100",
			"",
			"- 200: bravo, 20, street-200",
			"--end--",
		},
	}.run(t)
	TqlTestCase{
		Name: "sqlite-update-100",
		Script: `
			SQL(bridge('sqlite'), 'update example_sql set weight=? where id = ?', 45.67, 100)
			CSV(heading(false))
		`,
		ExpectCSV: []string{"a row updated.", "\n"},
	}.run(t)
	TqlTestCase{
		Name: "sqlite-update-200",
		Script: `
			SQL(bridge('sqlite'), 'update example_sql set weight=? where id = ?', 56.789, 200)
			CSV(heading(false))
		`,
		ExpectCSV: []string{"a row updated.", "\n"},
	}.run(t)
	TqlTestCase{
		Name: "sqlite-source-to-sink-insert",
		Script: `
			SQL(bridge('sqlite'), "select 400 as id, 'delta' as name, 40 as age, 'street-400' as address union all select 500, 'echo' as name, 50 as age, 'street-500' as address")
			SQL(bridge('sqlite'), "insert into example_sql(id,name,age,address) values(?,?,?,?)", value(0), value(1), value(2), value(3))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool())
			require.Equal(t, "success", gjson.Get(result, "reason").String())
			require.Equal(t, "2 rows inserted.", gjson.Get(result, "data.message").String())
		},
	}.run(t)
	TqlTestCase{
		Name: "sqlite-source-to-sink-count",
		Script: `
			SQL(bridge('sqlite'), "select count(*) as cnt from example_sql where id in (400,500)")
			JSON()
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool())
			require.Equal(t, `[[2]]`, gjson.Get(result, "data.rows").Raw)
		},
	}.run(t)
	TqlTestCase{
		Name: "sqlite-source-to-sink-cleanup",
		Script: `
			SQL(bridge('sqlite'), "delete from example_sql where id in (400,500)")
			CSV(heading(false))
		`,
		ExpectCSV: []string{"2 rows deleted.", "\n"},
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
		Name: "sqlite-delete-syntax-error",
		Script: `
			SQL(bridge('sqlite'), 'delete example_sql where id = ?', 100)
			CSV(heading(false))
			`,
		ExpectErr: "near \"example_sql\": syntax error",
	}.run(t)
	TqlTestCase{
		Name: "sqlite-delete-before-count",
		Script: `
			SQL(bridge('sqlite'), 'select count(*) from example_sql where id = ?', param('id'))
			JSON()
			`,
		Params: map[string][]string{"id": {"100"}},
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool())
			// FIXME: count(*) should be integer instead of string
			require.Equal(t, `{"columns":["count(*)"],"types":["string"],"rows":[[1]]}`, gjson.Get(result, "data").Raw)
		},
	}.run(t)
	TqlTestCase{
		Name: "sqlite-delete",
		Script: `
			SQL(bridge('sqlite'), 'delete from example_sql where id = ?', param('id'))
			CSV(heading(false))
			`,
		Params:    map[string][]string{"id": {"100"}},
		ExpectCSV: []string{"a row deleted.", "\n"},
	}.run(t)
	TqlTestCase{
		Name: "sqlite-delete-after-count",
		Script: `
			SQL(bridge('sqlite'), 'select count(*) from example_sql where id = ?', param('id'))
			JSON()
			`,
		Params: map[string][]string{"id": {"100"}},
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool())
			// FIXME: count(*) should be integer instead of string
			require.Equal(t, `{"columns":["count(*)"],"types":["string"],"rows":[[0]]}`, gjson.Get(result, "data").Raw)
		},
	}.run(t)
	TqlTestCase{
		Name: "sqlite-select-no-rows",
		Script: `
			SQL(bridge('sqlite'), "select * from example_sql where id = ?", param('id'))
			CSV(heading(true))
			`,
		Params:    map[string][]string{"id": {"-1"}},
		ExpectCSV: []string{"id,name,age,address,weight,memo", "\n"},
	}.run(t)
	TqlTestCase{
		Name: "sqlite-select-no-rows-no-header",
		Script: `
			SQL(bridge('sqlite'), "select * from example_sql where id = ?", param('id'))
			CSV(heading(false))
			`,
		Params:    map[string][]string{"id": {"-1"}},
		ExpectCSV: []string{"\n"},
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
			require.Equal(t, `["int64","string","int64","string"]`, gjson.Get(result, "data.types").Raw, result)
			require.Equal(t, `[300,"charlie",30,"street-300"]`, gjson.Get(result, "data.rows.0").Raw, result)
		},
	}.run(t)
}

func TestBinary(t *testing.T) {
	TqlTestCase{
		Name:       "create-tqlbin",
		CtxTimeout: 15 * time.Second,
		Script: `
			SCRIPT("js", {
				var ret = $.db().exec("create tag table tqlbin (name varchar(40) primary key, time datetime basetime, value binary)");
				if (ret instanceof Error) {
					$.yield(ret.message);
				} else {
					$.yield("create-tqlbin done");
				}
			})
			CSV()`,
		ExpectFunc: func(t *testing.T, result string) {
			require.Equal(t, "create-tqlbin done\n\n", result)
		},
	}.run(t)
	// INSERT binary data
	TqlTestCase{
		Name: "insert-binary",
		Script: `
			SCRIPT({
				$.yield('bin1', 1692686707380411000, '0x0102030405060708090a');
			})
			INSERT('name', 'time', 'value', table('tqlbin'))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.Contains(t, result, "1 row inserted.")
			conn, _ := spi.Connect(t.Context(), "sys")
			conn.ExecContext(t.Context(), "EXEC table_flush(tqlbin)")
			conn.Close()
		},
	}.run(t)
	TqlTestCase{
		Name: "select-binary-hex",
		Script: `
			SQL("select NAME, VALUE from tqlbin where name = 'bin1'")
			CSV(header(true))
		`,
		ExpectCSV: []string{
			"NAME,VALUE",
			"bin1,0x0102030405060708090a",
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "select-binary-bytes",
		Script: `
			SQL("select NAME, VALUE from tqlbin where name = 'bin1'")
			CSV(header(true), binaryformat('preview'))
		`,
		ExpectCSV: []string{
			"NAME,VALUE",
			"bin1,0x0102030405..",
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "select-binary-base64",
		Script: `
			SQL("select NAME, VALUE from tqlbin where name = 'bin1'")
			CSV(header(true), binaryformat('base64'))
		`,
		ExpectCSV: []string{
			"NAME,VALUE",
			"bin1,AQIDBAUGBwgJCg==",
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "select-binary-bytes",
		Script: `
			SQL("select NAME, VALUE from tqlbin where name = 'bin1'")
			CSV(header(true), binaryformat('bytes'))
		`,
		ExpectCSV: []string{
			"NAME,VALUE",
			"bin1,[1 2 3 4 5 6 7 8 9 10]",
			"\n",
		},
	}.run(t)
	// APPEND binary data
	TqlTestCase{
		Name: "append-binary",
		Script: `
			SCRIPT({
				$.yield('bin2', 1692686707380411000, '0x0102030405060708090a');
				$.yield('bin2', 1692686707380412000, '0x02030405060708090a10');
				$.yield('bin2', 1692686707380413000, '0x030405060708090a1011');
				$.yield('bin2', 1692686707380414000, '0x0405060708090a101213');
				$.yield('bin2', 1692686707380415000, '0x05060708090a10121314');
			})
			APPEND(table('tqlbin'))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.Contains(t, result, "append 5 rows (success 5, fail 0)")

			// flush appender
			spi.FlushAppendWorkers("tqlbin")

			// flush table
			conn, _ := spi.Connect(t.Context(), "sys")
			time.Sleep(3 * time.Second)
			conn.ExecContext(t.Context(), "EXEC table_flush(tqlbin)")
			conn.Close()
		},
	}.run(t)
	TqlTestCase{
		Name: "append-select-binary-hex",
		Script: `
			SQL("select NAME, VALUE from tqlbin where name = 'bin2'")
			CSV(header(true))
		`,
		ExpectCSV: []string{
			"NAME,VALUE",
			"bin2,0x0102030405060708090a",
			"bin2,0x02030405060708090a10",
			"bin2,0x030405060708090a1011",
			"bin2,0x0405060708090a101213",
			"bin2,0x05060708090a10121314",
			"\n",
		},
	}.run(t)
	// FLUSH before DROP TABLE
	TqlTestCase{
		Name: "flush-before-drop",
		Script: `
			FAKE( once(1) )
			DISCARD()`,
		ExpectFunc: func(t *testing.T, result string) {
			// flush appender workers to ensure all pending writes are done
			spi.FlushAppendWorkers("tqlbin")

			// flush table
			conn, _ := spi.Connect(t.Context(), "sys")
			time.Sleep(100 * time.Millisecond)
			conn.ExecContext(t.Context(), "EXEC table_flush(tqlbin)")
			conn.Close()
		},
	}.run(t)
	// DROP TABLE
	TqlTestCase{
		Name: "drop-table",
		Script: `
			SCRIPT("js", {
				var ret = $.db().exec("drop table tqlbin");
				if (ret instanceof Error) {
					console.error(ret.message);
				}
			})
			DISCARD()`,
		CtxTimeout: 30 * time.Second,
		ExpectFunc: func(t *testing.T, result string) {
			require.Empty(t, result)
		},
	}.run(t)
}

func TestSHELL(t *testing.T) {
	tql.ShellExecutable = func(addr, path string) ([]string, error) {
		return []string{"/bin/bash", path}, nil
	}
	TqlTestCase{
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
	}.run(t)
}

func TestCSV(t *testing.T) {
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	levelInfo := tql.INFO
	TqlTestCase{
		Name: "CSV_with_logProgress",
		Script: `
			CSV("1,line1\n2,line2\n3,\n4,line4", logProgress(2))
			CSV( heading(true) )
			`,
		LogLevel: &levelInfo,
		ExpectLog: []string{
			"[INFO] Loading 2 records",
			"[INFO] Loading 4 records",
		},
		ExpectCSV: []string{
			"column0,column1",
			"1,line1",
			"2,line2",
			"3,",
			"4,line4",
			"\n",
		},
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
		Name: "CSV_charset_jp",
		Script: `
			CSV(file("/euc-jp.csv"), charset("EUC-JP"))
			CSV()
			`,
		ExpectCSV: []string{
			`利用されてきた文字コー,1701913182,3.141592`,
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "CSV_nullValue",
		Script: `
			FAKE(json({ ["A", 123], ["B", null], ["C", 234] }))
			CSV( nullValue("<NULL>") )
		`,
		ExpectCSV: []string{
			"A,123",
			"B,<NULL>",
			"C,234",
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "CSV_substituteNull",
		Script: `
			FAKE(json({ ["A", 123], ["B", null], ["C", 234] }))
			CSV( substituteNull("<NULL>") )
		`,
		ExpectCSV: []string{
			"A,123",
			"B,<NULL>",
			"C,234",
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "CSV_nullValue_boolean",
		Script: `
			FAKE(json({ ["A", 123], ["B", null], ["C", 234] }))
			CSV( nullValue(false) )
		`,
		ExpectCSV: []string{
			"A,123",
			"B,false",
			"C,234",
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "CSV_nullValue_double",
		Script: `
			FAKE(json({ ["A", 123], ["B", null], ["C", 234] }))
			CSV( nullValue(3.14) )
		`,
		ExpectCSV: []string{
			"A,123",
			"B,3.14",
			"C,234",
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "CSV_nullValue_precision",
		Script: `
			FAKE(json({ ["A", 123], ["B", null], ["C", 234] }))
			CSV( nullValue(3.14), precision(1) )
		`,
		ExpectCSV: []string{
			"A,123.0",
			"B,3.1",
			"C,234.0",
			"\n",
		},
	}.run(t)
}

func TestJSON(t *testing.T) {
	TqlTestCase{
		Name: "CSV_to_JSON",
		Script: `
			CSV("A,123\nB,456\nC,789")
			JSON()
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool())
			require.Equal(t, "success", gjson.Get(result, "reason").String())
			require.Equal(t, `["column0","column1"]`, gjson.Get(result, "data.columns").String())
			require.Equal(t, `["string","string"]`, gjson.Get(result, "data.types").String())
			require.Equal(t, `[["A","123"],["B","456"],["C","789"]]`, gjson.Get(result, "data.rows").String())
		},
	}.run(t)
	TqlTestCase{
		Name: "JSON_empty_resultset",
		Script: `
			SQL("select * from tag_simple where name = 'no_name'")
			JSON()
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool())
			require.Equal(t, "success", gjson.Get(result, "reason").String())
			require.Equal(t, `["NAME","TIME","VALUE"]`, gjson.Get(result, "data.columns").String())
			require.Equal(t, `["string","datetime","double"]`, gjson.Get(result, "data.types").String())
			require.Equal(t, `[]`, gjson.Get(result, "data.rows").String())
		},
	}.run(t)
	TqlTestCase{
		Name: "JSON_no_yield",
		Script: `
			SCRIPT({
				$.result = {columns:["NAME","TIME","VALUE"],types:["string","datetime","double"]}
			},{
				$.db().query("select * from tag_simple where name = 'no_name'").forEach(r => console.log(r))
			})
			JSON()
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool())
			require.Equal(t, "success", gjson.Get(result, "reason").String())
			require.Equal(t, `["NAME","TIME","VALUE"]`, gjson.Get(result, "data.columns").String())
			require.Equal(t, `["string","datetime","double"]`, gjson.Get(result, "data.types").String())
			require.Equal(t, `[]`, gjson.Get(result, "data.rows").String())
		},
	}.run(t)
}

func TestMARKDOWN(t *testing.T) {
	TqlTestCase{
		Name: "MARKDOWN_invalid_option",
		Script: `STRING(file('/lines.txt'), separator('\\n'))
				MARKDOWN(true)`,
		ExpectErr: "line 2, column 1: encoder 'markdown' invalid option true (bool) [statement: MARKDOWN(true)]",
	}.run(t)
	TqlTestCase{
		Name: "MARKDOWN_html",
		Script: `
			STRING(file('/lines.txt'), separator('\n'))
			PUSHKEY('test')
			MARKDOWN(html(true))
		`,
		ExpectText: loadLines("./test/markdown_xhtml.txt"),
	}.run(t)
	TqlTestCase{
		Name: "MARKDOWN_html_false",
		Script: `
			STRING(file('/lines.txt'), separator('\n'))
			PUSHKEY('test')
			MARKDOWN(html(false))
		`,
		ExpectText: []string{
			"|ROWNUM|STRING|",
			"|:-----|:-----|",
			"|1|line1|",
			"|2|line2|",
			"|3||",
			"|4|line4|",
			"",
		},
	}.run(t)

	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
		Name: "CSV_payload_MAPVALUE_MARKDOWN_TEMPLATE",
		Script: `
			CSV(payload(), header(false))
			MAPVALUE(2, value(2) != "VALUE" ? parseFloat(value(2))*10 : value(2))
			MARKDOWN({
{{ if .IsFirst }}## demo
{{ end }}{{ .Value 0 }},{{ .Value 2 }}
{{ if .IsLast }}--------
{{ end }}
			})
			`,
		Payload: strings.Join([]string{
			`NAME,TIME,VALUE,BOOL`,
			`wave.sin,1676432361,0.000000,true`,
			`wave.cos,1676432361,1.0000000,false`,
			`wave.sin,1676432362,0.406736,true`,
			`wave.cos,1676432362,0.913546,false`,
			`wave.sin,1676432363,0.743144,true`,
		}, "\n") + "\n",
		ExpectFunc: func(t *testing.T, result string) {
			require.Contains(t, result, "## demo")
			require.Contains(t, result, "NAME,VALUE")
			require.Contains(t, result, "wave.sin,0")
			require.Contains(t, result, "wave.cos,10")
			require.Contains(t, result, "wave.sin,4.067")
			require.Contains(t, result, "wave.cos,9.135")
			require.Contains(t, result, "--------")
		},
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
}

func TestNDJSON(t *testing.T) {
	TqlTestCase{
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
	}.run(t)
}

func TestTql(t *testing.T) {
	TqlTestCase{
		Name: "pragma-log-level",
		Script: `
			#pragma log-level=warn
			FAKE( linspace(1, 5, 5))
			SCRIPT("js", { console.log("-", $.values[0]); $.yield($.values[0]) })
			JSON()
			`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool())
			require.Equal(t, "success", gjson.Get(result, "reason").String())
			require.Equal(t, `5`, gjson.Get(result, "data.rows.#").String())
		},
	}.run(t)
	TqlTestCase{
		Name: "strSprintf",
		Script: `
			FAKE(json(strSprintf('[%.f, %q]', 123, "hello")))
			CSV( heading(false) )
			`,
		ExpectCSV: []string{
			`123,hello`,
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "UTIL_sqlTimeformat_csv",
		Script: `
			FAKE( json({
				[1701345032123456789, 10],
				[1701345043219876543, 11]
			}))
			MAPVALUE(0, time(value(0)))
			CSV(sqlTimeformat("YYYY-MM-DD HH24:MI:SS.nnnnnn"), tz("Asia/Seoul"))
			`,
		ExpectCSV: []string{
			`2023-11-30 20:50:32.123456,10`,
			`2023-11-30 20:50:43.219876,11`,
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "UTIL_ansiTimeformat_csv",
		Script: `
			FAKE( json({
				[1701345032123456789, 10],
				[1701345043219876543, 11]
			}))
			MAPVALUE(0, time(value(0)))
			CSV(ansiTimeformat("yyyy-mm-dd hh:nn:ss.ffffff"), tz("UTC"))
			`,
		ExpectCSV: []string{
			`2023-11-30 11:50:32.123456,10`,
			`2023-11-30 11:50:43.219876,11`,
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "UTIL_string_trim_replace",
		Script: `
			FAKE( json({
				["prefix-hello-suffix"]
			}))
			MAPVALUE(0, strTrimPrefix(value(0), "prefix-"))
			MAPVALUE(0, strTrimSuffix(value(0), "-suffix"))
			MAPVALUE(0, strReplace(value(0), "l", "L", 1))
			CSV()
			`,
		ExpectCSV: []string{
			`heLlo`,
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "UTIL_string_predicates",
		Script: `
			FAKE( json({
				["prefix-hello-suffix"],
				["hello"]
			}))
			PUSHVALUE(1, strHasPrefix(value(0), "prefix-"))
			PUSHVALUE(2, strHasSuffix(value(0), "-suffix"))
			CSV()
			`,
		ExpectCSV: []string{
			`prefix-hello-suffix,true,true`,
			`hello,false,false`,
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "UTIL_string_replace_all",
		Script: `
			FAKE( json({
				["a-b-c"]
			}))
			MAPVALUE(0, strReplaceAll(value(0), "-", "_"))
			CSV()
			`,
		ExpectCSV: []string{
			`a_b_c`,
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "MAP_pushkey_manual",
		Script: `
			FAKE( linspace(1, 2, 2) )
			PUSHKEY("k")
			CSV()
			`,
		ExpectCSV: []string{
			`1,1`,
			`2,2`,
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "MAP_popkey_manual",
		Script: `
			FAKE( json({
				["TAG0", 1, 10],
				["TAG1", 2, 20]
			}))
			POPKEY()
			CSV()
			`,
		ExpectCSV: []string{
			`1,10`,
			`2,20`,
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "MAP_transpose_header_manual",
		Script: `
			FAKE(csv("CITY,DATE,TEMPERATURE,HUMIDITY\nTokyo,2023/12/07,23,30"))
			TRANSPOSE(header(true))
			CSV()
			`,
		ExpectCSV: []string{
			`CITY,Tokyo`,
			`DATE,2023/12/07`,
			`TEMPERATURE,23`,
			`HUMIDITY,30`,
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "MAP_take_offset_count_manual",
		Script: `
			FAKE( json({
				["TAG0", 1, 10],
				["TAG0", 2, 11],
				["TAG0", 3, 12],
				["TAG0", 4, 13],
				["TAG0", 5, 14],
				["TAG0", 6, 15]
			}))
			TAKE(3, 2)
			CSV()
			`,
		ExpectCSV: []string{
			`TAG0,4,13`,
			`TAG0,5,14`,
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "MAP_drop_offset_count_manual",
		Script: `
			FAKE( json({
				["TAG0", 1, 10],
				["TAG0", 2, 11],
				["TAG0", 3, 12],
				["TAG0", 4, 13],
				["TAG0", 5, 14],
				["TAG0", 6, 15]
			}))
			DROP(2, 3)
			CSV()
			`,
		ExpectCSV: []string{
			`TAG0,1,10`,
			`TAG0,2,11`,
			`TAG0,6,15`,
			"\n",
		},
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
		Name: "MAP_MOVAVG",
		Script: `
			FAKE( linspace(0, 100, 100) )
			MAP_MOVAVG(1, value(0), 10)
			CSV( precision(4) )
			`,
		ExpectCSV: loadLines("./test/movavg_result.csv"),
	}.run(t)
	TqlTestCase{
		Name: "MAP_MOVAVG_nowait",
		Script: `
			FAKE( linspace(0, 100, 100) )
			MAP_MOVAVG(1, value(0), 10, noWait(true))
			CSV( precision(4) )
			`,
		ExpectCSV: loadLines("./test/movavg_result_nowait.csv"),
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
		Name: "MAP_DIFF",
		Script: `
			FAKE( csv("1\n3\n2\n7") )
			MAP_DIFF(0, value(0))
			CSV()
			`,
		ExpectCSV: []string{"NULL", "2", "-1", "5", "\n"},
	}.run(t)
	TqlTestCase{
		Name: "MAP_NONEGDIFF",
		Script: `
			FAKE( csv("1\n3\n2\n7") )
			MAP_NONEGDIFF(0, value(0))
			CSV()
			`,
		ExpectCSV: []string{"NULL", "2", "0", "5", "\n"},
	}.run(t)
	TqlTestCase{
		Name: "MAP_ABSDIFF",
		Script: `
			FAKE( csv("1\n3\n2\n7") )
			MAP_ABSDIFF(0, value(0))
			CSV()
			`,
		ExpectCSV: []string{"NULL", "2", "1", "5", "\n"},
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
		Name: "FAKE_sphere_4_4",
		Script: `
			FAKE( sphere(4, 4) )
			PUSHKEY('test')
			CSV( header(true), precision(6) )
			`,
		ExpectCSV: loadLines("./test/sphere_4_4.csv"),
	}.run(t)
	TqlTestCase{
		Name: "FAKE_sphere_0_0",
		Script: `
			FAKE( sphere(0, 0) )
			PUSHKEY('test')
			CSV( header(false), precision(6) )
			`,
		ExpectCSV: loadLines("./test/sphere_0_0.csv"),
	}.run(t)
}

func TestFAKE(t *testing.T) {
	TqlTestCase{
		Name: "FAKE_invalid_generator_type",
		Script: `
			FAKE( 123 )
			CSV()
			`,
		ExpectErr: "f(FAKE) arg(0) should be fakeSource, but float64",
	}.run(t)
	TqlTestCase{
		Name: "FAKE_json",
		Script: `
			FAKE(
				json({
					["A", 1, true],
					["B", 2, false],
					["C", 3, true]
				})
			)
			MAPVALUE(1, value(1)*10)
			CSV()
			`,
		ExpectCSV: []string{
			`A,10,true`,
			`B,20,false`,
			`C,30,true`,
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "FAKE_csv",
		Script: `
			FAKE(
				csv(
					strTrimSpace(` + "`" + `
						A,1,true
						B,2,false
						C,3,true
					` + "`" + `)
				)
			)
			MAPVALUE(0, strTrimSpace(value(0)))
			MAPVALUE(1, parseFloat(value(1))*10)
			MAPVALUE(2, parseBool(value(2)))
			CSV()
			`,
		ExpectCSV: []string{
			`A,10,true`,
			`B,20,false`,
			`C,30,true`,
			"\n",
		},
	}.run(t)
}

func TestFAKE_arrange(t *testing.T) {
	TqlTestCase{
		Name: "FAKE_arrange_zero_step",
		Script: `FAKE( arrange(10, 30, 0) )
				CSV()`,
		ExpectErr: `FUNCTION "arrange" step can not be 0`,
	}.run(t)
	TqlTestCase{
		Name: "FAKE_arrange_start_stop_equal",
		Script: `FAKE( arrange(10, 10, 10) )
				CSV()`,
		ExpectErr: `FUNCTION "arrange" start, stop can not be equal`,
	}.run(t)
	TqlTestCase{
		Name: "FAKE_arrange_start_stop_invalid1",
		Script: `FAKE( arrange(10, 30, -10) )
				CSV()`,
		ExpectErr: `FUNCTION "arrange" step can not be less than 0`,
	}.run(t)
	TqlTestCase{
		Name: "FAKE_arrange_start_stop_invalid2",
		Script: `FAKE( arrange(30, 10, 10) )
				CSV()`,
		ExpectErr: `FUNCTION "arrange" step can not be greater than 0`,
	}.run(t)
	TqlTestCase{
		Name: "FAKE_arrange_csv",
		Script: `FAKE( arrange(0, 2, 1) )
				CSV( heading(true), precision(1) )`,
		ExpectCSV: []string{
			"x",
			"0.0",
			"1.0",
			"2.0",
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "FAKE_arrange_csv_desc",
		Script: `FAKE( arrange(2, 0, -1) )
				CSV( heading(true), precision(1) )`,
		ExpectCSV: []string{
			"x",
			"2.0",
			"1.0",
			"0.0",
			"\n",
		},
	}.run(t)
}

func TestFAKE_linspace(t *testing.T) {
	TqlTestCase{
		Name: "FAKE_linspace_wrong_args",
		Script: `
			FAKE( linspace(0, 1, -1))
			MARKDOWN()
		`,
		ExpectText: []string{
			`|x|`,
			`|:-----|`,
			"",
			"> *No record*",
			"",
		},
	}.run(t)

	TqlTestCase{
		Name: "FAKE_linspace",
		Script: `
			FAKE( linspace(0, 2, 3))
			CSV( heading(true), precision(1) )
		`,
		ExpectCSV: []string{
			"x",
			"0.0",
			"1.0",
			"2.0",
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "FAKE_linspace",
		Script: `
			FAKE( linspace(0, 1, 2))
			MARKDOWN()
		`,
		ExpectText: []string{
			`|x|`,
			`|:-----|`,
			`|0.000000|`,
			`|1.000000|`,
			"",
		},
	}.run(t)
	TqlTestCase{
		Name: "FAKE_linspace_md_rownum",
		Script: `
			FAKE( linspace(0, 1, 2))
			PUSHKEY('signal.md')
			MARKDOWN()`,
		ExpectText: []string{
			`|ROWNUM|x|`,
			`|:-----|:-----|`,
			`|1|0.000000|`,
			`|2|1.000000|`,
			"",
		},
	}.run(t)
}

func TestFAKE_meshgrid(t *testing.T) {
	TqlTestCase{
		Name: "FAKE_meshgrid",
		Script: `
			FAKE( meshgrid(linspace(1, 2, 2), linspace(10, 20, 2)) )
			CSV()`,
		ExpectCSV: []string{
			`1,10`,
			`1,20`,
			`2,10`,
			`2,20`,
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "FAKE_meshgrid_precision",
		Script: `
			FAKE( meshgrid(linspace(0, 2, 3), linspace(0, 2, 3)) )
			CSV( heading(true), precision(6) )
		`,
		ExpectCSV: []string{
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
			"\n",
		},
	}.run(t)
}

func TestFAKE_oscillator(t *testing.T) {
	back := util.StandardTimeNow
	util.StandardTimeNow = func() time.Time {
		return time.Unix(0, 1692329338315327000)
	}
	defer func() {
		util.StandardTimeNow = back
	}()

	TqlTestCase{
		Name: "FAKE_oscillator_no_args",
		Script: `
			FAKE( oscillator() )
			JSON()`,
		ExpectErr: "f(oscillator) no time range is defined",
	}.run(t)
	TqlTestCase{
		Name: "FAKE_oscillator_invalid_args",
		Script: `
			FAKE( oscillator(123) )
			JSON()`,
		ExpectErr: "f(oscillator) invalid arg type 'float64'",
	}.run(t)
	TqlTestCase{
		Name: "FAKE_oscillator_no_time_range",
		Script: `
			FAKE( oscillator(freq(1.0, 1.0)) )
			JSON()
		`,
		ExpectErr: "f(oscillator) no time range is defined",
	}.run(t)
	TqlTestCase{
		Name: "FAKE_oscillator_dup_time_range",
		Script: `
			FAKE( oscillator(freq(1.0, 1.0), range(time('now-1s'), '1s', '200ms'), range(time('now-1s'), '1s', '200ms')) )
			JSON()
		`,
		ExpectErr: "f(oscillator) duplicated time range",
	}.run(t)
	TqlTestCase{
		Name: "FAKE_oscillator_minus_time_range",
		Script: `
			FAKE( oscillator(freq(1.0, 1.0), range(time('now-1s'), '1s', '-200ms')) )
			JSON()
		`,
		ExpectErr: "f(oscillator) period should be positive",
	}.run(t)
	TqlTestCase{
		Name: "FAKE_oscillator_1",
		Script: `
			FAKE( oscillator(freq(1.0, 1.0), range(time('now-1s'), '1s', '200ms')) )
			JSON(precision(16))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool(), "result: %q", result)
			require.Equal(t, `["time","value"]`, gjson.Get(result, "data.columns").Raw, result)
			require.Equal(t, `["datetime","double"]`, gjson.Get(result, "data.types").Raw, result)
			require.Equal(t, `[1692329337315327000,0.9169371548618853]`, gjson.Get(result, "data.rows.0").Raw, result)
			require.Equal(t, `[[1692329337315327000,0.9169371548618853],[1692329337515327000,-0.0961529923781393],[1692329337715327000,-0.9763628786653529],[1692329337915327000,-0.5072715014883364],[1692329338115327000,0.6628509149282410]]`, gjson.Get(result, "data.rows").Raw, result)
		},
	}.run(t)
	TqlTestCase{
		Name: "FAKE_oscillator_2",
		Script: `
			FAKE( oscillator(freq(1.0, 1.0), range(time('now'), '-1s', '200ms')) )
			JSON(precision(16))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool(), "result: %q", result)
			require.Equal(t, `["time","value"]`, gjson.Get(result, "data.columns").Raw, result)
			require.Equal(t, `["datetime","double"]`, gjson.Get(result, "data.types").Raw, result)
			require.Equal(t, `[1692329337315327000,0.9169371548618853]`, gjson.Get(result, "data.rows.0").Raw, result)
			require.Equal(t, `[[1692329337315327000,0.9169371548618853],[1692329337515327000,-0.0961529923781393],[1692329337715327000,-0.9763628786653529],[1692329337915327000,-0.5072715014883364],[1692329338115327000,0.6628509149282410]]`, gjson.Get(result, "data.rows").Raw, result)
		},
	}.run(t)
	TqlTestCase{
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
	}.run(t)
}

func TestFAKE_statz(t *testing.T) {
	var mustSeriesID = func(t *testing.T, id, title string, period time.Duration, maxCount int) metric.SeriesID {
		t.Helper()
		seriesID, err := metric.NewSeriesID(id, title, period, maxCount)
		require.NoError(t, err)
		return seriesID
	}
	seriesID := mustSeriesID(t, "CPU_USAGE", "CPU Usage", time.Second, 8)
	collector := metric.NewCollector(
		metric.WithSamplingInterval(time.Second),
		metric.WithSeries(seriesID),
	)
	collector.Start()
	t.Cleanup(collector.Stop)

	collector.Send(metric.Measure{Name: "cpu:usage", Value: 1, Type: metric.CounterType(metric.UnitScalar)})
	time.Sleep(1100 * time.Millisecond)
	collector.Send(metric.Measure{Name: "cpu:usage", Value: 2, Type: metric.CounterType(metric.UnitScalar)})

	org := spi.SetCollector(collector)
	defer spi.SetCollector(org)

	require.Eventually(t, func() bool {
		mts := collector.Timeseries("cpu:usage")
		if len(mts) == 0 {
			return false
		}
		_, values := mts[0].All()
		samples := make([]float64, 0, len(values))
		for _, raw := range values {
			v, ok := raw.(*metric.CounterValue)
			if !ok || v.Samples == 0 {
				continue
			}
			samples = append(samples, v.Value)
		}
		if len(samples) < 2 {
			return false
		}
		return samples[len(samples)-2] == 1 && samples[len(samples)-1] == 2
	}, 3*time.Second, 50*time.Millisecond)

	TqlTestCase{
		Name: "FAKE_statz",
		Script: `
			FAKE( statz(0, 'cpu:usage') )
			FILTER( value(1) != NULL )
			CSV(timeformat('15:04:05'), heading(true), precision(0))`,
		ExpectFunc: func(t *testing.T, result string) {
			lines := strings.Split(result, "\n")
			require.Equal(t, "time,cpu:usage", lines[0])
			// 2026-06-12 08:14:22,1
			require.True(t, regexp.MustCompile(`^[0-9]{2}:[0-9]{2}:[0-9]{2},1$`).MatchString(lines[1]), "line: %q", lines[1])
			// 2026-06-12 08:14:23,2
			require.True(t, regexp.MustCompile(`^[0-9]{2}:[0-9]{2}:[0-9]{2},2$`).MatchString(lines[2]), "line: %q", lines[2])
			require.Equal(t, "", lines[3])
		},
	}.run(t)
}

func TestSTRING(t *testing.T) {
	TqlTestCase{
		Name: "string",
		Script: `
			STRING("line1\nline2\n\nline4", separator("\n"))
			PUSHKEY('test')
			CSV( heading(true) )
		`,
		ExpectCSV: []string{
			"ROWNUM,STRING",
			"1,line1",
			"2,line2",
			"3,",
			"4,line4",
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "string_file",
		Script: `
			STRING(file("/lines.txt"), separator("\n"), trimspace(true))
			PUSHKEY('test')
			CSV( heading(true) )
		`,
		ExpectCSV: []string{
			"ROWNUM,STRING",
			"1,line1",
			"2,line2",
			"3,",
			"4,line4",
			"\n",
		},
	}.run(t)
}

func TestBYTES(t *testing.T) {
	TqlTestCase{
		Name: "bytes",
		Script: `
			BYTES("line1\nline2\n\nline4", separator("\n"))
			PUSHKEY('test')
			CSV( heading(true), binaryformat("hex") )
		`,
		ExpectCSV: []string{
			"ROWNUM,BYTES",
			`1,0x6c696e6531`,
			`2,0x6c696e6532`,
			`3,`,
			`4,0x6c696e6534`,
			"\n",
		},
	}.run(t)

	TqlTestCase{
		Name: "bytes_file",
		Script: `
			BYTES(file("/lines.txt"), separator("\n"))
			CSV( header(true), binaryformat("hex") )
		`,
		ExpectCSV: []string{
			"BYTES",
			`0x6c696e6531`,
			`0x6c696e6532`,
			``,
			`0x6c696e6534`,
			"\n",
		},
	}.run(t)
}

func TestTEXT_template(t *testing.T) {
	TqlTestCase{
		Name: "js-array-template",
		Script: `
				SCRIPT({
					$.yield(1, 2, 3);
					$.yield(4, 5, 6);
				})
				TEXT('{{- .Value 0 }},{{ .Value 1 }},{{ .Value 2 }}{{"\\n"}}')
			`,
		ExpectFunc: func(t *testing.T, result string) {
			require.Equal(t, "1,2,3\n4,5,6\n", result, result)
		},
	}.run(t)
	TqlTestCase{
		Name: "js-obj-template",
		Script: `
			SCRIPT({
				$.yield("John", 30);
				$.yield("Jane", 25);
			})
			TEXT({
				{{- with .V -}}
					{{ .column0 }}:{{ .column1 }}{{"\n"}}
				{{- end -}}
			})
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.Equal(t, "John:30\nJane:25\n", result, result)
		},
	}.run(t)
	TqlTestCase{
		Name: "js-obj-template",
		Script: `
			SCRIPT({
				$.result = {
					columns: ["name", "age"],
					types: ["string", "int64"]
				};
				$.yield("John", 30);
				$.yield("Jane", 25);
			})
			TEXT({
				{{- with .V -}}
					{{ .name }}:{{ .age }}{{"\n"}}
				{{- end -}}
			})
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.Equal(t, "John:30\nJane:25\n", result, result)
		},
	}.run(t)
	TqlTestCase{
		Name: "js-obj-template",
		Script: `
			SCRIPT({
				$.yield({name: "John", age: 30});
				$.yield({name: "Jane", age: 25});
			})
			TEXT({
				{{- with .Value 0 -}}
					{{ .name }}:{{ .age }}{{"\n"}}
				{{- end -}}
			})
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.Equal(t, "John:30\nJane:25\n", result, result)
		},
	}.run(t)
}

func TestGEOMAP(t *testing.T) {
	TqlTestCase{
		Name: "js-geojson-point",
		Script: `
			SCRIPT("js", {
				var lat = 37.497850;
				var lon =  127.027756;
				var name = "Gangnam-cross";
				$.yield({
					type: "Feature",
					geometry: {
						type: "Point",
						coordinates: [lon, lat]
					}
				});
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
	}.run(t)
	TqlTestCase{
		Name: "js-parse-geojson-point",
		Script: `
				SCRIPT("js", {
					var lat = 37.497850;
					var lon =  127.027756;
					var name = "Gangnam-cross";
					m = require("mathx/spatial");
					var obj = m.parseGeoJSON({
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
	}.run(t)
	TqlTestCase{
		Name: "js-geojson-polygon",
		Script: `
				SCRIPT("js", {
					m = require("mathx/spatial");
					obj = m.parseGeoJSON({
						type:"Feature",
						geometry: {
							type: "MultiPolygon",
							coordinates: [
								[
									[ [ 2.291863239086439, 48.8577137262115 ], [ 2.293452085617105, 48.856693553273885 ], [ 2.2968403487010107, 48.85892279314069 ], [ 2.2951175030651143, 48.86006886087142 ], [ 2.291863239086439, 48.8577137262115 ] ]
								],
								[
									[ [ 2.288226120523035, 48.86156752523257 ], [ 2.2899681088877344, 48.86042149181674 ], [ 2.290810388976098, 48.86063558796482 ], [ 2.2909826735397587, 48.8611015587675 ], [ 2.28947039792655, 48.862234983151495 ], [ 2.288226120523035, 48.86156752523257 ] ]
								],
								[
									[ [ 2.2912927602678224, 48.85709062155263 ], [ 2.2905402133688426, 48.85661663833349 ], [ 2.291917551492446, 48.855746990243716 ], [ 2.2926328654095016, 48.85624492205244 ], [ 2.2912927602678224, 48.85709062155263 ] ]
								]
							]
						}
					})
					$.yield(obj)
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
			b, _ := os.ReadFile("./test/js-geojson-polygon.js")
			expect := strings.ReplaceAll(string(b), "\r\n", "\n")
			require.Equal(t, expect, mock.buff.String())
		},
	}.run(t)
}

func TestTHROTTLE(t *testing.T) {
	t.Skip("throttle test is not stable")
	TqlTestCase{
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
	}.run(t)
}

func TestHISTOGRAM(t *testing.T) {
	TqlTestCase{
		Name: "histogram",
		Script: `
			FAKE( arrange(1, 100, 1) )
			MAPVALUE(0, (simplex(12, value(0)) + 1) * 100)
			HISTOGRAM(value(0), bins(0, 200, 20))
			CSV( precision(0) )`,
		ExpectCSV: []string{
			"0,20,0",
			"20,40,2",
			"40,60,12",
			"60,80,19",
			"80,100,25",
			"100,120,22",
			"120,140,8",
			"140,160,8",
			"160,180,4",
			"180,200,0",
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "histogram_bins",
		Script: `
			FAKE( arrange(1, 100, 1) )
			MAPVALUE(0, (simplex(12, value(0)) + 1) * 100)
			HISTOGRAM(value(0), bins(80, 120, 13))
			CSV( precision(0), header(true) )`,
		ExpectCSV: []string{
			"low,high,count",
			"-Inf,80,19",
			"80,93,28",
			"93,106,19",
			"106,119,14",
			"119,+Inf,20",
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "histogram_bins2",
		Script: `
			FAKE( arrange(1, 100, 1) )
			MAPVALUE(0, (simplex(12, value(0)) + 1) * 100)
			HISTOGRAM(value(0), bins(20, 180, 20))
			CSV( header(true), precision(0) )
		`,
		ExpectCSV: []string{
			"low,high,count",
			"20,40,2",
			"40,60,12",
			"60,80,19",
			"80,100,25",
			"100,120,22",
			"120,140,8",
			"140,160,8",
			"160,180,4",
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "histogram_category",
		Script: `
		FAKE( arrange(1, 100, 1) )
		MAPVALUE(0, (simplex(12, value(0)) + 1) * 100)
		PUSHVALUE(0, key() % 2 == 0 ? "Cat.A" : "Cat.B")
		HISTOGRAM(value(1), bins(0, 200, 20), category(value(0)), order("Cat.B", "Cat.A"))
		CSV( header(true), precision(0) )`,
		ExpectCSV: []string{
			"low,high,Cat.B,Cat.A",
			"0,20,0,0",
			"20,40,1,1",
			"40,60,5,7",
			"60,80,6,13",
			"80,100,14,11",
			"100,120,14,8",
			"120,140,4,4",
			"140,160,5,3",
			"160,180,1,3",
			"180,200,0,0",
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "histogram_unpredicatedBins",
		Script: `
			FAKE( arrange(1, 100, 1) )
			MAPVALUE(0, (simplex(12, value(0)) + 1) * 100)
			HISTOGRAM(value(0), bins(10))
			CSV( header(true), precision(0) )`,
		ExpectCSV: []string{
			"value,count",
			"23,1",
			"44,6",
			"59,12",
			"80,26",
			"99,20",
			"113,18",
			"129,5",
			"141,2",
			"153,7",
			"170,3",
			"\n",
		},
	}.run(t)
}

func TestBOXPLOT(t *testing.T) {
	src := `FAKE(json({
			["A", 850, 740, 900, 1070, 930, 850, 950, 980, 980, 880, 1000, 980, 930, 650, 760, 810, 1000, 1000, 960, 960],
			["B", 960, 940, 960, 940, 880, 800, 850, 880, 900, 840, 830, 790, 810, 880, 880, 830, 800, 790, 760, 800],
			["C", 880, 880, 880, 860, 720, 720, 620, 860, 970, 950, 880, 910, 850, 870, 840, 840, 850, 840, 840, 840],
			["D", 890, 810, 810, 820, 800, 770, 760, 740, 750, 760, 910, 920, 890, 860, 880, 720, 840, 850, 850, 780],
			["E", 890, 840, 780, 810, 760, 810, 790, 810, 820, 850, 870, 870, 810, 740, 810, 940, 950, 800, 810, 870]
		}))
		`
	TqlTestCase{
		Name: "boxplot",
		Script: src + `
			TRANSPOSE(fixed(0))
			BOXPLOT(value(1), category(value(0)), order("A", "D","C","B","E"), boxplotInterp(true, false, true))
			FILTER(value(0) != "OUTLIER")
			CSV( header(true), precision(0) )`,
		ExpectCSV: []string{
			"CATEGORY,A,D,C,B,E",
			"MIN,650,720,620,760,740",
			"LOWER,655,610,780,680,695",
			"Q1,850,760,840,800,800",
			"Q2,930,810,850,840,810",
			"Q3,980,860,880,880,870",
			"UPPER,1175,1010,940,1000,975",
			"MAX,1070,920,970,960,950",
			"IQR,130,100,40,80,70",
			"\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "boxplot_dict",
		Script: src + `
			TRANSPOSE(fixed(0))
			BOXPLOT(value(1), category(value(0)), order("A", "D","C","B","E"), boxplotInterp(true, false, true), boxplotOutput("dict"))
			JSON()
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool())
			require.Equal(t, `success`, gjson.Get(result, "reason").String())
			require.Equal(t, `["A","D","C","B","E"]`, gjson.Get(result, "data.columns").String())
			require.Equal(t, `["dict","dict","dict","dict","dict"]`, gjson.Get(result, "data.types").String())
			require.Equal(t, int64(130), gjson.Get(result, "data.rows.0.0.iqr").Int())
			require.Equal(t, int64(655), gjson.Get(result, "data.rows.0.0.lower").Int())
			require.Equal(t, int64(1070), gjson.Get(result, "data.rows.0.0.max").Int())
			require.Equal(t, int64(650), gjson.Get(result, "data.rows.0.0.min").Int())
			require.Equal(t, int64(850), gjson.Get(result, "data.rows.0.0.q1").Int())
			require.Equal(t, int64(930), gjson.Get(result, "data.rows.0.0.q2").Int())
			require.Equal(t, int64(980), gjson.Get(result, "data.rows.0.0.q3").Int())
			require.Equal(t, int64(1175), gjson.Get(result, "data.rows.0.0.upper").Int())
			require.Equal(t, `[650]`, gjson.Get(result, "data.rows.0.0.outlier").String())

			require.Equal(t, int64(100), gjson.Get(result, "data.rows.0.1.iqr").Int())
			require.Equal(t, int64(610), gjson.Get(result, "data.rows.0.1.lower").Int())
			require.Equal(t, int64(920), gjson.Get(result, "data.rows.0.1.max").Int())
			require.Equal(t, int64(720), gjson.Get(result, "data.rows.0.1.min").Int())
			require.Equal(t, int64(760), gjson.Get(result, "data.rows.0.1.q1").Int())
			require.Equal(t, int64(810), gjson.Get(result, "data.rows.0.1.q2").Int())
			require.Equal(t, int64(860), gjson.Get(result, "data.rows.0.1.q3").Int())
			require.Equal(t, int64(1010), gjson.Get(result, "data.rows.0.1.upper").Int())
			require.Equal(t, ``, gjson.Get(result, "data.rows.0.1.outlier").String())

			require.Equal(t, int64(40), gjson.Get(result, "data.rows.0.2.iqr").Int())
			require.Equal(t, int64(780), gjson.Get(result, "data.rows.0.2.lower").Int())
			require.Equal(t, int64(970), gjson.Get(result, "data.rows.0.2.max").Int())
			require.Equal(t, int64(620), gjson.Get(result, "data.rows.0.2.min").Int())
			require.Equal(t, int64(840), gjson.Get(result, "data.rows.0.2.q1").Int())
			require.Equal(t, int64(850), gjson.Get(result, "data.rows.0.2.q2").Int())
			require.Equal(t, int64(880), gjson.Get(result, "data.rows.0.2.q3").Int())
			require.Equal(t, int64(940), gjson.Get(result, "data.rows.0.2.upper").Int())
			require.Equal(t, `[620,720,720,950,970]`, gjson.Get(result, "data.rows.0.2.outlier").String())

			require.Equal(t, int64(80), gjson.Get(result, "data.rows.0.3.iqr").Int())
			require.Equal(t, int64(680), gjson.Get(result, "data.rows.0.3.lower").Int())
			require.Equal(t, int64(960), gjson.Get(result, "data.rows.0.3.max").Int())
			require.Equal(t, int64(760), gjson.Get(result, "data.rows.0.3.min").Int())
			require.Equal(t, int64(800), gjson.Get(result, "data.rows.0.3.q1").Int())
			require.Equal(t, int64(840), gjson.Get(result, "data.rows.0.3.q2").Int())
			require.Equal(t, int64(880), gjson.Get(result, "data.rows.0.3.q3").Int())
			require.Equal(t, int64(1000), gjson.Get(result, "data.rows.0.3.upper").Int())
			require.Equal(t, ``, gjson.Get(result, "data.rows.0.3.outlier").String())

			require.Equal(t, int64(70), gjson.Get(result, "data.rows.0.4.iqr").Int())
			require.Equal(t, int64(695), gjson.Get(result, "data.rows.0.4.lower").Int())
			require.Equal(t, int64(950), gjson.Get(result, "data.rows.0.4.max").Int())
			require.Equal(t, int64(740), gjson.Get(result, "data.rows.0.4.min").Int())
			require.Equal(t, int64(800), gjson.Get(result, "data.rows.0.4.q1").Int())
			require.Equal(t, int64(810), gjson.Get(result, "data.rows.0.4.q2").Int())
			require.Equal(t, int64(870), gjson.Get(result, "data.rows.0.4.q3").Int())
			require.Equal(t, int64(975), gjson.Get(result, "data.rows.0.4.upper").Int())
			require.Equal(t, ``, gjson.Get(result, "data.rows.0.4.outlier").String())
		},
	}.run(t)
	TqlTestCase{
		Name: "boxplot_chart",
		Script: src + `
			TRANSPOSE(fixed(0))
			BOXPLOT(value(1), category(value(0)), order("A", "D","C","B","E"), boxplotInterp(true, false, true), boxplotOutput("chart"))
			CSV(header(true))`,
		ExpectCSV: []string{
			"CATEGORY,BOXPLOT,OUTLIER",
			"A,[]interface {},[]interface {}",
			"D,[]interface {},[]interface {}",
			"C,[]interface {},[]interface {}",
			"B,[]interface {},[]interface {}",
			"E,[]interface {},[]interface {}",
			"\n",
		},
	}.run(t)
}

func TestFFT(t *testing.T) {
	TqlTestCase{
		Name: "FFT",
		Script: `
			FAKE( oscillator( range(timeAdd(1685714509*1000000000,'1s'), '1s', '100us'), freq(10, 1.0), freq(50, 2.0)))
			MAPKEY('samples')
			GROUPBYKEY(lazy(false))
			FFT(minHz(0), maxHz(60))
			CSV(precision(6))
			`,
		ExpectCSV: loadLines("./test/fft2d.csv"),
	}.run(t)
	TqlTestCase{
		Name: "FFT_not_enough_samples_0",
		Script: `
			FAKE( linspace(0, 10, 100) )
			FFT()
			CSV()
			`,
		ExpectCSV: []string{"\n"},
	}.run(t)
	TqlTestCase{
		Name: "FFT_not_enough_samples_16",
		Script: `
			FAKE( meshgrid(linspace(0, 10, 100), linspace(0, 10, 1000)) )
			PUSHKEY('sample')
			GROUPBYKEY()
			FFT()
			CSV()
			`,
		ExpectErr: "f(FFT) sample should be a tuple of (time, value), but len=3",
	}.run(t)
	TqlTestCase{
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
	}.run(t)
}

func TestHTTP(t *testing.T) {
	TqlTestCase{
		Name: "rest-client-query-csv",
		Script: fmt.Sprintf(`
			HTTP({
				GET %s/db/query
				?q=select * from tag_simple limit 2
				&format=csv
			})
			TEXT()
			`, testHttpAddress),
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, strings.HasPrefix(result, "HTTP/1.1 200 OK"))
			require.Contains(t, result, "Content-Type: text/csv")
		},
	}.run(t)

}

func TestDISCARD(t *testing.T) {
	TqlTestCase{
		Name: "discard",
		Script: `
			#pragma log-level=INFO
			CSV("1,line-1\n2,line-2\n3,line-3")
			MAPVALUE(0, parseFloat(value(0)))
			WHEN(
				value(0) == 2 && 
				strHasPrefix( strToUpper(value(1)), "LINE-") &&
				strHasSuffix(value(1), "-2"),
				do(value(0), strToUpper(value(1)), {
					ARGS()
					WHEN(true, doLog("OUTPUT:", value(0), strToLower(value(1)) ))
					CSV()
				})
			)
			DISCARD()
		`,
		ExpectLog: []string{
			"[WARN] do: CSV() sink does not work in a sub-routine",
			"[INFO] OUTPUT: 2 line-2",
		},
		ExpectFunc: func(t *testing.T, result string) {
			require.Empty(t, result)
		},
	}.run(t)
	TqlTestCase{
		Name: "discard-utf8",
		Script: `
			#pragma log-level=INFO
			FAKE( json({         
				[ 1, "hello" ],   
				[ 2, "你好"],   
				[ 3, "world" ],
				[ 4, "世界"]
			}))
			WHEN(
				mod(value(0), 2) == 0,
				do( value(0), strToUpper(value(1)), {
					ARGS()
					WHEN( true, doLog("OUTPUT:", value(0), value(1)))
					DISCARD()
				})
			)
			CSV()
		`,
		ExpectLog: []string{
			"[INFO] OUTPUT: 2 你好",
			"[INFO] OUTPUT: 4 世界",
		},
		ExpectCSV: []string{
			"1,hello",
			"2,你好",
			"3,world",
			"4,世界",
			"\n",
		},
	}.run(t)
}

func TestSCRIPT_fft(t *testing.T) {
	TqlTestCase{
		Name: "js-fft",
		Script: `
			FAKE( oscillator( range(timeAdd(1685714509*1000000000,'1s'), '1s', '100us'), freq(10, 1.0), freq(50, 2.0)))
			SCRIPT("js", {
				m = require("mathx");
				times = [];
				values = [];
			}, {
				times.push($.values[0]);
				values.push($.values[1]);
			}, {
				result = m.fft(times, values);
				for( i = 0; i < result.length; i++ ) {
					if (result[i][0] > 60)
						break
					$.yield(result[i][0], result[i][1])
				}
			})
			CSV(precision(6))
			`,
		ExpectCSV: loadLines("./test/fft2d.csv"),
	}.run(t)
	TqlTestCase{
		Name: "js-fft_not_enough_samples_0",
		Script: `
			FAKE( linspace(0, 10, 100) )
			SCRIPT("js", {
				m = require("mathx");
				times = [];
				values = [];
			}, {
				times.push($.values[0]);
				values.push($.values[1]);
			}, {
				try{
					result = m.fft(times, values);
					for( i = 0; i < result.length; i++ ) {
						if (result[i][0] > 60)
							break
						$.yield(result[i][0], result[i][1])
					}
				} catch (e) {
					console.error(e.message);
				}
			})
			CSV()
			`,
		ExpectLog: []string{"[ERROR] fft invalid 0th sample value, but <nil>"},
		ExpectCSV: []string{"\n"},
	}.run(t)
}

func TestSCRIPT(t *testing.T) {
	TqlTestCase{
		Name: "script_src",
		Script: `
			SCRIPT({
				for (i = 0; i < 10; i++) {
					$.yieldKey("test", i, i*10)
				}
			})
			CSV()
		`,
		ExpectCSV: []string{
			"0,0", "1,10", "2,20", "3,30", "4,40", "5,50", "6,60", "7,70", "8,80", "9,90", "\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "script_src_map",
		Script: `
			SCRIPT({
				a = 10*2+1
				// comment

				$.yield(a)
			})
			SCRIPT({
				a = $.values[0];
				$.yield(a+1, 2, 3, 4)
			})
			CSV()
			`,
		ExpectCSV: []string{"22,2,3,4", "\n"},
	}.run(t)
	TqlTestCase{
		Name: "script_2",
		Script: `
			FAKE( linspace(1,2,2))
			MAPKEY("hello")
			SCRIPT("js", {
				c = 0;
				if ($.params.temp !== undefined) {
					c = $.params.temp;
				}
				$.yield($.key, $.values[0], c)
			})
			MAPVALUE(0, value(0), "key")
			MAPVALUE(1, value(1), "value")
			MAPVALUE(2, value(2), "parameter")
			CSV(header(true))
		`,
		ExpectCSV: []string{
			`key,value,parameter`, `hello,1,0`, `hello,2,0`, "\n",
		},
	}.run(t)
	TqlTestCase{
		Name: "js-console-log",
		Script: `
			SCRIPT("js", "console.log('Hello, World!')")
			DISCARD()`,
		ExpectFunc: func(t *testing.T, result string) {
			require.Empty(t, result)
		},
	}.run(t)
	TqlTestCase{
		Name: "js-console-log",
		Script: `
			#pragma log-level=INFO
			SCRIPT("js", "console.log('Hello, World!'); console.println('Hi Everyone!');")
			DISCARD()`,
		ExpectLog: []string{
			"[INFO] Hello, World!",
			"[INFO] Hi Everyone!",
		},
		ExpectFunc: func(t *testing.T, result string) {
			require.Empty(t, result)
		},
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
		Name: "js-yieldArray-number",
		Script: `
			STRING('1.2,2.3,3.4,5.6', separator('\n'))
			SCRIPT("js", {
				$.yieldArray($.values[0].split(',').map( (v) => { return parseFloat(v) }))
			})
			JSON()
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool())
			require.Equal(t, `["STRING"]`, gjson.Get(result, "data.columns").Raw)
			require.Equal(t, `["string"]`, gjson.Get(result, "data.types").Raw)
			require.Equal(t, `[[1.2,2.3,3.4,5.6]]`, gjson.Get(result, "data.rows").Raw)
		},
	}.run(t)
	TqlTestCase{
		Name: "js-yieldArray-number-mixed",
		Script: `
			SCRIPT("js", {
				$.result = {
					columns: ["a", "b", "c", "d"],
					types: ["int64", "double", "string", "bool"]
				};
				var arr = [1, 2.3, '3.4', true];
				$.yield(...arr);
			})
			JSON()
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool(), "success message should be true: %s", result)
			require.Equal(t, `["a","b","c","d"]`, gjson.Get(result, "data.columns").Raw)
			require.Equal(t, `["int64","double","string","bool"]`, gjson.Get(result, "data.types").Raw)
			require.Equal(t, `[[1,2.3,"3.4",true]]`, gjson.Get(result, "data.rows").Raw)
		},
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
		Name: "js-yield-object",
		Script: `
			SCRIPT("js", {
				$.yield({name:"John", age: 30, flag: true});
				$.yield({name:"Jane", age: 25, flag: false});
			})
			JSON(rowsFlatten(true))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool())
			require.Equal(t, `["column0"]`, gjson.Get(result, "data.columns").Raw)
			require.Equal(t, `["any"]`, gjson.Get(result, "data.types").Raw)
			require.Equal(t, `{"age":30,"flag":true,"name":"John"}`, gjson.Get(result, "data.rows.0").Raw)
			require.Equal(t, `{"age":25,"flag":false,"name":"Jane"}`, gjson.Get(result, "data.rows.1").Raw)
		},
	}.run(t)
	TqlTestCase{
		Name: "js-system-free-os-memory",
		Script: `
			SCRIPT("js", {
				m = require("@jsh/system");
				m.free_os_memory();
				$.yield("ok");
			})
			CSV()
		`,
		ExpectCSV: []string{"ok", "\n"},
	}.run(t)
	TqlTestCase{
		Name: "js-system-gc",
		Script: `
			SCRIPT("js", {
				m = require("@jsh/system");
				m.gc();
				$.yield("ok");
			})
			CSV()
		`,
		ExpectCSV: []string{"ok", "\n"},
	}.run(t)
	TqlTestCase{
		Name: "js-system-now",
		Script: `
			SCRIPT("js", {
				m = require("@jsh/system");
				let now = m.now();
				$.yield("ok", now.unix());
			})
			JSON()
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool())
			require.Equal(t, `["column0","column1"]`, gjson.Get(result, "data.columns").Raw)
			require.Equal(t, `["string","int64"]`, gjson.Get(result, "data.types").Raw)
			require.NotEmpty(t, gjson.Get(result, "data.rows").Raw)
		},
	}.run(t)
	TqlTestCase{
		Name:    "js-payload-csv",
		Payload: `1,2,3,4,5`,
		Script: `
			SCRIPT("js", {
				$.payload.split(",").forEach((v) => {
					$.yield(parseInt(v));
				});
			})
			CSV()`,
		ExpectCSV: []string{"1", "2", "3", "4", "5", "\n"},
	}.run(t)
	TqlTestCase{
		Name: "js-compile-err",
		Script: `
			SCRIPT("js", {
				var1 + 1;
			})
			CSV()`,
		ExpectErr: `ReferenceError: var1 is not defined at SCRIPT main:3:5(0)`,
	}.run(t)
	TqlTestCase{
		Name: "js-invalid-module",
		Script: `
			SCRIPT("js", {
				// hello world
				//
				//
				//
				const y = require("invalid_module");
			})
			CSV()`,
		ExpectErr: `Invalid module, SCRIPT main:7:22`,
	}.run(t)
	TqlTestCase{
		Name: "js-params",
		Script: `
			SCRIPT("js", {
				var1 = $.params.p1;
				var2 = $.params["p2"];
				$.yield(...var1, var2);
			})
			CSV()`,
		Params:    map[string][]string{"p1": {"1", "2"}, "p2": {"abc"}},
		ExpectCSV: []string{"1,2,abc", "\n"},
	}.run(t)
}

func TestSCRIPT_exception(t *testing.T) {
	TqlTestCase{
		Name: "js-exception",
		Script: `
			SCRIPT("js", {
				o = {a: 1, other: ()=>{throw "other error";}};
				o.a++;
				$.yield(o.a)
				try {
					o.undef_function();
				} catch (e) {
					console.error(e.message);
				}
				try {
					o.other();
				} catch (e) {
					console.error(e);
				}
			})
			CSV()
		`,
		ExpectLog: []string{
			"[ERROR] Object has no member 'undef_function'",
			"[ERROR] other error",
		},
		ExpectCSV: []string{"2", "\n"},
	}.run(t)
}

func TestSCRIPT_interrupt(t *testing.T) {
	requireNoPayload := func(t *testing.T, result string) {
		// Timeout interrupts may flush either "" or "\n" depending on writer/runtime timing.
		// The semantic contract is that no payload rows are produced.
		require.Equal(t, "", strings.TrimSpace(result))
	}

	// Give the JS runtime enough time to start on slower CI runners so these
	// cases validate interrupt handling rather than startup scheduling.
	interruptTimeout := 500 * time.Millisecond

	TqlTestCase{
		Name: "js-timeout",
		Script: `
				FAKE( linspace(1,10,10))
				SCRIPT("js", {
					while(true) {
					}
					$.yield(123)
				})
				CSV()
			`,
		CtxTimeout: interruptTimeout,
		ExpectLog:  []string{"[ERROR] interrupt at SCRIPT main:1:1(0)"},
		ExpectFunc: func(t *testing.T, result string) {
			requireNoPayload(t, result)
		},
	}.run(t)
	TqlTestCase{
		Name: "js-timeout-init",
		Script: `
			FAKE( linspace(1,10,10))
			SCRIPT("js", {
				while(true) {
				}
			},{
				$.yield(123)
			})
			CSV()
		`,
		CtxTimeout: interruptTimeout,
		ExpectFunc: func(t *testing.T, result string) {
			requireNoPayload(t, result)
		},
	}.run(t)
	TqlTestCase{
		Name: "js-timeout-finalize",
		Script: `
			FAKE( linspace(1,10,10))
			SCRIPT("js", {
				function finalize(){
					while(true) {}
				}
			},{
				$.yield($.values[0])
			})
			CSV()
		`,
		CtxTimeout: interruptTimeout,
		ExpectLog:  []string{"[ERROR] SCRIPT finalize, interrupt at finalize (<eval>:2:5(1))"},
		ExpectFunc: func(t *testing.T, result string) {
			// SCRIPT was interrupted during the finalize()
			// so the result exists
			require.Equal(t, "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n\n", result)
		},
	}.run(t)
}

func TestSCRIPT_inflight(t *testing.T) {
	TqlTestCase{
		Name: "js-set-value",
		Script: `
				FAKE( linspace(1,2,1))
				SCRIPT("js", {
					$.inflight().set("key1", 123);
					$.inflight().set("key2", "abc");
					$.yield("");
				})
				MAPVALUE(0, $key1)
				MAPVALUE(1, $key2)
				CSV()
			`,
		ExpectCSV: []string{"123,abc", "\n"},
	}.run(t)
	TqlTestCase{
		Name: "js-get-value",
		Script: `
				FAKE( linspace(1,2,1))
				SET(key1, 123)
				SET(key2, "abc")
				SCRIPT("js", {
					$.yield($.inflight().get("key1"), $.inflight().get("key2"));
				})
				CSV()
			`,
		ExpectCSV: []string{"123,abc", "\n"},
	}.run(t)
}

func TestSCRIPT_request(t *testing.T) {
	TqlTestCase{
		Name: "js-request",
		Script: fmt.Sprintf(`
				SCRIPT("js", {
					$.request("%s/db/query?q="+encodeURIComponent("select name, time, value from tag_simple limit 2"), {method: "GET"})
					 .do( (rsp) => {
					 	rsp.text((body) => {
							obj = JSON.parse(body);
							$.yield(obj.reason, obj.success);
						})
					})
				})
				CSV()`, testHttpAddress),
		ExpectCSV: []string{"success,true", "\n"},
	}.run(t)
	TqlTestCase{
		Name: "js-request-json",
		Script: fmt.Sprintf(`
				SCRIPT("js", {
					$.request("%s/db/query?q="+encodeURIComponent("select name, time, value from tag_simple limit 2"), {method: "GET"})
					 .do( (rsp) => {
					 	rsp.json((body) => {
							$.yield(...body.data.columns);
							$.yield(...body.data.types);
						})
					})
				})
				CSV()`, testHttpAddress),
		ExpectCSV: []string{
			`name,time,value`, `string,datetime,double`, "\n",
		},
	}.run(t)
}

func TestSCRIPT_db(t *testing.T) {
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
	}.run(t)
	TqlTestCase{
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
		CtxTimeout: 15 * time.Second, // increase timeout for slow CI/CD environment
		ExpectFunc: func(t *testing.T, result string) {
			require.Empty(t, result)
		},
	}.run(t)
	TqlTestCase{
		Name: "js-db-query",
		Script: `
			SCRIPT("js", {
				db = $.db();
				db.exec("create tag table if not exists js_table(name varchar(100) primary key, time datetime basetime, value double)");
				db.exec("insert into js_table(name, time, value) values(?, ?, ?)", "js-db-query", 1696118400000000000, 1.234);
				db.exec("EXEC table_flush(js_table)")
			},{
				db.query("select NAME, TIME, VALUE from js_table limit ?", 2).yield();
				db.query("select NAME, TIME, VALUE from js_table limit ?", 2).forEach((row) => {
					$.yield(...row);
				});
			},{
				db.exec("drop table js_table");
			})
			JSON(timeformat("s"))
		`,
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool())
			require.Equal(t, `["NAME","TIME","VALUE"]`, gjson.Get(result, "data.columns").Raw)
			require.Equal(t, `["string","datetime","double"]`, gjson.Get(result, "data.types").Raw)
			require.Equal(t, `["js-db-query",1696118400,1.234]`, gjson.Get(result, "data.rows.0").Raw)
			require.Equal(t, `["js-db-query",1696118400,1.234]`, gjson.Get(result, "data.rows.1").Raw)
		},
	}.run(t)
	TqlTestCase{
		Name: "js-db-query-module",
		Script: `
			SCRIPT("js", {
				db = require("@jsh/db");
			},{
				client = new db.Client();
				try{
					conn = client.connect();
					conn.exec("create tag table if not exists js_table2(name varchar(100) primary key, time datetime base time, value double)");
					conn.exec("insert into js_table2(name, time, value) values(?, ?, ?)", "js-db-query", 1696118400000000000, 1.234);
					conn.exec("EXEC table_flush(js_table2)")

					rows = conn.query("select NAME, TIME, VALUE from js_table2 limit ?", 2)
					$.result = rows.columns();
					for( let row of rows ) {
						$.yield(row.NAME, row.TIME.unix(), row.VALUE);
					}
				}catch(e) {
					console.log("Error:", e);
				}finally{
					// intentionally not closing the rows
					// rows.close();
					conn.exec("drop table js_table2");
					conn.close();
				}
			})
			JSON(timeformat("s"))
		`,
		// ExpectLog: []string{
		// 	"WARNING: db rows not closed!!!",
		// },
		ExpectFunc: func(t *testing.T, result string) {
			require.True(t, gjson.Get(result, "success").Bool(), result)
			require.Equal(t, `["NAME","TIME","VALUE"]`, gjson.Get(result, "data.columns").Raw)
			require.Equal(t, `["string","datetime","double"]`, gjson.Get(result, "data.types").Raw)
			require.Equal(t, `["js-db-query",1696118400,1.234]`, gjson.Get(result, "data.rows.0").Raw)
		},
	}.run(t)
}

func TestSCRIPT_opcua(t *testing.T) {
	svr, endpoint := startOPCUAServer(t)
	t.Cleanup(func() {
		require.NoError(t, svr.Close())
	})

	TqlTestCase{
		Name: "js-opcua-read",
		Script: fmt.Sprintf(`
			SCRIPT("js", {
				ua = require("opcua");
				nodes = [
					"ns=1;s=ro_bool",   // true
					"ns=1;s=rw_bool",   // true
					"ns=1;s=ro_int32",  // int32(5)
					"ns=1;s=rw_int32",  // int32(5)
				];
				client = new ua.Client({ endpoint: %q });
				vs = client.read({ nodes: nodes, timestampsToReturn: ua.TimestampsToReturn.Both});
				vs.forEach((v, idx) => {
					$.yield(nodes[idx], v.status, v.value, v.type);
				})
				client.close();
			})
			CSV(timeformat('default'), tz('UTC'))
		`, endpoint),
		ExpectCSV: []string{
			"ns=1;s=ro_bool,0,true,Boolean",
			"ns=1;s=rw_bool,0,true,Boolean",
			"ns=1;s=ro_int32,0,5,Int32",
			"ns=1;s=rw_int32,0,5,Int32",
			"\n"},
	}.run(t)
	TqlTestCase{
		Name: "js-opcua-read-perms",
		Script: fmt.Sprintf(`
			SCRIPT("js", {
				ua = require("opcua");
				nodes = [
					"ns=1;s=NoPermVariable",    // ua.StatusOK, int32(742)
					"ns=1;s=ReadWriteVariable", // ua.StatusOK, 12.34
					"ns=1;s=ReadOnlyVariable",  // ua.StatusOK, 9.87
					"ns=1;s=NoAccessVariable",  // ua.StatusBadUserAccessDenied
				];
				client = new ua.Client({ endpoint: %q });
				vs = client.read({ nodes: nodes});
				vs.forEach((v, idx) => {
					$.yield(nodes[idx], v.statusCode, v.value, v.type);
				})
				client.close();
			})
			CSV()
		`, endpoint),
		ExpectCSV: []string{
			"ns=1;s=NoPermVariable,StatusGood,742,Int32",
			"ns=1;s=ReadWriteVariable,StatusGood,12.34,Double",
			"ns=1;s=ReadOnlyVariable,StatusGood,9.87,Double",
			"ns=1;s=NoAccessVariable,StatusBadUserAccessDenied,NULL,Null",
			"\n"},
	}.run(t)
}

func startOPCUAServer(t *testing.T) (*opc_server.Server, string) {
	t.Helper()

	var opts []opc_server.Option
	port := freeOPCUAPort(t)
	endpoint := fmt.Sprintf("opc.tcp://127.0.0.1:%d", port)

	opts = append(opts,
		opc_server.EnableSecurity("None", ua.MessageSecurityModeNone),
		opc_server.EnableSecurity("Basic128Rsa15", ua.MessageSecurityModeSign),
		opc_server.EnableSecurity("Basic128Rsa15", ua.MessageSecurityModeSignAndEncrypt),
		opc_server.EnableSecurity("Basic256", ua.MessageSecurityModeSign),
		opc_server.EnableSecurity("Basic256", ua.MessageSecurityModeSignAndEncrypt),
		opc_server.EnableSecurity("Basic256Sha256", ua.MessageSecurityModeSignAndEncrypt),
		opc_server.EnableSecurity("Basic256Sha256", ua.MessageSecurityModeSign),
		opc_server.EnableSecurity("Aes128_Sha256_RsaOaep", ua.MessageSecurityModeSign),
		opc_server.EnableSecurity("Aes128_Sha256_RsaOaep", ua.MessageSecurityModeSignAndEncrypt),
		opc_server.EnableSecurity("Aes256_Sha256_RsaPss", ua.MessageSecurityModeSign),
		opc_server.EnableSecurity("Aes256_Sha256_RsaPss", ua.MessageSecurityModeSignAndEncrypt),
	)

	opts = append(opts,
		opc_server.EnableAuthMode(ua.UserTokenTypeAnonymous),
		opc_server.EnableAuthMode(ua.UserTokenTypeUserName),
		opc_server.EnableAuthMode(ua.UserTokenTypeCertificate),
		//		server.EnableAuthWithoutEncryption(), // Dangerous and not recommended, shown for illustration only
	)

	opts = append(opts,
		opc_server.EndPoint("127.0.0.1", port),
	)

	s := opc_server.New(opts...)

	root_ns, _ := s.Namespace(0)
	obj_node := root_ns.Objects()

	// Create a new node namespace.  You can add namespaces before or after starting the server.
	nodeNS := opc_server.NewNodeNameSpace(s, "NodeNamespace")
	// add it to the server.
	s.AddNamespace(nodeNS)
	nns_obj := nodeNS.Objects()
	// add the reference for this namespace's root object folder to the server's root object folder
	obj_node.AddRef(nns_obj, id.HasComponent, true)

	// Create some nodes for it.
	n := nodeNS.AddNewVariableStringNode("ro_bool", true)
	n.SetAttribute(ua.AttributeIDUserAccessLevel, &ua.DataValue{EncodingMask: ua.DataValueValue, Value: ua.MustVariant(byte(1))})
	nns_obj.AddRef(n, id.HasComponent, true)
	n = nodeNS.AddNewVariableStringNode("rw_bool", true)
	nns_obj.AddRef(n, id.HasComponent, true)

	n = nodeNS.AddNewVariableStringNode("ro_int32", int32(5))
	n.SetAttribute(ua.AttributeIDUserAccessLevel, &ua.DataValue{EncodingMask: ua.DataValueValue, Value: ua.MustVariant(byte(1))})
	nns_obj.AddRef(n, id.HasComponent, true)
	n = nodeNS.AddNewVariableStringNode("rw_int32", int32(5))
	nns_obj.AddRef(n, id.HasComponent, true)

	var3 := opc_server.NewNode(
		ua.NewStringNodeID(nodeNS.ID(), "NoPermVariable"), // you can use whatever node id you want here, whether it's numeric, string, guid, etc...
		map[ua.AttributeID]*ua.DataValue{
			ua.AttributeIDBrowseName: opc_server.DataValueFromValue(attrs.BrowseName("NoPermVariable")),
			ua.AttributeIDNodeClass:  opc_server.DataValueFromValue(uint32(ua.NodeClassVariable)),
		},
		nil,
		func() *ua.DataValue { return opc_server.DataValueFromValue(int32(742)) },
	)
	nodeNS.AddNode(var3)
	nns_obj.AddRef(var3, id.HasComponent, true)

	var4 := opc_server.NewNode(
		ua.NewStringNodeID(nodeNS.ID(), "ReadWriteVariable"), // you can use whatever node id you want here, whether it's numeric, string, guid, etc...
		map[ua.AttributeID]*ua.DataValue{
			ua.AttributeIDAccessLevel:     opc_server.DataValueFromValue(byte(ua.AccessLevelTypeCurrentRead | ua.AccessLevelTypeCurrentWrite)),
			ua.AttributeIDUserAccessLevel: opc_server.DataValueFromValue(byte(ua.AccessLevelTypeCurrentRead | ua.AccessLevelTypeCurrentWrite)),
			ua.AttributeIDBrowseName:      opc_server.DataValueFromValue(attrs.BrowseName("ReadWriteVariable")),
			ua.AttributeIDNodeClass:       opc_server.DataValueFromValue(uint32(ua.NodeClassVariable)),
		},
		nil,
		func() *ua.DataValue { return opc_server.DataValueFromValue(12.34) },
	)
	nodeNS.AddNode(var4)
	nns_obj.AddRef(var4, id.HasComponent, true)

	var5 := opc_server.NewNode(
		ua.NewStringNodeID(nodeNS.ID(), "ReadOnlyVariable"), // you can use whatever node id you want here, whether it's numeric, string, guid, etc...
		map[ua.AttributeID]*ua.DataValue{
			ua.AttributeIDAccessLevel:     opc_server.DataValueFromValue(byte(ua.AccessLevelTypeCurrentRead)),
			ua.AttributeIDUserAccessLevel: opc_server.DataValueFromValue(byte(ua.AccessLevelTypeCurrentRead)),
			ua.AttributeIDBrowseName:      opc_server.DataValueFromValue(attrs.BrowseName("ReadOnlyVariable")),
			ua.AttributeIDNodeClass:       opc_server.DataValueFromValue(uint32(ua.NodeClassVariable)),
		},
		nil,
		func() *ua.DataValue { return opc_server.DataValueFromValue(9.87) },
	)
	nodeNS.AddNode(var5)
	nns_obj.AddRef(var5, id.HasComponent, true)

	var6 := opc_server.NewNode(
		ua.NewStringNodeID(nodeNS.ID(), "NoAccessVariable"), // you can use whatever node id you want here, whether it's numeric, string, guid, etc...
		map[ua.AttributeID]*ua.DataValue{
			ua.AttributeIDAccessLevel:     opc_server.DataValueFromValue(byte(ua.AccessLevelTypeNone)),
			ua.AttributeIDUserAccessLevel: opc_server.DataValueFromValue(byte(ua.AccessLevelTypeNone)),
			ua.AttributeIDBrowseName:      opc_server.DataValueFromValue(attrs.BrowseName("NoAccessVariable")),
			ua.AttributeIDNodeClass:       opc_server.DataValueFromValue(uint32(ua.NodeClassVariable)),
		},
		nil,
		func() *ua.DataValue { return opc_server.DataValueFromValue(55.43) },
	)
	nodeNS.AddNode(var6)
	nns_obj.AddRef(var6, id.HasComponent, true)

	// Create a new node namespace.  You can add namespaces before or after starting the server.
	gopcuaNS := opc_server.NewNodeNameSpace(s, "http://gopcua.com/")
	// add it to the server.
	s.AddNamespace(gopcuaNS)
	nns_obj = gopcuaNS.Objects()
	// add the reference for this namespace's root object folder to the server's root object folder
	obj_node.AddRef(nns_obj, id.HasComponent, true)

	// Create a new node namespace.  You can add namespaces before or after starting the server.
	// Start the server
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("failed to start OPC UA test server at %s: %s", endpoint, err)
	}
	return s, endpoint
}

func freeOPCUAPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
