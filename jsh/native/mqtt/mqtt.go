package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/dop251/goja"
	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/machbase/neo-server/v8/jsh/engine"
)

func Module(rt *goja.Runtime, module *goja.Object) {
	// Export native functions
	m := module.Get("exports").(*goja.Object)
	m.Set("parseConfig", ParseConfig)
	m.Set("NewClient", NewClient)
}

type Config struct {
	Servers                       []string      `json:"servers"`
	Username                      string        `json:"username"`
	Password                      string        `json:"password"`
	KeepAlive                     uint16        `json:"keepAlive"`
	ConnectRetryDelay             time.Duration `json:"connectRetryDelay"`
	CleanStartOnInitialConnection bool          `json:"cleanStartOnInitialConnection"`
	ConnectTimeout                time.Duration `json:"connectTimeout"`
}

func ParseConfig(data string) (*autopaho.ClientConfig, error) {
	conf := Config{}
	json.Unmarshal([]byte(data), &conf)

	ret := &autopaho.ClientConfig{}
	for _, s := range conf.Servers {
		u, err := url.Parse(s)
		if err != nil {
			return nil, err
		}
		ret.ServerUrls = append(ret.ServerUrls, u)
	}
	ret.ConnectUsername = conf.Username
	ret.ConnectPassword = []byte(conf.Password)
	ret.KeepAlive = conf.KeepAlive
	ret.ReconnectBackoff = func(i int) time.Duration {
		if i == 0 {
			return 0
		}
		return conf.ConnectRetryDelay * time.Millisecond
	}
	ret.CleanStartOnInitialConnection = conf.CleanStartOnInitialConnection
	ret.ConnectTimeout = conf.ConnectTimeout * time.Millisecond
	return ret, nil
}

func NewClient(obj *goja.Object, dispatch engine.EventDispatchFunc) (*Client, error) {
	ret := &Client{}
	ret.ctx, ret.cancel = context.WithCancel(context.Background())
	ret.emit = func(event string, data any) {
		dispatch(obj, event, data)
	}
	return ret, nil
}

type Client struct {
	ctx    context.Context
	cancel context.CancelFunc
	conn   *autopaho.ConnectionManager
	emit   func(event string, data any)
	closed bool
}

func (c *Client) Connect(config autopaho.ClientConfig) error {
	// Establish the connection, it will retry until successful or context is canceled
	if conn, err := autopaho.NewConnection(c.ctx, config); err != nil {
		return fmt.Errorf("failed to create connection manager: %w", err)
	} else {
		c.conn = conn
	}
	return c.conn.AwaitConnection(c.ctx)
}

func (c *Client) IsClosed() bool {
	return c.closed
}

func (c *Client) Disconnect() {
	c.closed = true
	c.cancel()
	if c.conn != nil {
		c.conn.Disconnect(c.ctx)
	}
}

// Subscribe to a topic
// Returns a list of reason codes for the subscription
func (c *Client) Subscribe(topic string) (any, error) {
	subAck, err := c.conn.Subscribe(c.ctx, &paho.Subscribe{
		Subscriptions: []paho.SubscribeOptions{
			{Topic: topic, QoS: 1},
		},
	})

	c.conn.AddOnPublishReceived(func(pr autopaho.PublishReceived) (bool, error) {
		c.emit("message", map[string]any{
			"topic":   pr.Packet.Topic,
			"payload": string(pr.Packet.Payload),
		})
		return true, nil
	})

	reasons := make([]int, len(subAck.Reasons))
	for i, r := range subAck.Reasons {
		reasons[i] = int(r)
	}
	if len(reasons) == 1 {
		return reasons[0], err
	} else {
		return reasons, err
	}
}

// Publish a message to a topic
// Returns the reason code for the publish
func (c *Client) Publish(topic string, data any) (int, error) {
	var payload []byte
	switch val := data.(type) {
	case string:
		payload = []byte(val)
	case []byte:
		payload = val
	default:
		return 0, fmt.Errorf("invalid payload type: %T", data)
	}
	pubAck, err := c.conn.Publish(c.ctx, &paho.Publish{
		Topic:   topic,
		Payload: payload,
		QoS:     0,
	})
	return int(pubAck.ReasonCode), err
}
