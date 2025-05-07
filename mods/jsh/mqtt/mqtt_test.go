package mqtt_test

import (
	"bytes"
	"context"
	"regexp"
	"testing"

	"github.com/machbase/neo-server/v8/mods/jsh"
	"github.com/machbase/neo-server/v8/mods/server"
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
		jsh.WithNativeModules("@jsh/process", "@jsh/mqtt"),
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
				const {println, sleep} = require("@jsh/process");
				const mqtt = require("@jsh/mqtt")

				const clientConfig = {
					serverUrls: ["tcp://127.0.0.1:1236"],
					keepAlive: 30,
					cleanStart: true,
					onConnect: (ack) => {
						println("connected.");
					},
					onConnectError: (err) => {
						println("connect error", err);
					},
					onDisconnect: (disconn) => {
						println("disconnected.");
					},
					onMessage: (msg) => {
						println("recv:", msg.topic, msg.qos, msg.payload.string())
					},
				}
				const client = new mqtt.Client(clientConfig);
				try {
					client.connect();
					client.awaitConnection(10*1000);

					client.subscribe({subscriptions:[{topic:"test/topic", qos:2}]});
					client.publish("test/topic", "Hello, MQTT?", 2);
					sleep(3000); // wait onMessage() to be called
				} catch (e) {
				 	println(e);
				}finally {
					client.disconnect();
					println("disconnected.");
				}
			`,
			Expect: []string{
				"connected.",
				"recv: test/topic 2 Hello, MQTT?",
				"disconnected.",
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

var serverAddress = "127.0.0.1:1236"

func TestMain(m *testing.M) {
	chStarted := make(chan struct{})
	svr, err := server.NewMqtt(nil,
		server.WithMqttTcpListener(serverAddress, nil),
		server.WithMqttOnStarted(func() {
			close(chStarted)
		}),
	)
	if err != nil {
		panic(err)
	}
	if err := svr.Start(); err != nil {
		panic(err)
	}
	<-chStarted
	m.Run()
	defer svr.Stop()
}
