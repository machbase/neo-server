package nats

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/jsh/engine"
	gnats "github.com/nats-io/nats.go"
)

//go:embed nats.js
var nats_js []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"nats.js": nats_js,
	}
}

func Module(rt *goja.Runtime, module *goja.Object) {
	m := module.Get("exports").(*goja.Object)
	m.Set("parseConfig", ParseConfig)
	m.Set("NewClient", NewClient)
}

type Config struct {
	Servers              []string `json:"servers"`
	Name                 string   `json:"name"`
	User                 string   `json:"user"`
	Password             string   `json:"password"`
	Token                string   `json:"token"`
	NoRandomize          *bool    `json:"noRandomize"`
	NoEcho               *bool    `json:"noEcho"`
	Verbose              *bool    `json:"verbose"`
	Pedantic             *bool    `json:"pedantic"`
	AllowReconnect       *bool    `json:"allowReconnect"`
	MaxReconnect         *int     `json:"maxReconnect"`
	ReconnectWait        *int64   `json:"reconnectWait"`
	Timeout              *int64   `json:"timeout"`
	DrainTimeout         *int64   `json:"drainTimeout"`
	FlusherTimeout       *int64   `json:"flusherTimeout"`
	PingInterval         *int64   `json:"pingInterval"`
	MaxPingsOut          *int     `json:"maxPingsOut"`
	RetryOnFailedConnect *bool    `json:"retryOnFailedConnect"`
	SkipHostLookup       *bool    `json:"skipHostLookup"`
}

func ParseConfig(data string) (*gnats.Options, error) {
	conf := Config{}
	if err := json.Unmarshal([]byte(data), &conf); err != nil {
		return nil, err
	}
	opts := gnats.GetDefaultOptions()
	if len(conf.Servers) > 0 {
		opts.Servers = append([]string{}, conf.Servers...)
	}
	if conf.Name != "" {
		opts.Name = conf.Name
	}
	if conf.User != "" {
		opts.User = conf.User
	}
	if conf.Password != "" {
		opts.Password = conf.Password
	}
	if conf.Token != "" {
		opts.Token = conf.Token
	}
	if conf.NoRandomize != nil {
		opts.NoRandomize = *conf.NoRandomize
	}
	if conf.NoEcho != nil {
		opts.NoEcho = *conf.NoEcho
	}
	if conf.Verbose != nil {
		opts.Verbose = *conf.Verbose
	}
	if conf.Pedantic != nil {
		opts.Pedantic = *conf.Pedantic
	}
	if conf.AllowReconnect != nil {
		opts.AllowReconnect = *conf.AllowReconnect
	}
	if conf.MaxReconnect != nil {
		opts.MaxReconnect = *conf.MaxReconnect
	}
	if conf.ReconnectWait != nil {
		opts.ReconnectWait = time.Duration(*conf.ReconnectWait) * time.Millisecond
	}
	if conf.Timeout != nil {
		opts.Timeout = time.Duration(*conf.Timeout) * time.Millisecond
	}
	if conf.DrainTimeout != nil {
		opts.DrainTimeout = time.Duration(*conf.DrainTimeout) * time.Millisecond
	}
	if conf.FlusherTimeout != nil {
		opts.FlusherTimeout = time.Duration(*conf.FlusherTimeout) * time.Millisecond
	}
	if conf.PingInterval != nil {
		opts.PingInterval = time.Duration(*conf.PingInterval) * time.Millisecond
	}
	if conf.MaxPingsOut != nil {
		opts.MaxPingsOut = *conf.MaxPingsOut
	}
	if conf.RetryOnFailedConnect != nil {
		opts.RetryOnFailedConnect = *conf.RetryOnFailedConnect
	}
	if conf.SkipHostLookup != nil {
		opts.SkipHostLookup = *conf.SkipHostLookup
	}
	return &opts, nil
}

func NewClient(obj *goja.Object, dispatch engine.EventDispatchFunc) (*Client, error) {
	return &Client{
		emit: func(event string, data any) {
			dispatch(obj, event, data)
		},
		closed: false,
	}, nil
}

type Client struct {
	conn   *gnats.Conn
	emit   func(event string, data any)
	closed bool
}

func (c *Client) Connect(options gnats.Options) error {
	opts := options
	conn, err := opts.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to NATS server: %w", err)
	}
	c.conn = conn
	c.closed = false
	return nil
}

func (c *Client) IsClosed() bool {
	if c.conn == nil {
		return c.closed
	}
	if c.closed {
		return true
	}
	return c.conn.IsClosed()
}

func (c *Client) Close() {
	if c.closed {
		return
	}
	c.closed = true
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Client) Subscribe(subject string, options map[string]any) (int, error) {
	if c.conn == nil || c.conn.IsClosed() {
		return 0, fmt.Errorf("nats connection is not open")
	}
	handler := func(msg *gnats.Msg) {
		data := map[string]any{
			"topic":   msg.Subject,
			"subject": msg.Subject,
			"reply":   msg.Reply,
			"payload": string(msg.Data),
		}
		c.emit("message", data)
	}
	var err error
	if options != nil {
		if queue, ok := options["queue"].(string); ok && queue != "" {
			_, err = c.conn.QueueSubscribe(subject, queue, handler)
		} else {
			_, err = c.conn.Subscribe(subject, handler)
		}
	} else {
		_, err = c.conn.Subscribe(subject, handler)
	}
	if err != nil {
		return 0, err
	}
	if err := c.conn.Flush(); err != nil {
		return 0, err
	}
	return 1, nil
}

func (c *Client) Publish(subject string, data any, options map[string]any) (int, error) {
	if c.conn == nil || c.conn.IsClosed() {
		return 0, fmt.Errorf("nats connection is not open")
	}
	payload, err := encodePayload(data)
	if err != nil {
		return 0, err
	}
	if options != nil {
		if reply, ok := options["reply"].(string); ok && reply != "" {
			err = c.conn.PublishRequest(subject, reply, payload)
		} else {
			err = c.conn.Publish(subject, payload)
		}
	} else {
		err = c.conn.Publish(subject, payload)
	}
	if err != nil {
		return 0, err
	}
	if err := c.conn.FlushTimeout(5 * time.Second); err != nil {
		return 0, err
	}
	return 0, nil
}

func encodePayload(data any) ([]byte, error) {
	switch val := data.(type) {
	case nil:
		return nil, nil
	case string:
		return []byte(val), nil
	case []byte:
		return val, nil
	default:
		ret, err := json.Marshal(val)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
		return ret, nil
	}
}
