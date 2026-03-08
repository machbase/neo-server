package mqtt_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
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

func TestMqttConfig(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "config",
			Script: `
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
			Output: []string{
				fmt.Sprintf("SERVERS: [%s]", brokerAddr),
				"USERNAME: user",
				"PASSWORD: &[112 97 115 115](*[]uint8)",
				"KEEPALIVE: 60",
				"RECONNECT_BACKOFF: 2s",
				"CLEAN_START: true",
				"CONNECT_TIMEOUT: 10s",
			},
			Vars: map[string]any{
				"brokerAddr": brokerAddr,
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestMqtt(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "basic_ops",
			Script: `
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
			Output: []string{
				"Connected",
				"Subscribed to: test/topic reason: 1",
				"Published to: test/topic Payload: 0",
				"Message received on topic: test/topic payload: Hello, MQTT!",
				"Disconnected",
			},
			Vars: map[string]any{
				"brokerAddr": brokerAddr,
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}
