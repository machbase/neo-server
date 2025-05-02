package mqtt

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	js "github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
)

func NewModuleLoader(ctx context.Context) require.ModuleLoader {
	return func(rt *js.Runtime, module *js.Object) {
		// m = require("@jsh/mqtt")
		o := module.Get("exports").(*js.Object)
		// c = new mqtt.Client()
		o.Set("Client", new_client(ctx, rt))
	}
}

func new_client(ctx context.Context, rt *js.Runtime) func(call js.ConstructorCall) *js.Object {
	return func(call js.ConstructorCall) *js.Object {
		if len(call.Arguments) < 1 {
			panic(rt.ToValue("missing arguments"))
		}
		opts := struct {
			ServerUrls     []string    `json:"serverUrls"`
			KeepAlive      uint16      `json:"keepAlive,omitempty"`
			CleanStart     bool        `json:"cleanStart,omitempty"`
			Username       string      `json:"username,omitempty"`
			Password       string      `json:"password,omitempty"`
			ClientID       string      `json:"clientID,omitempty"`
			OnConnect      js.Callable `json:"onConnect,omitempty"`
			OnConnectError js.Callable `json:"onConnectError,omitempty"`
			OnDisconnect   js.Callable `json:"onDisconnect,omitempty"`
			OnClientError  js.Callable `json:"onClientError,omitempty"`
			OnMessage      js.Callable `json:"onMessage,omitempty"`
		}{}
		if err := rt.ExportTo(call.Arguments[0], &opts); err != nil {
			panic(rt.ToValue(err.Error()))
		}

		serverUrls := make([]*url.URL, len(opts.ServerUrls))
		for i, addr := range opts.ServerUrls {
			if u, err := url.Parse(addr); err != nil {
				panic(rt.ToValue(err.Error()))
			} else {
				serverUrls[i] = u
			}
		}

		ret := rt.NewObject()

		client := &Client{
			ctx: ctx,
			rt:  rt,
			obj: ret,
			config: autopaho.ClientConfig{
				ConnectUsername:               opts.Username,
				ConnectPassword:               []byte(opts.Password),
				ServerUrls:                    serverUrls,
				KeepAlive:                     opts.KeepAlive,
				CleanStartOnInitialConnection: opts.CleanStart,
				ConnectRetryDelay:             1 * time.Second, // default 10s
				ConnectTimeout:                5 * time.Second, // default 10s
			},
			OnConnect:      opts.OnConnect,
			OnConnectError: opts.OnConnectError,
			OnDisconnect:   opts.OnDisconnect,
			OnClientError:  opts.OnClientError,
			OnMessage:      opts.OnMessage,
		}
		if opts.OnConnectError != nil {
			client.config.OnConnectError = client.handleConnectError
		}
		if opts.OnConnect != nil {
			client.config.OnConnectionUp = client.handleConnectionUp
		}
		if opts.OnDisconnect != nil {
			client.config.OnServerDisconnect = client.handleServerDisconnect
		}
		if opts.OnClientError != nil {
			client.config.ClientConfig.OnClientError = client.handleClientError
		}
		if opts.OnMessage != nil {
			client.config.ClientConfig.OnPublishReceived =
				[]func(paho.PublishReceived) (bool, error){
					client.handlePublishReceived,
				}
		}
		client.config.ClientConfig.ClientID = opts.ClientID

		// c.connect()
		ret.Set("connect", client.Connect)
		// c.disconnect()
		ret.Set("disconnect", client.Disconnect)
		// c.subscribe(subs)
		ret.Set("subscribe", client.Subscribe)
		// c.publish(topic, payload, qos)
		// c.publish(topic, payload)
		ret.Set("publish", client.Publish)
		// c.awaitConnection()
		ret.Set("awaitConnection", client.AwaitConnection)
		// c.subscribe(subs)
		return ret
	}
}

type Client struct {
	ctx     context.Context
	rt      *js.Runtime
	config  autopaho.ClientConfig
	connMgr *autopaho.ConnectionManager
	obj     *js.Object

	OnConnect      js.Callable
	OnConnectError js.Callable
	OnDisconnect   js.Callable
	OnClientError  js.Callable
	OnMessage      js.Callable
}

func (c *Client) Connect(call js.FunctionCall) js.Value {
	if c.connMgr != nil {
		panic(c.rt.ToValue("already connected"))
	}
	if cm, err := autopaho.NewConnection(c.ctx, c.config); err != nil {
		panic(c.rt.ToValue(err.Error()))
	} else {
		c.connMgr = cm
		if cleaner, ok := c.ctx.(Cleaner); ok {
			cleaner.AddCleanup(func(out io.Writer) {
				if c.connMgr != nil {
					io.WriteString(out, "forced a mqtt connection to close by cleanup\n")
					c.connMgr.Disconnect(c.ctx)
				}
			})
		}
	}
	return js.Undefined()
}

type Cleaner interface {
	AddCleanup(func(io.Writer)) int64
	RemoveCleanup(int64)
}

func (c *Client) Disconnect(call js.FunctionCall) js.Value {
	if c.connMgr != nil {
		c.connMgr.Disconnect(c.ctx)
		c.connMgr = nil
	}
	return js.Undefined()
}

func (c *Client) AwaitConnection(call js.FunctionCall) js.Value {
	if c.connMgr == nil {
		panic(c.rt.ToValue("not connected"))
	}
	timeout := 0
	if len(call.Arguments) > 0 {
		if err := c.rt.ExportTo(call.Arguments[0], &timeout); err != nil {
			panic(c.rt.ToValue(err.Error()))
		}
		if timeout < 0 {
			panic(c.rt.ToValue("invalid timeout"))
		}
	}
	var ctx context.Context
	if timeout == 0 {
		ctx = c.ctx
	} else {
		c, cancel := context.WithTimeout(c.ctx, time.Duration(timeout)*time.Millisecond)
		ctx = c
		defer cancel()
	}
	if err := c.connMgr.AwaitConnection(ctx); err != nil {
		panic(c.rt.ToValue(err.Error()))
	}
	return js.Undefined()
}

func (c *Client) Publish(call js.FunctionCall) js.Value {
	if c.connMgr == nil {
		panic(c.rt.ToValue("not connected"))
	}
	if len(call.Arguments) < 2 {
		panic(c.rt.ToValue("missing arguments"))
	}
	topic := call.Arguments[0].String()
	payload := []byte{}
	if err := c.rt.ExportTo(call.Arguments[1], &payload); err != nil {
		panic(c.rt.ToValue(err.Error()))
	}
	qos := 0
	if len(call.Arguments) > 2 {
		if err := c.rt.ExportTo(call.Arguments[2], &qos); err != nil {
			panic(c.rt.ToValue(err.Error()))
		}
		if qos < 0 || qos > 2 {
			panic(c.rt.ToValue("invalid qos"))
		}
	}

	pubReq := &paho.Publish{
		Topic:   topic,
		QoS:     byte(qos),
		Payload: payload,
	}
	pubRsp, err := c.connMgr.Publish(c.ctx, pubReq)
	if err != nil {
		panic(c.rt.ToValue(err.Error()))
	}
	if pubRsp != nil {
		_ = pubRsp.Properties
		_ = pubRsp.ReasonCode
	}
	return js.Undefined()
}

func (c *Client) Subscribe(call js.FunctionCall) js.Value {
	if c.connMgr == nil {
		panic(c.rt.ToValue("not connected"))
	}
	if len(call.Arguments) < 1 {
		panic(c.rt.ToValue("missing arguments"))
	}
	subs := struct {
		UserProperties map[string]string `json:"userProperties"`
		Subscriptions  []struct {
			Topic             string `json:"topic"`
			QoS               byte   `json:"qos"`
			RetainHandling    byte   `json:"retainHandling"`
			NoLocal           bool   `json:"noLocal"`
			RetainAsPublished bool   `json:"retainAsPublished"`
		} `json:"subscriptions"`
	}{}
	if err := c.rt.ExportTo(call.Arguments[0], &subs); err != nil {
		panic(c.rt.ToValue(err.Error()))
	}

	subReq := &paho.Subscribe{}
	subReq.Properties = &paho.SubscribeProperties{}
	subReq.Subscriptions = make([]paho.SubscribeOptions, len(subs.Subscriptions))
	for i, sub := range subs.Subscriptions {
		subReq.Subscriptions[i].Topic = sub.Topic
		subReq.Subscriptions[i].QoS = sub.QoS
		subReq.Subscriptions[i].RetainHandling = sub.RetainHandling
		subReq.Subscriptions[i].NoLocal = sub.NoLocal
		subReq.Subscriptions[i].RetainAsPublished = sub.RetainAsPublished
	}
	for k, v := range subs.UserProperties {
		subReq.Properties.User = append(subReq.Properties.User, paho.UserProperty{
			Key:   k,
			Value: v,
		})
	}

	subRsp, err := c.connMgr.Subscribe(c.ctx, subReq)
	if err != nil {
		panic(c.rt.ToValue(err.Error()))
	}
	_ = subRsp.Properties
	return js.Undefined()
}

func (c *Client) logError(err error) {
	console, ok := c.rt.Get("console").(*js.Object)
	if ok && console != nil {
		callable, ok := js.AssertFunction(console.Get("error"))
		if ok {
			callable(c.obj, c.rt.ToValue(err.Error()))
		}
	}
}

func (c *Client) handleClientError(err error) {
	if c.OnClientError == nil {
		return
	}
	_, e := c.OnClientError(c.obj, c.rt.ToValue(err.Error()))
	if e != nil {
		c.logError(e)
		return
	}
}

func (c *Client) handleConnectError(err error) {
	if c.OnConnectError == nil {
		return
	}
	_, e := c.OnConnectError(c.obj, c.rt.ToValue(err.Error()))
	if e != nil {
		c.logError(e)
		return
	}
}

func (c *Client) handleConnectionUp(_ *autopaho.ConnectionManager, ack *paho.Connack) {
	if c.OnConnect == nil {
		return
	}
	r, err := c.OnConnect(c.obj, c.rt.ToValue(ack))
	if err != nil {
		c.logError(err)
		return
	}
	if r != js.Undefined() {
		rv := r.Export()
		_ = rv
	}
}

func (c *Client) handlePublishReceived(p paho.PublishReceived) (bool, error) {
	if c.OnMessage == nil {
		return false, nil
	}
	packet := c.rt.NewObject()
	packet.Set("packetID", p.Packet.PacketID)
	packet.Set("qos", int(p.Packet.QoS))
	packet.Set("retain", p.Packet.Retain)
	packet.Set("topic", p.Packet.Topic)

	payload := c.rt.NewObject()
	payload.Set("bytes", c.rt.NewArrayBuffer(p.Packet.Payload))
	payload.Set("string", func(call js.FunctionCall) js.Value {
		return c.rt.ToValue(string(p.Packet.Payload))
	})
	packet.Set("payload", payload)

	if p.Packet.Properties != nil {
		props := c.rt.NewObject()
		props.Set("correlationData", p.Packet.Properties.CorrelationData)
		props.Set("contentType", p.Packet.Properties.ContentType)
		props.Set("responseTopic", p.Packet.Properties.ResponseTopic)
		props.Set("payloadFormat", p.Packet.Properties.PayloadFormat)
		props.Set("messageExpiry", p.Packet.Properties.MessageExpiry)
		props.Set("subscriptionIdentifier", p.Packet.Properties.SubscriptionIdentifier)
		props.Set("topicAlias", p.Packet.Properties.TopicAlias)
		userProps := c.rt.NewObject()
		for _, v := range p.Packet.Properties.User {
			userProps.Set(v.Key, c.rt.ToValue(v.Value))
		}
		props.Set("user", userProps)
		packet.Set("properties", props)
	}
	r, err := c.OnMessage(c.obj, packet)
	if err != nil {
		c.logError(err)
		return false, err
	}

	var ret bool
	if err := c.rt.ExportTo(r, &ret); err != nil {
		c.logError(err)
		return true, err
	}
	// TODO: shoule we return 'ret' that returned from js?
	return true, nil
}

func (c *Client) handleServerDisconnect(dc *paho.Disconnect) {
	if c.OnDisconnect == nil {
		return
	}
	r, err := c.OnDisconnect(c.obj, c.rt.ToValue(dc))
	if err != nil {
		c.logError(err)
		return
	}
	if r != js.Undefined() {
		rv := r.Export()
		fmt.Println(rv)
	}
}
