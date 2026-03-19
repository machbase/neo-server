package nats_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
	nserver "github.com/nats-io/nats-server/v2/server"
)

var natsAddr string

func TestMain(m *testing.M) {
	opts := &nserver.Options{
		Host:   "127.0.0.1",
		Port:   -1,
		NoLog:  true,
		NoSigs: true,
	}
	svr, err := nserver.NewServer(opts)
	if err != nil {
		panic(err)
	}
	go svr.Start()
	if !svr.ReadyForConnections(10 * time.Second) {
		panic("nats server did not start in time")
	}
	natsAddr = "nats://" + svr.Addr().String()
	code := m.Run()
	svr.Shutdown()
	os.Exit(code)
}

func TestNatsConfig(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "config",
			Script: `
				const addr = require("process").env.get('natsAddr');
				const nativeNats = require('@jsh/nats');
				const config = nativeNats.parseConfig(JSON.stringify({
					servers: [addr],
					name: "test-client",
					allowReconnect: true,
					maxReconnect: 10,
					reconnectWait: 2000,
					timeout: 10*1000,
				}));
				console.println("SERVERS:", config.servers);
				console.println("NAME:", config.name);
				console.println("ALLOW_RECONNECT:", config.allowReconnect);
				console.println("MAX_RECONNECT:", config.maxReconnect);
				console.println("RECONNECT_WAIT:", config.reconnectWait);
				console.println("TIMEOUT:", config.timeout);
			`,
			Output: []string{
				fmt.Sprintf("SERVERS: [%s]", natsAddr),
				"NAME: test-client",
				"ALLOW_RECONNECT: true",
				"MAX_RECONNECT: 10",
				"RECONNECT_WAIT: 2s",
				"TIMEOUT: 10s",
			},
			Vars: map[string]any{
				"natsAddr": natsAddr,
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestNats(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "basic_ops",
			Script: `
				const addr = require("process").env.get('natsAddr');
				const nats = require("nats");
				const client = new nats.Client({
					servers: [addr],
					name: "test-client",
					timeout: 10*1000,
				});
				client.on('open', () => {
					console.println("Connected");
					client.subscribe('test.subject');
				});
				client.on('error', (err) => {
					console.println("Error:", err.message);
				});
				client.on('close', () => {
					console.println("Disconnected");
				});
				client.on('message', (msg) => {
					console.println("Message received on subject:", msg.subject, "payload:", msg.payload);
					setTimeout(() => {
						client.close();
					}, 200);
				});
				client.on('subscribed', (subject, reason) => {
					console.println("Subscribed to:", subject, "reason:", reason);
					client.publish('test.subject', 'Hello, NATS!');
				});
				client.on('published', (subject, reason) => {
					console.println("Published to:", subject, "reason:", reason);
				});
			`,
			Output: []string{
				"Connected",
				"Subscribed to: test.subject reason: 1",
				"Published to: test.subject reason: 0",
				"Message received on subject: test.subject payload: Hello, NATS!",
				"Disconnected",
			},
			Vars: map[string]any{
				"natsAddr": natsAddr,
			},
		},
		{
			Name: "request_reply",
			Script: `
				const addr = require("process").env.get('natsAddr');
				const nats = require("nats");
				const subscriber = new nats.Client({
					servers: [addr],
					name: "request-handler",
					timeout: 10*1000,
				});
				const requester = new nats.Client({
					servers: [addr],
					name: "requester",
					timeout: 10*1000,
				});
				let handlerReady = false;
				let replyReady = false;
				let sent = false;

				function sendRequest() {
					if (!handlerReady || !replyReady || sent) {
						return;
					}
					sent = true;
					requester.publish('request.subject', 'ping', { reply: 'reply.subject' });
				}

				subscriber.on('open', () => {
					subscriber.subscribe('request.subject');
				});
				subscriber.on('subscribed', (subject, reason) => {
					console.println('Handler subscribed:', subject, 'reason:', reason);
					handlerReady = true;
					sendRequest();
				});
				subscriber.on('message', (msg) => {
					console.println('Handler reply subject:', msg.reply);
					subscriber.publish(msg.reply, 'pong');
				});

				requester.on('open', () => {
					requester.subscribe('reply.subject');
				});
				requester.on('subscribed', (subject, reason) => {
					console.println('Requester subscribed:', subject, 'reason:', reason);
					replyReady = true;
					sendRequest();
				});
				requester.on('message', (msg) => {
					console.println('Requester received:', msg.payload);
					setTimeout(() => {
						requester.close();
						subscriber.close();
					}, 100);
				});
			`,
			Output: []string{
				"Handler subscribed: request.subject reason: 1",
				"Requester subscribed: reply.subject reason: 1",
				"Handler reply subject: reply.subject",
				"Requester received: pong",
			},
			Vars: map[string]any{
				"natsAddr": natsAddr,
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}
