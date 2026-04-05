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

func Module(ctx context.Context, rt *goja.Runtime, module *goja.Object) {
	// Export native functions
	m := module.Get("exports").(*goja.Object)
	m.Set("parseConfig", ParseConfig)
	m.Set("NewClient", func(obj *goja.Object, dispatch engine.EventDispatchFunc) (*Client, error) {
		return NewClient(ctx, obj, dispatch)
	})
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

func NewClient(ctx context.Context, obj *goja.Object, dispatch engine.EventDispatchFunc) (*Client, error) {
	ret := &Client{}
	ret.ctx, ret.cancel = context.WithCancel(ctx)
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

type eventMessage struct {
	topic   string
	payload []byte
	props   *paho.PublishProperties
}

func (msg eventMessage) EventValue(vm *goja.Runtime) goja.Value {
	obj := vm.NewObject()
	obj.Set("topic", msg.topic)
	obj.Set("payload", vm.NewArrayBuffer(msg.payload))
	if msg.props != nil {
		obj.Set("properties", exportPublishProperties(vm, msg.props))
	}
	return obj
}

func exportUserProperties(vm *goja.Runtime, props paho.UserProperties) goja.Value {
	valuesByKey := map[string][]string{}
	order := []string{}
	for _, prop := range props {
		if _, exists := valuesByKey[prop.Key]; !exists {
			order = append(order, prop.Key)
		}
		valuesByKey[prop.Key] = append(valuesByKey[prop.Key], prop.Value)
	}
	obj := vm.NewObject()
	for _, key := range order {
		values := valuesByKey[key]
		if len(values) == 1 {
			obj.Set(key, values[0])
		} else {
			obj.Set(key, values)
		}
	}
	return obj
}

func exportPublishProperties(vm *goja.Runtime, props *paho.PublishProperties) goja.Value {
	obj := vm.NewObject()
	if props.PayloadFormat != nil {
		obj.Set("payloadFormat", int(*props.PayloadFormat))
	}
	if props.MessageExpiry != nil {
		obj.Set("messageExpiry", int(*props.MessageExpiry))
	}
	if props.ContentType != "" {
		obj.Set("contentType", props.ContentType)
	}
	if props.ResponseTopic != "" {
		obj.Set("responseTopic", props.ResponseTopic)
	}
	if len(props.CorrelationData) > 0 {
		obj.Set("correlationData", vm.NewArrayBuffer(append([]byte(nil), props.CorrelationData...)))
	}
	if props.TopicAlias != nil {
		obj.Set("topicAlias", int(*props.TopicAlias))
	}
	if props.SubscriptionIdentifier != nil {
		obj.Set("subscriptionIdentifier", *props.SubscriptionIdentifier)
	}
	if len(props.User) > 0 {
		obj.Set("user", exportUserProperties(vm, props.User))
	}
	return obj
}

func (c *Client) ensureConnected() error {
	if c.closed {
		return fmt.Errorf("mqtt client is closed")
	}
	if c.conn == nil {
		return fmt.Errorf("mqtt client is not connected")
	}
	return nil
}

func applyUserProperties(userProps map[string]any, user *paho.UserProperties) {
	for k, v := range userProps {
		user.Add(k, fmt.Sprintf("%v", v))
	}
}

func buildSubscribeOptions(topic string, options map[string]any) paho.SubscribeOptions {
	sub := paho.SubscribeOptions{Topic: topic, QoS: 1}
	if options == nil {
		return sub
	}
	if qos, ok := asByte(options["qos"]); ok {
		sub.QoS = qos
	}
	if retainHandling, ok := asByte(options["retainHandling"]); ok {
		sub.RetainHandling = retainHandling
	}
	if noLocal, ok := options["noLocal"].(bool); ok {
		sub.NoLocal = noLocal
	}
	if retainAsPublished, ok := options["retainAsPublished"].(bool); ok {
		sub.RetainAsPublished = retainAsPublished
	}
	return sub
}

func buildSubscribeProperties(options map[string]any) *paho.SubscribeProperties {
	if options == nil {
		return nil
	}
	props, ok := options["properties"].(map[string]any)
	if !ok {
		return nil
	}
	ret := &paho.SubscribeProperties{}
	if si, ok := asInt(props["subscriptionIdentifier"]); ok {
		ret.SubscriptionIdentifier = &si
	}
	if userProps, ok := props["user"].(map[string]any); ok {
		applyUserProperties(userProps, &ret.User)
	}
	if ret.SubscriptionIdentifier == nil && len(ret.User) == 0 {
		return nil
	}
	return ret
}

func buildUnsubscribeProperties(options map[string]any) *paho.UnsubscribeProperties {
	if options == nil {
		return nil
	}
	props, ok := options["properties"].(map[string]any)
	if !ok {
		return nil
	}
	ret := &paho.UnsubscribeProperties{}
	if userProps, ok := props["user"].(map[string]any); ok {
		applyUserProperties(userProps, &ret.User)
	}
	if len(ret.User) == 0 {
		return nil
	}
	return ret
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
func (c *Client) Subscribe(topic string, options map[string]any) (any, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}
	subAck, err := c.conn.Subscribe(c.ctx, &paho.Subscribe{
		Properties: buildSubscribeProperties(options),
		Subscriptions: []paho.SubscribeOptions{
			buildSubscribeOptions(topic, options),
		},
	})
	if err != nil {
		return nil, err
	}

	c.conn.AddOnPublishReceived(func(pr autopaho.PublishReceived) (bool, error) {
		c.emit("message", eventMessage{
			topic:   pr.Packet.Topic,
			payload: append([]byte(nil), pr.Packet.Payload...),
			props:   pr.Packet.Properties,
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

// Unsubscribe from a topic
// Returns a list of reason codes for the unsubscription

func (c *Client) Unsubscribe(topic string, options map[string]any) (any, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}
	unsubAck, err := c.conn.Unsubscribe(c.ctx, &paho.Unsubscribe{
		Topics:     []string{topic},
		Properties: buildUnsubscribeProperties(options),
	})
	if err != nil {
		return nil, err
	}

	reasons := make([]int, len(unsubAck.Reasons))
	for i, r := range unsubAck.Reasons {
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
	if err := c.ensureConnected(); err != nil {
		return 0, err
	}
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
				applyUserProperties(userProps, &pub.Properties.User)
			}
		}
	}
	pubAck, err := c.conn.Publish(c.ctx, pub)
	return int(pubAck.ReasonCode), err
}
