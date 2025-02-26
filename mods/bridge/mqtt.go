package bridge

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/machbase/neo-server/v8/mods/logging"
)

type MqttBridge struct {
	log  logging.Log
	name string
	path string

	client     paho.Client
	clientOpts *paho.ClientOptions
	alive      bool
	stopSig    chan bool

	connectListeners    []func(any)
	disconnectListeners []func(any)

	serverAddresses    []string
	keepAlive          time.Duration
	cleanSession       bool
	certPath           string
	keyPath            string
	caCertPath         string
	clientId           string
	username           string
	password           string
	reconnectMaxWait   time.Duration
	connectTimeout     time.Duration
	subscribeTimeout   time.Duration
	unsubscribeTimeout time.Duration
	publishTimeout     time.Duration

	inMsgs   uint64
	outMsgs  uint64
	inBytes  uint64
	outBytes uint64
	WriteStats
}

func NewMqttBridge(name string, path string) *MqttBridge {
	return &MqttBridge{
		log:     logging.GetLog("mqtt-bridge"),
		name:    name,
		path:    path,
		stopSig: make(chan bool),

		keepAlive:    30 * time.Second,
		cleanSession: true,

		reconnectMaxWait:   10 * time.Second,
		connectTimeout:     5 * time.Second,
		subscribeTimeout:   3 * time.Second,
		unsubscribeTimeout: 3 * time.Second,
		publishTimeout:     3 * time.Second,
	}
}

func (c *MqttBridge) BeforeRegister() error {
	cfg := paho.NewClientOptions()
	cfg.SetCleanSession(true)
	cfg.SetProtocolVersion(4)
	cfg.SetConnectRetry(false)
	cfg.SetAutoReconnect(false)
	cfg.SetKeepAlive(30 * time.Second)

	fields := strings.Fields(c.path)
	for _, field := range fields {
		kv := strings.SplitN(field, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		switch key {
		case "broker", "host", "server":
			c.serverAddresses = append(c.serverAddresses, val)
		case "id":
			c.clientId = val
		case "username":
			c.username = val
		case "password":
			c.password = val
		case "keepalive", "k":
			if k, err := time.ParseDuration(val); err == nil {
				c.keepAlive = k
			}
		case "cleansession", "c":
			if flag, err := strconv.ParseBool(val); err == nil {
				c.cleanSession = flag
			}
		case "cafile":
			c.caCertPath = val
		case "key":
			c.keyPath = val
		case "cert":
			c.certPath = val
		default:
			c.log.Infof("unknown option, %s=%s", key, val)
		}
	}
	cfg.SetCleanSession(c.cleanSession)
	if len(c.username) > 0 {
		cfg.SetUsername(c.username)
	}
	if len(c.password) > 0 {
		cfg.SetPassword(c.password)
	}
	if c.keepAlive >= 1*time.Second {
		cfg.SetKeepAlive(c.keepAlive)
	}
	for _, addr := range c.serverAddresses {
		cfg.AddBroker(addr)
	}
	if len(c.clientId) > 0 {
		cfg.SetClientID(c.clientId)
	}
	if len(c.keyPath) > 0 && len(c.certPath) > 0 && len(c.caCertPath) > 0 {
		// tls
		rootCAs := x509.NewCertPool()
		ca, err := os.ReadFile(c.caCertPath)
		if err != nil {
			return err
		}
		rootCAs.AppendCertsFromPEM(ca)

		tlsCert, err := tls.LoadX509KeyPair(c.certPath, c.keyPath)
		if err != nil {
			return err
		}

		tlsCfg := &tls.Config{
			InsecureSkipVerify: true,
			RootCAs:            rootCAs,
			ClientAuth:         tls.NoClientCert,
			ClientCAs:          nil,
			Certificates:       []tls.Certificate{tlsCert},
		}
		cfg.SetTLSConfig(tlsCfg)
	}

	c.clientOpts = cfg
	if len(c.serverAddresses) > 0 {
		go c.run()
	} else {
		c.log.Warnf("bridge '%s' no broker address", c.name)
	}

	return nil
}

func (c *MqttBridge) AfterUnregister() error {
	if c.alive {
		c.stopSig <- true
	}
	return nil
}

func (c *MqttBridge) String() string {
	return fmt.Sprintf("bridge '%s' (mqtt)", c.name)
}

func (c *MqttBridge) Name() string {
	return c.name
}

type MqttStats struct {
	InMsgs   uint64
	OutMsgs  uint64
	InBytes  uint64
	OutBytes uint64
	Appended uint64
	Inserted uint64
}

func (c *MqttBridge) Stats() MqttStats {
	ret := MqttStats{}
	if c.client == nil {
		return ret
	}
	ret.InBytes = atomic.LoadUint64(&c.inBytes)
	ret.InMsgs = atomic.LoadUint64(&c.inMsgs)
	ret.OutBytes = atomic.LoadUint64(&c.outBytes)
	ret.OutMsgs = atomic.LoadUint64(&c.outMsgs)
	ret.Appended = atomic.LoadUint64(&c.Appended)
	ret.Inserted = atomic.LoadUint64(&c.Inserted)
	return ret
}

func (c *MqttBridge) IsConnected() bool {
	if !c.alive || c.client == nil || !c.client.IsConnected() {
		return false
	}
	return true
}

func (c *MqttBridge) run() {
	var fallbackWait = 1 * time.Second
	ticker := time.NewTicker(1 * time.Second)
	c.alive = true
	for c.alive {
		select {
		case <-ticker.C:
			if c.client == nil || !c.client.IsConnected() {
				c.log.Tracef("bridge [%s] connecting... %v", c.name, c.clientOpts.Servers)
				c.client = paho.NewClient(c.clientOpts)
				clientToken := c.client.Connect()
				if beforeTimedout := clientToken.WaitTimeout(c.connectTimeout); c.client.IsConnected() {
					c.log.Tracef("bridge [%s] connected.", c.name)
					go c.notifyConnectListeners()
					ticker.Reset(10 * time.Second)
					fallbackWait = 1 * time.Second
				} else {
					if beforeTimedout {
						c.log.Tracef("bridge [%s] connect rejected", c.name)
					} else {
						c.log.Tracef("bridge [%s] connect timed out", c.name)
					}
					c.log.Tracef("bridge [%s] connecting fallback wait %s.", c.name, fallbackWait)
					go c.notifyDisconnectListeners()
					ticker.Reset(fallbackWait)
					fallbackWait *= 2
					if fallbackWait > c.reconnectMaxWait {
						fallbackWait = c.reconnectMaxWait
					}
				}
			}
		case <-c.stopSig:
			c.alive = false
		}
	}
	ticker.Stop()
	if c.client != nil && c.client.IsConnected() {
		c.client.Disconnect(300)
	}
}

func (c *MqttBridge) notifyConnectListeners() {
	for _, cb := range c.connectListeners {
		cb(c)
	}
}

func (c *MqttBridge) notifyDisconnectListeners() {
	for _, cb := range c.disconnectListeners {
		cb(c)
	}
}

func (c *MqttBridge) OnConnect(cb func(br any)) {
	if cb == nil {
		return
	}
	c.connectListeners = append(c.connectListeners, cb)
	if c.IsConnected() {
		cb(c)
	}
}

func (c *MqttBridge) OnDisconnect(cb func(br any)) {
	if cb == nil {
		return
	}
	c.disconnectListeners = append(c.disconnectListeners, cb)
	if !c.IsConnected() {
		cb(c)
	}
}

type MqttSubscription struct {
	bridge     *MqttBridge
	topic      string
	writeStats *WriteStats
}

func (ns *MqttSubscription) Unsubscribe() error {
	if ns.bridge == nil || ns.bridge.client == nil || !ns.bridge.client.IsConnected() {
		return fmt.Errorf("mqtt connection is unavailable")
	}
	token := ns.bridge.client.Unsubscribe(ns.topic)
	success := token.WaitTimeout(ns.bridge.unsubscribeTimeout)
	if !success {
		return fmt.Errorf("mqtt unsubscribe timeout")
	}
	return nil
}

func (ns *MqttSubscription) AddAppended(delta uint64) {
	atomic.AddUint64(&ns.writeStats.Appended, delta)
}

func (ns *MqttSubscription) AddInserted(delta uint64) {
	atomic.AddUint64(&ns.writeStats.Inserted, delta)
}

func (c *MqttBridge) Subscribe(topic string, qos byte, cb func(topic string, payload []byte, msgId int, dup bool, retained bool)) (*MqttSubscription, error) {
	if c.client == nil || !c.client.IsConnected() {
		return nil, fmt.Errorf("mqtt connection is unavailable")
	}
	token := c.client.Subscribe(topic, qos, func(_ paho.Client, msg paho.Message) {
		atomic.AddUint64(&c.inMsgs, 1)
		atomic.AddUint64(&c.inBytes, uint64(len(msg.Payload())))
		cb(msg.Topic(), msg.Payload(), int(msg.MessageID()), msg.Duplicate(), msg.Retained())
	})
	success := token.WaitTimeout(c.subscribeTimeout)
	if !success {
		return nil, fmt.Errorf("mqtt subscribe timeout")
	}
	ret := &MqttSubscription{
		bridge:     c,
		topic:      topic,
		writeStats: &c.WriteStats,
	}
	return ret, nil
}

func (c *MqttBridge) Publish(topic string, payload any) (bool, error) {
	if c.client == nil || !c.client.IsConnected() {
		return false, fmt.Errorf("mqtt connection is unavailable")
	}
	var data []byte
	switch raw := payload.(type) {
	case []byte:
		data = raw
	case string:
		data = []byte(raw)
	default:
		return false, fmt.Errorf("mqtt bridge can not publish %T", raw)
	}
	atomic.AddUint64(&c.outMsgs, 1)
	atomic.AddUint64(&c.outBytes, uint64(len(data)))
	var qos byte = 1
	token := c.client.Publish(topic, qos, false, data)
	success := token.WaitTimeout(c.publishTimeout)
	return success, nil
}

func (c *MqttBridge) TestConnection() (bool, string) {
	connected := c.IsConnected()
	if !connected {
		return false, "not connected"
	}

	return true, "success"
}
