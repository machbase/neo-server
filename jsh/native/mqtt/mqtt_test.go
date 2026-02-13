package mqtt

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/root"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

var brokerAddr = "tcp://127.0.0.1:1883"

func setupTestBroker() *mqtt.Server {
	// slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
	// 	Level: slog.LevelDebug,
	// })))
	broker := mqtt.New(&mqtt.Options{
		//Logger:                 slog.Default(),
		InlineClient:           true,
		SysTopicResendInterval: 5,
		Capabilities:           mqtt.NewDefaultServerCapabilities(),
	})
	err := broker.AddListener(listeners.NewTCP(listeners.Config{
		ID:      "tcp-listener",
		Address: strings.TrimPrefix(brokerAddr, "tcp://"),
	}))
	if err != nil {
		panic(err)
	}
	broker.AddHook(&AuthHook{HookBase: &mqtt.HookBase{}}, nil)
	return broker
}

type AuthHook struct {
	*mqtt.HookBase
}

var _ mqtt.Hook = (*AuthHook)(nil)

func (h *AuthHook) ID() string {
	return "auth-hook"
}

// Provides indicates which hook methods this hook provides.
func (h *AuthHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnStarted,
		mqtt.OnStopped,
		mqtt.OnConnectAuthenticate,
		mqtt.OnACLCheck,
	}, []byte{b})
}

func (h *AuthHook) OnStarted() {
	slog.Info("AuthHook started")
}
func (h *AuthHook) OnStopped() {
	slog.Info("AuthHook stopped")
}
func (h *AuthHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	return true
}
func (h *AuthHook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	username := pk.Connect.Username
	password := pk.Connect.Password
	if string(username) == "user" && string(password) == "pass" {
		return true
	}
	return false
}

func TestMain(m *testing.M) {
	broker := setupTestBroker()
	if err := broker.Serve(); err != nil {
		panic(err)
	}
	defer broker.Close()
	m.Run()
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
		jr.RegisterNativeModule("@jsh/mqtt", Module)

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

func TestMqttConfig(t *testing.T) {
	tests := []TestCase{
		{
			name: "config",
			script: `
				const addr = require("/lib/process").env.get('brokerAddr');
				const mqtt = require("/lib/mqtt");
				const client = new mqtt.Client({
					servers: [addr],
					username: "user",
					password: "pass",
					keepAlive: 60,
					cleanStartOnInitialConnection: true,
					connectRetryDelay: 2000,
					connectTimeout: 10*1000,
				});
				console.println("SERVERS:", client.config.serverUrls);
				console.println("USERNAME:", client.config.connectUsername);
				console.println("PASSWORD:", client.config.connectPassword);
				console.println("KEEPALIVE:", client.config.keepAlive);
				console.println("RECONNECT_BACKOFF:", client.config.reconnectBackoff(1));
				console.println("CLEAN_START:", client.config.cleanStartOnInitialConnection);
				console.println("CONNECT_TIMEOUT:", client.config.connectTimeout);
				client.close();
			`,
			output: []string{
				fmt.Sprintf("SERVERS: [%s]", brokerAddr),
				"USERNAME: user",
				"PASSWORD: &[112 97 115 115](*[]uint8)",
				"KEEPALIVE: 60",
				"RECONNECT_BACKOFF: 2s",
				"CLEAN_START: true",
				"CONNECT_TIMEOUT: 10s",
			},
			vars: map[string]any{
				"brokerAddr": brokerAddr,
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestMqtt(t *testing.T) {
	tests := []TestCase{
		{
			name: "basic_ops",
			script: `
				const addr = require("/lib/process").env.get('brokerAddr');
				const mqtt = require("/lib/mqtt");
				const client = new mqtt.Client({
					servers: [addr],
					username: "user",
					password: "pass",
					keepAlive: 60,
					cleanStartOnInitialConnection: true,
					connectRetryDelay: 2000,
					connectTimeout: 10*1000,
				});
				client.on('open', () => {
					console.println("Connected");
					client.subscribe('test/topic');
				});
				client.on('error', (err) => {
					console.println("Error:", err.message);
				});
				client.on('close', () => {
					console.println("Disconnected");
				});
				client.on('message', (msg) => {
					console.println("Message received on topic:", msg.topic, "payload:", msg.payload);
					setTimeout(() => {
						client.close();
					}, 500);
				});
				client.on('subscribed', (topic, reason) => {
					console.println("Subscribed to:", topic, "reason:", reason);
					client.publish('test/topic', 'Hello, MQTT!');
				});
				client.on('published', (topic, reason) => {
					console.println("Published to:", topic, "Payload:", reason);
				});
			`,
			output: []string{
				"Connected",
				"Subscribed to: test/topic reason: 1",
				"Published to: test/topic Payload: 0",
				"Message received on topic: test/topic payload: Hello, MQTT!",
				"Disconnected",
			},
			vars: map[string]any{
				"brokerAddr": brokerAddr,
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}
