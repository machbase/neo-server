package nats

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/mods/logging"
	natsio "github.com/nats-io/nats.go"
)

type bridge struct {
	log  logging.Log
	name string
	path string

	alive      bool
	stopSig    chan bool
	natsOpts   natsio.Options
	natsStatus natsio.Status
	natsConn   *natsio.Conn

	subscriberWait sync.WaitGroup
	subscribers    map[*SubscriptionToken]bool
	subscriberLock sync.Mutex
}

func New(name string, path string) *bridge {
	return &bridge{
		log:         logging.GetLog("nats-bridge"),
		name:        name,
		path:        path,
		stopSig:     make(chan bool),
		subscribers: map[*SubscriptionToken]bool{},
	}
}

func (c *bridge) BeforeRegister() error {
	c.natsOpts = natsio.GetDefaultOptions()

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
		if conn, err := c.natsOpts.Connect(); err != nil {
			return err
		} else {
			c.log.Info(c.name + " connected")
			c.alive = true
			c.natsConn = conn
			c.natsStatus = natsio.CONNECTED
			go c.run()
		}
	}

	return nil
}

func (c *bridge) AfterUnregister() error {
	if c.alive {
		c.stopSig <- true
	}
	return nil
}

func (c *bridge) String() string {
	return fmt.Sprintf("bridge '%s' (nats)", c.name)
}

func (c *bridge) Name() string {
	return c.name
}

func (c *bridge) IsConnected() bool {
	return c.natsStatus == natsio.CONNECTED
}

func (c *bridge) run() {
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

type SubscriptionToken struct {
	subscription *natsio.Subscription
	subject      string
	sigChan      chan bool
	msgChan      chan *natsio.Msg
}

func (c *bridge) Subscribe(topic string, cb func(topic string, data []byte, header map[string][]string, respond func([]byte))) (any, error) {
	return c.subscribe0(topic, "", cb)
}

func (c *bridge) QueueSubscribe(topic string, queue string,
	cb func(topic string, data []byte, header map[string][]string, respond func([]byte))) (any, error) {
	return c.subscribe0(topic, queue, cb)
}

const DefaultPendingMessageLimit = 1_000_000

func (c *bridge) subscribe0(topic string, queue string, cb func(topic string, data []byte, header map[string][]string, respond func([]byte))) (any, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("nats connection is unavailable")
	}
	c.subscriberLock.Lock()
	defer c.subscriberLock.Unlock()

	msgChan := make(chan *natsio.Msg, DefaultPendingMessageLimit)
	var subscription *natsio.Subscription
	if queue == "" {
		if s, err := c.natsConn.ChanSubscribe(topic, msgChan); err != nil {
			return nil, err
		} else {
			subscription = s
		}
	} else {
		if s, err := c.natsConn.ChanQueueSubscribe(topic, queue, msgChan); err != nil {
			return nil, err
		} else {
			subscription = s
		}
	}

	st := &SubscriptionToken{
		subscription: subscription,
		subject:      topic,
		sigChan:      make(chan bool),
		msgChan:      msgChan,
	}
	c.subscribers[st] = true

	c.subscriberWait.Add(1)
	go func(st *SubscriptionToken) {
	loop:
		for c.alive {
			select {
			case <-st.sigChan:
				break loop
			case msg := <-st.msgChan:
				var respond func([]byte)
				if msg.Reply != "" {
					respond = func(rdata []byte) {
						msg.Respond(rdata)
					}
				}
				cb(msg.Subject, msg.Data, msg.Header, respond)
				if respond == nil {
					msg.Ack()
				}
			}
		}
		st.subscription.Unsubscribe()
		st.subscription = nil
		c.subscriberWait.Done()
	}(st)
	return st, nil
}

func (c *bridge) Unsubscribe(token any) (bool, error) {
	st, ok := token.(*SubscriptionToken)
	if !ok {
		return false, fmt.Errorf("nats subscription token is not vaild %T", token)
	}
	if !c.IsConnected() {
		return false, fmt.Errorf("nats connection is unavailable")
	}
	c.subscriberLock.Lock()
	defer c.subscriberLock.Unlock()

	if st.subscription != nil {
		st.sigChan <- true
	}
	delete(c.subscribers, st)
	return true, nil
}

func (c *bridge) Publish(topic string, payload any) (bool, error) {
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
