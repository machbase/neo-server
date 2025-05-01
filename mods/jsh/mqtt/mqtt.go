package mqtt

import (
	"context"
	"fmt"
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
		if len(call.Arguments) < 2 {
			panic(rt.ToValue("missing arguments"))
		}
		opts := struct {
			ServerUrls []string `json:"serverUrls"`
			KeepAlive  uint16   `json:"keepAlive"`
			CleanStart bool     `json:"cleanStart"`
		}{}
		if err := rt.ExportTo(call.Arguments[0], &opts); err != nil {
			panic(rt.ToValue(err.Error()))
		}
		var callback js.Callable
		if err := rt.ExportTo(call.Arguments[1], &callback); err != nil {
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
			ctx:      ctx,
			rt:       rt,
			callback: callback,
			obj:      ret,
			clientConfig: autopaho.ClientConfig{
				ServerUrls:                    serverUrls,
				KeepAlive:                     opts.KeepAlive,
				CleanStartOnInitialConnection: opts.CleanStart,
				ConnectRetryDelay:             1 * time.Second, // default 10s
				ConnectTimeout:                5 * time.Second, // default 10s
			},
		}
		client.clientConfig.OnConnectError = client.OnClientError
		client.clientConfig.OnConnectionUp = client.OnConnectionUp
		client.clientConfig.ClientConfig.OnPublishReceived = []func(paho.PublishReceived) (bool, error){client.OnPublishReceived}
		client.clientConfig.OnServerDisconnect = client.OnServerDisconnect
		client.clientConfig.ClientConfig.OnClientError = client.OnClientError

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
	ctx          context.Context
	rt           *js.Runtime
	clientConfig autopaho.ClientConfig
	connMgr      *autopaho.ConnectionManager
	obj          *js.Object
	callback     js.Callable
}

func (c *Client) Connect(call js.FunctionCall) js.Value {
	if c.connMgr != nil {
		panic(c.rt.ToValue("already connected"))
	}
	if cm, err := autopaho.NewConnection(c.ctx, c.clientConfig); err != nil {
		panic(c.rt.ToValue(err.Error()))
	} else {
		c.connMgr = cm
	}
	return js.Undefined()
}

func (c *Client) Disconnect(call js.FunctionCall) js.Value {
	if c.connMgr != nil {
		c.connMgr.Disconnect(c.ctx)
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

func (c *Client) OnClientError(err error) {
	r, e := c.callback(js.Null(), c.rt.ToValue("onClientError"), c.rt.ToValue(err.Error()))
	if e != nil {
		panic(c.rt.ToValue(e.Error()))
	}
	if r != js.Undefined() {
		rv := r.Export()
		fmt.Println(rv)
	}
}

func (c *Client) OnConnectError(err error) {
	r, e := c.callback(js.Null(), c.rt.ToValue("onConnectError"), c.rt.ToValue(err.Error()))
	if e != nil {
		panic(c.rt.ToValue(e.Error()))
	}
	if r != js.Undefined() {
		rv := r.Export()
		fmt.Println(rv)
	}
}

func (c *Client) OnConnectionUp(_ *autopaho.ConnectionManager, ack *paho.Connack) {
	r, e := c.callback(js.Null(), c.rt.ToValue("onConnectionUp"), c.rt.ToValue(ack))
	if e != nil {
		panic(c.rt.ToValue(e.Error()))
	}
	if r != js.Undefined() {
		rv := r.Export()
		fmt.Println(rv)
	}
}

func (c *Client) OnPublishReceived(p paho.PublishReceived) (bool, error) {
	packet := c.rt.NewObject()
	packet.Set("packetID", p.Packet.PacketID)
	packet.Set("qos", p.Packet.QoS)
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
	r, e := c.callback(c.obj, c.rt.ToValue("onPublishReceived"), packet)
	if e != nil {
		panic(c.rt.ToValue(e.Error()))
	}
	if r != js.Undefined() {
		rv := r.Export()
		fmt.Println(rv)
	}
	return true, nil
}

func (c *Client) OnServerDisconnect(_ *paho.Disconnect) {
	r, e := c.callback(js.Null(), c.rt.ToValue("onServerDisconnect"), js.Undefined())
	if e != nil {
		panic(c.rt.ToValue(e.Error()))
	}
	if r != js.Undefined() {
		rv := r.Export()
		fmt.Println(rv)
	}
}
