package tql_test

import (
	"context"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/gopcua/opcua/id"
	opc_server "github.com/gopcua/opcua/server"
	"github.com/gopcua/opcua/server/attrs"
	"github.com/gopcua/opcua/ua"
)

func TestScriptJS(t *testing.T) {
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

func TestScriptSystemInflight(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-set-value",
			Script: `
				FAKE( linspace(1,2,1))
				SCRIPT("js", {
					inflight = require("system").inflight();
					inflight.set("key1", 123);
					inflight.set("key2", "abc");
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
					inflight = require("system").inflight();
					$.yield(inflight.get("key1"), inflight.get("key2"));
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

func TestScriptSystemStatz(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-statz",
			Script: `
				SCRIPT("js", {
					statz = require("system").statz("1m", "go:goroutine_max");
					last = statz.length - 1;
					$.yield(statz[last].time, ...statz[last].values);
				})
				CSV()
			`,
			ExpectFunc: func(t *testing.T, result string) {
				require.True(t, len(result) > 20, result)
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
				SCRIPT("js", {
					m = require("analysis");
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
					m = require("analysis");
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

func TestScriptGeneratorSimpleX(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-simplex",
			Script: `
				SCRIPT("js", {
					gen = require("generator").simplex(123);
				},{
					for(i=0; i < 5; i++) {
						$.yield(i, gen.eval(i, i * 0.6) );
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

func TestScriptGeneratorUUID(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-uuid",
			Script: `
				SCRIPT("js", {
					gen = require("generator").uuid(1);
				},{
					for(i=0; i < 5; i++) {
						$.yield(gen.eval());
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

func TestScriptGeneratorMeshgrid(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-meshgrid",
			Script: `
				SCRIPT("js", {
					gen = require("generator").meshgrid([1,2,3], [4,5]);
				},{
					for(i=0; i < gen.length; i++) {
						$.yield(...gen[i]);
					}
				})
				CSV(header(false))
			`,
			ExpectCSV: []string{
				"1,4",
				"1,5",
				"2,4",
				"2,5",
				"3,4",
				"3,5",
				"\n"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestScriptFilterLowpass(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-filter-lowpass",
			Script: `SCRIPT("js", {
				const { arrange } = require("generator");
				const lowpass = require("filter").lowpass(0.3);
				const simplex = require("generator").simplex(1);
			},{
				for( x of arrange(1, 10, 1) ) {
					v = x + simplex.eval(x) * 3;
					$.yield(x, v, lowpass.eval(v));
				}
			})			
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
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestScriptFilterAvg(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-filter-avg",
			Script: `
			FAKE( arrange(10, 30, 10) )
			SCRIPT("js", {
				const avg = require("filter").avg();
			},{
				$.yield($.values[0], avg.eval($.values[0]));
			})			
			CSV(precision(0))
			`,
			ExpectCSV: []string{
				"10,10",
				"20,15",
				"30,20",
				"\n",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestScriptFilterMovAvg(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-filter-movavg",
			Script: `SCRIPT("js", {
				const { linspace } = require("generator");
				const movavg = require("filter").movavg(10);
			},{
				for( x of linspace(0, 100, 100) ) {
					$.yield(x, movavg.eval(x));
				}
			})			
			CSV(precision(4))
			`,
			ExpectCSV: loadLines("./test/movavg_result_nowait.csv"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestScriptFilterKalman(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-filter-kalman",
			Script: `
				FAKE(json({[1.3], [10.2], [5.0], [3.4]}))
				SCRIPT("js", {
					const kalman = require("filter").kalman(1.0, 1.0, 2.0);
				},{
					$.yield($.values[0], kalman.eval(new Date(1744868877), $.values[0]));
				})
				CSV(precision(1))
				`,
			ExpectCSV: []string{
				`1.3,1.3`,
				`10.2,3.5`,
				`5.0,3.8`,
				`3.4,3.7`,
				"\n",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestScriptAnalysis(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-cdf",
			Script: `
				FAKE( arrange(1, 100, 1) )
				SCRIPT("js", {
					m = require("analysis");
					x = [];
				},{
					x.push($.values[0]);
				},{
					result = m.cdf(1.0, x);
					$.yield(result);
				})
				CSV(precision(2))
			`,
			ExpectCSV: []string{"0.01", "\n"},
		},
		{
			Name: "js-circular-mean",
			Script: `
				SCRIPT("js", {
					m = require("analysis");
					x = [0, 0.25 * Math.PI, 0.75 * Math.PI];
					w = [1, 2, 2.5];
				},{
					$.yield(m.circularMean(x));
					$.yield(m.circularMean(x, w));
				})
				CSV(precision(2))
			`,
			ExpectCSV: []string{"0.96", "1.37", "\n"},
		},
		{
			Name: "js-correlation",
			Script: `
				SCRIPT("js", {
					m = require("analysis");
					x = [8, -3, 7, 8, -4];
					y = [10, 5, 6, 3, -1];
					w = [2, 1.5, 3, 3, 2];
				},{
					result = m.correlation(x, y, w);
					$.yield(result);
				})
				CSV(precision(5))
			`,
			ExpectCSV: []string{"0.59915", "\n"},
		},
		{
			Name: "js-covariance",
			Script: `
				SCRIPT("js", {
					m = require("analysis");
					x = [8, -3, 7, 8, -4];
					y1 = [10, 2, 2, 4, 1];
					y2 = [12, 1, 11, 12, 0];
				},{
					$.yield(m.covariance(x, y1));
					$.yield(m.covariance(x, y2));
					$.yield(m.variance(x));
				})
				CSV(precision(4))
			`,
			ExpectCSV: []string{"13.8000", "37.7000", "37.7000", "\n"},
		},
		{
			Name: "js-entropy",
			Script: `
				SCRIPT("js", {
					m = require("analysis");
					$.yield(m.entropy([0.05, 0.1, 0.9, 0.05]));
					$.yield(m.entropy([0.2, 0.4, 0.25, 0.15]));
					$.yield(m.entropy([0.2, 0, 0, 0.5, 0, 0.2, 0.1, 0, 0, 0]));
					$.yield(m.entropy([0, 0, 1, 0]));
				})
				CSV(precision(4))`,
			ExpectCSV: []string{"0.6247", "1.3195", "1.2206", "0.0000", "\n"},
		},
		{
			Name: "js-geometric-mean",
			Script: `
				SCRIPT("js", {
					m = require("analysis");
					x = [8, 2, 9, 15, 4];
					w = [2, 2, 6, 7, 1];
					$.yield(m.mean(x, w));
					$.yield(m.geometricMean(x, w));
					log_x = [];
					for( v of x ) {
						log_x.push(Math.log(v));
					}
					$.yield(Math.exp(m.mean(log_x, w)));
				})
				CSV(precision(4))`,
			ExpectCSV: []string{"10.1667", "8.7637", "8.7637", "\n"},
		},
		{
			Name: "js-harmonic-mean",
			Script: `
				SCRIPT("js", {
					m = require("analysis");
					x = [8, 2, 9, 15, 4];
					w = [2, 2, 6, 7, 1];
					$.yield(m.mean(x, w));
					$.yield(m.harmonicMean(x, w));
				})
				CSV(precision(4))`,
			ExpectCSV: []string{"10.1667", "6.8354", "\n"},
		},
		{
			Name: "js-median",
			Script: `
				FAKE( arrange(1, 100, 1) )
				SCRIPT("js", {
					m = require("analysis");
					x = [];
				},{
					x.push($.values[0]);
				},{
					result = m.median(x);
					$.yield(result);
				})
				CSV()
			`,
			ExpectCSV: []string{"50", "\n"},
		},
		{
			Name: "js-quantile",
			Script: `
				FAKE( arrange(1, 100, 1) )
				SCRIPT("js", {
					m = require("analysis");
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
		{
			Name: "js-mean",
			Script: `
				FAKE( arrange(1, 100, 1) )
				SCRIPT("js", {
					m = require("analysis");
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
		{
			Name: "js-stddev",
			Script: `
				SCRIPT("js", {
					m = require("analysis");
					x = [8, 2, -9, 15, 4];
					w = [2, 2, 6, 7, 1];
				},{
					$.yield(m.stdDev(x));
					$.yield(m.stdDev(x, w));
				})
				CSV(precision(4))
			`,
			ExpectCSV: []string{"8.8034", "10.5733", "\n"},
		},
		{
			Name: "js-stderr",
			Script: `
				SCRIPT("js", {
					m = require("analysis");
					x = [8, 2, -9, 15, 4];
					w = [2, 2, 6, 7, 1];
					mean = m.mean(x, w);
					stddev = m.stdDev(x, w);
					nSamples = m.sum(w);
					stdErr = m.stdErr(stddev, nSamples);
					$.yield("stddev", stddev);
					$.yield("nSamples", nSamples);
					$.yield("mean", mean);
					$.yield("stderr", stdErr);
				})
				CSV(precision(4))`,
			ExpectCSV: []string{
				"stddev,10.5733",
				"nSamples,18.0000",
				"mean,4.1667",
				"stderr,2.4921",
				"\n",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestScriptAnalysisInterpolate(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-interpolate",
			Script: `
				SCRIPT("js", {
					const {simplex} = require("generator").simplex(123);
					m = require("analysis");
					xs = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11];
					ys = [0, 0.001, 0.002, 0.1, 1, 2, 2.5, -10, -10.01, 2.49, 2.53, 2.55];
					pc = m.interpPiecewiseConstant(xs, ys);
					pl = m.interpPiecewiseLinear(xs, ys);
					as = m.interpAkimaSpline(xs, ys);
					fb = m.interpFritschButland(xs, ys);
				},{
					n = xs.length;
					dx = 0.25;
					nPts = Math.round((n-1)/dx)+1;
					for( i = 0; i < nPts; i++ ) {
						x = xs[0] + i * dx;
						$.yield(x, pc.predict(x), pl.predict(x), as.predict(x), fb.predict(x));
					}				
				})
				CSV(precision(2))
			`,
			ExpectCSV: []string{
				"0.00,0.00,0.00,0.00,0.00",
				"0.25,0.00,0.00,0.00,0.00",
				"0.50,0.00,0.00,0.00,0.00",
				"0.75,0.00,0.00,0.00,0.00",
				"1.00,0.00,0.00,0.00,0.00",
				"1.25,0.00,0.00,0.00,0.00",
				"1.50,0.00,0.00,0.00,0.00",
				"1.75,0.00,0.00,0.00,0.00",
				"2.00,0.00,0.00,0.00,0.00",
				"2.25,0.10,0.03,-0.01,0.01",
				"2.50,0.10,0.05,-0.01,0.03",
				"2.75,0.10,0.08,0.02,0.06",
				"3.00,0.10,0.10,0.10,0.10",
				"3.25,1.00,0.33,0.26,0.22",
				"3.50,1.00,0.55,0.49,0.45",
				"3.75,1.00,0.78,0.75,0.73",
				"4.00,1.00,1.00,1.00,1.00",
				"4.25,2.00,1.25,1.24,1.26",
				"4.50,2.00,1.50,1.50,1.54",
				"4.75,2.00,1.75,1.75,1.79",
				"5.00,2.00,2.00,2.00,2.00",
				"5.25,2.50,2.12,2.22,2.17",
				"5.50,2.50,2.25,2.37,2.33",
				"5.75,2.50,2.38,2.47,2.45",
				"6.00,2.50,2.50,2.50,2.50",
				"6.25,-10.00,-0.62,0.83,0.55",
				"6.50,-10.00,-3.75,-2.98,-3.75",
				"6.75,-10.00,-6.88,-7.18,-8.04",
				"7.00,-10.00,-10.00,-10.00,-10.00",
				"7.25,-10.01,-10.00,-11.16,-10.00",
				"7.50,-10.01,-10.00,-11.55,-10.01",
				"7.75,-10.01,-10.01,-11.18,-10.01",
				"8.00,-10.01,-10.01,-10.01,-10.01",
				"8.25,2.49,-6.88,-7.18,-8.06",
				"8.50,2.49,-3.76,-2.99,-3.77",
				"8.75,2.49,-0.63,0.82,0.53",
				"9.00,2.49,2.49,2.49,2.49",
				"9.25,2.53,2.50,2.50,2.51",
				"9.50,2.53,2.51,2.51,2.52",
				"9.75,2.53,2.52,2.52,2.52",
				"10.00,2.53,2.53,2.53,2.53",
				"10.25,2.55,2.53,2.54,2.54",
				"10.50,2.55,2.54,2.54,2.54",
				"10.75,2.55,2.54,2.55,2.55",
				"11.00,2.55,2.55,2.55,2.55",
				"\n"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestScriptSpatialHaversine(t *testing.T) {
	tests := []TqlTestCase{
		{
			Name: "js-haversine",
			Script: `
				SCRIPT("js", {
					m = require("spatial");
					//buenos aires
					lat1 = -34.83333;
					lon1 = -58.5166646;
					//paris
					lat2 = 49.0083899664;
					lon2 = 2.53844117956;
					distance = m.haversine(lat1, lon1, lat2, lon2);
					$.yield(distance);
				})
				CSV(precision(0))
			`,
			ExpectCSV: []string{"8337886", "\n"},
		},
		{
			Name: "js-haversine-latlon",
			Script: `
				SCRIPT("js", {
					m = require("spatial");
					//buenos aires
					coord1 = [-34.83333, -58.5166646];
					//paris
					coord2 = [49.0083899664, 2.53844117956];
					distance = m.haversine(coord1, coord2);
					$.yield(distance);
				})
				CSV(precision(0))
			`,
			ExpectCSV: []string{"8337886", "\n"},
		},
		{
			Name: "js-haversine-coordinates",
			Script: `
				SCRIPT("js", {
					m = require("spatial");
					//buenos aires
					coord1 = [-34.83333, -58.5166646];
					//paris
					coord2 = [49.0083899664, 2.53844117956];
					distance = m.haversine({radius: 6371000, coordinates: [coord1, coord2]});
					$.yield(distance);
				})
				CSV(precision(0))
			`,
			ExpectCSV: []string{"8328556", "\n"},
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

func TestScriptOpcua(t *testing.T) {
	svr := startOPCUAServer()
	defer svr.Close()

	time.Sleep(1 * time.Second)

	tests := []TqlTestCase{
		{
			Name: "js-opcua",
			Script: `
				SCRIPT("js", {
					ua = require("opcua");
					client = ua.client({
						endpoint: "opc.tcp://localhost:4840",
						maxAge: 1000,
					});
					vs = client.read({
						nodes: [
							"ns=1;s=NoPermVariable",    // ua.StatusOK, int32(742)
							"ns=1;s=ReadWriteVariable", // ua.StatusOK, 12.34
							"ns=1;s=ReadOnlyVariable",  // ua.StatusOK, 9.87
							"ns=1;s=NoAccessVariable",  // ua.StatusBadUserAccessDenied
						],
					});
					for(v of vs) {
						$.yield(v.status.toString(16), v.value);
					}
				})
				CSV()
			`,
			ExpectCSV: []string{"0,742", "0,12.34", "0,9.87", "801f0000,NULL", "\n"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func startOPCUAServer() *opc_server.Server {
	var opts []opc_server.Option
	port := 4840

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
		opc_server.EndPoint("localhost", port),
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
	n.SetAttribute(ua.AttributeIDUserAccessLevel, &ua.DataValue{EncodingMask: ua.DataValueValue, Value: ua.MustVariant(uint32(1))})
	nns_obj.AddRef(n, id.HasComponent, true)
	n = nodeNS.AddNewVariableStringNode("rw_bool", true)
	nns_obj.AddRef(n, id.HasComponent, true)

	n = nodeNS.AddNewVariableStringNode("ro_int32", int32(5))
	n.SetAttribute(ua.AttributeIDUserAccessLevel, &ua.DataValue{EncodingMask: ua.DataValueValue, Value: ua.MustVariant(uint32(1))})
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
		log.Fatalf("Error starting server, exiting: %s", err)
	}
	return s
}
