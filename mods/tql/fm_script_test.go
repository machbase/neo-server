package tql_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestScriptES6(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-console-log",
			Script: `
				//+ es5=false
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
				//+ es5=false
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
				//+ es5=false
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
				//+ es5=false
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
				//+ es5=false
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
				//+ es5=false
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
				//+ es5=false
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
				//+ es5=false
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
				//+ es5=false
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
				//+ es5=false
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
				//+ es5=false
				SCRIPT("js", {
					$.db().exec("create tag table if not exists js_table(name varchar(100) primary key, time datetime basetime, value double)");
					$.db().exec("insert into js_table(name, time, value) values(?, ?, ?)", "js-db-query", 1696118400000000000, 1.234);
					finalize = ()=>{
						$.db().exec("drop table js_table");
					}
				},{
					$.db().query("select name, time, value from js_table limit ?", 2).yield();
					$.db().query("select name, time, value from js_table limit ?", 2).forEach((row) => {
						$.yield(...row);
					});
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
				//+ es5=false
				SCRIPT("js", {
					$.system().free_os_memory();
					$.yield("ok");
				})
				CSV()
			`,
			ExpectCSV: []string{"ok", "\n"},
		},
		{
			Name: "js-system-gc",
			Script: `
				//+ es5=false
				SCRIPT("js", {
					$.system().gc();
					$.yield("ok");
				})
				CSV()
			`,
			ExpectCSV: []string{"ok", "\n"},
		},
		{
			Name: "js-system-now",
			Script: `
				//+ es5=false
				SCRIPT("js", {
					let now = $.system().now();
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
				//+ es5=false
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
				//+ es5=false
				SCRIPT("js", {
					var1 + 1;
				})
				CSV()`,
			ExpectErr: `ReferenceError: var1 is not defined at <eval>:4:6(0)`,
		},
		{
			Name: "js-params",
			Script: `
				//+ es5=false
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
				//+ es5=false
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
				//+ es5=false
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

func TestScriptFFT(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-fft",
			Script: `
				FAKE( oscillator( range(timeAdd(1685714509*1000000000,'1s'), '1s', '100us'), freq(10, 1.0), freq(50, 2.0)))
				//+ es5=false
				SCRIPT("js", {
					times = [];
					values = [];
					function finalize() {
						result = $.num().fft(times, values);
						for( i = 0; i < result.x.length; i++ ) {
							if (result.x[i] > 60)
								break
							$.yield(result.x[i], result.y[i])
						}
					}
				}, {
					times.push($.values[0]);
					values.push($.values[1]);
				})
				CSV(precision(6))
				`,
			ExpectCSV: loadLines("./test/fft2d.csv"),
		},
		{
			Name: "js-fft_not_enough_samples_0",
			Script: `
				FAKE( linspace(0, 10, 100) )
				//+ es5=false
				SCRIPT("js", {
					times = [];
					values = [];
					function finalize() {
						result = $.num().fft(times, values);
						for( i = 0; i < result.x.length; i++ ) {
							if (result.x[i] > 60)
								break
							$.yield(result.x[i], result.y[i])
						}
					}
				}, {
					times.push($.values[0]);
					values.push($.values[1]);
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
