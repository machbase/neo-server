package bridge

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/nats-io/nats.go"
)

type NatsBridge struct {
	log  logging.Log
	name string
	path string

	alive      bool
	stopSig    chan bool
	natsOpts   nats.Options
	natsStatus nats.Status
	natsConn   *nats.Conn

	natsConnMutex sync.Mutex

	subscriberWait sync.WaitGroup
	subscribers    map[*NatsSubscription]bool
	subscriberLock sync.Mutex

	WriteStats
}

func NewNatsBridge(name string, path string) *NatsBridge {
	return &NatsBridge{
		log:         logging.GetLog("nats-bridge"),
		name:        name,
		path:        path,
		stopSig:     make(chan bool),
		subscribers: map[*NatsSubscription]bool{},
	}
}

func (c *NatsBridge) BeforeRegister() error {
	c.natsOpts = nats.GetDefaultOptions()
	c.natsOpts.MaxReconnect = -1
	fields := strings.Fields(c.path)
	for _, field := range fields {
		kv := strings.SplitN(field, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		switch strings.ToLower(key) {
		case "server":
			c.natsOpts.Servers = append(c.natsOpts.Servers, val)
		case "norandomize", "no-randomize":
			if k, err := strconv.ParseBool(val); err == nil {
				c.natsOpts.NoRandomize = k
			}
		case "noecho", "no-echo":
			if k, err := strconv.ParseBool(val); err == nil {
				c.natsOpts.NoEcho = k
			}
		case "name":
			c.natsOpts.Name = val
		case "verbose":
			if k, err := strconv.ParseBool(val); err == nil {
				c.natsOpts.Verbose = k
			}
		case "pedantic":
			if k, err := strconv.ParseBool(val); err == nil {
				c.natsOpts.Pedantic = k
			}
		case "allowreconnect", "allow-reconnect":
			if k, err := strconv.ParseBool(val); err == nil {
				c.natsOpts.AllowReconnect = k
			}
		case "maxreconnect", "max-reconnect":
			if k, err := strconv.ParseInt(val, 10, 32); err == nil {
				c.natsOpts.MaxReconnect = int(k)
			}
		case "reconnectwait", "reconnect-wait":
			if k, err := time.ParseDuration(val); err == nil {
				c.natsOpts.ReconnectWait = k
			}
		case "timeout":
			if k, err := time.ParseDuration(val); err == nil {
				c.natsOpts.Timeout = k
			}
		case "draintimeout", "drain-timeout":
			if k, err := time.ParseDuration(val); err == nil {
				c.natsOpts.DrainTimeout = k
			}
		case "flushertimeout", "flusher-timeout":
			if k, err := time.ParseDuration(val); err == nil {
				c.natsOpts.FlusherTimeout = k
			}
		case "pinginterval", "ping-interval":
			if k, err := time.ParseDuration(val); err == nil {
				c.natsOpts.PingInterval = k
			}
		case "maxpingsout", "max-pings-out":
			if k, err := strconv.ParseInt(val, 10, 32); err == nil {
				c.natsOpts.MaxPingsOut = int(k)
			}
		case "user":
			c.natsOpts.User = val
		case "password":
			c.natsOpts.Password = val
		case "token":
			c.natsOpts.Token = val
		case "retryonfailedconnect", "retry-on-failed-connect":
			if k, err := strconv.ParseBool(val); err == nil {
				c.natsOpts.RetryOnFailedConnect = k
			}
		case "skiphostlookup", "skip-host-lookup":
			if k, err := strconv.ParseBool(val); err == nil {
				c.natsOpts.SkipHostLookup = k
			}
		default:
			c.log.Infof("unknown option, %s=%s", key, val)
		}
	}
	if len(c.natsOpts.Servers) > 0 {
		c.tryConnect()
	}

	return nil
}

func (c *NatsBridge) tryConnect() error {
	if c.IsConnected() {
		return nil
	}
	if conn, err := c.natsOpts.Connect(); err != nil {
		return err
	} else {
		c.log.Info(c.name + " connected")
		c.alive = true
		c.natsConn = conn
		c.natsStatus = nats.CONNECTED
		go c.run()
	}
	return nil
}

func (c *NatsBridge) AfterUnregister() error {
	if c.alive {
		c.stopSig <- true
	}
	return nil
}

func (c *NatsBridge) String() string {
	return fmt.Sprintf("bridge '%s' (nats)", c.name)
}

func (c *NatsBridge) Name() string {
	return c.name
}

func (c *NatsBridge) IsConnected() bool {
	if c.natsConn != nil && c.natsConn.IsConnected() {
		return true
	}
	return false
}

type NatsStats struct {
	nats.Statistics
	Appended uint64
	Inserted uint64
}

func (c *NatsBridge) Stats() NatsStats {
	if c.natsConn == nil {
		return NatsStats{}
	}
	return NatsStats{
		Statistics: c.natsConn.Stats(),
		Appended:   atomic.LoadUint64(&c.Appended),
		Inserted:   atomic.LoadUint64(&c.Inserted),
	}
}

func (c *NatsBridge) run() {
	stsChan := c.natsConn.StatusChanged()
	for c.alive {
		select {
		case status := <-stsChan:
			c.natsStatus = status
			c.log.Info(c.name, "status", status.String())
		case <-c.stopSig:
			c.alive = false
		}
	}
	for st := range c.subscribers {
		if st.subscription != nil {
			st.sigChan <- true
		}
	}

	c.subscriberWait.Wait()
	c.natsConn.Close()
}

type NatsSubscription struct {
	bridge       *NatsBridge
	subscription *nats.Subscription
	subject      string
	queueName    string
	streamName   string
	sigChan      chan bool
	msgChan      chan *nats.Msg
	msgChanSize  int
	writeStats   *WriteStats
}

func (ns *NatsSubscription) AddAppended(delta uint64) {
	atomic.AddUint64(&ns.writeStats.Appended, delta)
}

func (ns *NatsSubscription) AddInserted(delta uint64) {
	atomic.AddUint64(&ns.writeStats.Inserted, delta)
}

func (ns *NatsSubscription) Unsubscribe() error {
	if ns.bridge == nil || !ns.bridge.IsConnected() {
		return fmt.Errorf("nats connection is unavailable")
	}
	ns.bridge.subscriberLock.Lock()
	defer ns.bridge.subscriberLock.Unlock()

	if ns.subscription != nil {
		ns.sigChan <- true
	}
	delete(ns.bridge.subscribers, ns)
	return nil
}

type NatsMsgHandler func(*nats.Msg)

type NatsSubscribeOption func(ns *NatsSubscription)

const NatsDefaultPendingMessageLimit = 1_000_000

func NatsPendingMessageLimit(size int) NatsSubscribeOption {
	return func(s *NatsSubscription) {
		s.msgChanSize = size
	}
}

func NatsQueueGroup(queueName string) NatsSubscribeOption {
	return func(s *NatsSubscription) {
		s.queueName = queueName
	}
}

func NatsStreamName(streamName string) NatsSubscribeOption {
	return func(s *NatsSubscription) {
		s.streamName = streamName
	}
}

func (c *NatsBridge) Subscribe(topic string, cb NatsMsgHandler, opts ...NatsSubscribeOption) (*NatsSubscription, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("nats connection is unavailable")
	}

	st := &NatsSubscription{
		bridge:     c,
		subject:    topic,
		sigChan:    make(chan bool),
		writeStats: &c.WriteStats,
	}

	for _, o := range opts {
		o(st)
	}

	if st.msgChanSize <= 0 {
		st.msgChanSize = NatsDefaultPendingMessageLimit
	}

	c.subscriberLock.Lock()
	defer c.subscriberLock.Unlock()

	st.msgChan = make(chan *nats.Msg, st.msgChanSize)

	if st.streamName != "" {
		var js nats.JetStreamContext
		if stream, err := c.natsConn.JetStream(); err != nil {
			return nil, err
		} else {
			js = stream
		}
		consumerName := "neo_sub"
		consumerConfig := &nats.ConsumerConfig{
			Name:          consumerName,
			DeliverPolicy: nats.DeliverNewPolicy,
		}

		if _, err := js.AddConsumer(st.streamName, consumerConfig); err != nil {
			if jserr, ok := err.(nats.JetStreamError); ok && jserr.APIError().Is(nats.ErrConsumerNameAlreadyInUse) {
				if err2 := js.DeleteConsumer(st.streamName, consumerName); err2 != nil {
					return nil, err2
				}
				if _, err := js.AddConsumer(st.streamName, consumerConfig); err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
		subOpts := []nats.SubOpt{nats.Bind(st.streamName, consumerName)}
		if st.queueName == "" {
			if s, err := js.ChanSubscribe(st.subject, st.msgChan, subOpts...); err != nil {
				return nil, err
			} else {
				st.subscription = s
			}
		} else {
			if s, err := js.ChanQueueSubscribe(st.subject, st.queueName, st.msgChan, subOpts...); err != nil {
				return nil, err
			} else {
				st.subscription = s
			}
		}
	} else {
		if st.queueName == "" {
			if s, err := c.natsConn.ChanSubscribe(st.subject, st.msgChan); err != nil {
				return nil, err
			} else {
				st.subscription = s
			}
		} else {
			if s, err := c.natsConn.ChanQueueSubscribe(st.subject, st.queueName, st.msgChan); err != nil {
				return nil, err
			} else {
				st.subscription = s
			}
		}
	}

	c.subscribers[st] = true

	c.subscriberWait.Add(1)
	go func(st *NatsSubscription) {
	loop:
		for c.alive {
			select {
			case <-st.sigChan:
				break loop
			case msg := <-st.msgChan:
				cb(msg)
			}
		}
		st.subscription.Unsubscribe()
		st.subscription = nil
		c.subscriberWait.Done()
	}(st)
	return st, nil
}

func (c *NatsBridge) Publish(topic string, payload any) (bool, error) {
	if !c.IsConnected() {
		return false, fmt.Errorf("nats connection is unavailable")
	}
	var data []byte
	switch raw := payload.(type) {
	case []byte:
		data = raw
	case string:
		data = []byte(raw)
	default:
		return false, fmt.Errorf("nats bridge can not publish %T", raw)
	}
	err := c.natsConn.Publish(topic, data)
	return err == nil, err
}

func (c *NatsBridge) TestConnection() (bool, string) {
	c.natsConnMutex.Lock()
	defer c.natsConnMutex.Unlock()

	connected := c.IsConnected()
	if !connected {
		if err := c.tryConnect(); err != nil {
			c.log.Error("failed to connect", err)
		}
		if connected := c.IsConnected(); !connected {
			return false, "not connected"
		}
	}

	if err := c.natsConn.FlushTimeout(10 * time.Second); err != nil {
		c.log.Error("error to connect", err.Error())
		return false, "error to connect"
	}

	return true, "success"
}
