package http

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/root"
)

var serverAddress = "127.0.0.1:29876"

func TestServer(t *testing.T) {
	script := `
		const http = require("http")
		const svr = new http.Server({network:'tcp', address:'` + serverAddress + `'})
		svr.get("/hello", (ctx) => {
			reqId = ctx.request.getHeader("X-Request-Id")
			ctx.setHeader("X-Request-Id", reqId)
			ctx.text(http.status.OK, "Hello World")
		})
		svr.get("/hello/:name", (ctx) => {
			name = ctx.param("name")
			greeting = ctx.query("greeting")
			ctx.json(http.status.OK, {
				"greeting": greeting,
				"name": name,
			})
		})
		svr.get("/hello/:name/:greeting", (ctx) => {
			name = ctx.param("name")
			greeting = ctx.param("greeting")
			ctx.redirect(http.status.Found, ` + "`/hello/${name}?greeting=${greeting}`" + `)
		})
		svr.get("/formats/text", ctx => {
			name = "PI";
    		pi = 3.1415;
    		ctx.text(http.status.OK, "Hello %s, %3.2f", name, pi);
		})
		svr.get("/formats/json", ctx => {
			ctx.json(http.status.OK, {str:"Hello World", num: 123, bool: true})
		})
		svr.get("/formats/json-indent", ctx => {
			ctx.json(http.status.OK, {str:"Hello World", num: 123, bool: true}, {indent: true})
		})
		svr.get("/formats/json-array", ctx => {
			ctx.json(http.status.OK, ["Hello", "World"])
		})
		svr.get("/formats/yaml", ctx => {
			ctx.yaml(http.status.OK, {str:"Hello World", num: 123, bool: true})
		})
		svr.get("/formats/toml", ctx => {
			ctx.toml(http.status.OK, {str:"Hello World", num: 123, bool: true})
		})
		svr.get("/formats/xml", ctx => {
			ctx.xml(http.status.OK, {str:"Hello World", num: 123, bool: true})
		})
		
		svr.loadHTMLGlob("/docs/template/*.html")
		svr.get("/formats/html", ctx => {
			ctx.html(http.status.OK, "hello.html", {str:"Hello World", num: 123, bool: true})
		})
		svr.static("/html", "/docs/html")
		svr.staticFile("/test_file", "/docs/html/hello.txt")
		svr.serve((result)=>{ console.println("server started", result.network, result.address) });

		setTimeout(() => { svr.close(); }, 5000);
	`

	conf := engine.Config{
		Name: "TestHttpServer",
		Code: script,
		FSTabs: []engine.FSTab{
			root.RootFSTab(),
			{MountPoint: "/work", Source: "../../test/"},
			{MountPoint: "/docs", Source: "./test"},
		},
		Env:    map[string]any{},
		Reader: &bytes.Buffer{},
		Writer: &bytes.Buffer{},
	}
	jr, err := engine.New(conf)
	if err != nil {
		t.Fatalf("Failed to create JSRuntime: %v", err)
	}
	jr.RegisterNativeModule("@jsh/process", jr.Process)
	jr.RegisterNativeModule("@jsh/http", Module)

	go func() {
		if err := jr.Run(); err != nil {
			panic(err)
		}
	}()

	time.Sleep(1 * time.Second) // wait for server start

	tests := []TestCase{
		{
			name: "response_hello",
			script: `
				const http = require('http');
				const {env} = require('process');
				http.get(env.get("testURL"), {headers:{"X-Request-Id": "123"}}, (r) => {
					console.println("header:", r.headers['X-Request-Id']);
					console.println("text:", r.text());
				});
			`,
			vars: map[string]any{
				"testURL": "http://" + serverAddress + "/hello",
			},
			output: []string{
				"header: 123",
				"text: Hello World",
			},
		},
		{
			name: "response_text",
			script: `
				const http = require('http');
				const {env} = require('process');
				const req = http.request(env.get("testURL"));
				req.setHeader("X-Request-Id", "123");
				req.end((r)=>{
					let o = r.json();
					console.println("json:", o.greeting, o.name);
				});
			`,
			vars: map[string]any{
				"testURL": "http://" + serverAddress + "/hello/World?greeting=Hi",
			},
			output: []string{
				"json: Hi World",
			},
		},
		{
			name: "response_redirect",
			script: `
				const http = require('http');
				const {env} = require('process');
				http.get(env.get("testURL"), (r) => {
					console.println("status:", r.statusCode, r.statusMessage);
					console.println("text:", r.text());
				});
			`,
			vars: map[string]any{
				"testURL": "http://" + serverAddress + "/hello/World/Hi",
			},
			output: []string{
				"status: 200 200 OK",
				`text: {"greeting":"Hi","name":"World"}`,
			},
		},
		{
			name: "response_formats_text",
			script: `
				const http = require('http');
				const {env} = require('process');
				http.get(env.get("testURL"), (r) => {
					console.println("text:", r.text());
				});
			`,
			vars: map[string]any{
				"testURL": "http://" + serverAddress + "/formats/text",
			},
			output: []string{
				`text: Hello PI, 3.14`,
			},
		},
		{
			name: "response_formats_json",
			script: `
				const http = require('http');
				const {env} = require('process');
				http.get(env.get("testURL"), (r) => {
					console.println(r.headers["Content-Type"]);
					let o = r.json();
					console.println("json:", o.str, o.num, o.bool);
				});
			`,
			vars: map[string]any{
				"testURL": "http://" + serverAddress + "/formats/json",
			},
			output: []string{
				`application/json; charset=utf-8`,
				`json: Hello World 123 true`,
			},
		},
		{
			name: "response_formats_json_indent",
			script: `
				const http = require('http');
				const {env} = require('process');
				http.get(env.get("testURL"), (r) => {
					console.print(r.readBody());
				});
			`,
			vars: map[string]any{
				"testURL": "http://" + serverAddress + "/formats/json-indent",
			},
			outputFunc: func(t *testing.T, result string) {
				if !strings.HasPrefix(result, "{\n") {
					t.Errorf("Expected JSON output to start with '{\\n', got: %s", result)
				}
				if !strings.Contains(result, `    "str": "Hello World"`) {
					t.Errorf("Expected JSON output to contain indented 'str' field, got: %s", result)
				}
				if !strings.Contains(result, `    "num": 123`) {
					t.Errorf("Expected JSON output to contain indented 'num' field, got: %s", result)
				}
				if !strings.Contains(result, `    "bool": true`) {
					t.Errorf("Expected JSON output to contain indented 'bool' field, got: %s", result)
				}
				if !strings.HasSuffix(result, "\n}") {
					t.Errorf("Expected JSON output to end with '\\n}\\n', got: %s", result)
				}
				if l := len(strings.Split(result, "\n")); l != 5 {
					t.Errorf("Expected indented JSON output to have multiple lines(%d), got: %s", l, result)
				}
			},
		},
		{
			name: "response_formats_json_array",
			script: `
				const http = require('http');
				const {env} = require('process');
				http.get(env.get("testURL"), (r) => {
					console.println(r.headers["Content-Type"]);
					let o = r.json();
					console.println("array:", JSON.stringify(o));
				});
			`,
			vars: map[string]any{
				"testURL": "http://" + serverAddress + "/formats/json-array",
			},
			output: []string{
				`application/json; charset=utf-8`,
				`array: ["Hello","World"]`,
			},
		},
		{
			name: "response_formats_yaml",
			script: `
				const http = require('http');
				const {env} = require('process');
				http.get(env.get("testURL"), (r) => {
					console.println(r.headers["Content-Type"]);
					console.print(r.readBody());
				});
			`,
			vars: map[string]any{
				"testURL": "http://" + serverAddress + "/formats/yaml",
			},
			output: []string{
				`application/yaml; charset=utf-8`,
				`bool: true`,
				`num: 123`,
				`str: Hello World`,
			},
		},
		{
			name: "response_formats_toml",
			script: `
				const http = require('http');
				const {env} = require('process');
				http.get(env.get("testURL"), (r) => {
					console.println(r.headers["Content-Type"]);
					console.print(r.readBody());
				});
			`,
			vars: map[string]any{
				"testURL": "http://" + serverAddress + "/formats/toml",
			},
			output: []string{
				`application/toml; charset=utf-8`,
				`bool = true`,
				`num = 123`,
				`str = 'Hello World'`,
			},
		},
		{
			name: "response_formats_xml",
			script: `
				const http = require('http');
				const {env} = require('process');
				http.get(env.get("testURL"), (r) => {
					console.println(r.headers["Content-Type"]);
					console.println(r.readBody());
				});
			`,
			vars: map[string]any{
				"testURL": "http://" + serverAddress + "/formats/xml",
			},
			outputFunc: func(t *testing.T, result string) {
				if !strings.HasPrefix(result, "application/xml; charset=utf-8\n<map>") {
					t.Errorf("Expected XML output to start with '<map>', got: %s", result)
				}
				if !strings.HasSuffix(result, "</map>\n") {
					t.Errorf("Expected XML output to end with '</map>', got: %s", result)
				}
				if !strings.Contains(result, `<str>Hello World</str>`) {
					t.Errorf("Expected XML output to contain '<str>Hello World</str>', got: %s", result)
				}
				if !strings.Contains(result, `<num>123</num>`) {
					t.Errorf("Expected XML output to contain '<num>123</num>', got: %s", result)
				}
				if !strings.Contains(result, `<bool>true</bool>`) {
					t.Errorf("Expected XML output to contain '<bool>true</bool>', got: %s", result)
				}
			},
		},
		{
			name: "response_formats_html",
			script: `
				const http = require('http');
				const {env} = require('process');
				http.get(env.get("testURL"), (r) => {
					console.println(r.headers["Content-Type"]);
					console.print(r.readBody());
				});
			`,
			vars: map[string]any{
				"testURL": "http://" + serverAddress + "/formats/html",
			},
			output: []string{
				`text/html; charset=utf-8`,
				`<html><body>`,
				`  <h1>Hello, Hello World!</h1>`,
				`  <p>num: 123</p>`,
				`  <p>bool: true</p>`,
				`</body></html>`,
			},
		},
		{
			name: "response_static_dir",
			script: `
				const http = require('http');
				const {env} = require('process');
				http.get(env.get("testURL"), (r) => {
					console.println(r.text());
				});
			`,
			vars: map[string]any{
				"testURL": "http://" + serverAddress + "/html",
			},
			output: []string{
				`<html>`,
				`<body>`,
				`    <h1>Test HTML</h1>`,
				`</body>`,
				`</html>`,
			},
		},
		{
			name: "response_static_file",
			script: `
				const http = require('http');
				const {env} = require('process');
				http.get(env.get("testURL"), (r) => {
					console.println(r.text());
				});
			`,
			vars: map[string]any{
				"testURL": "http://" + serverAddress + "/test_file",
			},
			output: []string{
				`Hello, Text!`,
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}

}
