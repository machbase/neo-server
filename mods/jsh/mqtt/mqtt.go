package mqtt

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"sync/atomic"
	"time"

	js "github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/autopaho/queue"
	"github.com/eclipse/paho.golang/autopaho/queue/memory"
	"github.com/eclipse/paho.golang/paho"
	"github.com/machbase/neo-server/v8/mods/jsh/builtin"
)

func NewModuleLoader(ctx context.Context) require.ModuleLoader {
	return func(rt *js.Runtime, module *js.Object) {
		// m = require("@jsh/mqtt")
		o := module.Get("exports").(*js.Object)
		// c = new mqtt.Client()
		o.Set("Client", new_client(ctx, rt))
	}
}

var clientIdSer = int64(0)

func new_client(ctx context.Context, rt *js.Runtime) func(call js.ConstructorCall) *js.Object {
	return func(call js.ConstructorCall) *js.Object {
		if len(call.Arguments) < 1 {
			panic(rt.ToValue("missing arguments"))
		}
		clientId := atomic.AddInt64(&clientIdSer, 1)
		opts := struct {
			ServerUrls            []string `json:"serverUrls"`
			KeepAlive             uint16   `json:"keepAlive,omitempty"`
			CleanStart            bool     `json:"cleanStart,omitempty"`
			Username              string   `json:"username,omitempty"`
			Password              string   `json:"password,omitempty"`
			ClientID              string   `json:"clientID,omitempty"`
			Debug                 bool     `json:"debug,omitempty"`
			Queue                 string   `json:"queue,omitempty"`
			SessionExpiryInterval uint32   `json:"sessionExpiryInterval,omitempty"`
			ConnectRetryDelay     int      `json:"connectRetryDelay,omitempty"`
			ConnectTimeout        int      `json:"connectTimeout,omitempty"`
			PacketTimeout         int      `json:"packetTimeout,omitempty"`
		}{
			KeepAlive:             60,
			CleanStart:            true,
			ClientID:              fmt.Sprintf("mqtt-client-%d", clientId),
			Queue:                 "memory",
			SessionExpiryInterval: 60,
			ConnectRetryDelay:     10,
			ConnectTimeout:        10,
			PacketTimeout:         5,
		}
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

		var q queue.Queue
		if opts.Queue == "memory" || opts.Queue == "" {
			q = memory.New()
		}
		ret := rt.NewObject()

		client := &Client{
			ctx: ctx,
			rt:  rt,
			obj: ret,
			config: &autopaho.ClientConfig{
				Queue:                         q,
				ConnectUsername:               opts.Username,
				ConnectPassword:               []byte(opts.Password),
				ServerUrls:                    serverUrls,
				KeepAlive:                     opts.KeepAlive,
				CleanStartOnInitialConnection: opts.CleanStart,
				SessionExpiryInterval:         opts.SessionExpiryInterval,
				ConnectRetryDelay:             time.Duration(opts.ConnectRetryDelay) * time.Second,
				ConnectTimeout:                time.Duration(opts.ConnectTimeout) * time.Second,
				ClientConfig: paho.ClientConfig{
					ClientID:      opts.ClientID,
					PacketTimeout: time.Duration(opts.PacketTimeout) * time.Second,
				},
			},
		}
		client.config.OnConnectError = client.handleConnectError
		client.config.OnConnectionUp = client.handleConnectionUp
		client.config.OnServerDisconnect = client.handleServerDisconnect
		client.config.ClientConfig.OnClientError = client.handleClientError
		client.config.OnPublishReceived = []func(paho.PublishReceived) (bool, error){client.handlePublishReceived}
		if opts.Debug {
			client.config.Debug = client
			client.config.Errors = client
			client.config.PahoDebug = client
			client.config.PahoErrors = client
		}

		// c.connect()
		ret.Set("connect", client.Connect)
		// c.disconnect()
		ret.Set("disconnect", client.Disconnect)
		// c.subscribe(subs)
		ret.Set("subscribe", client.Subscribe)
		// c.unsubscribe(unsubs)
		ret.Set("unsubscribe", client.Unsubscribe)
		// c.publish(topic, payload, qos)
		// c.publish(topic, payload)
		ret.Set("publish", client.Publish)
		return ret
	}
}

type Client struct {
	ctx       context.Context
	rt        *js.Runtime
	config    *autopaho.ClientConfig
	connMgr   *autopaho.ConnectionManager
	obj       *js.Object
	connReady chan struct{}
}

func (c *Client) Connect(call js.FunctionCall) js.Value {
	if c.connMgr != nil {
		panic(c.rt.ToValue("already connected"))
	}

	c.connReady = make(chan struct{}, 1)
	defer func() {
		select {
		case <-c.connReady:
		case <-c.ctx.Done():
		}
	}()

	opts := struct {
		Timeout int `json:"timeout,omitempty"`
	}{}
	if len(call.Arguments) > 0 {
		if err := c.rt.ExportTo(call.Arguments[0], &opts); err != nil {
			panic(c.rt.ToValue(err.Error()))
		}
	}
	if cm, err := autopaho.NewConnection(c.ctx, *c.config); err != nil {
		panic(c.rt.ToValue(err.Error()))
	} else {
		c.connMgr = cm
	}
	if cleaner, ok := c.ctx.(builtin.JshContext); ok {
		cleaner.AddCleanup(func(out io.Writer) {
			if c.connMgr != nil {
				io.WriteString(out, "forced a mqtt connection to close by cleanup\n")
				c.cleanup()
			}
		})
	}
	var waitCtx context.Context
	if opts.Timeout > 0 {
		c, cancel := context.WithTimeout(c.ctx, time.Duration(opts.Timeout)*time.Millisecond)
		waitCtx = c
		defer cancel()
	} else {
		waitCtx = c.ctx
	}

	if err := c.connMgr.AwaitConnection(waitCtx); err != nil {
		panic(c.rt.ToValue(err.Error()))
	}
	return js.Undefined()
}

func (c *Client) cleanup() {
	if c.connMgr != nil {
		c.connMgr.Disconnect(c.ctx)
		c.connMgr = nil
	}
}

func (c *Client) Disconnect(call js.FunctionCall) js.Value {
	if c.connMgr == nil {
		panic(c.rt.ToValue("not connected"))
	}

	opts := struct {
		WaitForEmptyQueue bool `json:"waitForEmptyQueue,omitempty"`
		Timeout           int  `json:"timeout,omitempty"`
	}{}
	if len(call.Arguments) > 0 {
		if err := c.rt.ExportTo(call.Arguments[0], &opts); err != nil {
			panic(c.rt.ToValue(err.Error()))
		}
	}
	var waitCtx context.Context
	if opts.Timeout > 0 {
		c, cancel := context.WithTimeout(context.Background(), time.Duration(opts.Timeout)*time.Millisecond)
		defer cancel()
		waitCtx = c
	} else {
		waitCtx = c.ctx
	}

	if opts.WaitForEmptyQueue && c.config.Queue != nil {
		if q, ok := c.config.Queue.(interface{ WaitForEmpty() chan struct{} }); ok {
			if waitCtx != nil {
				select {
				case <-q.WaitForEmpty():
				case <-waitCtx.Done():
				}
			} else {
				<-q.WaitForEmpty()
			}
		}
	}
	if c.connMgr != nil {
		c.connMgr.Disconnect(c.ctx)
		select {
		case <-waitCtx.Done():
		case <-c.connMgr.Done():
		}
		c.connMgr = nil
	}
	return js.Undefined()
}

func (c *Client) Publish(call js.FunctionCall) js.Value {
	if c.connMgr == nil {
		panic(c.rt.ToValue("not connected"))
	}
	if len(call.Arguments) != 2 {
		panic(c.rt.ToValue("missing arguments"))
	}
	pub := struct {
		PacketID   uint16 `json:"packetID"`
		QoS        byte   `json:"qos"`
		Retain     bool   `json:"retain"`
		Topic      string `json:"topic"`
		Properties struct {
			CorrelationData        []byte            `json:"correlationData,omitempty"`
			ContentType            string            `json:"contentType,omitempty"`
			ResponseTopic          string            `json:"responseTopic,omitempty"`
			PayloadFormat          *byte             `json:"payloadFormat,omitempty"`
			MessageExpiry          *uint32           `json:"messageExpiry,omitempty"`
			SubscriptionIdentifier *int              `json:"subscriptionIdentifier,omitempty"`
			TopicAlias             *uint16           `json:"topicAlias,omitempty"`
			User                   map[string]string `json:"user"`
		} `json:"properties"`
	}{}

	if err := c.rt.ExportTo(call.Arguments[0], &pub); err != nil {
		panic(c.rt.ToValue(err.Error()))
	}
	payload := []byte{}
	if err := c.rt.ExportTo(call.Arguments[1], &payload); err != nil {
		panic(c.rt.ToValue(err.Error()))
	}

	pubReq := &paho.Publish{
		PacketID: pub.PacketID,
		QoS:      pub.QoS,
		Retain:   pub.Retain,
		Topic:    pub.Topic,
		Payload:  payload,
		Properties: &paho.PublishProperties{
			ContentType:   pub.Properties.ContentType,
			ResponseTopic: pub.Properties.ResponseTopic,
		},
	}
	for k, v := range pub.Properties.User {
		pubReq.Properties.User = append(pubReq.Properties.User,
			paho.UserProperty{Key: k, Value: v})
	}

	var pubRsp *paho.PublishResponse
	var err error
	if c.config.Queue != nil && pubReq.QoS > 0 {
		err = c.connMgr.PublishViaQueue(c.ctx, &autopaho.QueuePublish{Publish: pubReq})
	} else {
		pubRsp, err = c.connMgr.Publish(c.ctx, pubReq)
	}
	if err != nil {
		panic(c.rt.ToValue(err.Error()))
	}

	var reasonCode int
	ret := c.rt.NewObject()
	if pubRsp != nil {
		if rp := pubRsp.Properties; rp != nil {
			prop := c.rt.NewObject()
			for _, v := range rp.User {
				prop.Set(v.Key, c.rt.ToValue(v.Value))
			}
			prop.Set("correlationData", rp.ReasonString)
			ret.Set("properties", prop)
		}
		reasonCode = int(pubRsp.ReasonCode)
	}
	ret.Set("reasonCode", reasonCode)
	return ret
}

func (c *Client) Subscribe(call js.FunctionCall) js.Value {
	if c.connMgr == nil {
		panic(c.rt.ToValue("not connected"))
	}
	if len(call.Arguments) < 1 {
		panic(c.rt.ToValue("missing arguments"))
	}
	defer func() {
		if r := recover(); r != nil {
			c.logError(fmt.Errorf("subscribe: %v", r))
		}
	}()

	subs := struct {
		Properties struct {
			User map[string]string `json:"user"`
		} `json:"properties"`
		Subscriptions []struct {
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
	for k, v := range subs.Properties.User {
		subReq.Properties.User = append(subReq.Properties.User, paho.UserProperty{
			Key:   k,
			Value: v,
		})
	}

	subAck, err := c.connMgr.Subscribe(c.ctx, subReq)
	if err != nil {
		panic(c.rt.ToValue(err.Error()))
	}
	ackObj := c.rt.NewObject()
	reasons := make([]js.Value, len(subAck.Reasons))
	for i, reason := range subAck.Reasons {
		reasons[i] = c.rt.ToValue(int(reason))
	}
	ackObj.Set("reasons", reasons)
	props := c.rt.NewObject()
	user := c.rt.NewObject()
	if p := subAck.Properties; p != nil {
		props.Set("reasonString", c.rt.ToValue(p.ReasonString))
		for _, v := range p.User {
			user.Set(v.Key, c.rt.ToValue(v.Value))
		}
	}
	props.Set("user", user)
	ackObj.Set("properties", props)
	return ackObj
}

func (c *Client) Unsubscribe(call js.FunctionCall) js.Value {
	if c.connMgr == nil {
		panic(c.rt.ToValue("not connected"))
	}
	if len(call.Arguments) < 1 {
		panic(c.rt.ToValue("missing arguments"))
	}
	defer func() {
		if r := recover(); r != nil {
			c.logError(fmt.Errorf("subscribe: %v", r))
		}
	}()

	unsubs := struct {
		Topics     []string `json:"topics"`
		Properties struct {
			User map[string]string `json:"user"`
		} `json:"properties"`
	}{}
	if err := c.rt.ExportTo(call.Arguments[0], &unsubs); err != nil {
		panic(c.rt.ToValue(err.Error()))
	}

	unsubReq := &paho.Unsubscribe{Topics: unsubs.Topics}
	unsubReq.Properties = &paho.UnsubscribeProperties{}
	for k, v := range unsubs.Properties.User {
		unsubReq.Properties.User = append(unsubReq.Properties.User, paho.UserProperty{
			Key:   k,
			Value: v,
		})
	}

	unsubAck, err := c.connMgr.Unsubscribe(c.ctx, unsubReq)
	if err != nil {
		panic(c.rt.ToValue(err.Error()))
	}
	ackObj := c.rt.NewObject()
	reasons := make([]js.Value, len(unsubAck.Reasons))
	for i, reason := range unsubAck.Reasons {
		reasons[i] = c.rt.ToValue(int(reason))
	}
	ackObj.Set("reasons", reasons)
	props := c.rt.NewObject()
	user := c.rt.NewObject()
	if p := unsubAck.Properties; p != nil {
		props.Set("reasonString", c.rt.ToValue(p.ReasonString))
		for _, v := range p.User {
			user.Set(v.Key, c.rt.ToValue(v.Value))
		}
	}
	props.Set("user", user)
	ackObj.Set("properties", props)
	return ackObj
}

func (c *Client) Println(args ...interface{}) {
	fmt.Println(args...)
}

func (c *Client) Printf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
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
	var callback js.Callable
	if v, ok := js.AssertFunction(c.obj.Get("onClientError")); ok && v != nil {
		callback = v
	}
	if c.connMgr == nil || callback == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			c.logError(fmt.Errorf("handleClientError: %v", r))
			c.cleanup()
		}
	}()
	_, e := callback(c.obj, c.rt.ToValue(err.Error()))
	if e != nil {
		c.logError(e)
		return
	}
}

func (c *Client) handleConnectError(err error) {
	var callback js.Callable
	if v, ok := js.AssertFunction(c.obj.Get("onConnectError")); ok && v != nil {
		callback = v
	}
	if c.connMgr == nil || callback == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			c.logError(fmt.Errorf("handleConnectError: %v", r))
			c.cleanup()
		}
	}()
	_, e := callback(c.obj, c.rt.ToValue(err.Error()))
	if e != nil {
		c.logError(e)
		return
	}
}

func (c *Client) handleConnectionUp(_ *autopaho.ConnectionManager, ack *paho.Connack) {
	var callback js.Callable
	if v, ok := js.AssertFunction(c.obj.Get("onConnect")); ok && v != nil {
		callback = v
	}
	if c.connMgr == nil || callback == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			c.logError(fmt.Errorf("handleConnectionUp: %v", r))
			c.cleanup()
		}
		if c.connReady != nil {
			close(c.connReady)
		}
	}()
	ackObj := c.rt.NewObject()
	ackObj.Set("sessionPresent", ack.SessionPresent)
	ackObj.Set("reasonCode", int(ack.ReasonCode))
	props := c.rt.NewObject()
	userProps := c.rt.NewObject()
	if ack.Properties != nil {
		props.Set("reasonString", ack.Properties.ReasonString)
		props.Set("reasonInfo", ack.Properties.ResponseInfo)
		props.Set("assignedClientID", ack.Properties.AssignedClientID)
		props.Set("authMethod", ack.Properties.AuthMethod)
		if ack.Properties.ServerKeepAlive != nil {
			props.Set("serverKeepAlive", c.rt.ToValue(*ack.Properties.ServerKeepAlive))
		} else {
			props.Set("serverKeepAlive", js.Undefined())
		}
		if ack.Properties.SessionExpiryInterval != nil {
			props.Set("sessionExpiryInterval", c.rt.ToValue(*ack.Properties.SessionExpiryInterval))
		} else {
			props.Set("sessionExpiryInterval", js.Undefined())
		}
		for _, v := range ack.Properties.User {
			userProps.Set(v.Key, c.rt.ToValue(v.Value))
		}
	}
	props.Set("user", userProps)
	ackObj.Set("properties", props)

	r, err := callback(c.obj, ackObj)
	if err != nil {
		c.logError(err)
		return
	}
	if r != nil && r != js.Undefined() && r != js.Null() {
		rv := r.Export()
		_ = rv
	}
}

func (c *Client) handlePublishReceived(p paho.PublishReceived) (bool, error) {
	var callback js.Callable
	if v, ok := js.AssertFunction(c.obj.Get("onMessage")); ok && v != nil {
		callback = v
	}
	if c.connMgr == nil || callback == nil {
		return false, nil
	}
	defer func() {
		if r := recover(); r != nil {
			c.logError(fmt.Errorf("handlePublishReceived: %v", r))
		}
	}()
	pktObj := c.rt.NewObject()
	pktObj.Set("packetID", p.Packet.PacketID)
	pktObj.Set("topic", p.Packet.Topic)
	pktObj.Set("qos", int(p.Packet.QoS))
	pktObj.Set("retain", p.Packet.Retain)

	payload := c.rt.NewObject()
	payload.Set("bytes", func(call js.FunctionCall) js.Value {
		return c.rt.ToValue(p.Packet.Payload)
	})
	payload.Set("string", func(call js.FunctionCall) js.Value {
		return c.rt.ToValue(string(p.Packet.Payload))
	})
	pktObj.Set("payload", payload)

	props := c.rt.NewObject()
	userProps := c.rt.NewObject()
	if p.Packet.Properties != nil {
		props.Set("correlationData", p.Packet.Properties.CorrelationData)
		props.Set("contentType", p.Packet.Properties.ContentType)
		props.Set("responseTopic", p.Packet.Properties.ResponseTopic)
		if p.Packet.Properties.PayloadFormat != nil {
			props.Set("payloadFormat", c.rt.ToValue(*p.Packet.Properties.PayloadFormat))
		} else {
			props.Set("payloadFormat", js.Undefined())
		}
		if p.Packet.Properties.MessageExpiry != nil {
			props.Set("messageExpiry", c.rt.ToValue(*p.Packet.Properties.MessageExpiry))
		} else {
			props.Set("messageExpiry", js.Undefined())
		}
		if p.Packet.Properties.SubscriptionIdentifier != nil {
			props.Set("subscriptionIdentifier", c.rt.ToValue(*p.Packet.Properties.SubscriptionIdentifier))
		} else {
			props.Set("subscriptionIdentifier", js.Undefined())
		}
		if p.Packet.Properties.TopicAlias != nil {
			props.Set("topicAlias", c.rt.ToValue(*p.Packet.Properties.TopicAlias))
		} else {
			props.Set("topicAlias", js.Undefined())
		}
		for _, v := range p.Packet.Properties.User {
			userProps.Set(v.Key, c.rt.ToValue(v.Value))
		}
	}
	props.Set("user", userProps)
	pktObj.Set("properties", props)
	r, err := callback(c.obj, pktObj)
	if err != nil {
		c.logError(err)
		return true, err
	}
	if r == nil || r == js.Undefined() || r == js.Null() {
		return true, nil
	}
	var ret bool
	if err := c.rt.ExportTo(r, &ret); err != nil {
		c.logError(err)
		return true, err
	}
	return ret, nil
}

func (c *Client) handleServerDisconnect(dc *paho.Disconnect) {
	var callback js.Callable
	if v, ok := js.AssertFunction(c.obj.Get("onDisconnect")); ok && v != nil {
		callback = v
	}
	if callback == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			c.logError(fmt.Errorf("handleServerDisconnect: %v", r))
		}
	}()
	r, err := callback(c.obj, c.rt.ToValue(dc))
	if err != nil {
		c.logError(err)
		return
	}
	if r != js.Undefined() {
		rv := r.Export()
		fmt.Println(rv)
	}
}
