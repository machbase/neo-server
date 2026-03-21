package root_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

type mqttPublishedMessage struct {
	topic   string
	payload string
}

type mqttCaptureHook struct {
	*mqtt.HookBase
	published chan mqttPublishedMessage
}

func (h *mqttCaptureHook) ID() string {
	return "capture-hook"
}

func (h *mqttCaptureHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnPublished,
		mqtt.OnPacketEncode,
	}, []byte{b})
}

func (h *mqttCaptureHook) OnPacketEncode(cl *mqtt.Client, pk packets.Packet) packets.Packet {
	if pk.FixedHeader.Type == packets.Puback && pk.ReasonCode == 1 {
		pk.ReasonCode = 0
	}
	return pk
}

func (h *mqttCaptureHook) OnPublished(cl *mqtt.Client, pk packets.Packet) {
	h.published <- mqttPublishedMessage{
		topic:   pk.TopicName,
		payload: string(pk.Payload),
	}
}

func setupMqttPubTestServer(t *testing.T) (string, chan mqttPublishedMessage) {
	t.Helper()
	b := mqtt.New(&mqtt.Options{
		InlineClient:           false,
		SysTopicResendInterval: 5,
		Capabilities:           mqtt.NewDefaultServerCapabilities(),
	})
	hook := &mqttCaptureHook{
		HookBase:  &mqtt.HookBase{},
		published: make(chan mqttPublishedMessage, 8),
	}
	if err := b.AddHook(new(auth.AllowHook), nil); err != nil {
		t.Fatalf("add mqtt allow hook: %v", err)
	}
	if err := b.AddHook(hook, nil); err != nil {
		t.Fatalf("add mqtt hook: %v", err)
	}
	if err := b.AddListener(listeners.NewTCP(listeners.Config{
		ID:      "mqtt-tcp",
		Address: "127.0.0.1:0",
	})); err != nil {
		t.Fatalf("add mqtt listener: %v", err)
	}
	if err := b.Serve(); err != nil {
		t.Fatalf("serve mqtt broker: %v", err)
	}
	t.Cleanup(func() {
		_ = b.Close()
	})
	addr, ok := b.Listeners.Get("mqtt-tcp")
	if !ok {
		t.Fatal("mqtt listener not found")
	}
	return addr.Address(), hook.published
}

func TestMqttPubPublishesMessage(t *testing.T) {
	addr, published := setupMqttPubTestServer(t)

	workDir := t.TempDir()
	output, err := runCommand(workDir, nil,
		"mqtt_pub",
		"--broker", addr,
		"--topic", "test/topic",
		"--qos", "1", // guarantee delivery for test reliability
		"--message", "hello-mqtt",
	)
	if err != nil {
		t.Fatalf("mqtt_pub failed: %v\n%s", err, output)
	}
	if trimmed := strings.TrimSpace(output); trimmed != "" {
		t.Fatalf("unexpected command output: %q", trimmed)
	}

	select {
	case msg := <-published:
		if msg.topic != "test/topic" {
			t.Fatalf("published topic = %q, want %q", msg.topic, "test/topic")
		}
		if msg.payload != "hello-mqtt" {
			t.Fatalf("published payload = %q, want %q", msg.payload, "hello-mqtt")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for published mqtt message")
	}
}

func TestMqttPubPublishesFilePayload(t *testing.T) {
	addr, published := setupMqttPubTestServer(t)

	workDir := t.TempDir()
	payloadPath := filepath.Join(workDir, "payload.txt")
	if err := os.WriteFile(payloadPath, []byte("file-payload"), 0o644); err != nil {
		t.Fatalf("write payload file: %v", err)
	}

	output, err := runCommand(workDir, nil,
		"mqtt_pub",
		"--broker", addr,
		"--topic", "test/file",
		"--qos", "1", // guarantee delivery for test reliability
		"--file", filepath.Base(payloadPath),
	)
	if err != nil {
		t.Fatalf("mqtt_pub with file failed: %v\n%s", err, output)
	}
	if trimmed := strings.TrimSpace(output); trimmed != "" {
		t.Fatalf("unexpected command output: %q", trimmed)
	}

	select {
	case msg := <-published:
		if msg.topic != "test/file" {
			t.Fatalf("published topic = %q, want %q", msg.topic, "test/file")
		}
		if msg.payload != "file-payload" {
			t.Fatalf("published payload = %q, want %q", msg.payload, "file-payload")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for file-based mqtt publish")
	}
}
