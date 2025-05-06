package http_test

import (
	"bytes"
	"context"
	"regexp"
	"testing"

	"github.com/machbase/neo-server/v8/mods/jsh"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
)

type TestCase struct {
	Name      string
	Script    string
	UseRegex  bool
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
			if !bytes.Equal(line, []byte(tc.Expect[i])) {
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
		const http = require("@jsh/http")
		const lsnr = new http.Listener({network:'tcp', address:'` + serverAddress + `'})
		lsnr.get("/hello", (ctx) => {
			reqId = ctx.request.getHeader("X-Request-Id")
			ctx.setHeader("X-Request-Id", reqId)
			ctx.TEXT(http.status.OK, "Hello World")
		})
		lsnr.get("/hello/:name", (ctx) => {
			name = ctx.param("name")
			greeting = ctx.query("greeting")
			ctx.JSON(http.status.OK, {
				"greeting": greeting,
				"name": name,
			})
		})
		lsnr.get("/hello/:name/:greeting", (ctx) => {
			name = ctx.param("name")
			greeting = ctx.param("greeting")
			ctx.redirect(http.status.Found, ` + "`/hello/${name}?greeting=${greeting}`" + `)
		})
		lsnr.static("/html", "/html")
		lsnr.staticFile("/test_file", "/html/hello.txt")
		lsnr.listen();
	`
	go func() {
		err := j.Run("testServer", script, nil)
		if err != nil {
			panic(err)
		}
	}()
	m.Run()
	ctxCancel()
}
