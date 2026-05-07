package ws_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib"
	httpmod "github.com/machbase/neo-server/v8/jsh/lib/http"
	wsmod "github.com/machbase/neo-server/v8/jsh/lib/ws"
	"github.com/machbase/neo-server/v8/jsh/root"
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
					console.println("constructor error:", e.message);
				}
			`,
			Output: []string{
				"constructor error: URL must be a string, got undefined",
			},
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

func TestWebSocketServer(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test file path")
	}
	baseDir := filepath.Dir(filename)
	workSource := filepath.Join(baseDir, "..", "..", "test")

	serverOutput := &bytes.Buffer{}
	serverDone := make(chan error, 1)
	address := "127.0.0.1:29887"
	serverScript := `
		const http = require('http');
		const {WebSocketServer} = require('ws');
		const server = new http.Server({network:'tcp', address:'` + address + `'});
		const wss = new WebSocketServer({server, path:'/ws'});
		const verified = new WebSocketServer({
			server,
			path:'/verify',
			verifyClient: ({req}) => req.query('token') === 'allow',
		});
		const protocols = new WebSocketServer({
			server,
			path:'/proto',
			handleProtocols: (items, req) => {
				if (items.indexOf('machbase.rpc') >= 0) {
					return 'machbase.rpc';
				}
				return false;
			},
		});
		const tracked = new WebSocketServer({server, path:'/tracked'});
		wss.on('error', (err) => {
			console.println('wss error:', err.message || String(err));
		});
		setTimeout(() => {
			wss.close();
			verified.close();
			protocols.close();
			tracked.close();
			server.close();
		}, 3000);

		server.get('/health', (ctx) => {
			ctx.text(http.status.OK, 'ok');
		});
		server.get('/tracked-size', (ctx) => {
			ctx.text(http.status.OK, String(tracked.clients.size));
		});
		server.get('/shutdown', (ctx) => {
			ctx.text(http.status.OK, 'bye');
			setImmediate(() => {
				wss.close();
				verified.close();
				protocols.close();
				tracked.close();
				server.close();
			});
		});

		wss.on('connection', (socket) => {
			socket.on('message', (event) => {
				socket.send('echo:' + event.data);
				socket.close();
			});
		});

		verified.on('connection', (socket, request) => {
			socket.send('verified:' + request.query('token') + ':' + request.httpVersion + ':' + String(request.socket.remoteAddress !== ''));
			socket.close();
		});

		protocols.on('connection', (socket, request) => {
			socket.send('protocol:' + socket.protocol + ':' + String(request.hasHeader('sec-websocket-protocol')));
			socket.close();
		});

		tracked.on('connection', (socket) => {
			socket.send('tracked:' + tracked.clients.size);
		});

		server.ws('/sugar', (socket, request) => {
			socket.on('message', (event) => {
				socket.send('sugar:' + event.data + ':' + request.path + ':' + request.httpVersion);
				socket.close();
			});
		});

		server.serve((result) => {
			console.println('server started:', result.address);
		});
	`

	conf := engine.Config{
		Name: "TestWebSocketServer",
		Code: serverScript,
		FSTabs: []engine.FSTab{
			root.RootFSTab(),
			{MountPoint: "/work", Source: workSource},
			{MountPoint: "/lib", FS: lib.LibFS()},
		},
		Env:    map[string]any{},
		Reader: &bytes.Buffer{},
		Writer: serverOutput,
	}
	jr, err := engine.New(conf)
	if err != nil {
		t.Fatalf("Failed to create JSRuntime: %v", err)
	}
	jr.RegisterNativeModule("@jsh/process", jr.Process)
	jr.RegisterNativeModule("@jsh/http", httpmod.Module)
	jr.RegisterNativeModule("@jsh/ws", wsmod.Module)
	lib.Enable(jr)
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		serverDone <- jr.RunContext(runCtx)
	}()

	time.Sleep(300 * time.Millisecond)

	tests := []test_engine.TestCase{
		{
			Name: "ws_server_http_coexist",
			Script: `
				const http = require('http');
				http.get('http://127.0.0.1:29887/health', (response) => {
					console.println('health:', response.text());
				});
			`,
			Output: []string{
				"health: ok",
			},
		},
		{
			Name: "ws_server_echo",
			Script: `
				const {WebSocket} = require('ws');
				const ws = new WebSocket('ws://127.0.0.1:29887/ws?client=neo');
				const hold = setInterval(() => {}, 100);
				let closed = false;
				ws.on('open', () => {
					console.println('client open');
					ws.send('ping');
				});
				ws.on('message', (event) => {
					console.println('client recv:', event.data);
					ws.close();
				});
				ws.on('close', () => {
					if (closed) {
						return;
					}
					closed = true;
					clearInterval(hold);
					console.println('client close');
				});
			`,
			Output: []string{
				"client open",
				"client recv: echo:ping",
				"client close",
			},
		},
		{
			Name: "ws_server_verify_client_accept",
			Script: `
				const {WebSocket} = require('ws');
				const hold = setInterval(() => {}, 100);
				const ws = new WebSocket('ws://127.0.0.1:29887/verify?token=allow');
				let closed = false;
				ws.on('message', (event) => {
					console.println('verify recv:', event.data);
					ws.close();
				});
				ws.on('close', () => {
					if (closed) {
						return;
					}
					closed = true;
					clearInterval(hold);
					console.println('verify close');
				});
			`,
			Output: []string{
				"verify recv: verified:allow:1.1:true",
				"verify close",
			},
		},
		{
			Name: "ws_server_verify_client_reject",
			Script: `
				const {WebSocket} = require('ws');
				const hold = setInterval(() => {}, 100);
				const ws = new WebSocket('ws://127.0.0.1:29887/verify?token=deny');
				ws.on('error', (err) => {
					console.println('verify error:', err.message);
					clearInterval(hold);
				});
			`,
			Output: []string{
				"verify error: websocket: bad handshake",
			},
		},
		{
			Name: "ws_server_handle_protocols",
			Script: `
				const {WebSocket} = require('ws');
				const hold = setInterval(() => {}, 100);
				const ws = new WebSocket('ws://127.0.0.1:29887/proto', ['chat', 'machbase.rpc']);
				let closed = false;
				ws.on('open', () => {
					console.println('proto open:', ws.protocol);
				});
				ws.on('message', (event) => {
					console.println('proto recv:', event.data);
					ws.close();
				});
				ws.on('close', () => {
					if (closed) {
						return;
					}
					closed = true;
					clearInterval(hold);
					console.println('proto close');
				});
			`,
			Output: []string{
				"proto open: machbase.rpc",
				"proto recv: protocol:machbase.rpc:true",
				"proto close",
			},
		},
		{
			Name: "ws_server_clients_tracking",
			Script: `
				const http = require('http');
				const {WebSocket} = require('ws');
				const hold = setInterval(() => {}, 100);
				const ws = new WebSocket('ws://127.0.0.1:29887/tracked');
				let closed = false;
				let attempts = 0;
				function checkTrackedSize() {
					http.get('http://127.0.0.1:29887/tracked-size', (response) => {
						const size = response.text();
						if (size === '0' || attempts >= 20) {
							console.println('tracked size:', size);
							clearInterval(hold);
							return;
						}
						attempts++;
						setTimeout(checkTrackedSize, 20);
					});
				}
				ws.on('message', (event) => {
					console.println('tracked recv:', event.data);
					ws.close();
				});
				ws.on('close', () => {
					if (closed) {
						return;
					}
					closed = true;
					console.println('tracked close');
					checkTrackedSize();
				});
			`,
			Output: []string{
				"tracked recv: tracked:1",
				"tracked close",
				"tracked size: 0",
			},
		},
		{
			Name: "ws_server_sugar",
			Script: `
				const {WebSocket} = require('ws');
				const hold = setInterval(() => {}, 100);
				const ws = new WebSocket('ws://127.0.0.1:29887/sugar');
				let closed = false;
				ws.on('open', () => {
					ws.send('hello');
				});
				ws.on('message', (event) => {
					console.println('sugar recv:', event.data);
					ws.close();
				});
				ws.on('close', () => {
					if (closed) {
						return;
					}
					closed = true;
					clearInterval(hold);
					console.println('sugar close');
				});
			`,
			Output: []string{
				"sugar recv: sugar:hello:/sugar:1.1",
				"sugar close",
			},
		},
		{
			Name: "ws_server_shutdown",
			Script: `
				const http = require('http');
				http.get('http://127.0.0.1:29887/shutdown', (response) => {
					console.println('shutdown:', response.text());
				});
			`,
			Output: []string{
				"shutdown: bye",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}
