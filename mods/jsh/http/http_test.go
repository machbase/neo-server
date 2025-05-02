package http_test

import (
	"bytes"
	"context"
	"net/http"
	"regexp"
	"testing"

	"github.com/machbase/neo-server/v8/mods/jsh"
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

func TestMqtt(t *testing.T) {
	tests := []TestCase{
		{
			Name: "mqtt-client",
			Script: `
				const {println} = require("@jsh/process");
				const http = require("@jsh/http")
				try {
					req = http.request("http://127.0.0.1:29876/hello",{
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
					})
				} catch (e) {
				 	println(e.toString());
				}
			`,
			Expect: []string{
				"url: http://127.0.0.1:29876/hello",
				"error: <nil>",
				"status: 200",
				"statusText: 200 OK",
				"content-type: text/plain",
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

var serverAddress = "127.0.0.1:29876"

func TestMain(m *testing.M) {
	http.HandleFunc("/hello", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Hello World\n"))
		reqId := req.Header.Get("X-Request-Id")
		w.Write([]byte("X-Request-Id: " + reqId))
	})

	server := &http.Server{
		Addr:    serverAddress,
		Handler: http.DefaultServeMux,
	}
	go server.ListenAndServe()
	m.Run()
	defer server.Close()
}
