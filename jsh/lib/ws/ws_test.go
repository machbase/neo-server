package ws_test

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/machbase/neo-server/v8/jsh/test_engine"
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

func TestWebSocket(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "module",
			Script: `
				const {WebSocket} = require("ws");
				console.println("CONNECTING:", WebSocket.CONNECTING);
				console.println("OPEN:", WebSocket.OPEN);
				console.println("CLOSING:", WebSocket.CLOSING);
				console.println("CLOSED:", WebSocket.CLOSED);
			`,
			Output: []string{
				"CONNECTING: 0",
				"OPEN: 1",
				"CLOSING: 2",
				"CLOSED: 3",
			},
		},
		{
			Name: "constructor-no-args",
			Script: `
				const m1 = require("ws");
				try {
					new m1.WebSocket();
				} catch(e) {
					throw e;
				}
			`,
			Err: "URL must be a string",
		},
		{
			Name: "constructor",
			Script: `
				const {WebSocket} = require("ws");
				const ws = new WebSocket("ws://localhost:8080");
				console.println(ws.url);
			`,
			Output: []string{
				"ws://localhost:8080",
			},
		},
	}
	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestWebSocketConnection(t *testing.T) {

	tests := []test_engine.TestCase{
		{
			Name: "connect",
			Script: `
				const {env} = require('process');
				const {WebSocket} = require("ws");
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
			Output: []string{
				"INFO  websocket open",
				"INFO  websocket closed",
			},
		},
		{
			Name: "close",
			Script: `
				const {env} = require('process');
				const {WebSocket} = require("ws");
				const ws = new WebSocket(env.get("testURL"));
				ws.on("open", function() {
					console.println("websocket open");
					ws.close();
				});
				ws.on("close", ()=>{
					console.println("websocket closed");
				});
			`,
			Output: []string{
				"websocket open",
				"websocket closed",
			},
		},
		{
			Name: "send_receive",
			Script: `
				const {env} = require('process');
				const {WebSocket} = require("ws");
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
			Output: []string{
				"INFO  websocket open",
				"test message 0",
				"test message 1",
				"test message 2",
				"INFO  websocket closed",
			},
		},
		{
			Name: "multiple_event_listeners",
			Script: `
				const {env} = require('process');
				const {WebSocket} = require("ws");
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
			Output: []string{
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
		tc.Vars = map[string]any{
			"testURL": wsURL,
		}
		test_engine.RunTest(t, tc)
	}
}

func TestWebSocketConnectionError(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "connection_error",
			Script: `
				const {WebSocket} = require("ws");
				const ws = new WebSocket("ws://127.0.0.1:9999");
				ws.on("error", function(err){
					console.println("err:",err.message);
				});
			`,
			Output: []string{
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
			Name: "send_without_connection",
			Script: `
				const {WebSocket} = require("ws");
				const ws = new WebSocket("ws://127.0.0.1:9999");
				ws.on("error", function(err){
					console.println("err:", err.message);
				});
				setTimeout(function() {
					ws.send("test message");
				}, 500);
			`,
			Output: []string{
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
		test_engine.RunTest(t, tc)
	}
}
