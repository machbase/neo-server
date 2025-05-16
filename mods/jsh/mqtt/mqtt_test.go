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
					serverUrls: ["tcp://127.0.0.1:12365"],
					queue: "memory",
					clientId: "mqtt-client-tester",
				}
				const client = new mqtt.Client(clientConfig);
				let counter = 0;
				try {
					client.onConnectError = err => { println("connect error", err); }
					client.onClientError = err => { println("client error", err); }
					client.onConnect = ack => {
						println("connected.", ack.reasonCode);
						client.onMessage = msg => {
							println("recv:", msg.topic, msg.qos, msg.payload.string())
							counter++;
						}
						client.subscribe({subscriptions:[{topic:"test/topic", qos:2}]});
					}
					client.connect({timeout:10*1000});

					let r = client.publish({topic:"test/topic", qos: 2}, "Hello, MQTT?");
					client.publish({topic:"test/topic", qos: 1}, "reason code: "+r.reasonCode);

					// wait until called publish received
					for (let i=0; i<20; i++) {
						if (counter >= 2) break;
						sleep(100);
					}
					client.unsubscribe({topics:["test/topic"]});
					client.disconnect({waitForEmptyQueue: true, timeout: 10*1000});
					println("disconnected.");
				} catch (e) {
				 	println("exception:", e);
				}
			`,
			Expect: []string{
				"connected. 0",
				"recv: test/topic 2 Hello, MQTT?",
				"recv: test/topic 1 reason code: 0",
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

var serverAddress = "127.0.0.1:12365"

func TestMain(m *testing.M) {
	// logging.Configure(&logging.Config{
	// 	Console:                     true,
	// 	Filename:                    "-",
	// 	Append:                      false,
	// 	DefaultPrefixWidth:          10,
	// 	DefaultEnableSourceLocation: true,
	// 	DefaultLevel:                "INFO",
	// })

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
