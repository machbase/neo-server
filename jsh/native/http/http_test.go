package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/root"
)

func echoServer(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/notfound" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch r.Method {
	case "GET":
		xTestHeader := r.Header.Get("X-Test-Header")            // just to show we can read headers
		w.Header().Set("Date", "Fri, 12 Dec 2025 12:20:01 GMT") // fixed date for testing
		w.Header().Set("X-Test-Header", xTestHeader)
		w.WriteHeader(http.StatusOK)
		if r.URL.Query().Get("echo") != "" {
			w.Write([]byte(r.URL.Query().Get("echo")))
			return
		}
		w.Write([]byte("Hello, World!"))
	case "POST":
		if r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Content-Type must be application/json"))
			return
		}
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		obj := struct {
			Message string `json:"message"`
			Reply   string `json:"reply,omitempty"`
		}{}
		if err := json.Unmarshal(body, &obj); err != nil {
			fmt.Println("echoServer: invalid JSON:", err, ":", string(body))
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Invalid JSON"))
			return
		}
		w.WriteHeader(http.StatusOK)
		obj.Reply = "Received"
		b, _ := json.Marshal(&obj) // just to verify it's valid JSON
		w.Write(b)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

type TestCase struct {
	name       string
	script     string
	input      []string
	output     []string
	outputFunc func(t *testing.T, result string)
	err        string
	vars       map[string]any
}

func RunTest(t *testing.T, tc TestCase) {
	t.Helper()
	t.Run(tc.name, func(t *testing.T) {
		t.Helper()
		conf := engine.Config{
			Name:   tc.name,
			Code:   tc.script,
			FSTabs: []engine.FSTab{root.RootFSTab(), {MountPoint: "/work", Source: "../../test/"}},
			Env:    tc.vars,
			Reader: &bytes.Buffer{},
			Writer: &bytes.Buffer{},
		}
		jr, err := engine.New(conf)
		if err != nil {
			t.Fatalf("Failed to create JSRuntime: %v", err)
		}
		jr.RegisterNativeModule("@jsh/process", jr.Process)
		jr.RegisterNativeModule("@jsh/http", Module)

		if err := jr.Run(); err != nil {
			if tc.err == "" || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("Unexpected error: %v", err)
			}
			return
		}

		gotOutput := conf.Writer.(*bytes.Buffer).String()
		if tc.outputFunc != nil {
			tc.outputFunc(t, gotOutput)
			return
		}
		lines := strings.Split(gotOutput, "\n")
		if runtime.GOOS == "windows" {
			for i := 0; i < len(lines); i++ {
				lines[i] = strings.TrimRight(lines[i], "\r")
			}
		}
		if len(lines) != len(tc.output)+1 { // +1 for trailing newline
			t.Fatalf("Expected %d output lines, got %d\n%s", len(tc.output), len(lines)-1, gotOutput)
		}
		for i, expectedLine := range tc.output {
			if lines[i] != expectedLine {
				t.Errorf("Output line %d: expected %q, got %q", i, expectedLine, lines[i])
			}
		}
	})
}

func TestHttpRequest(t *testing.T) {
	tests := []TestCase{
		{
			name: "http_request_get",
			script: `
				const http = require('http');
				const {env} = require('process');
				const url = new URL(env.get("testURL")+"?echo=Hello?");
				const req = http.request(url);
				req.end((response) => {
					const {statusCode, statusMessage} = response;
				    console.println("Status Code:", statusCode);
					console.println("Status Message:", statusMessage);
				});
			`,
			output: []string{
				"Status Code: 200",
				"Status Message: 200 OK",
			},
		},
		{
			name: "http_request_method_url",
			script: `
				const http = require('http');
				const {env} = require('process');
				const url = new URL(env.get("testURL")+"?echo=Hello?");
				const req = http.request(url, {
					host: url.host,
					port: url.port,
					path: url.pathname + url.search,
					method: "get",
					agent: new http.Agent(),
				});
				req.end();
				req.on("response", (response) => {
					if (!response.ok) {
						throw new Error("Request failed with status "+response.statusCode);
					}
					const {statusCode, statusMessage} = response;
				    console.println("Status Code:", statusCode);
					console.println("Status:", statusMessage);
				});
			`,
			output: []string{
				"Status Code: 200",
				"Status: 200 OK",
			},
		},
		{
			name: "http_request_post",
			script: `
				const http = require('http');
				const {env} = require('process');
				const req = http.request(
					env.get("testURL"),
					{ method:"POST", headers: {"Content-Type":"application/json"} },
				);
				req.on("response", (response) => {
					if (!response.ok) {
						throw new Error("Request failed with status "+response.statusCode);
					}
					const {statusCode, statusMessage} = response;
					console.println("Status Code:", statusCode);
					console.println("Status:", statusMessage);
					const body = response.json()
					console.println("message:"+ body.message + ", " + "reply:" + body.reply);
				});
				req.on("error", (err) => {
					console.println("Request error:", err.message);
				});
				req.write('{"message": "Hello, ');
				req.end('World!"}');
			`,
			output: []string{
				"Status Code: 200",
				"Status: 200 OK",
				"message:Hello, World!, reply:Received",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(echoServer))
	defer server.Close()

	for _, tc := range tests {
		tc.vars = map[string]any{
			"testURL": server.URL,
		}
		RunTest(t, tc)
	}
}

func TestHttpGet(t *testing.T) {
	tests := []TestCase{
		{
			name: "http_get_string",
			script: `
				const http = require('http');
				const {env} = require('process');
				const url = env.get("testURL")+"?echo=Hi?";
				http.get(url, (response)=> {
				    console.println("Status Code:", response.statusCode);
					console.println("Status:", response.statusMessage);
				})
			`,
			output: []string{
				"Status Code: 200",
				"Status: 200 OK",
			},
		},
		{
			name: "http_get_string_on",
			script: `
				const http = require('http');
				const {env} = require('process');
				const url = env.get("testURL")+"?echo=Hi?";
				const req = http.get(url)
				req.on("response", (response)=> {
				    console.println("Status Code:", response.statusCode);
					console.println("Status:", response.statusMessage);
				});
			`,
			output: []string{
				"Status Code: 200",
				"Status: 200 OK",
			},
		},
		{
			name: "http_get_url",
			script: `
				const http = require('http');
				const {env} = require('process');
				const url = new URL(env.get("testURL")+"?echo=Hi?");
				http.get(url, (response)=> {
				    console.println("Status Code:", response.statusCode);
					console.println("Status:", response.statusMessage);
				})
			`,
			output: []string{
				"Status Code: 200",
				"Status: 200 OK",
			},
		},
		{
			name: "http_get_string_options",
			script: `
				const http = require('http');
				const {env} = require('process');
				const url = env.get("testURL")+"?echo=Hi?";
				const opt = {
					headers: {"X-Test-Header": "TestValue"}
				};
				http.get(url, opt, (response)=> {
				    console.println("Status Code:", response.statusCode);
					console.println("Status:", response.statusMessage);
					console.println("X-Test-Header:", response.headers["X-Test-Header"]);
				})
			`,
			output: []string{
				"Status Code: 200",
				"Status: 200 OK",
				"X-Test-Header: TestValue",
			},
		},
		{
			name: "http_get_url_options",
			script: `
				const http = require('http');
				const {env} = require('process');
				const url = new URL(env.get("testURL")+"?echo=Hi?");
				const opt = {
					headers: {"X-Test-Header": "TestValue"}
				};
				http.get(url, opt, (response)=> {
				    console.println("Status Code:", response.statusCode);
					console.println("Status:", response.statusMessage);
					console.println("X-Test-Header:", response.headers["X-Test-Header"]);
				})
			`,
			output: []string{
				"Status Code: 200",
				"Status: 200 OK",
				"X-Test-Header: TestValue",
			},
		},
		{
			name: "http_get_options",
			script: `
				const http = require('http');
				const {env} = require('process');
				const opt = {
					url: new URL(env.get("testURL")+"?echo=Hi?"),
					headers: {"X-Test-Header": "TestValue"},
				};
				http.get(opt, (response) => {
					const {statusCode, statusMessage} = response;
				    console.println("Status Code:", statusCode);
					console.println("Status:", statusMessage);
					console.println("Body:", response.text());
					
					contentLength = response.headers["Content-Length"];
					contentType = response.headers["Content-Type"];
					dateHeader = response.headers["Date"];
					if (contentLength != "3") {
						throw new Error("Unexpected Content-Length: "+contentLength);
					}
					if (!/^text\/plain/.test(contentType)) {
						throw new Error("Unexpected Content-Type:"+contentType);
					}
					if (contentType != "text/plain; charset=utf-8") {
						throw new Error("Unexpected Content-Type: "+contentType);
					}
					if (dateHeader != "Fri, 12 Dec 2025 12:20:01 GMT") {
						throw new Error("Unexpected Date header: "+dateHeader);
					}
				});
			`,
			output: []string{
				"Status Code: 200",
				"Status: 200 OK",
				"Body: Hi?",
			},
		},
		{
			name: "http_get_not_found",
			script: `
				const http = require('http');
				const {env} = require('process');
				const url = env.get("testURL")+"/notfound";
				http.get(url, (response)=> {
				    console.println("Status Code:", response.statusCode);
					console.println("Status:", response.statusMessage);
				})
			`,
			output: []string{
				"Status Code: 404",
				"Status: 404 Not Found",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(echoServer))
	defer server.Close()

	for _, tc := range tests {
		tc.vars = map[string]any{
			"testURL": server.URL,
		}
		RunTest(t, tc)
	}
}

func TestHttpHeaders(t *testing.T) {
	tests := []TestCase{
		{
			name: "header_set_get",
			script: `
				const http = require('http');
				const {env} = require('process');
				const req = http.request(env.get("testURL"));
				
				// Set headers
				req.setHeader("Content-Type", "application/json");
				req.setHeader("X-Custom-Header", "CustomValue");
				req.setHeader("Authorization", "Bearer token123");
				
				// Get headers
				console.println("Content-Type:", req.getHeader("Content-Type"));
				console.println("X-Custom-Header:", req.getHeader("X-Custom-Header"));
				console.println("Authorization:", req.getHeader("Authorization"));
				
				// Case-insensitive get
				console.println("content-type:", req.getHeader("content-type"));
				console.println("x-custom-header:", req.getHeader("x-custom-header"));
				
				req.end();
			`,
			output: []string{
				"Content-Type: application/json",
				"X-Custom-Header: CustomValue",
				"Authorization: Bearer token123",
				"content-type: application/json",
				"x-custom-header: CustomValue",
			},
		},
		{
			name: "header_has_remove",
			script: `
				const http = require('http');
				const {env} = require('process');
				const req = http.request(env.get("testURL"));
				
				req.setHeader("X-Test-Header", "value");
				console.println("Has X-Test-Header:", req.hasHeader("X-Test-Header"));
				console.println("Has X-Other-Header:", req.hasHeader("X-Other-Header"));
				
				req.removeHeader("X-Test-Header");
				console.println("After remove:", req.hasHeader("X-Test-Header"));
				
				req.end();
			`,
			output: []string{
				"Has X-Test-Header: true",
				"Has X-Other-Header: false",
				"After remove: false",
			},
		},
		{
			name: "header_get_all",
			script: `
				const http = require('http');
				const {env} = require('process');
				const req = http.request(env.get("testURL"));
				
				req.setHeader("Content-Type", "text/plain");
				req.setHeader("X-Custom-1", "value1");
				req.setHeader("X-Custom-2", "value2");
				
				const headers = req.getHeaders();
				const names = req.getHeaderNames().sort();
				
				console.println("Header count:", Object.keys(headers).length);
				console.println("Header names:", names.join(", "));
				
				req.end();
			`,
			output: []string{
				"Header count: 3",
				"Header names: Content-Type, X-Custom-1, X-Custom-2",
			},
		},
		{
			name: "header_chaining",
			script: `
				const http = require('http');
				const {env} = require('process');
				const req = http.request(env.get("testURL"))
					.setHeader("Content-Type", "application/json")
					.setHeader("Accept", "application/json")
					.removeHeader("Accept")
					.setHeader("X-Final", "value");
				
				console.println("Content-Type:", req.getHeader("Content-Type"));
				console.println("Accept:", req.getHeader("Accept"));
				console.println("X-Final:", req.getHeader("X-Final"));
				
				req.end();
			`,
			output: []string{
				"Content-Type: application/json",
				"Accept: null",
				"X-Final: value",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(echoServer))
	defer server.Close()

	for _, tc := range tests {
		tc.vars = map[string]any{
			"testURL": server.URL,
		}
		RunTest(t, tc)
	}
}

func TestIncomingMessage(t *testing.T) {
	tests := []TestCase{
		{
			name: "response_properties",
			script: `
				const http = require('http');
				const {env} = require('process');
				
				http.get(env.get("testURL") + "?echo=test", (response) => {
					console.println("statusCode:", response.statusCode);
					console.println("statusMessage:", response.statusMessage);
					console.println("ok:", response.ok);
					console.println("httpVersion:", response.httpVersion);
					console.println("complete:", response.complete);
					console.println("has headers:", typeof response.headers === "object");
					console.println("has rawHeaders:", Array.isArray(response.rawHeaders));
				});
			`,
			output: []string{
				"statusCode: 200",
				"statusMessage: 200 OK",
				"ok: true",
				"httpVersion: 1.1",
				"complete: true",
				"has headers: true",
				"has rawHeaders: true",
			},
		},
		{
			name: "response_ok_status",
			script: `
				const http = require('http');
				const {env} = require('process');
				
				http.get(env.get("testURL") + "?echo=ok", (response) => {
					console.println("200 ok:", response.ok);
				});
				
				http.get(env.get("testURL") + "/notfound", (response) => {
					console.println("404 ok:", response.ok);
				});
			`,
			output: []string{
				"200 ok: true",
				"404 ok: false",
			},
		},
		{
			name: "response_json",
			script: `
				const http = require('http');
				const {env} = require('process');
				
				const req = http.request(env.get("testURL"), {
					method: "POST",
					headers: {"Content-Type": "application/json"}
				});
				
				req.on("response", (response) => {
					const data = response.json();
					console.println("message:", data.message);
					console.println("reply:", data.reply);
				});
				
				req.end('{"message": "Hello"}');
			`,
			output: []string{
				"message: Hello",
				"reply: Received",
			},
		},
		{
			name: "response_text",
			script: `
				const http = require('http');
				const {env} = require('process');
				
				http.get(env.get("testURL") + "?echo=HelloWorld", (response) => {
					const text = response.text();
					console.println("text:", text);
					console.println("length:", text.length);
				});
			`,
			output: []string{
				"text: HelloWorld",
				"length: 10",
			},
		},
		{
			name: "response_headers_lowercase",
			script: `
				const http = require('http');
				const {env} = require('process');
				
				http.get(env.get("testURL") + "?echo=test", {
					headers: {"X-Test-Header": "TestValue"}
				}, (response) => {
					// Go's Headers map keys are preserved as-is
					const hasContentType = response.headers["Content-Type"] !== undefined;
					const hasDate = response.headers["Date"] !== undefined;
					const xTest = response.headers["X-Test-Header"];
					
					console.println("has content-type:", hasContentType);
					console.println("has date:", hasDate);
					console.println("x-test-header:", xTest);
				});
			`,
			output: []string{
				"has content-type: true",
				"has date: true",
				"x-test-header: TestValue",
			},
		},
		{
			name: "response_raw_headers",
			script: `
				const http = require('http');
				const {env} = require('process');
				
				http.get(env.get("testURL") + "?echo=test", (response) => {
					console.println("rawHeaders is array:", Array.isArray(response.rawHeaders));
					console.println("rawHeaders length > 0:", response.rawHeaders.length > 0);
					// rawHeaders should be key-value pairs
					console.println("even length:", response.rawHeaders.length % 2 === 0);
				});
			`,
			output: []string{
				"rawHeaders is array: true",
				"rawHeaders length > 0: true",
				"even length: true",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(echoServer))
	defer server.Close()

	for _, tc := range tests {
		tc.vars = map[string]any{
			"testURL": server.URL,
		}
		RunTest(t, tc)
	}
}

func TestHttpEvents(t *testing.T) {
	tests := []TestCase{
		{
			name: "request_events",
			script: `
				const http = require('http');
				const {env} = require('process');
				
				const req = http.request(env.get("testURL"));
				
				let responseReceived = false;
				let endReceived = false;
				
				req.on("response", (response) => {
					responseReceived = true;
					console.println("response event");
				});
				
				req.on("end", () => {
					endReceived = true;
					console.println("end event");
					console.println("events received:", responseReceived && endReceived);
				});
				
				req.end();
			`,
			output: []string{
				"response event",
				"end event",
				"events received: true",
			},
		},
		{
			name: "error_event",
			script: `
				const http = require('http');
				const {env} = require('process');
				
				const req = http.request(env.get("testURL"), {
					method: "POST",
					headers: {"Content-Type": "text/plain"}  // Wrong content type
				});
				
				req.on("error", (err) => {
					console.println("error event:", err.message.includes("Content-Type"));
				});
				
				req.on("response", (response) => {
					if (response.statusCode === 400) {
						console.println("bad request");
					}
				});
				
				req.end('{"test": "data"}');
			`,
			output: []string{
				"bad request",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(echoServer))
	defer server.Close()

	for _, tc := range tests {
		tc.vars = map[string]any{
			"testURL": server.URL,
		}
		RunTest(t, tc)
	}
}

func TestHttpAgent(t *testing.T) {
	tests := []TestCase{
		{
			name: "agent_reuse",
			script: `
				const http = require('http');
				const {env} = require('process');
				
				const agent = new http.Agent();
				
				// First request
				const req1 = http.request(env.get("testURL"), {agent: agent});
				req1.end((response) => {
					console.println("request 1:", response.statusCode);
				});
				
				// Second request with same agent
				const req2 = http.request(env.get("testURL") + "?echo=second", {agent: agent});
				req2.end((response) => {
					console.println("request 2:", response.statusCode);
				});
			`,
			output: []string{
				"request 1: 200",
				"request 2: 200",
			},
		},
		{
			name: "agent_per_request",
			script: `
				const http = require('http');
				const {env} = require('process');
				
				// Without explicit agent (creates default agent per request)
				http.get(env.get("testURL"), (response) => {
					console.println("no agent:", response.statusCode);
				});
				
				// With explicit agent
				const agent = new http.Agent();
				http.get(env.get("testURL"), {agent: agent}, (response) => {
					console.println("with agent:", response.statusCode);
				});
			`,
			output: []string{
				"no agent: 200",
				"with agent: 200",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(echoServer))
	defer server.Close()

	for _, tc := range tests {
		tc.vars = map[string]any{
			"testURL": server.URL,
		}
		RunTest(t, tc)
	}
}

func TestHttpEdgeCases(t *testing.T) {
	tests := []TestCase{
		{
			name: "multiple_writes",
			script: `
				const http = require('http');
				const {env} = require('process');
				
				const req = http.request(env.get("testURL"), {
					method: "POST",
					headers: {"Content-Type": "application/json"}
				});
				
				req.on("response", (response) => {
					const data = response.json();
					console.println("message:", data.message);
				});
				
				// Multiple writes
				req.write('{"mes');
				req.write('sage": "Hel');
				req.write('lo, ');
				req.end('World!"}');
			`,
			output: []string{
				"message: Hello, World!",
			},
		},
		{
			name: "empty_response",
			script: `
				const http = require('http');
				const {env} = require('process');
				
				http.get(env.get("testURL"), (response) => {
					console.println("statusCode:", response.statusCode);
					const text = response.text();
					console.println("has body:", text.length > 0);
				});
			`,
			output: []string{
				"statusCode: 200",
				"has body: true",
			},
		},
		{
			name: "url_with_query",
			script: `
				const http = require('http');
				const {env} = require('process');
				
				const url = new URL(env.get("testURL") + "?echo=test&param=value");
				http.get(url, (response) => {
					console.println("status:", response.statusCode);
					console.println("body:", response.text());
				});
			`,
			output: []string{
				"status: 200",
				"body: test",
			},
		},
		{
			name: "request_without_callback",
			script: `
				const http = require('http');
				const {env} = require('process');
				
				const req = http.request(env.get("testURL"));
				req.on("response", (response) => {
					console.println("got response:", response.statusCode);
				});
				req.end();  // No callback
			`,
			output: []string{
				"got response: 200",
			},
		},
		{
			name: "header_number_value",
			script: `
				const http = require('http');
				const {env} = require('process');
				
				const req = http.request(env.get("testURL"));
				req.setHeader("X-Number", 12345);
				req.setHeader("X-Boolean", true);
				
				console.println("X-Number:", req.getHeader("X-Number"));
				console.println("X-Boolean:", req.getHeader("X-Boolean"));
				
				req.end();
			`,
			output: []string{
				"X-Number: 12345",
				"X-Boolean: true",
			},
		},
		{
			name: "connection_refused",
			script: `
				const http = require('http');
				
				let errorOccurred = false;
				
				const req = http.request({
					protocol: 'http:',
					host: '127.0.0.1',
					port: 59999,  // Port that should not be listening
					path: '/test',
					method: 'GET'
				});
				
				req.on('error', (err) => {
					errorOccurred = true;
					console.println("error event fired:", err.message.includes("connection refused") || err.message.includes("connect") || err.message.includes("dial"));
				});
				
				req.on('response', (res) => {
					console.println("unexpected response received");
				});
				
				req.end();
				
				// Wait a bit for error event to fire
				setTimeout(() => {
					console.println("error occurred:", errorOccurred);
				}, 100);
			`,
			output: []string{
				"error event fired: true",
				"error occurred: true",
			},
		},
		{
			name: "invalid_hostname",
			script: `
				const http = require('http');
				
				let errorOccurred = false;
				
				const req = http.request({
					protocol: 'http:',
					host: 'invalid-hostname-that-does-not-exist-12345.com',
					port: 80,
					path: '/test',
					method: 'GET'
				});
				
				req.on('error', (err) => {
					errorOccurred = true;
					console.println("error event fired:", true);
				});
				
				req.on('response', (res) => {
					console.println("unexpected response received");
				});
				
				req.end();
				
				// Wait for DNS resolution to fail
				setTimeout(() => {
					console.println("error occurred:", errorOccurred);
				}, 200);
			`,
			output: []string{
				"error event fired: true",
				"error occurred: true",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(echoServer))
	defer server.Close()

	for _, tc := range tests {
		tc.vars = map[string]any{
			"testURL": server.URL,
		}
		RunTest(t, tc)
	}
}
