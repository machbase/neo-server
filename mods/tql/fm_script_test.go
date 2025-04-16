package tql_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestScriptES6(t *testing.T) {
	tests := []TqlTestCase{
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
		},
		{
			Name: "js-yieldArray-number-int64",
			Script: `
				STRING('1,2,3,4,5', separator('\n'))
				SCRIPT("js", {
					$.yieldArray($.values[0].split(',').map( (v) => { return parseInt(v) }))
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
					$.yield(...arr);
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
		{
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
		},
		{
			Name: "js-db-query",
			Script: `
				SCRIPT("js", {
					db = $.db();
					db.exec("create tag table if not exists js_table(name varchar(100) primary key, time datetime basetime, value double)");
					db.exec("insert into js_table(name, time, value) values(?, ?, ?)", "js-db-query", 1696118400000000000, 1.234);
				},{
					db.query("select name, time, value from js_table limit ?", 2).yield();
					db.query("select name, time, value from js_table limit ?", 2).forEach((row) => {
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
		},
		{
			Name: "js-system-free-os-memory",
			Script: `
				SCRIPT("js", {
					m = require("system");
					m.free_os_memory();
					$.yield("ok");
				})
				CSV()
			`,
			ExpectCSV: []string{"ok", "\n"},
		},
		{
			Name: "js-system-gc",
			Script: `
				SCRIPT("js", {
					m = require("system");
					m.gc();
					$.yield("ok");
				})
				CSV()
			`,
			ExpectCSV: []string{"ok", "\n"},
		},
		{
			Name: "js-system-now",
			Script: `
				SCRIPT("js", {
					m = require("system");
					let now = m.now();
					$.yield("ok", now.Unix());
				})
				JSON()
			`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, gjson.Get(result, "success").Bool())
				require.Equal(t, `["column0","column1"]`, gjson.Get(result, "data.columns").Raw)
				require.Equal(t, `["string","double"]`, gjson.Get(result, "data.types").Raw)
				require.NotEmpty(t, gjson.Get(result, "data.rows").Raw)
			},
		},
		{
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
		},
		{
			Name: "js-compile-err",
			Script: `
				SCRIPT("js", {
					var1 + 1;
				})
				CSV()`,
			ExpectErr: `ReferenceError: var1 is not defined at <eval>:3:6(0)`,
		},
		{
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
		},
		{
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
		},
		{
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
				`NAME,TIME,VALUE`, `string,datetime,double`, "\n",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestScriptGetSetValue(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-set-value",
			Script: `
				FAKE( linspace(1,2,1))
				SCRIPT("js", {
					$.set("key1", 123);
					$.set("key2", "abc");
					$.yield("");
				})
				MAPVALUE(0, $key1)
				MAPVALUE(1, $key2)
				CSV()
			`,
			ExpectCSV: []string{"123,abc", "\n"},
		},
		{
			Name: "js-get-value",
			Script: `
				FAKE( linspace(1,2,1))
				SET(key1, 123)
				SET(key2, "abc")
				SCRIPT("js", {
					$.yield($.get("key1"), $.get("key2"));
				})
				CSV()
			`,
			ExpectCSV: []string{"123,abc", "\n"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestScriptFFT(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-fft",
			Script: `
				FAKE( oscillator( range(timeAdd(1685714509*1000000000,'1s'), '1s', '100us'), freq(10, 1.0), freq(50, 2.0)))
				SCRIPT("js", {
					m = require("dsp");
					times = [];
					values = [];
				}, {
					times.push($.values[0]);
					values.push($.values[1]);
				}, {
					result = m.fft(times, values);
					for( i = 0; i < result.x.length; i++ ) {
						if (result.x[i] > 60)
							break
						$.yield(result.x[i], result.y[i])
					}
				})
				CSV(precision(6))
				`,
			ExpectCSV: loadLines("./test/fft2d.csv"),
		},
		{
			Name: "js-fft_not_enough_samples_0",
			Script: `
				FAKE( linspace(0, 10, 100) )
				SCRIPT("js", {
					m = require("dsp");
					times = [];
					values = [];
				}, {
					times.push($.values[0]);
					values.push($.values[1]);
				}, {
					result = dsp.fft(times, values);
					for( i = 0; i < result.x.length; i++ ) {
						if (result.x[i] > 60)
							break
						$.yield(result.x[i], result.y[i])
					}
				})
				CSV()
				`,
			ExpectCSV: []string{"\n"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestScriptSimpleX(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-simplex",
			Script: `
				SCRIPT("js", {
					gen = require("generator").simplex(123);
				},{
					for(i=0; i < 5; i++) {
						$.yield(i, gen.Eval(i, i * 0.6) );
					}
				})
				CSV(precision(3))
			`,
			ExpectCSV: []string{
				"0.000,0.000",
				"1.000,0.349",
				"2.000,0.319",
				"3.000,0.038",
				"4.000,-0.364",
				"\n"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestScriptUUID(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-uuid",
			Script: `
				SCRIPT("js", {
					gen = require("generator").uuid(1);
				},{
					for(i=0; i < 5; i++) {
						$.yield(gen.Eval());
					}
				})
				CSV(header(false))
			`,
			ExpectFunc: func(t *testing.T, result string) {
				rows := strings.Split(strings.TrimSpace(result), "\n")
				require.Equal(t, 5, len(rows), result)
				for _, l := range rows {
					require.Equal(t, 36, len(l))
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

func TestScriptNumQuantile(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-quantile",
			Script: `
				FAKE( arrange(1, 100, 1) )
				SCRIPT("js", {
					m = require("stat");
					x = [];
				},{
					x.push($.values[0]);
				},{
					result = m.quantile(0.25, x);
					$.yield(result);
				})
				CSV()
			`,
			ExpectCSV: []string{"25", "\n"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestScriptNumMean(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-quantile",
			Script: `
				FAKE( arrange(1, 100, 1) )
				SCRIPT("js", {
					m = require("stat");
					x = [];
				},{
					x.push($.values[0]);
				},{
					result = m.mean(x);
					$.yield(result);
				})
				CSV()
			`,
			ExpectCSV: []string{"50.5", "\n"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestScriptNumStdDev(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-stddev",
			Script: `
				FAKE( arrange(1, 100, 1) )
				SCRIPT("js", {
					m = require("stat");
					x = [];
				},{
					x.push($.values[0]);
				},{
					result = m.stdDev(x);
					$.yield(result);
				})
				CSV(precision(2))
			`,
			ExpectCSV: []string{"29.01", "\n"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestScriptModule(t *testing.T) {
	tql.ClearPredefModules()
	tql.RegisterPredefModule("/m.js", []byte(`
		function test() {
        	return "passed";
		}
		module.exports = {
        	test: test
		}
	`))
	t.Cleanup(func() {
		tql.UnregisterPredefModule("/m.js")
	})

	tests := []TqlTestCase{
		{
			Name: "js-module",
			Script: `
				SCRIPT("js", {
					var m = require("/m.js");
				},{
					$.yield(m.test());
				})
				CSV()
			`,
			ExpectCSV: []string{"passed", "\n"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}
