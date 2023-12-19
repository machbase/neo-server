package mqtt

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/machbase/neo-server/mods/logging"
)

type bridge struct {
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
}

func New(name string, path string) *bridge {
	return &bridge{
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

func (c *bridge) BeforeRegister() error {
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
	}

	return nil
}

func (c *bridge) AfterUnregister() error {
	c.stopSig <- true
	return nil
}

func (c *bridge) String() string {
	return fmt.Sprintf("bridge '%s' (mqtt)", c.name)
}

func (c *bridge) Name() string {
	return c.name
}

func (c *bridge) IsConnected() bool {
	if !c.alive || c.client == nil || !c.client.IsConnected() {
		return false
	}
	return true
}

func (c *bridge) run() {
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
			c.log.Tracef("bridge [%s] stop", c.name)
			c.alive = false
		}
	}
	ticker.Stop()
	if c.client != nil && c.client.IsConnected() {
		c.client.Disconnect(300)
	}
}

func (c *bridge) notifyConnectListeners() {
	for _, cb := range c.connectListeners {
		cb(c)
	}
}

func (c *bridge) notifyDisconnectListeners() {
	for _, cb := range c.disconnectListeners {
		cb(c)
	}
}

func (c *bridge) OnConnect(cb func(br any)) {
	if cb == nil {
		return
	}
	c.connectListeners = append(c.connectListeners, cb)
	if c.IsConnected() {
		cb(c)
	}
}

func (c *bridge) OnDisconnect(cb func(br any)) {
	if cb == nil {
		return
	}
	c.disconnectListeners = append(c.disconnectListeners, cb)
	if !c.IsConnected() {
		cb(c)
	}
}

func (c *bridge) Subscribe(topic string, qos byte, cb func(topic string, payload []byte, msgId int, dup bool, retained bool)) (bool, error) {
	if c.client == nil || !c.client.IsConnected() {
		return false, fmt.Errorf("mqtt connection is unavailable")
	}
	token := c.client.Subscribe(topic, qos, func(_ paho.Client, msg paho.Message) {
		cb(msg.Topic(), msg.Payload(), int(msg.MessageID()), msg.Duplicate(), msg.Retained())
	})
	success := token.WaitTimeout(c.subscribeTimeout)
	return success, nil
}

func (c *bridge) Unsubscribe(topics ...string) (bool, error) {
	if c.client == nil || !c.client.IsConnected() {
		return false, fmt.Errorf("mqtt connection is unavailable")
	}
	token := c.client.Unsubscribe(topics...)
	success := token.WaitTimeout(c.unsubscribeTimeout)
	return success, nil
}

func (c *bridge) Publish(topic string, payload any) (bool, error) {
	if c.client == nil || !c.client.IsConnected() {
		return false, fmt.Errorf("mqtt connection is unavailable")
	}
	var qos byte = 1
	token := c.client.Publish(topic, qos, false, payload)
	success := token.WaitTimeout(c.publishTimeout)
	return success, nil
}
