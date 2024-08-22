package mqtt2

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/service/security"
	"github.com/machbase/neo-server/mods/tql"
	"github.com/machbase/neo-server/mods/util"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/pkg/errors"
)

type Service interface {
	Start() error
	Stop()
	Statz() map[string]any
	WsHandlerFunc() func(w http.ResponseWriter, r *http.Request)
}

type Option func(s *mqtt2) error

func New(db api.Database, opts ...Option) (Service, error) {
	log := logging.GetLog("mqtt-v2")

	caps := mqtt.NewDefaultServerCapabilities()
	svr := &mqtt2{
		log:       log,
		db:        db,
		appenders: cmap.New(),
		broker: mqtt.New(&mqtt.Options{
			Logger:                 logging.Wrap(log),
			InlineClient:           true,
			SysTopicResendInterval: 5,
			Capabilities:           caps,
		}),
		defaultReplyTopic: "db/reply",
	}
	for _, opt := range opts {
		if err := opt(svr); err != nil {
			return nil, err
		}
	}
	if err := svr.broker.AddHook(&AuthHook{svr: svr}, nil); err != nil {
		return nil, err
	}
	return svr, nil
}

func LoadTlsConfig(certFile string, keyFile string, loadSystemCA bool, loadPrivateCA bool) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	var rootCAs *x509.CertPool
	if loadSystemCA {
		rootCAs, _ = x509.SystemCertPool()
	}
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	if loadPrivateCA {
		// append root ca
		ca, err := os.ReadFile(certFile)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("fail to load ca key: %s\n", certFile))
		}
		if ok := rootCAs.AppendCertsFromPEM(ca); !ok {
			return nil, errors.Wrap(err, fmt.Sprintf("fail to add ca key: %s\n", certFile))
		}
	}
	ret := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		ClientAuth:         tls.RequireAndVerifyClientCert,
		ClientCAs:          rootCAs,
		GetConfigForClient: configureTlsConn(),
	}

	return ret, nil
}

func configureTlsConn() func(*tls.ClientHelloInfo) (*tls.Config, error) {
	return func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
		if conn, ok := hello.Conn.(*net.TCPConn); ok {
			configureTcpConn(conn)
		}
		return nil, nil
	}
}

func configureTcpConn(conn *net.TCPConn) {
	soLinger := 0
	keepAlive := 10
	noDelay := true
	if conn == nil {
		return
	}
	conn.SetLinger(soLinger)
	if keepAlive > 0 {
		conn.SetKeepAlive(true)
		conn.SetKeepAlivePeriod(time.Duration(keepAlive) * time.Second)
	} else {
		conn.SetKeepAlive(false)
	}
	conn.SetNoDelay(noDelay)
}

func WithWsHandleListener(addr string) Option {
	return func(s *mqtt2) error {
		s.wsListener = &WsListener{svr: s, id: "mqtt2-ws", addr: addr}
		return s.broker.AddListener(s.wsListener)
	}
}

// WithTcpListener creates a new TCP listener with the given address and TLS configuration.
// If tlsConfig is nil, the listener will not use TLS.
func WithTcpListener(addr string, tlsConfig *tls.Config) Option {
	return func(s *mqtt2) error {
		qty := s.broker.Listeners.Len()
		id := fmt.Sprintf("tcp-%d", qty)
		if tlsConfig != nil {
			id = fmt.Sprintf("tls-%d", qty)
		}
		tcp := listeners.NewTCP(listeners.Config{
			ID:        id,
			Address:   addr,
			TLSConfig: tlsConfig,
		})
		return s.broker.AddListener(tcp)
	}
}

// WithWebsocketListener creates a new Websocket listener with the given address and TLS configuration.
// If tlsConfig is nil, the listener will not use TLS.
func WithWebsocketListener(addr string, tlsConfig *tls.Config) Option {
	return func(s *mqtt2) error {
		qty := s.broker.Listeners.Len()
		id := fmt.Sprintf("ws-%d", qty)
		if tlsConfig != nil {
			id = fmt.Sprintf("wss-%d", qty)
		}
		if tlsConfig != nil {
			s.log.Info("MQTT TLS enabled")
		}

		ws := listeners.NewWebsocket(listeners.Config{
			ID:        id,
			Address:   addr,
			TLSConfig: tlsConfig,
		})
		return s.broker.AddListener(ws)
	}
}

func WithAuthServer(authSvc security.AuthServer, enableTokenAuth bool) Option {
	return func(s *mqtt2) error {
		s.authServer = authSvc
		s.enableTokenAuth = enableTokenAuth
		if s.enableTokenAuth {
			s.log.Info("MQTT token authentication enabled")
		} else {
			s.log.Infof("MQTT token authentication disabled")
		}
		return nil
	}
}

func WithTqlLoader(loader tql.Loader) Option {
	return func(s *mqtt2) error {
		s.tqlLoader = loader
		return nil
	}
}

// WithMaxMessageSizeLimit sets the maximum message size allowed for incoming messages.
// If zero, no limit is enforced.
func WithMaxMessageSizeLimit(limit int) Option {
	return func(s *mqtt2) error {
		s.broker.Options.Capabilities.MaximumPacketSize = uint32(limit)
		return nil
	}
}

type mqtt2 struct {
	log               logging.Log
	db                api.Database
	broker            *mqtt.Server
	appenders         cmap.ConcurrentMap
	authServer        security.AuthServer
	enableTokenAuth   bool
	tqlLoader         tql.Loader
	defaultReplyTopic string
	wsListener        *WsListener
}

func (s *mqtt2) Start() error {
	go s.broker.Serve()
	return nil
}

func (s *mqtt2) Stop() {
	if s.broker != nil {
		s.broker.Close()
	}
}

func (s *mqtt2) Statz() map[string]any {
	nfo := s.broker.Info
	buf, _ := json.Marshal(nfo)
	ret := map[string]any{}
	json.Unmarshal(buf, &ret)
	delete(ret, "version")
	delete(ret, "uptime")
	delete(ret, "time")
	delete(ret, "started")
	delete(ret, "threads")
	delete(ret, "memory_alloc")
	return ret
}

func (s *mqtt2) WsHandlerFunc() func(w http.ResponseWriter, r *http.Request) {
	return s.wsListener.WsHandler
}

func (s *mqtt2) onACLCheck(_ *mqtt.Client, topic string, write bool) bool {
	if topic == "db/query" && !write {
		// can not subscribe 'db/query'
		return false
	} else if (topic == "db/reply" || strings.HasPrefix(topic, "db/reply/")) && write {
		// can not publish 'db/reply/#'
		return false
	} else if (topic == "db/tql" || strings.HasPrefix(topic, "db/tql/")) && !write {
		// can not subscribe 'db/tql/#'
		return false
	} else if topic == "db" {
		// can not subscribe & publish 'db'
		return false
	} else if strings.HasPrefix(topic, "db/#") && !write {
		// can not subscribe 'db/#'
		return false
	} else if strings.HasPrefix(topic, "$SYS") && write {
		// can not publish '$SYS/#'
		return false
	}
	return true
}

func (s *mqtt2) onPublished(cl *mqtt.Client, pk packets.Packet) {
	defer func() {
		if r := recover(); r != nil {
			s.log.Warn("panic", "error", r)
		}
	}()
	// s.log.Tracef("%s published %s", cl.Net.Remote, pk.TopicName)
	if pk.TopicName == "db/query" {
		s.handleQuery(cl, pk)
	} else if strings.HasPrefix(pk.TopicName, "db/write/") {
		s.handleWrite(cl, pk)
	} else if strings.HasPrefix(pk.TopicName, "db/append/") {
		s.handleAppend(cl, pk)
	} else if strings.HasPrefix(pk.TopicName, "db/metrics/") {
		s.handleMetrics(cl, pk)
	} else if strings.HasPrefix(pk.TopicName, "db/tql/") {
		s.handleTql(cl, pk)
	}
}

func (s *mqtt2) onDisconnect(cl *mqtt.Client, err error, expire bool) {
	s.log.Debugf("%s disconnected, err=%v, expired=%t", cl.Net.Remote, err, expire)
	peerId := cl.Net.Remote
	s.appenders.RemoveCb(peerId, func(key string, v interface{}, exists bool) bool {
		if !exists {
			return false
		}
		appenders := v.([]*AppenderWrapper)
		for _, aw := range appenders {
			succ, fail, err := aw.appender.Close()
			s.log.Debugf("%s close appender %s, succ=%d, fail=%d, err=%v", peerId, aw.appender.TableName(), succ, fail, err)
			aw.conn.Close()
			aw.ctxCancel()
		}
		return true
	})
}

type AuthHook struct {
	mqtt.HookBase
	svr *mqtt2
}

// ID returns the ID of the hook.
func (h *AuthHook) ID() string {
	return "auth-mqtt2"
}

// Provides indicates which hook methods this hook provides.
func (h *AuthHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnectAuthenticate,
		mqtt.OnACLCheck,
		mqtt.OnConnect,
		mqtt.OnPublished,
		mqtt.OnDisconnect,
	}, []byte{b})
}

// Init configures the hook with the auth ledger to be used for checking.
func (h *AuthHook) Init(config any) error {
	return nil
}

// OnConnectAuthenticate returns true if the connecting client has rules which provide access
// in the auth ledger.
func (h *AuthHook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	if h.svr.enableTokenAuth {
		if h.svr.authServer == nil {
			h.svr.log.Warn("token auth is enabled but auth server is not set.")
			return false
		}
		clientId := cl.ID
		username := string(pk.Connect.Username) // contains token
		h.svr.log.Tracef("MQTT auth '%s' token '%s'", clientId, username)
		if !strings.HasPrefix(username, clientId) {
			return false
		}
		pass, err := h.svr.authServer.ValidateClientToken(username)
		if err != nil {
			h.svr.log.Warn("fail to validate token", err)
			return false
		}
		if !pass {
			return false
		}
	}
	return true
}

// OnACLCheck returns true if the connecting client has matching read or write access to subscribe
// or publish to a given topic.
func (h *AuthHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	return h.svr.onACLCheck(cl, topic, write)
}

func (h *AuthHook) OnConnect(cl *mqtt.Client, pk packets.Packet) error {
	if conn, ok := cl.Net.Conn.(*net.TCPConn); ok {
		configureTcpConn(conn)
	}
	return nil
}

func (h *AuthHook) OnPublished(cl *mqtt.Client, pk packets.Packet) {
	h.svr.onPublished(cl, pk)
}

func (h *AuthHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	h.svr.onDisconnect(cl, err, expire)
}

func parseTimeLocation(str string, def *time.Location) *time.Location {
	if str == "" {
		return def
	}
	tz := strings.ToLower(str)
	if tz == "local" {
		return time.Local
	} else if tz == "utc" {
		return time.UTC
	} else {
		if loc, err := util.GetTimeLocation(str); err != nil {
			loc, err := time.LoadLocation(str)
			if err != nil {
				return def
			}
			return loc
		} else {
			return loc
		}
	}
}
