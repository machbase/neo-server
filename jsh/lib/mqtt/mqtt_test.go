package mqtt_test

import (
	"bytes"
	"encoding/json"
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
	broker.AddHook(&AuthHook{HookBase: &mqtt.HookBase{}, broker: broker}, nil)
	return broker
}

type AuthHook struct {
	*mqtt.HookBase
	broker *mqtt.Server
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
		mqtt.OnPublished,
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

func (h *AuthHook) OnPublished(cl *mqtt.Client, pk packets.Packet) {
	if h.broker == nil {
		return
	}
	if pk.TopicName != "test/echo-user-properties" {
		return
	}
	if pk.Properties.ResponseTopic == "" {
		return
	}

	user := map[string]string{}
	for _, prop := range pk.Properties.User {
		user[prop.Key] = prop.Val
	}
	payload, err := json.Marshal(map[string]any{
		"topic":         pk.TopicName,
		"payload":       string(pk.Payload),
		"responseTopic": pk.Properties.ResponseTopic,
		"user":          user,
	})
	if err != nil {
		return
	}
	_ = h.broker.Publish(pk.Properties.ResponseTopic, payload, false, 1)
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
				const addr = require("process").env.get('brokerAddr');
				const mqtt = require("mqtt");
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
				"PASSWORD: pass",
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
				const addr = require("process").env.get('brokerAddr');
				const mqtt = require("mqtt");
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
					client.subscribe('test/topic', {
						qos: 0,
						noLocal: false,
						retainAsPublished: true,
						retainHandling: 0,
						properties: {
							subscriptionIdentifier: 7,
							user: {
								source: 'basic_ops',
							},
						},
					});
				});
				client.on('error', (err) => {
					console.println("Error:", err.message);
				});
				client.on('close', () => {
					console.println("Disconnected");
				});
				client.on('message', (msg) => {
					console.println("Message received on topic:", msg.topic, "payload:", msg.payloadText);
					client.unsubscribe(msg.topic, {
						properties: {
							user: {
								source: 'basic_ops',
							},
						},
					});
				});
				client.on('subscribed', (topic, reason) => {
					console.println("Subscribed to:", topic, "reason:", reason);
					client.publish('test/topic', 'Hello, MQTT!');
				});
				client.on('unsubscribed', (topic, reason) => {
					console.println("Unsubscribed from:", topic, "reason:", reason);
					setTimeout(() => {
						client.close();
					}, 500);
				});
				client.on('published', (topic, reason) => {
					console.println("Published to:", topic, "Payload:", reason);
				});
			`,
			Output: []string{
				"Connected",
				"Subscribed to: test/topic reason: 0",
				"Published to: test/topic Payload: 0",
				"Message received on topic: test/topic payload: Hello, MQTT!",
				"Unsubscribed from: test/topic reason: 0",
				"Disconnected",
			},
			Vars: map[string]any{
				"brokerAddr": brokerAddr,
			},
		},
		{
			Name: "binary_message_payload",
			Script: `
				const addr = require("process").env.get('brokerAddr');
				const mqtt = require("mqtt");
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
					client.subscribe('test/binary');
				});
				client.on('error', (err) => {
					console.println("Error:", err.message);
				});
				client.on('close', () => {
					console.println("Disconnected");
				});
				client.on('subscribed', (topic, reason) => {
					console.println("Subscribed to:", topic, "reason:", reason);
					client.publish(topic, new Uint8Array([0, 1, 2, 255]));
				});
				client.on('published', (topic, reason) => {
					console.println("Published to:", topic, "Payload:", reason);
				});
				client.on('message', (msg) => {
					console.println("Payload is buffer:", Buffer.isBuffer(msg.payload));
					console.println("Payload bytes:", Array.from(msg.payload).join(','));
					client.unsubscribe(msg.topic);
				});
				client.on('unsubscribed', (topic, reason) => {
					console.println("Unsubscribed from:", topic, "reason:", reason);
					client.close();
				});
			`,
			Output: []string{
				"Connected",
				"Subscribed to: test/binary reason: 1",
				"Published to: test/binary Payload: 0",
				"Payload is buffer: true",
				"Payload bytes: 0,1,2,255",
				"Unsubscribed from: test/binary reason: 0",
				"Disconnected",
			},
			Vars: map[string]any{
				"brokerAddr": brokerAddr,
			},
		},
		{
			Name: "message_properties",
			Script: `
				const addr = require("process").env.get('brokerAddr');
				const mqtt = require("mqtt");
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
					client.subscribe('test/properties', {
						qos: 1,
						properties: {
							subscriptionIdentifier: 9,
						},
					});
				});
				client.on('error', (err) => {
					console.println("Error:", err.message);
				});
				client.on('close', () => {
					console.println("Disconnected");
				});
				client.on('subscribed', (topic, reason) => {
					console.println("Subscribed to:", topic, "reason:", reason);
					client.publish(topic, 'payload-with-properties', {
						qos: 1,
						properties: {
							payloadFormat: 1,
							messageExpiry: 30,
							contentType: 'text/plain',
							responseTopic: 'test/reply',
							correlationData: 'cid-123',
							user: {
								source: 'properties_test',
								format: 'text',
							},
						},
					});
				});
				client.on('published', (topic, reason) => {
					console.println("Published to:", topic, "Payload:", reason);
				});
				client.on('message', (msg) => {
					console.println("Payload text:", msg.payloadText);
					console.println("Content type:", msg.properties.contentType);
					console.println("Response topic:", msg.properties.responseTopic);
					console.println("Payload format:", msg.properties.payloadFormat);
					console.println("Message expiry:", msg.properties.messageExpiry);
					console.println("Correlation data is buffer:", Buffer.isBuffer(msg.properties.correlationData));
					console.println("Correlation data text:", msg.properties.correlationData.toString());
					console.println("User source:", msg.properties.user.source);
					console.println("User format:", msg.properties.user.format);
					client.unsubscribe(msg.topic);
				});
				client.on('unsubscribed', (topic, reason) => {
					console.println("Unsubscribed from:", topic, "reason:", reason);
					client.close();
				});
			`,
			Output: []string{
				"Connected",
				"Subscribed to: test/properties reason: 1",
				"Published to: test/properties Payload: 1",
				"Payload text: payload-with-properties",
				"Content type: text/plain",
				"Response topic: test/reply",
				"Payload format: 1",
				"Message expiry: 30",
				"Correlation data is buffer: true",
				"Correlation data text: cid-123",
				"User source: properties_test",
				"User format: text",
				"Unsubscribed from: test/properties reason: 0",
				"Disconnected",
			},
			Vars: map[string]any{
				"brokerAddr": brokerAddr,
			},
		},
		{
			Name: "publish_user_properties_broker_roundtrip",
			Script: `
				const addr = require("process").env.get('brokerAddr');
				const mqtt = require("mqtt");
				const replyTopic = 'test/echo-user-properties/reply';
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
					client.subscribe(replyTopic, { qos: 1 });
				});
				client.on('error', (err) => {
					console.println("Error:", err.message);
				});
				client.on('close', () => {
					console.println("Disconnected");
				});
				client.on('subscribed', (topic, reason) => {
					console.println("Subscribed to:", topic, "reason:", reason);
					if (topic === replyTopic) {
						client.publish('test/echo-user-properties', 'verify-user-props', {
							qos: 1,
							properties: {
								responseTopic: replyTopic,
								user: {
									source: 'broker_roundtrip',
									format: 'json',
									count: 42,
								},
							},
						});
					}
				});
				client.on('published', (topic, reason) => {
					console.println("Published to:", topic, "Payload:", reason);
				});
				client.on('message', (msg) => {
					const body = JSON.parse(msg.payloadText);
					console.println("Reply topic:", body.responseTopic);
					console.println("Reply payload:", body.payload);
					console.println("Reply user source:", body.user.source);
					console.println("Reply user format:", body.user.format);
					console.println("Reply user count:", body.user.count);
					client.unsubscribe(replyTopic);
				});
				client.on('unsubscribed', (topic, reason) => {
					console.println("Unsubscribed from:", topic, "reason:", reason);
					client.close();
				});
			`,
			Output: []string{
				"Connected",
				"Subscribed to: test/echo-user-properties/reply reason: 1",
				"Published to: test/echo-user-properties Payload: 1",
				"Reply topic: test/echo-user-properties/reply",
				"Reply payload: verify-user-props",
				"Reply user source: broker_roundtrip",
				"Reply user format: json",
				"Reply user count: 42",
				"Unsubscribed from: test/echo-user-properties/reply reason: 0",
				"Disconnected",
			},
			Vars: map[string]any{
				"brokerAddr": brokerAddr,
			},
		},
		{
			Name: "unsubscribe_closed_client",
			Script: `
				const addr = require("process").env.get('brokerAddr');
				const mqtt = require("mqtt");
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
					client.close();
					client.unsubscribe('test/topic');
				});
				client.on('error', (err) => {
					console.println("Error:", err.message);
				});
				client.on('close', () => {
					console.println("Disconnected");
				});
			`,
			Output: []string{
				"Connected",
				"Disconnected",
				"Error: mqtt client is closed",
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
