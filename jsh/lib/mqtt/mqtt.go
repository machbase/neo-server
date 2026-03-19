package mqtt

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/dop251/goja"
	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/machbase/neo-server/v8/jsh/engine"
)

func asByte(value any) (byte, bool) {
	switch number := value.(type) {
	case float64:
		return byte(number), true
	case float32:
		return byte(number), true
	case int:
		return byte(number), true
	case int8:
		return byte(number), true
	case int16:
		return byte(number), true
	case int32:
		return byte(number), true
	case int64:
		return byte(number), true
	case uint:
		return byte(number), true
	case uint8:
		return byte(number), true
	case uint16:
		return byte(number), true
	case uint32:
		return byte(number), true
	case uint64:
		return byte(number), true
	default:
		return 0, false
	}
}

func asUint32(value any) (uint32, bool) {
	switch number := value.(type) {
	case float64:
		return uint32(number), true
	case float32:
		return uint32(number), true
	case int:
		return uint32(number), true
	case int8:
		return uint32(number), true
	case int16:
		return uint32(number), true
	case int32:
		return uint32(number), true
	case int64:
		return uint32(number), true
	case uint:
		return uint32(number), true
	case uint8:
		return uint32(number), true
	case uint16:
		return uint32(number), true
	case uint32:
		return number, true
	case uint64:
		return uint32(number), true
	default:
		return 0, false
	}
}

func asUint16(value any) (uint16, bool) {
	switch number := value.(type) {
	case float64:
		return uint16(number), true
	case float32:
		return uint16(number), true
	case int:
		return uint16(number), true
	case int8:
		return uint16(number), true
	case int16:
		return uint16(number), true
	case int32:
		return uint16(number), true
	case int64:
		return uint16(number), true
	case uint:
		return uint16(number), true
	case uint8:
		return uint16(number), true
	case uint16:
		return number, true
	case uint32:
		return uint16(number), true
	case uint64:
		return uint16(number), true
	default:
		return 0, false
	}
}

func asInt(value any) (int, bool) {
	switch number := value.(type) {
	case float64:
		return int(number), true
	case float32:
		return int(number), true
	case int:
		return number, true
	case int8:
		return int(number), true
	case int16:
		return int(number), true
	case int32:
		return int(number), true
	case int64:
		return int(number), true
	case uint:
		return int(number), true
	case uint8:
		return int(number), true
	case uint16:
		return int(number), true
	case uint32:
		return int(number), true
	case uint64:
		return int(number), true
	default:
		return 0, false
	}
}

//go:embed mqtt.js
var mqtt_js []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"mqtt.js": mqtt_js,
	}
}

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
	if ret.KeepAlive == 0 {
		ret.KeepAlive = 30
	}
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
func (c *Client) Publish(topic string, data any, options map[string]any) (int, error) {
	var payload []byte
	switch val := data.(type) {
	case string:
		payload = []byte(val)
	case []byte:
		payload = val
	case map[string]any, []any: // If the data is an object or array from JS. Marshal it to JSON
		if b, err := json.Marshal(val); err != nil {
			return 0, fmt.Errorf("failed to marshal payload: %w", err)
		} else {
			payload = b
		}
	default:
		return 0, fmt.Errorf("invalid payload type: %T", data)
	}
	pub := &paho.Publish{Topic: topic, Payload: payload, QoS: 0}
	if options != nil {
		if qos, ok := asByte(options["qos"]); ok {
			pub.QoS = qos
		}
		if retain, ok := options["retain"].(bool); ok {
			pub.Retain = retain
		}
		if props, ok := options["properties"].(map[string]any); ok {
			pub.Properties = &paho.PublishProperties{}
			if pf, ok := asByte(props["payloadFormat"]); ok {
				pub.Properties.PayloadFormat = &pf
			}
			if me, ok := asUint32(props["messageExpiry"]); ok {
				pub.Properties.MessageExpiry = &me
			}
			if ct, ok := props["contentType"].(string); ok {
				pub.Properties.ContentType = ct
			}
			if rt, ok := props["responseTopic"].(string); ok {
				pub.Properties.ResponseTopic = rt
			}
			if cd, ok := props["correlationData"].(string); ok {
				pub.Properties.CorrelationData = []byte(cd)
			}
			if ta, ok := asUint16(props["topicAlias"]); ok {
				pub.Properties.TopicAlias = &ta
			}
			if si, ok := asInt(props["subscriptionIdentifier"]); ok {
				pub.Properties.SubscriptionIdentifier = &si
			}
			if userProps, ok := props["user"].(map[string]any); ok {
				for k, v := range userProps {
					pub.Properties.User.Add(k, fmt.Sprintf("%v", v))
				}
			}
		}
	}
	pubAck, err := c.conn.Publish(c.ctx, pub)
	return int(pubAck.ReasonCode), err
}
