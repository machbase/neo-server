package http_test

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"sort"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/jsh"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
)

type TestCase struct {
	Name      string
	Script    string
	UseRegex  bool
	UseSort   bool
	Expect    []string
	ExpectLog []string
}

func runTest(t *testing.T, tc TestCase) {
	t.Helper()
	ctx := context.TODO()
	w := &bytes.Buffer{}
	j := jsh.NewJsh(ctx,
		jsh.WithNativeModules("@jsh/process", "@jsh/http"),
		jsh.WithWriter(w),
	)
	err := j.Run(tc.Name, tc.Script, nil)
	if err != nil {
		t.Fatalf("Error running script: %s", err)
	}
	lines := bytes.Split(w.Bytes(), []byte{'\n'})
	if tc.UseSort {
		sort.Slice(lines, func(i, j int) bool {
			return bytes.Compare(lines[i], lines[j]) < 0
		})
	}
	for i, line := range lines {
		if i >= len(tc.Expect) {
			break
		}
		if tc.UseRegex {
			re, err := regexp.Compile(tc.Expect[i])
			if err != nil {
				t.Fatalf("Error compiling regex: %s", err)
			}
			if !re.Match(line) {
				t.Errorf("Expected regex %q, got %q", tc.Expect[i], line)
			}
		} else {
			if !bytes.Equal(bytes.TrimSuffix(line, []byte("\r")), []byte(tc.Expect[i])) {
				t.Errorf("Expected %q, got %q", tc.Expect[i], line)
			}
		}
	}
	if len(lines) > len(tc.Expect) {
		t.Errorf("Expected %d lines, got %d", len(tc.Expect), len(lines))
	}
}

func TestHttp(t *testing.T) {
	tests := []TestCase{
		{
			Name: "http-request",
			Script: `
				const {println} = require("@jsh/process");
				const http = require("@jsh/http")
				try {
					req = http.request("http://` + serverAddress + `/hello",{
						headers: {
							"X-Request-Id": "1234567890",
						},
					})
					req.do((rsp) => {
						println("url:", rsp.url);
						println("error:", rsp.error());
						println("status:", rsp.status);
						println("statusText:", rsp.statusText);
						println("content-type:", rsp.headers["Content-Type"]);
						println("body:", rsp.text());
						println("X-Request-Id:", rsp.headers["X-Request-Id"]);
					})
				} catch (e) {
				 	println(e.toString());
				}
			`,
			Expect: []string{
				"url: http://" + serverAddress + "/hello",
				"error: <nil>",
				"status: 200",
				"statusText: 200 OK",
				"content-type: text/plain; charset=utf-8",
				"body: Hello World",
				"X-Request-Id: 1234567890",
				"",
			},
		},
		{
			Name: "http-client",
			Script: `
				const {println} = require("@jsh/process");
				const http = require("@jsh/http")
				try {
					client = new http.Client();
					client.do("http://` + serverAddress + `/hello",{
						headers: {
							"X-Request-Id": "1234567890",
						},
					}, (rsp) => {
						println("url:", rsp.url);
						println("error:", rsp.error());
						println("status:", rsp.status);
						println("statusText:", rsp.statusText);
						println("content-type:", rsp.headers["Content-Type"]);
						println("body:", rsp.text());
						println("X-Request-Id:", rsp.headers["X-Request-Id"]);
					})
				} catch (e) {
				 	println(e.toString());
				}
			`,
			Expect: []string{
				"url: http://" + serverAddress + "/hello",
				"error: <nil>",
				"status: 200",
				"statusText: 200 OK",
				"content-type: text/plain; charset=utf-8",
				"body: Hello World",
				"X-Request-Id: 1234567890",
				"",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTest(t, tc)
		})
	}
}

func TestHttpQueryParam(t *testing.T) {
	tests := []TestCase{
		{
			Name: "http-query-param",
			Script: `
				const {println} = require("@jsh/process");
				const http = require("@jsh/http")
				try {
					req = http.request("http://` + serverAddress + `/hello/machbase?greeting=Hi")
					req.do((rsp) => {
						println("statusText:", rsp.statusText);
						println("content-type:", rsp.headers["Content-Type"]);
						println("body:", rsp.text());
					})
				} catch (e) {
				 	println(e.toString());
				}
			`,
			Expect: []string{
				"statusText: 200 OK",
				"content-type: application/json; charset=utf-8",
				`body: {"greeting":"Hi","name":"machbase"}`,
				"",
			},
		},
		{
			Name: "http-query-param-redirect",
			Script: `
				const {println} = require("@jsh/process");
				const http = require("@jsh/http")
				try {
					req = http.request("http://` + serverAddress + `/hello/boys/good_morning")
					req.do((rsp) => {
						println("statusText:", rsp.statusText);
						println("content-type:", rsp.headers["Content-Type"]);
						println("body:", rsp.text());
					})
				} catch (e) {
				 	println(e.toString());
				}
			`,
			Expect: []string{
				"statusText: 200 OK",
				"content-type: application/json; charset=utf-8",
				`body: {"greeting":"good_morning","name":"boys"}`,
				"",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTest(t, tc)
		})
	}
}

func TestHttpStatic(t *testing.T) {
	tests := []TestCase{
		{
			Name: "http-static-html",
			Script: `
				const {println} = require("@jsh/process");
				const http = require("@jsh/http")
				try {
					req = http.request("http://` + serverAddress + `/html")
					req.do((rsp) => {
						println("statusText:", rsp.statusText);
						println("content-type:", rsp.headers["Content-Type"]);
						println(rsp.text());
					})
				} catch (e) {
				 	println(e.toString());
				}
			`,
			Expect: []string{
				"statusText: 200 OK",
				"content-type: text/html; charset=utf-8",
				`<html>`,
				`<body>`,
				`    <h1>Test HTML</h1>`,
				`</body>`,
				`</html>`,
				``,
			},
		},
		{
			Name: "http-static-file",
			Script: `
				const {println} = require("@jsh/process");
				const http = require("@jsh/http")
				try {
					req = http.request("http://` + serverAddress + `/test_file")
					req.do((rsp) => {
						println("statusText:", rsp.statusText);
						println("content-type:", rsp.headers["Content-Type"]);
						println(rsp.text());
					})
				} catch (e) {
				 	println(e.toString());
				}
			`,
			Expect: []string{
				"statusText: 200 OK",
				"content-type: text/plain; charset=utf-8",
				`Hello, Text!`,
				``,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTest(t, tc)
		})
	}
}

func TestHttpFormats(t *testing.T) {
	tests := []struct {
		format   string
		useRegex bool
		useSort  bool
		expect   []string
	}{
		{
			format: "text",
			expect: []string{
				"text/plain; charset=utf-8",
				`Hello World`,
				"",
			},
		},
		{
			format: "text-fmt",
			expect: []string{
				"text/plain; charset=utf-8",
				`Hello PI, 3.14`,
				"",
			},
		},
		{
			format: "json",
			expect: []string{
				"application/json; charset=utf-8",
				`{"bool":true,"num":123,"str":"Hello World"}`,
				"",
			},
		},
		{
			format: "json-indent",
			expect: []string{
				"application/json; charset=utf-8",
				`{`,
				`    "bool": true,`,
				`    "num": 123,`,
				`    "str": "Hello World"`,
				`}`,
				"",
			},
		},
		{
			format: "json-array",
			expect: []string{"application/json; charset=utf-8",
				`["Hello","World"]`,
				"",
			},
		},
		{
			format: "yaml",
			expect: []string{
				"application/yaml; charset=utf-8",
				`bool: true`,
				`num: 123`,
				`str: Hello World`,
				"",
				"",
				"",
			},
		},
		{
			format: "toml",
			expect: []string{
				"application/toml; charset=utf-8",
				`bool = true`,
				`num = 123`,
				`str = 'Hello World'`,
				"",
				"",
			},
		},
		{
			format:   "xml",
			useRegex: true,
			expect: []string{
				"application/xml; charset=utf-8",
				`<map>((<str>Hello World</str>)|(<num>123</num>)||(<bool>true</bool>))+</map>`,
				"",
			},
		},
		{
			format: "html",
			expect: []string{
				"text/html; charset=utf-8",
				`<html><body>`,
				`  <h1>Hello, Hello World!</h1>`,
				`  <p>num: 123</p>`,
				`  <p>bool: true</p>`,
				`</body></html>`,
				"",
				"",
			},
		},
		{
			format: "html-tmpl",
			expect: []string{
				"text/html; charset=utf-8",
				`<html><body>`,
				`  <h1>Hello, Hello World!</h1>`,
				`  <p>num: 123</p>`,
				`  <p>bool: true</p>`,
				`</body></html>`,
				"",
				"",
			},
		},
	}

	for _, tn := range tests {
		name := fmt.Sprintf("http-formats-%s", tn.format)
		script := fmt.Sprintf(`
				const {println} = require("@jsh/process");
				const http = require("@jsh/http")
				try {
					req = http.request("http://%s/formats/%s")
					req.do((rsp) => {
						println(rsp.headers["Content-Type"]);
						println(rsp.text());
					})
				} catch (e) {
				 	println(e.toString());
				}`, serverAddress, tn.format)

		tc := TestCase{
			Name:     name,
			Script:   script,
			UseRegex: tn.useRegex,
			UseSort:  tn.useSort,
			Expect:   tn.expect,
		}
		t.Run(name, func(t *testing.T) {
			runTest(t, tc)
		})
	}
}

var serverAddress = "127.0.0.1:29876"

func TestMain(m *testing.M) {
	serverFs, _ := ssfs.NewServerSideFileSystem([]string{"/=./test"})
	ssfs.SetDefault(serverFs)

	ctx, ctxCancel := context.WithCancel(context.Background())
	w := &bytes.Buffer{}
	j := jsh.NewJsh(ctx,
		jsh.WithNativeModules("@jsh/process", "@jsh/http"),
		jsh.WithWriter(w),
	)
	script := `
		const {println} = require("@jsh/process");
		const http = require("@jsh/http")
		const svr = new http.Server({network:'tcp', address:'` + serverAddress + `'})
		svr.get("/hello", (ctx) => {
			reqId = ctx.request.getHeader("X-Request-Id")
			ctx.setHeader("X-Request-Id", reqId)
			ctx.TEXT(http.status.OK, "Hello World")
		})
		svr.get("/hello/:name", (ctx) => {
			name = ctx.param("name")
			greeting = ctx.query("greeting")
			ctx.JSON(http.status.OK, {
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
			ctx.TEXT(http.status.OK, "Hello World")
		})
		svr.get("/formats/text-fmt", ctx => {
			name = "PI";
    		pi = 3.1415;
    		ctx.TEXT(http.status.OK, "Hello %s, %3.2f", name, pi);
		})
		svr.get("/formats/json", ctx => {
			ctx.JSON(http.status.OK, {str:"Hello World", num: 123, bool: true})
		})
		svr.get("/formats/json-indent", ctx => {
			ctx.JSON(http.status.OK, {str:"Hello World", num: 123, bool: true}, {indent: true})
		})
		svr.get("/formats/json-array", ctx => {
			ctx.JSON(http.status.OK, ["Hello", "World"])
		})
		svr.get("/formats/yaml", ctx => {
			ctx.YAML(http.status.OK, {str:"Hello World", num: 123, bool: true})
		})
		svr.get("/formats/toml", ctx => {
			ctx.TOML(http.status.OK, {str:"Hello World", num: 123, bool: true})
		})
		svr.get("/formats/xml", ctx => {
			ctx.XML(http.status.OK, {str:"Hello World", num: 123, bool: true})
		})
		//svr.loadHTMLFiles("/html/hello.tmpl")
		svr.loadHTMLGlob("/", "**/*.html")
		svr.get("/formats/html", ctx => {
			ctx.HTML(http.status.OK, "hello.html", {str:"Hello World", num: 123, bool: true})
		})
		svr.get("/formats/html-tmpl", ctx => {
			ctx.HTML(http.status.OK, "hello_tmpl.html", {str:"Hello World", num: 123, bool: true})
		})
		svr.static("/html", "/html")
		svr.staticFile("/test_file", "/html/hello.txt")
		svr.serve((result)=>{ println("server started", result.network, result.address) });
	`
	go func() {
		err := j.Run("testServer", script, nil)
		if err != nil {
			panic(err)
		}
	}()
	for {
		time.Sleep(100 * time.Millisecond)
		// wait for server to start and print("server started")
		if w.Len() > 0 {
			break
		}
	}
	m.Run()
	ctxCancel()
}
