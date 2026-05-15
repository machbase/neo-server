package bridge

import (
	"errors"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

type mqttTokenStub struct {
	ok  bool
	err error
}

func (t mqttTokenStub) Wait() bool {
	return t.ok
}

func (t mqttTokenStub) WaitTimeout(time.Duration) bool {
	return t.ok
}

func (t mqttTokenStub) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (t mqttTokenStub) Error() error {
	return t.err
}

type mqttClientStub struct {
	connected         bool
	publishToken      paho.Token
	subscribeToken    paho.Token
	unsubscribeToken  paho.Token
	publishedTopic    string
	publishedPayload  any
	subscribedTopic   string
	unsubscribedTopic string
}

func (c *mqttClientStub) IsConnected() bool {
	return c.connected
}

func (c *mqttClientStub) IsConnectionOpen() bool {
	return c.connected
}

func (c *mqttClientStub) Connect() paho.Token {
	return mqttTokenStub{ok: c.connected}
}

func (c *mqttClientStub) Disconnect(uint) {
	c.connected = false
}

func (c *mqttClientStub) Publish(topic string, _ byte, _ bool, payload any) paho.Token {
	c.publishedTopic = topic
	c.publishedPayload = payload
	if c.publishToken != nil {
		return c.publishToken
	}
	return mqttTokenStub{ok: true}
}

func (c *mqttClientStub) Subscribe(topic string, _ byte, _ paho.MessageHandler) paho.Token {
	c.subscribedTopic = topic
	if c.subscribeToken != nil {
		return c.subscribeToken
	}
	return mqttTokenStub{ok: true}
}

func (c *mqttClientStub) SubscribeMultiple(map[string]byte, paho.MessageHandler) paho.Token {
	return mqttTokenStub{ok: true}
}

func (c *mqttClientStub) Unsubscribe(topics ...string) paho.Token {
	if len(topics) > 0 {
		c.unsubscribedTopic = topics[0]
	}
	if c.unsubscribeToken != nil {
		return c.unsubscribeToken
	}
	return mqttTokenStub{ok: true}
}

func (c *mqttClientStub) AddRoute(string, paho.MessageHandler) {}

func (c *mqttClientStub) OptionsReader() paho.ClientOptionsReader {
	return paho.NewOptionsReader(paho.NewClientOptions())
}

func TestMqttBridgeOptionsAndClientAccess(t *testing.T) {
	br := NewMqttBridge("mqtt_test", "id=client username=user password=pass keepalive=2s cleansession=false unknown=value")
	require.NoError(t, br.BeforeRegister())
	require.Equal(t, "client", br.clientId)
	require.Equal(t, "user", br.username)
	require.Equal(t, "pass", br.password)
	require.Equal(t, 2*time.Second, br.keepAlive)
	require.False(t, br.cleanSession)
	require.False(t, br.IsConnected())
	require.Zero(t, br.Stats())
	require.Equal(t, "bridge 'mqtt_test' (mqtt)", br.String())
	require.Equal(t, "mqtt_test", br.Name())

	called := false
	br.OnDisconnect(func(any) { called = true })
	require.True(t, called)
	br.OnConnect(nil)
	br.OnDisconnect(nil)

	client := &mqttClientStub{connected: true}
	br.setClient(client)
	br.alive.Store(true)
	require.Same(t, client, br.getClient())
	require.True(t, br.IsConnected())
}

func TestMqttBridgePublishSubscribeAndStats(t *testing.T) {
	br := NewMqttBridge("mqtt_test", "")
	client := &mqttClientStub{connected: true}
	br.setClient(client)
	br.alive.Store(true)

	ok, err := br.Publish("topic/a", "payload")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "topic/a", client.publishedTopic)
	require.Equal(t, []byte("payload"), client.publishedPayload)
	require.Equal(t, uint64(1), br.Stats().OutMsgs)
	require.Equal(t, uint64(len("payload")), br.Stats().OutBytes)

	ok, err = br.Publish("topic/a", []byte("raw"))
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, []byte("raw"), client.publishedPayload)

	_, err = br.Publish("topic/a", 1)
	require.EqualError(t, err, "mqtt bridge can not publish int")

	sub, err := br.Subscribe("topic/a", 1, func(string, []byte, int, bool, bool) {})
	require.NoError(t, err)
	require.NotNil(t, sub)
	require.Equal(t, "topic/a", client.subscribedTopic)
	require.NoError(t, sub.Unsubscribe())
	require.Equal(t, "topic/a", client.unsubscribedTopic)

	client.unsubscribeToken = mqttTokenStub{ok: false}
	require.EqualError(t, sub.Unsubscribe(), "mqtt unsubscribe timeout")

	client.subscribeToken = mqttTokenStub{ok: false}
	_, err = br.Subscribe("topic/b", 1, func(string, []byte, int, bool, bool) {})
	require.EqualError(t, err, "mqtt subscribe timeout")

	client.publishToken = mqttTokenStub{ok: true, err: errors.New("ignored")}
	ok, err = br.Publish("topic/a", "payload")
	require.NoError(t, err)
	require.True(t, ok)
}

func TestMqttBridgeUnavailablePaths(t *testing.T) {
	br := NewMqttBridge("mqtt_test", "")
	_, err := br.Subscribe("topic", 1, func(string, []byte, int, bool, bool) {})
	require.EqualError(t, err, "mqtt connection is unavailable")
	_, err = br.Publish("topic", "payload")
	require.EqualError(t, err, "mqtt connection is unavailable")
	require.EqualError(t, (&MqttSubscription{}).Unsubscribe(), "mqtt connection is unavailable")

	br.alive.Store(true)
	done := make(chan struct{})
	go func() {
		require.NoError(t, br.AfterUnregister())
		close(done)
	}()
	select {
	case <-br.stopSig:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for mqtt stop signal")
	}
	<-done
}

func TestNatsBridgeOptionsAndUnavailablePaths(t *testing.T) {
	br := NewNatsBridge("nats_test", "name=client no-randomize=true no-echo=true verbose=true pedantic=true allow-reconnect=false max-reconnect=3 reconnect-wait=2s timeout=3s drain-timeout=4s flusher-timeout=5s ping-interval=6s max-pings-out=7 user=user password=pass token=tok retry-on-failed-connect=true skip-host-lookup=true unknown=value")
	require.NoError(t, br.BeforeRegister())
	require.Equal(t, "client", br.natsOpts.Name)
	require.True(t, br.natsOpts.NoRandomize)
	require.True(t, br.natsOpts.NoEcho)
	require.True(t, br.natsOpts.Verbose)
	require.True(t, br.natsOpts.Pedantic)
	require.False(t, br.natsOpts.AllowReconnect)
	require.Equal(t, 3, br.natsOpts.MaxReconnect)
	require.Equal(t, 2*time.Second, br.natsOpts.ReconnectWait)
	require.Equal(t, 3*time.Second, br.natsOpts.Timeout)
	require.Equal(t, 4*time.Second, br.natsOpts.DrainTimeout)
	require.Equal(t, 5*time.Second, br.natsOpts.FlusherTimeout)
	require.Equal(t, 6*time.Second, br.natsOpts.PingInterval)
	require.Equal(t, 7, br.natsOpts.MaxPingsOut)
	require.Equal(t, "user", br.natsOpts.User)
	require.Equal(t, "pass", br.natsOpts.Password)
	require.Equal(t, "tok", br.natsOpts.Token)
	require.True(t, br.natsOpts.RetryOnFailedConnect)
	require.True(t, br.natsOpts.SkipHostLookup)
	require.Equal(t, "bridge 'nats_test' (nats)", br.String())
	require.Equal(t, "nats_test", br.Name())
	require.False(t, br.IsConnected())
	require.Zero(t, br.Stats())

	br.setConn(nil)
	require.Nil(t, br.getConn())
	_, err := br.Subscribe("topic", func(*nats.Msg) {})
	require.EqualError(t, err, "nats connection is unavailable")
	_, err = br.Publish("topic", "payload")
	require.EqualError(t, err, "nats connection is unavailable")
	require.EqualError(t, (&NatsSubscription{}).Unsubscribe(), "nats connection is unavailable")

	sub := &NatsSubscription{writeStats: &br.WriteStats}
	sub.AddAppended(2)
	sub.AddInserted(3)
	require.Equal(t, uint64(2), br.Appended)
	require.Equal(t, uint64(3), br.Inserted)
}

func TestNatsBridgeConnectionAccessorsAndStopSignal(t *testing.T) {
	br := NewNatsBridge("nats_test", "server=nats://127.0.0.1:1 timeout=1ms max-reconnect=0")
	require.NoError(t, br.BeforeRegister())
	require.Len(t, br.natsOpts.Servers, 1)
	require.False(t, br.IsConnected())
	connected, reason := br.TestConnection()
	require.False(t, connected)
	require.Equal(t, "not connected", reason)

	br.alive.Store(true)
	done := make(chan struct{})
	go func() {
		require.NoError(t, br.AfterUnregister())
		close(done)
	}()
	select {
	case <-br.stopSig:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for nats stop signal")
	}
	<-done

	sub := &NatsSubscription{}
	require.Nil(t, sub.getSubscription())
	sub.setSubscription(&nats.Subscription{})
	require.NotNil(t, sub.getSubscription())
	sub.setSubscription(nil)
	require.Nil(t, sub.getSubscription())
}

func TestNatsSubscribeOptions(t *testing.T) {
	sub := &NatsSubscription{}
	NatsPendingMessageLimit(10)(sub)
	NatsQueueGroup("workers")(sub)
	NatsStreamName("events")(sub)
	require.Equal(t, 10, sub.msgChanSize)
	require.Equal(t, "workers", sub.queueName)
	require.Equal(t, "events", sub.streamName)
}
