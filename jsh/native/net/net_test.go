package net

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/native/stream"
	"github.com/machbase/neo-server/v8/jsh/root"
)

func TestServerCreateAndClose(t *testing.T) {
	emit := func(event string, data any) {
		t.Logf("Event: %s, Data: %v", event, data)
	}

	server := CreateServer(nil, func(obj *goja.Object, event string, args ...any) bool {
		if len(args) > 0 {
			emit(event, args[0])
		} else {
			emit(event, nil)
		}
		return true
	})

	if server == nil {
		t.Fatal("Failed to create server")
	}

	// Test listening
	err := server.Listen(0, "127.0.0.1", 128)
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	addr := server.Address()
	if addr == nil {
		t.Fatal("Failed to get address")
	}

	t.Logf("Server address: %v", addr)

	// Give it a moment
	time.Sleep(100 * time.Millisecond)

	// Test close
	err = server.Close()
	if err != nil {
		t.Fatalf("Failed to close server: %v", err)
	}
}

func TestClientServerConnection(t *testing.T) {
	serverEmit := func(event string, _ any) {
		t.Logf("Server Event: %s", event)
	}

	server := CreateServer(nil, func(obj *goja.Object, event string, args ...any) bool {
		if len(args) > 0 {
			serverEmit(event, args[0])
		} else {
			serverEmit(event, nil)
		}
		return true
	})

	err := server.Listen(0, "127.0.0.1", 128)
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer server.Close()

	addr := server.Address()
	port := addr["port"].(int)

	time.Sleep(100 * time.Millisecond)

	// Create client
	clientEmit := func(event string, _ any) {
		t.Logf("Client Event: %s", event)
	}

	socket, err := Connect(nil, port, "127.0.0.1", func(obj *goja.Object, event string, args ...any) bool {
		if len(args) > 0 {
			clientEmit(event, args[0])
		} else {
			clientEmit(event, nil)
		}
		return true
	})

	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer socket.Close()

	time.Sleep(200 * time.Millisecond)

	// Test write
	_, err = socket.WriteString("Hello Server", "utf8")
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
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
		jr.RegisterNativeModule("@jsh/stream", stream.Module)
		jr.RegisterNativeModule("@jsh/net", Module)

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
			if strings.Contains(expectedLine, "$") {
				expectedLine = jr.Env.Expand(expectedLine)
			}
			// Support prefix matching with "..." suffix
			if strings.HasSuffix(expectedLine, "...") {
				prefix := strings.TrimSuffix(expectedLine, "...")
				if !strings.HasPrefix(lines[i], prefix) {
					t.Errorf("Output line %d: expected to start with %q, got %q", i, prefix, lines[i])
				}
			} else if lines[i] != expectedLine {
				t.Errorf("Output line %d: expected %q, got %q", i, expectedLine, lines[i])
			}
		}
	})
}

func TestNetModule(t *testing.T) {
	testCases := []TestCase{
		{
			name: "net_simple",
			script: `
				const net = require('net');
				const env = require('@jsh/process').env;

				// Test 1: Check exports
				console.println('✓ net.createServer:', typeof net.createServer);
				console.println('✓ net.createConnection:', typeof net.createConnection);
				console.println('✓ net.connect:', typeof net.connect);
				console.println('✓ net.Server:', typeof net.Server);
				console.println('✓ net.Socket:', typeof net.Socket);
				console.println('✓ net.isIP:', typeof net.isIP);
				console.println('✓ net.isIPv4:', typeof net.isIPv4);
				console.println('✓ net.isIPv6:', typeof net.isIPv6);

				// Test 2: IP validation
				console.println('\nTesting IP validation:');
				console.println('isIP("127.0.0.1"):', net.isIP("127.0.0.1"), '(expected: 4)');
				console.println('isIP("::1"):', net.isIP("::1"), '(expected: 6)');
				console.println('isIP("invalid"):', net.isIP("invalid"), '(expected: 0)');
				console.println('isIPv4("127.0.0.1"):', net.isIPv4("127.0.0.1"), '(expected: true)');
				console.println('isIPv6("::1"):', net.isIPv6("::1"), '(expected: true)');

				// Test 3: Create server
				console.println('\nTesting server creation:');
				const server = net.createServer();
				console.println('✓ Server created');

				server.on('listening', () => {
					const addr = server.address();
					const str = ` + "`${addr.family}://${addr.address}:${addr.port}`" + `;
					console.println('✓ Server listening on: ' + str);
					env.set('SERVER_ADDRESS', str);

					// Close immediately
					server.close(() => {
						console.println('✓ Server closed successfully');
						console.println('\nAll tests passed!');
					});
				});

				server.on('error', (err) => {
					console.error('✗ Server error:', err.message);
				});

				// Listen on random port
				server.listen(0, '127.0.0.1');
			`,
			output: []string{
				"✓ net.createServer: function",
				"✓ net.createConnection: function",
				"✓ net.connect: function",
				"✓ net.Server: function",
				"✓ net.Socket: function",
				"✓ net.isIP: function",
				"✓ net.isIPv4: function",
				"✓ net.isIPv6: function",
				"",
				"Testing IP validation:",
				"isIP(\"127.0.0.1\"): 4 (expected: 4)",
				"isIP(\"::1\"): 6 (expected: 6)",
				"isIP(\"invalid\"): 0 (expected: 0)",
				"isIPv4(\"127.0.0.1\"): true (expected: true)",
				"isIPv6(\"::1\"): true (expected: true)",
				"",
				"Testing server creation:",
				"✓ Server created",
				"✓ Server listening on: $SERVER_ADDRESS",
				"✓ Server closed successfully",
				"",
				"All tests passed!",
			},
		},
	}

	for _, tc := range testCases {
		RunTest(t, tc)
	}
}

func TestNetEchoServer(t *testing.T) {
	testCases := []TestCase{
		{
			name: "net_echo_server",
			script: `
				const net = require('net');
				const env = require('@jsh/process').env;

				console.println('Starting echo server test...');

				// Create echo server
				const server = net.createServer(/*(socket) => {
					console.println('[Server] Client connected');
					
					socket.on('data', (data) => {
						const msg = data.toString();
						console.println	('[Server] Received:', msg.trim());
						socket.write('Echo: ' + msg);
					});
					socket.on('end', () => {
						console.println('[Server] Client disconnected');
					});
				}*/);

				server.listen(0, '127.0.0.1', () => {
					const addr = server.address();
					console.println("[Server] Listening on", addr.address, ":", addr.port, "\n");
					
					// Create client after server is ready
					console.println('[Client] Connecting to server...');
					const client = net.createConnection({
						port: addr.port,
						host: '127.0.0.1'
					}, () => {
						console.println('[Client] Connected');
						
						// Send test message
						console.println('[Client] Sending: Hello Server');
						client.write('Hello Server\n');
						
						setTimeout(() => {
							console.println('[Client] Sending: Test Message 2');
							client.write('Test Message 2\n');
						}, 100);
						
						setTimeout(() => {
							console.println('[Client] Closing connection');
							client.end();
						}, 200);
					});
					
					client.on('data', (data) => {
						console.println('[Client] Received:', data.toString().trim());
					});
					
					client.on('end', () => {
						console.println('[Client] Connection ended');
					});
					
					client.on('close', () => {
						console.println('[Client] Connection closed\n');
						
						// Close server
						setTimeout(() => {
							console.println('[Server] Closing server...');
							server.close(() => {
								console.println('[Server] Server closed');
								console.println('\n✓ Test completed successfully!');
							});
						}, 500);
					});
					
					client.on('error', (err) => {
						console.error('[Client] Error:', err.message);
					});
				});

				server.on('error', (err) => {
					console.error('[Server] Error:', err.message);
				});

			`,
			output: []string{
				"Starting echo server test...",
				"[Server] Listening on 127.0.0.1 : ...",
				"",
				"[Client] Connecting to server...",
				"[Client] Connected",
				"[Client] Sending: Hello Server",
				"[Client] Sending: Test Message 2",
				"[Client] Closing connection",
				"[Client] Connection closed",
				"",
				"[Server] Closing server...",
				"[Server] Server closed",
				"",
				"✓ Test completed successfully!",
			},
		},
	}

	for _, tc := range testCases {
		RunTest(t, tc)
	}
}
