package mqtt

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"
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
	runWait    sync.WaitGroup

	serverAddresses    []string
	keepAlive          time.Duration
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
		name: name,
		path: path,

		reconnectMaxWait:   10 * time.Second,
		connectTimeout:     5 * time.Second,
		subscribeTimeout:   3 * time.Second,
		unsubscribeTimeout: 3 * time.Second,
		publishTimeout:     3 * time.Second,
	}
}

func (c *bridge) BeforeRegister() error {
	cfg := paho.NewClientOptions()
	cfg.SetKeepAlive(c.keepAlive)
	cfg.SetCleanSession(true)
	cfg.SetConnectRetry(false)
	cfg.SetAutoReconnect(false)
	cfg.SetProtocolVersion(4)
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
	c.alive = true
	go c.run()

	return nil
}

func (c *bridge) AfterUnregister() error {
	c.alive = false
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
	for c.alive {
		c.log.Tracef("reconnecting... %v", c.clientOpts.Servers)
		c.client = paho.NewClient(c.clientOpts)
		clientToken := c.client.Connect()
		if clientToken.WaitTimeout(c.connectTimeout); c.client.IsConnected() {
			fallbackWait = 1 * time.Second
		} else {
			c.log.Tracef("reconnecting fallback wait %s.", fallbackWait)
			time.Sleep(fallbackWait)
			fallbackWait *= 2
			if fallbackWait > c.reconnectMaxWait {
				fallbackWait = c.reconnectMaxWait
			}
			continue
		}

		c.log.Trace("connected.")

		c.runWait.Add(1)
		c.sentinel()
		c.runWait.Wait()
		if c.alive {
			c.log.Tracef("reconnecting...")
		}
	}
}

func (c *bridge) sentinel() {
	go func() {
		for c.alive {
			time.Sleep(10 * time.Second)
			if !c.client.IsConnected() {
				c.runWait.Done()
			}
		}
	}()
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

func (c *bridge) Publish(topic string, qos byte, payload []byte) (bool, error) {
	if c.client == nil || !c.client.IsConnected() {
		return false, fmt.Errorf("mqtt connection is unavailable")
	}
	token := c.client.Publish(topic, qos, false, payload)
	success := token.WaitTimeout(c.publishTimeout)
	return success, nil
}
