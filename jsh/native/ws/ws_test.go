package ws

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/root"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// echoServer is a simple WebSocket echo server for testing
func echoServer(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if err := conn.WriteMessage(messageType, message); err != nil {
			break
		}
	}
}

type TestCase struct {
	name   string
	script string
	input  []string
	output []string
	err    string
	vars   map[string]any
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
		jr.RegisterNativeModule("@jsh/ws", Module)

		if len(tc.input) > 0 {
			conf.Reader.(*bytes.Buffer).WriteString(strings.Join(tc.input, ""))
		}
		if err := jr.Run(); err != nil {
			if tc.err == "" || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("Unexpected error: %v", err)
			}
			return
		}

		gotOutput := conf.Writer.(*bytes.Buffer).String()
		lines := strings.Split(gotOutput, "\n")
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

func TestWebSocket(t *testing.T) {
	tests := []TestCase{
		{
			name: "module",
			script: `
				const {WebSocket} = require("/lib/ws");
				console.println("CONNECTING:", WebSocket.CONNECTING);
				console.println("OPEN:", WebSocket.OPEN);
				console.println("CLOSING:", WebSocket.CLOSING);
				console.println("CLOSED:", WebSocket.CLOSED);
			`,
			output: []string{
				"CONNECTING: 0",
				"OPEN: 1",
				"CLOSING: 2",
				"CLOSED: 3",
			},
		},
		{
			name: "constructor-no-args",
			script: `
				const m1 = require("/lib/ws");
				try {
					new m1.WebSocket();
				} catch(e) {
					throw e;
				}
			`,
			err: "URL must be a string",
		},
		{
			name: "constructor",
			script: `
				const {WebSocket} = require("/lib/ws");
				const ws = new WebSocket("ws://localhost:8080");
				console.println(ws.url);
			`,
			output: []string{
				"ws://localhost:8080",
			},
		},
	}
	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestWebSocketConnection(t *testing.T) {

	tests := []TestCase{
		{
			name: "connect",
			script: `
				const {env} = require('/lib/process');
				const {WebSocket} = require("/lib/ws");
				const ws = new WebSocket(env.get("testURL"));
				ws.on("error", function(err){
					console.log("websocket error: " + err.message);
				});
				ws.on("open", function(){
					console.log("websocket open");
					setTimeout(()=>{ ws.close() }, 500);
				});
				ws.on("close", function(){ 
					console.log("websocket closed");
				});
			`,
			output: []string{
				"INFO  websocket open",
				"INFO  websocket closed",
			},
		},
		{
			name: "close",
			script: `
				const {env} = require('/lib/process');
				const {WebSocket} = require("/lib/ws");
				const ws = new WebSocket(env.get("testURL"));
				ws.on("open", function() {
					console.println("websocket open");
					ws.close();
				});
				ws.on("close", ()=>{
					console.println("websocket closed");
				});
			`,
			output: []string{
				"websocket open",
				"websocket closed",
			},
		},
		{
			name: "send_receive",
			script: `
				const {env} = require('/lib/process');
				const {WebSocket} = require("/lib/ws");
				const ws = new WebSocket(env.get("testURL"));
				ws.on("error", function(err){
					console.log("websocket error: " + err);
				});
				ws.on("close", function(evt){
					console.log("websocket closed");
				});
				ws.on("open", function() {
					console.log("websocket open");
					for (let i = 0; i < 3; i++) {
						ws.send("test message "+i);
					}
				});
				ws.on("message", (evt) => {
					console.println(evt.data);
				});
				setTimeout(function(){ ws.close(); }, 100);	
			`,
			output: []string{
				"INFO  websocket open",
				"test message 0",
				"test message 1",
				"test message 2",
				"INFO  websocket closed",
			},
		},
		{
			name: "multiple_event_listeners",
			script: `
				const {env} = require('/lib/process');
				const {WebSocket} = require("/lib/ws");
				const ws = new WebSocket(env.get("testURL"));
				const onMessage = function(m) {
					console.println("got: "+m.data);
				}
				ws.on("message", onMessage);
				ws.addListener("message", onMessage);
				ws.on("open", function() {
					ws.send("trigger message");
					setTimeout(function() { ws.close(); }, 500);
				});
				ws.on("close", () => { console.println("websocket closed"); });
			`,
			output: []string{
				"got: trigger message",
				"got: trigger message",
				"websocket closed",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(echoServer))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	for _, tc := range tests {
		tc.vars = map[string]any{
			"testURL": wsURL,
		}
		RunTest(t, tc)
	}
}

func TestWebSocketConnectionError(t *testing.T) {
	tests := []TestCase{
		{
			name: "connection_error",
			script: `
				const {WebSocket} = require("/lib/ws");
				const ws = new WebSocket("ws://127.0.0.1:9999");
				ws.on("error", function(err){
					console.println("err:",err.message);
				});
			`,
			output: []string{
				func() string {
					if runtime.GOOS == "windows" {
						return "err: dial tcp 127.0.0.1:9999: connectex: No connection could be made because the target machine actively refused it."
					} else {
						return "err: dial tcp 127.0.0.1:9999: connect: connection refused"
					}
				}(),
			},
		},
		{
			name: "send_without_connection",
			script: `
				const {WebSocket} = require("/lib/ws");
				const ws = new WebSocket("ws://127.0.0.1:9999");
				ws.on("error", function(err){
					console.println("err:", err.message);
				});
				setTimeout(function() {
					ws.send("test message");
				}, 500);
			`,
			output: []string{
				func() string {
					if runtime.GOOS == "windows" {
						return "err: dial tcp 127.0.0.1:9999: connectex: No connection could be made because the target machine actively refused it."
					} else {
						return "err: dial tcp 127.0.0.1:9999: connect: connection refused"
					}
				}(),
				"err: websocket is not open",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}
