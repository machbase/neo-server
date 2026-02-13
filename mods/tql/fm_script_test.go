package tql_test

import (
	"context"
	"fmt"
	"log"
	"testing"

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
				require.True(t, gjson.Get(result, "success").Bool(), "success message should be true: %s", result)
				require.Equal(t, `["a","b","c","d"]`, gjson.Get(result, "data.columns").Raw)
				require.Equal(t, `["int64","double","string","bool"]`, gjson.Get(result, "data.types").Raw)
				require.Equal(t, `[[1,2.3,"3.4",true]]`, gjson.Get(result, "data.rows").Raw)
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
				require.True(t, gjson.Get(result, "success").Bool(), "success message should be true: %s", result)
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
		},
		{
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
		},
		{
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
			ExpectErr: `ReferenceError: var1 is not defined at SCRIPT main:3:6(0)`,
		},
		{
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
			ExpectErr: `Invalid module, SCRIPT main:7:23`,
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
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestScriptJS_panic(t *testing.T) {
	tests := []TqlTestCase{}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestScriptJS2(t *testing.T) {
	t.Skip("skipping not implemented test")
	tests := []TqlTestCase{
		{
			Name: "js-db-query",
			Script: `
				SCRIPT("js", {
					db = $.db();
					db.exec("create tag table if not exists js_table(name varchar(100) primary key, time datetime basetime, value double)");
					db.exec("insert into js_table(name, time, value) values(?, ?, ?)", "js-db-query", 1696118400000000000, 1.234);
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
		},
		{
			Name: "js-db-query-module",
			Script: `
				SCRIPT("js", {
					db = require("@jsh/db");
				},{
					client = new db.Client();
					try{
						conn = client.connect();
						conn.exec("create tag table if not exists js_table2(name varchar(100) primary key, time datetime basetime, value double)");
						conn.exec("insert into js_table2(name, time, value) values(?, ?, ?)", "js-db-query", 1696118400000000000, 1.234);
						conn.exec("EXEC table_flush(tag_data)")

						rows = conn.query("select NAME, TIME, VALUE from js_table2 limit ?", 2)
						$.result = rows.columns();
						for( let row of rows ) {
							$.yield(row.NAME, row.TIME.Unix(), row.VALUE);
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
				`name,time,value`, `string,datetime,double`, "\n",
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
					$.inflight().set("key1", 123);
					$.inflight().set("key2", "abc");
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
					$.yield($.inflight().get("key1"), $.inflight().get("key2"));
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
	t.Skip("skipping unstable test")
	tests := []TqlTestCase{
		{
			Name: "js-statz",
			Script: `
				SCRIPT("js", {
					statz = require("@jsh/system").statz("1m", "machbase:session:conn:wait_time");
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
					m = require("@jsh/stats");
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
					m = require("@jsh/stats");
					times = [];
					values = [];
				}, {
					times.push($.values[0]);
					values.push($.values[1]);
				}, {
					try{
						result = m.fft(times, values);
						for( i = 0; i < result.x.length; i++ ) {
							if (result.x[i] > 60)
								break
							$.yield(result.x[i], result.y[i])
						}
					} catch (e) {
					 	console.error(e);
					}
				})
				CSV()
				`,
			ExpectLog: []string{"[ERROR] fft invalid 0th sample value, but <nil>"},
			ExpectCSV: []string{"\n"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestScriptToTemplate(t *testing.T) {
	tests := []TqlTestCase{
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestScriptException(t *testing.T) {
	tests := []TqlTestCase{
		{
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
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestScriptOPCUA(t *testing.T) {
	svr := startOPCUAServer()
	defer svr.Close()

	tests := []TqlTestCase{
		{
			Name: "js-opcua-read",
			Script: `
				SCRIPT("js", {
					ua = require("@jsh/opcua");
					nodes = [
						"ns=1;s=ro_bool",   // true
						"ns=1;s=rw_bool",   // true
						"ns=1;s=ro_int32",  // int32(5)
						"ns=1;s=rw_int32",  // int32(5)
					];
					client = new ua.Client({ endpoint: "opc.tcp://localhost:4840" });
					vs = client.read({ nodes: nodes, timestampsToReturn: ua.TimestampsToReturn.Both});
					vs.forEach((v, idx) => {
						$.yield(nodes[idx], v.status, v.value, v.type);
					})
					client.close();
				})
				CSV(timeformat('default'), tz('UTC'))
			`,
			ExpectCSV: []string{
				"ns=1;s=ro_bool,0,true,Boolean",
				"ns=1;s=rw_bool,0,true,Boolean",
				"ns=1;s=ro_int32,0,5,Int32",
				"ns=1;s=rw_int32,0,5,Int32",
				"\n"},
		},
		{
			Name: "js-opcua-read-perms",
			Script: `
				SCRIPT("js", {
					ua = require("@jsh/opcua");
					nodes = [
						"ns=1;s=NoPermVariable",    // ua.StatusOK, int32(742)
						"ns=1;s=ReadWriteVariable", // ua.StatusOK, 12.34
						"ns=1;s=ReadOnlyVariable",  // ua.StatusOK, 9.87
						"ns=1;s=NoAccessVariable",  // ua.StatusBadUserAccessDenied
					];
					client = new ua.Client({ endpoint: "opc.tcp://localhost:4840" });
					vs = client.read({ nodes: nodes});
					vs.forEach((v, idx) => {
						$.yield(nodes[idx], v.statusCode, v.value, v.type);
					})
					client.close();
				})
				CSV()
			`,
			ExpectCSV: []string{
				"ns=1;s=NoPermVariable,StatusGood,742,Int32",
				"ns=1;s=ReadWriteVariable,StatusGood,12.34,Double",
				"ns=1;s=ReadOnlyVariable,StatusGood,9.87,Double",
				"ns=1;s=NoAccessVariable,StatusBadUserAccessDenied,NULL,Null",
				"\n"},
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
		log.Fatalf("Error starting server, exiting: %s", err)
	}
	return s
}
