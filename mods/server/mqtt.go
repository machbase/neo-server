package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	badgerdb "github.com/dgraph-io/badger/v4"
	"github.com/gorilla/websocket"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/tql"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/storage/badger"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

type MqttOption func(s *mqttd) error

func NewMqtt(db api.Database, opts ...MqttOption) (*mqttd, error) {
	log := logging.GetLog("mqttd")

	caps := mqtt.NewDefaultServerCapabilities()
	svr := &mqttd{
		log: log,
		db:  db,
		broker: mqtt.New(&mqtt.Options{
			Logger:                 logging.Wrap(log, logFilter),
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
	svr.broker.Info.Version = strings.TrimPrefix(mods.DisplayVersion(), "v")
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
			return nil, fmt.Errorf("fail to load ca key: %s, %s", certFile, err.Error())
		}
		if ok := rootCAs.AppendCertsFromPEM(ca); !ok {
			return nil, fmt.Errorf("fail to add ca key: %s", certFile)
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

func WithMqttWsHandleListener(httpListeners []string) MqttOption {
	return func(s *mqttd) error {
		s.wsListener = &WsListener{svr: s, id: "mqtt2-ws", httpListeners: httpListeners}
		return s.broker.AddListener(s.wsListener)
	}
}

// WithMqttTcpListener creates a new TCP listener with the given address and TLS configuration.
// If tlsConfig is nil, the listener will not use TLS.
func WithMqttTcpListener(addr string, tlsConfig *tls.Config) MqttOption {
	return func(s *mqttd) error {
		qty := s.broker.Listeners.Len()
		id := fmt.Sprintf("mqtt-tcp-%d", qty)
		if tlsConfig != nil {
			id = fmt.Sprintf("mqtt-tls-%d", qty)
		}
		tcp := listeners.NewTCP(listeners.Config{
			ID:        id,
			Address:   addr,
			TLSConfig: tlsConfig,
		})
		return s.broker.AddListener(tcp)
	}
}

func WithMqttUnixSockListener(addr string) MqttOption {
	return func(s *mqttd) error {
		qty := s.broker.Listeners.Len()
		id := fmt.Sprintf("mqtt-unix-%d", qty)
		lsnr := listeners.NewUnixSock(listeners.Config{
			ID:      id,
			Address: addr,
		})
		return s.broker.AddListener(lsnr)
	}
}

// WithMqttWebsocketListener creates a new Websocket listener with the given address and TLS configuration.
// If tlsConfig is nil, the listener will not use TLS.
func WithMqttWebsocketListener(addr string, tlsConfig *tls.Config) MqttOption {
	return func(s *mqttd) error {
		qty := s.broker.Listeners.Len()
		id := fmt.Sprintf("mqtt-ws-%d", qty)
		if tlsConfig != nil {
			id = fmt.Sprintf("mqtt-wss-%d", qty)
		}
		ws := listeners.NewWebsocket(listeners.Config{
			ID:        id,
			Address:   addr,
			TLSConfig: tlsConfig,
		})
		return s.broker.AddListener(ws)
	}
}

func WithMqttAuthServer(authSvc AuthServer, enableTokenAuth bool) MqttOption {
	return func(s *mqttd) error {
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

func WithMqttBadgerPersistent(badgerPath string) MqttOption {
	return func(s *mqttd) error {
		badgerOpts := badgerdb.DefaultOptions(badgerPath) // BadgerDB options. Adjust according to your actual scenario.
		badgerOpts.ValueLogFileSize = 100 * (1 << 20)     // Set the default size of the log file to 100 MB.
		// AddHook adds a BadgerDB hook to the server with the specified options.
		// Refer to https://dgraph.io/docs/badger/get-started/#garbage-collection for more information.
		hook := &badger.Hook{}
		err := s.broker.AddHook(hook, &badger.Options{
			Path: badgerPath,
			// GcInterval specifies the interval at which BadgerDB garbage collection process runs.
			GcInterval: 5 * 60,
			// GcDiscardRatio specifies the ratio of log discard compared to the maximum possible log discard.
			// Setting it to a higher value would result in fewer space reclaims, while setting it to a lower value
			// would result in more space reclaims at the cost of increased activity on the LSM tree.
			// discardRatio must be in the range (0.0, 1.0), both endpoints excluded, otherwise, it will be set to the default value of 0.5.
			GcDiscardRatio: 0.5,
			Options:        &badgerOpts,
		})
		if err == nil {
			hook.Log = logging.Wrap(logging.GetLog("mqttd-persist"), logFilter)
		}
		return err
	}
}

func WithMqttTqlLoader(loader tql.Loader) MqttOption {
	return func(s *mqttd) error {
		s.tqlLoader = loader
		return nil
	}
}

// WithMqttMaxMessageSizeLimit sets the maximum message size allowed for incoming messages.
// If zero, no limit is enforced.
func WithMqttMaxMessageSizeLimit(limit int) MqttOption {
	return func(s *mqttd) error {
		s.broker.Options.Capabilities.MaximumPacketSize = uint32(limit)
		return nil
	}
}

type mqttd struct {
	log               logging.Log
	db                api.Database
	broker            *mqtt.Server
	authServer        AuthServer
	enableTokenAuth   bool
	tqlLoader         tql.Loader
	defaultReplyTopic string
	wsListener        *WsListener
	restrictTopics    bool
}

func (s *mqttd) Start() error {
	go s.broker.Serve()
	return nil
}

func (s *mqttd) Stop() {
	if s.broker != nil {
		s.broker.Close()
	}
}

func (s *mqttd) WsHandlerFunc() func(w http.ResponseWriter, r *http.Request) {
	return s.wsListener.WsHandler
}

func (s *mqttd) onACLCheck(_ *mqtt.Client, topic string, write bool) bool {
	if s.restrictTopics {
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
		}
	}
	if strings.HasPrefix(topic, "$SYS") && write {
		// can not publish '$SYS/#'
		return false
	}
	return true
}

func (s *mqttd) onPublished(cl *mqtt.Client, pk packets.Packet) {
	defer func() {
		if r := recover(); r != nil {
			s.log.Warn("panic", "onPublished", r)
		}
	}()
	if pk.TopicName == "db/query" {
		s.handleQuery(cl, pk)
	} else if strings.HasPrefix(pk.TopicName, "db/write/") {
		useAppend := false
		if pk.ProtocolVersion == 5 {
			for _, p := range pk.Properties.User {
				if p.Key == "method" && p.Val == "append" {
					useAppend = true
					break
				}
			}
		}
		if useAppend {
			s.handleAppend(cl, pk)
		} else {
			s.handleWrite(cl, pk)
		}
	} else if strings.HasPrefix(pk.TopicName, "db/append/") {
		s.handleAppend(cl, pk)
	} else if strings.HasPrefix(pk.TopicName, "db/metrics/") {
		s.handleMetrics(cl, pk)
	} else if strings.HasPrefix(pk.TopicName, "db/tql/") {
		s.handleTql(cl, pk)
	}
}

func (s *mqttd) onConnect(cl *mqtt.Client, pk packets.Packet) error {
	s.log.Debugf("%s connected listener=%s v=%d", cl.Net.Remote, cl.Net.Listener, pk.ProtocolVersion)
	if conn, ok := cl.Net.Conn.(*net.TCPConn); ok {
		configureTcpConn(conn)
	}
	return nil
}

func (s *mqttd) onDisconnect(cl *mqtt.Client, err error, expire bool) {
	s.log.Debugf("%s disconnected listener=%s expired=%t err=%v", cl.Net.Remote, cl.Net.Listener, expire, err)
}

type AuthHook struct {
	mqtt.HookBase
	svr *mqttd
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
		mqtt.OnPacketEncode,
	}, []byte{b})
}

// Init configures the hook with the auth ledger to be used for checking.
func (h *AuthHook) Init(config any) error {
	return nil
}

func (h *AuthHook) OnPacketEncode(cl *mqtt.Client, pk packets.Packet) packets.Packet {
	if pk.FixedHeader.Type == packets.Puback {
		// investigate the reason code of the puback packet
		// why it is not 0
		if pk.ReasonCode == 1 {
			pk.ReasonCode = 0
		}
	}
	return pk
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
	return h.svr.onConnect(cl, pk)
}

func (h *AuthHook) OnPublished(cl *mqtt.Client, pk packets.Packet) {
	h.svr.onPublished(cl, pk)
}

func (h *AuthHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	h.svr.onDisconnect(cl, err, expire)
}

type WsListener struct {
	sync.RWMutex
	svr           *mqttd
	id            string
	httpListeners []string
	log           *slog.Logger
	upgrader      *websocket.Upgrader
	establish     listeners.EstablishFn
}

var _ listeners.Listener = (*WsListener)(nil)

func (l *WsListener) ID() string {
	return l.id
}

func (l *WsListener) Address() string {
	for _, addr := range l.httpListeners {
		tok := strings.SplitN(addr, "://", 2)
		if len(tok) == 2 {
			if tok[0] != "tcp" {
				continue
			}
			return fmt.Sprintf("%s/web/api/mqtt", tok[1])
		} else {
			return fmt.Sprintf("%s/web/api/mqtt", tok[0])
		}
	}
	return ""
}

func (l *WsListener) Protocol() string {
	return "ws"
}

func (l *WsListener) Init(log *slog.Logger) error {
	l.upgrader = &websocket.Upgrader{
		Subprotocols: []string{"mqtt"},
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	return nil
}
func (l *WsListener) Close(closeClients listeners.CloseFn) {
	l.Lock()
	defer l.Unlock()

	closeClients(l.id)
}

func (l *WsListener) Serve(establish listeners.EstablishFn) {
	l.establish = establish
}

func (l *WsListener) WsHandler(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			l.log.Warn("panic", "error", r)
		}
	}()
	c, err := l.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer c.Close()

	err = l.establish(l.id, &wsConn{Conn: c.UnderlyingConn(), c: c})
	if err != nil {
		l.log.Warn("mqtt-ws", "error", err)
	}
}

type wsConn struct {
	net.Conn
	c *websocket.Conn

	// reader for the current message (can be nil)
	r io.Reader
}

// Read reads the next span of bytes from the websocket connection and returns the number of bytes read.
func (ws *wsConn) Read(p []byte) (int, error) {
	if ws.r == nil {
		op, r, err := ws.c.NextReader()
		if err != nil {
			return 0, err
		}
		if op != websocket.BinaryMessage {
			err = listeners.ErrInvalidMessage
			return 0, err
		}
		ws.r = r
	}

	var n int
	for {
		// buffer is full, return what we've read so far
		if n == len(p) {
			return n, nil
		}
		br, err := ws.r.Read(p[n:])
		n += br
		if err != nil {
			// when ANY error occurs, we consider this the end of the current message (either because it really is, via
			// io.EOF, or because something bad happened, in which case we want to drop the remainder)
			ws.r = nil

			if errors.Is(err, io.EOF) {
				err = nil
			}
			return n, err
		}
	}
}

// Write writes bytes to the websocket connection.
func (ws *wsConn) Write(p []byte) (int, error) {
	err := ws.c.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close signals the underlying websocket conn to close.
func (ws *wsConn) Close() error {
	return ws.Conn.Close()
}

func logFilter(name string, ctx context.Context, r slog.Record) bool {
	if !slices.Contains([]string{"mqttd", "mqttd-persist"}, name) {
		return true
	}
	if name == "mqttd" {
		if strings.Contains(r.Message, "mqtt starting") || strings.Contains(r.Message, "mqtt server st") {
			return false
		}
		r.Attrs(func(a slog.Attr) bool {
			if err, ok := a.Value.Any().(error); ok {
				msg := err.Error()
				if strings.Contains(msg, "use of closed network") {
					r.Level = slog.LevelDebug
				} else if strings.Contains(msg, "i/o timeout") {
					r.Level = slog.LevelDebug
				} else if err == io.EOF {
					return false
				}
			}
			return true
		})
	} else if name == "mqttd-persist" {
		if r.Level < slog.LevelInfo {
			return false
		}
	}
	return true
}
