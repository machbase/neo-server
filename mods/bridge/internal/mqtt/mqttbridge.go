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
	fields := strings.Fields(c.path)
	for _, field := range fields {
		kv := strings.SplitN(field, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		switch key {
		case "broker":
			fallthrough
		case "host":
			fallthrough
		case "server":
			c.serverAddresses = append(c.serverAddresses, val)
		case "id":
			c.clientId = val
		case "k":
			fallthrough
		case "keepalive":
			if k, err := time.ParseDuration(val); err == nil {
				c.keepAlive = k
			}
		case "c":
		case "cleansession":
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
	cfg := paho.NewClientOptions()
	cfg.SetProtocolVersion(4)
	cfg.SetConnectRetry(false)
	cfg.SetAutoReconnect(false)
	cfg.SetCleanSession(c.cleanSession)
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
		c.alive = true
		go c.run()
	}

	return nil
}

func (c *bridge) AfterUnregister() error {
	c.alive = false
	c.stopSig <- true
	if c.client == nil && c.client.IsConnected() {
		c.client.Disconnect(100)
	}
	return nil
}

func (c *bridge) String() string {
	return fmt.Sprintf("bridge '%s' (mqtt)", c.name)
}

func (c *bridge) Name() string {
	return c.name
}

func (c *bridge) run() {
	var fallbackWait = 1 * time.Second

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for c.alive {
		select {
		case <-ticker.C:
			if c.client == nil || !c.client.IsConnected() {
				c.log.Tracef("connecting... %v", c.clientOpts.Servers)
				c.client = paho.NewClient(c.clientOpts)
				clientToken := c.client.Connect()
				if beforeTimedout := clientToken.WaitTimeout(c.connectTimeout); c.client.IsConnected() {
					c.log.Trace("connected.")
					go c.notifyConnectListeners()
					ticker.Reset(10 * time.Second)
					fallbackWait = 1 * time.Second
				} else {
					if beforeTimedout {
						c.log.Trace("connect rejected")
					} else {
						c.log.Trace("connect timed out")
					}
					c.log.Tracef("connecting fallback wait %s.", fallbackWait)
					go c.notifyDisconnectListeners()
					ticker.Reset(fallbackWait)
					fallbackWait *= 2
					if fallbackWait > c.reconnectMaxWait {
						fallbackWait = c.reconnectMaxWait
					}
				}
			}
		case <-c.stopSig:
			c.log.Tracef("stop")
			return
		}
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
	c.connectListeners = append(c.connectListeners, cb)
}

func (c *bridge) OnDisconnect(cb func(br any)) {
	c.disconnectListeners = append(c.disconnectListeners, cb)
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
